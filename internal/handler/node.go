package handler

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"net/http"
	"os/exec"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/sboard-go/sboard/internal/database"
	"golang.org/x/crypto/curve25519"
)

// CreateNodeRequest 创建节点请求
type CreateNodeRequest struct {
	Tag      string `json:"tag" binding:"required"`
	Protocol string `json:"protocol" binding:"required"`
	Listen   string `json:"listen"`
	Port     int    `json:"port" binding:"required"`

	// TLS
	TlsEnabled bool   `json:"tls_enabled"`
	ServerName string `json:"server_name"`
	CertPath   string `json:"cert_path"`
	KeyPath    string `json:"key_path"`

	// Reality
	RealityEnabled bool   `json:"reality_enabled"`
	RealityServer  string `json:"reality_server"`
	RealityPubkey  string `json:"reality_pubkey"`
	RealityPrivkey string `json:"reality_privkey"`
	RealityShortId string `json:"reality_short_id"`

	// Transport
	TransportEnabled bool   `json:"transport_enabled"`
	TransportType    string `json:"transport_type"`
	WsPath           string `json:"ws_path"`
	GrpcService      string `json:"grpc_service"`
	TransportHost    string `json:"transport_host"`

	// Flow
	Flow string `json:"flow"`

	// Shadowsocks
	SsMethod   string `json:"ss_method"`
	SsPassword string `json:"ss_password"`

	// Hysteria2
	Hy2Password     string `json:"hy2_password"`
	Hy2UpMbps       int    `json:"hy2_up_mbps"`
	Hy2DownMbps     int    `json:"hy2_down_mbps"`
	Hy2Obfs         string `json:"hy2_obfs"`
	Hy2ObfsPassword string `json:"hy2_obfs_password"`

	// Status
	Enabled bool   `json:"enabled"`
	Notes   string `json:"notes"`
}

// UpdateNodeRequest 更新节点请求
type UpdateNodeRequest struct {
	Tag      string `json:"tag"`
	Protocol string `json:"protocol"`
	Listen   string `json:"listen"`
	Port     int    `json:"port"`

	// TLS
	TlsEnabled bool   `json:"tls_enabled"`
	ServerName string `json:"server_name"`
	CertPath   string `json:"cert_path"`
	KeyPath    string `json:"key_path"`

	// Reality
	RealityEnabled bool   `json:"reality_enabled"`
	RealityServer  string `json:"reality_server"`
	RealityPubkey  string `json:"reality_pubkey"`
	RealityPrivkey string `json:"reality_privkey"`
	RealityShortId string `json:"reality_short_id"`

	// Transport
	TransportEnabled bool   `json:"transport_enabled"`
	TransportType    string `json:"transport_type"`
	WsPath           string `json:"ws_path"`
	GrpcService      string `json:"grpc_service"`
	TransportHost    string `json:"transport_host"`

	// Flow
	Flow string `json:"flow"`

	// Shadowsocks
	SsMethod   string `json:"ss_method"`
	SsPassword string `json:"ss_password"`

	// Hysteria2
	Hy2Password     string `json:"hy2_password"`
	Hy2UpMbps       int    `json:"hy2_up_mbps"`
	Hy2DownMbps     int    `json:"hy2_down_mbps"`
	Hy2Obfs         string `json:"hy2_obfs"`
	Hy2ObfsPassword string `json:"hy2_obfs_password"`

	// Status
	Enabled bool   `json:"enabled"`
	Notes   string `json:"notes"`
}

// handleListNodes 获取节点列表
func (s *Server) handleListNodes(c *gin.Context) {
	var nodes []database.InboundNode

	query := database.DB.Order("id ASC")

	// 支持协议过滤
	if protocol := c.Query("protocol"); protocol != "" {
		query = query.Where("protocol = ?", protocol)
	}

	// 支持启用状态过滤
	if enableStr := c.Query("enabled"); enableStr != "" {
		enabled := enableStr == "true" || enableStr == "1"
		query = query.Where("enabled = ?", enabled)
	}

	if err := query.Find(&nodes).Error; err != nil {
		errorJSON(c, http.StatusInternalServerError, "查询失败")
		return
	}

	// 获取统计信息
	var total int64
	database.DB.Model(&database.InboundNode{}).Count(&total)

	var enabledCount int64
	database.DB.Model(&database.InboundNode{}).Where("enabled = ?", true).Count(&enabledCount)

	// 按协议统计
	type ProtocolStat struct {
		Protocol string `json:"protocol"`
		Count    int64  `json:"count"`
	}
	var protocolStats []ProtocolStat
	database.DB.Model(&database.InboundNode{}).Select("protocol, count(*) as count").Group("protocol").Scan(&protocolStats)

	successJSON(c, gin.H{
		"nodes": nodes,
		"stats": gin.H{
			"total":     total,
			"enabled":   enabledCount,
			"protocols": protocolStats,
		},
	})
}

