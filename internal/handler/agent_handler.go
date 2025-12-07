package handler

import (
	"encoding/json"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/sboard-go/sboard/internal/agent"
	"github.com/sboard-go/sboard/internal/database"
	"gorm.io/gorm"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

// AgentConnection Agent 连接
type AgentConnection struct {
	ID            string
	Conn          *websocket.Conn
	ServerID      uint
	RegisterTime  time.Time
	LastHeartbeat time.Time
	Status        *agent.StatusData
	mu            sync.RWMutex
}

// AgentHub Agent 连接管理器
type AgentHub struct {
	agents     map[string]*AgentConnection // agentID -> connection
	serverMap  map[uint]*AgentConnection   // serverID -> connection
	mu         sync.RWMutex
	msgChan    chan *AgentMessage
	pendingReq map[string]chan *agent.Message // msgID -> response channel
	pendingMu  sync.RWMutex
}

// AgentMessage 带连接的消息
type AgentMessage struct {
	Conn    *AgentConnection
	Message *agent.Message
}

var agentHub = &AgentHub{
	agents:     make(map[string]*AgentConnection),
	serverMap:  make(map[uint]*AgentConnection),
	msgChan:    make(chan *AgentMessage, 100),
	pendingReq: make(map[string]chan *agent.Message),
}

func init() {
	go agentHub.run()
}

func (h *AgentHub) run() {
	for msg := range h.msgChan {
		h.handleMessage(msg)
	}
}

func (h *AgentHub) handleMessage(am *AgentMessage) {
	msg := am.Message
	conn := am.Conn

	switch msg.Type {
	case agent.MsgTypeRegister:
		h.handleRegister(conn, msg)
	case agent.MsgTypeHeartbeat:
		h.handleHeartbeat(conn, msg)
	case agent.MsgTypeStatus:
		h.handleStatus(conn, msg)
	case agent.MsgTypeCommandResp:
		h.handleCommandResp(msg)
	}
}

func (h *AgentHub) handleRegister(conn *AgentConnection, msg *agent.Message) {
	var data agent.RegisterData
	if err := msg.ParseData(&data); err != nil {
		return
	}

	// 验证 Token 并查找服务器
	var server database.Server
	if err := database.DB.Where("agent_token = ?", data.Token).First(&server).Error; err != nil {
		conn.Conn.WriteJSON(&agent.Message{
			Type:  agent.MsgTypeRegisterResp,
			Error: "认证失败",
		})
		conn.Conn.Close()
		return
	}

	// 更新连接信息
	conn.ID = data.AgentID
	conn.ServerID = server.ID
	conn.RegisterTime = time.Now()
	conn.LastHeartbeat = time.Now()

	// 注册到 hub
	h.mu.Lock()
	h.agents[data.AgentID] = conn
	h.serverMap[server.ID] = conn
	h.mu.Unlock()

	// 更新服务器状态
	updates := map[string]interface{}{
		"agent_online":   true,
		"agent_id":       data.AgentID,
		"agent_version":  data.Version,
		"last_heartbeat": time.Now(),
	}
	// 使用 Agent 上报的本机 IP (从网卡获取) 更新服务器 host
	if data.LocalIP != "" {
		updates["host"] = data.LocalIP
	}
	database.DB.Model(&server).Updates(updates)

	// 发送注册成功响应
	conn.Conn.WriteJSON(&agent.Message{
		Type:      agent.MsgTypeRegisterResp,
		Timestamp: time.Now().Unix(),
	})
}

func (h *AgentHub) handleHeartbeat(conn *AgentConnection, msg *agent.Message) {
	var data agent.HeartbeatData
	if err := msg.ParseData(&data); err != nil {
		return
	}

	conn.mu.Lock()
	conn.LastHeartbeat = time.Now()
	if conn.Status == nil {
		conn.Status = &agent.StatusData{}
	}
	conn.Status.HeartbeatData = data
	conn.mu.Unlock()

	// 更新数据库
	if conn.ServerID > 0 {
		// 先获取上次的 transfer 值
		var server database.Server
		if err := database.DB.Select("last_net_in_transfer", "last_net_out_transfer").
			Where("id = ?", conn.ServerID).First(&server).Error; err == nil {

			// 计算差值（仅当新值大于旧值时累加，避免服务器重启导致负值）
			var deltaIn, deltaOut uint64
			if data.NetInTransfer > server.LastNetInTransfer && server.LastNetInTransfer > 0 {
				deltaIn = data.NetInTransfer - server.LastNetInTransfer
			}
			if data.NetOutTransfer > server.LastNetOutTransfer && server.LastNetOutTransfer > 0 {
				deltaOut = data.NetOutTransfer - server.LastNetOutTransfer
			}

			// 更新实时状态和上次 transfer 值
			database.DB.Model(&database.Server{}).Where("id = ?", conn.ServerID).Updates(map[string]interface{}{
				"last_heartbeat":        time.Now(),
				"cpu_usage":             data.CPUPercent,
				"mem_usage":             data.MemPercent,
				"disk_usage":            data.DiskPercent,
				"net_in":                data.NetIn,
				"net_out":               data.NetOut,
				"last_net_in_transfer":  data.NetInTransfer,
				"last_net_out_transfer": data.NetOutTransfer,
			})

			// 累加月度流量（使用精确差值）
			if deltaIn > 0 || deltaOut > 0 {
				database.DB.Model(&database.Server{}).Where("id = ?", conn.ServerID).
					UpdateColumn("monthly_in", gorm.Expr("monthly_in + ?", deltaIn)).
					UpdateColumn("monthly_out", gorm.Expr("monthly_out + ?", deltaOut))
			}
		}
	}
}

func (h *AgentHub) handleStatus(conn *AgentConnection, msg *agent.Message) {
	var data agent.StatusData
	if err := msg.ParseData(&data); err != nil {
		return
	}

	conn.mu.Lock()
	conn.Status = &data
	conn.mu.Unlock()
}

func (h *AgentHub) handleCommandResp(msg *agent.Message) {
	if msg.ID == "" {
		return
	}

	h.pendingMu.RLock()
	ch, ok := h.pendingReq[msg.ID]
	h.pendingMu.RUnlock()

	if ok {
		select {
		case ch <- msg:
		default:
		}
	}
}

// SendCommand 发送命令到 Agent
func (h *AgentHub) SendCommand(serverID uint, msg *agent.Message, timeout time.Duration) (*agent.Message, error) {
	h.mu.RLock()
	conn, ok := h.serverMap[serverID]
	h.mu.RUnlock()

	if !ok || conn == nil {
		return nil, &AgentOfflineError{ServerID: serverID}
	}

	// 生成消息 ID
	msg.ID = generateMsgID()
	msg.Timestamp = time.Now().Unix()

	// 创建响应通道
	respChan := make(chan *agent.Message, 1)
	h.pendingMu.Lock()
	h.pendingReq[msg.ID] = respChan
	h.pendingMu.Unlock()

	defer func() {
		h.pendingMu.Lock()
		delete(h.pendingReq, msg.ID)
		h.pendingMu.Unlock()
	}()

	// 发送消息
	if err := conn.Conn.WriteJSON(msg); err != nil {
		return nil, err
	}

	// 等待响应
	select {
	case resp := <-respChan:
		return resp, nil
	case <-time.After(timeout):
		return nil, &TimeoutError{MsgID: msg.ID}
	}
}

// IsAgentOnline 检查 Agent 是否在线
func (h *AgentHub) IsAgentOnline(serverID uint) bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	conn, ok := h.serverMap[serverID]
	return ok && conn != nil
}

