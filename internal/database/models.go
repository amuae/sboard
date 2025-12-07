package database

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// Admin 管理员表
type Admin struct {
	ID        uint           `gorm:"primaryKey" json:"id"`
	Username  string         `gorm:"uniqueIndex;size:50;not null" json:"username"`
	Password  string         `gorm:"size:255;not null" json:"-"`
	Email     string         `gorm:"size:100" json:"email"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

// SetPassword 设置密码 (bcrypt加密)
func (a *Admin) SetPassword(password string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	a.Password = string(hash)
	return nil
}

// CheckPassword 验证密码
func (a *Admin) CheckPassword(password string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(a.Password), []byte(password))
	return err == nil
}

// ProxyUser 代理用户表
type ProxyUser struct {
	ID           uint           `gorm:"primaryKey" json:"id"`
	Name         string         `gorm:"size:100;not null" json:"name"`
	UUID         string         `gorm:"uniqueIndex;size:36;not null" json:"uuid"`
	Level        int            `gorm:"default:1" json:"level"`                     // 1,2,3 等级
	ExpiryDate   string         `gorm:"size:10;not null" json:"expiry_date"`        // YYYY-MM-DD
	Enabled      int            `gorm:"default:1" json:"enabled"`                   // 0/1
	TrafficLimit int64          `gorm:"default:0" json:"traffic_limit"`             // 流量限制(GB)，0表示无限制
	TrafficUsed  int64          `gorm:"default:0" json:"traffic_used"`              // 已用流量(GB)
	DnsResolve   string         `gorm:"size:10;default:default" json:"dns_resolve"` // default/ipv4/ipv6
	Notes        string         `gorm:"type:text" json:"notes"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
	DeletedAt    gorm.DeletedAt `gorm:"index" json:"-"`
}

// BeforeCreate 创建前自动生成UUID
func (u *ProxyUser) BeforeCreate(tx *gorm.DB) error {
	if u.UUID == "" {
		u.UUID = uuid.New().String()
	}
	return nil
}

