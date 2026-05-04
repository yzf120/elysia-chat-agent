package rag

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// ==================== 中文分词器 ====================
// 基于正向最大匹配 + 字符级 bigram 的中文分词方案
// 无需 CGO 依赖，内置编程教育领域词典

// Tokenizer 中文分词器
type Tokenizer struct {
	dict    map[string]bool // 词典
	maxLen  int             // 词典中最长词的字符数
	stopSet map[string]bool // 停用词集合
}

var defaultTokenizer *Tokenizer

// InitTokenizer 初始化分词器
func InitTokenizer() {
	defaultTokenizer = NewTokenizer()
}

// GetTokenizer 获取默认分词器
func GetTokenizer() *Tokenizer {
	if defaultTokenizer == nil {
		InitTokenizer()
	}
	return defaultTokenizer
}

// NewTokenizer 创建分词器实例
func NewTokenizer() *Tokenizer {
	t := &Tokenizer{
		dict:    make(map[string]bool),
		maxLen:  6,
		stopSet: make(map[string]bool),
	}
	t.loadDict()
	t.loadStopWords()
	return t
}

// Tokenize 对文本进行分词，返回分词结果
func (t *Tokenizer) Tokenize(text string) []string {
	if text == "" {
		return nil
	}

	var tokens []string

	// 将文本按中英文分段处理
	segments := t.segmentByType(text)
	for _, seg := range segments {
		if seg.isChinese {
			// 中文部分：正向最大匹配分词
			chTokens := t.forwardMaxMatch(seg.text)
			tokens = append(tokens, chTokens...)
			// 补充 bigram 增强召回
			bigrams := t.generateBigrams(seg.text)
			tokens = append(tokens, bigrams...)
		} else {
			// 英文/数字部分：按空格分割
			words := strings.Fields(seg.text)
			for _, w := range words {
				w = strings.ToLower(strings.TrimSpace(w))
				if len(w) > 1 {
					tokens = append(tokens, w)
				}
			}
		}
	}

	return tokens
}

// TokenizeForIndex 用于建立索引时的分词（更细粒度）
func (t *Tokenizer) TokenizeForIndex(text string) []string {
	if text == "" {
		return nil
	}

	var tokens []string

	segments := t.segmentByType(text)
	for _, seg := range segments {
		if seg.isChinese {
			// 正向最大匹配
			chTokens := t.forwardMaxMatch(seg.text)
			tokens = append(tokens, chTokens...)
			// bigram
			bigrams := t.generateBigrams(seg.text)
			tokens = append(tokens, bigrams...)
			// unigram（单字，用于兜底匹配）
			unigrams := t.generateUnigrams(seg.text)
			tokens = append(tokens, unigrams...)
		} else {
			words := strings.Fields(seg.text)
			for _, w := range words {
				w = strings.ToLower(strings.TrimSpace(w))
				if len(w) > 1 {
					tokens = append(tokens, w)
				}
			}
		}
	}

	return tokens
}

// TokenizeForQuery 用于查询时的分词（去停用词）
func (t *Tokenizer) TokenizeForQuery(text string) []string {
	tokens := t.Tokenize(text)
	var result []string
	seen := make(map[string]bool)
	for _, tok := range tokens {
		lower := strings.ToLower(tok)
		if t.stopSet[lower] || seen[lower] {
			continue
		}
		seen[lower] = true
		result = append(result, tok)
	}
	return result
}

// ==================== 内部方法 ====================

// textSegment 文本段
type textSegment struct {
	text      string
	isChinese bool
}

// segmentByType 将文本按中文/非中文分段
func (t *Tokenizer) segmentByType(text string) []textSegment {
	var segments []textSegment
	var current strings.Builder
	var currentIsChinese bool
	first := true

	for _, r := range text {
		isCh := isChinese(r)
		if first {
			currentIsChinese = isCh
			first = false
		}

		if isCh != currentIsChinese {
			if current.Len() > 0 {
				segments = append(segments, textSegment{
					text:      current.String(),
					isChinese: currentIsChinese,
				})
				current.Reset()
			}
			currentIsChinese = isCh
		}

		if !isPunctuation(r) || !isCh {
			current.WriteRune(r)
		}
	}

	if current.Len() > 0 {
		segments = append(segments, textSegment{
			text:      current.String(),
			isChinese: currentIsChinese,
		})
	}

	return segments
}

