package service

import (
	"context"
	"fmt"
	"io"
	"log"
	"strings"
	"time"

	reactagent "github.com/yzf120/elysia-chat-agent/agent"
	"github.com/yzf120/elysia-chat-agent/model"
	agent "github.com/yzf120/elysia-chat-agent/proto/agent"
	"github.com/yzf120/elysia-chat-agent/rpc"
	llmpb "github.com/yzf120/elysia-llm-tool/proto/llm"
	"gorm.io/gorm"
)

// AgentServiceImpl Agent 服务实现
type AgentServiceImpl struct {
	reactEngine *reactagent.ReactEngine
	useReact    bool // 是否启用 ReAct 编排（可通过配置开关）
}

// NewAgentServiceImpl 创建 Agent 服务实现
func NewAgentServiceImpl() *AgentServiceImpl {
	return &AgentServiceImpl{
		useReact: false,
	}
}

// NewAgentServiceImplWithDB 创建带数据库的 Agent 服务实现（启用 ReAct 编排）
func NewAgentServiceImplWithDB(db *gorm.DB) *AgentServiceImpl {
	return &AgentServiceImpl{
		reactEngine: reactagent.NewReactEngine(db),
		useReact:    true,
	}
}

// StreamChat 流式对话：支持 ReAct 编排模式和直通模式
func (s *AgentServiceImpl) StreamChat(req *agent.AgentStreamChatRequest, stream agent.AgentService_StreamChatServer) error {
	log.Printf("[AgentService] StreamChat 开始，模型: %s，消息数: %d, ReAct: %v", req.ModelID, len(req.Messages), s.useReact)
	startTime := time.Now()

	// ===== ReAct 编排模式 =====
	if s.useReact && s.reactEngine != nil {
		return s.streamChatWithReact(req, stream, startTime)
	}

	// ===== 直通模式（兼容原有逻辑）=====
	return s.streamChatDirect(req, stream, startTime)
}

// streamChatWithReact ReAct 编排模式的流式对话
func (s *AgentServiceImpl) streamChatWithReact(req *agent.AgentStreamChatRequest, stream agent.AgentService_StreamChatServer, startTime time.Time) error {
	ctx := stream.Context()

	agentCtx := &model.AgentContext{
		ModelID:     req.ModelID,
		ExtraParams: req.ExtraParams,
	}

	if req.ExtraParams != nil {
		agentCtx.UserID = req.ExtraParams["user_id"]
		agentCtx.UserRole = req.ExtraParams["user_role"]
		agentCtx.SessionID = req.ExtraParams["session_id"]
		agentCtx.ConversationId = req.ExtraParams["conversation_id"]
		agentCtx.ProblemID = req.ExtraParams["problem_id"]
		agentCtx.ProblemInfo = req.ExtraParams["problem_info"]
		agentCtx.StudentCode = req.ExtraParams["student_code"]
		agentCtx.JudgeResult = req.ExtraParams["judge_result"]
		agentCtx.FailedCases = req.ExtraParams["failed_cases"]
		agentCtx.Language = req.ExtraParams["language"]
		agentCtx.ErrorMessage = req.ExtraParams["error_message"]
	}

	if agentCtx.UserRole == "" {
		agentCtx.UserRole = model.RoleStudent
	}

	for _, msg := range req.Messages {
		agentCtx.Messages = append(agentCtx.Messages, model.ChatMessage{
			Role:    msg.Role,
			Content: msg.Content,
		})
	}

	for i := len(req.Messages) - 1; i >= 0; i-- {
		if req.Messages[i].Role == "user" {
			agentCtx.OriginalQuery = req.Messages[i].Content
			break
		}
	}

	err := s.reactEngine.Execute(ctx, agentCtx, stream)
	if err != nil {
		log.Printf("[AgentService] ReAct 编排失败: %v，降级到直通模式", err)
		return s.streamChatDirect(req, stream, startTime)
	}

	durationMs := time.Since(startTime).Milliseconds()
	log.Printf("[AgentService] StreamChat(ReAct) 完成，模型: %s，耗时: %dms", req.ModelID, durationMs)
	return nil
}

