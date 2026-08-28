package expedition

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"gorm.io/gorm"

	"qq-pet-saas/core"
	"qq-pet-saas/gameplay"
	"qq-pet-saas/models"
)

func upsertTestSpecies(t *testing.T, db *gorm.DB, species models.PetSpeciesConfig) {
	t.Helper()
	if species.Key == "" {
		species.Key = species.Name
	}
	if species.FamilyKey == "" {
		species.FamilyKey = species.Key
	}
	species.Adoptable = true
	if err := db.Where("name = ?", species.Name).Assign(species).FirstOrCreate(&models.PetSpeciesConfig{}).Error; err != nil {
		t.Fatal(err)
	}
}

func TestPlayerHandlersUseTheUnifiedReplyConstructor(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		content, readErr := os.ReadFile(name)
		if readErr != nil {
			t.Fatal(readErr)
		}
		count := strings.Count(string(content), "core.OutboundMessage{Text:")
		if name == "commands.go" {
			count-- // text() is the single audited constructor.
		}
		if count > 0 {
			t.Fatalf("%s bypasses the unified reply constructor", name)
		}
	}
}

func TestMenuUsesConfiguredSceneReplyAndImage(t *testing.T) {
	service, _, _ := newTestService(t)
	if err := service.DB.AutoMigrate(&models.MenuConfig{}); err != nil {
		t.Fatal(err)
	}
	if err := service.DB.Save(&models.MenuConfig{
		Name: "主菜单", Reply: "欢迎来到图文菜单", Markdown: "# 欢迎来到图文菜单", Image: "https://cdn.example.com/menu.webp",
	}).Error; err != nil {
		t.Fatal(err)
	}

	message, err := handleMenu(context.Background(), oneBotEvent("100", "menu-image", "宠物菜单"), service)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(message.Text, "欢迎来到图文菜单") || !strings.Contains(message.Text, "今日待办") || message.Image != "https://cdn.example.com/menu.webp" {
		t.Fatalf("菜单场景未返回配置图文: %#v", message)
	}
	if message.Markdown == nil || !strings.Contains(message.Markdown.Content, "# 欢迎来到图文菜单") || !strings.Contains(message.Markdown.Content, "今日待办") {
		t.Fatalf("菜单场景未返回独立 Markdown: %#v", message.Markdown)
	}
}

func TestMenuEmptyOrInvalidImageStillSendsText(t *testing.T) {
	service, _, _ := newTestService(t)
	if err := service.DB.AutoMigrate(&models.MenuConfig{}); err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name  string
		image string
	}{
		{name: "empty", image: ""},
		{name: "missing-file", image: "上传/not-exist.webp"},
		{name: "path-escape", image: "../secret.png"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := service.DB.Save(&models.MenuConfig{
				Name: "主菜单", Reply: "纯文字菜单", Image: tc.image,
			}).Error; err != nil {
				t.Fatal(err)
			}
			message, err := handleMenu(context.Background(), oneBotEvent("100", "menu-fallback-"+tc.name, "主菜单"), service)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(message.Text, "纯文字菜单") || !strings.Contains(message.Text, "今日待办") || message.Image != "" {
				t.Fatalf("无效配图应降级为纯文字: %#v", message)
			}
			if message.Markdown != nil {
				t.Fatalf("空 Markdown 不应复制纯文本: %#v", message.Markdown)
			}
		})
	}
}

func TestAdoptListFollowsConfiguredStarterPets(t *testing.T) {
	service, db, _ := newTestService(t)
	if err := db.Save(&models.SystemConfig{Key: "Core.InitialPets", Value: "团子,苔须灵"}).Error; err != nil {
		t.Fatal(err)
	}
	upsertTestSpecies(t, db, models.PetSpeciesConfig{Name: "团子", Description: "这是一个可爱的小兔子。"})
	upsertTestSpecies(t, db, models.PetSpeciesConfig{Name: "苔须灵", Description: "林地向导"})
	router := core.NewCommandRouter()
	if err := RegisterCommands(router.Register, func() *Service { return service }); err != nil {
		t.Fatal(err)
	}
	message, handled, err := router.Route(context.Background(), oneBotEvent("100", "adopt-cfg", "领养宠物"))
	if err != nil || !handled {
		t.Fatalf("领养列表失败: handled=%v err=%v", handled, err)
	}
	if !strings.Contains(message.Text, "1. 团子｜这是一个可爱的小兔子") || !strings.Contains(message.Text, "2. 苔须灵") || strings.Contains(message.Text, "烬爪兽") {
		t.Fatalf("领养列表未使用配置: %q", message.Text)
	}
	if !strings.Contains(message.Text, "领养 团子") {
		t.Fatalf("领养提示应使用配置中的第一只宠物: %q", message.Text)
	}

	rejected, _, err := router.Route(context.Background(), oneBotEvent("100", "adopt-cfg", "领养 烬爪兽"))
	if err != nil || !strings.Contains(rejected.Text, "没有找到这位伙伴") {
		t.Fatalf("未配置的宠物应被拒绝: err=%v message=%q", err, rejected.Text)
	}
	accepted, _, err := router.Route(context.Background(), oneBotEvent("100", "adopt-cfg", "领养 团子"))
	if err != nil || !strings.Contains(accepted.Text, "领养成功") {
		t.Fatalf("配置中的宠物应可领养: err=%v message=%q", err, accepted.Text)
	}
}

func TestTextCommandsSupportAdoptAndExpeditionFlow(t *testing.T) {
	service, _, _ := newTestService(t)
	router := core.NewCommandRouter()
	if err := RegisterCommands(router.Register, func() *Service { return service }); err != nil {
		t.Fatal(err)
	}
	event := oneBotEvent("100", "42", "状态")
	message, handled, err := router.Route(context.Background(), event)
	if err != nil || !handled || !strings.Contains(message.Text, "领养") {
		t.Fatalf("unexpected initial status: handled=%v err=%v message=%q", handled, err, message.Text)
	}
	event.Text = "领养 光芽兽"
	message, _, err = router.Route(context.Background(), event)
	if err != nil || !strings.Contains(message.Text, "领养成功") {
		t.Fatalf("unexpected adopt response: err=%v message=%q", err, message.Text)
	}
	event.Text = "远征 2"
	message, _, err = router.Route(context.Background(), event)
	if err != nil || !strings.Contains(message.Text, "遗迹调查已开始") || !strings.Contains(message.Text, "远征状态") {
		t.Fatalf("unexpected expedition response: err=%v message=%q", err, message.Text)
	}
}

