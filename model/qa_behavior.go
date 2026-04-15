package model

import "time"

// ==================== 问答行为记录 ====================

// QABehavior 学生问答行为记录（对应 student_qa_behavior 表）
type QABehavior struct {
	Id                int64     `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	StudentId         string    `gorm:"column:student_id" json:"student_id"`
	ConversationId    string    `gorm:"column:conversation_id" json:"conversation_id"`
	ProblemId         int64     `gorm:"column:problem_id" json:"problem_id"`
	IntentCode        string    `gorm:"column:intent_code" json:"intent_code"`
	QuestionSummary   string    `gorm:"column:question_summary" json:"question_summary"`
	KnowledgeTags     string    `gorm:"column:knowledge_tags" json:"knowledge_tags"` // JSON 数组
	DifficultyScore   float64   `gorm:"column:difficulty_score" json:"difficulty_score"`
	IsResolved        int       `gorm:"column:is_resolved" json:"is_resolved"` // 0-未知 1-已解决 2-未解决
	ConversationTurns int       `gorm:"column:conversation_turns" json:"conversation_turns"`
	ConversationTime  time.Time `gorm:"column:conversation_time" json:"conversation_time"`
	CreateTime        time.Time `gorm:"column:create_time;autoCreateTime" json:"create_time"`
}

func (QABehavior) TableName() string {
	return "student_qa_behavior"
}

// QABehaviorSummary 问答行为摘要（用于注入 AI 提示词的最近 N 条记录）
type QABehaviorSummary struct {
	QuestionSummary   string   `json:"question_summary"`
	KnowledgeTags     []string `json:"knowledge_tags"`
	DifficultyScore   float64  `json:"difficulty_score"`
	IntentCode        string   `json:"intent_code"`
	IsResolved        int      `json:"is_resolved"`
	ConversationTurns int      `json:"conversation_turns"`
	ConversationTime  string   `json:"conversation_time"`
}

// QAProfileAnalysis 问答画像 Agent 的分析结果
type QAProfileAnalysis struct {
	QuestionSummary string   `json:"question_summary"` // 问题摘要
	KnowledgeTags   []string `json:"knowledge_tags"`   // 涉及知识点标签（2-4个）
	IsResolved      int      `json:"is_resolved"`      // 问题是否解决 0-未知 1-已解决 2-未解决
}
