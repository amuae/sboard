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
	SsObfsMode string `json:"ss_obfs_mode"`
	SsObfsHost string `json:"ss_obfs_host"`

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
	SsObfsMode string `json:"ss_obfs_mode"`
	SsObfsHost string `json:"ss_obfs_host"`

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

	query := database.GetDB().Order("id ASC")

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
	database.GetDB().Model(&database.InboundNode{}).Count(&total)

	var enabledCount int64
	database.GetDB().Model(&database.InboundNode{}).Where("enabled = ?", true).Count(&enabledCount)

	// 按协议统计
	type ProtocolStat struct {
		Protocol string `json:"protocol"`
		Count    int64  `json:"count"`
	}
	var protocolStats []ProtocolStat
	database.GetDB().Model(&database.InboundNode{}).Select("protocol, count(*) as count").Group("protocol").Scan(&protocolStats)

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
	if err := database.GetDB().First(&node, id).Error; err != nil {
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
	validProtocols := []string{"trojan", "vless", "vmess", "anytls", "shadowsocks", "hysteria2", "naive"}
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
	database.GetDB().Model(&database.InboundNode{}).Where("tag = ?", req.Tag).Count(&count)
	if count > 0 {
		errorJSON(c, http.StatusBadRequest, "节点 Tag 已存在")
		return
	}

	// 检查端口是否已存在
	database.GetDB().Model(&database.InboundNode{}).Where("port = ?", req.Port).Count(&count)
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

	// Shadowsocks 协议：如果密码为空，根据加密方法自动生成正确格式的密码
	if req.Protocol == "shadowsocks" && req.SsPassword == "" {
		if req.SsMethod == "" {
			req.SsMethod = "aes-256-gcm" // 默认方法
		}
		req.SsPassword = generateSS2022Password(req.SsMethod)
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
		SsObfsMode:       req.SsObfsMode,
		SsObfsHost:       req.SsObfsHost,
		Hy2Password:      req.Hy2Password,
		Hy2UpMbps:        req.Hy2UpMbps,
		Hy2DownMbps:      req.Hy2DownMbps,
		Hy2Obfs:          req.Hy2Obfs,
		Hy2ObfsPassword:  req.Hy2ObfsPassword,
		Enabled:          req.Enabled,
		Notes:            req.Notes,
	}

	if err := database.GetDB().Create(&node).Error; err != nil {
		errorJSON(c, http.StatusInternalServerError, "创建失败: "+err.Error())
		return
	}

	// 自动链接所有用户到该节点
	s.linkAllUsersToNode(&node)

	// 广播配置更新到所有在线 Agent
	go agentHub.BroadcastConfigUpdate()

	successDataMsgJSON(c, "节点创建成功", node)
}

