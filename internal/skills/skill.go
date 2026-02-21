// Package skills 提供用户自定义 skill 管理功能（SKILL.md 格式）
package skills

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const (
	// OpenSkillsRepoURL 是 open-skills 社区仓库地址
	OpenSkillsRepoURL = "https://github.com/besoeasy/open-skills"
	// OpenSkillsSyncMarker 同步标记文件
	OpenSkillsSyncMarker = ".highclaw-open-skills-sync"
	// OpenSkillsSyncIntervalDays 同步间隔（天）
	OpenSkillsSyncIntervalDays = 7
)

// Skill 表示一个用户自定义 skill
type Skill struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Version     string      `json:"version"`
	Author      string      `json:"author,omitempty"`
	Tags        []string    `json:"tags,omitempty"`
	Tools       []SkillTool `json:"tools,omitempty"`
	Prompts     []string    `json:"prompts,omitempty"`
	Location    string      `json:"location,omitempty"`
	Source      string      `json:"source,omitempty"`
}

// SkillTool 表示 skill 定义的工具
type SkillTool struct {
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Kind        string            `json:"kind"`
	Command     string            `json:"command"`
	Args        map[string]string `json:"args,omitempty"`
}

// Manager 管理用户自定义 skills
type Manager struct {
	workspaceDir  string
	openSkillsDir string
}

// NewManager 创建 skill 管理器
func NewManager(workspaceDir string) *Manager {
	homeDir, _ := os.UserHomeDir()
	openSkillsDir := filepath.Join(homeDir, "open-skills")
	return &Manager{
		workspaceDir:  workspaceDir,
		openSkillsDir: openSkillsDir,
	}
}

// SkillsDir 返回 workspace 下的 skills 目录路径
func (m *Manager) SkillsDir() string {
	return filepath.Join(m.workspaceDir, "skills")
}

// LoadAll 加载所有 skills（open-skills + workspace skills）
func (m *Manager) LoadAll() []Skill {
	var skills []Skill
	if m.ensureOpenSkills() {
		skills = append(skills, m.loadOpenSkills()...)
	}
	skills = append(skills, m.loadWorkspaceSkills()...)
	sort.Slice(skills, func(i, j int) bool {
		return skills[i].Name < skills[j].Name
	})
	return skills
}

// loadWorkspaceSkills 加载 workspace 中的 skills
func (m *Manager) loadWorkspaceSkills() []Skill {
	skillsDir := m.SkillsDir()
	if _, err := os.Stat(skillsDir); os.IsNotExist(err) {
		return nil
	}
	return m.loadSkillsFromDir(skillsDir, "local")
}

// loadOpenSkills 加载 open-skills 仓库中的 skills
func (m *Manager) loadOpenSkills() []Skill {
	skillsSubdir := filepath.Join(m.openSkillsDir, "skills")
	scanDir := m.openSkillsDir
	if info, err := os.Stat(skillsSubdir); err == nil && info.IsDir() {
		scanDir = skillsSubdir
	}
	return m.loadSkillsFromDir(scanDir, "open-skills")
}

// loadSkillsFromDir 从目录加载 skills
func (m *Manager) loadSkillsFromDir(dir string, source string) []Skill {
	var skills []Skill
	entries, err := os.ReadDir(dir)
	if err != nil {
		return skills
	}
	for _, entry := range entries {
		skillDir := filepath.Join(dir, entry.Name())
		info, err := os.Stat(skillDir)
		if err != nil || !info.IsDir() {
			continue
		}
		skillMD := filepath.Join(skillDir, "SKILL.md")
		if _, err := os.Stat(skillMD); os.IsNotExist(err) {
			continue
		}
		skill, err := m.loadSkillFromMD(skillMD, entry.Name())
		if err != nil {
			continue
		}
		skill.Source = source
		if source == "open-skills" {
			skill.Version = "open-skills"
			skill.Author = "besoeasy/open-skills"
			skill.Tags = []string{"open-skills"}
		}
		skills = append(skills, skill)
	}
	return skills
}

// loadSkillFromMD 从 SKILL.md 文件加载 skill
func (m *Manager) loadSkillFromMD(path string, name string) (Skill, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return Skill{}, err
	}
	contentStr := string(content)
	return Skill{
		Name:        name,
		Description: extractDescription(contentStr),
		Version:     "0.1.0",
		Prompts:     []string{contentStr},
		Location:    path,
	}, nil
}

// extractDescription 从 Markdown 内容提取首行非标题文本作为描述
func extractDescription(content string) string {
	scanner := bufio.NewScanner(strings.NewReader(content))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "---") {
			continue
		}
		if len(line) > 100 {
			return line[:100] + "..."
		}
		return line
	}
	return "No description"
}

// ensureOpenSkills 确保 open-skills 仓库存在并按需更新
func (m *Manager) ensureOpenSkills() bool {
	if env := os.Getenv("HIGHCLAW_OPEN_SKILLS_ENABLED"); env != "" {
		val := strings.ToLower(strings.TrimSpace(env))
		if val == "0" || val == "false" || val == "off" || val == "no" {
			return false
		}
	}
	if _, err := os.Stat(m.openSkillsDir); os.IsNotExist(err) {
		return m.cloneOpenSkills()
	}
	if m.shouldSyncOpenSkills() {
		m.pullOpenSkills()
		m.markOpenSkillsSynced()
	}
	return true
}

// cloneOpenSkills 克隆 open-skills 仓库
func (m *Manager) cloneOpenSkills() bool {
	cmd := exec.Command("git", "clone", "--depth", "1", OpenSkillsRepoURL, m.openSkillsDir)
	if err := cmd.Run(); err != nil {
		return false
	}
	m.markOpenSkillsSynced()
	return true
}

