package handler

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/sboard-go/sboard/internal/database"
	"gopkg.in/yaml.v3"
)

// handleGetDeployFolder 获取部署目录（ZIP包含配置文件、证书、部署脚本等）
func (s *Server) handleGetDeployFolder(c *gin.Context) {
	serverID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		errorJSON(c, http.StatusBadRequest, "无效的服务器ID")
		return
	}

	var server database.Server
	if err := database.DB.First(&server, serverID).Error; err != nil {
		errorJSON(c, http.StatusNotFound, "服务器不存在")
		return
	}

	coreType := c.Query("type")
	if coreType == "" {
		coreType = server.CoreType
		if coreType == "" {
			coreType = "sing-box"
		}
	}

	// 获取服务器节点和配置
	nodes, nodeConfigs := getServerNodesAndConfigs(uint(serverID))

	// 创建 ZIP 文件
	buf := new(bytes.Buffer)
	zipWriter := zip.NewWriter(buf)

	// 1. 添加配置文件
	var configContent []byte
	var configFilename string
	if coreType == "mihomo" {
		configContent, err = generateMihomoServerConfig(server, nodes, nodeConfigs)
		configFilename = "config.yaml"
	} else {
		configContent, err = generateSingBoxServerConfig(server, nodes, nodeConfigs)
		configFilename = "config.json"
	}
	if err != nil {
		errorJSON(c, http.StatusInternalServerError, "生成配置失败: "+err.Error())
		return
	}
	addFileToZip(zipWriter, configFilename, configContent)

	// 2. 添加部署目录中的文件（二进制、证书、服务文件等）
	storagePath := filepath.Join("storage", "configs", coreType)
	addDirectoryToZip(zipWriter, storagePath, "")

	zipWriter.Close()

	filename := fmt.Sprintf("%s-deploy-%s.zip", server.Name, coreType)
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
	c.Data(http.StatusOK, "application/zip", buf.Bytes())
}

// getServerNodesAndConfigs 获取服务器关联的节点和配置
func getServerNodesAndConfigs(serverID uint) ([]database.InboundNode, map[uint]database.ServerNodeConfig) {
	var server database.Server
	database.DB.First(&server, serverID)

	// 解析节点ID（Node1/2/3 存储的是字符串形式的 ID）
	nodeIDs := []uint{}
	for _, nodeStr := range []string{server.Node1, server.Node2, server.Node3} {
		if nodeStr != "" {
			if id, err := strconv.ParseUint(nodeStr, 10, 64); err == nil && id > 0 {
				nodeIDs = append(nodeIDs, uint(id))
			}
		}
	}

	// 获取节点
	var nodes []database.InboundNode
	if len(nodeIDs) > 0 {
		database.DB.Where("id IN ?", nodeIDs).Find(&nodes)
	}

	// 获取节点配置
	var nodeConfigs []database.ServerNodeConfig
	database.DB.Where("server_id = ?", serverID).Find(&nodeConfigs)

	configMap := make(map[uint]database.ServerNodeConfig)
	for _, nc := range nodeConfigs {
		configMap[nc.NodeID] = nc
	}

	return nodes, configMap
}

// addFileToZip 添加文件到 ZIP
func addFileToZip(w *zip.Writer, filename string, data []byte) error {
	f, err := w.Create(filename)
	if err != nil {
		return err
	}
	_, err = f.Write(data)
	return err
}

// addDirectoryToZip 添加目录到 ZIP
func addDirectoryToZip(w *zip.Writer, dirPath, prefix string) error {
	return filepath.Walk(dirPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // 忽略错误
		}
		if info.IsDir() {
			return nil
		}

		// 计算相对路径
		relPath, _ := filepath.Rel(dirPath, path)
		if prefix != "" {
			relPath = filepath.Join(prefix, relPath)
		}

		// 读取文件
		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}

		// 创建 ZIP 条目
		header := &zip.FileHeader{
			Name:   relPath,
			Method: zip.Deflate,
		}
		// 保持可执行权限
		if info.Mode()&0111 != 0 {
			header.SetMode(0755)
		} else {
			header.SetMode(0644)
		}

		f, err := w.CreateHeader(header)
		if err != nil {
			return err
		}
		_, err = io.Copy(f, bytes.NewReader(data))
		return err
	})
}

