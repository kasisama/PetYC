package admin

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	appconfig "qq-pet-saas/config"
	"qq-pet-saas/models"
)

type configValidationError struct{ message string }

func (err configValidationError) Error() string { return err.message }

func validateDomainConfig(name string, payload json.RawMessage) error {
	switch name {
	case "commands":
		var rows []models.CommandConfig
		if err := json.Unmarshal(payload, &rows); err != nil {
			return err
		}
		seen := make(map[string]string)
		for index, row := range rows {
			if strings.TrimSpace(row.FuncName) == "" || strings.TrimSpace(row.Command) == "" || strings.TrimSpace(row.DisplayName) == "" || strings.TrimSpace(row.Category) == "" || strings.TrimSpace(row.Description) == "" {
				return configValidationError{fmt.Sprintf("第 %d 个命令的信息不完整", index+1)}
			}
			if !row.Enabled {
				continue
			}
			trigger := strings.TrimSpace(row.Command)
			if previous, exists := seen[trigger]; exists {
				return configValidationError{fmt.Sprintf("触发词“%s”同时用于%s和%s", trigger, previous, row.DisplayName)}
			}
			seen[trigger] = row.DisplayName
		}
	case "menus":
		var rows []models.MenuConfig
		if err := json.Unmarshal(payload, &rows); err != nil {
			return err
		}
		seen := make(map[string]struct{}, len(rows))
		for index, row := range rows {
			name := strings.TrimSpace(row.Name)
			if name == "" {
				return configValidationError{fmt.Sprintf("第 %d 个菜单场景缺少名称", index+1)}
			}
			if strings.TrimSpace(row.Reply) == "" {
				return configValidationError{fmt.Sprintf("菜单场景“%s”缺少机器人回复", name)}
			}
			if _, exists := seen[name]; exists {
				return configValidationError{fmt.Sprintf("菜单场景名称重复: %s", name)}
			}
			seen[name] = struct{}{}
		}
	case "live_events":
		var rows []models.LiveEventConfig
		if err := json.Unmarshal(payload, &rows); err != nil {
			return err
		}
		for index := range rows {
			row := rows[index]
			if strings.TrimSpace(row.Key) == "" || strings.TrimSpace(row.Name) == "" || strings.TrimSpace(row.Region) == "" || !row.StartsAt.Before(row.EndsAt) {
				return configValidationError{fmt.Sprintf("第 %d 个活动的名称、区域或时间无效", index+1)}
			}
			var choices []string
			if err := json.Unmarshal([]byte(row.StoryChoices), &choices); err != nil || len(choices) < 2 || len(choices) > 5 {
				return configValidationError{fmt.Sprintf("活动 %s 必须配置 2 到 5 个故事选项", row.Key)}
			}
			for _, choice := range choices {
				if strings.TrimSpace(choice) == "" {
					return configValidationError{fmt.Sprintf("活动 %s 的故事选项不能为空", row.Key)}
				}
			}
			for otherIndex := 0; otherIndex < index; otherIndex++ {
				other := rows[otherIndex]
				if row.Active && other.Active && row.StartsAt.Before(other.EndsAt) && other.StartsAt.Before(row.EndsAt) {
					return configValidationError{fmt.Sprintf("活动 %s 与 %s 时间重叠", row.Key, other.Key)}
				}
			}
		}
	case "reward_tracks":
		var rows []models.RewardTrackConfig
		if err := json.Unmarshal(payload, &rows); err != nil {
			return err
		}
		seen := make(map[string]struct{}, len(rows))
		for index, row := range rows {
			if strings.TrimSpace(row.EventKey) == "" || (row.RewardType != "item" && row.RewardType != "currency") || strings.TrimSpace(row.RewardKey) == "" || strings.TrimSpace(row.RewardName) == "" || row.Milestone <= 0 || row.Quantity <= 0 {
				return configValidationError{fmt.Sprintf("第 %d 个奖励里程碑无效", index+1)}
			}
			key := fmt.Sprintf("%s:%d:%s:%s", row.EventKey, row.Milestone, row.RewardType, row.RewardKey)
			if _, exists := seen[key]; exists {
				return configValidationError{fmt.Sprintf("同一里程碑不能重复配置同一奖励: %s", key)}
			}
			seen[key] = struct{}{}
		}
	case "growth_roles":
		var rows []models.GrowthRoleConfig
		if err := json.Unmarshal(payload, &rows); err != nil {
			return err
		}
		seen := make(map[string]struct{}, len(rows))
		for index, row := range rows {
			if strings.TrimSpace(row.Name) == "" || strings.TrimSpace(row.Description) == "" || strings.TrimSpace(row.Skill1) == "" || strings.TrimSpace(row.Skill2) == "" || strings.TrimSpace(row.Skill3) == "" {
				return configValidationError{fmt.Sprintf("第 %d 个宠物定位信息不完整", index+1)}
			}
			if _, exists := seen[row.Name]; exists {
				return configValidationError{fmt.Sprintf("宠物定位名称重复: %s", row.Name)}
			}
			seen[row.Name] = struct{}{}
		}
	case "growth_stances":
		var rows []models.GrowthStanceConfig
		if err := json.Unmarshal(payload, &rows); err != nil {
			return err
		}
		for index, row := range rows {
			if strings.TrimSpace(row.Name) == "" || strings.TrimSpace(row.Description) == "" {
				return configValidationError{fmt.Sprintf("第 %d 个远征姿态信息不完整", index+1)}
			}
		}
	case "personality_rules":
		var rows []models.PersonalityRuleConfig
		if err := json.Unmarshal(payload, &rows); err != nil {
			return err
		}
		validDimensions := map[string]bool{"care": true, "explore": true, "support": true}
		for index, row := range rows {
			if strings.TrimSpace(row.Name) == "" || strings.TrimSpace(row.Description) == "" || !validDimensions[row.Dimension] || row.MinThreshold <= 0 {
				return configValidationError{fmt.Sprintf("第 %d 个性格形成规则无效", index+1)}
			}
		}
	case "codex_catalog":
		var rows []models.CodexCatalogConfig
		if err := json.Unmarshal(payload, &rows); err != nil {
			return err
		}
		seen := make(map[string]struct{}, len(rows))
		for index, row := range rows {
			if strings.TrimSpace(row.Category) == "" || strings.TrimSpace(row.EntryKey) == "" || strings.TrimSpace(row.Region) == "" {
				return configValidationError{fmt.Sprintf("第 %d 个图鉴条目信息不完整", index+1)}
			}
			key := row.Category + ":" + row.EntryKey
			if _, exists := seen[key]; exists {
				return configValidationError{fmt.Sprintf("图鉴条目重复: %s", key)}
			}
			seen[key] = struct{}{}
		}
	case "expedition_templates":
		var rows []models.ExpeditionTemplateConfig
		if err := json.Unmarshal(payload, &rows); err != nil {
			return err
		}
		seen := make(map[int]struct{}, len(rows))
		for index, row := range rows {
			if row.Tier <= 0 || strings.TrimSpace(row.Name) == "" || row.DurationMinutes <= 0 || row.HungerCost < 0 || row.ReadinessCost < 0 || strings.TrimSpace(row.RewardItem) == "" || row.RewardQuantity <= 0 || row.RewardRecords < 0 || row.RewardGrowth < 0 || row.CodexProgress < 0 || row.CodexProgress > 100 {
				return configValidationError{fmt.Sprintf("第 %d 个远征模板配置无效", index+1)}
			}
			if row.RequiredQuantity < 0 || (row.RequiredItem == "" && row.RequiredQuantity > 0) {
				return configValidationError{fmt.Sprintf("第 %d 个远征模板消耗物品配置无效", index+1)}
			}
			if _, exists := seen[row.Tier]; exists {
				return configValidationError{fmt.Sprintf("远征档位重复: %d", row.Tier)}
			}
			seen[row.Tier] = struct{}{}
		}
	case "chance_games":
		var rows []models.ChanceGameConfig
		if err := json.Unmarshal(payload, &rows); err != nil {
			return err
		}
		seen := make(map[string]struct{}, len(rows))
		for index, row := range rows {
			if strings.TrimSpace(row.GameKey) == "" || strings.TrimSpace(row.Name) == "" || row.CostCurrency < 0 || row.CostQuantity < 0 || row.DailyLimit < 0 || row.PityThreshold < 0 || row.DurationSecond < 0 {
				return configValidationError{fmt.Sprintf("第 %d 个概率玩法配置无效", index+1)}
			}
			if row.CostQuantity > 0 && strings.TrimSpace(row.CostItem) == "" {
				return configValidationError{fmt.Sprintf("玩法 %s 的消耗物品不能为空", row.GameKey)}
			}
			if row.PityThreshold > 0 && strings.TrimSpace(row.PityRewardKey) == "" {
				return configValidationError{fmt.Sprintf("玩法 %s 的保底奖励不能为空", row.GameKey)}
			}
			if _, exists := seen[row.GameKey]; exists {
				return configValidationError{fmt.Sprintf("概率玩法键重复: %s", row.GameKey)}
			}
			seen[row.GameKey] = struct{}{}
		}
	case "chance_rewards":
		var rows []models.ChanceRewardConfig
		if err := json.Unmarshal(payload, &rows); err != nil {
			return err
		}
		seen := make(map[string]struct{}, len(rows))
		for index, row := range rows {
			if strings.TrimSpace(row.GameKey) == "" || strings.TrimSpace(row.RewardKey) == "" || strings.TrimSpace(row.Name) == "" || row.Weight <= 0 || row.Quantity < 0 || row.Currency < 0 || (row.Quantity == 0 && row.Currency == 0) || (row.Quantity > 0 && strings.TrimSpace(row.ItemName) == "") {
				return configValidationError{fmt.Sprintf("第 %d 个概率奖励配置无效", index+1)}
			}
			key := row.GameKey + ":" + row.RewardKey
			if _, exists := seen[key]; exists {
				return configValidationError{fmt.Sprintf("概率奖励键重复: %s", key)}
			}
			seen[key] = struct{}{}
		}
	}
	return nil
}