// handleGetNode 获取单个节点
func (s *Server) handleGetNode(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		errorJSON(c, http.StatusBadRequest, "无效的节点ID")
		return
	}

	var node database.InboundNode
	if err := database.DB.First(&node, id).Error; err != nil {
		errorJSON(c, http.StatusNotFound, "节点不存在")
		return
	}

	successJSON(c, node)
}

// handleCreateNode 创建节点
func (s *Server) handleCreateNode(c *gin.Context) {
	var req CreateNodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errorJSON(c, http.StatusBadRequest, "请求参数错误: "+err.Error())
		return
	}

	// 验证协议
	validProtocols := []string{"trojan", "vless", "vmess", "anytls", "shadowsocks", "hysteria2"}
	isValidProtocol := false
	for _, p := range validProtocols {
		if req.Protocol == p {
			isValidProtocol = true
			break
		}
	}
	if !isValidProtocol {
		errorJSON(c, http.StatusBadRequest, "无效的协议类型")
		return
	}

	// 检查 tag 是否已存在
	var count int64
	database.DB.Model(&database.InboundNode{}).Where("tag = ?", req.Tag).Count(&count)
	if count > 0 {
		errorJSON(c, http.StatusBadRequest, "节点 Tag 已存在")
		return
	}

	// 检查端口是否已存在
	database.DB.Model(&database.InboundNode{}).Where("port = ?", req.Port).Count(&count)
	if count > 0 {
		errorJSON(c, http.StatusBadRequest, "端口已被使用")
		return
	}

	// 设置默认值
	if req.Listen == "" {
		req.Listen = "::"
	}
	if req.ServerName == "" {
		req.ServerName = "down.dingtalk.com"
	}
	if req.CertPath == "" {
		req.CertPath = "server.crt"
	}
	if req.KeyPath == "" {
		req.KeyPath = "server.key"
	}
	if req.WsPath == "" {
		req.WsPath = "/"
	}
	if req.Hy2UpMbps == 0 {
		req.Hy2UpMbps = 100
	}
	if req.Hy2DownMbps == 0 {
		req.Hy2DownMbps = 100
	}

	// 如果启用 Reality 且 short_id 为空，自动生成随机 short_id
	if req.RealityEnabled && req.RealityShortId == "" {
		req.RealityShortId = generateRandomShortId()
	}

	node := database.InboundNode{
		Tag:              req.Tag,
		Protocol:         req.Protocol,
		Listen:           req.Listen,
		Port:             req.Port,
		TlsEnabled:       req.TlsEnabled,
		ServerName:       req.ServerName,
		CertPath:         req.CertPath,
		KeyPath:          req.KeyPath,
		RealityEnabled:   req.RealityEnabled,
		RealityServer:    req.RealityServer,
		RealityPubkey:    req.RealityPubkey,
		RealityPrivkey:   req.RealityPrivkey,
		RealityShortId:   req.RealityShortId,
		TransportEnabled: req.TransportEnabled,
		TransportType:    req.TransportType,
		WsPath:           req.WsPath,
		GrpcService:      req.GrpcService,
		TransportHost:    req.TransportHost,
		Flow:             req.Flow,
		SsMethod:         req.SsMethod,
		SsPassword:       req.SsPassword,
		Hy2Password:      req.Hy2Password,
		Hy2UpMbps:        req.Hy2UpMbps,
		Hy2DownMbps:      req.Hy2DownMbps,
		Hy2Obfs:          req.Hy2Obfs,
		Hy2ObfsPassword:  req.Hy2ObfsPassword,
		Enabled:          req.Enabled,
		Notes:            req.Notes,
	}

	if err := database.DB.Create(&node).Error; err != nil {
		errorJSON(c, http.StatusInternalServerError, "创建失败: "+err.Error())
		return
	}

	// 自动链接所有用户到该节点
	s.linkAllUsersToNode(&node)

	// 广播配置更新到所有在线 Agent
	go agentHub.BroadcastConfigUpdate()

	successDataMsgJSON(c, "节点创建成功", node)
}

