package main

import (
	"github.com/gorilla/mux"
	"github.com/joho/godotenv"
	"github.com/yzf120/elysia-chat-agent/client"
	"github.com/yzf120/elysia-chat-agent/dao"
	agent "github.com/yzf120/elysia-chat-agent/proto/agent"
	"github.com/yzf120/elysia-chat-agent/router"
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

	r := mux.NewRouter()
	router.RegisterRouter(r)

	// 创建trpc服务器
	s := trpc.NewServer()
	agent.RegisterAgentServiceService(s.Service("trpc.elysia.chat_agent.agent"), nil)
	router.Init()

	// 启动服务器
	if err := s.Serve(); err != nil {
		log.Fatalf("服务器启动失败: %v", err)
	}
}
