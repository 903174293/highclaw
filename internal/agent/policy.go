package agent

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/highclaw/highclaw/internal/config"
)

type SecurityPolicy struct {
	autonomy                 string
	allowed                  map[string]struct{}
	workspaceOnly            bool
	requireApprovalForMedium bool
	blockHighRisk            bool
}

func NewSecurityPolicy(cfg *config.Config) *SecurityPolicy {
	allowed := map[string]struct{}{}
	// Mirror ZeroClaw defaults: 允许常用的文件和开发命令
	list := []string{
		// 基础文件操作
		"ls", "cat", "grep", "find", "echo", "pwd", "wc", "head", "tail",
		"mkdir", "touch", "cp", "mv", "rm", "chmod", "chown", "stat",
		// 开发工具
		"git", "npm", "cargo", "go", "python", "python3", "pip", "pip3",
		"node", "yarn", "pnpm", "make", "cmake",
		// 文本处理
		"sort", "uniq", "awk", "sed", "cut", "tr", "tee", "xargs",
		// 系统信息
		"date", "which", "whoami", "uname", "env", "printenv",
		// 压缩解压
		"tar", "zip", "unzip", "gzip", "gunzip",
		// 其他常用
		"cd", "basename", "dirname", "realpath", "du", "df",
	}
	// Merge user-defined entries instead of replacing baseline, because some
	// existing configs store tool ids here (e.g. "bash"), not OS command names.
	list = append(list, cfg.Agent.Sandbox.Allow...)
	for _, cmd := range list {
		cmd = strings.TrimSpace(cmd)
		if cmd != "" {
			allowed[cmd] = struct{}{}
		}
	}
	autonomy := strings.ToLower(strings.TrimSpace(cfg.Autonomy.Level))
	if autonomy == "" {
		autonomy = "supervised"
	}
	// 与 ZeroClaw 保持一致：默认允许访问绝对路径，但高风险操作需要 approval
	// workspaceOnly 可以通过配置 sandbox.mode = "workspace-only" 启用
	workspaceOnly := strings.ToLower(strings.TrimSpace(cfg.Agent.Sandbox.Mode)) == "workspace-only"
	return &SecurityPolicy{
		autonomy:                 autonomy,
		allowed:                  allowed,
		workspaceOnly:            workspaceOnly,
		requireApprovalForMedium: true,
		blockHighRisk:            true,
	}
}

func (p *SecurityPolicy) ValidateBashInput(inputJSON string) error {
	var in struct {
		Command  string `json:"command"`
		Approved bool   `json:"approved"`
	}
	if err := json.Unmarshal([]byte(inputJSON), &in); err != nil {
		return fmt.Errorf("invalid bash input json: %w", err)
	}
	cmd := strings.TrimSpace(in.Command)
	if cmd == "" {
		return fmt.Errorf("command is required")
	}

	normalized := strings.ToLower(cmd)
	if strings.Contains(normalized, "`") || strings.Contains(normalized, "$(") || strings.Contains(normalized, "${") {
		return fmt.Errorf("command blocked by policy")
	}

	hasCommand := false
	highestRisk := "low"
	for _, seg := range splitCommandSegments(cmd) {
		risk := commandRiskLevel(seg)
		if risk == "high" {
			highestRisk = "high"
		} else if risk == "medium" && highestRisk != "high" {
			highestRisk = "medium"
		}

		base := baseCommand(seg)
		if base == "" {
			continue
		}
		hasCommand = true
		if _, ok := p.allowed[base]; !ok {
			return fmt.Errorf("command not allowed by policy: %s", base)
		}
	}
	if !hasCommand {
		return fmt.Errorf("command is required")
	}

	if p.workspaceOnly && hasAbsolutePath(cmd) {
		return fmt.Errorf("command blocked: absolute paths are disallowed in workspace-only mode")
	}

	if p.blockHighRisk && highestRisk == "high" {
		return fmt.Errorf("Command blocked: high-risk command is disallowed by policy")
	}
	if p.autonomy == "supervised" && p.requireApprovalForMedium && highestRisk == "medium" && !in.Approved {
		return fmt.Errorf("Command requires explicit approval (approved=true): medium-risk operation")
	}
	if p.autonomy == "readonly" {
		return fmt.Errorf("command execution is disabled in read-only mode")
	}

	return nil
}

func hasAbsolutePath(command string) bool {
	for _, token := range strings.Fields(command) {
		token = strings.Trim(token, "\"'")
		if token == "" {
			continue
		}
		if strings.HasPrefix(token, "~/") {
			return true
		}
		if filepath.IsAbs(token) {
			return true
		}
	}
	return false
}

func commandRiskLevel(segment string) string {
	base := strings.ToLower(baseCommand(segment))
	lowered := strings.ToLower(segment)

	highRisk := map[string]struct{}{
		"rm": {}, "mkfs": {}, "dd": {}, "shutdown": {}, "reboot": {}, "halt": {}, "poweroff": {},
		"sudo": {}, "su": {}, "chown": {}, "chmod": {}, "useradd": {}, "userdel": {}, "usermod": {},
		"passwd": {}, "mount": {}, "umount": {}, "iptables": {}, "ufw": {}, "firewall-cmd": {},
		"curl": {}, "wget": {}, "nc": {}, "ncat": {}, "netcat": {}, "scp": {}, "ssh": {}, "ftp": {}, "telnet": {},
	}
	if _, ok := highRisk[base]; ok {
		return "high"
	}
	if strings.Contains(lowered, "rm -rf /") || strings.Contains(lowered, "rm -fr /") || strings.Contains(lowered, ":(){:|:&};:") {
		return "high"
	}

	parts := strings.Fields(lowered)
	if len(parts) > 1 {
		switch base {
		case "git":
			medium := map[string]struct{}{
				"commit": {}, "push": {}, "reset": {}, "clean": {}, "rebase": {}, "merge": {},
				"cherry-pick": {}, "revert": {}, "branch": {}, "checkout": {},
			}
			if _, ok := medium[parts[1]]; ok {
				return "medium"
			}
		case "npm":
			if parts[1] == "publish" {
				return "medium"
			}
		case "cargo":
			if parts[1] == "publish" {
				return "medium"
			}
		}
	}
	return "low"
}

func splitCommandSegments(command string) []string {
	normalized := command
	for _, sep := range []string{"&&", "||", "\n", ";", "|"} {
		normalized = strings.ReplaceAll(normalized, sep, "\x00")
	}
	parts := strings.Split(normalized, "\x00")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if s := strings.TrimSpace(p); s != "" {
			out = append(out, s)
		}
	}
	return out
}

func baseCommand(segment string) string {
	fields := strings.Fields(segment)
	if len(fields) == 0 {
		return ""
	}
	first := fields[0]
	// Skip env assignment: FOO=bar cmd
	if strings.Contains(first, "=") && len(fields) > 1 {
		first = fields[1]
	}
	if i := strings.LastIndex(first, "/"); i >= 0 {
		first = first[i+1:]
	}
	return strings.TrimSpace(first)
}
