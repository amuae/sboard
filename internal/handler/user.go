package handler

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/sboard-go/sboard/internal/database"
)

// CreateUserRequest 创建用户请求
type CreateUserRequest struct {
	Name         string `json:"name" binding:"required"`
	UUID         string `json:"uuid"`
	Level        int    `json:"level"`
	ExpiryDate   string `json:"expiry_date" binding:"required"`
	Enabled      int    `json:"enabled"`
	TrafficLimit int64  `json:"traffic_limit"`
	DnsResolve   string `json:"dns_resolve"`
	Notes        string `json:"notes"`
}

// UpdateUserRequest 更新用户请求
type UpdateUserRequest struct {
	Name         string `json:"name"`
	Level        int    `json:"level"`
	ExpiryDate   string `json:"expiry_date"`
	Enabled      int    `json:"enabled"`
	TrafficLimit int64  `json:"traffic_limit"`
	TrafficUsed  int64  `json:"traffic_used"`
	DnsResolve   string `json:"dns_resolve"`
	Notes        string `json:"notes"`
}

// handleListUsers 获取用户列表（按等级分组）
func (s *Server) handleListUsers(c *gin.Context) {
	var users []database.ProxyUser

	query := database.DB.Order("level ASC, id ASC")

	// 支持搜索
	if search := c.Query("search"); search != "" {
		query = query.Where("name LIKE ?", "%"+search+"%")
	}

	// 支持启用状态过滤
	if enableStr := c.Query("enabled"); enableStr != "" {
		enabled, _ := strconv.Atoi(enableStr)
		query = query.Where("enabled = ?", enabled)
	}

	// 支持等级过滤
	if levelStr := c.Query("level"); levelStr != "" {
		level, _ := strconv.Atoi(levelStr)
		query = query.Where("level = ?", level)
	}

	if err := query.Find(&users).Error; err != nil {
		errorJSON(c, http.StatusInternalServerError, "查询失败")
		return
	}

	// 按等级分组
	level1Users := []database.ProxyUser{}
	level2Users := []database.ProxyUser{}
	level3Users := []database.ProxyUser{}

	for _, user := range users {
		switch user.Level {
		case 1:
			level1Users = append(level1Users, user)
		case 2:
			level2Users = append(level2Users, user)
		case 3:
			level3Users = append(level3Users, user)
		default:
			level1Users = append(level1Users, user)
		}
	}

	// 获取统计信息
	var total int64
	database.DB.Model(&database.ProxyUser{}).Count(&total)

	var enabledCount int64
	database.DB.Model(&database.ProxyUser{}).Where("enabled = ?", 1).Count(&enabledCount)

	successJSON(c, gin.H{
		"users": users,
		"grouped": gin.H{
			"level_1": level1Users,
			"level_2": level2Users,
			"level_3": level3Users,
		},
		"stats": gin.H{
			"total":   total,
			"enabled": enabledCount,
		},
	})
} // handleGetUser 获取单个用户
func (s *Server) handleGetUser(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		errorJSON(c, http.StatusBadRequest, "无效的用户ID")
		return
	}

	var user database.ProxyUser
	if err := database.DB.First(&user, id).Error; err != nil {
		errorJSON(c, http.StatusNotFound, "用户不存在")
		return
	}

	successJSON(c, user)
}

// handleCreateUser 创建用户
func (s *Server) handleCreateUser(c *gin.Context) {
	var req CreateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errorJSON(c, http.StatusBadRequest, "请求参数错误")
		return
	}

	// 检查名称是否已存在
	var count int64
	database.DB.Model(&database.ProxyUser{}).Where("name = ?", req.Name).Count(&count)
	if count > 0 {
		errorJSON(c, http.StatusBadRequest, "用户名已存在")
		return
	}

	// 设置 UUID：优先使用前端传来的，否则自动生成
	userUUID := req.UUID
	if _, err := uuid.Parse(userUUID); err != nil {
		userUUID = uuid.New().String()
	}

	// 设置默认值
	if req.Level == 0 {
		req.Level = 1
	}
	if req.DnsResolve == "" {
		req.DnsResolve = "default"
	}
	if req.Enabled == 0 {
		req.Enabled = 1
	}

	user := database.ProxyUser{
		Name:         req.Name,
		UUID:         userUUID,
		Level:        req.Level,
		ExpiryDate:   req.ExpiryDate,
		Enabled:      req.Enabled,
		TrafficLimit: req.TrafficLimit,
		TrafficUsed:  0,
		DnsResolve:   req.DnsResolve,
		Notes:        req.Notes,
	}

	if err := database.DB.Create(&user).Error; err != nil {
		errorJSON(c, http.StatusInternalServerError, "创建失败")
		return
	}

	// 自动链接到所有匹配等级的节点
	s.linkUserToNodes(&user)

	// 广播配置更新到所有在线 Agent
	go agentHub.BroadcastConfigUpdate()

	successDataMsgJSON(c, "用户创建成功", user)
}

