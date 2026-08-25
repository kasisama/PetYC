package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"qq-pet-saas/models"
)

const (
	ProfileSchemaVersion = 1
	ApplicationVersion   = "0.0.1"
	OfficialProfileID    = "00000000-0000-0000-0000-000000000001"
)

var LocalSystemKeys = map[string]struct{}{
	"Core.MasterQQ": {}, "Core.NotifyQQ": {}, "Core.ImageHost": {},
	"Core.CurrencySync": {}, "Core.CurrencySyncPath": {},
	"Core.CurrencySyncSection": {}, "Core.CurrencySyncKey": {},
}

// ConfigSnapshot is the only configuration contract accepted by profiles and
// package import/export. Operational and player tables are intentionally absent.
type ConfigSnapshot struct {
	System                   []models.SystemConfig                    `json:"system"`
	Commands                 []models.CommandConfig                   `json:"commands"`
	PetSpecies               []models.PetSpeciesConfig                `json:"pet_species"`
	Items                    []models.ItemConfig                      `json:"items"`
	ShopItems                []models.ShopItemConfig                  `json:"shop_items"`
	CheckinRewards           []models.CheckinRewardConfig             `json:"checkin_rewards"`
	WorkSettings             []models.WorkSettingConfig               `json:"work_settings"`
	Menus                    []models.MenuConfig                      `json:"menus"`
	Images                   []models.ImageConfig                     `json:"images"`
	LiveEvents               []models.LiveEventConfig                 `json:"live_events"`
	RewardTracks             []models.RewardTrackConfig               `json:"reward_tracks"`
	GrowthRoles              []models.GrowthRoleConfig                `json:"growth_roles"`
	GrowthStances            []models.GrowthStanceConfig              `json:"growth_stances"`
	PersonalityRules         []models.PersonalityRuleConfig           `json:"personality_rules"`
	CodexCatalog             []models.CodexCatalogConfig              `json:"codex_catalog"`
	ExpeditionTemplates      []models.ExpeditionTemplateConfig        `json:"expedition_templates"`
	ChanceGames              []models.ChanceGameConfig                `json:"chance_games"`
	ChanceRewards            []models.ChanceRewardConfig              `json:"chance_rewards"`
	AdventureMaps            []models.AdventureMapConfig              `json:"adventure_maps"`
	AdventureZones           []models.AdventureZoneConfig             `json:"adventure_zones"`
	AdventurePrereqs         []models.AdventureZonePrerequisiteConfig `json:"adventure_zone_prerequisites"`
	AdventureObjectives      []models.AdventureObjectiveConfig        `json:"adventure_objectives"`
	AdventureMonsters        []models.AdventureMonsterConfig          `json:"adventure_monsters"`
	AdventureSkills          []models.AdventureSkillConfig            `json:"adventure_skills"`
	AdventureMonsterSkills   []models.AdventureMonsterSkillConfig     `json:"adventure_monster_skills"`
	AdventureEncounters      []models.AdventureEncounterConfig        `json:"adventure_encounters"`
	AdventureLootPools       []models.AdventureLootPoolConfig         `json:"adventure_loot_pools"`
	AdventureLootEntries     []models.AdventureLootEntryConfig        `json:"adventure_loot_entries"`
	AdventureExpeditions     []models.AdventureExpeditionConfig       `json:"adventure_expeditions"`
	AdventureBosses          []models.AdventureBossConfig             `json:"adventure_bosses"`
	AdventureBossRewardTiers []models.AdventureBossRewardTierConfig   `json:"adventure_boss_reward_tiers"`
	EquipmentTemplates       []models.EquipmentTemplateConfig         `json:"equipment_templates"`
	EquipmentAffixes         []models.EquipmentAffixConfig            `json:"equipment_affixes"`
	EquipmentRecipes         []models.EquipmentRecipeConfig           `json:"equipment_recipes"`
	EquipmentRecipeMaterials []models.EquipmentRecipeMaterialConfig   `json:"equipment_recipe_materials"`
	LiveEventChoices         []models.LiveEventChoiceConfig           `json:"live_event_choices"`
	LiveEventSources         []models.LiveEventExpeditionSourceConfig `json:"live_event_sources"`
}

type SnapshotSummary struct {
	Schemas int `json:"schemas"`
	Rows    int `json:"rows"`
}

