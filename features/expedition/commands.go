package expedition

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"gorm.io/gorm"
	"qq-pet-saas/core"
	"qq-pet-saas/database"
	"qq-pet-saas/gameplayrules"
	"qq-pet-saas/models"
)

type commandRegister func(string, core.UnifiedHandler) error

func init() {
	register := func(command string, handler core.UnifiedHandler) error {
		return core.RegisterUnifiedFeature(commandFeature(command), handler)
	}
	if err := RegisterCommands(register, func() *Service { return NewService(database.DB) }); err != nil {
		panic(err)
	}
}

func commandFeature(command string) core.UnifiedFeature {
	feature := core.UnifiedFeature{FuncName: command, DefaultCommand: command, DisplayName: command, Category: "基础", Description: "执行“" + command + "”功能", Enabled: true, SortOrder: 500}
	groups := map[string][]string{
		"远征": {"远征", "远征状态", "领取"},
		"成长": {"定位", "编队", "技能", "图鉴"},
		"陪伴": {"喂养", "摸头", "散步", "送礼", "洗澡", "今日", "状态"},
		"社区": {"营地", "共建", "小队", "首领", "赛季", "设施", "求助", "求助列表", "支援"},
		"账号": {"生成绑定码", "绑定", "我的数据", "关闭通知", "开启通知", "解绑身份", "确认删除我的数据"},
	}
	for category, commands := range groups {
		for index, value := range commands {
			if value == command {
				feature.Category = category
				feature.SortOrder = index + map[string]int{"基础": 0, "远征": 100, "成长": 200, "陪伴": 300, "社区": 400, "账号": 600}[category]
			}
		}
	}
	descriptions := map[string]string{
		"状态": "查看宠物近况、准备度和当前远征", "今日": "查看今日推荐和陪伴进度", "远征": "选择短途、常规或深度远征", "远征状态": "查看进行中的远征与返回时间", "领取": "领取已完成远征的结算结果",
		"定位": "选择宠物长期成长定位", "编队": "调整探索、守护或支援姿态", "技能": "查看当前定位对应技能", "图鉴": "查看区域生物、材料与事件进度",
		"喂养": "进行一次不惩罚的日常照料", "摸头": "增加陪伴行为记录", "散步": "进行一次轻量探索陪伴", "送礼": "记录一次支援型陪伴", "洗澡": "恢复宠物准备度",
		"营地": "查看当前群或频道社区", "共建": "向社区设施贡献材料", "小队": "管理三至十二人的远征小队", "首领": "参加异步社区首领挑战", "求助": "发布社区物品求助", "支援": "响应其他成员的求助",
		"生成绑定码": "生成十分钟有效的一次性绑定码", "绑定": "将当前平台身份绑定到已有账号", "我的数据": "查看已保存的账号与隐私数据", "解绑身份": "解除当前平台身份绑定",
	}
	if description := descriptions[command]; description != "" {
		feature.Description = description
	}
	hiddenAliases := map[string]bool{"领养宠物": true, "我的背包": true, "签到": true, "确认放生": true}
	retired := map[string]bool{"抽奖": true, "猜拳": true, "偷袭": true, "回击": true, "宠物交易": true, "接受交易": true, "拒绝交易": true, "添加交易": true, "删除交易": true, "交易信息": true, "取消交易": true, "确认取消": true, "同意交易": true, "学习": true, "完成学习": true, "锻炼": true, "完成锻炼": true, "健身": true, "完成健身": true, "打工": true, "完成打工": true, "钓鱼": true, "宠物钓鱼": true, "抛竿": true, "收竿": true, "创建家族": true, "加入家族": true, "注销家族": true, "退出家族": true, "我的家族": true, "家族列表": true, "家族成员": true, "踢出成员": true, "神树浇水": true}
	feature.Hidden = hiddenAliases[command] || retired[command]
	return feature
}

