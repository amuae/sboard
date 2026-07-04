package handler

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/sboard-go/sboard/internal/database"
)

func generateSingBoxSubscription(servers []ServerWithNodes, nodeConfigs map[uint]map[uint]*database.ServerNodeConfig, user *database.ProxyUser, lv int, externalNodes []database.ExternalNode) (string, error) {
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
				tag := safeStringFromMap(outbound, "tag")
				if tag == "" {
					continue
				}

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
					tag := safeStringFromMap(outboundProxy, "tag")
					if tag == "" {
						continue
					}
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

	// 添加外部节点
	for _, ext := range externalNodes {
		outbound := buildSingBoxOutboundFromExternal(&ext)
		if outbound != nil {
			nodeOutbounds = append(nodeOutbounds, outbound)
			tag := safeStringFromMap(outbound, "tag")
			if tag == "" {
				continue
			}
			// 使用 Country 字段判断国家分组
			if ext.Country != "" {
				countryCode := strings.ToUpper(ext.Country)
				countryProxyTags[countryCode] = append(countryProxyTags[countryCode], tag)
			}
			overseasProxyTags = append(overseasProxyTags, tag)
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
		"strategy":        tpl.DNS.Strategy,
		"reverse_mapping": tpl.DNS.ReverseMapping,
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
						tag := safeStringFromMap(nodeOutbound, "tag")
						if tag != "" && pattern.MatchString(tag) {
							selectedTags = append(selectedTags, tag)
						}
					}
				}
			}
		case "all":
			// 所有节点
			for _, nodeOutbound := range nodeOutbounds {
				tag := safeStringFromMap(nodeOutbound, "tag")
				if tag != "" {
					selectedTags = append(selectedTags, tag)
				}
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

// buildSingBoxDNSRule 构建 SingBox DNS 规则（sing-box 1.14 显式 action）
func buildSingBoxDNSRule(rule SingBoxDNSRule) map[string]interface{} {
	result := map[string]interface{}{}

	// sing-box 1.14: DNS 规则需要显式 action
	// route → server（默认行为）, respond → 直接响应, evaluate → 评估
	if rule.Action == "" && rule.Server != "" {
		// 有 server 字段时默认 action=route
		result["action"] = "route"
	} else if rule.Action != "" {
		result["action"] = rule.Action
	}
	if rule.Server != "" {
		result["server"] = rule.Server
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

// buildSingBoxRouteRule 构建 SingBox 路由规则（sing-box 1.14 显式 action）
func buildSingBoxRouteRule(rule SingBoxRouteRule) map[string]interface{} {
	result := make(map[string]interface{})

	// sing-box 1.14: 路由规则使用显式 action 替代直接 outbound 字段
	// route → 携带 outbound 参数（默认行为）
	// direct → 直连, reject → 拒绝, hijack_dns → DNS 劫持, resolve → 解析 DNS
	if rule.Action != "" {
		result["action"] = rule.Action
	} else if rule.Outbound != "" {
		// 有 outbound 未设 action 时默认为 route
		result["action"] = "route"
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
		if node.SsObfsMode == "tls" || node.SsObfsMode == "http" {
			obfsHost := node.SsObfsHost
			if obfsHost == "" {
				obfsHost = node.ServerName
			}
			if obfsHost == "" {
				obfsHost = p.Host
			}
			outbound["plugin"] = "obfs-local"
			outbound["plugin_opts"] = fmt.Sprintf("obfs=%s;obfs-host=%s", node.SsObfsMode, obfsHost)
		}

	case "hysteria2":
		password := p.UUID
		if node.Hy2Password != "" {
			password = node.Hy2Password
		}
		outbound["type"] = "hysteria2"
		outbound["password"] = password
		outbound["up_mbps"] = node.Hy2UpMbps
		outbound["down_mbps"] = node.Hy2DownMbps
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

// buildSingBoxTLSExt 构建外部节点的 sing-box TLS 配置
func buildSingBoxTLSExt(ext *database.ExternalNode) map[string]interface{} {
	tls := map[string]interface{}{
		"enabled":  true,
		"insecure": true,
	}
	if ext.ServerName != "" {
		tls["server_name"] = ext.ServerName
	}
	if ext.RealityEnabled && ext.RealityPubkey != "" {
		if ext.RealityServer != "" {
			tls["server_name"] = ext.RealityServer
		}
		tls["reality"] = map[string]interface{}{
			"enabled":    true,
			"public_key": ext.RealityPubkey,
			"short_id":   ext.RealityShortId,
		}
	}
	tls["utls"] = map[string]interface{}{
		"enabled":     true,
		"fingerprint": "chrome",
	}
	return tls
}

// buildSingBoxTransportExt 构建外部节点的 sing-box 传输层配置
func buildSingBoxTransportExt(ext *database.ExternalNode) map[string]interface{} {
	transport := map[string]interface{}{
		"type": ext.TransportType,
	}
	switch ext.TransportType {
	case "ws":
		if ext.WsPath != "" {
			transport["path"] = ext.WsPath
		}
		if ext.TransportHost != "" {
			transport["headers"] = map[string]interface{}{
				"Host": ext.TransportHost,
			}
		}
	case "grpc":
		if ext.GrpcService != "" {
			transport["service_name"] = ext.GrpcService
		}
	case "http", "h2":
		transport["type"] = "http"
		if ext.TransportHost != "" {
			transport["host"] = []string{ext.TransportHost}
		}
		if ext.WsPath != "" {
			transport["path"] = ext.WsPath
		}
	case "httpupgrade":
		if ext.WsPath != "" {
			transport["path"] = ext.WsPath
		}
		if ext.TransportHost != "" {
			transport["host"] = ext.TransportHost
		}
	}
	return transport
}

func buildSingBoxOutboundFromExternal(ext *database.ExternalNode) SingBoxOutbound {
	outbound := SingBoxOutbound{
		"tag":         ext.Name,
		"server":      ext.Host,
		"server_port": ext.Port,
	}

	switch ext.Protocol {
	case "vmess":
		outbound["type"] = "vmess"
		outbound["uuid"] = ext.UUID
		outbound["security"] = "auto"
		outbound["alter_id"] = 0
		if ext.TransportEnabled && ext.TransportType != "" && ext.TransportType != "tcp" {
			outbound["transport"] = buildSingBoxTransportExt(ext)
		}
		if ext.TlsEnabled {
			outbound["tls"] = buildSingBoxTLSExt(ext)
		}

	case "vless":
		outbound["type"] = "vless"
		outbound["uuid"] = ext.UUID
		if ext.Flow != "" {
			outbound["flow"] = ext.Flow
		}
		if ext.TransportEnabled && ext.TransportType != "" && ext.TransportType != "tcp" {
			outbound["transport"] = buildSingBoxTransportExt(ext)
		}
		if ext.TlsEnabled || ext.RealityEnabled {
			outbound["tls"] = buildSingBoxTLSExt(ext)
		}

	case "trojan":
		outbound["type"] = "trojan"
		outbound["password"] = ext.UUID
		if ext.TransportEnabled && ext.TransportType != "" && ext.TransportType != "tcp" {
			outbound["transport"] = buildSingBoxTransportExt(ext)
		}
		outbound["tls"] = buildSingBoxTLSExt(ext)

	case "shadowsocks":
		outbound["type"] = "shadowsocks"
		outbound["method"] = ext.SsMethod
		outbound["password"] = ext.SsPassword

	case "hysteria2":
		outbound["type"] = "hysteria2"
		outbound["password"] = ext.Hy2Password
		outbound["up_mbps"] = ext.Hy2UpMbps
		outbound["down_mbps"] = ext.Hy2DownMbps
		outbound["tls"] = buildSingBoxTLSExt(ext)
		if ext.Hy2Obfs != "" {
			outbound["obfs"] = map[string]interface{}{
				"type":     ext.Hy2Obfs,
				"password": ext.Hy2ObfsPassword,
			}
		}

	case "anytls":
		outbound["type"] = "anytls"
		outbound["password"] = ext.UUID
		outbound["tls"] = buildSingBoxTLSExt(ext)

	default:
		return nil
	}

	return outbound
}
