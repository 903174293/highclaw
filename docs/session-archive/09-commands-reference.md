# HighClaw CLI 命令参考

## 核心命令

| 命令 | 说明 |
|------|------|
| `highclaw` | 启动交互式 agent |
| `highclaw chat` | 直接进入聊天模式 |
| `highclaw onboard` | 交互式配置向导 |
| `highclaw config show` | 显示当前配置 |
| `highclaw version` | 显示版本信息 |

## Task 管理

| 命令 | 说明 |
|------|------|
| `highclaw tasks list` | 列出任务记录 |
| `highclaw tasks stats` | 任务统计 |
| `highclaw tasks clean` | 清理过期记录 |

## Memory 管理

| 命令 | 说明 |
|------|------|
| `highclaw memory list` | 列出记忆条目 |
| `highclaw memory add` | 添加记忆条目 |
| `highclaw memory search` | 搜索记忆内容 |
| `highclaw memory clear` | 清空记忆 |

## Log 管理

| 命令 | 说明 |
|------|------|
| `highclaw logs list` | 列出日志文件 |
| `highclaw logs tail` | 实时跟踪日志 |
| `highclaw logs search` | 搜索日志内容 |
| `highclaw logs clean` | 清理过期日志 |

## Skill 管理

| 命令 | 说明 |
|------|------|
| `highclaw skill list` | 列出已安装 Skill |
| `highclaw skill info <name>` | 查看 Skill 详情 |
| `highclaw skill install <path>` | 安装 Skill |
| `highclaw skill remove <name>` | 卸载 Skill |

## Session 管理

| 命令 | 说明 |
|------|------|
| `highclaw session list` | 列出历史 Session |
| `highclaw session show <id>` | 查看 Session 详情 |

## 通用参数

| 参数 | 说明 |
|------|------|
| `--limit N` | 分页条数 |
| `--offset N` | 分页偏移 |
| `--search KEYWORD` | 关键词搜索 |
| `--since DATE` | 起始日期 |
| `--until DATE` | 截止日期 |
| `--sort FIELD:ORDER` | 排序（如 created_at:desc） |

## ZeroClaw 对应命令

| ZeroClaw | HighClaw | 说明 |
|----------|----------|------|
| `zeroclaw` | `highclaw` | 启动 agent |
| `zeroclaw chat` | `highclaw chat` | 聊天模式 |
| `zeroclaw onboard` | `highclaw onboard` | 配置向导 |
