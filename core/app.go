package core

import (
	"fmt"
	"log"

	"github.com/gin-gonic/gin"
	"qq-pet-saas/admin"
	"qq-pet-saas/config"
	"qq-pet-saas/database"
	"qq-pet-saas/security"
)

func NewAppRouter() *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.Use(gin.Recovery())
	router.Static("/images", "./图片")

	admin.BroadcastMessageFunc = BroadcastGroupMessage
	admin.OneBotStatusFunc = OneBotStatusSnapshot
	admin.ReloadCommandRoutesFunc = func() error { return RebuildUnifiedRouter(database.DB) }
	admin.PrepareModernContentFunc = func() error {
		if err := config.EnsureModernMenus(database.DB); err != nil {
			return err
		}
		if err := SyncUnifiedCommandConfigs(database.DB); err != nil {
			return err
		}
		return config.LoadAllConfigsFromDB(database.DB)
	}
	admin.RegisterRoutes(router)
	router.GET("/v1/ws", HandleWebSocket)
	return router
}

// StartApp 启动服务框架并监听指定端口。
func StartApp(listenAddress string, port int) {
	manager := NewServerManagerWithEndpoint(NewAppRouter(), persistRuntimeEndpoint)
	admin.PlatformPortChangeFunc = func(port int) (admin.PortHandoffResult, error) {
		handoff, err := manager.BeginPortHandoff(port)
		if err != nil {
			return admin.PortHandoffResult{}, err
		}
		return admin.PortHandoffResult{
			Address: handoff.Address, ConfirmationToken: handoff.ConfirmationToken, ExpiresAt: handoff.ExpiresAt,
		}, nil
	}
	admin.PlatformEndpointChangeFunc = func(address string, port int) (admin.PortHandoffResult, error) {
		handoff, err := manager.BeginEndpointHandoff(address, port)
		if err != nil {
			return admin.PortHandoffResult{}, err
		}
		return admin.PortHandoffResult{
			Address: handoff.Address, ConfirmationToken: handoff.ConfirmationToken, ExpiresAt: handoff.ExpiresAt,
		}, nil
	}
	admin.PlatformPortConfirmFunc = manager.ConfirmPortHandoff

	if err := manager.StartEndpoint(listenAddress, port); err != nil {
		log.Fatalf("[App] 启动服务器失败: %v", err)
	}
	log.Print(startupSummary(manager.Address()))
	if err := manager.Wait(); err != nil {
		log.Fatalf("[App] 服务器运行失败: %v", err)
	}
}

func startupSummary(address string) string {
	return fmt.Sprintf("[启动] QQ-Pet SaaS 已就绪\n  管理后台：http://%s/admin\n  OneBot：  ws://%s/v1/ws", address, address)
}

func persistRuntimeEndpoint(address string, port int) error {
	config, err := security.LoadRuntimeConfig()
	if err != nil {
		return err
	}
	config.ListenAddress = address
	config.Port = port
	if err := security.SaveRuntimeConfig(config); err != nil {
		return fmt.Errorf("save runtime port: %w", err)
	}
	return nil
}
