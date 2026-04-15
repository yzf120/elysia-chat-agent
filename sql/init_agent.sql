-- ============================================================
-- elysia-chat-agent 意图相关表结构
-- 注意：这些表与 elysia-backend 共享同一个数据库
-- 如果已在 elysia-backend 中执行过 oj_intent.sql，则无需重复执行
-- ============================================================

-- 检查并创建意图字典表
CREATE TABLE IF NOT EXISTS `oj_intent_dict` (
  `id` INT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键ID',
  `intent_level1` VARCHAR(50) NOT NULL COMMENT '一级意图',
  `intent_level2` VARCHAR(100) NOT NULL COMMENT '二级子意图',
  `intent_code` VARCHAR(30) NOT NULL COMMENT '意图编码',
  `description` VARCHAR(500) DEFAULT NULL COMMENT '意图描述',
  `match_keywords` TEXT COMMENT '匹配关键词',
  `example_queries` TEXT COMMENT '示例用户问题（JSON数组）',
  `rewrite_template` TEXT COMMENT '改写模板',
  `agent_route` VARCHAR(50) NOT NULL COMMENT '路由Agent',
  `priority` INT NOT NULL DEFAULT 0 COMMENT '优先级',
  `is_valid` TINYINT NOT NULL DEFAULT 1 COMMENT '是否有效',
  `create_time` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `update_time` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_intent_code` (`intent_code`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='意图字典表';

-- 检查并创建意图提示词模板表
CREATE TABLE IF NOT EXISTS `oj_intent_prompt_template` (
  `id` INT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键ID',
  `intent_code` VARCHAR(30) NOT NULL COMMENT '关联意图编码',
  `template_type` VARCHAR(30) NOT NULL COMMENT '模板类型',
  `template_name` VARCHAR(100) NOT NULL COMMENT '模板名称',
  `template_content` TEXT NOT NULL COMMENT '模板内容',
  `is_active` TINYINT NOT NULL DEFAULT 1 COMMENT '是否启用',
  `version` INT NOT NULL DEFAULT 1 COMMENT '版本号',
  `create_time` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `update_time` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  KEY `idx_intent_code` (`intent_code`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='意图提示词模板表';

-- 检查并创建用户意图记录表
CREATE TABLE IF NOT EXISTS `oj_user_intent_record` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键ID',
  `user_id` VARCHAR(64) NOT NULL COMMENT '用户ID',
  `session_id` VARCHAR(128) DEFAULT NULL COMMENT '会话ID',
  `question_id` VARCHAR(64) DEFAULT NULL COMMENT '题目ID',
  `original_request` TEXT NOT NULL COMMENT '原始请求',
  `intent_code` VARCHAR(30) NOT NULL COMMENT '意图编码',
  `intent_level1` VARCHAR(50) NOT NULL COMMENT '一级意图',
  `rewritten_request` TEXT COMMENT '改写后的请求',
  `intent_confidence` DECIMAL(5,2) DEFAULT NULL COMMENT '置信度',
  `response_time_ms` INT DEFAULT NULL COMMENT '耗时(ms)',
  `recognize_status` TINYINT NOT NULL DEFAULT 1 COMMENT '识别状态',
  `create_time` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  KEY `idx_user_id` (`user_id`),
  KEY `idx_intent_code` (`intent_code`),
  KEY `idx_create_time` (`create_time`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='用户意图记录表';

-- ============================================================
-- 补充教师场景意图（如果不存在）
-- ============================================================
INSERT IGNORE INTO `oj_intent_dict` (`intent_level1`, `intent_level2`, `intent_code`, `description`, `match_keywords`, `example_queries`, `agent_route`, `priority`) VALUES
('IDE调试', '编程报错', 'CODE_DEBUG', '用户在IDE中遇到编译错误或运行错误', '编译错误,编译报错,error,报错,compile,语法错误', '["编译报错了","这个error什么意思","语法错误怎么改"]', 'debug_agent', 75),
('测试用例', '自动生成', 'TESTCASE_GEN', '教师请求自动生成测试用例', '测试用例,测试数据,生成用例,自动出题,test case', '["帮我生成测试用例","自动出测试数据","生成边界用例"]', 'testcase_agent', 85),
('测试用例', '批量导入', 'TESTCASE_IMPORT', '教师请求批量导入测试用例', '导入用例,批量添加,导入测试', '["导入这些测试用例","批量添加用例"]', 'testcase_agent', 80),
('题目管理', '题目审核', 'PROBLEM_REVIEW', '教师请求审核题目描述', '检查题目,审核题目,题目描述', '["检查这道题的描述","题目有没有问题"]', 'knowledge_agent', 70),
('知识管理', '知识库维护', 'KNOWLEDGE_MANAGE', '教师请求管理知识库', '添加知识点,更新知识库,知识管理', '["添加这个知识点","更新知识库"]', 'knowledge_agent', 65);

-- ============================================================
-- 学生问答行为记录表
-- ============================================================
CREATE TABLE IF NOT EXISTS `student_qa_behavior` (
    `id`                 BIGINT       NOT NULL AUTO_INCREMENT,
    `student_id`         VARCHAR(64)  NOT NULL COMMENT '学生ID',
    `conversation_id`    VARCHAR(128) DEFAULT NULL COMMENT '对话唯一标识',
    `problem_id`         BIGINT       DEFAULT NULL COMMENT '关联题目ID',
    `intent_code`        VARCHAR(30)  DEFAULT NULL COMMENT '意图编码(SOLVE_THINK/SOLVE_BUG等)',
    `question_summary`   VARCHAR(500) DEFAULT NULL COMMENT '问题摘要(由LLM提取)',
    `knowledge_tags`     JSON         DEFAULT NULL COMMENT '涉及知识点标签(如["动态规划","递归"])',
    `difficulty_score`   DECIMAL(3,1) DEFAULT NULL COMMENT '问题难度分数(由知识点标签加权计算,1.0-5.0)',
    `is_resolved`        TINYINT(1)   DEFAULT 0 COMMENT '问题是否解决(0-未知 1-已解决 2-未解决)',
    `conversation_turns` INT          DEFAULT 1 COMMENT '对话轮数',
    `conversation_time`  DATETIME     NOT NULL COMMENT '对话发生时间',
    `create_time`        DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (`id`),
    KEY `idx_student_id` (`student_id`),
    KEY `idx_conversation_id` (`conversation_id`),
    KEY `idx_problem_id` (`problem_id`),
    KEY `idx_intent_code` (`intent_code`),
    KEY `idx_conversation_time` (`conversation_time`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='学生问答行为记录表';

-- ============================================================
-- 知识点标签库表（与代码中的 KnowledgeTagLibrary 对应，可选持久化）
-- ============================================================
CREATE TABLE IF NOT EXISTS `knowledge_tag_library` (
    `id`         INT          NOT NULL AUTO_INCREMENT,
    `name`       VARCHAR(50)  NOT NULL COMMENT '标签名称',
    `category`   VARCHAR(50)  NOT NULL COMMENT '所属分类',
    `difficulty` TINYINT      NOT NULL COMMENT '难度值(1-5)',
    `is_active`  TINYINT(1)   NOT NULL DEFAULT 1 COMMENT '是否启用',
    `create_time` DATETIME    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (`id`),
    UNIQUE KEY `uk_name` (`name`),
    KEY `idx_category` (`category`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='知识点标签库';

-- 初始化知识点标签数据（90个标签）
INSERT IGNORE INTO `knowledge_tag_library` (`name`, `category`, `difficulty`) VALUES
-- 基础编程
('变量与数据类型', '基础编程', 1),
('条件判断', '基础编程', 1),
('循环结构', '基础编程', 1),
('函数与递归基础', '基础编程', 1),
('输入输出处理', '基础编程', 1),
('数组基础', '基础编程', 1),
('字符串基础', '基础编程', 1),
('指针与引用', '基础编程', 2),
('结构体与类', '基础编程', 2),
('文件操作', '基础编程', 2),
-- 数据结构
('链表', '数据结构', 2),
('栈', '数据结构', 2),
('队列', '数据结构', 2),
('哈希表', '数据结构', 2),
('二叉树', '数据结构', 3),
('二叉搜索树', '数据结构', 3),
('堆/优先队列', '数据结构', 3),
('图的表示', '数据结构', 3),
('并查集', '数据结构', 3),
('字典树/Trie', '数据结构', 4),
-- 基础算法
('暴力枚举', '基础算法', 2),
('模拟', '基础算法', 2),
('排序算法', '基础算法', 2),
('二分查找', '基础算法', 2),
('双指针', '基础算法', 2),
('滑动窗口', '基础算法', 3),
('前缀和与差分', '基础算法', 3),
('贪心算法', '基础算法', 3),
('递归与分治', '基础算法', 3),
('位运算', '基础算法', 3),
-- 搜索与图论
('深度优先搜索(DFS)', '搜索与图论', 3),
('广度优先搜索(BFS)', '搜索与图论', 3),
('回溯法', '搜索与图论', 3),
('拓扑排序', '搜索与图论', 3),
('最短路径(Dijkstra)', '搜索与图论', 4),
('最短路径(Floyd)', '搜索与图论', 4),
('最短路径(Bellman-Ford)', '搜索与图论', 4),
('最小生成树', '搜索与图论', 4),
('二分图匹配', '搜索与图论', 4),
('网络流', '搜索与图论', 5),
-- 动态规划
('线性DP', '动态规划', 3),
('背包问题', '动态规划', 3),
('区间DP', '动态规划', 4),
('树形DP', '动态规划', 4),
('状态压缩DP', '动态规划', 4),
('数位DP', '动态规划', 5),
('概率/期望DP', '动态规划', 5),
('记忆化搜索', '动态规划', 3),
-- 数学与数论
('基础数学运算', '数学与数论', 2),
('素数与筛法', '数学与数论', 3),
('最大公约数/最小公倍数', '数学与数论', 2),
('快速幂', '数学与数论', 3),
('组合数学', '数学与数论', 4),
('矩阵运算', '数学与数论', 4),
('博弈论', '数学与数论', 4),
('容斥原理', '数学与数论', 5),
-- 高级数据结构
('线段树', '高级数据结构', 4),
('树状数组', '高级数据结构', 4),
('平衡二叉树(AVL/红黑树)', '高级数据结构', 5),
('跳表', '高级数据结构', 4),
('LCA(最近公共祖先)', '高级数据结构', 4),
('可持久化数据结构', '高级数据结构', 5),
-- 高级算法
('单调栈/单调队列', '高级算法', 4),
('启发式搜索(A*)', '高级算法', 4),
('随机化算法', '高级算法', 4),
('CDQ分治', '高级算法', 5),
('莫队算法', '高级算法', 5),
-- 字符串算法
('字符串匹配(KMP)', '字符串算法', 4),
('字符串哈希', '字符串算法', 3),
('Manacher算法', '字符串算法', 5),
('后缀数组', '字符串算法', 5),
('AC自动机', '字符串算法', 5),
-- 计算机基础
('时间复杂度分析', '计算机基础', 2),
('空间复杂度分析', '计算机基础', 2),
('进制转换', '计算机基础', 1),
('编码与字符集', '计算机基础', 2),
('内存管理基础', '计算机基础', 3),
-- 操作系统与网络
('进程与线程', '操作系统与网络', 3),
('并发与同步', '操作系统与网络', 4),
('死锁', '操作系统与网络', 3),
('TCP/IP协议', '操作系统与网络', 3),
('HTTP协议', '操作系统与网络', 2),
-- 数据库与工程
('SQL基础', '数据库与工程', 2),
('数据库索引', '数据库与工程', 3),
('事务与并发控制', '数据库与工程', 4),
('设计模式', '数据库与工程', 3),
('版本控制(Git)', '数据库与工程', 2);

-- ============================================================
-- 补充教师场景提示词模板
-- ============================================================
INSERT IGNORE INTO `oj_intent_prompt_template` (`intent_code`, `template_type`, `template_name`, `template_content`, `is_active`, `version`) VALUES
('CODE_DEBUG', 'system_prompt', 'IDE调试-系统提示词v1', '你是一位专业的OJ编程助教。学生在IDE中遇到了编程错误需要帮助。请注意：\n1. 解析错误信息，翻译为学生能理解的语言\n2. 指出错误所在的代码行\n3. 解释语法规则或运行时错误原因\n4. 给出修复建议和调试技巧', 1, 1),
('TESTCASE_GEN', 'system_prompt', '测试用例生成-系统提示词v1', '你是一位专业的OJ题目测试用例生成助手。请根据题目信息生成高质量的测试用例。请注意：\n1. 覆盖基础用例、边界用例、特殊用例和压力用例\n2. 确保用例符合题目约束条件\n3. 输出严格的JSON格式\n4. 每个用例附带说明', 1, 1);
