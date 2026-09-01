package config

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"qq-pet-saas/models"
)

func TestDefaultSnapshotsUseNewbieFriendlyCareCosts(t *testing.T) {
	official, err := LoadOfficialSnapshot()
	if err != nil {
		t.Fatal(err)
	}
	baseRaw, err := os.ReadFile("defaults/base_surface.json")
	if err != nil {
		t.Fatal(err)
	}
	var base ConfigSnapshot
	if err = json.Unmarshal(baseRaw, &base); err != nil {
		t.Fatal(err)
	}
	for name, snapshot := range map[string]ConfigSnapshot{"official": official, "base": base} {
		values := map[string]string{}
		for _, row := range snapshot.System {
			values[row.Key] = row.Value
		}
		if values["Core.InitialCoin"] != "240" || values["Core.TreatCost"] != "100" || values["Core.RenameCost"] != "120" {
			t.Fatalf("%s 默认新手数值不一致: %#v", name, values)
		}
	}
}

func TestOfficialSnapshotMeetsLaunchReadiness(t *testing.T) {
	snapshot, err := LoadOfficialSnapshot()
	if err != nil {
		t.Fatal(err)
	}
	if err = ValidateLaunchReadiness(snapshot); err != nil {
		t.Fatal(err)
	}
}

func TestOfficialSnapshotContainsCompleteNodeAdventureContent(t *testing.T) {
	snapshot, err := LoadOfficialSnapshot()
	if err != nil {
		t.Fatal(err)
	}
	if err = ValidateSnapshot(snapshot); err != nil {
		t.Fatal(err)
	}
	nodeZones := 0
	for _, zone := range snapshot.AdventureZones {
		if zone.ExplorationMode == "node" {
			nodeZones++
		}
	}
	if nodeZones != 12 || len(snapshot.AdventureStages) != 84 || len(snapshot.AdventureStoryEvents) != 36 || len(snapshot.AdventureStoryChoices) != 72 || len(snapshot.AdventureEncounters) != 174 || len(snapshot.AdventureEncounterEffects) != 68 {
		t.Fatalf("节点探索默认数据不完整: zones=%d stages=%d events=%d choices=%d encounters=%d effects=%d", nodeZones, len(snapshot.AdventureStages), len(snapshot.AdventureStoryEvents), len(snapshot.AdventureStoryChoices), len(snapshot.AdventureEncounters), len(snapshot.AdventureEncounterEffects))
	}
	for _, event := range snapshot.AdventureStoryEvents {
		for _, forbidden := range []string{"安全路线", "危险路线", "低风险", "高风险", "稳妥", "冒险"} {
			if strings.Contains(event.Description, forbidden) {
				t.Fatalf("事件 %s 含有剧透文案 %q", event.Key, forbidden)
			}
		}
	}
	for _, choice := range snapshot.AdventureStoryChoices {
		if strings.TrimSpace(choice.Description) != "" {
			t.Fatalf("事件选项 %s/%s 不应配置玩家可见的风险说明", choice.EventKey, choice.ChoiceKey)
		}
	}
}

func TestZoneEquipmentLootDoesNotExceedRecommendedLevel(t *testing.T) {
	snapshot, err := LoadOfficialSnapshot()
	if err != nil {
		t.Fatal(err)
	}
	if err = ValidateSnapshot(snapshot); err != nil {
		t.Fatal(err)
	}
	templates := map[string]models.EquipmentTemplateConfig{}
	for _, row := range snapshot.EquipmentTemplates {
		templates[row.Key] = row
	}
	zones := map[string]models.AdventureZoneConfig{}
	for _, row := range snapshot.AdventureZones {
		zones[row.Key] = row
	}
	var compass *models.EquipmentTemplateConfig
	for i := range snapshot.EquipmentTemplates {
		if snapshot.EquipmentTemplates[i].Name == "晨露罗盘" {
			compass = &snapshot.EquipmentTemplates[i]
			break
		}
	}
	if compass == nil || compass.RequiredLevel > 1 {
		t.Fatalf("萤草坡掉落的晨露罗盘必须 1 级可穿，实际 required_level=%v", compass)
	}
	for _, entry := range snapshot.AdventureLootEntries {
		if entry.RewardType != "equipment" || !strings.HasSuffix(entry.PoolKey, "_loot") {
			continue
		}
		zoneKey := strings.TrimSuffix(entry.PoolKey, "_loot")
		zone, ok := zones[zoneKey]
		if !ok {
			continue
		}
		template, ok := templates[entry.RewardKey]
		if !ok {
			t.Fatalf("奖励池 %s 引用了不存在的装备 %s", entry.PoolKey, entry.RewardKey)
		}
		if template.RequiredLevel > zone.RecommendedLevel {
			t.Fatalf("%s（推荐 %d）掉落 %s 需要等级 %d", zone.Name, zone.RecommendedLevel, template.Name, template.RequiredLevel)
		}
	}
}

