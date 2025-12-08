#!/bin/bash

# SBoard 面板一键安装脚本
# 支持: Linux (amd64, arm64, armv7), macOS (amd64, arm64)
#
# 支持的 Init 系统:
#   - systemd (Ubuntu, Debian, CentOS, Fedora, Arch 等)
#   - OpenRC (Alpine Linux, Gentoo)
#   - init.d/SysVinit (老版本系统)
#
# 用法: 
#   curl -fsSL https://raw.githubusercontent.com/amuae/sboard/main/scripts/install-sboard.sh | bash
#   curl -fsSL <url> | bash -s -- --port 9000 --user admin --pass mypassword
#   ./install-sboard.sh --path /opt/sboard --port 8080 --user admin --pass admin123

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
LISTEN_PORT="8080"
ADMIN_USER=""
ADMIN_PASS=""
INTERACTIVE="true"  # 是否交互式

# GitHub 加速配置 (国内加速，支持 API 和下载)
GH_PROXY="https://gh.llkk.cc/"
GH_PROXY_API="https://gh.llkk.cc/"
# 备用加速: https://ghfast.top/ https://gh-proxy.com/ https://mirror.ghproxy.com/

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
    echo "  install     安装 SBoard (默认)"
    echo "  update      更新 SBoard"
    echo "  uninstall   卸载 SBoard"
    echo "  help        显示帮助"
    echo ""
    echo "选项:"
    echo "  --path <path>     安装路径 (默认: /opt/sboard)"
    echo "  --port <port>     监听端口 (默认: 8080)"
    echo "  --user <user>     管理员用户名 (默认: admin)"
    echo "  --pass <pass>     管理员密码 (默认: admin123)"
    echo "  --no-interactive  非交互模式，使用默认值或指定参数"
    echo "  -h, --help        显示帮助"
    echo ""
    echo "示例:"
    echo "  # 交互式安装"
    echo "  curl -fsSL <url> | bash"
    echo ""
    echo "  # 指定参数安装"
    echo "  curl -fsSL <url> | bash -s -- --port 9000 --user admin --pass mypassword"
    echo ""
    echo "  # 非交互式安装（使用默认值）"
    echo "  curl -fsSL <url> | bash -s -- --no-interactive"
    echo ""
    echo "支持的平台:"
    echo "  Linux: amd64, arm64, armv7"
    echo "  macOS: amd64, arm64"
    echo ""
    echo "支持的 Init 系统:"
    echo "  - systemd (Ubuntu, Debian, CentOS, Fedora, Arch 等)"
    echo "  - OpenRC (Alpine Linux, Gentoo)"
    echo "  - init.d/SysVinit (老版本系统)"
    echo ""
}

# 解析命令行参数
parse_args() {
    COMMAND="install"
    
    while [[ $# -gt 0 ]]; do
        case $1 in
            install|update|uninstall)
                COMMAND="$1"
                shift
                ;;
            --path)
                INSTALL_DIR="$2"
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
            --no-interactive)
                INTERACTIVE="false"
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
    if [[ "$INTERACTIVE" != "true" ]]; then
        # 非交互模式，使用默认值
        if [[ -z "$ADMIN_USER" ]]; then
            ADMIN_USER="admin"
        fi
        if [[ -z "$ADMIN_PASS" ]]; then
            ADMIN_PASS="admin123"
        fi
        return
    fi
    
    echo ""
    echo -e "${CYAN}========================================${NC}"
    echo -e "${CYAN}         SBoard 安装配置${NC}"
    echo -e "${CYAN}========================================${NC}"
    echo ""
    
    # 安装路径
    read -p "安装路径 [${INSTALL_DIR}]: " input
    if [[ -n "$input" ]]; then
        INSTALL_DIR="$input"
        DATA_DIR="${INSTALL_DIR}/data"
    fi
    
    # 监听端口
    read -p "监听端口 [${LISTEN_PORT}]: " input
    if [[ -n "$input" ]]; then
        LISTEN_PORT="$input"
    fi
    
    # 管理员用户名
    default_user="${ADMIN_USER:-admin}"
    read -p "管理员用户名 [${default_user}]: " input
    if [[ -n "$input" ]]; then
        ADMIN_USER="$input"
    else
        ADMIN_USER="$default_user"
    fi
    
    # 管理员密码
    default_pass="${ADMIN_PASS:-admin123}"
    read -s -p "管理员密码 [${default_pass}]: " input
    echo ""
    if [[ -n "$input" ]]; then
        ADMIN_PASS="$input"
    else
        ADMIN_PASS="$default_pass"
    fi
    
    # 确认配置
    echo ""
    echo -e "${YELLOW}请确认安装配置:${NC}"
    echo "  安装路径: ${INSTALL_DIR}"
    echo "  数据目录: ${DATA_DIR}"
    echo "  监听端口: ${LISTEN_PORT}"
    echo "  管理员: ${ADMIN_USER}"
    echo "  密码: ******"
    echo ""
    read -p "确认安装? [Y/n]: " confirm
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
        aarch64|arm64)
            ARCH="arm64"
            ;;
        armv7l|armv7)
            ARCH="armv7"
            ;;
        *)
            error "不支持的架构: $ARCH_TYPE，仅支持 amd64, arm64, armv7"
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