func TestExpeditionMenuProvidesOptionalButtonsWithTextFallback(t *testing.T) {
	service, _, _ := newTestService(t)
	router := core.NewCommandRouter()
	if err := RegisterCommands(router.Register, func() *Service { return service }); err != nil {
		t.Fatal(err)
	}
	event := oneBotEvent("100", "42", "远征")
	message, handled, err := router.Route(context.Background(), event)
	if err != nil || !handled {
		t.Fatalf("menu route failed: handled=%v err=%v", handled, err)
	}
	if !strings.Contains(message.Text, "林间巡查") || !strings.Contains(message.Text, "远征 1") || message.Keyboard == nil || len(message.Keyboard.Rows) != 1 || len(message.Keyboard.Rows[0]) != 3 {
		t.Fatalf("expected text fallback and three optional buttons: %#v", message)
	}
}

func TestFamiliarPetCommandsRemainPrimaryAndDiscoverable(t *testing.T) {
	service, _, _ := newTestService(t)
	router := core.NewCommandRouter()
	if err := RegisterCommands(router.Register, func() *Service { return service }); err != nil {
		t.Fatal(err)
	}

	for _, command := range []string{"菜单", "宠物菜单", "帮助"} {
		message, handled, err := router.Route(context.Background(), oneBotEvent("100", "42", command))
		if err != nil || !handled {
			t.Fatalf("熟悉入口 %q 未生效: handled=%v err=%v", command, handled, err)
		}
		for _, expected := range []string{"领养宠物", "我的宠物", "签到", "我的背包", "远征"} {
			if !strings.Contains(message.Text, expected) {
				t.Fatalf("%q 菜单缺少 %q: %q", command, expected, message.Text)
			}
		}
		for _, decoration := range []string{"🐾", "🌟", "🍖", "🛍️", "🧭", "🏕️", "🔐", "💡"} {
			if !strings.Contains(message.Text, decoration) {
				t.Fatalf("%q 菜单缺少分区装饰 %q: %q", command, decoration, message.Text)
			}
		}
	}

	for _, command := range []string{"签到", "宠物签到", "今日", "我的宠物", "宠物状态", "状态"} {
		_, handled, err := router.Route(context.Background(), oneBotEvent("100", "42", command))
		if err != nil || !handled {
			t.Fatalf("兼容命令 %q 未生效: handled=%v err=%v", command, handled, err)
		}
	}
	if _, _, err := router.Route(context.Background(), oneBotEvent("100", "42", "领养 光芽兽")); err != nil {
		t.Fatal(err)
	}
	message, handled, err := router.Route(context.Background(), oneBotEvent("100", "42", "宠物摸头"))
	if err != nil || !handled || !strings.Contains(message.Text, "【摸头完成】") {
		t.Fatalf("旧前缀互动命令未正确标准化: handled=%v err=%v text=%q", handled, err, message.Text)
	}

	catalog := CommandCatalog()
	byCommand := make(map[string]core.UnifiedFeature, len(catalog))
	for _, item := range catalog {
		byCommand[item.DefaultCommand] = item
	}
	for _, command := range []string{"宠物菜单", "领养宠物", "我的宠物", "签到", "我的背包"} {
		item, exists := byCommand[command]
		if !exists || item.Hidden {
			t.Fatalf("熟悉主命令 %q 必须存在且可见: %#v", command, item)
		}
	}
}

func TestFamiliarShopJourneyUsesUnifiedPetWalletAndInventory(t *testing.T) {
	service, db, _ := newTestService(t)
	if err := db.Save(&models.SystemConfig{Key: "Core.InitialCoin", Value: "100"}).Error; err != nil {
		t.Fatal(err)
	}
	upsertTestSpecies(t, db, models.PetSpeciesConfig{
		Name: "光芽兽", Hunger: 100, HungerMax: 100, FavoriteFood: "小饼干",
	})
	if err := db.Create(&models.ItemConfig{
		Name: "小饼干", Status: "active", Type: "饱食", Effect: "15",
		Image: "物品/小饼干.png", Description: "一块香香脆脆的小饼干。", SellPrice: 4,
	}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&models.ShopItemConfig{
		ShopType: "shop_normal", Name: "小饼干", Image: "物品/小饼干.png",
		Stock: 5, Price: 10, Description: "适合刚开始照顾宠物时准备。",
	}).Error; err != nil {
		t.Fatal(err)
	}
	router := core.NewCommandRouter()
	if err := RegisterCommands(router.Register, func() *Service { return service }); err != nil {
		t.Fatal(err)
	}
	event := oneBotEvent("100", "shop-player", "领养 光芽兽")
	if message, handled, err := router.Route(context.Background(), event); err != nil || !handled || !strings.Contains(message.Text, "领养成功") {
		t.Fatalf("adoption failed: handled=%v text=%q err=%v", handled, message.Text, err)
	}
	event.Text = "商店"
	if message, handled, err := router.Route(context.Background(), event); err != nil || !handled || !strings.Contains(message.Text, "小饼干") || !strings.Contains(message.Text, "10") {
		t.Fatalf("shop list failed: handled=%v text=%q err=%v", handled, message.Text, err)
	}
	event.Text = "查看商品 小饼干"
	message, handled, err := router.Route(context.Background(), event)
	if err != nil || !handled || message.Image != "物品/小饼干.png" || !strings.Contains(message.Text, "库存：5") {
		t.Fatalf("shop detail failed: handled=%v message=%#v err=%v", handled, message, err)
	}
	event.Text = "购买 小饼干*2"
	message, handled, err = router.Route(context.Background(), event)
	if err != nil || !handled || !strings.Contains(message.Text, "购买成功") || !strings.Contains(message.Text, "余额：80") {
		t.Fatalf("purchase failed: handled=%v text=%q err=%v", handled, message.Text, err)
	}
	var account models.PlayerIdentity
	if err = db.First(&account, "platform = ? AND subject_id = ?", string(core.PlatformOneBot), "shop-player").Error; err != nil {
		t.Fatal(err)
	}
	if err = db.Model(&models.PetProfile{}).Where("account_id = ?", account.AccountID).Update("hunger", 50).Error; err != nil {
		t.Fatal(err)
	}
	event.Text = "宠物喂养 小饼干*1"
	message, handled, err = router.Route(context.Background(), event)
	if err != nil || !handled || message.Image != "物品/小饼干.png" || !strings.Contains(message.Text, "饱食：50 → 80") || !strings.Contains(message.Text, "效果翻倍") {
		t.Fatalf("feeding did not consume the purchased food through unified inventory: handled=%v message=%#v err=%v", handled, message, err)
	}
	event.Text = "我的背包"
	message, handled, err = router.Route(context.Background(), event)
	if err != nil || !handled || !strings.Contains(message.Text, "小饼干 ×1") {
		t.Fatalf("global inventory did not see purchase: handled=%v text=%q err=%v", handled, message.Text, err)
	}
	event.Text = "查看物品 小饼干"
	message, handled, err = router.Route(context.Background(), event)
	if err != nil || !handled || message.Image != "物品/小饼干.png" || !strings.Contains(message.Text, "恢复饱食：15") {
		t.Fatalf("item detail failed: handled=%v message=%#v err=%v", handled, message, err)
	}
	event.Text = "出售 小饼干*1"
	message, handled, err = router.Route(context.Background(), event)
	if err != nil || !handled || !strings.Contains(message.Text, "出售成功") || !strings.Contains(message.Text, "余额：84") {
		t.Fatalf("sale failed: handled=%v text=%q err=%v", handled, message.Text, err)
	}
	if err = gameplay.NewWalletService(db).Credit(context.Background(), account.AccountID, gameplay.DefaultCurrencyKey, 36); err != nil {
		t.Fatal(err)
	}
	event.Text = "改名 星星"
	message, handled, err = router.Route(context.Background(), event)
	if err != nil || !handled || !strings.Contains(message.Text, "改名成功") {
		t.Fatalf("rename failed: handled=%v text=%q err=%v", handled, message.Text, err)
	}
	event.Text = "我的宠物"
	message, handled, err = router.Route(context.Background(), event)
	if err != nil || !handled || !strings.Contains(message.Text, "【星星的近况】") {
		t.Fatalf("renamed pet was not read from PetProfile: handled=%v text=%q err=%v", handled, message.Text, err)
	}
	event.Text = "放生"
	message, handled, err = router.Route(context.Background(), event)
	if err != nil || !handled || !strings.Contains(message.Text, "确认放生 星星") || strings.Contains(message.Text, "永久失去") {
		t.Fatalf("rest confirmation should be explicit and non-destructive: handled=%v text=%q err=%v", handled, message.Text, err)
	}
	event.Text = "确认放生 星星"
	message, handled, err = router.Route(context.Background(), event)
	if err != nil || !handled || !strings.Contains(message.Text, "开始休养") {
		t.Fatalf("rest confirmation failed: handled=%v text=%q err=%v", handled, message.Text, err)
	}
	event.Text = "我的宠物"
	message, handled, err = router.Route(context.Background(), event)
	if err != nil || !handled || !strings.Contains(message.Text, "正在安心休养") {
		t.Fatalf("rest status was not reflected by unified pet profile: handled=%v text=%q err=%v", handled, message.Text, err)
	}
	event.Text = "找回"
	message, handled, err = router.Route(context.Background(), event)
	if err != nil || !handled || !strings.Contains(message.Text, "重新同行") {
		t.Fatalf("recover from rest failed: handled=%v text=%q err=%v", handled, message.Text, err)
	}
}

