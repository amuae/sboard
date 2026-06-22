package handler

import (
	"encoding/json"
	"fmt"
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
		// 允许无 Origin 头的请求（agent 直连）
		origin := r.Header.Get("Origin")
		if origin == "" {
			return true
		}
		// 仅允许 panel 自身域名的 WebSocket 连接
		host := r.Host
		if host == "" {
			return false
		}
		return origin == "http://"+host || origin == "https://"+host
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

	// 心跳存活检测：记录最近的心跳时间戳
	heartbeatTimes []time.Time
}

// IsAlive 判断 Agent 是否存活（12秒内心跳达到4次）
func (c *AgentConnection) IsAlive() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.Conn == nil {
		return false
	}

	// 统计 12 秒内的心跳次数
	cutoff := time.Now().Add(-12 * time.Second)
	count := 0
	for _, t := range c.heartbeatTimes {
		if t.After(cutoff) {
			count++
		}
	}
	return count >= 4
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

// InitAgentHub starts the agent hub message pump. Called from NewServer.
func InitAgentHub() {
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
	if data.LocalIPv6 != "" {
		updates["host_ipv6"] = data.LocalIPv6
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

	now := time.Now()
	conn.mu.Lock()
	conn.LastHeartbeat = now
	if conn.Status == nil {
		conn.Status = &agent.StatusData{}
	}
	conn.Status.HeartbeatData = data

	// 记录心跳时间，用于存活判断
	conn.heartbeatTimes = append(conn.heartbeatTimes, now)
	// 清理 12 秒前的旧记录
	cutoff := now.Add(-12 * time.Second)
	validTimes := make([]time.Time, 0, len(conn.heartbeatTimes))
	for _, t := range conn.heartbeatTimes {
		if t.After(cutoff) {
			validTimes = append(validTimes, t)
		}
	}
	conn.heartbeatTimes = validTimes
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
			updates := map[string]interface{}{
				"last_heartbeat":        time.Now(),
				"cpu_usage":             data.CPUPercent,
				"mem_usage":             data.MemPercent,
				"disk_usage":            data.DiskPercent,
				"net_in":                data.NetIn,
				"net_out":               data.NetOut,
				"last_net_in_transfer":  data.NetInTransfer,
				"last_net_out_transfer": data.NetOutTransfer,
			}
			if data.LocalIP != "" {
				updates["host"] = data.LocalIP
			}
			if data.LocalIPv6 != "" {
				updates["host_ipv6"] = data.LocalIPv6
			}
			database.DB.Model(&database.Server{}).Where("id = ?", conn.ServerID).Updates(updates)

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

// SendCommand 发送命令到 Agent（等待响应）
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

// SendNotify 发送单向通知到 Agent（不等待响应）
func (h *AgentHub) SendNotify(serverID uint, msg *agent.Message) error {
	h.mu.RLock()
	conn, ok := h.serverMap[serverID]
	h.mu.RUnlock()

	if !ok || conn == nil {
		return &AgentOfflineError{ServerID: serverID}
	}

	msg.ID = generateMsgID()
	msg.Timestamp = time.Now().Unix()

	return conn.Conn.WriteJSON(msg)
}

// IsAgentOnline 检查 Agent 是否在线（12秒内心跳达到4次）
func (h *AgentHub) IsAgentOnline(serverID uint) bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	conn, ok := h.serverMap[serverID]
	return ok && conn != nil && conn.IsAlive()
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

// BroadcastConfigUpdate 向所有存活的 Agent 并发广播配置更新（单向通知，不等待响应）
// 存活判断：12秒内心跳达到4次
// 完全异步执行，不阻塞调用者
func (h *AgentHub) BroadcastConfigUpdate() {
	// 快速收集存活的 Agent 连接信息，尽快释放锁
	h.mu.RLock()
	aliveAgents := make([]struct {
		ServerID uint
		Conn     *websocket.Conn
	}, 0, len(h.serverMap))

	for serverID, conn := range h.serverMap {
		if conn != nil && conn.IsAlive() {
			aliveAgents = append(aliveAgents, struct {
				ServerID uint
				Conn     *websocket.Conn
			}{serverID, conn.Conn})
		}
	}
	h.mu.RUnlock()

	if len(aliveAgents) == 0 {
		return
	}

	// 每个 Agent 独立的 goroutine 处理，完全异步
	for _, agentInfo := range aliveAgents {
		go func(serverID uint, conn *websocket.Conn) {
			// 数据库查询在 goroutine 中执行，不阻塞主流程
			var server database.Server
			if err := database.DB.First(&server, serverID).Error; err != nil {
				return
			}

			// 生成配置
			config, err := GenerateServerConfig(&server, "sing-box")
			if err != nil {
				return
			}

			// 构造消息
			data := &agent.SyncConfigData{
				ConfigType: "sing-box",
				Content:    config,
				Restart:    true,
				TargetPath: "/opt/sboard/sing-box",
			}
			rawData, _ := json.Marshal(data)

			msg := &agent.Message{
				ID:        generateMsgID(),
				Type:      agent.MsgTypeSyncConfig,
				Data:      rawData,
				Timestamp: time.Now().Unix(),
			}

			// 单向发送，不等待响应
			conn.WriteJSON(msg)
		}(agentInfo.ServerID, agentInfo.Conn)
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
		configType = "sing-box"
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
		TargetPath: "/opt/sboard/sing-box",
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

	// 目标路径，默认 /opt/sboard/sing-box
	targetPath := req.TargetPath
	if targetPath == "" {
		targetPath = "/opt/sboard/sing-box"
	}

	// 生成配置
	config, err := GenerateServerConfig(&server, "sing-box")
	if err != nil {
		errorJSON(c, http.StatusInternalServerError, "生成配置失败: "+err.Error())
		return
	}

	// 发送部署命令
	data := &agent.DeployCoreData{
		CoreType:   "sing-box",
		TargetPath: targetPath,
		Config:     config,
	}
	rawData, _ := json.Marshal(data)

	msg := &agent.Message{
		Type: agent.MsgTypeDeployCore,
		Data: rawData,
	}

	resp, err := agentHub.SendCommand(uint(serverID), msg, 5*time.Second)
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
	now := time.Now()
	return fmt.Sprintf("%s-%06d", now.Format("20060102150405"), now.Nanosecond()/1000)
}

// handleDeployAll 全部部署（向所有存活 Agent 发送部署核心指令）
// 异步执行，立即返回，不阻塞面板
func (s *Server) handleDeployAll(c *gin.Context) {
	// 获取所有启用的服务器
	var servers []database.Server
	if err := database.DB.Where("enabled = ?", true).Find(&servers).Error; err != nil {
		errorJSON(c, http.StatusInternalServerError, "获取服务器列表失败")
		return
	}

	if len(servers) == 0 {
		errorJSON(c, http.StatusBadRequest, "没有启用的服务器")
		return
	}

	// 收集存活的 Agent
	type aliveServer struct {
		Server database.Server
		Conn   *websocket.Conn
	}
	aliveServers := []aliveServer{}

	agentHub.mu.RLock()
	for _, server := range servers {
		if conn, ok := agentHub.serverMap[server.ID]; ok && conn != nil && conn.IsAlive() {
			aliveServers = append(aliveServers, aliveServer{
				Server: server,
				Conn:   conn.Conn,
			})
		}
	}
	agentHub.mu.RUnlock()

	if len(aliveServers) == 0 {
		errorJSON(c, http.StatusBadRequest, "没有存活的 Agent")
		return
	}

	// 立即返回，后台异步执行部署
	go func(aliveServers []aliveServer) {
		for _, srv := range aliveServers {
			go func(srv aliveServer) {
				// 生成配置
				config, err := GenerateServerConfig(&srv.Server, "sing-box")
				if err != nil {
					return
				}

				// 发送部署核心指令
				deployCoreData := &agent.DeployCoreData{
					CoreType:   "sing-box",
					TargetPath: "/opt/sboard/sing-box",
					Config:     config,
				}
				rawData, _ := json.Marshal(deployCoreData)
				deployCoreMsg := &agent.Message{
					ID:        generateMsgID(),
					Type:      agent.MsgTypeDeployCore,
					Data:      rawData,
					Timestamp: time.Now().Unix(),
				}

				agentHub.SendCommand(srv.Server.ID, deployCoreMsg, 5*time.Second)
			}(srv)
		}
	}(aliveServers)

	successJSON(c, gin.H{
		"message": "部署任务已启动",
		"total":   len(aliveServers),
	})
}

// handleUpdateAgents 更新所有 Agent（向所有存活 Agent 发送自我更新指令）
func (s *Server) handleUpdateAgents(c *gin.Context) {
	// 获取所有启用的服务器
	var servers []database.Server
	if err := database.DB.Where("enabled = ?", true).Find(&servers).Error; err != nil {
		errorJSON(c, http.StatusInternalServerError, "获取服务器列表失败")
		return
	}

	if len(servers) == 0 {
		errorJSON(c, http.StatusBadRequest, "没有启用的服务器")
		return
	}

	// 收集存活的 Agent
	aliveCount := 0
	agentHub.mu.RLock()
	for _, server := range servers {
		if conn, ok := agentHub.serverMap[server.ID]; ok && conn != nil && conn.IsAlive() {
			// 发送自我更新指令
			selfUpdateMsg := &agent.Message{
				ID:        generateMsgID(),
				Type:      agent.MsgTypeSelfUpdate,
				Timestamp: time.Now().Unix(),
			}
			conn.Conn.WriteJSON(selfUpdateMsg)
			aliveCount++
		}
	}
	agentHub.mu.RUnlock()

	if aliveCount == 0 {
		errorJSON(c, http.StatusBadRequest, "没有存活的 Agent")
		return
	}

	successJSON(c, gin.H{
		"message": "Agent 更新指令已发送",
		"total":   aliveCount,
	})
}
