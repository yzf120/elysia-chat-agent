package prompt

import (
	"fmt"
	"strings"

	"github.com/yzf120/elysia-chat-agent/model"
)

// DebugAgentSystemPrompt 生成IDE调试Agent的系统提示词
func DebugAgentSystemPrompt(ctx *model.AgentContext) string {
	var sb strings.Builder
	sb.WriteString(`你是 Elysia 教育平台的 IDE 编程调试助手，专注于帮助学生解决编程过程中遇到的编译错误、运行错误和逻辑问题。

`)

	// 注入用户画像（个性化教学策略）
	if ctx.UserProfile != nil {
		sb.WriteString(buildUserProfilePrompt(ctx.UserProfile))
	}

	// 注入题目上下文
	if ctx.ProblemInfo != "" {
		sb.WriteString(fmt.Sprintf("## 当前题目信息\n%s\n\n", ctx.ProblemInfo))
	}
	if ctx.Language != "" {
		sb.WriteString(fmt.Sprintf("## 编程语言: %s\n\n", ctx.Language))
	}
	if ctx.StudentCode != "" {
		sb.WriteString(fmt.Sprintf("## 学生代码\n```%s\n%s\n```\n\n", ctx.Language, ctx.StudentCode))
	}
	if ctx.ErrorMessage != "" {
		sb.WriteString(fmt.Sprintf("## 错误信息\n```\n%s\n```\n\n", ctx.ErrorMessage))
	}
	if ctx.JudgeResult != "" {
		sb.WriteString(fmt.Sprintf("## 判题结果: %s\n\n", ctx.JudgeResult))
	}
	if ctx.FailedCases != "" {
		sb.WriteString("## 未通过的测试用例\n```json\n" + ctx.FailedCases + "\n```\n\n")
	}
	if ctx.RAGContext != "" {
		sb.WriteString(fmt.Sprintf("## 参考资料\n%s\n\n", ctx.RAGContext))
	}

	sb.WriteString(`## 行为规范
### 编译错误处理
- 解析编译器错误信息，翻译为学生能理解的语言
- 指出错误所在的代码行
- 解释语法规则，给出修复示例

### 运行错误处理
- 分析段错误、栈溢出、数组越界等运行时错误
- 结合代码逻辑分析触发条件
- 给出调试方法（如添加打印语句、边界检查）

### 逻辑错误处理
- 对比预期输出和实际输出
- 用具体的测试用例走读代码逻辑
- 指出逻辑偏差点
`)

	sb.WriteString(outputFormatRules)
	sb.WriteString("\n6. 回复内容涵盖：错误定位、原因分析、修复建议（不直接给完整修正代码）、相关知识点\n7. 整体控制在合理篇幅内，不要过度展开")

	return sb.String()
}
