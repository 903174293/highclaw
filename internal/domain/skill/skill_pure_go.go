// Package skill defines pure Go skills (no Node.js dependencies).
package skill

import "context"

// PureGoSkillsInfo returns all skills implemented in pure Go.
var PureGoSkillsInfo = []SkillInfo{
	{
		ID:          "bash",
		Name:        "Bash",
		Icon:        "💻",
		Category:    "system",
		Description: "Execute bash commands",
		RequiresNode: false,
		BinaryDeps:   []string{"bash"},
		Status:       StatusEligible,
	},
	{
		ID:          "web-search",
		Name:        "Web Search",
		Icon:        "🔍",
		Category:    "search",
		Description: "Search the web using various engines",
		RequiresNode: false,
		RequiredEnvVars: []string{"SEARCH_API_KEY"}, // Optional
		Status:       StatusEligible,
	},
	{
		ID:          "file-read",
		Name:        "File Read",
		Icon:        "📄",
		Category:    "filesystem",
		Description: "Read files from disk",
		RequiresNode: false,
		Status:       StatusEligible,
	},
	{
		ID:          "file-write",
		Name:        "File Write",
		Icon:        "✍️",
		Category:    "filesystem",
		Description: "Write files to disk",
		RequiresNode: false,
		Status:       StatusEligible,
	},
	{
		ID:          "http-request",
		Name:        "HTTP Request",
		Icon:        "🌐",
		Category:    "web",
		Description: "Make HTTP requests",
		RequiresNode: false,
		Status:       StatusEligible,
	},
	{
		ID:          "json-parse",
		Name:        "JSON Parse",
		Icon:        "📊",
		Category:    "data",
		Description: "Parse and manipulate JSON",
		RequiresNode: false,
		Status:       StatusEligible,
	},
	{
		ID:          "sqlite",
		Name:        "SQLite",
		Icon:        "🗄️",
		Category:    "database",
		Description: "Query SQLite databases",
		RequiresNode: false,
		Status:       StatusEligible,
	},
	{
		ID:          "image-process",
		Name:        "Image Processing",
		Icon:        "🖼️",
		Category:    "media",
		Description: "Process images (resize, crop, etc.)",
		RequiresNode: false,
		Status:       StatusEligible,
	},
}

// BaseSkill provides common functionality for all skills.
type BaseSkill struct {
	id          string
	name        string
	description string
}

func (b *BaseSkill) ID() string          { return b.id }
func (b *BaseSkill) Name() string        { return b.name }
func (b *BaseSkill) Description() string { return b.description }
func (b *BaseSkill) Status() Status      { return StatusEligible }

// BashSkill executes bash commands.
type BashSkill struct {
	BaseSkill
}

func NewBashSkill() *BashSkill {
	return &BashSkill{
		BaseSkill: BaseSkill{
			id:          "bash",
			name:        "Bash",
			description: "Execute bash commands",
		},
	}
}

func (s *BashSkill) Execute(ctx context.Context, params map[string]any) (any, error) {
	// TODO: Implement bash execution
	return nil, nil
}

func (s *BashSkill) Schema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"command": map[string]any{
				"type":        "string",
				"description": "The bash command to execute",
			},
		},
		"required": []string{"command"},
	}
}

// WebSearchSkill searches the web.
type WebSearchSkill struct {
	BaseSkill
}

func NewWebSearchSkill() *WebSearchSkill {
	return &WebSearchSkill{
		BaseSkill: BaseSkill{
			id:          "web-search",
			name:        "Web Search",
			description: "Search the web",
		},
	}
}

func (s *WebSearchSkill) Execute(ctx context.Context, params map[string]any) (any, error) {
	// TODO: Implement web search
	return nil, nil
}

func (s *WebSearchSkill) Schema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"query": map[string]any{
				"type":        "string",
				"description": "The search query",
			},
		},
		"required": []string{"query"},
	}
}

// RegisterPureGoSkills registers all pure Go skills.
func RegisterPureGoSkills(registry *Registry) {
	registry.Register(NewBashSkill())
	registry.Register(NewWebSearchSkill())
	// Add more skills here
}

