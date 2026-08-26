//go:build ignore

// Command generate_v010 rebuilds the official v0.1.0 content profile.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"qq-pet-saas/config"
	"qq-pet-saas/models"
)

type family struct {
	key, archetype string
	names          [4]string
	stats          [4]int64
	favoriteFood   string
	favoriteGift   string
	descriptions   [4]string
}

func main() {
	raw, mustErr := os.ReadFile("config/defaults/base_surface.json")
	must(mustErr)
	var s config.ConfigSnapshot
	must(json.Unmarshal(raw, &s))

	setSystem(&s, "Core.CoinName", "星砂")
	setSystem(&s, "Core.InitialCoin", "240")
	setSystem(&s, "Core.InitialPets", "光芽兽#苔须灵#烬爪兽#岩甲犀#风耳狐")
	setSystem(&s, "Core.MaxPetSlots", "1")
	setSystem(&s, "Core.MaxConcurrentRuns", "1")
	setSystem(&s, "Interaction.WorkRewardItems", "调查便当*1")
	setSystem(&s, "Interaction.LotteryItem", "遗迹抽签券*1")
	setSystem(&s, "Interaction.LotteryRewardStr", "1%星辉晶核*1~1")
	setSystem(&s, "Interaction.FishSpecies", "溪纹鱼")
	setSystem(&s, "Interaction.TreeRewardItems", "晨露果*1")
	setSystem(&s, "Interaction.CreateFamilyItem", "共建标识*1")

	buildItems(&s)
	buildPets(&s)
	buildLevels(&s)
	buildWorld(&s)
	buildEquipment(&s)
	buildEconomy(&s)

	out, err := json.MarshalIndent(s, "", "  ")
	must(err)
	must(os.WriteFile("config/defaults/config_v0.1.0.json", append(out, '\n'), 0o644))
	fmt.Printf("generated config/defaults/config_v0.1.0.json (%d bytes)\n", len(out))
}

func setSystem(s *config.ConfigSnapshot, key, value string) {
	for i := range s.System {
		if s.System[i].Key == key {
			s.System[i].Value = value
			return
		}
	}
	s.System = append(s.System, models.SystemConfig{Key: key, Value: value})
}

func buildItems(s *config.ConfigSnapshot) {
	groups := []struct {
		category, rarity string
		items            [][2]string
	}{
		{"consumable", "common", [][2]string{{"field_ration", "调查便当"}, {"morning_berry", "晨露果"}, {"warm_soup", "暖叶汤"}, {"focus_tea", "清神茶"}, {"bandage", "软藤绷带"}, {"camp_kit", "便携营具"}, {"energy_biscuit", "活力脆饼"}, {"mist_antidote", "祛雾药剂"}}},
		{"gift", "common", [][2]string{{"wind_chime", "风铃草结"}, {"shell_music_box", "潮音匣"}, {"sunny_postcard", "晴野明信片"}, {"polished_stone", "温润卵石"}, {"feather_toy", "轻羽玩具"}, {"story_book", "遗迹故事册"}}},
		{"training", "fine", [][2]string{{"wisdom_notes", "观察笔记"}, {"strength_band", "负重藤环"}, {"defense_pad", "岩绒护垫"}, {"agility_ribbon", "追风缎带"}}},
		{"material", "common", [][2]string{{"meadow_fiber", "原野纤维"}, {"glow_pollen", "微光花粉"}, {"clear_dew", "澄澈露珠"}, {"tide_shell", "潮纹贝片"}, {"ruin_gear", "遗迹齿轮"}, {"salt_crystal", "盐晶砂"}, {"mist_wood", "雾纹木"}, {"spore_dust", "孢光粉"}, {"ancient_bark", "古树皮"}, {"soft_clay", "柔韧陶土"}, {"copper_thread", "导能铜丝"}, {"survey_ink", "调查墨水"}}},
		{"evolution", "rare", [][2]string{{"dawn_core", "晨曦晶核"}, {"tide_core", "潮痕晶核"}, {"mist_core", "雾冠晶核"}, {"resonance_seed", "共鸣之种"}, {"star_core", "星辉晶核"}}},
		{"boss_material", "epic", [][2]string{{"prairie_horn", "原野王角"}, {"tide_lens", "潮眼透镜"}, {"forest_heart", "森冠心木"}}},
		{"event", "rare", [][2]string{{"season_memento", "首季纪念叶"}, {"build_mark", "共建标识"}, {"ruin_ticket", "遗迹抽签券"}}},
		{"collectible", "rare", [][2]string{{"pressed_flower", "栖光压花"}, {"echo_shell", "回声贝"}, {"fog_map", "雾林手绘图"}, {"survey_badge_pin", "调查纪念章"}}},
	}
	for _, group := range groups {
		for _, item := range group.items {
			typeName, effect := "材料", ""
			if group.category == "consumable" {
				typeName, effect = "饱食", "18"
			}
			if group.category == "gift" {
				typeName = "礼物"
			}
			if group.category == "training" {
				typeName, effect = "成长", "6"
			}
			s.Items = append(s.Items, models.ItemConfig{Key: item[0], Name: item[1], Category: group.category, Rarity: group.rarity, Stackable: true, MaxStack: 9999, Usage: "陪伴、探索、制造或收藏", Status: "active", Type: typeName, Effect: effect, Description: item[1] + "，自然遗迹调查队的标准物资。", SellPrice: 2})
		}
	}
}

