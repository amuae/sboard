//go:build darwin
// +build darwin

package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
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
	return coreType // darwin: sing-box, mihomo
}

// GetConfigFileName 获取配置文件名
func (s *ServiceManager) GetConfigFileName(coreType string) string {
	if coreType == "sing-box" {
		return "config.json"
	}
	return "config.yaml"
}

// InstallService 安装 macOS LaunchDaemon 服务
func (s *ServiceManager) InstallService(serviceName, installPath, coreType string) error {
	plistPath := s.GetServiceFilePath(serviceName)
	plistContent := s.generateLaunchDaemonPlist(serviceName, installPath, coreType)

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
func (s *ServiceManager) generateLaunchDaemonPlist(serviceName, installPath, coreType string) string {
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
	case "mihomo":
		programArgs = fmt.Sprintf(`		<string>%s</string>
		<string>-d</string>
		<string>%s</string>`, binaryPath, installPath)
	}

	return fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>Label</key>
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
</dict>
</plist>
`, label, programArgs, installPath, logPath, errLogPath)
}

// NewServiceManager 创建服务管理器
func NewServiceManager() *ServiceManager {
	return &ServiceManager{}
}
