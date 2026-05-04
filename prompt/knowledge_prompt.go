package prompt

import (
	"strings"

	"github.com/yzf120/elysia-chat-agent/model"
)

// KnowledgeAgentSystemPrompt 生成知识答疑Agent的系统提示词
func KnowledgeAgentSystemPrompt(ctx *model.AgentContext) string {
	var sb strings.Builder
	sb.WriteString(`你是 Elysia 教育平台的编程知识答疑助教，专注于解答算法、数据结构、编程语言等知识问题。

## 核心原则
1. **通俗易懂**：用简洁的语言和生动的类比解释概念
2. **结合实践**：将抽象概念与 OJ 做题场景关联
3. **体系化**：帮助学生建立知识之间的联系

`)

	// 注入用户画像（个性化教学策略）
	if ctx.UserProfile != nil {
		sb.WriteString(buildUserProfilePrompt(ctx.UserProfile))
	}

	// 注入题目上下文
	InjectProblemContext(&sb, ctx)

	intentCode := ""
	if ctx.IntentResult != nil {
		intentCode = ctx.IntentResult.IntentCode
	}

	switch intentCode {
	case model.IntentKnowledgeAlgo:
		sb.WriteString(`## 行为规范（算法概念解释）
1. 用一句话概括核心概念
2. 用生活中的类比帮助理解
3. 给出该算法/数据结构的典型应用场景
4. 分析时间/空间复杂度
5. 与相关概念对比（如：贪心 vs 动态规划）
6. 推荐 1-2 道适合练习的题目
`)
	case model.IntentKnowledgeErr:
		sb.WriteString(`## 行为规范（错误原因解释）
1. 解释错误类型的含义（CE/RE/TLE/MLE/WA）
2. 列举该错误的 Top 3 常见原因
3. 给出排查方法和调试技巧
4. 给出预防建议
`)
	}

	sb.WriteString(outputFormatRules)
	sb.WriteString("\n6. 回复内容涵盖：概念解释、深入理解、代码示例（如适用）、复杂度分析\n7. 整体控制在合理篇幅内，不要过度展开")

	return sb.String()
}