type CompatibilityConflict struct {
	Kind          string   `json:"kind"`
	MissingKeys   []string `json:"missing_keys"`
	AffectedCount int64    `json:"affected_count"`
}

func (snapshot ConfigSnapshot) Summary() SnapshotSummary {
	counts := []int{len(snapshot.System), len(snapshot.Commands), len(snapshot.PetSpecies), len(snapshot.Items), len(snapshot.ShopItems), len(snapshot.CheckinRewards), len(snapshot.WorkSettings), len(snapshot.Menus), len(snapshot.Images), len(snapshot.LiveEvents), len(snapshot.RewardTracks), len(snapshot.GrowthRoles), len(snapshot.GrowthStances), len(snapshot.PersonalityRules), len(snapshot.CodexCatalog), len(snapshot.ExpeditionTemplates), len(snapshot.ChanceGames), len(snapshot.ChanceRewards), len(snapshot.AdventureMaps), len(snapshot.AdventureZones), len(snapshot.AdventurePrereqs), len(snapshot.AdventureObjectives), len(snapshot.AdventureMonsters), len(snapshot.AdventureSkills), len(snapshot.AdventureMonsterSkills), len(snapshot.AdventureEncounters), len(snapshot.AdventureLootPools), len(snapshot.AdventureLootEntries), len(snapshot.AdventureExpeditions), len(snapshot.AdventureBosses), len(snapshot.AdventureBossRewardTiers), len(snapshot.EquipmentTemplates), len(snapshot.EquipmentAffixes), len(snapshot.EquipmentRecipes), len(snapshot.EquipmentRecipeMaterials), len(snapshot.LiveEventChoices), len(snapshot.LiveEventSources)}
	rows, schemas := 0, 0
	for _, count := range counts {
		if count > 0 {
			schemas++
		}
		rows += count
	}
	return SnapshotSummary{Schemas: schemas, Rows: rows}
}

func CaptureSnapshot(db *gorm.DB) (ConfigSnapshot, error) {
	var snapshot ConfigSnapshot
	queries := []struct {
		target any
		order  string
	}{
		{&snapshot.Commands, "sort_order asc, func_name asc"}, {&snapshot.PetSpecies, "name asc"},
		{&snapshot.Items, "name asc"}, {&snapshot.ShopItems, "id asc"},
		{&snapshot.CheckinRewards, "type asc, day asc, id asc"}, {&snapshot.WorkSettings, "name asc"},
		{&snapshot.Menus, "name asc"}, {&snapshot.Images, "name asc"},
		{&snapshot.LiveEvents, "starts_at asc, key asc"}, {&snapshot.RewardTracks, "event_key asc, milestone asc, item_name asc"},
		{&snapshot.GrowthRoles, "sort_order asc, name asc"}, {&snapshot.GrowthStances, "sort_order asc, name asc"},
		{&snapshot.PersonalityRules, "sort_order asc, name asc"}, {&snapshot.CodexCatalog, "sort_order asc, category asc, entry_key asc"},
		{&snapshot.ExpeditionTemplates, "tier asc"}, {&snapshot.ChanceGames, "game_key asc"},
		{&snapshot.ChanceRewards, "game_key asc, sort_order asc, reward_key asc"},
		{&snapshot.AdventureMaps, "sort_order asc, key asc"}, {&snapshot.AdventureZones, "map_key asc, sort_order asc, key asc"},
		{&snapshot.AdventurePrereqs, "zone_key asc, prerequisite_zone_key asc"}, {&snapshot.AdventureObjectives, "zone_key asc, sort_order asc, key asc"},
		{&snapshot.AdventureMonsters, "key asc"}, {&snapshot.AdventureSkills, "key asc"},
		{&snapshot.AdventureMonsterSkills, "monster_key asc, sort_order asc, skill_key asc"}, {&snapshot.AdventureEncounters, "zone_key asc, sort_order asc, encounter_key asc"},
		{&snapshot.AdventureLootPools, "key asc"}, {&snapshot.AdventureLootEntries, "pool_key asc, sort_order asc, id asc"},
		{&snapshot.AdventureExpeditions, "zone_key asc"}, {&snapshot.AdventureBosses, "map_key asc, zone_key asc, key asc"},
		{&snapshot.AdventureBossRewardTiers, "boss_key asc, threshold asc"}, {&snapshot.EquipmentTemplates, "slot asc, rarity asc, key asc"},
		{&snapshot.EquipmentAffixes, "pool_key asc, key asc"}, {&snapshot.EquipmentRecipes, "equipment_key asc"},
		{&snapshot.EquipmentRecipeMaterials, "equipment_key asc, item_name asc"}, {&snapshot.LiveEventChoices, "event_key asc, sort_order asc, choice_key asc"},
		{&snapshot.LiveEventSources, "event_key asc, zone_key asc"},
	}
	localKeys := make([]string, 0, len(LocalSystemKeys))
	for key := range LocalSystemKeys {
		localKeys = append(localKeys, key)
	}
	if err := db.Where("key NOT IN ?", localKeys).Order("key asc").Find(&snapshot.System).Error; err != nil {
		return snapshot, err
	}
	for _, query := range queries {
		if err := db.Order(query.order).Find(query.target).Error; err != nil {
			return snapshot, err
		}
	}
	return snapshot, nil
}

