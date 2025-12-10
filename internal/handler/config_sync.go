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

	// TLS 配置（可选，trojan/anytls 也可以不启用 TLS）
	if node.TlsEnabled && node.Protocol != "shadowsocks" {
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
			tls["certificate_path"] = node.CertPath
			tls["key_path"] = node.KeyPath
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

	var nodeConfigs []database.ServerNodeConfig
	database.DB.Where("server_id = ?", server.ID).Preload("Node").Find(&nodeConfigs)
	nodeConfigMap := make(map[uint]*database.ServerNodeConfig)
	for i := range nodeConfigs {
		nodeConfigMap[nodeConfigs[i].NodeID] = &nodeConfigs[i]
	}

	switch configType {
	case "sing-box", "singbox":
		return generateServerSingBoxConfig(nodes, nodeConfigMap)
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
