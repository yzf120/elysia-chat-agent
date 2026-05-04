package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/joho/godotenv"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	reactagent "github.com/yzf120/elysia-chat-agent/agent"
	"github.com/yzf120/elysia-chat-agent/client"
	"github.com/yzf120/elysia-chat-agent/dao"
	"github.com/yzf120/elysia-chat-agent/model"
	promptpkg "github.com/yzf120/elysia-chat-agent/prompt"
	agentpb "github.com/yzf120/elysia-chat-agent/proto/agent"
	"github.com/yzf120/elysia-chat-agent/rag"
	"github.com/yzf120/elysia-chat-agent/rpc"
	llmpb "github.com/yzf120/elysia-llm-tool/proto/llm"
	"gorm.io/gorm"
)

// ==================== 全局初始化 ====================

var (
	testDB     *gorm.DB
	initOnce   sync.Once
	initErr    error
	testEngine *reactagent.ReactEngine
)

// initAll 初始化所有真实服务连接（只执行一次）
func initAll(t *testing.T) {
	initOnce.Do(func() {
		// 加载 .env 配置
		_ = godotenv.Load("../.env")

		// 1. 初始化 MySQL
		if err := dao.InitDB(); err != nil {
			initErr = fmt.Errorf("MySQL 初始化失败: %w", err)
			return
		}
		testDB = dao.GetDB()

		// 2. 初始化 Redis
		if err := client.InitRedisClient(); err != nil {
			initErr = fmt.Errorf("Redis 初始化失败: %w", err)
			return
		}

		// 3. 初始化 LLM RPC 客户端
		rpc.InitLLMClient()

		// 4. 初始化 RAG 服务
		rag.InitRAGService()

		// 5. 创建 ReactEngine
		testEngine = reactagent.NewReactEngine(testDB)

		log.Println("[集成测试] 所有服务初始化完成")
	})

	if initErr != nil {
		t.Fatalf("服务初始化失败: %v", initErr)
	}
}

// ==================== 环 1: MySQL 连接验证 ====================

func TestIntegration_Ring1_MySQLConnection(t *testing.T) {
	initAll(t)

	t.Run("数据库连接可用", func(t *testing.T) {
		sqlDB, err := testDB.DB()
		require.NoError(t, err, "获取底层 sql.DB 失败")

		err = sqlDB.Ping()
		assert.NoError(t, err, "MySQL Ping 失败")
		t.Log("✅ MySQL 连接正常")
	})

	t.Run("意图字典表可查询", func(t *testing.T) {
		intentDAO := dao.NewIntentDAO(testDB)
		dicts, err := intentDAO.ListValidIntentDicts()
		assert.NoError(t, err, "查询意图字典失败")
		t.Logf("✅ 意图字典表查询成功，有效记录数: %d", len(dicts))

		// 打印所有意图编码
		for _, d := range dicts {
			t.Logf("   - %s (%s / %s) priority=%d", d.IntentCode, d.IntentLevel1, d.IntentLevel2, d.Priority)
		}
	})

	t.Run("学生画像表可查询", func(t *testing.T) {
		profileDAO := dao.NewStudentProfileDAO(testDB)
		// 查询一个不存在的学生，验证表结构正确
		profile, err := profileDAO.GetProfileByStudentId("__integration_test_nonexist__")
		assert.NoError(t, err, "查询学生画像表失败")
		assert.Nil(t, profile, "不存在的学生应返回 nil")
		t.Log("✅ 学生画像表结构正常")
	})

	t.Run("问答行为表可查询", func(t *testing.T) {
		qaDAO := dao.NewQABehaviorDAO(testDB)
		records, err := qaDAO.GetRecentBehaviors("__integration_test_nonexist__", 5)
		assert.NoError(t, err, "查询问答行为表失败")
		assert.Empty(t, records)
		t.Log("✅ 问答行为表结构正常")
	})

	t.Run("提示词模板表可查询", func(t *testing.T) {
		intentDAO := dao.NewIntentDAO(testDB)
		// 尝试查询 SOLVE_BUG 的系统提示词模板
		tpl, err := intentDAO.GetActivePromptTemplate(model.IntentSolveBug, "system_prompt")
		assert.NoError(t, err, "查询提示词模板表失败")
		if tpl != nil {
			t.Logf("✅ 找到 SOLVE_BUG 的系统提示词模板 (ID=%d, 长度=%d)", tpl.Id, len(tpl.TemplateContent))
		} else {
			t.Log("⚠️ 未找到 SOLVE_BUG 的系统提示词模板（将使用代码内置模板）")
		}
	})
}

// ==================== 环 2: Redis 连接验证 ====================

