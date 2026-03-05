package main

import (
	"github.com/gorilla/mux"
	"github.com/joho/godotenv"
	"github.com/yzf120/elysia-chat-agent/client"
	"github.com/yzf120/elysia-chat-agent/dao"
	agent "github.com/yzf120/elysia-chat-agent/proto/agent"
	"github.com/yzf120/elysia-chat-agent/router"
	"github.com/yzf120/elysia-chat-agent/rpc"
	"github.com/yzf120/elysia-chat-agent/service_impl"
	"log"
	"trpc.group/trpc-go/trpc-go"
)

func main() {
	// 加载环境变量
	err := godotenv.Load()
	if err != nil {
		log.Println("未找到.env文件，使用系统环境变量")
	}

	// 初始化数据库
	err = dao.InitDB()
	if err != nil {
		log.Fatalf("数据库初始化失败: %v", err)
	}
	defer dao.CloseDB()

	// 初始化Redis
	err = client.InitRedisClient()
	if err != nil {
		log.Fatalf("Redis初始化失败: %v", err)
	}
	defer client.GetRedisClient().Close()

	// 初始化 llm-tool RPC 客户端
	rpc.InitLLMClient()

	r := mux.NewRouter()
	router.RegisterRouter(r)

	// 创建trpc服务器
	s := trpc.NewServer()

	// 注册 AgentService（包含 StreamChat 流式接口）
	agent.RegisterAgentServiceService(s.Service("trpc.elysia.chat_agent.agent"), service_impl.NewAgentServiceImpl())

	router.Init()

	log.Println("Chat Agent 服务启动成功！")
	log.Println("支持的接口:")
	log.Println("  - StreamChat: 流式对话（通过 llm-tool RPC 调用底层模型）")

	// 启动服务器
	if err := s.Serve(); err != nil {
		log.Fatalf("服务器启动失败: %v", err)
	}
}
