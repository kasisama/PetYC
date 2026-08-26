package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"runtime"

	"golang.org/x/term"
	"qq-pet-saas/admin"
	"qq-pet-saas/config"
	"qq-pet-saas/core"
	"qq-pet-saas/database"
	_ "qq-pet-saas/features"
	"qq-pet-saas/models"
	"qq-pet-saas/notifications"
	"qq-pet-saas/qqofficial"
	"qq-pet-saas/security"
	"qq-pet-saas/setupwizard"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "--version" {
		fmt.Println(version)
		return
	}
	runtimeConfig, err := security.LoadRuntimeConfig()
	if err != nil {
		log.Fatalf("[Main] 初始化平台运行配置失败: %v", err)
	}
	credentials, err := security.LoadCredentials()
	if err != nil {
		log.Fatalf("[Main] 初始化安全凭据失败: %v", err)
	}
	onboarding, err := security.LoadOnboardingState()
	if err != nil {
		log.Fatalf("[Main] 读取首次配置状态失败: %v", err)
	}
	if runtime.GOOS == "linux" && !onboarding.SetupCompleted {
		runtimeConfig, err = setupwizard.RunLinuxTerminal(runtimeConfig, credentials)
		if err != nil {
			log.Printf("[首次配置] %v，请在交互式终端中重新运行程序", err)
			os.Exit(2)
		}
		onboarding.SetupCompleted = true
	}
	log.Print("[启动] 运行配置与安全凭据已加载")

	if err := config.ExtractOfficialAssets("./图片"); err != nil {
		log.Printf("[Main] 自动释放官方图片资源失败: %v", err)
	}
	database.InitDB()
	if err := config.EnsureOfficialDefaults(database.DB); err != nil {
		log.Fatalf("[Main] 初始化官方 SQL 配置失败: %v", err)
	}
	if err := config.LoadAllConfigsFromDB(database.DB); err != nil {
		log.Fatalf("[Main] 从 SQLite 加载配置失败: %v", err)
	}
	if err := config.EnsureModernMenus(database.DB); err != nil {
		log.Fatalf("[Main] 更新默认菜单失败: %v", err)
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
	worker := notifications.NewWorker(database.DB, deliverNotification)
	go worker.Run(context.Background())

	var onReady func(string)
	if runtime.GOOS == "windows" && !onboarding.SetupCompleted {
		onReady = func(address string) {
			url := address + "/admin/login?first-run=1"
			if err := exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start(); err != nil {
				log.Printf("[首次配置] 无法自动打开浏览器，请访问 %s", url)
			}
		}
	}
	interactive := runtime.GOOS == "windows" && term.IsTerminal(int(os.Stdin.Fd()))
	if err := runServerWithInteractiveRetry(
		&runtimeConfig,
		interactive,
		os.Stdin,
		os.Stdout,
		onReady,
		core.StartAppWithReady,
		findNextAvailablePort,
	); err != nil {
		if errors.Is(err, errStartupCancelled) {
			log.Print("[启动] 用户已取消启动")
			os.Exit(2)
		}
		log.Printf("[App] %v", err)
		os.Exit(1)
	}
}

var version = "dev"

func deliverNotification(ctx context.Context, job models.NotificationJob) error {
	event := core.InboundEvent{
		Platform: core.Platform(job.Platform), SceneType: core.SceneType(job.SceneType), AppID: job.AppID,
		SpaceID: job.SpaceID, RoomID: job.RoomID, ActorID: job.ActorID, ActorName: job.ActorName,
	}
	message := core.OutboundMessage{MessageKey: job.MessageKey, Text: job.Message, Markdown: &core.MarkdownPayload{Content: job.Message}}
	switch event.Platform {
	case core.PlatformOneBot:
		return core.BroadcastNotification(event, message)
	case core.PlatformQQGroup, core.PlatformQQGuild:
		return qqofficial.SendDefault(ctx, event, message)
	default:
		return fmt.Errorf("不支持的通知平台: %s", event.Platform)
	}
}
