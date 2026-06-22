package main

import (
	"context"
	"embed"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/sboard-go/sboard/internal/config"
	"github.com/sboard-go/sboard/internal/database"
	"github.com/sboard-go/sboard/internal/geoip"
	"github.com/sboard-go/sboard/internal/handler"
	"github.com/sboard-go/sboard/internal/scheduler"
)

var (
	Version    = "3.0.0"
	CommitHash = "dev"
)

//go:embed all:web
var frontendFS embed.FS

func main() {
	// 命令行参数
	var (
		showVersion   bool
		configPath    string
		listenAddr    string
		dataDir       string
		initAdmin     bool
		initAdminUser string
		initAdminPass string
		permitLogin   bool
	)

	flag.BoolVar(&showVersion, "v", false, "显示版本号")
	flag.StringVar(&configPath, "c", "", "配置文件路径 (默认: <数据目录>/config.yaml)")
	flag.StringVar(&listenAddr, "l", "", "监听地址 (覆盖配置文件)")
	flag.StringVar(&dataDir, "d", "data", "数据目录")
	flag.BoolVar(&initAdmin, "init-admin", false, "初始化管理员账户后退出")
	flag.StringVar(&initAdminUser, "admin-user", "admin", "管理员用户名")
	flag.StringVar(&initAdminPass, "admin-pass", "admin123", "管理员密码")
	flag.BoolVar(&permitLogin, "permit-login", false, "强制允许密码登录（修改配置文件后退出）")
	flag.Parse()

	if showVersion {
		fmt.Printf("SBoard %s (commit: %s)\n", Version, CommitHash)
		os.Exit(0)
	}

	// 如果未指定配置文件路径，使用数据目录下的 config.yaml
	if configPath == "" {
		configPath = fmt.Sprintf("%s/config.yaml", dataDir)
	}

	// 确保数据目录存在
	if err := os.MkdirAll(dataDir, 0700); err != nil {
		log.Fatalf("创建数据目录失败: %v", err)
	}

	// 初始化数据库
	dbPath := fmt.Sprintf("%s/sboard.db", dataDir)
	if err := database.Init(dbPath); err != nil {
		log.Fatalf("数据库初始化失败: %v", err)
	}

	// 初始化管理员账户模式
	if initAdmin {
		initAdminAccount(initAdminUser, initAdminPass)
		os.Exit(0)
	}

	// 强制允许密码登录模式
	if permitLogin {
		enablePasswordLogin(configPath)
		os.Exit(0)
	}

	log.Printf("SBoard %s 启动中...", Version)

	// 加载配置
	cfg, err := config.Load(configPath)
	if err != nil {
		log.Printf("配置文件不存在，使用默认配置")
		cfg = config.Default()
	}

	// 命令行参数覆盖
	if listenAddr != "" {
		cfg.Server.Listen = listenAddr
	}
	if dataDir != "" {
		cfg.Data.Dir = dataDir
	}

	// 初始化 GeoIP 数据库（如果不存在会自动从 GitHub 下载）
	geoipPath := fmt.Sprintf("%s/country.mmdb", cfg.Data.Dir)
	if err := geoip.Init(geoipPath); err != nil {
		log.Printf("GeoIP 初始化失败: %v (订阅将无法按 IP 判断国内外)", err)
	}

	// 启动定时任务调度器
	sched := scheduler.New()
	sched.SetConfigSyncCallback(handler.BroadcastConfigUpdate)
	sched.Start()

	// 创建 HTTP 服务器
	server := handler.NewServer(cfg, frontendFS)

	// 优雅关闭
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// 在 goroutine 中启动服务器
	go func() {
		log.Printf("SBoard 服务启动于 %s", cfg.Server.Listen)
		if err := server.Run(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("服务器启动失败: %v", err)
		}
	}()

	// 等待关闭信号
	<-sigChan
	log.Println("收到关闭信号，正在停止...")

	// 停止调度器
	sched.Stop()

	// 优雅关闭 HTTP 服务器（等待最多 10 秒）
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		log.Printf("服务器关闭失败: %v", err)
	}

	// 关闭 GeoIP
	geoip.Close()

	log.Println("SBoard 已停止")
}

// initAdminAccount 初始化管理员账户（仅在无管理员时允许）
func initAdminAccount(username, password string) {
	// 安全检查：如果已存在任何管理员账户，拒绝通过命令行创建/修改
	var count int64
	database.DB.Model(&database.Admin{}).Count(&count)
	if count > 0 {
		log.Fatalf("安全限制: 已存在管理员账户，无法通过命令行初始化。请通过 Web 界面管理账户，或删除数据库文件后重新初始化。")
	}

	// 创建新管理员
	admin := database.Admin{Username: username}
	if err := admin.SetPassword(password); err != nil {
		log.Fatalf("密码加密失败: %v", err)
	}
	if err := database.DB.Create(&admin).Error; err != nil {
		log.Fatalf("创建管理员失败: %v", err)
	}
	log.Printf("管理员账户 '%s' 创建成功", username)
}

// enablePasswordLogin 强制允许密码登录
func enablePasswordLogin(configPath string) {
	cfg, err := config.Load(configPath)
	if err != nil {
		log.Fatalf("加载配置文件失败: %v", err)
	}

	if !cfg.OAuth.DisablePasswordLogin {
		log.Printf("密码登录已经是启用状态")
		return
	}

	cfg.OAuth.DisablePasswordLogin = false
	if err := cfg.Save(configPath); err != nil {
		log.Fatalf("保存配置文件失败: %v", err)
	}
	log.Printf("密码登录已重新启用，请重启服务使配置生效")
}
