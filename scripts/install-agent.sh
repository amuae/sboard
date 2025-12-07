#!/bin/bash

# SBoard Agent 一键安装脚本
# 支持: Linux (amd64, arm64, armv7, 386, mips, mipsle, mips64, mips64le, riscv64, s390x)
#       macOS (amd64, arm64), FreeBSD (amd64, arm64, arm, 386), Windows (amd64, arm64, 386)
# 用法: curl -fsSL https://your-panel.com/install-agent.sh | bash -s -- --token <token> [--core <core_type>]
# 或者: curl -fsSL https://raw.githubusercontent.com/amuae/sboard/main/scripts/install-agent.sh | bash -s -- --token <token> --panel <panel_url> [--core <core_type>]

set -e

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
BLUE='\033[0;34m'
NC='\033[0m'

# 配置
GITHUB_REPO="amuae/sboard"
INSTALL_DIR="/opt/sboard/agent"
SERVICE_NAME="sboard-agent"
BINARY_NAME="sboard-agent"
CONFIG_FILE="agent.json"

# 参数
PANEL_URL=""
TOKEN=""
CORE_TYPE="sing-box"

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
    echo "SBoard Agent 一键安装脚本"
    echo ""
    echo "用法: $0 [选项]"
    echo ""
    echo "选项:"
    echo "  --token <token>     Agent 认证 Token (必填)"
    echo "  --panel <url>       面板地址 (必填)"
    echo "  --core <type>       核心类型: sing-box 或 mihomo (默认: sing-box)"
    echo "  --uninstall         卸载 Agent"
    echo "  -h, --help          显示帮助"
    echo ""
    echo "示例:"
    echo "  $0 --token abc123 --panel https://panel.example.com"
    echo "  $0 --token abc123 --panel https://panel.example.com --core mihomo"
    echo "  $0 --uninstall"
    echo ""
    echo "支持的平台:"
    echo "  Linux:   amd64, arm64, armv7, 386, mips, mipsle, mips64, mips64le, riscv64, s390x"
    echo "  macOS:   amd64, arm64"
    echo "  FreeBSD: amd64, arm64, arm, 386"
    echo ""
}

# 检查 root 权限
check_root() {
    if [[ $EUID -ne 0 ]]; then
        error "请使用 root 权限运行此脚本: sudo bash $0 $*"
    fi
}

# 检测操作系统
detect_os() {
    OS_TYPE=$(uname -s | tr '[:upper:]' '[:lower:]')
    case "$OS_TYPE" in
        linux)
            OS="linux"
            ;;
        darwin)
            OS="darwin"
            ;;
        freebsd)
            OS="freebsd"
            ;;
        mingw*|msys*|cygwin*)
            OS="windows"
            ;;
        *)
            error "不支持的操作系统: $OS_TYPE"
            ;;
    esac
    info "检测到操作系统: $OS"
}

# 检测架构
detect_arch() {
    ARCH_TYPE=$(uname -m)
    case "$ARCH_TYPE" in
        x86_64|amd64)
            ARCH="amd64"
            ;;
        aarch64|arm64)
            ARCH="arm64"
            ;;
        armv7l|armv7)
            ARCH="armv7"
            ;;
        armv6l|armv5l|arm)
            ARCH="arm"
            ;;
        i386|i686)
            ARCH="386"
            ;;
        mips)
            ARCH="mips"
            ;;
        mipsel|mipsle)
            ARCH="mipsle"
            ;;
        mips64)
            ARCH="mips64"
            ;;
        mips64el|mips64le)
            ARCH="mips64le"
            ;;
        riscv64)
            ARCH="riscv64"
            ;;
        s390x)
            ARCH="s390x"
            ;;
        *)
            error "不支持的架构: $ARCH_TYPE"
            ;;
    esac
    info "检测到架构: $ARCH"
}

# 解析参数
parse_args() {
    while [[ $# -gt 0 ]]; do
        case $1 in
            --token)
                TOKEN="$2"
                shift 2
                ;;
            --panel)
                PANEL_URL="$2"
                shift 2
                ;;
            --core)
                CORE_TYPE="$2"
                shift 2
                ;;
            --uninstall)
                uninstall
                exit 0
                ;;
            -h|--help)
                show_help
                exit 0
                ;;
            *)
                error "未知参数: $1"
                ;;
        esac
    done

    # 验证必填参数
    if [[ -z "$TOKEN" ]]; then
        error "缺少参数: --token"
    fi
    if [[ -z "$PANEL_URL" ]]; then
        error "缺少参数: --panel"
    fi

    # 移除末尾的斜杠
    PANEL_URL="${PANEL_URL%/}"
}