// validateImmutableInternalKey prevents API clients from silently changing an
// existing runtime identifier while they are editing its display name.
func validateImmutableInternalKey(tx *gorm.DB, schema string, payload json.RawMessage) error {
	switch schema {
	case "pet_species":
		var incoming []models.PetSpeciesConfig
		if err := json.Unmarshal(payload, &incoming); err != nil {
			return err
		}
		var existing []models.PetSpeciesConfig
		if err := tx.Find(&existing).Error; err != nil {
			return err
		}
		byName := make(map[string]string, len(existing))
		for _, row := range existing {
			byName[row.Name] = row.Key
		}
		for _, row := range incoming {
			if previous, ok := byName[row.Name]; ok && previous != row.Key {
				return configValidationError{fmt.Sprintf("宠物“%s”的内部稳定标识不可修改，请复制新建后迁移关联", row.Name)}
			}
		}
	case "items":
		var incoming []models.ItemConfig
		if err := json.Unmarshal(payload, &incoming); err != nil {
			return err
		}
		var existing []models.ItemConfig
		if err := tx.Find(&existing).Error; err != nil {
			return err
		}
		byName := make(map[string]string, len(existing))
		for _, row := range existing {
			byName[row.Name] = row.Key
		}
		for _, row := range incoming {
			if previous, ok := byName[row.Name]; ok && previous != row.Key {
				return configValidationError{fmt.Sprintf("物品“%s”的内部稳定标识不可修改，请复制新建后迁移关联", row.Name)}
			}
		}
	}
	return nil
}

