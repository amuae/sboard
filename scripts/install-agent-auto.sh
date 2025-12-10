#!/bin/bash

# SBoard Agent 通用安装入口
# 自动检测操作系统并调用对应的安装脚本
#
# 用法:
#   Linux/macOS:
#     curl -fsSL https://raw.githubusercontent.com/amuae/sboard/main/scripts/install-agent-auto.sh | bash -s -- --token <token> --panel <panel_url>
#
#   Windows (PowerShell) - 直接使用:
#     $env:TOKEN="your_token"; $env:PANEL="https://panel.example.com"; irm https://raw.githubusercontent.com/amuae/sboard/main/scripts/install-agent.ps1 | iex
#
# 参数:
#   --token <token>   Agent 认证 Token (必填)
#   --panel <url>     面板地址 (必填)
#   --uninstall       卸载 Agent
#   -h, --help        显示帮助

set -e

# 颜色
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
BLUE='\033[0;34m'
NC='\033[0m'

# GitHub 加速
GH_PROXY="https://ghfast.top/"
REPO_URL="https://raw.githubusercontent.com/amuae/sboard/main/scripts"

info() { echo -e "${BLUE}[INFO]${NC} $1"; }
success() { echo -e "${GREEN}[SUCCESS]${NC} $1"; }
warning() { echo -e "${YELLOW}[WARNING]${NC} $1"; }
error() { echo -e "${RED}[ERROR]${NC} $1"; exit 1; }

# 显示帮助
show_help() {
    echo "SBoard Agent 通用安装入口"
    echo ""
    echo "用法:"
    echo "  curl -fsSL <url>/install-agent-auto.sh | bash -s -- [选项]"
    echo ""
    echo "选项:"
    echo "  --token <token>   Agent 认证 Token (必填)"
    echo "  --panel <url>     面板地址 (必填)"
    echo "  --uninstall       卸载 Agent"
    echo "  -h, --help        显示帮助"
    echo ""
    echo "示例:"
    echo "  # Linux/macOS"
    echo "  curl -fsSL https://raw.githubusercontent.com/amuae/sboard/main/scripts/install-agent-auto.sh | bash -s -- --token abc123 --panel https://panel.example.com"
    echo ""
    echo "  # Windows (PowerShell)"
    echo '  $env:TOKEN="abc123"; $env:PANEL="https://panel.example.com"; irm https://raw.githubusercontent.com/amuae/sboard/main/scripts/install-agent.ps1 | iex'
    echo ""
}

# 检测操作系统
detect_os() {
    case "$(uname -s)" in
        Linux*)
            OS="linux"
            ;;
        Darwin*)
            OS="darwin"
            ;;
        CYGWIN*|MINGW*|MSYS*)
            OS="windows-bash"
            ;;
        *)
            OS="unknown"
            ;;
    esac
}

# 下载并执行 Linux 脚本
run_linux_installer() {
    info "检测到 Linux/macOS 系统，下载安装脚本..."
    
    if ! curl -fsSL --connect-timeout 10 "${GH_PROXY}${REPO_URL}/install-agent.sh" -o /tmp/install-agent.sh; then
        error "下载安装脚本失败"
    fi
    info "下载成功"
    
    chmod +x /tmp/install-agent.sh
    # 传递所有参数给实际的安装脚本
    bash /tmp/install-agent.sh "$@"
    rm -f /tmp/install-agent.sh
}

# Windows Git Bash 环境提示
show_windows_help() {
    warning "检测到 Windows Git Bash / MSYS 环境"
    echo ""
    echo -e "${YELLOW}在 Windows 上请使用 PowerShell 安装:${NC}"
    echo ""
    echo -e "  ${GREEN}方式1: 设置环境变量后安装${NC}"
    echo '  $env:TOKEN="your_token"'
    echo '  $env:PANEL="https://panel.example.com"'
    echo '  irm https://raw.githubusercontent.com/amuae/sboard/main/scripts/install-agent.ps1 | iex'
    echo ""
    echo -e "  ${GREEN}方式2: 下载脚本后带参数运行${NC}"
    echo '  Invoke-WebRequest -Uri "https://raw.githubusercontent.com/amuae/sboard/main/scripts/install-agent.ps1" -OutFile "install-agent.ps1"'
    echo '  .\install-agent.ps1 -Token "your_token" -Panel "https://panel.example.com"'
    echo ""
    echo -e "  ${GREEN}使用加速下载:${NC}"
    echo '  irm https://ghfast.top/https://raw.githubusercontent.com/amuae/sboard/main/scripts/install-agent.ps1 -OutFile install-agent.ps1'
    echo ""
    exit 0
}

# 主函数
main() {
    # 检查是否请求帮助
    for arg in "$@"; do
        case "$arg" in
            -h|--help)
                show_help
                exit 0
                ;;
        esac
    done

    echo ""
    echo -e "${BLUE}========================================${NC}"
    echo -e "${BLUE}   SBoard Agent 通用安装入口${NC}"
    echo -e "${BLUE}========================================${NC}"
    echo ""
    
    detect_os
    info "检测到系统: $OS"
    
    case "$OS" in
        linux|darwin)
            run_linux_installer "$@"
            ;;
        windows-bash)
            show_windows_help
            ;;
        *)
            error "不支持的操作系统: $(uname -s)"
            ;;
    esac
}

main "$@"