func RegisterCommands(register commandRegister, serviceFactory func() *Service) error {
	commands := map[string]func(context.Context, core.InboundEvent, *Service) (core.OutboundMessage, error){
		"确认删除我的数据": handleConfirmDelete,
		"生成绑定码":    handleGenerateBind,
		"远征状态":     handleExpeditionStatus,
		"领养宠物":     handleAdoptList,
		"我的数据":     handleMyData,
		"关闭通知":     handleDisableNotifications,
		"开启通知":     handleEnableNotifications,
		"解绑身份":     handleUnbind,
		"小队":       handleSquad,
		"首领":       handleBoss,
		"赛季":       handleSeason,
		"设施":       handleFacility,
		"求助列表":     handleHelpList,
		"求助":       handleHelpRequest,
		"支援":       handleHelpSupport,
		"共建":       handleContribute,
		"营地":       handleCommunity,
		"图鉴":       handleCodex,
		"我的背包":     handleInventory,
		"背包":       handleInventory,
		"编队":       handleStance,
		"定位":       handleRole,
		"技能":       handleSkills,
		"领取":       handleClaim,
		"远征":       handleExpedition,
		"签到":       handleDaily,
		"今日":       handleDaily,
		"状态":       handleStatus,
		"喂养":       handleCompanion,
		"摸头":       handleCompanion,
		"散步":       handleCompanion,
		"送礼":       handleCompanion,
		"洗澡":       handleCompanion,
		"领养":       handleAdopt,
		"绑定":       handleRedeemBind,
		"帮助":       handleHelp,
		"放生":       handleFoster,
		"确认放生":     handleFoster,
		"抽奖":       handleRetiredRiskyFeature,
		"猜拳":       handleRetiredRiskyFeature,
		"偷袭":       handleRetiredRiskyFeature,
		"回击":       handleRetiredRiskyFeature,
		"宠物交易":     handleRetiredRiskyFeature,
		"接受交易":     handleRetiredRiskyFeature,
		"拒绝交易":     handleRetiredRiskyFeature,
		"添加交易":     handleRetiredRiskyFeature,
		"删除交易":     handleRetiredRiskyFeature,
		"交易信息":     handleRetiredRiskyFeature,
		"取消交易":     handleRetiredRiskyFeature,
		"确认取消":     handleRetiredRiskyFeature,
		"同意交易":     handleRetiredRiskyFeature,
		"学习":       handleRetiredTimedFeature,
		"完成学习":     handleRetiredTimedFeature,
		"锻炼":       handleRetiredTimedFeature,
		"完成锻炼":     handleRetiredTimedFeature,
		"健身":       handleRetiredTimedFeature,
		"完成健身":     handleRetiredTimedFeature,
		"打工":       handleRetiredTimedFeature,
		"完成打工":     handleRetiredTimedFeature,
		"钓鱼":       handleRetiredFishing,
		"宠物钓鱼":     handleRetiredFishing,
		"抛竿":       handleRetiredFishing,
		"收竿":       handleRetiredFishing,
		"创建家族":     handleLegacyFamily,
		"加入家族":     handleLegacyFamily,
		"注销家族":     handleLegacyFamily,
		"退出家族":     handleLegacyFamily,
		"我的家族":     handleLegacyFamily,
		"家族列表":     handleLegacyFamily,
		"家族成员":     handleLegacyFamily,
		"踢出成员":     handleLegacyFamily,
		"神树浇水":     handleLegacyFamily,
	}
	for command, handler := range commands {
		current := handler
		if err := register(command, func(ctx context.Context, event core.InboundEvent) (core.OutboundMessage, error) {
			service := serviceFactory()
			if service == nil || service.DB == nil {
				return core.OutboundMessage{}, errors.New("游戏数据库尚未初始化")
			}
			return current(ctx, event, service)
		}); err != nil {
			return err
		}
	}
	return nil
}

func handleCompanion(ctx context.Context, event core.InboundEvent, service *Service) (core.OutboundMessage, error) {
	account, err := resolve(ctx, service, event)
	if err != nil {
		return core.OutboundMessage{}, err
	}
	var pet models.PetProfile
	if err = service.DB.First(&pet, "account_id = ?", account.ID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return friendlyError(ErrPetRequired)
		}
		return core.OutboundMessage{}, err
	}
	action := strings.Fields(event.Text)[0]
	result := map[string]string{
		"喂养": "宠物吃得很满足，准备度保持稳定。", "摸头": "宠物靠近了你，留下了一次温柔陪伴记录。", "散步": "你们完成了一次轻松散步，没有强制奖励或惩罚。", "送礼": "宠物认真收下礼物，陪伴记录向支援方向增长。", "洗澡": "宠物恢复了清爽状态，准备度有所恢复。",
	}[action]
	err = service.DB.Transaction(func(tx *gorm.DB) error {
		profile := models.PetBehaviorProfile{AccountID: account.ID}
		if findErr := tx.FirstOrCreate(&profile, models.PetBehaviorProfile{AccountID: account.ID}).Error; findErr != nil {
			return findErr
		}
		switch action {
		case "散步":
			profile.Explore++
		case "送礼":
			profile.Support++
		default:
			profile.Care++
		}
		if action == "洗澡" || action == "喂养" {
			pet.Readiness += 3
			if pet.Readiness > 100 {
				pet.Readiness = 100
			}
			if saveErr := tx.Save(&pet).Error; saveErr != nil {
				return saveErr
			}
		}
		return tx.Save(&profile).Error
	})
	if err != nil {
		return core.OutboundMessage{}, err
	}
	return text("【陪伴完成】\n" + result + "\n\n发送“今日”查看下一项推荐。"), nil
}

func resolve(ctx context.Context, service *Service, event core.InboundEvent) (*models.PlayerAccount, error) {
	return service.ResolveAccount(ctx, event)
}

func text(message string) core.OutboundMessage {
	return core.OutboundMessage{Text: message, Markdown: &core.MarkdownPayload{Content: message}, ReplyTo: "source"}
}

func withKeyboard(message core.OutboundMessage, rows ...[]core.KeyboardButton) core.OutboundMessage {
	message.Keyboard = &core.KeyboardPayload{Rows: rows}
	return message
}

