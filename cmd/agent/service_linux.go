//go:build linux
// +build linux

package main

import (
	"bufio"
	"fmt"
	"log"
	"math/rand"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"time"
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
	return filepath.Join("/opt/sboard", coreType)
}

// GetBinaryName 获取二进制文件名
func (s *ServiceManager) GetBinaryName(coreType string) string {
	return coreType // linux: sing-box
}

// GetConfigFileName 获取配置文件名
func (s *ServiceManager) GetConfigFileName(coreType string) string {
	return "config.json"
}

// EnsureServiceUser 确保服务运行用户存在，如不存在则创建（使用随机可用 UID）。
// installPath 用作用户的 home 目录。
func (s *ServiceManager) EnsureServiceUser(serviceName, installPath string) (string, int, error) {
	username := serviceName // 用服务名作为用户名

	// 检查用户是否已存在
	u, err := user.Lookup(username)
	if err == nil {
		uid, _ := strconv.Atoi(u.Uid)
		log.Printf("服务用户 %s 已存在 (UID: %d)", username, uid)
		// 若 home 目录与 installPath 不一致，更新之
		if u.HomeDir != installPath {
			s.updateUserHome(username, installPath)
		}
		return username, uid, nil
	}

	// 用户不存在，生成随机可用 UID
	uid, err := findAvailableUID()
	if err != nil {
		return "", 0, fmt.Errorf("查找可用 UID 失败: %v", err)
	}

	// 根据系统类型创建用户
	switch s.initSystem {
	case InitProcd:
		// OpenWrt: busybox adduser 或直接写 /etc/passwd
		if err := s.createOpenWrtUser(username, uid, installPath); err != nil {
			return "", 0, err
		}
	default:
		// 其他 Linux: useradd
		cmd := exec.Command("useradd", "-r", "-s", "/usr/sbin/nologin",
			"-M", "-d", installPath, "-u", strconv.Itoa(uid), username)
		if output, err := cmd.CombinedOutput(); err != nil {
			return "", 0, fmt.Errorf("创建用户 %s 失败: %s %v", username, string(output), err)
		}
	}

	log.Printf("已创建服务用户 %s (UID: %d)", username, uid)
	return username, uid, nil
}

// updateUserHome 更新用户的 home 目录
func (s *ServiceManager) updateUserHome(username, newHome string) {
	if s.initSystem == InitProcd {
		// OpenWrt: 直接替换 /etc/passwd 中的 home 字段
		data, err := os.ReadFile("/etc/passwd")
		if err != nil {
			return
		}
		lines := strings.Split(string(data), "\n")
		for i, line := range lines {
			fields := strings.Split(line, ":")
			if len(fields) >= 7 && fields[0] == username {
				fields[5] = newHome
				lines[i] = strings.Join(fields, ":")
				break
			}
		}
		os.WriteFile("/etc/passwd", []byte(strings.Join(lines, "\n")), 0644)
	} else {
		// 标准 Linux: usermod
		exec.Command("usermod", "-d", newHome, username).Run()
	}
	log.Printf("已更新用户 %s 的 home 目录为 %s", username, newHome)
}

// createOpenWrtUser 在 OpenWrt 上创建用户（busybox 环境）
func (s *ServiceManager) createOpenWrtUser(username string, uid int, homeDir string) error {
	// 检查是否有 useradd
	if _, err := exec.LookPath("useradd"); err == nil {
		cmd := exec.Command("useradd", "-r", "-s", "/bin/false",
			"-M", "-d", homeDir, "-u", strconv.Itoa(uid), username)
		if output, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("useradd 创建用户失败: %s %v", string(output), err)
		}
		return nil
	}

	// 回退: 直接写 /etc/passwd 和 /etc/group
	passwdLine := fmt.Sprintf("%s:x:%d:%d:%s:%s:/bin/false\n", username, uid, uid, username, homeDir)
	groupLine := fmt.Sprintf("%s:x:%d:\n", username, uid)

	f, err := os.OpenFile("/etc/passwd", os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("打开 /etc/passwd 失败: %v", err)
	}
	defer f.Close()
	if _, err := f.WriteString(passwdLine); err != nil {
		return fmt.Errorf("写入 /etc/passwd 失败: %v", err)
	}

	gf, err := os.OpenFile("/etc/group", os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("打开 /etc/group 失败: %v", err)
	}
	defer gf.Close()
	if _, err := gf.WriteString(groupLine); err != nil {
		return fmt.Errorf("写入 /etc/group 失败: %v", err)
	}

	return nil
}

