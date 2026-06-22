package handler

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sboard-go/sboard/internal/database"
)

// ========== 配置生成 API ==========

// handleGetNodeConfig 获取节点配置（实时生成）
func (s *Server) handleGetNodeConfig(c *gin.Context) {
	configType := c.Query("type")
	if configType == "" {
		configType = "sing-box"
	}

	// 实时生成配置
	config, err := generateGlobalConfig(configType)
	if err != nil {
		errorJSON(c, http.StatusInternalServerError, "生成配置失败: "+err.Error())
		return
	}

	successJSON(c, gin.H{
		"type":   configType,
		"config": config,
	})
}

// handleGetServerConfig 获取服务器配置（实时生成）
func (s *Server) handleGetServerConfig(c *gin.Context) {
	serverID := c.Param("id")
	configType := c.Query("type")
	if configType == "" {
		configType = "sing-box"
	}

	var server database.Server
	if err := database.DB.First(&server, serverID).Error; err != nil {
		errorJSON(c, http.StatusNotFound, "服务器不存在")
		return
	}

	// 生成该服务器的配置
	config, err := GenerateServerConfig(&server, configType)
	if err != nil {
		errorJSON(c, http.StatusInternalServerError, "生成配置失败: "+err.Error())
		return
	}

	successJSON(c, gin.H{
		"type":   configType,
		"config": config,
	})
}

// ========== 配置生成核心逻辑 ==========

// generateGlobalConfig 生成全局配置（所有启用的节点和用户）
func generateGlobalConfig(configType string) (string, error) {
	switch configType {
	case "sing-box", "singbox":
		return generateGlobalSingBoxConfig()
	default:
		return "", fmt.Errorf("不支持的配置类型: %s", configType)
	}
}

// generateGlobalSingBoxConfig 生成全局 sing-box 配置
func generateGlobalSingBoxConfig() (string, error) {
	// 获取所有启用的节点
	var nodes []database.InboundNode
	database.DB.Where("enabled = ?", true).Order("id ASC").Find(&nodes)

	inbounds := []*OrderedMap{}

	for _, node := range nodes {
		users := getNodeUsers(node.ID)
		if len(users) == 0 {
			continue
		}

		inbound := buildSingBoxInbound(&node, users)
		if inbound != nil {
			inbounds = append(inbounds, inbound)
		}
	}

	config := NewOrderedMap()
	config.Set("log", map[string]interface{}{"disabled": true})
	config.Set("inbounds", inbounds)
	config.Set("outbounds", []interface{}{
		map[string]interface{}{"type": "direct", "tag": "direct"},
	})

	jsonData, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return "", err
	}

	return string(jsonData), nil
}

// ========== 有序 Map 工具 ==========

// OrderedMap 有序的键值对
type OrderedMap struct {
	keys   []string
	values map[string]interface{}
}

func NewOrderedMap() *OrderedMap {
	return &OrderedMap{keys: []string{}, values: map[string]interface{}{}}
}

func (o *OrderedMap) Set(key string, value interface{}) {
	if _, exists := o.values[key]; !exists {
		o.keys = append(o.keys, key)
	}
	o.values[key] = value
}

func (o *OrderedMap) MarshalJSON() ([]byte, error) {
	var buf strings.Builder
	buf.WriteString("{")
	for i, key := range o.keys {
		if i > 0 {
			buf.WriteString(",")
		}
		keyJSON, _ := json.Marshal(key)
		buf.Write(keyJSON)
		buf.WriteString(":")
		valJSON, _ := json.Marshal(o.values[key])
		buf.Write(valJSON)
	}
	buf.WriteString("}")
	return []byte(buf.String()), nil
}

// ========== sing-box 入站构建 ==========

