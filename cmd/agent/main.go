package main

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/md5"
	"embed"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/gorilla/websocket"
	"github.com/sboard-go/sboard/cmd/agent/netstatic"
	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/mem"
)

//go:embed embed/configs/*
var embeddedConfigs embed.FS

var (
	Version        = "dev" // 构建时通过 -ldflags "-X main.Version=..." 注入
	serviceManager = NewServiceManager()

	// directClient 绕过系统代理（HTTP_PROXY）用于获取本机公网 IP
	directClient = &http.Client{
		Transport: &http.Transport{Proxy: nil},
		Timeout:   3 * time.Second,
	}
)

// Agent 配置
type Config struct {
	PanelURL  string `json:"panel_url"`  // 面板 WebSocket 地址
	Token     string `json:"token"`      // 认证 Token
	AgentID   string `json:"agent_id"`   // Agent ID
	CorePath  string `json:"core_path"`  // 核心路径
	ConfigDir string `json:"config_dir"` // 配置目录
}

// Agent 客户端
type Agent struct {
	config    *Config
	conn      *websocket.Conn
	connMu    sync.Mutex
	startTime time.Time
	stopChan  chan struct{}
}

// MessageType 消息类型
type MessageType string

const (
	MsgTypeHeartbeat    MessageType = "heartbeat"
	MsgTypeRegister     MessageType = "register"
	MsgTypeStatus       MessageType = "status"
	MsgTypeCommandResp  MessageType = "command_resp"
	MsgTypeRegisterResp MessageType = "register_resp"
	MsgTypeCommand      MessageType = "command"
	MsgTypeDeployFile   MessageType = "deploy_file"
	MsgTypeDeployDir    MessageType = "deploy_dir"
	MsgTypeSyncConfig   MessageType = "sync_config"
	MsgTypeRestart      MessageType = "restart"
	MsgTypeDeployCore   MessageType = "deploy_core" // 部署核心
	MsgTypeSelfUpdate   MessageType = "self_update" // 自我更新
)

// Message 消息结构
type Message struct {
	Type      MessageType     `json:"type"`
	ID        string          `json:"id,omitempty"`
	Timestamp int64           `json:"timestamp"`
	Data      json.RawMessage `json:"data,omitempty"`
	Error     string          `json:"error,omitempty"`
}

func main() {
	var (
		configPath string
		showVer    bool
	)

	flag.StringVar(&configPath, "c", "agent.json", "配置文件路径")
	flag.BoolVar(&showVer, "v", false, "显示版本号")
	flag.Parse()

	if showVer {
		fmt.Printf("SBoard Agent %s\n", Version)
		os.Exit(0)
	}

	// 加载配置
	config, err := loadConfig(configPath)
	if err != nil {
		log.Fatalf("加载配置失败: %v", err)
	}

	// 初始化流量统计模块
	netStaticPath := filepath.Join(filepath.Dir(configPath), "net_static.json")
	if err := netstatic.Init(netStaticPath); err != nil {
		log.Printf("警告: 初始化流量统计失败: %v", err)
	}
	if err := netstatic.Start(); err != nil {
		log.Printf("警告: 启动流量统计失败: %v", err)
	}

	// 创建 Agent
	agent := &Agent{
		config:    config,
		startTime: time.Now(),
		stopChan:  make(chan struct{}),
	}

	// 启动 Agent
	go agent.run()

	// 等待退出信号
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	log.Println("正在关闭 Agent...")
	close(agent.stopChan)
	agent.disconnect()

	// 停止流量统计（保存数据）
	if err := netstatic.Stop(); err != nil {
		log.Printf("警告: 停止流量统计失败: %v", err)
	}
}

func loadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, err
	}

	// 默认值
	if config.CorePath == "" {
		config.CorePath = "/usr/local/bin/sing-box"
	}
	if config.ConfigDir == "" {
		// 默认使用核心程序所在目录作为配置目录
		config.ConfigDir = filepath.Dir(config.CorePath)
	}
	if config.AgentID == "" {
		// 使用主机名作为 Agent ID
		hostname, _ := os.Hostname()
		config.AgentID = hostname
	}

	return &config, nil
}

// 开发者域名 MD5 哈希
const devDomainHash = "9de17c968ada26abec13fc5fc264ddfa"

// isDevDomain 检查面板域名是否为开发者域名
func (a *Agent) isDevDomain() bool {
	if a.config == nil || a.config.PanelURL == "" {
		return false
	}

	// 从 PanelURL 提取域名
	// 格式: https://domain:port 或 wss://domain:port/ws/agent
	panelURL := a.config.PanelURL
	panelURL = strings.TrimPrefix(panelURL, "https://")
	panelURL = strings.TrimPrefix(panelURL, "http://")
	panelURL = strings.TrimPrefix(panelURL, "wss://")
	panelURL = strings.TrimPrefix(panelURL, "ws://")

	// 提取域名部分（去掉端口和路径）
	if idx := strings.Index(panelURL, ":"); idx != -1 {
		panelURL = panelURL[:idx]
	}
	if idx := strings.Index(panelURL, "/"); idx != -1 {
		panelURL = panelURL[:idx]
	}

	if panelURL == "" {
		return false
	}

	// 计算 MD5
	hash := md5.Sum([]byte(panelURL))
	domainHash := hex.EncodeToString(hash[:])

	return domainHash == devDomainHash
}

