package core

import (
	"context"
	"fmt"
	"log"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"
	"qq-pet-saas/admin"
	"qq-pet-saas/config"
	"qq-pet-saas/database"
	"qq-pet-saas/gameplayrules"
	"qq-pet-saas/security"
)

var BuildVersion = "dev"

func NewAppRouter() *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.Use(gin.Recovery())
	router.Static("/images", "./图片")

	admin.OneBotStatusFunc = OneBotStatusSnapshot
	admin.RebuildCommandRoutesFunc = func() error { return RebuildUnifiedRouter(database.DB) }
	admin.PrepareConfigDefaultsFunc = func() error {
		if err := config.EnsureModernMenus(database.DB); err != nil {
			return err
		}
		if err := gameplayrules.EnsureDefaults(database.DB); err != nil {
			return err
		}
		return SyncUnifiedCommandConfigs(database.DB)
	}
	admin.RegisterRoutes(router)
	router.GET("/healthz", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok", "version": BuildVersion})
	})
	router.GET("/v1/ws", HandleWebSocket)
	return router
}

// StartApp 启动服务框架并监听指定端口。
func StartApp(listenAddress string, port int) error {
	return StartAppWithReady(listenAddress, port, nil)
}

func StartAppWithReady(listenAddress string, port int, onReady func(string) error) error {
	return StartAppWithReadyContext(context.Background(), listenAddress, port, onReady)
}

func StartAppWithReadyContext(ctx context.Context, listenAddress string, port int, onReady func(string) error) error {
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
		return fmt.Errorf("启动服务器失败: %w", err)
	}
	defer manager.Close()
	log.Print(startupSummary(manager.Address()))
	if onReady != nil {
		if err := onReady(manager.Address()); err != nil {
			return fmt.Errorf("完成服务器启动: %w", err)
		}
	}
	if err := manager.WaitContext(ctx); err != nil {
		return fmt.Errorf("服务器运行失败: %w", err)
	}
	return nil
}

func startupSummary(address string) string {
	httpAddress := strings.TrimRight(strings.TrimSpace(address), "/")
	if !strings.Contains(httpAddress, "://") {
		httpAddress = "http://" + httpAddress
	}
	websocketAddress := httpAddress
	if parsed, err := url.Parse(httpAddress); err == nil && parsed.Host != "" {
		switch parsed.Scheme {
		case "https":
			parsed.Scheme = "wss"
		default:
			parsed.Scheme = "ws"
		}
		websocketAddress = parsed.String()
	}
	return fmt.Sprintf("[启动] QQ-Pet 已就绪\n  管理后台：%s/admin\n  OneBot：  %s/v1/ws", httpAddress, websocketAddress)
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