// generateSingBoxServerConfig 生成服务器的 Sing-Box 配置
func generateSingBoxServerConfig(_ database.Server, nodes []database.InboundNode, nodeConfigs map[uint]database.ServerNodeConfig) ([]byte, error) {
	config := map[string]interface{}{
		"log": map[string]interface{}{
			"level":     "info",
			"timestamp": true,
		},
		"dns": map[string]interface{}{
			"servers": []map[string]interface{}{
				{"tag": "local", "address": "local"},
			},
		},
	}

	inbounds := []map[string]interface{}{}
	outbounds := []map[string]interface{}{
		{"type": "direct", "tag": "direct"},
		{"type": "block", "tag": "block"},
	}

	for _, node := range nodes {
		nodeConfig := nodeConfigs[node.ID]
		port := node.Port
		if nodeConfig.ListenPort > 0 {
			port = nodeConfig.ListenPort
		}

		inbound := map[string]interface{}{
			"type":        node.Protocol,
			"tag":         node.Tag,
			"listen":      "::",
			"listen_port": port,
		}

		// 根据协议配置
		switch node.Protocol {
		case "trojan":
			inbound["users"] = []map[string]interface{}{
				{"name": "placeholder", "password": "placeholder"},
			}
		case "vless":
			inbound["users"] = []map[string]interface{}{
				{"name": "placeholder", "uuid": "00000000-0000-0000-0000-000000000000"},
			}
		case "vmess":
			inbound["users"] = []map[string]interface{}{
				{"name": "placeholder", "uuid": "00000000-0000-0000-0000-000000000000"},
			}
		case "shadowsocks":
			inbound["method"] = node.SsMethod
			inbound["password"] = node.SsPassword
			// 如果是多用户模式
			if strings.Contains(node.SsMethod, "2022") {
				inbound["users"] = []map[string]interface{}{
					{"name": "placeholder", "password": "placeholder"},
				}
			}
		case "hysteria2":
			inbound["users"] = []map[string]interface{}{
				{"name": "placeholder", "password": "placeholder"},
			}
			if node.Hy2UpMbps > 0 {
				inbound["up_mbps"] = node.Hy2UpMbps
			}
			if node.Hy2DownMbps > 0 {
				inbound["down_mbps"] = node.Hy2DownMbps
			}
		case "anytls":
			inbound["users"] = []map[string]interface{}{
				{"name": "placeholder", "password": "placeholder"},
			}
		}

		// TLS 配置
		if node.TlsEnabled {
			tlsConfig := map[string]interface{}{
				"enabled":     true,
				"server_name": node.ServerName,
			}
			if node.CertPath != "" {
				tlsConfig["certificate_path"] = node.CertPath
				tlsConfig["key_path"] = node.KeyPath
			} else {
				// 使用默认证书
				tlsConfig["certificate_path"] = "server.crt"
				tlsConfig["key_path"] = "server.key"
			}
			inbound["tls"] = tlsConfig
		}

		// Reality 配置
		if node.RealityEnabled {
			inbound["tls"] = map[string]interface{}{
				"enabled":     true,
				"server_name": node.ServerName,
				"reality": map[string]interface{}{
					"enabled": true,
					"handshake": map[string]interface{}{
						"server":      node.RealityServer,
						"server_port": 443,
					},
					"private_key": node.RealityPrivkey,
					"short_id":    []string{node.RealityShortId},
				},
			}
		}

		// 传输层配置
		if node.TransportEnabled {
			switch node.TransportType {
			case "ws":
				transport := map[string]interface{}{"type": "ws"}
				transport["path"] = node.WsPath
				if node.TransportHost != "" {
					transport["headers"] = map[string]string{"Host": node.TransportHost}
				}
				inbound["transport"] = transport
			case "grpc":
				transport := map[string]interface{}{"type": "grpc"}
				transport["service_name"] = node.GrpcService
				inbound["transport"] = transport
			case "", "tcp":
				// tcp 不需要传输层配置
			}
		}

		inbounds = append(inbounds, inbound)

		// 处理端口转发/落地出站
		if nodeConfig.ForwardEnabled {
			// 端口转发：添加直连到目标
			inbound["detour"] = fmt.Sprintf("forward-%d", node.ID)
			outbounds = append(outbounds, map[string]interface{}{
				"type":             "direct",
				"tag":              fmt.Sprintf("forward-%d", node.ID),
				"override_address": nodeConfig.ForwardHost,
				"override_port":    nodeConfig.ForwardPort,
			})
		} else if nodeConfig.OutboundEnabled {
			// 落地出站
			outboundTag := fmt.Sprintf("outbound-%d", node.ID)
			inbound["detour"] = outboundTag

			ob := map[string]interface{}{
				"type":        nodeConfig.OutboundProtocol,
				"tag":         outboundTag,
				"server":      nodeConfig.OutboundHost,
				"server_port": nodeConfig.OutboundPort,
			}

			switch nodeConfig.OutboundProtocol {
			case "shadowsocks", "ss":
				ob["type"] = "shadowsocks"
				ob["method"] = nodeConfig.OutboundMethod
				ob["password"] = nodeConfig.OutboundPassword
			case "trojan":
				ob["password"] = nodeConfig.OutboundPassword
				// Trojan 必须启用 TLS
				tls := map[string]interface{}{
					"enabled":  true,
					"insecure": true, // 跳过证书验证
				}
				if nodeConfig.OutboundSni != "" {
					tls["server_name"] = nodeConfig.OutboundSni
				}
				// uTLS fingerprint
				tls["utls"] = map[string]interface{}{
					"enabled":     true,
					"fingerprint": "chrome",
				}
				ob["tls"] = tls
			case "socks5":
				ob["type"] = "socks"
				if nodeConfig.OutboundUsername != "" {
					ob["username"] = nodeConfig.OutboundUsername
					ob["password"] = nodeConfig.OutboundPassword
				}
			case "anytls":
				ob["password"] = nodeConfig.OutboundPassword
				// AnyTLS 必须启用 TLS
				tls := map[string]interface{}{
					"enabled":  true,
					"insecure": true, // 跳过证书验证
				}
				if nodeConfig.OutboundSni != "" {
					tls["server_name"] = nodeConfig.OutboundSni
				}
				// uTLS fingerprint
				tls["utls"] = map[string]interface{}{
					"enabled":     true,
					"fingerprint": "chrome",
				}
				ob["tls"] = tls
			case "vless":
				ob["uuid"] = nodeConfig.OutboundUUID
				if nodeConfig.OutboundFlow != "" {
					ob["flow"] = nodeConfig.OutboundFlow
				}
				// TLS 或 Reality 配置
				if nodeConfig.OutboundReality {
					// Reality 配置
					tls := map[string]interface{}{
						"enabled": true,
						"reality": map[string]interface{}{
							"enabled":    true,
							"public_key": nodeConfig.OutboundPubKey,
							"short_id":   nodeConfig.OutboundShortId,
						},
					}
					if nodeConfig.OutboundSni != "" {
						tls["server_name"] = nodeConfig.OutboundSni
					}
					// uTLS fingerprint
					utls := map[string]interface{}{"enabled": true}
					if nodeConfig.OutboundFp != "" {
						utls["fingerprint"] = nodeConfig.OutboundFp
					} else {
						utls["fingerprint"] = "chrome"
					}
					tls["utls"] = utls
					ob["tls"] = tls
				} else if nodeConfig.OutboundTls {
					tls := map[string]interface{}{
						"enabled":  true,
						"insecure": true, // 跳过证书验证
					}
					if nodeConfig.OutboundSni != "" {
						tls["server_name"] = nodeConfig.OutboundSni
					}
					// uTLS fingerprint
					tls["utls"] = map[string]interface{}{
						"enabled":     true,
						"fingerprint": "chrome",
					}
					ob["tls"] = tls
				}
				// 传输层配置
				switch nodeConfig.OutboundNetwork {
				case "ws":
					transport := map[string]interface{}{"type": "ws"}
					if nodeConfig.OutboundWsPath != "" {
						transport["path"] = nodeConfig.OutboundWsPath
					}
					if nodeConfig.OutboundWsHost != "" {
						transport["headers"] = map[string]interface{}{
							"Host": nodeConfig.OutboundWsHost,
						}
					}
					ob["transport"] = transport
				case "grpc":
					transport := map[string]interface{}{"type": "grpc"}
					ob["transport"] = transport
				case "", "tcp":
					// tcp 不需要传输层配置
				}
			case "vmess":
				ob["uuid"] = nodeConfig.OutboundUUID
				ob["alter_id"] = nodeConfig.OutboundAlterId
				if nodeConfig.OutboundSecurity != "" {
					ob["security"] = nodeConfig.OutboundSecurity
				} else {
					ob["security"] = "auto"
				}
				// TLS 配置
				if nodeConfig.OutboundTls {
					tls := map[string]interface{}{
						"enabled":  true,
						"insecure": true, // 跳过证书验证
					}
					if nodeConfig.OutboundSni != "" {
						tls["server_name"] = nodeConfig.OutboundSni
					}
					// uTLS fingerprint
					tls["utls"] = map[string]interface{}{
						"enabled":     true,
						"fingerprint": "chrome",
					}
					ob["tls"] = tls
				}
				// 传输层配置
				switch nodeConfig.OutboundNetwork {
				case "ws":
					transport := map[string]interface{}{"type": "ws"}
					if nodeConfig.OutboundWsPath != "" {
						transport["path"] = nodeConfig.OutboundWsPath
					}
					if nodeConfig.OutboundWsHost != "" {
						transport["headers"] = map[string]interface{}{
							"Host": nodeConfig.OutboundWsHost,
						}
					}
					ob["transport"] = transport
				case "grpc":
					transport := map[string]interface{}{"type": "grpc"}
					ob["transport"] = transport
				case "", "tcp":
					// tcp 不需要传输层配置
				}
			case "hysteria2":
				ob["password"] = nodeConfig.OutboundPassword
				// Hysteria2 必须启用 TLS
				tls := map[string]interface{}{
					"enabled":  true,
					"insecure": true, // 跳过证书验证
				}
				if nodeConfig.OutboundSni != "" {
					tls["server_name"] = nodeConfig.OutboundSni
				}
				ob["tls"] = tls
				// obfs 配置（可选）
				if nodeConfig.OutboundObfs != "" {
					ob["obfs"] = map[string]interface{}{
						"type":     nodeConfig.OutboundObfs,
						"password": nodeConfig.OutboundObfsPwd,
					}
				}
			}

			outbounds = append(outbounds, ob)
		} else {
			inbound["detour"] = "direct"
		}
	}

	config["inbounds"] = inbounds
	config["outbounds"] = outbounds

	return json.MarshalIndent(config, "", "  ")
}

