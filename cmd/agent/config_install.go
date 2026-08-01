package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type installedConfig struct {
	configPath string
	backupPath string
}

func resolveCoreBinary(configuredPath, configDir, coreType string) string {
	candidates := []string{
		filepath.Join(configDir, serviceManager.GetBinaryName(coreType)),
		configuredPath,
	}
	for _, candidate := range candidates {
		if candidate == "" {
			continue
		}
		info, err := os.Stat(candidate)
		if err == nil && !info.IsDir() {
			return candidate
		}
	}
	return candidates[0]
}

func validateCoreConfig(corePath, configPath, workDir string) error {
	cmd := exec.Command(corePath, "check", "-c", configPath)
	cmd.Dir = workDir
	output, err := cmd.CombinedOutput()
	if err == nil {
		return nil
	}
	detail := strings.TrimSpace(string(bytes.TrimSpace(output)))
	if detail == "" {
		return fmt.Errorf("执行 %s check 失败: %w", filepath.Base(corePath), err)
	}
	return fmt.Errorf("执行 %s check 失败: %w: %s", filepath.Base(corePath), err, detail)
}

func installValidatedConfig(configPath string, content []byte, validate func(string) error) (_ *installedConfig, retErr error) {
	dir := filepath.Dir(configPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("创建配置目录失败: %w", err)
	}

	originalInfo, statErr := os.Stat(configPath)
	if statErr != nil && !os.IsNotExist(statErr) {
		return nil, fmt.Errorf("读取当前配置信息失败: %w", statErr)
	}

	tempFile, err := os.CreateTemp(dir, "."+filepath.Base(configPath)+".tmp-*")
	if err != nil {
		return nil, fmt.Errorf("创建临时配置失败: %w", err)
	}
	tempPath := tempFile.Name()
	defer func() {
		if retErr != nil {
			_ = os.Remove(tempPath)
		}
	}()

	if err := tempFile.Chmod(0600); err != nil {
		_ = tempFile.Close()
		return nil, fmt.Errorf("设置临时配置权限失败: %w", err)
	}
	if _, err := tempFile.Write(content); err != nil {
		_ = tempFile.Close()
		return nil, fmt.Errorf("写入临时配置失败: %w", err)
	}
	if err := tempFile.Sync(); err != nil {
		_ = tempFile.Close()
		return nil, fmt.Errorf("同步临时配置失败: %w", err)
	}
	if err := tempFile.Close(); err != nil {
		return nil, fmt.Errorf("关闭临时配置失败: %w", err)
	}

	if originalInfo != nil {
		if err := os.Chmod(tempPath, originalInfo.Mode().Perm()); err != nil {
			return nil, fmt.Errorf("继承配置权限失败: %w", err)
		}
		if err := applyFileOwnership(tempPath, originalInfo); err != nil {
			return nil, fmt.Errorf("继承配置属主失败: %w", err)
		}
	}

	if err := validate(tempPath); err != nil {
		return nil, fmt.Errorf("新配置无效: %w", err)
	}

	installed := &installedConfig{configPath: configPath}
	if originalInfo != nil {
		installed.backupPath = configPath + ".bak." + time.Now().Format("20060102150405.000000000")
		if err := os.Rename(configPath, installed.backupPath); err != nil {
			return nil, fmt.Errorf("备份当前配置失败: %w", err)
		}
	}

	if err := os.Rename(tempPath, configPath); err != nil {
		if installed.backupPath != "" {
			_ = os.Rename(installed.backupPath, configPath)
		}
		return nil, fmt.Errorf("替换配置失败: %w", err)
	}

	return installed, nil
}

func rollbackInstalledConfig(installed *installedConfig) error {
	if installed == nil || installed.backupPath == "" {
		return fmt.Errorf("没有可恢复的旧配置")
	}
	failedPath := installed.configPath + ".failed." + time.Now().Format("20060102150405.000000000")
	if err := os.Rename(installed.configPath, failedPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("保留失败配置失败: %w", err)
	}
	if err := os.Rename(installed.backupPath, installed.configPath); err != nil {
		_ = os.Rename(failedPath, installed.configPath)
		return fmt.Errorf("恢复旧配置失败: %w", err)
	}
	return nil
}
