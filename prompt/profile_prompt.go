package prompt

import (
	"fmt"
	"strings"

	"github.com/yzf120/elysia-chat-agent/model"
)

// buildUserProfilePrompt 根据用户画像生成个性化教学策略提示词
func buildUserProfilePrompt(profile *model.UserProfile) string {
	if profile == nil {
		return ""
	}

	var sb strings.Builder

	// 注入学生画像数据
	sb.WriteString(fmt.Sprintf(`## 学生画像
- 编程能力等级: %s
- 总提交次数: %d
- 通过率: %.0f%%
- 已解决题目: %d / 尝试过: %d
`,
		profile.DifficultyLevel,
		profile.TotalSubmissions,
		profile.AcceptRate*100,
		profile.SolvedProblemCount,
		profile.AttemptedProblemCount,
	))

	if profile.PreferredLanguage != "" {
		sb.WriteString(fmt.Sprintf("- 最常用语言: %s\n", profile.PreferredLanguage))
	}
	if len(profile.LanguageStats) > 0 {
		var langParts []string
		for lang, count := range profile.LanguageStats {
			langParts = append(langParts, fmt.Sprintf("%s(%d次)", lang, count))
		}
		sb.WriteString(fmt.Sprintf("- 语言使用统计: %s\n", strings.Join(langParts, "、")))
	}
	if len(profile.CommonErrors) > 0 {
		sb.WriteString(fmt.Sprintf("- 常见错误: %s\n", strings.Join(profile.CommonErrors, "、")))
	}

	// 注入最近问答行为画像（最近10条记录）
	if len(profile.RecentQABehaviors) > 0 {
		sb.WriteString(fmt.Sprintf("\n## 最近问答行为（最近 %d 次提问记录）\n", len(profile.RecentQABehaviors)))
		tagFreq := make(map[string]int)
		resolvedCount := 0
		unresolvedCount := 0
		for i, qa := range profile.RecentQABehaviors {
			sb.WriteString(fmt.Sprintf("%d. [%s] %s", i+1, qa.ConversationTime, qa.QuestionSummary))
			if len(qa.KnowledgeTags) > 0 {
				sb.WriteString(fmt.Sprintf("（涉及: %s）", strings.Join(qa.KnowledgeTags, "、")))
			}
			if qa.IsResolved == 1 {
				sb.WriteString(" ✅已解决")
				resolvedCount++
			} else if qa.IsResolved == 2 {
				sb.WriteString(" ❌未解决")
				unresolvedCount++
			}
			sb.WriteString("\n")
			for _, tag := range qa.KnowledgeTags {
				tagFreq[tag]++
			}
		}
		var frequentTags []string
		for tag, count := range tagFreq {
			if count >= 2 {
				frequentTags = append(frequentTags, fmt.Sprintf("%s(%d次)", tag, count))
			}
		}
		if len(frequentTags) > 0 {
			sb.WriteString(fmt.Sprintf("\n**高频提问知识点**: %s\n", strings.Join(frequentTags, "、")))
		}
		if resolvedCount+unresolvedCount > 0 {
			sb.WriteString(fmt.Sprintf("**问题解决率**: %.0f%%（%d/%d）\n",
				float64(resolvedCount)/float64(resolvedCount+unresolvedCount)*100,
				resolvedCount, resolvedCount+unresolvedCount))
		}
	}

	sb.WriteString("\n")

	// 根据能力等级生成个性化教学策略
	sb.WriteString("## 个性化教学策略\n")

	switch profile.DifficultyLevel {
	case "beginner":
		sb.WriteString(`该学生是编程初学者，请严格遵循以下规则：
1. **绝对不要直接给出完整代码答案**
2. 详细解释相关的基础知识点（变量、循环、条件判断、数组等）
3. 用生活中的类比帮助理解抽象概念
4. 分步骤引导，每次只讲一个知识点，避免信息过载
5. 在回答末尾推荐学习方向，例如："建议先学习 XXX 基础概念"
6. 多用鼓励性语言，增强学生信心
`)
	case "intermediate":
		sb.WriteString(`该学生有一定基础，请遵循以下规则：
1. 可以给出思路框架和关键代码片段，但不给完整实现
2. 重点讲解算法设计思想和时间复杂度分析
3. 引导学生自己发现代码中的问题
4. 适当推荐进阶学习方向
5. 对于常见错误，给出预防建议
`)
	case "advanced":
		sb.WriteString(`该学生水平较高，可以直接讨论算法细节：
1. 可以直接讨论算法细节和优化方向
2. 重点关注多种解法的权衡和最优解
3. 可以提供完整的代码示例
4. 推荐竞赛级别的思考方式和高级技巧
5. 讨论边界情况和特殊用例
`)
	}

	// 针对常见错误的额外提示
	if len(profile.CommonErrors) > 0 {
		sb.WriteString(fmt.Sprintf("\n注意：该学生经常犯 %s 类型的错误，回答时请特别关注这方面的引导。\n",
			strings.Join(profile.CommonErrors, "、")))
	}

	// 基于问答行为画像的额外教学策略
	if len(profile.RecentQABehaviors) > 0 {
		weakTags := make(map[string]bool)
		for _, qa := range profile.RecentQABehaviors {
			if qa.IsResolved == 2 {
				for _, tag := range qa.KnowledgeTags {
					weakTags[tag] = true
				}
			}
		}
		if len(weakTags) > 0 {
			var weakList []string
			for tag := range weakTags {
				weakList = append(weakList, tag)
			}
			sb.WriteString(fmt.Sprintf("\n该学生近期在 %s 方面的问题尚未完全解决，如果本次问题涉及这些知识点，请更加耐心细致地讲解，并适当回顾基础概念。\n",
				strings.Join(weakList, "、")))
		}
	}

	sb.WriteString("\n")
	return sb.String()
}

// BuildUserProfilePromptPublic 公开版本的画像提示词构建（供 react_engine 调用）
func BuildUserProfilePromptPublic(profile *model.UserProfile) string {
	return buildUserProfilePrompt(profile)
}