# 检查依赖
install_deps() {
    info "检查依赖..."
    
    for cmd in curl unzip; do
        if ! command -v $cmd &> /dev/null; then
            info "安装 $cmd..."
            if command -v apt-get &> /dev/null; then
                apt-get update -qq && apt-get install -y -qq $cmd
            elif command -v yum &> /dev/null; then
                yum install -y -q $cmd
            elif command -v dnf &> /dev/null; then
                dnf install -y -q $cmd
            elif command -v pacman &> /dev/null; then
                pacman -S --noconfirm $cmd
            elif command -v apk &> /dev/null; then
                apk add --no-cache $cmd
            elif command -v pkg &> /dev/null; then
                pkg install -y $cmd
            else
                error "无法安装 $cmd，请手动安装"
            fi
        fi
    done
}

# 获取最新版本
get_latest_version() {
    info "获取最新版本..."
    LATEST_VERSION=$(curl -fsSL "https://api.github.com/repos/${GITHUB_REPO}/releases/latest" 2>/dev/null | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')
    if [[ -z "$LATEST_VERSION" ]]; then
        error "无法获取最新版本，请检查网络连接"
    fi
    info "最新版本: $LATEST_VERSION"
}

# 停止服务
stop_service() {
    if [[ "$OS" == "linux" ]] && systemctl is-active --quiet "$SERVICE_NAME" 2>/dev/null; then
        info "停止现有服务..."
        systemctl stop "$SERVICE_NAME" || true
    fi
}

# 卸载
uninstall() {
    check_root
    detect_os
    
    info "开始卸载 SBoard Agent..."

    if [[ "$OS" == "linux" ]]; then
        # 停止并禁用服务
        if systemctl is-active --quiet "$SERVICE_NAME" 2>/dev/null; then
            systemctl stop "$SERVICE_NAME"
        fi
        if systemctl is-enabled --quiet "$SERVICE_NAME" 2>/dev/null; then
            systemctl disable "$SERVICE_NAME"
        fi

        # 删除服务文件
        rm -f "/etc/systemd/system/${SERVICE_NAME}.service"
        systemctl daemon-reload
    fi

    # 删除安装目录
    rm -rf "$INSTALL_DIR"

    success "SBoard Agent 已卸载"
}

# 下载 Agent
download_agent() {
    info "下载 Agent..."
    
    # 创建安装目录
    mkdir -p "$INSTALL_DIR"
    
    # 构建下载 URL
    DOWNLOAD_FILE="${BINARY_NAME}_${OS}_${ARCH}.zip"
    DOWNLOAD_URL="https://github.com/${GITHUB_REPO}/releases/download/${LATEST_VERSION}/${DOWNLOAD_FILE}"
    
    info "下载地址: $DOWNLOAD_URL"
    
    # 创建临时目录
    TMP_DIR=$(mktemp -d)
    cd "$TMP_DIR"
    
    # 下载
    if ! curl -fsSL "$DOWNLOAD_URL" -o "${DOWNLOAD_FILE}"; then
        rm -rf "$TMP_DIR"
        error "下载失败，请检查版本 ${LATEST_VERSION} 是否存在 ${OS}_${ARCH} 构建"
    fi
    
    # 解压
    unzip -q "${DOWNLOAD_FILE}"
    
    # 安装二进制文件
    if [[ "$OS" == "windows" ]]; then
        mv "${BINARY_NAME}.exe" "${INSTALL_DIR}/"
    else
        mv "${BINARY_NAME}" "${INSTALL_DIR}/"
        chmod +x "${INSTALL_DIR}/${BINARY_NAME}"
    fi
    
    # 清理
    cd /
    rm -rf "$TMP_DIR"
    
    success "Agent 下载完成"
}

# 生成配置文件
generate_config() {
    info "生成配置文件..."
    
    # 获取主机名作为 Agent ID
    AGENT_ID=$(hostname)
    
    # 设置核心路径
    case "$CORE_TYPE" in
        sing-box)
            CORE_PATH="/etc/sing-box/sing-box"
            CONFIG_DIR="/etc/sing-box"
            ;;
        mihomo)
            CORE_PATH="/etc/mihomo/mihomo"
            CONFIG_DIR="/etc/mihomo"
            ;;
        *)
            CORE_PATH="/root/${CORE_TYPE}/${CORE_TYPE}"
            CONFIG_DIR="/root/${CORE_TYPE}"
            ;;
    esac
    
    # 生成配置
    cat > "${INSTALL_DIR}/${CONFIG_FILE}" << EOF
{
    "panel_url": "${PANEL_URL}",
    "token": "${TOKEN}",
    "agent_id": "${AGENT_ID}",
    "core_type": "${CORE_TYPE}",
    "core_path": "${CORE_PATH}",
    "config_dir": "${CONFIG_DIR}"
}
EOF
    
    success "配置文件已生成"
}