// buildSingBoxInbound 构建 sing-box 入站配置 (有序字段)
func buildSingBoxInbound(node *database.InboundNode, users []NodeUser) *OrderedMap {
	inbound := NewOrderedMap()

	// 固定顺序: type, tag, listen, listen_port
	inbound.Set("type", node.Protocol)
	inbound.Set("tag", node.Tag)
	inbound.Set("listen", node.Listen)
	inbound.Set("listen_port", node.Port)

	// 构建用户列表
	userList := []map[string]interface{}{}
	for _, user := range users {
		userEntry := map[string]interface{}{"name": user.Name}

		switch node.Protocol {
		case "trojan":
			userEntry["password"] = user.UUID
		case "vless":
			userEntry["uuid"] = user.UUID
			if user.Flow != "" && (node.RealityEnabled || (node.TlsEnabled && !node.TransportEnabled)) {
				userEntry["flow"] = user.Flow
			}
		case "vmess":
			userEntry["uuid"] = user.UUID
			userEntry["alterId"] = 0
		case "anytls":
			userEntry["password"] = user.UUID
		case "shadowsocks":
			// Shadowsocks 用户密钥 = 单用户密钥（sing-box 的 password 字段只取用户密钥，
			// 主密钥已在 inbound 级 password 字段设置）
			userEntry["password"] = generateSS2022UserKey(user.UUID, node.SsMethod)
		case "hysteria2":
			if node.Hy2Password != "" {
				userEntry["password"] = node.Hy2Password
			} else {
				userEntry["password"] = user.UUID
			}
		case "naive":
			userEntry["username"] = user.Name
			userEntry["password"] = user.UUID
		}
		userList = append(userList, userEntry)
	}

	// 协议特殊处理
	switch node.Protocol {
	case "shadowsocks":
		method := node.SsMethod
		if method == "" {
			method = "aes-256-gcm"
		}
		inbound.Set("method", method)
		inbound.Set("password", node.SsPassword)
		if strings.HasPrefix(method, "2022-") {
			inbound.Set("users", userList)
		}
		if node.SsObfsMode == "tls" || node.SsObfsMode == "http" {
			inbound.Set("obfs_mode", node.SsObfsMode)
			inbound.Set("obfs_host", node.SsObfsHost)
		}
	case "hysteria2":
		inbound.Set("users", userList)
		inbound.Set("up_mbps", node.Hy2UpMbps)
		inbound.Set("down_mbps", node.Hy2DownMbps)
		if node.Hy2Obfs == "salamander" && node.Hy2ObfsPassword != "" {
			inbound.Set("obfs", map[string]interface{}{
				"type":     "salamander",
				"password": node.Hy2ObfsPassword,
			})
		}
	case "naive":
		// Naive 用户列表格式不同，只需 username 和 password
		naiveUsers := []map[string]interface{}{}
		for _, user := range users {
			naiveUsers = append(naiveUsers, map[string]interface{}{
				"username": user.Name,
				"password": user.UUID,
			})
		}
		inbound.Set("users", naiveUsers)
	default:
		inbound.Set("users", userList)
	}

	// TLS 配置
	// hysteria2/naive 协议必须启用 TLS（hysteria2 强制要求 TLS 加密）
	if node.TlsEnabled && node.Protocol != "shadowsocks" || node.Protocol == "naive" || node.Protocol == "hysteria2" {
		tls := map[string]interface{}{
			"enabled":     true,
			"server_name": node.ServerName,
		}

		if node.RealityEnabled && node.RealityPrivkey != "" {
			// Reality 配置（Reality 通常与传输层互斥，但由前端控制）
			realityHandshake := map[string]interface{}{
				"server":      node.RealityServer,
				"server_port": 443,
			}
			reality := map[string]interface{}{
				"enabled":     true,
				"handshake":   realityHandshake,
				"private_key": node.RealityPrivkey,
			}
			if node.RealityShortId != "" {
				reality["short_id"] = []string{node.RealityShortId}
			}
			tls["reality"] = reality
		} else {
			// 普通 TLS 证书配置
			tls["certificate_path"] = "./server.crt"
			tls["key_path"] = "./server.key"
		}

		inbound.Set("tls", tls)
	}

	// 传输层配置（可与 TLS 共存，但通常与 Reality 互斥）
	if node.TransportEnabled {
		transport := map[string]interface{}{}

		switch node.TransportType {
		case "ws":
			transport["type"] = "ws"
			transport["path"] = node.WsPath
		case "grpc":
			transport["type"] = "grpc"
			transport["service_name"] = node.GrpcService
		case "http":
			transport["type"] = "http"
			transport["path"] = node.WsPath
		case "httpupgrade":
			transport["type"] = "httpupgrade"
			transport["path"] = node.WsPath
		}

		if len(transport) > 0 {
			inbound.Set("transport", transport)
		}
	}

	return inbound
}

