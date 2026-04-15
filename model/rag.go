package model

// ==================== RAG 相关 ====================

// RAGDocument RAG 检索到的文档
type RAGDocument struct {
	ID         string  `json:"id"`
	Content    string  `json:"content"`     // 文档内容
	SourceType string  `json:"source_type"` // knowledge_base / problem_bank / error_pattern
	SourceID   string  `json:"source_id"`   // 关联的知识点/题目 ID
	Tags       string  `json:"tags"`        // 标签
	Score      float64 `json:"score"`       // 相关度评分
}

// RAGQuery RAG 检索请求
type RAGQuery struct {
	Query      string   `json:"query"`       // 检索查询
	Keywords   []string `json:"keywords"`    // 关键词
	TopK       int      `json:"top_k"`       // 返回数量
	Threshold  float64  `json:"threshold"`   // 相关度阈值
	SourceType string   `json:"source_type"` // 限定来源类型
}