func buildPets(s *config.ConfigSnapshot) {
	families := []family{
		{"lumisprout", "balanced", [4]string{"光芽兽", "曜叶兽", "曦冠灵", "月荫灵"}, [4]int64{14, 14, 14, 14}, "晨露果", "晴野明信片", [4]string{"背生双叶的原野伙伴，能感知遗迹中微弱的生命回响。", "叶芽舒展成日轮，攻守均衡，擅长稳定调查队节奏。", "以晨光强化治愈与协作，是长线探索的可靠核心。", "借月荫隐藏踪迹并捕捉破绽，偏向灵巧的均衡路线。"}},
		{"mosswhisk", "support", [4]string{"苔须灵", "苔语贤者", "森语祭司", "回响学者"}, [4]int64{13, 19, 10, 14}, "清神茶", "遗迹故事册", [4]string{"触须能读取苔痕年代的小兽，善于记录与照料同伴。", "苔须化作感知冠，能提前识别危险并修复队伍状态。", "引导林地回响守护群体，擅长持续治疗和净化。", "解析遗迹声纹削弱敌人，以知识换取更高调查效率。"}},
		{"emberpaw", "attacker", [4]string{"烬爪兽", "炎纹猎手", "炽阳斗士", "余烬游侠"}, [4]int64{13, 10, 21, 12}, "暖叶汤", "风铃草结", [4]string{"爪尖留有温热火纹，行动直接，喜欢冲在调查队前方。", "能够追踪热流与裂隙，连续攻击会逐步提高压制力。", "将积蓄的热量化作正面爆发，擅长快速结束强敌战。", "控制余烬制造弱点，以机动和持续灼痕应对长战。"}},
		{"stoneback", "guardian", [4]string{"岩甲犀", "岩脊守卫", "山岳壁垒", "晶壳卫士"}, [4]int64{19, 9, 11, 17}, "调查便当", "温润卵石", [4]string{"背甲由柔韧矿层构成，会本能挡在受惊的伙伴前方。", "岩脊稳定后可吸收冲击，是首领调查中的可靠前卫。", "让甲层与大地共鸣，为全队承担伤害并稳固阵线。", "结晶甲片能折射能量，兼顾防护与反制异常攻击。"}},
		{"galeear", "striker", [4]string{"风耳狐", "逐风灵狐", "天际追猎者", "岚影侦察官"}, [4]int64{12, 13, 19, 12}, "活力脆饼", "轻羽玩具", [4]string{"长耳可辨认远方气流变化，是敏捷而谨慎的先行侦察者。", "能沿风痕高速移动，以先手和暴击掌握短战优势。", "追逐高空气流发动致命突袭，偏向极限爆发路线。", "利用岚影标记目标并安全撤离，擅长侦察与持续输出。"}},
	}
	for familyIndex, f := range families {
		keys := [4]string{f.key + "_base", f.key + "_evolved", f.key + "_awaken_a", f.key + "_awaken_b"}
		for i, key := range keys {
			stage, previous := "base", ""
			mult := int64(1)
			if i == 1 {
				stage, previous, mult = "evolved", keys[0], 2
			}
			if i > 1 {
				stage, previous, mult = "awakened", keys[1], 3
			}
			wisdom, strength, defense := f.stats[1]*mult, f.stats[2]*mult, f.stats[3]*mult
			if i == 2 {
				wisdom += 3
				strength -= 3
			}
			if i == 3 {
				wisdom -= 3
				strength += 3
			}
			s.PetSpecies = append(s.PetSpecies, models.PetSpeciesConfig{Key: key, Name: f.names[i], FamilyKey: f.key, Stage: stage, PreviousFormKey: previous, Adoptable: i == 0, Archetype: f.archetype, CodexEntryKey: f.key, FavoriteFood: f.favoriteFood, FavoriteGift: f.favoriteGift, Health: 80 + f.stats[0]*mult, HealthMax: 120 + f.stats[0]*mult*3, Hunger: 100, HungerMax: 110, Wisdom: wisdom, Strength: strength, Defense: defense, WisdomMax: 120, StrengthMax: 120, DefenseMax: 120, Description: f.descriptions[i]})
		}
		standard := f.key + "_standard"
		s.PetEvolutionRules = append(s.PetEvolutionRules,
			models.PetEvolutionRuleConfig{Key: standard, FromFormKey: keys[0], ToFormKey: keys[1], RequiredGrowth: 750, RequiredAffection: 100, BranchLabel: "标准进化", Enabled: true, SortOrder: 10},
			models.PetEvolutionRuleConfig{Key: f.key + "_branch_a", FromFormKey: keys[1], ToFormKey: keys[2], RequiredGrowth: 4800, RequiredAffection: 720, BranchLabel: "曦光路线", Enabled: true, SortOrder: 20},
			models.PetEvolutionRuleConfig{Key: f.key + "_branch_b", FromFormKey: keys[1], ToFormKey: keys[3], RequiredGrowth: 4800, RequiredAffection: 720, BranchLabel: "月影路线", Enabled: true, SortOrder: 30})
		s.PetEvolutionCosts = append(s.PetEvolutionCosts,
			models.PetEvolutionCostConfig{EvolutionKey: standard, ItemKey: "resonance_seed", Quantity: 1},
			models.PetEvolutionCostConfig{EvolutionKey: f.key + "_branch_a", ItemKey: "dawn_core", Quantity: 3},
			models.PetEvolutionCostConfig{EvolutionKey: f.key + "_branch_b", ItemKey: "mist_core", Quantity: 3})
		for formIndex, formKey := range keys {
			familySkills := [4][3]int{{0, 1, 2}, {1, 2, 3}, {2, 3, 4}, {2, 3, 5}}
			for n, offset := range familySkills[formIndex] {
				s.PetSkillUnlocks = append(s.PetSkillUnlocks, models.PetSkillUnlockConfig{FormKey: formKey, SkillKey: fmt.Sprintf("pet_skill_%02d", familyIndex*6+offset+1), UnlockLevel: 1 + n*4, SortOrder: n * 10})
			}
		}
	}
}

