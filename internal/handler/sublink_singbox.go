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
	subscriptionConfig, err := GetEnabledSingBoxConfig()
	if err != nil {
		return "", fmt.Errorf("获取 SingBox 配置失败: %v", err)
	}
	if subscriptionConfig == nil {
		return "", fmt.Errorf("未发现 SingBox 订阅配置，请先在后台配置并启用")
	}

	tpl := subscriptionConfig.Config

	// 收集 node outbounds 和分类标签
	var nodeOutbounds []SingBoxOutbound
	countryProxyTags := make(map[string][]string)
	var domesticProxyTags []string
	var overseasProxyTags []string

	for _, swn := range servers {
		for _, node := range swn.Nodes {
			if !node.Enabled {
				continue
			}
			var nc *database.ServerNodeConfig
			if serverConfigs, ok := nodeConfigs[swn.Server.ID]; ok {
				nc = serverConfigs[node.ID]
			}
			outbound := buildSingBoxOutbound(&swn.Server, &node, nc, user, lv, nil)
			if outbound != nil {
				nodeOutbounds = append(nodeOutbounds, outbound)
				tag := safeStringFromMap(outbound, "tag")
				if tag == "" {
					continue
				}
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
	for _, ext := range externalNodes {
		outbound := buildSingBoxOutboundFromExternal(&ext)
		if outbound != nil {
			nodeOutbounds = append(nodeOutbounds, outbound)
			tag := safeStringFromMap(outbound, "tag")
			if tag == "" {
				continue
			}
			if ext.Country != "" {
				countryCode := strings.ToUpper(ext.Country)
				countryProxyTags[countryCode] = append(countryProxyTags[countryCode], tag)
			}
			overseasProxyTags = append(overseasProxyTags, tag)
		}
	}

	config := make(map[string]interface{})

	// Log
	config["log"] = map[string]interface{}{
		"disabled":  tpl.Log.Disabled,
		"level":     tpl.Log.Level,
		"output":    tpl.Log.Output,
		"timestamp": tpl.Log.Timestamp,
	}

	// DNS
	{
		dnsConfig := map[string]interface{}{
			"strategy":        tpl.DNS.Strategy,
			"reverse_mapping": tpl.DNS.ReverseMapping,
			"optimistic":      tpl.DNS.Optimistic,
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
		if tpl.DNS.CacheCapacity > 0 {
			dnsConfig["cache_capacity"] = tpl.DNS.CacheCapacity
		}

		var dnsServers []map[string]interface{}
		for _, srv := range tpl.DNS.Servers {
			server := map[string]interface{}{
				"tag":  srv.Tag,
				"type": srv.Type,
			}
			switch srv.Type {
			case "fakeip":
				if srv.Inet4Range != "" {
					server["inet4_range"] = srv.Inet4Range
				}
				if srv.Inet6Range != "" {
					server["inet6_range"] = srv.Inet6Range
				}
			case "group":
				server["servers"] = srv.Servers
			case "hosts":
				if srv.Predefined != nil {
					server["predefined"] = srv.Predefined
				}
				fallthrough
			default:
				if srv.Server != "" {
					server["server"] = srv.Server
				}
				if srv.ServerPort > 0 {
					server["server_port"] = srv.ServerPort
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

		var dnsRules []map[string]interface{}
		for _, rule := range tpl.DNS.Rules {
			dnsRule := buildSingBoxDNSRule(rule)
			if dnsRule != nil {
				dnsRules = append(dnsRules, dnsRule)
			}
		}
		dnsConfig["rules"] = dnsRules
		config["dns"] = dnsConfig
	}

	// NTP
	if tpl.NTP.Enabled {
		config["ntp"] = map[string]interface{}{
			"enabled":     true,
			"interval":    tpl.NTP.Interval,
			"server":      tpl.NTP.Server,
			"server_port": tpl.NTP.ServerPort,
		}
	}

	// Inbound (TUN)
	if tpl.Inbound.TunEnable {
		addresses := []string{}
		if tpl.Inbound.AddressIPv4 != "" {
			addresses = append(addresses, tpl.Inbound.AddressIPv4)
		}
		if tpl.Inbound.AddressIPv6 != "" {
			addresses = append(addresses, tpl.Inbound.AddressIPv6)
		}
		config["inbounds"] = []map[string]interface{}{{
			"type":           "tun",
			"tag":            "tun-in",
			"interface_name": tpl.Inbound.InterfaceName,
			"mtu":            tpl.Inbound.MTU,
			"address":        addresses,
			"stack":          tpl.Inbound.Stack,
			"auto_route":     tpl.Inbound.AutoRoute,
			"auto_redirect":  tpl.Inbound.AutoRedirect,
			"strict_route":   tpl.Inbound.StrictRoute,
		}}
	}

	// Outbounds
	outbounds := []map[string]interface{}{
		{"type": "direct", "tag": "DIRECT"},
	}
	for _, group := range tpl.OutboundGroups {
		outbound := map[string]interface{}{
			"type": group.Type,
			"tag":  group.Tag,
		}
		var selectedTags []string
		switch group.FilterMode {
		case "geoip-cn":
			switch group.Filter {
			case "cn":
				selectedTags = append([]string{"DIRECT"}, domesticProxyTags...)
			case "!cn":
				selectedTags = overseasProxyTags
			default:
				selectedTags = overseasProxyTags
			}
		case "geoip-country":
			countries := strings.Split(group.Filter, ",")
			for _, country := range countries {
				country = strings.TrimSpace(strings.ToUpper(country))
				if tags, ok := countryProxyTags[country]; ok {
					selectedTags = append(selectedTags, tags...)
				}
			}
		case "regex":
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
			for _, nodeOutbound := range nodeOutbounds {
				tag := safeStringFromMap(nodeOutbound, "tag")
				if tag != "" {
					selectedTags = append(selectedTags, tag)
				}
			}
		default:
			if group.Include != "" {
				// selector 模式：使用 include 正则匹配所有节点
				for _, nodeOutbound := range nodeOutbounds {
					tag := safeStringFromMap(nodeOutbound, "tag")
					if tag != "" {
						selectedTags = append(selectedTags, tag)
					}
				}
				if group.Include != "(?i)-" {
					outbound["include"] = group.Include
				}
			} else {
				selectedTags = overseasProxyTags
			}
		}
		if len(selectedTags) > 0 {
			if group.Type == "direct" {
				// direct 类型不需要 outbounds 列表
			} else {
				outbound["outbounds"] = selectedTags
			}
		}
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
	for _, nodeOutbound := range nodeOutbounds {
		outbounds = append(outbounds, nodeOutbound)
	}
	config["outbounds"] = outbounds

	// Route
	{
		routeConfig := map[string]interface{}{
			"final":                   tpl.Route.Final,
			"auto_detect_interface":   tpl.Route.AutoDetectInterface,
			"default_domain_resolver": tpl.Route.DefaultDomainResolver,
		}
		if tpl.Route.FindProcess {
			routeConfig["find_process"] = true
		}
		if tpl.Route.DefaultHttpClient != "" {
			routeConfig["default_http_client"] = tpl.Route.DefaultHttpClient
		}

		var routeRules []map[string]interface{}
		for _, rule := range tpl.Route.Rules {
			routeRule := buildSingBoxRouteRule(rule)
			if routeRule != nil {
				routeRules = append(routeRules, routeRule)
			}
		}
		routeConfig["rules"] = routeRules

		var ruleSets []map[string]interface{}
		for _, rs := range tpl.Route.RuleSets {
			ruleSet := map[string]interface{}{
				"tag":  rs.Tag,
				"type": rs.Type,
			}
			if rs.Path != "" {
				ruleSet["path"] = rs.Path
			}
			if rs.URL != "" {
				ruleSet["url"] = rs.URL
			}
			ruleSets = append(ruleSets, ruleSet)
		}
		routeConfig["rule_set"] = ruleSets
		config["route"] = routeConfig
	}

	// Experimental
	expConfig := map[string]interface{}{
		"cache_file": map[string]interface{}{
			"enabled":      tpl.Experimental.CacheFileEnabled,
			"store_fakeip": tpl.Experimental.StoreFakeip,
			"store_rdrc":   tpl.Experimental.StoreRdrc,
		},
		"clash_api": map[string]interface{}{
			"external_controller": tpl.Experimental.ExternalController,
			"external_ui":         tpl.Experimental.ExternalUi,
		},
	}
	if tpl.Experimental.ExternalUiDownloadUrl != "" {
		expConfig["clash_api"].(map[string]interface{})["external_ui_download_url"] = tpl.Experimental.ExternalUiDownloadUrl
	}
	if tpl.Experimental.ExternalUiHttpClient != "" {
		expConfig["clash_api"].(map[string]interface{})["external_ui_http_client"] = tpl.Experimental.ExternalUiHttpClient
	}
	if tpl.Experimental.DefaultMode != "" {
		expConfig["clash_api"].(map[string]interface{})["default_mode"] = tpl.Experimental.DefaultMode
	}
	if tpl.Experimental.UrlTestUnifiedDelay {
		expConfig["urltest_unified_delay"] = true
	}
	config["experimental"] = expConfig

	// HTTP Clients
	if len(tpl.HttpClients) > 0 {
		var httpClients []map[string]interface{}
		for _, hc := range tpl.HttpClients {
			client := map[string]interface{}{
				"tag":     hc.Tag,
				"version": hc.Version,
			}
			if len(hc.Headers) > 0 {
				client["headers"] = hc.Headers
			}
			if hc.Detour != "" {
				client["detour"] = hc.Detour
			}
			httpClients = append(httpClients, client)
		}
		config["http_clients"] = httpClients
	}

	// Services
	if len(tpl.Services) > 0 {
		var services []map[string]interface{}
		for _, svc := range tpl.Services {
			service := map[string]interface{}{
				"type":        svc.Type,
				"listen":      svc.Listen,
				"listen_port": svc.ListenPort,
			}
			services = append(services, service)
		}
		config["services"] = services
	}

	// Providers (注入订阅链接)
	if len(tpl.Providers) > 0 {
		var providers []map[string]interface{}
		for _, p := range tpl.Providers {
			provider := map[string]interface{}{
				"tag":    p.Tag,
				"type":   p.Type,
				"path":   p.Path,
				"http_client": p.HttpClient,
				"update_interval": p.UpdateInterval,
			}
			if p.URL != "" {
				provider["url"] = p.URL
			}
			providers = append(providers, provider)
		}
		config["providers"] = providers
	}

	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// buildSingBoxDNSRule 构建 SingBox DNS 规则（1.14）
func buildSingBoxDNSRule(rule SingBoxDNSRule) map[string]interface{} {
	result := map[string]interface{}{}
	if rule.Action == "" && rule.Server != "" {
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

// buildSingBoxRouteRule 构建 SingBox 路由规则（1.14）
func buildSingBoxRouteRule(rule SingBoxRouteRule) map[string]interface{} {
	result := make(map[string]interface{})

	if rule.Action != "" {
		result["action"] = rule.Action
	} else if rule.Outbound != "" {
		result["action"] = "route"
	}
	if rule.Outbound != "" {
		result["outbound"] = rule.Outbound
	}
	if rule.Action == "resolve" && rule.MatchOnly {
		result["match_only"] = true
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
	case "protocol":
		result["protocol"] = rule.Value
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
		if node.RealityEnabled && node.RealityPubkey != "" {
			if node.RealityServer != "" {
				naiveTLS["server_name"] = node.RealityServer
			}
			naiveTLS["utls"] = map[string]interface{}{
				"enabled":     true,
				"fingerprint": "chrome",
			}
		}
		outbound["tls"] = naiveTLS
	}

	return outbound
}

// buildSingBoxTLS 构建 sing-box TLS 配置
func buildSingBoxTLS(node *database.InboundNode) map[string]interface{} {
	tls := map[string]interface{}{
		"enabled":  true,
		"insecure": true,
	}
	if node.ServerName != "" {
		tls["server_name"] = node.ServerName
	}
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

// buildSingBoxOutboundFromExternal 构建外部节点出站
func buildSingBoxOutboundFromExternal(ext *database.ExternalNode) SingBoxOutbound {
	outbound := SingBoxOutbound{
		"tag":         ext.Name,
		"server":      ext.Host,
		"server_port": ext.Port,
		"type":        ext.Protocol,
	}

	switch ext.Protocol {
	case "trojan":
		outbound["password"] = ext.UUID
		if ext.TlsEnabled || ext.RealityEnabled {
			outbound["tls"] = buildSingBoxTLSExt(ext)
		}
		if ext.TransportEnabled && ext.TransportType == "ws" {
			transport := map[string]interface{}{"type": "ws"}
			if ext.WsPath != "" {
				transport["path"] = ext.WsPath
			}
			if ext.TransportHost != "" {
				transport["headers"] = map[string]interface{}{"Host": ext.TransportHost}
			}
			outbound["transport"] = transport
		}
	case "vless":
		outbound["uuid"] = ext.UUID
		if ext.Flow != "" {
			outbound["flow"] = ext.Flow
		}
		if ext.TlsEnabled || ext.RealityEnabled {
			outbound["tls"] = buildSingBoxTLSExt(ext)
		}
	case "vmess":
		outbound["uuid"] = ext.UUID
		outbound["security"] = "auto"
		outbound["alter_id"] = 0
		if ext.TlsEnabled {
			outbound["tls"] = buildSingBoxTLSExt(ext)
		}
		if ext.TransportEnabled && ext.TransportType == "ws" {
			transport := map[string]interface{}{"type": "ws"}
			if ext.WsPath != "" {
				transport["path"] = ext.WsPath
			}
			if ext.TransportHost != "" {
				transport["headers"] = map[string]interface{}{"Host": ext.TransportHost}
			}
			outbound["transport"] = transport
		}
	case "shadowsocks":
		outbound["method"] = ext.SsMethod
		outbound["password"] = ext.SsPassword
	case "hysteria2":
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
		outbound["password"] = ext.UUID
		if ext.TlsEnabled {
			outbound["tls"] = buildSingBoxTLSExt(ext)
		}
	}
	return outbound
}