func friendlyError(err error) (core.OutboundMessage, error) {
	if err == nil {
		return core.OutboundMessage{}, nil
	}
	known := []error{ErrPetRequired, ErrExpeditionActive, ErrExpeditionNotReady, ErrNothingToClaim, ErrInsufficientItem, ErrInvalidBindToken, ErrBindConflict}
	for _, candidate := range known {
		if errors.Is(err, candidate) {
			return text("暂时无法完成：" + err.Error() + "。\n发送“帮助”查看可用操作。"), nil
		}
	}
	return core.OutboundMessage{}, err
}

func handleAdoptList(_ context.Context, _ core.InboundEvent, _ *Service) (core.OutboundMessage, error) {
	return text("【可领养伙伴】\n1. 诺诺｜均衡探索\n2. 呱呱｜擅长守护\n3. 菀菀｜擅长支援\n\n发送“领养 诺诺”开始旅程。"), nil
}

func handleAdopt(ctx context.Context, event core.InboundEvent, service *Service) (core.OutboundMessage, error) {
	account, err := resolve(ctx, service, event)
	if err != nil {
		return core.OutboundMessage{}, err
	}
	petType := strings.TrimSpace(strings.TrimPrefix(event.Text, "领养"))
	allowed := map[string]bool{"诺诺": true, "呱呱": true, "菀菀": true}
	if !allowed[petType] {
		return text("没有找到这位伙伴。\n发送“领养宠物”查看可选伙伴。"), nil
	}
	if _, err = service.Adopt(ctx, account.ID, petType, petType); err != nil {
		if strings.Contains(err.Error(), "UNIQUE") {
			return text("你已经有同行伙伴了。\n发送“状态”查看它的近况。"), nil
		}
		return core.OutboundMessage{}, err
	}
	return text(fmt.Sprintf("【领养成功】\n%s 已成为你的同行伙伴。\n\n下一步：发送“今日”，或直接发送“远征 1”。", petType)), nil
}

func handleStatus(ctx context.Context, event core.InboundEvent, service *Service) (core.OutboundMessage, error) {
	account, err := resolve(ctx, service, event)
	if err != nil {
		return core.OutboundMessage{}, err
	}
	var pet models.PetProfile
	if err = service.DB.WithContext(ctx).First(&pet, "account_id = ?", account.ID).Error; errors.Is(err, gorm.ErrRecordNotFound) {
		return text("你还没有同行伙伴。\n发送“领养宠物”选择第一只宠物。"), nil
	} else if err != nil {
		return core.OutboundMessage{}, err
	}
	activity := "当前空闲，可发送“远征”选择行动。"
	if run, runErr := service.ActiveExpedition(ctx, account.ID); runErr == nil {
		if service.Now().Before(run.EndsAt) {
			activity = fmt.Sprintf("正在进行：%s", run.Name)
		} else {
			activity = fmt.Sprintf("%s 已完成，发送“领取”。", run.Name)
		}
	}
	trait := pet.Traits
	if trait == "" {
		trait = "尚在形成"
	}
	return text(fmt.Sprintf("【%s的近况】\n定位：%s｜姿态：%s\n性格：%s\n心情：%s｜准备度：%d/100\n羁绊：Lv.%d｜成长：%d\n%s", pet.Name, pet.Role, pet.Stance, trait, pet.Mood, pet.Readiness, pet.BondLevel, pet.Growth, activity)), nil
}

func handleDaily(ctx context.Context, event core.InboundEvent, service *Service) (core.OutboundMessage, error) {
	account, err := resolve(ctx, service, event)
	if err != nil {
		return core.OutboundMessage{}, err
	}
	var count int64
	if err = service.DB.WithContext(ctx).Model(&models.PetProfile{}).Where("account_id = ?", account.ID).Count(&count).Error; err != nil {
		return core.OutboundMessage{}, err
	}
	if count == 0 {
		return friendlyError(ErrPetRequired)
	}
	streak, rewarded, err := service.RecordDaily(ctx, account.ID, "陪伴")
	if err != nil {
		return core.OutboundMessage{}, err
	}
	if rewarded {
		return text(fmt.Sprintf("【今日陪伴日志】\n你和宠物认真打了招呼。\n滚动七日记录：%d/7\n获得：陪伴印记 ×1、羁绊经验 ×1\n\n今天推荐：发送“远征”选择行动。", streak)), nil
	}
	return text(fmt.Sprintf("今天已经记录过陪伴啦。\n滚动七日记录：%d/7\n发送“状态”或“远征”继续。", streak)), nil
}

func handleExpedition(ctx context.Context, event core.InboundEvent, service *Service) (core.OutboundMessage, error) {
	account, err := resolve(ctx, service, event)
	if err != nil {
		return core.OutboundMessage{}, err
	}
	argument := strings.TrimSpace(strings.TrimPrefix(event.Text, "远征"))
	if argument == "" {
		return withKeyboard(text("【远征委托】\n1. 林间巡查｜10分钟｜轻量调查\n2. 遗迹调查｜2小时｜常规探索\n3. 深层生态勘察｜8小时｜长期任务\n\n发送“远征 1/2/3”出发。"), []core.KeyboardButton{
			{Label: "林间巡查", Command: "远征 1"},
			{Label: "遗迹调查", Command: "远征 2"},
			{Label: "深层勘察", Command: "远征 3"},
		}), nil
	}
	tier, parseErr := strconv.Atoi(argument)
	if parseErr != nil || tier < 1 || tier > 3 {
		return text("远征档位无效。\n请发送“远征 1”“远征 2”或“远征 3”。"), nil
	}
	run, err := service.StartExpedition(ctx, account.ID, tier)
	if err != nil {
		return friendlyError(err)
	}
	return text(fmt.Sprintf("【%s已开始】\n预计返回：%s\n姿态：%s\n主要目标：%s\n奖励采用固定进度，不含抽奖。\n\n发送“远征状态”查看进度。", run.Name, run.EndsAt.Format("15:04"), run.Stance, run.RewardItem)), nil
}