// linkAllUsersToNode 将所有用户链接到节点
func (s *Server) linkAllUsersToNode(node *database.InboundNode) {
	var users []database.ProxyUser
	database.DB.Where("enabled = ?", 1).Find(&users)

	for _, user := range users {
		// 检查是否已存在关联
		var count int64
		database.DB.Model(&database.NodeUserRelation{}).
			Where("node_id = ? AND user_id = ?", node.ID, user.ID).
			Count(&count)
		if count > 0 {
			continue
		}

		// 创建关联
		relation := database.NodeUserRelation{
			NodeID: node.ID,
			UserID: user.ID,
			UUID:   user.UUID,
			Flow:   node.Flow,
		}
		database.DB.Create(&relation)
	}
}

// handleUpdateNode 更新节点
func (s *Server) handleUpdateNode(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		errorJSON(c, http.StatusBadRequest, "无效的节点ID")
		return
	}

	var node database.InboundNode
	if err := database.DB.First(&node, id).Error; err != nil {
		errorJSON(c, http.StatusNotFound, "节点不存在")
		return
	}

	var req UpdateNodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errorJSON(c, http.StatusBadRequest, "请求参数错误")
		return
	}

	// 如果有新 Tag，检查是否重复
	if req.Tag != "" && req.Tag != node.Tag {
		var count int64
		database.DB.Model(&database.InboundNode{}).Where("tag = ? AND id != ?", req.Tag, id).Count(&count)
		if count > 0 {
			errorJSON(c, http.StatusBadRequest, "节点 Tag 已存在")
			return
		}
		node.Tag = req.Tag
	}

	// 如果有新端口，检查是否重复
	if req.Port > 0 && req.Port != node.Port {
		var count int64
		database.DB.Model(&database.InboundNode{}).Where("port = ? AND id != ?", req.Port, id).Count(&count)
		if count > 0 {
			errorJSON(c, http.StatusBadRequest, "端口已被使用")
			return
		}
		node.Port = req.Port
	}

	// 更新字段
	if req.Protocol != "" {
		node.Protocol = req.Protocol
	}
	if req.Listen != "" {
		node.Listen = req.Listen
	}
	node.TlsEnabled = req.TlsEnabled
	if req.ServerName != "" {
		node.ServerName = req.ServerName
	}
	if req.CertPath != "" {
		node.CertPath = req.CertPath
	}
	if req.KeyPath != "" {
		node.KeyPath = req.KeyPath
	}
	node.RealityEnabled = req.RealityEnabled
	node.RealityServer = req.RealityServer
	node.RealityPubkey = req.RealityPubkey
	node.RealityPrivkey = req.RealityPrivkey
	// 如果启用 Reality 且 short_id 为空，自动生成随机 short_id
	if req.RealityEnabled && req.RealityShortId == "" && node.RealityShortId == "" {
		node.RealityShortId = generateRandomShortId()
	} else {
		node.RealityShortId = req.RealityShortId
	}
	node.TransportEnabled = req.TransportEnabled
	node.TransportType = req.TransportType
	node.WsPath = req.WsPath
	node.GrpcService = req.GrpcService
	node.TransportHost = req.TransportHost
	node.Flow = req.Flow
	node.SsMethod = req.SsMethod
	node.SsPassword = req.SsPassword
	node.Hy2Password = req.Hy2Password
	node.Hy2UpMbps = req.Hy2UpMbps
	node.Hy2DownMbps = req.Hy2DownMbps
	node.Hy2Obfs = req.Hy2Obfs
	node.Hy2ObfsPassword = req.Hy2ObfsPassword
	node.Enabled = req.Enabled
	node.Notes = req.Notes

	if err := database.DB.Save(&node).Error; err != nil {
		errorJSON(c, http.StatusInternalServerError, "保存失败")
		return
	}

	// 同步更新关联表中的 flow 字段
	database.DB.Model(&database.NodeUserRelation{}).
		Where("node_id = ?", node.ID).
		Update("flow", node.Flow)

	// 广播配置更新到所有在线 Agent
	go agentHub.BroadcastConfigUpdate()

	successDataMsgJSON(c, "节点更新成功", node)
}