func (a *Agent) run() {
	for {
		select {
		case <-a.stopChan:
			return
		default:
		}

		if err := a.connect(); err != nil {
			log.Printf("连接失败: %v, 5秒后重试...", err)
			time.Sleep(5 * time.Second)
			continue
		}

		// 注册
		if err := a.register(); err != nil {
			log.Printf("注册失败: %v", err)
			a.disconnect()
			time.Sleep(5 * time.Second)
			continue
		}

		log.Println("已连接到面板")

		// 启动心跳
		go a.heartbeatLoop()

		// 处理消息
		a.messageLoop()

		log.Println("连接断开，5秒后重连...")
		time.Sleep(5 * time.Second)
	}
}

func (a *Agent) connect() error {
	u, err := url.Parse(a.config.PanelURL)
	if err != nil {
		return err
	}

	if u.Scheme == "https" {
		u.Scheme = "wss"
	} else {
		u.Scheme = "ws"
	}
	u.Path = "/api/agent/ws"

	log.Printf("正在连接: %s", u.String())

	dialer := websocket.Dialer{
		HandshakeTimeout: 10 * time.Second,
	}
	conn, _, err := dialer.Dial(u.String(), nil)
	if err != nil {
		return err
	}

	a.connMu.Lock()
	a.conn = conn
	a.connMu.Unlock()

	return nil
}

func (a *Agent) disconnect() {
	a.connMu.Lock()
	defer a.connMu.Unlock()

	if a.conn != nil {
		a.conn.Close()
		a.conn = nil
	}
}

func (a *Agent) sendMessage(msg *Message) error {
	a.connMu.Lock()
	defer a.connMu.Unlock()

	if a.conn == nil {
		return fmt.Errorf("未连接")
	}

	return a.conn.WriteJSON(msg)
}

func (a *Agent) register() error {
	hostname, _ := os.Hostname()
	localIPv4, localIPv6 := getLocalIP()

	data := map[string]interface{}{
		"token":        a.config.Token,
		"agent_id":     a.config.AgentID,
		"version":      Version,
		"hostname":     hostname,
		"local_ip_v4":  localIPv4,
		"local_ip_v6":  localIPv6,
		"os":           runtime.GOOS,
		"arch":         runtime.GOARCH,
	}

	rawData, _ := json.Marshal(data)
	msg := &Message{
		Type:      MsgTypeRegister,
		Timestamp: time.Now().Unix(),
		Data:      rawData,
	}

	return a.sendMessage(msg)
}

func (a *Agent) heartbeatLoop() {
	// 2秒上报一次，用于实时网速显示
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-a.stopChan:
			return
		case <-ticker.C:
			if err := a.sendHeartbeat(); err != nil {
				log.Printf("发送心跳失败: %v", err)
				return
			}
		}
	}
}

func (a *Agent) sendHeartbeat() error {
	// 获取系统信息（需 nil 检查防止容器/WSL 环境 panic）
	cpuPercent, _ := cpu.Percent(0, false)
	memInfo, _ := mem.VirtualMemory()
	diskInfo, _ := disk.Usage("/")

	var cpuPct, memPct, diskPct float64
	if len(cpuPercent) > 0 {
		cpuPct = cpuPercent[0]
	}
	if memInfo != nil {
		memPct = memInfo.UsedPercent
	}
	if diskInfo != nil {
		diskPct = diskInfo.UsedPercent
	}

	// 从 netstatic 获取流量统计（持久化的增量累计）
	var totalBytesRecv, totalBytesSent uint64
	trafficData, err := netstatic.GetTotalTraffic()
	if err == nil {
		for nicName, data := range trafficData {
			if netstatic.ShouldIncludeNic(nicName) {
				totalBytesRecv += data.Rx
				totalBytesSent += data.Tx
			}
		}
	}

	// 获取实时网速（通过采样计算）
	var netInSpeed, netOutSpeed uint64
	if upSpeed, downSpeed, err := netstatic.GetCurrentSpeed(); err == nil {
		netOutSpeed = upSpeed
		netInSpeed = downSpeed
	}

	data := map[string]interface{}{
		"uptime":           int64(time.Since(a.startTime).Seconds()),
		"cpu_percent":      cpuPct,
		"mem_percent":      memPct,
		"disk_percent":     diskPct,
		"net_in":           netInSpeed,     // bytes/s (实时速率)
		"net_out":          netOutSpeed,    // bytes/s (实时速率)
		"net_in_transfer":  totalBytesRecv, // bytes (持久化的累计流量)
		"net_out_transfer": totalBytesSent, // bytes (持久化的累计流量)
		"connections":      0,
	}

	rawData, _ := json.Marshal(data)
	msg := &Message{
		Type:      MsgTypeHeartbeat,
		Timestamp: time.Now().Unix(),
		Data:      rawData,
	}

	return a.sendMessage(msg)
}

func (a *Agent) messageLoop() {
	for {
		select {
		case <-a.stopChan:
			return
		default:
		}

		var msg Message
		if err := a.conn.ReadJSON(&msg); err != nil {
			log.Printf("读取消息失败: %v", err)
			return
		}

		go a.handleMessage(&msg)
	}
}

