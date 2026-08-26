package expedition

import (
	"context"
	"errors"
	"log"
	"sort"
	"strings"

	"qq-pet-saas/core"
	"qq-pet-saas/database"
	"qq-pet-saas/playermsg"
)

type commandRegister func(string, core.UnifiedHandler) error
type featureRegister func(core.UnifiedFeature, core.UnifiedHandler) error
type commandHandler func(context.Context, core.InboundEvent, *Service) (core.OutboundMessage, error)

type commandDefinition struct {
	feature core.UnifiedFeature
	handler commandHandler
}

func init() {
	if err := RegisterCommandFeatures(core.RegisterUnifiedFeature, func() *Service { return NewService(database.DB) }); err != nil {
		panic(err)
	}
}

func feature(funcName, command, displayName, category, description string, sortOrder int, aliases ...string) core.UnifiedFeature {
	return core.UnifiedFeature{
		FuncName: funcName, DefaultCommand: command, Aliases: aliases, DisplayName: displayName,
		Category: category, Description: description, Enabled: true, SortOrder: sortOrder,
	}
}

func hiddenFeature(funcName, command, displayName, category, description string, sortOrder int, aliases ...string) core.UnifiedFeature {
	result := feature(funcName, command, displayName, category, description, sortOrder, aliases...)
	result.Hidden = true
	return result
}

