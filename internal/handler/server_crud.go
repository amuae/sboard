package handler

import (
	"bufio"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sboard-go/sboard/internal/agent"
	"github.com/sboard-go/sboard/internal/config"
	"github.com/sboard-go/sboard/internal/database"
	"golang.org/x/crypto/ssh"
	"gopkg.in/yaml.v3"
)

// CreateServerRequest 创建服务器请求
type CreateServerRequest struct {
	Name       string `json:"name" binding:"required"`
	Host       string `json:"host"`        // 由 Agent 上报，不再必填
	NodeDomain string `json:"node_domain"` // 节点域名（用于订阅 DNS 解析）
	Port       int    `json:"port"`
	Category   string `json:"category"`
	Enabled    int    `json:"enabled"`
	Node1      string `json:"node_1"`
	Node2      string `json:"node_2"`
	Node3      string `json:"node_3"`
	CoreType   string `json:"core_type"`
	DnsResolve string `json:"dns_resolve"`
	DeployMode string `json:"deploy_mode"` // ssh / agent
	SshUser    string `json:"ssh_user"`
	SshKeyPath string `json:"ssh_key_path"`
	Notes      string `json:"notes"`
}

// UpdateServerRequest 更新服务器请求
type UpdateServerRequest struct {
	Name       string `json:"name"`
	Host       string `json:"host"`
	NodeDomain string `json:"node_domain"` // 节点域名（用于订阅 DNS 解析）
	Port       int    `json:"port"`
	Category   string `json:"category"`
	Enabled    int    `json:"enabled"`
	Node1      string `json:"node_1"`
	Node2      string `json:"node_2"`
	Node3      string `json:"node_3"`
	CoreType   string `json:"core_type"`
	DnsResolve string `json:"dns_resolve"`
	DeployMode string `json:"deploy_mode"` // ssh / agent
	SshUser    string `json:"ssh_user"`
	SshKeyPath string `json:"ssh_key_path"`
	Notes      string `json:"notes"`
}

// handleListServers 获取服务器列表
func (s *Server) handleListServers(c *gin.Context) {
	var servers []database.Server

	query := database.DB.Order("sort_order ASC, id ASC")

	// 支持分类过滤
	if category := c.Query("category"); category != "" {
		query = query.Where("category = ?", category)
	}

	// 支持启用状态过滤
	if enableStr := c.Query("enabled"); enableStr != "" {
		enabled, _ := strconv.Atoi(enableStr)
		query = query.Where("enabled = ?", enabled)
	}

	if err := query.Find(&servers).Error; err != nil {
		errorJSON(c, http.StatusInternalServerError, "查询失败")
		return
	}

	// 按分类分组
	directServers := []database.Server{}
	relayServers := []database.Server{}
	homeServers := []database.Server{}

	for _, server := range servers {
		switch server.Category {
		case "direct":
			directServers = append(directServers, server)
		case "relay":
			relayServers = append(relayServers, server)
		case "home":
			homeServers = append(homeServers, server)
		default:
			directServers = append(directServers, server)
		}
	}

	// 获取统计信息
	var total int64
	database.DB.Model(&database.Server{}).Count(&total)

	var enabledCount int64
	database.DB.Model(&database.Server{}).Where("enabled = ?", 1).Count(&enabledCount)

	successJSON(c, gin.H{
		"servers": servers,
		"grouped": gin.H{
			"direct": directServers,
			"relay":  relayServers,
			"home":   homeServers,
		},
		"stats": gin.H{
			"total":   total,
			"enabled": enabledCount,
		},
	})
}

// ServerStatus 服务器状态
type ServerStatus struct {
	ID          uint    `json:"id"`
	AgentOnline bool    `json:"agent_online"`
	CpuUsage    float64 `json:"cpu_usage"`
	MemUsage    float64 `json:"mem_usage"`
	DiskUsage   float64 `json:"disk_usage"`
	NetIn       int64   `json:"net_in"`      // bytes/s
	NetOut      int64   `json:"net_out"`     // bytes/s
	MonthlyIn   int64   `json:"monthly_in"`  // 月度下行流量 (bytes)
	MonthlyOut  int64   `json:"monthly_out"` // 月度上行流量 (bytes)
}

