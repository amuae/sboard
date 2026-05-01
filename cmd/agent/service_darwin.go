//go:build darwin
// +build darwin

package main

import (
	"fmt"
	"log"
	"math/rand"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
)

// ServiceManager macOS 服务管理器
// 使用 launchctl 管理 launchd 服务
type ServiceManager struct{}

// GetServiceFilePath 获取 LaunchDaemon plist 文件路径
func (s *ServiceManager) GetServiceFilePath(serviceName string) string {
	return filepath.Join("/Library/LaunchDaemons", "com.sboard."+serviceName+".plist")
}

// GetDefaultInstallPath 获取默认安装路径
func (s *ServiceManager) GetDefaultInstallPath(coreType string) string {
	return filepath.Join("/usr/local/sboard", coreType)
}

// GetBinaryName 获取二进制文件名
func (s *ServiceManager) GetBinaryName(coreType string) string {
	return coreType // darwin: sing-box
}

// GetConfigFileName 获取配置文件名
func (s *ServiceManager) GetConfigFileName(coreType string) string {
	return "config.json"
}

// EnsureServiceUser 确保 macOS 服务运行用户存在
func (s *ServiceManager) EnsureServiceUser(serviceName, installPath string) (string, int, error) {
	username := "_" + serviceName // macOS 系统用户惯例以 _ 开头

	// 检查用户是否已存在
	u, err := user.Lookup(username)
	if err == nil {
		uid, _ := strconv.Atoi(u.Uid)
		log.Printf("服务用户 %s 已存在 (UID: %d)", username, uid)
		// 若 home 目录与 installPath 不一致，更新之
		if u.HomeDir != installPath {
			exec.Command("dscl", ".", "-create", "/Users/"+username, "NFSHomeDirectory", installPath).Run()
			log.Printf("已更新用户 %s 的 home 目录为 %s", username, installPath)
		}
		return username, uid, nil
	}

	// 查找可用 UID（macOS 系统守护进程用 200-400 范围）
	uid, err := findAvailableMacUID()
	if err != nil {
		return "", 0, err
	}

	// 使用 dscl 创建用户
	uidStr := strconv.Itoa(uid)
	cmds := [][]string{
		{"dscl", ".", "-create", "/Users/" + username},
		{"dscl", ".", "-create", "/Users/" + username, "UniqueID", uidStr},
		{"dscl", ".", "-create", "/Users/" + username, "PrimaryGroupID", uidStr},
		{"dscl", ".", "-create", "/Users/" + username, "UserShell", "/usr/bin/false"},
		{"dscl", ".", "-create", "/Users/" + username, "RealName", serviceName + " service"},
		{"dscl", ".", "-create", "/Users/" + username, "NFSHomeDirectory", installPath},
		{"dscl", ".", "-create", "/Groups/" + username},
		{"dscl", ".", "-create", "/Groups/" + username, "PrimaryGroupID", uidStr},
	}
	for _, args := range cmds {
		if output, err := exec.Command(args[0], args[1:]...).CombinedOutput(); err != nil {
			return "", 0, fmt.Errorf("dscl 创建用户失败 (%v): %s %v", args, string(output), err)
		}
	}

	// 隐藏用户（不在登录界面显示）
	exec.Command("dscl", ".", "-create", "/Users/"+username, "IsHidden", "1").Run()

	log.Printf("已创建服务用户 %s (UID: %d)", username, uid)
	return username, uid, nil
}

// findAvailableMacUID 在 macOS 上查找可用 UID
func findAvailableMacUID() (int, error) {
	usedUIDs := make(map[int]bool)
	output, err := exec.Command("dscl", ".", "-list", "/Users", "UniqueID").Output()
	if err == nil {
		for _, line := range strings.Split(string(output), "\n") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				if uid, err := strconv.Atoi(fields[len(fields)-1]); err == nil {
					usedUIDs[uid] = true
				}
			}
		}
	}

	const minUID, maxUID = 400, 499 // macOS 守护进程 UID 范围
	for attempts := 0; attempts < 200; attempts++ {
		uid := minUID + rand.Intn(maxUID-minUID+1)
		if !usedUIDs[uid] {
			return uid, nil
		}
	}
	for uid := minUID; uid <= maxUID; uid++ {
		if !usedUIDs[uid] {
			return uid, nil
		}
	}
	return 0, fmt.Errorf("在 %d-%d 范围内未找到可用 UID", minUID, maxUID)
}