func handleExpeditionStatus(ctx context.Context, event core.InboundEvent, service *Service) (core.OutboundMessage, error) {
	account, err := resolve(ctx, service, event)
	if err != nil {
		return core.OutboundMessage{}, err
	}
	status, err := service.FormatExpeditionStatus(ctx, account.ID)
	if err != nil {
		return friendlyError(err)
	}
	return text(status), nil
}

func handleClaim(ctx context.Context, event core.InboundEvent, service *Service) (core.OutboundMessage, error) {
	account, err := resolve(ctx, service, event)
	if err != nil {
		return core.OutboundMessage{}, err
	}
	result, err := service.ClaimExpedition(ctx, account.ID)
	if err != nil {
		return friendlyError(err)
	}
	return text(fmt.Sprintf("【%s完成】\n获得：%s ×%d、调查记录 ×%d\n宠物成长：+%d\n图鉴：%s %d%%\n\n发送“远征”选择下一次行动。", result.Name, result.Item, result.Quantity, result.Records, result.Growth, result.CodexEntry, result.Progress)), nil
}

func handleStance(ctx context.Context, event core.InboundEvent, service *Service) (core.OutboundMessage, error) {
	account, err := resolve(ctx, service, event)
	if err != nil {
		return core.OutboundMessage{}, err
	}
	stance := strings.TrimSpace(strings.TrimPrefix(event.Text, "编队"))
	if stance == "" {
		lines := []string{"【远征姿态】"}
		for _, configured := range gameplayrules.EnabledStances(service.DB.WithContext(ctx)) {
			lines = append(lines, configured.Name+"："+configured.Description)
		}
		lines = append(lines, "", "发送“编队 姿态名”切换。")
		return text(strings.Join(lines, "\n")), nil
	}
	if err = service.SetStance(ctx, account.ID, stance); err != nil {
		return text("暂时无法切换姿态：" + err.Error() + "。\n发送“编队”查看可选姿态。"), nil
	}
	return text("已切换为“" + stance + "”姿态，将在下一次远征生效。"), nil
}

func handleRole(ctx context.Context, event core.InboundEvent, service *Service) (core.OutboundMessage, error) {
	account, err := resolve(ctx, service, event)
	if err != nil {
		return core.OutboundMessage{}, err
	}
	role := strings.TrimSpace(strings.TrimPrefix(event.Text, "定位"))
	if role == "" {
		lines := []string{"【宠物定位】"}
		buttons := make([]core.KeyboardButton, 0)
		for _, configured := range gameplayrules.EnabledRoles(service.DB.WithContext(ctx)) {
			lines = append(lines, configured.Name+"："+configured.Description)
			buttons = append(buttons, core.KeyboardButton{Label: configured.Name, Command: "定位 " + configured.Name})
		}
		lines = append(lines, "", "发送“定位 定位名”选择。")
		return withKeyboard(text(strings.Join(lines, "\n")), buttons), nil
	}
	pet, roleErr := service.SetRole(ctx, account.ID, role)
	if roleErr != nil {
		return text("暂时无法更新定位：" + roleErr.Error() + "。\n发送“定位”查看可选定位。"), nil
	}
	return text(fmt.Sprintf("【定位已更新】\n%s｜技能：%s\n技能组固定透明，不通过抽取获得。", pet.Role, pet.Skills)), nil
}

func handleSkills(ctx context.Context, event core.InboundEvent, service *Service) (core.OutboundMessage, error) {
	account, err := resolve(ctx, service, event)
	if err != nil {
		return core.OutboundMessage{}, err
	}
	var pet models.PetProfile
	if err = service.DB.WithContext(ctx).First(&pet, "account_id = ?", account.ID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return friendlyError(ErrPetRequired)
		}
		return core.OutboundMessage{}, err
	}
	if pet.Skills == "" {
		return text("当前尚未配置技能组。\n发送“定位”选择职业定位。"), nil
	}
	return text(fmt.Sprintf("【%s技能组】\n%s\n\n技能由定位确定，可发送“定位”切换。", pet.Role, pet.Skills)), nil
}

func handleInventory(ctx context.Context, event core.InboundEvent, service *Service) (core.OutboundMessage, error) {
	account, err := resolve(ctx, service, event)
	if err != nil {
		return core.OutboundMessage{}, err
	}
	var items []models.GlobalInventoryItem
	if err = service.DB.WithContext(ctx).Where("account_id = ? AND quantity > 0", account.ID).Order("item_name").Limit(20).Find(&items).Error; err != nil {
		return core.OutboundMessage{}, err
	}
	if len(items) == 0 {
		return text("背包还是空的。\n完成远征后会获得固定材料与调查记录。"), nil
	}
	lines := []string{"【全局背包】"}
	for _, item := range items {
		lines = append(lines, fmt.Sprintf("- %s ×%d", item.ItemName, item.Quantity))
	}
	return text(strings.Join(lines, "\n")), nil
}

