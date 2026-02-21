#!/bin/bash
# HighClaw 代码指纹自动查重脚本
# 创建日期: 2026-02-19
# 用法: ./fingerprint-check.sh [--verbose] [--test] [--cron]

set -e

# ================================
# 配置区域（使用前请修改）
# ================================

# 指纹列表（替换为你的实际指纹）
# 从 .fingerprint.private 文件中获取
FINGERPRINTS=(
    "YOUR_FINGERPRINT_1"
    "YOUR_FINGERPRINT_2"
    "YOUR_FINGERPRINT_3"
)

# 你的 GitHub 用户名/仓库名（用于排除自己）
MY_REPOS=(
    "your-username/highclaw"
    "your-username/other-repo"
)

# 日志配置
LOG_DIR="$HOME/.highclaw/logs"
LOG_FILE="$LOG_DIR/fingerprint-check-$(date +%Y%m%d).log"
RESULT_FILE="$LOG_DIR/last-check-result.json"

# 邮件通知配置（替换为你的邮箱）
NOTIFY_EMAIL="your-email@example.com"
EMAIL_SUBJECT_PREFIX="[HighClaw指纹监控]"

# GitHub Token（可选，提高 API 限额）
# 获取方式: https://github.com/settings/tokens
GITHUB_TOKEN=""

# ================================
# 颜色输出
# ================================
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m'

# ================================
# 全局变量
# ================================
VERBOSE=false
CRON_MODE=false
TEST_MODE=false
FOUND_ISSUES=0
RESULTS=()

# ================================
# 函数定义
# ================================

log() {
    local msg="[$(date '+%Y-%m-%d %H:%M:%S')] $1"
    if [ "$CRON_MODE" = false ] || [ "$VERBOSE" = true ]; then
        echo -e "$msg"
    fi
    echo "$(echo -e "$msg" | sed 's/\x1b\[[0-9;]*m//g')" >> "$LOG_FILE"
}

log_success() { log "${GREEN}✓ $1${NC}"; }
log_warning() { log "${YELLOW}⚠ $1${NC}"; }
log_error() { log "${RED}✗ $1${NC}"; }
log_info() { log "${BLUE}ℹ $1${NC}"; }
log_debug() { 
    if [ "$VERBOSE" = true ]; then
        local msg="[$(date '+%Y-%m-%d %H:%M:%S')] ${CYAN}[DEBUG] $1${NC}"
        echo -e "$msg" >&2
    fi
}