// ========== 节点用户获取 ==========

// NodeUser 用户信息
type NodeUser struct {
	Name  string
	UUID  string
	Flow  string
	Level int
}

// getNodeUsers 获取节点关联的用户
func getNodeUsers(nodeID uint) []NodeUser {
	var relations []database.NodeUserRelation
	database.DB.Where("node_id = ?", nodeID).Find(&relations)

	userIDs := make([]uint, len(relations))
	relationMap := make(map[uint]database.NodeUserRelation)
	for i, rel := range relations {
		userIDs[i] = rel.UserID
		relationMap[rel.UserID] = rel
	}

	if len(userIDs) == 0 {
		return nil
	}

	var dbUsers []database.ProxyUser
	database.DB.Where("id IN ? AND enabled = ?", userIDs, 1).Find(&dbUsers)

	today := time.Now().Format("2006-01-02")
	users := []NodeUser{}
	for _, user := range dbUsers {
		rel := relationMap[user.ID]
		if user.ExpiryDate != "" && user.ExpiryDate < today {
			continue
		}
		users = append(users, NodeUser{
			Name:  user.Name,
			UUID:  rel.UUID,
			Flow:  rel.Flow,
			Level: user.Level,
		})
	}

	return users
}

// ========== 服务器配置生成 ==========

// GenerateServerConfig 为特定服务器生成配置
func GenerateServerConfig(server *database.Server, configType string) (string, error) {
	var nodes []database.InboundNode
	database.DB.Where("enabled = ?", true).Order("id ASC").Find(&nodes)

	var allUsers []database.ProxyUser
	today := time.Now().Format("2006-01-02")
	database.DB.Where("enabled = ? AND expiry_date >= ?", 1, today).Find(&allUsers)

	// 获取服务器的所有启用的落地出站（按槽位排序）
	var serverOutbounds []database.ServerOutbound
	database.DB.Where("server_id = ? AND enabled = ?", server.ID, true).Order("slot ASC").Find(&serverOutbounds)

	switch configType {
	case "sing-box", "singbox":
		return generateServerSingBoxConfig(nodes, allUsers, serverOutbounds)
	default:
		return "", fmt.Errorf("不支持的配置类型: %s", configType)
	}
}

