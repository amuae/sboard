package handler

import (
	"encoding/json"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/sboard-go/sboard/internal/database"
)

// ========== 订阅配置 API ==========

const (
	ConfigKeyMihomoConfigs  = "mihomo_configs"
	ConfigKeySingBoxConfigs = "singbox_configs"
)

// SubscriptionConfigsResponse 订阅配置响应
type SubscriptionConfigsResponse struct {
	MihomoConfigs  json.RawMessage `json:"mihomoConfigs"`
	SingBoxConfigs json.RawMessage `json:"singboxConfigs"`
}

// SubscriptionConfigsRequest 保存订阅配置请求
type SubscriptionConfigsRequest struct {
	MihomoConfigs  json.RawMessage `json:"mihomoConfigs"`
	SingBoxConfigs json.RawMessage `json:"singboxConfigs"`
}

// handleGetSubscriptionConfigs 获取订阅配置
func (s *Server) handleGetSubscriptionConfigs(c *gin.Context) {
	mihomoConfigsJSON, err := database.GetSystemConfig(ConfigKeyMihomoConfigs)
	if err != nil {
		errorJSON(c, http.StatusInternalServerError, "获取 Mihomo 配置失败")
		return
	}

	singboxConfigsJSON, err := database.GetSystemConfig(ConfigKeySingBoxConfigs)
	if err != nil {
		errorJSON(c, http.StatusInternalServerError, "获取 SingBox 配置失败")
		return
	}

	// 如果为空，返回空数组
	if mihomoConfigsJSON == "" {
		mihomoConfigsJSON = "[]"
	}
	if singboxConfigsJSON == "" {
		singboxConfigsJSON = "[]"
	}

	c.JSON(http.StatusOK, SubscriptionConfigsResponse{
		MihomoConfigs:  json.RawMessage(mihomoConfigsJSON),
		SingBoxConfigs: json.RawMessage(singboxConfigsJSON),
	})
}

// handleSaveSubscriptionConfigs 保存订阅配置
func (s *Server) handleSaveSubscriptionConfigs(c *gin.Context) {
	var req SubscriptionConfigsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errorJSON(c, http.StatusBadRequest, "请求参数错误")
		return
	}

	// 验证 JSON 格式
	if len(req.MihomoConfigs) > 0 {
		var test []interface{}
		if err := json.Unmarshal(req.MihomoConfigs, &test); err != nil {
			errorJSON(c, http.StatusBadRequest, "Mihomo 配置格式错误")
			return
		}
	}

	if len(req.SingBoxConfigs) > 0 {
		var test []interface{}
		if err := json.Unmarshal(req.SingBoxConfigs, &test); err != nil {
			errorJSON(c, http.StatusBadRequest, "SingBox 配置格式错误")
			return
		}
	}

	// 保存 Mihomo 配置
	mihomoJSON := "[]"
	if len(req.MihomoConfigs) > 0 {
		mihomoJSON = string(req.MihomoConfigs)
	}
	if err := database.SetSystemConfig(ConfigKeyMihomoConfigs, mihomoJSON, "Mihomo 订阅配置模板"); err != nil {
		errorJSON(c, http.StatusInternalServerError, "保存 Mihomo 配置失败")
		return
	}

	// 保存 SingBox 配置
	singboxJSON := "[]"
	if len(req.SingBoxConfigs) > 0 {
		singboxJSON = string(req.SingBoxConfigs)
	}
	if err := database.SetSystemConfig(ConfigKeySingBoxConfigs, singboxJSON, "SingBox 订阅配置模板"); err != nil {
		errorJSON(c, http.StatusInternalServerError, "保存 SingBox 配置失败")
		return
	}

	successMsgJSON(c, "保存成功")
}

// GetEnabledSingBoxConfig 获取启用的 SingBox 配置（供订阅生成使用）
func GetEnabledSingBoxConfig() (*SingBoxSubscriptionConfig, error) {
	configsJSON, err := database.GetSystemConfig(ConfigKeySingBoxConfigs)
	if err != nil {
		return nil, err
	}

	if configsJSON == "" || configsJSON == "[]" {
		return nil, nil
	}

	var configs []SingBoxSubscriptionConfig
	if err := json.Unmarshal([]byte(configsJSON), &configs); err != nil {
		return nil, err
	}

	// 找到启用的配置
	for i := range configs {
		if configs[i].Enabled {
			return &configs[i], nil
		}
	}

	return nil, nil
}

