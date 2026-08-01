#!/bin/bash

# SBoard 面板一键安装脚本
# 支持: Linux (amd64, 386, arm64, armv7, armv6), macOS (amd64, arm64)
#
# 支持的 Init 系统:
#   - systemd (Ubuntu, Debian, CentOS, Fedora, Arch 等)
#   - OpenRC (Alpine Linux, Gentoo)
#   - init.d/SysVinit (老版本系统)
#
# 用法: 
#   curl -fsSL https://raw.githubusercontent.com/amuae/sboard/main/scripts/install-sboard.sh | bash
#   curl -fsSL <url> | bash -s -- --port 9000 --user admin --pass mypassword
#   ./install-sboard.sh --path /opt/sboard --port 8080 --user admin --pass your-password

set -e

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

# 配置 (可通过参数覆盖)
GITHUB_REPO="amuae/sboard"
INSTALL_DIR="/opt/sboard"
DATA_DIR=""  # 默认为 INSTALL_DIR/data
SERVICE_NAME="sboard"
BINARY_NAME="sboard"
LISTEN_PORT=""
ADMIN_USER=""
ADMIN_PASS=""
PANEL_DOMAIN=""
DEV_MODE="false"
DEV_DOMAIN_HASH="9de17c968ada26abec13fc5fc264ddfa"

# GitHub 加速配置 (国内加速)
GH_PROXY="https://ghfast.top/"

# 服务文件路径
SYSTEMD_SERVICE="/etc/systemd/system/${SERVICE_NAME}.service"
OPENRC_SERVICE="/etc/init.d/${SERVICE_NAME}"
SYSVINIT_SERVICE="/etc/init.d/${SERVICE_NAME}"

# Init 系统类型
INIT_SYSTEM=""

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
    echo "SBoard 面板安装脚本"
    echo ""
    echo "用法:"
    echo "  install-sboard.sh [命令] [选项]"
    echo ""
    echo "命令:"
    echo "  (无参数)   进入交互式菜单 (直接运行时)"
    echo "  install    安装 SBoard (管道模式默认)"
    echo "  update     更新 SBoard"
    echo "  uninstall  卸载 SBoard"
    echo "  menu       进入交互式菜单"
    echo "  help       显示帮助"
    echo ""
    echo "选项:"
    echo "  --path <path>       安装路径 (默认: /opt/sboard)"
    echo "  --domain <domain>   面板入口域名"
    echo "  --port <port>       监听端口 (不指定则随机 5000-65535)"
    echo "  --user <user>       管理员用户名"
    echo "  --pass <pass>       管理员密码"
    echo "  --dev               强制使用预发布版本"
    echo "  -h, --help          显示帮助"
    echo ""
    echo "示例:"
    echo "  # 交互式安装"
    echo "  curl -fsSL <url> | bash"
    echo ""
    echo "  # 指定参数安装"
    echo "  curl -fsSL <url> | bash -s -- --domain panel.example.com --port 9000 --user admin --pass mypassword"
    echo ""
    echo "支持的平台:"
    echo "  Linux: amd64, 386, arm64, armv7, armv6"
    echo "  macOS: amd64, arm64"
    echo ""
    echo "支持的 Init 系统:"
    echo "  - systemd (Ubuntu, Debian, CentOS, Fedora, Arch 等)"
    echo "  - OpenRC (Alpine Linux, Gentoo)"
    echo "  - init.d/SysVinit (老版本系统)"
    echo ""
}

# 生成随机端口 (5000-65535)
generate_random_port() {
    local port
    while true; do
        port=$((RANDOM % 60536 + 5000))
        # 检查端口是否被占用
        if ! ss -tuln 2>/dev/null | grep -q ":${port} " && \
           ! netstat -tuln 2>/dev/null | grep -q ":${port} "; then
            echo "$port"
            return
        fi
    done
}

# 生成非交互安装使用的随机管理员密码
generate_random_password() {
    LC_ALL=C tr -dc 'a-zA-Z0-9' < /dev/urandom | head -c 20
}

# 检查域名是否为开发者域名
check_dev_domain() {
    local domain="$1"
    if [[ -z "$domain" ]]; then
        return
    fi
    
    # 计算域名的 MD5
    local domain_hash
    if command -v md5sum &> /dev/null; then
        domain_hash=$(echo -n "$domain" | md5sum | cut -d' ' -f1)
    elif command -v md5 &> /dev/null; then
        domain_hash=$(echo -n "$domain" | md5)
    else
        return
    fi
    
    # 检查是否匹配开发者域名
    if [[ "$domain_hash" == "$DEV_DOMAIN_HASH" ]]; then
        DEV_MODE="true"
    fi
}

# 检查是否可交互（终端可用且可读写）
can_interact() {
    # 标准输入是终端
    if [[ -t 0 ]]; then
        return 0
    fi
    # 或者 /dev/tty 可用（尝试读取测试）
    if [[ -e /dev/tty ]] && (echo "" > /dev/tty) 2>/dev/null; then
        return 0
    fi
    return 1
}