// GetAgentStatus 获取 Agent 状态
func (h *AgentHub) GetAgentStatus(serverID uint) *agent.StatusData {
	h.mu.RLock()
	conn, ok := h.serverMap[serverID]
	h.mu.RUnlock()

	if !ok || conn == nil {
		return nil
	}

	conn.mu.RLock()
	defer conn.mu.RUnlock()
	return conn.Status
}

// RemoveAgent 移除 Agent
func (h *AgentHub) RemoveAgent(agentID string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if conn, ok := h.agents[agentID]; ok {
		if conn.ServerID > 0 {
			delete(h.serverMap, conn.ServerID)
			// 更新服务器离线状态
			database.DB.Model(&database.Server{}).Where("id = ?", conn.ServerID).Update("agent_online", false)
		}
		delete(h.agents, agentID)
		conn.Conn.Close()
	}
}

// GetOnlineServers 获取所有在线的服务器 ID
func (h *AgentHub) GetOnlineServers() []uint {
	h.mu.RLock()
	defer h.mu.RUnlock()

	serverIDs := make([]uint, 0, len(h.serverMap))
	for serverID := range h.serverMap {
		serverIDs = append(serverIDs, serverID)
	}
	return serverIDs
}

// BroadcastConfigUpdate 向所有在线 Agent 广播配置更新
func (h *AgentHub) BroadcastConfigUpdate() {
	serverIDs := h.GetOnlineServers()
	if len(serverIDs) == 0 {
		return
	}

	for _, serverID := range serverIDs {
		go func(sid uint) {
			// 获取服务器信息
			var server database.Server
			if err := database.DB.First(&server, sid).Error; err != nil {
				return
			}

			// 生成配置
			config, err := GenerateServerConfig(&server, server.CoreType)
			if err != nil {
				return
			}

			// 发送同步命令
			data := &agent.SyncConfigData{
				ConfigType: server.CoreType,
				Content:    config,
				Restart:    true,
			}
			rawData, _ := json.Marshal(data)

			msg := &agent.Message{
				Type: agent.MsgTypeSyncConfig,
				Data: rawData,
			}

			_, err = h.SendCommand(sid, msg, 60*time.Second)
			if err != nil {
				// 同步失败时静默处理
			}
		}(serverID)
	}
}