# 获取最新版本
get_latest_version() {
    info "获取最新版本..."
    
    LATEST_VERSION=$(curl -fsSL --connect-timeout 10 "${GH_PROXY_API}https://api.github.com/repos/${GITHUB_REPO}/releases/latest" 2>/dev/null | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')
    
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
    DOWNLOAD_URL="${GH_PROXY}https://github.com/${GITHUB_REPO}/releases/download/${LATEST_VERSION}/${DOWNLOAD_FILE}"
    
    # 创建临时目录
    TMP_DIR=$(mktemp -d)
    cd "$TMP_DIR"
    
    info "下载: $DOWNLOAD_URL"
    if ! curl -fsSL --connect-timeout 30 "$DOWNLOAD_URL" -o "${DOWNLOAD_FILE}"; then
        rm -rf "$TMP_DIR"
        error "下载失败，请检查版本 ${LATEST_VERSION} 是否存在 ${OS}_${ARCH} 构建"
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
    
    cat > "${DATA_DIR}/config.yaml" << EOF
# SBoard 配置文件

server:
  listen: "0.0.0.0:${LISTEN_PORT}"
  debug: false

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
    "${INSTALL_DIR}/${BINARY_NAME}" -d "${DATA_DIR}" -init-admin -admin-user "${ADMIN_USER}" -admin-pass "${ADMIN_PASS}" 2>/dev/null || true
    
    success "管理员账户初始化完成"
}

# 创建 systemd 服务
create_systemd_service() {
    info "创建 systemd 服务..."
    
    cat > "$SYSTEMD_SERVICE" << EOF
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

# 创建 OpenRC 服务
create_openrc_service() {
    info "创建 OpenRC 服务..."
    
    cat > "$OPENRC_SERVICE" << 'EOF'
#!/sbin/openrc-run

name="SBoard Panel"
description="SBoard - Proxy Management Panel"

command="INSTALL_DIR_PLACEHOLDER/BINARY_NAME_PLACEHOLDER"
command_args="-d DATA_DIR_PLACEHOLDER"
command_background=true
pidfile="/run/${RC_SVCNAME}.pid"
directory="DATA_DIR_PLACEHOLDER"

depend() {
    need net
    after firewall
}

start_pre() {
    checkpath --directory --owner root:root --mode 0755 /run
}
EOF

    sed -i "s|INSTALL_DIR_PLACEHOLDER|${INSTALL_DIR}|g" "$OPENRC_SERVICE"
    sed -i "s|BINARY_NAME_PLACEHOLDER|${BINARY_NAME}|g" "$OPENRC_SERVICE"
    sed -i "s|DATA_DIR_PLACEHOLDER|${DATA_DIR}|g" "$OPENRC_SERVICE"
    
    chmod +x "$OPENRC_SERVICE"
    rc-update add ${SERVICE_NAME} default
    
    success "OpenRC 服务创建完成"
}

# 创建 SysVinit 服务
create_sysvinit_service() {
    info "创建 SysVinit 服务..."
    
    cat > "$SYSVINIT_SERVICE" << 'EOF'
#!/bin/bash
### BEGIN INIT INFO
# Provides:          sboard
# Required-Start:    $network $remote_fs $syslog
# Required-Stop:     $network $remote_fs $syslog
# Default-Start:     2 3 4 5
# Default-Stop:      0 1 6
# Short-Description: SBoard Panel
# Description:       SBoard - Proxy Management Panel
### END INIT INFO

NAME="sboard"
DAEMON="INSTALL_DIR_PLACEHOLDER/BINARY_NAME_PLACEHOLDER"
DAEMON_ARGS="-d DATA_DIR_PLACEHOLDER"
PIDFILE="/var/run/${NAME}.pid"
LOGFILE="/var/log/${NAME}.log"
WORKDIR="DATA_DIR_PLACEHOLDER"

start() {
    if [ -f "$PIDFILE" ] && kill -0 $(cat "$PIDFILE") 2>/dev/null; then
        echo "$NAME is already running"
        return 1
    fi
    echo "Starting $NAME..."
    cd "$WORKDIR"
    nohup "$DAEMON" $DAEMON_ARGS >> "$LOGFILE" 2>&1 &
    echo $! > "$PIDFILE"
    echo "$NAME started"
}

stop() {
    if [ ! -f "$PIDFILE" ]; then
        echo "$NAME is not running"
        return 1
    fi
    echo "Stopping $NAME..."
    kill $(cat "$PIDFILE") 2>/dev/null
    rm -f "$PIDFILE"
    echo "$NAME stopped"
}

restart() {
    stop
    sleep 2
    start
}

status() {
    if [ -f "$PIDFILE" ] && kill -0 $(cat "$PIDFILE") 2>/dev/null; then
        echo "$NAME is running (PID: $(cat $PIDFILE))"
    else
        echo "$NAME is not running"
        return 1
    fi
}

case "$1" in
    start)   start ;;
    stop)    stop ;;
    restart) restart ;;
    status)  status ;;
    *)
        echo "Usage: $0 {start|stop|restart|status}"
        exit 1
        ;;
