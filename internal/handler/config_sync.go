package handler

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sboard-go/sboard/internal/database"
	"gopkg.in/yaml.v3"
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
	case "mihomo", "clash":
		return generateGlobalMihomoConfig()
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

// generateGlobalMihomoConfig 生成全局 mihomo 配置
func generateGlobalMihomoConfig() (string, error) {
	// 获取所有启用的节点
	var nodes []database.InboundNode
	database.DB.Where("enabled = ?", true).Order("id ASC").Find(&nodes)

	listenersNode := &yaml.Node{Kind: yaml.SequenceNode}

	for _, node := range nodes {
		users := getNodeUsers(node.ID)
		if len(users) == 0 {
			continue
		}

		listener := buildMihomoListener(&node, users)
		if listener != nil {
			listenersNode.Content = append(listenersNode.Content, listener)
		}
	}

	config := yaml.Node{Kind: yaml.MappingNode}
	config.Content = append(config.Content,
		&yaml.Node{Kind: yaml.ScalarNode, Value: "log-level"},
		&yaml.Node{Kind: yaml.ScalarNode, Value: "silent"},
	)
	config.Content = append(config.Content,
		&yaml.Node{Kind: yaml.ScalarNode, Value: "listeners"},
		listenersNode,
	)

	var buf strings.Builder
	encoder := yaml.NewEncoder(&buf)
	encoder.SetIndent(2)
	if err := encoder.Encode(&config); err != nil {
		return "", err
	}
	encoder.Close()

	return buf.String(), nil
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
			// 2022-blake3-aes-256-gcm 需要 32 字节的密钥
			uuidBytes := []byte(user.UUID)
			keyBytes := make([]byte, 32)
			copy(keyBytes, uuidBytes)
			userEntry["password"] = base64.StdEncoding.EncodeToString(keyBytes)
		case "hysteria2":
			if node.Hy2Password != "" {
				userEntry["password"] = node.Hy2Password
			} else {
				userEntry["password"] = user.UUID
			}
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
	default:
		inbound.Set("users", userList)
	}

	// TLS 配置
	if node.TlsEnabled && node.Protocol != "shadowsocks" {
		tls := map[string]interface{}{
			"enabled":     true,
			"server_name": node.ServerName,
		}

		if node.RealityEnabled && node.RealityPrivkey != "" && !node.TransportEnabled {
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
			// 使用相对路径，核心和证书在同一目录
			tls["certificate_path"] = node.CertPath
			tls["key_path"] = node.KeyPath
		}

		inbound.Set("tls", tls)
	}

	// 传输层配置
	if node.TransportEnabled && !node.RealityEnabled {
		transport := map[string]interface{}{}

		switch node.TransportType {
		case "ws":
			transport["type"] = "ws"
			transport["path"] = node.WsPath
			// 服务端不需要验证 Host 头，由客户端订阅配置设置
		case "grpc":
			transport["type"] = "grpc"
			transport["service_name"] = node.GrpcService
		case "http":
			transport["type"] = "http"
			transport["path"] = node.WsPath
		case "httpupgrade":
			transport["type"] = "httpupgrade"
			transport["path"] = node.WsPath
			// 服务端不需要验证 Host 头
		}

		if len(transport) > 0 {
			inbound.Set("transport", transport)
		}
	}

	return inbound
}

// ========== mihomo listener 构建 ==========