# 解析命令行参数
parse_args() {
    # 如果直接从终端运行且没有参数，显示菜单
    if [[ $# -eq 0 ]] && can_interact; then
        COMMAND="menu"
        return
    fi
    
    # 默认命令为 install (用于管道模式)
    COMMAND="install"
    
    while [[ $# -gt 0 ]]; do
        case $1 in
            install|update|uninstall|menu)
                COMMAND="$1"
                shift
                ;;
            --path)
                INSTALL_DIR="$2"
                shift 2
                ;;
            --domain)
                PANEL_DOMAIN="$2"
                check_dev_domain "$2"
                shift 2
                ;;
            --port)
                LISTEN_PORT="$2"
                shift 2
                ;;
            --user)
                ADMIN_USER="$2"
                shift 2
                ;;
            --pass)
                ADMIN_PASS="$2"
                shift 2
                ;;
            --dev)
                DEV_MODE="true"
                shift
                ;;
            -h|--help|help)
                show_help
                exit 0
                ;;
            *)
                error "未知参数: $1，使用 --help 查看帮助"
                ;;
        esac
    done
    
    # 设置数据目录
    if [[ -z "$DATA_DIR" ]]; then
        DATA_DIR="${INSTALL_DIR}/data"
    fi
}

# 交互式配置
interactive_config() {
    # 检查是否可以交互
    if ! can_interact; then
        warning "检测到非交互模式，使用默认配置"
        warning "如需自定义配置，请使用参数: --domain <domain> --port <port> --user <user> --pass <pass>"
        
        # 使用默认值
        if [[ -z "$LISTEN_PORT" ]]; then
            LISTEN_PORT=$(generate_random_port)
            info "随机生成端口: $LISTEN_PORT"
        fi
        if [[ -z "$ADMIN_USER" ]]; then
            ADMIN_USER="admin"
        fi
        if [[ -z "$ADMIN_PASS" ]]; then
            ADMIN_PASS=$(generate_random_password)
            info "已生成随机管理员密码，安装完成后会显示"
        fi
        return
    fi
    
    echo ""
    echo -e "${CYAN}==========================================${NC}"
    echo -e "${CYAN}        SBoard 面板安装向导${NC}"
    echo -e "${CYAN}==========================================${NC}"
    echo ""
    
    # 步骤 1: 安装路径
    echo -e "${YELLOW}[1/5]${NC} 设置安装路径"
    read -p "安装路径 [${INSTALL_DIR}]: " input </dev/tty
    if [[ -n "$input" ]]; then
        INSTALL_DIR="$input"
    fi
    DATA_DIR="${INSTALL_DIR}/data"
    echo ""
    
    # 步骤 2: 面板入口域名
    echo -e "${YELLOW}[2/5]${NC} 设置面板入口域名"
    echo -e "  ${BLUE}提示:${NC} 用于访问面板的域名，如 panel.example.com"
    while [[ -z "$PANEL_DOMAIN" ]]; do
        read -p "面板域名: " PANEL_DOMAIN </dev/tty
        if [[ -z "$PANEL_DOMAIN" ]]; then
            echo -e "  ${RED}域名不能为空，请重新输入${NC}"
        fi
    done
    check_dev_domain "$PANEL_DOMAIN"
    echo ""
    
    # 步骤 3: 监听端口
    echo -e "${YELLOW}[3/5]${NC} 设置监听端口"
    echo -e "  ${BLUE}提示:${NC} 直接回车将随机生成 5000-65535 之间的端口"
    read -p "监听端口 [随机]: " input </dev/tty
    if [[ -n "$input" ]]; then
        # 验证端口号
        if [[ ! "$input" =~ ^[0-9]+$ ]] || [[ "$input" -lt 1 ]] || [[ "$input" -gt 65535 ]]; then
            error "无效的端口号: $input"
        fi
        LISTEN_PORT="$input"
    else
        LISTEN_PORT=$(generate_random_port)
        info "随机生成端口: $LISTEN_PORT"
    fi
    echo ""
    
    # 步骤 4: 管理员账户
    echo -e "${YELLOW}[4/5]${NC} 设置管理员账户"
    while [[ -z "$ADMIN_USER" ]]; do
        read -p "管理员用户名: " ADMIN_USER </dev/tty
        if [[ -z "$ADMIN_USER" ]]; then
            echo -e "  ${RED}用户名不能为空，请重新输入${NC}"
        fi
    done
    echo ""
    
    # 步骤 5: 管理员密码
    echo -e "${YELLOW}[5/5]${NC} 设置管理员密码"
    while true; do
        read -s -p "管理员密码: " ADMIN_PASS </dev/tty
        echo ""
        if [[ -z "$ADMIN_PASS" ]]; then
            echo -e "  ${RED}密码不能为空，请重新输入${NC}"
            continue
        fi
        if [[ ${#ADMIN_PASS} -lt 6 ]]; then
            echo -e "  ${RED}密码长度至少 6 位，请重新输入${NC}"
            continue
        fi
        
        read -s -p "确认密码: " confirm_pass </dev/tty
        echo ""
        if [[ "$ADMIN_PASS" != "$confirm_pass" ]]; then
            echo -e "  ${RED}两次输入的密码不一致，请重新输入${NC}"
            ADMIN_PASS=""
            continue
        fi
        break
    done
    echo ""
    
    # 确认配置
    echo -e "${CYAN}==========================================${NC}"
    echo -e "${CYAN}          请确认安装配置${NC}"
    echo -e "${CYAN}==========================================${NC}"
    echo ""
    echo -e "  安装路径: ${GREEN}${INSTALL_DIR}${NC}"
    echo -e "  数据目录: ${GREEN}${DATA_DIR}${NC}"
    echo -e "  面板域名: ${GREEN}${PANEL_DOMAIN}${NC}"
    echo -e "  监听端口: ${GREEN}${LISTEN_PORT}${NC}"
    echo -e "  管理员:   ${GREEN}${ADMIN_USER}${NC}"
    echo -e "  密码:     ${GREEN}******${NC}"
    if [[ "$DEV_MODE" == "true" ]]; then
        echo -e "  版本:     ${YELLOW}预发布版本${NC}"
    fi
    echo ""
    read -p "确认安装? [Y/n]: " confirm </dev/tty
    if [[ "$confirm" =~ ^[Nn] ]]; then
        echo "安装已取消"
        exit 0
    fi
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
        *)
            error "不支持的操作系统: $OS_TYPE，仅支持 Linux 和 macOS"
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
            error "不支持的架构: $ARCH_TYPE，支持: amd64, 386, arm64, armv7, armv6"
            ;;
    esac
    info "检测到架构: $ARCH"
}

# 检测 Init 系统
detect_init_system() {
    if [[ "$OS" != "linux" ]]; then
        INIT_SYSTEM="none"
        return
    fi
    
    # 检查 systemd
    if command -v systemctl &> /dev/null && systemctl --version &> /dev/null; then
        if [[ -d /run/systemd/system ]]; then
            INIT_SYSTEM="systemd"
            info "检测到 Init 系统: systemd"
            return
        fi
    fi
    
    # 检查 OpenRC
    if command -v rc-service &> /dev/null && command -v rc-update &> /dev/null; then
        INIT_SYSTEM="openrc"
        info "检测到 Init 系统: OpenRC"
        return
    fi
    
    # 检查 SysVinit
    if [[ -d /etc/init.d ]]; then
        if command -v update-rc.d &> /dev/null || command -v chkconfig &> /dev/null; then
            INIT_SYSTEM="sysvinit"
            info "检测到 Init 系统: SysVinit"
            return
        fi
    fi
    
    warning "未检测到支持的 Init 系统"
    INIT_SYSTEM="none"
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

# 下载并安装
download_and_install() {
    info "下载 SBoard..."
    
    # 构建下载 URL
    DOWNLOAD_FILE="${BINARY_NAME}_${OS}_${ARCH}.zip"
    if [[ "$DEV_MODE" == "true" ]]; then
        # 开发者模式：使用预发布版本
        warning "开发者模式：使用预发布版本"
        DOWNLOAD_URL="${GH_PROXY}https://github.com/${GITHUB_REPO}/releases/download/pre-release/${DOWNLOAD_FILE}"
    else
        # 正常模式：使用最新正式版
        DOWNLOAD_URL="${GH_PROXY}https://github.com/${GITHUB_REPO}/releases/latest/download/${DOWNLOAD_FILE}"
    fi
    
    # 创建临时目录
    TMP_DIR=$(mktemp -d)
    cd "$TMP_DIR"
    
    info "下载: $DOWNLOAD_URL"
    if ! curl -fsSL -L --connect-timeout 30 "$DOWNLOAD_URL" -o "${DOWNLOAD_FILE}"; then
        rm -rf "$TMP_DIR"
        error "下载失败，请检查网络或 ${OS}_${ARCH} 构建是否存在"
    fi
    info "下载成功"
    
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
    
    # 生成随机 JWT 密钥
    JWT_SECRET=$(head -c 32 /dev/urandom | base64 | tr -dc 'a-zA-Z0-9' | head -c 32)
    
    # 有域名时监听本地（配合 nginx 反代），无域名时监听所有接口
    local listen_addr="0.0.0.0"
    if [[ -n "$PANEL_DOMAIN" && "$PANEL_DOMAIN" != "localhost" ]]; then
        listen_addr="127.0.0.1"
    fi
    
    cat > "${DATA_DIR}/config.yaml" << EOF
# SBoard 配置文件
# 生成时间: $(date '+%Y-%m-%d %H:%M:%S')

server:
  listen: "${listen_addr}:${LISTEN_PORT}"
  debug: false
  domain: "${PANEL_DOMAIN}"

data:
  dir: "${DATA_DIR}"

security:
  jwt_secret: "${JWT_SECRET}"
  jwt_expire_hour: 168
  session_name: "sboard_token"

oauth:
  disable_password_login: false
EOF

    chmod 600 "${DATA_DIR}/config.yaml"
    success "配置文件创建完成: ${DATA_DIR}/config.yaml"
}

# 初始化管理员账户
init_admin() {
    info "初始化管理员账户..."

    # 运行 sboard 初始化管理员
    local output
    if ! output=$("${INSTALL_DIR}/${BINARY_NAME}" -d "${DATA_DIR}" -init-admin -admin-user "${ADMIN_USER}" -admin-pass "${ADMIN_PASS}" 2>&1); then
        error "管理员账户初始化失败: ${output}"
    fi

    success "管理员账户初始化完成"
}

# 创建 systemd 服务
create_systemd_service() {
    info "创建 systemd 服务..."
    
    cat > "$SYSTEMD_SERVICE" << EOF
[Unit]
Description=SBoard - Server Monitoring Panel
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

# 创建 OpenRC 服务
create_openrc_service() {
    info "创建 OpenRC 服务..."
    
    cat > "$OPENRC_SERVICE" << EOF
#!/sbin/openrc-run

name="SBoard Panel"
description="SBoard - Server Monitoring Panel"

command="${INSTALL_DIR}/${BINARY_NAME}"
command_args="-d ${DATA_DIR}"
command_background=true
pidfile="/run/\${RC_SVCNAME}.pid"
directory="${DATA_DIR}"

depend() {
    need net
    after firewall
}

start_pre() {
    checkpath --directory --owner root:root --mode 0755 /run
}
EOF

    chmod +x "$OPENRC_SERVICE"
    rc-update add ${SERVICE_NAME} default
    
    success "OpenRC 服务创建完成"
}

# 创建 SysVinit 服务
create_sysvinit_service() {
    info "创建 SysVinit 服务..."
    
    cat > "$SYSVINIT_SERVICE" << EOF
#!/bin/bash
### BEGIN INIT INFO
# Provides:          ${SERVICE_NAME}
# Required-Start:    \$network \$remote_fs
# Required-Stop:     \$network \$remote_fs
# Default-Start:     2 3 4 5
# Default-Stop:      0 1 6
# Short-Description: SBoard Panel
# Description:       SBoard - Server Monitoring Panel
### END INIT INFO

DAEMON="${INSTALL_DIR}/${BINARY_NAME}"
DAEMON_ARGS="-d ${DATA_DIR}"
PIDFILE="/var/run/${SERVICE_NAME}.pid"
LOGFILE="/var/log/${SERVICE_NAME}.log"

case "\$1" in
    start)
        echo "Starting ${SERVICE_NAME}..."
        start-stop-daemon --start --background --make-pidfile --pidfile \$PIDFILE --exec \$DAEMON -- \$DAEMON_ARGS >> \$LOGFILE 2>&1
        ;;
    stop)
        echo "Stopping ${SERVICE_NAME}..."
        start-stop-daemon --stop --pidfile \$PIDFILE
        rm -f \$PIDFILE
        ;;
    restart)
        \$0 stop
        sleep 1
        \$0 start
        ;;
    status)
        if [ -f \$PIDFILE ] && kill -0 \$(cat \$PIDFILE) 2>/dev/null; then
            echo "${SERVICE_NAME} is running"
        else
            echo "${SERVICE_NAME} is not running"
            exit 1
        fi
        ;;
    *)
        echo "Usage: \$0 {start|stop|restart|status}"
        exit 1
        ;;
