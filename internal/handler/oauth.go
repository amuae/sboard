package handler

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sboard-go/sboard/internal/database"
	"github.com/sboard-go/sboard/internal/middleware"
)

// GitHub OAuth 相关常量
const (
	githubAuthorizeURL = "https://github.com/login/oauth/authorize"
	githubTokenURL     = "https://github.com/login/oauth/access_token"
	githubUserURL      = "https://api.github.com/user"
)

// GitHubUser GitHub 用户信息
type GitHubUser struct {
	ID        int64  `json:"id"`
	Login     string `json:"login"`
	Email     string `json:"email"`
	AvatarURL string `json:"avatar_url"`
}

// oauthStates 存储 OAuth state（加互斥锁防止高并发 panic）
var (
	oauthStateMu sync.RWMutex
	oauthStates  = make(map[string]time.Time)
)

// setOAuthState 安全写入 OAuth state
func setOAuthState(state string, expiry time.Time) {
	oauthStateMu.Lock()
	defer oauthStateMu.Unlock()
	oauthStates[state] = expiry
}

// getOAuthState 安全读取并删除 OAuth state
func getOAuthState(state string) (time.Time, bool) {
	oauthStateMu.Lock()
	defer oauthStateMu.Unlock()
	expiry, ok := oauthStates[state]
	if ok {
		delete(oauthStates, state)
	}
	return expiry, ok
}

// generateState 生成随机 state
func generateState() string {
	b := make([]byte, 32)
	rand.Read(b)
	return base64.URLEncoding.EncodeToString(b)
}

// handleGetOAuthProviders 获取可用的 OAuth 提供商
func (s *Server) handleGetOAuthProviders(c *gin.Context) {
	providers := []gin.H{}

	// 从数据库获取 GitHub OAuth 配置
	githubConfig, githubEnabled, err := database.GetGitHubOAuthConfig()
	if err == nil && githubEnabled && githubConfig.ClientID != "" {
		providers = append(providers, gin.H{
			"name":    "github",
			"label":   "GitHub",
			"icon":    "github",
			"enabled": true,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"success":                true,
		"data":                   providers,
		"disable_password_login": database.GetDisablePasswordLogin() && len(providers) > 0,
	})
}

// handleGitHubLogin 发起 GitHub OAuth 登录
func (s *Server) handleGitHubLogin(c *gin.Context) {
	// 从数据库获取 GitHub OAuth 配置
	githubConfig, githubEnabled, err := database.GetGitHubOAuthConfig()
	if err != nil || !githubEnabled {
		errorJSON(c, http.StatusBadRequest, "GitHub 登录未启用")
		return
	}

	// 检查是否为授权模式
	authorizeMode := c.Query("authorize") == "true"

	// 生成 state 防止 CSRF
	state := generateState()
	setOAuthState(state, time.Now().Add(10*time.Minute))

	// 清理过期的 state
	go cleanupExpiredStates()

	// 构建授权 URL
	params := url.Values{}
	params.Set("client_id", githubConfig.ClientID)
	callbackURL := s.getGitHubCallbackURL(c)
	if authorizeMode {
		// 在回调 URL 中添加 authorize 标志
		callbackURL += "?authorize=true"
	}
	params.Set("redirect_uri", callbackURL)
	params.Set("scope", "read:user user:email")
	params.Set("state", state)

	authorizeURL := fmt.Sprintf("%s?%s", githubAuthorizeURL, params.Encode())

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"url": authorizeURL,
		},
	})
}