// pullOpenSkills 更新 open-skills 仓库
func (m *Manager) pullOpenSkills() bool {
	gitDir := filepath.Join(m.openSkillsDir, ".git")
	if _, err := os.Stat(gitDir); os.IsNotExist(err) {
		return true
	}
	cmd := exec.Command("git", "-C", m.openSkillsDir, "pull", "--ff-only")
	return cmd.Run() == nil
}

// shouldSyncOpenSkills 检查是否需要同步
func (m *Manager) shouldSyncOpenSkills() bool {
	markerPath := filepath.Join(m.openSkillsDir, OpenSkillsSyncMarker)
	info, err := os.Stat(markerPath)
	if err != nil {
		return true
	}
	age := time.Since(info.ModTime())
	return age > time.Duration(OpenSkillsSyncIntervalDays)*24*time.Hour
}

// markOpenSkillsSynced 标记已同步
func (m *Manager) markOpenSkillsSynced() {
	markerPath := filepath.Join(m.openSkillsDir, OpenSkillsSyncMarker)
	_ = os.WriteFile(markerPath, []byte("synced"), 0644)
}

// Install 安装 skill（支持 Git URL 或本地路径）
func (m *Manager) Install(source string) error {
	skillsDir := m.SkillsDir()
	if err := os.MkdirAll(skillsDir, 0755); err != nil {
		return fmt.Errorf("create skills dir: %w", err)
	}
	if strings.HasPrefix(source, "https://") || strings.HasPrefix(source, "http://") {
		cmd := exec.Command("git", "clone", "--depth", "1", source)
		cmd.Dir = skillsDir
		output, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("git clone failed: %s", string(output))
		}
		return nil
	}
	srcPath, err := filepath.Abs(source)
	if err != nil {
		return fmt.Errorf("invalid path: %w", err)
	}
	if _, err := os.Stat(srcPath); os.IsNotExist(err) {
		return fmt.Errorf("source path does not exist: %s", source)
	}
	name := filepath.Base(srcPath)
	destPath := filepath.Join(skillsDir, name)
	if err := os.Symlink(srcPath, destPath); err != nil {
		return m.copyDir(srcPath, destPath)
	}
	return nil
}

// Remove 移除已安装的 skill
func (m *Manager) Remove(name string) error {
	if strings.Contains(name, "..") || strings.Contains(name, "/") || strings.Contains(name, "\\") {
		return fmt.Errorf("invalid skill name: %s", name)
	}
	skillPath := filepath.Join(m.SkillsDir(), name)
	absSkillPath, err := filepath.Abs(skillPath)
	if err != nil {
		return err
	}
	absSkillsDir, err := filepath.Abs(m.SkillsDir())
	if err != nil {
		return err
	}
	if !strings.HasPrefix(absSkillPath, absSkillsDir) {
		return fmt.Errorf("invalid skill path")
	}
	if _, err := os.Stat(skillPath); os.IsNotExist(err) {
		return fmt.Errorf("skill not found: %s", name)
	}
	info, err := os.Lstat(skillPath)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return os.Remove(skillPath)
	}
	return os.RemoveAll(skillPath)
}

// copyDir 复制目录
func (m *Manager) copyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		relPath, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		dstPath := filepath.Join(dst, relPath)
		if info.IsDir() {
			return os.MkdirAll(dstPath, info.Mode())
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(dstPath, data, info.Mode())
	})
}

// ToSystemPrompt 将 skills 转换为 agent system prompt 片段（仅摘要，按需加载完整内容）
func ToSystemPrompt(skills []Skill) string {
	if len(skills) == 0 {
		return ""
	}
	var sb strings.Builder
	sb.WriteString("\n## Available Skills\n\n")
	sb.WriteString("Skills are loaded on demand. Use `skill_read` tool with the skill name to get full instructions.\n\n")
	sb.WriteString("<available_skills>\n")
	for _, skill := range skills {
		sb.WriteString("  <skill>\n")
		sb.WriteString(fmt.Sprintf("    <name>%s</name>\n", skill.Name))
		sb.WriteString(fmt.Sprintf("    <description>%s</description>\n", skill.Description))
		if skill.Location != "" {
			sb.WriteString(fmt.Sprintf("    <location>%s</location>\n", skill.Location))
		}
		sb.WriteString("  </skill>\n")
	}
	sb.WriteString("</available_skills>\n\n")
	return sb.String()
}

// InitSkillsDir 初始化 skills 目录并写入 README
func (m *Manager) InitSkillsDir() error {
	skillsDir := m.SkillsDir()
	if err := os.MkdirAll(skillsDir, 0755); err != nil {
		return err
	}
	readmePath := filepath.Join(skillsDir, "README.md")
	if _, err := os.Stat(readmePath); os.IsNotExist(err) {
		content := "# Skills Directory\n\nPlace your custom skills here. Each skill should be in its own subdirectory with a SKILL.md file.\n\n" +
			"## Structure\n\n```\nskills/\n├── my-skill/\n│   └── SKILL.md\n└── another-skill/\n    └── SKILL.md\n```\n\n" +
			"## Installing Skills\n\n```bash\n# From GitHub\nhighclaw skills install https://github.com/user/my-skill\n\n# From local path\nhighclaw skills install /path/to/skill\n```\n\n" +
			"## Creating a Skill\n\nCreate a SKILL.md file with your skill's instructions and capabilities.\nThe content will be included in the agent's system prompt.\n"
		if err := os.WriteFile(readmePath, []byte(content), 0644); err != nil {
			return err
		}
	}
	return nil
}
