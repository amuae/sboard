package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/sboard-go/sboard/internal/database"
)

// ========== 系统设置 API ==========

// 默认配置项
var defaultSettings = map[string]string{
	"subscribe_domain":    "",           // 订阅域名
	"subscribe_path":      "/sublink",   // 订阅路径
	"site_title":          "SBoard",     // 站点标题
	"default_core_type":   "sing-box",   // 默认核心类型
	"default_expiry_days": "30",         // 默认过期天数
	"ssh_private_key":     "",           // SSH 私钥路径
	"deploy_timeout":      "60",         // 部署超时时间(秒)
	"auto_sync_config":    "true",       // 自动同步配置
	"cert_path":           "server.crt", // 默认证书路径
	"key_path":            "server.key", // 默认密钥路径
}

// SettingsResponse 设置响应
type SettingsResponse struct {
	SubscribeDomain   string `json:"subscribe_domain"`
	SubscribePath     string `json:"subscribe_path"`
	SiteTitle         string `json:"site_title"`
	DefaultCoreType   string `json:"default_core_type"`
	DefaultExpiryDays string `json:"default_expiry_days"`
	SshPrivateKey     string `json:"ssh_private_key"`
	DeployTimeout     string `json:"deploy_timeout"`
	AutoSyncConfig    string `json:"auto_sync_config"`
	CertPath          string `json:"cert_path"`
	KeyPath           string `json:"key_path"`
}

// handleGetSettings 获取系统设置
func (s *Server) handleGetSettings(c *gin.Context) {
	settings := make(map[string]string)

	// 先填充默认值
	for k, v := range defaultSettings {
		settings[k] = v
	}

	// 从数据库加载配置
	var configs []database.SystemConfig
	if err := database.GetDB().Find(&configs).Error; err != nil {
		errorJSON(c, http.StatusInternalServerError, "读取系统设置失败")
		return
	}
	for _, cfg := range configs {
		settings[cfg.Key] = cfg.Value
	}

	successJSON(c, settings)
}

// handleUpdateSettings 更新系统设置
func (s *Server) handleUpdateSettings(c *gin.Context) {
	var req map[string]string
	if err := c.ShouldBindJSON(&req); err != nil {
		errorJSON(c, http.StatusBadRequest, "请求参数错误")
		return
	}

	// 遍历更新每个设置项
	for key, value := range req {
		// 检查是否为有效的配置项
		if _, ok := defaultSettings[key]; !ok {
			continue
		}

		// 查找或创建配置
		var config database.SystemConfig
		result := database.GetDB().Where("key = ?", key).First(&config)
		if result.Error != nil {
			// 创建新配置
			config = database.SystemConfig{
				Key:   key,
				Value: value,
			}
			database.GetDB().Create(&config)
		} else {
			// 更新现有配置
			config.Value = value
			database.GetDB().Save(&config)
		}
	}

	successMsgJSON(c, "设置保存成功")
}

// GetSetting 获取单个设置值
func GetSetting(key string) string {
	var config database.SystemConfig
	if err := database.GetDB().Where("key = ?", key).First(&config).Error; err != nil {
		// 返回默认值
		if defaultValue, ok := defaultSettings[key]; ok {
			return defaultValue
		}
		return ""
	}
	return config.Value
}

// SetSetting 设置单个配置值
func SetSetting(key, value string) error {
	var config database.SystemConfig
	result := database.GetDB().Where("key = ?", key).First(&config)
	if result.Error != nil {
		config = database.SystemConfig{
			Key:   key,
			Value: value,
		}
		return database.GetDB().Create(&config).Error
	}
	config.Value = value
	return database.GetDB().Save(&config).Error
}

// ========== 配置预览 API ==========

// handlePreviewConfig 预览配置文件
func (s *Server) handlePreviewConfig(c *gin.Context) {
	configType := c.Query("type")
	if configType == "" {
		configType = "sing-box"
	}

	serverIDStr := c.Query("server")

	// 如果指定了服务器，生成该服务器的配置
	if serverIDStr != "" {
		var server database.Server
		if err := database.GetDB().First(&server, serverIDStr).Error; err != nil {
			errorJSON(c, http.StatusNotFound, "服务器不存在")
			return
		}

		config, err := GenerateServerConfig(&server, configType)
		if err != nil {
			errorJSON(c, http.StatusInternalServerError, "生成配置失败: "+err.Error())
			return
		}

		successJSON(c, gin.H{
			"type":   configType,
			"server": server.Name,
			"config": config,
		})
		return
	}

	// 否则生成全局配置预览
	var content string
	var err error

	content, err = previewSingBoxConfig()

	if err != nil {
		errorJSON(c, http.StatusInternalServerError, "生成预览失败: "+err.Error())
		return
	}

	successJSON(c, gin.H{
		"type":   configType,
		"config": content,
	})
}

// previewSingBoxConfig 预览 Sing-Box 配置（实时生成）
func previewSingBoxConfig() (string, error) {
	return generateGlobalSingBoxConfig()
}