func handleCodex(ctx context.Context, event core.InboundEvent, service *Service) (core.OutboundMessage, error) {
	account, err := resolve(ctx, service, event)
	if err != nil {
		return core.OutboundMessage{}, err
	}
	var entries []models.CodexEntry
	if err = service.DB.WithContext(ctx).Where("account_id = ?", account.ID).Order("category, entry_key").Limit(20).Find(&entries).Error; err != nil {
		return core.OutboundMessage{}, err
	}
	if len(entries) == 0 {
		return text("图鉴尚未记录内容。\n完成第一次远征即可获得调查进度。"), nil
	}
	lines := []string{"【生态图鉴】"}
	for _, entry := range entries {
		lines = append(lines, fmt.Sprintf("- %s｜%s %d%%", entry.Category, entry.EntryKey, entry.Progress))
	}
	return text(strings.Join(lines, "\n")), nil
}

func handleCommunity(ctx context.Context, event core.InboundEvent, service *Service) (core.OutboundMessage, error) {
	account, err := resolve(ctx, service, event)
	if err != nil {
		return core.OutboundMessage{}, err
	}
	community, err := service.GetCommunity(ctx, event, account.ID)
	if err != nil {
		return core.OutboundMessage{}, err
	}
	return text(fmt.Sprintf("【社区栖息地】\n等级：Lv.%d\n建设材料：%d/下一阶段 %d\n\n发送“共建 木材 20”贡献材料。", community.Level, community.Materials, community.Level*100)), nil
}

func handleContribute(ctx context.Context, event core.InboundEvent, service *Service) (core.OutboundMessage, error) {
	account, err := resolve(ctx, service, event)
	if err != nil {
		return core.OutboundMessage{}, err
	}
	parts := strings.Fields(strings.TrimSpace(strings.TrimPrefix(event.Text, "共建")))
	if len(parts) != 2 {
		return text("共建格式不正确。\n请发送“共建 木材 20”。"), nil
	}
	quantity, parseErr := strconv.ParseInt(parts[1], 10, 64)
	if parseErr != nil || quantity <= 0 {
		return text("贡献数量需要是正整数。\n例如：“共建 木材 20”。"), nil
	}
	community, err := service.Contribute(ctx, event, account.ID, parts[0], quantity)
	if err != nil {
		return friendlyError(err)
	}
	return text(fmt.Sprintf("【共建完成】\n贡献：%s ×%d\n社区材料：%d\n社区等级：Lv.%d", parts[0], quantity, community.Materials, community.Level)), nil
}

func handleSquad(ctx context.Context, event core.InboundEvent, service *Service) (core.OutboundMessage, error) {
	account, err := resolve(ctx, service, event)
	if err != nil {
		return core.OutboundMessage{}, err
	}
	argument := strings.TrimSpace(strings.TrimPrefix(event.Text, "小队"))
	if strings.HasPrefix(argument, "创建 ") {
		squad, createErr := service.CreateSquad(ctx, event, account.ID, strings.TrimSpace(strings.TrimPrefix(argument, "创建 ")))
		if createErr != nil {
			return text("暂时无法创建小队：" + createErr.Error() + "。"), nil
		}
		return text(fmt.Sprintf("【远征小队创建成功】\n名称：%s\n人数：1/%d\n小队属于当前群或频道社区。", squad.Name, squad.MaxMembers)), nil
	}
	if strings.HasPrefix(argument, "加入 ") {
		name := strings.TrimSpace(strings.TrimPrefix(argument, "加入 "))
		if joinErr := service.JoinSquad(ctx, event, account.ID, name); joinErr != nil {
			return text("暂时无法加入小队：" + joinErr.Error() + "。"), nil
		}
		return text("已加入“" + name + "”远征小队。发送“小队”查看共享研究进度。"), nil
	}
	if argument == "列表" {
		var squads []models.ExpeditionSquad
		if err = service.DB.WithContext(ctx).Where("community_id = ?", communityID(event)).Order("created_at").Limit(20).Find(&squads).Error; err != nil {
			return core.OutboundMessage{}, err
		}
		if len(squads) == 0 {
			return text("当前社区还没有远征小队。\n发送“小队 创建 星光队”创建第一支小队。"), nil
		}
		lines := []string{"【当前社区远征小队】"}
		for _, squad := range squads {
			var count int64
			service.DB.WithContext(ctx).Model(&models.SquadMember{}).Where("squad_id = ?", squad.ID).Count(&count)
			lines = append(lines, fmt.Sprintf("- %s｜%d/%d", squad.Name, count, squad.MaxMembers))
		}
		lines = append(lines, "", "发送“小队 加入 名称”加入。")
		return text(strings.Join(lines, "\n")), nil
	}
	var member models.SquadMember
	if err = service.DB.WithContext(ctx).Where("community_id = ? AND account_id = ?", communityID(event), account.ID).First(&member).Error; errors.Is(err, gorm.ErrRecordNotFound) {
		return text("你还没有加入当前社区的小队。\n发送“小队 创建 星光队”创建小队。"), nil
	} else if err != nil {
		return core.OutboundMessage{}, err
	}
	var squad models.ExpeditionSquad
	if err = service.DB.WithContext(ctx).First(&squad, "id = ?", member.SquadID).Error; err != nil {
		return core.OutboundMessage{}, err
	}
	var members int64
	service.DB.WithContext(ctx).Model(&models.SquadMember{}).Where("squad_id = ?", squad.ID).Count(&members)
	return text(fmt.Sprintf("【%s】\n成员：%d/%d\n共享研究：%d\n小队玩法为异步协作，不设置强制考勤。", squad.Name, members, squad.MaxMembers, squad.Research)), nil
}