// InboundNode 入站节点表
type InboundNode struct {
	ID       uint   `gorm:"primaryKey" json:"id"`
	Tag      string `gorm:"uniqueIndex;size:100;not null" json:"tag"`
	Protocol string `gorm:"size:20;not null" json:"protocol"` // trojan/vless/vmess/anytls/shadowsocks/hysteria2
	Listen   string `gorm:"size:20;default:'::'" json:"listen"`
	Port     int    `gorm:"not null;uniqueIndex" json:"port"`

	// TLS 配置
	TlsEnabled bool   `gorm:"default:true" json:"tls_enabled"`
	ServerName string `gorm:"size:100;default:down.dingtalk.com" json:"server_name"`
	CertPath   string `gorm:"size:200;default:server.crt" json:"cert_path"`
	KeyPath    string `gorm:"size:200;default:server.key" json:"key_path"`

	// Reality 配置
	RealityEnabled bool   `gorm:"default:false" json:"reality_enabled"`
	RealityServer  string `gorm:"size:100" json:"reality_server"`
	RealityPubkey  string `gorm:"size:100" json:"reality_pubkey"`
	RealityPrivkey string `gorm:"size:100" json:"reality_privkey"`
	RealityShortId string `gorm:"size:20" json:"reality_short_id"`

	// 传输层配置
	TransportEnabled bool   `gorm:"default:false" json:"transport_enabled"`
	TransportType    string `gorm:"size:20" json:"transport_type"` // http/ws/grpc/httpupgrade
	WsPath           string `gorm:"size:200;default:/" json:"ws_path"`
	GrpcService      string `gorm:"size:100" json:"grpc_service"`
	TransportHost    string `gorm:"size:100" json:"transport_host"`

	// VLESS Flow
	Flow string `gorm:"size:50" json:"flow"`

	// Shadowsocks 配置
	SsMethod   string `gorm:"size:50" json:"ss_method"`    // aes-256-gcm/chacha20-ietf-poly1305/2022-blake3-aes-256-gcm
	SsPassword string `gorm:"size:100" json:"ss_password"` // 随机生成的密码

	// Hysteria2 配置
	Hy2Password     string `gorm:"size:100" json:"hy2_password"`
	Hy2UpMbps       int    `gorm:"default:100" json:"hy2_up_mbps"`
	Hy2DownMbps     int    `gorm:"default:100" json:"hy2_down_mbps"`
	Hy2Obfs         string `gorm:"size:20" json:"hy2_obfs"` // salamander or empty
	Hy2ObfsPassword string `gorm:"size:100" json:"hy2_obfs_password"`

	// 状态
	Enabled bool   `gorm:"default:true" json:"enabled"`
	Notes   string `gorm:"type:text" json:"notes"`

	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

// BeforeCreate 创建前自动生成 Shadowsocks 密码和 Reality Short ID
func (n *InboundNode) BeforeCreate(tx *gorm.DB) error {
	// 为 Shadowsocks 生成密码
	if n.Protocol == "shadowsocks" && n.SsPassword == "" {
		if n.SsMethod != "" && len(n.SsMethod) > 5 && n.SsMethod[:5] == "2022-" {
			// SS 2022 需要 base64 编码的 32 字节密钥
			bytes := make([]byte, 32)
			rand.Read(bytes)
			n.SsPassword = base64.StdEncoding.EncodeToString(bytes)
		} else {
			// 传统加密方法使用 32 位十六进制
			bytes := make([]byte, 16)
			rand.Read(bytes)
			n.SsPassword = hex.EncodeToString(bytes)
		}
	}

	// 为 Reality 生成 Short ID
	if n.RealityEnabled && n.RealityShortId == "" {
		bytes := make([]byte, 4)
		rand.Read(bytes)
		n.RealityShortId = hex.EncodeToString(bytes)
	}

	return nil
}

// NodeUserRelation 节点-用户关联表
type NodeUserRelation struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	NodeID    uint      `gorm:"index;not null" json:"node_id"`
	UserID    uint      `gorm:"index;not null" json:"user_id"`
	UUID      string    `gorm:"size:36;not null" json:"uuid"`
	Flow      string    `gorm:"size:50" json:"flow"`
	CreatedAt time.Time `json:"created_at"`

	Node InboundNode `gorm:"foreignKey:NodeID" json:"node,omitempty"`
	User ProxyUser   `gorm:"foreignKey:UserID" json:"user,omitempty"`
}

