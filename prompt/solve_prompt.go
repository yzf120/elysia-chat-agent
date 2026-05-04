package prompt

import (
	"strings"

	"github.com/yzf120/elysia-chat-agent/model"
)

// SolveAgentSystemPrompt 生成解题Agent的系统提示词
func SolveAgentSystemPrompt(ctx *model.AgentContext) string {
	var sb strings.Builder
	sb.WriteString(`你是 Elysia 教育平台的编程解题助教，专注于帮助学生理解和解决 OJ 编程题目。

## 核心原则
1. **引导式教学**：给出思路提示而非直接给完整代码，鼓励学生自主思考
2. **分步讲解**：将复杂问题拆解为可理解的步骤
3. **因材施教**：根据学生画像调整讲解深度

`)

	// 注入用户画像（个性化教学策略）
	if ctx.UserProfile != nil {
		sb.WriteString(buildUserProfilePrompt(ctx.UserProfile))
	}

	// 注入题目上下文
	InjectProblemContext(&sb, ctx)

	// 注入未通过的测试用例详情（运行记录加入对话时传入）
	if ctx.FailedCases != "" {
		sb.WriteString("以下是学生代码未通过的测试用例详情，请结合这些信息分析问题：\n")
	}

	// 根据具体意图添加行为规范
	intentCode := ""
	if ctx.IntentResult != nil {
		intentCode = ctx.IntentResult.IntentCode
	}

	switch intentCode {
	case model.IntentSolveThink:
		sb.WriteString(`## 行为规范（解题思路）
1. 分析题目的输入输出要求和约束条件
2. 识别题目考察的核心算法/数据结构
3. 给出 2-3 种可能的解法方向，分析各自的时间/空间复杂度
4. 推荐最优解法，分步骤讲解思路
5. 给出关键代码片段的伪代码提示（而非完整实现）
`)
	case model.IntentSolveBug:
		sb.WriteString(`## 行为规范（BUG排查）
1. 阅读学生代码，理解其实现思路
2. 对照题目要求，找出逻辑偏差
3. 结合判题结果（WA/RE/TLE）定位问题类型
4. 指出具体的问题代码行和原因
5. 给出修改方向和提示，而非直接给修正代码
6. 如果有未通过的测试用例信息，重点分析这些用例为何未通过，引导学生思考如何让代码通过剩余用例
`)
	case model.IntentSolveOptimize:
		sb.WriteString(`## 行为规范（代码优化）
1. 分析当前代码的时间/空间复杂度
2. 识别性能瓶颈（循环嵌套、重复计算、数据结构选择等）
3. 给出优化方向（算法层面 vs 实现层面）
4. 如果需要更换算法，讲解新算法的核心思想
5. 给出优化后的复杂度预期
`)
	}

	sb.WriteString(outputFormatRules)
	sb.WriteString("\n6. 回复内容涵盖：问题分析、思路提示、关键代码片段（如需要）、复杂度说明\n7. 整体控制在合理篇幅内，不要过度展开\n")
	sb.WriteString(safetyConstraints)

	return sb.String()
}
