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
	"github.com/yzf120/elysia-chat-agent/prompt"
	agentpb "github.com/yzf120/elysia-chat-agent/proto/agent"
	"github.com/yzf120/elysia-chat-agent/rag"
	"github.com/yzf120/elysia-chat-agent/rpc"
	llmpb "github.com/yzf120/elysia-llm-tool/proto/llm"
	"gorm.io/gorm"
)

// ==================== ReAct 编排引擎 ====================

// ReactEngine ReAct 编排引擎，串联意图分流 → RAG 检索 → 画像注入 → Prompt 选择 → LLM 调用
type ReactEngine struct {
	intentRouter *IntentRouter
	intentDAO    *dao.IntentDAO
	profileDAO   *dao.StudentProfileDAO
	ragService   *rag.Service
	maxSteps     int // ReAct 最大循环步数
}

// NewReactEngine 创建 ReAct 编排引擎
func NewReactEngine(db *gorm.DB) *ReactEngine {
	return &ReactEngine{
		intentRouter: NewIntentRouter(""),
		intentDAO:    dao.NewIntentDAO(db),
		profileDAO:   dao.NewStudentProfileDAO(db),
		ragService:   rag.GetRAGService(),
		maxSteps:     5,
	}
}

// StreamResponse 流式响应回调
type StreamResponse func(content string, isEnd bool, finishReason string) error

// Execute 执行 ReAct 编排流程（流式输出）
// 核心流程：意图分流 → RAG 检索 → Prompt 组装 → LLM 流式调用
func (e *ReactEngine) Execute(ctx context.Context, agentCtx *model.AgentContext, stream agentpb.AgentService_StreamChatServer) error {
	startTime := time.Now()
	trace := &model.ReActTrace{}

	log.Printf("[ReactEngine] 开始 ReAct 编排，用户: %s, 角色: %s, 模型: %s",
		agentCtx.UserID, agentCtx.UserRole, agentCtx.ModelID)

	// ==================== Step 1: Thought - 意图分析 ====================
	step1Start := time.Now()
	log.Printf("[ReactEngine] Step 1: 意图分析...")

	intentResult, err := e.intentRouter.Classify(ctx, agentCtx)
	if err != nil {
		log.Printf("[ReactEngine] 意图分析失败: %v，使用兜底", err)
		intentResult = &model.IntentResult{
			IntentCode:   model.IntentOtherChat,
			IntentLevel1: "无关兜底",
			IntentLevel2: "闲聊/无诉求",
			Confidence:   0.5,
			AgentRoute:   model.AgentRouteFallback,
		}
	}
	agentCtx.IntentResult = intentResult

	trace.Steps = append(trace.Steps, model.ReActStep{
		StepType:   "thought",
		Content:    fmt.Sprintf("意图分析: %s (%.0f%%)", intentResult.IntentCode, intentResult.Confidence*100),
		DurationMs: time.Since(step1Start).Milliseconds(),
	})

	log.Printf("[ReactEngine] 意图分析结果: code=%s, level1=%s, route=%s, confidence=%.2f",
		intentResult.IntentCode, intentResult.IntentLevel1, intentResult.AgentRoute, intentResult.Confidence)

	// ==================== Step 2: Action - RAG 检索 ====================
	step2Start := time.Now()
	log.Printf("[ReactEngine] Step 2: RAG 检索...")

	ragDocs, err := e.performRAGRetrieval(ctx, agentCtx)
	if err != nil {
		log.Printf("[ReactEngine] RAG 检索失败: %v，继续无 RAG 上下文", err)
	}

	if len(ragDocs) > 0 {
		agentCtx.RAGContext = rag.FormatRAGContext(ragDocs)
		log.Printf("[ReactEngine] RAG 检索到 %d 条相关文档", len(ragDocs))
	} else {
		log.Printf("[ReactEngine] RAG 未检索到相关文档")
	}

	trace.Steps = append(trace.Steps, model.ReActStep{
		StepType:   "action",
		Content:    fmt.Sprintf("RAG 检索: 召回 %d 条文档", len(ragDocs)),
		ToolName:   "rag_retrieve",
		DurationMs: time.Since(step2Start).Milliseconds(),
	})

	// ==================== Step 2.5: 加载用户画像 ====================
	if agentCtx.UserID != "" && agentCtx.UserRole == model.RoleStudent {
		e.loadUserProfile(agentCtx)
	}

	// ==================== Step 3: Thought - Prompt 组装 ====================
	step3Start := time.Now()
	log.Printf("[ReactEngine] Step 3: Prompt 组装...")

	systemPrompt := e.buildSystemPrompt(agentCtx)

	trace.Steps = append(trace.Steps, model.ReActStep{
		StepType:   "thought",
		Content:    fmt.Sprintf("Prompt 组装完成，系统提示词长度: %d", len(systemPrompt)),
		DurationMs: time.Since(step3Start).Milliseconds(),
	})

	// ==================== Step 4: Action - LLM 流式调用 ====================
	step4Start := time.Now()
	log.Printf("[ReactEngine] Step 4: LLM 流式调用，模型: %s", agentCtx.ModelID)

	err = e.streamLLMCall(ctx, agentCtx, systemPrompt, stream)

	trace.Steps = append(trace.Steps, model.ReActStep{
		StepType:   "action",
		Content:    "LLM 流式调用",
		ToolName:   "llm_stream_chat",
		DurationMs: time.Since(step4Start).Milliseconds(),
	})

	if err != nil {
		return fmt.Errorf("LLM 流式调用失败: %w", err)
	}

	// ==================== Step 5: 记录意图识别结果 ====================
	go e.recordIntent(agentCtx, time.Since(startTime).Milliseconds())

	trace.TotalSteps = len(trace.Steps)
	trace.TotalTimeMs = time.Since(startTime).Milliseconds()

	log.Printf("[ReactEngine] ReAct 编排完成，总步数: %d, 总耗时: %dms",
		trace.TotalSteps, trace.TotalTimeMs)

	return nil
}

