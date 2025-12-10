package main

import (
	"archive/zip"
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
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/gorilla/websocket"
	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/mem"
	psnet "github.com/shirou/gopsutil/v3/net"
)

//go:embed embed/configs/*
var embeddedConfigs embed.FS

var (
	Version        = "1.0.0"
	serviceManager = NewServiceManager() // 平台特定的服务管理器
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
	config      *Config
	conn        *websocket.Conn
	connMu      sync.Mutex
	startTime   time.Time
	lastNetIn   uint64
	lastNetOut  uint64
	lastNetTime time.Time // 上次网络采集时间
	stopChan    chan struct{}
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

	// 转换为 WebSocket URL
	if u.Scheme == "https" {
		u.Scheme = "wss"
	} else {
		u.Scheme = "ws"
	}
	u.Path = "/api/agent/ws"

	log.Printf("正在连接: %s", u.String())

	conn, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
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
	localIP := getLocalIP()

	data := map[string]interface{}{
		"token":    a.config.Token,
		"agent_id": a.config.AgentID,
		"version":  Version,
		"hostname": hostname,
		"local_ip": localIP,
		"os":       runtime.GOOS,
		"arch":     runtime.GOARCH,
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
	// 获取系统信息
	cpuPercent, _ := cpu.Percent(0, false)
	memInfo, _ := mem.VirtualMemory()
	diskInfo, _ := disk.Usage("/")

	// 获取主网卡的流量（排除虚拟网卡、loopback、TUN 等）
	netIO, _ := psnet.IOCounters(true) // true 表示按网卡分开统计

	var cpuPct float64
	if len(cpuPercent) > 0 {
		cpuPct = cpuPercent[0]
	}

	// 找到主网卡并计算网速
	var netInSpeed, netOutSpeed uint64
	var totalBytesRecv, totalBytesSent uint64

	for _, io := range netIO {
		// 跳过虚拟网卡、loopback、TUN/TAP 等
		name := io.Name
		if name == "lo" ||
			strings.HasPrefix(name, "docker") ||
			strings.HasPrefix(name, "br-") ||
			strings.HasPrefix(name, "veth") ||
			strings.HasPrefix(name, "virbr") ||
			strings.HasPrefix(name, "tun") ||
			strings.HasPrefix(name, "tap") ||
			strings.HasPrefix(name, "utun") || // macOS TUN
			strings.HasPrefix(name, "eth-iop") || // sing-box TUN
			strings.HasPrefix(name, "wg") || // WireGuard
			strings.Contains(name, "tun") || // 其他 TUN 设备
			strings.Contains(name, "sing") { // sing-box 相关
			continue
		}
		totalBytesRecv += io.BytesRecv
		totalBytesSent += io.BytesSent
	}

	now := time.Now()
	if !a.lastNetTime.IsZero() {
		elapsed := now.Sub(a.lastNetTime).Seconds()
		if elapsed > 0 && totalBytesRecv >= a.lastNetIn && totalBytesSent >= a.lastNetOut {
			netInSpeed = uint64(float64(totalBytesRecv-a.lastNetIn) / elapsed)
			netOutSpeed = uint64(float64(totalBytesSent-a.lastNetOut) / elapsed)
		}
	}
	a.lastNetIn = totalBytesRecv
	a.lastNetOut = totalBytesSent
	a.lastNetTime = now

	data := map[string]interface{}{
		"uptime":           int64(time.Since(a.startTime).Seconds()),
		"cpu_percent":      cpuPct,
		"mem_percent":      memInfo.UsedPercent,
		"disk_percent":     diskInfo.UsedPercent,
		"net_in":           netInSpeed,     // bytes/s (实时速率)
		"net_out":          netOutSpeed,    // bytes/s (实时速率)
		"net_in_transfer":  totalBytesRecv, // bytes (系统启动以来的总流量)
		"net_out_transfer": totalBytesSent, // bytes (系统启动以来的总流量)
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

	timeout := time.Duration(data.Timeout) * time.Second
	if timeout == 0 {
		timeout = 60 * time.Second
	}

	start := time.Now()
	cmd := exec.Command(data.Command, data.Args...)
	if data.WorkDir != "" {
		cmd.Dir = data.WorkDir
	}

	stdout, _ := cmd.Output()
	duration := time.Since(start).Milliseconds()

	exitCode := 0
	if cmd.ProcessState != nil {
		exitCode = cmd.ProcessState.ExitCode()
	}

	respData := map[string]interface{}{
		"exit_code": exitCode,
		"stdout":    string(stdout),
		"stderr":    "",
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

	// 解码内容
	content, err := base64.StdEncoding.DecodeString(data.Content)
	if err != nil {
		return nil, fmt.Errorf("解码内容失败: %v", err)
	}

	// 备份
	if data.Backup {
		if _, err := os.Stat(data.Path); err == nil {
			backupPath := data.Path + ".bak." + time.Now().Format("20060102150405")
			os.Rename(data.Path, backupPath)
		}
	}

	// 确保目录存在
	dir := filepath.Dir(data.Path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("创建目录失败: %v", err)
	}

	// 写入文件
	mode := os.FileMode(0644)
	if data.Mode > 0 {
		mode = os.FileMode(data.Mode)
	}
	if err := os.WriteFile(data.Path, content, mode); err != nil {
		return nil, fmt.Errorf("写入文件失败: %v", err)
	}

	log.Printf("已部署文件: %s", data.Path)

	// 重启服务 (使用平台特定的服务管理器)
	if data.RestartSvc != "" {
		serviceManager.RestartService(data.RestartSvc)
	}

	respData := map[string]interface{}{
		"success": true,
		"path":    data.Path,
	}
	rawData, _ := json.Marshal(respData)
	return &Message{
		Type:      MsgTypeCommandResp,
		Timestamp: time.Now().Unix(),
		Data:      rawData,
	}, nil
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

	// 备份
	if data.Backup {
		if _, err := os.Stat(data.Path); err == nil {
			backupPath := data.Path + ".bak." + time.Now().Format("20060102150405")
			os.Rename(data.Path, backupPath)
		}
	}

	// 清空目录
	if data.Clean {
		os.RemoveAll(data.Path)
	}

	// 创建目录
	if err := os.MkdirAll(data.Path, 0755); err != nil {
		return nil, fmt.Errorf("创建目录失败: %v", err)
	}

	// 写入文件
	for filename, contentB64 := range data.Files {
		content, err := base64.StdEncoding.DecodeString(contentB64)
		if err != nil {
			continue
		}

		filePath := filepath.Join(data.Path, filename)

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

	log.Printf("已部署目录: %s (%d 个文件)", data.Path, len(data.Files))

	respData := map[string]interface{}{
		"success": true,
		"path":    data.Path,
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
	}
	if err := json.Unmarshal(msg.Data, &data); err != nil {
		log.Printf("解析配置同步消息失败: %v", err)
		return
	}

	// 确定配置路径
	var configPath string
	var serviceName string
	coreType := strings.ToLower(data.ConfigType)
	switch coreType {
	case "sing-box":
		configPath = filepath.Join(a.config.ConfigDir, "config.json")
		serviceName = "sing-box"
	default:
		log.Printf("未知配置类型: %s", data.ConfigType)
		return
	}

	// 备份旧配置
	if _, err := os.Stat(configPath); err == nil {
		backupPath := configPath + ".bak"
		os.Rename(configPath, backupPath)
	}

	// 写入新配置
	if err := os.MkdirAll(filepath.Dir(configPath), 0755); err != nil {
		log.Printf("创建配置目录失败: %v", err)
		return
	}
	if err := os.WriteFile(configPath, []byte(data.Content), 0644); err != nil {
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

// getLocalIP 获取本机公网 IP，优先使用网卡 IP，如果是内网则通过外部 API 获取
func getLocalIP() string {
	// 主方案：从网卡获取 IP
	localIP := getNetworkIP()
	if localIP != "" && !isPrivateIP(localIP) {
		// 网卡获取的是公网 IP，直接使用
		return localIP
	}

	// 备选方案：网卡 IP 是内网，通过外部 API 获取公网 IP
	publicIP := getPublicIPFromAPI()
	if publicIP != "" {
		return publicIP
	}

	// 都失败了，返回网卡 IP（即使是内网）
	return localIP
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

// getNetworkIP 从网卡获取 IP 地址
func getNetworkIP() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return ""
	}

	for _, addr := range addrs {
		if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				return ipnet.IP.String()
			}
		}
	}
	return ""
}

// getPublicIPFromAPI 通过外部 API 获取公网 IP
func getPublicIPFromAPI() string {
	// 使用国内 API
	apis := []string{
		"https://4.ipw.cn",
		"https://6.ipw.cn",
	}

	client := &http.Client{Timeout: 5 * time.Second}

	for _, api := range apis {
		resp, err := client.Get(api)
		if err != nil {
			continue
		}
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			continue
		}

		ip := strings.TrimSpace(string(body))
		// 验证是否为有效 IP
		if net.ParseIP(ip) != nil {
			return ip
		}
	}
	return ""
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

	log.Printf("开始部署核心: %s -> %s (平台: %s/%s)", data.CoreType, targetPath, runtime.GOOS, runtime.GOARCH)

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
	if err := os.MkdirAll(targetPath, 0755); err != nil {
		return nil, fmt.Errorf("创建目标目录失败: %v", err)
	}

	// 3. 检查二进制文件是否存在（已有或需要从嵌入资源复制）
	binaryPath := filepath.Join(targetPath, binaryName)
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

		// 如果已有二进制，跳过嵌入的二进制
		if isBinary && binaryExists {
			log.Printf("  跳过嵌入二进制（使用现有）: %s", relPath)
			return nil
		}

		targetFile := filepath.Join(targetPath, targetFileName)

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
		configPath := filepath.Join(targetPath, configFileName)
		if err := os.WriteFile(configPath, []byte(data.Config), 0644); err != nil {
			return nil, fmt.Errorf("写入配置文件失败: %v", err)
		}
		log.Printf("  更新配置文件: %s", configPath)
	}

	// 5. 安装并启动服务 (使用平台特定的 ServiceManager)
	log.Printf("  安装服务: %s", serviceName)
	if err := serviceManager.InstallService(serviceName, targetPath, data.CoreType); err != nil {
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
		"target_path":  targetPath,
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

	// 下载 URL（使用 ghfast.top 加速域名）
	downloadURL := fmt.Sprintf("https://ghfast.top/https://github.com/amuae/sboard/releases/latest/download/%s", zipFileName)

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