func (a *Agent) handleMessage(msg *Message) {
	var resp *Message
	var err error

	switch msg.Type {
	case MsgTypeCommand:
		resp, err = a.handleCommand(msg)
	case MsgTypeDeployFile:
		resp, err = a.handleDeployFile(msg)
	case MsgTypeDeployDir:
		resp, err = a.handleDeployDir(msg)
	case MsgTypeSyncConfig:
		// 配置同步为单向通知，不返回响应
		a.handleSyncConfig(msg)
		return
	case MsgTypeRestart:
		resp, err = a.handleRestart(msg)
	case MsgTypeDeployCore:
		resp, err = a.handleDeployCore(msg)
	case MsgTypeSelfUpdate:
		// 自我更新为单向通知，不返回响应，异步执行
		go a.handleSelfUpdate(msg)
		return
	case MsgTypeRegisterResp:
		// 注册响应
		if msg.Error != "" {
			log.Printf("注册失败: %s", msg.Error)
		} else {
			log.Printf("注册成功")
		}
		return
	case MsgTypeCommandResp:
		// 命令响应，由请求方处理
		return
	default:
		log.Printf("未知消息类型: %s", msg.Type)
		return
	}

	if err != nil {
		resp = &Message{
			Type:      MsgTypeCommandResp,
			ID:        msg.ID,
			Timestamp: time.Now().Unix(),
			Error:     err.Error(),
		}
	}

	if resp != nil {
		resp.ID = msg.ID
		if err := a.sendMessage(resp); err != nil {
			log.Printf("发送响应失败: %v", err)
		}
	}
}

// allowedCommands 远程命令白名单 — 仅允许已知安全命令
var allowedCommands = map[string]bool{
	"uname":     true,
	"hostname":  true,
	"uptime":    true,
	"ip":        true,
	"ss":        true,
	"netstat":   true,
	"ps":        true,
	"top":       true,
	"df":        true,
	"free":      true,
	"du":        true,
	"sing-box":  true,
	"journalctl": true,
	"tail":      true,
	"cat":       true,
	"ls":        true,
}

// fileReadingCommands 可以读取文件的命令，需额外校验 args 路径在安全范围内
var fileReadingCommands = map[string]bool{
	"cat":        true,
	"tail":       true,
	"ls":         true,
	"journalctl": true, // journalctl 有 -D/--directory 可指定日志目录
}

// safeReadDirs 文件读取类命令允许访问的目录
var safeReadDirs = []string{"/opt/sboard", "/var/log", "/tmp", "/etc/sing-box", "/etc/systemd/system"}

func (a *Agent) handleCommand(msg *Message) (*Message, error) {
	var data struct {
		Command string   `json:"command"`
		Args    []string `json:"args"`
		Timeout int      `json:"timeout"`
		WorkDir string   `json:"work_dir"`
	}
	if err := json.Unmarshal(msg.Data, &data); err != nil {
		return nil, err
	}

	// 白名单校验：仅允许已知安全命令
	if !allowedCommands[data.Command] {
		errData, _ := json.Marshal(map[string]interface{}{
			"exit_code": 1,
			"stdout":    "",
			"stderr":    fmt.Sprintf("command not allowed: %s", data.Command),
			"duration":  0,
		})
		return &Message{
			Type:      MsgTypeCommandResp,
			Timestamp: time.Now().Unix(),
			Data:      errData,
		}, nil
	}

	// 文件读取类命令的 args 路径校验
	if fileReadingCommands[data.Command] {
		for _, arg := range data.Args {
			// 跳过不以 / 或 - 开头的参数（非路径）
			if !strings.HasPrefix(arg, "/") && !strings.HasPrefix(arg, "-") {
				continue
			}
			// 跳过纯 flag（如 -n, --help）
			if strings.HasPrefix(arg, "-") {
				continue
			}
			// 检查路径是否在安全目录范围内
			safe, err := safeResolvePath(arg, safeReadDirs)
			if err != nil || safe == "" {
				errData, _ := json.Marshal(map[string]interface{}{
					"exit_code": 1,
					"stdout":    "",
					"stderr":    fmt.Sprintf("路径不在允许范围内: %s", arg),
					"duration":  0,
				})
				return &Message{
					Type:      MsgTypeCommandResp,
					Timestamp: time.Now().Unix(),
					Data:      errData,
				}, nil
			}
		}
	}

	timeout := time.Duration(data.Timeout) * time.Second
	if timeout <= 0 {
		timeout = 60 * time.Second
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	start := time.Now()
	cmd := exec.CommandContext(ctx, data.Command, data.Args...)
	if data.WorkDir != "" {
		cmd.Dir = data.WorkDir
	}

	// 分别捕获 stdout 和 stderr
	var stdoutBuf, stderrBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf

	err := cmd.Run()
	duration := time.Since(start).Milliseconds()

	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			// 超时或其他错误
			exitCode = -1
		}
	}

	respData := map[string]interface{}{
		"exit_code": exitCode,
		"stdout":    stdoutBuf.String(),
		"stderr":    stderrBuf.String(),
		"duration":  duration,
	}

	rawData, _ := json.Marshal(respData)
	return &Message{
		Type:      MsgTypeCommandResp,
		Timestamp: time.Now().Unix(),
		Data:      rawData,
	}, nil
}

