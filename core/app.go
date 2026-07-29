package core

import (
	"log"

	"github.com/gin-gonic/gin"
	"qq-pet-saas/admin"
)

// StartApp 启动服务框架并监听指定的地址与端口
func StartApp(addr string) {
	// 设置为生产模式以提高运行性能
	gin.SetMode(gin.ReleaseMode)

	r := gin.New()

	// 全局安全崩溃恢复中间件，防止单个协程崩溃导致主进程退出
	r.Use(gin.Recovery())

	// 静态图片服务，支持远程 SaaS OneBot 客户端通过 HTTP 拉取图片
	r.Static("/images", "./图片")

	// 绑定群发广播回调以解耦核心与后台，避免循环引用
	admin.BroadcastMessageFunc = BroadcastGroupMessage

	// 注册管理后台静态资源和 API 路由
	admin.RegisterRoutes(r)

	// WebSocket 接入路由
	r.GET("/v1/ws", HandleWebSocket)

	log.Printf("[App] Go QQ-Pet SaaS 平台已安全启动，正在监听 %s 端口...", addr)
	if err := r.Run(addr); err != nil {
		log.Fatalf("[App] 启动服务器失败: %v", err)
	}
}
