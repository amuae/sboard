package handler

import (
	"net"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/sboard-go/sboard/internal/database"
	"github.com/sboard-go/sboard/internal/geoip"
)

// ========== UA 检测 ==========

// detectUserAgent 根据 User-Agent 检测客户端类型
func detectUserAgent(c *gin.Context) string {
	ua := strings.ToLower(c.GetHeader("User-Agent"))

	if strings.Contains(ua, "sing-box") || strings.Contains(ua, "singbox") {
		return "singbox"
	}
	if strings.Contains(ua, "clash") || strings.Contains(ua, "mihomo") || strings.Contains(ua, "stash") {
		return "mihomo"
	}
	if strings.Contains(ua, "shadowrocket") {
		return "v2ray"
	}
	if strings.Contains(ua, "v2ray") || strings.Contains(ua, "v2rayng") || strings.Contains(ua, "quantumult") {
		return "v2ray"
	}
	if strings.Contains(ua, "surge") {
		return "v2ray"
	}
	if strings.Contains(ua, "loon") {
		return "v2ray"
	}
	if strings.Contains(ua, "passwall") {
		return "v2ray"
	}

	// 默认返回 mihomo
	return "mihomo"
}

// ========== 用户等级降级策略 ==========

// categoryToLevel 将 category 字符串转为等级数字
// direct -> 1, relay -> 2, home -> 3
func categoryToLevel(category string) int {
	switch strings.ToLower(category) {
	case "direct":
		return 1
	case "relay":
		return 2
	case "home":
		return 3
	default:
		return 1
	}
}

// filterServersByLevel 根据 lv 参数过滤服务器
// 降级策略：
// lv1: 服务器 category direct(1)+relay(2)，使用一级节点名
// lv2: 服务器 category direct(1)+relay(2)，使用二级节点名
// lv3: 服务器 category direct+relay+home(全部)，category direct/relay 用二级节点名，category home 用三级节点名
func filterServersByLevel(servers []database.Server, userLevel int, lv int) []database.Server {
	var filtered []database.Server

	// 确定实际使用的等级（用户等级和 lv 参数取较小值）
	effectiveLevel := userLevel
	if lv > 0 && lv < userLevel {
		effectiveLevel = lv
	}

	for _, server := range servers {
		serverLevel := categoryToLevel(server.Category)

		if effectiveLevel == 3 {
			// lv3: 获取所有服务器 (category 1+2+3)
			filtered = append(filtered, server)
		} else {
			// lv1 和 lv2: 只获取 category direct(1) 和 relay(2)
			if serverLevel == 1 || serverLevel == 2 {
				filtered = append(filtered, server)
			}
		}
	}

	return filtered
}

// ========== 节点名称策略 ==========

// getNodeName 根据 lv 参数和服务器 category 返回节点名称
// 降级策略：
// lv1: 使用一级节点名 (node_1)
// lv2: 使用二级节点名 (node_2)
// lv3: category direct/relay 使用二级节点名，category home 使用三级节点名
func getNodeName(server *database.Server, userLevel int, lv int) string {
	// 确定实际使用的等级
	effectiveLevel := userLevel
	if lv > 0 && lv < userLevel {
		effectiveLevel = lv
	}

	serverLevel := categoryToLevel(server.Category)

	switch effectiveLevel {
	case 1:
		// lv1: 使用一级节点名
		if server.Node1 != "" {
			return server.Node1
		}
	case 2:
		// lv2: 使用二级节点名
		if server.Node2 != "" {
			return server.Node2
		}
	case 3:
		// lv3: category home(3) 用三级节点名，其他用二级节点名
		if serverLevel == 3 {
			if server.Node3 != "" {
				return server.Node3
			}
		} else {
			if server.Node2 != "" {
				return server.Node2
			}
		}
	}

	// 默认返回服务器名称
	return server.Name
}

// ========== GeoIP 判断 ==========

// getNodeEffectiveIP 获取节点的有效 IP（优先使用落地出站 IP，否则用 Agent 上报 IP）
func getNodeEffectiveIP(server *database.Server, _ *database.InboundNode, nc *database.ServerNodeConfig) string {
	// 如果配置了落地出站，使用出站 IP
	if nc != nil && nc.OutboundEnabled && nc.OutboundHost != "" {
		return nc.OutboundHost
	}

	// 使用服务器 Agent 上报的 IP
	return server.Host
}


// isDomesticByIP 通过 GeoIP 判断 IP 是否为国内
func isDomesticByIP(ipOrHost string) bool {
	// 如果 GeoIP 数据库未加载，回退到节点名判断
	if !geoip.IsLoaded() {
		return false // 无法判断时默认为国外
	}

	// 不是 IP 则无法判断
	if net.ParseIP(ipOrHost) == nil {
		return false
	}

	// 内网 IP 默认视为国内（云服务器 VPC 内网 IP）
	if isPrivateIP(ipOrHost) {
		return true
	}

	return geoip.IsChinaIP(ipOrHost)
}

// isPrivateIP 判断是否为内网 IP
func isPrivateIP(ipStr string) bool {
	ip := net.ParseIP(ipStr)
	if ip == nil {
		return false
	}

	// 检查私有地址段
	privateBlocks := []string{
		"10.0.0.0/8",
		"172.16.0.0/12",
		"192.168.0.0/16",
		"100.64.0.0/10",  // CGNAT
		"169.254.0.0/16", // Link-local
	}

	for _, block := range privateBlocks {
		_, cidr, _ := net.ParseCIDR(block)
		if cidr.Contains(ip) {
			return true
		}
	}
	return false
}

// formatHostForURL 格式化 host 用于 URL，IPv6 地址需要加方括号
func formatHostForURL(host string) string {
	ip := net.ParseIP(host)
	if ip != nil && ip.To4() == nil {
		// 是 IPv6 地址，加方括号
		return "[" + host + "]"
	}
	return host
}

// getCountryCode 通过 GeoIP 获取 IP 所属国家代码
func getCountryCode(ipOrHost string) string {
	// 如果 GeoIP 数据库未加载，返回空
	if !geoip.IsLoaded() {
		return ""
	}

	// 不是 IP 则无法判断
	if net.ParseIP(ipOrHost) == nil {
		return ""
	}

	// 内网 IP 默认视为国内
	if isPrivateIP(ipOrHost) {
		return "CN"
	}

	return geoip.LookupCountry(ipOrHost)
}