// generateServerSingBoxConfig 为服务器生成 sing-box 配置（新版：支持多落地出站）
func generateServerSingBoxConfig(nodes []database.InboundNode, allUsers []database.ProxyUser, serverOutbounds []database.ServerOutbound) (string, error) {
	inbounds := []*OrderedMap{}
	outbounds := []interface{}{
		map[string]interface{}{"type": "direct", "tag": "direct"},
	}
	routeRules := []interface{}{}

	// 构建落地出站配置（key 为槽位）
	outboundSlotMap := make(map[int]*database.ServerOutbound)
	for i := range serverOutbounds {
		outboundSlotMap[serverOutbounds[i].Slot] = &serverOutbounds[i]
	}

	// 添加所有启用的落地出站到 outbounds
	for _, ob := range serverOutbounds {
		outbound := buildOutboundFromServerOutbound(&ob)
		if outbound != nil {
			outbounds = append(outbounds, outbound)
		}
	}

	// 获取已配置的落地出站槽位列表
	configuredSlots := make([]int, 0, len(serverOutbounds))
	for _, ob := range serverOutbounds {
		configuredSlots = append(configuredSlots, ob.Slot)
	}

	// 预加载用户 ID → User 映射（启用且未过期）
	userMap := make(map[uint]*database.ProxyUser)
	for i := range allUsers {
		if allUsers[i].Enabled == 1 {
			userMap[allUsers[i].ID] = &allUsers[i]
		}
	}

	// 批量加载所有节点的用户关系（一次查询，消除 N+1）
	nodeIDs := make([]uint, len(nodes))
	for i, node := range nodes {
		nodeIDs[i] = node.ID
	}
	var allRelations []database.NodeUserRelation
	if len(nodeIDs) > 0 {
		database.DB.Where("node_id IN ?", nodeIDs).Find(&allRelations)
	}
	// 按 nodeID 分组
	nodeRelationsMap := make(map[uint][]database.NodeUserRelation)
	for _, rel := range allRelations {
		nodeRelationsMap[rel.NodeID] = append(nodeRelationsMap[rel.NodeID], rel)
	}

	for _, node := range nodes {
		relations := nodeRelationsMap[node.ID]
		if len(relations) == 0 {
			continue
		}

		// 过滤出有对应用户的关系
		var nodeUserRelations []NodeUserRelation
		for _, rel := range relations {
			if user, exists := userMap[rel.UserID]; exists {
				nodeUserRelations = append(nodeUserRelations, NodeUserRelation{
					UserID: rel.UserID,
					UUID:   rel.UUID,
					Flow:   rel.Flow,
				})
				_ = user
			}
		}
		if len(nodeUserRelations) == 0 {
			continue
		}

		// 构建入站用户列表（包含主 UUID + 配置了落地出站的额外 UUID）
		inbound := buildSingBoxInboundWithExtraUUIDs(&node, nodeUserRelations, userMap, configuredSlots)
		if inbound != nil {
			inbounds = append(inbounds, inbound)
		}

		// 生成路由规则：额外 UUID 使用对应槽位的落地出站
		for _, rel := range nodeUserRelations {
			user := userMap[rel.UserID]
			if user == nil {
				continue
			}

			// 只为已配置的槽位创建路由规则
			for _, slot := range configuredSlots {
				extraUUID := user.GetExtraUUID(slot)
				if extraUUID == "" {
					continue
				}

				// 路由规则：指定入站 + 指定用户（额外 UUID）→ 对应的落地出站
				if ob, exists := outboundSlotMap[slot]; exists {
					routeRules = append(routeRules, map[string]interface{}{
						"inbound":   []string{node.Tag},
						"auth_user": []string{fmt.Sprintf("%s-%d", user.Name, slot)},
						"outbound":  ob.Remark,
					})
				}
			}
		}
	}

	config := NewOrderedMap()
	config.Set("log", map[string]interface{}{"disabled": true})
	config.Set("inbounds", inbounds)
	config.Set("outbounds", outbounds)

	// 添加路由配置
	if len(routeRules) > 0 {
		route := map[string]interface{}{
			"rules": routeRules,
			"final": "direct",
		}
		config.Set("route", route)
	}

	jsonData, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return "", err
	}

	return string(jsonData), nil
}

// NodeUserRelation 用于配置生成的用户关系
type NodeUserRelation struct {
	UserID uint
	UUID   string
	Flow   string
}

