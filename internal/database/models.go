package database

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// Admin 管理员表
type Admin struct {
	ID           uint           `gorm:"primaryKey" json:"id"`
	Username     string         `gorm:"uniqueIndex;size:50;not null" json:"username"`
	Password     string         `gorm:"size:255" json:"-"` // 本地账户密码，OAuth 用户可为空
	Email        string         `gorm:"size:100" json:"email"`
	AuthProvider string         `gorm:"size:20;default:local" json:"auth_provider"` // local, github
	OAuthID      string         `gorm:"size:100;index" json:"-"`                    // OAuth 提供商的用户 ID
	AvatarURL    string         `gorm:"size:255" json:"avatar_url"`                 // 头像 URL
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
	DeletedAt    gorm.DeletedAt `gorm:"index" json:"-"`
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
	ID           uint   `gorm:"primaryKey" json:"id"`
	Name         string `gorm:"size:100;not null" json:"name"`
	UUID         string `gorm:"uniqueIndex;size:36;not null" json:"uuid"`
	Level        int    `gorm:"default:1" json:"level"`                     // 1,2,3 等级
	ExpiryDate   string `gorm:"size:10;not null" json:"expiry_date"`        // YYYY-MM-DD
	Enabled      int    `gorm:"default:1" json:"enabled"`                   // 0/1
	TrafficLimit int64  `gorm:"default:0" json:"traffic_limit"`             // 流量限制(GB)，0表示无限制
	TrafficUsed  int64  `gorm:"default:0" json:"traffic_used"`              // 已用流量(GB)
	DnsResolve   string `gorm:"size:10;default:default" json:"dns_resolve"` // default/ipv4/ipv6
	Notes        string `gorm:"type:text" json:"notes"`

	// 额外 UUID (用于落地出站路由，与 ServerOutbound 序号对应)
	// 保留 UUID1-UUID10 列用于向后兼容，新代码应通过 UserExtraUUID 表操作
	UUID1  string `gorm:"size:36" json:"-"`
	UUID2  string `gorm:"size:36" json:"-"`
	UUID3  string `gorm:"size:36" json:"-"`
	UUID4  string `gorm:"size:36" json:"-"`
	UUID5  string `gorm:"size:36" json:"-"`
	UUID6  string `gorm:"size:36" json:"-"`
	UUID7  string `gorm:"size:36" json:"-"`
	UUID8  string `gorm:"size:36" json:"-"`
	UUID9  string `gorm:"size:36" json:"-"`
	UUID10 string `gorm:"size:36" json:"-"`

	// extraUUIDs 运行时缓存（懒加载），不持久化到数据库
	extraUUIDs []string `gorm:"-" json:"-"`

	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

// BeforeCreate 创建前自动生成主UUID，初始化额外UUID缓存为空（不再预生成10个）
func (u *ProxyUser) BeforeCreate(tx *gorm.DB) error {
	if u.UUID == "" {
		u.UUID = uuid.New().String()
	}
	// 懒加载：初始化空槽位缓存，UUID 在首次 SetExtraUUID 时按需生成
	u.extraUUIDs = make([]string, 10)
	return nil
}

// AfterCreate 创建后无操作 — 额外 UUID 由 SetExtraUUID 按需创建到 UserExtraUUID 表
func (u *ProxyUser) AfterCreate(tx *gorm.DB) error {
	return nil
}

// GetExtraUUID 获取指定序号的额外UUID (1-10)。使用懒加载缓存，不执行 DB 查询。
func (u *ProxyUser) GetExtraUUID(index int) string {
	u.ensureExtraUUIDsLoaded()
	if index >= 1 && index <= len(u.extraUUIDs) {
		return u.extraUUIDs[index-1]
	}
	return ""
}

// GetAllExtraUUIDs 获取所有额外UUID列表 (非空的)
func (u *ProxyUser) GetAllExtraUUIDs() []string {
	u.ensureExtraUUIDsLoaded()
	var result []string
	for _, uid := range u.extraUUIDs {
		if uid != "" {
			result = append(result, uid)
		}
	}
	return result
}

// ensureExtraUUIDsLoaded 懒加载 extraUUIDs 缓存：优先从 UserExtraUUID 表，回退到 UUID1-UUID10 字段
func (u *ProxyUser) ensureExtraUUIDsLoaded() {
	if u.extraUUIDs != nil {
		return
	}

	// 先以 UUID1-UUID10 作为回退基线
	u.extraUUIDs = []string{
		u.UUID1, u.UUID2, u.UUID3, u.UUID4, u.UUID5,
		u.UUID6, u.UUID7, u.UUID8, u.UUID9, u.UUID10,
	}

	// 如果 ID 为 0（未持久化），仅使用回退值
	if u.ID == 0 {
		return
	}

	// 尝试从 UserExtraUUID 表加载（覆盖回退值）
	db := GetDB()
	var records []UserExtraUUID
	if err := db.Where("user_id = ?", u.ID).Order("slot asc").Find(&records).Error; err == nil {
		for _, r := range records {
			if r.Slot >= 1 && r.Slot <= 10 {
				u.extraUUIDs[r.Slot-1] = r.UUID
			}
		}
	}
}

// SetExtraUUID 设置指定槽位的额外 UUID（懒加载：首次设置时自动生成）
func (u *ProxyUser) SetExtraUUID(slot int, val string) error {
	if slot < 1 || slot > 10 {
		return fmt.Errorf("slot must be between 1 and 10, got %d", slot)
	}

	u.ensureExtraUUIDsLoaded()

	// 懒加载：如果槽位尚无 UUID 且调用方未提供，自动生成
	if u.extraUUIDs[slot-1] == "" && val == "" {
		val = uuid.New().String()
	}

	if val != "" {
		u.extraUUIDs[slot-1] = val
	}

	// 向后兼容：同步到 UUID1-UUID10 列
	switch slot {
	case 1:
		u.UUID1 = val
	case 2:
		u.UUID2 = val
	case 3:
		u.UUID3 = val
	case 4:
		u.UUID4 = val
	case 5:
		u.UUID5 = val
	case 6:
		u.UUID6 = val
	case 7:
		u.UUID7 = val
	case 8:
		u.UUID8 = val
	case 9:
		u.UUID9 = val
	case 10:
		u.UUID10 = val
	}

	// 尚未持久化时仅更新内存缓存
	if u.ID == 0 {
		return nil
	}

	// 写入 UserExtraUUID 表（upsert）
	db := GetDB()
	var existing UserExtraUUID
	if err := db.Where("user_id = ? AND slot = ?", u.ID, slot).First(&existing).Error; err == nil {
		existing.UUID = val
		return db.Save(&existing).Error
	}
	return db.Create(&UserExtraUUID{
		UserID: u.ID,
		Slot:   slot,
		UUID:   val,
	}).Error
}

// UserExtraUUID 用户额外 UUID 关联表（替代硬编码的 UUID1-UUID10）
type UserExtraUUID struct {
	ID     uint   `gorm:"primaryKey" json:"id"`
	UserID uint   `gorm:"uniqueIndex:idx_user_slot;not null" json:"user_id"`
	Slot   int    `gorm:"uniqueIndex:idx_user_slot;not null" json:"slot"` // 槽位 1-10，与 ServerOutbound 对应
	UUID   string `gorm:"size:36;not null" json:"uuid"`
}

// InboundNode 入站节点表
type InboundNode struct {
	ID       uint   `gorm:"primaryKey" json:"id"`
	Tag      string `gorm:"uniqueIndex;size:100;not null" json:"tag"`
	Protocol string `gorm:"size:20;not null" json:"protocol"` // trojan/vless/vmess/anytls/shadowsocks/hysteria2/naive
	Listen   string `gorm:"size:20;default:'::'" json:"listen"`
	Port     int    `gorm:"not null;uniqueIndex" json:"port"`

	// TLS 配置
	TlsEnabled bool   `gorm:"default:false" json:"tls_enabled"`
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
	SsObfsMode string `gorm:"size:20" json:"ss_obfs_mode"` // tls/http (reF1nd sing-box SS SNI伪装)
	SsObfsHost string `gorm:"size:100" json:"ss_obfs_host"` // SNI伪装域名

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

// ExternalNode 外部节点（第三方分享的代理节点）
type ExternalNode struct {
	ID       uint   `gorm:"primaryKey" json:"id"`
	Name     string `gorm:"size:100;not null" json:"name"`     // 用户可见的节点名
	Protocol string `gorm:"size:20;not null" json:"protocol"` // trojan/vless/vmess/shadowsocks/hysteria2/anytls
	Host     string `gorm:"size:200;not null" json:"host"`    // 服务器地址
	Port     int    `gorm:"not null" json:"port"`
	UUID     string `gorm:"size:100" json:"uuid"`             // 用户名/UUID/密码(trojan)

	// TLS
	TlsEnabled bool   `json:"tls_enabled"`
	ServerName string `gorm:"size:100" json:"server_name"`
	Alpn       string `gorm:"size:100" json:"alpn"`

	// Reality
	RealityEnabled bool   `json:"reality_enabled"`
	RealityServer  string `gorm:"size:100" json:"reality_server"`
	RealityPubkey  string `gorm:"size:100" json:"reality_pubkey"`
	RealityShortId string `gorm:"size:20" json:"reality_short_id"`

	// Transport
	TransportEnabled bool   `json:"transport_enabled"`
	TransportType    string `gorm:"size:20" json:"transport_type"` // ws/grpc
	WsPath           string `gorm:"size:200" json:"ws_path"`
	GrpcService      string `gorm:"size:100" json:"grpc_service"`
	TransportHost    string `gorm:"size:100" json:"transport_host"`

	// VLESS
	Flow string `gorm:"size:50" json:"flow"`

	// Shadowsocks
	SsMethod   string `gorm:"size:50" json:"ss_method"`
	SsPassword string `gorm:"size:100" json:"ss_password"`
	SsObfsMode string `gorm:"size:20" json:"ss_obfs_mode"` // tls/http (reF1nd)
	SsObfsHost string `gorm:"size:100" json:"ss_obfs_host"`

	// Hysteria2
	Hy2Password     string `gorm:"size:100" json:"hy2_password"`
	Hy2UpMbps       int    `json:"hy2_up_mbps"`
	Hy2DownMbps     int    `json:"hy2_down_mbps"`
	Hy2Obfs         string `gorm:"size:20" json:"hy2_obfs"`
	Hy2ObfsPassword string `gorm:"size:100" json:"hy2_obfs_password"`

	// 控制字段
	Level     int    `gorm:"default:1" json:"level"`          // 使用等级（1/2/3）
	Enabled   bool   `gorm:"default:true" json:"enabled"`     // 是否启用
	SortOrder int    `gorm:"default:0" json:"sort_order"`     // 排序
	Country   string `gorm:"size:10" json:"country"`          // 国家代码（用于显示旗帜）
	Notes     string `gorm:"type:text" json:"notes"`

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
	Host      string `gorm:"size:100" json:"host"` // 由 Agent 上报的服务器 IPv4
	HostIPv6  string `gorm:"size:100" json:"host_ipv6"` // 由 Agent 上报的服务器 IPv6
	Port      int    `gorm:"default:22" json:"port"`
	Category  string `gorm:"size:20;not null;default:direct" json:"category"` // direct/relay/home
	Enabled   int    `gorm:"default:1" json:"enabled"`
	SortOrder int    `gorm:"default:0" json:"sort_order"` // 排序顺序，越小越前

	// 节点域名（用于订阅 DNS 解析，不随 IP 变化）
	NodeDomain string `gorm:"size:200" json:"node_domain"`

	// 节点名称（用于显示）
	Node1 string `gorm:"size:100" json:"node_1"`
	Node2 string `gorm:"size:100" json:"node_2"`
	Node3 string `gorm:"size:100" json:"node_3"`

	// DNS 解析
	DnsResolve string `gorm:"size:10;default:ipv4" json:"dns_resolve"` // none/ipv4/ipv6

	// Agent 配置
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
	OutboundProtocol string `gorm:"size:20" json:"outbound_protocol"` // ss/trojan/anytls/socks5/vless/vmess/hysteria2
	OutboundHost     string `gorm:"size:100" json:"outbound_host"`
	OutboundPort     int    `json:"outbound_port"`
	OutboundPassword string `gorm:"size:200" json:"outbound_password"`
	OutboundMethod   string `gorm:"size:50" json:"outbound_method"`   // Shadowsocks 加密方式
	OutboundUsername string `gorm:"size:50" json:"outbound_username"` // SOCKS5 用户名
	OutboundSni      string `gorm:"size:100" json:"outbound_sni"`     // TLS SNI

	// VLESS/VMess 配置
	OutboundUUID     string `gorm:"size:100" json:"outbound_uuid"`         // VLESS/VMess UUID
	OutboundFlow     string `gorm:"size:50" json:"outbound_flow"`          // VLESS flow: xtls-rprx-vision
	OutboundSecurity string `gorm:"size:50" json:"outbound_security"`      // VMess 加密方式
	OutboundAlterId  int    `gorm:"default:0" json:"outbound_alter_id"`    // VMess alterId
	OutboundTls      bool   `gorm:"default:false" json:"outbound_tls"`     // 是否启用 TLS
	OutboundReality  bool   `gorm:"default:false" json:"outbound_reality"` // 是否启用 Reality
	OutboundPubKey   string `gorm:"size:200" json:"outbound_pub_key"`      // Reality public key
	OutboundShortId  string `gorm:"size:50" json:"outbound_short_id"`      // Reality short id
	OutboundFp       string `gorm:"size:50" json:"outbound_fp"`            // uTLS fingerprint

	// Hysteria2 配置
	OutboundObfs    string `gorm:"size:50" json:"outbound_obfs"`      // Hysteria2 obfs 类型: salamander
	OutboundObfsPwd string `gorm:"size:200" json:"outbound_obfs_pwd"` // Hysteria2 obfs 密码

	// 传输层配置 (VMess/VLESS)
	OutboundNetwork string `gorm:"size:20" json:"outbound_network"`  // tcp/ws/grpc/http
	OutboundWsPath  string `gorm:"size:200" json:"outbound_ws_path"` // WebSocket path
	OutboundWsHost  string `gorm:"size:200" json:"outbound_ws_host"` // WebSocket host header

	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`

	Server Server      `gorm:"foreignKey:ServerID" json:"server,omitempty"`
	Node   InboundNode `gorm:"foreignKey:NodeID" json:"node,omitempty"`
}

// ServerOutbound 服务器落地出站表（最多10个，与用户额外UUID对应）
type ServerOutbound struct {
	ID       uint `gorm:"primaryKey" json:"id"`
	ServerID uint `gorm:"index;not null" json:"server_id"`
	Slot     int  `gorm:"not null" json:"slot"` // 槽位 1-10，与用户 UUID1-UUID10 对应

	// 基本配置
	Enabled bool   `gorm:"default:true" json:"enabled"`
	Remark  string `gorm:"size:100" json:"remark"` // 备注（显示为按钮名称）

	// 出站配置
	Protocol string `gorm:"size:20" json:"protocol"` // ss/trojan/anytls/socks5/vless/vmess/hysteria2
	Host     string `gorm:"size:100" json:"host"`
	Port     int    `json:"port"`
	Password string `gorm:"size:200" json:"password"`
	Method   string `gorm:"size:50" json:"method"`   // Shadowsocks 加密方式
	Username string `gorm:"size:50" json:"username"` // SOCKS5 用户名
	Sni      string `gorm:"size:100" json:"sni"`     // TLS SNI

	// VLESS/VMess 配置
	UUID     string `gorm:"size:100" json:"uuid"`         // VLESS/VMess UUID
	Flow     string `gorm:"size:50" json:"flow"`          // VLESS flow: xtls-rprx-vision
	Security string `gorm:"size:50" json:"security"`      // VMess 加密方式
	AlterId  int    `gorm:"default:0" json:"alter_id"`    // VMess alterId
	Tls      bool   `gorm:"default:false" json:"tls"`     // 是否启用 TLS
	Reality  bool   `gorm:"default:false" json:"reality"` // 是否启用 Reality
	PubKey   string `gorm:"size:200" json:"pub_key"`      // Reality public key
	ShortId  string `gorm:"size:50" json:"short_id"`      // Reality short id
	Fp       string `gorm:"size:50" json:"fp"`            // uTLS fingerprint

	// Hysteria2 配置
	Obfs    string `gorm:"size:50" json:"obfs"`      // Hysteria2 obfs 类型: salamander
	ObfsPwd string `gorm:"size:200" json:"obfs_pwd"` // Hysteria2 obfs 密码

	// 传输层配置 (VMess/VLESS)
	Network string `gorm:"size:20" json:"network"`  // tcp/ws/grpc/http
	WsPath  string `gorm:"size:200" json:"ws_path"` // WebSocket path
	WsHost  string `gorm:"size:200" json:"ws_host"` // WebSocket host header

	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`

	Server Server `gorm:"foreignKey:ServerID" json:"server,omitempty"`
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

// OAuthProvider OAuth 提供商配置表
type OAuthProvider struct {
	Name      string    `gorm:"primaryKey;size:50;not null" json:"name"` // github, google 等
	Enabled   bool      `gorm:"default:false" json:"enabled"`
	Addition  string    `gorm:"type:text" json:"addition"` // JSON 格式存储具体配置
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// GitHubOAuthAddition GitHub OAuth 配置详情
type GitHubOAuthAddition struct {
	ClientID     string   `json:"client_id"`
	ClientSecret string   `json:"client_secret"`
	AllowedUsers []string `json:"allowed_users"` // 允许登录的 GitHub 用户名列表
}
