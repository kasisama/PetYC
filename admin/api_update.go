package admin

import (
	"strings"

	"github.com/gin-gonic/gin"
	"qq-pet-saas/updater"
)

var UpdateService *updater.Service

func RegisterUpdateRoutes(group *gin.RouterGroup) {
	group.GET("/updates/check", checkForUpdates)
	group.GET("/updates/status", updateStatus)
	group.POST("/updates/install", installUpdate)
}

func checkForUpdates(c *gin.Context) {
	if UpdateService == nil {
		Error(c, codeInternalError, "更新服务尚未初始化")
		return
	}
	force := strings.EqualFold(c.Query("force"), "true") || c.Query("force") == "1"
	result, err := UpdateService.Check(c.Request.Context(), force)
	if err != nil {
		Error(c, codeInternalError, "检查更新失败: "+err.Error())
		return
	}
	Success(c, result)
}

func updateStatus(c *gin.Context) {
	if UpdateService == nil {
		Error(c, codeInternalError, "更新服务尚未初始化")
		return
	}
	Success(c, UpdateService.Status())
}

func installUpdate(c *gin.Context) {
	if UpdateService == nil {
		Error(c, codeInternalError, "更新服务尚未初始化")
		return
	}
	if err := UpdateService.StartInstall(); err != nil {
		Error(c, 4090, err.Error())
		return
	}
	Success(c, UpdateService.Status())
}
