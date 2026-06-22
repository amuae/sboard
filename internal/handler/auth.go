package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/sboard-go/sboard/internal/database"
	"github.com/sboard-go/sboard/internal/middleware"
)

// LoginRequest 登录请求
type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// handleLogin 处理登录
func (s *Server) handleLogin(c *gin.Context) {
	// 检查是否禁用密码登录
	if database.GetDisablePasswordLogin() {
		// 检查是否有启用的 OAuth 提供商
		_, githubEnabled, _ := database.GetGitHubOAuthConfig()
		if githubEnabled {
			errorJSON(c, http.StatusForbidden, "密码登录已禁用，请使用 OAuth 登录")
			return
		}
	}

	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errorJSON(c, http.StatusBadRequest, "请求参数错误")
		return
	}

	// 查找管理员
	var admin database.Admin
	if err := database.GetDB().Where("username = ?", req.Username).First(&admin).Error; err != nil {
		errorJSON(c, http.StatusUnauthorized, "用户名或密码错误")
		return
	}

	// 验证密码
	if !admin.CheckPassword(req.Password) {
		errorJSON(c, http.StatusUnauthorized, "用户名或密码错误")
		return
	}

	// 生成 JWT token
	token, err := middleware.GenerateToken(admin.ID, admin.Username, s.config.Security.JWTExpireHour)
	if err != nil {
		errorJSON(c, http.StatusInternalServerError, "生成令牌失败")
		return
	}

	// 设置 cookie（HTTPS-only 如果面板运行在 TLS 后）
	isSecure := c.Request.TLS != nil || c.GetHeader("X-Forwarded-Proto") == "https"
	c.SetCookie(
		s.config.Security.SessionName,
		token,
		s.config.Security.JWTExpireHour*3600,
		"/",
		"",
		isSecure,
		true,
	)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "登录成功",
		"data": gin.H{
			"token":    token,
			"username": admin.Username,
		},
	})
}

// handleLogout 处理登出
func (s *Server) handleLogout(c *gin.Context) {
	// 清除 cookie
	c.SetCookie(
		s.config.Security.SessionName,
		"",
		-1,
		"/",
		"",
		false,
		true,
	)
	successMsgJSON(c, "登出成功")
}

// ChangePasswordRequest 修改密码请求
type ChangePasswordRequest struct {
	OldPassword     string `json:"old_password" binding:"required"`
	NewPassword     string `json:"new_password" binding:"required,min=6"`
	ConfirmPassword string `json:"confirm_password" binding:"required"`
}

// handleChangePassword 处理修改密码
func (s *Server) handleChangePassword(c *gin.Context) {
	var req ChangePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errorJSON(c, http.StatusBadRequest, "请求参数错误")
		return
	}

	if req.NewPassword != req.ConfirmPassword {
		errorJSON(c, http.StatusBadRequest, "两次输入的密码不一致")
		return
	}

	userID := c.GetUint("user_id")
	var admin database.Admin
	if err := database.GetDB().First(&admin, userID).Error; err != nil {
		errorJSON(c, http.StatusNotFound, "用户不存在")
		return
	}

	// 验证旧密码
	if !admin.CheckPassword(req.OldPassword) {
		errorJSON(c, http.StatusUnauthorized, "当前密码错误")
		return
	}

	// 更新密码
	if err := admin.SetPassword(req.NewPassword); err != nil {
		errorJSON(c, http.StatusInternalServerError, "密码加密失败")
		return
	}

	if err := database.GetDB().Save(&admin).Error; err != nil {
		errorJSON(c, http.StatusInternalServerError, "保存失败")
		return
	}

	successMsgJSON(c, "密码修改成功")
}

// ChangeUsernameRequest 修改用户名请求
type ChangeUsernameRequest struct {
	Password    string `json:"password" binding:"required"`
	NewUsername string `json:"new_username" binding:"required,min=3"`
}

// handleChangeUsername 处理修改用户名
func (s *Server) handleChangeUsername(c *gin.Context) {
	var req ChangeUsernameRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errorJSON(c, http.StatusBadRequest, "请求参数错误")
		return
	}

	userID := c.GetUint("user_id")
	var admin database.Admin
	if err := database.GetDB().First(&admin, userID).Error; err != nil {
		errorJSON(c, http.StatusNotFound, "用户不存在")
		return
	}

	// 验证密码
	if !admin.CheckPassword(req.Password) {
		errorJSON(c, http.StatusUnauthorized, "密码错误")
		return
	}

	// 检查新用户名是否已存在
	var count int64
	database.GetDB().Model(&database.Admin{}).Where("username = ? AND id != ?", req.NewUsername, userID).Count(&count)
	if count > 0 {
		errorJSON(c, http.StatusBadRequest, "用户名已存在")
		return
	}

	// 更新用户名
	admin.Username = req.NewUsername
	if err := database.GetDB().Save(&admin).Error; err != nil {
		errorJSON(c, http.StatusInternalServerError, "保存失败")
		return
	}

	successMsgJSON(c, "用户名修改成功")
}

// handleGetCurrentUser 获取当前用户信息
func (s *Server) handleGetCurrentUser(c *gin.Context) {
	username := c.GetString("username")
	successJSON(c, gin.H{
		"username": username,
	})
}

