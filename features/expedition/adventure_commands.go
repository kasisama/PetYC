package expedition

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"qq-pet-saas/core"
	"qq-pet-saas/models"
)

func handleAdventureMaps(ctx context.Context, event core.InboundEvent, service *Service) (core.OutboundMessage, error) {
	account, err := resolve(ctx, service, event)
	if err != nil {
		return core.OutboundMessage{}, err
	}
	maps, err := service.ListAdventureMaps(ctx, account.ID)
	if err != nil {
		return core.OutboundMessage{}, err
	}
	if len(maps) == 0 {
		return text("当前还没有发布可探索的大地图。"), nil
	}
	lines := []string{"🗺️【冒险地图】", "先手动探索并击败区域目标，之后才能派宠物挂机远征。", ""}
	buttons := make([]core.KeyboardButton, 0)
	for _, item := range maps {
		lines = append(lines, fmt.Sprintf("▌%s｜%s｜推荐 Lv.%d", item.Map.Name, item.Map.Region, item.Map.RecommendedLevel))
		for _, zone := range item.Zones {
			status := fmt.Sprintf("探索度 %d%%", zone.ExplorationPercent)
			if !zone.Unlocked {
				status = "未解锁：需要 " + strings.Join(zone.MissingPrerequisites, "、")
			} else if zone.ExpeditionUnlocked {
				status += "｜已开放远征"
			}
			lines = append(lines, fmt.Sprintf("  · %s｜推荐 Lv.%d｜%s", zone.Zone.Name, zone.Zone.RecommendedLevel, status))
			if zone.Unlocked {
				buttons = append(buttons, core.KeyboardButton{Label: "探索 " + zone.Zone.Name, Command: "探索 " + zone.Zone.Key})
			}
		}
	}
	return withKeyboard(text(strings.Join(lines, "\n")), buttons), nil
}

func findAdventureZone(ctx context.Context, service *Service, raw string) (models.AdventureZoneConfig, error) {
	var zone models.AdventureZoneConfig
	result := service.DB.WithContext(ctx).Where("enabled = ? AND (key = ? OR name = ?)", true, raw, raw).Limit(1).Find(&zone)
	if result.Error != nil {
		return zone, result.Error
	}
	if result.RowsAffected == 0 {
		return zone, ErrZoneLocked
	}
	return zone, nil
}

func handleAdventureExplore(ctx context.Context, event core.InboundEvent, service *Service) (core.OutboundMessage, error) {
	account, err := resolve(ctx, service, event)
	if err != nil {
		return core.OutboundMessage{}, err
	}
	argument := strings.TrimSpace(strings.TrimPrefix(event.Text, "探索"))
	if argument == "" {
		return handleAdventureMaps(ctx, event, service)
	}
	zone, err := findAdventureZone(ctx, service, argument)
	if err != nil {
		return adventureBusinessError(err)
	}
	result, err := service.ExploreZoneInCommunity(ctx, account.ID, communityID(event), zone.Key)
	if err != nil {
		return adventureBusinessError(err)
	}
	lines := []string{fmt.Sprintf("🧭【%s｜探索开始】", zone.Name), result.Encounter.Name, result.Encounter.Description}
	message := text(strings.Join(lines, "\n"))
	if result.Combat != nil {
		lines = append(lines, "", fmt.Sprintf("遭遇：%s｜敌方生命 %d", result.Encounter.Name, result.Combat.MonsterHealth), "请选择：普攻／防御／战斗技能 名称／撤退")
		message = withKeyboard(text(strings.Join(lines, "\n")), []core.KeyboardButton{{Label: "普攻", Command: "普攻"}, {Label: "防御", Command: "防御"}, {Label: "撤退", Command: "撤退"}})
	} else {
		lines = append(lines, "", fmt.Sprintf("区域探索度：%d%%", result.Progress.ExplorationPercent))
		message = withKeyboard(text(strings.Join(lines, "\n")), []core.KeyboardButton{{Label: "继续探索", Command: "探索 " + zone.Key}, {Label: "查看地图", Command: "地图"}})
	}
	message.Image = zone.Image
	return message, nil
}

