package dao

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/yzf120/elysia-chat-agent/model"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// setupTestDB 创建 SQLite 内存数据库用于测试
func setupTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err, "创建测试数据库失败")

	// 自动迁移所有需要的表
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

// ==================== IntentDAO 测试 ====================

func TestIntentDAO_ListValidIntentDicts(t *testing.T) {
	db := setupTestDB(t)
	dao := NewIntentDAO(db)

	t.Run("空表_返回空列表", func(t *testing.T) {
		dicts, err := dao.ListValidIntentDicts()
		assert.NoError(t, err)
		assert.Empty(t, dicts)
	})

	t.Run("有数据_只返回有效记录", func(t *testing.T) {
		// 插入有效和无效记录
		db.Create(&model.IntentDict{IntentCode: "SOLVE_BUG", IntentLevel1: "解题相关", IsValid: 1, Priority: 10})
		db.Create(&model.IntentDict{IntentCode: "DISABLED", IntentLevel1: "已禁用", IsValid: 0, Priority: 5})
		db.Create(&model.IntentDict{IntentCode: "SOLVE_THINK", IntentLevel1: "解题相关", IsValid: 1, Priority: 20})

		dicts, err := dao.ListValidIntentDicts()
		assert.NoError(t, err)
		assert.Len(t, dicts, 2, "应只返回有效记录")

		// 验证按 priority DESC 排序
		assert.Equal(t, "SOLVE_THINK", dicts[0].IntentCode, "高优先级应排在前面")
		assert.Equal(t, "SOLVE_BUG", dicts[1].IntentCode)
	})
}

func TestIntentDAO_GetIntentDictByCode(t *testing.T) {
	db := setupTestDB(t)
	dao := NewIntentDAO(db)

	// 插入测试数据
	db.Create(&model.IntentDict{IntentCode: "SOLVE_BUG", IntentLevel1: "解题相关", IsValid: 1})
	db.Create(&model.IntentDict{IntentCode: "DISABLED", IntentLevel1: "已禁用", IsValid: 0})

	t.Run("存在且有效", func(t *testing.T) {
		dict, err := dao.GetIntentDictByCode("SOLVE_BUG")
		assert.NoError(t, err)
		assert.NotNil(t, dict)
		assert.Equal(t, "SOLVE_BUG", dict.IntentCode)
	})

	t.Run("存在但无效_返回nil", func(t *testing.T) {
		dict, err := dao.GetIntentDictByCode("DISABLED")
		assert.NoError(t, err)
		assert.Nil(t, dict, "无效记录应返回 nil")
	})

	t.Run("不存在_返回nil", func(t *testing.T) {
		dict, err := dao.GetIntentDictByCode("NOT_EXIST")
		assert.NoError(t, err)
		assert.Nil(t, dict, "不存在的记录应返回 nil")
	})
}

func TestIntentDAO_GetActivePromptTemplate(t *testing.T) {
	db := setupTestDB(t)
	dao := NewIntentDAO(db)

	t.Run("不存在_返回nil", func(t *testing.T) {
		tpl, err := dao.GetActivePromptTemplate("SOLVE_BUG", "system_prompt")
		assert.NoError(t, err)
		assert.Nil(t, tpl)
	})

	t.Run("存在且启用", func(t *testing.T) {
		db.Create(&model.IntentPromptTemplate{
			IntentCode:      "SOLVE_BUG",
			TemplateType:    "system_prompt",
			TemplateContent: "你是BUG排查助手",
			IsActive:        1,
		})

		tpl, err := dao.GetActivePromptTemplate("SOLVE_BUG", "system_prompt")
		assert.NoError(t, err)
		assert.NotNil(t, tpl)
		assert.Contains(t, tpl.TemplateContent, "BUG排查")
	})

	t.Run("存在但未启用_返回nil", func(t *testing.T) {
		db.Create(&model.IntentPromptTemplate{
			IntentCode:      "SOLVE_THINK",
			TemplateType:    "system_prompt",
			TemplateContent: "你是解题助手",
			IsActive:        0,
		})

		tpl, err := dao.GetActivePromptTemplate("SOLVE_THINK", "system_prompt")
		assert.NoError(t, err)
		assert.Nil(t, tpl, "未启用的模板应返回 nil")
	})
}

func TestIntentDAO_CreateIntentRecord(t *testing.T) {
	db := setupTestDB(t)
	dao := NewIntentDAO(db)

	t.Run("正常创建", func(t *testing.T) {
		record := &model.UserIntentRecord{
			UserID:           "stu001",
			SessionID:        "sess001",
			IntentCode:       "SOLVE_BUG",
			IntentLevel1:     "解题相关",
			OriginalRequest:  "代码有bug",
			IntentConfidence: 92.0,
			ResponseTimeMs:   150,
			RecognizeStatus:  1,
		}
		err := dao.CreateIntentRecord(record)
		assert.NoError(t, err)
		assert.NotZero(t, record.Id, "创建后应有自增 ID")
	})

	t.Run("多次创建", func(t *testing.T) {
		for i := 0; i < 3; i++ {
			record := &model.UserIntentRecord{
				UserID:     "stu002",
				IntentCode: "SOLVE_THINK",
			}
			err := dao.CreateIntentRecord(record)
			assert.NoError(t, err)
		}

		// 验证记录数
		var count int64
		db.Model(&model.UserIntentRecord{}).Where("user_id = ?", "stu002").Count(&count)
		assert.Equal(t, int64(3), count)
	})
}

