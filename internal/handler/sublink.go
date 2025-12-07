package handler

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sboard-go/sboard/internal/database"
	"gopkg.in/yaml.v3"
)

// ServerWithNodes 服务器及其节点
type ServerWithNodes struct {
	Server database.Server
	Nodes  []database.InboundNode
}

// handleSubByUUID 处理短订阅链接 /sub/:uuid 或 /subscribe/:uuid
func (s *Server) handleSubByUUID(c *gin.Context) {
	// 从路径参数获取 UUID
	userUUID := c.Param("uuid")

	// 将其他参数转发到 handleSublink
	c.Request.URL.RawQuery = "uuid=" + userUUID + "&" + c.Request.URL.RawQuery
	c.Set("uuid", userUUID)

	// 调用主订阅处理器
	s.handleSublink(c)
}

// handleSublinkPHP 处理旧的 PHP 订阅链接格式
// 兼容: /sublink.php?user=xxx&type=xxx
func (s *Server) handleSublinkPHP(c *gin.Context) {
	// 旧格式使用 "user" 参数，新格式使用 "uuid"
	userUUID := c.Query("user")
	if userUUID == "" {
		userUUID = c.Query("uuid") // 也支持 uuid 参数
	}

	if userUUID != "" {
		c.Set("uuid", userUUID)
	}

	// 调用主订阅处理器
	s.handleSublink(c)
}

// handleSublink 处理订阅链接
func (s *Server) handleSublink(c *gin.Context) {
	// 获取参数 (支持 uuid 和 user 两种参数名)
	userUUID := c.Query("uuid")
	if userUUID == "" {
		userUUID = c.Query("user") // 兼容旧 PHP 格式
	}
	// 如果 query 中没有，尝试从 context 获取（短链接模式）
	if userUUID == "" {
		if uuid, exists := c.Get("uuid"); exists {
			userUUID = uuid.(string)
		}
	}

	// 格式参数：优先使用 URL 参数，否则根据 UA 自动检测
	format := c.Query("format")
	if format == "" {
		format = c.Query("ua") // 兼容 ua 参数
	}
	if format == "" {
		format = detectUserAgent(c) // 自动检测客户端
	}

	serverIDStr := c.Query("server")
	nodeType := c.Query("type") // 节点类型过滤 (协议类型)

	// 用户等级参数
	lvStr := c.Query("lv")
	lv := 0
	if lvStr != "" {
		if v, err := strconv.Atoi(lvStr); err == nil {
			lv = v
		}
	}

	if userUUID == "" {
		c.String(http.StatusBadRequest, "缺少 UUID 参数")
		return
	}

	// 验证用户
	var user database.ProxyUser
	if err := database.DB.Where("uuid = ? AND enabled = ?", userUUID, 1).First(&user).Error; err != nil {
		c.String(http.StatusNotFound, "用户不存在或已禁用")
		return
	}

	// 检查过期
	expiryTime, err := time.Parse("2006-01-02", user.ExpiryDate)
	if err == nil && expiryTime.Before(time.Now()) {
		c.String(http.StatusForbidden, "订阅已过期")
		return
	}

	// 获取用户关联的节点
	var userNodeRelations []database.NodeUserRelation
	database.DB.Where("user_id = ?", user.ID).Find(&userNodeRelations)

	// 构建用户有权访问的节点 ID 集合
	userNodeIDs := make(map[uint]bool)
	for _, rel := range userNodeRelations {
		userNodeIDs[rel.NodeID] = true
	}

	// 获取用户有权限的节点
	var userNodes []database.InboundNode
	if len(userNodeIDs) > 0 {
		nodeIDs := make([]uint, 0, len(userNodeIDs))
		for id := range userNodeIDs {
			nodeIDs = append(nodeIDs, id)
		}
		database.DB.Where("id IN ? AND enabled = ?", nodeIDs, 1).Find(&userNodes)
	}

	// 按节点标签（tag）过滤节点
	if nodeType != "" {
		var filteredNodes []database.InboundNode
		for _, node := range userNodes {
			if node.Tag == nodeType {
				filteredNodes = append(filteredNodes, node)
			}
		}
		userNodes = filteredNodes
	}

	if len(userNodes) == 0 {
		c.String(http.StatusNotFound, "没有可用的节点")
		return
	}

	// 获取服务器
	var servers []database.Server
	query := database.DB.Where("enabled = ?", 1)
	if serverIDStr != "" {
		query = query.Where("id = ?", serverIDStr)
	}
	if err := query.Find(&servers).Error; err != nil {
		c.String(http.StatusInternalServerError, "获取服务器失败")
		return
	}

	// 应用用户等级降级策略过滤服务器
	servers = filterServersByLevel(servers, user.Level, lv)

	if len(servers) == 0 {
		c.String(http.StatusNotFound, "没有可用的服务器")
		return
	}

	// 获取服务器的节点配置（如果有）
	serverNodeConfigs := make(map[uint]map[uint]*database.ServerNodeConfig)
	for _, server := range servers {
		var nodeConfigs []database.ServerNodeConfig
		database.DB.Where("server_id = ?", server.ID).Find(&nodeConfigs)
		if len(nodeConfigs) > 0 {
			serverNodeConfigs[server.ID] = make(map[uint]*database.ServerNodeConfig)
			for i := range nodeConfigs {
				serverNodeConfigs[server.ID][nodeConfigs[i].NodeID] = &nodeConfigs[i]
			}
		}
	}

	// 构建服务器与节点的组合
	var serversWithNodes []ServerWithNodes
	for _, server := range servers {
		serversWithNodes = append(serversWithNodes, ServerWithNodes{
			Server: server,
			Nodes:  userNodes,
		})
	}

	// 默认格式
	if format == "" {
		format = "mihomo"
	}

	var content string
	var contentType string

	switch format {
	case "mihomo", "clash":
		content, err = generateMihomoSubscription(serversWithNodes, serverNodeConfigs, &user, lv)
		contentType = "text/yaml; charset=utf-8"
	case "singbox", "sing-box":
		content, err = generateSingBoxSubscription(serversWithNodes, serverNodeConfigs, &user, lv)
		contentType = "application/json; charset=utf-8"
	case "v2ray", "base64":
		content, err = generateV2RaySubscription(serversWithNodes, serverNodeConfigs, &user, lv)
		contentType = "text/plain; charset=utf-8"
	default:
		c.String(http.StatusBadRequest, "不支持的格式: "+format)
		return
	}

	if err != nil {
		c.String(http.StatusInternalServerError, "生成订阅失败: "+err.Error())
		return
	}

	expiryUnix := int64(0)
	if expiryTime, err := time.Parse("2006-01-02", user.ExpiryDate); err == nil {
		expiryUnix = expiryTime.Unix()
	}

	// 直接显示文本，不触发下载
	c.Header("Subscription-Userinfo", fmt.Sprintf("upload=0; download=%d; total=%d; expire=%d",
		user.TrafficUsed*1024*1024*1024, user.TrafficLimit*1024*1024*1024, expiryUnix))
	c.Data(http.StatusOK, contentType, []byte(content))
}

