package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"syscall"
	"time"

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
	"qq-pet-saas/updater"
)

func main() {
	if handled, err := updater.MaybeRunHelper(os.Args[1:]); handled {
		if err != nil {
			log.Printf("[更新] %v", err)
			os.Exit(1)
		}
		return
	}
	if len(os.Args) > 1 && os.Args[1] == "--version" {
		fmt.Println(version)
		return
	}
	rootCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	core.BuildVersion = version
	updateService := updater.NewService(updater.Config{CurrentVersion: version})
	admin.UpdateService = updateService
	if executable, err := os.Executable(); err == nil {
		updater.CleanupStaleHelpers(executable)
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
	if shouldRunLinuxTerminalSetup(runtime.GOOS, onboarding.SetupCompleted) {
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
	if sqlDB, dbErr := database.DB.DB(); dbErr == nil {
		defer sqlDB.Close()
	}
	if err := config.EnsureOfficialDefaults(database.DB); err != nil {
		log.Fatalf("[Main] 初始化官方 SQL 配置失败: %v", err)
	}
	if err := config.LoadAllConfigsFromDB(database.DB); err != nil {
		log.Fatalf("[Main] 从 SQLite 加载配置失败: %v", err)
	}
	interactiveMenu := term.IsTerminal(int(os.Stdin.Fd()))
	if err := config.ApplyModernMenus(database.DB, config.PromptMenuOverwrite(os.Stdin, os.Stdout, interactiveMenu)); err != nil {
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
	admin.QQOfficialSyncDiscoveryFunc = func(ctx context.Context) (interface{}, error) {
		rows := make([]models.CommandConfig, 0)
		if err := database.DB.WithContext(ctx).Where("enabled = ?", true).Order("sort_order asc").Find(&rows).Error; err != nil {
			return nil, err
		}
		commands := make([]qqofficial.DiscoveryCommand, 0, len(rows))
		for _, row := range rows {
			commands = append(commands, qqofficial.DiscoveryCommand{
				Key: row.FuncName, Command: row.Command, DisplayName: row.DisplayName,
				Description: row.Description, SortOrder: row.SortOrder,
			})
		}
		return qqofficial.SyncDefaultDiscovery(ctx, commands)
	}
	worker := notifications.NewWorker(database.DB, deliverNotification)
	workerDone := make(chan struct{})
	go func() {
		defer close(workerDone)
		worker.Run(rootCtx)
	}()

	onReady := func(address string) {
		updateService.SetRuntime(address+"/healthz", stop)
	}
	if runtime.GOOS == "windows" && !onboarding.SetupCompleted {
		initializeUpdater := onReady
		onReady = func(address string) {
			initializeUpdater(address)
			url := address + "/admin/login?first-run=1"
			if err := exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start(); err != nil {
				log.Printf("[首次配置] 无法自动打开浏览器，请访问 %s", url)
			}
		}
	}
	interactive := runtime.GOOS == "windows" && term.IsTerminal(int(os.Stdin.Fd()))
	serverErr := runServerWithInteractiveRetry(
		&runtimeConfig,
		interactive,
		os.Stdin,
		os.Stdout,
		onReady,
		func(address string, port int, callback func(string) error) error {
			return core.StartAppWithReadyContext(rootCtx, address, port, callback)
		},
		findNextAvailablePort,
	)
	stop()
	select {
	case <-workerDone:
	case <-time.After(5 * time.Second):
		log.Print("[停止] 通知任务未在 5 秒内退出，将继续关闭进程")
	}
	if serverErr != nil {
		if errors.Is(serverErr, errStartupCancelled) {
			log.Print("[启动] 用户已取消启动")
			os.Exit(2)
		}
		log.Printf("[App] %v", serverErr)
		os.Exit(1)
	}
}

func shouldRunLinuxTerminalSetup(goos string, setupCompleted bool) bool {
	return goos == "linux" && !setupCompleted && !security.WebSetupEnabled()
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