esac

exit 0
EOF

    chmod +x "$SYSVINIT_SERVICE"
    
    if command -v update-rc.d &> /dev/null; then
        update-rc.d ${SERVICE_NAME} defaults
    elif command -v chkconfig &> /dev/null; then
        chkconfig --add ${SERVICE_NAME}
        chkconfig ${SERVICE_NAME} on
    fi
    
    success "SysVinit 服务创建完成"
}

# 创建服务
create_service() {
    case "$INIT_SYSTEM" in
        systemd)
            create_systemd_service
            ;;
        openrc)
            create_openrc_service
            ;;
        sysvinit)
            create_sysvinit_service
            ;;
        none)
            warning "未检测到 Init 系统，跳过服务创建"
            warning "请手动运行: ${INSTALL_DIR}/${BINARY_NAME} -d ${DATA_DIR}"
            ;;
    esac
}

# 启动服务
start_service() {
    info "启动 SBoard 服务..."
    
    case "$INIT_SYSTEM" in
        systemd)
            systemctl start ${SERVICE_NAME}
            ;;
        openrc)
            rc-service ${SERVICE_NAME} start
            ;;
        sysvinit)
            ${SYSVINIT_SERVICE} start
            ;;
        none)
            warning "请手动启动服务"
            return
            ;;
    esac
    
    success "服务已启动"
}

