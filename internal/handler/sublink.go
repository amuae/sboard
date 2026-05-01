package handler

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sboard-go/sboard/internal/database"
	"gopkg.in/yaml.v3"
)

// getEffectiveDnsResolve 获取最终的 IP 选择策略（仅控制使用 IPv4 还是 IPv6）
// 优先使用用户的策略，如果用户设置为 "default"，则使用服务器的策略
func getEffectiveDnsResolve(user *database.ProxyUser, server *database.Server) string {
	// 用户策略不为空且不为 "default"，使用用户策略
	if user.DnsResolve != "" && user.DnsResolve != "default" {
		return user.DnsResolve
	}
	// 否则使用服务器策略
	return server.DnsResolve
}

// ServerWithNodes 服务器及其节点
type ServerWithNodes struct {
	Server    database.Server
	Nodes     []database.InboundNode
	Outbounds []database.ServerOutbound // 落地出站列表
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
	} else {
		// 当不带 type 参数时，只保留第一个入站节点（按 ID 排序，取最小的）
		if len(userNodes) > 0 {
			// 找到 ID 最小的节点
			minNode := userNodes[0]
			for _, node := range userNodes {
				if node.ID < minNode.ID {
					minNode = node
				}
			}
			userNodes = []database.InboundNode{minNode}
		}
	}

	if len(userNodes) == 0 {
		c.String(http.StatusNotFound, "没有可用的节点")
		return
	}

	// 获取服务器（按排序顺序）
	var servers []database.Server
	query := database.DB.Where("enabled = ?", 1).Order("sort_order ASC, id ASC")
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

	// 获取每个服务器的落地出站
	serverOutbounds := make(map[uint][]database.ServerOutbound)
	for _, server := range servers {
		var outbounds []database.ServerOutbound
		database.DB.Where("server_id = ? AND enabled = ?", server.ID, true).Order("slot ASC").Find(&outbounds)
		if len(outbounds) > 0 {
			serverOutbounds[server.ID] = outbounds
		}
	}

	// 构建服务器与节点的组合
	var serversWithNodes []ServerWithNodes
	for _, server := range servers {
		serversWithNodes = append(serversWithNodes, ServerWithNodes{
			Server:    server,
			Nodes:     userNodes,
			Outbounds: serverOutbounds[server.ID],
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

	// 获取站点标题
	siteTitle := GetSetting("site_title")
	if siteTitle == "" {
		siteTitle = "SBoard"
	}

	// 获取订阅域名作为网站 URL
	subscribeDomain := GetSetting("subscribe_domain")
	profileWebPageURL := ""
	if subscribeDomain != "" {
		if !strings.HasPrefix(subscribeDomain, "http://") && !strings.HasPrefix(subscribeDomain, "https://") {
			profileWebPageURL = "https://" + subscribeDomain
		} else {
			profileWebPageURL = subscribeDomain
		}
	}

	// 设置订阅响应头
	// Subscription-Userinfo: 流量信息（上传、下载、总量、过期时间）
	c.Header("Subscription-Userinfo", fmt.Sprintf("upload=0; download=%d; total=%d; expire=%d",
		user.TrafficUsed*1024*1024*1024, user.TrafficLimit*1024*1024*1024, expiryUnix))
	// profile-title: 机场名称（base64 编码以支持中文）
	c.Header("profile-title", "base64:"+base64.StdEncoding.EncodeToString([]byte(siteTitle)))
	// profile-update-interval: 自动更新间隔（小时）
	c.Header("profile-update-interval", "24")
	// profile-web-page-url: 网站地址
	if profileWebPageURL != "" {
		c.Header("profile-web-page-url", profileWebPageURL)
	}

	c.Data(http.StatusOK, contentType, []byte(content))
}

// ========== Mihomo/Clash 格式 ==========

// MihomoProxy 使用有序 slice 存储键值对
type MihomoProxy []MihomoProxyField

type MihomoProxyField struct {
	Key   string
	Value interface{}
}

// GetName 获取代理名称
func (p MihomoProxy) GetName() string {
	for _, field := range p {
		if field.Key == "name" {
			if name, ok := field.Value.(string); ok {
				return name
			}
		}
	}
	return ""
}

// MarshalYAML 自定义 YAML 序列化以保持字段顺序
func (p MihomoProxy) MarshalYAML() (interface{}, error) {
	node := &yaml.Node{
		Kind: yaml.MappingNode,
	}
	for _, field := range p {
		keyNode := &yaml.Node{
			Kind:  yaml.ScalarNode,
			Value: field.Key,
		}
		var valueNode *yaml.Node
		switch v := field.Value.(type) {
		case string:
			valueNode = &yaml.Node{Kind: yaml.ScalarNode, Value: v}
		case int:
			valueNode = &yaml.Node{Kind: yaml.ScalarNode, Value: fmt.Sprintf("%d", v), Tag: "!!int"}
		case bool:
			valueNode = &yaml.Node{Kind: yaml.ScalarNode, Value: fmt.Sprintf("%t", v), Tag: "!!bool"}
		case map[string]interface{}:
			valueNode = &yaml.Node{Kind: yaml.MappingNode}
			for k, val := range v {
				valueNode.Content = append(valueNode.Content,
					&yaml.Node{Kind: yaml.ScalarNode, Value: k},
					&yaml.Node{Kind: yaml.ScalarNode, Value: fmt.Sprintf("%v", val)},
				)
			}
		default:
			valueNode = &yaml.Node{Kind: yaml.ScalarNode, Value: fmt.Sprintf("%v", v)}
		}
		node.Content = append(node.Content, keyNode, valueNode)
	}
	return node, nil
}

type MihomoGroup struct {
	Name    string   `yaml:"name"`
	Type    string   `yaml:"type"`
	Proxies []string `yaml:"proxies"`
}

// MihomoGroupFull 带完整参数的策略组
type MihomoGroupFull struct {
	Name          string   `yaml:"name"`
	Type          string   `yaml:"type"`
	Proxies       []string `yaml:"proxies,omitempty"`
	URL           string   `yaml:"url,omitempty"`
	Interval      int      `yaml:"interval,omitempty"`
	Timeout       int      `yaml:"timeout,omitempty"`
	Tolerance     int      `yaml:"tolerance,omitempty"`
	Lazy          bool     `yaml:"lazy,omitempty"`
	Hidden        bool     `yaml:"hidden,omitempty"`
	Strategy      string   `yaml:"strategy,omitempty"`
	ExcludeFilter string   `yaml:"exclude-filter,omitempty"`
}

// SingBox 类型定义
type SingBoxOutbound map[string]interface{}

func generateMihomoSubscription(servers []ServerWithNodes, nodeConfigs map[uint]map[uint]*database.ServerNodeConfig, user *database.ProxyUser, lv int) (string, error) {
	// 获取启用的 Mihomo 配置
	mihomoConfig, err := GetEnabledMihomoConfig()
	if err != nil {
		return "", fmt.Errorf("获取 Mihomo 订阅配置失败: %v", err)
	}
	if mihomoConfig == nil {
		return "", fmt.Errorf("未发现 Mihomo 订阅配置，请先在后台配置并启用")
	}

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

			// 生成直连节点（使用主 UUID）
			proxy := buildMihomoProxy(&swn.Server, &node, nc, user, lv, nil)
			if proxy != nil {
				proxies = append(proxies, proxy)
				proxyName := proxy.GetName()

				// 使用 GeoIP 判断节点是否为国内
				nodeIP := getNodeEffectiveIP(&swn.Server, &node, nc)
				if isDomesticByIP(nodeIP) {
					domesticProxies = append(domesticProxies, proxyName)
				} else {
					overseasProxies = append(overseasProxies, proxyName)
				}
			}

			// 生成落地出站节点（使用额外 UUID）
			for _, outbound := range swn.Outbounds {
				extraUUID := user.GetExtraUUID(outbound.Slot)
				if extraUUID == "" {
					continue
				}
				customOpts := &ProxyCustomOptions{
					CustomName: swn.Server.Name + "-" + outbound.Remark,
					CustomUUID: extraUUID,
				}
				outboundProxy := buildMihomoProxy(&swn.Server, &node, nc, user, lv, customOpts)
				if outboundProxy != nil {
					proxies = append(proxies, outboundProxy)
					proxyName := outboundProxy.GetName()
					// 根据落地出站的 Host 判断国家分组
					if isDomesticByIP(outbound.Host) {
						domesticProxies = append(domesticProxies, proxyName)
					} else {
						overseasProxies = append(overseasProxies, proxyName)
					}
				}
			}
		}
	}

	// 构建策略组 - 从配置读取或使用默认
	proxyGroups := buildMihomoProxyGroups(mihomoConfig, overseasProxies, domesticProxies)

	// 构建基础配置
	baseConfig := buildMihomoBaseConfig(mihomoConfig)

	// 构建带固定键顺序的 YAML 配置
	var ordered yamlOrderedMap

	// 基础设置的键顺序（匹配前端编辑器）
	baseKeys := []string{
		"mixed-port", "redir-port", "tproxy-port", "mode", "bind-address",
		"ipv6", "allow-lan", "unified-delay", "tcp-concurrent",
		"log-level", "find-process-mode", "global-client-fingerprint",
		"external-controller", "external-ui",
		"profile", "sniffer", "tun", "dns",
	}
	for _, k := range baseKeys {
		if v, ok := baseConfig[k]; ok {
			ordered = append(ordered, yamlMapEntry{Key: k, Value: v})
		}
	}

	// 添加代理节点（DNS_Hijack 在最前）
	proxiesWithDNS := append([]MihomoProxy{
		{{Key: "name", Value: "DNS_Hijack"}, {Key: "type", Value: "dns"}},
	}, proxies...)
	ordered = append(ordered, yamlMapEntry{Key: "proxies", Value: proxiesWithDNS})

	// 添加策略组
	ordered = append(ordered, yamlMapEntry{Key: "proxy-groups", Value: proxyGroups})

	// 构建规则集
	ordered = append(ordered, yamlMapEntry{Key: "rule-providers", Value: buildMihomoRuleProviders(mihomoConfig)})

	// 构建路由规则
	ordered = append(ordered, yamlMapEntry{Key: "rules", Value: buildMihomoRules(mihomoConfig)})

	data, err := yaml.Marshal(ordered)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// ProxyCustomOptions 用于传递自定义的节点名称和 UUID
type ProxyCustomOptions struct {
	CustomName string // 自定义节点名称
	CustomUUID string // 自定义 UUID（用于落地出站）
}

// ========== Mihomo 传输层辅助函数 ==========

// buildMihomoTransportOpts 构建 Mihomo 传输层配置
func buildMihomoTransportOpts(node *database.InboundNode, add func(string, interface{})) {
	if !node.TransportEnabled || node.TransportType == "" || node.TransportType == "tcp" {
		return
	}
	add("network", node.TransportType)
	switch node.TransportType {
	case "ws":
		wsOpts := map[string]interface{}{}
		if node.WsPath != "" {
			wsOpts["path"] = node.WsPath
		}
		if node.TransportHost != "" {
			wsOpts["headers"] = map[string]interface{}{"Host": node.TransportHost}
		}
		if len(wsOpts) > 0 {
			add("ws-opts", wsOpts)
		}
	case "grpc":
		if node.GrpcService != "" {
			add("grpc-opts", map[string]interface{}{"grpc-service-name": node.GrpcService})
		}
	case "h2":
		h2Opts := map[string]interface{}{}
		if node.TransportHost != "" {
			h2Opts["host"] = []string{node.TransportHost}
		}
		if node.WsPath != "" {
			h2Opts["path"] = node.WsPath
		}
		if len(h2Opts) > 0 {
			add("h2-opts", h2Opts)
		}
	case "http":
		httpOpts := map[string]interface{}{}
		if node.TransportHost != "" {
			httpOpts["headers"] = map[string]interface{}{"Host": []string{node.TransportHost}}
		}
		if node.WsPath != "" {
			httpOpts["path"] = []string{node.WsPath}
		}
		if len(httpOpts) > 0 {
			add("http-opts", httpOpts)
		}
	}
}

// buildMihomoTLSOpts 构建 Mihomo TLS 配置
func buildMihomoTLSOpts(node *database.InboundNode, add func(string, interface{}), useSNI bool) {
	if useSNI {
		// trojan/anytls/hysteria2 使用 sni 字段
		if node.ServerName != "" {
			add("sni", node.ServerName)
		}
	} else {
		// vmess/vless 使用 servername 字段
		if node.RealityEnabled && node.RealityServer != "" {
			add("servername", node.RealityServer)
		} else if node.ServerName != "" {
			add("servername", node.ServerName)
		}
	}
	add("skip-cert-verify", true)
	add("client-fingerprint", "chrome")
	// Reality 配置
	if node.RealityEnabled && node.RealityPubkey != "" {
		add("reality-opts", map[string]interface{}{
			"public-key": node.RealityPubkey,
			"short-id":   node.RealityShortId,
		})
	}
}

// ProxyBuildParams 节点构建参数（公共初始化结果）
type ProxyBuildParams struct {
	Name string
	UUID string
	Host string
	Port int
}

// initProxyBuildParams 初始化节点构建的公共参数
func initProxyBuildParams(server *database.Server, node *database.InboundNode, nc *database.ServerNodeConfig, user *database.ProxyUser, lv int, customOpts *ProxyCustomOptions) ProxyBuildParams {
	name := getNodeName(server, user.Level, lv)
	if customOpts != nil && customOpts.CustomName != "" {
		name = customOpts.CustomName
	}
	uuid := user.UUID
	if customOpts != nil && customOpts.CustomUUID != "" {
		uuid = customOpts.CustomUUID
	}
	// 根据 IP 选择策略选取 Agent 上报的 IP
	host := server.Host
	dnsStrategy := getEffectiveDnsResolve(user, server)
	// 如果策略为 ipv6 且服务器有 IPv6 地址，优先使用 IPv6
	if dnsStrategy == "ipv6" && server.HostIPv6 != "" {
		host = server.HostIPv6
	}
	port := node.Port
	if nc != nil && nc.ForwardEnabled && nc.ForwardHost != "" {
		host = nc.ForwardHost
		port = nc.ForwardPort
	}
	return ProxyBuildParams{Name: name, UUID: uuid, Host: host, Port: port}
}

// getSS2022Password 为 Shadowsocks 2022 多用户模式生成客户端密码
// 格式: 服务器密钥:用户密钥
func getSS2022Password(node *database.InboundNode, uuid string) string {
	// 判断是否为 SS 2022 方法
	if !strings.HasPrefix(node.SsMethod, "2022-") {
		// 非 2022 方法，直接返回服务器密码
		return node.SsPassword
	}

	// SS 2022 多用户模式需要: 服务器密钥:用户密钥
	userKey := generateSS2022UserKeyForSublink(uuid, node.SsMethod)
	return node.SsPassword + ":" + userKey
}

// generateSS2022UserKeyForSublink 为 Shadowsocks 2022 生成用户密钥
func generateSS2022UserKeyForSublink(uuid string, method string) string {
	hash := sha256.Sum256([]byte(uuid))

	var keyLen int
	switch method {
	case "2022-blake3-aes-128-gcm":
		keyLen = 16
	case "2022-blake3-aes-256-gcm", "2022-blake3-chacha20-poly1305":
		keyLen = 32
	default:
		return uuid
	}

	return base64.StdEncoding.EncodeToString(hash[:keyLen])
}

func buildMihomoProxy(server *database.Server, node *database.InboundNode, nc *database.ServerNodeConfig, user *database.ProxyUser, lv int, customOpts *ProxyCustomOptions) MihomoProxy {
	p := initProxyBuildParams(server, node, nc, user, lv, customOpts)

	var proxy MihomoProxy
	add := func(key string, value interface{}) {
		proxy = append(proxy, MihomoProxyField{Key: key, Value: value})
	}

	switch node.Protocol {
	case "vmess":
		add("type", "vmess")
		add("name", p.Name)
		add("server", p.Host)
		add("port", p.Port)
		add("uuid", p.UUID)
		add("alterId", 0)
		add("cipher", "auto")
		add("udp", true)
		buildMihomoTransportOpts(node, add)
		if node.TlsEnabled {
			add("tls", true)
			buildMihomoTLSOpts(node, add, false)
		}

	case "vless":
		add("type", "vless")
		add("name", p.Name)
		add("server", p.Host)
		add("port", p.Port)
		add("uuid", p.UUID)
		add("udp", true)
		buildMihomoTransportOpts(node, add)
		if node.TlsEnabled || node.RealityEnabled {
			add("tls", true)
			buildMihomoTLSOpts(node, add, false)
		}
		if node.Flow != "" {
			add("flow", node.Flow)
		}

	case "trojan":
		add("type", "trojan")
		add("name", p.Name)
		add("server", p.Host)
		add("port", p.Port)
		add("password", p.UUID)
		add("udp", true)
		buildMihomoTransportOpts(node, add)
		buildMihomoTLSOpts(node, add, true)

	case "anytls":
		add("type", "anytls")
		add("name", p.Name)
		add("server", p.Host)
		add("port", p.Port)
		add("password", p.UUID)
		add("udp", true)
		buildMihomoTLSOpts(node, add, true)

	case "shadowsocks":
		add("type", "ss")
		add("name", p.Name)
		add("server", p.Host)
		add("port", p.Port)
		add("cipher", node.SsMethod)
		add("password", getSS2022Password(node, p.UUID))
		add("udp", true)

	case "hysteria2":
		add("type", "hysteria2")
		add("name", p.Name)
		add("server", p.Host)
		add("port", p.Port)
		add("password", p.UUID)
		add("udp", true)
		buildMihomoTLSOpts(node, add, true)
		if node.Hy2Obfs != "" {
			add("obfs", node.Hy2Obfs)
			add("obfs-password", node.Hy2ObfsPassword)
		}

	default:
		return nil
	}

	return proxy
}

func generateSingBoxSubscription(servers []ServerWithNodes, nodeConfigs map[uint]map[uint]*database.ServerNodeConfig, user *database.ProxyUser, lv int) (string, error) {
	// 获取用户配置的 SingBox 订阅模板
	subscriptionConfig, err := GetEnabledSingBoxConfig()
	if err != nil {
		return "", fmt.Errorf("获取 SingBox 配置失败: %v", err)
	}

	var nodeOutbounds []SingBoxOutbound
	// 按 GeoIP 国家分类的节点标签
	countryProxyTags := make(map[string][]string) // 国家代码 -> 节点标签列表
	var domesticProxyTags []string                // 国内节点（geoip-cn）
	var overseasProxyTags []string                // 非国内节点

	for _, swn := range servers {
		for _, node := range swn.Nodes {
			if !node.Enabled {
				continue
			}

			var nc *database.ServerNodeConfig
			if serverConfigs, ok := nodeConfigs[swn.Server.ID]; ok {
				nc = serverConfigs[node.ID]
			}

			// 生成直连节点（使用主 UUID）
			outbound := buildSingBoxOutbound(&swn.Server, &node, nc, user, lv, nil)
			if outbound != nil {
				nodeOutbounds = append(nodeOutbounds, outbound)
				tag := outbound["tag"].(string)

				// 使用 GeoIP 判断节点所属国家
				nodeIP := getNodeEffectiveIP(&swn.Server, &node, nc)
				countryCode := getCountryCode(nodeIP)

				if countryCode != "" {
					countryProxyTags[countryCode] = append(countryProxyTags[countryCode], tag)
				}

				if isDomesticByIP(nodeIP) {
					domesticProxyTags = append(domesticProxyTags, tag)
				} else {
					overseasProxyTags = append(overseasProxyTags, tag)
				}
			}

			// 生成落地出站节点（使用额外 UUID）
			for _, ob := range swn.Outbounds {
				extraUUID := user.GetExtraUUID(ob.Slot)
				if extraUUID == "" {
					continue
				}
				customOpts := &ProxyCustomOptions{
					CustomName: swn.Server.Name + "-" + ob.Remark,
					CustomUUID: extraUUID,
				}
				outboundProxy := buildSingBoxOutbound(&swn.Server, &node, nc, user, lv, customOpts)
				if outboundProxy != nil {
					nodeOutbounds = append(nodeOutbounds, outboundProxy)
					tag := outboundProxy["tag"].(string)
					// 根据落地出站的 Host 判断国家分组
					countryCode := getCountryCode(ob.Host)
					if countryCode != "" {
						countryProxyTags[countryCode] = append(countryProxyTags[countryCode], tag)
					}
					if isDomesticByIP(ob.Host) {
						domesticProxyTags = append(domesticProxyTags, tag)
					} else {
						overseasProxyTags = append(overseasProxyTags, tag)
					}
				}
			}
		}
	}

	// 使用配置模板构建完整配置
	config := make(map[string]interface{})

	// 如果没有找到启用的配置，返回错误
	if subscriptionConfig == nil {
		return "", fmt.Errorf("未发现 SingBox 订阅配置，请先在后台配置并启用")
	}

	tpl := subscriptionConfig.Config

	// Log 配置
	config["log"] = map[string]interface{}{
		"disabled": tpl.Log.Disabled,
	}
	if tpl.Log.Level != "" {
		config["log"].(map[string]interface{})["level"] = tpl.Log.Level
	}

	// DNS 配置
	dnsConfig := map[string]interface{}{
		"strategy":          tpl.DNS.Strategy,
		"reverse_mapping":   tpl.DNS.ReverseMapping,
	}
	if tpl.DNS.Final != "" {
		dnsConfig["final"] = tpl.DNS.Final
	}
	if tpl.DNS.ClientSubnet != "" {
		dnsConfig["client_subnet"] = tpl.DNS.ClientSubnet
	}
	if tpl.DNS.DisableCache {
		dnsConfig["disable_cache"] = true
	}

	// DNS servers
	var dnsServers []map[string]interface{}
	for _, srv := range tpl.DNS.Servers {
		server := map[string]interface{}{
			"tag":  srv.Tag,
			"type": srv.Type,
		}
		if srv.Type == "fakeip" {
			// fakeip 类型使用 inet4_range 和 inet6_range
			server["inet4_range"] = "198.18.0.0/15"
			server["inet6_range"] = "fc00::/18"
		} else {
			if srv.Server != "" {
				server["server"] = srv.Server
			}
			if srv.Detour != "" {
				server["detour"] = srv.Detour
			}
			if srv.DomainResolver != "" {
				server["domain_resolver"] = srv.DomainResolver
			}
		}
		dnsServers = append(dnsServers, server)
	}
	dnsConfig["servers"] = dnsServers

	// DNS rules
	var dnsRules []map[string]interface{}
	for _, rule := range tpl.DNS.Rules {
		dnsRule := buildSingBoxDNSRule(rule)
		if dnsRule != nil {
			dnsRules = append(dnsRules, dnsRule)
		}
	}
	dnsConfig["rules"] = dnsRules
	config["dns"] = dnsConfig

	// Inbound 配置
	if tpl.Inbound.TunEnable {
		addresses := []string{}
		if tpl.Inbound.AddressIPv4 != "" {
			addresses = append(addresses, tpl.Inbound.AddressIPv4)
		}
		if tpl.Inbound.AddressIPv6 != "" {
			addresses = append(addresses, tpl.Inbound.AddressIPv6)
		}
		inbound := map[string]interface{}{
			"tag":            "tun-in",
			"type":           "tun",
			"address":        addresses,
			"mtu":            tpl.Inbound.MTU,
			"interface_name": tpl.Inbound.InterfaceName,
			"stack":          tpl.Inbound.Stack,
			"auto_route":     tpl.Inbound.AutoRoute,
			"auto_redirect":  tpl.Inbound.AutoRedirect,
			"strict_route":   tpl.Inbound.StrictRoute,
		}
		config["inbounds"] = []map[string]interface{}{inbound}
	}

	// Outbound 配置
	outbounds := []map[string]interface{}{
		{"type": "direct", "tag": "直连"},
	}

	// 根据配置的策略组构建 outbounds
	for _, group := range tpl.OutboundGroups {
		outbound := map[string]interface{}{
			"type": group.Type,
			"tag":  group.Tag,
		}

		// 根据过滤模式选择节点
		var selectedTags []string
		switch group.FilterMode {
		case "geoip-cn":
			// 国内/非国内过滤
			switch group.Filter {
			case "cn":
				// 国内分组必须包含直连选项
				selectedTags = append([]string{"直连"}, domesticProxyTags...)
			case "!cn":
				selectedTags = overseasProxyTags
			}
		case "geoip-country":
			// 按国家过滤
			countries := strings.Split(group.Filter, ",")
			for _, country := range countries {
				country = strings.TrimSpace(strings.ToUpper(country))
				if tags, ok := countryProxyTags[country]; ok {
					selectedTags = append(selectedTags, tags...)
				}
			}
		case "regex":
			// 正则过滤
			if group.Filter != "" {
				pattern, err := regexp.Compile(group.Filter)
				if err == nil {
					for _, nodeOutbound := range nodeOutbounds {
						tag := nodeOutbound["tag"].(string)
						if pattern.MatchString(tag) {
							selectedTags = append(selectedTags, tag)
						}
					}
				}
			}
		case "all":
			// 所有节点
			for _, nodeOutbound := range nodeOutbounds {
				selectedTags = append(selectedTags, nodeOutbound["tag"].(string))
			}
		default:
			// 默认使用所有海外节点
			selectedTags = overseasProxyTags
		}

		if len(selectedTags) > 0 {
			outbound["outbounds"] = selectedTags
		}

		// URL-test 类型需要额外参数
		if group.Type == "urltest" {
			if group.URL != "" {
				outbound["url"] = group.URL
			}
			if group.Interval != "" {
				outbound["interval"] = group.Interval
			}
		}

		outbounds = append(outbounds, outbound)
	}

	// 添加所有节点 outbound
	for _, nodeOutbound := range nodeOutbounds {
		outbounds = append(outbounds, nodeOutbound)
	}
	config["outbounds"] = outbounds

	// Route 配置
	routeConfig := map[string]interface{}{
		"final":                   tpl.Route.Final,
		"auto_detect_interface":   tpl.Route.AutoDetectInterface,
		"default_domain_resolver": tpl.Route.DefaultDomainResolver,
	}

	// Route rules
	var routeRules []map[string]interface{}
	for _, rule := range tpl.Route.Rules {
		routeRule := buildSingBoxRouteRule(rule)
		if routeRule != nil {
			routeRules = append(routeRules, routeRule)
		}
	}
	routeConfig["rules"] = routeRules

	// Rule sets
	var ruleSets []map[string]interface{}
	for _, rs := range tpl.Route.RuleSets {
		ruleSet := map[string]interface{}{
			"tag":  rs.Tag,
			"type": rs.Type,
			"url":  rs.URL,
		}
		if rs.Format != "" {
			ruleSet["format"] = rs.Format
		}
		ruleSets = append(ruleSets, ruleSet)
	}
	routeConfig["rule_set"] = ruleSets
	config["route"] = routeConfig

	// Experimental 配置
	config["experimental"] = map[string]interface{}{
		"cache_file": map[string]interface{}{
			"enabled":      tpl.Experimental.CacheFileEnabled,
			"store_fakeip": tpl.Experimental.StoreFakeip,
		},
		"clash_api": map[string]interface{}{
			"external_controller":      tpl.Experimental.ExternalController,
			"external_ui":              tpl.Experimental.ExternalUi,
			"external_ui_download_url": tpl.Experimental.ExternalUiDownloadUrl,
		},
	}

	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// buildSingBoxDNSRule 构建 SingBox DNS 规则
func buildSingBoxDNSRule(rule SingBoxDNSRule) map[string]interface{} {
	result := map[string]interface{}{}

	// 设置 server 或 action
	if rule.Server != "" {
		result["server"] = rule.Server
	}
	if rule.Action != "" {
		result["action"] = rule.Action
	}

	switch rule.Type {
	case "clash_mode":
		result["clash_mode"] = rule.Value
	case "rule_set":
		if len(rule.Values) == 1 {
			result["rule_set"] = rule.Values[0]
		} else if len(rule.Values) > 1 {
			result["rule_set"] = rule.Values
		}
	case "query_type":
		types := strings.Split(rule.Value, ",")
		for i := range types {
			types[i] = strings.TrimSpace(types[i])
		}
		result["query_type"] = types
		if rule.RewriteTtl {
			result["rewrite_ttl"] = 1
		}
	case "domain_suffix":
		suffixes := strings.Split(rule.Value, ",")
		for i := range suffixes {
			suffixes[i] = strings.TrimSpace(suffixes[i])
		}
		result["domain_suffix"] = suffixes
	case "domain_keyword":
		keywords := strings.Split(rule.Value, ",")
		for i := range keywords {
			keywords[i] = strings.TrimSpace(keywords[i])
		}
		result["domain_keyword"] = keywords
	default:
		if rule.Value != "" {
			result[rule.Type] = rule.Value
		}
	}

	return result
}

// buildSingBoxRouteRule 构建 SingBox 路由规则
func buildSingBoxRouteRule(rule SingBoxRouteRule) map[string]interface{} {
	result := make(map[string]interface{})

	// 设置 action 或 outbound
	if rule.Action != "" {
		result["action"] = rule.Action
	}
	if rule.Outbound != "" {
		result["outbound"] = rule.Outbound
	}

	switch rule.Type {
	case "logical":
		result["type"] = "logical"
		result["mode"] = rule.Mode
		var subRules []map[string]interface{}
		for _, sr := range rule.SubRules {
			subRule := buildSingBoxSubRule(sr)
			if subRule != nil {
				subRules = append(subRules, subRule)
			}
		}
		result["rules"] = subRules
	case "rule_set":
		if len(rule.Values) == 1 {
			result["rule_set"] = rule.Values[0]
		} else if len(rule.Values) > 1 {
			result["rule_set"] = rule.Values
		}
	case "protocol":
		if rule.Value != "" {
			protocols := strings.Split(rule.Value, ",")
			for i := range protocols {
				protocols[i] = strings.TrimSpace(protocols[i])
			}
			if len(protocols) == 1 {
				result["protocol"] = protocols[0]
			} else {
				result["protocol"] = protocols
			}
		}
	case "ip_is_private":
		result["ip_is_private"] = true
	case "domain_suffix":
		suffixes := strings.Split(rule.Value, ",")
		for i := range suffixes {
			suffixes[i] = strings.TrimSpace(suffixes[i])
		}
		result["domain_suffix"] = suffixes
	case "domain_keyword":
		keywords := strings.Split(rule.Value, ",")
		for i := range keywords {
			keywords[i] = strings.TrimSpace(keywords[i])
		}
		result["domain_keyword"] = keywords
	case "ip_cidr":
		cidrs := strings.Split(rule.Value, ",")
		for i := range cidrs {
			cidrs[i] = strings.TrimSpace(cidrs[i])
		}
		result["ip_cidr"] = cidrs
	case "port":
		ports := strings.Split(rule.Value, ",")
		var portInts []int
		for _, p := range ports {
			p = strings.TrimSpace(p)
			if port, err := strconv.Atoi(p); err == nil {
				portInts = append(portInts, port)
			}
		}
		result["port"] = portInts
	default:
		if rule.Value != "" {
			result[rule.Type] = rule.Value
		}
	}

	return result
}

// buildSingBoxSubRule 构建 SingBox 子规则
func buildSingBoxSubRule(rule SingBoxSubRule) map[string]interface{} {
	result := make(map[string]interface{})

	switch rule.Type {
	case "domain_suffix":
		suffixes := strings.Split(rule.Value, ",")
		for i := range suffixes {
			suffixes[i] = strings.TrimSpace(suffixes[i])
		}
		result["domain_suffix"] = suffixes
	case "domain_keyword":
		keywords := strings.Split(rule.Value, ",")
		for i := range keywords {
			keywords[i] = strings.TrimSpace(keywords[i])
		}
		result["domain_keyword"] = keywords
	case "ip_cidr":
		cidrs := strings.Split(rule.Value, ",")
		for i := range cidrs {
			cidrs[i] = strings.TrimSpace(cidrs[i])
		}
		result["ip_cidr"] = cidrs
	case "port":
		ports := strings.Split(rule.Value, ",")
		var portInts []int
		for _, p := range ports {
			p = strings.TrimSpace(p)
			if port, err := strconv.Atoi(p); err == nil {
				portInts = append(portInts, port)
			}
		}
		result["port"] = portInts
	default:
		if rule.Value != "" {
			result[rule.Type] = rule.Value
		}
	}

	return result
}

func buildSingBoxOutbound(server *database.Server, node *database.InboundNode, nc *database.ServerNodeConfig, user *database.ProxyUser, lv int, customOpts *ProxyCustomOptions) SingBoxOutbound {
	p := initProxyBuildParams(server, node, nc, user, lv, customOpts)

	outbound := SingBoxOutbound{
		"tag":         p.Name,
		"server":      p.Host,
		"server_port": p.Port,
	}

	switch node.Protocol {
	case "vmess":
		outbound["type"] = "vmess"
		outbound["uuid"] = p.UUID
		outbound["security"] = "auto"
		outbound["alter_id"] = 0
		if node.TransportEnabled && node.TransportType != "" && node.TransportType != "tcp" {
			outbound["transport"] = buildSingBoxTransport(node)
		}
		if node.TlsEnabled {
			outbound["tls"] = buildSingBoxTLS(node)
		}

	case "vless":
		outbound["type"] = "vless"
		outbound["uuid"] = p.UUID
		if node.Flow != "" {
			outbound["flow"] = node.Flow
		}
		if node.TransportEnabled && node.TransportType != "" && node.TransportType != "tcp" {
			outbound["transport"] = buildSingBoxTransport(node)
		}
		if node.TlsEnabled || node.RealityEnabled {
			outbound["tls"] = buildSingBoxTLS(node)
		}

	case "trojan":
		outbound["type"] = "trojan"
		outbound["password"] = p.UUID
		if node.TransportEnabled && node.TransportType != "" && node.TransportType != "tcp" {
			outbound["transport"] = buildSingBoxTransport(node)
		}
		// Trojan 协议必须有 TLS（根据 Sub-Store 参考实现）
		outbound["tls"] = buildSingBoxTLS(node)

	case "shadowsocks":
		outbound["type"] = "shadowsocks"
		outbound["method"] = node.SsMethod
		outbound["password"] = getSS2022Password(node, p.UUID)

	case "hysteria2":
		outbound["type"] = "hysteria2"
		outbound["password"] = p.UUID
		outbound["tls"] = buildSingBoxTLS(node)
		if node.Hy2Obfs != "" {
			outbound["obfs"] = map[string]interface{}{
				"type":     node.Hy2Obfs,
				"password": node.Hy2ObfsPassword,
			}
		}

	case "anytls":
		outbound["type"] = "anytls"
		outbound["password"] = p.UUID
		outbound["tls"] = buildSingBoxTLS(node)

	case "naive":
		outbound["type"] = "naive"
		outbound["username"] = user.Name
		outbound["password"] = p.UUID
		naiveTLS := map[string]interface{}{"enabled": true}
		if node.ServerName != "" {
			naiveTLS["server_name"] = node.ServerName
		}
		outbound["tls"] = naiveTLS

	default:
		return nil
	}

	return outbound
}

// buildSingBoxTLS 构建 sing-box TLS 配置
func buildSingBoxTLS(node *database.InboundNode) map[string]interface{} {
	tls := map[string]interface{}{
		"enabled":  true,
		"insecure": true,
	}

	// 服务器名称
	if node.ServerName != "" {
		tls["server_name"] = node.ServerName
	}

	// Reality 配置
	if node.RealityEnabled && node.RealityPubkey != "" {
		if node.RealityServer != "" {
			tls["server_name"] = node.RealityServer
		}
		tls["reality"] = map[string]interface{}{
			"enabled":    true,
			"public_key": node.RealityPubkey,
			"short_id":   node.RealityShortId,
		}
	}

	// uTLS 指纹
	tls["utls"] = map[string]interface{}{
		"enabled":     true,
		"fingerprint": "chrome",
	}

	return tls
}

// buildSingBoxTransport 构建 sing-box 传输层配置
func buildSingBoxTransport(node *database.InboundNode) map[string]interface{} {
	transport := map[string]interface{}{
		"type": node.TransportType,
	}

	switch node.TransportType {
	case "ws":
		if node.WsPath != "" {
			transport["path"] = node.WsPath
		}
		if node.TransportHost != "" {
			transport["headers"] = map[string]interface{}{
				"Host": node.TransportHost,
			}
		}
	case "grpc":
		if node.GrpcService != "" {
			transport["service_name"] = node.GrpcService
		}
	case "http", "h2":
		transport["type"] = "http"
		if node.TransportHost != "" {
			transport["host"] = []string{node.TransportHost}
		}
		if node.WsPath != "" {
			transport["path"] = node.WsPath
		}
	case "httpupgrade":
		if node.WsPath != "" {
			transport["path"] = node.WsPath
		}
		if node.TransportHost != "" {
			transport["host"] = node.TransportHost
		}
	}

	return transport
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

			// 生成直连节点（使用主 UUID）
			link := buildV2RayLink(&swn.Server, &node, nc, user, lv, nil)
			if link != "" {
				links = append(links, link)
			}

			// 生成落地出站节点（使用额外 UUID）
			for _, ob := range swn.Outbounds {
				extraUUID := user.GetExtraUUID(ob.Slot)
				if extraUUID == "" {
					continue
				}
				customOpts := &ProxyCustomOptions{
					CustomName: swn.Server.Name + "-" + ob.Remark,
					CustomUUID: extraUUID,
				}
				outboundLink := buildV2RayLink(&swn.Server, &node, nc, user, lv, customOpts)
				if outboundLink != "" {
					links = append(links, outboundLink)
				}
			}
		}
	}

	return base64.StdEncoding.EncodeToString([]byte(strings.Join(links, "\n"))), nil
}