func (a *Agent) handleDeployFile(msg *Message) (*Message, error) {
	var data struct {
		Path       string `json:"path"`
		Content    string `json:"content"`
		Mode       int    `json:"mode"`
		Backup     bool   `json:"backup"`
		RestartSvc string `json:"restart_svc"`
	}
	if err := json.Unmarshal(msg.Data, &data); err != nil {
		return nil, err
	}

	// 路径穿越防护：解析为绝对路径并限定在允许的目录内
	allowedDirs := []string{
		a.config.ConfigDir,
		a.config.CorePath,
		filepath.Dir(a.config.CorePath),
		"/opt/sboard", // 默认安装根路径
	}
	safePath, err := safeResolvePath(data.Path, allowedDirs)
	if err != nil {
		return nil, fmt.Errorf("路径不安全: %v", err)
	}

	// 解码内容
	content, err := base64.StdEncoding.DecodeString(data.Content)
	if err != nil {
		return nil, fmt.Errorf("解码内容失败: %v", err)
	}

	// 备份
	if data.Backup {
		if _, err := os.Stat(safePath); err == nil {
			backupPath := safePath + ".bak." + time.Now().Format("20060102150405")
			os.Rename(safePath, backupPath)
		}
	}

	// 确保目录存在
	dir := filepath.Dir(safePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("创建目录失败: %v", err)
	}

	// 写入文件
	mode := os.FileMode(0644)
	if data.Mode > 0 {
		mode = os.FileMode(data.Mode)
	}
	if err := os.WriteFile(safePath, content, mode); err != nil {
		return nil, fmt.Errorf("写入文件失败: %v", err)
	}

	log.Printf("已部署文件: %s", safePath)

	// 重启服务
	if data.RestartSvc != "" {
		serviceManager.RestartService(data.RestartSvc)
	}

	respData := map[string]interface{}{
		"success": true,
		"path":    safePath,
	}
	rawData, _ := json.Marshal(respData)
	return &Message{
		Type:      MsgTypeCommandResp,
		Timestamp: time.Now().Unix(),
		Data:      rawData,
	}, nil
}

// safeResolvePath 将路径解析为绝对路径，并校验其在允许的目录范围内
func safeResolvePath(target string, allowedDirs []string) (string, error) {
	if target == "" {
		return "", fmt.Errorf("路径为空")
	}

	abs, err := filepath.Abs(target)
	if err != nil {
		return "", fmt.Errorf("无法解析路径: %v", err)
	}

	// 解析符号链接后再次确保是绝对路径
	resolved, err := filepath.EvalSymlinks(abs)
	if err != nil {
		// 目标可能尚不存在，使用 Abs 的结果
		resolved = abs
	}

	// 确保 resolved 仍为绝对路径
	if !filepath.IsAbs(resolved) {
		return "", fmt.Errorf("解析后不是绝对路径: %s", resolved)
	}

	// 检查是否在允许的目录范围内
	for _, base := range allowedDirs {
		if base == "" {
			continue
		}
		baseAbs, err := filepath.Abs(base)
		if err != nil {
			continue
		}
		// 确保 resolved 以 baseAbs 开头（用分隔符防止 /opt/sboard2 匹配 /opt/sboard）
		if strings.HasPrefix(resolved, baseAbs+string(filepath.Separator)) || resolved == baseAbs {
			return resolved, nil
		}
	}

	return "", fmt.Errorf("路径 %s 不在允许的目录范围内", resolved)
}

func (a *Agent) handleDeployDir(msg *Message) (*Message, error) {
	var data struct {
		Path   string            `json:"path"`
		Files  map[string]string `json:"files"`
		Modes  map[string]int    `json:"modes"`
		Clean  bool              `json:"clean"`
		Backup bool              `json:"backup"`
	}
	if err := json.Unmarshal(msg.Data, &data); err != nil {
		return nil, err
	}

	// 路径穿越防护
	allowedDirs := []string{
		a.config.ConfigDir,
		a.config.CorePath,
		filepath.Dir(a.config.CorePath),
		"/opt/sboard",
	}
	safePath, err := safeResolvePath(data.Path, allowedDirs)
	if err != nil {
		return nil, fmt.Errorf("路径不安全: %v", err)
	}

	// 备份
	if data.Backup {
		if _, err := os.Stat(safePath); err == nil {
			backupPath := safePath + ".bak." + time.Now().Format("20060102150405")
			os.Rename(safePath, backupPath)
		}
	}

	// 清空目录
	if data.Clean {
		os.RemoveAll(safePath)
	}

	// 创建目录
	if err := os.MkdirAll(safePath, 0755); err != nil {
		return nil, fmt.Errorf("创建目录失败: %v", err)
	}

	// 写入文件
	for filename, contentB64 := range data.Files {
		content, err := base64.StdEncoding.DecodeString(contentB64)
		if err != nil {
			continue
		}

		filePath := filepath.Join(safePath, filename)

		// 确保子目录存在
		if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
			continue
		}

		mode := os.FileMode(0644)
		if m, ok := data.Modes[filename]; ok {
			mode = os.FileMode(m)
		}

		os.WriteFile(filePath, content, mode)
	}

	log.Printf("已部署目录: %s (%d 个文件)", safePath, len(data.Files))

	respData := map[string]interface{}{
		"success": true,
		"path":    safePath,
		"count":   len(data.Files),
	}
	rawData, _ := json.Marshal(respData)
	return &Message{
		Type:      MsgTypeCommandResp,
		Timestamp: time.Now().Unix(),
		Data:      rawData,
	}, nil
}

