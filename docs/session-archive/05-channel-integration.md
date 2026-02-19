# Channel 集成

## 支持的 Channel

### 原有（继承 ZeroClaw/OpenClaw）
- **Telegram** — 最成熟的 channel
- **Discord** — 社区场景
- **Slack** — 企业协作

### HighClaw 新增
- **Feishu（飞书）** — 企业协作平台，基于 Bot API
- **WeCom（企业微信）** — 企业内部通讯
- **WeChat（微信）** — 公众号/个人号（via bridge）

## Channel 接口设计

```go
type Channel interface {
    Name() string
    Send(ctx context.Context, msg Message) error
    Listen(ctx context.Context, handler MessageHandler) error
    HealthCheck(ctx context.Context) error
    Close() error
}
```

## Feishu 实现要点

- 使用飞书 Open API
- Bot Webhook + Event Subscription
- 支持文本和富文本消息
- 支持 Typing Indicator

## WeCom 实现要点

- 企业微信 Server API
- 消息回调验证
- 支持应用消息推送

## WeChat 实现要点

- 通过 Bridge 模式接入
- 支持公众号和个人号两种模式
- 消息加解密

## Channel 模块回滚恢复

与 Skill 类似，Channel 集成代码在 Memory 回滚时丢失，后从对话历史中手动恢复。

## 本地化修正

Channel 描述和 onboard 向导中的中文文案全部替换为英文，保持与其他选项风格一致。
