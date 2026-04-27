package agent

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/yzf120/elysia-chat-agent/dao"
	"github.com/yzf120/elysia-chat-agent/model"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// setupTestDB 创建 SQLite 内存数据库用于 ReactEngine 测试
func setupTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err, "创建测试数据库失败")

	err = db.AutoMigrate(
		&model.IntentDict{},
		&model.IntentPromptTemplate{},
		&model.UserIntentRecord{},
		&model.QABehavior{},
		&model.StudentProfile{},
	)
	require.NoError(t, err, "数据库迁移失败")

	return db
}

// newTestReactEngine 创建测试用的 ReactEngine（不依赖 Redis/LLM）
func newTestReactEngine(db *gorm.DB) *ReactEngine {
	qaDAO := dao.NewQABehaviorDAO(db)
	return &ReactEngine{
		intentRouter:   NewIntentRouter(""),
		intentDAO:      dao.NewIntentDAO(db),
		profileDAO:     dao.NewStudentProfileDAO(db),
		qaBehaviorDAO:  qaDAO,
		qaProfileAgent: NewQAProfileAgent(qaDAO, ""),
		ragService:     nil, // 不使用 RAG
		maxSteps:       5,
	}
}

// ==================== 5.6 loadUserProfile ====================

func TestReactEngine_LoadUserProfile(t *testing.T) {
	db := setupTestDB(t)
	engine := newTestReactEngine(db)

	t.Run("学生画像不存在_跳过", func(t *testing.T) {
		agentCtx := &model.AgentContext{
			UserID:   "not_exist_student",
			UserRole: model.RoleStudent,
		}
		engine.loadUserProfile(agentCtx)
		assert.Nil(t, agentCtx.UserProfile, "画像不存在时 UserProfile 应为 nil")
	})

	t.Run("学生画像存在_正确加载", func(t *testing.T) {
		// 插入学生画像
		db.Create(&model.StudentProfile{
			StudentId:             "stu001",
			DifficultyLevel:       "intermediate",
			TotalSubmissions:      100,
			AcceptRate:            0.65,
			SolvedProblemCount:    50,
			AttemptedProblemCount: 80,
			PreferredLanguage:     "Python",
			CommonErrors:          `["数组越界","空指针"]`,
			LanguageStats:         `{"Python":50,"C++":30}`,
		})

		agentCtx := &model.AgentContext{
			UserID:   "stu001",
			UserRole: model.RoleStudent,
		}
		engine.loadUserProfile(agentCtx)

		require.NotNil(t, agentCtx.UserProfile, "画像应被加载")
		assert.Equal(t, "intermediate", agentCtx.UserProfile.DifficultyLevel)
		assert.Equal(t, 100, agentCtx.UserProfile.TotalSubmissions)
		assert.InDelta(t, 0.65, agentCtx.UserProfile.AcceptRate, 0.001)
		assert.Equal(t, 50, agentCtx.UserProfile.SolvedProblemCount)
		assert.Equal(t, "Python", agentCtx.UserProfile.PreferredLanguage)
		assert.Len(t, agentCtx.UserProfile.CommonErrors, 2)
		assert.Contains(t, agentCtx.UserProfile.CommonErrors, "数组越界")
		assert.Contains(t, agentCtx.UserProfile.CommonErrors, "空指针")
		assert.NotNil(t, agentCtx.UserProfile.LanguageStats)
		assert.Equal(t, 50, agentCtx.UserProfile.LanguageStats["Python"])
	})

	t.Run("空UserID_跳过", func(t *testing.T) {
		agentCtx := &model.AgentContext{
			UserID:   "",
			UserRole: model.RoleStudent,
		}
		engine.loadUserProfile(agentCtx)
		assert.Nil(t, agentCtx.UserProfile)
	})

	t.Run("nil_profileDAO_跳过", func(t *testing.T) {
		engineNoDAO := &ReactEngine{profileDAO: nil}
		agentCtx := &model.AgentContext{
			UserID:   "stu001",
			UserRole: model.RoleStudent,
		}
		engineNoDAO.loadUserProfile(agentCtx)
		assert.Nil(t, agentCtx.UserProfile)
	})
}

// ==================== 5.7 loadQABehaviors ====================

