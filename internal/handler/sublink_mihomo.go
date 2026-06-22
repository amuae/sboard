package handler

import (
	"fmt"
	"strings"

	"github.com/sboard-go/sboard/internal/database"
	"gopkg.in/yaml.v3"
)

func generateMihomoSubscription(servers []ServerWithNodes, nodeConfigs map[uint]map[uint]*database.ServerNodeConfig, user *database.ProxyUser, lv int, externalNodes []database.ExternalNode) (string, error) {
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

	// 添加外部节点
	for _, ext := range externalNodes {
		proxy := buildMihomoProxyFromExternal(&ext)
		if proxy != nil {
			proxies = append(proxies, proxy)
			proxyName := proxy.GetName()
			overseasProxies = append(overseasProxies, proxyName)
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
		if node.SsObfsMode == "tls" || node.SsObfsMode == "http" {
			obfsHost := node.SsObfsHost
			if obfsHost == "" {
				obfsHost = node.ServerName
			}
			if obfsHost == "" {
				obfsHost = p.Host
			}
			add("plugin", "obfs")
			add("plugin-opts", map[string]interface{}{
				"mode": node.SsObfsMode,
				"host": obfsHost,
			})
		}

	case "hysteria2":
		password := p.UUID
		if node.Hy2Password != "" {
			password = node.Hy2Password
		}
		add("type", "hysteria2")
		add("name", p.Name)
		add("server", p.Host)
		add("port", p.Port)
		add("password", password)
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

// buildMihomoTransportOptsExt 构建外部节点的 Mihomo 传输层配置
func buildMihomoTransportOptsExt(ext *database.ExternalNode, add func(string, interface{})) {
	if !ext.TransportEnabled || ext.TransportType == "" || ext.TransportType == "tcp" {
		return
	}
	add("network", ext.TransportType)
	switch ext.TransportType {
	case "ws":
		wsOpts := map[string]interface{}{}
		if ext.WsPath != "" {
			wsOpts["path"] = ext.WsPath
		}
		if ext.TransportHost != "" {
			wsOpts["headers"] = map[string]interface{}{"Host": ext.TransportHost}
		}
		if len(wsOpts) > 0 {
			add("ws-opts", wsOpts)
		}
	case "grpc":
		if ext.GrpcService != "" {
			add("grpc-opts", map[string]interface{}{"grpc-service-name": ext.GrpcService})
		}
	case "h2":
		h2Opts := map[string]interface{}{}
		if ext.TransportHost != "" {
			h2Opts["host"] = []string{ext.TransportHost}
		}
		if ext.WsPath != "" {
			h2Opts["path"] = ext.WsPath
		}
		if len(h2Opts) > 0 {
			add("h2-opts", h2Opts)
		}
	case "http":
		httpOpts := map[string]interface{}{}
		if ext.TransportHost != "" {
			httpOpts["headers"] = map[string]interface{}{"Host": []string{ext.TransportHost}}
		}
		if ext.WsPath != "" {
			httpOpts["path"] = []string{ext.WsPath}
		}
		if len(httpOpts) > 0 {
			add("http-opts", httpOpts)
		}
	}
}

// buildMihomoTLSOptsExt 构建外部节点的 Mihomo TLS 配置
func buildMihomoTLSOptsExt(ext *database.ExternalNode, add func(string, interface{}), useSNI bool) {
	if useSNI {
		if ext.ServerName != "" {
			add("sni", ext.ServerName)
		}
	} else {
		if ext.RealityEnabled && ext.RealityServer != "" {
			add("servername", ext.RealityServer)
		} else if ext.ServerName != "" {
			add("servername", ext.ServerName)
		}
	}
	add("skip-cert-verify", true)
	add("client-fingerprint", "chrome")
	if ext.RealityEnabled && ext.RealityPubkey != "" {
		add("reality-opts", map[string]interface{}{
			"public-key": ext.RealityPubkey,
			"short-id":   ext.RealityShortId,
		})
	}
}

func buildMihomoProxyFromExternal(ext *database.ExternalNode) MihomoProxy {
	var proxy MihomoProxy
	add := func(key string, value interface{}) {
		proxy = append(proxy, MihomoProxyField{Key: key, Value: value})
	}

	switch ext.Protocol {
	case "vmess":
		add("type", "vmess")
		add("name", ext.Name)
		add("server", ext.Host)
		add("port", ext.Port)
		add("uuid", ext.UUID)
		add("alterId", 0)
		add("cipher", "auto")
		add("udp", true)
		buildMihomoTransportOptsExt(ext, add)
		if ext.TlsEnabled {
			add("tls", true)
			buildMihomoTLSOptsExt(ext, add, false)
		}

	case "vless":
		add("type", "vless")
		add("name", ext.Name)
		add("server", ext.Host)
		add("port", ext.Port)
		add("uuid", ext.UUID)
		add("udp", true)
		buildMihomoTransportOptsExt(ext, add)
		if ext.TlsEnabled || ext.RealityEnabled {
			add("tls", true)
			buildMihomoTLSOptsExt(ext, add, false)
		}
		if ext.Flow != "" {
			add("flow", ext.Flow)
		}

	case "trojan":
		add("type", "trojan")
		add("name", ext.Name)
		add("server", ext.Host)
		add("port", ext.Port)
		add("password", ext.UUID)
		add("udp", true)
		buildMihomoTransportOptsExt(ext, add)
		buildMihomoTLSOptsExt(ext, add, true)

	case "anytls":
		add("type", "anytls")
		add("name", ext.Name)
		add("server", ext.Host)
		add("port", ext.Port)
		add("password", ext.UUID)
		add("udp", true)
		buildMihomoTLSOptsExt(ext, add, true)

	case "shadowsocks":
		add("type", "ss")
		add("name", ext.Name)
		add("server", ext.Host)
		add("port", ext.Port)
		add("cipher", ext.SsMethod)
		add("password", ext.SsPassword)
		add("udp", true)

	case "hysteria2":
		add("type", "hysteria2")
		add("name", ext.Name)
		add("server", ext.Host)
		add("port", ext.Port)
		add("password", ext.Hy2Password)
		add("udp", true)
		buildMihomoTLSOptsExt(ext, add, true)
		if ext.Hy2Obfs != "" {
			add("obfs", ext.Hy2Obfs)
			add("obfs-password", ext.Hy2ObfsPassword)
		}

	default:
		return nil
	}

	return proxy
}

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