func EncodeSnapshot(snapshot ConfigSnapshot) (string, error) {
	if err := ValidateSnapshot(snapshot); err != nil {
		return "", err
	}
	raw, err := json.Marshal(snapshot)
	return string(raw), err
}

func DecodeSnapshot(payload string) (ConfigSnapshot, error) {
	var snapshot ConfigSnapshot
	decoder := json.NewDecoder(strings.NewReader(payload))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&snapshot); err != nil {
		return snapshot, fmt.Errorf("配置数据格式无效: %w", err)
	}
	if err := ValidateSnapshot(snapshot); err != nil {
		return snapshot, err
	}
	return snapshot, nil
}

func ValidateSnapshot(snapshot ConfigSnapshot) error {
	if len(snapshot.System) == 0 || len(snapshot.Commands) == 0 || len(snapshot.PetSpecies) == 0 || len(snapshot.Items) == 0 {
		return errors.New("配置方案缺少系统、指令、宠物或物品基础配置")
	}
	items, pets, events, games, rewards := map[string]bool{}, map[string]bool{}, map[string]bool{}, map[string]bool{}, map[string]bool{}
	for _, row := range snapshot.System {
		if strings.TrimSpace(row.Key) == "" {
			return errors.New("系统配置键不能为空")
		}
		if _, local := LocalSystemKeys[row.Key]; local {
			return fmt.Errorf("方案不能包含本机配置项 %s", row.Key)
		}
	}
	for _, row := range snapshot.Items {
		key := strings.TrimSpace(row.Name)
		if key == "" || items[key] {
			return fmt.Errorf("物品名称为空或重复: %s", key)
		}
		items[key] = true
	}
	for _, row := range snapshot.PetSpecies {
		key := strings.TrimSpace(row.Name)
		if key == "" || pets[key] {
			return fmt.Errorf("宠物名称为空或重复: %s", key)
		}
		pets[key] = true
		if err := validateItemList(row.AwakenItems, items, "宠物觉醒 "+key); err != nil {
			return err
		}
	}
	for _, row := range snapshot.LiveEvents {
		key := strings.TrimSpace(row.Key)
		if key == "" || events[key] || !row.StartsAt.Before(row.EndsAt) {
			return fmt.Errorf("活动配置无效: %s", key)
		}
		events[key] = true
	}
	for _, row := range snapshot.ChanceGames {
		key := strings.TrimSpace(row.GameKey)
		if key == "" || games[key] {
			return fmt.Errorf("概率玩法键为空或重复: %s", key)
		}
		games[key] = true
		if row.CostItem != "" && !items[row.CostItem] {
			return fmt.Errorf("概率玩法 %s 引用了不存在的物品 %s", key, row.CostItem)
		}
	}
	for _, row := range snapshot.ChanceRewards {
		key := row.GameKey + ":" + row.RewardKey
		if !games[row.GameKey] || rewards[key] {
			return fmt.Errorf("概率奖励配置无效: %s", key)
		}
		rewards[key] = true
		if row.Quantity > 0 && !items[row.ItemName] {
			return fmt.Errorf("概率奖励 %s 引用了不存在的物品 %s", key, row.ItemName)
		}
	}
	for _, row := range snapshot.ShopItems {
		if !items[row.Name] {
			return fmt.Errorf("商店引用了不存在的物品 %s", row.Name)
		}
	}
	for _, row := range snapshot.CheckinRewards {
		if err := validateItemList(row.Items, items, "签到奖励"); err != nil {
			return err
		}
	}
	for _, row := range snapshot.WorkSettings {
		if err := validateItemList(row.RewardItems, items, "打工奖励 "+row.Name); err != nil {
			return err
		}
	}
	for _, row := range snapshot.RewardTracks {
		if !events[row.EventKey] {
			return fmt.Errorf("奖励轨道引用了不存在的活动 %s", row.EventKey)
		}
		if !items[row.ItemName] {
			return fmt.Errorf("奖励轨道引用了不存在的物品 %s", row.ItemName)
		}
	}
	for _, row := range snapshot.ExpeditionTemplates {
		if row.RewardItem != "" && !items[row.RewardItem] {
			return fmt.Errorf("远征模板引用了不存在的奖励物品 %s", row.RewardItem)
		}
		if row.RequiredItem != "" && !items[row.RequiredItem] {
			return fmt.Errorf("远征模板引用了不存在的消耗物品 %s", row.RequiredItem)
		}
	}
	for _, path := range snapshotImageReferences(snapshot) {
		normalized := filepath.ToSlash(strings.TrimSpace(path))
		if strings.Contains(normalized, "://") || normalized == "" {
			continue
		}
		if filepath.IsAbs(path) || normalized == ".." || strings.HasPrefix(normalized, "../") || strings.Contains(normalized, "/../") {
			return fmt.Errorf("图片引用路径不安全: %s", path)
		}
	}
	if err := validateAdventureSnapshot(snapshot, items, events); err != nil {
		return err
	}
	return nil
}

