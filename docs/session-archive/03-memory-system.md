# Memory 系统

## 架构

HighClaw 的 Memory 系统基于 Markdown 文件存储，保持与 ZeroClaw/OpenClaw 兼容的最小方案。

### 存储层
- 单个 Markdown 文件 `~/.highclaw/memory/memory.md`
- 分段管理：用 `## Section` 分隔不同主题
- 条目格式：`- [content] (timestamp)`

### 查询增强
- **分页**：`--limit` + `--offset`
- **搜索**：`--search` 关键词匹配（基于正则匹配）
- **日期过滤**：`--since` / `--until`
- **排序**：`--sort` 参数（如 `created_at:desc`）

### 排序白名单
最初 Memory 排序使用固定 `createdAt` 字段，后统一为与 TaskLog 相同的 map 白名单方式：

```go
allowedSortCols := map[string]bool{
    "created_at": true,
    "content":    true,
}
```

## 与 ZeroClaw 对比

### 相同点
- 基于 Markdown 文件的简单存储
- session 隔离

### HighClaw 增强
- 分页/搜索/排序/日期过滤（CLI 层面）
- 轻量实现，无额外依赖

## Memory 回滚事件

开发中曾尝试对齐 OpenClaw 的高级 memory 特性（embedding、向量搜索等），后评估为 overweight，用户明确要求回退到简单 Markdown 方案。使用 `git checkout` 完成回滚。

## 相关文档
- `docs/system-features-guide.md`
- `images/memory-architecture.svg`
