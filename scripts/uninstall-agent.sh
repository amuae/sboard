#!/bin/bash

# SBoard Agent 一键卸载脚本
# 卸载 Agent 及其部署的所有核心服务 (sing-box, mihomo)
#
# 支持的 Init 系统:
#   - systemd (Ubuntu, Debian, CentOS, Fedora, Arch 等)
#   - OpenRC (Alpine Linux, Gentoo)
#   - procd (OpenWrt)
#   - init.d/SysVinit (老版本系统)
#
# 用法: curl -fsSL https://raw.githubusercontent.com/amuae/sboard/main/scripts/uninstall-agent.sh | bash

set -e

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m'

# 配置
AGENT_SERVICE="sboard-agent"
CORE_SERVICES=("sing-box" "mihomo")
AGENT_INSTALL_DIR="/opt/sboard-agent"
CORE_INSTALL_DIRS=("/root/sing-box" "/root/mihomo" "/opt/sing-box" "/opt/mihomo")

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

# 检查 root 权限
check_root() {
    if [[ $EUID -ne 0 ]]; then
        error "请使用 root 权限运行此脚本"
    fi
}

# 检测是否为 OpenWrt
is_openwrt() {
    [[ -f /etc/openwrt_release ]] || [[ -f /etc/openwrt_version ]]
}

# 检测 Init 系统
detect_init_system() {
    # 检查 systemd
    if command -v systemctl &> /dev/null && [[ -d /run/systemd/system ]]; then
        INIT_SYSTEM="systemd"
        info "检测到 Init 系统: systemd"
        return
    fi
    
    # 检查 OpenWrt procd
    if is_openwrt; then
        INIT_SYSTEM="procd"
        info "检测到 Init 系统: procd (OpenWrt)"
        return
    fi
    
    # 检查 OpenRC
    if command -v rc-service &> /dev/null && command -v rc-update &> /dev/null; then
        INIT_SYSTEM="openrc"
        info "检测到 Init 系统: OpenRC"
        return
    fi
    
    # 检查 SysVinit
    if [[ -d /etc/init.d ]]; then
        INIT_SYSTEM="sysvinit"
        info "检测到 Init 系统: SysVinit"
        return
    fi
    
    warning "未检测到支持的 Init 系统"
    INIT_SYSTEM="unknown"
}

# 停止并卸载 systemd 服务
uninstall_systemd_service() {
    local service_name=$1
    
    if systemctl list-unit-files | grep -q "^${service_name}.service"; then
        info "停止服务: $service_name"
        systemctl stop "$service_name" 2>/dev/null || true
        systemctl disable "$service_name" 2>/dev/null || true
        rm -f "/etc/systemd/system/${service_name}.service"
        success "已卸载 systemd 服务: $service_name"
    fi
}

# 停止并卸载 OpenRC 服务
uninstall_openrc_service() {
    local service_name=$1
    
    if [[ -f "/etc/init.d/$service_name" ]]; then
        info "停止服务: $service_name"
        rc-service "$service_name" stop 2>/dev/null || true
        rc-update del "$service_name" default 2>/dev/null || true
        rm -f "/etc/init.d/$service_name"
        success "已卸载 OpenRC 服务: $service_name"
    fi
}

