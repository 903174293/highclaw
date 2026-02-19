# Session 管理

## 概述

HighClaw 的 session 管理负责隔离不同对话上下文，保证多轮对话的连贯性。

## Session 生命周期

1. **创建**：用户发起新对话时，自动分配 SessionID
2. **持续**：在同一 session 内，所有 memory 和 task record 关联该 SessionID
3. **结束**：用户退出或超时后 session 关闭

## 存储

- SessionID 记录在 TaskRecord 中
- Memory 按 session 隔离（可选）
- 日志中标记 session 上下文

## CLI 相关

| 命令 | 说明 |
|------|------|
| `highclaw session list` | 列出历史 session |
| `highclaw session show <id>` | 查看 session 详情 |

## 回归测试

在 MILESTONE_TEST_REPORT.md 中验证了 session 的隔离性和持久性。
