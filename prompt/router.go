package prompt

import "github.com/yzf120/elysia-chat-agent/model"

// GetSystemPromptByIntent 根据意图获取对应的系统提示词
func GetSystemPromptByIntent(ctx *model.AgentContext) string {
	if ctx.IntentResult == nil {
		return FallbackAgentSystemPrompt(ctx)
	}

	switch ctx.IntentResult.IntentCode {
	case model.IntentSolveThink, model.IntentSolveBug, model.IntentSolveOptimize:
		return SolveAgentSystemPrompt(ctx)
	case model.IntentKnowledgeAlgo, model.IntentKnowledgeErr:
		return KnowledgeAgentSystemPrompt(ctx)
	case model.IntentTestcaseGen, model.IntentTestcaseImport:
		return TestCaseGenAgentSystemPrompt(ctx)
	case model.IntentCodeDebug:
		return DebugAgentSystemPrompt(ctx)
	case model.IntentOperatePlat, model.IntentOperateDialog:
		return OperateAgentSystemPrompt()
	default:
		return FallbackAgentSystemPrompt(ctx)
	}
}