// validatePetLineage keeps the display hierarchy (FamilyKey/PreviousFormKey)
// consistent with the deterministic evolution rules consumed by gameplay.
func validatePetLineage(tx *gorm.DB, schema string, payload json.RawMessage) error {
	if schema != "pet_species" && schema != "pet_evolution_rules" {
		return nil
	}
	var pets []models.PetSpeciesConfig
	var rules []models.PetEvolutionRuleConfig
	if schema == "pet_species" {
		if err := json.Unmarshal(payload, &pets); err != nil {
			return err
		}
		if err := tx.Find(&rules).Error; err != nil {
			return err
		}
	} else {
		if err := tx.Find(&pets).Error; err != nil {
			return err
		}
		if err := json.Unmarshal(payload, &rules); err != nil {
			return err
		}
	}
	return validatePetLineageRows(pets, rules)
}

// validatePetLineageRows validates a complete effective pet configuration. It is
// shared by the generic schema endpoints and the lineage workspace endpoint so
// both paths enforce exactly the same gameplay invariants.
func validatePetLineageRows(pets []models.PetSpeciesConfig, rules []models.PetEvolutionRuleConfig) error {
	if len(pets) == 0 {
		return nil // generic schema validation owns the "配置列表不能为空" response.
	}

	stageRank := map[string]int{"base": 0, "evolved": 1, "awakened": 2}
	byKey := make(map[string]models.PetSpeciesConfig, len(pets))
	bases := make(map[string]int)
	for _, pet := range pets {
		key := strings.TrimSpace(pet.Key)
		name := strings.TrimSpace(pet.Name)
		if key == "" {
			return configValidationError{fmt.Sprintf("宠物形态“%s”缺少内部稳定标识", name)}
		}
		if _, exists := byKey[key]; exists {
			return configValidationError{fmt.Sprintf("宠物形态稳定键重复: %s", key)}
		}
		if strings.TrimSpace(pet.FamilyKey) == "" {
			return configValidationError{fmt.Sprintf("宠物形态“%s”缺少谱系标识", nameOrKey(name, key))}
		}
		if _, ok := stageRank[pet.Stage]; !ok {
			return configValidationError{fmt.Sprintf("宠物形态“%s”的阶段无效: %s", nameOrKey(name, key), pet.Stage)}
		}
		byKey[key] = pet
		if pet.Stage == "base" {
			bases[pet.FamilyKey]++
		}
	}
	for family, count := range bases {
		if count != 1 {
			return configValidationError{fmt.Sprintf("谱系“%s”必须且只能包含一个基础形态，当前为 %d 个", family, count)}
		}
	}
	for _, pet := range pets {
		key := strings.TrimSpace(pet.Key)
		name := nameOrKey(strings.TrimSpace(pet.Name), key)
		previousKey := strings.TrimSpace(pet.PreviousFormKey)
		if pet.Stage == "base" {
			if previousKey != "" {
				return configValidationError{fmt.Sprintf("基础形态“%s”不应配置前置形态", name)}
			}
			continue
		}
		if previousKey == "" {
			return configValidationError{fmt.Sprintf("非基础形态“%s”必须配置前置形态", name)}
		}
		previous, exists := byKey[previousKey]
		if !exists {
			return configValidationError{fmt.Sprintf("宠物形态“%s”的前置形态不存在: %s", name, previousKey)}
		}
		if previous.FamilyKey != pet.FamilyKey {
			return configValidationError{fmt.Sprintf("宠物形态“%s”的前置形态必须属于同一谱系", name)}
		}
		if stageRank[previous.Stage] >= stageRank[pet.Stage] {
			return configValidationError{fmt.Sprintf("宠物形态“%s”的前置形态阶段必须低于自身", name)}
		}
	}
	for _, pet := range pets {
		if bases[pet.FamilyKey] != 1 {
			return configValidationError{fmt.Sprintf("谱系“%s”必须且只能包含一个基础形态，当前为 %d 个", pet.FamilyKey, bases[pet.FamilyKey])}
		}
	}
	state := make(map[string]uint8, len(byKey))
	var walk func(string) error
	walk = func(key string) error {
		switch state[key] {
		case 1:
			return configValidationError{fmt.Sprintf("宠物进化链存在循环引用: %s", key)}
		case 2:
			return nil
		}
		state[key] = 1
		if previous := strings.TrimSpace(byKey[key].PreviousFormKey); previous != "" {
			if err := walk(previous); err != nil {
				return err
			}
		}
		state[key] = 2
		return nil
	}
	for key := range byKey {
		if err := walk(key); err != nil {
			return err
		}
	}

	seenRules := make(map[string]struct{}, len(rules))
	for _, rule := range rules {
		ruleKey := strings.TrimSpace(rule.Key)
		if ruleKey == "" {
			return configValidationError{"进化规则缺少稳定键"}
		}
		if _, exists := seenRules[ruleKey]; exists {
			return configValidationError{fmt.Sprintf("进化规则稳定键重复: %s", ruleKey)}
		}
		seenRules[ruleKey] = struct{}{}
		from, fromExists := byKey[strings.TrimSpace(rule.FromFormKey)]
		to, toExists := byKey[strings.TrimSpace(rule.ToFormKey)]
		if !fromExists || !toExists || rule.FromFormKey == rule.ToFormKey {
			return configValidationError{fmt.Sprintf("进化规则“%s”引用了不存在或相同的形态", ruleKey)}
		}
		if from.FamilyKey != to.FamilyKey {
			return configValidationError{fmt.Sprintf("进化规则“%s”的两端形态必须属于同一谱系", ruleKey)}
		}
		if strings.TrimSpace(to.PreviousFormKey) != strings.TrimSpace(rule.FromFormKey) {
			return configValidationError{fmt.Sprintf("进化规则“%s”与目标形态的前置形态不一致", ruleKey)}
		}
		if stageRank[from.Stage] >= stageRank[to.Stage] {
			return configValidationError{fmt.Sprintf("进化规则“%s”的目标形态阶段必须高于来源形态", ruleKey)}
		}
	}
	return nil
}

func nameOrKey(name, key string) string {
	if name != "" {
		return name
	}
	return key
}