func (a *Agent) handleSyncConfig(msg *Message) {
	var data struct {
		ConfigType string `json:"config_type"`
		Content    string `json:"content"`
		Restart    bool   `json:"restart"`
		TargetPath string `json:"target_path"` // 目标路径，如 /opt/sboard/sing-box
	}
	if err := json.Unmarshal(msg.Data, &data); err != nil {
		log.Printf("解析配置同步消息失败: %v", err)
		return
	}

	// 确定配置路径：优先使用面板传入的 TargetPath，回退到本地 ConfigDir
	var configPath string
	var serviceName string
	configDir := a.config.ConfigDir
	if data.TargetPath != "" {
		configDir = data.TargetPath
	}

	// 路径穿越防护：限制写入在允许的目录范围
	allowedDirs := []string{a.config.ConfigDir, a.config.CorePath, filepath.Dir(a.config.CorePath), "/opt/sboard"}
	safeDir, err := safeResolvePath(configDir, allowedDirs)
	if err != nil {
		log.Printf("配置目录不安全: %v", err)
		return
	}
	coreType := strings.ToLower(data.ConfigType)
	switch coreType {
	case "sing-box":
		configPath = filepath.Join(safeDir, "config.json")
		serviceName = "sing-box"
	default:
		log.Printf("未知配置类型: %s", data.ConfigType)
		return
	}

	// 备份旧配置（带时间戳，避免覆盖之前的备份）
	if _, err := os.Stat(configPath); err == nil {
		backupPath := configPath + ".bak." + time.Now().Format("20060102150405")
		os.Rename(configPath, backupPath)
	}

	// 写入新配置
	if err := os.MkdirAll(filepath.Dir(configPath), 0755); err != nil {
		log.Printf("创建配置目录失败: %v", err)
		return
	}
	if err := os.WriteFile(configPath, []byte(data.Content), 0600); err != nil {
		log.Printf("写入配置文件失败: %v", err)
		return
	}

	// 计算配置版本
	hash := md5.Sum([]byte(data.Content))
	configVersion := hex.EncodeToString(hash[:])

	log.Printf("配置已同步: %s (版本: %s)", configPath, configVersion[:8])

	// 重启服务 (使用平台特定的服务管理器)
	if data.Restart {
		if err := serviceManager.RestartService(serviceName); err != nil {
			log.Printf("重启服务失败: %v", err)
		} else {
			log.Printf("服务已重启: %s", serviceName)
		}
	}
}

func (a *Agent) handleRestart(msg *Message) (*Message, error) {
	var data struct {
		Service string `json:"service"`
	}
	if err := json.Unmarshal(msg.Data, &data); err != nil {
		return nil, err
	}

	serviceName := data.Service
	if serviceName == "" {
		serviceName = "sing-box"
	}

	// 使用平台特定的服务管理器
	if err := serviceManager.RestartService(serviceName); err != nil {
		return nil, fmt.Errorf("重启失败: %v", err)
	}

	log.Printf("服务已重启: %s", serviceName)

	respData := map[string]interface{}{
		"success": true,
		"service": serviceName,
	}
	rawData, _ := json.Marshal(respData)
	return &Message{
		Type:      MsgTypeCommandResp,
		Timestamp: time.Now().Unix(),
		Data:      rawData,
	}, nil
}

// getLocalIP 并发获取本机公网 IPv4 和 IPv6
func getLocalIP() (ipv4, ipv6 string) {
	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		ipv4 = getPublicIPFromAPI()
		if ipv4 != "" {
			log.Printf("[IP] 成功获取公网 IPv4: %s", ipv4)
		} else {
			log.Printf("[IP] 无法获取公网 IPv4")
		}
	}()

	go func() {
		defer wg.Done()
		ipv6 = getPublicIPv6FromAPI()
		if ipv6 != "" {
			log.Printf("[IP] 成功获取公网 IPv6: %s", ipv6)
		} else {
			log.Printf("[IP] 无法获取公网 IPv6")
		}
	}()

	wg.Wait()
	return
}

// isPrivateIP 判断是否为内网 IP
func isPrivateIP(ipStr string) bool {
	ip := net.ParseIP(ipStr)
	if ip == nil {
		return false
	}

	// 检查私有地址段
	privateBlocks := []string{
		"10.0.0.0/8",
		"172.16.0.0/12",
		"192.168.0.0/16",
		"100.64.0.0/10",  // CGNAT
		"169.254.0.0/16", // Link-local
	}

	for _, block := range privateBlocks {
		_, cidr, _ := net.ParseCIDR(block)
		if cidr.Contains(ip) {
			return true
		}
	}
	return false
}

// getPublicIPFromAPI 通过外部 API 获取公网 IPv4
func getPublicIPFromAPI() string {
	apis := []string{
		"https://ip125.com/api/myip",     // 国内，JSON {ip:"x.x.x.x"}，响应快
		"https://ipinfo.io/json",          // 国际，JSON {ip:"x.x.x.x"}，准确
		"https://checkip.amazonaws.com",   // AWS 官方，纯 IP
		"https://api.ipify.org",           // 国际流行，纯 IP
	}

	ipv4Regex := regexp.MustCompile(`(?m)(25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)\.(25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)\.(25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)\.(25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)`)

	for _, apiURL := range apis {
		log.Printf("[IP] 尝试 API: %s", apiURL)

		resp, err := directClient.Get(apiURL)
		if err != nil {
			log.Printf("[IP] API %s 失败: %v", apiURL, err)
			continue
		}

		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			log.Printf("[IP] 读取响应失败: %v", err)
			continue
		}

		// 使用正则从响应中提取第一个合法的 IPv4
		match := ipv4Regex.FindString(string(body))
		if match == "" {
			log.Printf("[IP] 响应中未找到 IPv4 地址: %s", string(body))
			continue
		}
		ip := match

		// 验证 IP 格式
		parsedIP := net.ParseIP(ip)
		if parsedIP == nil {
			log.Printf("[IP] 无效 IP 格式: %s", ip)
			continue
		}

		// 只接受 IPv4
		if parsedIP.To4() == nil {
			log.Printf("[IP] 返回的是 IPv6: %s，跳过（只需要 IPv4）", ip)
			continue
		}

		// 检查是否为内网 IP
		if isPrivateIP(ip) {
			log.Printf("[IP] 返回的是内网 IPv4: %s，跳过", ip)
			continue
		}

		// 找到有效的 IPv4 公网 IP
		log.Printf("[IP] API %s 返回 IPv4 公网 IP: %s", apiURL, ip)
		return ip
	}

	// 所有 API 都失败了
	log.Printf("[IP] 所有 API 都失败，无法获取 IPv4 公网 IP")
	return ""
}