func TestAdventureValidationRejectsOverlevelZoneEquipmentLoot(t *testing.T) {
	snapshot, err := LoadOfficialSnapshot()
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for i := range snapshot.EquipmentTemplates {
		if snapshot.EquipmentTemplates[i].Name == "晨露罗盘" {
			snapshot.EquipmentTemplates[i].RequiredLevel = 99
			found = true
		}
	}
	if !found {
		t.Fatal("官方快照缺少晨露罗盘")
	}
	err = ValidateSnapshot(snapshot)
	if err == nil || (!strings.Contains(err.Error(), "晨露罗盘") && !strings.Contains(err.Error(), "equipment_03")) {
		t.Fatalf("高于区域推荐等级的掉落应被拦截，实际 %v", err)
	}
}

func TestAdventureValidationRejectsNodeEventWithoutEnoughChoices(t *testing.T) {
	snapshot := ConfigSnapshot{
		AdventureMaps:         []models.AdventureMapConfig{{Key: "node-map", Name: "节点地图", Region: "试炼", RecommendedLevel: 1}},
		AdventureZones:        []models.AdventureZoneConfig{{Key: "node-zone", MapKey: "node-map", Name: "节点区域", RecommendedLevel: 1, DifficultyPermille: 1000, ExplorationMode: "node"}},
		AdventureStages:       []models.AdventureExplorationStageConfig{{Key: "node-stage", ZoneKey: "node-zone", Name: "开始调查", ProgressStart: 0, ProgressEnd: 100, EventKey: "node-event"}},
		AdventureStoryEvents:  []models.AdventureStoryEventConfig{{Key: "node-event", ZoneKey: "node-zone", StageKey: "node-stage", Name: "调查选择", EventType: "mainline", Weight: 1}},
		AdventureStoryChoices: []models.AdventureStoryEventChoiceConfig{{EventKey: "node-event", ChoiceKey: "only-choice", Label: "唯一选择", RiskLevel: "low"}},
	}
	if err := validateAdventureSnapshot(snapshot, map[string]bool{}, map[string]bool{}); err == nil || !strings.Contains(err.Error(), "必须配置 2 到 3 个选项") {
		t.Fatalf("节点主线事件缺少选项应被拦截，实际 %v", err)
	}
}

func TestLaunchReadinessRejectsSeasonTokenItem(t *testing.T) {
	snapshot, err := LoadOfficialSnapshot()
	if err != nil {
		t.Fatal(err)
	}
	snapshot.Items = append(snapshot.Items, snapshot.Items[0])
	snapshot.Items[len(snapshot.Items)-1].Key = "season_token"
	if err = ValidateLaunchReadiness(snapshot); err == nil {
		t.Fatal("expected season_token item to fail")
	}
}

func TestLaunchReadinessRejectsMissingMap(t *testing.T) {
	snapshot, err := LoadOfficialSnapshot()
	if err != nil {
		t.Fatal(err)
	}
	snapshot.AdventureMaps = snapshot.AdventureMaps[:2]
	if err = ValidateLaunchReadiness(snapshot); err == nil {
		t.Fatal("expected missing map to fail")
	}
}

func TestLaunchReadinessRejectsMissingCurrencySource(t *testing.T) {
	snapshot, err := LoadOfficialSnapshot()
	if err != nil {
		t.Fatal(err)
	}
	filtered := make([]models.AdventureLootEntryConfig, 0, len(snapshot.AdventureLootEntries))
	for _, entry := range snapshot.AdventureLootEntries {
		if entry.RewardType == "currency" && entry.RewardKey == "journey_badge" {
			continue
		}
		filtered = append(filtered, entry)
	}
	snapshot.AdventureLootEntries = filtered
	if err = ValidateLaunchReadiness(snapshot); err == nil {
		t.Fatal("expected missing journey_badge source to fail")
	}
}

func TestLaunchReadinessRejectsMissingEliteEncounter(t *testing.T) {
	snapshot, err := LoadOfficialSnapshot()
	if err != nil {
		t.Fatal(err)
	}
	for i := range snapshot.AdventureMonsters {
		snapshot.AdventureMonsters[i].Elite = false
	}
	if err = ValidateLaunchReadiness(snapshot); err == nil {
		t.Fatal("expected missing elite encounter to fail")
	}
}

