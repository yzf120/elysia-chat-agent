package dao

import (
	"fmt"
	"time"

	"gorm.io/gorm"
)

// StudentProfile 学生做题画像（对应 student_profile 表，只读）
type StudentProfile struct {
	Id                    int64      `gorm:"column:id;primaryKey" json:"id"`
	StudentId             string     `gorm:"column:student_id" json:"student_id"`
	TotalSubmissions      int        `gorm:"column:total_submissions" json:"total_submissions"`
	AcceptedCount         int        `gorm:"column:accepted_count" json:"accepted_count"`
	WrongAnswerCount      int        `gorm:"column:wrong_answer_count" json:"wrong_answer_count"`
	CompileErrorCount     int        `gorm:"column:compile_error_count" json:"compile_error_count"`
	RuntimeErrorCount     int        `gorm:"column:runtime_error_count" json:"runtime_error_count"`
	TLECount              int        `gorm:"column:tle_count" json:"tle_count"`
	MLECount              int        `gorm:"column:mle_count" json:"mle_count"`
	AcceptRate            float64    `gorm:"column:accept_rate" json:"accept_rate"`
	SolvedProblemCount    int        `gorm:"column:solved_problem_count" json:"solved_problem_count"`
	AttemptedProblemCount int        `gorm:"column:attempted_problem_count" json:"attempted_problem_count"`
	PreferredLanguage     string     `gorm:"column:preferred_language" json:"preferred_language"`
	LanguageStats         string     `gorm:"column:language_stats" json:"language_stats"`
	AvgTimeCost           int64      `gorm:"column:avg_time_cost" json:"avg_time_cost"`
	CommonErrors          string     `gorm:"column:common_errors" json:"common_errors"`
	DifficultyLevel       string     `gorm:"column:difficulty_level" json:"difficulty_level"`
	LastSubmitTime        *time.Time `gorm:"column:last_submit_time" json:"last_submit_time"`
}

func (StudentProfile) TableName() string {
	return "student_profile"
}

// StudentProfileDAO 学生画像数据访问层（只读，用于 AI 对话时查询画像）
type StudentProfileDAO struct {
	db *gorm.DB
}

// NewStudentProfileDAO 创建学生画像 DAO
func NewStudentProfileDAO(db *gorm.DB) *StudentProfileDAO {
	return &StudentProfileDAO{db: db}
}

// GetProfileByStudentId 根据学生ID查询画像
func (d *StudentProfileDAO) GetProfileByStudentId(studentId string) (*StudentProfile, error) {
	var p StudentProfile
	err := d.db.Where("student_id = ?", studentId).First(&p).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("查询学生画像失败: %w", err)
	}
	return &p, nil
}
