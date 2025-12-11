package netstatic

import (
	"os"
	"path/filepath"
	"strings"
)

// 预定义常见的回环和虚拟接口名称前缀
var loopbackPrefixes = []string{
	"lo",        // Loopback
	"docker",    // Docker
	"br-",       // Docker bridge / OpenWrt bridge
	"veth",      // Docker veth
	"virbr",     // KVM
	"vmbr",      // Proxmox
	"cni",       // Kubernetes CNI
	"flannel",   // Flannel
	"podman",    // Podman
	"tun",       // TUN 设备
	"tap",       // TAP 设备
	"utun",      // macOS TUN
	"wg",        // WireGuard
	"tailscale", // Tailscale
	"eth-",      // sing-box TUN (eth-xxx)
	"sit",       // IPv6 over IPv4 tunnel
	"dummy",     // Dummy 接口
	"teql",      // Traffic equalizer
	"ppp",       // PPP/PPPoE 拨号接口（与物理口流量重复）
	"ipv6",      // IPv6 隧道接口（流量经物理口发送）
}

// 需要排除的包含字符串
var excludeContains = []string{
	"tun",
	"sing",
}

// isBridgeMember 检查网卡是否是网桥的成员（Linux 专用）
// 如果是网桥成员，流量会和网桥接口重复，应该排除
func isBridgeMember(nicName string) bool {
	// Linux: 检查 /sys/class/net/<nic>/master 是否存在
	// 如果存在，说明这个网卡是某个网桥/bond 的成员
	masterPath := filepath.Join("/sys/class/net", nicName, "master")
	if _, err := os.Stat(masterPath); err == nil {
		return true
	}

	// 也检查 /sys/class/net/<nic>/brport 目录
	// 这是网桥端口的标志
	brportPath := filepath.Join("/sys/class/net", nicName, "brport")
	if _, err := os.Stat(brportPath); err == nil {
		return true
	}

	return false
}

// ShouldIncludeNic 判断网卡是否应该被统计
func ShouldIncludeNic(nicName string) bool {
	// 检查前缀
	for _, prefix := range loopbackPrefixes {
		if strings.HasPrefix(nicName, prefix) {
			return false
		}
	}

	// 检查包含
	for _, s := range excludeContains {
		if strings.Contains(nicName, s) {
			return false
		}
	}

	// 检查是否是网桥成员（流量会重复）
	if isBridgeMember(nicName) {
		return false
	}

	return true
}

// GetIncludedNics 获取所有应该被统计的网卡名称
func GetIncludedNics(allNics []string) []string {
	var result []string
	for _, nic := range allNics {
		if ShouldIncludeNic(nic) {
			result = append(result, nic)
		}
	}
	return result
}
