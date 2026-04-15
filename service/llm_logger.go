package service

import (
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	agent "github.com/yzf120/elysia-chat-agent/proto/agent"
)

// ==================== LLM 日志管理 ====================

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
	_ = systemPrompt
}
