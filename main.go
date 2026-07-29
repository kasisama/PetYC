package main

import (
	"embed"
	"log"
	"os"
	"strings"

	"qq-pet-saas/config"
	"qq-pet-saas/core"
	"qq-pet-saas/database"
	_ "qq-pet-saas/features" // 隐式导入 features 聚合包，触发所有玩法的 init() 自路由注册
	"qq-pet-saas/security"
)

//go:embed 初始数据/*
var SeedFS embed.FS

func main() {
	if _, err := security.LoadCredentials(); err != nil {
		log.Fatalf("[Main] 初始化安全凭据失败: %v", err)
	}
	log.Printf("[Main] 已加载管理员账号与 WebSocket 凭据；WebSocket 令牌可通过 QQPET_WS_TOKEN 覆盖（凭据文件：%s）", security.CredentialsPath())

	// 1. 如果本地无“图片”目录，从嵌入资源中自动释放游戏素材图片
	if err := config.ExtractImages(SeedFS); err != nil {
		log.Printf("[Main] 自动释放嵌入图片资源失败: %v", err)
	}

	// 2. 初始化 SQLite 数据库并执行表结构迁移
	database.InitDB()

	// 3. 将本地配置文件数据加载/同步到 SQLite 数据库中 (空库时读取嵌入的 SeedFS 进行种子填充)
	if err := config.SyncWithDB(database.DB, SeedFS); err != nil {
		log.Fatalf("[Main] 初始化配置失败: %v", err)
	}

	// 4. 启动核心 HTTP 与 WebSocket 监听服务 (支持环境变量 PORT，默认监听 :8080)
	port := os.Getenv("PORT")
	if port == "" {
		port = ":8080"
	} else if !strings.HasPrefix(port, ":") {
		port = ":" + port
	}

	core.StartApp(port)
}