func validateRewardItems(db *gorm.DB, payload json.RawMessage) error {
	var rows []models.RewardTrackConfig
	if err := json.Unmarshal(payload, &rows); err != nil {
		return err
	}
	for _, row := range rows {
		switch row.RewardType {
		case "item":
			var item models.ItemConfig
			if err := db.First(&item, "key = ?", row.RewardKey).Error; err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					return configValidationError{fmt.Sprintf("奖励物品不存在: %s", row.RewardKey)}
				}
				return err
			}
			if item.Status == "hidden" || item.Status == "disabled" {
				return configValidationError{fmt.Sprintf("奖励物品当前不可发放: %s", row.RewardKey)}
			}
		case "currency":
			var currency models.CurrencyConfig
			if err := db.First(&currency, "key = ? AND enabled = ?", row.RewardKey, true).Error; err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					return configValidationError{fmt.Sprintf("奖励货币不存在或未启用: %s", row.RewardKey)}
				}
				return err
			}
		}
	}
	return nil
}

// 配置接口使用的稳定业务错误码；HTTP 状态同时反映错误类别。
const (
	codeInvalidPayload = 4000
	codeSchemaNotFound = 4004
	codeInternalError  = 5000
)

// configSchema 描述一张可被后台管理的配置表。
//
// fetch 与 save 都是强类型闭包：每个 schema 绑定到具体的 models 结构体，
// 因此表名与列名永远来自代码而不是请求参数，杜绝了拼接表名带来的注入面。
type configSchema struct {
	// orderBy 是列表返回时的排序列，保证前端展示顺序稳定。
	orderBy string
	fetch   func(db *gorm.DB, orderBy string) (interface{}, error)
	save    func(tx *gorm.DB, payload json.RawMessage) error
}

type schemaReplace[T any] struct {
	key           func(T) string
	allowEmpty    bool
	allowEmptyKey bool
	beforeDelete  func(tx *gorm.DB, removing []string) error
}

// newConfigSchema 为某个配置模型生成读写闭包。
func newConfigSchema[T any](orderBy string, replace schemaReplace[T]) configSchema {
	return configSchema{
		orderBy: orderBy,
		fetch: func(db *gorm.DB, orderBy string) (interface{}, error) {
			// 初始化为空切片而不是 nil，确保空表序列化为 [] 而不是 null。
			rows := make([]T, 0)
			if err := db.Order(orderBy).Find(&rows).Error; err != nil {
				return nil, err
			}
			return rows, nil
		},
		save: func(tx *gorm.DB, payload json.RawMessage) error {
			return replaceConfigRows(tx, payload, replace)
		},
	}
}

func replaceConfigRows[T any](tx *gorm.DB, payload json.RawMessage, replace schemaReplace[T]) error {
	var rows []T
	if err := json.Unmarshal(payload, &rows); err != nil {
		return err
	}
	if !replace.allowEmpty && len(rows) == 0 {
		return configValidationError{"配置列表不能为空"}
	}
	keep := map[string]struct{}{}
	for _, row := range rows {
		key := ""
		if replace.key != nil {
			key = strings.TrimSpace(replace.key(row))
		}
		if key == "" {
			if replace.allowEmptyKey {
				continue
			}
			return configValidationError{"配置项缺少稳定键"}
		}
		keep[key] = struct{}{}
	}
	var existing []T
	if err := tx.Find(&existing).Error; err != nil {
		return err
	}
	removing := make([]string, 0)
	toDelete := make([]T, 0)
	for _, row := range existing {
		key := ""
		if replace.key != nil {
			key = strings.TrimSpace(replace.key(row))
		}
		if key == "" {
			continue
		}
		if _, found := keep[key]; !found {
			removing = append(removing, key)
			toDelete = append(toDelete, row)
		}
	}
	if replace.beforeDelete != nil {
		if err := replace.beforeDelete(tx, removing); err != nil {
			return err
		}
	}
	for index := range toDelete {
		if err := tx.Delete(&toDelete[index]).Error; err != nil {
			return err
		}
	}
	for index := range rows {
		if err := tx.Save(&rows[index]).Error; err != nil {
			return err
		}
	}
	return nil
}