func validateItemList(raw string, items map[string]bool, owner string) error {
	for _, entry := range strings.Split(raw, "#") {
		name := strings.TrimSpace(strings.SplitN(strings.ReplaceAll(entry, "×", "*"), "*", 2)[0])
		if name != "" && !items[name] {
			return fmt.Errorf("%s 引用了不存在的物品 %s", owner, name)
		}
	}
	return nil
}

func snapshotImageReferences(snapshot ConfigSnapshot) []string {
	result := make([]string, 0)
	add := func(values ...string) { result = append(result, values...) }
	for _, row := range snapshot.PetSpecies {
		add(row.Image, row.AdoptImage, row.TrainStartImg, row.TrainEndImg, row.StudyStartImg, row.StudyEndImg, row.FitnessStartImg, row.FitnessEndImg, row.EvolutionImage, row.AwakenImage)
	}
	for _, row := range snapshot.Items {
		add(row.Image)
	}
	for _, row := range snapshot.ShopItems {
		add(row.Image)
	}
	for _, row := range snapshot.CheckinRewards {
		add(row.Image)
	}
	for _, row := range snapshot.WorkSettings {
		add(row.StartImage, row.EndImage)
	}
	for _, row := range snapshot.Images {
		add(row.Path)
	}
	for _, row := range snapshot.ExpeditionTemplates {
		add(row.StartImage, row.EndImage)
	}
	for _, row := range snapshot.AdventureMaps {
		add(row.Image)
	}
	for _, row := range snapshot.AdventureZones {
		add(row.Image)
	}
	for _, row := range snapshot.AdventureMonsters {
		add(row.Image)
	}
	for _, row := range snapshot.AdventureExpeditions {
		add(row.StartImage, row.EndImage)
	}
	for _, row := range snapshot.EquipmentTemplates {
		add(row.Image)
	}
	return result
}

