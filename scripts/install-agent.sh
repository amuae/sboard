#!/bin/bash

# SBoard Agent 一键安装脚本
# 支持: Linux (amd64, 386, arm64, armv7, armv6), Windows (amd64, arm64, 386), macOS (amd64, arm64)
# 
# 支持的 Init 系统:
#   - systemd (Ubuntu, Debian, CentOS, Fedora, Arch 等)
#   - OpenRC (Alpine Linux, Gentoo)
#   - procd (OpenWrt)
#   - init.d/SysVinit (老版本系统)
#
# 用法: curl -fsSL https://your-panel.com/install-agent.sh | bash -s -- --token <token>
# 或者: curl -fsSL https://raw.githubusercontent.com/amuae/sboard/main/scripts/install-agent.sh | bash -s -- --token <token> --panel <panel_url>

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

# GitHub 加速配置 (国内加速)
GH_PROXY="https://ghfast.top/"

# 服务文件路径
SYSTEMD_SERVICE="/etc/systemd/system/${SERVICE_NAME}.service"
OPENRC_SERVICE="/etc/init.d/${SERVICE_NAME}"
PROCD_SERVICE="/etc/init.d/${SERVICE_NAME}"
SYSVINIT_SERVICE="/etc/init.d/${SERVICE_NAME}"

# Init 系统类型
INIT_SYSTEM=""

# 参数
PANEL_URL=""
TOKEN=""

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
    echo "  --uninstall         卸载 Agent"
    echo "  -h, --help          显示帮助"
    echo ""
    echo "示例:"
    echo "  $0 --token abc123 --panel https://panel.example.com"
    echo "  $0 --uninstall"
    echo ""
    echo "支持的平台:"
    echo "  Linux:   amd64, 386, arm64, armv7, armv6"
    echo "  Windows: amd64, arm64, 386"
    echo "  macOS:   amd64, arm64"
    echo ""
    echo "支持的 Init 系统:"
    echo "  - systemd (Ubuntu, Debian, CentOS, Fedora, Arch 等)"
    echo "  - OpenRC (Alpine Linux, Gentoo)"
    echo "  - procd (OpenWrt)"
    echo "  - init.d/SysVinit (老版本系统)"
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
    OS_TYPE=$(uname -s)
    case "$OS_TYPE" in
        Linux|linux)
            OS="linux"
            ;;
        Darwin|darwin)
            OS="darwin"
            ;;
        MINGW*|MSYS*|CYGWIN*|mingw*|msys*|cygwin*)
            OS="windows"
            ;;
        *)
            error "不支持的操作系统: $OS_TYPE，仅支持 Linux, Windows, macOS"
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

# 检测是否为 OpenWrt
is_openwrt() {
    [[ -f /etc/openwrt_release ]] || [[ -f /etc/openwrt_version ]]
}

# 检测 Init 系统
detect_init_system() {
    if [[ "$OS" != "linux" ]]; then
        INIT_SYSTEM="none"
        return
    fi
    
    # 检查 systemd
    if command -v systemctl &> /dev/null && systemctl --version &> /dev/null; then
        # 确认 systemd 是实际运行的 init
        if [[ -d /run/systemd/system ]]; then
            INIT_SYSTEM="systemd"
            info "检测到 Init 系统: systemd"
            return
        fi
    fi
    
    # 检查 OpenWrt procd (优先于 OpenRC)
    if is_openwrt; then
        INIT_SYSTEM="procd"
        info "检测到 Init 系统: OpenWrt procd"
        return
    fi
    
    # 检查 OpenRC (Alpine Linux, Gentoo)
    if command -v rc-service &> /dev/null && command -v rc-update &> /dev/null; then
        INIT_SYSTEM="openrc"
        info "检测到 Init 系统: OpenRC"
        return
    fi
    
    # 检查 SysVinit / init.d
    if [[ -d /etc/init.d ]]; then
        # 检查是否有 update-rc.d (Debian 系) 或 chkconfig (RHEL 系)
        if command -v update-rc.d &> /dev/null || command -v chkconfig &> /dev/null; then
            INIT_SYSTEM="sysvinit"
            info "检测到 Init 系统: SysVinit"
            return
        fi
    fi
    
    warning "未检测到支持的 Init 系统，将无法创建开机自启服务"
    INIT_SYSTEM="none"
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

# 停止服务
stop_service() {
    info "停止现有服务..."
    
    case "$INIT_SYSTEM" in
        systemd)
            if systemctl is-active --quiet "$SERVICE_NAME" 2>/dev/null; then
                systemctl stop "$SERVICE_NAME" || true
            fi
            ;;
        openrc)
            if rc-service "$SERVICE_NAME" status &>/dev/null; then
                rc-service "$SERVICE_NAME" stop || true
            fi
            ;;
        sysvinit)
            if [[ -f "$SYSVINIT_SERVICE" ]]; then
                "$SYSVINIT_SERVICE" stop 2>/dev/null || true
            fi
            ;;
        *)
            # 尝试通过进程名停止
            pkill -f "${BINARY_NAME}" 2>/dev/null || true
            ;;
    esac
}