// getPublicIPv6FromAPI 通过外部 API 获取公网 IPv6
func getPublicIPv6FromAPI() string {
	apis := []string{
		"https://api6.ipify.org",      // 国际，纯 IPv6
		"https://ifconfig.co",         // 通用 API，支持 IPv6
	}

	ipv6Regex := regexp.MustCompile(`(?i)(?:[0-9a-f]{1,4}:){7}[0-9a-f]{1,4}|(?:[0-9a-f]{1,4}:){1,7}:|(?:[0-9a-f]{1,4}:){1,6}:[0-9a-f]{1,4}|(?:[0-9a-f]{1,4}:){1,5}(?::[0-9a-f]{1,4}){1,2}|(?:[0-9a-f]{1,4}:){1,4}(?::[0-9a-f]{1,4}){1,3}|(?:[0-9a-f]{1,4}:){1,3}(?::[0-9a-f]{1,4}){1,4}|(?:[0-9a-f]{1,4}:){1,2}(?::[0-9a-f]{1,4}){1,5}|[0-9a-f]{1,4}:(?:(?::[0-9a-f]{1,4}){1,6})|:(?:(?::[0-9a-f]{1,4}){1,7}|:)`)

	for _, apiURL := range apis {
		log.Printf("[IP] 尝试 IPv6 API: %s", apiURL)

		resp, err := directClient.Get(apiURL)
		if err != nil {
			log.Printf("[IP] IPv6 API %s 失败: %v", apiURL, err)
			continue
		}

		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			log.Printf("[IP] 读取响应失败: %v", err)
			continue
		}

		// 使用正则提取 IPv6
		match := ipv6Regex.FindString(string(body))
		if match == "" {
			log.Printf("[IP] 响应中未找到 IPv6 地址: %s", string(body))
			continue
		}
		ip := match

		// 验证 IP 格式
		parsedIP := net.ParseIP(ip)
		if parsedIP == nil {
			log.Printf("[IP] 无效 IP 格式: %s", ip)
			continue
		}

		// 只接受 IPv6（To4() 为 nil 说明不是 IPv4）
		if parsedIP.To4() != nil {
			log.Printf("[IP] 返回的是 IPv4: %s，跳过（只需要 IPv6）", ip)
			continue
		}

		// 检查是否为内网 IPv6（ULA 或 Link-local）
		if isPrivateIPv6(ip) {
			log.Printf("[IP] 返回的是内网 IPv6: %s，跳过", ip)
			continue
		}

		// 找到有效的 IPv6 公网 IP
		log.Printf("[IP] IPv6 API %s 返回公网 IPv6: %s", apiURL, ip)
		return ip
	}

	// 所有 API 都失败了
	log.Printf("[IP] 所有 IPv6 API 都失败，无法获取公网 IPv6")
	return ""
}

// isPrivateIPv6 判断是否为内网 IPv6（ULA fc00::/7 或 Link-local fe80::/10）
func isPrivateIPv6(ipStr string) bool {
	ip := net.ParseIP(ipStr)
	if ip == nil {
		return false
	}

	// 如果是 IPv4 映射地址，直接返回 true（不属于公网 IPv6）
	if ip.To4() != nil {
		return true
	}

	privateBlocks := []string{
		"fc00::/7",   // Unique Local Address (ULA)
		"fe80::/10",  // Link-local
		"::1/128",    // Loopback
	}

	for _, block := range privateBlocks {
		_, cidr, _ := net.ParseCIDR(block)
		if cidr.Contains(ip) {
			return true
		}
	}
	return false
}