// InstallService 安装 macOS LaunchDaemon 服务
func (s *ServiceManager) InstallService(serviceName, installPath, coreType string) error {
	plistPath := s.GetServiceFilePath(serviceName)

	// 1. 确保服务用户存在
	username, _, err := s.EnsureServiceUser(serviceName, installPath)
	if err != nil {
		log.Printf("创建服务用户失败，将以 root 运行: %v", err)
		username = "root"
	}

	// 2. 设置安装目录归属
	if username != "root" {
		cmd := exec.Command("chown", "-R", username, installPath)
		if output, err := cmd.CombinedOutput(); err != nil {
			log.Printf("chown 失败: %s %v", string(output), err)
		}
	}

	// 3. 生成并写入 plist
	plistContent := s.generateLaunchDaemonPlist(serviceName, installPath, coreType, username)

	// 确保目录存在
	if err := os.MkdirAll(filepath.Dir(plistPath), 0755); err != nil {
		return fmt.Errorf("创建 LaunchDaemons 目录失败: %v", err)
	}

	// 写入 plist 文件
	if err := os.WriteFile(plistPath, []byte(plistContent), 0644); err != nil {
		return fmt.Errorf("写入 plist 文件失败: %v", err)
	}

	// 加载服务
	label := "com.sboard." + serviceName
	if err := exec.Command("launchctl", "load", "-w", plistPath).Run(); err != nil {
		return fmt.Errorf("加载服务失败: %v", err)
	}

	// 启用服务（macOS 10.10+）
	exec.Command("launchctl", "enable", "system/"+label).Run()

	return nil
}

// StartService 启动服务
func (s *ServiceManager) StartService(serviceName string) error {
	label := "com.sboard." + serviceName

	// 尝试使用 kickstart (macOS 10.10+)
	if err := exec.Command("launchctl", "kickstart", "-k", "system/"+label).Run(); err != nil {
		// 回退到 start 命令
		return exec.Command("launchctl", "start", label).Run()
	}
	return nil
}

// StopService 停止服务
func (s *ServiceManager) StopService(serviceName string) error {
	label := "com.sboard." + serviceName

	// 尝试使用 kill (macOS 10.10+)
	if err := exec.Command("launchctl", "kill", "SIGTERM", "system/"+label).Run(); err != nil {
		// 回退到 stop 命令
		return exec.Command("launchctl", "stop", label).Run()
	}
	return nil
}

// RestartService 重启服务
func (s *ServiceManager) RestartService(serviceName string) error {
	label := "com.sboard." + serviceName

	// 使用 kickstart -k 重启
	if err := exec.Command("launchctl", "kickstart", "-k", "system/"+label).Run(); err != nil {
		// 回退到 stop + start
		s.StopService(serviceName)
		return s.StartService(serviceName)
	}
	return nil
}

// IsServiceRunning 检查服务是否运行
func (s *ServiceManager) IsServiceRunning(serviceName string) bool {
	label := "com.sboard." + serviceName

	// 使用 list 检查服务状态
	output, err := exec.Command("launchctl", "list").Output()
	if err != nil {
		return false
	}

	// 检查输出中是否包含服务标签
	return strings.Contains(string(output), label)
}

// UninstallService 卸载服务
func (s *ServiceManager) UninstallService(serviceName string) error {
	label := "com.sboard." + serviceName
	plistPath := s.GetServiceFilePath(serviceName)

	// 停止服务
	s.StopService(serviceName)

	// 卸载服务
	exec.Command("launchctl", "bootout", "system/"+label).Run()
	exec.Command("launchctl", "unload", "-w", plistPath).Run()

	// 删除 plist 文件
	os.Remove(plistPath)

	return nil
}

// generateLaunchDaemonPlist 生成 LaunchDaemon plist 文件内容
func (s *ServiceManager) generateLaunchDaemonPlist(serviceName, installPath, coreType, username string) string {
	binaryPath := filepath.Join(installPath, coreType)
	label := "com.sboard." + serviceName
	logPath := filepath.Join("/var/log", serviceName+".log")
	errLogPath := filepath.Join("/var/log", serviceName+".err.log")

	var programArgs string
	switch coreType {
	case "sing-box":
		configPath := filepath.Join(installPath, "config.json")
		programArgs = fmt.Sprintf(`		<string>%s</string>
		<string>run</string>
		<string>-c</string>
		<string>%s</string>
		<string>-D</string>
		<string>%s</string>`, binaryPath, configPath, installPath)
	default:
		configPath := filepath.Join(installPath, "config.json")
		programArgs = fmt.Sprintf(`		<string>%s</string>
		<string>run</string>
		<string>-c</string>
		<string>%s</string>
		<string>-D</string>
		<string>%s</string>`, binaryPath, configPath, installPath)
	}

	return fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>Label</key>
	<string>%s</string>
	<key>UserName</key>
	<string>%s</string>
	<key>ProgramArguments</key>
	<array>
%s
	</array>
	<key>WorkingDirectory</key>
	<string>%s</string>
	<key>RunAtLoad</key>
	<true/>
	<key>KeepAlive</key>
	<true/>
	<key>StandardOutPath</key>
	<string>%s</string>
	<key>StandardErrorPath</key>
	<string>%s</string>
	<key>SoftResourceLimits</key>
	<dict>
		<key>NumberOfFiles</key>
		<integer>1000000</integer>
	</dict>
	<key>HardResourceLimits</key>
	<dict>
		<key>NumberOfFiles</key>
		<integer>1000000</integer>
	</dict>
</dict>
</plist>
`, label, username, programArgs, installPath, logPath, errLogPath)
}

// NewServiceManager 创建服务管理器
func NewServiceManager() *ServiceManager {
	return &ServiceManager{}
}