// GetEnabledMihomoConfig 获取启用的 Mihomo 配置（供订阅生成使用）
func GetEnabledMihomoConfig() (*MihomoSubscriptionConfig, error) {
	configsJSON, err := database.GetSystemConfig(ConfigKeyMihomoConfigs)
	if err != nil {
		return nil, err
	}

	if configsJSON == "" || configsJSON == "[]" {
		return nil, nil
	}

	var configs []MihomoSubscriptionConfig
	if err := json.Unmarshal([]byte(configsJSON), &configs); err != nil {
		return nil, err
	}

	// 找到启用的配置
	for i := range configs {
		if configs[i].Enabled {
			return &configs[i], nil
		}
	}

	return nil, nil
}

// ========== 配置数据结构 ==========

// SingBoxSubscriptionConfig SingBox 订阅配置
type SingBoxSubscriptionConfig struct {
	ID          int                   `json:"id"`
	Name        string                `json:"name"`
	Description string                `json:"description"`
	Enabled     bool                  `json:"enabled"`
	Modules     []string              `json:"modules"`
	Config      SingBoxConfigTemplate `json:"config"`
}

// SingBoxConfigTemplate SingBox 配置模板（1.14）
type SingBoxConfigTemplate struct {
	Log            SingBoxLogConfig          `json:"log"`
	DNS            SingBoxDNSConfig          `json:"dns"`
	NTP            SingBoxNTPConfig          `json:"ntp"`
	Inbound        SingBoxInboundConfig      `json:"inbound"`
	OutboundGroups []SingBoxOutboundGroup    `json:"outboundGroups"`
	Route          SingBoxRouteConfig        `json:"route"`
	Experimental   SingBoxExperimentalConfig `json:"experimental"`
	HttpClients    []SingBoxHttpClient       `json:"httpClients"`
	Services       []SingBoxServiceConfig    `json:"services"`
	Providers      []SingBoxProviderConfig   `json:"providers"`
}

// SingBoxLogConfig SingBox 日志配置（1.14 新增 timestamp）
type SingBoxLogConfig struct {
	Disabled  bool   `json:"disabled"`
	Level     string `json:"level"`
	Output    string `json:"output"`
	Timestamp bool   `json:"timestamp"`
}

// SingBoxNTPConfig SingBox NTP 配置
type SingBoxNTPConfig struct {
	Enabled    bool   `json:"enabled"`
	Interval   string `json:"interval"`
	Server     string `json:"server"`
	ServerPort int    `json:"serverPort"`
}

// SingBoxDNSConfig SingBox DNS 配置（1.14）
type SingBoxDNSConfig struct {
	Servers       []SingBoxDNSServer `json:"servers"`
	Rules         []SingBoxDNSRule   `json:"rules"`
	Strategy      string             `json:"strategy"`
	Final         string             `json:"final"`
	ClientSubnet  string             `json:"clientSubnet"`
	Optimistic    bool               `json:"optimistic"`
	ReverseMapping bool              `json:"reverseMapping"`
	DisableCache  bool               `json:"disableCache"`
	CacheCapacity int                `json:"cacheCapacity"`
}

// SingBoxDNSServer SingBox DNS 服务器（支持 hosts/udp/quic/https/fakeip/group）
type SingBoxDNSServer struct {
	Tag            string              `json:"tag"`
	Type           string              `json:"type"`
	Server         string              `json:"server"`
	ServerPort     int                 `json:"serverPort"`
	Detour         string              `json:"detour"`
	DomainResolver string              `json:"domainResolver"`
	Inet4Range     string              `json:"inet4Range"`
	Inet6Range     string              `json:"inet6Range"`
	Predefined     map[string][]string `json:"predefined"`
	Servers        []string            `json:"servers"` // group type uses this
}

// SingBoxDNSRule SingBox DNS 规则（1.14 支持 rule_set）
type SingBoxDNSRule struct {
	Type       string   `json:"type"`
	Value      string   `json:"value"`
	Values     []string `json:"values"`
	Server     string   `json:"server"`
	Action     string   `json:"action"`
	RewriteTtl bool     `json:"rewriteTtl"`
}