func TestDailyCheckinShowsPlayerFacingCurrencyName(t *testing.T) {
	service, db, _ := newTestService(t)
	if err := db.Save(&models.SystemConfig{Key: "Core.CoinName", Value: "星砂"}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&models.CurrencyConfig{Key: gameplay.PrimaryCurrencyKey, Name: "星砂", Enabled: true}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&models.CheckinRewardConfig{Type: "checkin_newbie", Day: "1", Currency: 88, Affection: 8, Items: "调查便当*1"}).Error; err != nil {
		t.Fatal(err)
	}
	upsertTestSpecies(t, db, models.PetSpeciesConfig{Name: "光芽兽"})
	router := core.NewCommandRouter()
	if err := RegisterCommands(router.Register, func() *Service { return service }); err != nil {
		t.Fatal(err)
	}
	event := oneBotEvent("100", "checkin-player", "领养 光芽兽")
	if _, _, err := router.Route(context.Background(), event); err != nil {
		t.Fatal(err)
	}
	event.Text = "宠物签到"
	message, handled, err := router.Route(context.Background(), event)
	if err != nil || !handled {
		t.Fatalf("签到失败: handled=%v err=%v", handled, err)
	}
	if strings.Contains(message.Text, "primary_coin") {
		t.Fatalf("玩家回复不得出现内部货币键: %q", message.Text)
	}
	if !strings.Contains(message.Text, "星砂 +88") || !strings.Contains(message.Text, "好感 +8") || !strings.Contains(message.Text, "调查便当 ×1") {
		t.Fatalf("签到奖励展示不符合预期: %q", message.Text)
	}
}

func TestTimedGrowthCommandsUseActivityRunAndFamiliarCompletionCommand(t *testing.T) {
	service, db, now := newTestService(t)
	for key, value := range map[string]string{
		"Interaction.StudyLimit": "3", "Interaction.StudyHungerCost": "5", "Interaction.StudyGrowth": "2",
	} {
		if err := db.Save(&models.SystemConfig{Key: key, Value: value}).Error; err != nil {
			t.Fatal(err)
		}
	}
	upsertTestSpecies(t, db, models.PetSpeciesConfig{
		Name: "光芽兽", Wisdom: 10, WisdomMax: 100, Hunger: 100, HungerMax: 100,
		StudyStartImg: "宠物/学习开始.png", StudyEndImg: "宠物/学习完成.png",
	})
	if err := db.Create(&models.ItemConfig{Name: "专业书本", Status: "active", Type: "智慧", Effect: "4", Time: 1}).Error; err != nil {
		t.Fatal(err)
	}
	router := core.NewCommandRouter()
	if err := RegisterCommands(router.Register, func() *Service { return service }); err != nil {
		t.Fatal(err)
	}
	event := oneBotEvent("100", "activity-player", "领养 光芽兽")
	if _, _, err := router.Route(context.Background(), event); err != nil {
		t.Fatal(err)
	}
	account, err := service.ResolveAccount(context.Background(), event)
	if err != nil {
		t.Fatal(err)
	}
	if err = gameplay.NewInventoryService(db).Credit(context.Background(), account.ID, "专业书本", 1); err != nil {
		t.Fatal(err)
	}
	event.Text = "宠物学习 专业书本"
	message, handled, err := router.Route(context.Background(), event)
	if err != nil || !handled || message.Image != "宠物/学习开始.png" || !strings.Contains(message.Text, "【学习开始】") || !strings.Contains(message.Text, "饱食：100 → 95") {
		t.Fatalf("study start command mismatch: handled=%v message=%#v err=%v", handled, message, err)
	}
	*now = now.Add(2 * time.Minute)
	event.Text = "完成学习"
	message, handled, err = router.Route(context.Background(), event)
	if err != nil || !handled || message.Image != "宠物/学习完成.png" || !strings.Contains(message.Text, "【学习完成】") || !strings.Contains(message.Text, "智慧：10 → 15") {
		t.Fatalf("study completion command mismatch: handled=%v message=%#v err=%v", handled, message, err)
	}
	event.Text = "我的宠物"
	message, handled, err = router.Route(context.Background(), event)
	if err != nil || !handled || !strings.Contains(message.Text, "当前空闲") {
		t.Fatalf("completed activity did not restore pet to idle: handled=%v text=%q err=%v", handled, message.Text, err)
	}
}

