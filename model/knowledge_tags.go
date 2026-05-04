package model

import "strings"

// KnowledgeTag 知识点标签定义
type KnowledgeTag struct {
	Name       string `json:"name"`       // 标签名称
	Category   string `json:"category"`   // 所属分类
	Difficulty int    `json:"difficulty"` // 难度值 1-5（1=入门，2=基础，3=中等，4=进阶，5=困难）
}

// KnowledgeTagLibrary 知识点标签库（共 90 个标签）
// 分类：基础编程、数据结构、基础算法、搜索与图论、动态规划、数学与数论、
//
//	高级数据结构、高级算法、字符串算法、计算机基础、操作系统与网络、数据库与工程
var KnowledgeTagLibrary = []KnowledgeTag{
	// ==================== 基础编程（难度 1-2）====================
	{Name: "变量与数据类型", Category: "基础编程", Difficulty: 1},
	{Name: "条件判断", Category: "基础编程", Difficulty: 1},
	{Name: "循环结构", Category: "基础编程", Difficulty: 1},
	{Name: "函数与递归基础", Category: "基础编程", Difficulty: 1},
	{Name: "输入输出处理", Category: "基础编程", Difficulty: 1},
	{Name: "数组基础", Category: "基础编程", Difficulty: 1},
	{Name: "字符串基础", Category: "基础编程", Difficulty: 1},
	{Name: "指针与引用", Category: "基础编程", Difficulty: 2},
	{Name: "结构体与类", Category: "基础编程", Difficulty: 2},
	{Name: "文件操作", Category: "基础编程", Difficulty: 2},

	// ==================== 数据结构（难度 2-4）====================
	{Name: "链表", Category: "数据结构", Difficulty: 2},
	{Name: "栈", Category: "数据结构", Difficulty: 2},
	{Name: "队列", Category: "数据结构", Difficulty: 2},
	{Name: "哈希表", Category: "数据结构", Difficulty: 2},
	{Name: "二叉树", Category: "数据结构", Difficulty: 3},
	{Name: "二叉搜索树", Category: "数据结构", Difficulty: 3},
	{Name: "堆/优先队列", Category: "数据结构", Difficulty: 3},
	{Name: "图的表示", Category: "数据结构", Difficulty: 3},
	{Name: "并查集", Category: "数据结构", Difficulty: 3},
	{Name: "字典树/Trie", Category: "数据结构", Difficulty: 4},

	// ==================== 基础算法（难度 2-3）====================
	{Name: "暴力枚举", Category: "基础算法", Difficulty: 2},
	{Name: "模拟", Category: "基础算法", Difficulty: 2},
	{Name: "排序算法", Category: "基础算法", Difficulty: 2},
	{Name: "二分查找", Category: "基础算法", Difficulty: 2},
	{Name: "双指针", Category: "基础算法", Difficulty: 2},
	{Name: "滑动窗口", Category: "基础算法", Difficulty: 3},
	{Name: "前缀和与差分", Category: "基础算法", Difficulty: 3},
	{Name: "贪心算法", Category: "基础算法", Difficulty: 3},
	{Name: "递归与分治", Category: "基础算法", Difficulty: 3},
	{Name: "位运算", Category: "基础算法", Difficulty: 3},

	// ==================== 搜索与图论（难度 3-4）====================
	{Name: "深度优先搜索(DFS)", Category: "搜索与图论", Difficulty: 3},
	{Name: "广度优先搜索(BFS)", Category: "搜索与图论", Difficulty: 3},
	{Name: "回溯法", Category: "搜索与图论", Difficulty: 3},
	{Name: "拓扑排序", Category: "搜索与图论", Difficulty: 3},
	{Name: "最短路径(Dijkstra)", Category: "搜索与图论", Difficulty: 4},
	{Name: "最短路径(Floyd)", Category: "搜索与图论", Difficulty: 4},
	{Name: "最短路径(Bellman-Ford)", Category: "搜索与图论", Difficulty: 4},
	{Name: "最小生成树", Category: "搜索与图论", Difficulty: 4},
	{Name: "二分图匹配", Category: "搜索与图论", Difficulty: 4},
	{Name: "网络流", Category: "搜索与图论", Difficulty: 5},

	// ==================== 动态规划（难度 3-5）====================
	{Name: "线性DP", Category: "动态规划", Difficulty: 3},
	{Name: "背包问题", Category: "动态规划", Difficulty: 3},
	{Name: "区间DP", Category: "动态规划", Difficulty: 4},
	{Name: "树形DP", Category: "动态规划", Difficulty: 4},
	{Name: "状态压缩DP", Category: "动态规划", Difficulty: 4},
	{Name: "数位DP", Category: "动态规划", Difficulty: 5},
	{Name: "概率/期望DP", Category: "动态规划", Difficulty: 5},
	{Name: "记忆化搜索", Category: "动态规划", Difficulty: 3},

	// ==================== 数学与数论（难度 2-5）====================
	{Name: "基础数学运算", Category: "数学与数论", Difficulty: 2},
	{Name: "素数与筛法", Category: "数学与数论", Difficulty: 3},
	{Name: "最大公约数/最小公倍数", Category: "数学与数论", Difficulty: 2},
	{Name: "快速幂", Category: "数学与数论", Difficulty: 3},
	{Name: "组合数学", Category: "数学与数论", Difficulty: 4},
	{Name: "矩阵运算", Category: "数学与数论", Difficulty: 4},
	{Name: "博弈论", Category: "数学与数论", Difficulty: 4},
	{Name: "容斥原理", Category: "数学与数论", Difficulty: 5},

	// ==================== 高级数据结构（难度 4-5）====================
	{Name: "线段树", Category: "高级数据结构", Difficulty: 4},
	{Name: "树状数组", Category: "高级数据结构", Difficulty: 4},
	{Name: "平衡二叉树(AVL/红黑树)", Category: "高级数据结构", Difficulty: 5},
	{Name: "跳表", Category: "高级数据结构", Difficulty: 4},
	{Name: "LCA(最近公共祖先)", Category: "高级数据结构", Difficulty: 4},
	{Name: "可持久化数据结构", Category: "高级数据结构", Difficulty: 5},

	// ==================== 高级算法（难度 4-5）====================
	{Name: "单调栈/单调队列", Category: "高级算法", Difficulty: 4},
	{Name: "启发式搜索(A*)", Category: "高级算法", Difficulty: 4},
	{Name: "随机化算法", Category: "高级算法", Difficulty: 4},
	{Name: "CDQ分治", Category: "高级算法", Difficulty: 5},
	{Name: "莫队算法", Category: "高级算法", Difficulty: 5},

	// ==================== 字符串算法（难度 3-5）====================
	{Name: "字符串匹配(KMP)", Category: "字符串算法", Difficulty: 4},
	{Name: "字符串哈希", Category: "字符串算法", Difficulty: 3},
	{Name: "Manacher算法", Category: "字符串算法", Difficulty: 5},
	{Name: "后缀数组", Category: "字符串算法", Difficulty: 5},
	{Name: "AC自动机", Category: "字符串算法", Difficulty: 5},

	// ==================== 计算机基础（难度 1-3）====================
	{Name: "时间复杂度分析", Category: "计算机基础", Difficulty: 2},
	{Name: "空间复杂度分析", Category: "计算机基础", Difficulty: 2},
	{Name: "进制转换", Category: "计算机基础", Difficulty: 1},
	{Name: "编码与字符集", Category: "计算机基础", Difficulty: 2},
	{Name: "内存管理基础", Category: "计算机基础", Difficulty: 3},

	// ==================== 操作系统与网络（难度 2-4）====================
	{Name: "进程与线程", Category: "操作系统与网络", Difficulty: 3},
	{Name: "并发与同步", Category: "操作系统与网络", Difficulty: 4},
	{Name: "死锁", Category: "操作系统与网络", Difficulty: 3},
	{Name: "TCP/IP协议", Category: "操作系统与网络", Difficulty: 3},
	{Name: "HTTP协议", Category: "操作系统与网络", Difficulty: 2},

	// ==================== 数据库与工程（难度 2-4）====================
	{Name: "SQL基础", Category: "数据库与工程", Difficulty: 2},
	{Name: "数据库索引", Category: "数据库与工程", Difficulty: 3},
	{Name: "事务与并发控制", Category: "数据库与工程", Difficulty: 4},
	{Name: "设计模式", Category: "数据库与工程", Difficulty: 3},
	{Name: "版本控制(Git)", Category: "数据库与工程", Difficulty: 2},
}