// configSchemas 是配置中心的白名单。键与旧接口 /api/admin/configs/<key> 保持一致，
// 便于前端逐步迁移；值绑定到 database.InitDB 中迁移的 9 张配置表。
var configSchemas = map[string]configSchema{
	"system": newConfigSchema[models.SystemConfig]("key asc", schemaReplace[models.SystemConfig]{
		key: func(row models.SystemConfig) string { return row.Key },
	}),
	"commands": newConfigSchema[models.CommandConfig]("func_name asc", schemaReplace[models.CommandConfig]{
		key: func(row models.CommandConfig) string { return row.FuncName },
	}),
	"pet_species": newConfigSchema[models.PetSpeciesConfig]("name asc", schemaReplace[models.PetSpeciesConfig]{
		// 稳定键是运行时引用，名称仅是运营展示字段；以 Key 作为替换身份，允许安全改名。
		key: func(row models.PetSpeciesConfig) string { return row.Key },
	}),
	"pet_evolution_rules": newConfigSchema[models.PetEvolutionRuleConfig]("sort_order asc, key asc", schemaReplace[models.PetEvolutionRuleConfig]{
		key: func(row models.PetEvolutionRuleConfig) string { return row.Key }, allowEmpty: true,
	}),
	"pet_evolution_costs": newConfigSchema[models.PetEvolutionCostConfig]("evolution_key asc, item_key asc", schemaReplace[models.PetEvolutionCostConfig]{
		key: func(row models.PetEvolutionCostConfig) string {
			return strings.TrimSpace(row.EvolutionKey) + "|" + strings.TrimSpace(row.ItemKey)
		},
		allowEmpty: true,
	}),
	"pet_skill_unlocks": newConfigSchema[models.PetSkillUnlockConfig]("form_key asc, sort_order asc", schemaReplace[models.PetSkillUnlockConfig]{
		key: func(row models.PetSkillUnlockConfig) string {
			return strings.TrimSpace(row.FormKey) + "|" + strings.TrimSpace(row.SkillKey)
		},
		allowEmpty: true,
	}),
	"adventure_levels": newConfigSchema[models.AdventureLevelConfig]("level asc", schemaReplace[models.AdventureLevelConfig]{
		key: func(row models.AdventureLevelConfig) string { return strconv.Itoa(row.Level) },
	}),
	"items": newConfigSchema[models.ItemConfig]("name asc", schemaReplace[models.ItemConfig]{
		// 与宠物一致，物品改名不应被识别为删除并重新创建。
		key: func(row models.ItemConfig) string { return row.Key },
		beforeDelete: func(tx *gorm.DB, removing []string) error {
			if len(removing) == 0 {
				return nil
			}
			reason, err := itemDeleteBlocker(tx, removing)
			if err != nil {
				return err
			}
			if reason != "" {
				return configValidationError{reason}
			}
			return nil
		},
	}),
	"shop_items": newConfigSchema[models.ShopItemConfig]("id asc", schemaReplace[models.ShopItemConfig]{
		key: func(row models.ShopItemConfig) string {
			if row.ID == 0 {
				return ""
			}
			return strconv.FormatUint(uint64(row.ID), 10)
		},
		allowEmptyKey: true,
	}),
	"checkin_rewards": newConfigSchema[models.CheckinRewardConfig]("type asc, day asc", schemaReplace[models.CheckinRewardConfig]{
		key: func(row models.CheckinRewardConfig) string {
			return strings.TrimSpace(row.Type) + "|" + strings.TrimSpace(row.Day)
		},
	}),
	"work_settings": newConfigSchema[models.WorkSettingConfig]("name asc", schemaReplace[models.WorkSettingConfig]{
		key: func(row models.WorkSettingConfig) string { return row.Name },
	}),
	"menus": newConfigSchema[models.MenuConfig]("name asc", schemaReplace[models.MenuConfig]{
		key: func(row models.MenuConfig) string { return row.Name },
	}),
	"images": newConfigSchema[models.ImageConfig]("name asc", schemaReplace[models.ImageConfig]{
		key: func(row models.ImageConfig) string { return row.Name },
	}),
	"live_events": newConfigSchema[models.LiveEventConfig]("starts_at desc", schemaReplace[models.LiveEventConfig]{
		key: func(row models.LiveEventConfig) string { return row.Key }, allowEmpty: true,
	}),
	"reward_tracks": newConfigSchema[models.RewardTrackConfig]("event_key asc, milestone asc", schemaReplace[models.RewardTrackConfig]{
		key: func(row models.RewardTrackConfig) string {
			return fmt.Sprintf("%s:%d:%s:%s", row.EventKey, row.Milestone, row.RewardType, row.RewardKey)
		},
		allowEmpty: true,
	}),
	"growth_roles": newConfigSchema[models.GrowthRoleConfig]("sort_order asc, name asc", schemaReplace[models.GrowthRoleConfig]{
		key: func(row models.GrowthRoleConfig) string { return row.Name }, allowEmpty: true,
	}),
	"growth_stances": newConfigSchema[models.GrowthStanceConfig]("sort_order asc, name asc", schemaReplace[models.GrowthStanceConfig]{
		key: func(row models.GrowthStanceConfig) string { return row.Name }, allowEmpty: true,
	}),
	"personality_rules": newConfigSchema[models.PersonalityRuleConfig]("sort_order asc, name asc", schemaReplace[models.PersonalityRuleConfig]{
		key: func(row models.PersonalityRuleConfig) string { return row.Name }, allowEmpty: true,
	}),
	"codex_catalog": newConfigSchema[models.CodexCatalogConfig]("sort_order asc, category asc, entry_key asc", schemaReplace[models.CodexCatalogConfig]{
		key: func(row models.CodexCatalogConfig) string {
			return strings.TrimSpace(row.Category) + "|" + strings.TrimSpace(row.EntryKey)
		},
		allowEmpty: true,
	}),
	"expedition_templates": newConfigSchema[models.ExpeditionTemplateConfig]("tier asc", schemaReplace[models.ExpeditionTemplateConfig]{
		key: func(row models.ExpeditionTemplateConfig) string { return strconv.Itoa(row.Tier) }, allowEmpty: true,
	}),
	"chance_games": newConfigSchema[models.ChanceGameConfig]("game_key asc", schemaReplace[models.ChanceGameConfig]{
		key: func(row models.ChanceGameConfig) string { return row.GameKey }, allowEmpty: true,
	}),
	"chance_rewards": newConfigSchema[models.ChanceRewardConfig]("game_key asc, sort_order asc, id asc", schemaReplace[models.ChanceRewardConfig]{
		key: func(row models.ChanceRewardConfig) string {
			return strings.TrimSpace(row.GameKey) + "|" + strings.TrimSpace(row.RewardKey)
		},
		allowEmpty: true,
	}),
}

var configConsumers = map[string][]string{
	"system":               {"服务启动", "游戏参数"},
	"commands":             {"统一命令路由", "OneBot", "QQ 官方群", "QQ 官方频道"},
	"pet_species":          {"领养", "宠物状态", "进化觉醒", "消息图片"},
	"pet_evolution_rules":  {"进化预览", "确认进化"},
	"pet_evolution_costs":  {"进化材料扣除"},
	"pet_skill_unlocks":    {"宠物技能"},
	"adventure_levels":     {"冒险等级"},
	"items":                {"统一背包", "物品效果", "社区共建", "安全交易"},
	"shop_items":           {"商店", "购买出售", "补货"},
	"checkin_rewards":      {"签到", "每日奖励结算"},
	"work_settings":        {"打工菜单", "活动执行与结算"},
	"menus":                {"菜单", "帮助"},
	"images":               {"消息图片", "后台预览"},
	"live_events":          {"活动进度", "赛季故事", "社区影响"},
	"reward_tracks":        {"活动里程碑结算"},
	"growth_roles":         {"宠物定位", "技能"},
	"growth_stances":       {"远征", "社区首领"},
	"personality_rules":    {"陪伴性格"},
	"codex_catalog":        {"图鉴", "远征图鉴授予"},
	"expedition_templates": {"远征菜单", "远征出发与结算"},
	"chance_games":         {"钓鱼", "抽奖"},
	"chance_rewards":       {"钓鱼概率与保底", "抽奖概率与保底"},
}

