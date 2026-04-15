package dao

import (
	"fmt"

	"github.com/yzf120/elysia-chat-agent/model"
	"gorm.io/gorm"
)

// QABehaviorDAO 问答行为数据访问层
type QABehaviorDAO struct {
	db *gorm.DB
}

// NewQABehaviorDAO 创建问答行为 DAO
func NewQABehaviorDAO(db *gorm.DB) *QABehaviorDAO {
	return &QABehaviorDAO{db: db}
}

// CreateQABehavior 创建问答行为记录
func (d *QABehaviorDAO) CreateQABehavior(record *model.QABehavior) error {
	if err := d.db.Create(record).Error; err != nil {
		return fmt.Errorf("创建问答行为记录失败: %w", err)
	}
	return nil
}

// GetRecentBehaviors 获取某学生最近 N 条问答行为记录（不区分会话，按时间倒序）
func (d *QABehaviorDAO) GetRecentBehaviors(studentId string, limit int) ([]*model.QABehavior, error) {
	var records []*model.QABehavior
	err := d.db.Where("student_id = ?", studentId).
		Order("conversation_time DESC").
		Limit(limit).
		Find(&records).Error
	if err != nil {
		return nil, fmt.Errorf("查询最近问答行为失败: %w", err)
	}
	return records, nil
}

// GetBehaviorsByConversation 根据会话ID查询问答行为
func (d *QABehaviorDAO) GetBehaviorsByConversation(conversationId string) ([]*model.QABehavior, error) {
	var records []*model.QABehavior
	err := d.db.Where("conversation_id = ?", conversationId).
		Order("create_time ASC").
		Find(&records).Error
	if err != nil {
		return nil, fmt.Errorf("查询会话问答行为失败: %w", err)
	}
	return records, nil
}