func buildV2RayLink(server *database.Server, node *database.InboundNode, nc *database.ServerNodeConfig, user *database.ProxyUser, lv int, customOpts *ProxyCustomOptions) string {
	p := initProxyBuildParams(server, node, nc, user, lv, customOpts)

	switch node.Protocol {
	case "vmess":
		vmessConfig := map[string]interface{}{
			"v":    "2",
			"ps":   p.Name,
			"add":  p.Host,
			"port": p.Port,
			"id":   p.UUID,
			"aid":  0,
			"scy":  "auto",
			"type": "none",
		}
		// 传输层配置 - 参考 Sub-Store V2RayN 格式
		if node.TransportEnabled && node.TransportType != "" && node.TransportType != "tcp" {
			if node.TransportType == "http" {
				vmessConfig["net"] = "tcp"
				vmessConfig["type"] = "http"
				if node.WsPath != "" {
					vmessConfig["path"] = node.WsPath
				}
				if node.TransportHost != "" {
					vmessConfig["host"] = node.TransportHost
				}
			} else {
				vmessConfig["net"] = node.TransportType
				if node.TransportType == "ws" {
					if node.WsPath != "" {
						vmessConfig["path"] = node.WsPath
					}
					if node.TransportHost != "" {
						vmessConfig["host"] = node.TransportHost
					}
				} else if node.TransportType == "grpc" && node.GrpcService != "" {
					vmessConfig["path"] = node.GrpcService
					vmessConfig["type"] = "gun"
				} else if node.TransportType == "h2" {
					if node.WsPath != "" {
						vmessConfig["path"] = node.WsPath
					}
					if node.TransportHost != "" {
						vmessConfig["host"] = node.TransportHost
					}
				}
			}
		} else {
			vmessConfig["net"] = "tcp"
		}
		// TLS 配置（只有启用时才添加）
		if node.TlsEnabled {
			vmessConfig["tls"] = "tls"
			if node.ServerName != "" {
				vmessConfig["sni"] = node.ServerName
			}
		} else {
			vmessConfig["tls"] = ""
		}
		jsonData, _ := json.Marshal(vmessConfig)
		return "vmess://" + base64.StdEncoding.EncodeToString(jsonData)

	case "vless":
		params := url.Values{}
		// 传输层配置
		if node.TransportEnabled && node.TransportType != "" && node.TransportType != "tcp" {
			params.Set("type", node.TransportType)
			if node.TransportType == "ws" {
				if node.WsPath != "" {
					params.Set("path", node.WsPath)
				}
				if node.TransportHost != "" {
					params.Set("host", node.TransportHost)
				}
			} else if node.TransportType == "grpc" && node.GrpcService != "" {
				params.Set("serviceName", node.GrpcService)
			}
		} else {
			params.Set("type", "tcp")
		}
		// TLS / Reality 配置 - 参考 Sub-Store
		if node.RealityEnabled && node.RealityPubkey != "" {
			params.Set("security", "reality")
			params.Set("pbk", node.RealityPubkey)
			params.Set("sid", node.RealityShortId)
			if node.RealityServer != "" {
				params.Set("sni", node.RealityServer)
			}
			params.Set("fp", "chrome")
		} else if node.TlsEnabled {
			params.Set("security", "tls")
			if node.ServerName != "" {
				params.Set("sni", node.ServerName)
			}
			params.Set("allowInsecure", "1")
			params.Set("fp", "chrome")
		} else {
			params.Set("security", "none")
		}
		if node.Flow != "" {
			params.Set("flow", node.Flow)
		}
		return fmt.Sprintf("vless://%s@%s:%d?%s#%s", p.UUID, formatHostForURL(p.Host), p.Port, params.Encode(), url.PathEscape(p.Name))

	case "trojan":
		params := url.Values{}
		if node.TransportEnabled && node.TransportType != "" && node.TransportType != "tcp" {
			params.Set("type", node.TransportType)
			if node.TransportType == "ws" {
				if node.WsPath != "" {
					params.Set("path", node.WsPath)
				}
				if node.TransportHost != "" {
					params.Set("host", node.TransportHost)
				}
			} else if node.TransportType == "grpc" && node.GrpcService != "" {
				params.Set("serviceName", node.GrpcService)
				params.Set("mode", "gun")
			}
		}
		if node.ServerName != "" {
			params.Set("sni", node.ServerName)
		} else {
			params.Set("sni", p.Host)
		}
		params.Set("allowInsecure", "1")
		params.Set("fp", "chrome")
		return fmt.Sprintf("trojan://%s@%s:%d?%s#%s", p.UUID, formatHostForURL(p.Host), p.Port, params.Encode(), url.PathEscape(p.Name))

	case "anytls":
		params := url.Values{}
		if node.ServerName != "" {
			params.Set("sni", node.ServerName)
		}
		params.Set("insecure", "1")
		return fmt.Sprintf("anytls://%s@%s:%d?%s#%s", p.UUID, formatHostForURL(p.Host), p.Port, params.Encode(), url.PathEscape(p.Name))

	case "shadowsocks":
		userInfo := base64.StdEncoding.EncodeToString([]byte(node.SsMethod + ":" + getSS2022Password(node, p.UUID)))
		return fmt.Sprintf("ss://%s@%s:%d#%s", userInfo, formatHostForURL(p.Host), p.Port, url.PathEscape(p.Name))

	case "hysteria2":
		params := url.Values{}
		if node.ServerName != "" {
			params.Set("sni", node.ServerName)
		}
		params.Set("insecure", "1")
		if node.Hy2Obfs != "" {
			params.Set("obfs", node.Hy2Obfs)
			params.Set("obfs-password", node.Hy2ObfsPassword)
		}
		return fmt.Sprintf("hysteria2://%s@%s:%d?%s#%s", p.UUID, formatHostForURL(p.Host), p.Port, params.Encode(), url.PathEscape(p.Name))

	case "naive":
		params := url.Values{}
		if node.ServerName != "" {
			params.Set("sni", node.ServerName)
		}
		queryStr := ""
		if len(params) > 0 {
			queryStr = "?" + params.Encode()
		}
		return fmt.Sprintf("naive+https://%s:%s@%s:%d%s#%s", url.PathEscape(user.Name), url.PathEscape(p.UUID), formatHostForURL(p.Host), p.Port, queryStr, url.PathEscape(p.Name))

	default:
		return ""
	}
}

