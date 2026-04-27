package agent

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/yzf120/elysia-chat-agent/model"
)

// ==================== 1.1 containsAny ====================

func TestContainsAny(t *testing.T) {
	tests := []struct {
		name     string
		s        string
		keywords []string
		want     bool
	}{
		{"匹配单个关键词", "这道题怎么做", []string{"怎么做"}, true},
		{"匹配多个关键词之一", "代码有bug", []string{"错了", "bug", "wa"}, true},
		{"无匹配", "今天天气不错", []string{"bug", "错了"}, false},
		{"空字符串", "", []string{"bug"}, false},
		{"空关键词列表", "hello", nil, false},
		{"空关键词列表_显式空切片", "hello", []string{}, false},
		{"关键词在字符串中间", "我的代码有bug了", []string{"bug"}, true},
		{"中文关键词匹配", "动态规划是什么", []string{"动态规划"}, true},
		{"多个关键词全部匹配_返回true", "代码有bug错了", []string{"bug", "错了"}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := containsAny(tt.s, tt.keywords...)
			assert.Equal(t, tt.want, got)
		})
	}
}

// ==================== 1.2 intentCodeToRoute ====================

func TestIntentCodeToRoute(t *testing.T) {
	tests := []struct {
		code string
		want string
	}{
		// 解题相关 → solve_agent
		{model.IntentSolveThink, model.AgentRouteSolve},
		{model.IntentSolveBug, model.AgentRouteSolve},
		{model.IntentSolveOptimize, model.AgentRouteSolve},
		// 知识答疑 → knowledge_agent
		{model.IntentKnowledgeAlgo, model.AgentRouteKnowledge},
		{model.IntentKnowledgeErr, model.AgentRouteKnowledge},
		// 测试用例 → testcase_agent
		{model.IntentTestcaseGen, model.AgentRouteTestcase},
		{model.IntentTestcaseImport, model.AgentRouteTestcase},
		// IDE 调试 → debug_agent
		{model.IntentCodeDebug, model.AgentRouteDebug},
		// 操作控制 → operate_agent
		{model.IntentOperatePlat, model.AgentRouteOperate},
		{model.IntentOperateDialog, model.AgentRouteOperate},
		// 题目审核/知识管理 → knowledge_agent
		{model.IntentProblemReview, model.AgentRouteKnowledge},
		{model.IntentKnowledgeMgmt, model.AgentRouteKnowledge},
		// 闲聊兜底 → fallback_agent
		{model.IntentOtherChat, model.AgentRouteFallback},
		// 未知编码 → fallback_agent
		{"UNKNOWN_CODE", model.AgentRouteFallback},
		{"", model.AgentRouteFallback},
	}
	for _, tt := range tests {
		t.Run(tt.code, func(t *testing.T) {
			got := intentCodeToRoute(tt.code)
			assert.Equal(t, tt.want, got)
		})
	}
}

// ==================== 1.3 fallbackClassify ====================