# 停止服务
stop_service() {
    info "停止 SBoard 服务..."
    
    case "$INIT_SYSTEM" in
        systemd)
            systemctl stop ${SERVICE_NAME} 2>/dev/null || true
            ;;
        openrc)
            rc-service ${SERVICE_NAME} stop 2>/dev/null || true
            ;;
        sysvinit)
            ${SYSVINIT_SERVICE} stop 2>/dev/null || true
            ;;
    esac
}

# 创建 sboard 管理命令
create_sboard_command() {
    info "创建 sboard 管理命令..."
    
    cat > /usr/local/bin/sboard << 'SBOARD_CMD'
#!/bin/bash
# SBoard 管理命令
# 用法: sboard [命令]
#   sboard          - 显示管理菜单
#   sboard start    - 启动服务
#   sboard stop     - 停止服务
#   sboard restart  - 重启服务
#   sboard status   - 查看状态
#   sboard logs     - 查看日志
#   sboard update   - 更新面板
#   sboard uninstall - 卸载面板

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
NC='\033[0m'

SERVICE_NAME="sboard"
INSTALL_DIR="/opt/sboard"

# 检测 init 系统
detect_init_system() {
    if command -v systemctl &> /dev/null && systemctl --version &> /dev/null; then
        INIT_SYSTEM="systemd"
    elif command -v rc-service &> /dev/null; then
        INIT_SYSTEM="openrc"
    elif [[ -f /etc/init.d/${SERVICE_NAME} ]]; then
        INIT_SYSTEM="sysvinit"
    else
        INIT_SYSTEM="none"
    fi
}

do_start() {
    detect_init_system
    case "$INIT_SYSTEM" in
        systemd) systemctl start ${SERVICE_NAME} ;;
        openrc) rc-service ${SERVICE_NAME} start ;;
        sysvinit) /etc/init.d/${SERVICE_NAME} start ;;
        *) echo -e "${RED}无法启动服务${NC}"; exit 1 ;;
    esac
    echo -e "${GREEN}服务已启动${NC}"
}