// buildMihomoListener 构建 mihomo listener 配置 (有序 YAML)
func buildMihomoListener(node *database.InboundNode, users []NodeUser) *yaml.Node {
	result := &yaml.Node{Kind: yaml.MappingNode}

	// 基础字段
	addYamlField(result, "name", node.Tag)
	addYamlField(result, "type", node.Protocol)
	addYamlField(result, "port", node.Port)
	addYamlField(result, "listen", node.Listen)

	// 根据协议类型设置用户格式
	switch node.Protocol {
	case "anytls":
		usersNode := &yaml.Node{Kind: yaml.MappingNode}
		for _, user := range users {
			addYamlField(usersNode, user.Name, user.UUID)
		}
		addYamlNode(result, "users", usersNode)

	case "vless", "vmess", "trojan":
		usersNode := &yaml.Node{Kind: yaml.SequenceNode}
		for _, user := range users {
			userNode := &yaml.Node{Kind: yaml.MappingNode}
			addYamlField(userNode, "username", user.Name)
			switch node.Protocol {
			case "vless":
				addYamlField(userNode, "uuid", user.UUID)
				if user.Flow != "" {
					addYamlField(userNode, "flow", user.Flow)
				}
			case "vmess":
				addYamlField(userNode, "uuid", user.UUID)
			case "trojan":
				addYamlField(userNode, "password", user.UUID)
			}
			usersNode.Content = append(usersNode.Content, userNode)
		}
		addYamlNode(result, "users", usersNode)

	case "shadowsocks", "ss":
		method := node.SsMethod
		if method == "" {
			method = "aes-256-gcm"
		}
		addYamlField(result, "cipher", method)
		addYamlField(result, "password", node.SsPassword)

	case "hysteria2":
		usersNode := &yaml.Node{Kind: yaml.MappingNode}
		password := node.Hy2Password
		if password == "" && len(users) > 0 {
			password = users[0].UUID
		}
		for _, user := range users {
			addYamlField(usersNode, user.Name, password)
		}
		addYamlNode(result, "users", usersNode)
		if node.Hy2UpMbps > 0 {
			addYamlField(result, "up", fmt.Sprintf("%d Mbps", node.Hy2UpMbps))
		}
		if node.Hy2DownMbps > 0 {
			addYamlField(result, "down", fmt.Sprintf("%d Mbps", node.Hy2DownMbps))
		}
	}

	// TLS 配置
	if node.TlsEnabled && node.Protocol != "shadowsocks" {
		if node.RealityEnabled && node.RealityPrivkey != "" {
			// Reality 配置 (mihomo 使用 reality-config)
			realityConfig := &yaml.Node{Kind: yaml.MappingNode}
			if node.RealityServer != "" {
				addYamlField(realityConfig, "dest", node.RealityServer+":443")
			}
			addYamlField(realityConfig, "private-key", node.RealityPrivkey)
			if node.RealityShortId != "" {
				shortIdList := &yaml.Node{Kind: yaml.SequenceNode}
				shortIdList.Content = append(shortIdList.Content, &yaml.Node{Kind: yaml.ScalarNode, Value: node.RealityShortId})
				addYamlNode(realityConfig, "short-id", shortIdList)
			}
			if node.ServerName != "" {
				serverNames := &yaml.Node{Kind: yaml.SequenceNode}
				serverNames.Content = append(serverNames.Content, &yaml.Node{Kind: yaml.ScalarNode, Value: node.ServerName})
				addYamlNode(realityConfig, "server-names", serverNames)
			}
			addYamlNode(result, "reality-config", realityConfig)
		} else if node.CertPath != "" {
			// 普通 TLS 证书配置，使用相对路径
			addYamlField(result, "certificate", node.CertPath)
			addYamlField(result, "private-key", node.KeyPath)
		}
	}

	// 传输层
	if node.TransportEnabled {
		switch node.TransportType {
		case "ws":
			addYamlField(result, "ws-path", node.WsPath)
		case "grpc":
			addYamlField(result, "grpc-service-name", node.GrpcService)
		}
	}

	return result
}

// ========== YAML 辅助函数 ==========

func addYamlField(node *yaml.Node, key string, value interface{}) {
	keyNode := &yaml.Node{Kind: yaml.ScalarNode, Value: key}
	var valueNode *yaml.Node

	switch v := value.(type) {
	case string:
		valueNode = &yaml.Node{Kind: yaml.ScalarNode, Value: v}
	case int:
		valueNode = &yaml.Node{Kind: yaml.ScalarNode, Value: fmt.Sprintf("%d", v), Tag: "!!int"}
	case bool:
		valueNode = &yaml.Node{Kind: yaml.ScalarNode, Value: fmt.Sprintf("%v", v), Tag: "!!bool"}
	default:
		valueNode = &yaml.Node{Kind: yaml.ScalarNode, Value: fmt.Sprintf("%v", v)}
	}

	node.Content = append(node.Content, keyNode, valueNode)
}

