package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// ==================== 1.11 KnowledgeTagMap 完整性 ====================

func TestKnowledgeTagMapCompleteness(t *testing.T) {
	// 验证 Map 和 Library 数量一致
	assert.Equal(t, len(KnowledgeTagLibrary), len(KnowledgeTagMap),
		"KnowledgeTagMap 与 KnowledgeTagLibrary 数量不一致")

	// 验证每个标签都能在 Map 中找到，且字段一致
	for _, tag := range KnowledgeTagLibrary {
		found, ok := KnowledgeTagMap[tag.Name]
		assert.True(t, ok, "标签 %q 未在 KnowledgeTagMap 中找到", tag.Name)
		if ok {
			assert.Equal(t, tag.Category, found.Category, "标签 %q 的 Category 不一致", tag.Name)
			assert.Equal(t, tag.Difficulty, found.Difficulty, "标签 %q 的 Difficulty 不一致", tag.Name)
		}
	}
}

func TestKnowledgeTagMapNoDuplicateNames(t *testing.T) {
	// 验证标签库中没有重复名称
	seen := make(map[string]bool)
	for _, tag := range KnowledgeTagLibrary {
		assert.False(t, seen[tag.Name], "标签名称 %q 重复出现", tag.Name)
		seen[tag.Name] = true
	}
}

func TestKnowledgeTagLibraryFieldsValid(t *testing.T) {
	for _, tag := range KnowledgeTagLibrary {
		assert.NotEmpty(t, tag.Name, "标签名称不能为空")
		assert.NotEmpty(t, tag.Category, "标签 %q 的分类不能为空", tag.Name)
		assert.GreaterOrEqual(t, tag.Difficulty, 1, "标签 %q 的难度值应 >= 1", tag.Name)
		assert.LessOrEqual(t, tag.Difficulty, 5, "标签 %q 的难度值应 <= 5", tag.Name)
	}
}

// ==================== 1.12 CalcDifficultyByTags ====================

func TestCalcDifficultyByTags(t *testing.T) {
	tests := []struct {
		name     string
		tags     []string
		expected float64
	}{
		{
			name:     "空标签列表",
			tags:     nil,
			expected: 0,
		},
		{
			name:     "空切片",
			tags:     []string{},
			expected: 0,
		},
		{
			name:     "全部无效标签",
			tags:     []string{"不存在的标签A", "不存在的标签B"},
			expected: 0,
		},
		{
			name:     "单个有效标签_哈希表",
			tags:     []string{"哈希表"},
			expected: 2.0, // 哈希表 difficulty=2
		},
		{
			name:     "两个有效标签_取平均",
			tags:     []string{"哈希表", "线段树"},
			expected: 3.0, // (2+4)/2 = 3.0
		},
		{
			name:     "有效标签和无效标签混合",
			tags:     []string{"哈希表", "不存在的标签", "线段树"},
			expected: 3.0, // 只计算有效标签: (2+4)/2 = 3.0
		},
		{
			name:     "多个相同难度标签",
			tags:     []string{"变量与数据类型", "条件判断", "循环结构"},
			expected: 1.0, // 全部 difficulty=1
		},
		{
			name:     "高难度标签",
			tags:     []string{"网络流", "数位DP"},
			expected: 5.0, // (5+5)/2 = 5.0
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CalcDifficultyByTags(tt.tags)
			assert.InDelta(t, tt.expected, result, 0.001, "难度计算结果不符预期")
		})
	}
}

// ==================== 1.13 GetKnowledgeTagNamesByCategory ====================

func TestGetKnowledgeTagNamesByCategory(t *testing.T) {
	result := GetKnowledgeTagNamesByCategory()

	expectedCategories := []string{
		"基础编程", "数据结构", "基础算法", "搜索与图论", "动态规划",
		"数学与数论", "高级数据结构", "高级算法", "字符串算法",
		"计算机基础", "操作系统与网络", "数据库与工程",
	}

	// 验证所有 12 个分类都存在
	assert.Equal(t, len(expectedCategories), len(result), "分类数量不一致")
	for _, cat := range expectedCategories {
		tags, ok := result[cat]
		assert.True(t, ok, "分类 %q 不存在", cat)
		assert.NotEmpty(t, tags, "分类 %q 下应有标签", cat)
	}

	// 验证总标签数等于 Library 数量
	totalTags := 0
	for _, tags := range result {
		totalTags += len(tags)
	}
	assert.Equal(t, len(KnowledgeTagLibrary), totalTags, "按分类汇总的标签总数应等于标签库总数")
}

func TestGetKnowledgeTagNamesByCategorySpecificCounts(t *testing.T) {
	result := GetKnowledgeTagNamesByCategory()

	// 验证部分分类的标签数量
	assert.Len(t, result["基础编程"], 10, "基础编程应有 10 个标签")
	assert.Len(t, result["数据结构"], 10, "数据结构应有 10 个标签")
	assert.Len(t, result["基础算法"], 10, "基础算法应有 10 个标签")
	assert.Len(t, result["动态规划"], 8, "动态规划应有 8 个标签")
}

// ==================== GetKnowledgeTagNames ====================

func TestGetKnowledgeTagNames(t *testing.T) {
	names := GetKnowledgeTagNames()

	assert.Equal(t, len(KnowledgeTagLibrary), len(names), "标签名称列表长度应等于标签库长度")

	// 验证顺序与 Library 一致
	for i, tag := range KnowledgeTagLibrary {
		assert.Equal(t, tag.Name, names[i], "第 %d 个标签名称不一致", i)
	}
}
