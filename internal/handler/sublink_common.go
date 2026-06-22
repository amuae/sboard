package handler

import (
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"net/http"
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

	// 获取外部节点（按用户等级过滤）
	var externalNodes []database.ExternalNode
	extQuery := database.DB.Where("enabled = ?", true)
	extQuery.Order("sort_order ASC, id ASC").Find(&externalNodes)

	// 按等级过滤
	var filteredExternalNodes []database.ExternalNode
	for _, ext := range externalNodes {
		if ext.Level <= user.Level {
			filteredExternalNodes = append(filteredExternalNodes, ext)
		}
	}
	externalNodes = filteredExternalNodes

	if len(userNodes) == 0 && len(externalNodes) == 0 {
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
		content, err = generateMihomoSubscription(serversWithNodes, serverNodeConfigs, &user, lv, externalNodes)
		contentType = "text/yaml; charset=utf-8"
	case "singbox", "sing-box":
		content, err = generateSingBoxSubscription(serversWithNodes, serverNodeConfigs, &user, lv, externalNodes)
		contentType = "application/json; charset=utf-8"
	case "v2ray", "base64":
		content, err = generateV2RaySubscription(serversWithNodes, serverNodeConfigs, &user, lv, externalNodes)
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