// BroadcastConfigUpdate 包级别导出函数，供外部调用
func BroadcastConfigUpdate() {
	agentHub.BroadcastConfigUpdate()
}

// AgentOfflineError Agent 离线错误
type AgentOfflineError struct {
	ServerID uint
}

func (e *AgentOfflineError) Error() string {
	return "Agent 离线"
}

// TimeoutError 超时错误
type TimeoutError struct {
	MsgID string
}

func (e *TimeoutError) Error() string {
	return "请求超时"
}

// handleAgentWebSocket 处理 Agent WebSocket 连接
func (s *Server) handleAgentWebSocket(c *gin.Context) {
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}

	agentConn := &AgentConnection{
		Conn: conn,
	}

	// 读取消息循环
	for {
		var msg agent.Message
		if err := conn.ReadJSON(&msg); err != nil {
			break
		}

		agentHub.msgChan <- &AgentMessage{
			Conn:    agentConn,
			Message: &msg,
		}
	}

	// 清理连接
	if agentConn.ID != "" {
		agentHub.RemoveAgent(agentConn.ID)
	}
}

// handleGetAgentStatus 获取 Agent 状态
func (s *Server) handleGetAgentStatus(c *gin.Context) {
	serverID := parseUint(c.Param("id"))
	if serverID == 0 {
		errorJSON(c, http.StatusBadRequest, "无效的服务器ID")
		return
	}

	online := agentHub.IsAgentOnline(uint(serverID))
	status := agentHub.GetAgentStatus(uint(serverID))

	successJSON(c, gin.H{
		"online": online,
		"status": status,
	})
}

