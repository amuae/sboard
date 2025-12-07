#!/bin/bash

# SBoard Go 版本一键安装脚本
# 支持: Debian 10+, Ubuntu 18.04+, CentOS 7+
# 用法: curl -fsSL https://your-domain.com/install.sh | bash

set -e

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# 配置
GITHUB_REPO="sboard-go/sboard"
INSTALL_DIR="/opt/sboard"
CONFIG_DIR="/etc/sboard"
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

# 检测系统
detect_os() {
    if [[ -f /etc/os-release ]]; then
        . /etc/os-release
        OS=$ID
        VERSION=$VERSION_ID
    elif [[ -f /etc/redhat-release ]]; then
        OS="centos"
    else
        error "不支持的操作系统"
    fi
    
    info "检测到系统: $OS $VERSION"
}

# 检测架构
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

# 安装依赖
install_deps() {
    info "安装依赖..."
    case $OS in
        ubuntu|debian)
            apt-get update -qq
            apt-get install -y -qq curl wget tar
            ;;
        centos|rhel|fedora)
            yum install -y -q curl wget tar
            ;;
        *)
            warning "无法自动安装依赖，请手动安装: curl wget tar"
            ;;
    esac
}

# 获取最新版本
get_latest_version() {
    info "获取最新版本..."
    LATEST_VERSION=$(curl -fsSL "https://api.github.com/repos/${GITHUB_REPO}/releases/latest" | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')
    if [[ -z "$LATEST_VERSION" ]]; then
        LATEST_VERSION="v3.0.0"
        warning "无法获取最新版本，使用默认版本: $LATEST_VERSION"
    fi
    info "最新版本: $LATEST_VERSION"
}

# 下载并安装
download_and_install() {
    info "下载 SBoard..."
    
    DOWNLOAD_URL="https://github.com/${GITHUB_REPO}/releases/download/${LATEST_VERSION}/${BINARY_NAME}-linux-${ARCH}.tar.gz"
    
    # 创建临时目录
    TMP_DIR=$(mktemp -d)
    cd "$TMP_DIR"
    
    # 下载
    if ! curl -fsSL "$DOWNLOAD_URL" -o sboard.tar.gz; then
        # 如果下载失败，尝试从源码构建
        warning "下载失败，尝试从源码构建..."
        build_from_source
        return
    fi
    
    # 解压
    tar -xzf sboard.tar.gz
    
    # 创建目录
    mkdir -p "$INSTALL_DIR"
    mkdir -p "$CONFIG_DIR"
    mkdir -p "$DATA_DIR"
    
    # 安装二进制文件
    mv "${BINARY_NAME}" "${INSTALL_DIR}/"
    chmod +x "${INSTALL_DIR}/${BINARY_NAME}"
    
    # 清理
    cd /
    rm -rf "$TMP_DIR"
    
    success "SBoard 安装完成"
}

# 从源码构建
build_from_source() {
    info "从源码构建..."
    
    # 检查 Go
    if ! command -v go &> /dev/null; then
        info "安装 Go..."
        install_golang
    fi
    
    # 克隆源码
    TMP_DIR=$(mktemp -d)
    cd "$TMP_DIR"
    
    git clone "https://github.com/${GITHUB_REPO}.git" sboard
    cd sboard
    
    # 构建
    go build -o "${INSTALL_DIR}/${BINARY_NAME}" ./cmd/sboard
    
    # 清理
    cd /
    rm -rf "$TMP_DIR"
    
    success "从源码构建完成"
}

# 安装 Go
install_golang() {
    GO_VERSION="1.21.5"
    GO_URL="https://go.dev/dl/go${GO_VERSION}.linux-${ARCH}.tar.gz"
    
    curl -fsSL "$GO_URL" -o /tmp/go.tar.gz
    rm -rf /usr/local/go
    tar -C /usr/local -xzf /tmp/go.tar.gz
    rm /tmp/go.tar.gz
    
    export PATH=$PATH:/usr/local/go/bin
    echo 'export PATH=$PATH:/usr/local/go/bin' >> /etc/profile
}

# 创建配置文件
create_config() {
    if [[ -f "${CONFIG_DIR}/config.yaml" ]]; then
        warning "配置文件已存在，跳过创建"
        return
    fi
    
    info "创建配置文件..."
    
    # 生成随机 JWT 密钥
    JWT_SECRET=$(head -c 32 /dev/urandom | base64 | tr -dc 'a-zA-Z0-9' | head -c 32)
    
    cat > "${CONFIG_DIR}/config.yaml" << EOF
# SBoard 配置文件

server:
  listen: "0.0.0.0:8080"
  debug: false

data:
  database: "${DATA_DIR}/sboard.db"

security:
  jwt_secret: "${JWT_SECRET}"
  jwt_expire_hour: 168  # 7 天
  session_name: "sboard_token"

ssh:
  timeout: 30
  key_path: ""

core:
  sing_box_path: "/etc/sing-box"
  mihomo_path: "/etc/mihomo"
EOF

    chmod 600 "${CONFIG_DIR}/config.yaml"
    success "配置文件创建完成: ${CONFIG_DIR}/config.yaml"
}

# 创建 systemd 服务
create_service() {
    info "创建 systemd 服务..."
    
    cat > /etc/systemd/system/${SERVICE_NAME}.service << EOF
[Unit]
Description=SBoard - Proxy Management Panel
Documentation=https://github.com/${GITHUB_REPO}
After=network.target

[Service]
Type=simple
User=root
ExecStart=${INSTALL_DIR}/${BINARY_NAME} -c ${CONFIG_DIR}/config.yaml
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
    echo ""
    echo "=========================================="
    echo -e "${GREEN}SBoard 安装成功!${NC}"
    echo "=========================================="
    echo ""
    echo "安装目录: ${INSTALL_DIR}"
    echo "配置文件: ${CONFIG_DIR}/config.yaml"
    echo "数据目录: ${DATA_DIR}"
    echo ""
    echo "管理命令:"
    echo "  启动: systemctl start ${SERVICE_NAME}"
    echo "  停止: systemctl stop ${SERVICE_NAME}"
    echo "  重启: systemctl restart ${SERVICE_NAME}"
    echo "  状态: systemctl status ${SERVICE_NAME}"
    echo "  日志: journalctl -u ${SERVICE_NAME} -f"
    echo ""
    echo "访问地址: http://$(hostname -I | awk '{print $1}'):8080"
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
    
    # 停止服务
    systemctl stop ${SERVICE_NAME} 2>/dev/null || true
    systemctl disable ${SERVICE_NAME} 2>/dev/null || true
    
    # 删除文件
    rm -f /etc/systemd/system/${SERVICE_NAME}.service
    rm -rf "${INSTALL_DIR}"
    
    # 询问是否删除配置和数据
    read -p "是否删除配置文件和数据? [y/N]: " -n 1 -r
    echo
    if [[ $REPLY =~ ^[Yy]$ ]]; then
        rm -rf "${CONFIG_DIR}"
        rm -rf "${DATA_DIR}"
    fi
    
    systemctl daemon-reload
    
    success "SBoard 卸载完成"
}

# 更新
update() {
    info "更新 SBoard..."
    
    # 停止服务
    systemctl stop ${SERVICE_NAME}
    
    # 获取最新版本并下载
    get_latest_version
    download_and_install
    
    # 启动服务
    start_service
    
    success "SBoard 更新完成"
}

# 显示帮助
show_help() {
    echo "SBoard 安装脚本"
    echo ""
    echo "用法:"
    echo "  install.sh [命令]"
    echo ""
    echo "命令:"
    echo "  install   安装 SBoard (默认)"
    echo "  update    更新 SBoard"
    echo "  uninstall 卸载 SBoard"
    echo "  help      显示帮助"
    echo ""
}

# 主函数
main() {
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
            detect_arch
            update
            ;;
        uninstall)
            check_root
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