# 检查依赖
check_dependencies() {
    local missing=()
    
    for cmd in curl jq; do
        if ! command -v $cmd &> /dev/null; then
            missing+=($cmd)
        fi
    done
    
    if [ ${#missing[@]} -ne 0 ]; then
        log_error "缺少依赖: ${missing[*]}"
        log_info "macOS 安装: brew install ${missing[*]}"
        log_info "Ubuntu 安装: sudo apt install ${missing[*]}"
        exit 1
    fi
}

# GitHub 代码搜索
search_github() {
    local query="$1"
    local exclude=""
    
    # 构建排除条件
    for repo in "${MY_REPOS[@]}"; do
        exclude="$exclude+-repo:$repo"
    done
    
    # URL 编码
    local encoded_query=$(python3 -c "import urllib.parse; print(urllib.parse.quote('\"$query\"'))" 2>/dev/null || echo "$query")
    local url="https://api.github.com/search/code?q=${encoded_query}${exclude}"
    
    log_debug "GitHub API URL: $url"
    
    # 构建请求头
    local headers=(-H "Accept: application/vnd.github.v3+json")
    if [ -n "$GITHUB_TOKEN" ]; then
        headers+=(-H "Authorization: token $GITHUB_TOKEN")
    fi
    
    # 发起请求
    local response=$(curl -s "${headers[@]}" "$url" 2>/dev/null)
    
    if [ -z "$response" ]; then
        log_warning "GitHub API 请求失败"
        echo "-1"
        return
    fi
    
    # 检查错误
    local error_msg=$(echo "$response" | jq -r '.message // empty' 2>/dev/null)
    if [ -n "$error_msg" ]; then
        log_debug "GitHub API 错误: $error_msg"
        echo "-1"
        return
    fi
    
    local count=$(echo "$response" | jq -r '.total_count // 0' 2>/dev/null)
    local items=$(echo "$response" | jq -r '.items // []' 2>/dev/null)
    
    # 保存详细结果
    if [ "$count" != "0" ] && [ "$count" != "-1" ]; then
        echo "$response" | jq -r '.items[] | "  → \(.repository.full_name): \(.path)"' 2>/dev/null | head -5 >> "$LOG_FILE"
    fi
    
    echo "$count"
}

# Sourcegraph 搜索（简化版）
search_sourcegraph() {
    local query="$1"
    
    # Sourcegraph GraphQL API
    local response=$(curl -s "https://sourcegraph.com/.api/search/stream?q=$query&patternType=literal" 2>/dev/null | head -20)
    
    if echo "$response" | grep -q '"results"'; then
        echo "possible"
    else
        echo "none"
    fi
}

# Google 搜索提示（无法自动化，仅提供链接）
generate_google_links() {
    local query="$1"
    local encoded=$(python3 -c "import urllib.parse; print(urllib.parse.quote('\"$query\"'))" 2>/dev/null || echo "$query")
    
    echo "https://www.google.com/search?q=${encoded}+site:github.com"
    echo "https://www.google.com/search?q=${encoded}+site:gitlab.com"
    echo "https://www.google.com/search?q=${encoded}+site:gitee.com"
}

# 发送邮件通知
send_email() {
    local subject="$1"
    local body="$2"
    
    if [ -z "$NOTIFY_EMAIL" ] || [ "$NOTIFY_EMAIL" = "your-email@example.com" ]; then
        log_debug "未配置邮箱，跳过邮件通知"
        return
    fi
    
    log_info "发送邮件通知到: $NOTIFY_EMAIL"
    
    # 方法 1: 使用 mail 命令（需要配置 MTA）
    if command -v mail &> /dev/null; then
        echo -e "$body" | mail -s "$EMAIL_SUBJECT_PREFIX $subject" "$NOTIFY_EMAIL" 2>/dev/null && {
            log_success "邮件发送成功 (mail)"
            return 0
        }
    fi
    
    # 方法 2: 使用 sendmail
    if command -v sendmail &> /dev/null; then
        {
            echo "To: $NOTIFY_EMAIL"
            echo "Subject: $EMAIL_SUBJECT_PREFIX $subject"
            echo "Content-Type: text/plain; charset=utf-8"
            echo ""
            echo -e "$body"
        } | sendmail -t 2>/dev/null && {
            log_success "邮件发送成功 (sendmail)"
            return 0
        }
    fi
    
    # 方法 3: 使用 msmtp（推荐，支持 QQ 邮箱）
    if command -v msmtp &> /dev/null; then
        {
            echo "To: $NOTIFY_EMAIL"
            echo "Subject: $EMAIL_SUBJECT_PREFIX $subject"
            echo "Content-Type: text/plain; charset=utf-8"
            echo ""
            echo -e "$body"
        } | msmtp "$NOTIFY_EMAIL" 2>/dev/null && {
            log_success "邮件发送成功 (msmtp)"
            return 0
        }
    fi
    
    # 如果以上都不可用，保存到文件等待手动发送
    local email_file="$LOG_DIR/pending-email-$(date +%Y%m%d%H%M%S).txt"
    {
        echo "To: $NOTIFY_EMAIL"
        echo "Subject: $EMAIL_SUBJECT_PREFIX $subject"
        echo "Date: $(date)"
        echo ""
        echo -e "$body"
    } > "$email_file"
    
    log_warning "无法自动发送邮件，已保存到: $email_file"
    log_info "请手动发送或配置 msmtp"
    
    return 1
}

# macOS 系统通知
send_macos_notification() {
    local title="$1"
    local body="$2"
    
    if command -v osascript &> /dev/null; then
        osascript -e "display notification \"$body\" with title \"$title\" sound name \"Glass\"" 2>/dev/null || true
    fi
}

# 生成检查报告
generate_report() {
    local status="$1"
    local details="$2"
    
    cat << EOF
================================================================================
                    HighClaw 代码指纹查重报告
================================================================================

检查时间: $(date '+%Y-%m-%d %H:%M:%S')
检查状态: $status
检查指纹数: ${#FINGERPRINTS[@]}
发现问题数: $FOUND_ISSUES

--------------------------------------------------------------------------------
详细结果:
--------------------------------------------------------------------------------
$details

--------------------------------------------------------------------------------
搜索链接（手动验证）:
--------------------------------------------------------------------------------
EOF

    for fp in "${FINGERPRINTS[@]}"; do
        echo ""
        echo "指纹: $fp"
        generate_google_links "$fp" | while read link; do
            echo "  $link"
        done
    done
    
    cat << EOF

--------------------------------------------------------------------------------
后续操作建议:
--------------------------------------------------------------------------------
1. 如果发现可疑结果，请手动访问上述链接确认
2. 确认侵权后，截图保存证据
3. 联系侵权方要求删除或署名
4. 如不配合，提交 DMCA: https://github.com/contact/dmca-takedown

================================================================================
EOF
}

# 主要查重逻辑
run_check() {
    local details=""
    
    # 检查配置
    if [ "${FINGERPRINTS[0]}" = "YOUR_FINGERPRINT_1" ]; then
        log_error "请先配置 FINGERPRINTS 数组！"
        log_info "参考 .fingerprint.private 文件获取指纹"
        exit 1
    fi
    
    log_info "=========================================="
    log_info "HighClaw 代码指纹查重开始"
    log_info "检查时间: $(date '+%Y-%m-%d %H:%M:%S')"
    log_info "=========================================="
    echo ""
    
    for fp in "${FINGERPRINTS[@]}"; do
        log_info "搜索指纹: $fp"
        details+="指纹: $fp\n"
        
        # GitHub 搜索
        log "  → GitHub 搜索中..."
        local gh_count=$(search_github "$fp")
        
        if [ "$gh_count" = "-1" ] || [ -z "$gh_count" ]; then
            log_warning "  GitHub: API 需要认证或请求失败"
            details+="  GitHub: API 需要认证\n"
        elif [ "$gh_count" = "0" ]; then
            log_success "  GitHub: 未发现异常 ✓"
            details+="  GitHub: 未发现异常\n"
        else
            # 确保是正整数
            if [[ "$gh_count" =~ ^[0-9]+$ ]] && [ "$gh_count" -gt 0 ]; then
                log_error "  GitHub: 发现 $gh_count 个可疑结果!"
                details+="  GitHub: ⚠️ 发现 $gh_count 个可疑结果!\n"
                RESULTS+=("GitHub 发现指纹 '$fp' 共 $gh_count 个结果")
                FOUND_ISSUES=$((FOUND_ISSUES + 1))
            else
                log_warning "  GitHub: API 返回异常"
                details+="  GitHub: API 返回异常\n"
            fi
        fi
        
        # Sourcegraph 搜索
        log "  → Sourcegraph 搜索中..."
        local sg_result=$(search_sourcegraph "$fp")
        
        if [ "$sg_result" = "possible" ]; then
            log_warning "  Sourcegraph: 可能有结果，建议手动确认"
            details+="  Sourcegraph: 可能有结果\n"
        else
            log_success "  Sourcegraph: 未发现异常 ✓"
            details+="  Sourcegraph: 未发现异常\n"
        fi
        
        details+="\n"
        echo ""
        
        # 避免 API 限流
        sleep 3
    done
    
    # 生成报告
    local status="正常"
    if [ $FOUND_ISSUES -gt 0 ]; then
        status="⚠️ 发现 $FOUND_ISSUES 个可疑情况"
    fi
    
    local report=$(generate_report "$status" "$details")
    
    # 保存报告到日志
    echo "$report" >> "$LOG_FILE"
    
    # 保存结果 JSON
    cat > "$RESULT_FILE" << EOF
{
    "check_time": "$(date -u +%Y-%m-%dT%H:%M:%SZ)",
    "status": "$status",
    "found_issues": $FOUND_ISSUES,
    "fingerprints_checked": ${#FINGERPRINTS[@]},
    "results": $(printf '%s\n' "${RESULTS[@]}" | jq -R . | jq -s .)
}
EOF
    
    # 输出结果
    echo ""
    log_info "=========================================="
    if [ $FOUND_ISSUES -eq 0 ]; then
        log_success "查重完成，未发现侵权行为 ✓"
    else
        log_error "发现 $FOUND_ISSUES 个可疑情况!"
        for r in "${RESULTS[@]}"; do
            log_error "  - $r"
        done
        
        # 发送通知
        send_email "发现可疑代码复制" "$report"
        send_macos_notification "HighClaw 指纹监控" "发现 $FOUND_ISSUES 个可疑复制!"
    fi
    log_info "=========================================="
    log_info "日志文件: $LOG_FILE"
    log_info "结果文件: $RESULT_FILE"
    
    return $FOUND_ISSUES
}

# 测试模式
run_test() {
    log_info "=========================================="
    log_info "测试模式 - 检查脚本配置"
    log_info "=========================================="
    echo ""
    
    # 检查依赖
    log_info "检查依赖..."
    check_dependencies
    log_success "依赖检查通过"
    echo ""
    
    # 检查配置
    log_info "检查配置..."
    if [ "${FINGERPRINTS[0]}" = "YOUR_FINGERPRINT_1" ]; then
        log_warning "指纹未配置（使用默认占位符）"
    else
        log_success "指纹已配置: ${#FINGERPRINTS[@]} 个"
    fi
    log "  排除仓库: ${MY_REPOS[*]}"
    log "  通知邮箱: $NOTIFY_EMAIL"
    log "  日志目录: $LOG_DIR"
    echo ""
    
    # 测试 GitHub API
    log_info "测试 GitHub API..."
    local test_query="test"
    local gh_result=$(search_github "$test_query")
    if [ "$gh_result" = "-1" ]; then
        log_warning "GitHub API 可能受限，建议配置 GITHUB_TOKEN"
    else
        log_success "GitHub API 正常"
    fi
    echo ""
    
    # 测试邮件发送
    if [ "$NOTIFY_EMAIL" != "your-email@example.com" ]; then
        log_info "测试邮件发送..."
        local test_body="这是一封测试邮件。\n\n如果您收到此邮件，说明 HighClaw 指纹监控邮件通知配置成功。\n\n检查时间: $(date)"
        send_email "测试邮件 - 请忽略" "$test_body"
    else
        log_warning "邮箱未配置，跳过邮件测试"
    fi
    echo ""
    
    # 测试 macOS 通知
    log_info "测试系统通知..."
    send_macos_notification "HighClaw 测试" "配置测试完成"
    log_success "测试完成"
    echo ""
    
    log_info "=========================================="
    log_info "测试结果汇总"
    log_info "=========================================="
    log "  ✓ 依赖检查: 通过"
    log "  ? 配置检查: $([ "${FINGERPRINTS[0]}" != "YOUR_FINGERPRINT_1" ] && echo "已配置" || echo "需配置")"
    log "  ? GitHub API: $([ "$gh_result" != "-1" ] && echo "正常" || echo "受限")"
    log "  ? 邮件发送: $([ "$NOTIFY_EMAIL" != "your-email@example.com" ] && echo "请检查邮箱" || echo "未配置")"
    echo ""
    log_info "配置完成后可使用 --cron 模式设置定时任务"
}

# 显示帮助
show_help() {
    cat << EOF
HighClaw 代码指纹查重脚本 v1.0

用法: $0 [选项]

选项:
  --help, -h      显示帮助信息
  --verbose, -v   显示详细输出
  --test, -t      测试模式（检查配置和发送测试邮件）
  --cron          静默模式（仅在发现问题时输出和通知）

配置步骤:
  1. 编辑脚本顶部的 FINGERPRINTS 数组，填入你的指纹
  2. 编辑 MY_REPOS 数组，填入你的仓库名
  3. 编辑 NOTIFY_EMAIL，填入通知邮箱
  4. （可选）填入 GITHUB_TOKEN 提高 API 限额

示例:
  $0              # 运行一次完整查重
  $0 --test       # 测试配置和邮件
  $0 --verbose    # 详细输出模式
  $0 --cron       # 静默模式（用于定时任务）

定时任务设置:
  crontab -e
  
  # 每周一早上 9 点运行
  0 9 * * 1 /path/to/fingerprint-check.sh --cron
  
  # 每天早上 8 点运行
  0 8 * * * /path/to/fingerprint-check.sh --cron

邮件配置 (推荐使用 msmtp):
  1. 安装: brew install msmtp (macOS) / apt install msmtp (Ubuntu)
  2. 创建配置文件: ~/.msmtprc
  3. 配置 SMTP 服务器和授权码
  4. 设置权限: chmod 600 ~/.msmtprc
  5. 测试: echo "test" | msmtp your-email@example.com

日志位置:
  \$HOME/.highclaw/logs/

EOF
}

# ================================
# 主程序
# ================================

# 创建目录
mkdir -p "$LOG_DIR"

# 解析参数
while [[ $# -gt 0 ]]; do
    case $1 in
        --help|-h)
            show_help
            exit 0
            ;;
        --verbose|-v)
            VERBOSE=true
            shift
            ;;
        --test|-t)
            TEST_MODE=true
            shift
            ;;
        --cron)
            CRON_MODE=true
            shift
            ;;
        *)
            echo "未知选项: $1"
            show_help
            exit 1
            ;;
    esac
done

# 检查依赖
check_dependencies

# 运行
if [ "$TEST_MODE" = true ]; then
    run_test
else
    run_check
    exit $?
fi