esac
exit 0
EOF

    sed -i "s|INSTALL_DIR_PLACEHOLDER|${INSTALL_DIR}|g" "$SYSVINIT_SERVICE"
    sed -i "s|BINARY_NAME_PLACEHOLDER|${BINARY_NAME}|g" "$SYSVINIT_SERVICE"
    sed -i "s|DATA_DIR_PLACEHOLDER|${DATA_DIR}|g" "$SYSVINIT_SERVICE"
    
    chmod +x "$SYSVINIT_SERVICE"
    
    if command -v update-rc.d &> /dev/null; then
        update-rc.d ${SERVICE_NAME} defaults
    elif command -v chkconfig &> /dev/null; then
        chkconfig --add ${SERVICE_NAME}
        chkconfig ${SERVICE_NAME} on
    fi
    
    success "SysVinit 服务创建完成"
}

# 创建服务 (仅 Linux)
create_service() {
    if [[ "$OS" != "linux" ]]; then
        warning "非 Linux 系统，跳过服务创建"
        return
    fi
    
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
        *)
            warning "未知的 Init 系统，跳过服务创建"
            warning "请手动启动: ${INSTALL_DIR}/${BINARY_NAME} -d ${DATA_DIR}"
            ;;
    esac
}

# 启动服务
start_service() {
    if [[ "$OS" != "linux" ]]; then
        warning "非 Linux 系统，请手动启动: ${INSTALL_DIR}/${BINARY_NAME} -d ${DATA_DIR}"
        return
    fi
    
    info "启动 SBoard..."
    
    case "$INIT_SYSTEM" in
        systemd)
            systemctl start ${SERVICE_NAME}
            sleep 2
            if systemctl is-active --quiet ${SERVICE_NAME}; then
                success "SBoard 启动成功"
            else
                error "SBoard 启动失败，请检查日志: journalctl -u ${SERVICE_NAME}"
            fi
            ;;
        openrc)
            rc-service ${SERVICE_NAME} start
            sleep 2
            if rc-service ${SERVICE_NAME} status &>/dev/null; then
                success "SBoard 启动成功"
            else
                error "SBoard 启动失败，请检查日志: /var/log/${SERVICE_NAME}.log"
            fi
            ;;
        sysvinit)
            "$SYSVINIT_SERVICE" start
            sleep 2
            if "$SYSVINIT_SERVICE" status &>/dev/null; then
                success "SBoard 启动成功"
            else
                error "SBoard 启动失败，请检查日志: /var/log/${SERVICE_NAME}.log"
            fi
            ;;
        *)
            warning "请手动启动: ${INSTALL_DIR}/${BINARY_NAME} -d ${DATA_DIR}"
            ;;
    esac
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
    echo "监听端口: ${LISTEN_PORT}"
    echo "Init 系统: ${INIT_SYSTEM}"
    echo ""
    if [[ "$OS" == "linux" ]]; then
        echo "管理命令:"
        case "$INIT_SYSTEM" in
            systemd)
                echo "  启动: systemctl start ${SERVICE_NAME}"
                echo "  停止: systemctl stop ${SERVICE_NAME}"
                echo "  重启: systemctl restart ${SERVICE_NAME}"
                echo "  状态: systemctl status ${SERVICE_NAME}"
                echo "  日志: journalctl -u ${SERVICE_NAME} -f"
                ;;
            openrc)
                echo "  启动: rc-service ${SERVICE_NAME} start"
                echo "  停止: rc-service ${SERVICE_NAME} stop"
                echo "  重启: rc-service ${SERVICE_NAME} restart"
                echo "  状态: rc-service ${SERVICE_NAME} status"
                echo "  日志: tail -f /var/log/${SERVICE_NAME}.log"
                ;;
            sysvinit)
                echo "  启动: /etc/init.d/${SERVICE_NAME} start"
                echo "  停止: /etc/init.d/${SERVICE_NAME} stop"
                echo "  重启: /etc/init.d/${SERVICE_NAME} restart"
                echo "  状态: /etc/init.d/${SERVICE_NAME} status"
                echo "  日志: tail -f /var/log/${SERVICE_NAME}.log"
                ;;
        esac
        echo ""
    fi
    echo -e "访问地址: ${CYAN}http://${LOCAL_IP}:${LISTEN_PORT}${NC}"
    echo ""
    echo -e "${YELLOW}登录信息:${NC}"
    echo "  用户名: ${ADMIN_USER}"
    echo "  密码:   ${ADMIN_PASS}"
    echo ""
    if [[ "$ADMIN_PASS" == "admin123" ]]; then
        echo -e "${RED}警告: 您使用的是默认密码，请务必及时修改!${NC}"
        echo ""
    fi
    echo -e "${GREEN}快捷命令:${NC} 输入 ${CYAN}sboard${NC} 可呼出管理菜单"
    echo ""
}

