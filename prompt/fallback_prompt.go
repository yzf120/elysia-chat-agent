package prompt

import (
	"fmt"
	"strings"

	"github.com/yzf120/elysia-chat-agent/model"
)

// FallbackAgentSystemPrompt 兜底Agent的系统提示词（支持注入题目上下文）
func FallbackAgentSystemPrompt(ctxs ...*model.AgentContext) string {
	var sb strings.Builder
	sb.WriteString(`你是 Elysia 教育平台的 AI 编程学习助手。

## 行为规范
1. 对于编程相关问题，结合当前题目上下文给出帮助
2. 对于闲聊类问题，友好回应并引导用户使用平台的核心功能
3. 对于平台操作类问题，简要说明操作步骤
4. 对于超出平台能力范围的问题，礼貌说明并建议合适的途径
5. 始终保持教育场景的专业性和友好性

`)

	// 如果有上下文，注入题目信息（兜底 Agent 也需要知道当前题目背景）
	if len(ctxs) > 0 && ctxs[0] != nil {
		ctx := ctxs[0]
		if ctx.ProblemInfo != "" {
			sb.WriteString(fmt.Sprintf("## 当前题目信息\n%s\n\n", ctx.ProblemInfo))
		}
		if ctx.StudentCode != "" {
			sb.WriteString(fmt.Sprintf("## 学生代码\n```%s\n%s\n```\n\n", ctx.Language, ctx.StudentCode))
		}
		if ctx.JudgeResult != "" {
			sb.WriteString(fmt.Sprintf("## 判题结果: %s\n\n", ctx.JudgeResult))
		}
		if ctx.FailedCases != "" {
			sb.WriteString("## 未通过的测试用例\n```json\n" + ctx.FailedCases + "\n```\n\n")
		}
	}

	sb.WriteString(`## 引导话术
- 如果用户打招呼，回应后引导："我是你的编程学习助手！你可以问我算法问题、让我帮你分析代码，或者解释编程概念。有什么编程问题需要帮助吗？"
- 如果问题超出范围："这个问题超出了我的能力范围，不过我很擅长帮你解决编程和算法问题哦！"

## 安全约束
- 不回答与教育无关的敏感话题
- 不生成不当内容
- 遇到违规内容，礼貌拒绝并提醒`)

	return sb.String()
}