func TestFallbackClassify(t *testing.T) {
	r := &IntentRouter{}

	tests := []struct {
		name       string
		query      string
		role       string
		wantIntent string
		wantRoute  string
		minConf    float64
	}{
		// 教师场景
		{
			name:       "教师_测试用例生成",
			query:      "帮我生成测试用例",
			role:       model.RoleTeacher,
			wantIntent: model.IntentTestcaseGen,
			wantRoute:  model.AgentRouteTestcase,
			minConf:    0.7,
		},
		{
			name:       "教师_非测试用例_走通用逻辑",
			query:      "这道题怎么做",
			role:       model.RoleTeacher,
			wantIntent: model.IntentSolveThink,
			wantRoute:  model.AgentRouteSolve,
			minConf:    0.7,
		},
		// 学生场景 - 解题思路
		{
			name:       "学生_解题思路_怎么做",
			query:      "这道题怎么做",
			role:       model.RoleStudent,
			wantIntent: model.IntentSolveThink,
			wantRoute:  model.AgentRouteSolve,
			minConf:    0.7,
		},
		{
			name:       "学生_解题思路_解法",
			query:      "有什么好的解法",
			role:       model.RoleStudent,
			wantIntent: model.IntentSolveThink,
			wantRoute:  model.AgentRouteSolve,
			minConf:    0.7,
		},
		// 学生场景 - BUG 排查
		{
			name:       "学生_BUG排查_bug",
			query:      "代码有bug",
			role:       model.RoleStudent,
			wantIntent: model.IntentSolveBug,
			wantRoute:  model.AgentRouteSolve,
			minConf:    0.7,
		},
		{
			name:       "学生_BUG排查_wa",
			query:      "提交了但是wa了",
			role:       model.RoleStudent,
			wantIntent: model.IntentSolveBug,
			wantRoute:  model.AgentRouteSolve,
			minConf:    0.7,
		},
		{
			name:       "学生_BUG排查_答案错误",
			query:      "答案错误怎么办",
			role:       model.RoleStudent,
			wantIntent: model.IntentSolveBug,
			wantRoute:  model.AgentRouteSolve,
			minConf:    0.7,
		},
		// 学生场景 - 代码优化
		{
			name:       "学生_代码优化_超时",
			query:      "运行超时了怎么办",
			role:       model.RoleStudent,
			wantIntent: model.IntentSolveOptimize,
			wantRoute:  model.AgentRouteSolve,
			minConf:    0.7,
		},
		{
			name:       "学生_代码优化_tle",
			query:      "tle了",
			role:       model.RoleStudent,
			wantIntent: model.IntentSolveOptimize,
			wantRoute:  model.AgentRouteSolve,
			minConf:    0.7,
		},
		// 学生场景 - 知识概念
		{
			name:       "学生_知识概念_是什么",
			query:      "动态规划是什么",
			role:       model.RoleStudent,
			wantIntent: model.IntentKnowledgeAlgo,
			wantRoute:  model.AgentRouteKnowledge,
			minConf:    0.6,
		},
		{
			name:       "学生_知识概念_复杂度",
			query:      "时间复杂度怎么分析",
			role:       model.RoleStudent,
			wantIntent: model.IntentKnowledgeAlgo,
			wantRoute:  model.AgentRouteKnowledge,
			minConf:    0.6,
		},
		// 学生场景 - IDE 调试
		{
			name:       "学生_IDE调试_编译错误",
			query:      "编译错误了",
			role:       model.RoleStudent,
			wantIntent: model.IntentCodeDebug,
			wantRoute:  model.AgentRouteDebug,
			minConf:    0.7,
		},
		{
			name:       "学生_IDE调试_compile",
			query:      "compile error怎么解决",
			role:       model.RoleStudent,
			wantIntent: model.IntentCodeDebug,
			wantRoute:  model.AgentRouteDebug,
			minConf:    0.7,
		},
		// 兜底
		{
			name:       "学生_闲聊兜底",
			query:      "今天天气真好",
			role:       model.RoleStudent,
			wantIntent: model.IntentOtherChat,
			wantRoute:  model.AgentRouteFallback,
			minConf:    0.5,
		},
		{
			name:       "学生_空查询兜底",
			query:      "",
			role:       model.RoleStudent,
			wantIntent: model.IntentOtherChat,
			wantRoute:  model.AgentRouteFallback,
			minConf:    0.5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := r.fallbackClassify(tt.query, tt.role)
			assert.Equal(t, tt.wantIntent, result.IntentCode, "意图编码不匹配")
			assert.Equal(t, tt.wantRoute, result.AgentRoute, "路由不匹配")
			assert.GreaterOrEqual(t, result.Confidence, tt.minConf, "置信度应 >= %.2f", tt.minConf)
			assert.NotEmpty(t, result.IntentLevel1, "一级分类不应为空")
			assert.NotEmpty(t, result.Reasoning, "推理原因不应为空")
		})
	}
}

// ==================== 1.4 parseIntentResponse ====================

func TestParseIntentResponse(t *testing.T) {
	tests := []struct {
		name     string
		text     string
		wantErr  bool
		wantCode string
		wantConf float64
	}{
		{
			name:     "正常JSON",
			text:     `{"intent_code":"SOLVE_BUG","intent_level1":"解题相关","confidence":0.9}`,
			wantErr:  false,
			wantCode: "SOLVE_BUG",
			wantConf: 0.9,
		},
		{
			name:     "Markdown_json代码块包裹",
			text:     "```json\n{\"intent_code\":\"SOLVE_THINK\",\"confidence\":0.8}\n```",
			wantErr:  false,
			wantCode: "SOLVE_THINK",
			wantConf: 0.8,
		},
		{
			name:     "前后有多余文本",
			text:     "分析结果如下：{\"intent_code\":\"KNOWLEDGE_ALGO\",\"confidence\":0.7} 以上是分析",
			wantErr:  false,
			wantCode: "KNOWLEDGE_ALGO",
			wantConf: 0.7,
		},
		{
			name:     "带extracted_entities",
			text:     `{"intent_code":"CODE_DEBUG","confidence":0.85,"extracted_entities":{"language":"python","error_type":"SyntaxError"}}`,
			wantErr:  false,
			wantCode: "CODE_DEBUG",
			wantConf: 0.85,
		},
		{
			name:    "无效JSON",
			text:    "这不是JSON格式的内容",
			wantErr: true,
		},
		{
			name:    "空intent_code",
			text:    `{"intent_code":"","confidence":0.9}`,
			wantErr: true,
		},
		{
			name:    "完全空字符串",
			text:    "",
			wantErr: true,
		},
		{
			name:    "只有花括号",
			text:    "{}",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parseIntentResponse(tt.text)
			if tt.wantErr {
				assert.Error(t, err, "应返回错误")
			} else {
				assert.NoError(t, err, "不应返回错误")
				assert.NotNil(t, result)
				assert.Equal(t, tt.wantCode, result.IntentCode, "意图编码不匹配")
				if tt.wantConf > 0 {
					assert.InDelta(t, tt.wantConf, result.Confidence, 0.001, "置信度不匹配")
				}
			}
		})
	}
}