func TestCommandCatalogUsesStableFeaturesAndUniqueTriggers(t *testing.T) {
	catalog := CommandCatalog()
	funcNames := make(map[string]struct{}, len(catalog))
	triggers := make(map[string]string)
	for _, item := range catalog {
		if item.FuncName == "" || item.DefaultCommand == "" {
			t.Fatalf("命令目录存在不完整条目: %#v", item)
		}
		if _, exists := funcNames[item.FuncName]; exists {
			t.Fatalf("稳定功能标识重复: %s", item.FuncName)
		}
		funcNames[item.FuncName] = struct{}{}
		for _, trigger := range append([]string{item.DefaultCommand}, item.Aliases...) {
			if owner, exists := triggers[trigger]; exists {
				t.Fatalf("触发词 %q 同时属于 %s 和 %s", trigger, owner, item.FuncName)
			}
			triggers[trigger] = item.FuncName
		}
	}
	for alias, owner := range map[string]string{
		"菜单": "pet_menu", "状态": "pet_status", "宠物状态": "pet_status",
		"今日": "daily_checkin", "宠物签到": "daily_checkin", "背包": "inventory",
	} {
		if triggers[alias] != owner {
			t.Fatalf("兼容命令 %q 应归属 %s，实际为 %s", alias, owner, triggers[alias])
		}
	}
}

func TestMyPetIncludesConfiguredSpeciesImage(t *testing.T) {
	service, db, _ := newTestService(t)
	upsertTestSpecies(t, db, models.PetSpeciesConfig{Name: "光芽兽", Image: "宠物图片\\光芽兽.png"})
	router := core.NewCommandRouter()
	if err := RegisterCommands(router.Register, func() *Service { return service }); err != nil {
		t.Fatal(err)
	}
	event := oneBotEvent("100", "42", "领养 光芽兽")
	if _, _, err := router.Route(context.Background(), event); err != nil {
		t.Fatal(err)
	}
	event.Text = "我的宠物"
	message, handled, err := router.Route(context.Background(), event)
	if err != nil || !handled {
		t.Fatalf("查看宠物失败: handled=%v err=%v", handled, err)
	}
	if message.Image != "宠物图片\\光芽兽.png" {
		t.Fatalf("未带出宠物图片: %#v", message)
	}
}

