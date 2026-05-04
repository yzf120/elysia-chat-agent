package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"strings"
	"time"

	"github.com/yzf120/elysia-chat-agent/model"
	"github.com/yzf120/elysia-chat-agent/prompt"
	"github.com/yzf120/elysia-chat-agent/rpc"
	llmpb "github.com/yzf120/elysia-llm-tool/proto/llm"
)

// IntentRouter 意图路由 Agent，负责分析用户意图并路由到对应的业务 Agent
type IntentRouter struct {
	// 意图分类使用的模型（可以用较小的模型以降低延迟）
	classifyModelID string
}

// NewIntentRouter 创建意图路由 Agent
func NewIntentRouter(classifyModelID string) *IntentRouter {
	if classifyModelID == "" {
		classifyModelID = "qwen3-omni-flash" // 意图分类固定使用千问模型
	}
	return &IntentRouter{
		classifyModelID: classifyModelID,
	}
}

// Classify 分析用户意图
func (r *IntentRouter) Classify(ctx context.Context, agentCtx *model.AgentContext) (*model.IntentResult, error) {
	startTime := time.Now()

	// 获取用户最后一条消息
	userQuery := agentCtx.OriginalQuery
	if userQuery == "" && len(agentCtx.Messages) > 0 {
		for i := len(agentCtx.Messages) - 1; i >= 0; i-- {
			if agentCtx.Messages[i].Role == "user" {
				userQuery = agentCtx.Messages[i].Content
				break
			}
		}
	}

	if userQuery == "" {
		return &model.IntentResult{
			IntentCode:   model.IntentOtherChat,
			IntentLevel1: "无关兜底",
			IntentLevel2: "闲聊/无诉求",
			Confidence:   1.0,
			Reasoning:    "用户未提供有效输入",
			AgentRoute:   model.AgentRouteFallback,
		}, nil
	}

	// ===== 运行记录加入对话：前置意图判断 =====
	// 如果是从运行记录触发的对话，根据判题结果直接确定意图
	if agentCtx.JudgeResult != "" {
		switch agentCtx.JudgeResult {
		case "accepted":
			// 完全通过 → 强制走代码优化意图
			log.Printf("[IntentRouter] 运行记录触发（完全通过），强制路由到 SOLVE_OPTIMIZE")
			return &model.IntentResult{
				IntentCode:   model.IntentSolveOptimize,
				IntentLevel1: "解题相关",
				IntentLevel2: "代码优化",
				Confidence:   1.0,
				Reasoning:    "运行记录加入对话：代码已全部通过，自动触发代码优化分析",
				AgentRoute:   model.AgentRouteSolve,
			}, nil
		case "partial_pass":
			// 部分通过 → 根据用户提示词由 LLM 判断是 SOLVE_OPTIMIZE 还是 SOLVE_BUG
			log.Printf("[IntentRouter] 运行记录触发（部分通过），交由 LLM 判断意图（SOLVE_BUG / SOLVE_OPTIMIZE）")
			// 继续走下面的 LLM 意图分类流程
		}
	}

	// 构建意图分类请求
	systemPrompt := prompt.IntentRouterSystemPrompt(agentCtx.UserRole)

	// 构建带上下文的用户消息（让意图分类 LLM 能看到题目、代码等上下文）
	classifyInput := userQuery
	var contextParts []string
	if agentCtx.ProblemInfo != "" {
		contextParts = append(contextParts, "[当前题目信息] "+agentCtx.ProblemInfo)
	}
	if agentCtx.StudentCode != "" {
		contextParts = append(contextParts, "[学生代码] "+agentCtx.StudentCode)
	}
	if agentCtx.JudgeResult != "" {
		contextParts = append(contextParts, "[判题结果] "+agentCtx.JudgeResult)
	}
	if agentCtx.FailedCases != "" {
		contextParts = append(contextParts, "[未通过用例] "+agentCtx.FailedCases)
	}
	if len(contextParts) > 0 {
		classifyInput = strings.Join(contextParts, "\n") + "\n\n[用户提问] " + userQuery
	}

	messages := []*llmpb.ChatMessage{
		{
			Role: "system",
			Content: []*llmpb.ContentPart{
				{Type: "text", Text: systemPrompt},
			},
		},
		{
			Role: "user",
			Content: []*llmpb.ContentPart{
				{Type: "text", Text: classifyInput},
			},
		},
	}

	// 调用 LLM 进行意图分类（非流式，收集完整响应）
	// 意图分类不需要深度思考，显式禁用以加速
	llmReq := &llmpb.StreamChatRequest{
		ModelId:  r.classifyModelID,
		Messages: messages,
		ExtraParams: map[string]string{
			"enable_thinking": "false",
		},
	}

	llmStream, err := rpc.GetLLMClient().GetProxy().StreamChat(ctx, llmReq)
	if err != nil {
		log.Printf("[IntentRouter] 调用 LLM 意图分类失败: %v，使用关键词降级", err)
		return r.fallbackClassify(userQuery, agentCtx.UserRole), nil
	}

	// 收集完整响应
	var responseBuilder strings.Builder
	for {
		resp, err := llmStream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Printf("[IntentRouter] 接收 LLM 响应失败: %v，使用关键词降级", err)
			return r.fallbackClassify(userQuery, agentCtx.UserRole), nil
		}
		if len(resp.Choices) > 0 && resp.Choices[0].Delta != nil {
			responseBuilder.WriteString(resp.Choices[0].Delta.Content)
		}
		if resp.IsEnd {
			break
		}
	}

	responseText := strings.TrimSpace(responseBuilder.String())
	log.Printf("[IntentRouter] LLM 意图分类响应: %s", responseText)

	// 解析 JSON 响应
	result, err := parseIntentResponse(responseText)
	if err != nil {
		log.Printf("[IntentRouter] 解析意图响应失败: %v，使用关键词降级", err)
		return r.fallbackClassify(userQuery, agentCtx.UserRole), nil
	}

	// 设置 AgentRoute
	result.AgentRoute = intentCodeToRoute(result.IntentCode)

	durationMs := time.Since(startTime).Milliseconds()
	log.Printf("[IntentRouter] 意图分类完成: code=%s, confidence=%.2f, route=%s, 耗时=%dms",
		result.IntentCode, result.Confidence, result.AgentRoute, durationMs)

	return result, nil
}

