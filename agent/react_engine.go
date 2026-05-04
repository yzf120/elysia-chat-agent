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
	intentRouter   *IntentRouter
	intentDAO      *dao.IntentDAO
	profileDAO     *dao.StudentProfileDAO
	qaBehaviorDAO  *dao.QABehaviorDAO
	qaProfileAgent *QAProfileAgent
	ragService     *rag.Service
	maxSteps       int // ReAct 最大循环步数
}

// NewReactEngine 创建 ReAct 编排引擎
func NewReactEngine(db *gorm.DB) *ReactEngine {
	qaDAO := dao.NewQABehaviorDAO(db)
	return &ReactEngine{
		intentRouter:   NewIntentRouter(""),
		intentDAO:      dao.NewIntentDAO(db),
		profileDAO:     dao.NewStudentProfileDAO(db),
		qaBehaviorDAO:  qaDAO,
		qaProfileAgent: NewQAProfileAgent(qaDAO, ""),
		ragService:     rag.GetRAGService(),
		maxSteps:       5,
	}
}

// StreamResponse 流式响应回调
type StreamResponse func(content string, isEnd bool, finishReason string) error

// Execute 执行 ReAct 编排流程（流式输出）
// 核心流程：Fast Path 快判 → 意图分流 + RAG 检索 + 画像加载（并行） → Prompt 组装 → LLM 流式调用
func (e *ReactEngine) Execute(ctx context.Context, agentCtx *model.AgentContext, stream agentpb.AgentService_StreamChatServer) error {
	startTime := time.Now()
	trace := &model.ReActTrace{}

	log.Printf("[ReactEngine] 开始 ReAct 编排，用户: %s, 角色: %s, 模型: %s",
		agentCtx.UserID, agentCtx.UserRole, agentCtx.ModelID)

	// ==================== Fast Path: 简单问题快速路径 ====================
	if e.shouldFastPath(agentCtx) {
		log.Printf("[ReactEngine] 命中 Fast Path，跳过完整 ReAct 编排")
		return e.executeFastPath(ctx, agentCtx, stream, startTime)
	}

	// ==================== Step 1: 并行执行意图分析 + RAG 检索 + 画像加载 ====================
	parallelStart := time.Now()
	log.Printf("[ReactEngine] Step 1: 并行执行意图分析 + RAG 检索 + 画像加载...")

	// 通道：接收意图分析结果
	type intentResultWrapper struct {
		result *model.IntentResult
		err    error
		durMs  int64
	}
	intentCh := make(chan intentResultWrapper, 1)

	// 通道：接收 RAG 检索结果
	type ragResultWrapper struct {
		docs  []model.RAGDocument
		err   error
		durMs int64
	}
	ragCh := make(chan ragResultWrapper, 1)

	// 通道：画像加载完成信号
	profileDoneCh := make(chan int64, 1)

	// 并行任务1: 意图分析
	go func() {
		s := time.Now()
		result, err := e.intentRouter.Classify(ctx, agentCtx)
		intentCh <- intentResultWrapper{result: result, err: err, durMs: time.Since(s).Milliseconds()}
	}()

	// 并行任务2: RAG 检索（不带 sourceType，全量检索后再过滤）
	go func() {
		s := time.Now()
		docs, err := e.performRAGRetrievalNoFilter(ctx, agentCtx)
		ragCh <- ragResultWrapper{docs: docs, err: err, durMs: time.Since(s).Milliseconds()}
	}()

	// 并行任务3: 画像加载
	go func() {
		s := time.Now()
		if agentCtx.UserID != "" && agentCtx.UserRole == model.RoleStudent {
			e.loadUserProfile(agentCtx)
		}
		profileDoneCh <- time.Since(s).Milliseconds()
	}()

	// 等待所有并行任务完成
	intentWrapper := <-intentCh
	ragWrapper := <-ragCh
	profileDurMs := <-profileDoneCh

	// 处理意图分析结果
	intentResult := intentWrapper.result
	if intentWrapper.err != nil {
		log.Printf("[ReactEngine] 意图分析失败: %v，使用兜底", intentWrapper.err)
		intentResult = &model.IntentResult{
			IntentCode:   model.IntentOtherChat,
			IntentLevel1: "无关兜底",
			IntentLevel2: "闲聊/无诉求",
			Confidence:   0.5,
			AgentRoute:   model.AgentRouteFallback,
		}
	}
	agentCtx.IntentResult = intentResult

	log.Printf("[ReactEngine] 意图分析结果: code=%s, level1=%s, route=%s, confidence=%.2f, 耗时=%dms",
		intentResult.IntentCode, intentResult.IntentLevel1, intentResult.AgentRoute, intentResult.Confidence, intentWrapper.durMs)

	trace.Steps = append(trace.Steps, model.ReActStep{
		StepType:   "thought",
		Content:    fmt.Sprintf("意图分析: %s (%.0f%%)", intentResult.IntentCode, intentResult.Confidence*100),
		DurationMs: intentWrapper.durMs,
	})

	// 处理 RAG 检索结果
	ragDocs := ragWrapper.docs
	if ragWrapper.err != nil {
		log.Printf("[ReactEngine] RAG 检索失败: %v，继续无 RAG 上下文", ragWrapper.err)
	}

	if len(ragDocs) > 0 {
		agentCtx.RAGContext = rag.FormatRAGContext(ragDocs)
		log.Printf("[ReactEngine] RAG 检索到 %d 条相关文档, 耗时=%dms", len(ragDocs), ragWrapper.durMs)
		// 打印每条召回文档的得分和摘要
		for i, doc := range ragDocs {
			contentPreview := doc.Content
			if len(contentPreview) > 100 {
				contentPreview = contentPreview[:100] + "..."
			}
			log.Printf("[ReactEngine] RAG #%d: score=%.2f, source=%s, id=%s, preview=%s",
				i+1, doc.Score, doc.SourceType, doc.ID, contentPreview)
		}
	} else {
		log.Printf("[ReactEngine] RAG 未检索到相关文档, 耗时=%dms", ragWrapper.durMs)
	}

	trace.Steps = append(trace.Steps, model.ReActStep{
		StepType:   "action",
		Content:    fmt.Sprintf("RAG 检索: 召回 %d 条文档", len(ragDocs)),
		ToolName:   "rag_retrieve",
		DurationMs: ragWrapper.durMs,
	})

	log.Printf("[ReactEngine] 画像加载耗时=%dms", profileDurMs)
	log.Printf("[ReactEngine] Step 1 并行阶段总耗时=%dms (意图=%dms, RAG=%dms, 画像=%dms)",
		time.Since(parallelStart).Milliseconds(), intentWrapper.durMs, ragWrapper.durMs, profileDurMs)

	// ==================== Step 2: Thought - Prompt 组装 ====================
	step2Start := time.Now()
	log.Printf("[ReactEngine] Step 2: Prompt 组装...")
	log.Printf("[ReactEngine][DEBUG] agentCtx 上下文: ProblemInfo长度=%d, StudentCode长度=%d, JudgeResult=%s, FailedCases长度=%d",
		len(agentCtx.ProblemInfo), len(agentCtx.StudentCode), agentCtx.JudgeResult, len(agentCtx.FailedCases))

	systemPrompt := e.buildSystemPrompt(agentCtx)

	// 估算 Prompt token 数（中文约1.5字/token，英文约4字符/token，粗略按字符数/2估算）
	estPromptTokens := len([]rune(systemPrompt)) / 2
	var totalMsgChars int
	for _, msg := range agentCtx.Messages {
		totalMsgChars += len([]rune(msg.Content))
	}
	estMsgTokens := totalMsgChars / 2
	log.Printf("[ReactEngine][DEBUG] Prompt 估算: system_prompt=%d字(~%d tokens), 对话历史=%d字(~%d tokens), 总计~%d tokens",
		len([]rune(systemPrompt)), estPromptTokens, totalMsgChars, estMsgTokens, estPromptTokens+estMsgTokens)

	if len(systemPrompt) > 500 {
		log.Printf("[ReactEngine][DEBUG] 系统提示词前500字符: %s", systemPrompt[:500])
	} else {
		log.Printf("[ReactEngine][DEBUG] 系统提示词全文: %s", systemPrompt)
	}

	trace.Steps = append(trace.Steps, model.ReActStep{
		StepType:   "thought",
		Content:    fmt.Sprintf("Prompt 组装完成，系统提示词长度: %d, 估算总tokens: %d", len(systemPrompt), estPromptTokens+estMsgTokens),
		DurationMs: time.Since(step2Start).Milliseconds(),
	})

	// ==================== Step 3: Action - LLM 流式调用 ====================
	step3LLMStart := time.Now()
	log.Printf("[ReactEngine] Step 3: LLM 流式调用，模型: %s", agentCtx.ModelID)

	var fullResponse strings.Builder
	err := e.streamLLMCallWithCapture(ctx, agentCtx, systemPrompt, stream, &fullResponse)

	llmDurationMs := time.Since(step3LLMStart).Milliseconds()
	trace.Steps = append(trace.Steps, model.ReActStep{
		StepType:   "action",
		Content:    fmt.Sprintf("LLM 流式调用, 回复长度: %d字", len([]rune(fullResponse.String()))),
		ToolName:   "llm_stream_chat",
		DurationMs: llmDurationMs,
	})

	log.Printf("[ReactEngine] LLM 流式调用完成, 耗时=%dms, 回复长度=%d字", llmDurationMs, len([]rune(fullResponse.String())))

	if err != nil {
		return fmt.Errorf("LLM 流式调用失败: %w", err)
	}

	// ==================== Step 4: 记录意图识别结果 ====================
	go e.recordIntent(agentCtx, time.Since(startTime).Milliseconds())

	// ==================== Step 5: 异步问答画像分析 ====================
	if agentCtx.UserRole == model.RoleStudent && e.qaProfileAgent != nil {
		conversationTurns := len(agentCtx.Messages) / 2 // 粗略计算对话轮数
		if conversationTurns < 1 {
			conversationTurns = 1
		}
		e.qaProfileAgent.AnalyzeAndRecord(agentCtx, fullResponse.String(), agentCtx.ConversationId, conversationTurns)
	}

	trace.TotalSteps = len(trace.Steps)
	trace.TotalTimeMs = time.Since(startTime).Milliseconds()

	log.Printf("[ReactEngine] ReAct 编排完成，总步数: %d, 总耗时: %dms",
		trace.TotalSteps, trace.TotalTimeMs)

	return nil
}