// handleDeleteNode 删除节点
func (s *Server) handleDeleteNode(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		errorJSON(c, http.StatusBadRequest, "无效的节点ID")
		return
	}

	// 删除节点的用户关联
	database.DB.Unscoped().Where("node_id = ?", id).Delete(&database.NodeUserRelation{})

	// 删除节点的服务器配置
	database.DB.Unscoped().Where("node_id = ?", id).Delete(&database.ServerNodeConfig{})

	result := database.DB.Unscoped().Delete(&database.InboundNode{}, id)
	if result.Error != nil {
		errorJSON(c, http.StatusInternalServerError, "删除失败")
		return
	}

	if result.RowsAffected == 0 {
		errorJSON(c, http.StatusNotFound, "节点不存在")
		return
	}

	// 广播配置更新到所有在线 Agent
	go agentHub.BroadcastConfigUpdate()

	successMsgJSON(c, "节点删除成功")
}

// handleGenerateRealityKeys 生成 Reality 密钥对
func (s *Server) handleGenerateRealityKeys(c *gin.Context) {
	// 优先使用 sing-box 命令生成
	privkey, pubkey, err := generateRealityKeysViaSingBox()
	if err != nil {
		// 如果 sing-box 不可用，使用 Go 原生生成
		privkey, pubkey, err = generateX25519KeyPair()
		if err != nil {
			errorJSON(c, http.StatusInternalServerError, "生成密钥对失败")
			return
		}
	}

	// 生成 short_id (8字节随机十六进制)
	shortIdBytes := make([]byte, 4)
	if _, err := rand.Read(shortIdBytes); err != nil {
		errorJSON(c, http.StatusInternalServerError, "生成 short_id 失败")
		return
	}
	shortId := hex.EncodeToString(shortIdBytes)

	successJSON(c, gin.H{
		"private_key": privkey,
		"public_key":  pubkey,
		"short_id":    shortId,
	})
}

// generateRealityKeysViaSingBox 使用 sing-box 生成 Reality 密钥对
func generateRealityKeysViaSingBox() (privkey, pubkey string, err error) {
	cmd := exec.Command("sing-box", "generate", "reality-keypair")
	output, err := cmd.Output()
	if err != nil {
		return "", "", err
	}

	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "PrivateKey:") {
			privkey = strings.TrimSpace(strings.TrimPrefix(line, "PrivateKey:"))
		} else if strings.HasPrefix(line, "PublicKey:") {
			pubkey = strings.TrimSpace(strings.TrimPrefix(line, "PublicKey:"))
		}
	}

	if privkey == "" || pubkey == "" {
		return "", "", exec.ErrNotFound
	}

	return privkey, pubkey, nil
}

// generateX25519KeyPair 生成 X25519 密钥对
func generateX25519KeyPair() (privateKeyBase64, publicKeyBase64 string, err error) {
	// 生成私钥 (32 字节随机数)
	privateKey := make([]byte, 32)
	if _, err := rand.Read(privateKey); err != nil {
		return "", "", err
	}

	// 计算公钥
	var publicKey [32]byte
	var privateKeyArray [32]byte
	copy(privateKeyArray[:], privateKey)
	curve25519.ScalarBaseMult(&publicKey, &privateKeyArray)

	// Base64 编码
	privateKeyBase64 = base64.RawURLEncoding.EncodeToString(privateKey)
	publicKeyBase64 = base64.RawURLEncoding.EncodeToString(publicKey[:])

	return privateKeyBase64, publicKeyBase64, nil
}

// handleBatchDeleteNodes 批量删除节点
func (s *Server) handleBatchDeleteNodes(c *gin.Context) {
	var req struct {
		IDs []uint `json:"ids" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		errorJSON(c, http.StatusBadRequest, "请求参数错误")
		return
	}

	// 删除节点的用户关联
	database.DB.Unscoped().Where("node_id IN ?", req.IDs).Delete(&database.NodeUserRelation{})

	// 删除节点的服务器配置
	database.DB.Unscoped().Where("node_id IN ?", req.IDs).Delete(&database.ServerNodeConfig{})

	result := database.DB.Unscoped().Delete(&database.InboundNode{}, req.IDs)
	if result.Error != nil {
		errorJSON(c, http.StatusInternalServerError, "删除失败")
		return
	}

	successMsgJSON(c, "批量删除成功，共删除 "+strconv.FormatInt(result.RowsAffected, 10)+" 个节点")
}

// generateRandomShortId 生成随机 Reality short_id (4-8字节16进制字符串)
func generateRandomShortId() string {
	// 随机生成 4-8 字节
	lengthByte := make([]byte, 1)
	rand.Read(lengthByte)
	length := 4 + int(lengthByte[0])%5 // 4, 5, 6, 7, 或 8 字节
	bytes := make([]byte, length)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)
}