// parseIntentResponse 解析 LLM 返回的意图 JSON
func parseIntentResponse(text string) (*model.IntentResult, error) {
	// 尝试提取 JSON（LLM 可能返回 markdown 代码块包裹的 JSON）
	jsonStr := text
	if idx := strings.Index(text, "{"); idx >= 0 {
		if endIdx := strings.LastIndex(text, "}"); endIdx > idx {
			jsonStr = text[idx : endIdx+1]
		}
	}

	var result model.IntentResult
	if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
		return nil, fmt.Errorf("JSON 解析失败: %w, 原始文本: %s", err, text)
	}

	// 验证必要字段
	if result.IntentCode == "" {
		return nil, fmt.Errorf("意图编码为空")
	}

	return &result, nil
}

// fallbackClassify 关键词降级分类（当 LLM 不可用时）
func (r *IntentRouter) fallbackClassify(query string, userRole string) *model.IntentResult {
	query = strings.ToLower(query)

	// 教师场景优先匹配
	if userRole == model.RoleTeacher {
		if containsAny(query, "测试用例", "测试数据", "生成用例", "自动出题") {
			return &model.IntentResult{
				IntentCode:   model.IntentTestcaseGen,
				IntentLevel1: "测试用例",
				IntentLevel2: "自动生成",
				Confidence:   0.7,
				Reasoning:    "关键词降级匹配：测试用例生成",
				AgentRoute:   model.AgentRouteTestcase,
			}
		}
	}

	// 解题相关
	if containsAny(query, "怎么做", "解法", "思路", "解题", "算法方案") {
		return &model.IntentResult{
			IntentCode:   model.IntentSolveThink,
			IntentLevel1: "解题相关",
			IntentLevel2: "题目解题思路",
			Confidence:   0.7,
			Reasoning:    "关键词降级匹配：解题思路",
			AgentRoute:   model.AgentRouteSolve,
		}
	}
	if containsAny(query, "bug", "错了", "wa", "re", "没过", "答案错误", "运行错误") {
		return &model.IntentResult{
			IntentCode:   model.IntentSolveBug,
			IntentLevel1: "解题相关",
			IntentLevel2: "代码BUG排查",
			Confidence:   0.7,
			Reasoning:    "关键词降级匹配：BUG排查",
			AgentRoute:   model.AgentRouteSolve,
		}
	}
	if containsAny(query, "优化", "超时", "tle", "mle", "太慢") {
		return &model.IntentResult{
			IntentCode:   model.IntentSolveOptimize,
			IntentLevel1: "解题相关",
			IntentLevel2: "代码优化",
			Confidence:   0.7,
			Reasoning:    "关键词降级匹配：代码优化",
			AgentRoute:   model.AgentRouteSolve,
		}
	}

	// 知识答疑
	if containsAny(query, "是什么", "什么是", "概念", "原理", "区别", "复杂度") {
		return &model.IntentResult{
			IntentCode:   model.IntentKnowledgeAlgo,
			IntentLevel1: "知识答疑",
			IntentLevel2: "算法概念解释",
			Confidence:   0.6,
			Reasoning:    "关键词降级匹配：知识概念",
			AgentRoute:   model.AgentRouteKnowledge,
		}
	}

	// IDE 调试
	if containsAny(query, "编译错误", "编译报错", "error", "报错", "compile") {
		return &model.IntentResult{
			IntentCode:   model.IntentCodeDebug,
			IntentLevel1: "IDE调试",
			IntentLevel2: "编程报错",
			Confidence:   0.7,
			Reasoning:    "关键词降级匹配：编译/运行报错",
			AgentRoute:   model.AgentRouteDebug,
		}
	}

	// 兜底
	return &model.IntentResult{
		IntentCode:   model.IntentOtherChat,
		IntentLevel1: "无关兜底",
		IntentLevel2: "闲聊/无诉求",
		Confidence:   0.5,
		Reasoning:    "关键词降级匹配：未匹配到明确意图",
		AgentRoute:   model.AgentRouteFallback,
	}
}

// intentCodeToRoute 意图编码映射到 Agent 路由
func intentCodeToRoute(intentCode string) string {
	switch intentCode {
	case model.IntentSolveThink, model.IntentSolveBug, model.IntentSolveOptimize:
		return model.AgentRouteSolve
	case model.IntentKnowledgeAlgo, model.IntentKnowledgeErr:
		return model.AgentRouteKnowledge
	case model.IntentTestcaseGen, model.IntentTestcaseImport:
		return model.AgentRouteTestcase
	case model.IntentCodeDebug:
		return model.AgentRouteDebug
	case model.IntentOperatePlat, model.IntentOperateDialog:
		return model.AgentRouteOperate
	case model.IntentProblemReview, model.IntentKnowledgeMgmt:
		return model.AgentRouteKnowledge
	default:
		return model.AgentRouteFallback
	}
}

// containsAny 检查字符串是否包含任一关键词
func containsAny(s string, keywords ...string) bool {
	for _, kw := range keywords {
		if strings.Contains(s, strings.ToLower(kw)) {
			return true
		}
	}
	return false
}
