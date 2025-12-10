//go:build linux
// +build linux

package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// InitSystem 表示 Init 系统类型
type InitSystem string

const (
	InitSystemd  InitSystem = "systemd"
	InitOpenRC   InitSystem = "openrc"
	InitProcd    InitSystem = "procd" // OpenWrt
	InitSysVinit InitSystem = "sysvinit"
	InitUnknown  InitSystem = "unknown"
)

// ServiceManager Linux 服务管理器
type ServiceManager struct {
	initSystem InitSystem
}

// detectInitSystem 检测 Init 系统类型
func detectInitSystem() InitSystem {
	// 检查 systemd
	if _, err := exec.LookPath("systemctl"); err == nil {
		if _, err := os.Stat("/run/systemd/system"); err == nil {
			return InitSystemd
		}
	}

	// 检查 OpenWrt procd (优先于 OpenRC)
	if isOpenWrt() {
		return InitProcd
	}

	// 检查 OpenRC
	if _, err := exec.LookPath("rc-service"); err == nil {
		if _, err := exec.LookPath("rc-update"); err == nil {
			return InitOpenRC
		}
	}

	// 检查 SysVinit
	if _, err := os.Stat("/etc/init.d"); err == nil {
		if _, err := exec.LookPath("update-rc.d"); err == nil {
			return InitSysVinit
		}
		if _, err := exec.LookPath("chkconfig"); err == nil {
			return InitSysVinit
		}
	}

	return InitUnknown
}

// isOpenWrt 检测是否为 OpenWrt 系统
func isOpenWrt() bool {
	// 检查 /etc/openwrt_release 文件
	if _, err := os.Stat("/etc/openwrt_release"); err == nil {
		return true
	}
	// 检查 /etc/openwrt_version 文件
	if _, err := os.Stat("/etc/openwrt_version"); err == nil {
		return true
	}
	// 检查 procd 进程
	if _, err := exec.LookPath("/sbin/procd"); err == nil {
		if _, err := os.Stat("/lib/functions/procd.sh"); err == nil {
			return true
		}
	}
	return false
}

// GetServiceFilePath 获取服务文件路径
func (s *ServiceManager) GetServiceFilePath(serviceName string) string {
	switch s.initSystem {
	case InitSystemd:
		return filepath.Join("/etc/systemd/system", serviceName+".service")
	case InitOpenRC, InitSysVinit, InitProcd:
		return filepath.Join("/etc/init.d", serviceName)
	default:
		return ""
	}
}

// GetDefaultInstallPath 获取默认安装路径
func (s *ServiceManager) GetDefaultInstallPath(coreType string) string {
	return filepath.Join("/root", coreType)
}

// GetBinaryName 获取二进制文件名
func (s *ServiceManager) GetBinaryName(coreType string) string {
	return coreType // linux: sing-box
}

// GetConfigFileName 获取配置文件名
func (s *ServiceManager) GetConfigFileName(coreType string) string {
	return "config.json"
}

// InstallService 安装服务
func (s *ServiceManager) InstallService(serviceName, installPath, coreType string) error {
	serviceFilePath := s.GetServiceFilePath(serviceName)

	switch s.initSystem {
	case InitSystemd:
		return s.installSystemdService(serviceName, installPath, coreType, serviceFilePath)
	case InitOpenRC:
		return s.installOpenRCService(serviceName, installPath, coreType, serviceFilePath)
	case InitProcd:
		return s.installProcdService(serviceName, installPath, coreType, serviceFilePath)
	case InitSysVinit:
		return s.installSysVinitService(serviceName, installPath, coreType, serviceFilePath)
	default:
		return fmt.Errorf("未检测到支持的 Init 系统")
	}
}

