#!/bin/bash

# SBoard Agent 一键安装脚本
# 用法: curl -fsSL https://your-panel.com/install-agent.sh | bash -s -- --token <token> [--core <core_type>]
# 或者: curl -fsSL https://raw.githubusercontent.com/amuae/sboard/main/scripts/install-agent.sh | bash -s -- --token <token> --panel <panel_url> [--core <core_type>]

set -e

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
BLUE='\033[0;34m'
NC='\033[0m'

# 默认配置
INSTALL_DIR="/opt/sboard-agent"
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
}

# 检查 root 权限
check_root() {
    if [[ $EUID -ne 0 ]]; then
        error "请使用 root 权限运行此脚本: sudo bash $0 $*"
    fi
}

# 检测系统架构
detect_arch() {
    ARCH=$(uname -m)
    case $ARCH in
        x86_64)
            ARCH="amd64"
            ;;
        aarch64)
            ARCH="arm64"
            ;;
        armv7l)
            ARCH="armv7"
            ;;
        *)
            error "不支持的架构: $ARCH"
            ;;
    esac
    info "检测到架构: $ARCH"
}

# 检测系统
detect_os() {
    if [[ -f /etc/os-release ]]; then
        . /etc/os-release
        OS=$ID
        VERSION=$VERSION_ID
    elif [[ -f /etc/redhat-release ]]; then
        OS="centos"
    else
        OS="unknown"
    fi
    info "检测到系统: $OS"
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

# 停止服务
stop_service() {
    if systemctl is-active --quiet "$SERVICE_NAME" 2>/dev/null; then
        info "停止现有服务..."
        systemctl stop "$SERVICE_NAME" || true
    fi
}

# 卸载
uninstall() {
    check_root
    info "开始卸载 SBoard Agent..."

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

    # 删除安装目录
    rm -rf "$INSTALL_DIR"

    success "SBoard Agent 已卸载"
}

# 下载 Agent
download_agent() {
    info "下载 Agent..."
    
    # 创建安装目录
    mkdir -p "$INSTALL_DIR"
    
    # 从面板下载 Agent
    DOWNLOAD_URL="${PANEL_URL}/download/agent-linux-${ARCH}"
    
    if command -v curl &> /dev/null; then
        curl -fsSL "$DOWNLOAD_URL" -o "${INSTALL_DIR}/${BINARY_NAME}" || error "下载 Agent 失败，请检查面板地址是否正确"
    elif command -v wget &> /dev/null; then
        wget -q "$DOWNLOAD_URL" -O "${INSTALL_DIR}/${BINARY_NAME}" || error "下载 Agent 失败，请检查面板地址是否正确"
    else
        error "请安装 curl 或 wget"
    fi
    
    # 设置可执行权限
    chmod +x "${INSTALL_DIR}/${BINARY_NAME}"
    
    success "Agent 下载完成"
}

# 生成配置文件
generate_config() {
    info "生成配置文件..."
    
    # 获取主机名作为 Agent ID
    AGENT_ID=$(hostname)
    
    # 生成配置
    cat > "${INSTALL_DIR}/${CONFIG_FILE}" << EOF
{
    "panel_url": "${PANEL_URL}",
    "token": "${TOKEN}",
    "agent_id": "${AGENT_ID}",
    "core_type": "${CORE_TYPE}",
    "core_path": "/root/${CORE_TYPE}/${CORE_TYPE}",
    "config_dir": "/root/${CORE_TYPE}"
}
EOF
    
    success "配置文件已生成"
}

# 创建 systemd 服务
create_service() {
    info "创建 systemd 服务..."
    
    cat > "/etc/systemd/system/${SERVICE_NAME}.service" << EOF
[Unit]
Description=SBoard Agent
Documentation=https://github.com/sboard-go/sboard
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

# 安全设置
NoNewPrivileges=false
ProtectSystem=false
ProtectHome=false

[Install]
WantedBy=multi-user.target
EOF

    # 重载 systemd
    systemctl daemon-reload
    
    success "systemd 服务已创建"
}

# 启动服务
start_service() {
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
    echo "安装目录: $INSTALL_DIR"
    echo "配置文件: ${INSTALL_DIR}/${CONFIG_FILE}"
    echo "服务名称: $SERVICE_NAME"
    echo "核心类型: $CORE_TYPE"
    echo ""
    echo "常用命令:"
    echo "  查看状态: systemctl status $SERVICE_NAME"
    echo "  查看日志: journalctl -u $SERVICE_NAME -f"
    echo "  重启服务: systemctl restart $SERVICE_NAME"
    echo "  停止服务: systemctl stop $SERVICE_NAME"
    echo "  卸载: curl -fsSL ${PANEL_URL}/install-agent.sh | bash -s -- --uninstall"
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
