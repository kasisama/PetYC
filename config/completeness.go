package config

import (
	"fmt"
	"strings"

	"qq-pet-saas/models"
)

func ValidateLaunchReadiness(snapshot ConfigSnapshot) error {
	if len(snapshot.AdventureMaps) != 3 {
		return fmt.Errorf("需要 3 张永久地图，实际 %d", len(snapshot.AdventureMaps))
	}
	if len(snapshot.AdventureZones) != 12 {
		return fmt.Errorf("需要 12 个区域，实际 %d", len(snapshot.AdventureZones))
	}
	zonesByMap := map[string]int{}
	for _, zone := range snapshot.AdventureZones {
		zonesByMap[zone.MapKey]++
	}
	for _, m := range snapshot.AdventureMaps {
		if zonesByMap[m.Key] != 4 {
			return fmt.Errorf("地图 %s 需要 4 个区域", m.Key)
		}
	}
	if len(snapshot.PetSpecies) != 20 {
		return fmt.Errorf("需要 20 个宠物形态，实际 %d", len(snapshot.PetSpecies))
	}
	families := map[string][]models.PetSpeciesConfig{}
	for _, pet := range snapshot.PetSpecies {
		families[pet.FamilyKey] = append(families[pet.FamilyKey], pet)
	}
	if len(families) != 5 {
		return fmt.Errorf("需要 5 条谱系，实际 %d", len(families))
	}
	for key, forms := range families {
		if len(forms) != 4 {
			return fmt.Errorf("谱系 %s 需要 4 个形态", key)
		}
	}
	if len(snapshot.AdventureLevels) != 25 {
		return fmt.Errorf("需要 25 级冒险等级表，实际 %d", len(snapshot.AdventureLevels))
	}
	if len(snapshot.RewardTracks) != 20 {
		return fmt.Errorf("需要 20 档奖励轨，实际 %d", len(snapshot.RewardTracks))
	}
	if len(snapshot.AdventureSkills) < 30 || len(snapshot.EquipmentTemplates) < 30 || len(snapshot.EquipmentAffixes) < 30 || len(snapshot.EquipmentRecipes) < 12 {
		return fmt.Errorf("技能、装备、词条或配方数量不足")
	}
	for _, skill := range snapshot.AdventureSkills {
		if strings.HasPrefix(skill.Name, "调查战技·") {
			return fmt.Errorf("技能 %s 仍使用编号占位名称", skill.Key)
		}
	}
	for _, equipment := range snapshot.EquipmentTemplates {
		if strings.HasPrefix(equipment.Name, "调查装备·") {
			return fmt.Errorf("装备 %s 仍使用编号占位名称", equipment.Key)
		}
	}
	for _, affix := range snapshot.EquipmentAffixes {
		if strings.HasPrefix(affix.Name, "调查词条") {
			return fmt.Errorf("词条 %s 仍使用编号占位名称", affix.Key)
		}
	}
	pools := map[string]struct{}{}
	for _, affix := range snapshot.EquipmentAffixes {
		pools[affix.PoolKey] = struct{}{}
	}
	if len(pools) < 6 {
		return fmt.Errorf("需要至少 6 个词条池")
	}
	for _, key := range []string{"primary_coin", "journey_badge", "season_token"} {
		found := false
		for _, currency := range snapshot.Currencies {
			if currency.Key == key && currency.Enabled {
				found = true
			}
		}
		if !found {
			return fmt.Errorf("缺少启用中的货币 %s", key)
		}
	}
	for _, item := range snapshot.Items {
		if item.Key == "season_token" {
			return fmt.Errorf("season_token 不能再作为普通物品")
		}
	}
	encountersByMap := map[string]map[string]bool{}
	zoneToMap := map[string]string{}
	for _, zone := range snapshot.AdventureZones {
		zoneToMap[zone.Key] = zone.MapKey
	}
	for _, encounter := range snapshot.AdventureEncounters {
		kinds := encountersByMap[zoneToMap[encounter.ZoneKey]]
		if kinds == nil {
			kinds = map[string]bool{}
			encountersByMap[zoneToMap[encounter.ZoneKey]] = kinds
		}
		kinds[encounter.EncounterType] = true
	}
	bossMaps := map[string]bool{}
	for _, boss := range snapshot.AdventureBosses {
		bossMaps[boss.MapKey] = true
	}
	expeditionZones := map[string]bool{}
	for _, row := range snapshot.AdventureExpeditions {
		if row.Enabled {
			expeditionZones[row.ZoneKey] = true
		}
	}
	eliteMonsters := map[string]bool{}
	for _, monster := range snapshot.AdventureMonsters {
		if monster.Elite && monster.Enabled {
			eliteMonsters[monster.Key] = true
		}
	}
	eliteByMap := map[string]bool{}
	for _, encounter := range snapshot.AdventureEncounters {
		if eliteMonsters[encounter.TargetKey] {
			eliteByMap[zoneToMap[encounter.ZoneKey]] = true
		}
	}
	for _, m := range snapshot.AdventureMaps {
		kinds := encountersByMap[m.Key]
		if !kinds["monster"] || !kinds["landmark"] || !kinds["safe"] {
			return fmt.Errorf("地图 %s 缺少普通遭遇、地标或安全事件", m.Key)
		}
		if !eliteByMap[m.Key] {
			return fmt.Errorf("地图 %s 缺少精英遭遇", m.Key)
		}
		if !bossMaps[m.Key] {
			return fmt.Errorf("地图 %s 缺少群首领", m.Key)
		}
	}
	for _, zone := range snapshot.AdventureZones {
		if !expeditionZones[zone.Key] {
			return fmt.Errorf("区域 %s 没有可解锁远征", zone.Key)
		}
	}
	itemKeys := map[string]models.ItemConfig{}
	for _, item := range snapshot.Items {
		itemKeys[item.Key] = item
	}
	used := map[string]bool{}
	sourced := map[string]bool{}
	markList := func(raw string, asSource, asUse bool) {
		for _, part := range strings.FieldsFunc(raw, func(r rune) bool { return r == '#' || r == ',' || r == '，' }) {
			name := strings.TrimSpace(strings.Split(part, "*")[0])
			if name == "" {
				continue
			}
			for _, item := range snapshot.Items {
				if item.Key == name || item.Name == name {
					if asSource {
						sourced[item.Key] = true
					}
					if asUse {
						used[item.Key] = true
					}
				}
			}
		}
	}
	for _, shop := range snapshot.ShopItems {
		markList(shop.Name, true, true)
	}
	for _, reward := range snapshot.CheckinRewards {
		markList(reward.Items, true, true)
	}
	for _, work := range snapshot.WorkSettings {
		markList(work.RewardItems, true, true)
	}
	for _, chance := range snapshot.ChanceRewards {
		markList(chance.ItemName, true, true)
		markList(chance.RewardKey, true, true)
	}
	for _, game := range snapshot.ChanceGames {
		markList(game.PityRewardKey, true, true)
	}
	for _, pet := range snapshot.PetSpecies {
		markList(pet.FavoriteFood, false, true)
		markList(pet.FavoriteGift, false, true)
	}
	for _, listing := range snapshot.AdventureShopItems {
		if listing.ProductType == "item" {
			sourced[listing.ProductKey] = true
			used[listing.ProductKey] = true
		}
	}
	for _, entry := range snapshot.AdventureLootEntries {
		if entry.RewardType == "item" {
			sourced[entry.RewardKey] = true
		}
	}
	for _, effect := range snapshot.AdventureEncounterEffects {
		if effect.EffectType == "item" {
			sourced[effect.TargetKey] = true
		}
	}
	for _, cost := range snapshot.PetEvolutionCosts {
		used[cost.ItemKey] = true
	}
	for _, material := range snapshot.EquipmentRecipeMaterials {
		used[material.ItemName] = true
	}
	for _, track := range snapshot.RewardTracks {
		if track.RewardType == "item" {
			sourced[track.RewardKey] = true
			used[track.RewardKey] = true
		}
	}
	for key, item := range itemKeys {
		if item.SellPrice > 0 {
			used[key] = true
		}
		if !sourced[key] {
			return fmt.Errorf("物品 %s 没有可识别来源", key)
		}
		if !used[key] {
			return fmt.Errorf("物品 %s 没有有效用途", key)
		}
	}
	currencySource := map[string]bool{}
	currencySink := map[string]bool{}
	for _, reward := range snapshot.CheckinRewards {
		if reward.Currency > 0 {
			currencySource["primary_coin"] = true
		}
	}
	for _, work := range snapshot.WorkSettings {
		if work.RewardCoin > 0 {
			currencySource["primary_coin"] = true
		}
	}
	for _, shop := range snapshot.ShopItems {
		if shop.Price > 0 {
			currencySink["primary_coin"] = true
		}
	}
	for _, listing := range snapshot.AdventureShopItems {
		if listing.CurrencyKey != "" && listing.Price > 0 && listing.Enabled {
			currencySink[listing.CurrencyKey] = true
		}
	}
	for _, entry := range snapshot.AdventureLootEntries {
		if entry.RewardType == "currency" {
			currencySource[entry.RewardKey] = true
		}
	}
	for _, track := range snapshot.RewardTracks {
		if track.RewardType == "currency" {
			currencySource[track.RewardKey] = true
		}
	}
	seasonShop, journeyShop := currencySink["season_token"], currencySink["journey_badge"]
	if !seasonShop || !journeyShop {
		return fmt.Errorf("赛季商店和永久调查商店都必须存在")
	}
	for _, key := range []string{"primary_coin", "journey_badge", "season_token"} {
		if !currencySource[key] {
			return fmt.Errorf("货币 %s 没有可识别来源", key)
		}
		if !currencySink[key] {
			return fmt.Errorf("货币 %s 没有可识别消耗", key)
		}
	}
	baseStats := map[string][3]int64{}
	for _, pet := range snapshot.PetSpecies {
		if pet.Stage != "base" {
			continue
		}
		baseStats[pet.FamilyKey] = [3]int64{pet.Wisdom, pet.Strength, pet.Defense}
	}
	for family, stats := range baseStats {
		for other, compared := range baseStats {
			if family == other {
				continue
			}
			if stats[0] >= compared[0] && stats[1] >= compared[1] && stats[2] >= compared[2] && (stats[0] > compared[0] || stats[1] > compared[1] || stats[2] > compared[2]) {
				return fmt.Errorf("谱系 %s 基础形态全面压过 %s", family, other)
			}
		}
	}
	for _, recipe := range snapshot.EquipmentRecipes {
		var craft int64
		for _, material := range snapshot.EquipmentRecipeMaterials {
			if material.EquipmentKey != recipe.EquipmentKey {
				continue
			}
			item, ok := itemKeys[material.ItemName]
			if !ok {
				for _, candidate := range snapshot.Items {
					if candidate.Name == material.ItemName {
						item, ok = candidate, true
					}
				}
			}
			if !ok {
				return fmt.Errorf("配方 %s 引用了未知材料 %s", recipe.EquipmentKey, material.ItemName)
			}
			price := item.SellPrice
			if price <= 0 {
				price = 2
			}
			craft += price * material.Quantity
		}
		var salvage int64
		for _, template := range snapshot.EquipmentTemplates {
			if template.Key != recipe.EquipmentKey {
				continue
			}
			item, ok := itemKeys[template.SalvageItem]
			if !ok {
				continue
			}
			price := item.SellPrice
			if price <= 0 {
				price = 2
			}
			salvage = price * template.SalvageQuantity
		}
		if salvage > 0 && salvage >= craft && craft > 0 {
			return fmt.Errorf("装备 %s 分解收益不低于制造成本", recipe.EquipmentKey)
		}
	}
	for _, image := range snapshot.Images {
		path := strings.ToLower(image.Path)
		for _, banned := range []string{"伊布", "呱呱", "诺诺", "菀菀", "蔓蔓", "雷诺", "神树"} {
			if strings.Contains(image.Name, banned) || strings.Contains(path, banned) {
				return fmt.Errorf("图片配置仍引用旧 IP: %s", image.Name)
			}
		}
	}
	blueprintSources := map[string]bool{}
	for _, entry := range snapshot.AdventureLootEntries {
		if entry.RewardType == "blueprint_fragment" && entry.MinQuantity > 0 {
			blueprintSources[entry.RewardKey] = true
		}
	}
	for _, recipe := range snapshot.EquipmentRecipes {
		if recipe.Enabled && !blueprintSources[recipe.EquipmentKey] {
			return fmt.Errorf("装备配方 %s 没有蓝图碎片来源", recipe.EquipmentKey)
		}
	}
	return nil
}
