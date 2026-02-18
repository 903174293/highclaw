package agent

import (
	"strings"

	"github.com/highclaw/highclaw/internal/config"
)

// MemoryEntryDTO 是面向外部包的记忆条目结构
type MemoryEntryDTO struct {
	Key       string
	Content   string
	Category  string
	Score     float64
	CreatedAt string
	UpdatedAt string
}

func toDTO(e memoryEntry) MemoryEntryDTO {
	return MemoryEntryDTO{
		Key:       e.Key,
		Content:   e.Content,
		Category:  e.Category,
		Score:     e.Score,
		CreatedAt: e.CreatedAt,
		UpdatedAt: e.UpdatedAt,
	}
}

func toDTOs(entries []memoryEntry) []MemoryEntryDTO {
	out := make([]MemoryEntryDTO, len(entries))
	for i, e := range entries {
		out[i] = toDTO(e)
	}
	return out
}

// 根据 config 创建对应的 memoryStore 实例
func resolveMemoryStore(cfg *config.Config) memoryStore {
	backend := strings.ToLower(strings.TrimSpace(cfg.Memory.Backend))
	switch backend {
	case "sqlite":
		return newSQLiteMemoryStore(cfg)
	case "markdown", "none", "":
		ws := strings.TrimSpace(cfg.Agent.Workspace)
		return newMarkdownMemoryStore(ws)
	default:
		ws := strings.TrimSpace(cfg.Agent.Workspace)
		return newMarkdownMemoryStore(ws)
	}
}

// SearchMemory 在记忆后端中搜索
func SearchMemory(cfg *config.Config, query string, limit int, category string) ([]MemoryEntryDTO, error) {
	if limit <= 0 {
		limit = 20
	}
	ms := resolveMemoryStore(cfg)
	if err := ms.init(); err != nil {
		return nil, err
	}
	entries, err := ms.recall(query, "", "", limit)
	if err != nil {
		return nil, err
	}
	if category != "" {
		filtered := make([]memoryEntry, 0, len(entries))
		for _, e := range entries {
			if strings.EqualFold(e.Category, category) {
				filtered = append(filtered, e)
			}
		}
		entries = filtered
	}
	return toDTOs(entries), nil
}

// GetMemory 按 key 精确查找
func GetMemory(cfg *config.Config, key string) (*MemoryEntryDTO, error) {
	ms := resolveMemoryStore(cfg)
	if err := ms.init(); err != nil {
		return nil, err
	}
	entry, err := ms.get(key)
	if err != nil {
		return nil, err
	}
	if entry == nil {
		return nil, nil
	}
	dto := toDTO(*entry)
	return &dto, nil
}

// ListMemory 列出记忆条目
func ListMemory(cfg *config.Config, category string, limit int) ([]MemoryEntryDTO, error) {
	ms := resolveMemoryStore(cfg)
	if err := ms.init(); err != nil {
		return nil, err
	}
	entries, err := ms.list(category)
	if err != nil {
		return nil, err
	}
	if limit > 0 && len(entries) > limit {
		entries = entries[:limit]
	}
	return toDTOs(entries), nil
}

// CountMemory 返回记忆条目总数
func CountMemory(cfg *config.Config) (int, error) {
	ms := resolveMemoryStore(cfg)
	if err := ms.init(); err != nil {
		return 0, err
	}
	return ms.count()
}

// MemoryLocation 返回记忆后端存储位置
func MemoryLocation(cfg *config.Config) string {
	ms := resolveMemoryStore(cfg)
	_ = ms.init()
	return ms.location()
}

// MemoryHealth 返回记忆后端健康状态
func MemoryHealth(cfg *config.Config) bool {
	ms := resolveMemoryStore(cfg)
	if err := ms.init(); err != nil {
		return false
	}
	return ms.healthCheck()
}

// ReindexMemory 重建 FTS 索引并补全缺失 embedding（仅 sqlite 后端）
func ReindexMemory(cfg *config.Config) (int, error) {
	ms := resolveMemoryStore(cfg)
	if err := ms.init(); err != nil {
		return 0, err
	}
	if sq, ok := ms.(*sqliteMemoryStore); ok {
		return sq.reindex()
	}
	return 0, nil
}

// ChunkMarkdownDTO 文档分块的导出结构
type ChunkMarkdownDTO struct {
	Index   int
	Content string
	Heading string
}

// ChunkMarkdown 将 Markdown 文本按标题和段落分块
func ChunkMarkdown(text string, maxTokens int) []ChunkMarkdownDTO {
	chunks := chunkMarkdown(text, maxTokens)
	out := make([]ChunkMarkdownDTO, len(chunks))
	for i, c := range chunks {
		out[i] = ChunkMarkdownDTO{Index: c.Index, Content: c.Content, Heading: c.Heading}
	}
	return out
}