do_stop() {
    detect_init_system
    case "$INIT_SYSTEM" in
        systemd) systemctl stop ${SERVICE_NAME} ;;
        openrc) rc-service ${SERVICE_NAME} stop ;;
        sysvinit) /etc/init.d/${SERVICE_NAME} stop ;;
        *) echo -e "${RED}无法停止服务${NC}"; exit 1 ;;
    esac
    echo -e "${GREEN}服务已停止${NC}"
}

do_restart() {
    detect_init_system
    case "$INIT_SYSTEM" in
        systemd) systemctl restart ${SERVICE_NAME} ;;
        openrc) rc-service ${SERVICE_NAME} restart ;;
        sysvinit) /etc/init.d/${SERVICE_NAME} restart ;;
        *) echo -e "${RED}无法重启服务${NC}"; exit 1 ;;
    esac
    echo -e "${GREEN}服务已重启${NC}"
}

do_status() {
    detect_init_system
    case "$INIT_SYSTEM" in
        systemd) systemctl status ${SERVICE_NAME} ;;
        openrc) rc-service ${SERVICE_NAME} status ;;
        sysvinit) /etc/init.d/${SERVICE_NAME} status ;;
        *) echo -e "${RED}无法获取服务状态${NC}"; exit 1 ;;
    esac
}

do_logs() {
    detect_init_system
    if [[ "$INIT_SYSTEM" == "systemd" ]]; then
        journalctl -u ${SERVICE_NAME} -f --no-pager
    else
        echo -e "${YELLOW}日志位置: /var/log/sboard.log${NC}"
        if [[ -f /var/log/sboard.log ]]; then
            tail -f /var/log/sboard.log
        else
            echo -e "${RED}日志文件不存在${NC}"
        fi
    fi
}

do_update() {
    echo -e "${CYAN}正在更新 SBoard...${NC}"
    local tmp_script=$(mktemp)
    if curl -fsSL -o "$tmp_script" "https://ghfast.top/https://raw.githubusercontent.com/amuae/sboard/main/scripts/install-sboard.sh"; then
        chmod +x "$tmp_script"
        bash "$tmp_script" update
        rm -f "$tmp_script"
    else
        echo -e "${RED}下载更新脚本失败${NC}"
        rm -f "$tmp_script"
    fi
}

do_uninstall() {
    echo -e "${CYAN}正在卸载 SBoard...${NC}"
    local tmp_script=$(mktemp)
    if curl -fsSL -o "$tmp_script" "https://ghfast.top/https://raw.githubusercontent.com/amuae/sboard/main/scripts/install-sboard.sh"; then
        chmod +x "$tmp_script"
        bash "$tmp_script" uninstall
        rm -f "$tmp_script"
    else
        echo -e "${RED}下载卸载脚本失败${NC}"
        rm -f "$tmp_script"
    fi
}