func buildLevels(s *config.ConfigSnapshot) {
	for level := 1; level <= 25; level++ {
		xp := int64(60 + level*level*8)
		if level == 25 {
			xp = 0
		}
		s.AdventureLevels = append(s.AdventureLevels, models.AdventureLevelConfig{Level: level, XPToNext: xp, PowerAllowance: int64(60 + level*18)})
	}
}

func buildWorld(s *config.ConfigSnapshot) {
	maps := []struct {
		key, name, region string
		level             int
	}{{"sunlit_steppe", "栖光原野", "晨光草原", 1}, {"tide_ruins", "潮痕遗址", "沉潮遗迹", 7}, {"mist_crown_forest", "雾冠深林", "古树雾林", 15}}
	zoneNames := [][]string{{"萤草坡", "风车溪谷", "石环牧径", "日落高台"}, {"退潮长廊", "回声工坊", "盐晶中庭", "潮眼核心"}, {"孢光浅径", "倒悬根谷", "雾钟圣所", "冠层心庭"}}
	normalNames := []string{"萤绒团", "草铃雀", "坡角鼹", "溪纹蜥", "风籽蜂", "石环蟹", "牧径獾", "暮光蛾", "贝甲虫", "潮纹鳗", "齿轮寄居蟹", "回声水母", "盐晶蜗", "锈羽鸥", "古管蛇", "潮眼鳐", "孢灯虫", "雾茸兔", "根谷螳", "藤面猿", "钟叶蝶", "苔冠鹿", "空枝鸮", "心庭蕈兽"}
	eliteNames := []string{"辉角巡兽", "暮风猎隼", "铜潮守卫", "盐晶巨螯", "雾根卫士", "冠层梦魇"}
	for i, m := range maps {
		s.AdventureMaps = append(s.AdventureMaps, models.AdventureMapConfig{Key: m.key, Name: m.name, Region: m.region, Description: "调查队第" + fmt.Sprint(i+1) + "阶段永久调查地图。", RecommendedLevel: m.level, Enabled: true, SortOrder: (i + 1) * 10})
		for z := 0; z < 4; z++ {
			zoneKey := fmt.Sprintf("%s_z%d", m.key, z+1)
			level := m.level + z*2
			objectiveKey := zoneKey + "_survey"
			s.AdventureZones = append(s.AdventureZones, models.AdventureZoneConfig{Key: zoneKey, MapKey: m.key, Name: zoneNames[i][z], Description: "包含地标、安全事件、普通生物与精英目标的调查区域。", RecommendedLevel: level, DifficultyPermille: 950 + level*35, HungerCost: int64(2 + i + z), ReadinessCost: 2 + i + z, ExpeditionUnlockObjectiveKey: objectiveKey, Enabled: true, SortOrder: (z + 1) * 10})
			if i > 0 || z > 0 {
				prevMap, prevZone := i, z-1
				if z == 0 {
					prevMap, prevZone = i-1, 3
				}
				s.AdventurePrereqs = append(s.AdventurePrereqs, models.AdventureZonePrerequisiteConfig{ZoneKey: zoneKey, PrerequisiteZoneKey: fmt.Sprintf("%s_z%d", maps[prevMap].key, prevZone+1)})
			}
			normalA, normalB := i*8+z*2, i*8+z*2+1
			eliteIndex := i*2 + z/2
			normalKeyA, normalKeyB, eliteKey := fmt.Sprintf("normal_%02d", normalA+1), fmt.Sprintf("normal_%02d", normalB+1), fmt.Sprintf("elite_%02d", eliteIndex+1)
			s.AdventureObjectives = append(s.AdventureObjectives,
				models.AdventureObjectiveConfig{Key: zoneKey + "_enter", ZoneKey: zoneKey, Name: "抵达" + zoneNames[i][z], ObjectiveType: "enter", RequiredCount: 1, Weight: 25, Enabled: true, SortOrder: 10},
				models.AdventureObjectiveConfig{Key: objectiveKey, ZoneKey: zoneKey, Name: "完成区域生态调查", ObjectiveType: "monster_kill", TargetKey: normalKeyA, RequiredCount: 3, Weight: 75, CodexCategory: "区域生态", CodexEntry: zoneKey, CodexProgress: 100, Enabled: true, SortOrder: 20})
			s.CodexCatalog = append(s.CodexCatalog, models.CodexCatalogConfig{Category: "区域生态", EntryKey: zoneKey, Region: m.name, Description: zoneNames[i][z] + "调查记录", Enabled: true, SortOrder: i*100 + z*10, SourceType: "zone", SourceKey: zoneKey})
			pool := zoneKey + "_loot"
			s.AdventureLootPools = append(s.AdventureLootPools, models.AdventureLootPoolConfig{Key: pool, Name: zoneNames[i][z] + "调查掉落", Rolls: 2})
			materialKeys := []string{"meadow_fiber", "glow_pollen", "tide_shell", "ruin_gear", "mist_wood", "spore_dust"}
			s.AdventureLootEntries = append(s.AdventureLootEntries,
				models.AdventureLootEntryConfig{PoolKey: pool, RewardType: "item", RewardKey: materialKeys[i*2+z%2], MinQuantity: 1, MaxQuantity: 3, Weight: 80, Guaranteed: true, SortOrder: 10},
				models.AdventureLootEntryConfig{PoolKey: pool, RewardType: "currency", RewardKey: "journey_badge", MinQuantity: 1, MaxQuantity: 2, Weight: 25, SortOrder: 15})
			for _, encounter := range []struct {
				key, typ, target, name string
				weight                 int
			}{{zoneKey + "_a", "monster", normalKeyA, normalNames[normalA], 45}, {zoneKey + "_b", "monster", normalKeyB, normalNames[normalB], 35}, {zoneKey + "_elite", "monster", eliteKey, eliteNames[eliteIndex], 8}, {zoneKey + "_landmark", "landmark", "", "调查地标", 6}, {zoneKey + "_safe", "safe", "", "安全营地", 6}} {
				s.AdventureEncounters = append(s.AdventureEncounters, models.AdventureEncounterConfig{ZoneKey: zoneKey, EncounterKey: encounter.key, EncounterType: encounter.typ, TargetKey: encounter.target, Name: encounter.name, Description: "调查途中记录到的" + encounter.name + "。", Weight: encounter.weight, Enabled: true})
			}
			s.AdventureEncounterEffects = append(s.AdventureEncounterEffects,
				models.AdventureEncounterEffectConfig{EncounterKey: zoneKey + "_landmark", EffectType: "item", TargetKey: "survey_ink", MinValue: 1, MaxValue: 2, Weight: 1, Enabled: true},
				models.AdventureEncounterEffectConfig{EncounterKey: zoneKey + "_safe", EffectType: "readiness", MinValue: 4, MaxValue: 8, Weight: 1, Enabled: true})
			s.AdventureExpeditions = append(s.AdventureExpeditions, models.AdventureExpeditionConfig{ZoneKey: zoneKey, Name: zoneNames[i][z] + "定期调查", Description: "离线调查并向开始时的宠物结算成长。", DurationMinutes: int64(15 + level*12), HungerCost: int64(4 + i + z), ReadinessCost: 4 + i + z, FixedLootPoolKey: pool, AdventureXP: int64(28 + level*8), EventProgressPoints: int64(26 + z), RecommendedPower: int64(55 + level*20), Enabled: true})
		}
	}
	for i, core := range []string{"resonance_seed", "tide_core", "dawn_core", "mist_core", "star_core"} {
		pool := s.AdventureLootPools[min(i*2, len(s.AdventureLootPools)-1)].Key
		s.AdventureLootEntries = append(s.AdventureLootEntries, models.AdventureLootEntryConfig{PoolKey: pool, RewardType: "item", RewardKey: core, MinQuantity: 1, MaxQuantity: 1, Weight: 8, SortOrder: 20})
	}
	for i, source := range []struct {
		pool, item string
	}{
		{"sunlit_steppe_z2_loot", "clear_dew"},
		{"tide_ruins_z2_loot", "copper_thread"},
		{"tide_ruins_z3_loot", "salt_crystal"},
		{"mist_crown_forest_z2_loot", "ancient_bark"},
		{"sunlit_steppe_z4_loot", "pressed_flower"},
		{"tide_ruins_z4_loot", "echo_shell"},
		{"mist_crown_forest_z4_loot", "fog_map"},
		{"sunlit_steppe_z3_loot", "ruin_ticket"},
	} {
		s.AdventureLootEntries = append(s.AdventureLootEntries, models.AdventureLootEntryConfig{PoolKey: source.pool, RewardType: "item", RewardKey: source.item, MinQuantity: 1, MaxQuantity: 1, Weight: 5, SortOrder: 80 + i})
	}
	for i, name := range normalNames {
		level := 1 + i
		pool := s.AdventureLootPools[(i / 2)].Key
		s.AdventureMonsters = append(s.AdventureMonsters, models.AdventureMonsterConfig{Key: fmt.Sprintf("normal_%02d", i+1), Name: name, Description: "遗迹生态中的常见生物。", Level: level, MaxHealth: int64(35 + level*13), Attack: int64(7 + level*3), Defense: int64(3 + level*2), Wisdom: int64(2 + level), AdventureXP: int64(18 + level*4), AIProfile: "balanced", FixedLootPoolKey: pool, Enabled: true})
	}
	for i, name := range eliteNames {
		level := 4 + i*4
		pool := s.AdventureLootPools[i*2+1].Key
		s.AdventureMonsters = append(s.AdventureMonsters, models.AdventureMonsterConfig{Key: fmt.Sprintf("elite_%02d", i+1), Name: name, Description: "区域精英调查目标。", Level: level, MaxHealth: int64(150 + level*22), Attack: int64(20 + level*4), Defense: int64(12 + level*3), Wisdom: int64(8 + level*2), AdventureXP: int64(70 + level*8), AIProfile: "aggressive", FixedLootPoolKey: pool, Elite: true, Enabled: true})
	}
	buildSkillsAndBosses(s, maps)
}

