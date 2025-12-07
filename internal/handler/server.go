package handler

import (
	"embed"
	"io/fs"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/sboard-go/sboard/internal/config"
	"github.com/sboard-go/sboard/internal/middleware"
)

// Server HTTP 服务器
type Server struct {
	config     *config.Config
	router     *gin.Engine
	frontendFS embed.FS
}

// NewServer 创建新的服务器实例
func NewServer(cfg *config.Config, frontendFS embed.FS) *Server {
	// 设置 Gin 模式
	if cfg.Server.Debug {
		gin.SetMode(gin.DebugMode)
	} else {
		gin.SetMode(gin.ReleaseMode)
	}

	// 初始化 JWT
	middleware.InitJWT(cfg.Security.JWTSecret)

	router := gin.New()
	router.Use(gin.Recovery())
	// 不输出请求日志
	router.Use(middleware.CORSMiddleware())

	s := &Server{
		config:     cfg,
		router:     router,
		frontendFS: frontendFS,
	}

	s.setupRoutes()
	return s
}

// setupRoutes 设置路由
func (s *Server) setupRoutes() {
	// API 路由组
	api := s.router.Group("/api")
	{
		// 认证路由（无需登录）
		auth := api.Group("/auth")
		{
			auth.POST("/login", s.handleLogin)
		}

		// 需要认证的路由
		protected := api.Group("")
		protected.Use(middleware.AuthMiddleware())
		{
			// 认证相关
			protected.POST("/auth/logout", s.handleLogout)
			protected.POST("/auth/change-password", s.handleChangePassword)
			protected.POST("/auth/change-username", s.handleChangeUsername)
			protected.GET("/auth/me", s.handleGetCurrentUser)

			// 用户管理
			users := protected.Group("/users")
			{
				users.GET("", s.handleListUsers)
				users.POST("", s.handleCreateUser)
				users.GET("/:id", s.handleGetUser)
				users.PUT("/:id", s.handleUpdateUser)
				users.DELETE("/:id", s.handleDeleteUser)
			}

			// 节点管理
			nodes := protected.Group("/nodes")
			{
				nodes.GET("", s.handleListNodes)
				nodes.POST("", s.handleCreateNode)
				nodes.GET("/:id", s.handleGetNode)
				nodes.PUT("/:id", s.handleUpdateNode)
				nodes.DELETE("/:id", s.handleDeleteNode)
				nodes.POST("/generate-reality-keys", s.handleGenerateRealityKeys)
				nodes.POST("/batch-delete", s.handleBatchDeleteNodes)
			}

			// 服务器管理
			servers := protected.Group("/servers")
			{
				servers.GET("", s.handleListServers)
				servers.GET("/status", s.handleGetAllServersStatus) // 批量获取状态
				servers.POST("", s.handleCreateServer)
				servers.POST("/reorder", s.handleReorderServers) // 重新排序
				servers.GET("/:id", s.handleGetServer)
				servers.PUT("/:id", s.handleUpdateServer)
				servers.DELETE("/:id", s.handleDeleteServer)
				servers.POST("/:id/nodes", s.handleSetServerNodes)
				servers.POST("/:id/deploy", s.handleDeployServer)
				servers.GET("/:id/test", s.handleTestServer)
				servers.GET("/:id/status", s.handleGetServerStatus)
				// 节点配置管理
				servers.GET("/:id/node-configs", s.handleGetServerNodeConfigs)
				servers.POST("/:id/node-configs/:nodeId", s.handleSaveServerNodeConfig)
				servers.DELETE("/:id/node-configs/:nodeId", s.handleDeleteServerNodeConfig)
				// 服务器配置文件
				servers.GET("/:id/config", s.handleGetServerConfig)
				servers.GET("/:id/deploy-folder", s.handleGetDeployFolder)
				// Agent 相关
				servers.GET("/:id/agent/status", s.handleGetAgentStatus)
				servers.POST("/:id/agent/command", s.handleSendAgentCommand)
				servers.POST("/:id/agent/sync", s.handleSyncConfigToAgent)
				servers.POST("/:id/agent/deploy", s.handleDeployCoreToAgent)
				servers.POST("/:id/agent/regenerate-token", s.handleRegenerateAgentToken)
			}

			protected.GET("/nodes/config", s.handleGetNodeConfig) // 系统设置
			protected.GET("/settings", s.handleGetSettings)
			protected.POST("/settings", s.handleUpdateSettings)

			// 配置预览
			protected.GET("/config/preview", s.handlePreviewConfig)

			// 用户到期检查
			protected.POST("/users/check-expiry", s.handleCheckExpiredUsers)
			protected.GET("/users/expiring", s.handleGetExpiringUsers)
		}
	}

	// 订阅链接（无需认证，使用 UUID）
	s.router.GET("/sublink", s.handleSublink)
	s.router.GET("/sublink.php", s.handleSublinkPHP)    // 兼容旧 PHP 路径
	s.router.GET("/sub/:uuid", s.handleSubByUUID)       // 短订阅链接
	s.router.GET("/subscribe/:uuid", s.handleSubByUUID) // 兼容格式

	// Agent WebSocket 端点（使用 token 认证）
	s.router.GET("/api/agent/ws", s.handleAgentWebSocket)

	// Agent 安装脚本和二进制下载（无需认证）
	s.router.GET("/install-agent.sh", s.handleInstallAgentScript)
	s.router.GET("/download/agent-linux-:arch", s.handleDownloadAgent)

	// 静态文件服务（前端）
	s.serveFrontend()
}

// serveFrontend 提供前端静态文件服务
func (s *Server) serveFrontend() {
	// 尝试从嵌入的文件系统提供静态文件
	webFS, err := fs.Sub(s.frontendFS, "web")
	if err != nil {
		// 如果没有嵌入的前端文件，使用简单的响应
		s.router.NoRoute(func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{
				"message": "SBoard API Server",
				"version": "3.0.0",
				"docs":    "/api",
			})
		})
		return
	}

	// 提供 assets 静态文件
	assetsFS, _ := fs.Sub(webFS, "assets")
	s.router.StaticFS("/assets", http.FS(assetsFS))

	// SPA 回退
	s.router.NoRoute(func(c *gin.Context) {
		// 尝试提供 index.html
		data, err := fs.ReadFile(webFS, "index.html")
		if err != nil {
			c.JSON(http.StatusOK, gin.H{
				"message": "SBoard API Server",
				"version": "3.0.0",
			})
			return
		}
		c.Data(http.StatusOK, "text/html; charset=utf-8", data)
	})
}

// Run 启动服务器
func (s *Server) Run() error {
	return s.router.Run(s.config.Server.Listen)
}
