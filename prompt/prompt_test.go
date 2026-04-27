package prompt

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/yzf120/elysia-chat-agent/model"
)

// ==================== 2.1 GetSystemPromptByIntent ====================

func TestGetSystemPromptByIntent(t *testing.T) {
	t.Run("nil_IntentResult_返回兜底prompt", func(t *testing.T) {
		ctx := &model.AgentContext{}
		p := GetSystemPromptByIntent(ctx)
		assert.NotEmpty(t, p, "nil IntentResult 应返回兜底 prompt")
	})

	// 测试所有意图编码都能返回非空 prompt
	intentCodes := []struct {
		code string
		desc string
	}{
		{model.IntentSolveThink, "解题思路"},
		{model.IntentSolveBug, "BUG排查"},
		{model.IntentSolveOptimize, "代码优化"},
		{model.IntentKnowledgeAlgo, "算法概念"},
		{model.IntentKnowledgeErr, "错误解释"},
		{model.IntentTestcaseGen, "测试用例生成"},
		{model.IntentTestcaseImport, "测试用例导入"},
		{model.IntentCodeDebug, "IDE调试"},
		{model.IntentOperatePlat, "平台操作"},
		{model.IntentOperateDialog, "对话控制"},
		{model.IntentOtherChat, "闲聊兜底"},
		{model.IntentProblemReview, "题目审核"},
		{model.IntentKnowledgeMgmt, "知识管理"},
	}

	for _, tc := range intentCodes {
		t.Run(tc.desc+"_"+tc.code, func(t *testing.T) {
			ctx := &model.AgentContext{
				IntentResult: &model.IntentResult{IntentCode: tc.code},
			}
			p := GetSystemPromptByIntent(ctx)
			assert.NotEmpty(t, p, "意图 %s 应返回非空 prompt", tc.code)
		})
	}

	t.Run("未知意图编码_返回兜底prompt", func(t *testing.T) {
		ctx := &model.AgentContext{
			IntentResult: &model.IntentResult{IntentCode: "UNKNOWN_CODE"},
		}
		p := GetSystemPromptByIntent(ctx)
		assert.NotEmpty(t, p, "未知意图编码应返回兜底 prompt")
	})
}

// ==================== 2.2 BuildUserProfilePromptPublic ====================

func TestBuildUserProfilePromptPublic(t *testing.T) {
	t.Run("nil_profile_返回空", func(t *testing.T) {
		result := BuildUserProfilePromptPublic(nil)
		assert.Empty(t, result)
	})

	t.Run("beginner_初学者策略", func(t *testing.T) {
		profile := &model.UserProfile{
			DifficultyLevel:    "beginner",
			TotalSubmissions:   10,
			AcceptRate:         0.3,
			SolvedProblemCount: 3,
		}
		p := BuildUserProfilePromptPublic(profile)
		assert.Contains(t, p, "beginner")
		assert.Contains(t, p, "初学者")
		assert.Contains(t, p, "30%") // AcceptRate * 100
	})

	t.Run("intermediate_中等策略", func(t *testing.T) {
		profile := &model.UserProfile{
			DifficultyLevel:    "intermediate",
			TotalSubmissions:   100,
			AcceptRate:         0.6,
			SolvedProblemCount: 50,
		}
		p := BuildUserProfilePromptPublic(profile)
		assert.Contains(t, p, "intermediate")
		assert.Contains(t, p, "一定基础")
	})

	t.Run("advanced_高级策略", func(t *testing.T) {
		profile := &model.UserProfile{
			DifficultyLevel:    "advanced",
			TotalSubmissions:   500,
			AcceptRate:         0.85,
			SolvedProblemCount: 300,
		}
		p := BuildUserProfilePromptPublic(profile)
		assert.Contains(t, p, "advanced")
		assert.Contains(t, p, "水平较高")
	})

	t.Run("带常见错误", func(t *testing.T) {
		profile := &model.UserProfile{
			DifficultyLevel: "beginner",
			CommonErrors:    []string{"数组越界", "空指针"},
		}
		p := BuildUserProfilePromptPublic(profile)
		assert.Contains(t, p, "数组越界")
		assert.Contains(t, p, "空指针")
	})

	t.Run("带编程语言信息", func(t *testing.T) {
		profile := &model.UserProfile{
			DifficultyLevel:   "intermediate",
			PreferredLanguage: "Python",
			LanguageStats:     map[string]int{"Python": 50, "C++": 30},
		}
		p := BuildUserProfilePromptPublic(profile)
		assert.Contains(t, p, "Python")
	})

	t.Run("带问答行为记录", func(t *testing.T) {
		profile := &model.UserProfile{
			DifficultyLevel: "intermediate",
			RecentQABehaviors: []model.QABehaviorSummary{
				{
					QuestionSummary:  "两数之和解题思路",
					KnowledgeTags:    []string{"哈希表"},
					IsResolved:       1,
					ConversationTime: "2026-04-20 10:00",
				},
				{
					QuestionSummary:  "背包问题不理解",
					KnowledgeTags:    []string{"动态规划", "背包问题"},
					IsResolved:       2,
					ConversationTime: "2026-04-19 15:00",
				},
			},
		}
		p := BuildUserProfilePromptPublic(profile)
		assert.Contains(t, p, "两数之和解题思路")
		assert.Contains(t, p, "背包问题不理解")
		assert.Contains(t, p, "✅已解决")
		assert.Contains(t, p, "❌未解决")
		assert.Contains(t, p, "最近问答行为")
	})

	t.Run("带未解决问题_生成薄弱知识点提示", func(t *testing.T) {
		profile := &model.UserProfile{
			DifficultyLevel: "beginner",
			RecentQABehaviors: []model.QABehaviorSummary{
				{
					QuestionSummary: "动态规划不理解",
					KnowledgeTags:   []string{"动态规划"},
					IsResolved:      2,
				},
			},
		}
		p := BuildUserProfilePromptPublic(profile)
		assert.Contains(t, p, "动态规划")
		assert.Contains(t, p, "尚未完全解决")
	})
}

// ==================== 2.3 IntentRouterSystemPrompt ====================

func TestIntentRouterSystemPrompt(t *testing.T) {
	t.Run("学生角色", func(t *testing.T) {
		p := IntentRouterSystemPrompt(model.RoleStudent)
		assert.Contains(t, p, "student")
		assert.Contains(t, p, "意图路由引擎")
		assert.Contains(t, p, "SOLVE_THINK")
		assert.Contains(t, p, "SOLVE_BUG")
		assert.Contains(t, p, "KNOWLEDGE_ALGO")
		assert.Contains(t, p, "OTHER_CHAT")
	})

	t.Run("教师角色", func(t *testing.T) {
		p := IntentRouterSystemPrompt(model.RoleTeacher)
		assert.Contains(t, p, "teacher")
		assert.Contains(t, p, "TESTCASE_GEN")
		assert.Contains(t, p, "TESTCASE_IMPORT")
		assert.Contains(t, p, "PROBLEM_REVIEW")
	})

	t.Run("包含JSON输出格式", func(t *testing.T) {
		p := IntentRouterSystemPrompt(model.RoleStudent)
		assert.Contains(t, p, "intent_code")
		assert.Contains(t, p, "confidence")
		assert.Contains(t, p, "JSON")
	})
}
