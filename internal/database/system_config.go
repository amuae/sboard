package database

import (
	"strconv"

	"gorm.io/gorm"
)

// GetSystemConfig 获取系统配置
func GetSystemConfig(key string) (string, error) {
	var config SystemConfig
	err := DB.Where("key = ?", key).First(&config).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return "", nil
		}
		return "", err
	}
	return config.Value, nil
}

// SetSystemConfig 设置系统配置
func SetSystemConfig(key, value, description string) error {
	var config SystemConfig
	err := DB.Where("key = ?", key).First(&config).Error

	if err == gorm.ErrRecordNotFound {
		// 创建新配置
		config = SystemConfig{
			Key:         key,
			Value:       value,
			Description: description,
		}
		return DB.Create(&config).Error
	} else if err != nil {
		return err
	}

	// 更新现有配置
	return DB.Model(&config).Updates(map[string]interface{}{
		"value":       value,
		"description": description,
	}).Error
}

// GetDisablePasswordLogin 获取是否禁用密码登录
func GetDisablePasswordLogin() bool {
	value, err := GetSystemConfig("disable_password_login")
	if err != nil || value == "" {
		return false
	}
	disabled, _ := strconv.ParseBool(value)
	return disabled
}

// SetDisablePasswordLogin 设置是否禁用密码登录
func SetDisablePasswordLogin(disabled bool) error {
	value := "false"
	if disabled {
		value = "true"
	}
	return SetSystemConfig("disable_password_login", value, "禁用密码登录，强制使用 OAuth")
}
