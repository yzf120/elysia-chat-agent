package dao

import (
	"fmt"
	"time"

	"gorm.io/gorm"
)

// ==================== 知识库文档 DAO ====================

// KnowledgeDocument 知识库文档模型（对应 oj_knowledge_document 表）
type KnowledgeDocument struct {
	ID         uint64    `gorm:"primaryKey;autoIncrement" json:"id"`
	DocID      string    `gorm:"column:doc_id;type:varchar(64);uniqueIndex" json:"doc_id"`
	FileName   string    `gorm:"column:file_name;type:varchar(255)" json:"file_name"`
	FileSize   int64     `gorm:"column:file_size" json:"file_size"`
	FileType   string    `gorm:"column:file_type;type:varchar(20)" json:"file_type"`
	Content    string    `gorm:"column:content;type:longtext" json:"content"`
	SourceType string    `gorm:"column:source_type;type:varchar(50);default:knowledge_base" json:"source_type"`
	SourceID   string    `gorm:"column:source_id;type:varchar(64)" json:"source_id"`
	Tags       string    `gorm:"column:tags;type:varchar(500)" json:"tags"`
	ChunkCount int       `gorm:"column:chunk_count;default:0" json:"chunk_count"`
	Status     int       `gorm:"column:status;default:0" json:"status"` // 0-待处理 1-处理中 2-已处理 3-处理失败
	ErrorMsg   string    `gorm:"column:error_msg;type:varchar(500)" json:"error_msg"`
	UploadedBy string    `gorm:"column:uploaded_by;type:varchar(64)" json:"uploaded_by"`
	CreateTime time.Time `gorm:"column:create_time;autoCreateTime" json:"create_time"`
	UpdateTime time.Time `gorm:"column:update_time;autoUpdateTime" json:"update_time"`
}

func (KnowledgeDocument) TableName() string {
	return "oj_knowledge_document"
}

// KnowledgeChunk 知识库文档分块模型（对应 oj_knowledge_chunk 表）
type KnowledgeChunk struct {
	ID          uint64    `gorm:"primaryKey;autoIncrement" json:"id"`
	ChunkID     string    `gorm:"column:chunk_id;type:varchar(64);uniqueIndex" json:"chunk_id"`
	DocID       string    `gorm:"column:doc_id;type:varchar(64);index" json:"doc_id"`
	ChunkIndex  int       `gorm:"column:chunk_index;default:0" json:"chunk_index"`
	Content     string    `gorm:"column:content;type:text" json:"content"`
	TokensCount int       `gorm:"column:tokens_count;default:0" json:"tokens_count"`
	CreateTime  time.Time `gorm:"column:create_time;autoCreateTime" json:"create_time"`
}

func (KnowledgeChunk) TableName() string {
	return "oj_knowledge_chunk"
}

// KnowledgeDocDAO 知识库文档数据访问对象
type KnowledgeDocDAO struct {
	db *gorm.DB
}

// NewKnowledgeDocDAO 创建知识库文档 DAO
func NewKnowledgeDocDAO() *KnowledgeDocDAO {
	return &KnowledgeDocDAO{db: GetDB()}
}

// CreateDocument 创建文档记录
func (d *KnowledgeDocDAO) CreateDocument(doc *KnowledgeDocument) error {
	return d.db.Create(doc).Error
}

// GetDocumentByDocID 根据 doc_id 获取文档
func (d *KnowledgeDocDAO) GetDocumentByDocID(docID string) (*KnowledgeDocument, error) {
	var doc KnowledgeDocument
	err := d.db.Where("doc_id = ?", docID).First(&doc).Error
	if err != nil {
		return nil, err
	}
	return &doc, nil
}

// UpdateDocumentStatus 更新文档状态
func (d *KnowledgeDocDAO) UpdateDocumentStatus(docID string, status int, errMsg string) error {
	updates := map[string]interface{}{
		"status": status,
	}
	if errMsg != "" {
		updates["error_msg"] = errMsg
	}
	return d.db.Model(&KnowledgeDocument{}).Where("doc_id = ?", docID).Updates(updates).Error
}

// UpdateDocumentChunkCount 更新文档分块数量
func (d *KnowledgeDocDAO) UpdateDocumentChunkCount(docID string, count int) error {
	return d.db.Model(&KnowledgeDocument{}).Where("doc_id = ?", docID).Update("chunk_count", count).Error
}

// DeleteDocument 删除文档记录
func (d *KnowledgeDocDAO) DeleteDocument(docID string) error {
	return d.db.Where("doc_id = ?", docID).Delete(&KnowledgeDocument{}).Error
}

// ListDocuments 分页查询文档列表
func (d *KnowledgeDocDAO) ListDocuments(page, pageSize int, keyword string) ([]KnowledgeDocument, int64, error) {
	var docs []KnowledgeDocument
	var total int64

	query := d.db.Model(&KnowledgeDocument{})
	if keyword != "" {
		query = query.Where("file_name LIKE ?", "%"+keyword+"%")
	}

	// 先查总数
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// 分页查询（不返回 content 大字段，减少传输量）
	offset := (page - 1) * pageSize
	err := query.Select("id, doc_id, file_name, file_size, file_type, source_type, source_id, tags, chunk_count, status, error_msg, uploaded_by, create_time, update_time").
		Order("create_time DESC").
		Offset(offset).Limit(pageSize).
		Find(&docs).Error
	if err != nil {
		return nil, 0, err
	}

	return docs, total, nil
}

// CreateChunk 创建分块记录
func (d *KnowledgeDocDAO) CreateChunk(chunk *KnowledgeChunk) error {
	return d.db.Create(chunk).Error
}

// CreateChunks 批量创建分块记录
func (d *KnowledgeDocDAO) CreateChunks(chunks []KnowledgeChunk) error {
	if len(chunks) == 0 {
		return nil
	}
	return d.db.CreateInBatches(chunks, 100).Error
}

// GetChunksByDocID 获取文档的所有分块
func (d *KnowledgeDocDAO) GetChunksByDocID(docID string) ([]KnowledgeChunk, error) {
	var chunks []KnowledgeChunk
	err := d.db.Where("doc_id = ?", docID).Order("chunk_index ASC").Find(&chunks).Error
	return chunks, err
}

// DeleteChunksByDocID 删除文档的所有分块
func (d *KnowledgeDocDAO) DeleteChunksByDocID(docID string) error {
	return d.db.Where("doc_id = ?", docID).Delete(&KnowledgeChunk{}).Error
}

// DeleteDocumentWithChunks 在同一个事务中删除文档及其所有分块，保证原子性
func (d *KnowledgeDocDAO) DeleteDocumentWithChunks(docID string) error {
	return d.db.Transaction(func(tx *gorm.DB) error {
		// 先删除分块
		if err := tx.Where("doc_id = ?", docID).Delete(&KnowledgeChunk{}).Error; err != nil {
			return fmt.Errorf("删除分块失败: %w", err)
		}
		// 再删除文档
		if err := tx.Where("doc_id = ?", docID).Delete(&KnowledgeDocument{}).Error; err != nil {
			return fmt.Errorf("删除文档失败: %w", err)
		}
		return nil
	})
}