// generateMihomoServerConfig 生成服务器的 Mihomo 配置
func generateMihomoServerConfig(_ database.Server, nodes []database.InboundNode, nodeConfigs map[uint]database.ServerNodeConfig) ([]byte, error) {
	listeners := []map[string]interface{}{}

	for _, node := range nodes {
		nodeConfig := nodeConfigs[node.ID]
		port := node.Port
		if nodeConfig.ListenPort > 0 {
			port = nodeConfig.ListenPort
		}

		listener := map[string]interface{}{
			"name": node.Tag,
			"type": node.Protocol,
			"port": port,
		}

		// 根据协议配置
		switch node.Protocol {
		case "trojan":
			listener["users"] = []map[string]interface{}{
				{"username": "placeholder", "password": "placeholder"},
			}
		case "vless":
			listener["users"] = []map[string]interface{}{
				{"username": "placeholder", "uuid": "00000000-0000-0000-0000-000000000000"},
			}
		case "vmess":
			listener["users"] = []map[string]interface{}{
				{"username": "placeholder", "uuid": "00000000-0000-0000-0000-000000000000"},
			}
		case "shadowsocks", "ss":
			listener["cipher"] = node.SsMethod
			listener["password"] = node.SsPassword
		case "hysteria2":
			listener["users"] = []map[string]interface{}{
				{"username": "placeholder", "password": "placeholder"},
			}
			if node.Hy2UpMbps > 0 {
				listener["up"] = fmt.Sprintf("%d Mbps", node.Hy2UpMbps)
			}
			if node.Hy2DownMbps > 0 {
				listener["down"] = fmt.Sprintf("%d Mbps", node.Hy2DownMbps)
			}
		case "anytls":
			listener["users"] = []map[string]interface{}{
				{"username": "placeholder", "password": "placeholder"},
			}
		}

		// TLS 配置
		if node.TlsEnabled {
			if node.CertPath != "" {
				listener["certificate"] = node.CertPath
				listener["private-key"] = node.KeyPath
			} else {
				listener["certificate"] = "server.crt"
				listener["private-key"] = "server.key"
			}
		}

		// 传输层
		if node.TransportEnabled {
			switch node.TransportType {
			case "ws":
				listener["ws-path"] = node.WsPath
			case "grpc":
				listener["grpc-service-name"] = node.GrpcService
			}
		}

		// 代理目标
		if nodeConfig.ForwardEnabled {
			listener["proxy"] = fmt.Sprintf("%s:%d", nodeConfig.ForwardHost, nodeConfig.ForwardPort)
		} else if nodeConfig.OutboundEnabled {
			// Mihomo 不直接支持落地出站，使用 proxy-chain
			listener["proxy"] = fmt.Sprintf("outbound-%d", node.ID)
		} else {
			listener["proxy"] = "DIRECT"
		}

		listeners = append(listeners, listener)
	}

	// 构建出站代理（如果有）
	proxies := []map[string]interface{}{}
	for _, node := range nodes {
		nodeConfig := nodeConfigs[node.ID]
		if nodeConfig.OutboundEnabled {
			proxy := map[string]interface{}{
				"name":   fmt.Sprintf("outbound-%d", node.ID),
				"type":   nodeConfig.OutboundProtocol,
				"server": nodeConfig.OutboundHost,
				"port":   nodeConfig.OutboundPort,
				"udp":    true, // 启用 UDP 支持
			}

			switch nodeConfig.OutboundProtocol {
			case "ss", "shadowsocks":
				proxy["type"] = "ss"
				proxy["cipher"] = nodeConfig.OutboundMethod
				proxy["password"] = nodeConfig.OutboundPassword
			case "trojan":
				proxy["password"] = nodeConfig.OutboundPassword
				// Trojan 需要 TLS 配置
				if nodeConfig.OutboundSni != "" {
					proxy["sni"] = nodeConfig.OutboundSni
				}
				proxy["skip-cert-verify"] = true
				proxy["client-fingerprint"] = "chrome"
			case "socks5":
				if nodeConfig.OutboundUsername != "" {
					proxy["username"] = nodeConfig.OutboundUsername
					proxy["password"] = nodeConfig.OutboundPassword
				}
			case "anytls":
				proxy["type"] = "anytls"
				proxy["password"] = nodeConfig.OutboundPassword
				if nodeConfig.OutboundSni != "" {
					proxy["sni"] = nodeConfig.OutboundSni
				}
				proxy["skip-cert-verify"] = true
				proxy["client-fingerprint"] = "chrome"
			case "vless":
				proxy["uuid"] = nodeConfig.OutboundUUID
				if nodeConfig.OutboundFlow != "" {
					proxy["flow"] = nodeConfig.OutboundFlow
				}
				// Reality 或 TLS 配置
				if nodeConfig.OutboundReality {
					proxy["tls"] = true
					proxy["reality-opts"] = map[string]interface{}{
						"public-key": nodeConfig.OutboundPubKey,
						"short-id":   nodeConfig.OutboundShortId,
					}
					if nodeConfig.OutboundSni != "" {
						proxy["servername"] = nodeConfig.OutboundSni
					}
					// client fingerprint
					if nodeConfig.OutboundFp != "" {
						proxy["client-fingerprint"] = nodeConfig.OutboundFp
					} else {
						proxy["client-fingerprint"] = "chrome"
					}
				} else if nodeConfig.OutboundTls {
					proxy["tls"] = true
					if nodeConfig.OutboundSni != "" {
						proxy["servername"] = nodeConfig.OutboundSni
					}
					proxy["skip-cert-verify"] = true
					proxy["client-fingerprint"] = "chrome"
				}
				// 传输层配置
				switch nodeConfig.OutboundNetwork {
				case "ws":
					proxy["network"] = "ws"
					wsOpts := map[string]interface{}{}
					if nodeConfig.OutboundWsPath != "" {
						wsOpts["path"] = nodeConfig.OutboundWsPath
					}
					if nodeConfig.OutboundWsHost != "" {
						wsOpts["headers"] = map[string]interface{}{
							"Host": nodeConfig.OutboundWsHost,
						}
					}
					if len(wsOpts) > 0 {
						proxy["ws-opts"] = wsOpts
					}
				case "grpc":
					proxy["network"] = "grpc"
					// TODO: 需要添加 OutboundGrpcService 字段支持
				case "", "tcp":
					// tcp 不需要传输层配置
				}
			case "vmess":
				proxy["uuid"] = nodeConfig.OutboundUUID
				proxy["alterId"] = nodeConfig.OutboundAlterId
				if nodeConfig.OutboundSecurity != "" {
					proxy["cipher"] = nodeConfig.OutboundSecurity
				} else {
					proxy["cipher"] = "auto"
				}
				// TLS 配置
				if nodeConfig.OutboundTls {
					proxy["tls"] = true
					if nodeConfig.OutboundSni != "" {
						proxy["servername"] = nodeConfig.OutboundSni
					}
					proxy["skip-cert-verify"] = true
					proxy["client-fingerprint"] = "chrome"
				}
				// 传输层配置
				switch nodeConfig.OutboundNetwork {
				case "ws":
					proxy["network"] = "ws"
					wsOpts := map[string]interface{}{}
					if nodeConfig.OutboundWsPath != "" {
						wsOpts["path"] = nodeConfig.OutboundWsPath
					}
					if nodeConfig.OutboundWsHost != "" {
						wsOpts["headers"] = map[string]interface{}{
							"Host": nodeConfig.OutboundWsHost,
						}
					}
					if len(wsOpts) > 0 {
						proxy["ws-opts"] = wsOpts
					}
				case "grpc":
					proxy["network"] = "grpc"
					// TODO: 需要添加 OutboundGrpcService 字段支持
				case "", "tcp":
					// tcp 不需要传输层配置
				}
			case "hysteria2":
				proxy["password"] = nodeConfig.OutboundPassword
				// Hysteria2 需要 TLS
				if nodeConfig.OutboundSni != "" {
					proxy["sni"] = nodeConfig.OutboundSni
				}
				proxy["skip-cert-verify"] = true
				// obfs 配置（可选）
				if nodeConfig.OutboundObfs != "" {
					proxy["obfs"] = nodeConfig.OutboundObfs
					proxy["obfs-password"] = nodeConfig.OutboundObfsPwd
				}
			}

			proxies = append(proxies, proxy)
		}
	}

	config := map[string]interface{}{
		"mixed-port":          7890,
		"allow-lan":           true,
		"mode":                "rule",
		"log-level":           "info",
		"external-controller": "0.0.0.0:9090",
		"listeners":           listeners,
	}

	if len(proxies) > 0 {
		config["proxies"] = proxies
	}

	return yaml.Marshal(config)
}
