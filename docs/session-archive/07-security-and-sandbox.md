# 安全与沙箱

## Security Policy

### workspaceOnly 策略
- 默认限制 agent 只能访问当前工作目录
- 防止文件系统越权操作

### Command Approval
- 高危命令（rm -rf、sudo 等）需要用户确认
- 白名单/黑名单机制

## Tool 安全

### Shell Tool
- 输入参数校验
- 命令注入防护
- 超时控制

### File Tool
- 路径遍历防护（`../` 检测）
- 读写权限检查

## 密钥管理

- API Key 存储在 `~/.highclaw/config.yaml`
- 日志中不输出密钥、token
- 配置文件权限建议 0600

## 沙箱模式

- 继承 ZeroClaw 的安全模型
- 默认 deny-by-default

## 相关参考
- `SECURITY.md`
- ZeroClaw `src/security/` 模块