// ==================== QABehaviorDAO 测试 ====================

func TestQABehaviorDAO_CreateQABehavior(t *testing.T) {
	db := setupTestDB(t)
	dao := NewQABehaviorDAO(db)

	record := &model.QABehavior{
		StudentId:         "stu001",
		ConversationId:    "conv001",
		ProblemId:         123,
		IntentCode:        "SOLVE_BUG",
		QuestionSummary:   "两数之和解题思路",
		KnowledgeTags:     `["哈希表","双指针"]`,
		DifficultyScore:   2.0,
		IsResolved:        1,
		ConversationTurns: 3,
		ConversationTime:  time.Now(),
	}

	err := dao.CreateQABehavior(record)
	assert.NoError(t, err)
	assert.NotZero(t, record.Id)
}

func TestQABehaviorDAO_GetRecentBehaviors(t *testing.T) {
	db := setupTestDB(t)
	dao := NewQABehaviorDAO(db)

	t.Run("无记录_返回空", func(t *testing.T) {
		records, err := dao.GetRecentBehaviors("stu_empty", 10)
		assert.NoError(t, err)
		assert.Empty(t, records)
	})

	t.Run("有记录_按时间倒序_限制数量", func(t *testing.T) {
		// 插入 5 条记录
		baseTime := time.Now()
		for i := 0; i < 5; i++ {
			db.Create(&model.QABehavior{
				StudentId:        "stu_recent",
				ConversationId:   "conv" + string(rune('A'+i)),
				QuestionSummary:  "问题" + string(rune('A'+i)),
				ConversationTime: baseTime.Add(time.Duration(i) * time.Hour),
			})
		}

		// 取最近 3 条
		records, err := dao.GetRecentBehaviors("stu_recent", 3)
		assert.NoError(t, err)
		assert.Len(t, records, 3, "应返回 3 条记录")

		// 验证按时间倒序（最新的在前）
		for i := 1; i < len(records); i++ {
			assert.True(t, records[i-1].ConversationTime.After(records[i].ConversationTime) ||
				records[i-1].ConversationTime.Equal(records[i].ConversationTime),
				"记录应按时间倒序排列")
		}
	})
}

func TestQABehaviorDAO_GetBehaviorsByConversation(t *testing.T) {
	db := setupTestDB(t)
	dao := NewQABehaviorDAO(db)

	t.Run("无记录_返回空", func(t *testing.T) {
		records, err := dao.GetBehaviorsByConversation("conv_not_exist")
		assert.NoError(t, err)
		assert.Empty(t, records)
	})

	t.Run("有记录_按创建时间正序", func(t *testing.T) {
		db.Create(&model.QABehavior{
			StudentId:        "stu_conv",
			ConversationId:   "conv_test",
			QuestionSummary:  "第一个问题",
			ConversationTime: time.Now(),
		})
		db.Create(&model.QABehavior{
			StudentId:        "stu_conv",
			ConversationId:   "conv_test",
			QuestionSummary:  "第二个问题",
			ConversationTime: time.Now(),
		})

		records, err := dao.GetBehaviorsByConversation("conv_test")
		assert.NoError(t, err)
		assert.Len(t, records, 2)
	})
}

// ==================== StudentProfileDAO 测试 ====================

func TestStudentProfileDAO_GetProfileByStudentId(t *testing.T) {
	db := setupTestDB(t)
	dao := NewStudentProfileDAO(db)

	t.Run("不存在_返回nil", func(t *testing.T) {
		profile, err := dao.GetProfileByStudentId("not_exist")
		assert.NoError(t, err)
		assert.Nil(t, profile)
	})

	t.Run("存在_返回画像", func(t *testing.T) {
		db.Create(&model.StudentProfile{
			StudentId:          "stu001",
			DifficultyLevel:    "intermediate",
			TotalSubmissions:   100,
			AcceptRate:         0.65,
			SolvedProblemCount: 50,
			PreferredLanguage:  "Python",
			CommonErrors:       `["数组越界","空指针"]`,
			LanguageStats:      `{"Python":50,"C++":30}`,
		})

		profile, err := dao.GetProfileByStudentId("stu001")
		assert.NoError(t, err)
		assert.NotNil(t, profile)
		assert.Equal(t, "stu001", profile.StudentId)
		assert.Equal(t, "intermediate", profile.DifficultyLevel)
		assert.Equal(t, 100, profile.TotalSubmissions)
		assert.InDelta(t, 0.65, profile.AcceptRate, 0.001)
		assert.Equal(t, "Python", profile.PreferredLanguage)
	})
}
