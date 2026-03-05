package rpc

import (
	"log"
	"os"

	llmpb "github.com/yzf120/elysia-llm-tool/proto/llm"
	"trpc.group/trpc-go/trpc-go/client"
)

// LLMClient llm-tool RPC 客户端
type LLMClient struct {
	proxy llmpb.LLMServiceClientProxy
}

var defaultLLMClient *LLMClient

// InitLLMClient 初始化 llm-tool RPC 客户端
func InitLLMClient() {
	llmToolAddr := os.Getenv("LLM_TOOL_ADDR")
	if llmToolAddr == "" {
		llmToolAddr = "127.0.0.1:9001"
	}

	proxy := llmpb.NewLLMServiceClientProxy(
		client.WithTarget("ip://"+llmToolAddr),
		client.WithTimeout(0), // 流式接口不设超时
	)

	defaultLLMClient = &LLMClient{proxy: proxy}
	log.Printf("LLM Tool RPC 客户端初始化完成，地址: %s", llmToolAddr)
}

// GetLLMClient 获取 llm-tool RPC 客户端
func GetLLMClient() *LLMClient {
	if defaultLLMClient == nil {
		InitLLMClient()
	}
	return defaultLLMClient
}

// GetProxy 获取底层 proxy
func (c *LLMClient) GetProxy() llmpb.LLMServiceClientProxy {
	return c.proxy
}