// handleGetAllServersStatus 批量获取所有服务器状态
func (s *Server) handleGetAllServersStatus(c *gin.Context) {
	var servers []database.Server
	if err := database.DB.Select("id, agent_online, cpu_usage, mem_usage, disk_usage, net_in, net_out, monthly_in, monthly_out").Find(&servers).Error; err != nil {
		errorJSON(c, http.StatusInternalServerError, "查询失败")
		return
	}

	statuses := make([]ServerStatus, len(servers))
	for i, srv := range servers {
		statuses[i] = ServerStatus{
			ID:          srv.ID,
			AgentOnline: srv.AgentOnline,
			CpuUsage:    srv.CpuUsage,
			MemUsage:    srv.MemUsage,
			DiskUsage:   srv.DiskUsage,
			NetIn:       srv.NetIn,
			NetOut:      srv.NetOut,
			MonthlyIn:   srv.MonthlyIn,
			MonthlyOut:  srv.MonthlyOut,
		}
	}

	successJSON(c, gin.H{
		"statuses": statuses,
	})
}

// handleGetServer 获取单个服务器
func (s *Server) handleGetServer(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		errorJSON(c, http.StatusBadRequest, "无效的服务器ID")
		return
	}

	var server database.Server
	if err := database.DB.First(&server, id).Error; err != nil {
		errorJSON(c, http.StatusNotFound, "服务器不存在")
		return
	}

	// 获取服务器的节点配置
	var nodeConfigs []database.ServerNodeConfig
	database.DB.Where("server_id = ?", id).Preload("Node").Find(&nodeConfigs)

	successJSON(c, gin.H{
		"server":       server,
		"node_configs": nodeConfigs,
	})
}

// handleCreateServer 创建服务器
func (s *Server) handleCreateServer(c *gin.Context) {
	var req CreateServerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errorJSON(c, http.StatusBadRequest, "请求参数错误: "+err.Error())
		return
	}

	// 设置默认值
	if req.Port == 0 {
		req.Port = 22
	}
	if req.SshUser == "" {
		req.SshUser = "root"
	}
	if req.CoreType == "" {
		req.CoreType = "sing-box"
	}
	if req.Category == "" {
		req.Category = "direct"
	}

	// 检查名称是否已存在
	var count int64
	database.DB.Model(&database.Server{}).Where("name = ?", req.Name).Count(&count)
	if count > 0 {
		errorJSON(c, http.StatusBadRequest, "服务器名称已存在")
		return
	}

	server := database.Server{
		Name:       req.Name,
		Host:       req.Host,
		NodeDomain: req.NodeDomain,
		DnsResolve: req.DnsResolve,
		Port:       22,
		Category:   req.Category,
		Enabled:    req.Enabled,
		Node1:      req.Node1,
		Node2:      req.Node2,
		Node3:      req.Node3,
		CoreType:   req.CoreType,
		DeployMode: "agent",              // 始终使用 Agent 模式
		AgentToken: generateAgentToken(), // 自动生成 Token
		Notes:      req.Notes,
	}

	if err := database.DB.Create(&server).Error; err != nil {
		errorJSON(c, http.StatusInternalServerError, "创建失败")
		return
	}

	successDataMsgJSON(c, "服务器创建成功", server)
}

// handleUpdateServer 更新服务器
func (s *Server) handleUpdateServer(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		errorJSON(c, http.StatusBadRequest, "无效的服务器ID")
		return
	}

	var server database.Server
	if err := database.DB.First(&server, id).Error; err != nil {
		errorJSON(c, http.StatusNotFound, "服务器不存在")
		return
	}

	var req UpdateServerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errorJSON(c, http.StatusBadRequest, "请求参数错误")
		return
	}

	// 如果有新名称，检查是否重复
	if req.Name != "" && req.Name != server.Name {
		var count int64
		database.DB.Model(&database.Server{}).Where("name = ? AND id != ?", req.Name, id).Count(&count)
		if count > 0 {
			errorJSON(c, http.StatusBadRequest, "服务器名称已存在")
			return
		}
		server.Name = req.Name
	}

	// 更新字段
	if req.Host != "" {
		server.Host = req.Host
	}
	// 节点域名可以设置为空，所以直接赋值
	server.NodeDomain = req.NodeDomain
	if req.Port != 0 {
		server.Port = req.Port
	}
	if req.Category != "" {
		server.Category = req.Category
	}
	server.Enabled = req.Enabled
	server.Node1 = req.Node1
	server.Node2 = req.Node2
	server.Node3 = req.Node3
	if req.CoreType != "" {
		server.CoreType = req.CoreType
	}
	// DNS 解析策略直接赋值（none/ipv4/ipv6 都是有效值）
	if req.DnsResolve != "" {
		server.DnsResolve = req.DnsResolve
	} else {
		server.DnsResolve = "none"
	}
	if req.SshUser != "" {
		server.SshUser = req.SshUser
	}
	if req.SshKeyPath != "" {
		server.SshKeyPath = req.SshKeyPath
	}
	server.Notes = req.Notes

	if err := database.DB.Save(&server).Error; err != nil {
		errorJSON(c, http.StatusInternalServerError, "保存失败")
		return
	}

	successDataMsgJSON(c, "服务器更新成功", server)
}