func handleBoss(ctx context.Context, event core.InboundEvent, service *Service) (core.OutboundMessage, error) {
	account, err := resolve(ctx, service, event)
	if err != nil {
		return core.OutboundMessage{}, err
	}
	argument := strings.TrimSpace(strings.TrimPrefix(event.Text, "首领"))
	if argument == "" {
		boss, bossErr := service.GetBoss(ctx, event, account.ID)
		if bossErr != nil {
			return core.OutboundMessage{}, bossErr
		}
		return text(fmt.Sprintf("【本周社区首领】\n%s｜调查进度 %d/%d\n周期：%s\n\n投入调查记录参与异步协作：\n“首领 支援 10”", boss.Name, boss.MaxHP-boss.CurrentHP, boss.MaxHP, boss.WeekKey)), nil
	}
	parts := strings.Fields(argument)
	if len(parts) != 2 || parts[0] != "支援" {
		return text("首领命令格式不正确。\n发送“首领”查看状态，或发送“首领 支援 10”。"), nil
	}
	records, parseErr := strconv.ParseInt(parts[1], 10, 64)
	if parseErr != nil {
		return text("支援数量需要是 1 到 50 的整数。"), nil
	}
	boss, damage, challengeErr := service.ChallengeBoss(ctx, event, account.ID, records)
	if challengeErr != nil {
		return text("暂时无法支援首领：" + challengeErr.Error() + "。\n发送“首领”查看当前状态。"), nil
	}
	status := fmt.Sprintf("剩余调查难度：%d/%d", boss.CurrentHP, boss.MaxHP)
	if boss.Defeated {
		status = "社区已完成本周首领调查，纪念进度永久保留。"
	}
	return text(fmt.Sprintf("【协作支援完成】\n投入：调查记录 ×%d\n贡献：%d\n%s", records, damage, status)), nil
}

func handleSeason(ctx context.Context, event core.InboundEvent, service *Service) (core.OutboundMessage, error) {
	account, err := resolve(ctx, service, event)
	if err != nil {
		return core.OutboundMessage{}, err
	}
	season := service.CurrentSeason()
	argument := strings.TrimSpace(strings.TrimPrefix(event.Text, "赛季"))
	if strings.HasPrefix(argument, "投票 ") {
		choice, parseErr := strconv.Atoi(strings.TrimSpace(strings.TrimPrefix(argument, "投票 ")))
		if parseErr != nil {
			return text("故事选择只能是 1、2 或 3。"), nil
		}
		if voteErr := service.VoteSeason(ctx, event, account.ID, choice); voteErr != nil {
			return text("暂时无法提交选择：" + voteErr.Error() + "。"), nil
		}
		return text(fmt.Sprintf("已选择：%s\n你可以在周期结束前修改选择；宠物、图鉴和收藏不会清零。", season.Choices[choice-1])), nil
	}
	return text(fmt.Sprintf("【%s｜%s】\n区域：%s\n结束时间：%s\n\n社区故事选择：\n1. %s\n2. %s\n3. %s\n\n回复“赛季 投票 1/2/3”。\n周期结束后永久图鉴与纪念记录不会清零。", season.Key, season.Name, season.Region, season.EndsAt.Format("2006-01-02"), season.Choices[0], season.Choices[1], season.Choices[2])), nil
}

func handleFacility(ctx context.Context, event core.InboundEvent, service *Service) (core.OutboundMessage, error) {
	account, err := resolve(ctx, service, event)
	if err != nil {
		return core.OutboundMessage{}, err
	}
	argument := strings.TrimSpace(strings.TrimPrefix(event.Text, "设施"))
	if strings.HasPrefix(argument, "升级 ") {
		name := strings.TrimSpace(strings.TrimPrefix(argument, "升级 "))
		facility, upgradeErr := service.UpgradeFacility(ctx, event, account.ID, name)
		if upgradeErr != nil {
			return text("暂时无法升级设施：" + upgradeErr.Error() + "。"), nil
		}
		return text(fmt.Sprintf("【设施升级完成】\n%s已升级到 Lv.%d。\n下一次升级需要 %d 社区建设材料。", facility.Name, facility.Level, facility.Level*100)), nil
	}
	facilities, err := service.GetFacilities(ctx, event, account.ID)
	if err != nil {
		return core.OutboundMessage{}, err
	}
	lines := []string{"【社区设施】"}
	for _, facility := range facilities {
		lines = append(lines, fmt.Sprintf("- %s Lv.%d", facility.Name, facility.Level))
	}
	lines = append(lines, "", "发送“设施 升级 研究站”等命令升级。")
	return text(strings.Join(lines, "\n")), nil
}

