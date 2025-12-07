package config

import (
	"os"

	"gopkg.in/yaml.v3"
)

// Config 应用配置
type Config struct {
	Server   ServerConfig   `yaml:"server"`
	Data     DataConfig     `yaml:"data"`
	Security SecurityConfig `yaml:"security"`
	SSH      SSHConfig      `yaml:"ssh"`
	Cores    map[string]CoreConfig `yaml:"cores"`
}

// ServerConfig 服务器配置
type ServerConfig struct {
	Listen string `yaml:"listen"`
	Debug  bool   `yaml:"debug"`
}

// DataConfig 数据配置
type DataConfig struct {
	Dir string `yaml:"dir"`
}

// SecurityConfig 安全配置
type SecurityConfig struct {
	JWTSecret     string `yaml:"jwt_secret"`
	JWTExpireHour int    `yaml:"jwt_expire_hour"`
	SessionName   string `yaml:"session_name"`
}

// SSHConfig SSH配置
type SSHConfig struct {
	PrivateKeyPath string `yaml:"private_key_path"`
	Timeout        int    `yaml:"timeout"`
}

// CoreConfig 代理核心配置
type CoreConfig struct {
	Name           string `yaml:"name"`
	ConfigPath     string `yaml:"config_path"`
	ConfigFile     string `yaml:"config_file"`
	RestartCommand string `yaml:"restart_command"`
}

// Default 返回默认配置
func Default() *Config {
	return &Config{
		Server: ServerConfig{
			Listen: "0.0.0.0:8080",
			Debug:  false,
		},
		Data: DataConfig{
			Dir: "data",
		},
		Security: SecurityConfig{
			JWTSecret:     generateRandomString(32),
			JWTExpireHour: 168, // 7天
			SessionName:   "sboard_token",
		},
		SSH: SSHConfig{
			PrivateKeyPath: "~/.ssh/sboard_ed25519",
			Timeout:        30,
		},
		Cores: map[string]CoreConfig{
			"sing-box": {
				Name:           "sing-box",
				ConfigPath:     "/etc/sing-box/",
				ConfigFile:     "config.json",
				RestartCommand: "systemctl restart sing-box",
			},
			"mihomo": {
				Name:           "mihomo",
				ConfigPath:     "/etc/mihomo/",
				ConfigFile:     "config.yaml",
				RestartCommand: "systemctl restart mihomo",
			},
		},
	}
}

// Load 从文件加载配置
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	cfg := Default()
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}

// Save 保存配置到文件
func (c *Config) Save(path string) error {
	data, err := yaml.Marshal(c)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// generateRandomString 生成随机字符串
func generateRandomString(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[i%len(charset)]
	}
	return string(b)
}
