package handler

import (
	"net/http"
	"os"
	"path/filepath"
	"runtime"

	"github.com/gin-gonic/gin"
)

// handleDownloadAgent 提供 Agent 二进制下载
func (s *Server) handleDownloadAgent(c *gin.Context) {
	arch := c.Param("arch")

	// 验证架构
	validArch := map[string]bool{
		"amd64": true,
		"arm64": true,
		"armv7": true,
	}
	if !validArch[arch] {
		errorJSON(c, http.StatusBadRequest, "不支持的架构: "+arch)
		return
	}

	// 查找 Agent 二进制文件
	// 优先从当前目录查找，然后从可执行文件同级目录查找
	var agentPath string
	searchPaths := []string{
		"sboard-agent",             // 当前目录
		"agent",                    // 当前目录的 agent
		"./sboard-agent",           // 当前目录
		"./agent",                  // 当前目录
		"/opt/sboard/sboard-agent", // 安装目录
		"/opt/sboard/agent",        // 安装目录
	}

	// 添加可执行文件同级目录
	if execPath, err := os.Executable(); err == nil {
		execDir := filepath.Dir(execPath)
		searchPaths = append(searchPaths,
			filepath.Join(execDir, "sboard-agent"),
			filepath.Join(execDir, "agent"),
		)
	}

	for _, p := range searchPaths {
		if _, err := os.Stat(p); err == nil {
			agentPath = p
			break
		}
	}

	if agentPath == "" {
		errorJSON(c, http.StatusNotFound, "Agent 二进制文件未找到 — 请确保 sboard-agent 或 agent 文件与 sboard 在同一目录")
		return
	}

	// 检查架构匹配（当前只支持同架构下载）
	currentArch := runtime.GOARCH
	if currentArch == "amd64" && arch != "amd64" ||
		currentArch == "arm64" && arch != "arm64" {
		errorJSON(c, http.StatusNotFound, "当前服务器架构为 "+currentArch+"，无法提供 "+arch+" 架构的 Agent，需要交叉编译")
		return
	}

	// 读取文件
	data, err := os.ReadFile(agentPath)
	if err != nil {
		errorJSON(c, http.StatusInternalServerError, "读取 Agent 文件失败: "+err.Error())
		return
	}

	// 返回文件
	c.Header("Content-Type", "application/octet-stream")
	c.Header("Content-Disposition", "attachment; filename=sboard-agent")
	c.Data(http.StatusOK, "application/octet-stream", data)
}
