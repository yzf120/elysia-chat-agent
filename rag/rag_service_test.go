package rag

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/yzf120/elysia-chat-agent/model"
)

// ==================== 1.8 extractKeywords ====================

func TestExtractKeywords(t *testing.T) {
	tests := []struct {
		name        string
		text        string
		wantMinLen  int
		shouldExist []string // 期望包含的关键词
		shouldNotIn []string // 期望不包含的关键词（停用词）
	}{
		{
			name:        "中文文本_提取关键词",
			text:        "动态规划 背包问题 怎么做",
			wantMinLen:  1,
			shouldExist: []string{"动态规划", "背包问题"},
			shouldNotIn: []string{"怎么"},
		},
		{
			name:        "英文文本_提取关键词",
			text:        "binary search algorithm implementation",
			wantMinLen:  2,
			shouldExist: []string{"binary", "search", "algorithm", "implementation"},
		},
		{
			name:        "停用词过滤",
			text:        "我 的 是 在 动态规划",
			wantMinLen:  1,
			shouldExist: []string{"动态规划"},
			shouldNotIn: []string{"我", "的", "是", "在"},
		},
		{
			name:       "去重",
			text:       "动态规划 动态规划 背包",
			wantMinLen: 2,
		},
		{
			name:       "空文本",
			text:       "",
			wantMinLen: 0,
		},
		{
			name:        "中文标点替换",
			text:        "动态规划，背包问题。怎么做？",
			wantMinLen:  1,
			shouldExist: []string{"动态规划", "背包问题"},
		},
		{
			name:        "英文标点替换",
			text:        "binary,search.algorithm?implementation!",
			wantMinLen:  2,
			shouldExist: []string{"binary", "search", "algorithm", "implementation"},
		},
		{
			name:        "混合中英文",
			text:        "请问 DFS 和 BFS 有什么区别",
			wantMinLen:  2,
			shouldExist: []string{"DFS", "BFS"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			kws := extractKeywords(tt.text)
			assert.GreaterOrEqual(t, len(kws), tt.wantMinLen, "关键词数量不足")

			for _, expected := range tt.shouldExist {
				found := false
				for _, kw := range kws {
					if kw == expected {
						found = true
						break
					}
				}
				assert.True(t, found, "应包含关键词 %q，实际: %v", expected, kws)
			}

			for _, notExpected := range tt.shouldNotIn {
				found := false
				for _, kw := range kws {
					if kw == notExpected {
						found = true
						break
					}
				}
				assert.False(t, found, "不应包含停用词 %q，实际: %v", notExpected, kws)
			}
		})
	}
}

func TestExtractKeywords_NoDuplicates(t *testing.T) {
	kws := extractKeywords("动态规划 动态规划 动态规划 背包")
	seen := make(map[string]bool)
	for _, kw := range kws {
		assert.False(t, seen[kw], "关键词 %q 重复出现", kw)
		seen[kw] = true
	}
}

// ==================== 1.9 calculateRelevance ====================

func TestCalculateRelevance(t *testing.T) {
	entry := &knowledgeEntry{
		Content:  "动态规划是一种算法设计方法，背包问题是经典应用",
		Keywords: []string{"动态规划", "背包问题", "算法"},
		Tags:     []string{"动态规划", "算法"},
	}

	t.Run("全命中_高分", func(t *testing.T) {
		score := calculateRelevance("动态规划 背包问题", []string{"动态规划", "背包问题"}, entry, 2)
		assert.Greater(t, score, 0.5, "全命中应得高分")
		assert.LessOrEqual(t, score, 1.0, "分数不应超过 1.0")
	})

	t.Run("部分命中_中等分", func(t *testing.T) {
		score := calculateRelevance("动态规划 排序", []string{"动态规划", "排序"}, entry, 1)
		assert.Greater(t, score, 0.0, "部分命中应得正分")
	})

	t.Run("全命中高于部分命中", func(t *testing.T) {
		fullScore := calculateRelevance("动态规划 背包问题", []string{"动态规划", "背包问题"}, entry, 2)
		partialScore := calculateRelevance("动态规划 排序", []string{"动态规划", "排序"}, entry, 1)
		assert.Greater(t, fullScore, partialScore, "全命中分数应高于部分命中")
	})

	t.Run("零命中_低分", func(t *testing.T) {
		score := calculateRelevance("网络协议 TCP", []string{"网络协议", "TCP"}, entry, 0)
		assert.Less(t, score, 0.3, "零命中应得低分")
	})

	t.Run("空关键词_零分", func(t *testing.T) {
		score := calculateRelevance("", []string{}, entry, 0)
		assert.Equal(t, 0.0, score, "空关键词应得零分")
	})

	t.Run("标签匹配加分", func(t *testing.T) {
		entryWithTags := &knowledgeEntry{
			Content:  "一些内容",
			Keywords: []string{"测试"},
			Tags:     []string{"动态规划"},
		}
		scoreWithTag := calculateRelevance("动态规划", []string{"动态规划"}, entryWithTags, 0)
		entryNoTags := &knowledgeEntry{
			Content:  "一些内容",
			Keywords: []string{"测试"},
			Tags:     []string{"无关标签"},
		}
		scoreNoTag := calculateRelevance("动态规划", []string{"动态规划"}, entryNoTags, 0)
		assert.Greater(t, scoreWithTag, scoreNoTag, "标签匹配应加分")
	})
}

// ==================== 1.10 FormatRAGContext ====================

func TestFormatRAGContext(t *testing.T) {
	t.Run("nil文档列表", func(t *testing.T) {
		result := FormatRAGContext(nil)
		assert.Equal(t, "", result)
	})

	t.Run("空文档列表", func(t *testing.T) {
		result := FormatRAGContext([]model.RAGDocument{})
		assert.Equal(t, "", result)
	})

	t.Run("单个文档", func(t *testing.T) {
		docs := []model.RAGDocument{
			{Content: "动态规划基础知识", SourceType: "knowledge_base", Score: 0.9},
		}
		result := FormatRAGContext(docs)
		assert.Contains(t, result, "动态规划基础知识")
		assert.Contains(t, result, "90%")
		assert.Contains(t, result, "knowledge_base")
		assert.Contains(t, result, "参考资料 1")
	})

	t.Run("多个文档_按序号排列", func(t *testing.T) {
		docs := []model.RAGDocument{
			{Content: "内容1", SourceType: "knowledge_base", Score: 0.9},
			{Content: "内容2", SourceType: "problem_bank", Score: 0.7},
			{Content: "内容3", SourceType: "error_pattern", Score: 0.5},
		}
		result := FormatRAGContext(docs)
		assert.Contains(t, result, "参考资料 1")
		assert.Contains(t, result, "参考资料 2")
		assert.Contains(t, result, "参考资料 3")
		assert.Contains(t, result, "内容1")
		assert.Contains(t, result, "内容2")
		assert.Contains(t, result, "内容3")
		assert.Contains(t, result, "90%")
		assert.Contains(t, result, "70%")
		assert.Contains(t, result, "50%")
	})
}