// knownConfigSchemas 返回排序后的白名单键，用于错误提示。
func knownConfigSchemas() []string {
	names := make([]string, 0, len(configSchemas))
	for name := range configSchemas {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// ConfigAPI 提供配置中心的 REST 接口。
type ConfigAPI struct {
	DB *gorm.DB
}

var RebuildCommandRoutesFunc func() error
var PrepareConfigDefaultsFunc func() error

func NewConfigAPI(db *gorm.DB) *ConfigAPI {
	return &ConfigAPI{DB: db}
}

// RegisterConfigRoutes 把配置中心接口挂到给定分组上。
// 调用方负责在分组上叠加 RequireAdminSession 等中间件。
func RegisterConfigRoutes(group *gin.RouterGroup, api *ConfigAPI) {
	group.GET("/config/schemas", api.ListSchemas)
	group.GET("/config/status", api.GetStatus)
	group.GET("/config/:schema/meta", api.GetConfigMeta)
	group.GET("/config/:schema", api.GetConfig)
	group.PUT("/config/:schema", api.SaveConfig)
	group.DELETE("/config/:schema/:key", api.DeleteConfigItem)
	group.POST("/config/reload", api.ReloadConfig)
	group.POST("/config/reset", api.ResetConfig)
	group.PUT("/content/pet-lineages/:familyKey", api.SavePetLineage)
}

type petLineagePayload struct {
	Pets  []models.PetSpeciesConfig       `json:"pets"`
	Rules []models.PetEvolutionRuleConfig `json:"rules"`
}

// SavePetLineage atomically replaces the forms and evolution rules belonging to
// a single lineage. Keeping this write separate from the generic table-replace
// endpoints lets the workspace save one lineage without overwriting concurrent
// changes made to other lineages.
func (api *ConfigAPI) SavePetLineage(c *gin.Context) {
	if api.DB == nil {
		Error(c, codeInternalError, "数据库未初始化")
		return
	}
	familyKey := strings.TrimSpace(c.Param("familyKey"))
	if familyKey == "" {
		Error(c, codeInvalidPayload, "谱系标识不能为空")
		return
	}
	var payload petLineagePayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		Error(c, codeInvalidPayload, "请求体不是合法的谱系配置: "+err.Error())
		return
	}
	for _, pet := range payload.Pets {
		if strings.TrimSpace(pet.FamilyKey) != familyKey {
			Error(c, codeInvalidPayload, "请求中包含不属于当前谱系的宠物形态")
			return
		}
	}

	if err := api.DB.Transaction(func(tx *gorm.DB) error {
		var allPets []models.PetSpeciesConfig
		var allRules []models.PetEvolutionRuleConfig
		if err := tx.Find(&allPets).Error; err != nil {
			return err
		}
		if err := tx.Find(&allRules).Error; err != nil {
			return err
		}

		// Existing display names are a convenient way to stop an editor from
		// changing the stable key of a previously saved form.
		existingByName := make(map[string]string, len(allPets))
		for _, pet := range allPets {
			existingByName[pet.Name] = pet.Key
		}
		for _, pet := range payload.Pets {
			if previous, exists := existingByName[pet.Name]; exists && previous != pet.Key {
				return configValidationError{fmt.Sprintf("宠物“%s”的内部稳定标识不可修改，请复制新建后迁移关联", pet.Name)}
			}
		}

		storedFamilyByForm := make(map[string]string, len(allPets))
		for _, pet := range allPets {
			storedFamilyByForm[pet.Key] = pet.FamilyKey
		}
		familyByForm := make(map[string]string, len(allPets)+len(payload.Pets))
		effectivePets := make([]models.PetSpeciesConfig, 0, len(allPets)+len(payload.Pets))
		for _, pet := range allPets {
			if pet.FamilyKey == familyKey {
				continue
			}
			effectivePets = append(effectivePets, pet)
			familyByForm[pet.Key] = pet.FamilyKey
		}
		for _, pet := range payload.Pets {
			effectivePets = append(effectivePets, pet)
			familyByForm[pet.Key] = pet.FamilyKey
		}
		for _, rule := range payload.Rules {
			if familyByForm[rule.FromFormKey] != familyKey || familyByForm[rule.ToFormKey] != familyKey {
				return configValidationError{"进化规则的来源和目标形态必须属于当前谱系"}
			}
		}

		effectiveRules := make([]models.PetEvolutionRuleConfig, 0, len(allRules)+len(payload.Rules))
		for _, rule := range allRules {
			if storedFamilyByForm[rule.FromFormKey] == familyKey || storedFamilyByForm[rule.ToFormKey] == familyKey {
				continue
			}
			effectiveRules = append(effectiveRules, rule)
		}
		effectiveRules = append(effectiveRules, payload.Rules...)
		if err := validatePetLineageRows(effectivePets, effectiveRules); err != nil {
			return err
		}

		// Rules are removed by their old endpoint membership so a malformed
		// historic cross-family rule cannot survive a successful lineage repair.
		oldRuleKeys := make([]string, 0)
		for _, rule := range allRules {
			if storedFamilyByForm[rule.FromFormKey] == familyKey || storedFamilyByForm[rule.ToFormKey] == familyKey {
				oldRuleKeys = append(oldRuleKeys, rule.Key)
			}
		}
		if len(oldRuleKeys) > 0 {
			if err := tx.Where("key IN ?", oldRuleKeys).Delete(&models.PetEvolutionRuleConfig{}).Error; err != nil {
				return err
			}
		}
		if err := tx.Where("family_key = ?", familyKey).Delete(&models.PetSpeciesConfig{}).Error; err != nil {
			return err
		}
		for index := range payload.Pets {
			if err := tx.Save(&payload.Pets[index]).Error; err != nil {
				return err
			}
		}
		for index := range payload.Rules {
			if err := tx.Save(&payload.Rules[index]).Error; err != nil {
				return err
			}
		}
		return appconfig.MarkConfigSaved(tx)
	}); err != nil {
		var validationErr configValidationError
		if errors.As(err, &validationErr) {
			Error(c, codeInvalidPayload, validationErr.Error())
			return
		}
		Error(c, codeInternalError, "保存宠物谱系失败: "+err.Error())
		return
	}

	Success(c, gin.H{"pets": payload.Pets, "rules": payload.Rules})
}

func (api *ConfigAPI) ReloadConfig(c *gin.Context) {
	status, err := appconfig.GetConfigStatus(api.DB)
	if err != nil {
		Error(c, codeInternalError, "读取配置版本失败")
		return
	}
	if err = appconfig.LoadAllConfigsFromDB(api.DB); err != nil {
		Error(c, codeInternalError, "加载配置失败")
		return
	}
	if RebuildCommandRoutesFunc != nil {
		if err = RebuildCommandRoutesFunc(); err != nil {
			Error(c, codeInternalError, "命令目录重载失败")
			return
		}
	}
	if err = appconfig.MarkConfigLoaded(api.DB, status.DBRevision); err != nil {
		Error(c, codeInternalError, "记录配置版本失败")
		return
	}
	Success(c, gin.H{"message": "配置已重载并生效"})
}

func (api *ConfigAPI) ResetConfig(c *gin.Context) {
	var request struct {
		Reason       string `json:"reason"`
		Confirmation string `json:"confirmation"`
	}
	if err := c.ShouldBindJSON(&request); err != nil {
		Error(c, codeInvalidPayload, "请填写操作原因并输入确认词")
		return
	}
	reason, err := requiredReason(request.Reason)
	if err != nil {
		Error(c, codeInvalidPayload, err.Error())
		return
	}
	if strings.TrimSpace(request.Confirmation) != "恢复出厂" {
		Error(c, codeInvalidPayload, "请输入「恢复出厂」以确认操作")
		return
	}
	var profile models.ConfigProfile
	if err := api.DB.First(&profile, "id = ?", appconfig.OfficialProfileID).Error; err != nil {
		Error(c, codeInternalError, "官方默认配置不存在")
		return
	}
	snapshot, err := appconfig.DecodeSnapshot(profile.Payload)
	if err != nil {
		Error(c, codeInternalError, "官方默认配置损坏")
		return
	}
	conflicts, err := appconfig.CheckSnapshotCompatibility(api.DB, snapshot)
	if err != nil {
		Error(c, codeInternalError, "默认配置兼容性检查失败")
		return
	}
	if len(conflicts) > 0 {
		c.JSON(409, gin.H{"code": 4092, "msg": "官方默认配置与现有玩家数据不兼容", "data": gin.H{"conflicts": conflicts}})
		return
	}
	if err = appconfig.ValidateLaunchReadiness(snapshot); err != nil {
		c.JSON(409, gin.H{"code": 4093, "msg": err.Error()})
		return
	}
	previous, err := appconfig.CaptureSnapshot(api.DB)
	if err != nil {
		Error(c, codeInternalError, "创建回滚快照失败")
		return
	}
	if err = api.DB.Transaction(func(tx *gorm.DB) error {
		if err := appconfig.ApplySnapshot(tx, snapshot); err != nil {
			return err
		}
		return appconfig.SetActiveProfile(tx, profile.ID, false)
	}); err != nil {
		Error(c, codeInternalError, "恢复默认配置失败")
		return
	}
	if err = reloadRuntimeConfig(api.DB); err != nil {
		_ = api.DB.Transaction(func(tx *gorm.DB) error { return appconfig.ApplySnapshot(tx, previous) })
		_ = reloadRuntimeConfig(api.DB)
		Error(c, codeInternalError, "恢复默认配置热重载失败，已回滚")
		return
	}
	_ = writeAudit(api.DB, "reset_config", "config_profile", profile.ID, reason, nil, gin.H{"active_profile_id": profile.ID}, true, nil)
	Success(c, gin.H{"message": "已切换到官方默认 v0.1.0，其他配置方案已保留"})
}

func (api *ConfigAPI) DeleteConfigItem(c *gin.Context) {
	schemaName := c.Param("schema")
	if _, ok := configSchemas[schemaName]; !ok {
		Error(c, codeSchemaNotFound, "未知的配置类型")
		return
	}
	key := strings.TrimSpace(c.Param("key"))
	if key == "" {
		Error(c, codeInvalidPayload, "配置键不能为空")
		return
	}
	err := api.DB.Transaction(func(tx *gorm.DB) error {
		var result *gorm.DB
		switch schemaName {
		case "commands":
			result = tx.Where("func_name = ?", key).Delete(&models.CommandConfig{})
		case "pet_species":
			result = tx.Where("key = ? OR name = ?", key, key).Delete(&models.PetSpeciesConfig{})
		case "items":
			if blocker, err := itemDeleteBlocker(tx, []string{key}); err != nil {
				return err
			} else if blocker != "" {
				return configValidationError{blocker}
			}
			result = tx.Where("key = ? OR name = ?", key, key).Delete(&models.ItemConfig{})
		case "shop_items":
			if id, err := strconv.ParseUint(key, 10, 64); err == nil {
				result = tx.Where("id = ?", id).Delete(&models.ShopItemConfig{})
			} else {
				result = tx.Where("name = ?", key).Delete(&models.ShopItemConfig{})
			}
		case "menus":
			result = tx.Where("name = ?", key).Delete(&models.MenuConfig{})
		case "images":
			result = tx.Where("name = ?", key).Delete(&models.ImageConfig{})
		default:
			return configValidationError{"此配置类型不支持单条删除，请在对应编辑器中保存完整列表"}
		}
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return gorm.ErrRecordNotFound
		}
		return appconfig.MarkConfigSaved(tx)
	})
	if err != nil {
		var validation configValidationError
		if errors.As(err, &validation) {
			Error(c, codeInvalidPayload, validation.Error())
			return
		}
		if errors.Is(err, gorm.ErrRecordNotFound) {
			Error(c, codeSchemaNotFound, "配置项不存在")
			return
		}
		Error(c, codeInternalError, "删除配置项失败")
		return
	}
	Success(c, gin.H{"message": "配置项已删除"})
}