// handleDeleteServer 删除服务器
func (s *Server) handleDeleteServer(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		errorJSON(c, http.StatusBadRequest, "无效的服务器ID")
		return
	}

	// 同时删除关联的节点配置
	database.DB.Unscoped().Where("server_id = ?", id).Delete(&database.ServerNodeConfig{})

	result := database.DB.Unscoped().Delete(&database.Server{}, id)
	if result.Error != nil {
		errorJSON(c, http.StatusInternalServerError, "删除失败")
		return
	}

	if result.RowsAffected == 0 {
		errorJSON(c, http.StatusNotFound, "服务器不存在")
		return
	}

	successMsgJSON(c, "服务器删除成功")
}

// handleReorderServers 重新排序服务器
func (s *Server) handleReorderServers(c *gin.Context) {
	var req struct {
		IDs []uint `json:"ids" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		errorJSON(c, http.StatusBadRequest, "请求参数错误")
		return
	}

	// 批量更新排序
	for i, id := range req.IDs {
		database.DB.Model(&database.Server{}).Where("id = ?", id).Update("sort_order", i)
	}

	successMsgJSON(c, "排序更新成功")
}

// ServerNodeConfigRequest 服务器节点配置请求
type ServerNodeConfigRequest struct {
	NodeID           uint   `json:"node_id" binding:"required"`
	ForwardEnabled   bool   `json:"forward_enabled"`
	ForwardHost      string `json:"forward_host"`
	ForwardPort      int    `json:"forward_port"`
	OutboundEnabled  bool   `json:"outbound_enabled"`
	OutboundProtocol string `json:"outbound_protocol"`
	OutboundHost     string `json:"outbound_host"`
	OutboundPort     int    `json:"outbound_port"`
	OutboundPassword string `json:"outbound_password"`
	OutboundMethod   string `json:"outbound_method"`
	OutboundUsername string `json:"outbound_username"`
	OutboundSni      string `json:"outbound_sni"`
}

// handleSetServerNodes 设置服务器节点配置
func (s *Server) handleSetServerNodes(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		errorJSON(c, http.StatusBadRequest, "无效的服务器ID")
		return
	}

	var server database.Server
	if err := database.DB.First(&server, id).Error; err != nil {
		errorJSON(c, http.StatusNotFound, "服务器不存在")
		return
	}

	var req struct {
		Configs []ServerNodeConfigRequest `json:"configs"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		errorJSON(c, http.StatusBadRequest, "请求参数错误")
		return
	}

	// 删除旧配置
	database.DB.Unscoped().Where("server_id = ?", id).Delete(&database.ServerNodeConfig{})

	// 创建新配置
	for _, cfgReq := range req.Configs {
		config := database.ServerNodeConfig{
			ServerID:         uint(id),
			NodeID:           cfgReq.NodeID,
			ForwardEnabled:   cfgReq.ForwardEnabled,
			ForwardHost:      cfgReq.ForwardHost,
			ForwardPort:      cfgReq.ForwardPort,
			OutboundEnabled:  cfgReq.OutboundEnabled,
			OutboundProtocol: cfgReq.OutboundProtocol,
			OutboundHost:     cfgReq.OutboundHost,
			OutboundPort:     cfgReq.OutboundPort,
			OutboundPassword: cfgReq.OutboundPassword,
			OutboundMethod:   cfgReq.OutboundMethod,
			OutboundUsername: cfgReq.OutboundUsername,
			OutboundSni:      cfgReq.OutboundSni,
		}
		database.DB.Create(&config)
	}

	successMsgJSON(c, "节点配置已更新")
}