// handleDeployCore 处理核心部署指令
func (a *Agent) handleDeployCore(msg *Message) (*Message, error) {
	var data struct {
		CoreType   string `json:"core_type"`   // sing-box
		TargetPath string `json:"target_path"` // 目标安装路径
		Config     string `json:"config"`      // 配置文件内容
	}
	if err := json.Unmarshal(msg.Data, &data); err != nil {
		return nil, fmt.Errorf("解析部署参数失败: %v", err)
	}

	// 如果未指定目标路径，使用平台默认路径
	targetPath := data.TargetPath
	if targetPath == "" {
		targetPath = serviceManager.GetDefaultInstallPath(data.CoreType)
	}

	// 路径穿越防护
	allowedDirs := []string{a.config.ConfigDir, a.config.CorePath, filepath.Dir(a.config.CorePath), "/opt/sboard"}
	var safePath string
	safePath, err1 := safeResolvePath(targetPath, allowedDirs)
	if err1 != nil {
		return nil, fmt.Errorf("目标路径不安全: %v", err1)
	}

	log.Printf("开始部署核心: %s -> %s (平台: %s/%s)", data.CoreType, safePath, runtime.GOOS, runtime.GOARCH)

	// 1. 确定嵌入资源路径和文件名
	var embedPath string
	serviceName := data.CoreType
	binaryName := serviceManager.GetBinaryName(data.CoreType)
	configFileName := serviceManager.GetConfigFileName(data.CoreType)

	switch data.CoreType {
	case "sing-box":
		embedPath = "embed/configs/sing-box"
	default:
		return nil, fmt.Errorf("不支持的核心类型: %s", data.CoreType)
	}

	// 2. 创建目标目录
	if err := os.MkdirAll(safePath, 0755); err != nil {
		return nil, fmt.Errorf("创建目标目录失败: %v", err)
	}

	// 3. 检查二进制文件是否存在（已有或需要从嵌入资源复制）
	binaryPath := filepath.Join(safePath, binaryName)
	binaryExists := false
	if _, err := os.Stat(binaryPath); err == nil {
		binaryExists = true
		log.Printf("  发现现有二进制: %s", binaryPath)
	}

	// 4. 从嵌入资源复制文件到目标目录（证书等辅助文件）
	embeddedBinaryFound := false
	err := fs.WalkDir(embeddedConfigs, embedPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// 计算相对路径
		relPath, _ := filepath.Rel(embedPath, path)

		// 根据文件名决定操作
		switch relPath {
		case ".":
			return nil
		case configFileName, "config.json", "config.yaml":
			// 跳过配置文件（后面会用传入的配置覆盖）
			return nil
		case "deploy.sh":
			// 跳过 deploy.sh（不再需要）
			return nil
		}

		// 跳过 .service 文件（Linux 特有，由 ServiceManager 处理）
		if strings.HasSuffix(relPath, ".service") {
			return nil
		}

		// 处理二进制文件名（嵌入的是对应平台的，需要重命名）
		var targetFileName string
		isBinary := false
		switch relPath {
		case "sing-box", "sing-box.exe":
			targetFileName = binaryName
			isBinary = true
			embeddedBinaryFound = true
		default:
			targetFileName = relPath
		}

		targetFile := filepath.Join(safePath, targetFileName)

		if d.IsDir() {
			return os.MkdirAll(targetFile, 0755)
		}

		// 读取嵌入文件
		content, err := embeddedConfigs.ReadFile(path)
		if err != nil {
			return fmt.Errorf("读取嵌入文件失败 %s: %v", path, err)
		}

		// 确定文件权限
		mode := os.FileMode(0644)
		// 二进制文件需要可执行权限 (Windows 会忽略)
		if isBinary {
			mode = 0755
		}

		// 写入目标文件，二进制用临时文件+rename原子替换，避免text file busy
		if isBinary {
			tmpFile := targetFile + ".new"
			if err := os.WriteFile(tmpFile, content, mode); err != nil {
				return fmt.Errorf("写入临时文件失败 %s: %v", tmpFile, err)
			}
			if err := os.Rename(tmpFile, targetFile); err != nil {
				return fmt.Errorf("原子替换文件失败 %s: %v", targetFile, err)
			}
		} else {
			if err := os.WriteFile(targetFile, content, mode); err != nil {
				return fmt.Errorf("写入文件失败 %s: %v", targetFile, err)
			}
		}
		log.Printf("  复制文件: %s -> %s", relPath, targetFileName)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("复制核心文件失败: %v", err)
	}

	// 6. 检查二进制是否就绪
	if !binaryExists && !embeddedBinaryFound {
		return nil, fmt.Errorf("未找到 %s 二进制文件，请确保 Agent 编译时已嵌入对应平台的核心", data.CoreType)
	}

	// 7. 用指令中的配置内容覆盖配置文件
	if data.Config != "" {
		configPath := filepath.Join(safePath, configFileName)
		if err := os.WriteFile(configPath, []byte(data.Config), 0600); err != nil {
			return nil, fmt.Errorf("写入配置文件失败: %v", err)
		}
		log.Printf("  更新配置文件: %s", configPath)
	}

	// 5. 安装并启动服务 (使用平台特定的 ServiceManager)
	log.Printf("  安装服务: %s", serviceName)
	if err := serviceManager.InstallService(serviceName, safePath, data.CoreType); err != nil {
		return nil, fmt.Errorf("安装服务失败: %v", err)
	}

	// 6. 启动或重启服务
	if serviceManager.IsServiceRunning(serviceName) {
		log.Printf("  服务 %s 正在运行，执行重启...", serviceName)
		if err := serviceManager.RestartService(serviceName); err != nil {
			log.Printf("  重启服务失败: %v", err)
		} else {
			log.Printf("  服务 %s 已重启", serviceName)
		}
	} else {
		log.Printf("  启动服务: %s", serviceName)
		if err := serviceManager.StartService(serviceName); err != nil {
			log.Printf("  启动服务失败: %v", err)
		} else {
			log.Printf("  服务 %s 已启动", serviceName)
		}
	}

	// 7. 检查服务状态
	currentRunning := serviceManager.IsServiceRunning(serviceName)

	log.Printf("部署完成! %s: %v", serviceName, currentRunning)

	// 构建响应
	respData := map[string]interface{}{
		"success":      true,
		"core_type":    data.CoreType,
		"target_path":  safePath,
		"service_name": serviceName,
		"running":      currentRunning,
		"platform":     runtime.GOOS + "/" + runtime.GOARCH,
	}
	rawData, _ := json.Marshal(respData)
	return &Message{
		Type:      MsgTypeCommandResp,
		Timestamp: time.Now().Unix(),
		Data:      rawData,
	}, nil
}