# 卸载
uninstall() {
    check_root
    detect_os
    detect_init_system
    
    info "开始卸载 SBoard Agent..."

    case "$INIT_SYSTEM" in
        systemd)
            # 停止并禁用服务
            if systemctl is-active --quiet "$SERVICE_NAME" 2>/dev/null; then
                systemctl stop "$SERVICE_NAME"
            fi
            if systemctl is-enabled --quiet "$SERVICE_NAME" 2>/dev/null; then
                systemctl disable "$SERVICE_NAME"
            fi
            # 删除服务文件
            rm -f "$SYSTEMD_SERVICE"
            systemctl daemon-reload
            ;;
        openrc)
            # 停止服务
            rc-service "$SERVICE_NAME" stop 2>/dev/null || true
            # 移除开机启动
            rc-update del "$SERVICE_NAME" default 2>/dev/null || true
            # 删除服务文件
            rm -f "$OPENRC_SERVICE"
            ;;
        procd)
            # 停止服务
            "$PROCD_SERVICE" stop 2>/dev/null || true
            # 禁用开机启动
            "$PROCD_SERVICE" disable 2>/dev/null || true
            # 删除服务文件
            rm -f "$PROCD_SERVICE"
            ;;
        sysvinit)
            # 停止服务
            "$SYSVINIT_SERVICE" stop 2>/dev/null || true
            # 移除开机启动
            if command -v update-rc.d &> /dev/null; then
                update-rc.d -f "$SERVICE_NAME" remove 2>/dev/null || true
            elif command -v chkconfig &> /dev/null; then
                chkconfig --del "$SERVICE_NAME" 2>/dev/null || true
            fi
            # 删除服务文件
            rm -f "$SYSVINIT_SERVICE"
            ;;
        *)
            # 尝试停止进程
            pkill -f "${BINARY_NAME}" 2>/dev/null || true
            ;;
    esac

    # 删除安装目录
    rm -rf "$INSTALL_DIR"

    success "SBoard Agent 已卸载"
}

# 下载 Agent
download_agent() {
    info "下载 Agent..."
    
    # 创建安装目录
    mkdir -p "$INSTALL_DIR"
    
    # 构建下载 URL (直接使用 latest/download/)
    DOWNLOAD_FILE="${BINARY_NAME}_${OS}_${ARCH}.zip"
    DOWNLOAD_URL="${GH_PROXY}https://github.com/${GITHUB_REPO}/releases/latest/download/${DOWNLOAD_FILE}"
    
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
    
    # 获取主机名作为 Agent ID (兼容 OpenWrt)
    if command -v hostname &> /dev/null; then
        AGENT_ID=$(hostname)
    elif [[ -f /proc/sys/kernel/hostname ]]; then
        AGENT_ID=$(cat /proc/sys/kernel/hostname)
    elif [[ -f /etc/hostname ]]; then
        AGENT_ID=$(cat /etc/hostname)
    else
        AGENT_ID="agent-$(date +%s)"
    fi
    
    # 设置核心路径 (固定为 sing-box)
    CORE_PATH="/root/sing-box/sing-box"
    CONFIG_DIR="/root/sing-box"
    
    # 生成配置
    cat > "${INSTALL_DIR}/${CONFIG_FILE}" << EOF
{
    "panel_url": "${PANEL_URL}",
    "token": "${TOKEN}",
    "agent_id": "${AGENT_ID}",
    "core_path": "${CORE_PATH}",
    "config_dir": "${CONFIG_DIR}"
}
EOF
    
    success "配置文件已生成"
}