show_menu() {
    while true; do
        echo ""
        echo -e "${CYAN}==========================================${NC}"
        echo -e "${CYAN}        SBoard 面板管理${NC}"
        echo -e "${CYAN}==========================================${NC}"
        echo ""
        echo "  1) 启动服务"
        echo "  2) 停止服务"
        echo "  3) 重启服务"
        echo "  4) 查看状态"
        echo "  5) 查看日志"
        echo "  6) 更新面板"
        echo "  7) 卸载面板"
        echo "  0) 退出"
        echo ""
        read -p "请选择 [0-7]: " choice
        
        case $choice in
            1) do_start ;;
            2) do_stop ;;
            3) do_restart ;;
            4) do_status ;;
            5) do_logs ;;
            6) do_update; break ;;
            7) do_uninstall; break ;;
            0) echo "再见!"; exit 0 ;;
            *) echo -e "${RED}无效选择${NC}" ;;
        esac
    done
}

# 主入口
case "${1:-}" in
    start) do_start ;;
    stop) do_stop ;;
    restart) do_restart ;;
    status) do_status ;;
    logs) do_logs ;;
    update) do_update ;;
    uninstall) do_uninstall ;;
    help|--help|-h)
        echo "SBoard 管理命令"
        echo ""
        echo "用法: sboard [命令]"
        echo ""
        echo "命令:"
        echo "  start      启动服务"
        echo "  stop       停止服务"
        echo "  restart    重启服务"
        echo "  status     查看状态"
        echo "  logs       查看日志"
        echo "  update     更新面板"
        echo "  uninstall  卸载面板"
        echo "  (无参数)   显示管理菜单"
        ;;
    "") show_menu ;;
    *)
        echo -e "${RED}未知命令: $1${NC}"
        echo "使用 'sboard help' 查看帮助"
        exit 1
        ;;
esac
SBOARD_CMD

    chmod +x /usr/local/bin/sboard
    success "sboard 命令已创建"
}

# 显示安装完成信息
show_install_info() {
    echo ""
    echo -e "${GREEN}==========================================${NC}"
    echo -e "${GREEN}        SBoard 安装完成!${NC}"
    echo -e "${GREEN}==========================================${NC}"
    echo ""
    echo -e "  安装路径: ${CYAN}${INSTALL_DIR}${NC}"
    echo -e "  数据目录: ${CYAN}${DATA_DIR}${NC}"
    echo -e "  配置文件: ${CYAN}${DATA_DIR}/config.yaml${NC}"
    echo ""
    echo -e "  面板域名: ${CYAN}${PANEL_DOMAIN}${NC}"
    echo -e "  监听端口: ${CYAN}${LISTEN_PORT}${NC}"
    
    if [[ -n "$PANEL_DOMAIN" && "$PANEL_DOMAIN" != "localhost" ]]; then
        echo -e "  监听地址: ${CYAN}127.0.0.1:${LISTEN_PORT}${NC} (仅本地)"
        echo -e "  访问地址: ${CYAN}https://${PANEL_DOMAIN}${NC} (需配置反向代理)"
    else
        echo -e "  监听地址: ${CYAN}0.0.0.0:${LISTEN_PORT}${NC}"
        echo -e "  访问地址: ${CYAN}http://服务器IP:${LISTEN_PORT}${NC}"
    fi
    echo ""
    echo -e "${YELLOW}==========================================${NC}"
    echo -e "${YELLOW}        管理员账户信息 (请牢记!)${NC}"
    echo -e "${YELLOW}==========================================${NC}"
    echo -e "  用户名: ${CYAN}${ADMIN_USER}${NC}"
    echo -e "  密码:   ${CYAN}${ADMIN_PASS}${NC}"
    echo ""
    echo -e "${RED}  重要提示:${NC}"
    echo -e "  - 管理员账户只能初始化一次，无法通过命令行修改"
    echo -e "  - 登录后可在 Web 界面修改密码"
    echo -e "  - 如忘记密码，需删除数据库文件后重新安装"
    echo ""
    
    echo -e "  管理命令: ${YELLOW}sboard${NC} (显示管理菜单)"
    echo ""
    if [[ "$INIT_SYSTEM" != "none" ]]; then
        echo -e "  服务管理命令:"
        case "$INIT_SYSTEM" in
            systemd)
                echo -e "    启动: ${YELLOW}systemctl start ${SERVICE_NAME}${NC}"
                echo -e "    停止: ${YELLOW}systemctl stop ${SERVICE_NAME}${NC}"
                echo -e "    重启: ${YELLOW}systemctl restart ${SERVICE_NAME}${NC}"
                echo -e "    状态: ${YELLOW}systemctl status ${SERVICE_NAME}${NC}"
                ;;
            openrc)
                echo -e "    启动: ${YELLOW}rc-service ${SERVICE_NAME} start${NC}"
                echo -e "    停止: ${YELLOW}rc-service ${SERVICE_NAME} stop${NC}"
                echo -e "    重启: ${YELLOW}rc-service ${SERVICE_NAME} restart${NC}"
                echo -e "    状态: ${YELLOW}rc-service ${SERVICE_NAME} status${NC}"
                ;;
            sysvinit)
                echo -e "    启动: ${YELLOW}${SYSVINIT_SERVICE} start${NC}"
                echo -e "    停止: ${YELLOW}${SYSVINIT_SERVICE} stop${NC}"
                echo -e "    重启: ${YELLOW}${SYSVINIT_SERVICE} restart${NC}"
                echo -e "    状态: ${YELLOW}${SYSVINIT_SERVICE} status${NC}"
                ;;
        esac
    fi
    echo ""
    if [[ -n "$PANEL_DOMAIN" && "$PANEL_DOMAIN" != "localhost" ]]; then
        echo -e "${YELLOW}==========================================${NC}"
        echo -e "${YELLOW}        Nginx 反向代理配置示例${NC}"
        echo -e "${YELLOW}==========================================${NC}"
        echo -e "server {"
        echo -e "    listen 80;"
        echo -e "    listen 443 ssl http2;"
        echo -e "    server_name ${PANEL_DOMAIN};"
        echo -e "    "
        echo -e "    location / {"
        echo -e "        proxy_pass http://127.0.0.1:${LISTEN_PORT};"
        echo -e "        proxy_set_header Host \$host;"
        echo -e "        proxy_set_header X-Real-IP \$remote_addr;"
        echo -e "        proxy_set_header X-Forwarded-For \$proxy_add_x_forwarded_for;"
        echo -e "        proxy_set_header X-Forwarded-Proto \$scheme;"
        echo -e "        proxy_http_version 1.1;"
        echo -e "        proxy_set_header Upgrade \$http_upgrade;"
        echo -e "        proxy_set_header Connection \"upgrade\";"
        echo -e "    }"
        echo -e "}"
        echo ""
        echo -e "${YELLOW}提示: 前端会验证访问域名是否与配置的域名一致${NC}"
        echo -e "${YELLOW}      其他域名访问将显示警告${NC}"
    else
        echo -e "${YELLOW}提示: 请确保防火墙已开放端口 ${LISTEN_PORT}${NC}"
    fi
    echo ""
}