// KnowledgeTagMap 知识点标签名称到标签的映射（用于快速查找）
var KnowledgeTagMap map[string]*KnowledgeTag

// CategoryAvgDifficulty 分类名到该分类平均难度的映射（用于 LLM 返回分类名时的降级匹配）
var CategoryAvgDifficulty map[string]int

func init() {
	KnowledgeTagMap = make(map[string]*KnowledgeTag, len(KnowledgeTagLibrary))
	for i := range KnowledgeTagLibrary {
		KnowledgeTagMap[KnowledgeTagLibrary[i].Name] = &KnowledgeTagLibrary[i]
	}

	// 计算每个分类的平均难度（用于降级匹配）
	catSum := make(map[string]int)
	catCount := make(map[string]int)
	for _, tag := range KnowledgeTagLibrary {
		catSum[tag.Category] += tag.Difficulty
		catCount[tag.Category]++
	}
	CategoryAvgDifficulty = make(map[string]int, len(catSum))
	for cat, sum := range catSum {
		CategoryAvgDifficulty[cat] = (sum + catCount[cat]/2) / catCount[cat] // 四舍五入
	}
}

// GetKnowledgeTagNames 获取所有知识点标签名称列表（用于提示词中列出可选标签）
func GetKnowledgeTagNames() []string {
	names := make([]string, len(KnowledgeTagLibrary))
	for i, tag := range KnowledgeTagLibrary {
		names[i] = tag.Name
	}
	return names
}