func TestIntegration_Ring2_RedisConnection(t *testing.T) {
	initAll(t)

	rc := client.GetRedisClient()
	require.NotNil(t, rc, "Redis 客户端未初始化")

	t.Run("Redis_Ping", func(t *testing.T) {
		err := rc.Client.Ping(context.Background()).Err()
		assert.NoError(t, err, "Redis Ping 失败")
		t.Log("✅ Redis 连接正常")
	})

	t.Run("Redis_读写测试", func(t *testing.T) {
		ctx := context.Background()
		testKey := "elysia:integration_test:ping"
		testVal := "pong_" + time.Now().Format("20060102150405")

		// 写入
		err := rc.Client.Set(ctx, testKey, testVal, 30*time.Second).Err()
		assert.NoError(t, err, "Redis Set 失败")

		// 读取
		val, err := rc.Client.Get(ctx, testKey).Result()
		assert.NoError(t, err, "Redis Get 失败")
		assert.Equal(t, testVal, val, "Redis 读写值不一致")

		// 清理
		rc.Client.Del(ctx, testKey)
		t.Log("✅ Redis 读写正常")
	})

	t.Run("RAG服务初始化状态", func(t *testing.T) {
		ragSvc := rag.GetRAGService()
		if ragSvc != nil {
			t.Log("✅ RAG 服务已初始化")
		} else {
			t.Log("⚠️ RAG 服务未初始化（Redis 可能未配置）")
		}
	})
}

// ==================== 环 3: LLM RPC 连接验证 ====================

func TestIntegration_Ring3_LLMConnection(t *testing.T) {
	initAll(t)

	llmClient := rpc.GetLLMClient()
	require.NotNil(t, llmClient, "LLM 客户端未初始化")
	require.NotNil(t, llmClient.GetProxy(), "LLM Proxy 为 nil")

	t.Run("ListModels_查询模型列表", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		resp, err := llmClient.GetProxy().ListModels(ctx, &llmpb.ListModelsRequest{})
		require.NoError(t, err, "ListModels RPC 调用失败")
		require.NotNil(t, resp, "ListModels 响应为 nil")

		t.Logf("✅ LLM 服务连接正常，支持 %d 个模型:", len(resp.Models))
		for _, m := range resp.Models {
			t.Logf("   - %s (%s) stream=%v vision=%v", m.ModelId, m.Provider, m.SupportStream, m.SupportVision)
		}
	})

	t.Run("StreamChat_基础流式调用", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		req := &llmpb.StreamChatRequest{
			ModelId: "doubao-seed-2-0-lite-260215",
			Messages: []*llmpb.ChatMessage{
				{
					Role:    "user",
					Content: []*llmpb.ContentPart{{Type: "text", Text: "你好，请用一句话回复"}},
				},
			},
		}

		stream, err := llmClient.GetProxy().StreamChat(ctx, req)
		require.NoError(t, err, "StreamChat RPC 调用失败")

		var fullResponse strings.Builder
		chunkCount := 0
		gotEnd := false

		for {
			resp, err := stream.Recv()
			if err == io.EOF {
				break
			}
			require.NoError(t, err, "接收流式响应失败")

			if len(resp.Choices) > 0 && resp.Choices[0].Delta != nil {
				fullResponse.WriteString(resp.Choices[0].Delta.Content)
				chunkCount++
			}

			if resp.IsEnd {
				gotEnd = true
				if resp.Usage != nil {
					t.Logf("   Token 用量: prompt=%d, completion=%d, total=%d",
						resp.Usage.PromptTokens, resp.Usage.CompletionTokens, resp.Usage.TotalTokens)
				}
				break
			}
		}

		assert.True(t, gotEnd, "应收到结束标记")
		assert.Greater(t, chunkCount, 0, "应收到至少一个 chunk")
		assert.NotEmpty(t, fullResponse.String(), "LLM 回复不应为空")
		t.Logf("✅ LLM 流式调用正常，收到 %d 个 chunk，回复: %s", chunkCount, truncate(fullResponse.String(), 100))
	})
}

// ==================== 环 4: 意图分类验证 ====================