# 安装流程
do_install() {
    echo ""
    echo -e "${CYAN}==========================================${NC}"
    echo -e "${CYAN}        SBoard 面板安装${NC}"
    echo -e "${CYAN}==========================================${NC}"
    echo ""
    
    check_root
    detect_os
    detect_arch
    detect_init_system
    
    # 交互式配置
    interactive_config
    
    # 检查是否已安装
    if [[ -f "${INSTALL_DIR}/${BINARY_NAME}" ]]; then
        warning "检测到已安装 SBoard"
        if can_interact; then
            read -p "是否覆盖安装? [y/N]: " confirm </dev/tty
            if [[ ! "$confirm" =~ ^[Yy] ]]; then
                echo "安装已取消"
                exit 0
            fi
        fi
        stop_service
    fi
    
    install_deps
    download_and_install
    create_config
    init_admin
    create_service
    create_sboard_command
    start_service
    
    show_install_info
}

# 更新流程
do_update() {
    echo ""
    echo -e "${CYAN}==========================================${NC}"
    echo -e "${CYAN}        SBoard 面板更新${NC}"
    echo -e "${CYAN}==========================================${NC}"
    echo ""
    
    check_root
    
    # 检查是否已安装
    if [[ ! -f "${INSTALL_DIR}/${BINARY_NAME}" ]]; then
        error "未检测到 SBoard 安装，请先安装"
    fi
    
    # 读取现有配置
    if [[ -f "${DATA_DIR}/config.yaml" ]]; then
        info "读取现有配置..."
        # 从配置文件读取域名用于判断是否使用预发布版本
        local config_domain=$(grep -E "^\s*domain:" "${DATA_DIR}/config.yaml" 2>/dev/null | head -1 | sed 's/.*domain:\s*"\?\([^"]*\)"\?.*/\1/')
        if [[ -n "$config_domain" ]]; then
            PANEL_DOMAIN="$config_domain"
            check_dev_domain "$config_domain"
        fi
    fi
    
    detect_os
    detect_arch
    detect_init_system
    
    install_deps

    # 下载失败或新版本无法启动时保留旧二进制并自动回滚，避免更新流程
    # 把正在运行的面板留在停机状态。
    stop_service
    local backup_path="${INSTALL_DIR}/${BINARY_NAME}.bak"
    rm -f "$backup_path"
    if [[ -f "${INSTALL_DIR}/${BINARY_NAME}" ]]; then
        mv "${INSTALL_DIR}/${BINARY_NAME}" "$backup_path"
    fi

    if ! (download_and_install); then
        rm -f "${INSTALL_DIR}/${BINARY_NAME}"
        if [[ -f "$backup_path" ]]; then
            mv "$backup_path" "${INSTALL_DIR}/${BINARY_NAME}"
        fi
        start_service || true
        error "下载新版本失败，已恢复旧版本"
    fi
    
    # 更新 sboard 管理命令
    create_sboard_command
    
    if ! start_service; then
        rm -f "${INSTALL_DIR}/${BINARY_NAME}"
        if [[ -f "$backup_path" ]]; then
            mv "$backup_path" "${INSTALL_DIR}/${BINARY_NAME}"
        fi
        start_service || true
        error "新版本启动失败，已恢复旧版本"
    fi
    
    # 删除备份
    rm -f "${INSTALL_DIR}/${BINARY_NAME}.bak"
    
    echo ""
    success "SBoard 更新完成!"
    echo ""
}

