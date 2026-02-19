# 任务日志系统（Task Log System）

## 设计目标

记录 HighClaw 运行过程中所有用户执行动作（CRUD、聊天、请求/响应、操作时间、会话信息），用于调试、审计和统计。

## 技术选型：SQLite + FTS5

- SQLite 嵌入式数据库，单文件存储，零外部依赖
- FTS5 全文搜索扩展，无需 Elasticsearch
- Trigger 自动同步 FTS5 索引

## TaskRecord 数据结构

核心字段：ID、TaskID、SessionID、Action、Module、Status、RequestBody、ResponseBody、Duration、CreatedAt

## 查询能力

### 分页
- `--limit` + `--offset`，默认 limit 上限防止 OOM

### 日期范围
- `--since` 和 `--until`，按 CreatedAt 筛选

### 排序
- `--sort` 参数，如 `created_at:desc`、`duration:asc`
- **白名单 map 防 SQL 注入**：

```go
allowedSortCols := map[string]bool{
    "id": true, "created_at": true, "duration": true,
    "action": true, "status": true, "module": true,
}
if p.SortBy != "" && allowedSortCols[p.SortBy] {
    sortCol = p.SortBy
}
```

### 全文搜索
- `--search` 参数，基于 FTS5 对 RequestBody、ResponseBody 等字段检索

## CLI 命令

| 命令 | 说明 |
|------|------|
| `highclaw tasks list` | 列出任务记录（支持分页、排序、日期、搜索） |
| `highclaw tasks stats` | 统计任务数量、成功率、平均耗时 |
| `highclaw tasks clean` | 清理过期记录 |

## 性能分析

- SQLite 单次写入：< 1ms
- LLM 单次调用：数百毫秒到数秒
- **结论：LLM 延迟远大于 SQLite 写入，对主业务影响可忽略**
