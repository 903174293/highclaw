# 日志管理系统（Log Management）

## 设计思路

- **文件日志为核心**：写到 `log/*.log` 文件，服务无法启动时也能直接查看
- 不依赖进程内缓冲或远程服务

## logger.Manager 实现

### 日志轮转
- **按大小**：超过 MaxSizeMB 时创建新文件
- **按天**：跨天时自动创建新文件

### 日志清理
- 超过 MaxAgeDays 的日志文件自动删除

### stderr 镜像
- StderrEnabled=true 时同时输出到终端，方便开发调试

## 配置项

| 配置 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| Dir | string | ~/.highclaw/logs | 日志目录 |
| Level | string | info | debug/info/warn/error |
| MaxAgeDays | int | 30 | 保留天数 |
| MaxSizeMB | int | 50 | 单文件上限 |
| StderrEnabled | bool | true | 是否输出到 stderr |

## CLI 命令

| 命令 | 说明 |
|------|------|
| `highclaw logs list` | 列出日志文件 |
| `highclaw logs tail` | 实时跟踪日志 |
| `highclaw logs search` | 搜索关键词 |
| `highclaw logs clean` | 清理过期日志 |

## 架构文档

详见 `docs/architecture-tasklog-logger.md`
