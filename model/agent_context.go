package model

// ==================== Agent 上下文 ====================

// AgentContext Agent 执行上下文，贯穿整个 ReAct 循环
type AgentContext struct {
	// 用户信息
	UserID   string `json:"user_id"`
	UserRole string `json:"user_role"` // student / teacher

	// 会话信息
	SessionID string `json:"session_id"`

	// 请求信息
	OriginalQuery string            `json:"original_query"` // 用户原始问题
	Messages      []ChatMessage     `json:"messages"`       // 完整对话历史
	ExtraParams   map[string]string `json:"extra_params"`   // 额外参数

	// 题目上下文（如果有）
	ProblemID    string `json:"problem_id,omitempty"`
	ProblemInfo  string `json:"problem_info,omitempty"`  // 题目描述
	StudentCode  string `json:"student_code,omitempty"`  // 学生代码
	JudgeResult  string `json:"judge_result,omitempty"`  // 判题结果
	FailedCases  string `json:"failed_cases,omitempty"`  // 未通过的测试用例（JSON 格式）
	Language     string `json:"language,omitempty"`      // 编程语言
	ErrorMessage string `json:"error_message,omitempty"` // 错误信息

	// 意图识别结果
	IntentResult *IntentResult `json:"intent_result,omitempty"`

	// RAG 检索结果
	RAGContext string `json:"rag_context,omitempty"`

	// 用户画像（AI 回答前参考，用于个性化教学策略）
	UserProfile *UserProfile `json:"user_profile,omitempty"`

	// 会话标识（用于问答行为记录）
	ConversationId string `json:"conversation_id,omitempty"`

	// 模型配置
	ModelID string `json:"model_id"`
}

// ChatMessage 对话消息
type ChatMessage struct {
	Role    string `json:"role"`    // system, user, assistant
	Content string `json:"content"` // 消息内容
}