# 创建快捷命令
create_shortcut() {
    info "创建快捷命令 sboard..."
    
    # 快捷命令脚本路径
    SHORTCUT_PATH="/usr/local/bin/sboard"
    
    # 创建快捷命令脚本
    cat > "$SHORTCUT_PATH" << 'SHORTCUTEOF'
#!/bin/bash

# SBoard 管理菜单
# 快捷命令: sboard

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m'

# 配置
INSTALL_DIR="/opt/sboard"
DATA_DIR="/opt/sboard/data"
SERVICE_NAME="sboard"

# 检测 Init 系统
detect_init_system() {
    if command -v systemctl &> /dev/null && [[ -d /run/systemd/system ]]; then
        echo "systemd"
    elif command -v rc-service &> /dev/null; then
        echo "openrc"
    elif [[ -d /etc/init.d ]]; then
        echo "sysvinit"
    else
        echo "unknown"
    fi
}

INIT_SYSTEM=$(detect_init_system)

# 获取服务状态
get_status() {
    case "$INIT_SYSTEM" in
        systemd)
            if systemctl is-active --quiet $SERVICE_NAME; then
                echo -e "${GREEN}运行中${NC}"
            else
                echo -e "${RED}已停止${NC}"
            fi
            ;;
        openrc)
            if rc-service $SERVICE_NAME status &>/dev/null; then
                echo -e "${GREEN}运行中${NC}"
            else
                echo -e "${RED}已停止${NC}"
            fi
            ;;
        sysvinit)
            if /etc/init.d/$SERVICE_NAME status &>/dev/null; then
                echo -e "${GREEN}运行中${NC}"
            else
                echo -e "${RED}已停止${NC}"
            fi
            ;;
        *)
            echo -e "${YELLOW}未知${NC}"
            ;;
    esac
}

