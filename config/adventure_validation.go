package config

import (
	"fmt"
	"strings"
)

func validateAdventureSnapshot(snapshot ConfigSnapshot, items map[string]bool, events map[string]bool) error {
	if len(snapshot.AdventureMaps)+len(snapshot.AdventureZones)+len(snapshot.AdventureMonsters) == 0 {
		return nil
	}
	maps := map[string]bool{}
	zones := map[string]string{}
	objectives := map[string]string{}
	monsters := map[string]bool{}
	skills := map[string]bool{}
	pools := map[string]bool{}
	equipment := map[string]bool{}
	bosses := map[string]bool{}
	currencies := map[string]bool{}

	for _, row := range snapshot.AdventureMaps {
		key := strings.TrimSpace(row.Key)
		if key == "" || maps[key] || strings.TrimSpace(row.Name) == "" || strings.TrimSpace(row.Region) == "" || row.RecommendedLevel < 1 {
			return fmt.Errorf("大地图配置无效或重复: %s", key)
		}
		maps[key] = true
	}
	for _, row := range snapshot.AdventureZones {
		key := strings.TrimSpace(row.Key)
		if key == "" || zones[key] != "" || !maps[row.MapKey] || strings.TrimSpace(row.Name) == "" || row.RecommendedLevel < 1 || row.DifficultyPermille <= 0 || row.HungerCost < 0 || row.ReadinessCost < 0 {
			return fmt.Errorf("区域配置无效或引用了不存在的大地图: %s", key)
		}
		zones[key] = row.MapKey
	}
	graph := map[string][]string{}
	for _, row := range snapshot.AdventurePrereqs {
		if zones[row.ZoneKey] == "" || zones[row.PrerequisiteZoneKey] == "" || row.ZoneKey == row.PrerequisiteZoneKey {
			return fmt.Errorf("区域前置关系无效: %s -> %s", row.PrerequisiteZoneKey, row.ZoneKey)
		}
		graph[row.ZoneKey] = append(graph[row.ZoneKey], row.PrerequisiteZoneKey)
	}
	visiting, visited := map[string]bool{}, map[string]bool{}
	var visit func(string) error
	visit = func(key string) error {
		if visiting[key] {
			return fmt.Errorf("区域前置关系存在循环: %s", key)
		}
		if visited[key] {
			return nil
		}
		visiting[key] = true
		for _, dependency := range graph[key] {
			if err := visit(dependency); err != nil {
				return err
			}
		}
		visiting[key], visited[key] = false, true
		return nil
	}
	for key := range zones {
		if err := visit(key); err != nil {
			return err
		}
	}

	validObjectiveTypes := map[string]bool{"enter": true, "monster_kill": true, "elite_kill": true, "landmark": true, "boss_kill": true}
	for _, row := range snapshot.AdventureObjectives {
		if row.Key == "" || objectives[row.Key] != "" || zones[row.ZoneKey] == "" || !validObjectiveTypes[row.ObjectiveType] || row.RequiredCount <= 0 || row.Weight <= 0 || row.CodexProgress < 0 || row.CodexProgress > 100 {
			return fmt.Errorf("探索目标配置无效或重复: %s", row.Key)
		}
		if (row.CodexCategory == "") != (row.CodexEntry == "") {
			return fmt.Errorf("探索目标 %s 的图鉴类别和条目必须同时配置", row.Key)
		}
		objectives[row.Key] = row.ZoneKey
	}
	for _, zone := range snapshot.AdventureZones {
		if zone.ExpeditionUnlockObjectiveKey != "" && objectives[zone.ExpeditionUnlockObjectiveKey] != zone.Key {
			return fmt.Errorf("区域 %s 的远征解锁目标不存在或属于其他区域", zone.Key)
		}
	}

	for _, row := range snapshot.AdventureMonsters {
		if row.Key == "" || monsters[row.Key] || row.Name == "" || row.Level < 1 || row.MaxHealth <= 0 || row.Attack < 0 || row.Defense < 0 || row.AdventureXP < 0 {
			return fmt.Errorf("怪物配置无效或重复: %s", row.Key)
		}
		monsters[row.Key] = true
	}
	validEffects := map[string]bool{"": true, "heal": true, "shield": true, "attack_up": true, "defense_down": true}
	for _, row := range snapshot.AdventureSkills {
		if row.Key == "" || skills[row.Key] || row.Name == "" || row.PowerPermille < 0 || row.AccuracyPermille < 0 || row.AccuracyPermille > 1000 || row.CooldownTurns < 0 || !validEffects[row.EffectType] {
			return fmt.Errorf("战斗技能配置无效或重复: %s", row.Key)
		}
		skills[row.Key] = true
	}
	for _, row := range snapshot.AdventureMonsterSkills {
		if !monsters[row.MonsterKey] || !skills[row.SkillKey] || row.Weight <= 0 {
			return fmt.Errorf("怪物技能关联无效: %s/%s", row.MonsterKey, row.SkillKey)
		}
	}
	validEncounterTypes := map[string]bool{"monster": true, "landmark": true, "safe": true}
	for _, row := range snapshot.AdventureEncounters {
		if zones[row.ZoneKey] == "" || row.EncounterKey == "" || row.Name == "" || !validEncounterTypes[row.EncounterType] || row.Weight <= 0 {
			return fmt.Errorf("区域遭遇配置无效: %s/%s", row.ZoneKey, row.EncounterKey)
		}
		if row.EncounterType == "monster" && !monsters[row.TargetKey] {
			return fmt.Errorf("遭遇 %s 引用了不存在的怪物 %s", row.EncounterKey, row.TargetKey)
		}
	}
	validEncounterEffects := map[string]bool{"currency": true, "item": true, "heal": true, "readiness": true, "codex": true}
	encounters := map[string]bool{}
	for _, row := range snapshot.AdventureEncounters {
		encounters[row.EncounterKey] = true
	}
	for _, row := range snapshot.AdventureEncounterEffects {
		if !encounters[row.EncounterKey] || !validEncounterEffects[row.EffectType] || row.Weight <= 0 || row.MaxValue < row.MinValue {
			return fmt.Errorf("遭遇效果配置无效: %s/%s", row.EncounterKey, row.EffectType)
		}
		if row.EffectType == "item" && !items[row.TargetKey] {
			return fmt.Errorf("遭遇效果 %s 引用了不存在的统一物品 %s", row.EncounterKey, row.TargetKey)
		}
	}
	for _, row := range snapshot.PetSkillUnlocks {
		if !skills[row.SkillKey] {
			return fmt.Errorf("宠物技能解锁引用了不存在的战斗技能: %s", row.SkillKey)
		}
	}

	for _, row := range snapshot.AdventureLootPools {
		if row.Key == "" || pools[row.Key] || row.Name == "" || row.Rolls < 0 {
			return fmt.Errorf("奖励池配置无效或重复: %s", row.Key)
		}
		pools[row.Key] = true
	}
	validRarities := map[string]bool{"common": true, "fine": true, "rare": true, "epic": true, "legendary": true}
	for _, row := range snapshot.Currencies {
		key := strings.TrimSpace(row.Key)
		if key == "" || currencies[key] || strings.TrimSpace(row.Name) == "" {
			return fmt.Errorf("货币配置无效或重复: %s", key)
		}
		currencies[key] = true
	}
	for _, required := range []string{"primary_coin", "journey_badge", "season_token"} {
		if !currencies[required] {
			return fmt.Errorf("配置缺少内置货币 %s", required)
		}
	}
	validSlots := map[string]bool{"weapon": true, "armor": true, "treasure": true}
	for _, row := range snapshot.EquipmentTemplates {
		if row.Key == "" || equipment[row.Key] || row.Name == "" || !validSlots[row.Slot] || !validRarities[row.Rarity] || row.RequiredLevel < 1 || row.MinAffixes < 0 || row.MaxAffixes < row.MinAffixes || row.SalvageQuantity < 0 {
			return fmt.Errorf("装备模板配置无效或重复: %s", row.Key)
		}
		if row.SalvageItem != "" && !items[row.SalvageItem] {
			return fmt.Errorf("装备 %s 引用了不存在的分解材料 %s", row.Key, row.SalvageItem)
		}
		equipment[row.Key] = true
	}
	validAttributes := map[string]bool{"attack": true, "defense": true, "health": true, "wisdom": true, "crit_rate": true, "dodge_rate": true, "damage_bonus": true, "damage_reduction": true}
	for _, row := range snapshot.EquipmentAffixes {
		if row.Key == "" || row.PoolKey == "" || row.Name == "" || !validAttributes[row.Attribute] || row.MinValue > row.MaxValue || row.Weight <= 0 {
			return fmt.Errorf("装备词条配置无效: %s", row.Key)
		}
	}
	for _, row := range snapshot.EquipmentRecipes {
		if !equipment[row.EquipmentKey] || row.BlueprintFragments <= 0 || row.CurrencyCost < 0 {
			return fmt.Errorf("装备配方配置无效: %s", row.EquipmentKey)
		}
	}
	for _, row := range snapshot.EquipmentRecipeMaterials {
		if !equipment[row.EquipmentKey] || !items[row.ItemName] || row.Quantity <= 0 {
			return fmt.Errorf("装备制造材料配置无效: %s/%s", row.EquipmentKey, row.ItemName)
		}
	}
	validRewardTypes := map[string]bool{"item": true, "currency": true, "equipment": true, "blueprint_fragment": true}
	for _, row := range snapshot.AdventureLootEntries {
		if !pools[row.PoolKey] || !validRewardTypes[row.RewardType] || row.MinQuantity <= 0 || row.MaxQuantity < row.MinQuantity || (!row.Guaranteed && row.Weight <= 0) {
			return fmt.Errorf("奖励池条目配置无效: %s/%s", row.PoolKey, row.RewardKey)
		}
		switch row.RewardType {
		case "item":
			if !items[row.RewardKey] {
				return fmt.Errorf("奖励池 %s 引用了不存在的统一物品 %s", row.PoolKey, row.RewardKey)
			}
		case "currency":
			if !currencies[row.RewardKey] {
				return fmt.Errorf("奖励池 %s 引用了不存在的货币 %s", row.PoolKey, row.RewardKey)
			}
		case "equipment":
			if !equipment[row.RewardKey] {
				return fmt.Errorf("奖励池 %s 引用了不存在的装备 %s", row.PoolKey, row.RewardKey)
			}
		case "blueprint_fragment":
			if !equipment[row.RewardKey] {
				return fmt.Errorf("奖励池 %s 引用了不存在的装备蓝图 %s", row.PoolKey, row.RewardKey)
			}
		}
	}
	validProducts := map[string]bool{"item": true, "equipment": true, "blueprint_fragment": true}
	validLimits := map[string]bool{"none": true, "daily": true, "weekly": true, "season": true, "lifetime": true}
	seenShopItems := map[string]bool{}
	for _, row := range snapshot.AdventureShopItems {
		key := strings.TrimSpace(row.Key)
		if key == "" || seenShopItems[key] || strings.TrimSpace(row.Name) == "" || !validProducts[row.ProductType] || !validLimits[row.LimitType] || row.Quantity <= 0 || row.Price < 0 || row.LimitQuantity < 0 || (row.LimitType != "none" && row.LimitQuantity <= 0) {
			return fmt.Errorf("远征商店商品配置无效或重复: %s", key)
		}
		if row.ProductType == "item" && !items[row.ProductKey] {
			return fmt.Errorf("远征商店商品 %s 引用了不存在的统一物品 %s", key, row.ProductKey)
		}
		if row.ProductType != "item" && !equipment[row.ProductKey] {
			return fmt.Errorf("远征商店商品 %s 引用了不存在的装备或蓝图 %s", key, row.ProductKey)
		}
		currencyKey := strings.TrimSpace(row.CurrencyKey)
		if currencyKey == "" {
			currencyKey = "journey_badge"
		}
		if !currencies[currencyKey] {
			return fmt.Errorf("远征商店商品 %s 引用了不存在的货币 %s", key, currencyKey)
		}
		seenShopItems[key] = true
	}
	checkPool := func(owner, key string) error {
		if key != "" && !pools[key] {
			return fmt.Errorf("%s 引用了不存在的奖励池 %s", owner, key)
		}
		return nil
	}
	for _, row := range snapshot.AdventureMonsters {
		if err := checkPool("怪物 "+row.Key, row.FixedLootPoolKey); err != nil {
			return err
		}
		if err := checkPool("怪物 "+row.Key, row.RandomLootPoolKey); err != nil {
			return err
		}
	}
	for _, row := range snapshot.AdventureExpeditions {
		if zones[row.ZoneKey] == "" || row.Name == "" || row.DurationMinutes <= 0 || row.HungerCost < 0 || row.ReadinessCost < 0 || row.RequiredQuantity < 0 || row.AdventureXP < 0 || row.EventProgressPoints < 0 {
			return fmt.Errorf("区域远征配置无效: %s", row.ZoneKey)
		}
		if row.RequiredItem != "" && !items[row.RequiredItem] {
			return fmt.Errorf("区域远征 %s 引用了不存在的统一物品 %s", row.ZoneKey, row.RequiredItem)
		}
		if err := checkPool("区域远征 "+row.ZoneKey, row.FixedLootPoolKey); err != nil {
			return err
		}
		if err := checkPool("区域远征 "+row.ZoneKey, row.RandomLootPoolKey); err != nil {
			return err
		}
	}
	for _, row := range snapshot.AdventureBosses {
		if row.Key == "" || bosses[row.Key] || zones[row.ZoneKey] != row.MapKey || !monsters[row.MonsterKey] || row.Name == "" || row.SpawnIntervalMinutes <= 0 || row.ActiveDurationMinutes <= 0 || row.ActiveDurationMinutes > row.SpawnIntervalMinutes || row.MaxHealth <= 0 || row.ChallengeCooldownMinutes < 0 || row.ChallengeLimit < 0 || row.MinimumContribution <= 0 {
			return fmt.Errorf("地图首领配置无效: %s", row.Key)
		}
		if err := checkPool("地图首领 "+row.Key, row.DefeatedLootPoolKey); err != nil {
			return err
		}
		if err := checkPool("地图首领 "+row.Key, row.ExpiredLootPoolKey); err != nil {
			return err
		}
		bosses[row.Key] = true
	}
	for _, row := range snapshot.AdventureBossRewardTiers {
		if !bosses[row.BossKey] || row.Threshold <= 0 || !pools[row.LootPoolKey] {
			return fmt.Errorf("地图首领奖励档位无效: %s/%d", row.BossKey, row.Threshold)
		}
	}

	choicesByEvent := map[string]int{}
	effectsByEvent := map[string]map[string]bool{}
	for _, row := range snapshot.LiveEventChoices {
		if !events[row.EventKey] || row.ChoiceKey == "" || row.Label == "" || row.EffectValue < 1 || row.EffectValue > 100 {
			return fmt.Errorf("活动故事选项配置无效: %s/%s", row.EventKey, row.ChoiceKey)
		}
		if row.EffectType != "community_material_gain_percent" && row.EffectType != "facility_upgrade_cost_reduction_percent" && row.EffectType != "boss_damage_gain_percent" {
			if row.EffectType != "adventure_xp_gain_percent" && row.EffectType != "expedition_reward_gain_percent" {
				return fmt.Errorf("活动故事选项效果类型无效: %s", row.EffectType)
			}
		}
		if effectsByEvent[row.EventKey] == nil {
			effectsByEvent[row.EventKey] = map[string]bool{}
		}
		if effectsByEvent[row.EventKey][row.EffectType] {
			return fmt.Errorf("活动 %s 重复配置了效果 %s", row.EventKey, row.EffectType)
		}
		effectsByEvent[row.EventKey][row.EffectType] = true
		choicesByEvent[row.EventKey]++
	}
	for _, event := range snapshot.LiveEvents {
		if choicesByEvent[event.Key] > 0 && (choicesByEvent[event.Key] < 2 || choicesByEvent[event.Key] > 5) {
			return fmt.Errorf("活动 %s 必须配置 2 到 5 个故事选项", event.Key)
		}
		if event.ProgressSourceMode != "" && event.ProgressSourceMode != "all_expeditions" && event.ProgressSourceMode != "selected" {
			return fmt.Errorf("活动 %s 的进度来源模式无效", event.Key)
		}
	}
	for _, row := range snapshot.LiveEventSources {
		if !events[row.EventKey] || zones[row.ZoneKey] == "" {
			return fmt.Errorf("活动远征来源无效: %s/%s", row.EventKey, row.ZoneKey)
		}
	}
	return nil
}