# 创建 systemd 服务
create_systemd_service() {
    info "创建 systemd 服务..."
    
    cat > "$SYSTEMD_SERVICE" << EOF
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

# 创建 OpenRC 服务 (Alpine Linux, Gentoo)
create_openrc_service() {
    info "创建 OpenRC 服务..."
    
    cat > "$OPENRC_SERVICE" << 'EOF'
#!/sbin/openrc-run

name="SBoard Agent"
description="SBoard Agent - Proxy Node Management"

command="INSTALL_DIR_PLACEHOLDER/BINARY_NAME_PLACEHOLDER"
command_args="-c INSTALL_DIR_PLACEHOLDER/CONFIG_FILE_PLACEHOLDER"
command_background=true
pidfile="/run/${RC_SVCNAME}.pid"
directory="INSTALL_DIR_PLACEHOLDER"

depend() {
    need net
    after firewall
}

start_pre() {
    checkpath --directory --owner root:root --mode 0755 /run
}
EOF

    # 替换占位符
    sed -i "s|INSTALL_DIR_PLACEHOLDER|${INSTALL_DIR}|g" "$OPENRC_SERVICE"
    sed -i "s|BINARY_NAME_PLACEHOLDER|${BINARY_NAME}|g" "$OPENRC_SERVICE"
    sed -i "s|CONFIG_FILE_PLACEHOLDER|${CONFIG_FILE}|g" "$OPENRC_SERVICE"
    
    chmod +x "$OPENRC_SERVICE"
    
    success "OpenRC 服务已创建"
}

# 创建 SysVinit 服务
create_sysvinit_service() {
    info "创建 SysVinit 服务..."
    
    cat > "$SYSVINIT_SERVICE" << 'EOF'
#!/bin/bash
### BEGIN INIT INFO
# Provides:          sboard-agent
# Required-Start:    $network $remote_fs $syslog
# Required-Stop:     $network $remote_fs $syslog
# Default-Start:     2 3 4 5
# Default-Stop:      0 1 6
# Short-Description: SBoard Agent
# Description:       SBoard Agent - Proxy Node Management
### END INIT INFO

NAME="sboard-agent"
DAEMON="INSTALL_DIR_PLACEHOLDER/BINARY_NAME_PLACEHOLDER"
DAEMON_ARGS="-c INSTALL_DIR_PLACEHOLDER/CONFIG_FILE_PLACEHOLDER"
PIDFILE="/var/run/${NAME}.pid"
LOGFILE="/var/log/${NAME}.log"
WORKDIR="INSTALL_DIR_PLACEHOLDER"

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
    start)
        start
        ;;
    stop)
        stop
        ;;
    restart)
        restart
        ;;
    status)
        status
        ;;
    *)
        echo "Usage: $0 {start|stop|restart|status}"
        exit 1
        ;;
esac

exit 0
EOF

    # 替换占位符
    sed -i "s|INSTALL_DIR_PLACEHOLDER|${INSTALL_DIR}|g" "$SYSVINIT_SERVICE"
    sed -i "s|BINARY_NAME_PLACEHOLDER|${BINARY_NAME}|g" "$SYSVINIT_SERVICE"
    sed -i "s|CONFIG_FILE_PLACEHOLDER|${CONFIG_FILE}|g" "$SYSVINIT_SERVICE"
    
    chmod +x "$SYSVINIT_SERVICE"
    
    success "SysVinit 服务已创建"
}

# 创建 OpenWrt procd 服务
create_procd_service() {
    info "创建 OpenWrt procd 服务..."
    
    cat > "$PROCD_SERVICE" << 'EOF'
#!/bin/sh /etc/rc.common

USE_PROCD=1
START=99

NAME="sboard-agent"
PROG="INSTALL_DIR_PLACEHOLDER/BINARY_NAME_PLACEHOLDER"

start_service() {
    mkdir -p INSTALL_DIR_PLACEHOLDER
    procd_open_instance "$NAME"
    procd_set_param command "$PROG" -c INSTALL_DIR_PLACEHOLDER/CONFIG_FILE_PLACEHOLDER
    procd_set_param respawn
    procd_set_param stdout 1
    procd_set_param stderr 1
    procd_set_param pidfile "/var/run/${NAME}.pid"
    procd_close_instance
}

stop_service() {
    service_stop "$PROG"
}

reload_service() {
    stop
    start
}
EOF

    # 替换占位符
    sed -i "s|INSTALL_DIR_PLACEHOLDER|${INSTALL_DIR}|g" "$PROCD_SERVICE"
    sed -i "s|BINARY_NAME_PLACEHOLDER|${BINARY_NAME}|g" "$PROCD_SERVICE"
    sed -i "s|CONFIG_FILE_PLACEHOLDER|${CONFIG_FILE}|g" "$PROCD_SERVICE"
    
    chmod +x "$PROCD_SERVICE"
    
    success "OpenWrt procd 服务已创建"
}

