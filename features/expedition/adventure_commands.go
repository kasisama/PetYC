package expedition

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"gorm.io/gorm"
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
	message, buttons := formatAdventureMapMessage(maps)
	return withKeyboardButtons(message, buttons, 3), nil
}

func formatAdventureMapMessage(maps []AdventureMapView) (core.OutboundMessage, []core.KeyboardButton) {
	plain := []string{"🗺️【冒险地图】", "先探索并完成区域目标，再派宠物挂机远征。", "发送“探索 区域名”开始探索。", ""}
	markdown := []string{"# 🗺️ 冒险地图", "", "先探索并完成区域目标，再派宠物挂机远征。", "发送“探索 区域名”开始探索。", ""}
	buttons := make([]core.KeyboardButton, 0)
	for _, item := range maps {
		unlocked := 0
		for _, zone := range item.Zones {
			if zone.Unlocked {
				unlocked++
				buttons = append(buttons, core.KeyboardButton{Label: "探索 " + zone.Zone.Name, Command: "探索 " + zone.Zone.Name})
			}
		}
		plain = append(plain, fmt.Sprintf("【%s】", item.Map.Name))
		markdown = append(markdown, "## "+item.Map.Name)
		meta := fmt.Sprintf("%s · 推荐 Lv.%d", item.Map.Region, item.Map.RecommendedLevel)
		if unlocked == 0 {
			hint := collapsedMapHint(item)
			plain = append(plain, meta+" · "+hint, "")
			markdown = append(markdown, meta+" · "+hint, "")
			continue
		}
		plain = append(plain, meta)
		markdown = append(markdown, meta, "")
		for _, zone := range item.Zones {
			status := adventureZoneStatus(zone)
			plain = append(plain, fmt.Sprintf("· %s  Lv.%d  %s", zone.Zone.Name, zone.Zone.RecommendedLevel, status))
			if zone.Unlocked {
				markdown = append(markdown, fmt.Sprintf("- **%s**  ·  Lv.%d  ·  %s", zone.Zone.Name, zone.Zone.RecommendedLevel, status))
			} else {
				markdown = append(markdown, fmt.Sprintf("- %s  ·  Lv.%d  ·  %s", zone.Zone.Name, zone.Zone.RecommendedLevel, status))
			}
		}
		plain = append(plain, "")
		markdown = append(markdown, "")
	}
	return menuText(strings.TrimSpace(strings.Join(plain, "\n")), strings.TrimSpace(strings.Join(markdown, "\n"))), buttons
}

func adventureZoneStatus(zone AdventureZoneView) string {
	if !zone.Unlocked {
		if len(zone.MissingPrerequisites) == 0 {
			return "锁定"
		}
		return "需先完成" + strings.Join(zone.MissingPrerequisites, "、")
	}
	status := fmt.Sprintf("探索度 %d%%", zone.ExplorationPercent)
	if strings.TrimSpace(zone.StageName) != "" {
		status = fmt.Sprintf("%s · %s · %d%%", zone.StageName, zone.CurrentGoal, zone.ExplorationPercent)
	}
	if zone.ExpeditionUnlocked {
		status += " · 可远征"
	}
	return status
}