// handleSendAgentCommand 发送命令到 Agent
func (s *Server) handleSendAgentCommand(c *gin.Context) {
	serverID := parseUint(c.Param("id"))
	if serverID == 0 {
		errorJSON(c, http.StatusBadRequest, "无效的服务器ID")
		return
	}

	var req struct {
		Command string   `json:"command"`
		Args    []string `json:"args"`
		Timeout int      `json:"timeout"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		errorJSON(c, http.StatusBadRequest, "请求参数错误")
		return
	}

	data := &agent.CommandData{
		Command: req.Command,
		Args:    req.Args,
		Timeout: req.Timeout,
	}
	rawData, _ := json.Marshal(data)

	msg := &agent.Message{
		Type: agent.MsgTypeCommand,
		Data: rawData,
	}

	timeout := time.Duration(req.Timeout) * time.Second
	if timeout == 0 {
		timeout = 60 * time.Second
	}

	resp, err := agentHub.SendCommand(uint(serverID), msg, timeout)
	if err != nil {
		if _, ok := err.(*AgentOfflineError); ok {
			errorJSON(c, http.StatusServiceUnavailable, "Agent 离线")
			return
		}
		errorJSON(c, http.StatusGatewayTimeout, err.Error())
		return
	}

	if resp.Error != "" {
		errorJSON(c, http.StatusInternalServerError, resp.Error)
		return
	}

	var respData agent.CommandRespData
	resp.ParseData(&respData)

	successJSON(c, respData)
}

// handleSyncConfigToAgent 同步配置到 Agent
func (s *Server) handleSyncConfigToAgent(c *gin.Context) {
	serverID := parseUint(c.Param("id"))
	if serverID == 0 {
		errorJSON(c, http.StatusBadRequest, "无效的服务器ID")
		return
	}

	var req struct {
		ConfigType string `json:"config_type"`
		Restart    bool   `json:"restart"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		errorJSON(c, http.StatusBadRequest, "请求参数错误")
		return
	}

	// 获取服务器
	var server database.Server
	if err := database.DB.First(&server, serverID).Error; err != nil {
		errorJSON(c, http.StatusNotFound, "服务器不存在")
		return
	}

	// 生成配置
	configType := req.ConfigType
	if configType == "" {
		configType = server.CoreType
	}

	config, err := GenerateServerConfig(&server, configType)
	if err != nil {
		errorJSON(c, http.StatusInternalServerError, "生成配置失败: "+err.Error())
		return
	}

	// 发送同步命令
	data := &agent.SyncConfigData{
		ConfigType: configType,
		Content:    config,
		Restart:    req.Restart,
	}
	rawData, _ := json.Marshal(data)

	msg := &agent.Message{
		Type: agent.MsgTypeSyncConfig,
		Data: rawData,
	}

	resp, err := agentHub.SendCommand(uint(serverID), msg, 60*time.Second)
	if err != nil {
		if _, ok := err.(*AgentOfflineError); ok {
			errorJSON(c, http.StatusServiceUnavailable, "Agent 离线")
			return
		}
		errorJSON(c, http.StatusGatewayTimeout, err.Error())
		return
	}

	if resp.Error != "" {
		errorJSON(c, http.StatusInternalServerError, resp.Error)
		return
	}

	successMsgJSON(c, "配置同步成功")
}

// handleDeployCoreToAgent 部署核心到 Agent
func (s *Server) handleDeployCoreToAgent(c *gin.Context) {
	serverID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		errorJSON(c, http.StatusBadRequest, "无效的服务器ID")
		return
	}

	var req struct {
		CoreType   string `json:"core_type"`   // sing-box 或 mihomo
		TargetPath string `json:"target_path"` // 目标安装路径
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		errorJSON(c, http.StatusBadRequest, "请求参数错误")
		return
	}

	// 获取服务器
	var server database.Server
	if err := database.DB.First(&server, serverID).Error; err != nil {
		errorJSON(c, http.StatusNotFound, "服务器不存在")
		return
	}

	// 核心类型，默认使用服务器配置的核心类型
	coreType := req.CoreType
	if coreType == "" {
		coreType = server.CoreType
	}

	// 目标路径，默认 /root/{core_type}
	targetPath := req.TargetPath
	if targetPath == "" {
		targetPath = "/root/" + coreType
	}

	// 生成配置
	config, err := GenerateServerConfig(&server, coreType)
	if err != nil {
		errorJSON(c, http.StatusInternalServerError, "生成配置失败: "+err.Error())
		return
	}

	// 发送部署命令
	data := &agent.DeployCoreData{
		CoreType:   coreType,
		TargetPath: targetPath,
		Config:     config,
	}
	rawData, _ := json.Marshal(data)

	msg := &agent.Message{
		Type: agent.MsgTypeDeployCore,
		Data: rawData,
	}

	resp, err := agentHub.SendCommand(uint(serverID), msg, 120*time.Second)
	if err != nil {
		if _, ok := err.(*AgentOfflineError); ok {
			errorJSON(c, http.StatusServiceUnavailable, "Agent 离线")
			return
		}
		errorJSON(c, http.StatusGatewayTimeout, err.Error())
		return
	}

	if resp.Error != "" {
		errorJSON(c, http.StatusInternalServerError, resp.Error)
		return
	}

	// 解析响应
	var respData agent.DeployCoreRespData
	if err := resp.ParseData(&respData); err != nil {
		errorJSON(c, http.StatusInternalServerError, "解析响应失败")
		return
	}

	successJSON(c, respData)
}

// 辅助函数
func parseUint(s string) uint64 {
	var n uint64
	for _, c := range s {
		if c >= '0' && c <= '9' {
			n = n*10 + uint64(c-'0')
		}
	}
	return n
}

func generateMsgID() string {
	return time.Now().Format("20060102150405.000000")
}