func TestIntegration_Ring4_IntentClassify(t *testing.T) {
	initAll(t)

	router := reactagent.NewIntentRouter("")

	testCases := []struct {
		name       string
		query      string
		role       string
		wantRoutes []string // 可接受的路由（LLM 可能有不同理解）
		wantCodes  []string // 可接受的意图编码
	}{
		{
			name:       "解题思路_学生",
			query:      "这道两数之和的题目怎么做？有什么好的算法思路吗？",
			role:       model.RoleStudent,
			wantRoutes: []string{model.AgentRouteSolve},
			wantCodes:  []string{model.IntentSolveThink},
		},
		{
			name:       "BUG排查_学生",
			query:      "我的代码提交后显示答案错误，但我本地测试是对的，能帮我看看哪里有bug吗？",
			role:       model.RoleStudent,
			wantRoutes: []string{model.AgentRouteSolve},
			wantCodes:  []string{model.IntentSolveBug},
		},
		{
			name:       "代码优化_学生",
			query:      "我的代码运行超时了，时间复杂度是O(n²)，怎么优化到O(nlogn)？",
			role:       model.RoleStudent,
			wantRoutes: []string{model.AgentRouteSolve},
			wantCodes:  []string{model.IntentSolveOptimize},
		},
		{
			name:       "知识概念_学生",
			query:      "什么是动态规划？它和贪心算法有什么区别？",
			role:       model.RoleStudent,
			wantRoutes: []string{model.AgentRouteKnowledge},
			wantCodes:  []string{model.IntentKnowledgeAlgo},
		},
		{
			name:       "编译报错_学生",
			query:      "编译报错了：error: expected ';' before '}' token，这是什么意思？",
			role:       model.RoleStudent,
			wantRoutes: []string{model.AgentRouteDebug},
			wantCodes:  []string{model.IntentCodeDebug},
		},
		{
			name:       "测试用例生成_教师",
			query:      "帮我为这道A+B问题生成10组测试用例，包含边界情况",
			role:       model.RoleTeacher,
			wantRoutes: []string{model.AgentRouteTestcase},
			wantCodes:  []string{model.IntentTestcaseGen},
		},
		{
			name:       "闲聊_学生",
			query:      "今天天气真好，你觉得呢？",
			role:       model.RoleStudent,
			wantRoutes: []string{model.AgentRouteFallback},
			wantCodes:  []string{model.IntentOtherChat},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			agentCtx := &model.AgentContext{
				OriginalQuery: tc.query,
				UserRole:      tc.role,
				Messages: []model.ChatMessage{
					{Role: "user", Content: tc.query},
				},
			}

			result, err := router.Classify(ctx, agentCtx)
			require.NoError(t, err, "意图分类不应返回错误")
			require.NotNil(t, result, "意图分类结果不应为 nil")

			t.Logf("   意图编码: %s", result.IntentCode)
			t.Logf("   一级分类: %s", result.IntentLevel1)
			t.Logf("   二级分类: %s", result.IntentLevel2)
			t.Logf("   置信度: %.2f", result.Confidence)
			t.Logf("   路由: %s", result.AgentRoute)
			t.Logf("   推理: %s", result.Reasoning)

			// 验证路由在可接受范围内
			routeOK := false
			for _, r := range tc.wantRoutes {
				if result.AgentRoute == r {
					routeOK = true
					break
				}
			}
			assert.True(t, routeOK, "路由 %s 不在预期范围 %v 内", result.AgentRoute, tc.wantRoutes)

			// 验证意图编码在可接受范围内
			codeOK := false
			for _, c := range tc.wantCodes {
				if result.IntentCode == c {
					codeOK = true
					break
				}
			}
			assert.True(t, codeOK, "意图编码 %s 不在预期范围 %v 内", result.IntentCode, tc.wantCodes)

			// 验证置信度合理
			assert.Greater(t, result.Confidence, 0.0, "置信度应 > 0")
			assert.LessOrEqual(t, result.Confidence, 1.0, "置信度应 <= 1.0")

			t.Logf("✅ 意图分类正确: %s → %s (%.0f%%)", tc.query[:min(20, len(tc.query))], result.IntentCode, result.Confidence*100)
		})
	}
}

// ==================== 环 5: RAG 检索验证 ====================