func handleAdventureCombatAction(ctx context.Context, event core.InboundEvent, service *Service) (core.OutboundMessage, error) {
	account, err := resolve(ctx, service, event)
	if err != nil {
		return core.OutboundMessage{}, err
	}
	action := ""
	switch {
	case strings.HasPrefix(event.Text, "普攻"):
		action = "attack"
	case strings.HasPrefix(event.Text, "防御"):
		action = "defend"
	case strings.HasPrefix(event.Text, "撤退"):
		action = "retreat"
	case strings.HasPrefix(event.Text, "战斗技能"):
		key := strings.TrimSpace(strings.TrimPrefix(event.Text, "战斗技能"))
		if key != "" {
			action = "skill:" + key
		}
	}
	if action == "" {
		return text("战斗行动无效，请选择普攻、防御、战斗技能或撤退。"), nil
	}
	result, err := service.CombatAction(ctx, account.ID, riskSourceKey(event), action)
	if err != nil {
		return adventureBusinessError(err)
	}
	lines := []string{fmt.Sprintf("⚔️【第 %d 回合】", result.Turn.Round)}
	if result.Turn.PlayerDamage > 0 {
		lines = append(lines, fmt.Sprintf("宠物造成 %d 点伤害。", result.Turn.PlayerDamage))
	}
	if result.Turn.MonsterDamage > 0 {
		lines = append(lines, fmt.Sprintf("宠物受到 %d 点伤害。", result.Turn.MonsterDamage))
	}
	switch result.Turn.Result {
	case "victory":
		lines = append(lines, "🎉 战斗胜利！", fmt.Sprintf("获得冒险经验：%d｜当前 Lv.%d", result.AdventureXP, result.AdventureLevel))
		for _, reward := range result.Rewards {
			lines = append(lines, rewardText(reward))
		}
		if result.ExpeditionUnlocked {
			lines = append(lines, fmt.Sprintf("区域探索度：%d%%｜挂机远征已开放", result.ZoneProgress))
		}
		return withKeyboard(text(strings.Join(lines, "\n")), []core.KeyboardButton{{Label: "查看地图", Command: "地图"}, {Label: "远征", Command: "远征"}}), nil
	case "defeat":
		return text(strings.Join(append(lines, "宠物受伤了，本次探索结束。发送“治疗”恢复后再来挑战。"), "\n")), nil
	case "retreated":
		return text("已安全撤出战斗，本次探索结束。"), nil
	}
	lines = append(lines, fmt.Sprintf("我方生命 %d｜敌方生命 %d", result.Session.PlayerHealth, result.Session.MonsterHealth))
	return withKeyboard(text(strings.Join(lines, "\n")), []core.KeyboardButton{{Label: "普攻", Command: "普攻"}, {Label: "防御", Command: "防御"}, {Label: "撤退", Command: "撤退"}}), nil
}

func rewardText(reward AdventureReward) string {
	if reward.Type == "equipment" && reward.Equipment != nil {
		return fmt.Sprintf("获得装备：%s（%s）", reward.Name, reward.Equipment.ID)
	}
	return fmt.Sprintf("获得：%s ×%d", reward.Name, reward.Quantity)
}

func adventureBusinessError(err error) (core.OutboundMessage, error) {
	switch {
	case errors.Is(err, ErrZoneLocked):
		return text("该区域尚未解锁。发送“地图”查看需要先完成的区域或目标。"), nil
	case errors.Is(err, ErrAdventureInjured):
		return text("宠物正在受伤状态，请先发送“治疗”。"), nil
	case errors.Is(err, ErrAdventureBusy), errors.Is(err, ErrExpeditionActive):
		return text("宠物正在进行其他行动，暂时不能开始新的探索。"), nil
	case errors.Is(err, ErrNoEncounter):
		return text("该区域还没有配置可用的探索遭遇，请联系管理员。"), nil
	case errors.Is(err, ErrNoActiveCombat):
		return text("当前没有进行中的地图战斗。发送“地图”开始探索。"), nil
	case errors.Is(err, ErrCombatExpired):
		return text("本场战斗已经超时，宠物进入受伤状态，请先治疗。"), nil
	case errors.Is(err, ErrInvalidCombatAction):
		return text("这个行动当前不能使用，请选择普攻、防御或撤退。"), nil
	case errors.Is(err, ErrBossUnavailable):
		return text("这个地图首领当前没有出现，发送“地图首领”查看实时状态。"), nil
	case errors.Is(err, ErrBossChallengeLimit):
		return text("本次首领的挑战次数已经用完。"), nil
	case errors.Is(err, ErrBossChallengeCooldown):
		return text("地图首领挑战仍在冷却，请稍后再试。"), nil
	case errors.Is(err, ErrBossRewardUnavailable):
		return text("暂时没有可领取的地图首领奖励，或奖励已经领取。"), nil
	default:
		return expeditionBusinessError(err)
	}
}