# 启动服务
do_start() {
    echo -e "${BLUE}启动 SBoard...${NC}"
    case "$INIT_SYSTEM" in
        systemd) systemctl start $SERVICE_NAME ;;
        openrc) rc-service $SERVICE_NAME start ;;
        sysvinit) /etc/init.d/$SERVICE_NAME start ;;
    esac
    sleep 1
    echo -e "状态: $(get_status)"
}

# 停止服务
do_stop() {
    echo -e "${BLUE}停止 SBoard...${NC}"
    case "$INIT_SYSTEM" in
        systemd) systemctl stop $SERVICE_NAME ;;
        openrc) rc-service $SERVICE_NAME stop ;;
        sysvinit) /etc/init.d/$SERVICE_NAME stop ;;
    esac
    sleep 1
    echo -e "状态: $(get_status)"
}

# 重启服务
do_restart() {
    echo -e "${BLUE}重启 SBoard...${NC}"
    case "$INIT_SYSTEM" in
        systemd) systemctl restart $SERVICE_NAME ;;
        openrc) rc-service $SERVICE_NAME restart ;;
        sysvinit) /etc/init.d/$SERVICE_NAME restart ;;
    esac
    sleep 1
    echo -e "状态: $(get_status)"
}

# 查看日志
do_logs() {
    echo -e "${BLUE}查看日志 (Ctrl+C 退出)...${NC}"
    case "$INIT_SYSTEM" in
        systemd) journalctl -u $SERVICE_NAME -f ;;
        *) tail -f /var/log/${SERVICE_NAME}.log ;;
    esac
}

# 更新 SBoard
do_update() {
    echo -e "${BLUE}更新 SBoard...${NC}"
    curl -fsSL https://raw.githubusercontent.com/amuae/sboard/main/scripts/install-sboard.sh | bash -s update
}

# 卸载 SBoard
do_uninstall() {
    echo -e "${YELLOW}确定要卸载 SBoard 吗? [y/N]${NC}"
    read -r confirm
    if [[ "$confirm" =~ ^[Yy]$ ]]; then
        curl -fsSL https://raw.githubusercontent.com/amuae/sboard/main/scripts/install-sboard.sh | bash -s uninstall
    fi
}

# 显示配置信息
do_info() {
    echo ""
    echo -e "${CYAN}========== SBoard 信息 ==========${NC}"
    if [[ -f "$INSTALL_DIR/sboard" ]]; then
        VERSION=$($INSTALL_DIR/sboard -v 2>/dev/null || echo "未知")
        echo -e "版本: ${GREEN}$VERSION${NC}"
    fi
    echo -e "状态: $(get_status)"
    echo -e "安装目录: $INSTALL_DIR"
    echo -e "数据目录: $DATA_DIR"
    echo -e "Init 系统: $INIT_SYSTEM"
    
    # 获取本机 IP
    LOCAL_IP=$(hostname -I 2>/dev/null | awk '{print $1}' || echo "localhost")
    echo -e "访问地址: ${GREEN}http://${LOCAL_IP}:8080${NC}"
    echo ""
}

# 显示菜单
show_menu() {
    clear
    echo -e "${CYAN}"
    echo "  ____  ____                      _ "
    echo " / ___|| __ )  ___   __ _ _ __ __| |"
    echo " \\___ \\|  _ \\ / _ \\ / _\` | '__/ _\` |"
    echo "  ___) | |_) | (_) | (_| | | | (_| |"
    echo " |____/|____/ \\___/ \\__,_|_|  \\__,_|"
    echo -e "${NC}"
    echo -e "${CYAN}========== SBoard 管理面板 ==========${NC}"
    echo ""
    echo -e "  当前状态: $(get_status)"
    echo ""
    echo -e "  ${GREEN}1.${NC} 启动 SBoard"
    echo -e "  ${GREEN}2.${NC} 停止 SBoard"
    echo -e "  ${GREEN}3.${NC} 重启 SBoard"
    echo -e "  ${GREEN}4.${NC} 查看日志"
    echo -e "  ${GREEN}5.${NC} 查看信息"
    echo -e "  ${YELLOW}6.${NC} 更新 SBoard"
    echo -e "  ${RED}7.${NC} 卸载 SBoard"
    echo -e "  ${BLUE}0.${NC} 退出"
    echo ""
    echo -e "${CYAN}=====================================${NC}"
    echo ""
}

