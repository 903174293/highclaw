# Skill 系统

## 概述

Skill 系统让 HighClaw agent 可以加载、列出和执行预定义的技能包（Markdown + TOML 配置），扩展 agent 的能力。

## Skill 结构

每个 Skill 是 `~/.highclaw/skills/<name>/` 目录，包含：
- `skill.toml`：元数据（name、version、description、author、tags）
- `prompt.md`：Skill 的 system prompt 扩展
- 可选附加文件

## CLI 命令

| 命令 | 说明 |
|------|------|
| `highclaw skill list` | 列出所有已安装 Skill |
| `highclaw skill info <name>` | 显示 Skill 详情 |
| `highclaw skill install <path>` | 安装 Skill |
| `highclaw skill remove <name>` | 卸载 Skill |

## 实现要点

- TOML 解析使用 `github.com/BurntSushi/toml`
- Skill 在 agent 启动时加载进上下文
- 不修改 agent 核心逻辑，通过 prompt injection 扩展能力

## 回滚恢复

Skill 功能在 Memory 回滚时意外丢失（因 Git 回滚范围涉及相关代码）。后通过从对话历史中恢复代码完成重建，确保不影响已有逻辑。

## 相关文档
- `docs/skill-context.md`
