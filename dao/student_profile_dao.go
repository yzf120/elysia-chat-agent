package dao

import (
	"fmt"

	"github.com/yzf120/elysia-chat-agent/model"
	"gorm.io/gorm"
)

// StudentProfileDAO 学生画像数据访问层（只读，用于 AI 对话时查询画像）
type StudentProfileDAO struct {
	db *gorm.DB
}

// NewStudentProfileDAO 创建学生画像 DAO
func NewStudentProfileDAO(db *gorm.DB) *StudentProfileDAO {
	return &StudentProfileDAO{db: db}
}

// GetProfileByStudentId 根据学生ID查询画像
func (d *StudentProfileDAO) GetProfileByStudentId(studentId string) (*model.StudentProfile, error) {
	var p model.StudentProfile
	err := d.db.Where("student_id = ?", studentId).First(&p).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("查询学生画像失败: %w", err)
	}
	return &p, nil
}