func handleHelpRequest(ctx context.Context, event core.InboundEvent, service *Service) (core.OutboundMessage, error) {
	account, err := resolve(ctx, service, event)
	if err != nil {
		return core.OutboundMessage{}, err
	}
	parts := strings.Fields(strings.TrimSpace(strings.TrimPrefix(event.Text, "求助")))
	if len(parts) != 2 {
		return text("求助格式不正确。\n请发送“求助 木材 5”，每条最多求助20件物品。"), nil
	}
	quantity, parseErr := strconv.ParseInt(parts[1], 10, 64)
	if parseErr != nil {
		return text("求助数量需要是 1 到 20 的整数。"), nil
	}
	request, requestErr := service.CreateHelpRequest(ctx, event, account.ID, parts[0], quantity)
	if requestErr != nil {
		return text("暂时无法发布求助：" + requestErr.Error() + "。"), nil
	}
	return text(fmt.Sprintf("【求助已发布】\n编号：%s\n需要：%s ×%d\n有效期：24小时\n\n其他成员可发送“支援 %s 1”。", request.Code, request.ItemName, request.Quantity, request.Code)), nil
}

func handleHelpList(ctx context.Context, event core.InboundEvent, service *Service) (core.OutboundMessage, error) {
	account, err := resolve(ctx, service, event)
	if err != nil {
		return core.OutboundMessage{}, err
	}
	if _, err = service.GetCommunity(ctx, event, account.ID); err != nil {
		return core.OutboundMessage{}, err
	}
	var requests []models.CommunityHelpRequest
	if err = service.DB.WithContext(ctx).Where("community_id = ? AND status = ? AND expires_at > ?", communityID(event), "open", service.Now()).Order("created_at").Limit(10).Find(&requests).Error; err != nil {
		return core.OutboundMessage{}, err
	}
	if len(requests) == 0 {
		return text("当前社区没有进行中的求助。\n发送“求助 木材 5”发布一条限额求助。"), nil
	}
	lines := []string{"【社区求助单】"}
	for _, request := range requests {
		lines = append(lines, fmt.Sprintf("- %s｜%s %d/%d", request.Code, request.ItemName, request.Fulfilled, request.Quantity))
	}
	lines = append(lines, "", "发送“支援 编号 数量”提供帮助；每天最多赠送20件。")
	return text(strings.Join(lines, "\n")), nil
}

func handleHelpSupport(ctx context.Context, event core.InboundEvent, service *Service) (core.OutboundMessage, error) {
	account, err := resolve(ctx, service, event)
	if err != nil {
		return core.OutboundMessage{}, err
	}
	parts := strings.Fields(strings.TrimSpace(strings.TrimPrefix(event.Text, "支援")))
	if len(parts) != 2 {
		return text("支援格式不正确。\n请发送“支援 ABC123 3”。"), nil
	}
	quantity, parseErr := strconv.ParseInt(parts[1], 10, 64)
	if parseErr != nil {
		return text("支援数量需要是正整数。"), nil
	}
	request, supportErr := service.SupportHelpRequest(ctx, event, account.ID, parts[0], quantity)
	if supportErr != nil {
		return text("暂时无法支援：" + supportErr.Error() + "。"), nil
	}
	return text(fmt.Sprintf("【支援完成】\n已送出：%s ×%d\n求助进度：%d/%d\n这是限额赠礼，不开放自由交易。", request.ItemName, quantity, request.Fulfilled, request.Quantity)), nil
}

func handleGenerateBind(ctx context.Context, event core.InboundEvent, service *Service) (core.OutboundMessage, error) {
	account, err := resolve(ctx, service, event)
	if err != nil {
		return core.OutboundMessage{}, err
	}
	token, err := service.GenerateBindToken(ctx, account.ID)
	if err != nil {
		return core.OutboundMessage{}, err
	}
	return text(fmt.Sprintf("【一次性绑定码】\n%s\n\n10分钟内在目标群或频道发送“绑定 %s”。\n请勿把绑定码转发给其他人。", token, token)), nil
}

func handleRedeemBind(ctx context.Context, event core.InboundEvent, service *Service) (core.OutboundMessage, error) {
	token := strings.TrimSpace(strings.TrimPrefix(event.Text, "绑定"))
	if token == "" {
		return text("请发送“绑定 ABC12345”。\n绑定码可通过“生成绑定码”获得，有效期10分钟。"), nil
	}
	if _, err := service.RedeemBindToken(ctx, event, token); err != nil {
		return friendlyError(err)
	}
	return text("身份绑定成功。你的宠物、背包、图鉴和长期成长现在跨场景共享。"), nil
}

