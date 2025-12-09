#!/bin/bash

# SBoard 万能安装脚本
# 自动检测系统并调用对应的安装脚本
#
# 用法:
#   curl -fsSL https://ghfast.top/https://raw.githubusercontent.com/amuae/sboard/main/scripts/install.sh | bash
#   curl -fsSL <url> | bash -s -- [sboard|agent] [options]
#
# 支持:
#   - Linux (amd64, 386, arm64, armv7, armv6)
#   - macOS (amd64, arm64)
#   - Windows (通过 Git Bash/MSYS2/WSL)

set -e

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m'

# GitHub 配置
GITHUB_REPO="amuae/sboard"
GH_PROXY="https://ghfast.top/"
BRANCH="main"

# 脚本 URL
SBOARD_SCRIPT_URL="${GH_PROXY}https://raw.githubusercontent.com/${GITHUB_REPO}/${BRANCH}/scripts/install-sboard.sh"
AGENT_SCRIPT_URL="${GH_PROXY}https://raw.githubusercontent.com/${GITHUB_REPO}/${BRANCH}/scripts/install-agent.sh"

# 函数: 打印信息
info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

error() {
    echo -e "${RED}[ERROR]${NC} $1"
    exit 1
}

# 显示帮助
show_help() {
    echo -e "${CYAN}========================================${NC}"
    echo -e "${CYAN}      SBoard 万能安装脚本${NC}"
    echo -e "${CYAN}========================================${NC}"
    echo ""
    echo "用法:"
    echo "  curl -fsSL <url> | bash"
    echo "  curl -fsSL <url> | bash -s -- [组件] [选项]"
    echo ""
    echo "组件:"
    echo "  sboard    安装 SBoard 面板 (默认)"
    echo "  agent     安装 SBoard Agent"
    echo ""
    echo "SBoard 面板选项:"
    echo "  --port <port>     监听端口 (默认: 8080)"
    echo "  --user <user>     管理员用户名"
    echo "  --pass <pass>     管理员密码"
    echo "  --path <path>     安装路径"
    echo "  --no-interactive  非交互模式"
    echo "  update            更新面板"
    echo "  uninstall         卸载面板"
    echo ""
    echo "Agent 选项:"
    echo "  --token <token>   Agent Token (必填)"
    echo "  --panel <url>     面板地址 (必填)"
    echo "  --core <type>     核心类型: sing-box/mihomo"
    echo "  --uninstall       卸载 Agent"
    echo ""
    echo "示例:"
    echo "  # 安装面板 (交互式)"
    echo "  curl -fsSL <url> | bash"
    echo ""
    echo "  # 安装面板 (指定参数)"
    echo "  curl -fsSL <url> | bash -s -- sboard --port 9000 --user admin --pass mypass"
    echo ""
    echo "  # 安装 Agent"
    echo "  curl -fsSL <url> | bash -s -- agent --token abc123 --panel https://panel.example.com"
    echo ""
    echo "  # 更新面板"
    echo "  curl -fsSL <url> | bash -s -- sboard update"
    echo ""
    echo "  # 卸载"
    echo "  curl -fsSL <url> | bash -s -- sboard uninstall"
    echo "  curl -fsSL <url> | bash -s -- agent --uninstall"
    echo ""
    echo "支持的平台:"
    echo "  Linux:   amd64, 386, arm64, armv7, armv6"
    echo "  macOS:   amd64, arm64"
    echo "  Windows: 请使用 PowerShell 脚本"
    echo ""
    echo "Windows PowerShell 安装:"
    echo "  面板: irm https://ghfast.top/https://raw.githubusercontent.com/amuae/sboard/main/scripts/install-sboard.ps1 | iex"
    echo "  Agent: irm https://ghfast.top/https://raw.githubusercontent.com/amuae/sboard/main/scripts/install-agent.ps1 | iex"
    echo ""
}

# 检测操作系统
detect_os() {
    OS_TYPE=$(uname -s 2>/dev/null || echo "Unknown")
    case "$OS_TYPE" in
        Linux|linux)
            OS="linux"
            ;;
        Darwin|darwin)
            OS="darwin"
            ;;
        MINGW*|MSYS*|CYGWIN*)
            OS="windows"
            warning "检测到 Windows 环境 (Git Bash/MSYS2/Cygwin)"
            warning "建议使用 PowerShell 脚本安装:"
            echo ""
            echo "  面板: irm https://ghfast.top/https://raw.githubusercontent.com/amuae/sboard/main/scripts/install-sboard.ps1 | iex"
            echo "  Agent: irm https://ghfast.top/https://raw.githubusercontent.com/amuae/sboard/main/scripts/install-agent.ps1 | iex"
            echo ""
            exit 0
            ;;
        *)
            error "不支持的操作系统: $OS_TYPE"
            ;;
    esac
}

# 检测架构
detect_arch() {
    ARCH_TYPE=$(uname -m 2>/dev/null || echo "Unknown")
    case "$ARCH_TYPE" in
        x86_64|amd64)
            ARCH="amd64"
            ;;
        i386|i486|i586|i686|x86)
            ARCH="386"
            ;;
        aarch64|arm64)
            ARCH="arm64"
            ;;
        armv7l|armv7)
            ARCH="armv7"
            ;;
        armv6l)
            ARCH="armv6"
            ;;
        armv5l|arm)
            ARCH="arm"
            ;;
        *)
            error "不支持的架构: $ARCH_TYPE"
            ;;
    esac
}

# 检查依赖
check_deps() {
    for cmd in curl; do
        if ! command -v $cmd &> /dev/null; then
            error "缺少依赖: $cmd，请先安装"
        fi
    done
}

# 安装面板
install_sboard() {
    info "下载并执行 SBoard 面板安装脚本..."
    curl -fsSL "$SBOARD_SCRIPT_URL" | bash -s -- "$@"
}

# 安装 Agent
install_agent() {
    info "下载并执行 SBoard Agent 安装脚本..."
    curl -fsSL "$AGENT_SCRIPT_URL" | bash -s -- "$@"
}

# 主函数
main() {
    echo ""
    echo -e "${CYAN}========================================${NC}"
    echo -e "${CYAN}      SBoard 万能安装脚本${NC}"
    echo -e "${CYAN}========================================${NC}"
    echo ""
    
    # 检查依赖
    check_deps
    
    # 检测系统
    detect_os
    detect_arch
    info "系统: ${OS}/${ARCH}"
    
    # 解析第一个参数确定组件
    COMPONENT="sboard"
    ARGS=()
    
    if [[ $# -gt 0 ]]; then
        case "$1" in
            sboard|panel)
                COMPONENT="sboard"
                shift
                ARGS=("$@")
                ;;
            agent)
                COMPONENT="agent"
                shift
                ARGS=("$@")
                ;;
            -h|--help|help)
                show_help
                exit 0
                ;;
            *)
                # 不是组件名，可能是选项，默认安装 sboard
                ARGS=("$@")
                ;;
        esac
    fi
    
    # 根据组件执行安装
    case "$COMPONENT" in
        sboard)
            install_sboard "${ARGS[@]}"
            ;;
        agent)
            install_agent "${ARGS[@]}"
            ;;
    esac
}

# 执行
main "$@"