func TestIntegration_Ring5_RAGRetrieval(t *testing.T) {
	initAll(t)

	ragSvc := rag.GetRAGService()
	if ragSvc == nil {
		t.Skip("RAG 服务未初始化，跳过 RAG 测试")
	}

	ctx := context.Background()

	t.Run("存储知识条目", func(t *testing.T) {
		// 存储测试知识
		testDocs := []model.RAGDocument{
			{
				ID:         "integration_test_001",
				Content:    "动态规划（Dynamic Programming）是一种通过将复杂问题分解为更小的子问题来求解的算法设计方法。它的核心思想是记忆化，避免重复计算。常见的动态规划问题包括背包问题、最长公共子序列、编辑距离等。",
				SourceType: "knowledge_base",
				SourceID:   "kb_dp_001",
				Tags:       "动态规划,算法,背包问题",
			},
			{
				ID:         "integration_test_002",
				Content:    "哈希表（Hash Table）是一种通过哈希函数将键映射到值的数据结构，支持O(1)的平均查找、插入和删除操作。常见应用包括两数之和、字符频率统计等。",
				SourceType: "knowledge_base",
				SourceID:   "kb_hash_001",
				Tags:       "哈希表,数据结构",
			},
			{
				ID:         "integration_test_003",
				Content:    "数组越界（Array Index Out of Bounds）是编程中常见的运行时错误。在C/C++中不会自动检查边界，可能导致未定义行为；在Java/Python中会抛出异常。解决方法：检查循环边界条件，使用size()而非硬编码长度。",
				SourceType: "error_pattern",
				SourceID:   "err_001",
				Tags:       "数组越界,运行时错误,调试",
			},
		}

		for _, doc := range testDocs {
			err := ragSvc.StoreKnowledge(ctx, &doc)
			assert.NoError(t, err, "存储知识条目 %s 失败", doc.ID)
		}
		t.Logf("✅ 成功存储 %d 条测试知识", len(testDocs))
	})

	t.Run("检索_动态规划相关", func(t *testing.T) {
		query := &model.RAGQuery{
			Query:      "动态规划 背包问题 怎么做",
			TopK:       3,
			Threshold:  0.2,
			SourceType: "knowledge_base",
		}

		docs, err := ragSvc.Retrieve(ctx, query)
		assert.NoError(t, err, "RAG 检索失败")
		assert.NotEmpty(t, docs, "应检索到相关文档")

		for i, doc := range docs {
			t.Logf("   文档 %d: [%s] score=%.2f content=%s", i+1, doc.SourceType, doc.Score, truncate(doc.Content, 60))
		}

		// 验证第一条应该是动态规划相关
		if len(docs) > 0 {
			assert.Contains(t, docs[0].Content, "动态规划", "最相关的文档应包含'动态规划'")
			t.Logf("✅ RAG 检索正确，最相关文档: %s (score=%.2f)", truncate(docs[0].Content, 40), docs[0].Score)
		}
	})

	t.Run("检索_哈希表相关", func(t *testing.T) {
		query := &model.RAGQuery{
			Query:     "两数之和 哈希表 怎么解",
			TopK:      3,
			Threshold: 0.2,
		}

		docs, err := ragSvc.Retrieve(ctx, query)
		assert.NoError(t, err, "RAG 检索失败")

		for i, doc := range docs {
			t.Logf("   文档 %d: [%s] score=%.2f content=%s", i+1, doc.SourceType, doc.Score, truncate(doc.Content, 60))
		}

		if len(docs) > 0 {
			t.Logf("✅ RAG 检索到 %d 条相关文档", len(docs))
		}
	})

	t.Run("检索_错误模式", func(t *testing.T) {
		query := &model.RAGQuery{
			Query:      "数组越界 怎么解决",
			TopK:       3,
			Threshold:  0.2,
			SourceType: "error_pattern",
		}

		docs, err := ragSvc.Retrieve(ctx, query)
		assert.NoError(t, err, "RAG 检索失败")

		if len(docs) > 0 {
			assert.Contains(t, docs[0].Content, "数组越界", "应检索到数组越界相关文档")
			t.Logf("✅ 错误模式检索正确: %s", truncate(docs[0].Content, 60))
		}
	})

	t.Run("FormatRAGContext_格式化输出", func(t *testing.T) {
		docs := []model.RAGDocument{
			{Content: "测试内容1", SourceType: "knowledge_base", Score: 0.9},
			{Content: "测试内容2", SourceType: "problem_bank", Score: 0.7},
		}
		formatted := rag.FormatRAGContext(docs)
		assert.Contains(t, formatted, "参考资料 1")
		assert.Contains(t, formatted, "参考资料 2")
		assert.Contains(t, formatted, "90%")
		t.Logf("✅ RAG 上下文格式化正常")
	})

	// 清理测试数据
	t.Cleanup(func() {
		rc := client.GetRedisClient()
		if rc != nil {
			ctx := context.Background()
			rc.Client.Del(ctx,
				"elysia:knowledge:integration_test_001",
				"elysia:knowledge:integration_test_002",
				"elysia:knowledge:integration_test_003",
			)
			// 清理索引
			rc.Client.SRem(ctx, "elysia:knowledge:index:knowledge_base", "integration_test_001", "integration_test_002")
			rc.Client.SRem(ctx, "elysia:knowledge:index:error_pattern", "integration_test_003")
			t.Log("🧹 已清理 RAG 测试数据")
		}
	})
}

// ==================== 环 6: 用户画像加载验证 ====================