// forwardMaxMatch 正向最大匹配分词
func (t *Tokenizer) forwardMaxMatch(text string) []string {
	runes := []rune(text)
	var tokens []string
	i := 0

	for i < len(runes) {
		// 跳过标点和空白
		if isPunctuation(runes[i]) || unicode.IsSpace(runes[i]) {
			i++
			continue
		}

		matched := false
		// 从最大长度开始尝试匹配
		maxL := t.maxLen
		if i+maxL > len(runes) {
			maxL = len(runes) - i
		}

		for l := maxL; l >= 2; l-- {
			word := string(runes[i : i+l])
			if t.dict[word] {
				tokens = append(tokens, word)
				i += l
				matched = true
				break
			}
		}

		if !matched {
			// 未匹配到词典词，取单字
			i++
		}
	}

	return tokens
}

// generateBigrams 生成中文 bigram（相邻两字组合）
func (t *Tokenizer) generateBigrams(text string) []string {
	runes := []rune(text)
	var bigrams []string

	// 过滤掉标点
	var filtered []rune
	for _, r := range runes {
		if !isPunctuation(r) && !unicode.IsSpace(r) {
			filtered = append(filtered, r)
		}
	}

	for i := 0; i < len(filtered)-1; i++ {
		bigram := string(filtered[i : i+2])
		bigrams = append(bigrams, bigram)
	}

	return bigrams
}

// generateUnigrams 生成中文 unigram（单字）
func (t *Tokenizer) generateUnigrams(text string) []string {
	var unigrams []string
	for _, r := range text {
		if !isPunctuation(r) && !unicode.IsSpace(r) && isChinese(r) {
			unigrams = append(unigrams, string(r))
		}
	}
	return unigrams
}

// isChinese 判断是否为中文字符
func isChinese(r rune) bool {
	return unicode.Is(unicode.Han, r)
}

// isPunctuation 判断是否为标点符号
func isPunctuation(r rune) bool {
	return unicode.IsPunct(r) || unicode.IsSymbol(r) ||
		r == '，' || r == '。' || r == '？' || r == '！' ||
		r == '、' || r == '：' || r == '；' || r == '\u201c' ||
		r == '\u201d' || r == '\u2018' || r == '\u2019' || r == '（' ||
		r == '）' || r == '【' || r == '】' || r == '《' ||
		r == '》' || r == '\n' || r == '\t'
}

// loadStopWords 加载停用词
func (t *Tokenizer) loadStopWords() {
	stopWords := []string{
		// 中文停用词
		"的", "了", "是", "在", "我", "有", "和", "就", "不", "人",
		"都", "一", "一个", "上", "也", "很", "到", "说", "要", "去",
		"你", "会", "着", "没有", "看", "好", "自己", "这", "他", "她",
		"吗", "吧", "呢", "啊", "哦", "把", "被", "让", "给", "从",
		"对", "而", "但", "如果", "那", "这个", "那个", "什么", "怎么",
		"为什么", "如何", "能不能", "可以", "帮", "请", "帮我", "请问",
		"能", "想", "用", "做", "来", "还", "以", "及", "等", "或",
		// 英文停用词
		"the", "a", "an", "is", "are", "was", "were", "be", "been",
		"being", "have", "has", "had", "do", "does", "did", "will",
		"would", "could", "should", "may", "might", "can", "shall",
		"i", "me", "my", "we", "our", "you", "your", "he", "him",
		"his", "she", "her", "it", "its", "they", "them", "their",
		"this", "that", "these", "those", "what", "which", "who",
		"how", "when", "where", "why", "if", "then", "else",
		"for", "to", "of", "in", "on", "at", "by", "with", "from",
		"and", "or", "not", "no", "but", "so", "as", "than",
	}

	for _, w := range stopWords {
		t.stopSet[w] = true
	}
}