// ========== Mihomo 配置构建帮助函数 ==========

// buildMihomoProxyGroups 根据配置构建策略组
func buildMihomoProxyGroups(cfg *MihomoSubscriptionConfig, overseasProxies, domesticProxies []string) []map[string]interface{} {
	var groups []map[string]interface{}

	if cfg != nil && len(cfg.Config.ProxyGroups) > 0 {
		// 使用数据库配置的策略组
		for _, pg := range cfg.Config.ProxyGroups {
			group := map[string]interface{}{
				"name": pg.Name,
				"type": pg.Type,
			}

			// 根据 filterMode 过滤节点
			var selectedProxies []string
			switch pg.FilterMode {
			case "geoip-cn":
				if pg.Filter == "cn" {
					selectedProxies = domesticProxies
				} else {
					selectedProxies = overseasProxies
				}
			case "regex":
				// 正则过滤
				if pg.Filter != "" && pg.Filter != ".*" {
					for _, name := range append(overseasProxies, domesticProxies...) {
						matched, _ := matchRegex(pg.Filter, name)
						if matched {
							selectedProxies = append(selectedProxies, name)
						}
					}
				} else if pg.IncludeAll {
					selectedProxies = append(overseasProxies, domesticProxies...)
				} else {
					selectedProxies = overseasProxies
				}
			default:
				if pg.IncludeAll {
					selectedProxies = append(overseasProxies, domesticProxies...)
				} else {
					selectedProxies = overseasProxies
				}
			}

			// 排除过滤
			if pg.ExcludeFilter != "" {
				var filtered []string
				for _, name := range selectedProxies {
					excluded, _ := matchRegex(pg.ExcludeFilter, name)
					if !excluded {
						filtered = append(filtered, name)
					}
				}
				selectedProxies = filtered
			}

			// 国内分组必须包含 DIRECT 选项
			if pg.FilterMode == "geoip-cn" && pg.Filter == "cn" {
				// 确保 DIRECT 在最前面且不重复
				if len(selectedProxies) == 0 || selectedProxies[0] != "DIRECT" {
					selectedProxies = append([]string{"DIRECT"}, selectedProxies...)
				}
			}

			if len(selectedProxies) == 0 {
				selectedProxies = overseasProxies
			}
			group["proxies"] = selectedProxies

			// 添加其他参数 (url-test, fallback 等类型需要)
			if pg.Type != "select" && pg.Type != "relay" {
				if pg.URL != "" {
					group["url"] = pg.URL
				}
				if pg.Interval > 0 {
					group["interval"] = pg.Interval
				}
				if pg.Timeout > 0 {
					group["timeout"] = pg.Timeout
				}
				if pg.Tolerance > 0 {
					group["tolerance"] = pg.Tolerance
				}
				if pg.Lazy {
					group["lazy"] = pg.Lazy
				}
			}
			if pg.Hidden {
				group["hidden"] = pg.Hidden
			}
			if pg.Strategy != "" && pg.Type == "load-balance" {
				group["strategy"] = pg.Strategy
			}

			groups = append(groups, group)
		}
	}

	return groups
}

