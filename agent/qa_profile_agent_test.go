package agent

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/yzf120/elysia-chat-agent/model"
)

// ==================== 1.5 extractJSON ====================

func TestExtractJSON(t *testing.T) {
	tests := []struct {
		name string
		text string
		want string
	}{
		{
			name: "裸JSON",
			text: `{"key":"value"}`,
			want: `{"key":"value"}`,
		},
		{
			name: "json代码块包裹",
			text: "```json\n{\"key\":\"value\"}\n```",
			want: `{"key":"value"}`,
		},
		{
			name: "普通代码块包裹",
			text: "```\n{\"key\":\"value\"}\n```",
			want: `{"key":"value"}`,
		},
		{
			name: "前后有文本_提取花括号",
			text: "结果是{\"key\":\"value\"}以上",
			want: `{"key":"value"}`,
		},
		{
			name: "无JSON_返回原文",
			text: "纯文本内容没有花括号",
			want: "纯文本内容没有花括号",
		},
		{
			name: "空字符串",
			text: "",
			want: "",
		},
		{
			name: "嵌套JSON",
			text: `{"question_summary":"两数之和","knowledge_tags":["哈希表","双指针"],"is_resolved":1}`,
			want: `{"question_summary":"两数之和","knowledge_tags":["哈希表","双指针"],"is_resolved":1}`,
		},
		{
			name: "json代码块_带换行",
			text: "分析结果：\n```json\n{\"question_summary\":\"测试\"}\n```\n以上是结果",
			want: `{"question_summary":"测试"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractJSON(tt.text)
			assert.Equal(t, tt.want, got)
		})
	}
}

// ==================== 1.6 parseProblemId ====================

func TestParseProblemId(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  int64
	}{
		{"正常数字", "123", 123},
		{"大数字", "999999", 999999},
		{"零", "0", 0},
		{"空字符串", "", 0},
		{"非数字", "abc", 0},
		{"混合内容", "123abc", 123}, // Sscanf 会解析前面的数字部分
		{"负数", "-1", -1},
		{"带空格", " 123", 123}, // Sscanf 会跳过前导空格
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseProblemId(tt.input)
			assert.Equal(t, tt.want, got)
		})
	}
}

// ==================== 1.7 getIntentCode ====================

func TestGetIntentCode(t *testing.T) {
	tests := []struct {
		name     string
		ctx      *model.AgentContext
		wantCode string
	}{
		{
			name:     "无IntentResult",
			ctx:      &model.AgentContext{},
			wantCode: "",
		},
		{
			name: "有IntentResult",
			ctx: &model.AgentContext{
				IntentResult: &model.IntentResult{IntentCode: "SOLVE_BUG"},
			},
			wantCode: "SOLVE_BUG",
		},
		{
			name: "IntentResult_空编码",
			ctx: &model.AgentContext{
				IntentResult: &model.IntentResult{IntentCode: ""},
			},
			wantCode: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getIntentCode(tt.ctx)
			assert.Equal(t, tt.wantCode, got)
		})
	}
}

// ==================== 2.5 buildAnalysisInput ====================

func TestBuildAnalysisInput(t *testing.T) {
	agent := &QAProfileAgent{modelID: "test-model"}

	t.Run("完整上下文", func(t *testing.T) {
		ctx := &model.AgentContext{
			ProblemInfo:   "给定一个整数数组和目标值",
			StudentCode:   "def twoSum(nums, target): pass",
			Language:      "python",
			OriginalQuery: "这道题怎么做",
			IntentResult: &model.IntentResult{
				IntentLevel1: "解题相关",
				IntentLevel2: "解题思路",
			},
		}
		aiResponse := "你可以使用哈希表来解决这个问题"

		result := agent.buildAnalysisInput(ctx, aiResponse)
		assert.Contains(t, result, "[题目信息]")
		assert.Contains(t, result, "给定一个整数数组和目标值")
		assert.Contains(t, result, "[学生代码]")
		assert.Contains(t, result, "def twoSum")
		assert.Contains(t, result, "[学生问题]")
		assert.Contains(t, result, "这道题怎么做")
		assert.Contains(t, result, "[AI回复]")
		assert.Contains(t, result, "哈希表")
		assert.Contains(t, result, "[意图分类]")
	})

	t.Run("无题目信息", func(t *testing.T) {
		ctx := &model.AgentContext{
			OriginalQuery: "动态规划是什么",
		}
		aiResponse := "动态规划是一种算法设计方法"

		result := agent.buildAnalysisInput(ctx, aiResponse)
		assert.NotContains(t, result, "[题目信息]")
		assert.NotContains(t, result, "[学生代码]")
		assert.Contains(t, result, "[学生问题]")
		assert.Contains(t, result, "[AI回复]")
	})

	t.Run("AI回复超长截断", func(t *testing.T) {
		ctx := &model.AgentContext{
			OriginalQuery: "测试",
		}
		// 构造超过 500 字符的回复
		longResponse := ""
		for i := 0; i < 100; i++ {
			longResponse += "这是一段很长的回复内容"
		}
		assert.Greater(t, len(longResponse), 500)

		result := agent.buildAnalysisInput(ctx, longResponse)
		assert.Contains(t, result, "...")
	})

	t.Run("空AI回复", func(t *testing.T) {
		ctx := &model.AgentContext{
			OriginalQuery: "测试",
		}
		result := agent.buildAnalysisInput(ctx, "")
		assert.Contains(t, result, "[AI回复]")
	})
}

// ==================== 2.4 buildAnalysisPrompt ====================

func TestBuildAnalysisPrompt(t *testing.T) {
	agent := &QAProfileAgent{modelID: "test-model"}
	prompt := agent.buildAnalysisPrompt()

	// 验证 prompt 包含关键内容
	assert.Contains(t, prompt, "问答行为分析引擎")
	assert.Contains(t, prompt, "知识点标签库")
	assert.Contains(t, prompt, "question_summary")
	assert.Contains(t, prompt, "knowledge_tags")
	assert.Contains(t, prompt, "is_resolved")

	// 验证所有 12 个分类都在 prompt 中
	categories := []string{
		"基础编程", "数据结构", "基础算法", "搜索与图论", "动态规划",
		"数学与数论", "高级数据结构", "高级算法", "字符串算法",
		"计算机基础", "操作系统与网络", "数据库与工程",
	}
	for _, cat := range categories {
		assert.Contains(t, prompt, cat, "prompt 中应包含分类 %q", cat)
	}
}