func TestIntegration_Ring6_UserProfile(t *testing.T) {
	initAll(t)

	profileDAO := dao.NewStudentProfileDAO(testDB)
	qaDAO := dao.NewQABehaviorDAO(testDB)

	// 准备测试数据
	testStudentID := "__integration_test_stu_" + time.Now().Format("150405") + "__"

	t.Run("创建测试学生画像", func(t *testing.T) {
		profile := &model.StudentProfile{
			StudentId:             testStudentID,
			DifficultyLevel:       "intermediate",
			TotalSubmissions:      120,
			AcceptRate:            0.65,
			SolvedProblemCount:    60,
			AttemptedProblemCount: 90,
			PreferredLanguage:     "C++",
			CommonErrors:          `["数组越界","空指针"]`,
			LanguageStats:         `{"C++":80,"Python":40}`,
		}
		err := testDB.Create(profile).Error
		require.NoError(t, err, "创建测试学生画像失败")
		t.Logf("✅ 创建测试学生画像: %s", testStudentID)
	})

	t.Run("查询学生画像", func(t *testing.T) {
		profile, err := profileDAO.GetProfileByStudentId(testStudentID)
		require.NoError(t, err, "查询学生画像失败")
		require.NotNil(t, profile, "学生画像不应为 nil")

		assert.Equal(t, "intermediate", profile.DifficultyLevel)
		assert.Equal(t, 120, profile.TotalSubmissions)
		assert.InDelta(t, 0.65, profile.AcceptRate, 0.01)
		assert.Equal(t, "C++", profile.PreferredLanguage)
		t.Logf("✅ 学生画像查询正确: level=%s, submissions=%d, accept_rate=%.0f%%",
			profile.DifficultyLevel, profile.TotalSubmissions, profile.AcceptRate*100)
	})

	t.Run("创建并查询问答行为记录", func(t *testing.T) {
		// 创建问答行为记录
		records := []*model.QABehavior{
			{
				StudentId:         testStudentID,
				ConversationId:    "conv_test_001",
				ProblemId:         1001,
				IntentCode:        model.IntentSolveThink,
				QuestionSummary:   "两数之和解题思路",
				KnowledgeTags:     `["哈希表","双指针"]`,
				DifficultyScore:   2.0,
				IsResolved:        1,
				ConversationTurns: 3,
				ConversationTime:  time.Now().Add(-1 * time.Hour),
			},
			{
				StudentId:         testStudentID,
				ConversationId:    "conv_test_002",
				ProblemId:         1002,
				IntentCode:        model.IntentKnowledgeAlgo,
				QuestionSummary:   "动态规划背包问题不理解",
				KnowledgeTags:     `["动态规划","背包问题"]`,
				DifficultyScore:   3.5,
				IsResolved:        2,
				ConversationTurns: 5,
				ConversationTime:  time.Now(),
			},
		}

		for _, r := range records {
			err := qaDAO.CreateQABehavior(r)
			require.NoError(t, err, "创建问答行为记录失败")
		}

		// 查询最近记录
		recentRecords, err := qaDAO.GetRecentBehaviors(testStudentID, 10)
		require.NoError(t, err, "查询问答行为记录失败")
		assert.Len(t, recentRecords, 2, "应有 2 条记录")

		// 验证按时间倒序（最新的在前）
		assert.Equal(t, "动态规划背包问题不理解", recentRecords[0].QuestionSummary)
		assert.Equal(t, "两数之和解题思路", recentRecords[1].QuestionSummary)

		t.Logf("✅ 问答行为记录创建和查询正确，共 %d 条", len(recentRecords))
		for _, r := range recentRecords {
			t.Logf("   - %s (resolved=%d, tags=%s)", r.QuestionSummary, r.IsResolved, r.KnowledgeTags)
		}
	})

	t.Run("画像注入到AgentContext", func(t *testing.T) {
		// 模拟 ReactEngine 的 loadUserProfile 流程
		profile, err := profileDAO.GetProfileByStudentId(testStudentID)
		require.NoError(t, err)
		require.NotNil(t, profile)

		var commonErrors []string
		_ = json.Unmarshal([]byte(profile.CommonErrors), &commonErrors)
		var languageStats map[string]int
		_ = json.Unmarshal([]byte(profile.LanguageStats), &languageStats)

		agentCtx := &model.AgentContext{
			UserID:   testStudentID,
			UserRole: model.RoleStudent,
			UserProfile: &model.UserProfile{
				DifficultyLevel:       profile.DifficultyLevel,
				TotalSubmissions:      profile.TotalSubmissions,
				AcceptRate:            profile.AcceptRate,
				SolvedProblemCount:    profile.SolvedProblemCount,
				AttemptedProblemCount: profile.AttemptedProblemCount,
				PreferredLanguage:     profile.PreferredLanguage,
				LanguageStats:         languageStats,
				CommonErrors:          commonErrors,
			},
		}

		assert.NotNil(t, agentCtx.UserProfile)
		assert.Equal(t, "intermediate", agentCtx.UserProfile.DifficultyLevel)
		assert.Len(t, agentCtx.UserProfile.CommonErrors, 2)
		assert.Equal(t, 80, agentCtx.UserProfile.LanguageStats["C++"])
		t.Log("✅ 画像注入到 AgentContext 正确")
	})

	// 清理测试数据
	t.Cleanup(func() {
		testDB.Where("student_id = ?", testStudentID).Delete(&model.StudentProfile{})
		testDB.Where("student_id = ?", testStudentID).Delete(&model.QABehavior{})
		t.Logf("🧹 已清理测试学生数据: %s", testStudentID)
	})
}

