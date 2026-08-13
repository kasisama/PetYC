package gameplayrules

import (
	"fmt"
	"sort"
	"strings"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"qq-pet-saas/models"
)

type RuleSet struct {
	Roles         []models.GrowthRoleConfig
	Stances       []models.GrowthStanceConfig
	Personalities []models.PersonalityRuleConfig
	Codex         []models.CodexCatalogConfig
}

func DefaultRules() RuleSet {
	return RuleSet{
		Roles: []models.GrowthRoleConfig{
			{Name: "探索者", Description: "擅长寻路、观察与环境采样", Skill1: "寻路", Skill2: "观察", Skill3: "采样", Enabled: true, SortOrder: 10},
			{Name: "守护者", Description: "降低远征风险并保护同行伙伴", Skill1: "护盾", Skill2: "警戒", Skill3: "稳固", Enabled: true, SortOrder: 20},
			{Name: "学者", Description: "解析遗迹线索并提升调查效率", Skill1: "解析", Skill2: "记录", Skill3: "推演", Enabled: true, SortOrder: 30},
			{Name: "支援者", Description: "提供急救、鼓舞与协作增益", Skill1: "急救", Skill2: "鼓舞", Skill3: "协同", Enabled: true, SortOrder: 40},
		},
		Stances: []models.GrowthStanceConfig{
			{Name: "攻击", Description: "提高突破能力", Enabled: true, SortOrder: 10},
			{Name: "守护", Description: "降低事件风险", Enabled: true, SortOrder: 20},
			{Name: "支援", Description: "强化协作贡献", Enabled: true, SortOrder: 30},
			{Name: "探索", Description: "提高调查进度", Enabled: true, SortOrder: 40},
		},
		Personalities: []models.PersonalityRuleConfig{
			{Name: "温柔", Dimension: "care", MinThreshold: 3, Description: "长期照料行为更突出", Enabled: true, SortOrder: 10},
			{Name: "好奇", Dimension: "explore", MinThreshold: 3, Description: "长期探索行为更突出", Enabled: true, SortOrder: 20},
			{Name: "可靠", Dimension: "support", MinThreshold: 3, Description: "长期支援行为更突出", Enabled: true, SortOrder: 30},
		},
		Codex: []models.CodexCatalogConfig{
			{Category: "生物", EntryKey: "林间足迹", Region: "森林", Description: "森林远征中发现的神秘足迹", Enabled: true, SortOrder: 10},
			{Category: "遗迹", EntryKey: "遗迹守卫", Region: "遗迹", Description: "沉睡于古代遗迹的守卫记录", Enabled: true, SortOrder: 20},
			{Category: "生态", EntryKey: "深层生态", Region: "深层区域", Description: "深层区域的生态调查档案", Enabled: true, SortOrder: 30},
		},
	}
}

func EnsureDefaults(db *gorm.DB) error {
	defaults := DefaultRules()
	return db.Transaction(func(tx *gorm.DB) error {
		for index := range defaults.Roles {
			if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&defaults.Roles[index]).Error; err != nil {
				return err
			}
		}
		for index := range defaults.Stances {
			if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&defaults.Stances[index]).Error; err != nil {
				return err
			}
		}
		for index := range defaults.Personalities {
			if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&defaults.Personalities[index]).Error; err != nil {
				return err
			}
		}
		for index := range defaults.Codex {
			if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&defaults.Codex[index]).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func Load(db *gorm.DB) (RuleSet, []string) {
	defaults := DefaultRules()
	rules := RuleSet{}
	warnings := make([]string, 0, 4)
	if err := db.Order("sort_order asc, name asc").Find(&rules.Roles).Error; err != nil || len(rules.Roles) == 0 {
		rules.Roles = defaults.Roles
		warnings = append(warnings, "宠物定位配置缺失，正在使用内置默认规则")
	}
	if err := db.Order("sort_order asc, name asc").Find(&rules.Stances).Error; err != nil || len(rules.Stances) == 0 {
		rules.Stances = defaults.Stances
		warnings = append(warnings, "远征姿态配置缺失，正在使用内置默认规则")
	}
	if err := db.Order("sort_order asc, name asc").Find(&rules.Personalities).Error; err != nil || len(rules.Personalities) == 0 {
		rules.Personalities = defaults.Personalities
		warnings = append(warnings, "性格规则配置缺失，正在使用内置默认规则")
	}
	if err := db.Order("sort_order asc, category asc, entry_key asc").Find(&rules.Codex).Error; err != nil || len(rules.Codex) == 0 {
		rules.Codex = defaults.Codex
		warnings = append(warnings, "图鉴目录配置缺失，正在使用内置默认目录")
	}
	return rules, warnings
}

func EnabledRoles(db *gorm.DB) []models.GrowthRoleConfig {
	rules, _ := Load(db)
	result := make([]models.GrowthRoleConfig, 0, len(rules.Roles))
	for _, role := range rules.Roles {
		if role.Enabled {
			result = append(result, role)
		}
	}
	return result
}

func EnabledStances(db *gorm.DB) []models.GrowthStanceConfig {
	rules, _ := Load(db)
	result := make([]models.GrowthStanceConfig, 0, len(rules.Stances))
	for _, stance := range rules.Stances {
		if stance.Enabled {
			result = append(result, stance)
		}
	}
	return result
}

func Skills(role models.GrowthRoleConfig) string {
	return strings.Join([]string{role.Skill1, role.Skill2, role.Skill3}, "、")
}

func ResolveTrait(db *gorm.DB, profile models.PetBehaviorProfile) string {
	rules, _ := Load(db)
	type candidate struct {
		name  string
		value int64
		order int
	}
	candidates := make([]candidate, 0, len(rules.Personalities))
	for _, rule := range rules.Personalities {
		if !rule.Enabled || rule.MinThreshold <= 0 {
			continue
		}
		value := map[string]int64{"care": profile.Care, "explore": profile.Explore, "support": profile.Support}[rule.Dimension]
		if value >= rule.MinThreshold {
			candidates = append(candidates, candidate{name: rule.Name, value: value, order: rule.SortOrder})
		}
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].value == candidates[j].value {
			return candidates[i].order < candidates[j].order
		}
		return candidates[i].value > candidates[j].value
	})
	if len(candidates) == 0 {
		return ""
	}
	return candidates[0].name
}

func Validate(rules RuleSet) error {
	if len(rules.Roles) == 0 || len(rules.Stances) == 0 || len(rules.Personalities) == 0 || len(rules.Codex) == 0 {
		return fmt.Errorf("成长规则域不能为空")
	}
	return nil
}