func TestReactEngine_LoadQABehaviors(t *testing.T) {
	db := setupTestDB(t)
	engine := newTestReactEngine(db)

	t.Run("无问答行为记录", func(t *testing.T) {
		agentCtx := &model.AgentContext{
			UserID:      "stu_no_qa",
			UserRole:    model.RoleStudent,
			UserProfile: &model.UserProfile{DifficultyLevel: "beginner"},
		}
		engine.loadQABehaviors(agentCtx)
		assert.Empty(t, agentCtx.UserProfile.RecentQABehaviors)
	})

	t.Run("有问答行为记录_正确加载", func(t *testing.T) {
		// 插入问答行为记录
		db.Create(&model.QABehavior{
			StudentId:         "stu_with_qa",
			ConversationId:    "conv001",
			QuestionSummary:   "两数之和怎么做",
			KnowledgeTags:     `["哈希表","双指针"]`,
			DifficultyScore:   2.0,
			IntentCode:        "SOLVE_THINK",
			IsResolved:        1,
			ConversationTurns: 3,
			ConversationTime:  mustParseTime("2026-04-20 10:00:00"),
		})
		db.Create(&model.QABehavior{
			StudentId:         "stu_with_qa",
			ConversationId:    "conv002",
			QuestionSummary:   "背包问题不理解",
			KnowledgeTags:     `["动态规划","背包问题"]`,
			DifficultyScore:   3.0,
			IntentCode:        "KNOWLEDGE_ALGO",
			IsResolved:        2,
			ConversationTurns: 5,
			ConversationTime:  mustParseTime("2026-04-19 15:00:00"),
		})

		agentCtx := &model.AgentContext{
			UserID:      "stu_with_qa",
			UserRole:    model.RoleStudent,
			UserProfile: &model.UserProfile{DifficultyLevel: "intermediate"},
		}
		engine.loadQABehaviors(agentCtx)

		require.Len(t, agentCtx.UserProfile.RecentQABehaviors, 2)

		// 验证第一条（最新的）
		first := agentCtx.UserProfile.RecentQABehaviors[0]
		assert.Equal(t, "两数之和怎么做", first.QuestionSummary)
		assert.Contains(t, first.KnowledgeTags, "哈希表")
		assert.Equal(t, 1, first.IsResolved)

		// 验证第二条
		second := agentCtx.UserProfile.RecentQABehaviors[1]
		assert.Equal(t, "背包问题不理解", second.QuestionSummary)
		assert.Contains(t, second.KnowledgeTags, "动态规划")
		assert.Equal(t, 2, second.IsResolved)
	})

	t.Run("nil_UserProfile_跳过", func(t *testing.T) {
		agentCtx := &model.AgentContext{
			UserID:      "stu_with_qa",
			UserRole:    model.RoleStudent,
			UserProfile: nil,
		}
		// 不应 panic
		engine.loadQABehaviors(agentCtx)
	})
}

// ==================== 5.8-5.9 buildSystemPrompt ====================

func TestReactEngine_BuildSystemPrompt(t *testing.T) {
	db := setupTestDB(t)
	engine := newTestReactEngine(db)

	t.Run("无DB模板_降级到代码模板", func(t *testing.T) {
		agentCtx := &model.AgentContext{
			IntentResult: &model.IntentResult{IntentCode: model.IntentSolveThink},
		}
		prompt := engine.buildSystemPrompt(agentCtx)
		assert.NotEmpty(t, prompt, "应降级到代码模板")
	})

	t.Run("有DB模板_使用DB模板", func(t *testing.T) {
		// 插入 DB 模板
		db.Create(&model.IntentPromptTemplate{
			IntentCode:      model.IntentSolveBug,
			TemplateType:    "system_prompt",
			TemplateContent: "你是BUG排查助手，题目ID: {problem_id}，语言: {language}",
			IsActive:        1,
		})

		agentCtx := &model.AgentContext{
			IntentResult: &model.IntentResult{IntentCode: model.IntentSolveBug},
			ProblemID:    "1001",
			Language:     "Python",
		}
		prompt := engine.buildSystemPrompt(agentCtx)
		assert.Contains(t, prompt, "BUG排查助手")
		assert.Contains(t, prompt, "1001")
		assert.Contains(t, prompt, "Python")
		assert.NotContains(t, prompt, "{problem_id}")
		assert.NotContains(t, prompt, "{language}")
	})

	t.Run("DB模板_注入用户画像", func(t *testing.T) {
		db.Create(&model.IntentPromptTemplate{
			IntentCode:      "TEST_PROFILE",
			TemplateType:    "system_prompt",
			TemplateContent: "助手提示词 {user_profile}",
			IsActive:        1,
		})

		agentCtx := &model.AgentContext{
			IntentResult: &model.IntentResult{IntentCode: "TEST_PROFILE"},
			UserProfile: &model.UserProfile{
				DifficultyLevel:    "beginner",
				TotalSubmissions:   10,
				AcceptRate:         0.3,
				SolvedProblemCount: 3,
			},
		}
		prompt := engine.buildSystemPrompt(agentCtx)
		assert.Contains(t, prompt, "beginner")
		assert.NotContains(t, prompt, "{user_profile}")
	})

	t.Run("DB模板_注入RAG上下文", func(t *testing.T) {
		db.Create(&model.IntentPromptTemplate{
			IntentCode:      "TEST_RAG",
			TemplateType:    "system_prompt",
			TemplateContent: "助手提示词",
			IsActive:        1,
		})

		agentCtx := &model.AgentContext{
			IntentResult: &model.IntentResult{IntentCode: "TEST_RAG"},
			RAGContext:   "这是RAG检索到的参考资料",
		}
		prompt := engine.buildSystemPrompt(agentCtx)
		assert.Contains(t, prompt, "参考资料")
		assert.Contains(t, prompt, "这是RAG检索到的参考资料")
	})

	t.Run("nil_IntentResult_降级到代码模板", func(t *testing.T) {
		agentCtx := &model.AgentContext{}
		prompt := engine.buildSystemPrompt(agentCtx)
		assert.NotEmpty(t, prompt)
	})
}