// SingBoxInboundConfig SingBox 入站配置
type SingBoxInboundConfig struct {
	TunEnable      bool   `json:"tunEnable"`
	InterfaceName  string `json:"interfaceName"`
	Stack          string `json:"stack"`
	MTU            int    `json:"mtu"`
	AddressIPv4    string `json:"addressIpv4"`
	AddressIPv6    string `json:"addressIpv6"`
	AutoRoute      bool   `json:"autoRoute"`
	AutoRedirect   bool   `json:"autoRedirect"`
	StrictRoute    bool   `json:"strictRoute"`
}

// SingBoxOutboundGroup SingBox 策略组（支持 selector 类型）
type SingBoxOutboundGroup struct {
	Tag        string `json:"tag"`
	Type       string `json:"type"` // selector/urltest/direct
	FilterMode string `json:"filterMode"`
	Filter     string `json:"filter"`
	Include    string `json:"include"` // selector 的 include 正则
	URL        string `json:"url"`
	Interval   string `json:"interval"`
}

// SingBoxRouteConfig SingBox 路由配置（1.14）
type SingBoxRouteConfig struct {
	Final                 string             `json:"final"`
	DefaultDomainResolver string             `json:"defaultDomainResolver"`
	DefaultHttpClient     string             `json:"defaultHttpClient"`
	AutoDetectInterface   bool               `json:"autoDetectInterface"`
	FindProcess           bool               `json:"findProcess"`
	DefaultMark           int                `json:"defaultMark"`
	Rules                 []SingBoxRouteRule `json:"rules"`
	RuleSets              []SingBoxRuleSet   `json:"ruleSets"`
}

// SingBoxRouteRule SingBox 路由规则（支持 1.14 action）
type SingBoxRouteRule struct {
	Type      string           `json:"type"`
	Value     string           `json:"value"`
	Values    []string         `json:"values"`
	Action    string           `json:"action"`
	Outbound  string           `json:"outbound"`
	Mode      string           `json:"mode"`
	SubRules  []SingBoxSubRule `json:"subRules"`
	MatchOnly bool             `json:"matchOnly"` // resolve action 的 match_only
}

// SingBoxSubRule SingBox 子规则
type SingBoxSubRule struct {
	Type  string `json:"type"`
	Value string `json:"value"`
}

// SingBoxRuleSet SingBox 规则集
type SingBoxRuleSet struct {
	Tag    string `json:"tag"`
	Type   string `json:"type"`
	Format string `json:"format"`
	URL    string `json:"url"`
	Path   string `json:"path"` // remote proxy provider 使用
}

// SingBoxExperimentalConfig SingBox 实验性配置（1.14）
type SingBoxExperimentalConfig struct {
	CacheFileEnabled           bool   `json:"cacheFileEnabled"`
	StoreFakeip                bool   `json:"storeFakeip"`
	StoreRdrc                  bool   `json:"storeRdrc"`
	ExternalController         string `json:"externalController"`
	ExternalUi                 string `json:"externalUi"`
	ExternalUiDownloadUrl      string `json:"externalUiDownloadUrl"`
	ExternalUiHttpClient       string `json:"externalUiHttpClient"`
	DefaultMode                string `json:"defaultMode"`
	UrlTestUnifiedDelay        bool   `json:"urltestUnifiedDelay"`
}

// SingBoxHttpClient HTTP 客户端配置（1.14 完整）
type SingBoxHttpClient struct {
	Tag                  string            `json:"tag"`
	Version              int               `json:"version"`
	Headers              map[string]string `json:"headers"`
	Detour               string            `json:"detour"`
	StreamReceiveWindow  int               `json:"streamReceiveWindow"`
	ConnectionReceiveWindow int            `json:"connectionReceiveWindow"`
}

// SingBoxServiceConfig 服务配置
type SingBoxServiceConfig struct {
	Type       string `json:"type"`
	Listen     string `json:"listen"`
	ListenPort int    `json:"listenPort"`
}

// SingBoxProviderConfig 代理提供商配置（remote 类型）
type SingBoxProviderConfig struct {
	Type          string `json:"type"`
	Tag           string `json:"tag"`
	URL           string `json:"url"`
	Path          string `json:"path"`
	HttpClient    string `json:"httpClient"`
	UpdateInterval string `json:"updateInterval"`
}

// ========== 以下为 Mihomo 模型（未变） ==========

// MihomoSubscriptionConfig Mihomo 订阅配置
type MihomoSubscriptionConfig struct {
	ID          int                  `json:"id"`
	Name        string               `json:"name"`
	Description string               `json:"description"`
	Enabled     bool                 `json:"enabled"`
	Modules     []string             `json:"modules"`
	Config      MihomoConfigTemplate `json:"config"`
}

