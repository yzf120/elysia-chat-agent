package model

// ==================== 测试用例相关 ====================

// TestCase 测试用例
type TestCase struct {
	Input          string `json:"input"`
	ExpectedOutput string `json:"expected_output"`
	IsSample       int    `json:"is_sample"` // 0-隐藏用例 1-示例用例
	Explanation    string `json:"explanation"`
	Category       string `json:"category"` // basic / boundary / special / stress
}

// TestCaseGenResult 测试用例生成结果
type TestCaseGenResult struct {
	TestCases      []TestCase     `json:"test_cases"`
	Showcase       []TestCase     `json:"showcase"`
	CoverageReport map[string]int `json:"coverage_report"`
}
