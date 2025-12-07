package agent

import (
	"encoding/json"
	"time"
)

// MessageType 消息类型
type MessageType string

const (
	// Agent -> Panel
	MsgTypeHeartbeat   MessageType = "heartbeat"    // 心跳
	MsgTypeRegister    MessageType = "register"     // 注册
	MsgTypeStatus      MessageType = "status"       // 状态上报
	MsgTypeCommandResp MessageType = "command_resp" // 命令响应

	// Panel -> Agent
	MsgTypeRegisterResp MessageType = "register_resp" // 注册响应
	MsgTypeCommand      MessageType = "command"       // 执行命令
	MsgTypeDeployFile   MessageType = "deploy_file"   // 部署单文件
	MsgTypeDeployDir    MessageType = "deploy_dir"    // 部署目录
	MsgTypeSyncConfig   MessageType = "sync_config"   // 同步配置
	MsgTypeRestart      MessageType = "restart"       // 重启服务
	MsgTypeDeployCore   MessageType = "deploy_core"   // 部署核心
)

// Message WebSocket 消息结构
type Message struct {
	Type      MessageType     `json:"type"`
	ID        string          `json:"id,omitempty"`    // 消息ID，用于请求响应匹配
	Timestamp int64           `json:"timestamp"`       // 时间戳
	Data      json.RawMessage `json:"data,omitempty"`  // 消息数据
	Error     string          `json:"error,omitempty"` // 错误信息
}

// RegisterData 注册数据
type RegisterData struct {
	Token    string `json:"token"`     // 认证 Token
	AgentID  string `json:"agent_id"`  // Agent 唯一标识
	Version  string `json:"version"`   // Agent 版本
	Hostname string `json:"hostname"`  // 主机名
	LocalIP  string `json:"local_ip"`  // 本机 IP (从网卡获取)
	OS       string `json:"os"`        // 操作系统
	Arch     string `json:"arch"`      // CPU 架构
	CoreType string `json:"core_type"` // 核心类型: sing-box/mihomo
}

// HeartbeatData 心跳数据
type HeartbeatData struct {
	Uptime         int64   `json:"uptime"`           // 运行时长(秒)
	CPUPercent     float64 `json:"cpu_percent"`      // CPU 使用率
	MemPercent     float64 `json:"mem_percent"`      // 内存使用率
	DiskPercent    float64 `json:"disk_percent"`     // 磁盘使用率
	NetIn          int64   `json:"net_in"`           // 网络入流量速率(bytes/s)
	NetOut         int64   `json:"net_out"`          // 网络出流量速率(bytes/s)
	NetInTransfer  uint64  `json:"net_in_transfer"`  // 系统启动以来的入流量总量(bytes)
	NetOutTransfer uint64  `json:"net_out_transfer"` // 系统启动以来的出流量总量(bytes)
	Connections    int     `json:"connections"`      // 当前连接数
}

// StatusData 状态数据
type StatusData struct {
	HeartbeatData
	Hostname      string    `json:"hostname"`
	PublicIP      string    `json:"public_ip"`
	LocalIP       string    `json:"local_ip"`
	OS            string    `json:"os"`
	Arch          string    `json:"arch"`
	CoreType      string    `json:"core_type"`
	CoreVersion   string    `json:"core_version"`
	CoreRunning   bool      `json:"core_running"`
	ConfigVersion string    `json:"config_version"` // 配置版本(MD5)
	LastSync      time.Time `json:"last_sync"`
}

// CommandData 命令数据
type CommandData struct {
	Command string   `json:"command"`            // 命令
	Args    []string `json:"args,omitempty"`     // 参数
	Timeout int      `json:"timeout,omitempty"`  // 超时(秒)
	WorkDir string   `json:"work_dir,omitempty"` // 工作目录
}

// CommandRespData 命令响应数据
type CommandRespData struct {
	ExitCode int    `json:"exit_code"` // 退出码
	Stdout   string `json:"stdout"`    // 标准输出
	Stderr   string `json:"stderr"`    // 标准错误
	Duration int64  `json:"duration"`  // 执行时长(ms)
}

// DeployFileData 部署文件数据
type DeployFileData struct {
	Path       string `json:"path"`                  // 目标路径
	Content    string `json:"content"`               // 文件内容(base64)
	Mode       int    `json:"mode,omitempty"`        // 文件权限
	Backup     bool   `json:"backup,omitempty"`      // 是否备份
	RestartSvc string `json:"restart_svc,omitempty"` // 部署后重启的服务
}

// DeployDirData 部署目录数据
type DeployDirData struct {
	Path   string            `json:"path"`             // 目标路径
	Files  map[string]string `json:"files"`            // 文件名 -> 内容(base64)
	Modes  map[string]int    `json:"modes,omitempty"`  // 文件名 -> 权限
	Clean  bool              `json:"clean,omitempty"`  // 是否清空目录
	Backup bool              `json:"backup,omitempty"` // 是否备份
}

// SyncConfigData 同步配置数据
type SyncConfigData struct {
	ConfigType string `json:"config_type"` // sing-box / mihomo
	Content    string `json:"content"`     // 配置内容
	Restart    bool   `json:"restart"`     // 是否重启服务
}

// RestartData 重启服务数据
type RestartData struct {
	Service string `json:"service"` // 服务名: sing-box / mihomo
}

// DeployCoreData 部署核心数据
type DeployCoreData struct {
	CoreType   string `json:"core_type"`   // sing-box 或 mihomo
	TargetPath string `json:"target_path"` // 目标安装路径，如 /root/sing-box
	Config     string `json:"config"`      // 配置文件内容
}

// DeployCoreRespData 部署核心响应数据
type DeployCoreRespData struct {
	Success      bool   `json:"success"`
	CoreType     string `json:"core_type"`
	TargetPath   string `json:"target_path"`
	ServiceName  string `json:"service_name"`
	Running      bool   `json:"running"`
	OtherService string `json:"other_service"`
	OtherRunning bool   `json:"other_running"`
}

// NewMessage 创建新消息
func NewMessage(msgType MessageType, data interface{}) (*Message, error) {
	var rawData json.RawMessage
	if data != nil {
		var err error
		rawData, err = json.Marshal(data)
		if err != nil {
			return nil, err
		}
	}
	return &Message{
		Type:      msgType,
		Timestamp: time.Now().Unix(),
		Data:      rawData,
	}, nil
}

// ParseData 解析消息数据
func (m *Message) ParseData(v interface{}) error {
	if m.Data == nil {
		return nil
	}
	return json.Unmarshal(m.Data, v)
}