func collapsedMapHint(item AdventureMapView) string {
	for _, zone := range item.Zones {
		if len(zone.MissingPrerequisites) > 0 {
			return "需先完成" + zone.MissingPrerequisites[0]
		}
	}
	return "尚未开放"
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

type adventureCombatView struct {
	Title          string
	Description    string
	PlayerName     string
	MonsterName    string
	PlayerHP       int64
	MonsterHP      int64
	PlayerDamage   int64
	MonsterDamage  int64
	PlayerDefended bool
	Outcome        string
	ExtraLines     []string
	SkillNames     []string
	SkillCooldowns map[string]int
}

func formatAdventureCombatMessage(view adventureCombatView) core.OutboundMessage {
	player := strings.TrimSpace(view.PlayerName)
	if player == "" {
		player = "我方"
	}
	monster := strings.TrimSpace(view.MonsterName)
	if monster == "" {
		monster = "敌方"
	}
	title := strings.TrimSpace(view.Title)
	if title == "" {
		title = "战斗"
	}
	plain := []string{"⚔️【" + title + "】"}
	markdown := []string{"# " + title}
	if description := strings.TrimSpace(view.Description); description != "" {
		plain = append(plain, "", description)
		markdown = append(markdown, "", description)
	}
	plain = append(plain, "", player+"  vs  "+monster, fmt.Sprintf("生命 %d  ·  生命 %d", view.PlayerHP, view.MonsterHP), "")
	markdown = append(markdown, "", fmt.Sprintf("**%s**  vs  **%s**", player, monster), fmt.Sprintf("生命 %d  ·  生命 %d", view.PlayerHP, view.MonsterHP), "")
	if view.PlayerDefended {
		plain = append(plain, player+" 进入防御")
		markdown = append(markdown, player+" 进入防御")
	}
	if view.PlayerDamage > 0 {
		plain = append(plain, fmt.Sprintf("%s 对 %s 造成 %d 点伤害", player, monster, view.PlayerDamage))
		markdown = append(markdown, fmt.Sprintf("%s 对 %s 造成 **%d** 点伤害", player, monster, view.PlayerDamage))
	}
	if view.MonsterDamage > 0 {
		plain = append(plain, fmt.Sprintf("%s 对 %s 造成 %d 点伤害", monster, player, view.MonsterDamage))
		markdown = append(markdown, fmt.Sprintf("%s 对 %s 造成 **%d** 点伤害", monster, player, view.MonsterDamage))
	}
	switch view.Outcome {
	case "victory":
		plain = append(plain, player+" 击败了 "+monster)
		markdown = append(markdown, player+" 击败了 "+monster)
	case "defeat":
		plain = append(plain, player+" 受伤了，本次探索结束。发送“治疗”恢复后再来挑战。")
		markdown = append(markdown, player+" 受伤了，本次探索结束。发送“治疗”恢复后再来挑战。")
	case "retreated":
		plain = append(plain, "已安全撤出战斗，本次探索结束。")
		markdown = append(markdown, "已安全撤出战斗，本次探索结束。")
	}
	for _, line := range view.ExtraLines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		plain = append(plain, line)
		markdown = append(markdown, line)
	}
	if view.Outcome == "" {
		plain = append(plain, "", "发送“普攻”、“防御”或“撤退”。")
		markdown = append(markdown, "", "发送“普攻”、“防御”或“撤退”。")
		if len(view.SkillNames) > 0 {
			plain = append(plain, "战斗技能："+strings.Join(view.SkillNames, "、"), "例如：发送“战斗技能 "+view.SkillNames[0]+"”。")
			markdown = append(markdown, "战斗技能："+strings.Join(view.SkillNames, "、"), "例如：发送“战斗技能 "+view.SkillNames[0]+"”。")
		}
		if len(view.SkillCooldowns) > 0 {
			cooldowns := make([]string, 0, len(view.SkillCooldowns))
			for _, name := range view.SkillNames {
				if turns := view.SkillCooldowns[name]; turns > 0 {
					cooldowns = append(cooldowns, fmt.Sprintf("%s（还需 %d 回合）", name, turns))
				}
			}
			if len(cooldowns) > 0 {
				line := "技能冷却：" + strings.Join(cooldowns, "、")
				plain = append(plain, line)
				markdown = append(markdown, line)
			}
		}
	}
	return menuText(strings.TrimSpace(strings.Join(plain, "\n")), strings.TrimSpace(strings.Join(markdown, "\n")))
}

func combatSkillNames(ctx context.Context, service *Service, petID string) []string {
	if service == nil || service.DB == nil || strings.TrimSpace(petID) == "" {
		return nil
	}
	var pet models.PetProfile
	lookup := service.DB.WithContext(ctx).Select("skills").Limit(1).Find(&pet, "id = ?", petID)
	if lookup.Error != nil || lookup.RowsAffected == 0 {
		return nil
	}
	var values []string
	if json.Unmarshal([]byte(pet.Skills), &values) != nil {
		values = strings.FieldsFunc(pet.Skills, func(r rune) bool { return r == ',' || r == '，' || r == '#' || r == '/' })
	}
	names := make([]string, 0, len(values))
	for _, raw := range values {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		var skill models.AdventureSkillConfig
		result := service.DB.WithContext(ctx).Select("name").Limit(1).Find(&skill, "enabled = ? AND (key = ? OR name = ?)", true, raw, raw)
		if result.Error == nil && result.RowsAffected > 0 && strings.TrimSpace(skill.Name) != "" {
			names = append(names, skill.Name)
		}
	}
	return names
}

func combatSkillCooldowns(ctx context.Context, service *Service, petID, raw string) map[string]int {
	if service == nil || service.DB == nil || strings.TrimSpace(petID) == "" || strings.TrimSpace(raw) == "" {
		return nil
	}
	values := map[string]int{}
	if json.Unmarshal([]byte(raw), &values) != nil {
		return nil
	}
	result := make(map[string]int, len(values))
	for key, turns := range values {
		if turns <= 0 {
			continue
		}
		var skill models.AdventureSkillConfig
		lookup := service.DB.WithContext(ctx).Select("name").Limit(1).Find(&skill, "key = ? AND enabled = ?", key, true)
		if lookup.Error == nil && lookup.RowsAffected > 0 && strings.TrimSpace(skill.Name) != "" {
			result[skill.Name] = turns
		}
	}
	return result
}