// ==================== 环 7: Prompt 组装验证 ====================

func TestIntegration_Ring7_PromptAssembly(t *testing.T) {
	initAll(t)

	t.Run("解题思路_Prompt组装", func(t *testing.T) {
		agentCtx := &model.AgentContext{
			IntentResult: &model.IntentResult{IntentCode: model.IntentSolveThink},
			ProblemID:    "1001",
			ProblemInfo:  "给定一个整数数组 nums 和一个目标值 target，找出数组中和为目标值的两个数",
			Language:     "C++",
			UserProfile: &model.UserProfile{
				DifficultyLevel:    "intermediate",
				TotalSubmissions:   100,
				AcceptRate:         0.6,
				SolvedProblemCount: 50,
				PreferredLanguage:  "C++",
			},
			RAGContext: "### 参考资料 1（相关度: 90%，来源: knowledge_base）\n哈希表可以用于O(1)查找...",
		}

		// 使用代码内置模板
		sysPrompt := buildSystemPromptForTest(agentCtx)
		assert.NotEmpty(t, sysPrompt, "系统提示词不应为空")
		t.Logf("✅ 解题思路 Prompt 组装完成，长度: %d 字符", len(sysPrompt))
		t.Logf("   前 200 字符: %s", truncate(sysPrompt, 200))
	})

	t.Run("BUG排查_Prompt组装", func(t *testing.T) {
		agentCtx := &model.AgentContext{
			IntentResult: &model.IntentResult{IntentCode: model.IntentSolveBug},
			ProblemID:    "1002",
			StudentCode:  "int main() { int a[10]; cout << a[10]; }",
			Language:     "C++",
			JudgeResult:  "wrong_answer",
		}

		sysPrompt := buildSystemPromptForTest(agentCtx)
		assert.NotEmpty(t, sysPrompt, "系统提示词不应为空")
		t.Logf("✅ BUG排查 Prompt 组装完成，长度: %d 字符", len(sysPrompt))
	})

	t.Run("知识答疑_Prompt组装", func(t *testing.T) {
		agentCtx := &model.AgentContext{
			IntentResult: &model.IntentResult{IntentCode: model.IntentKnowledgeAlgo},
		}

		sysPrompt := buildSystemPromptForTest(agentCtx)
		assert.NotEmpty(t, sysPrompt, "系统提示词不应为空")
		t.Logf("✅ 知识答疑 Prompt 组装完成，长度: %d 字符", len(sysPrompt))
	})

	t.Run("闲聊兜底_Prompt组装", func(t *testing.T) {
		agentCtx := &model.AgentContext{
			IntentResult: &model.IntentResult{IntentCode: model.IntentOtherChat},
		}

		sysPrompt := buildSystemPromptForTest(agentCtx)
		assert.NotEmpty(t, sysPrompt, "系统提示词不应为空")
		t.Logf("✅ 闲聊兜底 Prompt 组装完成，长度: %d 字符", len(sysPrompt))
	})
}

// ==================== 环 8: 完整 ReAct 链路端到端测试 ====================

// mockStreamChatServer 模拟 gRPC 流式服务端，收集所有 chunk
type mockStreamChatServer struct {
	agentpb.AgentService_StreamChatServer
	chunks []*agentpb.AgentStreamChatResponse
	mu     sync.Mutex
	ctx    context.Context
}

func newMockStreamServer(ctx context.Context) *mockStreamChatServer {
	return &mockStreamChatServer{
		chunks: make([]*agentpb.AgentStreamChatResponse, 0),
		ctx:    ctx,
	}
}

func (m *mockStreamChatServer) Send(resp *agentpb.AgentStreamChatResponse) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.chunks = append(m.chunks, resp)
	return nil
}

func (m *mockStreamChatServer) Context() context.Context {
	return m.ctx
}

func (m *mockStreamChatServer) getFullResponse() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	var sb strings.Builder
	for _, c := range m.chunks {
		sb.WriteString(c.Content)
	}
	return sb.String()
}

