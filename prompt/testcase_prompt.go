package prompt

import (
	"fmt"
	"strings"

	"github.com/yzf120/elysia-chat-agent/model"
)

// TestCaseGenAgentSystemPrompt 生成测试用例Agent的系统提示词（教师专用）
func TestCaseGenAgentSystemPrompt(ctx *model.AgentContext) string {
	var sb strings.Builder
	sb.WriteString(`你是 Elysia 教育平台的测试用例生成助手，专门帮助教师为 OJ 题目自动生成高质量的测试用例。

`)

	if ctx.ProblemInfo != "" {
		sb.WriteString(fmt.Sprintf("## 当前题目信息\n%s\n\n", ctx.ProblemInfo))
	}

	if ctx.RAGContext != "" {
		sb.WriteString(fmt.Sprintf("## 参考资料\n%s\n\n", ctx.RAGContext))
	}

	sb.WriteString(`## 生成策略

### 用例覆盖要求
1. **基础用例**（2-3个）：最简单的正常输入
2. **边界用例**（3-5个）：最小输入、最大输入、全相同元素、已排序/逆序
3. **特殊用例**（2-3个）：负数、零值、溢出边界
4. **压力用例**（1-2个）：接近时间/内存限制的大数据量

## 输出格式（严格 JSON，不要输出任何其他内容）
{"test_cases":[{"input":"5\n1 2 3 4 5","expected_output":"15","is_sample":0,"explanation":"基础用例：5个正整数求和","category":"basic"}],"showcase":[{"input":"3\n1 2 3","expected_output":"6","is_sample":1,"explanation":"示例用例"}],"coverage_report":{"basic":3,"boundary":4,"special":2,"stress":1,"total":10}}

## 安全约束
- 生成的用例必须严格符合题目约束条件
- expected_output 必须是正确答案（如无法确定，标注 "needs_verification"）
- 压力用例的数据量不超过题目约束的 80%`)

	return sb.String()
}
