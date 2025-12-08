package database

import (
	"encoding/json"
)

// GetOAuthProvider 获取指定名称的 OAuth 提供商配置
func GetOAuthProvider(name string) (*OAuthProvider, error) {
	var provider OAuthProvider
	if err := DB.Where("name = ?", name).First(&provider).Error; err != nil {
		return nil, err
	}
	return &provider, nil
}

// GetAllOAuthProviders 获取所有 OAuth 提供商配置
func GetAllOAuthProviders() ([]OAuthProvider, error) {
	var providers []OAuthProvider
	if err := DB.Find(&providers).Error; err != nil {
		return nil, err
	}
	return providers, nil
}

// GetEnabledOAuthProviders 获取已启用的 OAuth 提供商
func GetEnabledOAuthProviders() ([]OAuthProvider, error) {
	var providers []OAuthProvider
	if err := DB.Where("enabled = ?", true).Find(&providers).Error; err != nil {
		return nil, err
	}
	return providers, nil
}

// SaveOAuthProvider 保存 OAuth 提供商配置
func SaveOAuthProvider(provider *OAuthProvider) error {
	return DB.Save(provider).Error
}

// DeleteOAuthProvider 删除 OAuth 提供商配置
func DeleteOAuthProvider(name string) error {
	return DB.Delete(&OAuthProvider{}, "name = ?", name).Error
}

// GetGitHubOAuthConfig 获取 GitHub OAuth 配置（便捷方法）
func GetGitHubOAuthConfig() (*GitHubOAuthAddition, bool, error) {
	provider, err := GetOAuthProvider("github")
	if err != nil {
		return nil, false, err
	}

	var addition GitHubOAuthAddition
	if provider.Addition != "" {
		if err := json.Unmarshal([]byte(provider.Addition), &addition); err != nil {
			return nil, provider.Enabled, err
		}
	}

	return &addition, provider.Enabled, nil
}

// SaveGitHubOAuthConfig 保存 GitHub OAuth 配置（便捷方法）
func SaveGitHubOAuthConfig(enabled bool, addition *GitHubOAuthAddition) error {
	additionJSON, err := json.Marshal(addition)
	if err != nil {
		return err
	}

	provider := &OAuthProvider{
		Name:     "github",
		Enabled:  enabled,
		Addition: string(additionJSON),
	}

	return SaveOAuthProvider(provider)
}

// InitDefaultOAuthProviders 初始化默认的 OAuth 提供商（如果不存在）
func InitDefaultOAuthProviders() error {
	// GitHub
	if _, err := GetOAuthProvider("github"); err != nil {
		provider := &OAuthProvider{
			Name:     "github",
			Enabled:  false,
			Addition: `{"client_id":"","client_secret":"","allowed_users":[]}`,
		}
		if err := SaveOAuthProvider(provider); err != nil {
			return err
		}
	}
	return nil
}