# 创建 systemd 服务
create_service() {
    if [[ "$OS" != "linux" ]]; then
        warning "非 Linux 系统，跳过 systemd 服务创建"
        return
    fi
    
    info "创建 systemd 服务..."
    
    cat > "/etc/systemd/system/${SERVICE_NAME}.service" << EOF
[Unit]
Description=SBoard Agent
Documentation=https://github.com/${GITHUB_REPO}
After=network.target network-online.target
Wants=network-online.target

[Service]
Type=simple
User=root
Group=root
WorkingDirectory=${INSTALL_DIR}
ExecStart=${INSTALL_DIR}/${BINARY_NAME} -c ${INSTALL_DIR}/${CONFIG_FILE}
Restart=always
RestartSec=5
LimitNOFILE=65535

[Install]
WantedBy=multi-user.target
EOF

    # 重载 systemd
    systemctl daemon-reload
    
    success "systemd 服务已创建"
}

# 启动服务
start_service() {
    if [[ "$OS" != "linux" ]]; then
        warning "非 Linux 系统，请手动启动: ${INSTALL_DIR}/${BINARY_NAME} -c ${INSTALL_DIR}/${CONFIG_FILE}"
        return
    fi
    
    info "启动服务..."
    
    # 启用并启动服务
    systemctl enable "$SERVICE_NAME"
    systemctl start "$SERVICE_NAME"
    
    # 等待服务启动
    sleep 2
    
    # 检查服务状态
    if systemctl is-active --quiet "$SERVICE_NAME"; then
        success "服务启动成功"
    else
        error "服务启动失败，请查看日志: journalctl -u $SERVICE_NAME -f"
    fi
}

# 显示状态
show_status() {
    echo ""
    echo "=========================================="
    echo -e "${GREEN}SBoard Agent 安装完成${NC}"
    echo "=========================================="
    echo ""
    echo "版本: ${LATEST_VERSION}"
    echo "安装目录: $INSTALL_DIR"
    echo "配置文件: ${INSTALL_DIR}/${CONFIG_FILE}"
    echo "服务名称: $SERVICE_NAME"
    echo "核心类型: $CORE_TYPE"
    echo ""
    if [[ "$OS" == "linux" ]]; then
        echo "常用命令:"
        echo "  查看状态: systemctl status $SERVICE_NAME"
        echo "  查看日志: journalctl -u $SERVICE_NAME -f"
        echo "  重启服务: systemctl restart $SERVICE_NAME"
        echo "  停止服务: systemctl stop $SERVICE_NAME"
        echo ""
    fi
    echo "卸载命令:"
    echo "  curl -fsSL ${PANEL_URL}/install-agent.sh | bash -s -- --uninstall"
    echo ""
}

# 主函数
main() {
    echo ""
    echo "=========================================="
    echo "       SBoard Agent 一键安装脚本"
    echo "=========================================="
    echo ""

    # 检查权限
    check_root

    # 解析参数
    parse_args "$@"

    # 检测系统
    detect_os
    detect_arch

    # 安装依赖
    install_deps

    # 获取最新版本
    get_latest_version

    # 停止现有服务
    stop_service

    # 下载 Agent
    download_agent

    # 生成配置
    generate_config

    # 创建服务
    create_service

    # 启动服务
    start_service

    # 显示状态
    show_status
}

# 执行主函数
main "$@"