func combatActionKeyboard(message core.OutboundMessage, skillNames []string) core.OutboundMessage {
	rows := [][]core.KeyboardButton{{
		{Label: "普攻", Command: "普攻"},
		{Label: "防御", Command: "防御"},
		{Label: "撤退", Command: "撤退"},
	}}
	if len(skillNames) > 6 {
		skillNames = skillNames[:6]
	}
	skillButtons := make([]core.KeyboardButton, 0, len(skillNames))
	for _, name := range skillNames {
		skillButtons = append(skillButtons, core.KeyboardButton{Label: "战斗技能 " + name, Command: "战斗技能 " + name})
	}
	rows = append(rows, chunkKeyboardButtons(skillButtons, 2)...)
	return withKeyboard(message, rows...)
}

func resolveCombatSkillKey(db *gorm.DB, raw string) (string, bool) {
	if db == nil || strings.TrimSpace(raw) == "" {
		return "", false
	}
	var skill models.AdventureSkillConfig
	result := db.Limit(1).Find(&skill, "enabled = ? AND (key = ? OR name = ?)", true, strings.TrimSpace(raw), strings.TrimSpace(raw))
	if result.Error != nil || result.RowsAffected == 0 {
		return "", false
	}
	return skill.Key, true
}

func combatActorNames(ctx context.Context, service *Service, petID, monsterKey string) (string, string) {
	playerName, monsterName := "我方", "敌方"
	if petID != "" {
		var pet models.PetProfile
		if err := service.DB.WithContext(ctx).Select("name, current_form").Limit(1).Find(&pet, "id = ?", petID).Error; err == nil {
			if name := strings.TrimSpace(pet.Name); name != "" {
				playerName = name
			} else if form := strings.TrimSpace(pet.CurrentForm); form != "" {
				playerName = form
			}
		}
	}
	if monsterKey != "" {
		var monster models.AdventureMonsterConfig
		if err := service.DB.WithContext(ctx).Select("name").Limit(1).Find(&monster, "key = ?", monsterKey).Error; err == nil && strings.TrimSpace(monster.Name) != "" {
			monsterName = monster.Name
		}
	}
	return playerName, monsterName
}

func combatTitle(ctx context.Context, service *Service, result *AdventureCombatResult) string {
	place := "战斗"
	if result.Session.ExplorationID != "" {
		var exploration models.AdventureExplorationSession
		if err := service.DB.WithContext(ctx).Select("zone_key").Limit(1).Find(&exploration, "id = ?", result.Session.ExplorationID).Error; err == nil && exploration.ZoneKey != "" {
			var zone models.AdventureZoneConfig
			if err := service.DB.WithContext(ctx).Select("name").Limit(1).Find(&zone, "key = ?", exploration.ZoneKey).Error; err == nil && zone.Name != "" {
				place = zone.Name
			}
		}
	} else if result.Session.BossInstanceID != "" {
		place = "地图首领"
	}
	switch result.Turn.Result {
	case "victory":
		return place + " · 胜利"
	case "defeat":
		return place + " · 战败"
	case "retreated":
		return place + " · 撤退"
	default:
		return fmt.Sprintf("%s · 第 %d 回合", place, result.Turn.Round)
	}
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
		if errors.Is(err, ErrAdventureBusy) {
			if message, busy := petBusyMessage(ctx, service, account.ID); busy {
				return message, nil
			}
		}
		return adventureBusinessError(err)
	}
	if result.Event != nil {
		return formatAdventureStoryEvent(zone, *result.Event), nil
	}
	lines := []string{fmt.Sprintf("🧭【%s｜探索开始】", zone.Name), result.Encounter.Name, result.Encounter.Description}
	if strings.TrimSpace(result.Goal) != "" {
		lines = append(lines, "", "当前目标："+result.Goal)
	}
	message := text(strings.Join(lines, "\n"))
	if result.Combat != nil {
		playerName, monsterName := combatActorNames(ctx, service, result.Combat.PetID, result.Combat.MonsterKey)
		skillNames := combatSkillNames(ctx, service, result.Combat.PetID)
		if strings.TrimSpace(result.Encounter.Name) != "" {
			monsterName = result.Encounter.Name
		}
		message = formatAdventureCombatMessage(adventureCombatView{
			Title: zone.Name + " · 遭遇", Description: result.Encounter.Description,
			PlayerName: playerName, MonsterName: monsterName,
			PlayerHP: result.Combat.PlayerHealth, MonsterHP: result.Combat.MonsterHealth,
			SkillNames: skillNames,
		})
		message = combatActionKeyboard(message, skillNames)
	} else {
		lines = append(lines, "", fmt.Sprintf("区域探索度：%d%%", result.Progress.ExplorationPercent))
		if result.Encounter.NodeRole == "side" {
			lines = append(lines, "本次是支线遭遇，获得收获但未推进主线线索。")
		} else if result.Encounter.NodeRole == "mainline" {
			lines = append(lines, "获得区域线索，探索主线正在推进。")
		}
		message = withKeyboard(text(strings.Join(lines, "\n")), []core.KeyboardButton{{Label: "继续探索", Command: "探索 " + zone.Name}, {Label: "查看地图", Command: "地图"}})
	}
	message.Image = zone.Image
	return message, nil
}

