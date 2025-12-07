package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// JSON 响应辅助函数

// successJSON 返回成功响应
func successJSON(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    data,
	})
}

// successMsgJSON 返回成功消息响应
func successMsgJSON(c *gin.Context, message string) {
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": message,
	})
}

// successDataMsgJSON 返回成功数据和消息响应
func successDataMsgJSON(c *gin.Context, message string, data interface{}) {
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": message,
		"data":    data,
	})
}

// errorJSON 返回错误响应
func errorJSON(c *gin.Context, code int, message string) {
	c.JSON(code, gin.H{
		"success": false,
		"error":   message,
	})
}
