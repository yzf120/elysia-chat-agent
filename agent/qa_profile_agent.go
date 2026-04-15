package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"strings"
	"time"

	"github.com/yzf120/elysia-chat-agent/dao"
	"github.com/yzf120/elysia-chat-agent/model"
	"github.com/yzf120/elysia-chat-agent/rpc"
	llmpb "github.com/yzf120/elysia-llm-tool/proto/llm"
)

// QAProfileAgent 问答画像 Agent，负责异步分析对话内容并生成问答行为记录
type QAProfileAgent struct {
	qaDAO   *dao.QABehaviorDAO
	modelID string // 用于分析的 LLM 模型（使用轻量模型降低成本）
}

// NewQAProfileAgent 创建问答画像 Agent
func NewQAProfileAgent(qaDAO *dao.QABehaviorDAO, modelID string) *QAProfileAgent {
	if modelID == "" {
		modelID = "qwen3-omni-flash" // 问答行为分析固定使用千问模型
	}
	return &QAProfileAgent{
		qaDAO:   qaDAO,
		modelID: modelID,
	}
}

// AnalyzeAndRecord 异步分析对话内容并记录问答行为
// 参数说明：
//   - agentCtx: 当前对话的 Agent 上下文
//   - aiResponse: AI 的完整回复内容
//   - conversationId: 本次对话的唯一标识
//   - conversationTurns: 本次对话的轮数
func (a *QAProfileAgent) AnalyzeAndRecord(agentCtx *model.AgentContext, aiResponse string, conversationId string, conversationTurns int) {
	if a.qaDAO == nil || agentCtx.UserID == "" {
		return
	}

	// 异步执行，不阻塞主流程
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		log.Printf("[QAProfileAgent] 开始分析对话，用户: %s, 会话: %s", agentCtx.UserID, conversationId)

		// 调用 LLM 分析对话内容
		analysis, err := a.analyzeConversation(ctx, agentCtx, aiResponse)
		if err != nil {
			log.Printf("[QAProfileAgent] 分析对话失败: %v", err)
			return
		}

		// 根据知识点标签计算难度分数
		difficultyScore := model.CalcDifficultyByTags(analysis.KnowledgeTags)

		// 序列化知识点标签为 JSON
		tagsJSON, _ := json.Marshal(analysis.KnowledgeTags)

		// 构建问答行为记录
		record := &model.QABehavior{
			StudentId:         agentCtx.UserID,
			ConversationId:    conversationId,
			ProblemId:         parseProblemId(agentCtx.ProblemID),
			IntentCode:        getIntentCode(agentCtx),
			QuestionSummary:   analysis.QuestionSummary,
			KnowledgeTags:     string(tagsJSON),
			DifficultyScore:   difficultyScore,
			IsResolved:        analysis.IsResolved,
			ConversationTurns: conversationTurns,
			ConversationTime:  time.Now(),
		}

		// 写入数据库
		if err := a.qaDAO.CreateQABehavior(record); err != nil {
			log.Printf("[QAProfileAgent] 写入问答行为记录失败: %v", err)
			return
		}

		log.Printf("[QAProfileAgent] 问答行为记录成功: user=%s, summary=%s, tags=%v, difficulty=%.1f, resolved=%d",
			agentCtx.UserID, analysis.QuestionSummary, analysis.KnowledgeTags, difficultyScore, analysis.IsResolved)
	}()
}

// analyzeConversation 调用 LLM 分析对话内容，提取问题摘要、知识点标签和解决状态
func (a *QAProfileAgent) analyzeConversation(ctx context.Context, agentCtx *model.AgentContext, aiResponse string) (*model.QAProfileAnalysis, error) {
	systemPrompt := a.buildAnalysisPrompt()
	userContent := a.buildAnalysisInput(agentCtx, aiResponse)

	// 构建 LLM 请求
	llmReq := &llmpb.StreamChatRequest{
		ModelId: a.modelID,
		Messages: []*llmpb.ChatMessage{
			{
				Role:    "system",
				Content: []*llmpb.ContentPart{{Type: "text", Text: systemPrompt}},
			},
			{
				Role:    "user",
				Content: []*llmpb.ContentPart{{Type: "text", Text: userContent}},
			},
		},
	}

	// 调用 LLM（流式接收，拼接完整响应）
	llmStream, err := rpc.GetLLMClient().GetProxy().StreamChat(ctx, llmReq)
	if err != nil {
		return nil, fmt.Errorf("调用 LLM 失败: %w", err)
	}

	var responseBuilder strings.Builder
	for {
		resp, err := llmStream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("接收 LLM 响应失败: %w", err)
		}
		if len(resp.Choices) > 0 && resp.Choices[0].Delta != nil {
			responseBuilder.WriteString(resp.Choices[0].Delta.Content)
		}
		if resp.IsEnd {
			break
		}
	}

	// 解析 LLM 返回的 JSON
	responseText := responseBuilder.String()
	responseText = extractJSON(responseText)

	var analysis model.QAProfileAnalysis
	if err := json.Unmarshal([]byte(responseText), &analysis); err != nil {
		log.Printf("[QAProfileAgent] 解析 LLM 响应失败，原始响应: %s, 错误: %v", responseText, err)
		return nil, fmt.Errorf("解析分析结果失败: %w", err)
	}

	// 校验知识点标签数量（2-4个）
	if len(analysis.KnowledgeTags) < 2 {
		// 标签太少，保留原样
	} else if len(analysis.KnowledgeTags) > 4 {
		analysis.KnowledgeTags = analysis.KnowledgeTags[:4]
	}

	return &analysis, nil
}