// matchRegex 安全的正则匹配
func matchRegex(pattern, text string) (bool, error) {
	if pattern == "" {
		return true, nil
	}
	re, err := regexp.Compile(pattern)
	if err != nil {
		return false, err
	}
	return re.MatchString(text), nil
}

// yamlOrderedMap 保持 YAML 键顺序
type yamlOrderedMap []yamlMapEntry
type yamlMapEntry struct {
	Key   string
	Value interface{}
}

func (m yamlOrderedMap) MarshalYAML() (interface{}, error) {
	node := &yaml.Node{Kind: yaml.MappingNode}
	for _, entry := range m {
		keyNode := &yaml.Node{Kind: yaml.ScalarNode, Value: entry.Key}
		valueNode, err := valueToYamlNode(entry.Value)
		if err != nil {
			return nil, err
		}
		node.Content = append(node.Content, keyNode, valueNode)
	}
	return node, nil
}

func valueToYamlNode(v interface{}) (*yaml.Node, error) {
	switch val := v.(type) {
	case string:
		return &yaml.Node{Kind: yaml.ScalarNode, Value: val}, nil
	case bool:
		return &yaml.Node{Kind: yaml.ScalarNode, Value: fmt.Sprintf("%t", val), Tag: "!!bool"}, nil
	case int:
		return &yaml.Node{Kind: yaml.ScalarNode, Value: strconv.Itoa(val), Tag: "!!int"}, nil
	case []string:
		node := &yaml.Node{Kind: yaml.SequenceNode}
		for _, s := range val {
			node.Content = append(node.Content, &yaml.Node{Kind: yaml.ScalarNode, Value: s})
		}
		return node, nil
	case []interface{}:
		node := &yaml.Node{Kind: yaml.SequenceNode}
		for _, item := range val {
			child, err := valueToYamlNode(item)
			if err != nil {
				return nil, err
			}
			node.Content = append(node.Content, child)
		}
		return node, nil
	case map[string]interface{}:
		// Use yaml.Marshal for generic maps — order doesn't matter as much for nested maps
		b, err := yaml.Marshal(val)
		if err != nil {
			return nil, err
		}
		var n yaml.Node
		if err := yaml.Unmarshal(b, &n); err != nil {
			return nil, err
		}
		if n.Kind == yaml.DocumentNode && len(n.Content) > 0 {
			return n.Content[0], nil
		}
		return &n, nil
	case yamlOrderedMap:
		n, err := val.MarshalYAML()
		if err != nil {
			return nil, err
		}
		return n.(*yaml.Node), nil
	default:
		// Fallback: marshal to yaml and parse back
		b, err := yaml.Marshal(val)
		if err != nil {
			return nil, err
		}
		var n yaml.Node
		if err := yaml.Unmarshal(b, &n); err != nil {
			return nil, err
		}
		if n.Kind == yaml.DocumentNode && len(n.Content) > 0 {
			return n.Content[0], nil
		}
		return &n, nil
	}
}