// ==================== 内部方法 ====================

// performRAGRetrieval 执行 RAG 检索
func (e *ReactEngine) performRAGRetrieval(ctx context.Context, agentCtx *model.AgentContext) ([]model.RAGDocument, error) {
	if e.ragService == nil {
		return nil, nil
	}

	// 根据意图确定检索来源类型
	sourceType := ""
	if agentCtx.IntentResult != nil {
		switch agentCtx.IntentResult.IntentCode {
		case model.IntentSolveThink, model.IntentSolveBug, model.IntentSolveOptimize:
			sourceType = "problem_bank"
		case model.IntentKnowledgeAlgo, model.IntentKnowledgeErr:
			sourceType = "knowledge_base"
		case model.IntentCodeDebug:
			sourceType = "error_pattern"
		}
	}

	query := &model.RAGQuery{
		Query:      agentCtx.OriginalQuery,
		TopK:       5,
		Threshold:  0.3,
		SourceType: sourceType,
	}

	return e.ragService.Retrieve(ctx, query)
}

// buildSystemPrompt 构建系统提示词
func (e *ReactEngine) buildSystemPrompt(agentCtx *model.AgentContext) string {
	// 优先从数据库获取自定义提示词模板
	if agentCtx.IntentResult != nil && e.intentDAO != nil {
		tpl, err := e.intentDAO.GetActivePromptTemplate(agentCtx.IntentResult.IntentCode, "system_prompt")
		if err == nil && tpl != nil {
			// 使用数据库中的模板，替换变量
			content := tpl.TemplateContent
			content = strings.ReplaceAll(content, "{problem_id}", agentCtx.ProblemID)
			content = strings.ReplaceAll(content, "{language}", agentCtx.Language)
			content = strings.ReplaceAll(content, "{judge_result}", agentCtx.JudgeResult)
			content = strings.ReplaceAll(content, "{error_message}", agentCtx.ErrorMessage)

			// 注入用户画像
			if agentCtx.UserProfile != nil {
				profilePrompt := prompt.BuildUserProfilePromptPublic(agentCtx.UserProfile)
				content = strings.ReplaceAll(content, "{user_profile}", profilePrompt)
				// 如果模板中没有 {user_profile} 占位符，则追加到末尾
				if !strings.Contains(tpl.TemplateContent, "{user_profile}") && profilePrompt != "" {
					content += "\n\n" + profilePrompt
				}
			}

			// 注入 RAG 上下文
			if agentCtx.RAGContext != "" {
				content += "\n\n## 参考资料（来自知识库检索）\n" + agentCtx.RAGContext
			}

			return content
		}
	}

	// 降级使用代码中的提示词模板
	return prompt.GetSystemPromptByIntent(agentCtx)
}