// handleDeployServer 部署服务器
func (s *Server) handleDeployServer(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		errorJSON(c, http.StatusBadRequest, "无效的服务器ID")
		return
	}

	// 获取请求参数
	var req struct {
		Type string `json:"type"` // config 或 folder
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		req.Type = "config" // 默认是 config
	}

	var server database.Server
	if err := database.DB.First(&server, id).Error; err != nil {
		errorJSON(c, http.StatusNotFound, "服务器不存在")
		return
	}

	// 根据部署模式选择不同的部署方式
	if server.DeployMode == "agent" {
		// Agent 模式
		s.handleDeployServerViaAgent(c, &server, req.Type)
		return
	}

	// SSH 模式 (默认)
	s.handleDeployServerViaSSH(c, &server, req.Type)
}

// handleDeployServerViaAgent 通过 Agent 部署
func (s *Server) handleDeployServerViaAgent(c *gin.Context, server *database.Server, deployType string) {
	// 生成配置
	// 为保证一致性，重新查出完整 server 结构体（与预览接口一致）
	var fullServer database.Server
	if err := database.DB.First(&fullServer, server.ID).Error; err != nil {
		errorJSON(c, http.StatusNotFound, "服务器不存在")
		return
	}
	config, err := GenerateServerConfig(&fullServer, fullServer.CoreType)
	if err != nil {
		errorJSON(c, http.StatusInternalServerError, "生成配置失败: "+err.Error())
		return
	}

	// 目标路径
	targetPath := "/root/" + server.CoreType

	// 根据部署类型决定发送的消息
	if deployType == "folder" {
		// 目录部署：部署核心程序和配置
		data := &agent.DeployCoreData{
			CoreType:   server.CoreType,
			TargetPath: targetPath,
			Config:     config,
		}
		rawData, _ := json.Marshal(data)

		msg := &agent.Message{
			Type: agent.MsgTypeDeployCore,
			Data: rawData,
		}

		resp, err := agentHub.SendCommand(uint(server.ID), msg, 120*time.Second)
		if err != nil {
			if _, ok := err.(*AgentOfflineError); ok {
				errorJSON(c, http.StatusServiceUnavailable, "Agent 离线")
				return
			}
			errorJSON(c, http.StatusGatewayTimeout, err.Error())
			return
		}

		if resp.Error != "" {
			errorJSON(c, http.StatusInternalServerError, resp.Error)
			return
		}

		// 解析响应
		var respData agent.DeployCoreRespData
		if err := resp.ParseData(&respData); err != nil {
			errorJSON(c, http.StatusInternalServerError, "解析响应失败")
			return
		}

		// 更新服务器的最后部署时间
		database.DB.Model(server).Update("last_deploy_at", time.Now())

		// 构建输出信息
		output := fmt.Sprintf("目录部署完成！\n核心类型: %s\n目标路径: %s\n服务名: %s\n运行状态: %v\n对侧服务: %s (运行状态: %v)",
			respData.CoreType, respData.TargetPath, respData.ServiceName, respData.Running,
			respData.OtherService, respData.OtherRunning)

		successJSON(c, gin.H{
			"output": output,
			"data":   respData,
		})
	} else {
		// 配置部署：只更新配置文件
		data := &agent.SyncConfigData{
			ConfigType: server.CoreType,
			Content:    config,
			Restart:    true,
		}
		rawData, _ := json.Marshal(data)

		msg := &agent.Message{
			Type: agent.MsgTypeSyncConfig,
			Data: rawData,
		}

		resp, err := agentHub.SendCommand(uint(server.ID), msg, 60*time.Second)
		if err != nil {
			if _, ok := err.(*AgentOfflineError); ok {
				errorJSON(c, http.StatusServiceUnavailable, "Agent 离线")
				return
			}
			errorJSON(c, http.StatusGatewayTimeout, err.Error())
			return
		}

		if resp.Error != "" {
			errorJSON(c, http.StatusInternalServerError, resp.Error)
			return
		}

		// 更新服务器的最后部署时间
		database.DB.Model(server).Update("last_deploy_at", time.Now())

		successJSON(c, gin.H{
			"output": "配置部署完成！已同步配置并重启服务。",
		})
	}
}