// ==================== 内部方法 ====================

// performRAGRetrieval 执行 RAG 检索（依赖意图结果来过滤 sourceType）
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

// performRAGRetrievalNoFilter 执行 RAG 检索（不带 sourceType 过滤，用于并行场景）
func (e *ReactEngine) performRAGRetrievalNoFilter(ctx context.Context, agentCtx *model.AgentContext) ([]model.RAGDocument, error) {
	if e.ragService == nil {
		return nil, nil
	}

	query := &model.RAGQuery{
		Query:     agentCtx.OriginalQuery,
		TopK:      5,
		Threshold: 0.3,
	}

	return e.ragService.Retrieve(ctx, query)
}

// buildSystemPrompt 构建系统提示词
func (e *ReactEngine) buildSystemPrompt(agentCtx *model.AgentContext) string {
	// 优先从数据库获取自定义提示词模板
	if agentCtx.IntentResult != nil && e.intentDAO != nil {
		tpl, err := e.intentDAO.GetActivePromptTemplate(agentCtx.IntentResult.IntentCode, "system_prompt")
		if err == nil && tpl != nil {
			log.Printf("[ReactEngine][DEBUG] buildSystemPrompt: 使用数据库模板，intent=%s, 模板长度=%d", agentCtx.IntentResult.IntentCode, len(tpl.TemplateContent))
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

			// 注入题目上下文（题目信息、学生代码、判题结果、未通过用例）
			var sb strings.Builder
			sb.WriteString(content)
			prompt.InjectProblemContext(&sb, agentCtx)
			content = sb.String()

			// 注入 RAG 上下文
			if agentCtx.RAGContext != "" {
				content += "\n\n## 参考资料（来自知识库检索）\n" + agentCtx.RAGContext
			}

			return content
		}
	}

	// 降级使用代码中的提示词模板
	log.Printf("[ReactEngine][DEBUG] buildSystemPrompt: 使用代码中的提示词模板（降级路径）")
	return prompt.GetSystemPromptByIntent(agentCtx)
}

