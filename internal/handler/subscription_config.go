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

// SingBoxConfigTemplate SingBox 配置模板
type SingBoxConfigTemplate struct {
	Log            SingBoxLogConfig          `json:"log"`
	DNS            SingBoxDNSConfig          `json:"dns"`
	Inbound        SingBoxInboundConfig      `json:"inbound"`
	OutboundGroups []SingBoxOutboundGroup    `json:"outboundGroups"`
	Route          SingBoxRouteConfig        `json:"route"`
	Experimental   SingBoxExperimentalConfig `json:"experimental"`
	HttpClients    []HttpClientConfig        `json:"httpClients"`
}

// SingBoxLogConfig SingBox 日志配置
type SingBoxLogConfig struct {
	Disabled bool   `json:"disabled"`
	Level    string `json:"level"`
	Output   string `json:"output"`
}

// SingBoxDNSConfig SingBox DNS 配置
type SingBoxDNSConfig struct {
	Servers        []SingBoxDNSServer `json:"servers"`
	Rules          []SingBoxDNSRule   `json:"rules"`
	Strategy       string             `json:"strategy"`
	Final          string             `json:"final"`
	ClientSubnet   string             `json:"clientSubnet"`
	Optimistic     bool               `json:"optimistic"`
	ReverseMapping bool               `json:"reverseMapping"`
	DisableCache   bool               `json:"disableCache"`
}

// SingBoxDNSServer SingBox DNS 服务器
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
}

// SingBoxDNSRule SingBox DNS 规则
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
	TunEnable     bool   `json:"tunEnable"`
	InterfaceName string `json:"interfaceName"`
	Stack         string `json:"stack"`
	MTU           int    `json:"mtu"`
	AddressIPv4   string `json:"addressIpv4"`
	AddressIPv6   string `json:"addressIpv6"`
	AutoRoute     bool   `json:"autoRoute"`
	AutoRedirect  bool   `json:"autoRedirect"`
	StrictRoute   bool   `json:"strictRoute"`
}

// SingBoxOutboundGroup SingBox 策略组
type SingBoxOutboundGroup struct {
	Tag        string `json:"tag"`
	Type       string `json:"type"`
	FilterMode string `json:"filterMode"`
	Filter     string `json:"filter"`
	URL        string `json:"url"`
	Interval   string `json:"interval"`
}

// SingBoxRouteConfig SingBox 路由配置
type SingBoxRouteConfig struct {
	Final                 string             `json:"final"`
	DefaultDomainResolver string             `json:"defaultDomainResolver"`
	AutoDetectInterface   bool               `json:"autoDetectInterface"`
	DefaultMark           int                `json:"defaultMark"`
	DefaultHttpClient     string             `json:"defaultHttpClient"`
	Rules                 []SingBoxRouteRule `json:"rules"`
	RuleSets              []SingBoxRuleSet   `json:"ruleSets"`
}

// SingBoxRouteRule SingBox 路由规则
type SingBoxRouteRule struct {
	Type     string           `json:"type"`
	Value    string           `json:"value"`
	Values   []string         `json:"values"`
	Action   string           `json:"action"`
	Outbound string           `json:"outbound"`
	Mode     string           `json:"mode"`
	SubRules []SingBoxSubRule `json:"subRules"`
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
}

// SingBoxExperimentalConfig SingBox 实验性配置
type SingBoxExperimentalConfig struct {
	CacheFileEnabled      bool   `json:"cacheFileEnabled"`
	StoreFakeip           bool   `json:"storeFakeip"`
	StoreDns              bool   `json:"storeDns"`
	ExternalController    string `json:"externalController"`
	ExternalUi            string `json:"externalUi"`
	ExternalUiDownloadUrl string `json:"externalUiDownloadUrl"`
}

// HttpClientConfig HTTP 客户端配置
type HttpClientConfig struct {
	Tag string `json:"tag"`
}

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
	// 基础设置
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
	// Profile 配置
	Profile MihomoProfileConfig `json:"profile"`
	// Sniffer
	Sniffer MihomoSnifferConfig `json:"sniffer"`
	// TUN
	Tun MihomoTunConfig `json:"tun"`
	// DNS
	DNS MihomoDNSConfig `json:"dns"`
	// 策略组
	ProxyGroups []MihomoProxyGroup `json:"proxyGroups"`
	// 规则集
	RuleProviders []MihomoRuleProvider `json:"ruleProviders"`
	// 路由规则
	Rules []MihomoRule `json:"rules"`
}

// MihomoSnifferConfig Mihomo 嗅探配置
type MihomoSnifferConfig struct {
	Enable              bool                           `json:"enable"`
	OverrideDestination bool                           `json:"overrideDestination"`
	Sniff               map[string]MihomoSniffProtocol `json:"sniff"`
	ForceDomain         string                         `json:"forceDomain"`
	SkipDomain          string                         `json:"skipDomain"`
}

// MihomoSniffProtocol Mihomo 嗅探协议配置
type MihomoSniffProtocol struct {
	OverrideDestination bool   `json:"overrideDestination"`
	Ports               string `json:"ports"`
}

// MihomoTunConfig Mihomo TUN 配置
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

// MihomoDNSConfig Mihomo DNS 配置
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

// MihomoProxyGroup Mihomo 策略组
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

// MihomoRuleProvider Mihomo 规则集
type MihomoRuleProvider struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	Behavior string `json:"behavior"`
	Format   string `json:"format"`
	URL      string `json:"url"`
	Interval int    `json:"interval"`
	Proxy    string `json:"proxy"`
}

// MihomoRule Mihomo 路由规则
type MihomoRule struct {
	Type      string          `json:"type"`
	Value     string          `json:"value"`
	Outbound  string          `json:"outbound"`
	NoResolve bool            `json:"noResolve"`
	SubRules  []MihomoSubRule `json:"subRules,omitempty"`
}

// MihomoSubRule Mihomo 子规则
type MihomoSubRule struct {
	Type  string `json:"type"`
	Value string `json:"value"`
}