func ApplySnapshot(tx *gorm.DB, snapshot ConfigSnapshot) error {
	if err := ValidateSnapshot(snapshot); err != nil {
		return err
	}
	localRows := make([]models.SystemConfig, 0)
	localKeys := make([]string, 0, len(LocalSystemKeys))
	for key := range LocalSystemKeys {
		localKeys = append(localKeys, key)
	}
	if err := tx.Where("key IN ?", localKeys).Find(&localRows).Error; err != nil {
		return err
	}
	tables := []any{&models.SystemConfig{}, &models.CommandConfig{}, &models.PetSpeciesConfig{}, &models.ItemConfig{}, &models.ShopItemConfig{}, &models.CheckinRewardConfig{}, &models.WorkSettingConfig{}, &models.MenuConfig{}, &models.ImageConfig{}, &models.RewardTrackConfig{}, &models.LiveEventExpeditionSourceConfig{}, &models.LiveEventChoiceConfig{}, &models.LiveEventConfig{}, &models.GrowthRoleConfig{}, &models.GrowthStanceConfig{}, &models.PersonalityRuleConfig{}, &models.CodexCatalogConfig{}, &models.ExpeditionTemplateConfig{}, &models.ChanceRewardConfig{}, &models.ChanceGameConfig{}, &models.EquipmentRecipeMaterialConfig{}, &models.EquipmentRecipeConfig{}, &models.EquipmentAffixConfig{}, &models.EquipmentTemplateConfig{}, &models.AdventureBossRewardTierConfig{}, &models.AdventureBossConfig{}, &models.AdventureExpeditionConfig{}, &models.AdventureLootEntryConfig{}, &models.AdventureLootPoolConfig{}, &models.AdventureEncounterConfig{}, &models.AdventureMonsterSkillConfig{}, &models.AdventureSkillConfig{}, &models.AdventureMonsterConfig{}, &models.AdventureObjectiveConfig{}, &models.AdventureZonePrerequisiteConfig{}, &models.AdventureZoneConfig{}, &models.AdventureMapConfig{}}
	for _, model := range tables {
		if err := tx.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(model).Error; err != nil {
			return err
		}
	}
	rows := []any{snapshot.System, snapshot.Commands, snapshot.PetSpecies, snapshot.Items, snapshot.ShopItems, snapshot.CheckinRewards, snapshot.WorkSettings, snapshot.Menus, snapshot.Images, snapshot.LiveEvents, snapshot.RewardTracks, snapshot.GrowthRoles, snapshot.GrowthStances, snapshot.PersonalityRules, snapshot.CodexCatalog, snapshot.ExpeditionTemplates, snapshot.ChanceGames, snapshot.ChanceRewards, snapshot.AdventureMaps, snapshot.AdventureZones, snapshot.AdventurePrereqs, snapshot.AdventureObjectives, snapshot.AdventureMonsters, snapshot.AdventureSkills, snapshot.AdventureMonsterSkills, snapshot.AdventureEncounters, snapshot.AdventureLootPools, snapshot.AdventureLootEntries, snapshot.AdventureExpeditions, snapshot.AdventureBosses, snapshot.AdventureBossRewardTiers, snapshot.EquipmentTemplates, snapshot.EquipmentAffixes, snapshot.EquipmentRecipes, snapshot.EquipmentRecipeMaterials, snapshot.LiveEventChoices, snapshot.LiveEventSources}
	for _, value := range rows {
		raw, _ := json.Marshal(value)
		if string(raw) != "[]" && string(raw) != "null" {
			if err := tx.Create(value).Error; err != nil {
				return err
			}
		}
	}
	if len(localRows) > 0 {
		if err := tx.Create(&localRows).Error; err != nil {
			return err
		}
	}
	return nil
}

