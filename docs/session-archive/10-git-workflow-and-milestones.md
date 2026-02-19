# Git 工作流与里程碑

## 提交规范

- **英文 commit message**（最终确认）
- 不包含 TicketNo、AR10AHT03、Description 前缀
- 不包含 Co-authored-by
- 使用 conventional commit 风格

## 关键里程碑

### v0.1.0 Tag
首个版本标签，标志 HighClaw 核心功能基本完成。

### 功能时间线

1. **初始移植**：从 ZeroClaw/OpenClaw 移植核心 agent 功能
2. **Task Log 系统**：SQLite + FTS5 实现
3. **Log 管理**：文件日志 + 轮转 + 清理
4. **Memory 增强**：分页、搜索、排序、日期过滤
5. **Memory 回滚**：放弃 embedding 方案，回归 Markdown
6. **Skill 系统**：TOML + Markdown 技能包
7. **Channel 集成**：新增飞书/企业微信/微信
8. **文档体系**：README、架构图、系统功能指南
9. **Web 控制台（已移除）**：经历 React -> 纯 Go HTML -> 完全移除
10. **本地化修正**：Channel 描述统一英文

## 回滚事件总结

### Memory 回滚
- **原因**：尝试对齐 OpenClaw 高级 memory 特性，评估为 overweight
- **方式**：`git checkout` 回退相关文件
- **副作用**：Skill 和 Channel 代码意外丢失
- **修复**：从对话历史中恢复代码

### Web 控制台移除
- **原因**：用户决定 HighClaw 定位为纯 Go 高性能代码，不需要 Web UI
- **经历**：React/TS -> 纯 Go embedded HTML -> 完全移除
- **清理**：删除 admin 包、CLI 命令、配置字段

## 重要文件

| 文件 | 说明 |
|------|------|
| `MILESTONE_TEST_REPORT.md` | 全功能回归测试报告 |
| `CODE_OF_CONDUCT.md` | 行为准则 |
| `CONTRIBUTING.md` | 贡献指南 |
| `LICENSE` | 开源协议 |
| `SECURITY.md` | 安全政策 |