# 创建服务 (根据 Init 系统选择)
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
        procd)
            create_procd_service
            ;;
        sysvinit)
            create_sysvinit_service
            ;;
        *)
            warning "未知的 Init 系统，跳过服务创建"
            warning "请手动启动: ${INSTALL_DIR}/${BINARY_NAME} -c ${INSTALL_DIR}/${CONFIG_FILE}"
            ;;
    esac
}

# 启动服务
start_service() {
    if [[ "$OS" != "linux" ]]; then
        warning "非 Linux 系统，请手动启动: ${INSTALL_DIR}/${BINARY_NAME} -c ${INSTALL_DIR}/${CONFIG_FILE}"
        return
    fi
    
    info "启动服务..."
    
    case "$INIT_SYSTEM" in
        systemd)
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
            ;;
        openrc)
            # 添加开机启动
            rc-update add "$SERVICE_NAME" default
            # 启动服务
            rc-service "$SERVICE_NAME" start
            # 等待启动
            sleep 2
            # 检查状态
            if rc-service "$SERVICE_NAME" status &>/dev/null; then
                success "服务启动成功"
            else
                error "服务启动失败，请查看日志: /var/log/${SERVICE_NAME}.log"
            fi
            ;;
        procd)
            # 启用开机启动
            "$PROCD_SERVICE" enable
            # 启动服务
            "$PROCD_SERVICE" start
            # 等待启动
            sleep 2
            # 检查状态
            if "$PROCD_SERVICE" status &>/dev/null; then
                success "服务启动成功"
            else
                error "服务启动失败，请查看日志: logread | grep ${SERVICE_NAME}"
            fi
            ;;
        sysvinit)
            # 添加开机启动
            if command -v update-rc.d &> /dev/null; then
                update-rc.d "$SERVICE_NAME" defaults
            elif command -v chkconfig &> /dev/null; then
                chkconfig --add "$SERVICE_NAME"
                chkconfig "$SERVICE_NAME" on
            fi
            # 启动服务
            "$SYSVINIT_SERVICE" start
            # 等待启动
            sleep 2
            # 检查状态
            if "$SYSVINIT_SERVICE" status &>/dev/null; then
                success "服务启动成功"
            else
                error "服务启动失败，请查看日志: /var/log/${SERVICE_NAME}.log"
            fi
            ;;
        *)
            warning "无法创建服务，请手动启动: ${INSTALL_DIR}/${BINARY_NAME} -c ${INSTALL_DIR}/${CONFIG_FILE}"
            ;;
    esac
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
    echo "Init 系统: $INIT_SYSTEM"
    echo ""
    
    if [[ "$OS" == "linux" ]]; then
        echo "常用命令:"
        case "$INIT_SYSTEM" in
            systemd)
                echo "  查看状态: systemctl status $SERVICE_NAME"
                echo "  查看日志: journalctl -u $SERVICE_NAME -f"
                echo "  重启服务: systemctl restart $SERVICE_NAME"
                echo "  停止服务: systemctl stop $SERVICE_NAME"
                ;;
            openrc)
                echo "  查看状态: rc-service $SERVICE_NAME status"
                echo "  查看日志: tail -f /var/log/${SERVICE_NAME}.log"
                echo "  重启服务: rc-service $SERVICE_NAME restart"
                echo "  停止服务: rc-service $SERVICE_NAME stop"
                ;;
            procd)
                echo "  查看状态: /etc/init.d/$SERVICE_NAME status"
                echo "  查看日志: logread | grep $SERVICE_NAME"
                echo "  重启服务: /etc/init.d/$SERVICE_NAME restart"
                echo "  停止服务: /etc/init.d/$SERVICE_NAME stop"
                ;;
            sysvinit)
                echo "  查看状态: /etc/init.d/$SERVICE_NAME status"
                echo "  查看日志: tail -f /var/log/${SERVICE_NAME}.log"
                echo "  重启服务: /etc/init.d/$SERVICE_NAME restart"
                echo "  停止服务: /etc/init.d/$SERVICE_NAME stop"
                ;;
        esac
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
    detect_init_system

    # 安装依赖
    install_deps

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
