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
	var request struct {
		Reason       string `json:"reason"`
		Confirmation string `json:"confirmation"`
	}
	if err := c.ShouldBindJSON(&request); err != nil {
		Error(c, codeInvalidPayload, "请填写操作原因并输入确认词")
		return
	}
	if _, err := requiredReason(request.Reason); err != nil {
		Error(c, codeInvalidPayload, err.Error())
		return
	}
	if strings.TrimSpace(request.Confirmation) != "安装更新" {
		Error(c, codeInvalidPayload, "请输入「安装更新」以确认操作")
		return
	}
	if err := UpdateService.StartInstall(); err != nil {
		Error(c, 4090, err.Error())
		return
	}
	Success(c, UpdateService.Status())
}
