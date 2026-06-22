package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/sboard-go/sboard/internal/database"
)

// ========== 服务器节点配置 API ==========

// NodeConfigRequest 节点配置请求
type NodeConfigRequest struct {
	ListenPort int `json:"listen_port"`

	// 端口转发配置
	ForwardEnabled bool   `json:"forward_enabled"`
	ForwardHost    string `json:"forward_host"`
	ForwardPort    int    `json:"forward_port"`

	// 落地出站配置
	OutboundEnabled  bool   `json:"outbound_enabled"`
	OutboundProtocol string `json:"outbound_protocol"` // ss/trojan/anytls/socks5
	OutboundHost     string `json:"outbound_host"`
	OutboundPort     int    `json:"outbound_port"`
	OutboundPassword string `json:"outbound_password"`
	OutboundMethod   string `json:"outbound_method"`   // Shadowsocks 加密方式
	OutboundUsername string `json:"outbound_username"` // SOCKS5 用户名
	OutboundSni      string `json:"outbound_sni"`      // TLS SNI
}

// handleGetServerNodeConfigs 获取服务器的节点配置列表
func (s *Server) handleGetServerNodeConfigs(c *gin.Context) {
	serverID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		errorJSON(c, http.StatusBadRequest, "无效的服务器ID")
		return
	}

	// 检查服务器是否存在
	var server database.Server
	if err := database.DB.First(&server, serverID).Error; err != nil {
		errorJSON(c, http.StatusNotFound, "服务器不存在")
		return
	}

	// 获取服务器的所有节点配置
	var nodeConfigs []database.ServerNodeConfig
	database.DB.Where("server_id = ?", serverID).Preload("Node").Find(&nodeConfigs)

	// 转换为 map 格式，key 为 nodeId
	configMap := map[string]interface{}{}
	for _, nc := range nodeConfigs {
		configMap[strconv.Itoa(int(nc.NodeID))] = map[string]interface{}{
			"id":                nc.ID,
			"node_id":           nc.NodeID,
			"node":              nc.Node,
			"listen_port":       nc.ListenPort,
			"forward_enabled":   nc.ForwardEnabled,
			"forward_host":      nc.ForwardHost,
			"forward_port":      nc.ForwardPort,
			"outbound_enabled":  nc.OutboundEnabled,
			"outbound_protocol": nc.OutboundProtocol,
			"outbound_host":     nc.OutboundHost,
			"outbound_port":     nc.OutboundPort,
			"outbound_password": nc.OutboundPassword,
			"outbound_method":   nc.OutboundMethod,
			"outbound_username": nc.OutboundUsername,
			"outbound_sni":      nc.OutboundSni,
		}
	}

	successJSON(c, configMap)
}

// handleSaveServerNodeConfig 保存服务器的节点配置
func (s *Server) handleSaveServerNodeConfig(c *gin.Context) {
	serverID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		errorJSON(c, http.StatusBadRequest, "无效的服务器ID")
		return
	}

	nodeID, err := strconv.ParseUint(c.Param("nodeId"), 10, 64)
	if err != nil {
		errorJSON(c, http.StatusBadRequest, "无效的节点ID")
		return
	}

	// 检查服务器和节点是否存在
	var server database.Server
	if err := database.DB.First(&server, serverID).Error; err != nil {
		errorJSON(c, http.StatusNotFound, "服务器不存在")
		return
	}

	var node database.InboundNode
	if err := database.DB.First(&node, nodeID).Error; err != nil {
		errorJSON(c, http.StatusNotFound, "节点不存在")
		return
	}

	var req NodeConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errorJSON(c, http.StatusBadRequest, "请求参数错误")
		return
	}

	// 查找或创建节点配置
	var nodeConfig database.ServerNodeConfig
	result := database.DB.Where("server_id = ? AND node_id = ?", serverID, nodeID).First(&nodeConfig)

	if result.Error != nil {
		// 创建新配置
		nodeConfig = database.ServerNodeConfig{
			ServerID: uint(serverID),
			NodeID:   uint(nodeID),
		}
	}

	// 更新配置
	nodeConfig.ListenPort = req.ListenPort
	nodeConfig.ForwardEnabled = req.ForwardEnabled
	nodeConfig.ForwardHost = req.ForwardHost
	nodeConfig.ForwardPort = req.ForwardPort
	nodeConfig.OutboundEnabled = req.OutboundEnabled
	nodeConfig.OutboundProtocol = req.OutboundProtocol
	nodeConfig.OutboundHost = req.OutboundHost
	nodeConfig.OutboundPort = req.OutboundPort
	nodeConfig.OutboundPassword = req.OutboundPassword
	nodeConfig.OutboundMethod = req.OutboundMethod
	nodeConfig.OutboundUsername = req.OutboundUsername
	nodeConfig.OutboundSni = req.OutboundSni

	// 如果没有设置监听端口，使用节点默认端口
	if nodeConfig.ListenPort == 0 {
		nodeConfig.ListenPort = node.Port
	}

	if err := database.DB.Save(&nodeConfig).Error; err != nil {
		errorJSON(c, http.StatusInternalServerError, "保存配置失败")
		return
	}

	successMsgJSON(c, "配置保存成功")
}

// handleDeleteServerNodeConfig 删除服务器的节点配置
func (s *Server) handleDeleteServerNodeConfig(c *gin.Context) {
	serverID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		errorJSON(c, http.StatusBadRequest, "无效的服务器ID")
		return
	}

	nodeID, err := strconv.ParseUint(c.Param("nodeId"), 10, 64)
	if err != nil {
		errorJSON(c, http.StatusBadRequest, "无效的节点ID")
		return
	}

	result := database.DB.Unscoped().Where("server_id = ? AND node_id = ?", serverID, nodeID).Delete(&database.ServerNodeConfig{})
	if result.RowsAffected == 0 {
		errorJSON(c, http.StatusNotFound, "配置不存在")
		return
	}

	successMsgJSON(c, "配置删除成功")
}
