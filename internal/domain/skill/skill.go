// Package skill defines the skill domain.
package skill

import "context"

// Skill represents a tool/skill that the agent can use.
type Skill interface {
	// ID returns the skill ID (e.g., "apple-notes", "web-search").
	ID() string

	// Name returns the human-readable skill name.
	Name() string

	// Description returns the skill description.
	Description() string

	// Execute executes the skill with given parameters.
	Execute(ctx context.Context, params map[string]any) (any, error)

	// Schema returns the JSON schema for the skill parameters.
	Schema() map[string]any

	// Status returns the skill status.
	Status() Status
}

// Status represents the status of a skill.
type Status string

const (
	StatusEligible         Status = "eligible"          // Ready to use
	StatusMissingDeps      Status = "missing_deps"      // Missing dependencies
	StatusMissingAPIKey    Status = "missing_api_key"   // Missing API key
	StatusBlockedAllowlist Status = "blocked_allowlist" // Blocked by allowlist
	StatusDisabled         Status = "disabled"          // Manually disabled
)

// SkillInfo contains metadata about a skill.
type SkillInfo struct {
	ID          string
	Name        string
	Description string
	Icon        string
	Category    string // "productivity", "search", "media", "system", etc.
	
	// Dependencies
	RequiresNode bool
	NodePackages []string // npm packages required
	BinaryDeps   []string // system binaries required
	
	// API Keys
	RequiredEnvVars []string // e.g., ["GOOGLE_PLACES_API_KEY"]
	
	// Allowlist
	AllowlistOnly bool
	
	// Status
	Status Status
	Reason string // Why not eligible
}

// Registry manages all skills.
type Registry struct {
	skills map[string]Skill
}

// NewRegistry creates a new skill registry.
func NewRegistry() *Registry {
	return &Registry{
		skills: make(map[string]Skill),
	}
}

// Register registers a skill.
func (r *Registry) Register(skill Skill) {
	r.skills[skill.ID()] = skill
}

// Get returns a skill by ID.
func (r *Registry) Get(id string) (Skill, bool) {
	s, ok := r.skills[id]
	return s, ok
}

// All returns all registered skills.
func (r *Registry) All() []Skill {
	skills := make([]Skill, 0, len(r.skills))
	for _, s := range r.skills {
		skills = append(skills, s)
	}
	return skills
}

// GetByStatus returns skills filtered by status.
func (r *Registry) GetByStatus(status Status) []Skill {
	var filtered []Skill
	for _, s := range r.skills {
		if s.Status() == status {
			filtered = append(filtered, s)
		}
	}
	return filtered
}

// AllSkillsInfo returns metadata for all known skills (from OpenClaw).
var AllSkillsInfo = []SkillInfo{
	{
		ID:          "apple-notes",
		Name:        "Apple Notes",
		Icon:        "📝",
		Category:    "productivity",
		Description: "Read and write Apple Notes",
		RequiresNode: true,
		NodePackages: []string{"@highclaw/skill-apple-notes"},
		BinaryDeps:   []string{},
		RequiredEnvVars: []string{},
	},
	{
		ID:          "blogwatcher",
		Name:        "Blog Watcher",
		Icon:        "📰",
		Category:    "search",
		Description: "Monitor blogs and RSS feeds",
		RequiresNode: true,
		NodePackages: []string{"@highclaw/skill-blogwatcher"},
	},
	{
		ID:          "goplaces",
		Name:        "Google Places",
		Icon:        "📍",
		Category:    "search",
		Description: "Search Google Places",
		RequiresNode: true,
		NodePackages: []string{"@highclaw/skill-goplaces"},
		RequiredEnvVars: []string{"GOOGLE_PLACES_API_KEY"},
	},
	{
		ID:          "web-search",
		Name:        "Web Search",
		Icon:        "🔍",
		Category:    "search",
		Description: "Search the web",
		RequiresNode: false,
		BinaryDeps:   []string{},
	},
	{
		ID:          "bash",
		Name:        "Bash",
		Icon:        "💻",
		Category:    "system",
		Description: "Execute bash commands",
		RequiresNode: false,
		BinaryDeps:   []string{"bash"},
	},
	{
		ID:          "browser",
		Name:        "Browser",
		Icon:        "🌐",
		Category:    "web",
		Description: "Control web browser",
		RequiresNode: true,
		NodePackages: []string{"puppeteer"},
	},
	{
		ID:          "obsidian",
		Name:        "Obsidian",
		Icon:        "💎",
		Category:    "productivity",
		Description: "Read and write Obsidian notes",
		RequiresNode: true,
		NodePackages: []string{"@highclaw/skill-obsidian"},
	},
	{
		ID:          "openai-whisper",
		Name:        "OpenAI Whisper",
		Icon:        "🎙️",
		Category:    "media",
		Description: "Transcribe audio with Whisper",
		RequiresNode: true,
		NodePackages: []string{"@highclaw/skill-openai-whisper"},
		RequiredEnvVars: []string{"OPENAI_API_KEY"},
	},
}