// linkAllUsersToNode 将所有用户链接到节点（批量操作，避免 N+1 查询）
func (s *Server) linkAllUsersToNode(node *database.InboundNode) {
	var users []database.ProxyUser
	database.GetDB().Where("enabled = ?", 1).Find(&users)

	if len(users) == 0 {
		return
	}

	// 1. 构建用户 ID 列表
	userIDs := make([]uint, len(users))
	for i, u := range users {
		userIDs[i] = u.ID
	}

	// 2. 单次查询已存在的关联
	var existing []database.NodeUserRelation
	database.GetDB().Where("node_id = ? AND user_id IN ?", node.ID, userIDs).Find(&existing)
	existingSet := make(map[uint]bool, len(existing))
	for _, r := range existing {
		existingSet[r.UserID] = true
	}

	// 3. 批量插入新关联
	var newRelations []database.NodeUserRelation
	for _, user := range users {
		if existingSet[user.ID] {
			continue
		}
		newRelations = append(newRelations, database.NodeUserRelation{
			NodeID: node.ID,
			UserID: user.ID,
			UUID:   user.UUID,
			Flow:   node.Flow,
		})
	}
	if len(newRelations) > 0 {
		database.GetDB().Create(&newRelations)
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
	if err := database.GetDB().First(&node, id).Error; err != nil {
		errorJSON(c, http.StatusNotFound, "节点不存在")
		return
	}

	var req UpdateNodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errorJSON(c, http.StatusBadRequest, "请求参数错误")
		return
	}

	// 跟踪配置相关字段变化（Notes/CertPath/KeyPath 不参与配置生成，忽略）
	configChanged := false
	oldFlow := node.Flow // 保存旧值用于后续 flow 同步判断

	// 如果有新 Tag，检查是否重复
	if req.Tag != "" && req.Tag != node.Tag {
		var count int64
		database.GetDB().Model(&database.InboundNode{}).Where("tag = ? AND id != ?", req.Tag, id).Count(&count)
		if count > 0 {
			errorJSON(c, http.StatusBadRequest, "节点 Tag 已存在")
			return
		}
		configChanged = true
		node.Tag = req.Tag
	}

	// 如果有新端口，检查是否重复
	if req.Port > 0 && req.Port != node.Port {
		var count int64
		database.GetDB().Model(&database.InboundNode{}).Where("port = ? AND id != ?", req.Port, id).Count(&count)
		if count > 0 {
			errorJSON(c, http.StatusBadRequest, "端口已被使用")
			return
		}
		configChanged = true
		node.Port = req.Port
	}

	// 更新字段 — 仅在配置相关字段变化时记录
	if req.Protocol != "" {
		validProtocols := []string{"trojan", "vless", "vmess", "anytls", "shadowsocks", "hysteria2", "naive"}
		isValid := false
		for _, p := range validProtocols {
			if req.Protocol == p {
				isValid = true
				break
			}
		}
		if !isValid {
			errorJSON(c, http.StatusBadRequest, "无效的协议类型")
			return
		}
		if req.Protocol != node.Protocol {
			configChanged = true
		}
		node.Protocol = req.Protocol
	}
	if req.Listen != "" {
		if req.Listen != node.Listen {
			configChanged = true
		}
		node.Listen = req.Listen
	}

	if req.TlsEnabled != node.TlsEnabled {
		configChanged = true
	}
	node.TlsEnabled = req.TlsEnabled
	if req.ServerName != "" && req.ServerName != node.ServerName {
		configChanged = true
		node.ServerName = req.ServerName
	}
	// CertPath/KeyPath 不参与配置生成，不跟踪变化
	if req.CertPath != "" {
		node.CertPath = req.CertPath
	}
	if req.KeyPath != "" {
		node.KeyPath = req.KeyPath
	}

	if req.RealityEnabled != node.RealityEnabled {
		configChanged = true
	}
	node.RealityEnabled = req.RealityEnabled
	if req.RealityServer != node.RealityServer {
		configChanged = true
	}
	node.RealityServer = req.RealityServer
	if req.RealityPubkey != node.RealityPubkey {
		configChanged = true
	}
	node.RealityPubkey = req.RealityPubkey
	if req.RealityPrivkey != node.RealityPrivkey {
		configChanged = true
	}
	node.RealityPrivkey = req.RealityPrivkey
	newShortId := req.RealityShortId
	if req.RealityEnabled && req.RealityShortId == "" && node.RealityShortId == "" {
		newShortId = generateRandomShortId()
	}
	if newShortId != node.RealityShortId {
		configChanged = true
	}
	node.RealityShortId = newShortId

	if req.TransportEnabled != node.TransportEnabled {
		configChanged = true
	}
	node.TransportEnabled = req.TransportEnabled
	if req.TransportType != node.TransportType {
		configChanged = true
	}
	node.TransportType = req.TransportType
	if req.WsPath != node.WsPath {
		configChanged = true
	}
	node.WsPath = req.WsPath
	if req.GrpcService != node.GrpcService {
		configChanged = true
	}
	node.GrpcService = req.GrpcService
	if req.TransportHost != node.TransportHost {
		configChanged = true
	}
	node.TransportHost = req.TransportHost

	if req.Flow != oldFlow {
		configChanged = true
	}
	node.Flow = req.Flow
	if req.SsMethod != node.SsMethod {
		configChanged = true
	}
	node.SsMethod = req.SsMethod
	if req.SsPassword != node.SsPassword {
		configChanged = true
	}
	node.SsPassword = req.SsPassword
	if req.SsObfsMode != node.SsObfsMode {
		configChanged = true
	}
	node.SsObfsMode = req.SsObfsMode
	if req.SsObfsHost != node.SsObfsHost {
		configChanged = true
	}
	node.SsObfsHost = req.SsObfsHost
	if req.Hy2Password != node.Hy2Password {
		configChanged = true
	}
	node.Hy2Password = req.Hy2Password
	if req.Hy2UpMbps != node.Hy2UpMbps {
		configChanged = true
	}
	node.Hy2UpMbps = req.Hy2UpMbps
	if req.Hy2DownMbps != node.Hy2DownMbps {
		configChanged = true
	}
	node.Hy2DownMbps = req.Hy2DownMbps
	if req.Hy2Obfs != node.Hy2Obfs {
		configChanged = true
	}
	node.Hy2Obfs = req.Hy2Obfs
	if req.Hy2ObfsPassword != node.Hy2ObfsPassword {
		configChanged = true
	}
	node.Hy2ObfsPassword = req.Hy2ObfsPassword

	if req.Enabled != node.Enabled {
		configChanged = true
	}
	node.Enabled = req.Enabled
	// Notes 不参与配置生成，不跟踪
	node.Notes = req.Notes

	if err := database.GetDB().Save(&node).Error; err != nil {
		errorJSON(c, http.StatusInternalServerError, "保存失败")
		return
	}

	// 同步更新关联表中的 flow 字段（只在 flow 变化时）
	if req.Flow != "" && req.Flow != oldFlow {
		database.GetDB().Model(&database.NodeUserRelation{}).
			Where("node_id = ?", node.ID).
			Update("flow", node.Flow)
	}

	// 只在配置相关字段变化时广播
	if configChanged {
		go agentHub.BroadcastConfigUpdate()
	}

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
	database.GetDB().Unscoped().Where("node_id = ?", id).Delete(&database.NodeUserRelation{})

	// 删除节点的服务器配置
	database.GetDB().Unscoped().Where("node_id = ?", id).Delete(&database.ServerNodeConfig{})

	result := database.GetDB().Unscoped().Delete(&database.InboundNode{}, id)
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
	database.GetDB().Unscoped().Where("node_id IN ?", req.IDs).Delete(&database.NodeUserRelation{})

	// 删除节点的服务器配置
	database.GetDB().Unscoped().Where("node_id IN ?", req.IDs).Delete(&database.ServerNodeConfig{})

	result := database.GetDB().Unscoped().Delete(&database.InboundNode{}, req.IDs)
	if result.Error != nil {
		errorJSON(c, http.StatusInternalServerError, "删除失败")
		return
	}

	// 广播配置更新到所有在线 Agent
	go agentHub.BroadcastConfigUpdate()

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

// generateRandomPassword 生成随机密码（用于 Shadowsocks）
func generateRandomPassword(length int) string {
	const chars = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	bytes := make([]byte, length)
	rand.Read(bytes)
	for i := range bytes {
		bytes[i] = chars[int(bytes[i])%len(chars)]
	}
	return string(bytes)
}

// generateSS2022Password 生成 Shadowsocks 2022 格式的密码（base64 编码的随机字节）
func generateSS2022Password(method string) string {
	var keyLen int
	switch method {
	case "2022-blake3-aes-128-gcm":
		keyLen = 16
	case "2022-blake3-aes-256-gcm", "2022-blake3-chacha20-poly1305":
		keyLen = 32
	default:
		// 非 2022 方法，生成普通随机密码
		return generateRandomPassword(32)
	}

	bytes := make([]byte, keyLen)
	rand.Read(bytes)
	return base64.StdEncoding.EncodeToString(bytes)
}
