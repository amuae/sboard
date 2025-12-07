#!/bin/bash

# SBoard 面板一键安装脚本
# 支持: Linux (amd64, arm64, armv7, 386), macOS (amd64, arm64), FreeBSD (amd64, arm64)
# 用法: curl -fsSL https://raw.githubusercontent.com/amuae/sboard/main/scripts/install-sboard.sh | bash

set -e

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# 配置
GITHUB_REPO="amuae/sboard"
INSTALL_DIR="/opt/sboard"
DATA_DIR="/var/lib/sboard"
SERVICE_NAME="sboard"
BINARY_NAME="sboard"

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

# 检查 root 权限
check_root() {
    if [[ $EUID -ne 0 ]]; then
        error "请使用 root 权限运行此脚本"
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
        i386|i686)
            ARCH="386"
            ;;
        *)
            error "不支持的架构: $ARCH_TYPE"
            ;;
    esac
    info "检测到架构: $ARCH"
}

# 检测包管理器并安装依赖
install_deps() {
    info "检查依赖..."
    
    # 检查必要的命令
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
            elif command -v brew &> /dev/null; then
                brew install $cmd
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

# 下载并安装
download_and_install() {
    info "下载 SBoard..."
    
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
    
    # 创建目录
    mkdir -p "$INSTALL_DIR"
    mkdir -p "$DATA_DIR"
    
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
    
    success "SBoard 下载完成"
}

# 创建配置文件
create_config() {
    if [[ -f "${DATA_DIR}/config.yaml" ]]; then
        warning "配置文件已存在，跳过创建"
        return
    fi
    
    info "创建配置文件..."
    
    # 生成随机 JWT 密钥和 Agent Token
    JWT_SECRET=$(head -c 32 /dev/urandom | base64 | tr -dc 'a-zA-Z0-9' | head -c 32)
    AGENT_TOKEN=$(head -c 16 /dev/urandom | base64 | tr -dc 'a-zA-Z0-9' | head -c 32)
    
    cat > "${DATA_DIR}/config.yaml" << EOF
# SBoard 配置文件

server:
  listen: "0.0.0.0:8080"
  debug: false

security:
  jwt_secret: "${JWT_SECRET}"
  jwt_expire_hour: 168

agent:
  token: "${AGENT_TOKEN}"
EOF

    chmod 600 "${DATA_DIR}/config.yaml"
    success "配置文件创建完成: ${DATA_DIR}/config.yaml"
}

# 创建 systemd 服务 (仅 Linux)
create_service() {
    if [[ "$OS" != "linux" ]]; then
        warning "非 Linux 系统，跳过 systemd 服务创建"
        return
    fi
    
    info "创建 systemd 服务..."
    
    cat > /etc/systemd/system/${SERVICE_NAME}.service << EOF
[Unit]
Description=SBoard - Proxy Management Panel
Documentation=https://github.com/${GITHUB_REPO}
After=network.target

[Service]
Type=simple
User=root
WorkingDirectory=${DATA_DIR}
ExecStart=${INSTALL_DIR}/${BINARY_NAME} -d ${DATA_DIR}
Restart=on-failure
RestartSec=5
LimitNOFILE=65535

[Install]
WantedBy=multi-user.target
EOF

    systemctl daemon-reload
    systemctl enable ${SERVICE_NAME}
    
    success "systemd 服务创建完成"
}

# 启动服务
start_service() {
    if [[ "$OS" != "linux" ]]; then
        warning "非 Linux 系统，请手动启动: ${INSTALL_DIR}/${BINARY_NAME} -d ${DATA_DIR}"
        return
    fi
    
    info "启动 SBoard..."
    
    systemctl start ${SERVICE_NAME}
    
    # 等待服务启动
    sleep 2
    
    if systemctl is-active --quiet ${SERVICE_NAME}; then
        success "SBoard 启动成功"
    else
        error "SBoard 启动失败，请检查日志: journalctl -u ${SERVICE_NAME}"
    fi
}

# 打印信息
print_info() {
    # 获取本机 IP
    if command -v hostname &> /dev/null; then
        LOCAL_IP=$(hostname -I 2>/dev/null | awk '{print $1}' || echo "localhost")
    else
        LOCAL_IP="localhost"
    fi
    
    echo ""
    echo "=========================================="
    echo -e "${GREEN}SBoard 安装成功!${NC}"
    echo "=========================================="
    echo ""
    echo "版本: ${LATEST_VERSION}"
    echo "安装目录: ${INSTALL_DIR}"
    echo "数据目录: ${DATA_DIR}"
    echo ""
    if [[ "$OS" == "linux" ]]; then
        echo "管理命令:"
        echo "  启动: systemctl start ${SERVICE_NAME}"
        echo "  停止: systemctl stop ${SERVICE_NAME}"
        echo "  重启: systemctl restart ${SERVICE_NAME}"
        echo "  状态: systemctl status ${SERVICE_NAME}"
        echo "  日志: journalctl -u ${SERVICE_NAME} -f"
        echo ""
    fi
    echo "访问地址: http://${LOCAL_IP}:8080"
    echo ""
    echo -e "${YELLOW}首次登录请使用:${NC}"
    echo "  用户名: admin"
    echo "  密码:   admin123"
    echo ""
    echo -e "${RED}请务必及时修改默认密码!${NC}"
    echo ""
}

# 卸载
uninstall() {
    info "卸载 SBoard..."
    
    if [[ "$OS" == "linux" ]]; then
        # 停止服务
        systemctl stop ${SERVICE_NAME} 2>/dev/null || true
        systemctl disable ${SERVICE_NAME} 2>/dev/null || true
        rm -f /etc/systemd/system/${SERVICE_NAME}.service
        systemctl daemon-reload
    fi
    
    # 删除安装目录
    rm -rf "${INSTALL_DIR}"
    
    # 询问是否删除数据
    read -p "是否删除数据目录 (${DATA_DIR})? [y/N]: " -n 1 -r
    echo
    if [[ $REPLY =~ ^[Yy]$ ]]; then
        rm -rf "${DATA_DIR}"
    fi
    
    success "SBoard 卸载完成"
}

# 更新
update() {
    info "更新 SBoard..."
    
    if [[ "$OS" == "linux" ]]; then
        systemctl stop ${SERVICE_NAME} 2>/dev/null || true
    fi
    
    get_latest_version
    download_and_install
    
    if [[ "$OS" == "linux" ]]; then
        systemctl start ${SERVICE_NAME}
    fi
    
    success "SBoard 更新到 ${LATEST_VERSION}"
}

# 显示帮助
show_help() {
    echo "SBoard 安装脚本"
    echo ""
    echo "用法:"
    echo "  install-sboard.sh [命令]"
    echo ""
    echo "命令:"
    echo "  install   安装 SBoard (默认)"
    echo "  update    更新 SBoard"
    echo "  uninstall 卸载 SBoard"
    echo "  help      显示帮助"
    echo ""
    echo "支持的平台:"
    echo "  Linux:   amd64, arm64, armv7, 386"
    echo "  macOS:   amd64, arm64"
    echo "  FreeBSD: amd64, arm64"
    echo ""
}

# 主函数
main() {
    echo ""
    echo "=========================================="
    echo "       SBoard 一键安装脚本"
    echo "=========================================="
    echo ""
    
    case "${1:-install}" in
        install)
            check_root
            detect_os
            detect_arch
            install_deps
            get_latest_version
            download_and_install
            create_config
            create_service
            start_service
            print_info
            ;;
        update)
            check_root
            detect_os
            detect_arch
            install_deps
            update
            ;;
        uninstall)
            check_root
            detect_os
            uninstall
            ;;
        help|--help|-h)
            show_help
            ;;
        *)
            error "未知命令: $1"
            ;;
    esac
}

main "$@"
