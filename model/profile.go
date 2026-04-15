package model

// ==================== 用户画像 ====================

// UserProfile 学生画像（做题画像 + 问答行为画像）
type UserProfile struct {
	// === 做题画像（从 student_profile 表加载）===
	DifficultyLevel       string         `json:"difficulty_level"`        // AI推断的能力等级：beginner/intermediate/advanced
	TotalSubmissions      int            `json:"total_submissions"`       // 总提交次数
	AcceptRate            float64        `json:"accept_rate"`             // 通过率
	SolvedProblemCount    int            `json:"solved_problem_count"`    // 已解决题目数
	AttemptedProblemCount int            `json:"attempted_problem_count"` // 尝试过的题目数
	PreferredLanguage     string         `json:"preferred_language"`      // 最常用编程语言
	LanguageStats         map[string]int `json:"language_stats"`          // 各编程语言使用次数
	CommonErrors          []string       `json:"common_errors"`           // 常见错误类型

	// === 问答行为画像（从 student_qa_behavior 最近10条记录加载）===
	RecentQABehaviors []QABehaviorSummary `json:"recent_qa_behaviors,omitempty"` // 最近10条问答行为摘要
}
