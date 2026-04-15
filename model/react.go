package model

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