func formatAdventureStoryEvent(zone models.AdventureZoneConfig, event AdventureStoryEventView) core.OutboundMessage {
	lines := []string{fmt.Sprintf("✨【%s｜%s】", zone.Name, event.Event.Name), event.Event.Description, "", "请选择行动："}
	buttons := make([]core.KeyboardButton, 0, len(event.Choices))
	for index, choice := range event.Choices {
		line := "· " + choice.Label
		if strings.TrimSpace(choice.Description) != "" {
			line += "：" + choice.Description
		}
		lines = append(lines, line)
		buttons = append(buttons, core.KeyboardButton{Label: choice.Label, Command: eventChoiceCommand(index + 1)})
	}
	return withKeyboard(text(strings.Join(lines, "\n")), chunkKeyboardButtons(buttons, 1)...)
}

func handleAdventureEventChoice(ctx context.Context, event core.InboundEvent, service *Service) (core.OutboundMessage, error) {
	account, err := resolve(ctx, service, event)
	if err != nil {
		return core.OutboundMessage{}, err
	}
	choiceKey := strings.TrimSpace(strings.TrimPrefix(event.Text, "探索选择"))
	if choiceKey == "" {
		return text("请选择当前事件提供的行动按钮。"), nil
	}
	result, err := service.ResolveNodeEvent(ctx, account.ID, choiceKey)
	if err != nil {
		return adventureBusinessError(err)
	}
	lines := []string{fmt.Sprintf("✨【%s】", result.Event.Name), "你选择了：" + result.Choice.Label}
	if strings.TrimSpace(result.Choice.Description) != "" {
		lines = append(lines, result.Choice.Description)
	}
	if result.Progress.ExpeditionUnlocked {
		lines = append(lines, "", "区域探索已完成，挂机远征已开放。")
		return withKeyboard(text(strings.Join(lines, "\n")), []core.KeyboardButton{{Label: "远征", Command: "远征"}, {Label: "查看地图", Command: "地图"}}), nil
	}
	lines = append(lines, "", fmt.Sprintf("探索度：%d%%｜下一阶段：%s", result.Progress.ExplorationPercent, result.Stage.Name))
	return withKeyboard(text(strings.Join(lines, "\n")), []core.KeyboardButton{{Label: "继续探索", Command: "探索 " + zoneName(ctx, service, result.Progress.ZoneKey)}, {Label: "查看地图", Command: "地图"}}), nil
}