// buildMihomoBaseConfig 构建 Mihomo 基础配置
func buildMihomoBaseConfig(cfg *MihomoSubscriptionConfig) map[string]interface{} {
	config := map[string]interface{}{}

	if cfg == nil {
		// 没有配置则返回空，让调用方处理
		return config
	}

	c := cfg.Config

	// 基础设置 - 直接使用数据库配置
	config["mixed-port"] = c.MixedPort
	config["redir-port"] = c.RedirPort
	config["tproxy-port"] = c.TproxyPort
	config["mode"] = c.Mode
	config["bind-address"] = c.BindAddress
	config["ipv6"] = c.Ipv6
	config["allow-lan"] = c.AllowLan
	config["unified-delay"] = c.UnifiedDelay
	config["tcp-concurrent"] = c.TcpConcurrent
	config["log-level"] = c.LogLevel
	config["find-process-mode"] = c.FindProcessMode
	config["global-client-fingerprint"] = c.GlobalClientFingerprint
	config["external-controller"] = c.ExternalController
	config["external-ui"] = c.ExternalUi

	// profile
	config["profile"] = map[string]interface{}{
		"store-selected": c.Profile.StoreSelected,
		"store-fake-ip":  c.Profile.StoreFakeip,
	}

	// sniffer
	sniff := map[string]interface{}{}
	for protocol, s := range c.Sniffer.Sniff {
		ports := parsePortsString(s.Ports)
		sniff[protocol] = map[string]interface{}{
			"ports":                ports,
			"override-destination": s.OverrideDestination,
		}
	}
	config["sniffer"] = map[string]interface{}{
		"enable":               c.Sniffer.Enable,
		"override-destination": c.Sniffer.OverrideDestination,
		"sniff":                sniff,
	}

	// TUN
	config["tun"] = map[string]interface{}{
		"enable":                c.Tun.Enable,
		"device":                c.Tun.Device,
		"stack":                 c.Tun.Stack,
		"dns-hijack":            parseDnsHijackString(c.Tun.DnsHijack),
		"udp-timeout":           c.Tun.UdpTimeout,
		"auto-route":            c.Tun.AutoRoute,
		"auto-redirect":         c.Tun.AutoRedirect,
		"auto-detect-interface": c.Tun.AutoDetectInterface,
		"strict-route":          c.Tun.StrictRoute,
	}

	// DNS
	dns := map[string]interface{}{
		"enable":        c.DNS.Enable,
		"ipv6":          c.DNS.Ipv6,
		"listen":        c.DNS.Listen,
		"enhanced-mode": c.DNS.EnhancedMode,
		"fake-ip-range": c.DNS.FakeIpRange,
	}
	if c.DNS.FakeIpFilter != "" {
		dns["fake-ip-filter"] = parseMultilineString(c.DNS.FakeIpFilter)
	}
	if c.DNS.DefaultNameserver != "" {
		dns["default-nameserver"] = parseMultilineString(c.DNS.DefaultNameserver)
	}
	if c.DNS.Nameserver != "" {
		dns["nameserver"] = parseMultilineString(c.DNS.Nameserver)
	}
	if c.DNS.ProxyServerNameserver != "" {
		dns["proxy-server-nameserver"] = parseMultilineString(c.DNS.ProxyServerNameserver)
	}
	if c.DNS.NameserverPolicy != "" {
		dns["nameserver-policy"] = parseNameserverPolicy(c.DNS.NameserverPolicy)
	}
	config["dns"] = dns

	return config
}

