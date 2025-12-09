#!/bin/bash

# SBoard 面板安装脚本入口
# 自动下载并执行最新的安装脚本
#
# 用法:
#   curl -fsSL https://ghfast.top/https://raw.githubusercontent.com/amuae/sboard/main/scripts/install.sh | bash
#   curl -fsSL <url> | bash -s -- [options]
#
# 支持:
#   - Linux (amd64, 386, arm64, armv7, armv6)
#   - macOS (amd64, arm64)

set -e

# GitHub 配置
GITHUB_REPO="amuae/sboard"
GH_PROXY="https://ghfast.top/"
BRANCH="main"

# 脚本 URL
SCRIPT_URL="${GH_PROXY}https://raw.githubusercontent.com/${GITHUB_REPO}/${BRANCH}/scripts/install-sboard.sh"

# 颜色定义
CYAN='\033[0;36m'
BLUE='\033[0;34m'
YELLOW='\033[0;33m'
RED='\033[0;31m'
NC='\033[0m'

# 检测操作系统
detect_os() {
    OS_TYPE=$(uname -s 2>/dev/null || echo "Unknown")
    case "$OS_TYPE" in
        Linux|linux|Darwin|darwin)
            ;;
        MINGW*|MSYS*|CYGWIN*)
            echo -e "${YELLOW}[WARNING]${NC} 检测到 Windows 环境"
            echo -e "${YELLOW}[WARNING]${NC} 请使用 PowerShell 安装:"
            echo ""
            echo "  irm ${GH_PROXY}https://raw.githubusercontent.com/${GITHUB_REPO}/${BRANCH}/scripts/install-sboard.ps1 | iex"
            echo ""
            exit 0
            ;;
        *)
            echo -e "${RED}[ERROR]${NC} 不支持的操作系统: $OS_TYPE"
            exit 1
            ;;
    esac
}

# 检查依赖
check_deps() {
    if ! command -v curl &> /dev/null; then
        echo -e "${RED}[ERROR]${NC} 缺少依赖: curl，请先安装"
        exit 1
    fi
}

# 主函数
main() {
    echo ""
    echo -e "${CYAN}==========================================${NC}"
    echo -e "${CYAN}      SBoard 面板安装脚本${NC}"
    echo -e "${CYAN}==========================================${NC}"
    echo ""
    
    # 检查
    check_deps
    detect_os
    
    # 下载并执行安装脚本
    echo -e "${BLUE}[INFO]${NC} 下载安装脚本..."
    curl -fsSL "$SCRIPT_URL" | bash -s -- "$@"
}

# 执行
main "$@"