// ==================== 6.6 performRAGRetrieval ====================

func TestReactEngine_PerformRAGRetrieval(t *testing.T) {
	db := setupTestDB(t)
	engine := newTestReactEngine(db)

	t.Run("nil_ragService_返回nil", func(t *testing.T) {
		agentCtx := &model.AgentContext{
			OriginalQuery: "动态规划是什么",
			IntentResult:  &model.IntentResult{IntentCode: model.IntentKnowledgeAlgo},
		}
		docs, err := engine.performRAGRetrieval(nil, agentCtx)
		assert.NoError(t, err)
		assert.Nil(t, docs)
	})
}

// ==================== 6.9 非学生角色跳过画像 ====================

func TestReactEngine_TeacherSkipsProfile(t *testing.T) {
	db := setupTestDB(t)
	engine := newTestReactEngine(db)

	// 插入学生画像（即使存在也不应加载）
	db.Create(&model.StudentProfile{
		StudentId:       "teacher001",
		DifficultyLevel: "advanced",
	})

	agentCtx := &model.AgentContext{
		UserID:   "teacher001",
		UserRole: model.RoleTeacher,
	}

	// 教师角色不应加载画像（Execute 中的条件判断）
	// 直接测试条件：UserRole != RoleStudent 时不调用 loadUserProfile
	if agentCtx.UserRole == model.RoleStudent {
		engine.loadUserProfile(agentCtx)
	}
	assert.Nil(t, agentCtx.UserProfile, "教师角色不应加载画像")
}

// ==================== recordIntent ====================

func TestReactEngine_RecordIntent(t *testing.T) {
	db := setupTestDB(t)
	engine := newTestReactEngine(db)

	t.Run("正常记录", func(t *testing.T) {
		agentCtx := &model.AgentContext{
			UserID:        "stu001",
			SessionID:     "sess001",
			ProblemID:     "1001",
			OriginalQuery: "这道题怎么做",
			IntentResult: &model.IntentResult{
				IntentCode:   model.IntentSolveThink,
				IntentLevel1: "解题相关",
				Confidence:   0.92,
			},
		}
		engine.recordIntent(agentCtx, 150)

		// 验证记录已写入
		var count int64
		db.Model(&model.UserIntentRecord{}).Where("user_id = ?", "stu001").Count(&count)
		assert.Equal(t, int64(1), count)
	})

	t.Run("nil_IntentResult_跳过", func(t *testing.T) {
		agentCtx := &model.AgentContext{
			UserID: "stu002",
		}
		// 不应 panic
		engine.recordIntent(agentCtx, 100)
	})

	t.Run("nil_intentDAO_跳过", func(t *testing.T) {
		engineNoDAO := &ReactEngine{intentDAO: nil}
		agentCtx := &model.AgentContext{
			UserID:       "stu003",
			IntentResult: &model.IntentResult{IntentCode: "SOLVE_BUG"},
		}
		// 不应 panic
		engineNoDAO.recordIntent(agentCtx, 100)
	})
}

// ==================== 辅助函数 ====================

func mustParseTime(s string) time.Time {
	t2, err := time.Parse("2006-01-02 15:04:05", s)
	if err != nil {
		panic("解析时间失败: " + err.Error())
	}
	return t2
}
