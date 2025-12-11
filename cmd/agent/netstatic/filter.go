package netstatic

import (
	"strings"
)

// 预定义常见的回环和虚拟接口名称前缀
var loopbackPrefixes = []string{
	"lo",        // Loopback
	"docker",    // Docker
	"br-",       // Docker bridge
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
}

// 需要排除的包含字符串
var excludeContains = []string{
	"tun",
	"sing",
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
