package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/sboard-go/sboard/internal/database"
)

// ========== 服务器落地出站 API ==========

const MaxOutboundSlots = 10 // 最大落地出站槽位数

// OutboundRequest 落地出站请求
type OutboundRequest struct {
	Enabled bool   `json:"enabled"`
	Remark  string `json:"remark"` // 备注（显示为按钮名称）

	// 出站配置
	Protocol string `json:"protocol"` // ss/trojan/anytls/socks5/vless/vmess/hysteria2
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Password string `json:"password"`
	Method   string `json:"method"`   // Shadowsocks 加密方式
	Username string `json:"username"` // SOCKS5 用户名
	Sni      string `json:"sni"`      // TLS SNI

	// VLESS/VMess 配置
	UUID     string `json:"uuid"`
	Flow     string `json:"flow"`
	Security string `json:"security"`
	AlterId  int    `json:"alter_id"`
	Tls      bool   `json:"tls"`
	Reality  bool   `json:"reality"`
	PubKey   string `json:"pub_key"`
	ShortId  string `json:"short_id"`
	Fp       string `json:"fp"`

	// Hysteria2 配置
	Obfs    string `json:"obfs"`
	ObfsPwd string `json:"obfs_pwd"`

	// 传输层配置 (VMess/VLESS)
	Network string `json:"network"`
	WsPath  string `json:"ws_path"`
	WsHost  string `json:"ws_host"`
}

// handleListServerOutbounds 获取服务器的所有落地出站
func (s *Server) handleListServerOutbounds(c *gin.Context) {
	serverID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		errorJSON(c, http.StatusBadRequest, "无效的服务器ID")
		return
	}

	// 检查服务器是否存在
	var server database.Server
	if err := database.GetDB().First(&server, serverID).Error; err != nil {
		errorJSON(c, http.StatusNotFound, "服务器不存在")
		return
	}

	// 获取服务器的所有落地出站（按槽位排序）
	var outbounds []database.ServerOutbound
	database.GetDB().Where("server_id = ?", serverID).Order("slot ASC").Find(&outbounds)

	successJSON(c, outbounds)
}

// handleGetServerOutbound 获取指定槽位的落地出站
func (s *Server) handleGetServerOutbound(c *gin.Context) {
	serverID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		errorJSON(c, http.StatusBadRequest, "无效的服务器ID")
		return
	}

	slot, err := strconv.Atoi(c.Param("slot"))
	if err != nil || slot < 1 || slot > MaxOutboundSlots {
		errorJSON(c, http.StatusBadRequest, "无效的槽位")
		return
	}

	var outbound database.ServerOutbound
	if err := database.GetDB().Where("server_id = ? AND slot = ?", serverID, slot).First(&outbound).Error; err != nil {
		errorJSON(c, http.StatusNotFound, "出站配置不存在")
		return
	}

	successJSON(c, outbound)
}

// handleCreateServerOutbound 创建落地出站
func (s *Server) handleCreateServerOutbound(c *gin.Context) {
	serverID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		errorJSON(c, http.StatusBadRequest, "无效的服务器ID")
		return
	}

	// 检查服务器是否存在
	var server database.Server
	if err := database.GetDB().First(&server, serverID).Error; err != nil {
		errorJSON(c, http.StatusNotFound, "服务器不存在")
		return
	}

	var req OutboundRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errorJSON(c, http.StatusBadRequest, "请求参数错误")
		return
	}

	// 查找下一个可用槽位
	var existingOutbounds []database.ServerOutbound
	database.GetDB().Where("server_id = ?", serverID).Order("slot ASC").Find(&existingOutbounds)

	if len(existingOutbounds) >= MaxOutboundSlots {
		errorJSON(c, http.StatusBadRequest, "已达到最大落地出站数量限制(10个)")
		return
	}

	// 重新分配槽位（从1开始连续分配）
	usedSlots := make(map[int]bool)
	for _, ob := range existingOutbounds {
		usedSlots[ob.Slot] = true
	}

	newSlot := 1
	for i := 1; i <= MaxOutboundSlots; i++ {
		if !usedSlots[i] {
			newSlot = i
			break
		}
	}

	// 创建新的落地出站
	outbound := database.ServerOutbound{
		ServerID: uint(serverID),
		Slot:     newSlot,
		Enabled:  req.Enabled,
		Remark:   req.Remark,
		Protocol: req.Protocol,
		Host:     req.Host,
		Port:     req.Port,
		Password: req.Password,
		Method:   req.Method,
		Username: req.Username,
		Sni:      req.Sni,
		UUID:     req.UUID,
		Flow:     req.Flow,
		Security: req.Security,
		AlterId:  req.AlterId,
		Tls:      req.Tls,
		Reality:  req.Reality,
		PubKey:   req.PubKey,
		ShortId:  req.ShortId,
		Fp:       req.Fp,
		Obfs:     req.Obfs,
		ObfsPwd:  req.ObfsPwd,
		Network:  req.Network,
		WsPath:   req.WsPath,
		WsHost:   req.WsHost,
	}

	if err := database.GetDB().Create(&outbound).Error; err != nil {
		errorJSON(c, http.StatusInternalServerError, "创建失败")
		return
	}

	// 广播配置更新到该服务器的 Agent
	go BroadcastConfigUpdateForce()

	successJSON(c, outbound)
}