func CheckSnapshotCompatibility(db *gorm.DB, snapshot ConfigSnapshot) ([]CompatibilityConflict, error) {
	pets, items, events := map[string]bool{}, map[string]bool{}, map[string]bool{}
	zones, monsters, equipment, bosses := map[string]bool{}, map[string]bool{}, map[string]bool{}, map[string]bool{}
	for _, row := range snapshot.PetSpecies {
		pets[row.Name] = true
	}
	for _, row := range snapshot.Items {
		items[row.Name] = true
	}
	for _, row := range snapshot.LiveEvents {
		events[row.Key] = true
	}
	for _, row := range snapshot.AdventureZones {
		zones[row.Key] = true
	}
	for _, row := range snapshot.AdventureMonsters {
		monsters[row.Key] = true
	}
	for _, row := range snapshot.EquipmentTemplates {
		equipment[row.Key] = true
	}
	for _, row := range snapshot.AdventureBosses {
		bosses[row.Key] = true
	}
	type conflictAccumulator struct {
		missing map[string]bool
		count   int64
	}
	accumulated := map[string]*conflictAccumulator{}
	checks := []struct {
		kind, table, column, condition string
		allowed                        map[string]bool
	}{
		{"pet_species", "pet_profiles", "pet_type", "pet_type <> ''", pets},
		{"items", "global_inventory_items", "item_name", "quantity > 0", items},
		{"items", "activity_runs", "input_item", "claimed_at IS NULL AND input_item <> ''", items},
		{"items", "expedition_runs", "reward_item", "claimed_at IS NULL AND reward_item <> ''", items},
		{"items", "expedition_runs", "required_item", "claimed_at IS NULL AND required_item <> ''", items},
		{"items", "trade_offers", "item_name", "status = 'open' AND item_name <> ''", items},
		{"items", "fishing_runs", "item_name", "claimed_at IS NULL AND item_name <> ''", items},
		{"live_events", "event_progresses", "event_key", "event_key <> ''", events},
		{"adventure_zones", "adventure_exploration_sessions", "zone_key", "status = 'active'", zones},
		{"adventure_zones", "adventure_expedition_runs", "zone_key", "status = 'running'", zones},
		{"adventure_monsters", "adventure_combat_sessions", "monster_key", "status = 'active'", monsters},
		{"equipment_templates", "player_equipments", "template_key", "template_key <> ''", equipment},
		{"adventure_bosses", "adventure_boss_instances", "boss_key", "status = 'active'", bosses},
		{"live_events", "adventure_expedition_runs", "event_key", "status = 'running' AND event_key <> ''", events},
	}
	for _, check := range checks {
		values := make([]string, 0)
		if err := db.Table(check.table).Where(check.condition).Distinct(check.column).Pluck(check.column, &values).Error; err != nil {
			return nil, err
		}
		missing := make([]string, 0)
		for _, value := range values {
			if value != "" && !check.allowed[value] {
				missing = append(missing, value)
			}
		}
		if len(missing) == 0 {
			continue
		}
		sort.Strings(missing)
		var count int64
		if err := db.Table(check.table).Where(check.condition).Where(check.column+" IN ?", missing).Count(&count).Error; err != nil {
			return nil, err
		}
		entry := accumulated[check.kind]
		if entry == nil {
			entry = &conflictAccumulator{missing: map[string]bool{}}
			accumulated[check.kind] = entry
		}
		entry.count += count
		for _, key := range missing {
			entry.missing[key] = true
		}
	}
	var pendingActivities []struct{ RewardItems string }
	if err := db.Table("activity_runs").Select("reward_items").Where("claimed_at IS NULL AND reward_items <> ''").Scan(&pendingActivities).Error; err != nil {
		return nil, err
	}
	for _, activity := range pendingActivities {
		rowMissing := map[string]bool{}
		for _, raw := range strings.Split(activity.RewardItems, "#") {
			name := strings.TrimSpace(strings.SplitN(strings.ReplaceAll(raw, "×", "*"), "*", 2)[0])
			if name != "" && !items[name] {
				rowMissing[name] = true
			}
		}
		if len(rowMissing) == 0 {
			continue
		}
		entry := accumulated["items"]
		if entry == nil {
			entry = &conflictAccumulator{missing: map[string]bool{}}
			accumulated["items"] = entry
		}
		entry.count++
		for key := range rowMissing {
			entry.missing[key] = true
		}
	}
	conflicts := make([]CompatibilityConflict, 0, len(accumulated))
	for kind, entry := range accumulated {
		missing := make([]string, 0, len(entry.missing))
		for key := range entry.missing {
			missing = append(missing, key)
		}
		sort.Strings(missing)
		conflicts = append(conflicts, CompatibilityConflict{Kind: kind, MissingKeys: missing, AffectedCount: entry.count})
	}
	sort.Slice(conflicts, func(i, j int) bool { return conflicts[i].Kind < conflicts[j].Kind })
	return conflicts, nil
}

func CreateProfileFromSnapshot(db *gorm.DB, name, description, source string, builtin bool, snapshot ConfigSnapshot) (models.ConfigProfile, error) {
	payload, err := EncodeSnapshot(snapshot)
	if err != nil {
		return models.ConfigProfile{}, err
	}
	profile := models.ConfigProfile{ID: uuid.NewString(), Name: strings.TrimSpace(name), Description: strings.TrimSpace(description), Source: source, SchemaVersion: ProfileSchemaVersion, AppVersion: ApplicationVersion, Payload: payload, Builtin: builtin}
	if profile.Name == "" {
		return profile, errors.New("方案名称不能为空")
	}
	if err := db.Create(&profile).Error; err != nil {
		return profile, err
	}
	return profile, nil
}

func SetActiveProfile(tx *gorm.DB, profileID string, dirty bool) error {
	state, err := getOrCreateConfigState(tx)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	return tx.Model(state).Updates(map[string]any{"active_profile_id": profileID, "profile_dirty": dirty, "profile_switched_at": now, "db_revision": gorm.Expr("db_revision + 1"), "loaded_revision": gorm.Expr("db_revision + 1"), "saved_at": now, "loaded_at": now}).Error
}