func handleEquipment(ctx context.Context, event core.InboundEvent, service *Service) (core.OutboundMessage, error) {
	account, err := resolve(ctx, service, event)
	if err != nil {
		return core.OutboundMessage{}, err
	}
	var rows []models.PlayerEquipment
	if err = service.DB.WithContext(ctx).Where("account_id = ?", account.ID).Order("equipped_slot desc, created_at desc").Limit(50).Find(&rows).Error; err != nil {
		return core.OutboundMessage{}, err
	}
	if len(rows) == 0 {
		return text("🎒【装备背包】\n还没有装备。探索地图、挑战首领或制造装备都能获得武器、防具与秘宝。"), nil
	}
	lines := []string{"🎒【装备背包】"}
	buttons := []core.KeyboardButton{}
	for _, row := range rows {
		status := "未穿戴"
		if row.EquippedSlot != "" {
			status = "已穿戴 " + row.EquippedSlot
		}
		lines = append(lines, fmt.Sprintf("%s｜%s｜%s｜编号 %s", row.TemplateKey, row.Rarity, status, row.ID))
		if row.EquippedSlot == "" {
			buttons = append(buttons, core.KeyboardButton{Label: "穿戴 " + row.TemplateKey, Command: "穿戴 " + row.ID})
		}
	}
	return withKeyboard(text(strings.Join(lines, "\n")), buttons), nil
}

func handleEquip(ctx context.Context, event core.InboundEvent, service *Service) (core.OutboundMessage, error) {
	account, err := resolve(ctx, service, event)
	if err != nil {
		return core.OutboundMessage{}, err
	}
	id := strings.TrimSpace(strings.TrimPrefix(event.Text, "穿戴"))
	equipment, err := service.Equip(ctx, account.ID, id)
	if err != nil {
		return adventureBusinessError(err)
	}
	return text("已穿戴装备 " + equipment.TemplateKey + "。"), nil
}
func handleUnequip(ctx context.Context, event core.InboundEvent, service *Service) (core.OutboundMessage, error) {
	account, err := resolve(ctx, service, event)
	if err != nil {
		return core.OutboundMessage{}, err
	}
	id := strings.TrimSpace(strings.TrimPrefix(event.Text, "卸下"))
	if err = service.Unequip(ctx, account.ID, id); err != nil {
		return adventureBusinessError(err)
	}
	return text("装备已卸下。"), nil
}

func handleBlueprints(ctx context.Context, event core.InboundEvent, service *Service) (core.OutboundMessage, error) {
	account, err := resolve(ctx, service, event)
	if err != nil {
		return core.OutboundMessage{}, err
	}
	var rows []models.PlayerBlueprintProgress
	if err = service.DB.WithContext(ctx).Where("account_id = ?", account.ID).Order("equipment_key asc").Find(&rows).Error; err != nil {
		return core.OutboundMessage{}, err
	}
	if len(rows) == 0 {
		return text("📜【蓝图背包】\n还没有收集到装备蓝图碎片。"), nil
	}
	lines := []string{"📜【蓝图背包】"}
	for _, row := range rows {
		status := "收集中"
		if row.Unlocked {
			status = "已解锁"
		}
		lines = append(lines, fmt.Sprintf("%s｜%d片｜%s", row.EquipmentKey, row.Fragments, status))
	}
	return text(strings.Join(lines, "\n")), nil
}
func handleCraftEquipment(ctx context.Context, event core.InboundEvent, service *Service) (core.OutboundMessage, error) {
	account, err := resolve(ctx, service, event)
	if err != nil {
		return core.OutboundMessage{}, err
	}
	key := strings.TrimSpace(strings.TrimPrefix(event.Text, "制造"))
	equipment, err := service.CraftEquipment(ctx, account.ID, key)
	if err != nil {
		return adventureBusinessError(err)
	}
	return text(fmt.Sprintf("🔨 制造完成：%s｜装备编号 %s", equipment.TemplateKey, equipment.ID)), nil
}
func handleSalvageEquipment(ctx context.Context, event core.InboundEvent, service *Service) (core.OutboundMessage, error) {
	id := strings.TrimSpace(strings.TrimPrefix(event.Text, "分解装备"))
	return text("分解后无法恢复。确认请发送“确认分解装备 " + id + "”。"), nil
}
func handleConfirmSalvageEquipment(ctx context.Context, event core.InboundEvent, service *Service) (core.OutboundMessage, error) {
	account, err := resolve(ctx, service, event)
	if err != nil {
		return core.OutboundMessage{}, err
	}
	id := strings.TrimSpace(strings.TrimPrefix(event.Text, "确认分解装备"))
	if err = service.SalvageEquipment(ctx, account.ID, id); err != nil {
		return adventureBusinessError(err)
	}
	return text("装备已分解，材料已经放入背包。"), nil
}