# 卸载流程
do_uninstall() {
    echo ""
    echo -e "${CYAN}==========================================${NC}"
    echo -e "${CYAN}        SBoard 面板卸载${NC}"
    echo -e "${CYAN}==========================================${NC}"
    echo ""
    
    check_root
    detect_init_system
    
    # 确认
    if can_interact; then
        echo -e "${RED}警告: 此操作将删除 SBoard 及所有数据!${NC}"
        read -p "确认卸载? [y/N]: " confirm </dev/tty
        if [[ ! "$confirm" =~ ^[Yy] ]]; then
            echo "卸载已取消"
            exit 0
        fi
    fi
    
    # 停止服务
    stop_service
    
    # 删除服务
    case "$INIT_SYSTEM" in
        systemd)
            systemctl disable ${SERVICE_NAME} 2>/dev/null || true
            rm -f "$SYSTEMD_SERVICE"
            systemctl daemon-reload
            ;;
        openrc)
            rc-update del ${SERVICE_NAME} default 2>/dev/null || true
            rm -f "$OPENRC_SERVICE"
            ;;
        sysvinit)
            if command -v update-rc.d &> /dev/null; then
                update-rc.d -f ${SERVICE_NAME} remove 2>/dev/null || true
            elif command -v chkconfig &> /dev/null; then
                chkconfig --del ${SERVICE_NAME} 2>/dev/null || true
            fi
            rm -f "$SYSVINIT_SERVICE"
            ;;
    esac
    
    # 删除文件
    rm -rf "$INSTALL_DIR"
    
    # 删除 sboard 命令
    rm -f /usr/local/bin/sboard
    
    echo ""
    success "SBoard 卸载完成!"
    echo ""
}

# 交互式菜单
show_menu() {
    while true; do
        echo ""
        echo -e "${CYAN}==========================================${NC}"
        echo -e "${CYAN}        SBoard 面板管理脚本${NC}"
        echo -e "${CYAN}==========================================${NC}"
        echo ""
        echo "  1) 安装 SBoard"
        echo "  2) 更新 SBoard"
        echo "  3) 卸载 SBoard"
        echo "  4) 查看状态"
        echo "  5) 重启服务"
        echo "  0) 退出"
        echo ""
        read -p "请选择 [0-5]: " choice </dev/tty
        
        case $choice in
            1)
                do_install
                ;;
            2)
                do_update
                ;;
            3)
                do_uninstall
                break
                ;;
            4)
                detect_init_system
                case "$INIT_SYSTEM" in
                    systemd)
                        systemctl status ${SERVICE_NAME}
                        ;;
                    openrc)
                        rc-service ${SERVICE_NAME} status
                        ;;
                    sysvinit)
                        ${SYSVINIT_SERVICE} status
                        ;;
                    *)
                        warning "无法获取服务状态"
                        ;;
                esac
                ;;
            5)
                detect_init_system
                case "$INIT_SYSTEM" in
                    systemd)
                        systemctl restart ${SERVICE_NAME}
                        success "服务已重启"
                        ;;
                    openrc)
                        rc-service ${SERVICE_NAME} restart
                        success "服务已重启"
                        ;;
                    sysvinit)
                        ${SYSVINIT_SERVICE} restart
                        success "服务已重启"
                        ;;
                    *)
                        warning "无法重启服务"
                        ;;
                esac
                ;;
            0)
                echo "再见!"
                exit 0
                ;;
            *)
                warning "无效选择"
                ;;
        esac
    done
}

# 主入口
main() {
    parse_args "$@"
    
    case "$COMMAND" in
        install)
            do_install
            ;;
        update)
            do_update
            ;;
        uninstall)
            do_uninstall
            ;;
        menu)
            show_menu
            ;;
        *)
            show_help
            ;;
    esac
}

# 执行
main "$@"
