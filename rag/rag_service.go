package rag

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/yzf120/elysia-chat-agent/client"
	"github.com/yzf120/elysia-chat-agent/model"
)

// ==================== RAG 服务 ====================

// Service RAG 检索服务
type Service struct {
	redisClient *redis.Client
	keyPrefix   string
}

var defaultService *Service

// InitRAGService 初始化 RAG 服务
func InitRAGService() {
	rc := client.GetRedisClient()
	if rc == nil {
		log.Println("[RAG] 警告: Redis 客户端未初始化，RAG 服务将不可用")
		return
	}
	defaultService = &Service{
		redisClient: rc.Client,
		keyPrefix:   "elysia:knowledge:",
	}
	log.Println("[RAG] RAG 检索服务初始化完成")
}

// GetRAGService 获取 RAG 服务实例
func GetRAGService() *Service {
	return defaultService
}

// ==================== 知识存储 ====================

// knowledgeEntry Redis 中存储的知识条目
type knowledgeEntry struct {
	ID         string   `json:"id"`
	Content    string   `json:"content"`
	SourceType string   `json:"source_type"` // knowledge_base / problem_bank / error_pattern
	SourceID   string   `json:"source_id"`
	Tags       []string `json:"tags"`
	Keywords   []string `json:"keywords"` // 用于关键词匹配的词列表
	CreatedAt  string   `json:"created_at"`
}

// StoreKnowledge 存储知识到 Redis
func (s *Service) StoreKnowledge(ctx context.Context, doc *model.RAGDocument) error {
	if s == nil || s.redisClient == nil {
		return fmt.Errorf("RAG 服务未初始化")
	}

	// 提取关键词（简单分词）
	keywords := extractKeywords(doc.Content)
	tags := strings.Split(doc.Tags, ",")

	entry := knowledgeEntry{
		ID:         doc.ID,
		Content:    doc.Content,
		SourceType: doc.SourceType,
		SourceID:   doc.SourceID,
		Tags:       tags,
		Keywords:   keywords,
		CreatedAt:  time.Now().Format(time.RFC3339),
	}

	data, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("序列化知识条目失败: %w", err)
	}

	key := s.keyPrefix + doc.ID
	if err := s.redisClient.Set(ctx, key, data, 0).Err(); err != nil {
		return fmt.Errorf("存储知识条目失败: %w", err)
	}

	// 将 ID 添加到索引集合中
	indexKey := s.keyPrefix + "index:" + doc.SourceType
	s.redisClient.SAdd(ctx, indexKey, doc.ID)

	// 为每个关键词建立倒排索引
	for _, kw := range keywords {
		kwKey := s.keyPrefix + "kw:" + strings.ToLower(kw)
		s.redisClient.SAdd(ctx, kwKey, doc.ID)
	}

	// 为每个标签建立索引
	for _, tag := range tags {
		tag = strings.TrimSpace(tag)
		if tag != "" {
			tagKey := s.keyPrefix + "tag:" + strings.ToLower(tag)
			s.redisClient.SAdd(ctx, tagKey, doc.ID)
		}
	}

	return nil
}

// ==================== 知识检索 ====================

// Retrieve 检索相关知识
func (s *Service) Retrieve(ctx context.Context, query *model.RAGQuery) ([]model.RAGDocument, error) {
	if s == nil || s.redisClient == nil {
		log.Println("[RAG] RAG 服务未初始化，返回空结果")
		return nil, nil
	}

	if query.TopK <= 0 {
		query.TopK = 5
	}
	if query.Threshold <= 0 {
		query.Threshold = 0.3
	}

	// 提取查询关键词
	queryKeywords := extractKeywords(query.Query)
	if len(query.Keywords) > 0 {
		queryKeywords = append(queryKeywords, query.Keywords...)
	}

	// 通过倒排索引找到候选文档 ID
	candidateIDs := make(map[string]int) // docID -> 命中关键词数
	for _, kw := range queryKeywords {
		kwKey := s.keyPrefix + "kw:" + strings.ToLower(kw)
		ids, err := s.redisClient.SMembers(ctx, kwKey).Result()
		if err != nil {
			continue
		}
		for _, id := range ids {
			candidateIDs[id]++
		}
	}

	// 如果指定了来源类型，从索引集合中获取候选
	if query.SourceType != "" {
		indexKey := s.keyPrefix + "index:" + query.SourceType
		ids, err := s.redisClient.SMembers(ctx, indexKey).Result()
		if err == nil {
			for _, id := range ids {
				if _, exists := candidateIDs[id]; !exists {
					candidateIDs[id] = 0
				}
			}
		}
	}

	if len(candidateIDs) == 0 {
		return nil, nil
	}

	// 获取候选文档并计算相关度
	type scoredDoc struct {
		doc   model.RAGDocument
		score float64
	}
	var results []scoredDoc

	for docID, hitCount := range candidateIDs {
		key := s.keyPrefix + docID
		data, err := s.redisClient.Get(ctx, key).Result()
		if err != nil {
			continue
		}

		var entry knowledgeEntry
		if err := json.Unmarshal([]byte(data), &entry); err != nil {
			continue
		}

		// 计算相关度评分（基于关键词匹配 + 内容相似度）
		score := calculateRelevance(query.Query, queryKeywords, &entry, hitCount)

		if score >= query.Threshold {
			results = append(results, scoredDoc{
				doc: model.RAGDocument{
					ID:         entry.ID,
					Content:    entry.Content,
					SourceType: entry.SourceType,
					SourceID:   entry.SourceID,
					Tags:       strings.Join(entry.Tags, ","),
					Score:      score,
				},
				score: score,
			})
		}
	}

	// 按相关度排序
	sort.Slice(results, func(i, j int) bool {
		return results[i].score > results[j].score
	})

	// 取 TopK
	if len(results) > query.TopK {
		results = results[:query.TopK]
	}

	docs := make([]model.RAGDocument, len(results))
	for i, r := range results {
		docs[i] = r.doc
	}

	return docs, nil
}