func (api *ConfigAPI) GetConfigMeta(c *gin.Context) {
	if _, ok := api.resolve(c); !ok {
		return
	}
	status, err := appconfig.GetConfigStatus(api.DB)
	if err != nil {
		Error(c, codeInternalError, "查询配置状态失败")
		return
	}
	Success(c, gin.H{
		"schema":             c.Param("schema"),
		"consumers":          configConsumers[c.Param("schema")],
		"effective_revision": status.LoadedRevision,
		"db_revision":        status.DBRevision,
		"pending_reload":     status.PendingReload,
	})
}

func (api *ConfigAPI) GetStatus(c *gin.Context) {
	if api.DB == nil {
		Error(c, codeInternalError, "数据库未初始化")
		return
	}
	status, err := appconfig.GetConfigStatus(api.DB)
	if err != nil {
		Error(c, codeInternalError, "查询配置状态失败: "+err.Error())
		return
	}
	Success(c, status)
}

// resolve 校验 schema 参数并确认数据库可用。
func (api *ConfigAPI) resolve(c *gin.Context) (configSchema, bool) {
	if api.DB == nil {
		Error(c, codeInternalError, "数据库未初始化")
		return configSchema{}, false
	}
	name := c.Param("schema")
	schema, exists := configSchemas[name]
	if !exists {
		Error(c, codeSchemaNotFound, "未知的配置类型: "+name+"，可用类型: "+strings.Join(knownConfigSchemas(), ", "))
		return configSchema{}, false
	}
	return schema, true
}