func addYamlNode(parent *yaml.Node, key string, child *yaml.Node) {
	keyNode := &yaml.Node{Kind: yaml.ScalarNode, Value: key}
	parent.Content = append(parent.Content, keyNode, child)
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

	var nodeConfigs []database.ServerNodeConfig
	database.DB.Where("server_id = ?", server.ID).Preload("Node").Find(&nodeConfigs)
	nodeConfigMap := make(map[uint]*database.ServerNodeConfig)
	for i := range nodeConfigs {
		nodeConfigMap[nodeConfigs[i].NodeID] = &nodeConfigs[i]
	}

	switch configType {
	case "sing-box", "singbox":
		return generateServerSingBoxConfig(nodes, nodeConfigMap)
	case "mihomo", "clash":
		return generateServerMihomoConfig(nodes, nodeConfigMap)
	default:
		return "", fmt.Errorf("不支持的配置类型: %s", configType)
	}
}

// generateServerSingBoxConfig 为服务器生成 sing-box 配置
func generateServerSingBoxConfig(nodes []database.InboundNode, nodeConfigMap map[uint]*database.ServerNodeConfig) (string, error) {
	inbounds := []*OrderedMap{}
	outbounds := []interface{}{
		map[string]interface{}{"type": "direct", "tag": "direct"},
	}
	routeRules := []interface{}{}

	for _, node := range nodes {
		nodeUsers := getNodeUsers(node.ID)
		if len(nodeUsers) == 0 {
			continue
		}

		inbound := buildSingBoxInbound(&node, nodeUsers)
		if inbound != nil {
			// 检查是否有落地出站配置
			if nodeConfig, ok := nodeConfigMap[node.ID]; ok && nodeConfig.OutboundEnabled {
				outboundTag := fmt.Sprintf("outbound-%d", node.ID)
				// 添加路由规则：从该入站进来的流量走对应的出站
				routeRules = append(routeRules, map[string]interface{}{
					"inbound":  []string{node.Tag},
					"outbound": outboundTag,
				})

				// 构建出站配置
				ob := map[string]interface{}{
					"tag":         outboundTag,
					"server":      nodeConfig.OutboundHost,
					"server_port": nodeConfig.OutboundPort,
				}

				switch nodeConfig.OutboundProtocol {
				case "shadowsocks", "ss":
					ob["type"] = "shadowsocks"
					ob["method"] = nodeConfig.OutboundMethod
					ob["password"] = nodeConfig.OutboundPassword
				case "trojan":
					ob["type"] = "trojan"
					ob["password"] = nodeConfig.OutboundPassword
					tls := map[string]interface{}{
						"enabled":  true,
						"insecure": true,
					}
					if nodeConfig.OutboundSni != "" {
						tls["server_name"] = nodeConfig.OutboundSni
					}
					tls["utls"] = map[string]interface{}{
						"enabled":     true,
						"fingerprint": "chrome",
					}
					ob["tls"] = tls
				case "socks5":
					ob["type"] = "socks"
					if nodeConfig.OutboundUsername != "" {
						ob["username"] = nodeConfig.OutboundUsername
						ob["password"] = nodeConfig.OutboundPassword
					}
				case "vless":
					ob["type"] = "vless"
					ob["uuid"] = nodeConfig.OutboundUUID
					if nodeConfig.OutboundFlow != "" {
						ob["flow"] = nodeConfig.OutboundFlow
					}
					if nodeConfig.OutboundReality {
						tls := map[string]interface{}{
							"enabled": true,
							"reality": map[string]interface{}{
								"enabled":    true,
								"public_key": nodeConfig.OutboundPubKey,
								"short_id":   nodeConfig.OutboundShortId,
							},
						}
						if nodeConfig.OutboundSni != "" {
							tls["server_name"] = nodeConfig.OutboundSni
						}
						utls := map[string]interface{}{"enabled": true}
						if nodeConfig.OutboundFp != "" {
							utls["fingerprint"] = nodeConfig.OutboundFp
						} else {
							utls["fingerprint"] = "chrome"
						}
						tls["utls"] = utls
						ob["tls"] = tls
					} else if nodeConfig.OutboundTls {
						tls := map[string]interface{}{
							"enabled":  true,
							"insecure": true,
						}
						if nodeConfig.OutboundSni != "" {
							tls["server_name"] = nodeConfig.OutboundSni
						}
						tls["utls"] = map[string]interface{}{
							"enabled":     true,
							"fingerprint": "chrome",
						}
						ob["tls"] = tls
					}
				case "vmess":
					ob["type"] = "vmess"
					ob["uuid"] = nodeConfig.OutboundUUID
					ob["alter_id"] = nodeConfig.OutboundAlterId
					if nodeConfig.OutboundSecurity != "" {
						ob["security"] = nodeConfig.OutboundSecurity
					} else {
						ob["security"] = "auto"
					}
					if nodeConfig.OutboundTls {
						tls := map[string]interface{}{
							"enabled":  true,
							"insecure": true,
						}
						if nodeConfig.OutboundSni != "" {
							tls["server_name"] = nodeConfig.OutboundSni
						}
						tls["utls"] = map[string]interface{}{
							"enabled":     true,
							"fingerprint": "chrome",
						}
						ob["tls"] = tls
					}
				default:
					ob["type"] = nodeConfig.OutboundProtocol
				}

				outbounds = append(outbounds, ob)
			}

			inbounds = append(inbounds, inbound)
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

// generateServerMihomoConfig 为服务器生成 mihomo 配置
func generateServerMihomoConfig(nodes []database.InboundNode, nodeConfigMap map[uint]*database.ServerNodeConfig) (string, error) {
	listenersNode := &yaml.Node{Kind: yaml.SequenceNode}
	proxiesNode := &yaml.Node{Kind: yaml.SequenceNode}
	hasProxies := false

	for _, node := range nodes {
		nodeUsers := getNodeUsers(node.ID)
		if len(nodeUsers) == 0 {
			continue
		}

		listener := buildMihomoListener(&node, nodeUsers)
		if listener != nil {
			// 检查是否有落地出站配置
			if nodeConfig, ok := nodeConfigMap[node.ID]; ok && nodeConfig.OutboundEnabled {
				outboundTag := fmt.Sprintf("outbound-%d", node.ID)
				// 设置 listener 的 proxy 为出站标签
				setMihomoListenerProxy(listener, outboundTag)

				// 构建出站代理
				proxy := buildMihomoOutboundProxy(nodeConfig, outboundTag)
				if proxy != nil {
					proxiesNode.Content = append(proxiesNode.Content, proxy)
					hasProxies = true
				}
			} else {
				// 默认直连
				setMihomoListenerProxy(listener, "DIRECT")
			}
			listenersNode.Content = append(listenersNode.Content, listener)
		}
	}

	config := yaml.Node{Kind: yaml.MappingNode}
	config.Content = append(config.Content,
		&yaml.Node{Kind: yaml.ScalarNode, Value: "log-level"},
		&yaml.Node{Kind: yaml.ScalarNode, Value: "silent"},
	)
	config.Content = append(config.Content,
		&yaml.Node{Kind: yaml.ScalarNode, Value: "listeners"},
		listenersNode,
	)
	if hasProxies {
		config.Content = append(config.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Value: "proxies"},
			proxiesNode,
		)
	}

	var buf strings.Builder
	encoder := yaml.NewEncoder(&buf)
	encoder.SetIndent(2)
	if err := encoder.Encode(&config); err != nil {
		return "", err
	}
	encoder.Close()

	return buf.String(), nil
}

// setMihomoListenerProxy 设置 mihomo listener 的 proxy 字段
func setMihomoListenerProxy(listener *yaml.Node, proxy string) {
	if listener == nil || listener.Kind != yaml.MappingNode {
		return
	}
	// 查找或添加 proxy 字段
	for i := 0; i < len(listener.Content)-1; i += 2 {
		if listener.Content[i].Value == "proxy" {
			listener.Content[i+1].Value = proxy
			return
		}
	}
	// 如果没有找到，添加新的
	listener.Content = append(listener.Content,
		&yaml.Node{Kind: yaml.ScalarNode, Value: "proxy"},
		&yaml.Node{Kind: yaml.ScalarNode, Value: proxy},
	)
}

// buildMihomoOutboundProxy 构建 mihomo 出站代理配置
func buildMihomoOutboundProxy(nodeConfig *database.ServerNodeConfig, tag string) *yaml.Node {
	if nodeConfig == nil || !nodeConfig.OutboundEnabled {
		return nil
	}

	proxy := &yaml.Node{Kind: yaml.MappingNode}

	addField := func(key, value string) {
		proxy.Content = append(proxy.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Value: key},
			&yaml.Node{Kind: yaml.ScalarNode, Value: value},
		)
	}
	addIntField := func(key string, value int) {
		proxy.Content = append(proxy.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Value: key},
			&yaml.Node{Kind: yaml.ScalarNode, Value: fmt.Sprintf("%d", value), Tag: "!!int"},
		)
	}
	addBoolField := func(key string, value bool) {
		proxy.Content = append(proxy.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Value: key},
			&yaml.Node{Kind: yaml.ScalarNode, Value: fmt.Sprintf("%t", value), Tag: "!!bool"},
		)
	}

	addField("name", tag)
	addField("server", nodeConfig.OutboundHost)
	addIntField("port", nodeConfig.OutboundPort)
	addBoolField("udp", true)

	switch nodeConfig.OutboundProtocol {
	case "ss", "shadowsocks":
		addField("type", "ss")
		addField("cipher", nodeConfig.OutboundMethod)
		addField("password", nodeConfig.OutboundPassword)
	case "trojan":
		addField("type", "trojan")
		addField("password", nodeConfig.OutboundPassword)
		if nodeConfig.OutboundSni != "" {
			addField("sni", nodeConfig.OutboundSni)
		}
		addBoolField("skip-cert-verify", true)
		addField("client-fingerprint", "chrome")
	case "socks5":
		addField("type", "socks5")
		if nodeConfig.OutboundUsername != "" {
			addField("username", nodeConfig.OutboundUsername)
			addField("password", nodeConfig.OutboundPassword)
		}
	case "vless":
		addField("type", "vless")
		addField("uuid", nodeConfig.OutboundUUID)
		if nodeConfig.OutboundFlow != "" {
			addField("flow", nodeConfig.OutboundFlow)
		}
		if nodeConfig.OutboundReality {
			addField("network", "tcp")
			addBoolField("tls", true)
			addField("reality-opts.public-key", nodeConfig.OutboundPubKey)
			addField("reality-opts.short-id", nodeConfig.OutboundShortId)
			if nodeConfig.OutboundSni != "" {
				addField("servername", nodeConfig.OutboundSni)
			}
			if nodeConfig.OutboundFp != "" {
				addField("client-fingerprint", nodeConfig.OutboundFp)
			} else {
				addField("client-fingerprint", "chrome")
			}
		} else if nodeConfig.OutboundTls {
			addBoolField("tls", true)
			if nodeConfig.OutboundSni != "" {
				addField("servername", nodeConfig.OutboundSni)
			}
			addBoolField("skip-cert-verify", true)
			addField("client-fingerprint", "chrome")
		}
	case "vmess":
		addField("type", "vmess")
		addField("uuid", nodeConfig.OutboundUUID)
		addIntField("alterId", nodeConfig.OutboundAlterId)
		if nodeConfig.OutboundSecurity != "" {
			addField("cipher", nodeConfig.OutboundSecurity)
		} else {
			addField("cipher", "auto")
		}
		if nodeConfig.OutboundTls {
			addBoolField("tls", true)
			if nodeConfig.OutboundSni != "" {
				addField("servername", nodeConfig.OutboundSni)
			}
			addBoolField("skip-cert-verify", true)
			addField("client-fingerprint", "chrome")
		}
	default:
		addField("type", nodeConfig.OutboundProtocol)
	}

	return proxy
}