// buildSingBoxInboundWithExtraUUIDs 构建包含额外 UUID 的入站配置
// configuredSlots: 已配置落地出站的槽位列表，只添加这些槽位对应的额外 UUID
func buildSingBoxInboundWithExtraUUIDs(node *database.InboundNode, relations []NodeUserRelation, userMap map[uint]*database.ProxyUser, configuredSlots []int) *OrderedMap {
	// 从预加载的 userMap 中获取用户（避免重复查询 DB）
	relMap := make(map[uint]NodeUserRelation)
	var users []database.ProxyUser
	for _, rel := range relations {
		relMap[rel.UserID] = rel
		if u, ok := userMap[rel.UserID]; ok {
			users = append(users, *u)
		}
	}

	if len(users) == 0 {
		return nil
	}

	// 构建已配置槽位的 map，方便查找
	slotSet := make(map[int]bool)
	for _, slot := range configuredSlots {
		slotSet[slot] = true
	}

	inbound := NewOrderedMap()

	// 固定顺序: type, tag, listen, listen_port
	inbound.Set("type", node.Protocol)
	inbound.Set("tag", node.Tag)
	inbound.Set("listen", node.Listen)
	inbound.Set("listen_port", node.Port)

	// 构建用户列表（包含主 UUID + 已配置槽位的额外 UUID）
	userList := []map[string]interface{}{}
	for _, user := range users {
		rel := relMap[user.ID]

		// 添加主 UUID 用户
		mainEntry := buildUserEntry(node.Protocol, user.Name, rel.UUID, rel.Flow, node)
		userList = append(userList, mainEntry)

		// 只添加已配置槽位对应的额外 UUID 用户
		for _, slot := range configuredSlots {
			extraUUID := user.GetExtraUUID(slot)
			if extraUUID == "" {
				continue
			}
			extraName := fmt.Sprintf("%s-%d", user.Name, slot)
			extraEntry := buildUserEntry(node.Protocol, extraName, extraUUID, rel.Flow, node)
			userList = append(userList, extraEntry)
		}
	}

	// 协议特殊处理
	switch node.Protocol {
	case "shadowsocks":
		method := node.SsMethod
		if method == "" {
			method = "aes-256-gcm"
		}
		inbound.Set("method", method)
		inbound.Set("password", node.SsPassword)
		if strings.HasPrefix(method, "2022-") {
			inbound.Set("users", userList)
		}
		if node.SsObfsMode == "tls" || node.SsObfsMode == "http" {
			inbound.Set("obfs_mode", node.SsObfsMode)
			inbound.Set("obfs_host", node.SsObfsHost)
		}
	case "hysteria2":
		inbound.Set("users", userList)
		inbound.Set("up_mbps", node.Hy2UpMbps)
		inbound.Set("down_mbps", node.Hy2DownMbps)
		if node.Hy2Obfs == "salamander" && node.Hy2ObfsPassword != "" {
			inbound.Set("obfs", map[string]interface{}{
				"type":     "salamander",
				"password": node.Hy2ObfsPassword,
			})
		}
	case "naive":
		// Naive 用户列表格式不同
		naiveUsers := []map[string]interface{}{}
		for _, user := range users {
			// 主用户
			naiveUsers = append(naiveUsers, map[string]interface{}{
				"username": user.Name,
				"password": user.UUID,
			})
			// 只添加已配置槽位对应的额外用户
			for _, slot := range configuredSlots {
				extraUUID := user.GetExtraUUID(slot)
				if extraUUID == "" {
					continue
				}
				naiveUsers = append(naiveUsers, map[string]interface{}{
					"username": fmt.Sprintf("%s-%d", user.Name, slot),
					"password": extraUUID,
				})
			}
		}
		inbound.Set("users", naiveUsers)
	default:
		inbound.Set("users", userList)
	}

	// TLS 配置
	// hysteria2/naive 协议必须启用 TLS（hysteria2 强制要求 TLS 加密）
	if node.TlsEnabled && node.Protocol != "shadowsocks" || node.Protocol == "naive" || node.Protocol == "hysteria2" {
		tls := map[string]interface{}{
			"enabled":     true,
			"server_name": node.ServerName,
		}

		if node.RealityEnabled && node.RealityPrivkey != "" {
			realityHandshake := map[string]interface{}{
				"server":      node.RealityServer,
				"server_port": 443,
			}
			reality := map[string]interface{}{
				"enabled":     true,
				"handshake":   realityHandshake,
				"private_key": node.RealityPrivkey,
			}
			if node.RealityShortId != "" {
				reality["short_id"] = []string{node.RealityShortId}
			}
			tls["reality"] = reality
		} else {
			tls["certificate_path"] = "./server.crt"
			tls["key_path"] = "./server.key"
		}

		inbound.Set("tls", tls)
	}

	// 传输层配置
	if node.TransportEnabled {
		transport := map[string]interface{}{}

		switch node.TransportType {
		case "ws":
			transport["type"] = "ws"
			transport["path"] = node.WsPath
		case "grpc":
			transport["type"] = "grpc"
			transport["service_name"] = node.GrpcService
		case "http":
			transport["type"] = "http"
			transport["path"] = node.WsPath
		case "httpupgrade":
			transport["type"] = "httpupgrade"
			transport["path"] = node.WsPath
		}

		if len(transport) > 0 {
			inbound.Set("transport", transport)
		}
	}

	return inbound
}

