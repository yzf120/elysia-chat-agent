package model

import "time"

// ==================== 学生做题画像数据模型 ====================

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