func (m *mockStreamChatServer) hasEndChunk() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, c := range m.chunks {
		if c.IsEnd {
			return true
		}
	}
	return false
}

func TestIntegration_Ring8_FullReActPipeline(t *testing.T) {
	initAll(t)
	require.NotNil(t, testEngine, "ReactEngine 未初始化")

	testCases := []struct {
		name        string
		query       string
		role        string
		problemInfo string
		studentCode string
		language    string
		judgeResult string
		modelID     string
		expectRoute string
	}{
		{
			name:        "学生_解题思路_两数之和",
			query:       "这道两数之和的题目怎么做？给我一些思路",
			role:        model.RoleStudent,
			problemInfo: "给定一个整数数组 nums 和一个目标值 target，请你在该数组中找出和为目标值的那两个整数，并返回它们的数组下标。",
			language:    "C++",
			modelID:     "doubao-seed-2-0-lite-260215",
			expectRoute: model.AgentRouteSolve,
		},
		{
			name:        "学生_知识概念_动态规划",
			query:       "什么是动态规划？它的核心思想是什么？",
			role:        model.RoleStudent,
			modelID:     "doubao-seed-2-0-lite-260215",
			expectRoute: model.AgentRouteKnowledge,
		},
		{
			name:        "学生_BUG排查_代码错误",
			query:       "我的代码提交后答案错误，能帮我看看哪里有问题吗？",
			role:        model.RoleStudent,
			problemInfo: "输入两个整数a和b，输出它们的和",
			studentCode: "#include<stdio.h>\nint main(){int a,b;scanf(\"%d%d\",&a,&b);printf(\"%d\",a-b);return 0;}",
			language:    "C",
			judgeResult: "wrong_answer",
			modelID:     "doubao-seed-2-0-lite-260215",
			expectRoute: model.AgentRouteSolve,
		},
		{
			name:        "学生_闲聊",
			query:       "你好呀，今天心情不错",
			role:        model.RoleStudent,
			modelID:     "doubao-seed-2-0-lite-260215",
			expectRoute: model.AgentRouteFallback,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			defer cancel()

			agentCtx := &model.AgentContext{
				UserID:         "__integration_test_e2e__",
				UserRole:       tc.role,
				SessionID:      "sess_integration_" + time.Now().Format("150405"),
				ConversationId: "conv_integration_" + time.Now().Format("150405"),
				OriginalQuery:  tc.query,
				ProblemInfo:    tc.problemInfo,
				StudentCode:    tc.studentCode,
				Language:       tc.language,
				JudgeResult:    tc.judgeResult,
				ModelID:        tc.modelID,
				Messages: []model.ChatMessage{
					{Role: "user", Content: tc.query},
				},
			}

			mockStream := newMockStreamServer(ctx)

			startTime := time.Now()
			err := testEngine.Execute(ctx, agentCtx, mockStream)
			duration := time.Since(startTime)

			require.NoError(t, err, "ReAct Execute 不应返回错误")

			// 验证收到了流式响应
			fullResponse := mockStream.getFullResponse()
			assert.NotEmpty(t, fullResponse, "LLM 回复不应为空")
			assert.True(t, mockStream.hasEndChunk(), "应收到结束 chunk")

			// 验证意图分类结果
			require.NotNil(t, agentCtx.IntentResult, "意图分类结果不应为 nil")
			t.Logf("   意图: %s (%s) confidence=%.2f",
				agentCtx.IntentResult.IntentCode, agentCtx.IntentResult.AgentRoute, agentCtx.IntentResult.Confidence)

			// 验证 chunk 数量合理
			t.Logf("   收到 %d 个 chunk，回复长度: %d 字符", len(mockStream.chunks), len(fullResponse))
			assert.Greater(t, len(mockStream.chunks), 1, "应收到多个 chunk（流式输出）")

			// 验证耗时合理
			t.Logf("   总耗时: %dms", duration.Milliseconds())
			assert.Less(t, duration, 60*time.Second, "总耗时不应超过 60 秒")

			t.Logf("✅ 完整链路测试通过: %s", tc.name)
			t.Logf("   回复前 150 字符: %s", truncate(fullResponse, 150))
		})
	}

	// 清理端到端测试产生的意图记录和问答行为记录
	t.Cleanup(func() {
		testDB.Where("user_id = ?", "__integration_test_e2e__").Delete(&model.UserIntentRecord{})
		testDB.Where("student_id = ?", "__integration_test_e2e__").Delete(&model.QABehavior{})
		t.Log("🧹 已清理端到端测试数据")
	})
}