// streamLLMCallWithCapture 流式调用 LLM 并透传给调用方，同时捕获完整回复
func (e *ReactEngine) streamLLMCallWithCapture(ctx context.Context, agentCtx *model.AgentContext, systemPrompt string, stream agentpb.AgentService_StreamChatServer, fullResponse *strings.Builder) error {
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

		// 捕获完整回复内容（用于问答画像分析）
		if content != "" && fullResponse != nil {
			fullResponse.WriteString(content)
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

	// 计算并打印能力等级的具体分值（与 backend inferDifficultyLevel 算法一致）
	scoreAcceptRate := profile.AcceptRate * 40
	scoreSolved := float64(profile.SolvedProblemCount) / 30.0
	if scoreSolved > 1.0 {
		scoreSolved = 1.0
	}
	scoreSolved *= 30
	scoreSubmit := float64(profile.TotalSubmissions) / 100.0
	if scoreSubmit > 1.0 {
		scoreSubmit = 1.0
	}
	scoreSubmit *= 30
	totalScore := scoreAcceptRate + scoreSolved + scoreSubmit

	log.Printf("[ReactEngine] 学生画像已加载: user_id=%s, level=%s (综合分=%.1f: 通过率分=%.1f + 解题分=%.1f + 提交分=%.1f), accept_rate=%.2f%%, solved=%d, submissions=%d",
		agentCtx.UserID, profile.DifficultyLevel, totalScore, scoreAcceptRate, scoreSolved, scoreSubmit,
		profile.AcceptRate*100, profile.SolvedProblemCount, profile.TotalSubmissions)

	// 加载最近10条问答行为记录
	e.loadQABehaviors(agentCtx)
}

// loadQABehaviors 加载学生最近的问答行为记录到 UserProfile
func (e *ReactEngine) loadQABehaviors(agentCtx *model.AgentContext) {
	if e.qaBehaviorDAO == nil || agentCtx.UserID == "" || agentCtx.UserProfile == nil {
		return
	}

	records, err := e.qaBehaviorDAO.GetRecentBehaviors(agentCtx.UserID, 10)
	if err != nil {
		log.Printf("[ReactEngine] 查询问答行为记录失败: %v", err)
		return
	}

	if len(records) == 0 {
		log.Printf("[ReactEngine] 无问答行为记录 (user_id=%s)", agentCtx.UserID)
		return
	}

	summaries := make([]model.QABehaviorSummary, 0, len(records))
	for _, r := range records {
		var tags []string
		if r.KnowledgeTags != "" {
			_ = json.Unmarshal([]byte(r.KnowledgeTags), &tags)
		}
		summaries = append(summaries, model.QABehaviorSummary{
			QuestionSummary:   r.QuestionSummary,
			KnowledgeTags:     tags,
			DifficultyScore:   r.DifficultyScore,
			IntentCode:        r.IntentCode,
			IsResolved:        r.IsResolved,
			ConversationTurns: r.ConversationTurns,
			ConversationTime:  r.ConversationTime.Format("2006-01-02 15:04"),
		})
	}

	agentCtx.UserProfile.RecentQABehaviors = summaries
	log.Printf("[ReactEngine] 问答行为记录已加载: user_id=%s, 记录数=%d", agentCtx.UserID, len(summaries))
	for i, s := range summaries {
		resolvedStr := "未知"
		if s.IsResolved == 1 {
			resolvedStr = "已解决"
		} else if s.IsResolved == 2 {
			resolvedStr = "未解决"
		}
		log.Printf("[ReactEngine]   QA[%d]: 时间=%s, 意图=%s, 摘要=%s, 标签=%v, 难度=%.1f, 状态=%s, 轮次=%d",
			i+1, s.ConversationTime, s.IntentCode, s.QuestionSummary, s.KnowledgeTags, s.DifficultyScore, resolvedStr, s.ConversationTurns)
	}

	// 打印难度趋势统计
	var totalDiff float64
	var diffCnt int
	for _, s := range summaries {
		if s.DifficultyScore > 0 {
			totalDiff += s.DifficultyScore
			diffCnt++
		}
	}
	if diffCnt >= 2 {
		avgDiff := totalDiff / float64(diffCnt)
		mid := diffCnt / 2
		var recentSum, olderSum float64
		var recentCnt, olderCnt int
		idx := 0
		for _, s := range summaries {
			if s.DifficultyScore > 0 {
				if idx < mid {
					recentSum += s.DifficultyScore
					recentCnt++
				} else {
					olderSum += s.DifficultyScore
					olderCnt++
				}
				idx++
			}
		}
		trend := "平稳"
		if recentCnt > 0 && olderCnt > 0 {
			diff := recentSum/float64(recentCnt) - olderSum/float64(olderCnt)
			if diff > 0.5 {
				trend = "📈上升"
			} else if diff < -0.5 {
				trend = "📉下降"
			}
		}
		log.Printf("[ReactEngine] 难度趋势: user_id=%s, 有效难度记录=%d, 平均难度=%.1f/5.0, 趋势=%s",
			agentCtx.UserID, diffCnt, avgDiff, trend)
	} else if diffCnt == 1 {
		log.Printf("[ReactEngine] 难度趋势: user_id=%s, 仅1条有效难度记录(%.1f), 暂无法判断趋势",
			agentCtx.UserID, totalDiff)
	} else {
		log.Printf("[ReactEngine] 难度趋势: user_id=%s, 无有效难度记录(均为0), 暂无法判断趋势", agentCtx.UserID)
	}
}

// ==================== Fast Path 快速路径 ====================

// fastPathGreetings 问候/打招呼类关键词
var fastPathGreetings = []string{
	"你好", "hello", "hi", "嗨", "在吗", "在不在", "hey",
	"早上好", "下午好", "晚上好", "早安", "晚安", "午安",
	"嘿", "哈喽", "您好", "大家好",
}

// fastPathFarewells 告别类关键词
var fastPathFarewells = []string{
	"再见", "拜拜", "bye", "goodbye", "回见", "下次见",
	"走了", "先走了", "不聊了", "结束", "退出",
}

// fastPathThanks 感谢/礼貌类关键词
var fastPathThanks = []string{
	"谢谢", "感谢", "thanks", "thank you", "多谢", "辛苦了",
	"太感谢了", "非常感谢", "谢谢你", "感谢你", "thx",
}

// fastPathAcknowledge 确认/应答类关键词
var fastPathAcknowledge = []string{
	"好的", "明白了", "了解了", "知道了", "收到", "ok", "okay",
	"嗯", "嗯嗯", "哦", "哦哦", "行", "可以", "没问题",
	"懂了", "理解了", "get", "got it", "明白", "了解",
	"对", "对的", "是的", "没错", "确实", "确认",
}

// fastPathDialogControl 对话控制类关键词
var fastPathDialogControl = []string{
	"继续", "接着说", "然后呢", "下一步", "还有呢",
	"再说一遍", "重新说", "重复一下", "说详细点", "展开说说",
	"简单点", "通俗点", "换个说法", "举个例子",
}

// fastPathIdentity 身份/能力询问类关键词
var fastPathIdentity = []string{
	"你是谁", "你是什么", "你叫什么", "你的名字",
	"你能做什么", "你会什么", "你有什么功能", "怎么用你",
	"你是ai", "你是机器人", "你是人工智能",
	"介绍一下你", "自我介绍", "介绍下自己",
}

// fastPathEmotional 情感/反馈类关键词
var fastPathEmotional = []string{
	"太难了", "好难", "不懂", "看不懂", "不理解", "不明白",
	"厉害", "牛", "666", "nb", "nice", "cool", "awesome",
	"哈哈", "哈哈哈", "笑死", "有趣", "好玩",
	"无聊", "烦", "累了", "不想学了", "学不会",
	"加油", "冲", "开始吧", "准备好了",
}

// fastPathSmallTalk 闲聊/非编程话题类关键词
var fastPathSmallTalk = []string{
	"讲个笑话", "今天天气", "几点了", "什么时候",
	"吃了吗", "干嘛呢", "在干嘛", "忙吗",
	"聊天", "无聊", "陪我聊聊", "说点什么",
	"你喜欢什么", "你觉得呢", "推荐点什么",
}

// fastPathExactMatch 精确匹配列表（完全等于这些内容时直接走 Fast Path）
var fastPathExactMatch = []string{
	"?", "？", "!", "！", "...", "。。。",
	"嗯", "哦", "啊", "呢", "吧", "呀",
	"1", "2", "3", "是", "否", "对", "不",
	"test", "测试", "ping",
}

// programmingKeywords 编程相关关键词（命中则不走 Fast Path）
var programmingKeywords = []string{
	"代码", "算法", "编程", "报错", "bug", "error", "exception",
	"题", "排序", "数组", "链表", "函数", "变量", "循环", "递归",
	"编译", "运行", "执行", "输出", "返回", "调试", "debug",
	"类", "对象", "继承", "接口", "方法", "属性",
	"指针", "内存", "栈", "队列", "树", "图", "哈希",
	"时间复杂度", "空间复杂度", "复杂度",
	"java", "python", "c++", "golang", "javascript",
	"for", "while", "if", "else", "switch", "return",
	"import", "include", "package", "class", "struct",
	"sql", "数据库", "查询", "索引",
	"api", "http", "请求", "响应", "接口",
	"git", "提交", "分支", "合并",
	"测试用例", "单元测试", "断言",
	"二分", "动态规划", "贪心", "回溯", "dfs", "bfs",
	"leetcode", "力扣", "acm", "oj",
	"怎么写", "怎么实现", "如何实现", "怎么解", "如何解",
	"什么意思", "是什么", "区别", "对比", "优缺点",
}

// shouldFastPath 判断是否应该走快速路径（跳过完整 ReAct 编排）
// 三层判断：1. 前置排除条件 2. 精确匹配 3. 关键词模糊匹配 + 短消息兜底
func (e *ReactEngine) shouldFastPath(agentCtx *model.AgentContext) bool {
	// ===== 前置排除：有题目上下文时，不走 Fast Path =====
	if agentCtx.ProblemInfo != "" || agentCtx.StudentCode != "" || agentCtx.JudgeResult != "" {
		return false
	}

	query := strings.TrimSpace(agentCtx.OriginalQuery)
	queryRunes := []rune(query)

	// 消息为空，走 Fast Path
	if len(queryRunes) == 0 {
		return true
	}

	queryLower := strings.ToLower(query)

	// ===== Layer 1: 精确匹配（完全等于某些短语时直接命中） =====
	for _, exact := range fastPathExactMatch {
		if queryLower == exact {
			log.Printf("[ReactEngine] Fast Path 精确匹配: '%s'", query)
			return true
		}
	}

	// ===== Layer 2: 编程关键词排除（只要包含编程相关词，一律不走 Fast Path） =====
	for _, kw := range programmingKeywords {
		if strings.Contains(queryLower, kw) {
			return false
		}
	}

	// ===== Layer 3: 闲聊/非编程关键词匹配（消息 ≤ 30字时检查） =====
	if len(queryRunes) <= 30 {
		// 按优先级检查各类关键词组
		allFastPathGroups := []struct {
			name     string
			keywords []string
		}{
			{"问候", fastPathGreetings},
			{"告别", fastPathFarewells},
			{"感谢", fastPathThanks},
			{"确认应答", fastPathAcknowledge},
			{"对话控制", fastPathDialogControl},
			{"身份询问", fastPathIdentity},
			{"情感反馈", fastPathEmotional},
			{"闲聊", fastPathSmallTalk},
		}

		for _, group := range allFastPathGroups {
			for _, kw := range group.keywords {
				if strings.Contains(queryLower, kw) {
					log.Printf("[ReactEngine] Fast Path 命中[%s]关键词: '%s'", group.name, kw)
					return true
				}
			}
		}
	}

	// ===== Layer 4: 纯短消息兜底（≤ 10字且未命中编程关键词，大概率是闲聊） =====
	if len(queryRunes) <= 10 {
		log.Printf("[ReactEngine] Fast Path 命中短消息兜底规则 (长度=%d)", len(queryRunes))
		return true
	}

	// ===== Layer 5: 纯标点/表情/数字消息检测 =====
	if isPunctuationOrEmoji(query) {
		log.Printf("[ReactEngine] Fast Path 命中纯标点/表情消息")
		return true
	}

	return false
}

// isPunctuationOrEmoji 检测消息是否全部由标点符号、表情、空格组成
func isPunctuationOrEmoji(s string) bool {
	for _, r := range s {
		// 允许：标点、空格、数字、常见表情符号范围
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' {
			return false // 包含英文字母，可能是有意义的内容
		}
		if r >= 0x4e00 && r <= 0x9fff {
			return false // 包含中文汉字，可能是有意义的内容
		}
	}
	return true
}