// buildUserEntry 构建用户条目
func buildUserEntry(protocol, name, uuid, flow string, node *database.InboundNode) map[string]interface{} {
	userEntry := map[string]interface{}{"name": name}

	switch protocol {
	case "trojan":
		userEntry["password"] = uuid
	case "vless":
		userEntry["uuid"] = uuid
		if flow != "" && (node.RealityEnabled || (node.TlsEnabled && !node.TransportEnabled)) {
			userEntry["flow"] = flow
		}
	case "vmess":
		userEntry["uuid"] = uuid
		userEntry["alterId"] = 0
	case "anytls":
		userEntry["password"] = uuid
	case "shadowsocks":
		// Shadowsocks 用户密钥 = 单用户密钥（同第一处）
		userEntry["password"] = generateSS2022UserKey(uuid, node.SsMethod)
	case "hysteria2":
		if node.Hy2Password != "" {
			userEntry["password"] = node.Hy2Password
		} else {
			userEntry["password"] = uuid
		}
	}

	return userEntry
}

// generateSS2022UserKey 为 Shadowsocks 2022 生成用户密钥
// 根据加密方法生成正确长度的密钥（16 或 32 字节）
func generateSS2022UserKey(uuid string, method string) string {
	// 使用 UUID 的 SHA256 哈希作为密钥来源
	hash := sha256.Sum256([]byte(uuid))

	var keyLen int
	switch method {
	case "2022-blake3-aes-128-gcm":
		keyLen = 16
	case "2022-blake3-aes-256-gcm", "2022-blake3-chacha20-poly1305":
		keyLen = 32
	default:
		// 非 2022 方法，直接返回 UUID
		return uuid
	}

	return base64.StdEncoding.EncodeToString(hash[:keyLen])
}