// streamLLMCall 流式调用 LLM 并透传给调用方
func (e *ReactEngine) streamLLMCall(ctx context.Context, agentCtx *model.AgentContext, systemPrompt string, stream agentpb.AgentService_StreamChatServer) error {
	// 构建 LLM 消息列表
	llmMessages := make([]*llmpb.ChatMessage, 0, len(agentCtx.Messages)+1)

	// 添加系统提示词
	if systemPrompt != "" {
		llmMessages = append(llmMessages, &llmpb.ChatMessage{
			Role: "system",
			Content: []*llmpb.ContentPart{
				{Type: "text", Text: systemPrompt},
			},
		})
	}

	// 添加对话历史
	for _, msg := range agentCtx.Messages {
		llmMessages = append(llmMessages, &llmpb.ChatMessage{
			Role: msg.Role,
			Content: []*llmpb.ContentPart{
				{Type: "text", Text: msg.Content},
			},
		})
	}

	// 构建 LLM 请求
	llmReq := &llmpb.StreamChatRequest{
		ModelId:     agentCtx.ModelID,
		Messages:    llmMessages,
		ExtraParams: agentCtx.ExtraParams,
	}

	// 调用 LLM
	llmStream, err := rpc.GetLLMClient().GetProxy().StreamChat(ctx, llmReq)
	if err != nil {
		return fmt.Errorf("调用 LLM 服务失败: %w", err)
	}

	// 流式透传
	for {
		llmResp, err := llmStream.Recv()
		if err == io.EOF {
			// 发送结束标记
			if sendErr := stream.Send(&agentpb.AgentStreamChatResponse{
				Content:      "",
				IsEnd:        true,
				FinishReason: "stop",
			}); sendErr != nil {
				log.Printf("[ReactEngine] 发送结束 chunk 失败: %v", sendErr)
			}
			break
		}
		if err != nil {
			return fmt.Errorf("接收 LLM 流式响应失败: %w", err)
		}

		content := ""
		finishReason := ""
		var promptTokens, completionTokens, totalTokens int32

		if len(llmResp.Choices) > 0 {
			choice := llmResp.Choices[0]
			if choice.Delta != nil {
				content = choice.Delta.Content
			}
			finishReason = choice.FinishReason
		}

		if llmResp.Usage != nil {
			promptTokens = llmResp.Usage.PromptTokens
			completionTokens = llmResp.Usage.CompletionTokens
			totalTokens = llmResp.Usage.TotalTokens
		}

		agentResp := &agentpb.AgentStreamChatResponse{
			Content:          content,
			IsEnd:            llmResp.IsEnd,
			FinishReason:     finishReason,
			PromptTokens:     promptTokens,
			CompletionTokens: completionTokens,
			TotalTokens:      totalTokens,
		}

		if sendErr := stream.Send(agentResp); sendErr != nil {
			log.Printf("[ReactEngine] 发送 chunk 给调用方失败: %v", sendErr)
			return sendErr
		}

		if llmResp.IsEnd {
			break
		}
	}

	return nil
}

// recordIntent 异步记录意图识别结果
func (e *ReactEngine) recordIntent(agentCtx *model.AgentContext, durationMs int64) {
	if e.intentDAO == nil || agentCtx.IntentResult == nil {
		return
	}

	record := &model.UserIntentRecord{
		UserID:           agentCtx.UserID,
		SessionID:        agentCtx.SessionID,
		QuestionID:       agentCtx.ProblemID,
		OriginalRequest:  agentCtx.OriginalQuery,
		IntentCode:       agentCtx.IntentResult.IntentCode,
		IntentLevel1:     agentCtx.IntentResult.IntentLevel1,
		IntentConfidence: agentCtx.IntentResult.Confidence * 100,
		ResponseTimeMs:   int(durationMs),
		RecognizeStatus:  1,
	}

	if err := e.intentDAO.CreateIntentRecord(record); err != nil {
		log.Printf("[ReactEngine] 记录意图失败: %v", err)
	}
}

// loadUserProfile 加载学生画像到 AgentContext
func (e *ReactEngine) loadUserProfile(agentCtx *model.AgentContext) {
	if e.profileDAO == nil || agentCtx.UserID == "" {
		return
	}

	profile, err := e.profileDAO.GetProfileByStudentId(agentCtx.UserID)
	if err != nil {
		log.Printf("[ReactEngine] 查询学生画像失败: %v", err)
		return
	}
	if profile == nil {
		log.Printf("[ReactEngine] 学生画像不存在 (user_id=%s)，跳过画像注入", agentCtx.UserID)
		return
	}

	// 解析常见错误类型
	var commonErrors []string
	if profile.CommonErrors != "" {
		_ = json.Unmarshal([]byte(profile.CommonErrors), &commonErrors)
	}

	// 解析各语言使用次数
	var languageStats map[string]int
	if profile.LanguageStats != "" {
		_ = json.Unmarshal([]byte(profile.LanguageStats), &languageStats)
	}

	agentCtx.UserProfile = &model.UserProfile{
		DifficultyLevel:       profile.DifficultyLevel,
		TotalSubmissions:      profile.TotalSubmissions,
		AcceptRate:            profile.AcceptRate,
		SolvedProblemCount:    profile.SolvedProblemCount,
		AttemptedProblemCount: profile.AttemptedProblemCount,
		PreferredLanguage:     profile.PreferredLanguage,
		LanguageStats:         languageStats,
		CommonErrors:          commonErrors,
	}

	log.Printf("[ReactEngine] 学生画像已加载: user_id=%s, level=%s, accept_rate=%.2f%%, solved=%d",
		agentCtx.UserID, profile.DifficultyLevel, profile.AcceptRate*100, profile.SolvedProblemCount)
}