// installSystemdService 安装 systemd 服务
func (s *ServiceManager) installSystemdService(serviceName, installPath, coreType, serviceFilePath string) error {
	serviceContent := s.generateSystemdServiceFile(serviceName, installPath, coreType)

	if err := os.WriteFile(serviceFilePath, []byte(serviceContent), 0644); err != nil {
		return fmt.Errorf("写入服务文件失败: %v", err)
	}

	if err := exec.Command("systemctl", "daemon-reload").Run(); err != nil {
		return fmt.Errorf("重载 systemd 失败: %v", err)
	}

	if err := exec.Command("systemctl", "enable", serviceName).Run(); err != nil {
		return fmt.Errorf("启用服务失败: %v", err)
	}

	return nil
}

// installOpenRCService 安装 OpenRC 服务
func (s *ServiceManager) installOpenRCService(serviceName, installPath, coreType, serviceFilePath string) error {
	serviceContent := s.generateOpenRCServiceFile(serviceName, installPath, coreType)

	if err := os.WriteFile(serviceFilePath, []byte(serviceContent), 0755); err != nil {
		return fmt.Errorf("写入服务文件失败: %v", err)
	}

	if err := exec.Command("rc-update", "add", serviceName, "default").Run(); err != nil {
		// 忽略已添加的错误
	}

	return nil
}

// installSysVinitService 安装 SysVinit 服务
func (s *ServiceManager) installSysVinitService(serviceName, installPath, coreType, serviceFilePath string) error {
	serviceContent := s.generateSysVinitServiceFile(serviceName, installPath, coreType)

	if err := os.WriteFile(serviceFilePath, []byte(serviceContent), 0755); err != nil {
		return fmt.Errorf("写入服务文件失败: %v", err)
	}

	// 添加开机启动
	if _, err := exec.LookPath("update-rc.d"); err == nil {
		exec.Command("update-rc.d", serviceName, "defaults").Run()
	} else if _, err := exec.LookPath("chkconfig"); err == nil {
		exec.Command("chkconfig", "--add", serviceName).Run()
		exec.Command("chkconfig", serviceName, "on").Run()
	}

	return nil
}

// installProcdService 安装 OpenWrt procd 服务
func (s *ServiceManager) installProcdService(serviceName, installPath, coreType, serviceFilePath string) error {
	serviceContent := s.generateProcdServiceFile(serviceName, installPath, coreType)

	if err := os.WriteFile(serviceFilePath, []byte(serviceContent), 0755); err != nil {
		return fmt.Errorf("写入服务文件失败: %v", err)
	}

	// OpenWrt 使用 /etc/rc.d/S99xxx 软链接启用服务
	exec.Command("/etc/init.d/"+serviceName, "enable").Run()

	return nil
}

// StartService 启动服务
func (s *ServiceManager) StartService(serviceName string) error {
	switch s.initSystem {
	case InitSystemd:
		return exec.Command("systemctl", "start", serviceName).Run()
	case InitOpenRC:
		return exec.Command("rc-service", serviceName, "start").Run()
	case InitProcd:
		return exec.Command("/etc/init.d/"+serviceName, "start").Run()
	case InitSysVinit:
		return exec.Command("/etc/init.d/"+serviceName, "start").Run()
	default:
		return fmt.Errorf("未知的 Init 系统")
	}
}

// StopService 停止服务
func (s *ServiceManager) StopService(serviceName string) error {
	switch s.initSystem {
	case InitSystemd:
		return exec.Command("systemctl", "stop", serviceName).Run()
	case InitOpenRC:
		return exec.Command("rc-service", serviceName, "stop").Run()
	case InitProcd:
		return exec.Command("/etc/init.d/"+serviceName, "stop").Run()
	case InitSysVinit:
		return exec.Command("/etc/init.d/"+serviceName, "stop").Run()
	default:
		return fmt.Errorf("未知的 Init 系统")
	}
}