// buildAnalysisPrompt 构建问答画像分析的系统提示词
func (a *QAProfileAgent) buildAnalysisPrompt() string {
	// 按分类组织知识点标签列表
	tagsByCategory := model.GetKnowledgeTagNamesByCategory()
	var tagListBuilder strings.Builder
	categoryOrder := []string{
		"基础编程", "数据结构", "基础算法", "搜索与图论", "动态规划",
		"数学与数论", "高级数据结构", "高级算法", "字符串算法",
		"计算机基础", "操作系统与网络", "数据库与工程",
	}
	for _, cat := range categoryOrder {
		if tags, ok := tagsByCategory[cat]; ok {
			tagListBuilder.WriteString(fmt.Sprintf("- %s: %s\n", cat, strings.Join(tags, "、")))
		}
	}

	return fmt.Sprintf(`你是 Elysia 教育平台的问答行为分析引擎。你的任务是分析一段学生与AI助教的对话，提取结构化的问答行为数据。

## 分析任务
1. **问题摘要**：用一句话（不超过50字）概括学生提出的核心问题
2. **知识点标签**：从下方知识点标签库中选择 2-4 个最相关的标签（必须严格使用标签库中的名称）
3. **解决状态**：判断学生的问题是否在对话中得到了解决

## 知识点标签库（必须从以下标签中选择，不要自创标签）
%s
## 解决状态判断规则
- 1（已解决）：AI给出了明确的解答，学生表示理解或没有继续追问
- 2（未解决）：学生仍在追问、表示不理解、或对话中断
- 0（未知）：无法判断，如闲聊、操作类问题等

## 输出格式（严格 JSON，不要输出任何其他内容）
{"question_summary":"学生询问两数之和的解题思路","knowledge_tags":["哈希表","双指针"],"is_resolved":1}

## 注意事项
- 知识点标签必须从标签库中精确选取，不要自创或修改标签名称
- 每次选择 2-4 个最相关的标签
- 问题摘要要简洁准确，不超过50字
- 如果对话内容是闲聊或操作类问题，知识点标签可以选择最接近的基础标签`, tagListBuilder.String())
}

// buildAnalysisInput 构建分析输入（对话内容）
func (a *QAProfileAgent) buildAnalysisInput(agentCtx *model.AgentContext, aiResponse string) string {
	var sb strings.Builder

	// 题目上下文
	if agentCtx.ProblemInfo != "" {
		sb.WriteString(fmt.Sprintf("[题目信息] %s\n\n", agentCtx.ProblemInfo))
	}

	// 学生代码
	if agentCtx.StudentCode != "" {
		sb.WriteString(fmt.Sprintf("[学生代码]\n```%s\n%s\n```\n\n", agentCtx.Language, agentCtx.StudentCode))
	}

	// 学生问题
	sb.WriteString(fmt.Sprintf("[学生问题] %s\n\n", agentCtx.OriginalQuery))

	// AI 回复（截取前 500 字符，避免 token 过多）
	truncatedResponse := aiResponse
	if len(truncatedResponse) > 500 {
		truncatedResponse = truncatedResponse[:500] + "..."
	}
	sb.WriteString(fmt.Sprintf("[AI回复] %s\n", truncatedResponse))

	// 意图信息
	if agentCtx.IntentResult != nil {
		sb.WriteString(fmt.Sprintf("\n[意图分类] %s - %s", agentCtx.IntentResult.IntentLevel1, agentCtx.IntentResult.IntentLevel2))
	}

	return sb.String()
}

// ==================== 辅助函数 ====================

// extractJSON 从 LLM 响应中提取 JSON 字符串（处理可能的 markdown 代码块包裹）
func extractJSON(text string) string {
	text = strings.TrimSpace(text)

	// 尝试提取 ```json ... ``` 中的内容
	if idx := strings.Index(text, "```json"); idx != -1 {
		start := idx + 7
		end := strings.Index(text[start:], "```")
		if end != -1 {
			return strings.TrimSpace(text[start : start+end])
		}
	}

	// 尝试提取 ``` ... ``` 中的内容
	if idx := strings.Index(text, "```"); idx != -1 {
		start := idx + 3
		// 跳过可能的语言标识行
		if nlIdx := strings.Index(text[start:], "\n"); nlIdx != -1 {
			start = start + nlIdx + 1
		}
		end := strings.Index(text[start:], "```")
		if end != -1 {
			return strings.TrimSpace(text[start : start+end])
		}
	}

	// 尝试提取 { ... } 中的内容
	startIdx := strings.Index(text, "{")
	endIdx := strings.LastIndex(text, "}")
	if startIdx != -1 && endIdx != -1 && endIdx > startIdx {
		return text[startIdx : endIdx+1]
	}

	return text
}

// parseProblemId 将字符串形式的题目ID转为 int64
func parseProblemId(problemID string) int64 {
	if problemID == "" {
		return 0
	}
	var id int64
	fmt.Sscanf(problemID, "%d", &id)
	return id
}

// getIntentCode 从 AgentContext 中获取意图编码
func getIntentCode(agentCtx *model.AgentContext) string {
	if agentCtx.IntentResult != nil {
		return agentCtx.IntentResult.IntentCode
	}
	return ""
}
