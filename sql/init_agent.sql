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
-- 补充教师场景提示词模板
-- ============================================================
INSERT IGNORE INTO `oj_intent_prompt_template` (`intent_code`, `template_type`, `template_name`, `template_content`, `is_active`, `version`) VALUES
('CODE_DEBUG', 'system_prompt', 'IDE调试-系统提示词v1', '你是一位专业的OJ编程助教。学生在IDE中遇到了编程错误需要帮助。请注意：\n1. 解析错误信息，翻译为学生能理解的语言\n2. 指出错误所在的代码行\n3. 解释语法规则或运行时错误原因\n4. 给出修复建议和调试技巧', 1, 1),
('TESTCASE_GEN', 'system_prompt', '测试用例生成-系统提示词v1', '你是一位专业的OJ题目测试用例生成助手。请根据题目信息生成高质量的测试用例。请注意：\n1. 覆盖基础用例、边界用例、特殊用例和压力用例\n2. 确保用例符合题目约束条件\n3. 输出严格的JSON格式\n4. 每个用例附带说明', 1, 1);
