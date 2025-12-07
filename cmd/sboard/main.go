package main

import (
	"embed"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/sboard-go/sboard/internal/config"
	"github.com/sboard-go/sboard/internal/database"
	"github.com/sboard-go/sboard/internal/geoip"
	"github.com/sboard-go/sboard/internal/handler"
	"github.com/sboard-go/sboard/internal/scheduler"
	"github.com/sboard-go/sboard/internal/storage"
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
		showVersion bool
		configPath  string
		listenAddr  string
		dataDir     string
	)

	flag.BoolVar(&showVersion, "v", false, "显示版本号")
	flag.StringVar(&configPath, "c", "data/config.yaml", "配置文件路径")
	flag.StringVar(&listenAddr, "l", "", "监听地址 (覆盖配置文件)")
	flag.StringVar(&dataDir, "d", "data", "数据目录")
	flag.Parse()

	if showVersion {
		fmt.Printf("SBoard %s (commit: %s)\n", Version, CommitHash)
		os.Exit(0)
	}

	log.Printf("SBoard %s 启动中...", Version)

	// 确保数据目录存在
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		log.Fatalf("创建数据目录失败: %v", err)
	}

	// 释放 storage 目录（如果不存在）
	if err := storage.ExtractStorage("storage"); err != nil {
		log.Printf("释放 storage 目录失败: %v", err)
	}

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

	// 初始化数据库
	dbPath := fmt.Sprintf("%s/sboard.db", cfg.Data.Dir)
	if err := database.Init(dbPath); err != nil {
		log.Fatalf("数据库初始化失败: %v", err)
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

	// 创建并启动 HTTP 服务器
	server := handler.NewServer(cfg, frontendFS)

	// 优雅关闭
	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		<-sigChan
		log.Println("收到关闭信号，正在停止...")
		sched.Stop()
		os.Exit(0)
	}()

	log.Printf("SBoard 服务启动于 %s", cfg.Server.Listen)
	if err := server.Run(); err != nil {
		log.Fatalf("服务器启动失败: %v", err)
	}
}
