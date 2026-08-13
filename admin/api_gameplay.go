package admin

import (
	"fmt"
	"strings"

	"github.com/gin-gonic/gin"
	"qq-pet-saas/gameplayrules"
	"qq-pet-saas/models"
)

type gameplayDistribution struct {
	Name        string  `json:"name"`
	Count       int64   `json:"count"`
	Percentage  float64 `json:"percentage"`
	Description string  `json:"description,omitempty"`
	Enabled     bool    `json:"enabled"`
	Skills      string  `json:"skills,omitempty"`
}

type gameplayGrowthSummary struct {
	PlayerCount              int64   `json:"player_count"`
	RoleCoverageRate         float64 `json:"role_coverage_rate"`
	PersonalityFormationRate float64 `json:"personality_formation_rate"`
	PersonalityUnformed      int64   `json:"personality_unformed"`
	ConfiguredRuleCount      int     `json:"configured_rule_count"`
	ConfigurationComplete    bool    `json:"configuration_complete"`
}

func percentage(count, total int64) float64 {
	if total == 0 {
		return 0
	}
	return float64(count) * 100 / float64(total)
}

func (api *EcosystemAPI) GameplayGrowth(c *gin.Context) {
	rules, warnings := gameplayrules.Load(api.DB)
	roleCounts := map[string]int64{}
	stanceCounts := map[string]int64{}
	traitCounts := map[string]int64{}
	var totalPlayers, formedPersonalities int64
	if err := api.DB.Model(&models.PetProfile{}).Count(&totalPlayers).Error; err != nil {
		Error(c, 5000, "成长统计读取失败")
		return
	}
	type countRow struct {
		Name  string
		Count int64
	}
	roleRows := make([]countRow, 0)
	stanceRows := make([]countRow, 0)
	traitRows := make([]countRow, 0)
	if err := api.DB.Model(&models.PetProfile{}).Select("role name, COUNT(*) count").Where("role <> ''").Group("role").Scan(&roleRows).Error; err != nil {
		Error(c, 5000, "定位统计读取失败")
		return
	}
	if err := api.DB.Model(&models.PetProfile{}).Select("stance name, COUNT(*) count").Where("stance <> ''").Group("stance").Scan(&stanceRows).Error; err != nil {
		Error(c, 5000, "姿态统计读取失败")
		return
	}
	if err := api.DB.Model(&models.PetBehaviorProfile{}).Select("trait name, COUNT(*) count").Where("trait <> ''").Group("trait").Scan(&traitRows).Error; err != nil {
		Error(c, 5000, "性格统计读取失败")
		return
	}
	for _, row := range roleRows {
		roleCounts[row.Name] = row.Count
	}
	for _, row := range stanceRows {
		stanceCounts[row.Name] = row.Count
	}
	for _, row := range traitRows {
		traitCounts[row.Name] = row.Count
		formedPersonalities += row.Count
	}

	roles := make([]gameplayDistribution, 0, len(rules.Roles))
	skills := make([]gameplayDistribution, 0, len(rules.Roles))
	var roleCovered int64
	for _, role := range rules.Roles {
		count := roleCounts[role.Name]
		roleCovered += count
		roles = append(roles, gameplayDistribution{Name: role.Name, Count: count, Percentage: percentage(count, totalPlayers), Description: role.Description, Enabled: role.Enabled, Skills: gameplayrules.Skills(role)})
		skills = append(skills, gameplayDistribution{Name: role.Name + "技能组", Count: count, Percentage: percentage(count, totalPlayers), Description: gameplayrules.Skills(role), Enabled: role.Enabled})
	}
	stances := make([]gameplayDistribution, 0, len(rules.Stances))
	for _, stance := range rules.Stances {
		count := stanceCounts[stance.Name]
		stances = append(stances, gameplayDistribution{Name: stance.Name, Count: count, Percentage: percentage(count, totalPlayers), Description: stance.Description, Enabled: stance.Enabled})
	}
	personalities := make([]gameplayDistribution, 0, len(rules.Personalities))
	for _, personality := range rules.Personalities {
		count := traitCounts[personality.Name]
		personalities = append(personalities, gameplayDistribution{Name: personality.Name, Count: count, Percentage: percentage(count, totalPlayers), Description: personality.Description, Enabled: personality.Enabled})
	}
	summary := gameplayGrowthSummary{
		PlayerCount: totalPlayers, RoleCoverageRate: percentage(roleCovered, totalPlayers),
		PersonalityFormationRate: percentage(formedPersonalities, totalPlayers),
		PersonalityUnformed:      totalPlayers - formedPersonalities,
		ConfiguredRuleCount:      len(rules.Roles) + len(rules.Stances) + len(rules.Personalities) + len(rules.Codex),
		ConfigurationComplete:    len(warnings) == 0,
	}
	Success(c, gin.H{
		"summary": summary, "roles": roles, "stances": stances, "personalities": personalities, "skills": skills,
		"rules":    gin.H{"roles": rules.Roles, "stances": rules.Stances, "personalities": rules.Personalities, "codex": rules.Codex},
		"warnings": warnings,
	})
}