// handleGitHubCallback GitHub OAuth 回调
func (s *Server) handleGitHubCallback(c *gin.Context) {
	// 从数据库获取 GitHub OAuth 配置
	githubConfig, githubEnabled, err := database.GetGitHubOAuthConfig()
	if err != nil || !githubEnabled {
		c.Redirect(http.StatusFound, "/?error=oauth_disabled")
		return
	}

	code := c.Query("code")
	state := c.Query("state")
	errorParam := c.Query("error")
	authorizeMode := c.Query("authorize") == "true" // 检查是否为授权模式

	if errorParam != "" {
		if authorizeMode {
			c.Redirect(http.StatusFound, "/?error=oauth_authorize_failed")
		} else {
			c.Redirect(http.StatusFound, "/?error="+errorParam)
		}
		return
	}

	// 验证 state
	if _, ok := getOAuthState(state); !ok {
		c.Redirect(http.StatusFound, "/?error=invalid_state")
		return
	}

	// 交换 code 获取 access token
	accessToken, err := s.exchangeGitHubCode(code, c, githubConfig)
	if err != nil {
		c.Redirect(http.StatusFound, "/?error=token_exchange_failed")
		return
	}

	// 获取 GitHub 用户信息
	githubUser, err := s.getGitHubUser(accessToken)
	if err != nil {
		c.Redirect(http.StatusFound, "/?error=user_fetch_failed")
		return
	}

	// 授权模式：将用户添加到允许列表
	if authorizeMode {
		// 检查用户是否已在列表中
		userExists := false
		for _, allowedUser := range githubConfig.AllowedUsers {
			if strings.EqualFold(allowedUser, githubUser.Login) {
				userExists = true
				break
			}
		}

		if !userExists {
			// 添加用户到允许列表
			githubConfig.AllowedUsers = append(githubConfig.AllowedUsers, githubUser.Login)
			if err := database.SaveGitHubOAuthConfig(githubEnabled, githubConfig); err != nil {
				c.Redirect(http.StatusFound, "/?error=save_config_failed")
				return
			}
		}

		// 重定向回管理界面，显示成功消息
		c.Redirect(http.StatusFound, "/?oauth_authorized="+url.QueryEscape(githubUser.Login))
		return
	}

	// 普通登录模式：检查用户是否在允许列表中
	if len(githubConfig.AllowedUsers) > 0 {
		allowed := false
		for _, allowedUser := range githubConfig.AllowedUsers {
			if strings.EqualFold(allowedUser, githubUser.Login) {
				allowed = true
				break
			}
		}
		if !allowed {
			c.Redirect(http.StatusFound, "/?error=user_not_allowed")
			return
		}
	}

	// 查找或创建管理员账户
	var admin database.Admin
	result := database.DB.Where("auth_provider = ? AND o_auth_id = ?", "github", fmt.Sprintf("%d", githubUser.ID)).First(&admin)

	if result.Error != nil {
		// 检查是否存在同名本地用户，如果存在则关联
		existingResult := database.DB.Where("username = ? AND auth_provider = ?", githubUser.Login, "local").First(&admin)
		if existingResult.Error == nil {
			// 将本地用户关联到 GitHub
			database.DB.Model(&admin).Updates(map[string]interface{}{
				"auth_provider": "github",
				"o_auth_id":     fmt.Sprintf("%d", githubUser.ID),
				"email":         githubUser.Email,
				"avatar_url":    githubUser.AvatarURL,
			})
		} else {
			// 创建新的 OAuth 管理员
			admin = database.Admin{
				Username:     githubUser.Login,
				AuthProvider: "github",
				OAuthID:      fmt.Sprintf("%d", githubUser.ID),
				Email:        githubUser.Email,
				AvatarURL:    githubUser.AvatarURL,
			}
			if err := database.DB.Create(&admin).Error; err != nil {
				// 用户名可能冲突，添加后缀
				admin.Username = fmt.Sprintf("%s_gh", githubUser.Login)
				if err := database.DB.Create(&admin).Error; err != nil {
					c.Redirect(http.StatusFound, "/?error=user_create_failed")
					return
				}
			}
		}
	} else {
		// 更新用户信息
		database.DB.Model(&admin).Updates(map[string]interface{}{
			"email":      githubUser.Email,
			"avatar_url": githubUser.AvatarURL,
		})
	}

	// 生成 JWT token
	token, err := middleware.GenerateToken(admin.ID, admin.Username, s.config.Security.JWTExpireHour)
	if err != nil {
		c.Redirect(http.StatusFound, "/?error=token_generate_failed")
		return
	}

	// 重定向到首页，通过 URL 参数传递 token
	c.Redirect(http.StatusFound, "/?oauth_token="+url.QueryEscape(token))
}