// MihomoProfileConfig Mihomo Profile 配置
type MihomoProfileConfig struct {
	StoreSelected bool `json:"storeSelected"`
	StoreFakeip   bool `json:"storeFakeip"`
}

// MihomoConfigTemplate Mihomo 配置模板
type MihomoConfigTemplate struct {
	Mode                    string `json:"mode"`
	BindAddress             string `json:"bindAddress"`
	MixedPort               int    `json:"mixedPort"`
	RedirPort               int    `json:"redirPort"`
	TproxyPort              int    `json:"tproxyPort"`
	LogLevel                string `json:"logLevel"`
	Ipv6                    bool   `json:"ipv6"`
	AllowLan                bool   `json:"allowLan"`
	UnifiedDelay            bool   `json:"unifiedDelay"`
	TcpConcurrent           bool   `json:"tcpConcurrent"`
	FindProcessMode         string `json:"findProcessMode"`
	GlobalClientFingerprint string `json:"globalClientFingerprint"`
	ExternalController      string `json:"externalController"`
	ExternalUi              string `json:"externalUi"`
	Profile MihomoProfileConfig `json:"profile"`
	Sniffer MihomoSnifferConfig `json:"sniffer"`
	Tun MihomoTunConfig `json:"tun"`
	DNS MihomoDNSConfig `json:"dns"`
	ProxyGroups []MihomoProxyGroup `json:"proxyGroups"`
	RuleProviders []MihomoRuleProvider `json:"ruleProviders"`
	Rules []MihomoRule `json:"rules"`
}

type MihomoSnifferConfig struct {
	Enable              bool                           `json:"enable"`
	OverrideDestination bool                           `json:"overrideDestination"`
	Sniff               map[string]MihomoSniffProtocol `json:"sniff"`
	ForceDomain         string                         `json:"forceDomain"`
	SkipDomain          string                         `json:"skipDomain"`
}

type MihomoSniffProtocol struct {
	OverrideDestination bool   `json:"overrideDestination"`
	Ports               string `json:"ports"`
}

type MihomoTunConfig struct {
	Enable              bool   `json:"enable"`
	Device              string `json:"device"`
	Stack               string `json:"stack"`
	DnsHijack           string `json:"dnsHijack"`
	UdpTimeout          int    `json:"udpTimeout"`
	AutoRoute           bool   `json:"autoRoute"`
	AutoRedirect        bool   `json:"autoRedirect"`
	AutoDetectInterface bool   `json:"autoDetectInterface"`
	StrictRoute         bool   `json:"strictRoute"`
}

type MihomoDNSConfig struct {
	Enable                bool   `json:"enable"`
	Ipv6                  bool   `json:"ipv6"`
	Listen                string `json:"listen"`
	EnhancedMode          string `json:"enhancedMode"`
	FakeIpRange           string `json:"fakeIpRange"`
	FakeIpFilter          string `json:"fakeIpFilter"`
	DefaultNameserver     string `json:"defaultNameserver"`
	Nameserver            string `json:"nameserver"`
	ProxyServerNameserver string `json:"proxyServerNameserver"`
	NameserverPolicy      string `json:"nameserverPolicy"`
}

type MihomoProxyGroup struct {
	Name          string `json:"name"`
	Type          string `json:"type"`
	FilterMode    string `json:"filterMode"`
	Filter        string `json:"filter"`
	ExcludeFilter string `json:"excludeFilter"`
	IncludeAll    bool   `json:"includeAll"`
	URL           string `json:"url"`
	Interval      int    `json:"interval"`
	Timeout       int    `json:"timeout"`
	Tolerance     int    `json:"tolerance"`
	Lazy          bool   `json:"lazy"`
	Hidden        bool   `json:"hidden"`
	Strategy      string `json:"strategy"`
}

type MihomoRuleProvider struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	Behavior string `json:"behavior"`
	Format   string `json:"format"`
	URL      string `json:"url"`
	Interval int    `json:"interval"`
	Proxy    string `json:"proxy"`
}

type MihomoRule struct {
	Type      string          `json:"type"`
	Value     string          `json:"value"`
	Outbound  string          `json:"outbound"`
	NoResolve bool            `json:"noResolve"`
	SubRules  []MihomoSubRule `json:"subRules,omitempty"`
}

type MihomoSubRule struct {
	Type  string `json:"type"`
	Value string `json:"value"`
}
