#!/bin/bash

# SBoard Agent 通用卸载入口
# 自动检测操作系统并调用对应的卸载脚本
#
# Linux/macOS:
#   curl -fsSL https://raw.githubusercontent.com/amuae/sboard/main/scripts/uninstall-agent-auto.sh | bash
#
# Windows (PowerShell) - 直接使用:
#   irm https://raw.githubusercontent.com/amuae/sboard/main/scripts/uninstall-agent.ps1 | iex

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
run_linux_uninstaller() {
    info "检测到 Linux/macOS 系统，下载卸载脚本..."
    
    # 先尝试直接下载
    if curl -fsSL --connect-timeout 5 "${REPO_URL}/uninstall-agent.sh" -o /tmp/uninstall-agent.sh 2>/dev/null; then
        info "直接下载成功"
    else
        info "使用加速下载..."
        curl -fsSL "${GH_PROXY}${REPO_URL}/uninstall-agent.sh" -o /tmp/uninstall-agent.sh
    fi
    
    chmod +x /tmp/uninstall-agent.sh
    bash /tmp/uninstall-agent.sh "$@"
    rm -f /tmp/uninstall-agent.sh
}

# Windows Git Bash 环境提示
run_windows_bash_uninstaller() {
    warning "检测到 Windows Git Bash / MSYS 环境"
    echo ""
    echo -e "${YELLOW}在 Windows 上推荐使用 PowerShell 卸载:${NC}"
    echo ""
    echo -e "  ${GREEN}irm https://raw.githubusercontent.com/amuae/sboard/main/scripts/uninstall-agent.ps1 | iex${NC}"
    echo ""
    echo "或者使用加速:"
    echo ""
    echo -e "  ${GREEN}irm ${GH_PROXY}https://raw.githubusercontent.com/amuae/sboard/main/scripts/uninstall-agent.ps1 | iex${NC}"
    echo ""
    exit 0
}

# 主函数
main() {
    echo ""
    echo -e "${BLUE}========================================${NC}"
    echo -e "${BLUE}   SBoard Agent 通用卸载入口${NC}"
    echo -e "${BLUE}========================================${NC}"
    echo ""
    
    detect_os
    info "检测到系统: $OS"
    
    case "$OS" in
        linux|darwin)
            run_linux_uninstaller "$@"
            ;;
        windows-bash)
            run_windows_bash_uninstaller
            ;;
        *)
            error "不支持的操作系统: $(uname -s)"
            ;;
    esac
}

main "$@"