// exchangeGitHubCode 用 code 交换 access token
func (s *Server) exchangeGitHubCode(code string, c *gin.Context, githubConfig *database.GitHubOAuthAddition) (string, error) {
	data := url.Values{}
	data.Set("client_id", githubConfig.ClientID)
	data.Set("client_secret", githubConfig.ClientSecret)
	data.Set("code", code)
	data.Set("redirect_uri", s.getGitHubCallbackURL(c))

	req, err := http.NewRequest("POST", githubTokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var result struct {
		AccessToken string `json:"access_token"`
		Error       string `json:"error"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", err
	}

	if result.Error != "" {
		return "", fmt.Errorf("github error: %s", result.Error)
	}

	return result.AccessToken, nil
}

// getGitHubUser 获取 GitHub 用户信息
func (s *Server) getGitHubUser(accessToken string) (*GitHubUser, error) {
	req, err := http.NewRequest("GET", githubUserURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var user GitHubUser
	if err := json.Unmarshal(body, &user); err != nil {
		return nil, err
	}

	return &user, nil
}

// getGitHubCallbackURL 获取回调 URL
func (s *Server) getGitHubCallbackURL(c *gin.Context) string {
	scheme := "http"
	if c.Request.TLS != nil || c.GetHeader("X-Forwarded-Proto") == "https" {
		scheme = "https"
	}
	host := c.Request.Host
	return fmt.Sprintf("%s://%s/api/auth/github/callback", scheme, host)
}

// cleanupExpiredStates 清理过期的 state
func cleanupExpiredStates() {
	oauthStateMu.Lock()
	defer oauthStateMu.Unlock()
	now := time.Now()
	for state, expiry := range oauthStates {
		if now.After(expiry) {
			delete(oauthStates, state)
		}
	}
}

// ========== OAuth 管理 API ==========

// OAuthProviderResponse OAuth 提供商响应
type OAuthProviderResponse struct {
	Name    string      `json:"name"`
	Label   string      `json:"label"`
	Enabled bool        `json:"enabled"`
	Config  interface{} `json:"config,omitempty"`
}

// handleGetOAuthProvidersAdmin 获取所有 OAuth 提供商（管理接口）
func (s *Server) handleGetOAuthProvidersAdmin(c *gin.Context) {
	providers := []OAuthProviderResponse{}

	// GitHub
	githubConfig, githubEnabled, err := database.GetGitHubOAuthConfig()
	githubResponse := OAuthProviderResponse{
		Name:    "github",
		Label:   "GitHub",
		Enabled: githubEnabled,
	}
	if err == nil {
		// 隐藏 client_secret 的真实值，只显示是否已配置
		githubResponse.Config = gin.H{
			"client_id":     githubConfig.ClientID,
			"has_secret":    githubConfig.ClientSecret != "",
			"allowed_users": githubConfig.AllowedUsers,
		}
	}
	providers = append(providers, githubResponse)

	c.JSON(http.StatusOK, gin.H{
		"success":                true,
		"data":                   providers,
		"disable_password_login": database.GetDisablePasswordLogin(),
	})
}

// handleGetOAuthProvider 获取指定的 OAuth 提供商配置
func (s *Server) handleGetOAuthProvider(c *gin.Context) {
	name := c.Param("name")

	switch name {
	case "github":
		githubConfig, githubEnabled, err := database.GetGitHubOAuthConfig()
		if err != nil {
			c.JSON(http.StatusOK, gin.H{
				"success": true,
				"data": gin.H{
					"name":    "github",
					"enabled": false,
					"config": gin.H{
						"client_id":     "",
						"has_secret":    false,
						"allowed_users": []string{},
					},
				},
			})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"data": gin.H{
				"name":    "github",
				"enabled": githubEnabled,
				"config": gin.H{
					"client_id":     githubConfig.ClientID,
					"has_secret":    githubConfig.ClientSecret != "",
					"allowed_users": githubConfig.AllowedUsers,
				},
			},
		})
	default:
		errorJSON(c, http.StatusNotFound, "未知的 OAuth 提供商")
	}
}

// SaveOAuthSettingsRequest 保存全局 OAuth 设置请求
type SaveOAuthSettingsRequest struct {
	DisablePasswordLogin bool `json:"disable_password_login"`
}

// handleSaveOAuthSettings 保存全局 OAuth 设置
func (s *Server) handleSaveOAuthSettings(c *gin.Context) {
	var req SaveOAuthSettingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errorJSON(c, http.StatusBadRequest, "无效的请求参数")
		return
	}

	// 保存到数据库
	if err := database.SetDisablePasswordLogin(req.DisablePasswordLogin); err != nil {
		errorJSON(c, http.StatusInternalServerError, "保存配置失败: "+err.Error())
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "全局 OAuth 设置已保存",
	})
}

// SaveGitHubOAuthRequest 保存 GitHub OAuth 请求
type SaveGitHubOAuthRequest struct {
	Enabled      bool     `json:"enabled"`
	ClientID     string   `json:"client_id"`
	ClientSecret string   `json:"client_secret"` // 空字符串表示不修改
	AllowedUsers []string `json:"allowed_users"`
}

// handleSaveOAuthProvider 保存 OAuth 提供商配置
func (s *Server) handleSaveOAuthProvider(c *gin.Context) {
	name := c.Param("name")

	switch name {
	case "github":
		var req SaveGitHubOAuthRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			errorJSON(c, http.StatusBadRequest, "无效的请求参数")
			return
		}

		// 如果 client_secret 为空，保留原有的 secret
		existingConfig, _, _ := database.GetGitHubOAuthConfig()
		clientSecret := req.ClientSecret
		if clientSecret == "" && existingConfig != nil {
			clientSecret = existingConfig.ClientSecret
		}

		addition := &database.GitHubOAuthAddition{
			ClientID:     req.ClientID,
			ClientSecret: clientSecret,
			AllowedUsers: req.AllowedUsers,
		}

		if err := database.SaveGitHubOAuthConfig(req.Enabled, addition); err != nil {
			errorJSON(c, http.StatusInternalServerError, "保存配置失败")
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"message": "保存成功",
		})
	default:
		errorJSON(c, http.StatusNotFound, "未知的 OAuth 提供商")
	}
}