// findAvailableUID 在 2000-59999 范围内随机查找一个未被使用的 UID
func findAvailableUID() (int, error) {
	// 初始化随机数种子
	rand.Seed(time.Now().UnixNano())

	usedUIDs := make(map[int]bool)

	f, err := os.Open("/etc/passwd")
	if err != nil {
		return 0, err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		fields := strings.Split(scanner.Text(), ":")
		if len(fields) >= 3 {
			if uid, err := strconv.Atoi(fields[2]); err == nil {
				usedUIDs[uid] = true
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return 0, fmt.Errorf("读取 /etc/passwd 失败: %v", err)
	}

	// 在 2000-59999 范围内随机选择
	const minUID, maxUID = 2000, 59999
	for attempts := 0; attempts < 1000; attempts++ {
		uid := minUID + rand.Intn(maxUID-minUID+1)
		if !usedUIDs[uid] {
			return uid, nil
		}
	}

	// 回退：顺序查找
	for uid := minUID; uid <= maxUID; uid++ {
		if !usedUIDs[uid] {
			return uid, nil
		}
	}

	return 0, fmt.Errorf("在 %d-%d 范围内未找到可用 UID", minUID, maxUID)
}

// setCapabilities 为二进制文件设置网络能力（允许非 root 用户管理网络）
func setCapabilities(binaryPath string) error {
	if _, err := exec.LookPath("setcap"); err != nil {
		log.Printf("setcap 不可用，跳过 capabilities 设置")
		return nil // setcap 不可用时静默跳过
	}
	cmd := exec.Command("setcap",
		"cap_net_admin,cap_net_bind_service,cap_net_raw+ep", binaryPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("setcap 失败: %s %v", string(output), err)
	}
	return nil
}

// InstallService 安装服务
func (s *ServiceManager) InstallService(serviceName, installPath, coreType string) error {
	serviceFilePath := s.GetServiceFilePath(serviceName)

	// 1. 确保服务用户存在
	username, _, err := s.EnsureServiceUser(serviceName, installPath)
	if err != nil {
		log.Printf("创建服务用户失败，将以 root 运行: %v", err)
		username = "root"
	}

	// 2. 为二进制设置 capabilities（允许非 root 用户管理网络）
	if username != "root" {
		binaryName := s.GetBinaryName(coreType)
		binaryPath := filepath.Join(installPath, binaryName)
		if err := setCapabilities(binaryPath); err != nil {
			log.Printf("设置 capabilities 失败: %v", err)
		}
	}

	// 3. 设置安装目录归属
	if username != "root" {
		cmd := exec.Command("chown", "-R", username+":"+username, installPath)
		if output, err := cmd.CombinedOutput(); err != nil {
			log.Printf("chown 失败: %s %v", string(output), err)
		}
	}

	// 4. 安装服务
	switch s.initSystem {
	case InitSystemd:
		return s.installSystemdService(serviceName, installPath, coreType, serviceFilePath, username)
	case InitOpenRC:
		return s.installOpenRCService(serviceName, installPath, coreType, serviceFilePath, username)
	case InitProcd:
		return s.installProcdService(serviceName, installPath, coreType, serviceFilePath, username)
	case InitSysVinit:
		return s.installSysVinitService(serviceName, installPath, coreType, serviceFilePath, username)
	default:
		return fmt.Errorf("未检测到支持的 Init 系统")
	}
}

// installSystemdService 安装 systemd 服务
func (s *ServiceManager) installSystemdService(serviceName, installPath, coreType, serviceFilePath, username string) error {
	serviceContent := s.generateSystemdServiceFile(serviceName, installPath, coreType, username)

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
func (s *ServiceManager) installOpenRCService(serviceName, installPath, coreType, serviceFilePath, username string) error {
	serviceContent := s.generateOpenRCServiceFile(serviceName, installPath, coreType, username)

	if err := os.WriteFile(serviceFilePath, []byte(serviceContent), 0755); err != nil {
		return fmt.Errorf("写入服务文件失败: %v", err)
	}

	if err := exec.Command("rc-update", "add", serviceName, "default").Run(); err != nil {
		// 忽略已添加的错误
	}

	return nil
}

// installSysVinitService 安装 SysVinit 服务
func (s *ServiceManager) installSysVinitService(serviceName, installPath, coreType, serviceFilePath, username string) error {
	serviceContent := s.generateSysVinitServiceFile(serviceName, installPath, coreType, username)

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
func (s *ServiceManager) installProcdService(serviceName, installPath, coreType, serviceFilePath, username string) error {
	serviceContent := s.generateProcdServiceFile(serviceName, installPath, coreType, username)

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
func (s *ServiceManager) generateSystemdServiceFile(serviceName, installPath, coreType, username string) string {
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
User=%s
Group=%s
WorkingDirectory=%s
ExecStart=%s
Restart=always
RestartSec=5
LimitNOFILE=1000000
AmbientCapabilities=CAP_NET_ADMIN CAP_NET_BIND_SERVICE CAP_NET_RAW
CapabilityBoundingSet=CAP_NET_ADMIN CAP_NET_BIND_SERVICE CAP_NET_RAW
NoNewPrivileges=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=%s /var/log /tmp

[Install]
WantedBy=multi-user.target
`, serviceName, username, username, workDir, execStart, installPath)
}

// generateOpenRCServiceFile 生成 OpenRC 服务文件内容
func (s *ServiceManager) generateOpenRCServiceFile(serviceName, installPath, coreType, username string) string {
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
command_user="%s"
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
    ulimit -n 1000000
    checkpath --directory --owner %s:%s --mode 0755 /run
}
`, serviceName, coreType, installPath, coreType, commandArgs, username, installPath, username, username)
}

// generateSysVinitServiceFile 生成 SysVinit 服务文件内容
func (s *ServiceManager) generateSysVinitServiceFile(serviceName, installPath, coreType, username string) string {
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
RUN_USER="%s"
PIDFILE="/var/run/${NAME}.pid"
LOGFILE="/var/log/${NAME}.log"
WORKDIR="%s"

start() {
    if [ -f "$PIDFILE" ] && kill -0 $(cat "$PIDFILE") 2>/dev/null; then
        echo "$NAME is already running"
        return 1
    fi
    echo "Starting $NAME as user $RUN_USER..."
    ulimit -n 1000000 2>/dev/null
    cd "$WORKDIR"
    if [ "$RUN_USER" = "root" ]; then
        nohup "$DAEMON" $DAEMON_ARGS >> "$LOGFILE" 2>&1 &
    else
        nohup su -s /bin/sh "$RUN_USER" -c "$DAEMON $DAEMON_ARGS" >> "$LOGFILE" 2>&1 &
    fi
    echo $! > "$PIDFILE"
    sleep 1
    if kill -0 $(cat "$PIDFILE") 2>/dev/null; then
        echo "$NAME started (PID: $(cat $PIDFILE))"
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
    local timeout=10
    while [ $timeout -gt 0 ] && kill -0 $(cat "$PIDFILE") 2>/dev/null; do
        sleep 1
        timeout=$((timeout - 1))
    done
    if kill -0 $(cat "$PIDFILE") 2>/dev/null; then
        kill -9 $(cat "$PIDFILE") 2>/dev/null
    fi
    rm -f "$PIDFILE"
    echo "$NAME stopped"
}

restart() {
    stop
    sleep 1
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
`, serviceName, coreType, coreType, serviceName, installPath, coreType, daemonArgs, username, installPath)
}

// generateProcdServiceFile 生成 OpenWrt procd 服务文件内容
func (s *ServiceManager) generateProcdServiceFile(serviceName, installPath, coreType, username string) string {
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
RUN_USER="%s"

start_service() {
    mkdir -p $WORKDIR
    chown -R $RUN_USER $WORKDIR 2>/dev/null
    procd_open_instance "$NAME"
    procd_set_param command "$PROG" %s
    procd_set_param user "$RUN_USER"
    procd_set_param respawn
    procd_set_param stdout 1
    procd_set_param stderr 1
    procd_set_param limits nofile="1000000 1000000"
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
`, serviceName, installPath, coreType, installPath, username, commandArgs)
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