// Server 服务器表
type Server struct {
	ID        uint   `gorm:"primaryKey" json:"id"`
	Name      string `gorm:"size:100;not null" json:"name"`
	Host      string `gorm:"size:100" json:"host"` // 由 Agent 上报的服务器 IP
	Port      int    `gorm:"default:22" json:"port"`
	Category  string `gorm:"size:20;not null;default:direct" json:"category"` // direct/relay/home
	Enabled   int    `gorm:"default:1" json:"enabled"`
	SortOrder int    `gorm:"default:0" json:"sort_order"` // 排序顺序，越小越前`

	// 节点域名（用于订阅 DNS 解析，不随 IP 变化）
	NodeDomain string `gorm:"size:200" json:"node_domain"`

	// 节点名称（用于显示）
	Node1 string `gorm:"size:100" json:"node_1"`
	Node2 string `gorm:"size:100" json:"node_2"`
	Node3 string `gorm:"size:100" json:"node_3"`

	// 核心类型
	CoreType string `gorm:"size:20;not null;default:sing-box" json:"core_type"` // sing-box/mihomo

	// DNS 解析
	DnsResolve string `gorm:"size:10;default:none" json:"dns_resolve"` // none/ipv4/ipv6

	// 部署模式: ssh / agent
	DeployMode string `gorm:"size:10;default:ssh" json:"deploy_mode"`

	// SSH 配置 (deploy_mode=ssh)
	SshUser    string `gorm:"size:50;default:root" json:"ssh_user"`
	SshKeyPath string `gorm:"size:200;default:storage/ssh/id_rsa" json:"ssh_key_path"`

	// Agent 配置 (deploy_mode=agent)
	AgentToken    string     `gorm:"size:64" json:"agent_token"`        // Agent 认证 Token
	AgentID       string     `gorm:"size:100" json:"agent_id"`          // Agent 唯一标识
	AgentVersion  string     `gorm:"size:20" json:"agent_version"`      // Agent 版本
	AgentOnline   bool       `gorm:"default:false" json:"agent_online"` // Agent 是否在线
	LastHeartbeat *time.Time `json:"last_heartbeat"`                    // 最后心跳时间

	// 服务器状态 (Agent 上报)
	CpuUsage  float64 `gorm:"default:0" json:"cpu_usage"`
	MemUsage  float64 `gorm:"default:0" json:"mem_usage"`
	DiskUsage float64 `gorm:"default:0" json:"disk_usage"`
	NetIn     int64   `gorm:"default:0" json:"net_in"`
	NetOut    int64   `gorm:"default:0" json:"net_out"`

	// 月度流量统计 (面板端记录)
	MonthlyIn    int64      `gorm:"default:0" json:"monthly_in"`  // 月度下行流量 (bytes)
	MonthlyOut   int64      `gorm:"default:0" json:"monthly_out"` // 月度上行流量 (bytes)
	TrafficReset *time.Time `json:"traffic_reset"`                // 上次流量重置时间`

	// 上次收到的流量总量 (用于差值计算)
	LastNetInTransfer  uint64 `gorm:"default:0" json:"-"` // 上次心跳的入流量总量
	LastNetOutTransfer uint64 `gorm:"default:0" json:"-"` // 上次心跳的出流量总量

	// 其他
	Notes        string     `gorm:"type:text" json:"notes"`
	LastDeployAt *time.Time `json:"last_deploy_at"`

	// 关联的节点配置
	NodeConfigs []ServerNodeConfig `gorm:"foreignKey:ServerID" json:"node_configs,omitempty"`

	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

// ServerNodeConfig 服务器节点配置表（端口转发/落地出站）
type ServerNodeConfig struct {
	ID       uint `gorm:"primaryKey" json:"id"`
	ServerID uint `gorm:"index;not null" json:"server_id"`
	NodeID   uint `gorm:"index;not null" json:"node_id"`

	// 监听端口
	ListenPort int `json:"listen_port"`

	// 端口转发配置
	ForwardEnabled bool   `gorm:"default:false" json:"forward_enabled"`
	ForwardHost    string `gorm:"size:100" json:"forward_host"`
	ForwardPort    int    `json:"forward_port"`

	// 落地出站配置
	OutboundEnabled  bool   `gorm:"default:false" json:"outbound_enabled"`
	OutboundProtocol string `gorm:"size:20" json:"outbound_protocol"` // ss/trojan/anytls/socks5
	OutboundHost     string `gorm:"size:100" json:"outbound_host"`
	OutboundPort     int    `json:"outbound_port"`
	OutboundPassword string `gorm:"size:200" json:"outbound_password"`
	OutboundMethod   string `gorm:"size:50" json:"outbound_method"`   // Shadowsocks 加密方式
	OutboundUsername string `gorm:"size:50" json:"outbound_username"` // SOCKS5 用户名
	OutboundSni      string `gorm:"size:100" json:"outbound_sni"`     // TLS SNI

	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`

	Server Server      `gorm:"foreignKey:ServerID" json:"server,omitempty"`
	Node   InboundNode `gorm:"foreignKey:NodeID" json:"node,omitempty"`
}

// DeployLog 部署日志表
type DeployLog struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	ServerID   uint      `gorm:"index;not null" json:"server_id"`
	Status     string    `gorm:"size:20;default:pending" json:"status"` // success/failed/pending
	Message    string    `gorm:"type:text" json:"message"`
	LogContent string    `gorm:"type:text" json:"log_content"`
	CreatedAt  time.Time `json:"created_at"`

	Server Server `gorm:"foreignKey:ServerID" json:"server,omitempty"`
}

// SystemConfig 系统配置表
type SystemConfig struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	Key         string    `gorm:"uniqueIndex;size:50;not null" json:"key"`
	Value       string    `gorm:"type:text" json:"value"`
	Description string    `gorm:"type:text" json:"description"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}