// buildOutboundFromServerOutbound 从 ServerOutbound 构建 sing-box 出站配置
func buildOutboundFromServerOutbound(ob *database.ServerOutbound) map[string]interface{} {
	outbound := map[string]interface{}{
		"tag":         ob.Remark, // 使用备注作为标签
		"server":      ob.Host,
		"server_port": ob.Port,
	}

	switch ob.Protocol {
	case "shadowsocks", "ss":
		outbound["type"] = "shadowsocks"
		outbound["method"] = ob.Method
		outbound["password"] = ob.Password

	case "trojan":
		outbound["type"] = "trojan"
		outbound["password"] = ob.Password
		tls := map[string]interface{}{
			"enabled":  true,
			"insecure": true,
		}
		if ob.Sni != "" {
			tls["server_name"] = ob.Sni
		}
		tls["utls"] = map[string]interface{}{
			"enabled":     true,
			"fingerprint": "chrome",
		}
		outbound["tls"] = tls

	case "anytls":
		outbound["type"] = "anytls"
		outbound["password"] = ob.Password
		tls := map[string]interface{}{
			"enabled":  true,
			"insecure": true,
		}
		if ob.Sni != "" {
			tls["server_name"] = ob.Sni
		}
		outbound["tls"] = tls

	case "socks5":
		outbound["type"] = "socks"
		if ob.Username != "" {
			outbound["username"] = ob.Username
			outbound["password"] = ob.Password
		}

	case "vless":
		outbound["type"] = "vless"
		outbound["uuid"] = ob.UUID
		if ob.Flow != "" {
			outbound["flow"] = ob.Flow
		}
		if ob.Reality {
			tls := map[string]interface{}{
				"enabled": true,
				"reality": map[string]interface{}{
					"enabled":    true,
					"public_key": ob.PubKey,
					"short_id":   ob.ShortId,
				},
			}
			if ob.Sni != "" {
				tls["server_name"] = ob.Sni
			}
			utls := map[string]interface{}{"enabled": true}
			if ob.Fp != "" {
				utls["fingerprint"] = ob.Fp
			} else {
				utls["fingerprint"] = "chrome"
			}
			tls["utls"] = utls
			outbound["tls"] = tls
		} else if ob.Tls {
			tls := map[string]interface{}{
				"enabled":  true,
				"insecure": true,
			}
			if ob.Sni != "" {
				tls["server_name"] = ob.Sni
			}
			tls["utls"] = map[string]interface{}{
				"enabled":     true,
				"fingerprint": "chrome",
			}
			outbound["tls"] = tls
		}
		// 传输层配置
		if ob.Network != "" && ob.Network != "tcp" {
			transport := buildOutboundTransport(ob)
			if transport != nil {
				outbound["transport"] = transport
			}
		}

	case "vmess":
		outbound["type"] = "vmess"
		outbound["uuid"] = ob.UUID
		outbound["alter_id"] = ob.AlterId
		if ob.Security != "" {
			outbound["security"] = ob.Security
		} else {
			outbound["security"] = "auto"
		}
		if ob.Tls {
			tls := map[string]interface{}{
				"enabled":  true,
				"insecure": true,
			}
			if ob.Sni != "" {
				tls["server_name"] = ob.Sni
			}
			tls["utls"] = map[string]interface{}{
				"enabled":     true,
				"fingerprint": "chrome",
			}
			outbound["tls"] = tls
		}
		// 传输层配置
		if ob.Network != "" && ob.Network != "tcp" {
			transport := buildOutboundTransport(ob)
			if transport != nil {
				outbound["transport"] = transport
			}
		}

	case "hysteria2":
		outbound["type"] = "hysteria2"
		outbound["password"] = ob.Password
		tls := map[string]interface{}{
			"enabled":  true,
			"insecure": true,
		}
		if ob.Sni != "" {
			tls["server_name"] = ob.Sni
		}
		outbound["tls"] = tls
		if ob.Obfs != "" {
			outbound["obfs"] = map[string]interface{}{
				"type":     ob.Obfs,
				"password": ob.ObfsPwd,
			}
		}

	default:
		outbound["type"] = ob.Protocol
	}

	return outbound
}

// buildOutboundTransport 构建出站传输层配置
func buildOutboundTransport(ob *database.ServerOutbound) map[string]interface{} {
	transport := map[string]interface{}{
		"type": ob.Network,
	}

	switch ob.Network {
	case "ws":
		if ob.WsPath != "" {
			transport["path"] = ob.WsPath
		}
		if ob.WsHost != "" {
			transport["headers"] = map[string]interface{}{
				"Host": ob.WsHost,
			}
		}
	case "grpc":
		if ob.WsPath != "" {
			transport["service_name"] = ob.WsPath
		}
	case "http", "h2":
		transport["type"] = "http"
		if ob.WsHost != "" {
			transport["host"] = []string{ob.WsHost}
		}
		if ob.WsPath != "" {
			transport["path"] = ob.WsPath
		}
	}

	return transport
}
