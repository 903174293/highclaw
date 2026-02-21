# HighClaw Scripts

本目录包含 HighClaw 项目的实用脚本。

## fingerprint-check.sh

代码指纹自动查重脚本，用于监控代码是否被未授权复制。

### 功能

- 自动搜索 GitHub、Sourcegraph 等平台
- 支持多个指纹同时监控
- 发现可疑复制时发送邮件通知
- 支持定时任务（cron）

### 使用前配置

1. 编辑脚本顶部的配置区域：

```bash
# 从 .fingerprint.private 获取你的指纹
FINGERPRINTS=(
    "your-fingerprint-1"
    "your-fingerprint-2"
)

# 填入你的仓库（用于排除）
MY_REPOS=(
    "your-username/highclaw"
)

# 填入通知邮箱
NOTIFY_EMAIL="your-email@example.com"
```

2. （可选）配置 GitHub Token 提高 API 限额：
   - 访问 https://github.com/settings/tokens
   - 生成一个 Token（无需特殊权限）
   - 填入 `GITHUB_TOKEN`

### 运行

```bash
# 测试配置
./fingerprint-check.sh --test

# 运行查重
./fingerprint-check.sh

# 详细输出
./fingerprint-check.sh --verbose
```

### 定时任务

```bash
crontab -e

# 每周一早上 9 点运行
0 9 * * 1 /path/to/scripts/fingerprint-check.sh --cron
```

### 邮件配置

推荐使用 `msmtp` 发送邮件：

```bash
# 安装
brew install msmtp        # macOS
sudo apt install msmtp    # Ubuntu

# 配置 ~/.msmtprc（参考 msmtprc.template）
```

### 日志

日志保存在 `~/.highclaw/logs/` 目录。
