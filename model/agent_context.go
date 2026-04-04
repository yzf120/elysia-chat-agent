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
	Language     string `json:"language,omitempty"`      // 编程语言
	ErrorMessage string `json:"error_message,omitempty"` // 错误信息

	// 意图识别结果
	IntentResult *IntentResult `json:"intent_result,omitempty"`

	// RAG 检索结果
	RAGContext string `json:"rag_context,omitempty"`

	// 用户画像（AI 回答前参考，用于个性化教学策略）
	UserProfile *UserProfile `json:"user_profile,omitempty"`

	// 模型配置
	ModelID string `json:"model_id"`
}

// UserProfile 学生做题画像（从 student_profile 表加载）
type UserProfile struct {
	DifficultyLevel       string         `json:"difficulty_level"`        // AI推断的能力等级：beginner/intermediate/advanced
	TotalSubmissions      int            `json:"total_submissions"`       // 总提交次数
	AcceptRate            float64        `json:"accept_rate"`             // 通过率
	SolvedProblemCount    int            `json:"solved_problem_count"`    // 已解决题目数
	AttemptedProblemCount int            `json:"attempted_problem_count"` // 尝试过的题目数
	PreferredLanguage     string         `json:"preferred_language"`      // 最常用编程语言
	LanguageStats         map[string]int `json:"language_stats"`          // 各编程语言使用次数
	CommonErrors          []string       `json:"common_errors"`           // 常见错误类型
}

// ChatMessage 对话消息
type ChatMessage struct {
	Role    string `json:"role"`    // system, user, assistant
	Content string `json:"content"` // 消息内容
}

// IntentResult 意图识别结果
type IntentResult struct {
	IntentCode   string            `json:"intent_code"`
	IntentLevel1 string            `json:"intent_level1"`
	IntentLevel2 string            `json:"intent_level2"`
	Confidence   float64           `json:"confidence"`
	Reasoning    string            `json:"reasoning"`
	Entities     map[string]string `json:"extracted_entities,omitempty"`
	AgentRoute   string            `json:"agent_route"`
}

// ==================== ReAct 步骤记录 ====================

// ReActStep ReAct 循环中的单步记录
type ReActStep struct {
	StepType   string `json:"step_type"`             // thought / action / observation
	Content    string `json:"content"`               // 步骤内容
	ToolName   string `json:"tool_name,omitempty"`   // 使用的工具名称
	ToolInput  string `json:"tool_input,omitempty"`  // 工具输入
	ToolOutput string `json:"tool_output,omitempty"` // 工具输出
	DurationMs int64  `json:"duration_ms"`           // 耗时
}

// ReActTrace ReAct 完整执行追踪
type ReActTrace struct {
	Steps       []ReActStep `json:"steps"`
	TotalSteps  int         `json:"total_steps"`
	TotalTimeMs int64       `json:"total_time_ms"`
}

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

// ==================== 测试用例相关 ====================

// TestCase 测试用例
type TestCase struct {
	Input          string `json:"input"`
	ExpectedOutput string `json:"expected_output"`
	IsSample       int    `json:"is_sample"` // 0-隐藏用例 1-示例用例
	Explanation    string `json:"explanation"`
	Category       string `json:"category"` // basic / boundary / special / stress
}

// TestCaseGenResult 测试用例生成结果
type TestCaseGenResult struct {
	TestCases      []TestCase     `json:"test_cases"`
	Showcase       []TestCase     `json:"showcase"`
	CoverageReport map[string]int `json:"coverage_report"`
}