# 主循环
main() {
    while true; do
        show_menu
        read -p "请选择操作 [0-7]: " choice
        echo ""
        case "$choice" in
            1) do_start; read -p "按回车键继续..." ;;
            2) do_stop; read -p "按回车键继续..." ;;
            3) do_restart; read -p "按回车键继续..." ;;
            4) do_logs ;;
            5) do_info; read -p "按回车键继续..." ;;
            6) do_update; read -p "按回车键继续..." ;;
            7) do_uninstall; break ;;
            0) echo -e "${GREEN}再见!${NC}"; exit 0 ;;
            *) echo -e "${RED}无效选择${NC}"; sleep 1 ;;
        esac
    done
}

main
SHORTCUTEOF
    
    chmod +x "$SHORTCUT_PATH"
    success "快捷命令创建完成，输入 'sboard' 即可呼出管理菜单"
}

# 删除快捷命令
remove_shortcut() {
    if [[ -f "/usr/local/bin/sboard" ]]; then
        rm -f "/usr/local/bin/sboard"
        info "已删除快捷命令"
    fi
}

# 卸载
uninstall() {
    info "卸载 SBoard..."
    
    if [[ "$OS" == "linux" ]]; then
        case "$INIT_SYSTEM" in
            systemd)
                systemctl stop ${SERVICE_NAME} 2>/dev/null || true
                systemctl disable ${SERVICE_NAME} 2>/dev/null || true
                rm -f "$SYSTEMD_SERVICE"
                systemctl daemon-reload
                ;;
            openrc)
                rc-service ${SERVICE_NAME} stop 2>/dev/null || true
                rc-update del ${SERVICE_NAME} default 2>/dev/null || true
                rm -f "$OPENRC_SERVICE"
                ;;
            sysvinit)
                "$SYSVINIT_SERVICE" stop 2>/dev/null || true
                if command -v update-rc.d &> /dev/null; then
                    update-rc.d -f ${SERVICE_NAME} remove 2>/dev/null || true
                elif command -v chkconfig &> /dev/null; then
                    chkconfig --del ${SERVICE_NAME} 2>/dev/null || true
                fi
                rm -f "$SYSVINIT_SERVICE"
                ;;
            *)
                pkill -f "${BINARY_NAME}" 2>/dev/null || true
                ;;
        esac
    fi
    
    # 删除安装目录
    rm -rf "${INSTALL_DIR}"
    
    # 删除快捷命令
    remove_shortcut
    
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
        case "$INIT_SYSTEM" in
            systemd)
                systemctl stop ${SERVICE_NAME} 2>/dev/null || true
                ;;
            openrc)
                rc-service ${SERVICE_NAME} stop 2>/dev/null || true
                ;;
            sysvinit)
                "$SYSVINIT_SERVICE" stop 2>/dev/null || true
                ;;
        esac
    fi
    
    get_latest_version
    download_and_install
    
    if [[ "$OS" == "linux" ]]; then
        case "$INIT_SYSTEM" in
            systemd)
                systemctl start ${SERVICE_NAME}
                ;;
            openrc)
                rc-service ${SERVICE_NAME} start
                ;;
            sysvinit)
                "$SYSVINIT_SERVICE" start
                ;;
        esac
    fi
    
    success "SBoard 更新到 ${LATEST_VERSION}"
}

# 主函数
main() {
    # 解析命令行参数
    parse_args "$@"
    
    echo ""
    echo "=========================================="
    echo "       SBoard 一键安装脚本"
    echo "=========================================="
    echo ""
    
    case "$COMMAND" in
        install)
            check_root
            detect_os
            detect_arch
            detect_init_system
            interactive_config
            install_deps
            get_latest_version
            download_and_install
            create_config
            init_admin
            create_service
            create_shortcut
            start_service
            print_info
            ;;
        update)
            check_root
            detect_os
            detect_arch
            detect_init_system
            install_deps
            update
            ;;
        uninstall)
            check_root
            detect_os
            detect_init_system
            uninstall
            ;;
        *)
            error "未知命令: $COMMAND"
            ;;
    esac
}

main "$@"