func handleAdventureBoss(ctx context.Context, event core.InboundEvent, service *Service) (core.OutboundMessage, error) {
	account, err := resolve(ctx, service, event)
	if err != nil {
		return core.OutboundMessage{}, err
	}
	if event.SceneType == core.SceneDirect {
		return text("大型地图首领只在群或频道出现，请前往已加入的社群参与。"), nil
	}
	argument := strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(event.Text, "地图首领"), "首领"))
	community := communityID(event)
	if strings.HasPrefix(argument, "挑战 ") {
		key := strings.TrimSpace(strings.TrimPrefix(argument, "挑战 "))
		combat, err := service.StartAdventureBossChallenge(ctx, account.ID, community, key)
		if err != nil {
			return adventureBusinessError(err)
		}
		return withKeyboard(text(fmt.Sprintf("🐲【首领挑战开始】\n共享生命：%d\n请选择本回合行动。", combat.MonsterHealth)), []core.KeyboardButton{{Label: "普攻", Command: "普攻"}, {Label: "防御", Command: "防御"}, {Label: "撤退", Command: "撤退"}}), nil
	}
	if strings.HasPrefix(argument, "领取 ") {
		id := strings.TrimSpace(strings.TrimPrefix(argument, "领取 "))
		result, err := service.ClaimAdventureBossReward(ctx, account.ID, id)
		if err != nil {
			return adventureBusinessError(err)
		}
		lines := []string{"🎁【地图首领奖励】"}
		for _, reward := range result.Rewards {
			lines = append(lines, rewardText(reward))
		}
		return text(strings.Join(lines, "\n")), nil
	}
	bosses, err := service.ListActiveAdventureBosses(ctx, community)
	if err != nil {
		return adventureBusinessError(err)
	}
	if len(bosses) == 0 {
		return text("当前没有正在出现的地图首领。"), nil
	}
	lines := []string{"🐲【限时地图首领】"}
	buttons := []core.KeyboardButton{}
	for _, boss := range bosses {
		lines = append(lines, fmt.Sprintf("%s｜生命 %d/%d｜%s结束", boss.Config.Name, boss.Instance.CurrentHealth, boss.Instance.MaxHealth, boss.Instance.ExpiresAt.Format("15:04")))
		buttons = append(buttons, core.KeyboardButton{Label: "挑战 " + boss.Config.Name, Command: "地图首领 挑战 " + boss.Config.Key}, core.KeyboardButton{Label: "领取奖励", Command: "地图首领 领取 " + boss.Instance.ID})
	}
	return withKeyboard(text(strings.Join(lines, "\n")), buttons), nil
}

func hasAdventureExpeditions(ctx context.Context, service *Service) (bool, error) {
	var count int64
	err := service.DB.WithContext(ctx).Model(&models.AdventureExpeditionConfig{}).Where("enabled = ?", true).Count(&count).Error
	return count > 0, err
}