# 停止并卸载 procd 服务 (OpenWrt)
uninstall_procd_service() {
    local service_name=$1
    
    if [[ -f "/etc/init.d/$service_name" ]]; then
        info "停止服务: $service_name"
        /etc/init.d/"$service_name" stop 2>/dev/null || true
        /etc/init.d/"$service_name" disable 2>/dev/null || true
        rm -f "/etc/init.d/$service_name"
        # 删除 rc.d 软链接
        rm -f /etc/rc.d/*"$service_name"
        success "已卸载 procd 服务: $service_name"
    fi
}

# 停止并卸载 SysVinit 服务
uninstall_sysvinit_service() {
    local service_name=$1
    
    if [[ -f "/etc/init.d/$service_name" ]]; then
        info "停止服务: $service_name"
        /etc/init.d/"$service_name" stop 2>/dev/null || true
        
        if command -v update-rc.d &> /dev/null; then
            update-rc.d -f "$service_name" remove 2>/dev/null || true
        elif command -v chkconfig &> /dev/null; then
            chkconfig --del "$service_name" 2>/dev/null || true
        fi
        
        rm -f "/etc/init.d/$service_name"
        success "已卸载 SysVinit 服务: $service_name"
    fi
}

# 卸载服务 (根据 Init 系统)
uninstall_service() {
    local service_name=$1
    
    case "$INIT_SYSTEM" in
        systemd)
            uninstall_systemd_service "$service_name"
            ;;
        openrc)
            uninstall_openrc_service "$service_name"
            ;;
        procd)
            uninstall_procd_service "$service_name"
            ;;
        sysvinit)
            uninstall_sysvinit_service "$service_name"
            ;;
        *)
            # 尝试杀死进程
            pkill -f "$service_name" 2>/dev/null || true
            rm -f "/etc/init.d/$service_name" 2>/dev/null || true
            ;;
    esac
}

# 删除目录
remove_directory() {
    local dir=$1
    
    if [[ -d "$dir" ]]; then
        info "删除目录: $dir"
        rm -rf "$dir"
        success "已删除: $dir"
    fi
}

# 显示将要卸载的内容
show_uninstall_preview() {
    echo ""
    echo -e "${CYAN}========================================${NC}"
    echo -e "${CYAN}       将要卸载以下内容${NC}"
    echo -e "${CYAN}========================================${NC}"
    echo ""
    
    # 检查 Agent 服务
    echo -e "${YELLOW}服务:${NC}"
    case "$INIT_SYSTEM" in
        systemd)
            for svc in "$AGENT_SERVICE" "${CORE_SERVICES[@]}"; do
                if systemctl list-unit-files | grep -q "^${svc}.service"; then
                    echo -e "  - $svc ${GREEN}(已安装)${NC}"
                fi
            done
            ;;
        openrc|procd|sysvinit)
            for svc in "$AGENT_SERVICE" "${CORE_SERVICES[@]}"; do
                if [[ -f "/etc/init.d/$svc" ]]; then
                    echo -e "  - $svc ${GREEN}(已安装)${NC}"
                fi
            done
            ;;
    esac
    
    # 检查目录
    echo ""
    echo -e "${YELLOW}目录:${NC}"
    if [[ -d "$AGENT_INSTALL_DIR" ]]; then
        echo -e "  - $AGENT_INSTALL_DIR ${GREEN}(存在)${NC}"
    fi
    for dir in "${CORE_INSTALL_DIRS[@]}"; do
        if [[ -d "$dir" ]]; then
            echo -e "  - $dir ${GREEN}(存在)${NC}"
        fi
    done
    
    echo ""
}

# 卸载 Agent
uninstall_agent() {
    info "卸载 SBoard Agent..."
    
    # 停止并卸载 Agent 服务
    uninstall_service "$AGENT_SERVICE"
    
    # 删除 Agent 目录
    remove_directory "$AGENT_INSTALL_DIR"
    
    success "SBoard Agent 卸载完成"
}

# 卸载核心服务
uninstall_cores() {
    info "卸载核心服务..."
    
    for svc in "${CORE_SERVICES[@]}"; do
        uninstall_service "$svc"
    done
    
    for dir in "${CORE_INSTALL_DIRS[@]}"; do
        remove_directory "$dir"
    done
    
    success "核心服务卸载完成"
}

# reload systemd
reload_systemd() {
    if [[ "$INIT_SYSTEM" == "systemd" ]]; then
        systemctl daemon-reload
    fi
}

# 确认卸载
confirm_uninstall() {
    echo ""
    echo -e "${RED}警告: 此操作将删除 Agent 及其部署的所有核心服务!${NC}"
    echo ""
    read -p "确定要继续吗? [y/N]: " -n 1 -r
    echo
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        echo "已取消卸载"
        exit 0
    fi
}

# 显示菜单
show_menu() {
    clear
    echo -e "${CYAN}"
    echo "  _   _       _           _        _ _ "
    echo " | | | |_ __ (_)_ __  ___| |_ __ _| | |"
    echo " | | | | '_ \\| | '_ \\/ __| __/ _\` | | |"
    echo " | |_| | | | | | | | \\__ \\ || (_| | | |"
    echo "  \\___/|_| |_|_|_| |_|___/\\__\\__,_|_|_|"
    echo -e "${NC}"
    echo -e "${CYAN}========== Agent 卸载工具 ==========${NC}"
    echo ""
    echo -e "  ${GREEN}1.${NC} 卸载 Agent 和所有核心服务"
    echo -e "  ${GREEN}2.${NC} 仅卸载核心服务 (保留 Agent)"
    echo -e "  ${GREEN}3.${NC} 仅卸载 Agent (保留核心服务)"
    echo -e "  ${GREEN}4.${NC} 查看将要卸载的内容"
    echo -e "  ${BLUE}0.${NC} 退出"
    echo ""
    echo -e "${CYAN}=====================================${NC}"
    echo ""
}

# 交互式菜单
interactive_menu() {
    while true; do
        show_menu
        read -p "请选择操作 [0-4]: " choice
        echo ""
        
        case "$choice" in
            1)
                confirm_uninstall
                uninstall_agent
                uninstall_cores
                reload_systemd
                echo ""
                success "全部卸载完成!"
                read -p "按回车键继续..."
                ;;
            2)
                echo -e "${YELLOW}将卸载核心服务 (sing-box, mihomo)${NC}"
                read -p "确定吗? [y/N]: " -n 1 -r
                echo
                if [[ $REPLY =~ ^[Yy]$ ]]; then
                    uninstall_cores
                    reload_systemd
                    success "核心服务卸载完成!"
                fi
                read -p "按回车键继续..."
                ;;
            3)
                echo -e "${YELLOW}将卸载 Agent (保留核心服务)${NC}"
                read -p "确定吗? [y/N]: " -n 1 -r
                echo
                if [[ $REPLY =~ ^[Yy]$ ]]; then
                    uninstall_agent
                    reload_systemd
                    success "Agent 卸载完成!"
                fi
                read -p "按回车键继续..."
                ;;
            4)
                show_uninstall_preview
                read -p "按回车键继续..."
                ;;
            0)
                echo -e "${GREEN}再见!${NC}"
                exit 0
                ;;
            *)
                echo -e "${RED}无效选择${NC}"
                sleep 1
                ;;
        esac
    done
}

# 直接卸载全部 (非交互模式)
uninstall_all() {
    show_uninstall_preview
    confirm_uninstall
    uninstall_agent
    uninstall_cores
    reload_systemd
    echo ""
    success "全部卸载完成!"
}

# 显示帮助
show_help() {
    echo "SBoard Agent 卸载脚本"
    echo ""
    echo "用法:"
    echo "  uninstall-agent.sh [命令]"
    echo ""
    echo "命令:"
    echo "  (无参数)    交互式菜单"
    echo "  all         卸载 Agent 和所有核心服务"
    echo "  agent       仅卸载 Agent"
    echo "  cores       仅卸载核心服务"
    echo "  preview     预览将要卸载的内容"
    echo "  help        显示帮助"
    echo ""
    echo "支持的 Init 系统:"
    echo "  - systemd (Ubuntu, Debian, CentOS, Fedora, Arch 等)"
    echo "  - OpenRC (Alpine Linux, Gentoo)"
    echo "  - procd (OpenWrt)"
    echo "  - init.d/SysVinit (老版本系统)"
    echo ""
}

# 主函数
main() {
    echo ""
    echo -e "${CYAN}========================================${NC}"
    echo -e "${CYAN}     SBoard Agent 卸载工具${NC}"
    echo -e "${CYAN}========================================${NC}"
    echo ""
    
    check_root
    detect_init_system
    
    case "${1:-}" in
        "")
            # 无参数，显示交互式菜单
            interactive_menu
            ;;
        all)
            uninstall_all
            ;;
        agent)
            uninstall_agent
            reload_systemd
            ;;
        cores)
            uninstall_cores
            reload_systemd
            ;;
        preview)
            show_uninstall_preview
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