func zoneName(ctx context.Context, service *Service, zoneKey string) string {
	var zone models.AdventureZoneConfig
	if service != nil && service.DB != nil {
		_ = service.DB.WithContext(ctx).Select("name").Limit(1).Find(&zone, "key = ?", zoneKey).Error
	}
	if strings.TrimSpace(zone.Name) != "" {
		return zone.Name
	}
	return zoneKey
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
		rawSkill := strings.TrimSpace(strings.TrimPrefix(event.Text, "战斗技能"))
		key, ok := resolveCombatSkillKey(service.DB.WithContext(ctx), rawSkill)
		if ok {
			action = "skill:" + key
		} else {
			return text("没有找到这个战斗技能。发送“技能”查看已解锁技能，或选择普攻、防御、撤退。"), nil
		}
	}
	if action == "" {
		return text("战斗行动无效，请选择普攻、防御、战斗技能或撤退。"), nil
	}
	result, err := service.CombatAction(ctx, account.ID, riskSourceKey(event), action)
	if err != nil {
		return adventureBusinessError(err)
	}
	playerName, monsterName := combatActorNames(ctx, service, result.Session.PetID, result.Session.MonsterKey)
	view := adventureCombatView{
		Title: combatTitle(ctx, service, result), PlayerName: playerName, MonsterName: monsterName,
		PlayerHP: result.Session.PlayerHealth, MonsterHP: result.Session.MonsterHealth,
		PlayerDamage: result.Turn.PlayerDamage, MonsterDamage: result.Turn.MonsterDamage,
		PlayerDefended: result.Turn.PlayerAction == "defend", Outcome: result.Turn.Result,
		SkillNames:     combatSkillNames(ctx, service, result.Session.PetID),
		SkillCooldowns: combatSkillCooldowns(ctx, service, result.Session.PetID, result.Session.CooldownsJSON),
	}
	if result.Turn.Result == "ongoing" {
		view.Outcome = ""
	}
	if result.Turn.Result == "victory" {
		view.ExtraLines = append(view.ExtraLines, fmt.Sprintf("获得冒险经验 %d · 当前 Lv.%d", result.AdventureXP, result.AdventureLevel))
		for _, reward := range result.Rewards {
			view.ExtraLines = append(view.ExtraLines, rewardText(reward))
		}
		if result.ExpeditionUnlocked {
			view.ExtraLines = append(view.ExtraLines, fmt.Sprintf("区域探索度 %d%% · 挂机远征已开放", result.ZoneProgress))
		}
	}
	message := formatAdventureCombatMessage(view)
	switch result.Turn.Result {
	case "victory":
		return withKeyboard(message, []core.KeyboardButton{{Label: "查看地图", Command: "地图"}, {Label: "远征", Command: "远征"}}), nil
	case "defeat", "retreated":
		return message, nil
	default:
		return combatActionKeyboard(message, view.SkillNames), nil
	}
}

func rewardText(reward AdventureReward) string {
	name := strings.TrimSpace(reward.Name)
	if name == "" {
		name = reward.Key
	}
	switch reward.Type {
	case "currency":
		return fmt.Sprintf("💰 %s +%d", name, reward.Quantity)
	case "equipment":
		return "🗡️ 获得装备：" + name
	case "blueprint_fragment":
		return fmt.Sprintf("📜 获得蓝图碎片：%s ×%d", name, reward.Quantity)
	case "item":
		return fmt.Sprintf("📦 获得物品：%s ×%d", name, reward.Quantity)
	default:
		return fmt.Sprintf("🎁 获得：%s ×%d", name, reward.Quantity)
	}
}