func handleConfiguredExpedition(ctx context.Context, event core.InboundEvent, service *Service, accountID, argument string) (core.OutboundMessage, error) {
	if argument == "" {
		var configs []models.AdventureExpeditionConfig
		if err := service.DB.WithContext(ctx).Where("enabled = ?", true).Order("zone_key asc").Find(&configs).Error; err != nil {
			return core.OutboundMessage{}, err
		}
		lines := []string{"🧭【区域远征】", "只有完成手动探索目标的区域才能派遣。", ""}
		buttons := []core.KeyboardButton{}
		for _, config := range configs {
			var zone models.AdventureZoneConfig
			if err := service.DB.WithContext(ctx).First(&zone, "key = ?", config.ZoneKey).Error; err != nil {
				continue
			}
			var progress models.PlayerZoneProgress
			_ = service.DB.WithContext(ctx).Limit(1).Find(&progress, "account_id = ? AND zone_key = ?", accountID, zone.Key).Error
			status := "未解锁"
			if progress.ExpeditionUnlocked {
				status = "可派遣"
				buttons = append(buttons, core.KeyboardButton{Label: zone.Name, Command: "远征 " + zone.Key})
			}
			lines = append(lines, fmt.Sprintf("%s｜%s｜%s｜活动积分 %d｜%s", zone.Name, expeditionDurationText(config.DurationMinutes), status, config.EventProgressPoints, config.Description))
		}
		return withKeyboard(text(strings.Join(lines, "\n")), buttons), nil
	}
	zone, err := findAdventureZone(ctx, service, argument)
	if err != nil {
		return adventureBusinessError(err)
	}
	run, err := service.StartAdventureExpeditionInCommunity(ctx, accountID, communityID(event), zone.Key)
	if err != nil {
		return adventureBusinessError(err)
	}
	var snapshot AdventureExpeditionSnapshot
	_ = json.Unmarshal([]byte(run.SnapshotJSON), &snapshot)
	lines := []string{fmt.Sprintf("🚩【%s已开始】", snapshot.Config.Name), fmt.Sprintf("区域：%s｜预计返回：%s", zone.Name, run.EndsAt.Format("01-02 15:04")), fmt.Sprintf("当前评价：%s｜战力 %d/%d", snapshot.Grade, snapshot.Power, snapshot.Config.RecommendedPower), "发送“远征状态”查看进度。"}
	message := text(strings.Join(lines, "\n"))
	message.Image = snapshot.Config.StartImage
	return message, nil
}

func configuredExpeditionStatus(ctx context.Context, service *Service, accountID string) (core.OutboundMessage, bool, error) {
	var run models.AdventureExpeditionRun
	lookup := service.DB.WithContext(ctx).Limit(1).Find(&run, "account_id = ? AND status = ?", accountID, "running")
	if lookup.Error != nil {
		return core.OutboundMessage{}, false, lookup.Error
	}
	if lookup.RowsAffected == 0 {
		return core.OutboundMessage{}, false, nil
	}
	var snapshot AdventureExpeditionSnapshot
	_ = json.Unmarshal([]byte(run.SnapshotJSON), &snapshot)
	remaining := run.EndsAt.Sub(service.Now())
	if remaining < 0 {
		remaining = 0
	}
	return text(fmt.Sprintf("🧭【%s】\n区域：%s\n评价：%s\n返回时间：%s\n剩余：%s", snapshot.Config.Name, snapshot.Zone.Name, snapshot.Grade, run.EndsAt.Format("01-02 15:04"), friendlyDuration(remaining))), true, nil
}

func claimConfiguredExpedition(ctx context.Context, service *Service, accountID string) (core.OutboundMessage, bool, error) {
	var count int64
	if err := service.DB.WithContext(ctx).Model(&models.AdventureExpeditionRun{}).Where("account_id = ? AND status = ?", accountID, "running").Count(&count).Error; err != nil {
		return core.OutboundMessage{}, false, err
	}
	if count == 0 {
		return core.OutboundMessage{}, false, nil
	}
	result, err := service.ClaimAdventureExpedition(ctx, accountID)
	if err != nil {
		message, handlerErr := adventureBusinessError(err)
		return message, true, handlerErr
	}
	lines := []string{fmt.Sprintf("🎊【%s完成】", result.Snapshot.Config.Name), fmt.Sprintf("远征评价：%s", result.Snapshot.Grade)}
	for _, reward := range result.Rewards {
		lines = append(lines, rewardText(reward))
	}
	lines = append(lines, fmt.Sprintf("冒险等级：Lv.%d", result.AdventureLevel))
	if result.EventProgress > 0 {
		lines = append(lines, fmt.Sprintf("活动进度：%d", result.EventProgress))
	}
	if result.Injured {
		lines = append(lines, "宠物在艰难的远征中受伤了，请先治疗。")
	}
	message := text(strings.Join(lines, "\n"))
	message.Image = result.Snapshot.Config.EndImage
	return message, true, nil
}
