package main

import (
	"context"
	"embed"
	"log"

	"qq-pet-saas/admin"
	"qq-pet-saas/config"
	"qq-pet-saas/core"
	"qq-pet-saas/database"
	_ "qq-pet-saas/features"
	"qq-pet-saas/qqofficial"
	"qq-pet-saas/security"
)

//go:embed 初始数据/*
var SeedFS embed.FS

func main() {
	runtimeConfig, err := security.LoadRuntimeConfig()
	if err != nil {
		log.Fatalf("[Main] 初始化平台运行配置失败: %v", err)
	}
	if _, err := security.LoadCredentials(); err != nil {
		log.Fatalf("[Main] 初始化安全凭据失败: %v", err)
	}
	log.Print("[启动] 运行配置与安全凭据已加载")

	if err := config.ExtractImages(SeedFS); err != nil {
		log.Printf("[Main] 自动释放嵌入图片资源失败: %v", err)
	}
	database.InitDB()
	if err := config.SyncWithDB(database.DB, SeedFS); err != nil {
		log.Fatalf("[Main] 初始化配置失败: %v", err)
	}
	if err := config.EnsureModernMenus(database.DB); err != nil {
		log.Fatalf("[Main] 更新新版菜单失败: %v", err)
	}
	if err := core.SyncUnifiedCommandConfigs(database.DB); err != nil {
		log.Fatalf("[Main] 更新命令目录失败: %v", err)
	}
	if err := config.MarkConfigLoaded(database.DB); err != nil {
		log.Fatalf("[Main] 初始化配置版本状态失败: %v", err)
	}

	if _, enabled, err := qqofficial.StartFromEnv(context.Background()); err != nil {
		log.Fatalf("[Main] 启动 QQ 官方机器人适配器失败: %v", err)
	} else if enabled {
		log.Printf("[启动] 接入方式：OneBot + QQ 官方机器人")
	} else {
		log.Printf("[启动] 接入方式：OneBot（QQ 官方机器人未启用）")
	}
	admin.QQOfficialStatusFunc = func() interface{} { return qqofficial.DefaultRuntimeSnapshot() }
	admin.QQOfficialReconnectFunc = qqofficial.ReconnectDefault
	admin.QQOfficialApplyConfigFunc = qqofficial.ApplyDefaultConfig

	core.StartApp(runtimeConfig.ListenAddress, runtimeConfig.Port)
}