func adventureBusinessError(err error) (core.OutboundMessage, error) {
	var cooldown *CombatSkillCooldownError
	var levelTooLow *EquipmentLevelTooLowError
	switch {
	case errors.As(err, &cooldown):
		return text(fmt.Sprintf("“%s”仍在冷却，还需等待 %d 回合。\n本回合可以选择普攻、防御或其他未冷却技能。", cooldown.SkillName, cooldown.RemainingTurns)), nil
	case errors.Is(err, ErrZoneLocked):
		return text("该区域尚未解锁。发送“地图”查看需要先完成的区域或目标。"), nil
	case errors.Is(err, ErrAdventureInjured):
		return text("宠物正在受伤状态，请先发送“治疗”。"), nil
	case errors.Is(err, ErrAdventureBusy), errors.Is(err, ErrExpeditionActive):
		return text("宠物正在进行其他行动，暂时不能开始新的探索。"), nil
	case errors.Is(err, ErrNoEncounter):
		return text("该区域还没有配置可用的探索遭遇，请联系管理员。"), nil
	case errors.Is(err, ErrNoPendingAdventureEvent):
		return text("当前没有待处理的探索事件。发送“探索 区域名”继续调查。"), nil
	case errors.Is(err, ErrInvalidAdventureEventChoice):
		return text("请选择当前事件提供的行动按钮。"), nil
	case errors.Is(err, ErrNoActiveCombat):
		return text("当前没有进行中的地图战斗。发送“地图”开始探索。"), nil
	case errors.Is(err, ErrCombatExpired):
		return text("⛺【安全撤离】\n你离开得有些久，本场战斗已自动结束。宠物没有受伤，也不会获得本场奖励。\n\n发送“地图”重新开始探索。"), nil
	case errors.Is(err, ErrInvalidCombatAction):
		return text("这个战斗技能当前不可用，可能尚未装备或已被替换。发送“技能”查看已解锁技能，或选择普攻、防御、撤退。"), nil
	case errors.Is(err, ErrBossUnavailable):
		return text("这个地图首领当前没有出现，发送“地图首领”查看实时状态。"), nil
	case errors.Is(err, ErrBossChallengeLimit):
		return text("本次首领的挑战次数已经用完。"), nil
	case errors.Is(err, ErrBossChallengeCooldown):
		return text("地图首领挑战仍在冷却，请稍后再试。"), nil
	case errors.Is(err, ErrBossRewardUnavailable):
		return text("暂时没有可领取的地图首领奖励，或奖励已经领取。"), nil
	case errors.Is(err, ErrEquipmentNotFound):
		return text("没有找到这件装备。发送“装备背包”查看可穿戴的装备。"), nil
	case errors.Is(err, ErrEquipmentLocked):
		return text("这件装备已锁定，先解锁再分解。"), nil
	case errors.Is(err, ErrEquipmentEquipped):
		return text("请先卸下装备再分解。"), nil
	case errors.Is(err, ErrRecipeLocked):
		return text("这件装备的蓝图还没解锁。发送“蓝图”查看进度。"), nil
	case errors.As(err, &levelTooLow):
		return text(fmt.Sprintf("这件装备需要冒险等级 %d 才能穿戴，当前等级 %d。继续探索提升等级后再试。", levelTooLow.RequiredLevel, levelTooLow.CurrentLevel)), nil
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
	templates := equipmentTemplatesByKey(ctx, service.DB, equipmentTemplateKeys(rows))
	level := 1
	var progress models.PlayerAdventureProgress
	if err = service.DB.WithContext(ctx).Limit(1).Find(&progress, "account_id = ?", account.ID).Error; err != nil {
		return core.OutboundMessage{}, err
	}
	if progress.Level > 0 {
		level = progress.Level
	}
	lines := []string{"🎒【装备背包】", "点击下方按钮穿戴，或发送“穿戴 1”。"}
	buttons := []core.KeyboardButton{}
	for index, row := range rows {
		template := templates[row.TemplateKey]
		name := template.Name
		if name == "" {
			name = row.TemplateKey
		}
		status := "未穿戴"
		if row.EquippedSlot != "" {
			status = "已穿戴 " + equipmentSlotLabel(row.EquippedSlot)
		}
		rarity := rarityName(row.Rarity)
		if rarity == "" {
			rarity = row.Rarity
		}
		line := fmt.Sprintf("%d. %s｜%s｜%s｜%s", index+1, name, equipmentSlotLabel(template.Slot), rarity, status)
		tooLow := template.RequiredLevel > level
		if tooLow {
			line += fmt.Sprintf("｜需等级%d", template.RequiredLevel)
		}
		lines = append(lines, line)
		if row.EquippedSlot == "" && !tooLow {
			buttons = append(buttons, core.KeyboardButton{Label: "穿戴 " + name, Command: fmt.Sprintf("穿戴 %d", index+1)})
		}
	}
	return withKeyboardButtons(text(strings.Join(lines, "\n")), buttons, 2), nil
}

func handleEquip(ctx context.Context, event core.InboundEvent, service *Service) (core.OutboundMessage, error) {
	account, err := resolve(ctx, service, event)
	if err != nil {
		return core.OutboundMessage{}, err
	}
	id, err := resolvePlayerEquipmentID(ctx, service, account.ID, strings.TrimSpace(strings.TrimPrefix(event.Text, "穿戴")))
	if err != nil {
		return adventureBusinessError(err)
	}
	equipment, err := service.Equip(ctx, account.ID, id)
	if err != nil {
		return adventureBusinessError(err)
	}
	return text("已穿戴装备 " + equipmentTemplateName(ctx, service.DB, equipment.TemplateKey) + "。"), nil
}
func handleUnequip(ctx context.Context, event core.InboundEvent, service *Service) (core.OutboundMessage, error) {
	account, err := resolve(ctx, service, event)
	if err != nil {
		return core.OutboundMessage{}, err
	}
	id, err := resolvePlayerEquipmentID(ctx, service, account.ID, strings.TrimSpace(strings.TrimPrefix(event.Text, "卸下")))
	if err != nil {
		return adventureBusinessError(err)
	}
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
	keys := make([]string, 0, len(rows))
	for _, row := range rows {
		keys = append(keys, row.EquipmentKey)
	}
	templates := equipmentTemplatesByKey(ctx, service.DB, keys)
	lines := []string{"📜【蓝图背包】"}
	for _, row := range rows {
		status := "收集中"
		if row.Unlocked {
			status = "已解锁"
		}
		name := templates[row.EquipmentKey].Name
		if name == "" {
			name = row.EquipmentKey
		}
		lines = append(lines, fmt.Sprintf("%s｜%d片｜%s", name, row.Fragments, status))
	}
	return text(strings.Join(lines, "\n")), nil
}
func handleCraftEquipment(ctx context.Context, event core.InboundEvent, service *Service) (core.OutboundMessage, error) {
	account, err := resolve(ctx, service, event)
	if err != nil {
		return core.OutboundMessage{}, err
	}
	key := strings.TrimSpace(strings.TrimPrefix(event.Text, "制造"))
	if resolved := resolveEquipmentTemplateKey(ctx, service.DB, key); resolved != "" {
		key = resolved
	}
	equipment, err := service.CraftEquipment(ctx, account.ID, key)
	if err != nil {
		return adventureBusinessError(err)
	}
	return text("🔨 制造完成：" + equipmentTemplateName(ctx, service.DB, equipment.TemplateKey)), nil
}
func handleSalvageEquipment(ctx context.Context, event core.InboundEvent, service *Service) (core.OutboundMessage, error) {
	account, err := resolve(ctx, service, event)
	if err != nil {
		return core.OutboundMessage{}, err
	}
	raw := strings.TrimSpace(strings.TrimPrefix(event.Text, "分解装备"))
	index, err := playerEquipmentListIndex(ctx, service, account.ID, raw)
	if err != nil {
		return adventureBusinessError(err)
	}
	return text(fmt.Sprintf("分解后无法恢复。确认请发送“确认分解装备 %d”。", index)), nil
}
func handleConfirmSalvageEquipment(ctx context.Context, event core.InboundEvent, service *Service) (core.OutboundMessage, error) {
	account, err := resolve(ctx, service, event)
	if err != nil {
		return core.OutboundMessage{}, err
	}
	id, err := resolvePlayerEquipmentID(ctx, service, account.ID, strings.TrimSpace(strings.TrimPrefix(event.Text, "确认分解装备")))
	if err != nil {
		return adventureBusinessError(err)
	}
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
			if errors.Is(err, ErrAdventureBusy) {
				if message, busy := petBusyMessage(ctx, service, account.ID); busy {
					return message, nil
				}
			}
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
		buttons = append(buttons, core.KeyboardButton{Label: "挑战 " + boss.Config.Name, Command: "地图首领 挑战 " + boss.Config.Name}, core.KeyboardButton{Label: "领取奖励", Command: "地图首领 领取 " + boss.Instance.ID})
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
		configByZone := make(map[string]models.AdventureExpeditionConfig, len(configs))
		for _, expedition := range configs {
			configByZone[expedition.ZoneKey] = expedition
		}
		maps, err := service.ListAdventureMaps(ctx, accountID)
		if err != nil {
			return core.OutboundMessage{}, err
		}
		lines := []string{"🧭【区域远征】", "先探索地图解锁区域，再派遣远征。限时首领请发送“地图首领”。", ""}
		buttons := []core.KeyboardButton{}
		for _, adventureMap := range maps {
			available := 0
			for _, zone := range adventureMap.Zones {
				if _, configured := configByZone[zone.Zone.Key]; configured && zone.ExpeditionUnlocked {
					available++
				}
			}
			lines = append(lines, "【"+adventureMap.Map.Name+"】")
			if available == 0 {
				lines = append(lines, collapsedMapHint(adventureMap), "")
				continue
			}
			for _, zone := range adventureMap.Zones {
				configured, exists := configByZone[zone.Zone.Key]
				if !exists || !zone.ExpeditionUnlocked {
					continue
				}
				buttons = append(buttons, core.KeyboardButton{Label: zone.Zone.Name, Command: "远征 " + zone.Zone.Name})
				lines = append(lines, fmt.Sprintf("· %s｜%s｜可派遣｜活动积分 %d｜%s", zone.Zone.Name, expeditionDurationText(configured.DurationMinutes), configured.EventProgressPoints, configured.Description))
			}
			lines = append(lines, "")
		}
		lines = append(lines, "发送“远征 区域名”开始派遣。")
		return withKeyboardButtons(text(strings.Join(lines, "\n")), buttons, 3), nil
	}
	zone, err := findAdventureZone(ctx, service, argument)
	if err != nil {
		return adventureBusinessError(err)
	}
	run, err := service.StartAdventureExpeditionInCommunity(ctx, accountID, communityID(event), zone.Key)
	if err != nil {
		if errors.Is(err, ErrAdventureBusy) {
			if message, busy := petBusyMessage(ctx, service, accountID); busy {
				return message, nil
			}
		}
		return adventureBusinessError(err)
	}
	var snapshot AdventureExpeditionSnapshot
	_ = json.Unmarshal([]byte(run.SnapshotJSON), &snapshot)
	lines := []string{fmt.Sprintf("🚩【%s已开始】", snapshot.Config.Name), fmt.Sprintf("区域：%s｜预计返回：%s", zone.Name, run.EndsAt.Format("01-02 15:04")), fmt.Sprintf("当前评价：%s｜战力 %d/%d", snapshot.Grade, snapshot.Power, snapshot.Config.RecommendedPower), "发送“远征状态”查看进度。"}
	message := text(strings.Join(lines, "\n"))
	message.Image = snapshot.Config.StartImage
	enqueueTimedNotification(ctx, service, event, accountID, "expedition_done", "adventure-expedition:"+run.ID+":done", run.EndsAt,
		fmt.Sprintf("🎉【远征归来】\n%s 已经结束啦！宠物正带着一路收集的宝贝在营地门口等你。\n发送“领取”迎接它带回的收获吧。", snapshot.Config.Name))
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
	if remaining <= 0 {
		message := text(fmt.Sprintf("🧭【%s已完成】\n区域：%s\n评价：%s\n返回时间：%s\n宠物已经回来了，发送“领取”结算奖励。", snapshot.Config.Name, snapshot.Zone.Name, snapshot.Grade, run.EndsAt.Format("01-02 15:04")))
		return withKeyboard(message, []core.KeyboardButton{{Label: "领取", Command: "领取"}}), true, nil
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

func equipmentTemplateKeys(rows []models.PlayerEquipment) []string {
	keys := make([]string, 0, len(rows))
	seen := make(map[string]bool, len(rows))
	for _, row := range rows {
		if row.TemplateKey == "" || seen[row.TemplateKey] {
			continue
		}
		seen[row.TemplateKey] = true
		keys = append(keys, row.TemplateKey)
	}
	return keys
}

func equipmentTemplatesByKey(ctx context.Context, db *gorm.DB, keys []string) map[string]models.EquipmentTemplateConfig {
	result := make(map[string]models.EquipmentTemplateConfig, len(keys))
	if len(keys) == 0 {
		return result
	}
	var templates []models.EquipmentTemplateConfig
	if err := db.WithContext(ctx).Where("key IN ?", keys).Find(&templates).Error; err != nil {
		return result
	}
	for _, template := range templates {
		result[template.Key] = template
	}
	return result
}

func equipmentTemplateName(ctx context.Context, db *gorm.DB, key string) string {
	if template, ok := equipmentTemplatesByKey(ctx, db, []string{key})[key]; ok && template.Name != "" {
		return template.Name
	}
	return key
}

func resolveEquipmentTemplateKey(ctx context.Context, db *gorm.DB, raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	var template models.EquipmentTemplateConfig
	lookup := db.WithContext(ctx).Where("enabled = ? AND (key = ? OR name = ?)", true, raw, raw).Limit(1).Find(&template)
	if lookup.Error != nil || lookup.RowsAffected == 0 {
		return ""
	}
	return template.Key
}

func listPlayerEquipment(ctx context.Context, service *Service, accountID string) ([]models.PlayerEquipment, error) {
	var rows []models.PlayerEquipment
	err := service.DB.WithContext(ctx).Where("account_id = ?", accountID).Order("equipped_slot desc, created_at desc").Limit(50).Find(&rows).Error
	return rows, err
}

func resolvePlayerEquipmentID(ctx context.Context, service *Service, accountID, raw string) (string, error) {
	index, err := playerEquipmentListIndex(ctx, service, accountID, raw)
	if err != nil {
		return "", err
	}
	rows, err := listPlayerEquipment(ctx, service, accountID)
	if err != nil {
		return "", err
	}
	return rows[index-1].ID, nil
}

func playerEquipmentListIndex(ctx context.Context, service *Service, accountID, raw string) (int, error) {
	raw = strings.TrimSpace(raw)
	rows, err := listPlayerEquipment(ctx, service, accountID)
	if err != nil {
		return 0, err
	}
	if len(rows) == 0 {
		return 0, ErrEquipmentNotFound
	}
	if n, convErr := strconv.Atoi(raw); convErr == nil && n >= 1 && n <= len(rows) {
		return n, nil
	}
	templates := equipmentTemplatesByKey(ctx, service.DB, equipmentTemplateKeys(rows))
	matched := 0
	for index, row := range rows {
		name := templates[row.TemplateKey].Name
		if name == "" {
			name = row.TemplateKey
		}
		if row.ID == raw || row.TemplateKey == raw || name == raw {
			if matched != 0 {
				return 0, ErrEquipmentNotFound
			}
			matched = index + 1
		}
	}
	if matched == 0 {
		return 0, ErrEquipmentNotFound
	}
	return matched, nil
}