// FormatRAGContext 将检索结果格式化为上下文字符串
func FormatRAGContext(docs []model.RAGDocument) string {
	if len(docs) == 0 {
		return ""
	}

	var sb strings.Builder
	for i, doc := range docs {
		sb.WriteString(fmt.Sprintf("### 参考资料 %d（相关度: %.0f%%，来源: %s）\n",
			i+1, doc.Score*100, doc.SourceType))
		sb.WriteString(doc.Content)
		sb.WriteString("\n\n")
	}
	return sb.String()
}

// ==================== 辅助函数 ====================

// extractKeywords 从文本中提取关键词（简单实现：按空格和标点分词，过滤停用词）
func extractKeywords(text string) []string {
	// 替换常见标点为空格
	replacer := strings.NewReplacer(
		"\uff0c", " ", "\u3002", " ", "\uff1f", " ", "\uff01", " ",
		"\u3001", " ", "\uff1a", " ", "\uff1b", " ", "\u201c", " ",
		"\u201d", " ", "\u2018", " ", "\u2019", " ", "\uff08", " ",
		"\uff09", " ", "\u3010", " ", "\u3011", " ", "\n", " ",
		"\t", " ", ",", " ", ".", " ", "?", " ",
		"!", " ", ":", " ", ";", " ", "(", " ",
		")", " ", "[", " ", "]", " ",
	)
	text = replacer.Replace(text)

	words := strings.Fields(text)

	// 停用词表
	stopWords := map[string]bool{
		"的": true, "了": true, "是": true, "在": true, "我": true,
		"有": true, "和": true, "就": true, "不": true, "人": true,
		"都": true, "一": true, "一个": true, "上": true, "也": true,
		"很": true, "到": true, "说": true, "要": true, "去": true,
		"你": true, "会": true, "着": true, "没有": true, "看": true,
		"好": true, "自己": true, "这": true, "他": true, "她": true,
		"吗": true, "吧": true, "呢": true, "啊": true, "哦": true,
		"the": true, "a": true, "an": true, "is": true, "are": true,
		"was": true, "were": true, "be": true, "been": true, "being": true,
		"have": true, "has": true, "had": true, "do": true, "does": true,
		"did": true, "will": true, "would": true, "could": true, "should": true,
		"may": true, "might": true, "can": true, "shall": true,
		"i": true, "me": true, "my": true, "we": true, "our": true,
		"you": true, "your": true, "he": true, "him": true, "his": true,
		"she": true, "her": true, "it": true, "its": true, "they": true,
		"them": true, "their": true, "this": true, "that": true,
		"帮我": true, "请问": true, "怎么": true, "什么": true, "为什么": true,
		"如何": true, "能不能": true, "可以": true, "帮": true, "请": true,
	}

	var keywords []string
	seen := make(map[string]bool)
	for _, w := range words {
		w = strings.TrimSpace(w)
		lower := strings.ToLower(w)
		if len(w) == 0 || stopWords[lower] || seen[lower] {
			continue
		}
		seen[lower] = true
		keywords = append(keywords, w)
	}

	return keywords
}

// calculateRelevance 计算查询与文档的相关度
func calculateRelevance(query string, queryKeywords []string, entry *knowledgeEntry, hitCount int) float64 {
	if len(queryKeywords) == 0 {
		return 0
	}

	// 1. 关键词命中率（权重 0.6）
	keywordScore := float64(hitCount) / float64(len(queryKeywords))
	if keywordScore > 1.0 {
		keywordScore = 1.0
	}

	// 2. 内容包含度（权重 0.3）：查询关键词在文档内容中出现的比例
	contentLower := strings.ToLower(entry.Content)
	containCount := 0
	for _, kw := range queryKeywords {
		if strings.Contains(contentLower, strings.ToLower(kw)) {
			containCount++
		}
	}
	contentScore := float64(containCount) / float64(len(queryKeywords))

	// 3. 标签匹配（权重 0.1）
	tagScore := 0.0
	for _, tag := range entry.Tags {
		tagLower := strings.ToLower(strings.TrimSpace(tag))
		for _, kw := range queryKeywords {
			if strings.Contains(tagLower, strings.ToLower(kw)) || strings.Contains(strings.ToLower(kw), tagLower) {
				tagScore = 1.0
				break
			}
		}
		if tagScore > 0 {
			break
		}
	}

	score := keywordScore*0.6 + contentScore*0.3 + tagScore*0.1

	// 归一化到 [0, 1]
	return math.Min(score, 1.0)
}