// ListSchemas 返回可管理的配置类型清单，供前端动态生成导航。
func (api *ConfigAPI) ListSchemas(c *gin.Context) {
	Success(c, knownConfigSchemas())
}

// GetConfig 读取某个配置表的全部记录。
func (api *ConfigAPI) GetConfig(c *gin.Context) {
	schema, ok := api.resolve(c)
	if !ok {
		return
	}

	rows, err := schema.fetch(api.DB, schema.orderBy)
	if err != nil {
		Error(c, codeInternalError, "查询配置失败: "+err.Error())
		return
	}
	Success(c, rows)
}

// SaveConfig 批量写入某个配置表。整批记录在单个事务内提交，任一条失败则全部回滚。
func (api *ConfigAPI) SaveConfig(c *gin.Context) {
	schema, ok := api.resolve(c)
	if !ok {
		return
	}

	var payload json.RawMessage
	if err := c.ShouldBindJSON(&payload); err != nil {
		Error(c, codeInvalidPayload, "请求体不是合法的 JSON: "+err.Error())
		return
	}
	if err := validateDomainConfig(c.Param("schema"), payload); err != nil {
		var validationErr configValidationError
		if errors.As(err, &validationErr) {
			Error(c, codeInvalidPayload, validationErr.Error())
			return
		}
		Error(c, codeInvalidPayload, "请求体结构无效: "+err.Error())
		return
	}

	if err := api.DB.Transaction(func(tx *gorm.DB) error {
		if err := validateImmutableInternalKey(tx, c.Param("schema"), payload); err != nil {
			return err
		}
		if err := validatePetLineage(tx, c.Param("schema"), payload); err != nil {
			return err
		}
		if c.Param("schema") == "reward_tracks" {
			if err := validateRewardItems(tx, payload); err != nil {
				return err
			}
		}
		if c.Param("schema") == "live_events" {
			if err := tx.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&models.LiveEventConfig{}).Error; err != nil {
				return err
			}
		}
		if c.Param("schema") == "reward_tracks" {
			if err := tx.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&models.RewardTrackConfig{}).Error; err != nil {
				return err
			}
		}
		switch c.Param("schema") {
		case "growth_roles":
			if err := tx.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&models.GrowthRoleConfig{}).Error; err != nil {
				return err
			}
		case "growth_stances":
			if err := tx.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&models.GrowthStanceConfig{}).Error; err != nil {
				return err
			}
		case "personality_rules":
			if err := tx.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&models.PersonalityRuleConfig{}).Error; err != nil {
				return err
			}
		case "codex_catalog":
			if err := tx.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&models.CodexCatalogConfig{}).Error; err != nil {
				return err
			}
		case "expedition_templates":
			if err := tx.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&models.ExpeditionTemplateConfig{}).Error; err != nil {
				return err
			}
		case "chance_games":
			if err := tx.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&models.ChanceGameConfig{}).Error; err != nil {
				return err
			}
		case "chance_rewards":
			if err := tx.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&models.ChanceRewardConfig{}).Error; err != nil {
				return err
			}
		}
		if err := schema.save(tx, payload); err != nil {
			return err
		}
		return appconfig.MarkConfigSaved(tx)
	}); err != nil {
		var validationErr configValidationError
		if errors.As(err, &validationErr) {
			Error(c, codeInvalidPayload, validationErr.Error())
			return
		}
		// 反序列化失败属于客户端问题，写库失败属于服务端问题，分别用不同错误码。
		if _, isTypeErr := err.(*json.UnmarshalTypeError); isTypeErr {
			Error(c, codeInvalidPayload, "请求体结构与该配置类型不匹配: "+err.Error())
			return
		}
		if _, isSyntaxErr := err.(*json.SyntaxError); isSyntaxErr {
			Error(c, codeInvalidPayload, "请求体不是合法的 JSON: "+err.Error())
			return
		}
		Error(c, codeInternalError, "保存配置失败: "+err.Error())
		return
	}

	Success(c, nil)
}