// buildMihomoRuleProviders 构建规则集
func buildMihomoRuleProviders(cfg *MihomoSubscriptionConfig) map[string]interface{} {
	providers := map[string]interface{}{}

	if cfg == nil || len(cfg.Config.RuleProviders) == 0 {
		return providers
	}

	for _, rp := range cfg.Config.RuleProviders {
		providers[rp.Name] = map[string]interface{}{
			"type":     rp.Type,
			"behavior": rp.Behavior,
			"format":   rp.Format,
			"interval": rp.Interval,
			"path":     "./rule_provider/" + rp.Name + "." + rp.Format,
			"url":      rp.URL,
			"proxy":    rp.Proxy,
		}
	}

	return providers
}

// buildMihomoRules 构建路由规则
func buildMihomoRules(cfg *MihomoSubscriptionConfig) []string {
	var rules []string

	if cfg == nil || len(cfg.Config.Rules) == 0 {
		return rules
	}

	for _, rule := range cfg.Config.Rules {
		ruleStr := buildMihomoRuleString(rule)
		if ruleStr != "" {
			rules = append(rules, ruleStr)
		}
	}

	return rules
}

// buildMihomoRuleString 构建单条规则字符串
func buildMihomoRuleString(rule MihomoRule) string {
	// 处理联合规则 (AND, OR, NOT)
	if rule.Type == "AND" || rule.Type == "OR" || rule.Type == "NOT" {
		if len(rule.SubRules) == 0 {
			return ""
		}
		var subRuleStrs []string
		for _, sub := range rule.SubRules {
			subStr := fmt.Sprintf("(%s,%s)", sub.Type, sub.Value)
			subRuleStrs = append(subRuleStrs, subStr)
		}
		result := fmt.Sprintf("%s,(%s),%s", rule.Type, strings.Join(subRuleStrs, ","), rule.Outbound)
		return result
	}

	// 处理 MATCH 规则
	if rule.Type == "MATCH" {
		return fmt.Sprintf("MATCH,%s", rule.Outbound)
	}

	// 普通规则
	result := fmt.Sprintf("%s,%s,%s", rule.Type, rule.Value, rule.Outbound)
	if rule.NoResolve {
		result += ",no-resolve"
	}
	return result
}

