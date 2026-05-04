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
	"unicode/utf8"

	"github.com/redis/go-redis/v9"
	"github.com/yzf120/elysia-chat-agent/client"
	"github.com/yzf120/elysia-chat-agent/dao"
	"github.com/yzf120/elysia-chat-agent/model"
)

// ==================== RAG 服务 ====================

// Service RAG 检索服务
type Service struct {
	redisClient *redis.Client
	keyPrefix   string
	docDAO      *dao.KnowledgeDocDAO
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
		docDAO:      dao.NewKnowledgeDocDAO(),
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

// StoreKnowledge 存储知识到 MySQL + Redis（双写）
func (s *Service) StoreKnowledge(ctx context.Context, doc *model.RAGDocument) error {
	if s == nil || s.redisClient == nil {
		return fmt.Errorf("RAG 服务未初始化")
	}

	// 使用中文分词器提取关键词（用于建立索引，更细粒度）
	tokenizer := GetTokenizer()
	keywords := tokenizer.TokenizeForIndex(doc.Content)
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

	// 将 ID 添加到全局文档列表
	s.redisClient.SAdd(ctx, s.keyPrefix+"all_docs", doc.ID)

	// 为每个关键词建立倒排索引（去重）
	seenKw := make(map[string]bool)
	for _, kw := range keywords {
		lower := strings.ToLower(kw)
		if seenKw[lower] {
			continue
		}
		seenKw[lower] = true
		kwKey := s.keyPrefix + "kw:" + lower
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

	log.Printf("[RAG] 知识条目已存储: id=%s, keywords=%d个", doc.ID, len(seenKw))
	return nil
}

// StoreKnowledgeDocument 存储完整文档（异步模式：同步写MySQL文档记录，异步处理分块+Redis索引）
func (s *Service) StoreKnowledgeDocument(ctx context.Context, docRecord *dao.KnowledgeDocument) error {
	if s == nil {
		return fmt.Errorf("RAG 服务未初始化")
	}

	// 1. 同步写入 MySQL 文档记录（状态=处理中），让前端立即可见
	docRecord.Status = 1 // 处理中
	if err := s.docDAO.CreateDocument(docRecord); err != nil {
		return fmt.Errorf("创建文档记录失败: %w", err)
	}

	// 2. 估算处理时间（基于内容长度）
	contentLen := utf8.RuneCountInString(docRecord.Content)
	estimatedSeconds := s.estimateProcessTime(contentLen)
	log.Printf("[RAG] 文档记录已创建: doc_id=%s, file=%s, 内容长度=%d字, 预计处理时间=%ds",
		docRecord.DocID, docRecord.FileName, contentLen, estimatedSeconds)

	// 3. 异步处理分块和Redis索引构建
	go s.asyncBuildIndex(docRecord)

	return nil
}

// estimateProcessTime 根据内容长度估算处理时间（秒）
func (s *Service) estimateProcessTime(contentLen int) int {
	// 每500字约1秒（分块+分词+Redis写入）
	seconds := contentLen / 500
	if seconds < 3 {
		seconds = 3
	}
	if seconds > 60 {
		seconds = 60
	}
	return seconds
}

// GetEstimatedProcessTime 获取预估处理时间（供外部调用）
func (s *Service) GetEstimatedProcessTime(contentLen int) int {
	return s.estimateProcessTime(contentLen)
}

// asyncBuildIndex 异步构建分块和Redis索引
func (s *Service) asyncBuildIndex(docRecord *dao.KnowledgeDocument) {
	ctx := context.Background()
	startTime := time.Now()

	log.Printf("[RAG] 开始异步构建索引: doc_id=%s, file=%s", docRecord.DocID, docRecord.FileName)

	// 1. 对文档内容进行分块
	chunks := splitContent(docRecord.Content, 500) // 每块约500字

	// 2. 每个分块写入 MySQL + Redis
	var dbChunks []dao.KnowledgeChunk
	for i, chunkContent := range chunks {
		chunkID := fmt.Sprintf("%s_chunk_%d", docRecord.DocID, i)

		// 写入 Redis 索引
		ragDoc := &model.RAGDocument{
			ID:         chunkID,
			Content:    chunkContent,
			SourceType: docRecord.SourceType,
			SourceID:   docRecord.SourceID,
			Tags:       docRecord.Tags,
		}
		if err := s.StoreKnowledge(ctx, ragDoc); err != nil {
			log.Printf("[RAG] 分块 %s 写入Redis失败: %v", chunkID, err)
		}

		// 准备 MySQL 分块记录
		dbChunks = append(dbChunks, dao.KnowledgeChunk{
			ChunkID:     chunkID,
			DocID:       docRecord.DocID,
			ChunkIndex:  i,
			Content:     chunkContent,
			TokensCount: utf8.RuneCountInString(chunkContent),
		})
	}

	// 3. 批量写入分块到 MySQL
	if err := s.docDAO.CreateChunks(dbChunks); err != nil {
		log.Printf("[RAG] 批量写入分块到MySQL失败: %v", err)
		s.docDAO.UpdateDocumentStatus(docRecord.DocID, 3, err.Error())
		return
	}

	// 4. 更新文档状态为已处理
	s.docDAO.UpdateDocumentChunkCount(docRecord.DocID, len(dbChunks))
	s.docDAO.UpdateDocumentStatus(docRecord.DocID, 2, "")

	duration := time.Since(startTime)
	log.Printf("[RAG] 异步索引构建完成: doc_id=%s, file=%s, chunks=%d, 耗时=%v",
		docRecord.DocID, docRecord.FileName, len(dbChunks), duration)
}

// DeleteKnowledgeDocument 删除文档（异步模式：同步删除MySQL文档记录，异步清理分块和Redis索引）
func (s *Service) DeleteKnowledgeDocument(ctx context.Context, docID string) error {
	if s == nil {
		return fmt.Errorf("RAG 服务未初始化")
	}

	// 1. 同步删除 MySQL 文档记录（用户刷新后立即看不到该文档）
	if err := s.docDAO.DeleteDocument(docID); err != nil {
		return fmt.Errorf("删除文档记录失败: %w", err)
	}

	log.Printf("[RAG] 文档记录已删除: doc_id=%s，开始异步清理分块和索引", docID)

	// 2. 异步清理分块（MySQL）和 Redis 索引
	go s.asyncCleanupChunksAndIndex(docID)

	return nil
}

// asyncCleanupChunksAndIndex 异步清理文档的分块记录和Redis索引
func (s *Service) asyncCleanupChunksAndIndex(docID string) {
	ctx := context.Background()
	startTime := time.Now()

	log.Printf("[RAG] 开始异步清理分块和索引: doc_id=%s", docID)

	// 1. 获取文档的所有分块（用于清理 Redis 索引）
	chunks, err := s.docDAO.GetChunksByDocID(docID)
	if err != nil {
		log.Printf("[RAG] 获取文档分块失败: doc_id=%s, err=%v", docID, err)
	}

	// 2. 从 Redis 中删除每个分块的索引
	redisDeleteCount := 0
	for _, chunk := range chunks {
		if err := s.DeleteKnowledge(ctx, chunk.ChunkID); err != nil {
			log.Printf("[RAG] 删除分块Redis索引失败: chunk_id=%s, err=%v", chunk.ChunkID, err)
		} else {
			redisDeleteCount++
		}
	}

	// 3. 删除 MySQL 中的分块记录
	if err := s.docDAO.DeleteChunksByDocID(docID); err != nil {
		log.Printf("[RAG] 删除MySQL分块记录失败: doc_id=%s, err=%v", docID, err)
	}

	duration := time.Since(startTime)
	log.Printf("[RAG] 异步清理完成: doc_id=%s, Redis索引删除 %d/%d, 耗时=%v",
		docID, redisDeleteCount, len(chunks), duration)
}

// ListKnowledgeDocuments 从 MySQL 分页查询文档列表
func (s *Service) ListKnowledgeDocuments(ctx context.Context, page, pageSize int, keyword string) ([]dao.KnowledgeDocument, int64, error) {
	if s == nil {
		return nil, 0, fmt.Errorf("RAG 服务未初始化")
	}
	return s.docDAO.ListDocuments(page, pageSize, keyword)
}

// splitContent 将文本按字数分块
func splitContent(content string, chunkSize int) []string {
	runes := []rune(content)
	if len(runes) <= chunkSize {
		return []string{content}
	}

	var chunks []string
	for i := 0; i < len(runes); i += chunkSize {
		end := i + chunkSize
		if end > len(runes) {
			end = len(runes)
		}
		// 尝试在句号、换行符处断句
		actualEnd := end
		if end < len(runes) {
			// 向后找最近的断句点（最多多看50字）
			for j := end; j < end+50 && j < len(runes); j++ {
				if runes[j] == '。' || runes[j] == '\n' || runes[j] == '.' {
					actualEnd = j + 1
					break
				}
			}
		}
		chunk := strings.TrimSpace(string(runes[i:actualEnd]))
		if chunk != "" {
			chunks = append(chunks, chunk)
		}
		if actualEnd > end {
			i = actualEnd - chunkSize // 调整下一次起始位置
		}
	}

	return chunks
}

// DeleteKnowledge 从 Redis 中删除知识条目（仅Redis索引）
func (s *Service) DeleteKnowledge(ctx context.Context, docID string) error {
	if s == nil || s.redisClient == nil {
		return fmt.Errorf("RAG 服务未初始化")
	}

	key := s.keyPrefix + docID

	// 先获取文档数据，以便清理倒排索引
	data, err := s.redisClient.Get(ctx, key).Result()
	if err != nil {
		// 文档不存在时不报错，可能已被删除
		log.Printf("[RAG] Redis中文档不存在，跳过: id=%s", docID)
		return nil
	}

	var entry knowledgeEntry
	if err := json.Unmarshal([]byte(data), &entry); err != nil {
		return fmt.Errorf("反序列化失败: %w", err)
	}

	// 删除倒排索引中的引用
	seenKw := make(map[string]bool)
	for _, kw := range entry.Keywords {
		lower := strings.ToLower(kw)
		if seenKw[lower] {
			continue
		}
		seenKw[lower] = true
		kwKey := s.keyPrefix + "kw:" + lower
		s.redisClient.SRem(ctx, kwKey, docID)
	}

	// 删除标签索引中的引用
	for _, tag := range entry.Tags {
		tag = strings.TrimSpace(tag)
		if tag != "" {
			tagKey := s.keyPrefix + "tag:" + strings.ToLower(tag)
			s.redisClient.SRem(ctx, tagKey, docID)
		}
	}

	// 从类型索引中移除
	indexKey := s.keyPrefix + "index:" + entry.SourceType
	s.redisClient.SRem(ctx, indexKey, docID)

	// 从全局文档列表中移除
	s.redisClient.SRem(ctx, s.keyPrefix+"all_docs", docID)

	// 删除文档本身
	s.redisClient.Del(ctx, key)

	log.Printf("[RAG] 知识条目已删除: id=%s", docID)
	return nil
}

// ListKnowledge 列出所有知识条目（分页）
func (s *Service) ListKnowledge(ctx context.Context, page, pageSize int) ([]model.RAGDocument, int, error) {
	if s == nil || s.redisClient == nil {
		return nil, 0, fmt.Errorf("RAG 服务未初始化")
	}

	// 获取所有文档 ID
	allIDs, err := s.redisClient.SMembers(ctx, s.keyPrefix+"all_docs").Result()
	if err != nil {
		return nil, 0, fmt.Errorf("获取文档列表失败: %w", err)
	}

	total := len(allIDs)
	if total == 0 {
		return nil, 0, nil
	}

	// 分页
	start := (page - 1) * pageSize
	if start >= total {
		return nil, total, nil
	}
	end := start + pageSize
	if end > total {
		end = total
	}
	pagedIDs := allIDs[start:end]

	var docs []model.RAGDocument
	for _, docID := range pagedIDs {
		key := s.keyPrefix + docID
		data, err := s.redisClient.Get(ctx, key).Result()
		if err != nil {
			continue
		}

		var entry knowledgeEntry
		if err := json.Unmarshal([]byte(data), &entry); err != nil {
			continue
		}

		docs = append(docs, model.RAGDocument{
			ID:         entry.ID,
			Content:    entry.Content,
			SourceType: entry.SourceType,
			SourceID:   entry.SourceID,
			Tags:       strings.Join(entry.Tags, ","),
		})
	}

	return docs, total, nil
}

// ==================== 知识检索 ====================

// Retrieve 检索相关知识
func (s *Service) Retrieve(ctx context.Context, query *model.RAGQuery) ([]model.RAGDocument, error) {
	if s == nil || s.redisClient == nil {
		log.Println("[RAG] RAG 服务未初始化，返回空结果")
		return nil, nil
	}

	retrieveStart := time.Now()

	if query.TopK <= 0 {
		query.TopK = 5
	}
	if query.Threshold <= 0 {
		query.Threshold = 0.3
	}

	// 使用中文分词器提取查询关键词（去停用词）
	tokenizeStart := time.Now()
	tokenizer := GetTokenizer()
	queryKeywords := tokenizer.TokenizeForQuery(query.Query)
	if len(query.Keywords) > 0 {
		queryKeywords = append(queryKeywords, query.Keywords...)
	}
	tokenizeDur := time.Since(tokenizeStart).Milliseconds()

	log.Printf("[RAG] 检索关键词: %v (分词耗时=%dms)", queryKeywords, tokenizeDur)

	// 通过倒排索引找到候选文档 ID
	indexStart := time.Now()
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
		log.Printf("[RAG] 候选文档为空，索引查询耗时=%dms", time.Since(indexStart).Milliseconds())
		return nil, nil
	}

	log.Printf("[RAG] 倒排索引查询完成: 候选文档=%d个, 耗时=%dms", len(candidateIDs), time.Since(indexStart).Milliseconds())

	// 获取候选文档并计算相关度
	scoreStart := time.Now()
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

	log.Printf("[RAG] 检索完成: 候选=%d, 过阈值后=%d, 返回TopK=%d, 评分耗时=%dms, 总耗时=%dms",
		len(candidateIDs), len(results), len(docs), time.Since(scoreStart).Milliseconds(), time.Since(retrieveStart).Milliseconds())

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

// extractKeywords 从文本中提取关键词（使用中文分词器）
func extractKeywords(text string) []string {
	tokenizer := GetTokenizer()
	return tokenizer.TokenizeForQuery(text)
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