// ========== Mihomo/Clash 格式 ==========

type MihomoProxy map[string]interface{}

type MihomoGroup struct {
	Name    string   `yaml:"name"`
	Type    string   `yaml:"type"`
	Proxies []string `yaml:"proxies"`
}

// SingBox 类型定义
type SingBoxOutbound map[string]interface{}

func generateMihomoSubscription(servers []ServerWithNodes, nodeConfigs map[uint]map[uint]*database.ServerNodeConfig, user *database.ProxyUser, lv int) (string, error) {
	var proxies []MihomoProxy
	var overseasProxies []string
	var domesticProxies []string

	for _, swn := range servers {
		for _, node := range swn.Nodes {
			if !node.Enabled {
				continue
			}

			// 查找是否有自定义节点配置
			var nc *database.ServerNodeConfig
			if serverConfigs, ok := nodeConfigs[swn.Server.ID]; ok {
				nc = serverConfigs[node.ID]
			}

			proxy := buildMihomoProxy(&swn.Server, &node, nc, user, lv)
			if proxy != nil {
				proxies = append(proxies, proxy)
				proxyName := proxy["name"].(string)

				// 使用 GeoIP 判断节点是否为国内
				nodeIP := getNodeEffectiveIP(&swn.Server, &node, nc)
				if isDomesticByIP(nodeIP) {
					domesticProxies = append(domesticProxies, proxyName)
				} else {
					overseasProxies = append(overseasProxies, proxyName)
				}
			}
		}
	}

	// 构建策略组
	proxyGroups := []MihomoGroup{}

	// 国外分组
	if len(overseasProxies) > 0 {
		proxyGroups = append(proxyGroups, MihomoGroup{
			Name:    "国外",
			Type:    "select",
			Proxies: overseasProxies,
		})
	}

	// 国内分组
	if len(domesticProxies) > 0 {
		proxyGroups = append(proxyGroups, MihomoGroup{
			Name:    "国内",
			Type:    "select",
			Proxies: append([]string{"DIRECT"}, domesticProxies...),
		})
	} else {
		proxyGroups = append(proxyGroups, MihomoGroup{
			Name:    "国内",
			Type:    "select",
			Proxies: []string{"DIRECT"},
		})
	}

	// AI 分组 (使用国外节点)
	if len(overseasProxies) > 0 {
		proxyGroups = append(proxyGroups, MihomoGroup{
			Name:    "AI",
			Type:    "select",
			Proxies: overseasProxies,
		})
	}

	// 流媒体分组 (使用国外节点)
	if len(overseasProxies) > 0 {
		proxyGroups = append(proxyGroups, MihomoGroup{
			Name:    "流媒体",
			Type:    "select",
			Proxies: overseasProxies,
		})
	}

	config := map[string]interface{}{
		"tproxy-port":               1536,
		"ipv6":                      true,
		"allow-lan":                 true,
		"log-level":                 "error",
		"unified-delay":             false,
		"tcp-concurrent":            false,
		"external-controller":       "127.0.0.1:9090",
		"external-ui":               "ui",
		"external-ui-url":           "https://github.com/Zephyruso/zashboard/archive/refs/heads/gh-pages.zip",
		"find-process-mode":         "strict",
		"global-client-fingerprint": "chrome",
		"profile": map[string]interface{}{
			"store-selected": true,
			"store-fake-ip":  true,
		},
		"sniffer": map[string]interface{}{
			"enable": true,
			"sniff": map[string]interface{}{
				"HTTP": map[string]interface{}{
					"ports":                []interface{}{80, "8080-8880"},
					"override-destination": false,
				},
				"TLS": map[string]interface{}{
					"ports": []int{443, 8443},
				},
				"QUIC": map[string]interface{}{
					"ports": []int{443, 8443},
				},
			},
			"skip-domain": []string{
				"Mijia Cloud",
				"+.push.apple.com",
			},
		},
		"tun": map[string]interface{}{
			"enable":                true,
			"stack":                 "gvisor",
			"device":                "eth-jkl",
			"dns-hijack":            []string{"any:53", "tcp://any:53"},
			"auto-route":            true,
			"auto-detect-interface": true,
		},
		"dns": map[string]interface{}{
			"enable":        true,
			"enhanced-mode": "fake-ip",
			"fake-ip-filter": []string{
				"*",
				"+.lan",
				"+.local",
				"+.market.xiaomi.com",
			},
			"default-nameserver": []string{
				"tls://223.5.5.5",
				"tls://223.6.6.6",
			},
			"nameserver": []string{
				"https://dns.alidns.com/dns-query",
			},
			"nameserver-policy": map[string]string{
				"rule-set:httpdns": "rcode://success",
			},
		},
		"proxies":      proxies,
		"proxy-groups": proxyGroups,
		"rules": []string{
			"RULE-SET,httpdns,REJECT",
			"RULE-SET,httpdns_ip,REJECT",
			"RULE-SET,private_ip,DIRECT",
			"RULE-SET,AI,AI",
			"RULE-SET,tiktok,流媒体",
			"RULE-SET,disney,流媒体",
			"RULE-SET,netflix,流媒体",
			"RULE-SET,spotify,流媒体",
			"RULE-SET,proxy,国外",
			"RULE-SET,proxy_ip,国外",
			"RULE-SET,cn,国内",
			"RULE-SET,cn_ip,国内",
			"MATCH,国外",
		},
		"rule-anchor": map[string]interface{}{
			"ip":     map[string]interface{}{"type": "http", "interval": 86400, "behavior": "ipcidr", "format": "mrs"},
			"domain": map[string]interface{}{"type": "http", "interval": 86400, "behavior": "domain", "format": "mrs"},
		},
		"rule-providers": map[string]interface{}{
			"httpdns": map[string]interface{}{
				"type":     "http",
				"interval": 86400,
				"behavior": "domain",
				"format":   "mrs",
				"url":      "https://raw.githubusercontent.com/QuixoticHeart/rule-set/refs/heads/ruleset/meta/domain/httpdns.mrs",
			},
			"cn": map[string]interface{}{
				"type":     "http",
				"interval": 86400,
				"behavior": "domain",
				"format":   "mrs",
				"url":      "https://raw.githubusercontent.com/QuixoticHeart/rule-set/refs/heads/ruleset/meta/domain/cn.mrs",
			},
			"proxy": map[string]interface{}{
				"type":     "http",
				"interval": 86400,
				"behavior": "domain",
				"format":   "mrs",
				"url":      "https://raw.githubusercontent.com/QuixoticHeart/rule-set/refs/heads/ruleset/meta/domain/proxy.mrs",
			},
			"AI": map[string]interface{}{
				"type":     "http",
				"interval": 86400,
				"behavior": "domain",
				"format":   "mrs",
				"url":      "https://raw.githubusercontent.com/QuixoticHeart/rule-set/refs/heads/ruleset/meta/domain/ai.mrs",
			},
			"tiktok": map[string]interface{}{
				"type":     "http",
				"interval": 86400,
				"behavior": "domain",
				"format":   "mrs",
				"url":      "https://raw.githubusercontent.com/QuixoticHeart/rule-set/refs/heads/ruleset/meta/domain/tiktok.mrs",
			},
			"disney": map[string]interface{}{
				"type":     "http",
				"interval": 86400,
				"behavior": "domain",
				"format":   "mrs",
				"url":      "https://raw.githubusercontent.com/QuixoticHeart/rule-set/refs/heads/ruleset/meta/domain/disney.mrs",
			},
			"netflix": map[string]interface{}{
				"type":     "http",
				"interval": 86400,
				"behavior": "domain",
				"format":   "mrs",
				"url":      "https://raw.githubusercontent.com/QuixoticHeart/rule-set/refs/heads/ruleset/meta/domain/netflix.mrs",
			},
			"spotify": map[string]interface{}{
				"type":     "http",
				"interval": 86400,
				"behavior": "domain",
				"format":   "mrs",
				"url":      "https://raw.githubusercontent.com/QuixoticHeart/rule-set/refs/heads/ruleset/meta/domain/spotify.mrs",
			},
			"httpdns_ip": map[string]interface{}{
				"type":     "http",
				"interval": 86400,
				"behavior": "ipcidr",
				"format":   "mrs",
				"url":      "https://raw.githubusercontent.com/QuixoticHeart/rule-set/refs/heads/ruleset/meta/ipcidr/httpdns.mrs",
			},
			"private_ip": map[string]interface{}{
				"type":     "http",
				"interval": 86400,
				"behavior": "ipcidr",
				"format":   "mrs",
				"url":      "https://raw.githubusercontent.com/QuixoticHeart/rule-set/refs/heads/ruleset/meta/ipcidr/private.mrs",
			},
			"cn_ip": map[string]interface{}{
				"type":     "http",
				"interval": 86400,
				"behavior": "ipcidr",
				"format":   "mrs",
				"url":      "https://raw.githubusercontent.com/QuixoticHeart/rule-set/refs/heads/ruleset/meta/ipcidr/cn.mrs",
			},
			"proxy_ip": map[string]interface{}{
				"type":     "http",
				"interval": 86400,
				"behavior": "ipcidr",
				"format":   "mrs",
				"url":      "https://raw.githubusercontent.com/QuixoticHeart/rule-set/refs/heads/ruleset/meta/ipcidr/proxy.mrs",
			},
		},
	}

	data, err := yaml.Marshal(&config)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func buildMihomoProxy(server *database.Server, node *database.InboundNode, nc *database.ServerNodeConfig, user *database.ProxyUser, lv int) MihomoProxy {
	name := getNodeName(server, user.Level, lv)
	host := server.Host

	// 如果服务器设置了节点域名，优先使用
	if server.NodeDomain != "" {
		host = server.NodeDomain
	}

	port := node.Port
	// 如果有自定义配置且启用了端口转发
	if nc != nil && nc.ForwardEnabled && nc.ForwardHost != "" {
		host = nc.ForwardHost
		port = nc.ForwardPort
	}

	proxy := MihomoProxy{
		"name":   name,
		"server": host,
		"port":   port,
	}

	switch node.Protocol {
	case "vmess":
		proxy["type"] = "vmess"
		proxy["uuid"] = user.UUID
		proxy["alterId"] = 0
		proxy["cipher"] = "auto"
		if node.TlsEnabled && node.ServerName != "" {
			proxy["tls"] = true
			proxy["servername"] = node.ServerName
		}

	case "vless":
		proxy["type"] = "vless"
		proxy["uuid"] = user.UUID
		proxy["udp"] = true
		if node.Flow != "" {
			proxy["flow"] = node.Flow
		}
		if node.TlsEnabled && node.ServerName != "" {
			proxy["tls"] = true
			proxy["servername"] = node.ServerName
			proxy["skip-cert-verify"] = true
		}
		if node.RealityEnabled && node.RealityPubkey != "" {
			proxy["tls"] = true
			proxy["client-fingerprint"] = "chrome"
			proxy["reality-opts"] = map[string]interface{}{
				"public-key": node.RealityPubkey,
				"short-id":   node.RealityShortId,
			}
			if node.RealityServer != "" {
				proxy["servername"] = node.RealityServer
			}
		}

	case "trojan":
		proxy["type"] = "trojan"
		proxy["password"] = user.UUID
		if node.TlsEnabled && node.ServerName != "" {
			proxy["sni"] = node.ServerName
		}

	case "anytls":
		proxy["type"] = "anytls"
		proxy["password"] = user.UUID
		if node.TlsEnabled && node.ServerName != "" {
			proxy["sni"] = node.ServerName
		}

	case "shadowsocks":
		proxy["type"] = "ss"
		proxy["cipher"] = node.SsMethod
		proxy["password"] = node.SsPassword

	case "hysteria2":
		proxy["type"] = "hysteria2"
		proxy["password"] = user.UUID // 使用用户 UUID 作为密码
		if node.TlsEnabled && node.ServerName != "" {
			proxy["sni"] = node.ServerName
		}
		if node.Hy2Obfs != "" {
			proxy["obfs"] = node.Hy2Obfs
			proxy["obfs-password"] = node.Hy2ObfsPassword
		}

	default:
		return nil
	}

	return proxy
}

func generateSingBoxSubscription(servers []ServerWithNodes, nodeConfigs map[uint]map[uint]*database.ServerNodeConfig, user *database.ProxyUser, lv int) (string, error) {
	var outbounds []SingBoxOutbound

	for _, swn := range servers {
		for _, node := range swn.Nodes {
			if !node.Enabled {
				continue
			}

			var nc *database.ServerNodeConfig
			if serverConfigs, ok := nodeConfigs[swn.Server.ID]; ok {
				nc = serverConfigs[node.ID]
			}

			outbound := buildSingBoxOutbound(&swn.Server, &node, nc, user, lv)
			if outbound != nil {
				outbounds = append(outbounds, outbound)
			}
		}
	}

	config := map[string]interface{}{
		"outbounds": outbounds,
	}

	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func buildSingBoxOutbound(server *database.Server, node *database.InboundNode, nc *database.ServerNodeConfig, user *database.ProxyUser, lv int) SingBoxOutbound {
	tag := getNodeName(server, user.Level, lv)
	host := server.Host

	if server.NodeDomain != "" {
		host = server.NodeDomain
	}

	port := node.Port
	if nc != nil && nc.ForwardEnabled && nc.ForwardHost != "" {
		host = nc.ForwardHost
		port = nc.ForwardPort
	}

	outbound := SingBoxOutbound{
		"tag":    tag,
		"server": host,
		"port":   port,
	}

	switch node.Protocol {
	case "vmess":
		outbound["type"] = "vmess"
		outbound["uuid"] = user.UUID

	case "vless":
		outbound["type"] = "vless"
		outbound["uuid"] = user.UUID
		outbound["flow"] = node.Flow

	case "trojan":
		outbound["type"] = "trojan"
		outbound["password"] = user.UUID

	case "shadowsocks":
		outbound["type"] = "shadowsocks"
		outbound["method"] = node.SsMethod
		outbound["password"] = node.SsPassword

	case "hysteria2":
		outbound["type"] = "hysteria2"
		outbound["password"] = user.UUID // 使用用户 UUID 作为密码

	default:
		return nil
	}

	return outbound
}

func generateV2RaySubscription(servers []ServerWithNodes, nodeConfigs map[uint]map[uint]*database.ServerNodeConfig, user *database.ProxyUser, lv int) (string, error) {
	var links []string

	for _, swn := range servers {
		for _, node := range swn.Nodes {
			if !node.Enabled {
				continue
			}

			var nc *database.ServerNodeConfig
			if serverConfigs, ok := nodeConfigs[swn.Server.ID]; ok {
				nc = serverConfigs[node.ID]
			}

			link := buildV2RayLink(&swn.Server, &node, nc, user, lv)
			if link != "" {
				links = append(links, link)
			}
		}
	}

	return base64.StdEncoding.EncodeToString([]byte(strings.Join(links, "\n"))), nil
}

func buildV2RayLink(server *database.Server, node *database.InboundNode, nc *database.ServerNodeConfig, user *database.ProxyUser, lv int) string {
	nodeName := getNodeName(server, user.Level, lv)
	name := fmt.Sprintf("%s - %s", nodeName, node.Tag)
	host := server.Host

	if server.NodeDomain != "" {
		host = server.NodeDomain
	}

	port := node.Port
	if nc != nil && nc.ForwardEnabled && nc.ForwardHost != "" {
		host = nc.ForwardHost
		port = nc.ForwardPort
	}

	switch node.Protocol {
	case "vmess":
		vmessConfig := map[string]interface{}{
			"v":    "2",
			"ps":   name,
			"add":  host,
			"port": port,
			"id":   user.UUID,
			"aid":  0,
			"net":  "tcp",
			"type": "none",
		}
		if node.TlsEnabled && node.ServerName != "" {
			vmessConfig["tls"] = "tls"
			vmessConfig["sni"] = node.ServerName
		}
		jsonData, _ := json.Marshal(vmessConfig)
		return "vmess://" + base64.StdEncoding.EncodeToString(jsonData)

	case "vless":
		params := url.Values{}
		params.Set("type", "tcp")
		if node.TlsEnabled && node.ServerName != "" {
			params.Set("security", "tls")
			params.Set("sni", node.ServerName)
		}
		if node.RealityEnabled && node.RealityPubkey != "" {
			params.Set("security", "reality")
			params.Set("pbk", node.RealityPubkey)
			params.Set("sid", node.RealityShortId)
			if node.RealityServer != "" {
				params.Set("sni", node.RealityServer)
			}
		}
		if node.Flow != "" {
			params.Set("flow", node.Flow)
		}
		return fmt.Sprintf("vless://%s@%s:%d?%s#%s", user.UUID, host, port, params.Encode(), url.PathEscape(name))

	case "trojan":
		params := url.Values{}
		if node.TlsEnabled && node.ServerName != "" {
			params.Set("sni", node.ServerName)
		}
		return fmt.Sprintf("trojan://%s@%s:%d?%s#%s", user.UUID, host, port, params.Encode(), url.PathEscape(name))

	case "anytls":
		params := url.Values{}
		if node.TlsEnabled && node.ServerName != "" {
			params.Set("sni", node.ServerName)
		}
		return fmt.Sprintf("anytls://%s@%s:%d?%s#%s", user.UUID, host, port, params.Encode(), url.PathEscape(name))

	case "shadowsocks":
		userInfo := base64.StdEncoding.EncodeToString([]byte(node.SsMethod + ":" + node.SsPassword))
		return fmt.Sprintf("ss://%s@%s:%d#%s", userInfo, host, port, url.PathEscape(name))

	case "hysteria2":
		params := url.Values{}
		if node.TlsEnabled && node.ServerName != "" {
			params.Set("sni", node.ServerName)
		}
		if node.Hy2Obfs != "" {
			params.Set("obfs", node.Hy2Obfs)
			params.Set("obfs-password", node.Hy2ObfsPassword)
		}
		return fmt.Sprintf("hysteria2://%s@%s:%d?%s#%s", user.UUID, host, port, params.Encode(), url.PathEscape(name))

	default:
		return ""
	}
}