// handleUpdateServerOutbound 更新落地出站
func (s *Server) handleUpdateServerOutbound(c *gin.Context) {
	serverID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		errorJSON(c, http.StatusBadRequest, "无效的服务器ID")
		return
	}

	slot, err := strconv.Atoi(c.Param("slot"))
	if err != nil || slot < 1 || slot > MaxOutboundSlots {
		errorJSON(c, http.StatusBadRequest, "无效的槽位")
		return
	}

	var outbound database.ServerOutbound
	if err := database.GetDB().Where("server_id = ? AND slot = ?", serverID, slot).First(&outbound).Error; err != nil {
		errorJSON(c, http.StatusNotFound, "出站配置不存在")
		return
	}

	var req OutboundRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errorJSON(c, http.StatusBadRequest, "请求参数错误")
		return
	}

	// 更新配置
	outbound.Enabled = req.Enabled
	outbound.Remark = req.Remark
	outbound.Protocol = req.Protocol
	outbound.Host = req.Host
	outbound.Port = req.Port
	outbound.Password = req.Password
	outbound.Method = req.Method
	outbound.Username = req.Username
	outbound.Sni = req.Sni
	outbound.UUID = req.UUID
	outbound.Flow = req.Flow
	outbound.Security = req.Security
	outbound.AlterId = req.AlterId
	outbound.Tls = req.Tls
	outbound.Reality = req.Reality
	outbound.PubKey = req.PubKey
	outbound.ShortId = req.ShortId
	outbound.Fp = req.Fp
	outbound.Obfs = req.Obfs
	outbound.ObfsPwd = req.ObfsPwd
	outbound.Network = req.Network
	outbound.WsPath = req.WsPath
	outbound.WsHost = req.WsHost

	if err := database.GetDB().Save(&outbound).Error; err != nil {
		errorJSON(c, http.StatusInternalServerError, "更新失败")
		return
	}

	// 广播配置更新到该服务器的 Agent
	go BroadcastConfigUpdateForce()

	successJSON(c, outbound)
}

// handleDeleteServerOutbound 删除落地出站
func (s *Server) handleDeleteServerOutbound(c *gin.Context) {
	serverID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		errorJSON(c, http.StatusBadRequest, "无效的服务器ID")
		return
	}

	slot, err := strconv.Atoi(c.Param("slot"))
	if err != nil || slot < 1 || slot > MaxOutboundSlots {
		errorJSON(c, http.StatusBadRequest, "无效的槽位")
		return
	}

	result := database.GetDB().Unscoped().Where("server_id = ? AND slot = ?", serverID, slot).Delete(&database.ServerOutbound{})
	if result.RowsAffected == 0 {
		errorJSON(c, http.StatusNotFound, "出站配置不存在")
		return
	}

	// 重新排序剩余的槽位（从1开始连续）
	reorderOutboundSlots(uint(serverID))

	// 广播配置更新到所有在线 Agent
	go BroadcastConfigUpdateForce()

	successMsgJSON(c, "删除成功")
}

// handleToggleServerOutbound 切换落地出站启用状态
func (s *Server) handleToggleServerOutbound(c *gin.Context) {
	serverID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		errorJSON(c, http.StatusBadRequest, "无效的服务器ID")
		return
	}

	slot, err := strconv.Atoi(c.Param("slot"))
	if err != nil || slot < 1 || slot > MaxOutboundSlots {
		errorJSON(c, http.StatusBadRequest, "无效的槽位")
		return
	}

	var outbound database.ServerOutbound
	if err := database.GetDB().Where("server_id = ? AND slot = ?", serverID, slot).First(&outbound).Error; err != nil {
		errorJSON(c, http.StatusNotFound, "出站配置不存在")
		return
	}

	outbound.Enabled = !outbound.Enabled
	if err := database.GetDB().Save(&outbound).Error; err != nil {
		errorJSON(c, http.StatusInternalServerError, "更新失败")
		return
	}

	// 广播配置更新到该服务器的 Agent
	go BroadcastConfigUpdateForce()

	successJSON(c, outbound)
}

// reorderOutboundSlots 重新排序槽位（删除后从1开始连续编号）
func reorderOutboundSlots(serverID uint) {
	var outbounds []database.ServerOutbound
	database.GetDB().Where("server_id = ?", serverID).Order("slot ASC").Find(&outbounds)

	for i, ob := range outbounds {
		newSlot := i + 1
		if ob.Slot != newSlot {
			database.GetDB().Model(&ob).Update("slot", newSlot)
		}
	}
}
