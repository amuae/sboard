package handler

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"github.com/sboard-go/sboard/internal/database"
)

func generateV2RaySubscription(servers []ServerWithNodes, nodeConfigs map[uint]map[uint]*database.ServerNodeConfig, user *database.ProxyUser, lv int, externalNodes []database.ExternalNode) (string, error) {
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
				extraUUID, err := user.EnsureExtraUUID(ob.Slot)
				if err != nil {
					return "", fmt.Errorf("生成落地出站 UUID 失败: %w", err)
				}
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

	// 添加外部节点
	for _, ext := range externalNodes {
		link := buildV2RayLinkFromExternal(&ext)
		if link != "" {
			links = append(links, link)
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
		baseURL := fmt.Sprintf("ss://%s@%s:%d", userInfo, formatHostForURL(p.Host), p.Port)
		params := url.Values{}
		if node.SsObfsMode == "tls" || node.SsObfsMode == "http" {
			obfsHost := node.SsObfsHost
			if obfsHost == "" {
				obfsHost = node.ServerName
			}
			if obfsHost == "" {
				obfsHost = p.Host
			}
			pluginOpts := url.QueryEscape("obfs=" + node.SsObfsMode + ";obfs-host=" + obfsHost)
			params.Set("plugin", "obfs-local;"+pluginOpts)
		}
		if len(params) > 0 {
			return baseURL + "?" + params.Encode() + "#" + url.PathEscape(p.Name)
		}
		return baseURL + "#" + url.PathEscape(p.Name)

	case "hysteria2":
		password := p.UUID
		if node.Hy2Password != "" {
			password = node.Hy2Password
		}
		params := url.Values{}
		if node.ServerName != "" {
			params.Set("sni", node.ServerName)
		}
		params.Set("insecure", "1")
		if node.Hy2Obfs != "" {
			params.Set("obfs", node.Hy2Obfs)
			params.Set("obfs-password", node.Hy2ObfsPassword)
		}
		return fmt.Sprintf("hysteria2://%s@%s:%d?%s#%s", password, formatHostForURL(p.Host), p.Port, params.Encode(), url.PathEscape(p.Name))

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

func buildV2RayLinkFromExternal(ext *database.ExternalNode) string {
	switch ext.Protocol {
	case "vmess":
		vmessConfig := map[string]interface{}{
			"v":    "2",
			"ps":   ext.Name,
			"add":  ext.Host,
			"port": ext.Port,
			"id":   ext.UUID,
			"aid":  0,
			"scy":  "auto",
			"type": "none",
		}
		if ext.TransportEnabled && ext.TransportType != "" && ext.TransportType != "tcp" {
			if ext.TransportType == "http" {
				vmessConfig["net"] = "tcp"
				vmessConfig["type"] = "http"
				if ext.WsPath != "" {
					vmessConfig["path"] = ext.WsPath
				}
				if ext.TransportHost != "" {
					vmessConfig["host"] = ext.TransportHost
				}
			} else {
				vmessConfig["net"] = ext.TransportType
				if ext.TransportType == "ws" {
					if ext.WsPath != "" {
						vmessConfig["path"] = ext.WsPath
					}
					if ext.TransportHost != "" {
						vmessConfig["host"] = ext.TransportHost
					}
				} else if ext.TransportType == "grpc" && ext.GrpcService != "" {
					vmessConfig["path"] = ext.GrpcService
					vmessConfig["type"] = "gun"
				} else if ext.TransportType == "h2" {
					if ext.WsPath != "" {
						vmessConfig["path"] = ext.WsPath
					}
					if ext.TransportHost != "" {
						vmessConfig["host"] = ext.TransportHost
					}
				}
			}
		} else {
			vmessConfig["net"] = "tcp"
		}
		if ext.TlsEnabled {
			vmessConfig["tls"] = "tls"
			if ext.ServerName != "" {
				vmessConfig["sni"] = ext.ServerName
			}
		} else {
			vmessConfig["tls"] = ""
		}
		jsonData, _ := json.Marshal(vmessConfig)
		return "vmess://" + base64.StdEncoding.EncodeToString(jsonData)

	case "vless":
		params := url.Values{}
		if ext.TransportEnabled && ext.TransportType != "" && ext.TransportType != "tcp" {
			params.Set("type", ext.TransportType)
			if ext.TransportType == "ws" {
				if ext.WsPath != "" {
					params.Set("path", ext.WsPath)
				}
				if ext.TransportHost != "" {
					params.Set("host", ext.TransportHost)
				}
			} else if ext.TransportType == "grpc" && ext.GrpcService != "" {
				params.Set("serviceName", ext.GrpcService)
			}
		} else {
			params.Set("type", "tcp")
		}
		if ext.RealityEnabled && ext.RealityPubkey != "" {
			params.Set("security", "reality")
			params.Set("pbk", ext.RealityPubkey)
			params.Set("sid", ext.RealityShortId)
			if ext.RealityServer != "" {
				params.Set("sni", ext.RealityServer)
			}
			params.Set("fp", "chrome")
		} else if ext.TlsEnabled {
			params.Set("security", "tls")
			if ext.ServerName != "" {
				params.Set("sni", ext.ServerName)
			}
			params.Set("allowInsecure", "1")
			params.Set("fp", "chrome")
		} else {
			params.Set("security", "none")
		}
		if ext.Flow != "" {
			params.Set("flow", ext.Flow)
		}
		return fmt.Sprintf("vless://%s@%s:%d?%s#%s", ext.UUID, formatHostForURL(ext.Host), ext.Port, params.Encode(), url.PathEscape(ext.Name))

	case "trojan":
		params := url.Values{}
		if ext.TransportEnabled && ext.TransportType != "" && ext.TransportType != "tcp" {
			params.Set("type", ext.TransportType)
			if ext.TransportType == "ws" {
				if ext.WsPath != "" {
					params.Set("path", ext.WsPath)
				}
				if ext.TransportHost != "" {
					params.Set("host", ext.TransportHost)
				}
			} else if ext.TransportType == "grpc" && ext.GrpcService != "" {
				params.Set("serviceName", ext.GrpcService)
				params.Set("mode", "gun")
			}
		}
		if ext.ServerName != "" {
			params.Set("sni", ext.ServerName)
		} else {
			params.Set("sni", ext.Host)
		}
		params.Set("allowInsecure", "1")
		params.Set("fp", "chrome")
		return fmt.Sprintf("trojan://%s@%s:%d?%s#%s", ext.UUID, formatHostForURL(ext.Host), ext.Port, params.Encode(), url.PathEscape(ext.Name))

	case "anytls":
		params := url.Values{}
		if ext.ServerName != "" {
			params.Set("sni", ext.ServerName)
		}
		params.Set("insecure", "1")
		return fmt.Sprintf("anytls://%s@%s:%d?%s#%s", ext.UUID, formatHostForURL(ext.Host), ext.Port, params.Encode(), url.PathEscape(ext.Name))

	case "shadowsocks":
		userInfo := base64.StdEncoding.EncodeToString([]byte(ext.SsMethod + ":" + ext.SsPassword))
		return fmt.Sprintf("ss://%s@%s:%d#%s", userInfo, formatHostForURL(ext.Host), ext.Port, url.PathEscape(ext.Name))

	case "hysteria2":
		params := url.Values{}
		if ext.ServerName != "" {
			params.Set("sni", ext.ServerName)
		}
		params.Set("insecure", "1")
		if ext.Hy2Obfs != "" {
			params.Set("obfs", ext.Hy2Obfs)
			params.Set("obfs-password", ext.Hy2ObfsPassword)
		}
		return fmt.Sprintf("hysteria2://%s@%s:%d?%s#%s", ext.Hy2Password, formatHostForURL(ext.Host), ext.Port, params.Encode(), url.PathEscape(ext.Name))

	default:
		return ""
	}
}
