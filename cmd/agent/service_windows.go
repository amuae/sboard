//go:build windows
// +build windows

package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
)

// ServiceManager Windows 服务管理器
// 使用 sc.exe 命令管理 Windows 服务
type ServiceManager struct{}

// GetServiceFilePath 获取服务相关文件路径 (Windows 没有服务文件，返回空)
func (s *ServiceManager) GetServiceFilePath(serviceName string) string {
	return ""
}

// GetDefaultInstallPath 获取默认安装路径
func (s *ServiceManager) GetDefaultInstallPath(coreType string) string {
	// Windows 安装到 %LOCALAPPDATA%\sboard\<coreType>
	localAppData := os.Getenv("LOCALAPPDATA")
	if localAppData == "" {
		localAppData = filepath.Join(os.Getenv("USERPROFILE"), "AppData", "Local")
	}
	return filepath.Join(localAppData, "sboard", coreType)
}

// GetBinaryName 获取二进制文件名
func (s *ServiceManager) GetBinaryName(coreType string) string {
	return coreType + ".exe" // windows: sing-box.exe
}

// GetConfigFileName 获取配置文件名
func (s *ServiceManager) GetConfigFileName(coreType string) string {
	return "config.json"
}

// InstallService 安装 Windows 服务
func (s *ServiceManager) InstallService(serviceName, installPath, coreType string) error {
	binaryPath := filepath.Join(installPath, s.GetBinaryName(coreType))
	var binPath string

	switch coreType {
	case "sing-box":
		configPath := filepath.Join(installPath, "config.json")
		binPath = fmt.Sprintf(`"%s" run -c "%s" -D "%s"`, binaryPath, configPath, installPath)
	default:
		configPath := filepath.Join(installPath, "config.json")
		binPath = fmt.Sprintf(`"%s" run -c "%s" -D "%s"`, binaryPath, configPath, installPath)
	}

	// 使用 sc.exe 创建服务
	// sc create <serviceName> binPath= "<path>" start= auto
	cmd := exec.Command("sc", "create", serviceName,
		"binPath=", binPath,
		"start=", "auto",
		"DisplayName=", serviceName+" Service")
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}

	output, err := cmd.CombinedOutput()
	if err != nil {
		// 如果服务已存在，先删除再创建
		if strings.Contains(string(output), "exists") {
			s.UninstallService(serviceName)
			return s.InstallService(serviceName, installPath, coreType)
		}
		return fmt.Errorf("创建服务失败: %s, %v", string(output), err)
	}

	// 设置服务描述
	descCmd := exec.Command("sc", "description", serviceName, coreType+" proxy service managed by sboard")
	descCmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	descCmd.Run()

	// 设置服务失败后自动重启
	failureCmd := exec.Command("sc", "failure", serviceName,
		"reset=", "86400",
		"actions=", "restart/5000/restart/10000/restart/30000")
	failureCmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	failureCmd.Run()

	// 设置延迟自动启动（避免系统启动时与其他服务抢占资源）
	delayCmd := exec.Command("sc", "config", serviceName, "start=", "delayed-auto")
	delayCmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	delayCmd.Run()

	return nil
}

// StartService 启动服务
func (s *ServiceManager) StartService(serviceName string) error {
	cmd := exec.Command("sc", "start", serviceName)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	output, err := cmd.CombinedOutput()
	if err != nil {
		// 如果服务已经在运行，不算错误
		if strings.Contains(string(output), "already been started") {
			return nil
		}
		return fmt.Errorf("启动服务失败: %s", string(output))
	}
	return nil
}

// StopService 停止服务
func (s *ServiceManager) StopService(serviceName string) error {
	cmd := exec.Command("sc", "stop", serviceName)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	output, err := cmd.CombinedOutput()
	if err != nil {
		// 如果服务没有运行，不算错误
		if strings.Contains(string(output), "not been started") ||
			strings.Contains(string(output), "STOPPED") {
			return nil
		}
		return fmt.Errorf("停止服务失败: %s", string(output))
	}
	return nil
}

// RestartService 重启服务
func (s *ServiceManager) RestartService(serviceName string) error {
	s.StopService(serviceName)
	return s.StartService(serviceName)
}

// IsServiceRunning 检查服务是否运行
func (s *ServiceManager) IsServiceRunning(serviceName string) bool {
	cmd := exec.Command("sc", "query", serviceName)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	output, err := cmd.CombinedOutput()
	if err != nil {
		return false
	}
	return strings.Contains(string(output), "RUNNING")
}

// UninstallService 卸载服务
func (s *ServiceManager) UninstallService(serviceName string) error {
	// 先停止服务
	s.StopService(serviceName)

	// 删除服务
	cmd := exec.Command("sc", "delete", serviceName)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	cmd.Run()

	return nil
}

// NewServiceManager 创建服务管理器
func NewServiceManager() *ServiceManager {
	return &ServiceManager{}
}
