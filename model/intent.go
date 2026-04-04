package model

import "time"

// ==================== 意图编码常量 ====================

const (
	// 学生场景意图
	IntentSolveThink    = "SOLVE_THINK"      // 解题思路
	IntentSolveBug      = "SOLVE_BUG"        // 代码BUG排查
	IntentSolveOptimize = "SOLVE_OPTIMIZE"   // 代码优化
	IntentKnowledgeAlgo = "KNOWLEDGE_ALGO"   // 算法概念
	IntentKnowledgeErr  = "KNOWLEDGE_ERROR"  // 错误解释
	IntentCodeDebug     = "CODE_DEBUG"       // IDE调试
	IntentOperatePlat   = "OPERATE_PLATFORM" // 平台操作
	IntentOperateDialog = "OPERATE_DIALOG"   // 对话控制
	IntentOtherChat     = "OTHER_CHAT"       // 闲聊兜底

	// 教师场景意图
	IntentTestcaseGen    = "TESTCASE_GEN"     // 测试用例生成
	IntentTestcaseImport = "TESTCASE_IMPORT"  // 测试用例导入
	IntentProblemReview  = "PROBLEM_REVIEW"   // 题目审核
	IntentKnowledgeMgmt  = "KNOWLEDGE_MANAGE" // 知识库管理
)

// ==================== Agent 路由常量 ====================

const (
	AgentRouteSolve     = "solve_agent"     // 解题Agent
	AgentRouteKnowledge = "knowledge_agent" // 知识答疑Agent
	AgentRouteTestcase  = "testcase_agent"  // 测试用例Agent
	AgentRouteDebug     = "debug_agent"     // IDE调试Agent
	AgentRouteOperate   = "operate_agent"   // 操作Agent
	AgentRouteFallback  = "fallback_agent"  // 兜底Agent
)

// ==================== 用户角色 ====================

const (
	RoleStudent = "student"
	RoleTeacher = "teacher"
)

// ==================== 数据模型 ====================

// IntentDict 意图字典（对应 oj_intent_dict 表）
type IntentDict struct {
	Id              int       `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	IntentLevel1    string    `gorm:"column:intent_level1" json:"intent_level1"`
	IntentLevel2    string    `gorm:"column:intent_level2" json:"intent_level2"`
	IntentCode      string    `gorm:"column:intent_code;uniqueIndex" json:"intent_code"`
	Description     string    `gorm:"column:description" json:"description"`
	MatchKeywords   string    `gorm:"column:match_keywords" json:"match_keywords"`
	ExampleQueries  string    `gorm:"column:example_queries" json:"example_queries"`
	RewriteTemplate string    `gorm:"column:rewrite_template" json:"rewrite_template"`
	AgentRoute      string    `gorm:"column:agent_route" json:"agent_route"`
	Priority        int       `gorm:"column:priority" json:"priority"`
	IsValid         int       `gorm:"column:is_valid" json:"is_valid"`
	CreateTime      time.Time `gorm:"column:create_time;autoCreateTime" json:"create_time"`
	UpdateTime      time.Time `gorm:"column:update_time;autoUpdateTime" json:"update_time"`
}

func (IntentDict) TableName() string {
	return "oj_intent_dict"
}

// IntentPromptTemplate 意图提示词模板（对应 oj_intent_prompt_template 表）
type IntentPromptTemplate struct {
	Id              int       `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	IntentCode      string    `gorm:"column:intent_code" json:"intent_code"`
	TemplateType    string    `gorm:"column:template_type" json:"template_type"`
	TemplateName    string    `gorm:"column:template_name" json:"template_name"`
	TemplateContent string    `gorm:"column:template_content" json:"template_content"`
	IsActive        int       `gorm:"column:is_active" json:"is_active"`
	Version         int       `gorm:"column:version" json:"version"`
	CreateTime      time.Time `gorm:"column:create_time;autoCreateTime" json:"create_time"`
	UpdateTime      time.Time `gorm:"column:update_time;autoUpdateTime" json:"update_time"`
}

func (IntentPromptTemplate) TableName() string {
	return "oj_intent_prompt_template"
}

// UserIntentRecord 用户意图记录（对应 oj_user_intent_record 表）
type UserIntentRecord struct {
	Id               int64     `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	UserID           string    `gorm:"column:user_id" json:"user_id"`
	SessionID        string    `gorm:"column:session_id" json:"session_id"`
	QuestionID       string    `gorm:"column:question_id" json:"question_id"`
	OriginalRequest  string    `gorm:"column:original_request" json:"original_request"`
	IntentCode       string    `gorm:"column:intent_code" json:"intent_code"`
	IntentLevel1     string    `gorm:"column:intent_level1" json:"intent_level1"`
	RewrittenRequest string    `gorm:"column:rewritten_request" json:"rewritten_request"`
	IntentConfidence float64   `gorm:"column:intent_confidence" json:"intent_confidence"`
	ResponseTimeMs   int       `gorm:"column:response_time_ms" json:"response_time_ms"`
	RecognizeStatus  int       `gorm:"column:recognize_status" json:"recognize_status"`
	CreateTime       time.Time `gorm:"column:create_time;autoCreateTime" json:"create_time"`
}

func (UserIntentRecord) TableName() string {
	return "oj_user_intent_record"
}