type codexCatalogItem struct {
	ID                uint    `json:"id"`
	Category          string  `json:"category"`
	EntryKey          string  `json:"entry_key"`
	Region            string  `json:"region"`
	Description       string  `json:"description"`
	Enabled           bool    `json:"enabled"`
	SortOrder         int     `json:"sort_order"`
	DiscoveredPlayers int64   `json:"discovered_players"`
	CompletedPlayers  int64   `json:"completed_players"`
	AverageProgress   float64 `json:"average_progress"`
}

func (api *EcosystemAPI) GameplayCodex(c *gin.Context) {
	rules, warnings := gameplayrules.Load(api.DB)
	items := make([]codexCatalogItem, 0, len(rules.Codex))
	queryText := strings.ToLower(strings.TrimSpace(c.Query("query")))
	category := strings.TrimSpace(c.Query("category"))
	region := strings.TrimSpace(c.Query("region"))
	status := strings.TrimSpace(c.Query("status"))
	for _, catalog := range rules.Codex {
		if category != "" && catalog.Category != category || region != "" && catalog.Region != region {
			continue
		}
		if status == "enabled" && !catalog.Enabled || status == "disabled" && catalog.Enabled {
			continue
		}
		if queryText != "" && !strings.Contains(strings.ToLower(catalog.EntryKey+" "+catalog.Description), queryText) {
			continue
		}
		var aggregate struct {
			Discovered int64
			Completed  int64
			Average    float64
		}
		if err := api.DB.Model(&models.CodexEntry{}).
			Select("COUNT(*) discovered, COALESCE(SUM(CASE WHEN progress >= 100 THEN 1 ELSE 0 END), 0) completed, COALESCE(AVG(progress), 0) average").
			Where("category = ? AND entry_key = ? AND progress > 0", catalog.Category, catalog.EntryKey).Scan(&aggregate).Error; err != nil {
			Error(c, 5000, fmt.Sprintf("图鉴统计读取失败: %s", catalog.EntryKey))
			return
		}
		items = append(items, codexCatalogItem{
			ID: catalog.ID, Category: catalog.Category, EntryKey: catalog.EntryKey, Region: catalog.Region, Description: catalog.Description,
			Enabled: catalog.Enabled, SortOrder: catalog.SortOrder, DiscoveredPlayers: aggregate.Discovered,
			CompletedPlayers: aggregate.Completed, AverageProgress: aggregate.Average,
		})
	}
	var discoveredEntries int64
	for _, item := range items {
		if item.DiscoveredPlayers > 0 {
			discoveredEntries++
		}
	}
	Success(c, gin.H{
		"summary": gin.H{"catalog_count": len(items), "discovered_entries": discoveredEntries, "discovery_rate": percentage(discoveredEntries, int64(len(items)))},
		"items":   items, "warnings": warnings,
	})
}