// fastPathSystemPrompt Fast Path 使用的轻量系统提示词
const fastPathSystemPrompt = `你是 Elysia 教育平台的 AI 编程学习助手。

## 行为规范
1. 友好、简洁地回应用户
2. 如果用户打招呼或闲聊，自然回应并引导："我是你的编程学习助手！你可以问我算法问题、让我帮你分析代码，或者解释编程概念。有什么编程问题需要帮助吗？"
3. 如果用户表示感谢或告别，礼貌回应
4. 如果用户说"继续"、"然后呢"等对话控制词，基于上下文继续之前的话题
5. 如果用户表达情绪（如"太难了"、"学不会"），给予鼓励和支持，并提供学习建议
6. 如果用户问你是谁/能做什么，简要介绍自己的能力
7. 始终保持教育场景的专业性和友好性
8. 回复简短自然，不要过度展开`

// executeFastPath 执行快速路径：跳过意图分析、RAG、画像，直接调用 LLM
func (e *ReactEngine) executeFastPath(ctx context.Context, agentCtx *model.AgentContext, stream agentpb.AgentService_StreamChatServer, startTime time.Time) error {
	var fullResponse strings.Builder

	// 直接调用 LLM 流式输出
	err := e.streamLLMCallWithCapture(ctx, agentCtx, fastPathSystemPrompt, stream, &fullResponse)
	if err != nil {
		log.Printf("[ReactEngine] Fast Path LLM 调用失败: %v", err)
		return fmt.Errorf("Fast Path LLM 调用失败: %w", err)
	}

	durationMs := time.Since(startTime).Milliseconds()
	log.Printf("[ReactEngine] Fast Path 完成，耗时: %dms, 回复长度: %d字", durationMs, len([]rune(fullResponse.String())))

	// 异步记录意图（标记为 OTHER_CHAT）
	agentCtx.IntentResult = &model.IntentResult{
		IntentCode:   model.IntentOtherChat,
		IntentLevel1: "无关兜底",
		IntentLevel2: "闲聊/无诉求",
		Confidence:   1.0,
		Reasoning:    "Fast Path 规则快判命中",
		AgentRoute:   model.AgentRouteFallback,
	}
	go e.recordIntent(agentCtx, durationMs)

	// 异步问答画像分析（Fast Path 也需要记录）
	if agentCtx.UserRole == model.RoleStudent && e.qaProfileAgent != nil {
		conversationTurns := len(agentCtx.Messages) / 2
		if conversationTurns < 1 {
			conversationTurns = 1
		}
		e.qaProfileAgent.AnalyzeAndRecord(agentCtx, fullResponse.String(), agentCtx.ConversationId, conversationTurns)
	}

	return nil
}