// streamChatDirect 直通模式的流式对话（原有逻辑）
func (s *AgentServiceImpl) streamChatDirect(req *agent.AgentStreamChatRequest, stream agent.AgentService_StreamChatServer, startTime time.Time) error {
	llmMessages := make([]*llmpb.ChatMessage, 0, len(req.Messages)+1)

	if req.SystemPrompt != "" {
		llmMessages = append(llmMessages, &llmpb.ChatMessage{
			Role: "system",
			Content: []*llmpb.ContentPart{
				{Type: "text", Text: req.SystemPrompt},
			},
		})
	}

	for _, msg := range req.Messages {
		llmMessages = append(llmMessages, &llmpb.ChatMessage{
			Role:    msg.Role,
			Content: buildLLMContentParts(msg.Content),
		})
	}

	llmReq := &llmpb.StreamChatRequest{
		ModelId:     req.ModelID,
		Messages:    llmMessages,
		ExtraParams: req.ExtraParams,
	}

	ctx := stream.Context()
	llmStream, err := rpc.GetLLMClient().GetProxy().StreamChat(ctx, llmReq)
	if err != nil {
		log.Printf("[AgentService] 调用 llm-tool StreamChat 失败: %v", err)
		return fmt.Errorf("调用 LLM 服务失败: %w", err)
	}

	provider := "doubao"
	if strings.HasPrefix(strings.ToLower(req.ModelID), "qwen") {
		provider = "qwen"
	}

	var fullResponseBuilder strings.Builder

	for {
		llmResp, err := llmStream.Recv()
		if err == io.EOF {
			if sendErr := stream.Send(&agent.AgentStreamChatResponse{
				Content:      "",
				IsEnd:        true,
				FinishReason: "stop",
			}); sendErr != nil {
				log.Printf("[AgentService] 发送结束 chunk 失败: %v", sendErr)
			}
			break
		}
		if err != nil {
			log.Printf("[AgentService] 接收 llm-tool 响应失败: %v", err)
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

		if content != "" {
			fullResponseBuilder.WriteString(content)
		}

		agentResp := &agent.AgentStreamChatResponse{
			Content:          content,
			IsEnd:            llmResp.IsEnd,
			FinishReason:     finishReason,
			PromptTokens:     promptTokens,
			CompletionTokens: completionTokens,
			TotalTokens:      totalTokens,
		}

		if sendErr := stream.Send(agentResp); sendErr != nil {
			log.Printf("[AgentService] 发送 chunk 给调用方失败: %v", sendErr)
			return sendErr
		}

		if llmResp.IsEnd {
			break
		}
	}

	durationMs := time.Since(startTime).Milliseconds()
	fullResponse := fullResponseBuilder.String()
	if fullResponse != "" {
		go logLLMResponse(provider, req.ModelID, req.SystemPrompt, req.Messages, fullResponse, durationMs)
	}

	log.Printf("[AgentService] StreamChat 完成，模型: %s，耗时: %dms，回复长度: %d", req.ModelID, durationMs, len(fullResponse))
	return nil
}

// ===== 以下为原有接口的空实现（暂不使用）=====

func (s *AgentServiceImpl) CreateAgent(ctx context.Context, req *agent.CreateAgentRequest) (*agent.CreateAgentResponse, error) {
	return &agent.CreateAgentResponse{}, nil
}

func (s *AgentServiceImpl) GetAgent(ctx context.Context, req *agent.GetAgentRequest) (*agent.GetAgentResponse, error) {
	return &agent.GetAgentResponse{}, nil
}

func (s *AgentServiceImpl) UpdateAgent(ctx context.Context, req *agent.UpdateAgentRequest) (*agent.UpdateAgentResponse, error) {
	return &agent.UpdateAgentResponse{}, nil
}

func (s *AgentServiceImpl) DeleteAgent(ctx context.Context, req *agent.DeleteAgentRequest) (*agent.DeleteAgentResponse, error) {
	return &agent.DeleteAgentResponse{}, nil
}

func (s *AgentServiceImpl) ExecuteAgent(ctx context.Context, req *agent.ExecuteAgentRequest) (*agent.ExecuteAgentResponse, error) {
	return &agent.ExecuteAgentResponse{}, nil
}

// ListModels 查询底层支持的模型列表（通过 llm-tool RPC）
func (s *AgentServiceImpl) ListModels(ctx context.Context, req *agent.AgentListModelsRequest) (*agent.AgentListModelsResponse, error) {
	log.Printf("[AgentService] ListModels 请求，provider: %s", req.Provider)

	llmReq := &llmpb.ListModelsRequest{
		Provider: req.Provider,
	}

	llmResp, err := rpc.GetLLMClient().GetProxy().ListModels(ctx, llmReq)
	if err != nil {
		log.Printf("[AgentService] 调用 llm-tool ListModels 失败: %v", err)
		return nil, fmt.Errorf("查询模型列表失败: %w", err)
	}

	models := make([]agent.AgentModelInfo, 0, len(llmResp.Models))
	for _, m := range llmResp.Models {
		models = append(models, agent.AgentModelInfo{
			ModelID:       m.ModelId,
			ModelName:     m.ModelName,
			Provider:      m.Provider,
			Description:   m.Description,
			SupportStream: m.SupportStream,
			SupportVision: m.SupportVision,
		})
	}

	log.Printf("[AgentService] ListModels 返回 %d 个模型", len(models))
	return &agent.AgentListModelsResponse{Models: models}, nil
}