// GetKnowledgeTagNamesByCategory 按分类获取知识点标签名称
func GetKnowledgeTagNamesByCategory() map[string][]string {
	result := make(map[string][]string)
	for _, tag := range KnowledgeTagLibrary {
		result[tag.Category] = append(result[tag.Category], tag.Name)
	}
	return result
}

// CalcDifficultyByTags 根据知识点标签计算加权平均难度
// 返回值范围 1-5，兜底最低返回 1.0（不会返回 0）
// 支持多种 LLM 返回格式的降级匹配：
//  1. 精确匹配标签名（如 "栈"）
//  2. 匹配分类名（如 "数据结构"），使用该分类平均难度
//  3. 匹配 "分类: 标签名" 格式（如 "数据结构: 栈"），拆分后分别尝试匹配
//  4. 以上全部未命中时，兜底返回 1.0
func CalcDifficultyByTags(tagNames []string) float64 {
	if len(tagNames) == 0 {
		return 1.0 // 兜底：无标签时返回最低难度
	}
	totalDifficulty := 0
	matchedCount := 0
	for _, name := range tagNames {
		name = strings.TrimSpace(name)
		if tag, ok := KnowledgeTagMap[name]; ok {
			// 精确匹配到具体标签
			totalDifficulty += tag.Difficulty
			matchedCount++
		} else if avgDiff, ok := CategoryAvgDifficulty[name]; ok {
			// 降级匹配：LLM 返回了分类名，使用该分类的平均难度
			totalDifficulty += avgDiff
			matchedCount++
		} else if strings.Contains(name, ":") || strings.Contains(name, "：") {
			// 降级匹配：LLM 返回了 "分类: 标签名" 格式（如 "数据结构: 栈"）
			// 同时支持英文冒号和中文冒号
			sep := ":"
			if strings.Contains(name, "：") {
				sep = "："
			}
			parts := strings.SplitN(name, sep, 2)
			if len(parts) == 2 {
				category := strings.TrimSpace(parts[0])
				tagName := strings.TrimSpace(parts[1])
				if tag, ok := KnowledgeTagMap[tagName]; ok {
					// 标签名部分精确匹配
					totalDifficulty += tag.Difficulty
					matchedCount++
				} else if avgDiff, ok := CategoryAvgDifficulty[category]; ok {
					// 分类名部分匹配
					totalDifficulty += avgDiff
					matchedCount++
				}
			}
		}
	}
	if matchedCount == 0 {
		return 1.0 // 兜底：全部未命中时返回最低难度
	}
	return float64(totalDifficulty) / float64(matchedCount)
}
