package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/sboard-go/sboard/internal/database"
)

// ========== 外部节点 CRUD API ==========

// handleListExternalNodes 获取外部节点列表
func (s *Server) handleListExternalNodes(c *gin.Context) {
	var nodes []database.ExternalNode
	if err := database.DB.Order("sort_order ASC, id ASC").Find(&nodes).Error; err != nil {
		errorJSON(c, http.StatusInternalServerError, "获取外部节点列表失败")
		return
	}
	successJSON(c, nodes)
}

// handleGetExternalNode 获取单个外部节点
func (s *Server) handleGetExternalNode(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		errorJSON(c, http.StatusBadRequest, "无效的节点ID")
		return
	}

	var node database.ExternalNode
	if err := database.DB.First(&node, id).Error; err != nil {
		errorJSON(c, http.StatusNotFound, "节点不存在")
		return
	}
	successJSON(c, node)
}

// handleCreateExternalNode 创建外部节点
func (s *Server) handleCreateExternalNode(c *gin.Context) {
	var node database.ExternalNode
	if err := c.ShouldBindJSON(&node); err != nil {
		errorJSON(c, http.StatusBadRequest, "请求参数错误")
		return
	}

	if node.Name == "" || node.Protocol == "" || node.Host == "" || node.Port == 0 {
		errorJSON(c, http.StatusBadRequest, "名称、协议、地址、端口不能为空")
		return
	}

	if err := database.DB.Create(&node).Error; err != nil {
		errorJSON(c, http.StatusInternalServerError, "创建节点失败")
		return
	}
	successJSON(c, node)
}

// handleUpdateExternalNode 更新外部节点
func (s *Server) handleUpdateExternalNode(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		errorJSON(c, http.StatusBadRequest, "无效的节点ID")
		return
	}

	var existing database.ExternalNode
	if err := database.DB.First(&existing, id).Error; err != nil {
		errorJSON(c, http.StatusNotFound, "节点不存在")
		return
	}

	var updates database.ExternalNode
	if err := c.ShouldBindJSON(&updates); err != nil {
		errorJSON(c, http.StatusBadRequest, "请求参数错误")
		return
	}

	if err := database.DB.Model(&existing).Updates(updates).Error; err != nil {
		errorJSON(c, http.StatusInternalServerError, "更新节点失败")
		return
	}
	successJSON(c, existing)
}

// handleDeleteExternalNode 删除外部节点
func (s *Server) handleDeleteExternalNode(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		errorJSON(c, http.StatusBadRequest, "无效的节点ID")
		return
	}

	if err := database.DB.Delete(&database.ExternalNode{}, id).Error; err != nil {
		errorJSON(c, http.StatusInternalServerError, "删除节点失败")
		return
	}
	successMsgJSON(c, "删除成功")
}
