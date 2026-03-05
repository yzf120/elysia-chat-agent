package service_impl

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	agent "github.com/yzf120/elysia-chat-agent/proto/agent"
	"github.com/yzf120/elysia-chat-agent/rpc"
	llmpb "github.com/yzf120/elysia-llm-tool/proto/llm"
)

// ===== 日志文件管理 =====

var (
	llmLogger     *log.Logger
	llmLoggerOnce sync.Once
)

// getLLMLogger 获取LLM回复日志记录器（单例，输出到 log/ 目录）
func getLLMLogger() *log.Logger {
	llmLoggerOnce.Do(func() {
		logDir := "log"
		if err := os.MkdirAll(logDir, 0755); err != nil {
			log.Printf("[AgentService] 创建日志目录失败: %v，将使用标准输出", err)
			llmLogger = log.New(os.Stdout, "[LLM] ", log.LstdFlags)
			return
		}
		logFile := filepath.Join(logDir, "llm_response.log")
		f, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			log.Printf("[AgentService] 打开日志文件失败: %v，将使用标准输出", err)
			llmLogger = log.New(os.Stdout, "[LLM] ", log.LstdFlags)
			return
		}
		llmLogger = log.New(f, "", 0)
		log.Printf("[AgentService] LLM回复日志文件: %s", logFile)
	})
	return llmLogger
}

// logLLMResponse 记录LLM完整回复到日志文件
func logLLMResponse(provider, modelID, systemPrompt string, messages []agent.AgentChatMessage, fullResponse string, durationMs int64) {
	logger := getLLMLogger()
	now := time.Now().Format("2006-01-02 15:04:05.000")

	// 取最后一条用户消息
	lastUserMsg := ""
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == "user" {
			lastUserMsg = messages[i].Content
			break
		}
	}

	logger.Printf("========== [%s] ==========\n"+
		"时间: %s\n"+
		"提供商: %s | 模型: %s | 耗时: %dms\n"+
		"用户消息: %s\n"+
		"AI回复:\n%s\n",
		provider,
		now,
		provider, modelID, durationMs,
		lastUserMsg,
		fullResponse,
	)
	_ = systemPrompt // 系统提示词不记录到日志（避免日志过大）
}

// ===== AgentServiceImpl =====

// parseMessageContent 解析消息内容，提取文本和图片
// 消息格式：文本内容\n[IMAGE:data:image/...;base64,...]\n[IMAGE:...]
// 返回：文本内容、图片URL列表
func parseMessageContent(content string) (text string, imageURLs []string) {
	const imagePrefix = "[IMAGE:"
	const imageSuffix = "]"

	lines := strings.Split(content, "\n")
	var textLines []string
	for _, line := range lines {
		if strings.HasPrefix(line, imagePrefix) && strings.HasSuffix(line, imageSuffix) {
			// 提取图片URL（data:image/...;base64,...）
			url := line[len(imagePrefix) : len(line)-len(imageSuffix)]
			if url != "" {
				imageURLs = append(imageURLs, url)
			}
		} else {
			textLines = append(textLines, line)
		}
	}
	text = strings.Join(textLines, "\n")
	// 去掉末尾多余的换行
	text = strings.TrimRight(text, "\n")
	return
}

// buildLLMContentParts 将消息内容转换为 llm-tool ContentPart 列表（支持多模态）
func buildLLMContentParts(content string) []*llmpb.ContentPart {
	text, imageURLs := parseMessageContent(content)

	if len(imageURLs) == 0 {
		// 纯文本消息
		return []*llmpb.ContentPart{
			{Type: "text", Text: content},
		}
	}

	// 多模态消息：先放文本，再放图片
	parts := make([]*llmpb.ContentPart, 0, 1+len(imageURLs))
	if text != "" {
		parts = append(parts, &llmpb.ContentPart{Type: "text", Text: text})
	}
	for _, url := range imageURLs {
		parts = append(parts, &llmpb.ContentPart{
			Type: "image_url",
			ImageUrl: &llmpb.ImageURL{
				Url:    url,
				Detail: "auto",
			},
		})
	}
	return parts
}

// AgentServiceImpl Agent 服务实现
type AgentServiceImpl struct{}

// NewAgentServiceImpl 创建 Agent 服务实现
func NewAgentServiceImpl() *AgentServiceImpl {
	return &AgentServiceImpl{}
}

// StreamChat 流式对话：调用 llm-tool RPC，将流式结果透传给调用方
func (s *AgentServiceImpl) StreamChat(req *agent.AgentStreamChatRequest, stream agent.AgentService_StreamChatServer) error {
	log.Printf("[AgentService] StreamChat 开始，模型: %s，消息数: %d", req.ModelID, len(req.Messages))
	startTime := time.Now()

	// 构建发送给 llm-tool 的消息列表
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

	// 构建 llm-tool 请求，透传 extra_params（包含 enable_thinking 等）
	llmReq := &llmpb.StreamChatRequest{
		ModelId:     req.ModelID,
		Messages:    llmMessages,
		ExtraParams: req.ExtraParams, // 透传前端传来的额外参数
	}

	ctx := stream.Context()
	llmStream, err := rpc.GetLLMClient().GetProxy().StreamChat(ctx, llmReq)
	if err != nil {
		log.Printf("[AgentService] 调用 llm-tool StreamChat 失败: %v", err)
		return fmt.Errorf("调用 LLM 服务失败: %w", err)
	}

	// 判断提供商（用于日志）
	provider := "doubao"
	if strings.HasPrefix(strings.ToLower(req.ModelID), "qwen") {
		provider = "qwen"
	}

	// 收集完整回复用于日志
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

		// 累积完整回复
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

	// 记录完整回复到日志文件
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
