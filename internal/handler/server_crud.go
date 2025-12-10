package handler

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sboard-go/sboard/internal/agent"
	"github.com/sboard-go/sboard/internal/database"
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
		// 使用实时的存活判断（12秒内心跳达到4次）
		agentOnline := agentHub.IsAgentOnline(srv.ID)
		statuses[i] = ServerStatus{
			ID:          srv.ID,
			AgentOnline: agentOnline,
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
	// DNS 解析策略直接赋值（none/ipv4/ipv6 都是有效值）
	if req.DnsResolve != "" {
		server.DnsResolve = req.DnsResolve
	} else {
		server.DnsResolve = "none"
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

	// 检查 Agent 是否在线（12秒内心跳达到4次）
	if !agentHub.IsAgentOnline(server.ID) {
		errorJSON(c, http.StatusBadRequest, "Agent 未在线，无法部署")
		return
	}

	s.handleDeployServerViaAgent(c, &server, req.Type)
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
	config, err := GenerateServerConfig(&fullServer, "sing-box")
	if err != nil {
		errorJSON(c, http.StatusInternalServerError, "生成配置失败: "+err.Error())
		return
	}

	// 目标路径
	targetPath := "/root/sing-box"

	// 根据部署类型决定发送的消息
	if deployType == "folder" {
		// 目录部署：部署核心程序和配置
		data := &agent.DeployCoreData{
			CoreType:   "sing-box",
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
			ConfigType: "sing-box",
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