// ==================== 环 9: 意图记录持久化验证 ====================

func TestIntegration_Ring9_IntentRecordPersistence(t *testing.T) {
	initAll(t)

	intentDAO := dao.NewIntentDAO(testDB)
	testUserID := "__integration_test_record_" + time.Now().Format("150405") + "__"

	t.Run("创建意图记录", func(t *testing.T) {
		record := &model.UserIntentRecord{
			UserID:           testUserID,
			SessionID:        "sess_test_001",
			QuestionID:       "1001",
			OriginalRequest:  "这道题怎么做",
			IntentCode:       model.IntentSolveThink,
			IntentLevel1:     "解题相关",
			IntentConfidence: 92.0,
			ResponseTimeMs:   150,
			RecognizeStatus:  1,
		}
		err := intentDAO.CreateIntentRecord(record)
		assert.NoError(t, err, "创建意图记录失败")
		assert.NotZero(t, record.Id, "记录 ID 应非零")
		t.Logf("✅ 意图记录创建成功: ID=%d", record.Id)
	})

	t.Run("验证记录已持久化", func(t *testing.T) {
		var count int64
		testDB.Model(&model.UserIntentRecord{}).Where("user_id = ?", testUserID).Count(&count)
		assert.Equal(t, int64(1), count, "应有 1 条意图记录")
		t.Logf("✅ 意图记录持久化验证通过，记录数: %d", count)
	})

	t.Cleanup(func() {
		testDB.Where("user_id = ?", testUserID).Delete(&model.UserIntentRecord{})
		t.Logf("🧹 已清理意图记录测试数据: %s", testUserID)
	})
}

// ==================== 环 10: 运行记录触发意图前置判断 ====================

func TestIntegration_Ring10_JudgeResultIntentOverride(t *testing.T) {
	initAll(t)

	router := reactagent.NewIntentRouter("")

	t.Run("accepted_强制走代码优化", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		agentCtx := &model.AgentContext{
			OriginalQuery: "帮我看看这段代码",
			UserRole:      model.RoleStudent,
			JudgeResult:   "accepted",
			Messages: []model.ChatMessage{
				{Role: "user", Content: "帮我看看这段代码"},
			},
		}

		result, err := router.Classify(ctx, agentCtx)
		require.NoError(t, err)
		assert.Equal(t, model.IntentSolveOptimize, result.IntentCode, "accepted 应强制路由到 SOLVE_OPTIMIZE")
		assert.Equal(t, 1.0, result.Confidence, "强制路由置信度应为 1.0")
		t.Logf("✅ accepted → SOLVE_OPTIMIZE 强制路由正确")
	})

	t.Run("partial_pass_交由LLM判断", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		agentCtx := &model.AgentContext{
			OriginalQuery: "我的代码只通过了一半的测试用例，帮我看看哪里有问题",
			UserRole:      model.RoleStudent,
			JudgeResult:   "partial_pass",
			FailedCases:   `[{"input":"5 3","expected":"8","actual":"2"}]`,
			Messages: []model.ChatMessage{
				{Role: "user", Content: "我的代码只通过了一半的测试用例，帮我看看哪里有问题"},
			},
		}

		result, err := router.Classify(ctx, agentCtx)
		require.NoError(t, err)
		// partial_pass 交由 LLM 判断，可能是 BUG 排查或代码优化
		validCodes := []string{model.IntentSolveBug, model.IntentSolveOptimize}
		codeOK := false
		for _, c := range validCodes {
			if result.IntentCode == c {
				codeOK = true
				break
			}
		}
		assert.True(t, codeOK, "partial_pass 应路由到 SOLVE_BUG 或 SOLVE_OPTIMIZE，实际: %s", result.IntentCode)
		t.Logf("✅ partial_pass → %s (LLM 判断正确)", result.IntentCode)
	})
}

// ==================== 辅助函数 ====================

// truncate 截断字符串
func truncate(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen]) + "..."
}

// min 取最小值
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// buildSystemPromptForTest 测试用的 Prompt 构建（使用代码内置模板）
func buildSystemPromptForTest(agentCtx *model.AgentContext) string {
	sysPrompt := promptpkg.GetSystemPromptByIntent(agentCtx)

	// 注入用户画像
	if agentCtx.UserProfile != nil {
		profilePrompt := promptpkg.BuildUserProfilePromptPublic(agentCtx.UserProfile)
		if profilePrompt != "" {
			sysPrompt += "\n\n" + profilePrompt
		}
	}

	// 注入 RAG 上下文
	if agentCtx.RAGContext != "" {
		sysPrompt += "\n\n## 参考资料（来自知识库检索）\n" + agentCtx.RAGContext
	}

	return sysPrompt
}