func buildSkillsAndBosses(s *config.ConfigSnapshot, maps []struct {
	key, name, region string
	level             int
}) {
	effects := []string{"", "heal", "shield", "attack_up", "defense_down"}
	skillNames := []string{
		"芽光连击", "晨叶回响", "原野守势", "日轮共鸣", "曦冠复苏", "月荫破绽",
		"苔语问诊", "孢光屏障", "根脉安抚", "森语合唱", "古痕解析", "回声封锁",
		"烬爪突袭", "炎纹追猎", "热流蓄势", "炽阳爆燃", "余烬刻印", "灰烬回旋",
		"岩角推进", "层甲承压", "地脉固守", "山岳同调", "晶壳折返", "磐心庇护",
		"风耳预警", "逐风切入", "气流标记", "天际俯冲", "岚影换位", "追迹终结",
	}
	skillDescriptions := []string{
		"以双叶引导连续打击，稳定积累调查优势。", "释放晨光治疗受损伙伴。", "展开叶脉护层降低正面冲击。", "均衡强化全队攻防节奏。", "曦光路线的强力群体复苏。", "月影路线标记敌方防御缺口。",
		"读取伤势并进行精准治疗。", "以孢光形成可持续护盾。", "安抚地脉，恢复并稳定队伍。", "森语路线强化群体续航。", "解析古代纹路提高队伍输出。", "回响路线压制敌方关键能力。",
		"用炽热前爪发动快速强攻。", "追踪热痕，提高后续攻击威力。", "压缩热流为下一击蓄势。", "炽阳路线释放高额爆发。", "余烬路线留下持续伤害印记。", "在长战中反复扩大灼烧优势。",
		"以岩角稳步推进并打断敌势。", "层叠背甲吸收大量伤害。", "连接地脉提高全队防御。", "山岳路线承担并分散伤害。", "晶壳路线反射部分能量冲击。", "在濒危时形成坚固群体屏障。",
		"提前捕捉气流，提升行动准确。", "沿风线抢占先手位置。", "记录目标轨迹并降低其防御。", "天际路线发动高暴击俯冲。", "岚影路线规避攻击并重新定位。", "对已标记目标完成精准收割。",
	}
	for i := 1; i <= 30; i++ {
		key := fmt.Sprintf("pet_skill_%02d", i)
		s.AdventureSkills = append(s.AdventureSkills, models.AdventureSkillConfig{Key: key, Name: skillNames[i-1], Description: skillDescriptions[i-1], PowerPermille: 850 + (i%7)*90, WisdomPermille: (i % 4) * 100, AccuracyPermille: 900 + (i%3)*40, CooldownTurns: i % 4, EffectType: effects[i%len(effects)], EffectValue: 5 + i%10, Enabled: true})
	}
	for i := 0; i < 30; i++ {
		monsterKey := fmt.Sprintf("normal_%02d", i%24+1)
		if i >= 24 {
			monsterKey = fmt.Sprintf("elite_%02d", i-23)
		}
		s.AdventureMonsterSkills = append(s.AdventureMonsterSkills, models.AdventureMonsterSkillConfig{MonsterKey: monsterKey, SkillKey: fmt.Sprintf("pet_skill_%02d", i+1), Weight: 10, SortOrder: 10})
	}
	bossNames := []string{"逐日原角王", "深潮观测体", "千年雾冠主"}
	bossMaterials := []string{"prairie_horn", "tide_lens", "forest_heart"}
	anchor := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	for i, name := range bossNames {
		monsterKey := fmt.Sprintf("boss_%02d", i+1)
		level := []int{8, 16, 25}[i]
		pool := fmt.Sprintf("boss_%02d_pool", i+1)
		s.AdventureLootPools = append(s.AdventureLootPools, models.AdventureLootPoolConfig{Key: pool, Name: name + "共享奖励", Rolls: 1})
		s.AdventureLootEntries = append(s.AdventureLootEntries, models.AdventureLootEntryConfig{PoolKey: pool, RewardType: "item", RewardKey: bossMaterials[i], MinQuantity: 1, MaxQuantity: 1, Weight: 1, Guaranteed: true})
		s.AdventureMonsters = append(s.AdventureMonsters, models.AdventureMonsterConfig{Key: monsterKey, Name: name, Description: "群体协作调查首领。", Level: level, MaxHealth: int64(2500 + i*5000), Attack: int64(45 + i*40), Defense: int64(25 + i*28), Wisdom: int64(20 + i*25), AdventureXP: int64(300 + i*300), AIProfile: "boss", Elite: true, Enabled: true})
		zoneKey := fmt.Sprintf("%s_z4", maps[i].key)
		bossKey := fmt.Sprintf("shared_boss_%02d", i+1)
		s.AdventureBosses = append(s.AdventureBosses, models.AdventureBossConfig{Key: bossKey, MapKey: maps[i].key, ZoneKey: zoneKey, MonsterKey: monsterKey, Name: name, Description: "阶段窗口内全群共享生命值，奖励按参与和共建发放。", ScheduleAnchor: anchor.Add(time.Duration([]int{10, 31, 56}[i]) * 24 * time.Hour), SpawnIntervalMinutes: 100800, ActiveDurationMinutes: 4320, RecommendedLevel: level, MaxHealth: int64(250000 + i*350000), Attack: int64(45 + i*40), Defense: int64(25 + i*28), Wisdom: int64(20 + i*25), ChallengeCooldownMinutes: 30, ChallengeLimit: 12, MinimumContribution: 1, DefeatedLootPoolKey: pool, ExpiredLootPoolKey: pool, Enabled: true})
		s.AdventureBossRewardTiers = append(s.AdventureBossRewardTiers, models.AdventureBossRewardTierConfig{BossKey: bossKey, Threshold: 1, LootPoolKey: pool, Description: "完成一次有效协作即可领取，不按伤害排名垄断资源"})
	}
}

