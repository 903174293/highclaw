// Package skill provides skill management services.
package skill

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"

	"github.com/highclaw/highclaw/internal/config"
	"github.com/highclaw/highclaw/internal/domain/skill"
)

// Manager manages all skills.
type Manager struct {
	config   *config.Config
	logger   *slog.Logger
	registry *skill.Registry
}

// NewManager creates a new skill manager.
func NewManager(cfg *config.Config, logger *slog.Logger) *Manager {
	return &Manager{
		config:   cfg,
		logger:   logger,
		registry: skill.NewRegistry(),
	}
}

// DiscoverSkills discovers all available skills.
func (m *Manager) DiscoverSkills(ctx context.Context) ([]skill.SkillInfo, error) {
	m.logger.Info("discovering skills")

	var skills []skill.SkillInfo

	for _, info := range skill.AllSkillsInfo {
		// Check status
		status, reason := m.checkSkillStatus(info)
		info.Status = status
		info.Reason = reason

		skills = append(skills, info)
	}

	m.logger.Info("skills discovered", "total", len(skills))

	return skills, nil
}

// checkSkillStatus checks if a skill is eligible to use.
func (m *Manager) checkSkillStatus(info skill.SkillInfo) (skill.Status, string) {
	// Check allowlist
	if info.AllowlistOnly {
		if !contains(m.config.Agent.Sandbox.Allow, info.ID) {
			return skill.StatusBlockedAllowlist, "not in allowlist"
		}
	}
	if len(m.config.Agent.Sandbox.Deny) > 0 && contains(m.config.Agent.Sandbox.Deny, info.ID) {
		return skill.StatusDisabled, "disabled in sandbox denylist"
	}

	// Check binary dependencies
	for _, binary := range info.BinaryDeps {
		if !m.isBinaryAvailable(binary) {
			return skill.StatusMissingDeps, fmt.Sprintf("missing binary: %s", binary)
		}
	}

	// Check Node.js dependencies
	if info.RequiresNode && len(info.NodePackages) > 0 {
		for _, pkg := range info.NodePackages {
			if !m.isNodePackageInstalled(pkg) {
				return skill.StatusMissingDeps, fmt.Sprintf("missing npm package: %s", pkg)
			}
		}
	}

	// Check API keys
	for _, envVar := range info.RequiredEnvVars {
		if os.Getenv(envVar) == "" {
			return skill.StatusMissingAPIKey, fmt.Sprintf("missing env var: %s", envVar)
		}
	}

	return skill.StatusEligible, ""
}

// isBinaryAvailable checks if a binary is available in PATH.
func (m *Manager) isBinaryAvailable(binary string) bool {
	_, err := exec.LookPath(binary)
	return err == nil
}

// isNodePackageInstalled checks if a Node.js package is installed.
func (m *Manager) isNodePackageInstalled(pkg string) bool {
	// Try to find the package in node_modules
	// This is a simplified check
	cmd := exec.Command("npm", "list", pkg, "--depth=0")
	err := cmd.Run()
	return err == nil
}

// GetSkillsSummary returns a summary of skills by status.
func (m *Manager) GetSkillsSummary(ctx context.Context) (map[string]int, error) {
	skills, err := m.DiscoverSkills(ctx)
	if err != nil {
		return nil, err
	}

	summary := map[string]int{
		"eligible":          0,
		"missing_deps":      0,
		"missing_api_key":   0,
		"blocked_allowlist": 0,
		"disabled":          0,
	}

	for _, s := range skills {
		switch s.Status {
		case skill.StatusEligible:
			summary["eligible"]++
		case skill.StatusMissingDeps:
			summary["missing_deps"]++
		case skill.StatusMissingAPIKey:
			summary["missing_api_key"]++
		case skill.StatusBlockedAllowlist:
			summary["blocked_allowlist"]++
		case skill.StatusDisabled:
			summary["disabled"]++
		}
	}

	return summary, nil
}

// InstallMissingDependencies installs missing Node.js dependencies.
func (m *Manager) InstallMissingDependencies(ctx context.Context, nodeManager string, skillIDs []string) error {
	m.logger.Info("installing missing dependencies", "nodeManager", nodeManager, "skills", skillIDs)

	skills, err := m.DiscoverSkills(ctx)
	if err != nil {
		return err
	}

	// Collect packages to install
	var packages []string
	for _, s := range skills {
		// Skip if not in requested list
		if len(skillIDs) > 0 && !contains(skillIDs, s.ID) {
			continue
		}

		if s.Status == skill.StatusMissingDeps && s.RequiresNode {
			packages = append(packages, s.NodePackages...)
		}
	}

	if len(packages) == 0 {
		m.logger.Info("no packages to install")
		return nil
	}

	// Install packages
	return m.installNodePackages(nodeManager, packages)
}

// installNodePackages installs Node.js packages using the specified manager.
func (m *Manager) installNodePackages(manager string, packages []string) error {
	m.logger.Info("installing node packages", "manager", manager, "packages", packages)

	var cmd *exec.Cmd
	switch manager {
	case "npm":
		args := append([]string{"install", "-g"}, packages...)
		cmd = exec.Command("npm", args...)
	case "pnpm":
		args := append([]string{"add", "-g"}, packages...)
		cmd = exec.Command("pnpm", args...)
	case "bun":
		args := append([]string{"add", "-g"}, packages...)
		cmd = exec.Command("bun", args...)
	default:
		return fmt.Errorf("unsupported node manager: %s", manager)
	}

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

// contains checks if a slice contains a string.
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