// RestartService 重启服务
func (s *ServiceManager) RestartService(serviceName string) error {
	switch s.initSystem {
	case InitSystemd:
		return exec.Command("systemctl", "restart", serviceName).Run()
	case InitOpenRC:
		return exec.Command("rc-service", serviceName, "restart").Run()
	case InitProcd:
		return exec.Command("/etc/init.d/"+serviceName, "restart").Run()
	case InitSysVinit:
		return exec.Command("/etc/init.d/"+serviceName, "restart").Run()
	default:
		return fmt.Errorf("未知的 Init 系统")
	}
}

// IsServiceRunning 检查服务是否运行
func (s *ServiceManager) IsServiceRunning(serviceName string) bool {
	switch s.initSystem {
	case InitSystemd:
		output, err := exec.Command("systemctl", "is-active", serviceName).Output()
		if err != nil {
			return false
		}
		return strings.TrimSpace(string(output)) == "active"
	case InitOpenRC:
		err := exec.Command("rc-service", serviceName, "status").Run()
		return err == nil
	case InitProcd:
		err := exec.Command("/etc/init.d/"+serviceName, "status").Run()
		return err == nil
	case InitSysVinit:
		err := exec.Command("/etc/init.d/"+serviceName, "status").Run()
		return err == nil
	default:
		return false
	}
}

// UninstallService 卸载服务
func (s *ServiceManager) UninstallService(serviceName string) error {
	switch s.initSystem {
	case InitSystemd:
		return s.uninstallSystemdService(serviceName)
	case InitOpenRC:
		return s.uninstallOpenRCService(serviceName)
	case InitProcd:
		return s.uninstallProcdService(serviceName)
	case InitSysVinit:
		return s.uninstallSysVinitService(serviceName)
	default:
		return fmt.Errorf("未知的 Init 系统")
	}
}

func (s *ServiceManager) uninstallSystemdService(serviceName string) error {
	exec.Command("systemctl", "stop", serviceName).Run()
	exec.Command("systemctl", "disable", serviceName).Run()
	os.Remove(s.GetServiceFilePath(serviceName))
	exec.Command("systemctl", "daemon-reload").Run()
	return nil
}

func (s *ServiceManager) uninstallOpenRCService(serviceName string) error {
	exec.Command("rc-service", serviceName, "stop").Run()
	exec.Command("rc-update", "del", serviceName, "default").Run()
	os.Remove(s.GetServiceFilePath(serviceName))
	return nil
}

func (s *ServiceManager) uninstallSysVinitService(serviceName string) error {
	exec.Command("/etc/init.d/"+serviceName, "stop").Run()
	if _, err := exec.LookPath("update-rc.d"); err == nil {
		exec.Command("update-rc.d", "-f", serviceName, "remove").Run()
	} else if _, err := exec.LookPath("chkconfig"); err == nil {
		exec.Command("chkconfig", "--del", serviceName).Run()
	}
	os.Remove(s.GetServiceFilePath(serviceName))
	return nil
}

func (s *ServiceManager) uninstallProcdService(serviceName string) error {
	exec.Command("/etc/init.d/"+serviceName, "stop").Run()
	exec.Command("/etc/init.d/"+serviceName, "disable").Run()
	os.Remove(s.GetServiceFilePath(serviceName))
	return nil
}

// generateSystemdServiceFile 生成 systemd 服务文件内容
func (s *ServiceManager) generateSystemdServiceFile(serviceName, installPath, coreType string) string {
	var execStart string
	workDir := installPath

	switch coreType {
	case "sing-box":
		execStart = fmt.Sprintf("%s/sing-box run -c %s/config.json -D %s", installPath, installPath, installPath)
	default:
		execStart = fmt.Sprintf("%s/sing-box run -c %s/config.json -D %s", installPath, installPath, installPath)
	}

	return fmt.Sprintf(`[Unit]
Description=%s Service
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=root
WorkingDirectory=%s
ExecStart=%s
Restart=on-failure
RestartSec=5
LimitNOFILE=1000000

[Install]
WantedBy=multi-user.target
`, serviceName, workDir, execStart)
}