// handleDeployServerViaSSH 通过 SSH 部署 (SSE)
func (s *Server) handleDeployServerViaSSH(c *gin.Context, server *database.Server, _ string) {
	// 获取服务器的节点配置
	var nodeConfigs []database.ServerNodeConfig
	database.DB.Where("server_id = ?", server.ID).Preload("Node").Find(&nodeConfigs)

	// 获取所有启用的用户
	var users []database.ProxyUser
	database.DB.Where("enabled = ?", 1).Find(&users)

	// 设置 SSE 头
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")

	sendSSE := func(eventType, message string) {
		c.SSEvent(eventType, message)
		c.Writer.Flush()
	}

	sendSSE("message", fmt.Sprintf("开始部署服务器: %s (%s)", server.Name, server.Host))

	// 连接 SSH
	sendSSE("message", "正在连接服务器...")
	client, err := connectSSH(server, s.config)
	if err != nil {
		sendSSE("error", fmt.Sprintf("SSH 连接失败: %s", err.Error()))
		return
	}
	defer client.Close()
	sendSSE("message", "SSH 连接成功")

	// 生成配置
	sendSSE("message", "正在生成核心配置...")
	configContent, err := generateCoreConfig(server, nodeConfigs, users)
	if err != nil {
		sendSSE("error", fmt.Sprintf("生成配置失败: %s", err.Error()))
		return
	}
	sendSSE("message", "配置生成成功")

	// 上传配置
	sendSSE("message", "正在上传配置文件...")
	configPath := getConfigPath(server.CoreType)
	if err := uploadFile(client, configPath, configContent); err != nil {
		sendSSE("error", fmt.Sprintf("上传配置失败: %s", err.Error()))
		return
	}
	sendSSE("message", "配置文件已上传")

	// 重启服务
	sendSSE("message", "正在重启核心服务...")
	serviceName := getServiceName(server.CoreType)
	if err := runSSHCommand(client, "systemctl restart "+serviceName, sendSSE); err != nil {
		sendSSE("error", fmt.Sprintf("重启服务失败: %s", err.Error()))
		return
	}
	sendSSE("message", "服务重启成功")

	// 检查服务状态
	sendSSE("message", "正在检查服务状态...")
	if err := runSSHCommand(client, "systemctl is-active "+serviceName, sendSSE); err != nil {
		sendSSE("warning", "服务可能未正常启动，请手动检查")
	} else {
		sendSSE("message", "服务运行正常")
	}

	sendSSE("success", "部署完成!")
}

// handleTestServer 测试服务器连接
func (s *Server) handleTestServer(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		errorJSON(c, http.StatusBadRequest, "无效的服务器ID")
		return
	}

	var server database.Server
	if err := database.DB.First(&server, id).Error; err != nil {
		errorJSON(c, http.StatusNotFound, "服务器不存在")
		return
	}

	// 尝试连接
	client, err := connectSSH(&server, s.config)
	if err != nil {
		errorJSON(c, http.StatusServiceUnavailable, fmt.Sprintf("连接失败: %s", err.Error()))
		return
	}
	defer client.Close()

	successMsgJSON(c, "连接成功")
}

// handleGetServerStatus 获取服务器状态
func (s *Server) handleGetServerStatus(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		errorJSON(c, http.StatusBadRequest, "无效的服务器ID")
		return
	}

	var server database.Server
	if err := database.DB.First(&server, id).Error; err != nil {
		errorJSON(c, http.StatusNotFound, "服务器不存在")
		return
	}

	client, err := connectSSH(&server, s.config)
	if err != nil {
		successJSON(c, gin.H{
			"connected":      false,
			"error":          err.Error(),
			"service_active": false,
		})
		return
	}
	defer client.Close()

	// 检查服务状态
	serviceName := getServiceName(server.CoreType)
	session, err := client.NewSession()
	if err != nil {
		successJSON(c, gin.H{
			"connected":      true,
			"service_active": false,
		})
		return
	}
	defer session.Close()

	output, err := session.CombinedOutput("systemctl is-active " + serviceName)
	serviceActive := err == nil && strings.TrimSpace(string(output)) == "active"

	successJSON(c, gin.H{
		"connected":      true,
		"service_active": serviceActive,
		"core_type":      server.CoreType,
	})
}

// SSH 辅助函数

