package dao

import (
	"fmt"

	"github.com/yzf120/elysia-chat-agent/model"
	"gorm.io/gorm"
)

// IntentDAO 意图相关的数据访问层
type IntentDAO struct {
	db *gorm.DB
}

// NewIntentDAO 创建意图 DAO
func NewIntentDAO(db *gorm.DB) *IntentDAO {
	return &IntentDAO{db: db}
}

// ==================== 意图字典 ====================

// ListValidIntentDicts 查询所有有效的意图字典
func (d *IntentDAO) ListValidIntentDicts() ([]*model.IntentDict, error) {
	var dicts []*model.IntentDict
	err := d.db.Where("is_valid = 1").Order("priority DESC").Find(&dicts).Error
	if err != nil {
		return nil, fmt.Errorf("查询有效意图字典失败: %w", err)
	}
	return dicts, nil
}

// GetIntentDictByCode 根据编码查询意图字典
func (d *IntentDAO) GetIntentDictByCode(code string) (*model.IntentDict, error) {
	var dict model.IntentDict
	err := d.db.Where("intent_code = ? AND is_valid = 1", code).First(&dict).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("查询意图字典失败: %w", err)
	}
	return &dict, nil
}

// ==================== 提示词模板 ====================

// GetActivePromptTemplate 获取某意图某类型的启用模板
func (d *IntentDAO) GetActivePromptTemplate(intentCode, templateType string) (*model.IntentPromptTemplate, error) {
	var tpl model.IntentPromptTemplate
	err := d.db.Where("intent_code = ? AND template_type = ? AND is_active = 1",
		intentCode, templateType).First(&tpl).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("查询提示词模板失败: %w", err)
	}
	return &tpl, nil
}

// ==================== 用户意图记录 ====================

// CreateIntentRecord 创建用户意图记录
func (d *IntentDAO) CreateIntentRecord(record *model.UserIntentRecord) error {
	if err := d.db.Create(record).Error; err != nil {
		return fmt.Errorf("创建意图记录失败: %w", err)
	}
	return nil
}