// generateOpenRCServiceFile 生成 OpenRC 服务文件内容
func (s *ServiceManager) generateOpenRCServiceFile(serviceName, installPath, coreType string) string {
	var commandArgs string

	switch coreType {
	case "sing-box":
		commandArgs = fmt.Sprintf("run -c %s/config.json -D %s", installPath, installPath)
	default:
		commandArgs = fmt.Sprintf("run -c %s/config.json -D %s", installPath, installPath)
	}

	return fmt.Sprintf(`#!/sbin/openrc-run

name="%s"
description="%s proxy service"

command="%s/%s"
command_args="%s"
command_background=true
pidfile="/run/${RC_SVCNAME}.pid"
directory="%s"

output_log="/var/log/${RC_SVCNAME}.log"
error_log="/var/log/${RC_SVCNAME}.err"

depend() {
    need net
    after firewall
}

start_pre() {
    checkpath --directory --owner root:root --mode 0755 /run
}
`, serviceName, coreType, installPath, coreType, commandArgs, installPath)
}

// generateSysVinitServiceFile 生成 SysVinit 服务文件内容
func (s *ServiceManager) generateSysVinitServiceFile(serviceName, installPath, coreType string) string {
	var daemonArgs string

	switch coreType {
	case "sing-box":
		daemonArgs = fmt.Sprintf("run -c %s/config.json -D %s", installPath, installPath)
	default:
		daemonArgs = fmt.Sprintf("run -c %s/config.json -D %s", installPath, installPath)
	}

	return fmt.Sprintf(`#!/bin/bash
### BEGIN INIT INFO
# Provides:          %s
# Required-Start:    $network $remote_fs $syslog
# Required-Stop:     $network $remote_fs $syslog
# Default-Start:     2 3 4 5
# Default-Stop:      0 1 6
# Short-Description: %s proxy service
# Description:       %s proxy service managed by sboard
### END INIT INFO

NAME="%s"
DAEMON="%s/%s"
DAEMON_ARGS="%s"
PIDFILE="/var/run/${NAME}.pid"
LOGFILE="/var/log/${NAME}.log"
WORKDIR="%s"

start() {
    if [ -f "$PIDFILE" ] && kill -0 $(cat "$PIDFILE") 2>/dev/null; then
        echo "$NAME is already running"
        return 1
    fi
    echo "Starting $NAME..."
    cd "$WORKDIR"
    nohup "$DAEMON" $DAEMON_ARGS >> "$LOGFILE" 2>&1 &
    echo $! > "$PIDFILE"
    sleep 1
    if kill -0 $(cat "$PIDFILE") 2>/dev/null; then
        echo "$NAME started"
    else
        echo "$NAME failed to start"
        rm -f "$PIDFILE"
        return 1
    fi
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
`, serviceName, coreType, coreType, serviceName, installPath, coreType, daemonArgs, installPath)
}

// generateProcdServiceFile 生成 OpenWrt procd 服务文件内容
func (s *ServiceManager) generateProcdServiceFile(serviceName, installPath, coreType string) string {
	var commandArgs string

	switch coreType {
	case "sing-box":
		commandArgs = fmt.Sprintf("run -c %s/config.json -D %s", installPath, installPath)
	default:
		commandArgs = fmt.Sprintf("run -c %s/config.json -D %s", installPath, installPath)
	}

	return fmt.Sprintf(`#!/bin/sh /etc/rc.common

USE_PROCD=1
START=99

NAME="%s"
PROG="%s/%s"
WORKDIR="%s"

start_service() {
    mkdir -p $WORKDIR
    procd_open_instance "$NAME"
    procd_set_param command "$PROG" %s
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
`, serviceName, installPath, coreType, installPath, commandArgs)
}

// GetInitSystem 获取当前 Init 系统类型
func (s *ServiceManager) GetInitSystem() string {
	return string(s.initSystem)
}

// NewServiceManager 创建服务管理器
func NewServiceManager() *ServiceManager {
	return &ServiceManager{
		initSystem: detectInitSystem(),
	}
}