func commandDefinitions() []commandDefinition {
	return []commandDefinition{
		{feature("pet_menu", "宠物菜单", "宠物菜单", "基础", "查看全部宠物功能和对应命令", 0, "菜单"), handleMenu},
		{feature("adopt_list", "领养宠物", "领养宠物", "基础", "查看并选择可领养的宠物", 10, "宠物领养"), handleAdoptList},
		{hiddenFeature("adopt", "领养", "确认领养", "基础", "领养指定宠物", 11), handleAdopt},
		{feature("pet_status", "我的宠物", "我的宠物", "基础", "查看宠物状态、属性和当前行动", 20, "状态", "宠物状态"), handleStatus},
		{feature("pet_list", "宠物列表", "宠物列表", "基础", "查看账号下全部调查伙伴", 21), handlePetList},
		{feature("switch_pet", "切换宠物", "切换宠物", "基础", "切换当前调查伙伴", 22), handleSwitchPet},
		{feature("daily_checkin", "签到", "每日签到", "基础", "完成每日签到并领取陪伴奖励", 30, "今日", "宠物签到"), handleDaily},
		{feature("inventory", "我的背包", "我的背包", "基础", "查看当前持有的物品", 40, "背包", "宠物背包"), handleInventory},
		{feature("rename_pet", "改名", "宠物改名", "基础", "为当前宠物修改名字", 45, "宠物改名"), handleRenamePet},
		{feature("treat", "治疗", "治疗宠物", "基础", "恢复受伤宠物的体力", 46, "宠物治疗"), handleTreatPet},
		{feature("recover_pet", "找回", "结束休养", "基础", "让休养中的宠物重新同行", 47, "宠物找回"), handleRecoverPet},
		{feature("foster", "放生", "宠物休养", "基础", "让宠物暂时休养并保留全部成长", 48), handleFoster},
		{hiddenFeature("confirm_foster", "确认放生", "确认宠物休养", "基础", "确认让指定宠物进入休养状态", 49), handleConfirmFoster},
		{feature("help", "帮助", "玩法帮助", "基础", "查看命令和玩法说明", 50), handleHelp},

		{feature("shop", "商店", "宠物商店", "物品", "查看可以使用货币购买的商品", 60, "宠物商店"), handleShop},
		{feature("affection_shop", "好感商店", "好感商店", "物品", "查看可以使用好感兑换的商品", 61), handleAffectionShop},
		{feature("shop_item", "查看商品", "商品详情", "物品", "查看商品价格、库存和图片", 62, "商品详情"), handleShopItem},
		{feature("item_detail", "查看物品", "物品详情", "物品", "查看物品效果、售价和图片", 63, "物品详情"), handleItemDetail},
		{hiddenFeature("buy", "购买", "购买商品", "物品", "购买指定商品和数量", 64), handleBuy},
		{hiddenFeature("sell", "出售", "出售物品", "物品", "出售指定物品和数量", 65), handleSell},
		{feature("use_item", "使用", "使用物品", "物品", "使用背包中的恢复或成长物品", 66, "使用物品"), handleUseItem},

		{feature("feed", "喂养", "喂养宠物", "陪伴", "使用食物照料宠物", 100, "宠物喂养"), handleCompanion},
		{feature("touch", "摸头", "摸摸宠物", "陪伴", "和宠物进行温柔互动", 110, "宠物摸头"), handleCompanion},
		{feature("walk", "散步", "带宠物散步", "陪伴", "和宠物一起外出散步", 120, "宠物散步"), handleCompanion},
		{feature("gift", "送礼", "给宠物送礼", "陪伴", "把背包中的礼物送给宠物", 130, "宠物送礼"), handleCompanion},
		{feature("wash", "洗澡", "给宠物洗澡", "陪伴", "帮助宠物恢复清爽状态", 140, "宠物洗澡"), handleCompanion},
		{feature("study", "学习", "宠物学习", "成长", "消耗智慧物品进行计时学习", 150, "宠物学习"), handleStartActivity},
		{hiddenFeature("finish_study", "完成学习", "完成宠物学习", "成长", "结算已经结束的学习", 151), handleFinishActivity},
		{feature("train", "锻炼", "宠物锻炼", "成长", "消耗力量物品进行计时锻炼", 160, "宠物锻炼"), handleStartActivity},
		{hiddenFeature("finish_train", "完成锻炼", "完成宠物锻炼", "成长", "结算已经结束的锻炼", 161), handleFinishActivity},
		{feature("fitness", "健身", "宠物健身", "成长", "消耗防御物品进行计时健身", 170, "宠物健身"), handleStartActivity},
		{hiddenFeature("finish_fitness", "完成健身", "完成宠物健身", "成长", "结算已经结束的健身", 171), handleFinishActivity},
		{feature("work", "打工", "宠物打工", "成长", "选择岗位进行计时打工", 180, "宠物打工"), handleStartActivity},
		{hiddenFeature("finish_work", "完成打工", "完成宠物打工", "成长", "结算已经结束的打工", 181), handleFinishActivity},
		{feature("evolution", "进化", "宠物进化", "成长", "预览下一形态和所需条件", 190, "宠物进化"), handleEvolutionPreview},
		{hiddenFeature("confirm_evolution", "确认进化", "确认宠物进化", "成长", "满足条件后确认进化", 191), handleEvolutionConfirm},
		{feature("awaken", "觉醒", "宠物觉醒", "成长", "预览觉醒形态和所需条件", 192, "宠物觉醒"), handleAwakenPreview},
		{hiddenFeature("confirm_awaken", "确认觉醒", "确认宠物觉醒", "成长", "满足条件并消耗物品后确认觉醒", 193), handleAwakenConfirm},

		{feature("expedition", "远征", "宠物远征", "远征", "选择短途、常规或深度远征", 200), handleExpedition},
		{feature("expedition_status", "远征状态", "远征状态", "远征", "查看进行中的远征与返回时间", 210), handleExpeditionStatus},
		{feature("expedition_claim", "领取", "领取远征奖励", "远征", "领取已完成远征的结算结果", 220), handleClaim},
		{feature("adventure_maps", "地图", "冒险地图", "远征", "查看大地图、区域解锁与探索度", 221), handleAdventureMaps},
		{feature("adventure_explore", "探索", "手动探索", "远征", "进入区域并触发怪物、地标或安全事件", 222), handleAdventureExplore},
		{feature("adventure_inventory", "远征背包", "远征背包", "远征", "查看旅途徽章、材料、装备和蓝图概览", 222), handleAdventureInventory},
		{feature("adventure_materials", "材料背包", "材料背包", "远征", "查看远征专属材料", 222), handleAdventureMaterials},
		{hiddenFeature("adventure_attack", "普攻", "战斗普攻", "远征", "在地图战斗中进行普通攻击", 223), handleAdventureCombatAction},
		{hiddenFeature("adventure_defend", "防御", "战斗防御", "远征", "降低地图战斗中下一次受到的伤害", 224), handleAdventureCombatAction},
		{hiddenFeature("adventure_skill", "战斗技能", "战斗技能", "远征", "在地图战斗中使用宠物技能", 225), handleAdventureCombatAction},
		{hiddenFeature("adventure_retreat", "撤退", "战斗撤退", "远征", "安全退出当前地图战斗", 226), handleAdventureCombatAction},
		{feature("equipment", "装备背包", "装备背包", "远征", "查看武器、防具和秘宝", 227, "装备"), handleEquipment},
		{hiddenFeature("equipment_equip", "穿戴", "穿戴装备", "远征", "为宠物穿戴一件装备", 228), handleEquip},
		{hiddenFeature("equipment_unequip", "卸下", "卸下装备", "远征", "卸下宠物当前装备", 229), handleUnequip},
		{feature("blueprints", "蓝图背包", "蓝图背包", "远征", "查看蓝图碎片和已解锁配方", 230, "蓝图"), handleBlueprints},
		{feature("adventure_shop", "远征商店", "远征商店", "远征", "使用旅途徽章购买远征物品、装备和蓝图", 230), handleAdventureShop},
		{hiddenFeature("adventure_purchase", "远征购买", "远征购买", "远征", "购买远征商店商品", 230), handleAdventurePurchase},
		{hiddenFeature("equipment_craft", "制造", "制造装备", "远征", "消耗材料制造已解锁装备", 231), handleCraftEquipment},
		{hiddenFeature("equipment_salvage", "分解装备", "分解装备", "远征", "预览装备分解确认", 232), handleSalvageEquipment},
		{hiddenFeature("equipment_salvage_confirm", "确认分解装备", "确认分解装备", "远征", "确认分解未穿戴且未锁定装备", 233), handleConfirmSalvageEquipment},
		{feature("adventure_boss", "地图首领", "地图首领", "远征", "查看和挑战群或频道共享的限时地图首领", 234), handleAdventureBoss},
		{feature("fishing", "钓鱼", "宠物钓鱼", "扩展", "查看垂钓成本、概率和保底", 230, "宠物钓鱼"), handleFishingMenu},
		{feature("fishing_cast", "抛竿", "抛竿", "扩展", "消耗金币开始一次垂钓", 231), handleCastFishing},
		{feature("fishing_claim", "收竿", "收竿", "扩展", "领取已经等待完成的垂钓收获", 232), handleClaimFishing},
		{feature("lottery", "抽奖", "幸运抽奖", "扩展", "查看公开概率并参与有保底的抽奖", 240, "宠物抽奖"), handleLottery},
		{feature("rock_paper_scissors", "猜拳", "宠物猜拳", "扩展", "参加等概率且有审计记录的猜拳", 250, "宠物猜拳", "战斗"), handleRockPaperScissors},
		{hiddenFeature("legacy_combat_guide", "偷袭", "对抗玩法说明", "扩展", "引导到公开规则的宠物猜拳", 251, "宠物偷袭", "回击", "宠物回击"), handleLegacyCombatGuide},
		{feature("trade", "宠物交易", "安全交易", "扩展", "发布物品与金币的托管交易", 260, "交易", "添加交易"), handleTrade},
		{feature("trade_list", "交易列表", "交易列表", "扩展", "查看当前可接受的托管交易", 261), handleTradeList},
		{feature("trade_accept", "接受交易", "接受交易", "扩展", "在统一事务中接受托管交易", 262, "同意交易"), handleAcceptTrade},
		{feature("trade_info", "交易信息", "交易信息", "扩展", "查看交易物品、价格与状态", 263), handleTradeInfo},
		{feature("trade_cancel", "取消交易", "取消交易", "扩展", "取消自己发布的交易并退回托管物品", 264, "删除交易", "确认取消", "拒绝交易"), handleCancelTrade},

		{feature("growth_role", "定位", "成长定位", "成长", "选择宠物长期成长定位", 300), handleRole},
		{feature("expedition_stance", "编队", "远征姿态", "成长", "调整探索、守护或支援姿态", 310), handleStance},
		{feature("skills", "技能", "宠物技能", "成长", "查看当前定位对应的技能", 320), handleSkills},
		{feature("codex", "图鉴", "探索图鉴", "成长", "查看区域生物、材料与事件进度", 330), handleCodex},

		{feature("community", "营地", "社区营地", "社区", "查看当前群或频道社区", 400), handleCommunity},
		{feature("community_contribute", "共建", "社区共建", "社区", "向社区设施贡献材料", 410), handleContribute},
		{feature("squad", "小队", "远征小队", "社区", "创建、加入或查看远征小队", 420), handleSquad},
		{feature("community_boss", "首领", "限时地图首领", "远征", "兼容入口：查看和挑战当前限时地图首领", 235), handleAdventureBoss},
		{feature("live_event", "活动", "限时活动", "社区", "查看活动进度并领取里程碑奖励", 435), handleEvent},
		{feature("season", "赛季", "社区赛季", "社区", "查看并参与社区故事选择", 440), handleSeason},
		{feature("facility", "设施", "社区设施", "社区", "查看或升级社区设施", 450), handleFacility},
		{feature("help_request", "求助", "发布求助", "社区", "发布社区物品求助", 460), handleHelpRequest},
		{feature("help_list", "求助列表", "社区求助列表", "社区", "查看当前社区求助", 470), handleHelpList},
		{feature("help_support", "支援", "支援求助", "社区", "响应其他成员的求助", 480), handleHelpSupport},

		{feature("bind_generate", "生成绑定码", "生成绑定码", "账号", "生成十分钟有效的一次性绑定码", 600), handleGenerateBind},
		{feature("bind_identity", "绑定", "绑定身份", "账号", "将当前平台身份绑定到已有账号", 610), handleRedeemBind},
		{feature("my_data", "我的数据", "我的数据", "账号", "查看账号、身份和隐私数据摘要", 620), handleMyData},
		{hiddenFeature("notifications_off", "关闭通知", "关闭通知", "账号", "关闭主动通知", 630), handleDisableNotifications},
		{hiddenFeature("notifications_on", "开启通知", "开启通知", "账号", "开启主动通知", 640), handleEnableNotifications},
		{hiddenFeature("unbind_identity", "解绑身份", "解绑身份", "账号", "解除当前平台身份绑定", 650), handleUnbind},
		{hiddenFeature("delete_account_confirm", "确认删除我的数据", "确认删除数据", "账号", "永久删除当前玩家数据", 660), handleConfirmDelete},
		{hiddenFeature("legacy_family", "创建家族", "家族玩法引导", "兼容", "引导到社区小队与共建玩法", 930,
			"加入家族", "注销家族", "退出家族", "我的家族", "家族列表", "家族成员", "踢出成员"), handleLegacyFamily},
	}
}