func connectSSH(server *database.Server, cfg *config.Config) (*ssh.Client, error) {
	var authMethods []ssh.AuthMethod

	// 使用密钥认证
	if server.SshKeyPath != "" {
		// 读取密钥文件
		keyPath := server.SshKeyPath
		keyContent, err := readFileContent(keyPath)
		if err == nil {
			signer, err := ssh.ParsePrivateKey([]byte(keyContent))
			if err == nil {
				authMethods = append(authMethods, ssh.PublicKeys(signer))
			}
		}
	}

	if len(authMethods) == 0 {
		return nil, fmt.Errorf("没有可用的认证方式，请检查 SSH 密钥配置")
	}

	sshConfig := &ssh.ClientConfig{
		User:            server.SshUser,
		Auth:            authMethods,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         time.Duration(cfg.SSH.Timeout) * time.Second,
	}

	addr := net.JoinHostPort(server.Host, strconv.Itoa(server.Port))
	return ssh.Dial("tcp", addr, sshConfig)
}

func readFileContent(_ string) (string, error) {
	// TODO: 使用 os.ReadFile 读取密钥文件
	return "", fmt.Errorf("not implemented")
}

func uploadFile(client *ssh.Client, remotePath, content string) error {
	session, err := client.NewSession()
	if err != nil {
		return err
	}
	defer session.Close()

	// 使用 cat 命令写入文件
	stdin, err := session.StdinPipe()
	if err != nil {
		return err
	}

	if err := session.Start(fmt.Sprintf("cat > %s", remotePath)); err != nil {
		return err
	}

	_, err = io.WriteString(stdin, content)
	if err != nil {
		return err
	}

	stdin.Close()
	return session.Wait()
}

func runSSHCommand(client *ssh.Client, command string, sendSSE func(string, string)) error {
	session, err := client.NewSession()
	if err != nil {
		return err
	}
	defer session.Close()

	stdout, err := session.StdoutPipe()
	if err != nil {
		return err
	}

	stderr, err := session.StderrPipe()
	if err != nil {
		return err
	}

	if err := session.Start(command); err != nil {
		return err
	}

	// 读取输出
	go func() {
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			sendSSE("output", scanner.Text())
		}
	}()

	go func() {
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			sendSSE("stderr", scanner.Text())
		}
	}()

	return session.Wait()
}

func getConfigPath(core string) string {
	switch core {
	case "mihomo":
		return "/etc/mihomo/config.yaml"
	case "sing-box":
		return "/etc/sing-box/config.json"
	default:
		return "/etc/sing-box/config.json"
	}
}

func getServiceName(core string) string {
	switch core {
	case "mihomo":
		return "mihomo"
	case "sing-box":
		return "sing-box"
	default:
		return "sing-box"
	}
}

func generateCoreConfig(server *database.Server, nodeConfigs []database.ServerNodeConfig, users []database.ProxyUser) (string, error) {
	switch server.CoreType {
	case "mihomo":
		return generateMihomoConfig(server, nodeConfigs, users)
	case "sing-box":
		return generateSingBoxConfig(server, nodeConfigs, users)
	default:
		return generateSingBoxConfig(server, nodeConfigs, users)
	}
}

// 配置生成函数 - 为特定服务器生成入站配置
func generateMihomoConfig(_ *database.Server, nodeConfigs []database.ServerNodeConfig, users []database.ProxyUser) (string, error) {
	config := map[string]interface{}{
		"log-level": "silent",
		"listeners": []interface{}{},
	}

	// 构建用户 UUID 映射
	userMap := make(map[uint]database.ProxyUser)
	for _, u := range users {
		userMap[u.ID] = u
	}

	listeners := []interface{}{}

	for _, nc := range nodeConfigs {
		if nc.Node.ID == 0 {
			continue
		}
		node := nc.Node

		// 获取该节点关联的启用用户
		nodeUsers := getNodeUsersFromList(node.ID, users)
		if len(nodeUsers) == 0 {
			continue
		}

		// 构建 listener
		listener := buildServerMihomoListener(&node, nodeUsers, &nc)
		if listener != nil {
			listeners = append(listeners, listener)
		}
	}

	config["listeners"] = listeners

	yamlData, err := yaml.Marshal(config)
	if err != nil {
		return "", err
	}

	return string(yamlData), nil
}