func handleMyData(ctx context.Context, event core.InboundEvent, service *Service) (core.OutboundMessage, error) {
	account, err := resolve(ctx, service, event)
	if err != nil {
		return core.OutboundMessage{}, err
	}
	var identities int64
	var codex int64
	service.DB.WithContext(ctx).Model(&models.PlayerIdentity{}).Where("account_id = ?", account.ID).Count(&identities)
	service.DB.WithContext(ctx).Model(&models.CodexEntry{}).Where("account_id = ?", account.ID).Count(&codex)
	return text(fmt.Sprintf("【我的数据】\n内部账号：%s\n已绑定身份：%d\n图鉴条目：%d\n\n可用隐私命令：关闭通知、解绑身份、删除我的数据。", account.ID, identities, codex)), nil
}

func handleConfirmDelete(ctx context.Context, event core.InboundEvent, service *Service) (core.OutboundMessage, error) {
	account, err := resolve(ctx, service, event)
	if err != nil {
		return core.OutboundMessage{}, err
	}
	if err = service.DeleteAccount(ctx, account.ID); err != nil {
		return core.OutboundMessage{}, err
	}
	return text("你的宠物、背包、图鉴、身份绑定与社区成员数据已删除。再次互动会创建全新账号。"), nil
}

func handleDisableNotifications(ctx context.Context, event core.InboundEvent, service *Service) (core.OutboundMessage, error) {
	return setNotifications(ctx, event, service, false)
}

func handleEnableNotifications(ctx context.Context, event core.InboundEvent, service *Service) (core.OutboundMessage, error) {
	return setNotifications(ctx, event, service, true)
}

func setNotifications(ctx context.Context, event core.InboundEvent, service *Service, enabled bool) (core.OutboundMessage, error) {
	account, err := resolve(ctx, service, event)
	if err != nil {
		return core.OutboundMessage{}, err
	}
	if err = service.SetNotifications(ctx, account.ID, enabled); err != nil {
		return core.OutboundMessage{}, err
	}
	if enabled {
		return text("远征完成和社区里程碑通知已开启；平台额度不足时仍只在你主动查询后回复。"), nil
	}
	return text("主动通知已关闭。你仍可随时发送“远征状态”和“营地”查询。"), nil
}

func handleUnbind(ctx context.Context, event core.InboundEvent, service *Service) (core.OutboundMessage, error) {
	account, err := resolve(ctx, service, event)
	if err != nil {
		return core.OutboundMessage{}, err
	}
	if err = service.UnbindIdentity(ctx, event, account.ID); err != nil {
		return text("暂时无法解绑：" + err.Error() + "。"), nil
	}
	return text("当前场景身份已解绑，其他已绑定身份不受影响。"), nil
}

func handleHelp(_ context.Context, event core.InboundEvent, service *Service) (core.OutboundMessage, error) {
	topic := strings.TrimSpace(strings.TrimPrefix(event.Text, "帮助"))
	menuName := map[string]string{"": "主菜单", "今日": "今日与状态", "状态": "今日与状态", "远征": "远征指南", "成长": "成长与图鉴", "图鉴": "成长与图鉴", "陪伴": "陪伴互动", "营地": "营地与小队", "小队": "营地与小队", "账号": "账号与隐私", "隐私": "账号与隐私"}[topic]
	if menuName != "" && service != nil && service.DB != nil {
		var menu models.MenuConfig
		if err := service.DB.First(&menu, "name = ?", menuName).Error; err == nil && strings.TrimSpace(menu.Reply) != "" {
			return text(menu.Reply), nil
		}
	}
	commands := []string{"状态 / 今日", "远征 / 远征状态 / 领取", "编队 / 背包 / 图鉴", "营地 / 共建 / 小队", "生成绑定码 / 我的数据"}
	sort.Strings(commands)
	return text("【宠物远征生态】\n" + strings.Join(commands, "\n") + "\n\n所有玩法都可仅用文本完成。发送“帮助 远征”查看详细流程。"), nil
}

func handleFoster(_ context.Context, _ core.InboundEvent, _ *Service) (core.OutboundMessage, error) {
	return text("新版不会永久失去宠物。\n暂时不想同行时，可以让它休养；状态和长期成长都会保留。"), nil
}

func handleRetiredRiskyFeature(_ context.Context, _ core.InboundEvent, _ *Service) (core.OutboundMessage, error) {
	return text("该对抗或随机玩法已经下线。\n现在可以通过“远征”、社区共建和协作首领获得明确进度奖励。"), nil
}

func handleRetiredTimedFeature(_ context.Context, _ core.InboundEvent, _ *Service) (core.OutboundMessage, error) {
	return text("学习、锻炼和打工已经合并为自动结算的远征委托。\n发送“远征”选择10分钟、2小时或8小时任务。"), nil
}

func handleRetiredFishing(_ context.Context, _ core.InboundEvent, _ *Service) (core.OutboundMessage, error) {
	return text("旧钓鱼玩法已升级为水域远征与鱼类图鉴，不再反复抛竿、收竿。\n发送“远征”选择任务，完成后发送“图鉴 水域”查看发现。"), nil
}

func handleLegacyFamily(_ context.Context, _ core.InboundEvent, _ *Service) (core.OutboundMessage, error) {
	return text("家族已经升级为“远征小队 + 社区栖息地”。\n发送“小队 列表”寻找伙伴，发送“营地”或“共建 木材 20”参与全员建设。"), nil
}
