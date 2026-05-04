-- ============================================================
-- elysia-chat-agent 知识库文档表
-- 用于持久化存储 RAG 知识库中的文档信息
-- ============================================================

-- 知识库文档表（主表，存储文档元信息）
CREATE TABLE IF NOT EXISTS `oj_knowledge_document` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键ID',
  `doc_id` VARCHAR(64) NOT NULL COMMENT '文档唯一标识（用于Redis索引关联）',
  `file_name` VARCHAR(255) NOT NULL COMMENT '原始文件名',
  `file_size` BIGINT NOT NULL DEFAULT 0 COMMENT '文件大小（字节）',
  `file_type` VARCHAR(20) NOT NULL COMMENT '文件类型（pdf/docx/txt）',
  `content` LONGTEXT NOT NULL COMMENT '文档解析后的文本内容',
  `source_type` VARCHAR(50) NOT NULL DEFAULT 'knowledge_base' COMMENT '来源类型（knowledge_base/problem_bank/error_pattern）',
  `source_id` VARCHAR(64) DEFAULT NULL COMMENT '关联的知识点/题目ID',
  `tags` VARCHAR(500) DEFAULT NULL COMMENT '标签（逗号分隔）',
  `chunk_count` INT NOT NULL DEFAULT 0 COMMENT '分块数量',
  `status` TINYINT NOT NULL DEFAULT 0 COMMENT '状态（0-待处理 1-处理中 2-已处理 3-处理失败）',
  `error_msg` VARCHAR(500) DEFAULT NULL COMMENT '处理失败时的错误信息',
  `uploaded_by` VARCHAR(64) DEFAULT NULL COMMENT '上传者ID',
  `create_time` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `update_time` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_doc_id` (`doc_id`),
  KEY `idx_source_type` (`source_type`),
  KEY `idx_status` (`status`),
  KEY `idx_file_name` (`file_name`),
  KEY `idx_create_time` (`create_time`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='知识库文档表';

-- 知识库文档分块表（存储文档拆分后的片段，每个片段独立入Redis索引）
CREATE TABLE IF NOT EXISTS `oj_knowledge_chunk` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键ID',
  `chunk_id` VARCHAR(64) NOT NULL COMMENT '分块唯一标识（对应Redis中的文档ID）',
  `doc_id` VARCHAR(64) NOT NULL COMMENT '所属文档ID',
  `chunk_index` INT NOT NULL DEFAULT 0 COMMENT '分块序号',
  `content` TEXT NOT NULL COMMENT '分块文本内容',
  `tokens_count` INT NOT NULL DEFAULT 0 COMMENT '分块token数（近似）',
  `create_time` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_chunk_id` (`chunk_id`),
  KEY `idx_doc_id` (`doc_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='知识库文档分块表';
