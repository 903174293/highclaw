package agent

import "strings"

// memoryChunk 表示一个文档分块
type memoryChunk struct {
	Index   int
	Content string
	Heading string
}

const defaultChunkMaxTokens = 512
const estimatedCharsPerToken = 4

// chunkMarkdown 按标题和段落将 Markdown 文档分块，每块不超过 maxTokens
func chunkMarkdown(text string, maxTokens int) []memoryChunk {
	if maxTokens <= 0 {
		maxTokens = defaultChunkMaxTokens
	}
	maxChars := maxTokens * estimatedCharsPerToken

	lines := strings.Split(text, "\n")
	var chunks []memoryChunk
	var current []string
	currentHeading := ""
	idx := 0

	flush := func() {
		joined := strings.TrimSpace(strings.Join(current, "\n"))
		if joined == "" {
			return
		}
		if len(joined) <= maxChars {
			chunks = append(chunks, memoryChunk{Index: idx, Content: joined, Heading: currentHeading})
			idx++
		} else {
			for _, sub := range splitByMaxChars(joined, maxChars) {
				chunks = append(chunks, memoryChunk{Index: idx, Content: sub, Heading: currentHeading})
				idx++
			}
		}
		current = current[:0]
	}

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if isMarkdownHeading(trimmed) {
			flush()
			currentHeading = strings.TrimLeft(trimmed, "# ")
			current = append(current, line)
			continue
		}
		if trimmed == "" && len(current) > 0 && estimateTokens(strings.Join(current, "\n")) >= maxTokens {
			flush()
			continue
		}
		current = append(current, line)
	}
	flush()
	return chunks
}

func isMarkdownHeading(line string) bool {
	return strings.HasPrefix(line, "# ") || strings.HasPrefix(line, "## ") || strings.HasPrefix(line, "### ")
}

func estimateTokens(text string) int {
	return len(text) / estimatedCharsPerToken
}

// splitByMaxChars 按段落或行拆分，保证每块不超过 maxChars
func splitByMaxChars(text string, maxChars int) []string {
	paragraphs := strings.Split(text, "\n\n")
	if len(paragraphs) > 1 {
		return mergeParts(paragraphs, maxChars, "\n\n")
	}
	lines := strings.Split(text, "\n")
	return mergeParts(lines, maxChars, "\n")
}

// mergeParts 贪心合并相邻块，保证每块不超过 maxChars
func mergeParts(parts []string, maxChars int, sep string) []string {
	var result []string
	var buf []string
	bufLen := 0

	for _, p := range parts {
		pLen := len(p)
		if bufLen > 0 && bufLen+len(sep)+pLen > maxChars {
			result = append(result, strings.TrimSpace(strings.Join(buf, sep)))
			buf = buf[:0]
			bufLen = 0
		}
		buf = append(buf, p)
		if bufLen > 0 {
			bufLen += len(sep)
		}
		bufLen += pLen
	}
	if len(buf) > 0 {
		result = append(result, strings.TrimSpace(strings.Join(buf, sep)))
	}
	return result
}