func generateSingBoxConfig(_ *database.Server, nodeConfigs []database.ServerNodeConfig, users []database.ProxyUser) (string, error) {
	config := map[string]interface{}{
		"log": map[string]interface{}{
			"disabled": true,
		},
		"inbounds": []interface{}{},
		"outbounds": []interface{}{
			map[string]interface{}{
				"type": "direct",
				"tag":  "direct",
			},
		},
	}

	inbounds := []interface{}{}

	for _, nc := range nodeConfigs {
		if nc.Node.ID == 0 {
			continue
		}
		node := nc.Node

		// 获取该节点关联的启用用户
		nodeUsers := getNodeUsersFromList(node.ID, users)
		if len(nodeUsers) == 0 {
			continue
		}

		// 构建 inbound
		inbound := buildServerSingBoxInbound(&node, nodeUsers, &nc)
		if inbound != nil {
			inbounds = append(inbounds, inbound)
		}
	}

	config["inbounds"] = inbounds

	jsonData, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return "", err
	}

	return string(jsonData), nil
}

// getNodeUsersFromList 从用户列表中获取节点关联的用户
func getNodeUsersFromList(nodeID uint, users []database.ProxyUser) []NodeUser {
	var relations []database.NodeUserRelation
	database.DB.Where("node_id = ?", nodeID).Find(&relations)

	userMap := make(map[uint]database.ProxyUser)
	for _, u := range users {
		userMap[u.ID] = u
	}

	result := []NodeUser{}
	for _, rel := range relations {
		if user, ok := userMap[rel.UserID]; ok {
			result = append(result, NodeUser{
				Name:  user.Name,
				UUID:  rel.UUID,
				Flow:  rel.Flow,
				Level: user.Level,
			})
		}
	}
	return result
}

// buildServerMihomoListener 为服务器构建 mihomo listener
func buildServerMihomoListener(node *database.InboundNode, users []NodeUser, nc *database.ServerNodeConfig) map[string]interface{} {
	// 使用转发端口
	port := nc.ForwardPort
	if port == 0 {
		port = node.Port
	}

	listener := map[string]interface{}{
		"name":   node.Tag,
		"type":   node.Protocol,
		"port":   port,
		"listen": nc.ForwardHost,
	}

	// 用户配置（与全局配置相同）
	switch node.Protocol {
	case "anytls":
		usersObj := map[string]string{}
		for _, user := range users {
			usersObj[user.Name] = user.UUID
		}
		listener["users"] = usersObj

	case "vless":
		usersArray := []map[string]interface{}{}
		for _, user := range users {
			userEntry := map[string]interface{}{
				"username": user.Name,
				"uuid":     user.UUID,
			}
			if user.Flow != "" {
				userEntry["flow"] = user.Flow
			}
			usersArray = append(usersArray, userEntry)
		}
		listener["users"] = usersArray

	case "vmess":
		usersArray := []map[string]interface{}{}
		for _, user := range users {
			usersArray = append(usersArray, map[string]interface{}{
				"username": user.Name,
				"uuid":     user.UUID,
				"alterId":  0,
			})
		}
		listener["users"] = usersArray

	case "trojan":
		usersArray := []map[string]interface{}{}
		for _, user := range users {
			usersArray = append(usersArray, map[string]interface{}{
				"username": user.Name,
				"password": user.UUID,
			})
		}
		listener["users"] = usersArray

	case "shadowsocks":
		method := node.SsMethod
		if method == "" {
			method = "aes-256-gcm"
		}
		listener["cipher"] = method
		listener["password"] = node.SsPassword
		listener["udp"] = true

	case "hysteria2":
		usersObj := map[string]string{}
		password := node.Hy2Password
		for _, user := range users {
			if password != "" {
				usersObj[user.Name] = password
			} else {
				usersObj[user.Name] = user.UUID
			}
		}
		listener["users"] = usersObj
		listener["up"] = node.Hy2UpMbps
		listener["down"] = node.Hy2DownMbps
	}

	// TLS 配置
	if node.TlsEnabled && node.Protocol != "shadowsocks" {
		if node.RealityEnabled && node.RealityPrivkey != "" {
			destServer := node.RealityServer
			if destServer == "" {
				destServer = "www.apple.com"
			}
			if !strings.Contains(destServer, ":") {
				destServer += ":443"
			}
			listener["reality-config"] = map[string]interface{}{
				"dest":         destServer,
				"private-key":  node.RealityPrivkey,
				"short-id":     []string{node.RealityShortId},
				"server-names": []string{node.ServerName},
			}
		} else {
			listener["certificate"] = "./server.crt"
			listener["private-key"] = "./server.key"
		}
	}

	return listener
}