func TestLaunchReadinessRejectsMissingSeasonShop(t *testing.T) {
	snapshot, err := LoadOfficialSnapshot()
	if err != nil {
		t.Fatal(err)
	}
	filtered := make([]models.AdventureShopItemConfig, 0, len(snapshot.AdventureShopItems))
	for _, listing := range snapshot.AdventureShopItems {
		if listing.CurrencyKey == "season_token" {
			continue
		}
		filtered = append(filtered, listing)
	}
	snapshot.AdventureShopItems = filtered
	if err = ValidateLaunchReadiness(snapshot); err == nil {
		t.Fatal("expected missing season shop to fail")
	}
}

func TestLaunchReadinessRejectsUnsourcedItem(t *testing.T) {
	snapshot, err := LoadOfficialSnapshot()
	if err != nil {
		t.Fatal(err)
	}
	filtered := snapshot.AdventureLootEntries[:0]
	for _, entry := range snapshot.AdventureLootEntries {
		if entry.RewardType == "item" && entry.RewardKey == "clear_dew" {
			continue
		}
		filtered = append(filtered, entry)
	}
	snapshot.AdventureLootEntries = filtered
	if err = ValidateLaunchReadiness(snapshot); err == nil {
		t.Fatal("expected unsourced material to fail")
	}
}

func TestLaunchReadinessRejectsRecipeWithoutBlueprintSource(t *testing.T) {
	snapshot, err := LoadOfficialSnapshot()
	if err != nil {
		t.Fatal(err)
	}
	filtered := snapshot.AdventureLootEntries[:0]
	for _, entry := range snapshot.AdventureLootEntries {
		if entry.RewardType == "blueprint_fragment" && entry.RewardKey == "equipment_19" {
			continue
		}
		filtered = append(filtered, entry)
	}
	snapshot.AdventureLootEntries = filtered
	if err = ValidateLaunchReadiness(snapshot); err == nil {
		t.Fatal("expected recipe without blueprint source to fail")
	}
}

func TestLaunchReadinessRejectsNumberedPlaceholderContent(t *testing.T) {
	snapshot, err := LoadOfficialSnapshot()
	if err != nil {
		t.Fatal(err)
	}
	snapshot.AdventureSkills[0].Name = "调查战技·01"
	if err = ValidateLaunchReadiness(snapshot); err == nil {
		t.Fatal("expected numbered placeholder skill to fail")
	}
}

func TestRequireLiveLaunchReadinessRejectsIncompleteDatabase(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err = db.AutoMigrate(
		&models.SystemConfig{}, &models.CommandConfig{}, &models.PetSpeciesConfig{}, &models.PetEvolutionRuleConfig{},
		&models.PetEvolutionCostConfig{}, &models.PetSkillUnlockConfig{}, &models.AdventureLevelConfig{}, &models.ItemConfig{}, &models.ShopItemConfig{},
		&models.CheckinRewardConfig{}, &models.WorkSettingConfig{}, &models.MenuConfig{}, &models.ImageConfig{}, &models.LiveEventConfig{},
		&models.RewardTrackConfig{}, &models.GrowthRoleConfig{}, &models.GrowthStanceConfig{}, &models.PersonalityRuleConfig{},
		&models.CodexCatalogConfig{}, &models.ExpeditionTemplateConfig{}, &models.ChanceGameConfig{}, &models.ChanceRewardConfig{},
		&models.AdventureMapConfig{}, &models.AdventureZoneConfig{}, &models.AdventureZonePrerequisiteConfig{},
		&models.AdventureObjectiveConfig{}, &models.AdventureExplorationStageConfig{}, &models.AdventureStoryEventConfig{}, &models.AdventureStoryEventChoiceConfig{},
		&models.AdventureMonsterConfig{}, &models.AdventureSkillConfig{},
		&models.AdventureMonsterSkillConfig{}, &models.AdventureEncounterConfig{}, &models.AdventureEncounterEffectConfig{}, &models.AdventureLootPoolConfig{},
		&models.AdventureLootEntryConfig{}, &models.CurrencyConfig{},
		&models.AdventureShopItemConfig{}, &models.AdventureExpeditionConfig{}, &models.AdventureBossConfig{},
		&models.AdventureBossRewardTierConfig{}, &models.EquipmentTemplateConfig{}, &models.EquipmentAffixConfig{},
		&models.EquipmentRecipeConfig{}, &models.EquipmentRecipeMaterialConfig{}, &models.LiveEventChoiceConfig{},
		&models.LiveEventExpeditionSourceConfig{},
	); err != nil {
		t.Fatal(err)
	}
	if err = db.Create(&models.AdventureMapConfig{Key: "only-map", Name: "唯一地图", Enabled: true}).Error; err != nil {
		t.Fatal(err)
	}
	err = RequireLiveLaunchReadiness(db)
	if err == nil || !strings.Contains(err.Error(), "需要 3 张永久地图") {
		t.Fatalf("残缺库应被启动完整度拦住，实际 %v", err)
	}
}