// parseMultilineString 解析多行字符串
func parseMultilineString(s string) []string {
	if s == "" {
		return nil
	}
	lines := strings.Split(s, "\n")
	var result []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			result = append(result, line)
		}
	}
	return result
}

func parsePortsString(s string) []interface{} {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	var result []interface{}
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		// 检查是否为范围 (如 "8080-8880")
		if strings.Contains(p, "-") {
			result = append(result, p)
		} else if port, err := strconv.Atoi(p); err == nil {
			result = append(result, port)
		} else {
			result = append(result, p)
		}
	}
	return result
}

func parseDnsHijackString(s string) []string {
	if s == "" {
		return nil
	}
	return parseMultilineString(s)
}

func parseNameserverPolicy(s string) map[string]interface{} {
	result := map[string]interface{}{}
	lines := strings.Split(s, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// 格式: "rule-set:cn_domain: 223.5.5.5"
		parts := strings.SplitN(line, ":", 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			// 可能还有第二个冒号分隔 key 和 value
			remaining := strings.TrimSpace(parts[1])
			remaining = strings.TrimPrefix(remaining, ":")
			// 检查 remaining 是否还有 : (key 是 rule-set:xxx)
			if strings.Contains(key, "rule-set") || strings.Contains(key, "geosite") {
				colonIdx := strings.Index(remaining, ":")
				if colonIdx > 0 {
					key = key + ":" + remaining[:colonIdx]
					remaining = remaining[colonIdx+1:]
				}
			}
			remaining = strings.TrimSpace(remaining)
			result[key] = []string{remaining}
		}
	}
	return result
}