func buildEquipment(s *config.ConfigSnapshot) {
	rarities := []string{"common", "fine", "rare", "epic", "legendary"}
	slots := []string{"weapon", "armor", "treasure"}
	poolNames := [6][5]string{
		{"稳击", "固守", "生息", "明察", "机敏"},
		{"原野锋芒", "草坡韧性", "晨露活力", "风向洞察", "追风灵巧"},
		{"潮痕锐意", "盐晶护层", "回声生机", "遗迹解析", "退潮迅捷"},
		{"雾冠穿透", "古木庇护", "孢光复苏", "雾径感知", "岚影暴击"},
		{"首领克制", "共建壁垒", "协作生命", "调查智慧", "破绽追击"},
		{"星辉锋刃", "永续守护", "森罗生机", "遗迹真知", "冠冕暴击"},
	}
	equipmentNames := []string{
		"原野短杖", "藤编护衣", "晨露罗盘", "测绘手斧", "石环背心", "风向铃",
		"潮纹刺刃", "盐织甲", "回声测距仪", "遗迹破拆锤", "铜脊护服", "旧城透镜",
		"雾木长弓", "孢光披肩", "根谷信标", "晶砂战镐", "古树层甲", "冠层星盘",
		"逐日角枪", "原王壁甲", "栖光圣印", "深潮切割器", "潮眼重铠", "回声核心",
		"雾冠裁决弓", "森心守望甲", "千年树轮仪", "星痕调查刃", "三域共建装甲", "遗迹初鸣秘宝",
	}
	equipmentDescriptions := []string{
		"轻便可靠的入门武器，适合第一次野外调查。", "以柔韧藤条编成的基础防护。", "会对附近生命回响轻轻偏转指针。", "兼顾采样与自卫的调查工具。", "嵌有石环碎片的耐用背心。", "根据气流变化发出不同音色。",
		"利用潮汐纹路引导连续攻击。", "盐晶纤维能缓冲遗迹中的冲击。", "记录回声距离并增强智慧判断。", "为开启旧机械结构设计的重型工具。", "导能铜丝强化了防具骨架。", "可看见潮痕遗址中残留的能量轨迹。",
		"雾纹木制成的精准远程武器。", "孢光层会随呼吸修复细小破损。", "帮助调查队在复杂根谷中定位。", "晶砂刃面能击穿高阶生态外壳。", "古树皮与雾纹木叠成的重甲。", "模拟冠层星光判断遗迹活动周期。",
		"以逐日原角打磨的首领级长枪。", "保留原野王角共鸣的坚固壁甲。", "凝聚栖光原野共同调查记录的秘印。", "以潮眼透镜校正攻击角度的利刃。", "能分散深潮压力的首领级重铠。", "保存回声工坊核心数据的秘宝。",
		"森冠心木与雾纹木共同制成的终阶弓。", "守护冠层调查者的传奇甲胄。", "记载千年树冠生长周期的测算仪。", "汇聚三域调查技术的精准武器。", "由群体共建成果强化的传奇防具。", "首季遗迹共鸣凝成的终阶秘宝。",
	}
	for p := 1; p <= 6; p++ {
		for a := 1; a <= 5; a++ {
			attributes := []string{"attack", "defense", "health", "wisdom", "crit_rate"}
			s.EquipmentAffixes = append(s.EquipmentAffixes, models.EquipmentAffixConfig{Key: fmt.Sprintf("affix_%d_%d", p, a), PoolKey: fmt.Sprintf("affix_pool_%d", p), Name: poolNames[p-1][a-1], Attribute: attributes[a-1], MinValue: int64(p + a), MaxValue: int64(p*3 + a*2), Weight: 20, Enabled: true})
		}
	}
	for i := 1; i <= 30; i++ {
		rarityIndex := (i - 1) / 6
		slot := slots[(i-1)%3]
		key := fmt.Sprintf("equipment_%02d", i)
		s.EquipmentTemplates = append(s.EquipmentTemplates, models.EquipmentTemplateConfig{Key: key, Name: equipmentNames[i-1], Description: equipmentDescriptions[i-1], Slot: slot, Rarity: rarities[rarityIndex], RequiredLevel: 1 + rarityIndex*5 + (i-1)%5, BaseAttack: int64(i * boolInt(slot == "weapon")), BaseDefense: int64(i * boolInt(slot == "armor")), BaseHealth: int64(i * 3 * boolInt(slot == "armor")), BaseWisdom: int64(i * boolInt(slot == "treasure")), AffixPoolKey: fmt.Sprintf("affix_pool_%d", i%6+1), MinAffixes: rarityIndex, MaxAffixes: rarityIndex + 1, SalvageItem: "soft_clay", SalvageQuantity: int64(1 + rarityIndex), Enabled: true})
		if i >= 19 && i <= 30 {
			s.EquipmentRecipes = append(s.EquipmentRecipes, models.EquipmentRecipeConfig{EquipmentKey: key, BlueprintFragmentItem: key, BlueprintFragments: int64(2 + rarityIndex*2), CurrencyCost: int64(120 + rarityIndex*90), Enabled: true})
			s.EquipmentRecipeMaterials = append(s.EquipmentRecipeMaterials,
				models.EquipmentRecipeMaterialConfig{EquipmentKey: key, ItemName: []string{"ruin_gear", "mist_wood"}[i%2], Quantity: int64(4 + rarityIndex*2)},
				models.EquipmentRecipeMaterialConfig{EquipmentKey: key, ItemName: "soft_clay", Quantity: int64(rarityIndex)})
			bossMaterial := []string{"prairie_horn", "tide_lens", "forest_heart"}[min((i-19)/4, 2)]
			s.EquipmentRecipeMaterials = append(s.EquipmentRecipeMaterials, models.EquipmentRecipeMaterialConfig{EquipmentKey: key, ItemName: bossMaterial, Quantity: 1})
		}
	}
	for i, zone := range s.AdventureZones {
		equipmentWeight := 1
		if i >= 4 {
			equipmentWeight = 2
		}
		if i >= 8 {
			equipmentWeight = 1
		}
		for sidx := 0; sidx < 3; sidx++ {
			// Epic and legendary templates (19-30) are crafting-only; normal pools stop at 18.
			template := s.EquipmentTemplates[min(i*2+sidx, 17)]
			s.AdventureLootEntries = append(s.AdventureLootEntries, models.AdventureLootEntryConfig{PoolKey: zone.Key + "_loot", RewardType: "equipment", RewardKey: template.Key, MinQuantity: 1, MaxQuantity: 1, Weight: equipmentWeight, SortOrder: 30 + sidx})
		}
	}
	blueprintPools := []string{"tide_ruins_z3_loot", "tide_ruins_z4_loot", "mist_crown_forest_z1_loot", "mist_crown_forest_z2_loot", "mist_crown_forest_z3_loot", "mist_crown_forest_z4_loot"}
	for i := 19; i <= 30; i++ {
		pool := blueprintPools[(i-19)/2]
		s.AdventureLootEntries = append(s.AdventureLootEntries, models.AdventureLootEntryConfig{PoolKey: pool, RewardType: "blueprint_fragment", RewardKey: fmt.Sprintf("equipment_%02d", i), MinQuantity: 5, MaxQuantity: 6, Weight: 12, SortOrder: 70 + i})
	}
}