// buildServerSingBoxInbound 为服务器构建 sing-box inbound
func buildServerSingBoxInbound(node *database.InboundNode, users []NodeUser, nc *database.ServerNodeConfig) map[string]interface{} {
	port := nc.ForwardPort
	if port == 0 {
		port = node.Port
	}

	inbound := map[string]interface{}{
		"type":        node.Protocol,
		"tag":         node.Tag,
		"listen":      nc.ForwardHost,
		"listen_port": port,
	}

	// 构建用户列表
	userList := []map[string]interface{}{}
	for _, user := range users {
		userEntry := map[string]interface{}{"name": user.Name}

		switch node.Protocol {
		case "trojan":
			userEntry["password"] = user.UUID
		case "vless":
			userEntry["uuid"] = user.UUID
			if node.TlsEnabled && !node.TransportEnabled && user.Flow != "" {
				userEntry["flow"] = user.Flow
			}
		case "vmess":
			userEntry["uuid"] = user.UUID
			userEntry["alterId"] = 0
		case "anytls":
			userEntry["password"] = user.UUID
		case "shadowsocks":
			userEntry["password"] = fmt.Sprintf("%x", []byte(user.UUID)[:16])
		case "hysteria2":
			if node.Hy2Password != "" {
				userEntry["password"] = node.Hy2Password
			} else {
				userEntry["password"] = user.UUID
			}
		}
		userList = append(userList, userEntry)
	}

	if node.Protocol == "shadowsocks" {
		method := node.SsMethod
		if method == "" {
			method = "aes-256-gcm"
		}
		inbound["method"] = method
		inbound["password"] = node.SsPassword
		if strings.HasPrefix(method, "2022-") {
			inbound["users"] = userList
		}
	} else {
		inbound["users"] = userList
		if node.Protocol == "hysteria2" {
			inbound["up_mbps"] = node.Hy2UpMbps
			inbound["down_mbps"] = node.Hy2DownMbps
		}
	}

	// TLS 配置
	if node.TlsEnabled && node.Protocol != "shadowsocks" {
		tls := map[string]interface{}{
			"enabled":     true,
			"server_name": node.ServerName,
		}

		if node.RealityEnabled && node.RealityPubkey != "" && !node.TransportEnabled {
			tls["reality"] = map[string]interface{}{
				"enabled": true,
				"handshake": map[string]interface{}{
					"server":      node.RealityServer,
					"server_port": 443,
				},
				"private_key": node.RealityPrivkey,
				"short_id":    []string{node.RealityShortId},
			}
		} else {
			tls["certificate_path"] = "/root/sing-box/server.crt"
			tls["key_path"] = "/root/sing-box/server.key"
		}

		inbound["tls"] = tls
	}

	// 传输层配置
	if node.TransportEnabled && !node.RealityEnabled {
		transport := map[string]interface{}{}
		switch node.TransportType {
		case "ws":
			transport["type"] = "ws"
			transport["path"] = node.WsPath
			if node.TransportHost != "" {
				transport["headers"] = map[string]string{"Host": node.TransportHost}
			}
		case "grpc":
			transport["type"] = "grpc"
			transport["service_name"] = node.GrpcService
		}
		if len(transport) > 0 {
			inbound["transport"] = transport
		}
	}

	return inbound
}

// generateAgentToken 生成 Agent Token
func generateAgentToken() string {
	bytes := make([]byte, 32)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)
}

// handleRegenerateAgentToken 重新生成 Agent Token
func (s *Server) handleRegenerateAgentToken(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		errorJSON(c, http.StatusBadRequest, "无效的服务器ID")
		return
	}

	var server database.Server
	if err := database.DB.First(&server, id).Error; err != nil {
		errorJSON(c, http.StatusNotFound, "服务器不存在")
		return
	}

	// 生成新 Token
	newToken := generateAgentToken()
	database.DB.Model(&server).Update("agent_token", newToken)

	successJSON(c, gin.H{
		"agent_token": newToken,
	})
}