// loadDict 加载编程教育领域词典
func (t *Tokenizer) loadDict() {
	// 编程教育领域核心词汇
	dictWords := []string{
		// ===== 数据结构 =====
		"数据结构", "数组", "链表", "栈", "队列", "堆", "树",
		"二叉树", "二叉搜索树", "平衡树", "红黑树", "B树",
		"哈希表", "散列表", "图", "有向图", "无向图", "邻接表",
		"邻接矩阵", "优先队列", "双端队列", "循环队列",
		"单链表", "双链表", "跳表", "字典树", "前缀树",
		"线段树", "树状数组", "并查集", "最小堆", "最大堆",

		// ===== 算法 =====
		"算法", "排序", "查找", "搜索", "遍历",
		"冒泡排序", "选择排序", "插入排序", "快速排序", "归并排序",
		"堆排序", "计数排序", "桶排序", "基数排序", "希尔排序",
		"二分查找", "线性查找", "深度优先搜索", "广度优先搜索",
		"深度优先", "广度优先", "回溯", "回溯法", "分治", "分治法",
		"贪心", "贪心算法", "动态规划", "记忆化搜索",
		"递归", "迭代", "双指针", "滑动窗口", "前缀和",
		"拓扑排序", "最短路径", "最小生成树", "最大流",
		"字符串匹配", "KMP算法", "暴力搜索",

		// ===== 编程语言 =====
		"编程语言", "编程", "程序设计", "面向对象", "函数式编程",
		"变量", "常量", "函数", "方法", "类", "对象", "接口",
		"继承", "多态", "封装", "抽象", "泛型", "模板",
		"指针", "引用", "值传递", "引用传递",
		"作用域", "生命周期", "垃圾回收", "内存管理",
		"异常处理", "错误处理", "断言",

		// ===== 具体语言 =====
		"Python", "Java", "JavaScript", "TypeScript",
		"Go", "Golang", "Rust", "Ruby",
		"C语言", "C++",

		// ===== 编程概念 =====
		"时间复杂度", "空间复杂度", "复杂度分析", "大O表示法",
		"最优解", "最坏情况", "平均情况", "渐进分析",
		"输入输出", "标准输入", "标准输出",
		"编译", "解释", "运行时", "编译器", "解释器",
		"调试", "断点", "单步执行", "日志",

		// ===== 常见问题类型 =====
		"两数之和", "三数之和", "最长公共子序列", "最长递增子序列",
		"最大子数组", "背包问题", "零一背包", "完全背包",
		"爬楼梯", "斐波那契", "汉诺塔", "八皇后",
		"最短路", "最长路", "环检测", "连通分量",
		"字符串反转", "回文", "括号匹配", "表达式求值",
		"二叉树遍历", "前序遍历", "中序遍历", "后序遍历", "层序遍历",
		"编辑距离", "最长公共前缀",

		// ===== 错误类型 =====
		"数组越界", "空指针", "栈溢出", "堆溢出", "内存泄漏",
		"死循环", "无限递归", "段错误", "运行时错误",
		"编译错误", "语法错误", "逻辑错误", "类型错误",
		"超时", "超出内存限制", "输出格式错误",
		"除零错误", "下标越界", "未定义行为",

		// ===== 编程教育 =====
		"编程教育", "算法竞赛", "在线评测", "代码提交",
		"测试用例", "样例输入", "样例输出", "题目描述",
		"输入格式", "输出格式", "时间限制", "内存限制",
		"通过", "未通过", "部分通过", "运行错误",
		"答案错误", "编译失败", "提交记录",

		// ===== 数学基础 =====
		"数学", "数论", "组合数学", "概率", "统计",
		"矩阵", "向量", "线性代数", "离散数学",
		"模运算", "最大公约数", "最小公倍数", "质数", "素数",
		"排列", "组合", "阶乘", "幂运算",
	}

	for _, w := range dictWords {
		t.dict[w] = true
		// 更新最大词长
		wLen := utf8.RuneCountInString(w)
		if wLen > t.maxLen {
			t.maxLen = wLen
		}
	}
}