func buildEconomy(s *config.ConfigSnapshot) {
	for i, key := range []string{"field_ration", "morning_berry", "warm_soup", "focus_tea", "bandage", "camp_kit", "energy_biscuit", "mist_antidote", "wind_chime", "shell_music_box", "sunny_postcard", "polished_stone", "feather_toy", "story_book", "wisdom_notes", "strength_band", "defense_pad", "agility_ribbon"} {
		name := itemName(s.Items, key)
		s.ShopItems = append(s.ShopItems, models.ShopItemConfig{ShopType: "shop_normal", Name: name, Stock: -1, Price: int64(36 + i*8), Description: "基础陪伴与调查补给"})
	}
	for day := 1; day <= 7; day++ {
		newbieItems := "调查便当*1"
		if day == 7 {
			newbieItems += "#共鸣之种*1"
		}
		s.CheckinRewards = append(s.CheckinRewards, models.CheckinRewardConfig{Type: "checkin_newbie", Day: fmt.Sprint(day), Currency: int64(80 + day*8), Affection: int64(8 + day/2), Items: newbieItems})
		s.CheckinRewards = append(s.CheckinRewards, models.CheckinRewardConfig{Type: "checkin_weekly", Day: fmt.Sprint(day), Currency: int64(90 + day*5), Affection: 5, Items: "调查便当*1"})
	}
	s.WorkSettings = []models.WorkSettingConfig{{Name: "整理样本", Time: 10, HungerCost: 6, RewardCoin: 160, RewardItems: "调查墨水*1"}, {Name: "维护营地", Time: 25, HungerCost: 10, RewardCoin: 230, RewardItems: "柔韧陶土*1"}, {Name: "协助测绘", Time: 45, HungerCost: 16, RewardCoin: 310, RewardItems: "共建标识*1"}}
	s.Currencies = []models.CurrencyConfig{
		{Key: "primary_coin", Name: "星砂", Description: "陪伴、商店与常规活动使用的主货币", Builtin: true, Enabled: true, SortOrder: 10},
		{Key: "journey_badge", Name: "调查徽章", Description: "跨赛季保留的永久调查货币", Builtin: true, Enabled: true, SortOrder: 20},
		{Key: "season_token", Name: "遗迹季印", Description: "仅在当前赛季有效，赛季结束时重置", Enabled: true, SortOrder: 30},
	}
	for i, key := range []string{"field_ration", "bandage", "survey_ink", "resonance_seed", "dawn_core", "mist_core"} {
		s.AdventureShopItems = append(s.AdventureShopItems, models.AdventureShopItemConfig{Key: "survey_shop_" + key, Name: itemName(s.Items, key), Description: "永久调查徽章兑换商品", ProductType: "item", ProductKey: key, Quantity: 1, Price: int64(8 + i*12), CurrencyKey: "journey_badge", LimitType: []string{"daily", "daily", "weekly", "weekly", "lifetime", "lifetime"}[i], LimitQuantity: int64(3 + i), Enabled: true, SortOrder: i * 10})
	}
	for i, key := range []string{"field_ration", "bandage", "resonance_seed", "survey_badge_pin", "season_memento"} {
		s.AdventureShopItems = append(s.AdventureShopItems, models.AdventureShopItemConfig{Key: "season_shop_" + key, Name: itemName(s.Items, key), Description: "首季遗迹季印兑换商品", ProductType: "item", ProductKey: key, Quantity: 1, Price: int64([]int{4, 6, 30, 45, 60}[i]), CurrencyKey: "season_token", LimitType: "season", LimitQuantity: int64([]int{20, 15, 3, 1, 1}[i]), Enabled: true, SortOrder: 100 + i*10})
	}
	start := time.Date(2026, 9, 1, 0, 0, 0, 0, time.Local)
	s.LiveEvents = []models.LiveEventConfig{{Key: "season_01_nature_ruins", Name: "首季·遗迹初鸣", Region: "三域联合调查", StoryChoices: `["修复观测站","扩建调查网","加固群营地"]`, ProgressSourceMode: "all_expeditions", StartsAt: start, EndsAt: start.Add(70 * 24 * time.Hour), Active: true}}
	for i := 1; i <= 20; i++ {
		rewardType, rewardKey, rewardName := "currency", "season_token", "遗迹季印"
		if i%5 == 0 {
			rewardType = "item"
			rewardKey = []string{"resonance_seed", "dawn_core", "mist_core", "star_core"}[i/5-1]
			rewardName = itemName(s.Items, rewardKey)
		}
		s.RewardTracks = append(s.RewardTracks, models.RewardTrackConfig{EventKey: "season_01_nature_ruins", Milestone: int64(i * 100), RewardType: rewardType, RewardKey: rewardKey, RewardName: rewardName, Quantity: int64(4 + i/5), Description: "首季个人调查里程碑"})
	}
	s.LiveEventChoices = []models.LiveEventChoiceConfig{{EventKey: "season_01_nature_ruins", ChoiceKey: "restore", Label: "修复观测站", EffectType: "expedition_reward_gain_percent", EffectValue: 12, SortOrder: 10}, {EventKey: "season_01_nature_ruins", ChoiceKey: "survey", Label: "扩建调查网", EffectType: "adventure_xp_gain_percent", EffectValue: 12, SortOrder: 20}, {EventKey: "season_01_nature_ruins", ChoiceKey: "defend", Label: "加固群营地", EffectType: "boss_damage_gain_percent", EffectValue: 10, SortOrder: 30}}
	for _, zone := range s.AdventureZones {
		s.LiveEventSources = append(s.LiveEventSources, models.LiveEventExpeditionSourceConfig{EventKey: "season_01_nature_ruins", ZoneKey: zone.Key})
	}
	s.ChanceGames = []models.ChanceGameConfig{{GameKey: "fishing", Name: "生态垂钓", Enabled: true, CostCurrency: 10, DailyLimit: 5, PityThreshold: 5, PityRewardKey: "echo_shell", DurationSecond: 60, Rules: "公开概率，第5次保底收藏品。"}, {GameKey: "lottery", Name: "遗迹抽签", Enabled: true, CostItem: "遗迹抽签券", CostQuantity: 1, DailyLimit: 3, PityThreshold: 10, PityRewardKey: "star_core", Rules: "不售卖抽签券，第10次保底星辉晶核。"}}
	for _, game := range s.ChanceGames {
		for i, key := range []string{"meadow_fiber", "survey_ink", "pressed_flower", "star_core"} {
			s.ChanceRewards = append(s.ChanceRewards, models.ChanceRewardConfig{GameKey: game.GameKey, RewardKey: game.GameKey + "_" + key, Name: itemName(s.Items, key), Weight: []int{60, 28, 10, 2}[i], ItemName: itemName(s.Items, key), Quantity: 1, Rare: i >= 2, Enabled: true, SortOrder: i * 10})
		}
	}
}

func itemName(items []models.ItemConfig, key string) string {
	for _, item := range items {
		if item.Key == key {
			return item.Name
		}
	}
	panic("missing item " + key)
}

func boolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}
func must(err error) {
	if err != nil {
		panic(err)
	}
}
