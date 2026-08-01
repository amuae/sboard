package config

import (
	crand "crypto/rand"
	mrand "math/rand"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"
)

// Config 应用配置
type Config struct {
	Server   ServerConfig   `yaml:"server"`
	Data     DataConfig     `yaml:"data"`
	Security SecurityConfig `yaml:"security"`
	OAuth    OAuthConfig    `yaml:"oauth"`
}

// ServerConfig 服务器配置
type ServerConfig struct {
	Listen string `yaml:"listen"`
	Debug  bool   `yaml:"debug"`
	Domain string `yaml:"domain"` // 面板入口域名
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

// OAuthConfig OAuth 认证配置
type OAuthConfig struct {
	DisablePasswordLogin bool `yaml:"disable_password_login"` // 启用 OAuth 后禁用密码登录
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
		OAuth: OAuthConfig{
			DisablePasswordLogin: false,
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

// LoadOrCreate loads an existing config or persists a new default config.
// Persisting the generated JWT secret keeps sessions and derived credentials
// stable across restarts.
func LoadOrCreate(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if !os.IsNotExist(err) {
			return nil, err
		}

		cfg := Default()
		if err := cfg.Save(path); err != nil {
			return nil, err
		}
		return cfg, nil
	}

	cfg := Default()
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, err
	}

	// Older or hand-written configs may omit the secret. Save the generated
	// value once instead of silently rotating it on every process start.
	var persisted struct {
		Security struct {
			JWTSecret string `yaml:"jwt_secret"`
		} `yaml:"security"`
	}
	if err := yaml.Unmarshal(data, &persisted); err != nil {
		return nil, err
	}
	if persisted.Security.JWTSecret == "" {
		if err := cfg.Save(path); err != nil {
			return nil, err
		}
	}

	return cfg, nil
}

// Save 保存配置到文件
func (c *Config) Save(path string) error {
	data, err := yaml.Marshal(c)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}

// generateRandomString 生成密码学安全的随机字符串
func generateRandomString(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, length)
	if _, err := crand.Read(b); err != nil {
		// fallback to pseudo-random with timestamp seed if CSPRNG fails
		rng := mrand.New(mrand.NewSource(time.Now().UnixNano()))
		for i := range b {
			b[i] = charset[rng.Intn(len(charset))]
		}
		return string(b)
	}
	for i := range b {
		b[i] = charset[int(b[i])%len(charset)]
	}
	return string(b)
}