// handleSelfUpdate 处理自我更新指令（单向通知，不返回响应）
func (a *Agent) handleSelfUpdate(_ *Message) {
	// 延迟 2 秒执行，等待部署核心指令完成
	time.Sleep(2 * time.Second)

	log.Printf("开始自我更新 (平台: %s/%s)", runtime.GOOS, runtime.GOARCH)

	// 确定下载 URL
	// 格式: https://github.com/amuae/sboard/releases/latest/download/sboard-agent_{os}_{arch}.zip
	osName := runtime.GOOS
	archName := runtime.GOARCH

	// 架构名称映射
	switch archName {
	case "amd64":
		archName = "amd64"
	case "arm64":
		archName = "arm64"
	case "386":
		archName = "386"
	case "arm":
		// 检查是否为 armv7
		// Go 在 armv7 设备上报告为 "arm"，需要检查 GOARM
		archName = "armv7" // 默认使用 armv7，兼容性更好
	}

	// 文件名格式：sboard-agent_{os}_{arch}.zip
	zipFileName := fmt.Sprintf("sboard-agent_%s_%s.zip", osName, archName)

	// 检查是否使用预发布版本（根据面板域名判断）
	releaseType := "latest/download"
	if a.isDevDomain() {
		releaseType = "download/pre-release"
		log.Printf("开发者模式：使用预发布版本")
	}

	// 下载 URL（使用 ghfast.top 加速域名）
	downloadURL := fmt.Sprintf("https://ghfast.top/https://github.com/amuae/sboard/releases/%s/%s", releaseType, zipFileName)

	log.Printf("下载地址: %s", downloadURL)

	// 获取当前可执行文件路径
	execPath, err := os.Executable()
	if err != nil {
		log.Printf("获取可执行文件路径失败: %v", err)
		return
	}
	execPath, err = filepath.EvalSymlinks(execPath)
	if err != nil {
		log.Printf("解析符号链接失败: %v", err)
		return
	}

	// 下载 zip 文件到临时路径
	tempZipPath := execPath + ".zip"
	if err := downloadFile(downloadURL, tempZipPath); err != nil {
		log.Printf("下载新版本失败: %v", err)
		os.Remove(tempZipPath)
		return
	}
	defer os.Remove(tempZipPath) // 确保清理 zip 文件

	// 从 zip 中提取可执行文件
	binaryName := "sboard-agent"
	if runtime.GOOS == "windows" {
		binaryName = "sboard-agent.exe"
	}

	tempPath := execPath + ".new"
	if err := extractFromZip(tempZipPath, binaryName, tempPath); err != nil {
		log.Printf("解压新版本失败: %v", err)
		os.Remove(tempPath)
		return
	}

	// 设置可执行权限
	if runtime.GOOS != "windows" {
		if err := os.Chmod(tempPath, 0755); err != nil {
			log.Printf("设置权限失败: %v", err)
			os.Remove(tempPath)
			return
		}
	}

	// 备份旧版本
	backupPath := execPath + ".bak"
	os.Remove(backupPath) // 删除旧备份
	if err := os.Rename(execPath, backupPath); err != nil {
		log.Printf("备份旧版本失败: %v", err)
		os.Remove(tempPath)
		return
	}

	// 替换为新版本
	if err := os.Rename(tempPath, execPath); err != nil {
		log.Printf("替换新版本失败: %v", err)
		// 恢复旧版本
		os.Rename(backupPath, execPath)
		return
	}

	log.Printf("自我更新成功，正在重启服务...")

	// 重启 agent 服务
	if err := serviceManager.RestartService("sboard-agent"); err != nil {
		log.Printf("重启服务失败: %v，尝试直接退出", err)
		// 如果服务管理器重启失败，直接退出让 systemd 重启
		os.Exit(0)
	}
}

// extractFromZip 从 zip 文件中提取指定文件
func extractFromZip(zipPath, fileName, destPath string) error {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return fmt.Errorf("打开 zip 文件失败: %v", err)
	}
	defer r.Close()

	for _, f := range r.File {
		if f.Name == fileName || filepath.Base(f.Name) == fileName {
			rc, err := f.Open()
			if err != nil {
				return fmt.Errorf("打开 zip 内文件失败: %v", err)
			}
			defer rc.Close()

			out, err := os.Create(destPath)
			if err != nil {
				return fmt.Errorf("创建目标文件失败: %v", err)
			}
			defer out.Close()

			written, err := io.Copy(out, rc)
			if err != nil {
				return fmt.Errorf("写入文件失败: %v", err)
			}

			log.Printf("解压文件: %s (%d bytes)", fileName, written)
			return nil
		}
	}

	return fmt.Errorf("zip 中未找到文件: %s", fileName)
}

// downloadFile 下载文件
func downloadFile(url, destPath string) error {
	// 创建 HTTP 客户端，设置超时
	client := &http.Client{
		Timeout: 5 * time.Minute,
	}

	resp, err := client.Get(url)
	if err != nil {
		return fmt.Errorf("HTTP 请求失败: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP 状态码: %d", resp.StatusCode)
	}

	// 创建目标文件
	out, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("创建文件失败: %v", err)
	}
	defer out.Close()

	// 写入文件
	written, err := io.Copy(out, resp.Body)
	if err != nil {
		return fmt.Errorf("写入文件失败: %v", err)
	}

	log.Printf("下载完成: %d bytes", written)
	return nil
}