func TestDailyJournalDoesNotResetAndOnlyRewardsOncePerDay(t *testing.T) {
	service, db, now := newTestService(t)
	account, err := service.ResolveAccount(context.Background(), oneBotEvent("100", "42", "今日"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = service.Adopt(context.Background(), account.ID, "光芽兽", "光芽兽"); err != nil {
		t.Fatal(err)
	}
	first, rewarded, err := service.RecordDaily(context.Background(), account.ID, "陪伴")
	if err != nil || !rewarded || first != 1 {
		t.Fatalf("unexpected first journal result: streak=%d rewarded=%v err=%v", first, rewarded, err)
	}
	_, rewarded, err = service.RecordDaily(context.Background(), account.ID, "陪伴")
	if err != nil || rewarded {
		t.Fatalf("same day must not reward twice: rewarded=%v err=%v", rewarded, err)
	}
	*now = now.AddDate(0, 0, 3)
	streak, rewarded, err := service.RecordDaily(context.Background(), account.ID, "陪伴")
	if err != nil || !rewarded || streak != 2 {
		t.Fatalf("rolling journal must retain earlier day: streak=%d rewarded=%v err=%v", streak, rewarded, err)
	}
	var pet models.PetProfile
	if err = db.First(&pet, "account_id = ?", account.ID).Error; err != nil {
		t.Fatal(err)
	}
	if pet.Affection != 2 {
		t.Fatalf("expected exactly two daily rewards, got affection=%d", pet.Affection)
	}
}

func TestValidatePlayerNameRejectsContactAndControlContent(t *testing.T) {
	for _, candidate := range []string{"a", "联系QQ123", "https://example.com", "坏\n名字"} {
		if err := ValidatePlayerName(candidate); err == nil {
			t.Fatalf("expected %q to be rejected", candidate)
		}
	}
	if err := ValidatePlayerName("星光小队"); err != nil {
		t.Fatalf("expected ordinary name to pass: %v", err)
	}
}

func TestCreateSquadIsCommunityScoped(t *testing.T) {
	service, _, _ := newTestService(t)
	event := oneBotEvent("100", "42", "小队 创建 星光队")
	account, err := service.ResolveAccount(context.Background(), event)
	if err != nil {
		t.Fatal(err)
	}
	squad, err := service.CreateSquad(context.Background(), event, account.ID, "星光队")
	if err != nil {
		t.Fatal(err)
	}
	if squad.MaxMembers != 12 || squad.CommunityID != communityID(event) {
		t.Fatalf("unexpected squad: %#v", squad)
	}
}

func TestDeleteAccountRemovesGlobalProgressAndIdentities(t *testing.T) {
	service, db, _ := newTestService(t)
	event := oneBotEvent("100", "42", "确认删除我的数据")
	account, err := service.ResolveAccount(context.Background(), event)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = service.Adopt(context.Background(), account.ID, "光芽兽", "光芽兽"); err != nil {
		t.Fatal(err)
	}
	if err = db.Create(&models.PetBehaviorProfile{AccountID: account.ID, Care: 3, Trait: "温柔"}).Error; err != nil {
		t.Fatal(err)
	}
	if err = db.Create(&models.SeasonVote{SeasonKey: "S001", CommunityID: "community", AccountID: account.ID, Choice: 1}).Error; err != nil {
		t.Fatal(err)
	}
	if err = service.DeleteAccount(context.Background(), account.ID); err != nil {
		t.Fatal(err)
	}
	var identities int64
	var pets int64
	var behaviors int64
	var votes int64
	db.Model(&models.PlayerIdentity{}).Where("account_id = ?", account.ID).Count(&identities)
	db.Model(&models.PetProfile{}).Where("account_id = ?", account.ID).Count(&pets)
	db.Model(&models.PetBehaviorProfile{}).Where("account_id = ?", account.ID).Count(&behaviors)
	db.Model(&models.SeasonVote{}).Where("account_id = ?", account.ID).Count(&votes)
	if identities != 0 || pets != 0 || behaviors != 0 || votes != 0 {
		t.Fatalf("account data remains: identities=%d pets=%d behaviors=%d votes=%d", identities, pets, behaviors, votes)
	}
}

func TestExpeditionStatusReportsRemainingTime(t *testing.T) {
	service, _, now := newTestService(t)
	event := oneBotEvent("100", "42", "远征状态")
	account, _ := service.ResolveAccount(context.Background(), event)
	_, _ = service.Adopt(context.Background(), account.ID, "光芽兽", "光芽兽")
	run, _ := service.StartExpedition(context.Background(), account.ID, 1)
	*now = now.Add(5 * time.Minute)
	text, err := service.FormatExpeditionStatus(context.Background(), account.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(text, run.Name) || !strings.Contains(text, "5分钟") {
		t.Fatalf("unexpected status text: %q", text)
	}
}

func TestExpeditionStartUsesPlayerFacingCopy(t *testing.T) {
	service, _, _ := newTestService(t)
	router := core.NewCommandRouter()
	if err := RegisterCommands(router.Register, func() *Service { return service }); err != nil {
		t.Fatal(err)
	}
	event := oneBotEvent("100", "42", "领养 光芽兽")
	_, _, _ = router.Route(context.Background(), event)
	event.Text = "远征 1"
	message, handled, err := router.Route(context.Background(), event)
	if err != nil || !handled {
		t.Fatalf("unexpected expedition start: handled=%v err=%v", handled, err)
	}
	if strings.Contains(message.Text, "固定进度") || strings.Contains(message.Text, "不含抽奖") {
		t.Fatalf("internal design copy leaked to player: %q", message.Text)
	}
	if !strings.Contains(message.Text, "主要目标") || !strings.Contains(message.Text, "消耗：饱食") || !strings.Contains(message.Text, "加成：") {
		t.Fatalf("expected player-facing expedition copy, got %q", message.Text)
	}
}

func TestBossCommandSupportsStatusAndAsynchronousSupport(t *testing.T) {
	service, db, now := newTestService(t)
	if err := db.Create(&models.AdventureMonsterConfig{
		Key: "rock-boss-monster", Name: "岩甲兽", Level: 3,
		MaxHealth: 50, Attack: 5, Defense: 2, Enabled: true,
	}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&models.AdventureBossConfig{
		Key: "rock-boss", MapKey: "starter-map", ZoneKey: "forest-edge", MonsterKey: "rock-boss-monster",
		Name: "岩甲兽", ScheduleAnchor: now.Add(-time.Minute), SpawnIntervalMinutes: 60,
		ActiveDurationMinutes: 30, RecommendedLevel: 1, MaxHealth: 50, Attack: 5,
		Defense: 2, MinimumContribution: 1, Enabled: true,
	}).Error; err != nil {
		t.Fatal(err)
	}
	router := core.NewCommandRouter()
	if err := RegisterCommands(router.Register, func() *Service { return service }); err != nil {
		t.Fatal(err)
	}
	event := oneBotEvent("100", "42", "领养 光芽兽")
	_, _, _ = router.Route(context.Background(), event)
	event.Text = "首领"
	message, handled, err := router.Route(context.Background(), event)
	if err != nil || !handled || !strings.Contains(message.Text, "岩甲兽") || !strings.Contains(message.Text, "限时地图首领") {
		t.Fatalf("unexpected boss status: handled=%v err=%v text=%q", handled, err, message.Text)
	}
	event.Text = "首领 挑战 rock-boss"
	message, _, err = router.Route(context.Background(), event)
	if err != nil || !strings.Contains(message.Text, "首领挑战开始") {
		t.Fatalf("unexpected boss challenge: err=%v text=%q", err, message.Text)
	}
}

func TestSeasonAndFacilityCommandsHaveTextOnlyFlows(t *testing.T) {
	service, db, _ := newTestService(t)
	router := core.NewCommandRouter()
	if err := RegisterCommands(router.Register, func() *Service { return service }); err != nil {
		t.Fatal(err)
	}
	event := oneBotEvent("100", "42", "赛季")
	message, handled, err := router.Route(context.Background(), event)
	if err != nil || !handled || !strings.Contains(message.Text, "回复“赛季 投票 1”到“赛季 投票 3”") {
		t.Fatalf("unexpected season text: handled=%v err=%v text=%q", handled, err, message.Text)
	}
	account, _ := service.ResolveAccount(context.Background(), event)
	community, _ := service.GetCommunity(context.Background(), event, account.ID)
	db.Model(&models.Community{}).Where("id = ?", community.ID).Update("materials", 100)
	event.Text = "设施 升级 研究站"
	message, handled, err = router.Route(context.Background(), event)
	if err != nil || !handled || !strings.Contains(message.Text, "研究站已升级到 Lv.2") {
		t.Fatalf("unexpected facility text: handled=%v err=%v text=%q", handled, err, message.Text)
	}
}

func TestEventCommandShowsTrackAndDoesNotDuplicateSettledReward(t *testing.T) {
	service, db, now := newTestService(t)
	if err := db.Create(&models.LiveEventConfig{
		Key: "forest-week", Name: "森林调查周", Region: "森林", Active: true,
		StartsAt: now.Add(-time.Hour), EndsAt: now.Add(time.Hour), StoryChoices: `["一","二","三"]`,
	}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&models.ItemConfig{Key: "wood", Name: "木材", Status: "active"}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&models.RewardTrackConfig{EventKey: "forest-week", Milestone: 5, RewardType: "item", RewardKey: "wood", RewardName: "木材", Quantity: 2}).Error; err != nil {
		t.Fatal(err)
	}
	router := core.NewCommandRouter()
	if err := RegisterCommands(router.Register, func() *Service { return service }); err != nil {
		t.Fatal(err)
	}
	event := oneBotEvent("100", "event-command", "活动")
	account, err := service.ResolveAccount(context.Background(), event)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err = service.AddEventProgress(context.Background(), account.ID, "test:progress", 5); err != nil {
		t.Fatal(err)
	}
	message, handled, err := router.Route(context.Background(), event)
	if err != nil || !handled || !strings.Contains(message.Text, "森林调查周") || !strings.Contains(message.Text, "活动进度：5") || !strings.Contains(message.Text, "✅ 5｜木材 ×2") {
		t.Fatalf("unexpected event status: handled=%v err=%v text=%q", handled, err, message.Text)
	}
	event.Text = "活动 领取"
	message, handled, err = router.Route(context.Background(), event)
	if err != nil || !handled || !strings.Contains(message.Text, "没有新的里程碑奖励") {
		t.Fatalf("already settled reward should not be duplicated: handled=%v err=%v text=%q", handled, err, message.Text)
	}
}

func TestHelpRequestCommandsUseCodeBasedLimitedSupport(t *testing.T) {
	service, _, _ := newTestService(t)
	router := core.NewCommandRouter()
	if err := RegisterCommands(router.Register, func() *Service { return service }); err != nil {
		t.Fatal(err)
	}
	event := oneBotEvent("100", "requester", "求助 木材 5")
	message, handled, err := router.Route(context.Background(), event)
	if err != nil || !handled || !strings.Contains(message.Text, "求助已发布") || !strings.Contains(message.Text, "支援 ") {
		t.Fatalf("unexpected help request response: handled=%v err=%v text=%q", handled, err, message.Text)
	}
	event = oneBotEvent("100", "viewer", "求助列表")
	message, handled, err = router.Route(context.Background(), event)
	if err != nil || !handled || !strings.Contains(message.Text, "木材 0/5") {
		t.Fatalf("unexpected help list response: handled=%v err=%v text=%q", handled, err, message.Text)
	}
}

func TestRoleCommandShowsDeterministicThreeSkillLoadout(t *testing.T) {
	service, _, _ := newTestService(t)
	router := core.NewCommandRouter()
	if err := RegisterCommands(router.Register, func() *Service { return service }); err != nil {
		t.Fatal(err)
	}
	event := oneBotEvent("100", "42", "领养 光芽兽")
	_, _, _ = router.Route(context.Background(), event)
	event.Text = "定位 守护者"
	message, handled, err := router.Route(context.Background(), event)
	if err != nil || !handled {
		t.Fatalf("unexpected role response: handled=%v err=%v text=%q", handled, err, message.Text)
	}
	if !strings.Contains(message.Text, "成长定位已更新") || !strings.Contains(message.Text, "守护者") {
		t.Fatalf("role response should confirm the growth badge: %q", message.Text)
	}
	if strings.Contains(message.Text, "护盾") || strings.Contains(message.Text, "寻路") {
		t.Fatalf("role must not copy growth-flavor skill names into combat skills: %q", message.Text)
	}
}

func TestSkillsCommandListsUnlockedAdventureSkillNames(t *testing.T) {
	service, db, _ := newTestService(t)
	if err := db.Create(&models.AdventureSkillConfig{Key: "pet_skill_01", Name: "芽光连击", Enabled: true}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&models.PetSkillUnlockConfig{FormKey: "光芽兽", SkillKey: "pet_skill_01", UnlockLevel: 1, SortOrder: 10}).Error; err != nil {
		t.Fatal(err)
	}
	router := core.NewCommandRouter()
	if err := RegisterCommands(router.Register, func() *Service { return service }); err != nil {
		t.Fatal(err)
	}
	event := oneBotEvent("100", "42", "领养 光芽兽")
	_, _, _ = router.Route(context.Background(), event)
	event.Text = "技能"
	message, handled, err := router.Route(context.Background(), event)
	if err != nil || !handled || !strings.Contains(message.Text, "战斗技能") || !strings.Contains(message.Text, "芽光连击") {
		t.Fatalf("skills command should list configured combat names: handled=%v err=%v text=%q", handled, err, message.Text)
	}
	if strings.Contains(message.Text, "寻路") || strings.Contains(message.Text, "pet_skill_01") {
		t.Fatalf("skills command leaked internal keys or growth flavor: %q", message.Text)
	}
}

func TestStatusShowsSpeciesChineseName(t *testing.T) {
	service, db, _ := newTestService(t)
	if err := db.Create(&models.PetSpeciesConfig{Key: "galeear_base", Name: "风耳狐", FamilyKey: "galeear", Stage: "base", Adoptable: true}).Error; err != nil {
		t.Fatal(err)
	}
	event := oneBotEvent("100", "form-name", "领养 光芽兽")
	if _, err := handleAdopt(context.Background(), event, service); err != nil {
		t.Fatal(err)
	}
	accountID := accountIDForTest(t, service, event)
	if err := db.Model(&models.PetProfile{}).Where("account_id = ?", accountID).Updates(map[string]any{"current_form": "galeear_base", "pet_type": "galeear_base"}).Error; err != nil {
		t.Fatal(err)
	}
	event.Text = "我的宠物"
	message, err := handleStatus(context.Background(), event, service)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(message.Text, "风耳狐") {
		t.Fatalf("状态页应显示物种中文名: %q", message.Text)
	}
	if strings.Contains(message.Text, "galeear_base") {
		t.Fatalf("状态页不应展示形态字段名: %q", message.Text)
	}
}

func TestPetListHidesFormKey(t *testing.T) {
	service, db, _ := newTestService(t)
	if err := db.Save(&models.SystemConfig{Key: gameplay.MaxPetSlotsConfigKey, Value: "2"}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&models.PetSpeciesConfig{Key: "galeear_base", Name: "风耳狐", FamilyKey: "galeear", Stage: "base", Adoptable: true}).Error; err != nil {
		t.Fatal(err)
	}
	event := oneBotEvent("100", "list-form", "领养 光芽兽")
	if _, err := handleAdopt(context.Background(), event, service); err != nil {
		t.Fatal(err)
	}
	accountID := accountIDForTest(t, service, event)
	if err := db.Model(&models.PetProfile{}).Where("account_id = ?", accountID).Updates(map[string]any{"current_form": "galeear_base"}).Error; err != nil {
		t.Fatal(err)
	}
	event.Text = "宠物列表"
	message, err := handlePetList(context.Background(), event, service)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(message.Text, "galeear_base") {
		t.Fatalf("宠物列表不应展示形态字段名: %q", message.Text)
	}
	if !strings.Contains(message.Text, "风耳狐") {
		t.Fatalf("宠物列表应显示中文形态: %q", message.Text)
	}
}

func TestCodexHidesEntryKey(t *testing.T) {
	service, db, _ := newTestService(t)
	event := oneBotEvent("100", "codex-name", "领养 光芽兽")
	if _, err := handleAdopt(context.Background(), event, service); err != nil {
		t.Fatal(err)
	}
	accountID := accountIDForTest(t, service, event)
	if err := db.Create(&models.AdventureZoneConfig{Key: "sunlit_steppe_z1", Name: "萤草坡", MapKey: "sunlit", Enabled: true}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&models.CodexCatalogConfig{Category: "区域生态", EntryKey: "sunlit_steppe_z1", Region: "栖光原野", Description: "萤草坡调查记录", SourceType: "zone", SourceKey: "sunlit_steppe_z1", Enabled: true}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&models.CodexEntry{AccountID: accountID, Category: "区域生态", EntryKey: "sunlit_steppe_z1", Progress: 40}).Error; err != nil {
		t.Fatal(err)
	}
	event.Text = "图鉴"
	message, err := handleCodex(context.Background(), event, service)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(message.Text, "sunlit_steppe_z1") {
		t.Fatalf("图鉴不应展示条目字段名: %q", message.Text)
	}
	if !strings.Contains(message.Text, "萤草坡") {
		t.Fatalf("图鉴应显示区域中文名: %q", message.Text)
	}
}

func TestSeasonTitleOmitsEventKey(t *testing.T) {
	service, _, _ := newTestService(t)
	event := oneBotEvent("100", "season-title", "赛季")
	message, err := handleSeason(context.Background(), event, service)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(message.Text, "test-season") {
		t.Fatalf("赛季标题不应展示活动字段名: %q", message.Text)
	}
	if !strings.Contains(message.Text, "测试活动") {
		t.Fatalf("赛季标题应显示活动中文名: %q", message.Text)
	}
}

func TestAwakenCommandRequiresExplicitBranchWhenMultipleRoutesExist(t *testing.T) {
	service, db, _ := newTestService(t)
	if err := db.Create(&[]models.PetSpeciesConfig{
		{Key: "lumisprout_evolved", Name: "曜叶兽", FamilyKey: "lumisprout", Stage: "evolved"},
		{Key: "lumisprout_awaken_a", Name: "曦冠灵", FamilyKey: "lumisprout", Stage: "awakened", PreviousFormKey: "lumisprout_evolved", Image: "宠物/曦冠灵.png"},
		{Key: "lumisprout_awaken_b", Name: "月冕灵", FamilyKey: "lumisprout", Stage: "awakened", PreviousFormKey: "lumisprout_evolved", Image: "宠物/月冕灵.png"},
	}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&[]models.PetEvolutionRuleConfig{
		{Key: "lumisprout_awaken_a_rule", FromFormKey: "lumisprout_evolved", ToFormKey: "lumisprout_awaken_a", RequiredGrowth: 1, RequiredAffection: 1, BranchLabel: "曦光路线", Enabled: true, SortOrder: 20},
		{Key: "lumisprout_awaken_b_rule", FromFormKey: "lumisprout_evolved", ToFormKey: "lumisprout_awaken_b", RequiredGrowth: 1, RequiredAffection: 1, BranchLabel: "月影路线", Enabled: true, SortOrder: 30},
	}).Error; err != nil {
		t.Fatal(err)
	}
	router := core.NewCommandRouter()
	if err := RegisterCommands(router.Register, func() *Service { return service }); err != nil {
		t.Fatal(err)
	}
	event := oneBotEvent("100", "awaken-choice", "领养 光芽兽")
	_, _, _ = router.Route(context.Background(), event)
	if err := db.Model(&models.PetProfile{}).Where("account_id = ?", accountIDForTest(t, service, event)).Updates(map[string]any{"current_form": "lumisprout_evolved", "growth": 20, "affection": 10}).Error; err != nil {
		t.Fatal(err)
	}
	event.Text = "觉醒"
	message, handled, err := router.Route(context.Background(), event)
	if err != nil || !handled || !strings.Contains(message.Text, "曦光路线") || !strings.Contains(message.Text, "月影路线") {
		t.Fatalf("awaken without a branch should list both routes: handled=%v err=%v text=%q", handled, err, message.Text)
	}
	if strings.Contains(message.Text, "确认觉醒") && !strings.Contains(message.Text, "发送“觉醒") {
		t.Fatalf("awaken without a branch must not confirm a locked route: %q", message.Text)
	}
	event.Text = "确认觉醒"
	message, handled, err = router.Route(context.Background(), event)
	if err != nil || !handled || !strings.Contains(message.Text, "选择") {
		t.Fatalf("confirm awaken without a branch must ask the player to choose: handled=%v err=%v text=%q", handled, err, message.Text)
	}
	event.Text = "觉醒 月影路线"
	message, handled, err = router.Route(context.Background(), event)
	if err != nil || !handled || !strings.Contains(message.Text, "月冕灵") || !strings.Contains(message.Text, "确认觉醒 月影路线") {
		t.Fatalf("selected branch preview mismatch: handled=%v err=%v text=%q", handled, err, message.Text)
	}
	event.Text = "确认觉醒 月影路线"
	message, handled, err = router.Route(context.Background(), event)
	if err != nil || !handled || !strings.Contains(message.Text, "月冕灵") {
		t.Fatalf("confirming the chosen branch failed: handled=%v err=%v text=%q", handled, err, message.Text)
	}
	var pet models.PetProfile
	if err = db.First(&pet, "account_id = ?", accountIDForTest(t, service, event)).Error; err != nil {
		t.Fatal(err)
	}
	if pet.CurrentForm != "lumisprout_awaken_b" {
		t.Fatalf("player-chosen awaken branch was not persisted: %#v", pet)
	}
}

func accountIDForTest(t *testing.T, service *Service, event core.InboundEvent) string {
	t.Helper()
	account, err := service.ResolveAccount(context.Background(), event)
	if err != nil {
		t.Fatal(err)
	}
	return account.ID
}

func TestInvalidTacticalInputReturnsActionableText(t *testing.T) {
	service, _, _ := newTestService(t)
	router := core.NewCommandRouter()
	if err := RegisterCommands(router.Register, func() *Service { return service }); err != nil {
		t.Fatal(err)
	}
	event := oneBotEvent("100", "42", "领养 光芽兽")
	_, _, _ = router.Route(context.Background(), event)
	event.Text = "编队 乱来"
	message, handled, err := router.Route(context.Background(), event)
	if err != nil || !handled || !strings.Contains(message.Text, "攻击、守护、支援或探索") {
		t.Fatalf("expected actionable validation response: handled=%v err=%v text=%q", handled, err, message.Text)
	}
}

func TestPlayerMenuSnapshotAndBannedTerms(t *testing.T) {
	message, err := handleMenu(context.Background(), core.InboundEvent{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	expected := "🐾【宠物菜单】\n\n" +
		"🌟 开始陪伴\n宠物菜单 · 领养宠物 · 我的宠物 · 宠物列表 · 切换宠物 · 签到 · 今日待办 · 我的背包 · 改名 · 治疗 · 找回 · 放生 · 帮助\n\n" +
		"🛍️ 背包与商店\n商店 · 好感商店 · 查看商品 · 查看物品 · 使用\n\n" +
		"🍖 日常陪伴\n喂养 · 摸头 · 散步 · 送礼 · 洗澡\n\n" +
		"📚 成长计划\n学习 · 锻炼 · 健身 · 打工 · 进化 · 觉醒 · 定位 · 编队 · 技能 · 图鉴\n\n" +
		"🧭 探索远征\n远征 · 远征状态 · 领取 · 地图 · 探索 · 远征背包 · 材料背包 · 装备背包 · 蓝图背包 · 远征商店 · 地图首领 · 首领\n\n" +
		"🎲 休闲玩法\n钓鱼 · 抛竿 · 收竿 · 抽奖 · 猜拳 · 宠物交易 · 交易列表 · 接受交易 · 交易信息 · 取消交易\n\n" +
		"🏕️ 社群协作\n营地 · 共建 · 小队 · 活动 · 赛季 · 设施 · 求助 · 求助列表 · 支援\n\n" +
		"🔐 账号与隐私\n生成绑定码 · 绑定 · 我的数据\n\n" +
		"💡 直接发送上面的命令即可使用\n例如：签到 / 我的宠物 / 远征\n不知道做什么？发送“今日待办”。"
	if message.Text != expected {
		t.Fatalf("menu snapshot changed:\n%s", message.Text)
	}
	if message.Markdown == nil || !strings.Contains(message.Markdown.Content, "# 🐾 宠物菜单") || !strings.Contains(message.Markdown.Content, "**🌟 开始陪伴**") {
		t.Fatalf("menu markdown snapshot missing formatting: %#v", message.Markdown)
	}
	for _, word := range []string{"新版", "旧版", "下线", "固定进度", "平台额度", "内部账号", "不含抽奖", "迁移"} {
		if strings.Contains(message.Text, word) {
			t.Fatalf("menu contains banned term %q", word)
		}
	}
}

func TestBindCommandConvertsTechnicalErrorToSafePlayerMessage(t *testing.T) {
	definition := commandDefinition{feature: feature("test_failure", "测试", "测试", "基础", "测试", 0), handler: func(context.Context, core.InboundEvent, *Service) (core.OutboundMessage, error) {
		return core.OutboundMessage{}, errors.New("SQL: secret path C:/private")
	}}
	handler := bindCommand(definition, func() *Service { return &Service{DB: &gorm.DB{}} })
	message, err := handler(context.Background(), core.InboundEvent{Text: "测试"})
	if err != nil || message.MessageKey != "system.temporarily_unavailable" || message.TechnicalResult != "error" || strings.Contains(message.Text, "SQL") || strings.Contains(message.Text, "C:/") {
		t.Fatalf("unsafe technical response: message=%#v err=%v", message, err)
	}
}

func TestBindCommandAssignsStableMessageKey(t *testing.T) {
	definition := commandDefinition{feature: feature("test_success", "测试", "测试", "基础", "测试", 0), handler: func(context.Context, core.InboundEvent, *Service) (core.OutboundMessage, error) {
		message := text("完成")
		message.BusinessResult = "success"
		return message, nil
	}}
	handler := bindCommand(definition, func() *Service { return &Service{DB: &gorm.DB{}} })
	message, err := handler(context.Background(), core.InboundEvent{Text: "测试"})
	if err != nil || message.MessageKey != "command.test_success.success" {
		t.Fatalf("stable message key missing: message=%#v err=%v", message, err)
	}
}

func TestLotteryRulesUsePlayerFacingCopy(t *testing.T) {
	service, db, _ := newTestService(t)
	if err := db.Create(&models.ChanceGameConfig{
		GameKey: "lottery", Name: "遗迹抽签", Enabled: true, CostItem: "遗迹抽签券", CostQuantity: 1,
		DailyLimit: 3, PityThreshold: 10, PityRewardKey: "star_core", Rules: "不售卖抽签券，第10次保底星辉晶核。",
	}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&[]models.ChanceRewardConfig{
		{GameKey: "lottery", RewardKey: "lottery_meadow_fiber", Name: "原野纤维", Weight: 60, ItemName: "原野纤维", Quantity: 1, Enabled: true, SortOrder: 10},
		{GameKey: "lottery", RewardKey: "lottery_star_core", Name: "星辉晶核", Weight: 2, ItemName: "星辉晶核", Quantity: 1, Rare: true, Enabled: true, SortOrder: 40},
	}).Error; err != nil {
		t.Fatal(err)
	}
	message, err := handleLottery(context.Background(), oneBotEvent("100", "lottery-copy", "抽奖"), service)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(message.Text, "不售卖") || strings.Contains(message.Text, "抽奖 1") {
		t.Fatalf("抽奖规则泄漏了内部备注或生硬指令: %q", message.Text)
	}
	if !strings.Contains(message.Text, "遗迹抽签券") || !strings.Contains(message.Text, "奖池") || !strings.Contains(message.Text, "原野纤维") || !strings.Contains(message.Text, "星辉晶核") {
		t.Fatalf("抽奖规则应说明消耗、奖池和保底: %q", message.Text)
	}
	if !strings.Contains(message.Text, "抽奖一次") {
		t.Fatalf("无 Markdown 的机器人也必须能靠纯文本指令抽奖: %q", message.Text)
	}
	assertPlainTextCompatible(t, message)
	if message.Keyboard == nil || len(message.Keyboard.Rows) == 0 || message.Keyboard.Rows[0][0].Command != "抽奖一次" {
		t.Fatalf("抽奖规则应提供抽取按钮: %#v", message.Keyboard)
	}
	plain := message.Render(false, false)
	if plain.Markdown != nil || plain.Keyboard != nil || !strings.Contains(plain.Text, "抽奖一次") {
		t.Fatalf("关闭 Markdown/键盘后应只保留纯文本指令: %#v", plain)
	}
}

func assertPlainTextCompatible(t *testing.T, message core.OutboundMessage) {
	t.Helper()
	text := message.Text
	if strings.Contains(text, "**") || strings.Contains(text, "```") || strings.Contains(text, "`") || strings.Contains(text, "\n# ") || strings.HasPrefix(strings.TrimSpace(text), "#") {
		t.Fatalf("纯文本通道不应包含 Markdown 语法: %q", text)
	}
}

func TestRestoredRiskyCommandsExposeRulesAndSafeFlows(t *testing.T) {
	service, _, _ := newTestService(t)
	router := core.NewCommandRouter()
	if err := RegisterCommands(router.Register, func() *Service { return service }); err != nil {
		t.Fatal(err)
	}
	expectations := map[string]string{"钓鱼": "奖池", "抽奖": "奖池", "宠物交易": "托管"}
	for command, expected := range expectations {
		message, handled, err := router.Route(context.Background(), oneBotEvent("100", "42", command))
		if err != nil || !handled || !strings.Contains(message.Text, expected) {
			t.Fatalf("restored command %q did not expose its real rules: handled=%v err=%v text=%q", command, handled, err, message.Text)
		}
	}
}

func TestRoleAndStanceMenusReadConfiguredRules(t *testing.T) {
	service, db, _ := newTestService(t)
	if err := db.Create(&models.GrowthRoleConfig{Name: "采集者", Description: "专注素材调查", Skill1: "辨识", Skill2: "采样", Skill3: "整理", Enabled: true}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&models.GrowthStanceConfig{Name: "潜行", Description: "避开风险", Enabled: true}).Error; err != nil {
		t.Fatal(err)
	}
	router := core.NewCommandRouter()
	if err := RegisterCommands(router.Register, func() *Service { return service }); err != nil {
		t.Fatal(err)
	}
	event := oneBotEvent("100", "configured-menu", "定位")
	message, handled, err := router.Route(context.Background(), event)
	if err != nil || !handled || !strings.Contains(message.Text, "采集者：专注素材调查") {
		t.Fatalf("role menu did not use configured rules: handled=%v err=%v text=%q", handled, err, message.Text)
	}
	event.Text = "编队"
	message, handled, err = router.Route(context.Background(), event)
	if err != nil || !handled || !strings.Contains(message.Text, "潜行：避开风险") {
		t.Fatalf("stance menu did not use configured rules: handled=%v err=%v text=%q", handled, err, message.Text)
	}
}