// CommandCatalog returns the single product command contract. Callers receive
// copies so aliases cannot mutate the live registration metadata.
func CommandCatalog() []core.UnifiedFeature {
	definitions := commandDefinitions()
	result := make([]core.UnifiedFeature, 0, len(definitions))
	for _, definition := range definitions {
		item := definition.feature
		item.Aliases = append([]string(nil), item.Aliases...)
		result = append(result, item)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].SortOrder < result[j].SortOrder })
	return result
}

func RegisterCommandFeatures(register featureRegister, serviceFactory func() *Service) error {
	for _, definition := range commandDefinitions() {
		current := definition
		handler := bindCommand(current, serviceFactory)
		if err := register(current.feature, handler); err != nil {
			return err
		}
	}
	return nil
}

// RegisterCommands expands the same catalog for isolated router tests.
func RegisterCommands(register commandRegister, serviceFactory func() *Service) error {
	for _, definition := range commandDefinitions() {
		handler := bindCommand(definition, serviceFactory)
		triggers := append([]string{definition.feature.DefaultCommand}, definition.feature.Aliases...)
		for _, trigger := range triggers {
			if err := register(trigger, handler); err != nil {
				return err
			}
		}
	}
	return nil
}

func bindCommand(definition commandDefinition, serviceFactory func() *Service) core.UnifiedHandler {
	return func(ctx context.Context, event core.InboundEvent) (core.OutboundMessage, error) {
		service := serviceFactory()
		if service == nil || service.DB == nil {
			return core.OutboundMessage{}, errors.New("游戏数据库尚未初始化")
		}
		event.Text = canonicalCommandText(event.Text, definition.feature)
		message, handlerErr := definition.handler(ctx, event, service)
		if handlerErr == nil {
			if message.MessageKey == "" {
				result := strings.TrimSpace(message.BusinessResult)
				if result == "" {
					result = "reply"
				}
				message.MessageKey = "command." + definition.feature.FuncName + "." + result
			}
			return message, nil
		}
		log.Printf("[命令] %s 处理失败: %v", definition.feature.FuncName, handlerErr)
		safe := text(playermsg.MustRender("system.temporarily_unavailable", map[string]string{"Command": definition.feature.DefaultCommand}))
		safe.BusinessResult = "not_completed"
		safe.TechnicalResult = "error"
		safe.MessageKey = "system.temporarily_unavailable"
		return safe, nil
	}
}

func canonicalCommandText(text string, feature core.UnifiedFeature) string {
	text = strings.TrimSpace(text)
	triggers := append([]string{feature.DefaultCommand}, feature.Aliases...)
	sort.Slice(triggers, func(i, j int) bool { return len(triggers[i]) > len(triggers[j]) })
	for _, trigger := range triggers {
		if strings.HasPrefix(text, trigger) {
			return feature.DefaultCommand + strings.TrimPrefix(text, trigger)
		}
	}
	return text
}
