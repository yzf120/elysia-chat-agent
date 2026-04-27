package service

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// ==================== 1.14 parseMessageContent ====================

func TestParseMessageContent(t *testing.T) {
	t.Run("纯文本", func(t *testing.T) {
		text, imgs := parseMessageContent("hello world")
		assert.Equal(t, "hello world", text)
		assert.Empty(t, imgs)
	})

	t.Run("多行纯文本", func(t *testing.T) {
		text, imgs := parseMessageContent("第一行\n第二行\n第三行")
		assert.Equal(t, "第一行\n第二行\n第三行", text)
		assert.Empty(t, imgs)
	})

	t.Run("带单张图片", func(t *testing.T) {
		content := "请看图片\n[IMAGE:data:image/png;base64,abc123]"
		text, imgs := parseMessageContent(content)
		assert.Equal(t, "请看图片", text)
		assert.Len(t, imgs, 1)
		assert.Equal(t, "data:image/png;base64,abc123", imgs[0])
	})

	t.Run("带多张图片", func(t *testing.T) {
		content := "[IMAGE:url1]\n文本内容\n[IMAGE:url2]\n更多文本\n[IMAGE:url3]"
		text, imgs := parseMessageContent(content)
		assert.Contains(t, text, "文本内容")
		assert.Contains(t, text, "更多文本")
		assert.Len(t, imgs, 3)
		assert.Equal(t, "url1", imgs[0])
		assert.Equal(t, "url2", imgs[1])
		assert.Equal(t, "url3", imgs[2])
	})

	t.Run("空内容", func(t *testing.T) {
		text, imgs := parseMessageContent("")
		assert.Equal(t, "", text)
		assert.Empty(t, imgs)
	})

	t.Run("只有图片_无文本", func(t *testing.T) {
		content := "[IMAGE:data:image/jpeg;base64,xyz]"
		text, imgs := parseMessageContent(content)
		assert.Equal(t, "", text)
		assert.Len(t, imgs, 1)
	})

	t.Run("IMAGE标记不完整_当作文本", func(t *testing.T) {
		content := "[IMAGE:没有结束方括号"
		text, imgs := parseMessageContent(content)
		assert.Contains(t, text, "[IMAGE:")
		assert.Empty(t, imgs)
	})
}

// ==================== 1.15 buildLLMContentParts ====================

func TestBuildLLMContentParts(t *testing.T) {
	t.Run("纯文本_单个text_part", func(t *testing.T) {
		parts := buildLLMContentParts("hello world")
		assert.Len(t, parts, 1)
		assert.Equal(t, "text", parts[0].Type)
		assert.Equal(t, "hello world", parts[0].Text)
	})

	t.Run("图文混合_多个parts", func(t *testing.T) {
		content := "描述文本\n[IMAGE:data:image/png;base64,abc]"
		parts := buildLLMContentParts(content)
		assert.Len(t, parts, 2)
		assert.Equal(t, "text", parts[0].Type)
		assert.Equal(t, "描述文本", parts[0].Text)
		assert.Equal(t, "image_url", parts[1].Type)
		assert.NotNil(t, parts[1].ImageUrl)
		assert.Equal(t, "data:image/png;base64,abc", parts[1].ImageUrl.Url)
		assert.Equal(t, "auto", parts[1].ImageUrl.Detail)
	})

	t.Run("多张图片", func(t *testing.T) {
		content := "文本\n[IMAGE:url1]\n[IMAGE:url2]"
		parts := buildLLMContentParts(content)
		assert.Len(t, parts, 3) // 1 text + 2 image_url
		assert.Equal(t, "text", parts[0].Type)
		assert.Equal(t, "image_url", parts[1].Type)
		assert.Equal(t, "image_url", parts[2].Type)
	})

	t.Run("只有图片_无文本part", func(t *testing.T) {
		content := "[IMAGE:data:image/jpeg;base64,xyz]"
		parts := buildLLMContentParts(content)
		// 只有图片时，text 为空，不应有 text part
		assert.GreaterOrEqual(t, len(parts), 1)
		// 最后一个应该是 image_url
		assert.Equal(t, "image_url", parts[len(parts)-1].Type)
	})
}