// linkUserToNodes 将用户链接到所有启用的节点（批量操作）
func (s *Server) linkUserToNodes(user *database.ProxyUser) {
	var nodes []database.InboundNode
	database.DB.Where("enabled = ?", true).Find(&nodes)
	if len(nodes) == 0 {
		return
	}

	// 1. 构建节点 ID 列表
	nodeIDs := make([]uint, len(nodes))
	for i, n := range nodes {
		nodeIDs[i] = n.ID
	}

	// 2. 单次查询已存在的关联
	var existing []database.NodeUserRelation
	database.DB.Where("user_id = ? AND node_id IN ?", user.ID, nodeIDs).Find(&existing)
	existingSet := make(map[uint]bool, len(existing))
	for _, r := range existing {
		existingSet[r.NodeID] = true
	}

	// 3. 批量插入新关联
	var newRelations []database.NodeUserRelation
	for _, node := range nodes {
		if existingSet[node.ID] {
			continue
		}
		newRelations = append(newRelations, database.NodeUserRelation{
			NodeID: node.ID,
			UserID: user.ID,
			UUID:   user.UUID,
			Flow:   node.Flow,
		})
	}
	if len(newRelations) > 0 {
		database.DB.Create(&newRelations)
	}
}

// handleUpdateUser 更新用户
func (s *Server) handleUpdateUser(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		errorJSON(c, http.StatusBadRequest, "无效的用户ID")
		return
	}

	var user database.ProxyUser
	if err := database.DB.First(&user, id).Error; err != nil {
		errorJSON(c, http.StatusNotFound, "用户不存在")
		return
	}

	var req UpdateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errorJSON(c, http.StatusBadRequest, "请求参数错误")
		return
	}

	// 如果有新名称，检查是否重复
	if req.Name != "" && req.Name != user.Name {
		var count int64
		database.DB.Model(&database.ProxyUser{}).Where("name = ? AND id != ?", req.Name, id).Count(&count)
		if count > 0 {
			errorJSON(c, http.StatusBadRequest, "用户名已存在")
			return
		}
		user.Name = req.Name
	}

	// 更新其他字段
	if req.Level > 0 {
		user.Level = req.Level
	}
	if req.ExpiryDate != "" {
		user.ExpiryDate = req.ExpiryDate
	}

	// 只在 enabled 变化时需要推送配置
	configChanged := req.Enabled != user.Enabled
	user.Enabled = req.Enabled
	user.TrafficLimit = req.TrafficLimit
	user.TrafficUsed = req.TrafficUsed
	if req.DnsResolve != "" {
		user.DnsResolve = req.DnsResolve
	}
	user.Notes = req.Notes

	if err := database.DB.Save(&user).Error; err != nil {
		errorJSON(c, http.StatusInternalServerError, "保存失败")
		return
	}

	// 同步更新 node_user_relations 中的 UUID
	database.DB.Model(&database.NodeUserRelation{}).
		Where("user_id = ?", user.ID).
		Update("uuid", user.UUID)

	// 只在 enabled 变化时广播配置更新
	if configChanged {
		go agentHub.BroadcastConfigUpdate()
	}

	successDataMsgJSON(c, "用户更新成功", user)
}

// handleDeleteUser 删除用户
func (s *Server) handleDeleteUser(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		errorJSON(c, http.StatusBadRequest, "无效的用户ID")
		return
	}

	// 先删除用户的节点关联
	database.DB.Unscoped().Where("user_id = ?", id).Delete(&database.NodeUserRelation{})

	result := database.DB.Unscoped().Delete(&database.ProxyUser{}, id)
	if result.Error != nil {
		errorJSON(c, http.StatusInternalServerError, "删除失败")
		return
	}

	if result.RowsAffected == 0 {
		errorJSON(c, http.StatusNotFound, "用户不存在")
		return
	}

	// 广播配置更新到所有在线 Agent
	go agentHub.BroadcastConfigUpdate()

	successMsgJSON(c, "用户删除成功")
}

// handleCheckExpiredUsers 检查并禁用到期用户
func (s *Server) handleCheckExpiredUsers(c *gin.Context) {
	today := time.Now().Format("2006-01-02")

	// 查找并禁用所有已到期的用户
	result := database.DB.Model(&database.ProxyUser{}).
		Where("enabled = ? AND expiry_date < ?", true, today).
		Update("enabled", false)

	if result.Error != nil {
		errorJSON(c, http.StatusInternalServerError, "检查失败: "+result.Error.Error())
		return
	}

	// 如果有用户被禁用，广播配置更新
	if result.RowsAffected > 0 {
		go agentHub.BroadcastConfigUpdate()
	}

	successDataMsgJSON(c, "检查完成", gin.H{
		"disabled_count": result.RowsAffected,
	})
}

// handleGetExpiringUsers 获取即将到期的用户
func (s *Server) handleGetExpiringUsers(c *gin.Context) {
	// 获取查询参数，默认7天内到期
	daysStr := c.DefaultQuery("days", "7")
	days, err := strconv.Atoi(daysStr)
	if err != nil || days < 0 {
		days = 7
	}

	today := time.Now().Format("2006-01-02")
	futureDate := time.Now().AddDate(0, 0, days).Format("2006-01-02")

	var users []database.ProxyUser
	database.DB.Where("enabled = ? AND expiry_date >= ? AND expiry_date <= ?",
		true, today, futureDate).
		Order("expiry_date ASC").
		Find(&users)

	// 也获取已过期但仍启用的用户（异常情况）
	var expiredUsers []database.ProxyUser
	database.DB.Where("enabled = ? AND expiry_date < ?", true, today).Find(&expiredUsers)

	successJSON(c, gin.H{
		"expiring": users,
		"expired":  expiredUsers,
	})
}
