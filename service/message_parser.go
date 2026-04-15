package service

import (
	"strings"

	llmpb "github.com/yzf120/elysia-llm-tool/proto/llm"
)

// ==================== 消息解析工具 ====================

// parseMessageContent 解析消息内容，提取文本和图片
// 消息格式：文本内容\n[IMAGE:data:image/...;base64,...]\n[IMAGE:...]
// 返回：文本内容、图片URL列表
func parseMessageContent(content string) (text string, imageURLs []string) {
	const imagePrefix = "[IMAGE:"
	const imageSuffix = "]"

	lines := strings.Split(content, "\n")
	var textLines []string
	for _, line := range lines {
		if strings.HasPrefix(line, imagePrefix) && strings.HasSuffix(line, imageSuffix) {
			url := line[len(imagePrefix) : len(line)-len(imageSuffix)]
			if url != "" {
				imageURLs = append(imageURLs, url)
			}
		} else {
			textLines = append(textLines, line)
		}
	}
	text = strings.Join(textLines, "\n")
	text = strings.TrimRight(text, "\n")
	return
}

// buildLLMContentParts 将消息内容转换为 llm-tool ContentPart 列表（支持多模态）
func buildLLMContentParts(content string) []*llmpb.ContentPart {
	text, imageURLs := parseMessageContent(content)

	if len(imageURLs) == 0 {
		return []*llmpb.ContentPart{
			{Type: "text", Text: content},
		}
	}

	parts := make([]*llmpb.ContentPart, 0, 1+len(imageURLs))
	if text != "" {
		parts = append(parts, &llmpb.ContentPart{Type: "text", Text: text})
	}
	for _, url := range imageURLs {
		parts = append(parts, &llmpb.ContentPart{
			Type: "image_url",
			ImageUrl: &llmpb.ImageURL{
				Url:    url,
				Detail: "auto",
			},
		})
	}
	return parts
}
