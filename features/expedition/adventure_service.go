package expedition

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"qq-pet-saas/gameplay"
	"qq-pet-saas/models"
)

var (
	ErrAdventureBusy       = errors.New("宠物正在进行其他行动")
	ErrAdventureInjured    = errors.New("宠物受伤了，需要先恢复")
	ErrZoneLocked          = errors.New("区域尚未解锁")
	ErrNoEncounter         = errors.New("区域没有可用的探索遭遇")
	ErrNoActiveCombat      = errors.New("当前没有进行中的探索战斗")
	ErrCombatExpired       = errors.New("战斗已超时")
	ErrInvalidCombatAction = errors.New("无效的战斗行动")
)

// CombatSkillCooldownError keeps a player-facing reason when a valid skill is
// temporarily unavailable. The remaining turns are counted after the current
// round has been resolved.
type CombatSkillCooldownError struct {
	SkillName      string
	RemainingTurns int
}

func (err *CombatSkillCooldownError) Error() string {
	return fmt.Sprintf("技能 %s 仍在冷却，还需等待 %d 回合", err.SkillName, err.RemainingTurns)
}

type AdventureZoneView struct {
	Zone                 models.AdventureZoneConfig `json:"zone"`
	Unlocked             bool                       `json:"unlocked"`
	ExplorationPercent   int                        `json:"exploration_percent"`
	ExpeditionUnlocked   bool                       `json:"expedition_unlocked"`
	MissingPrerequisites []string                   `json:"missing_prerequisites"`
	StageName            string                     `json:"stage_name,omitempty"`
	CurrentGoal          string                     `json:"current_goal,omitempty"`
}

type AdventureMapView struct {
	Map   models.AdventureMapConfig `json:"map"`
	Zones []AdventureZoneView       `json:"zones"`
}

type ExplorationResult struct {
	Session   models.AdventureExplorationSession `json:"session"`
	Encounter models.AdventureEncounterConfig    `json:"encounter"`
	Combat    *models.AdventureCombatSession     `json:"combat,omitempty"`
	Progress  models.PlayerZoneProgress          `json:"progress"`
	Event     *AdventureStoryEventView           `json:"event,omitempty"`
	StageName string                             `json:"stage_name,omitempty"`
	Goal      string                             `json:"goal,omitempty"`
}

type AdventureCombatResult struct {
	Session            models.AdventureCombatSession `json:"session"`
	Turn               models.AdventureCombatTurn    `json:"turn"`
	Rewards            []AdventureReward             `json:"rewards"`
	AdventureXP        int64                         `json:"adventure_xp"`
	AdventureLevel     int                           `json:"adventure_level"`
	ZoneProgress       int                           `json:"zone_progress"`
	ExpeditionUnlocked bool                          `json:"expedition_unlocked"`
}

func ensureAdventureProgressTx(tx *gorm.DB, accountID string, progress *models.PlayerAdventureProgress, now time.Time) error {
	seed := models.PlayerAdventureProgress{AccountID: accountID, Level: 1, XP: 0, UpdatedAt: now}
	if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&seed).Error; err != nil {
		return err
	}
	return tx.First(progress, "account_id = ?", accountID).Error
}

func adventureXPForNextLevelTx(tx *gorm.DB, level int) int64 {
	if level < 1 {
		level = 1
	}
	if tx.Migrator().HasTable(&models.AdventureLevelConfig{}) {
		var configured models.AdventureLevelConfig
		if result := tx.Limit(1).Find(&configured, "level = ?", level); result.Error == nil && result.RowsAffected > 0 {
			return configured.XPToNext
		}
	}
	return 100 + int64(50*(level-1))
}

func (service *Service) addAdventureXPTx(tx *gorm.DB, accountID string, amount int64) (models.PlayerAdventureProgress, error) {
	var progress models.PlayerAdventureProgress
	if err := ensureAdventureProgressTx(tx, accountID, &progress, service.Now()); err != nil {
		return progress, err
	}
	if amount <= 0 {
		return progress, nil
	}
	progress.XP += amount
	startedLevel := progress.Level
	for next := adventureXPForNextLevelTx(tx, progress.Level); next > 0 && progress.XP >= next; next = adventureXPForNextLevelTx(tx, progress.Level) {
		progress.XP -= next
		progress.Level++
	}
	progress.UpdatedAt = service.Now()
	if err := tx.Save(&progress).Error; err != nil {
		return progress, err
	}
	if progress.Level > startedLevel {
		var pets []models.PetProfile
		if err := tx.Where("account_id = ?", accountID).Find(&pets).Error; err != nil {
			return progress, err
		}
		for i := range pets {
			if err := gameplay.RefreshPetSkillsTx(tx, &pets[i]); err != nil {
				return progress, err
			}
		}
	}
	return progress, nil
}

func (service *Service) ListAdventureMaps(ctx context.Context, accountID string) ([]AdventureMapView, error) {
	var maps []models.AdventureMapConfig
	if err := service.DB.WithContext(ctx).Where("enabled = ?", true).Order("sort_order asc, key asc").Find(&maps).Error; err != nil {
		return nil, err
	}
	result := make([]AdventureMapView, 0, len(maps))
	for _, adventureMap := range maps {
		var zones []models.AdventureZoneConfig
		if err := service.DB.WithContext(ctx).Where("map_key = ? AND enabled = ?", adventureMap.Key, true).Order("sort_order asc, key asc").Find(&zones).Error; err != nil {
			return nil, err
		}
		view := AdventureMapView{Map: adventureMap, Zones: make([]AdventureZoneView, 0, len(zones))}
		for _, zone := range zones {
			unlocked, missing, err := service.zoneAccessibleTx(service.DB.WithContext(ctx), accountID, zone.Key)
			if err != nil {
				return nil, err
			}
			var progress models.PlayerZoneProgress
			lookup := service.DB.WithContext(ctx).Limit(1).Find(&progress, "account_id = ? AND zone_key = ?", accountID, zone.Key)
			if lookup.Error != nil {
				return nil, lookup.Error
			}
			stageName, currentGoal := "", ""
			if nodeMode(zone) {
				progress, stageName, currentGoal, err = service.nodeProgressForZone(ctx, accountID, zone)
				if err != nil {
					return nil, err
				}
			}
			view.Zones = append(view.Zones, AdventureZoneView{Zone: zone, Unlocked: unlocked, MissingPrerequisites: missing, ExplorationPercent: progress.ExplorationPercent, ExpeditionUnlocked: progress.ExpeditionUnlocked, StageName: stageName, CurrentGoal: currentGoal})
		}
		result = append(result, view)
	}
	return result, nil
}

func (service *Service) zoneAccessibleTx(tx *gorm.DB, accountID, zoneKey string) (bool, []string, error) {
	var prerequisites []models.AdventureZonePrerequisiteConfig
	if err := tx.Where("zone_key = ?", zoneKey).Find(&prerequisites).Error; err != nil {
		return false, nil, err
	}
	missing := make([]string, 0)
	for _, prerequisite := range prerequisites {
		var progress models.PlayerZoneProgress
		lookup := tx.Limit(1).Find(&progress, "account_id = ? AND zone_key = ? AND expedition_unlocked = ?", accountID, prerequisite.PrerequisiteZoneKey, true)
		if lookup.Error != nil {
			return false, nil, lookup.Error
		}
		if lookup.RowsAffected == 0 {
			missing = append(missing, zoneDisplayNameTx(tx, prerequisite.PrerequisiteZoneKey))
		}
	}
	return len(missing) == 0, missing, nil
}

func (service *Service) ExploreZone(ctx context.Context, accountID, zoneKey string) (*ExplorationResult, error) {
	return service.ExploreZoneInCommunity(ctx, accountID, "", zoneKey)
}

func (service *Service) ExploreZoneInCommunity(ctx context.Context, accountID, communityID, zoneKey string) (*ExplorationResult, error) {
	var configuredZone models.AdventureZoneConfig
	if err := service.DB.WithContext(ctx).First(&configuredZone, "key = ? AND enabled = ?", zoneKey, true).Error; err != nil {
		return nil, ErrZoneLocked
	}
	if nodeMode(configuredZone) {
		return service.exploreNodeZoneInCommunity(ctx, accountID, communityID, configuredZone)
	}
	var result ExplorationResult
	err := gameplay.WithTransactionRetry(ctx, service.DB, func(tx *gorm.DB) error {
		var zone models.AdventureZoneConfig
		if err := tx.First(&zone, "key = ? AND enabled = ?", zoneKey, true).Error; err != nil {
			return ErrZoneLocked
		}
		var adventureMap models.AdventureMapConfig
		if err := tx.First(&adventureMap, "key = ? AND enabled = ?", zone.MapKey, true).Error; err != nil {
			return ErrZoneLocked
		}
		accessible, missing, err := service.zoneAccessibleTx(tx, accountID, zone.Key)
		if err != nil {
			return err
		}
		if !accessible {
			return fmt.Errorf("%w: 需要先完成 %s", ErrZoneLocked, strings.Join(missing, "、"))
		}
		petRow, err := gameplay.ActivePetTx(tx, accountID)
		if err != nil {
			return err
		}
		pet := *petRow
		if pet.Status == "受伤" {
			return ErrAdventureInjured
		}
		if err := gameplay.ReservePetRunTx(tx, accountID, pet.ID); err != nil {
			if errors.Is(err, gameplay.ErrTooManyConcurrentRuns) || errors.Is(err, gameplay.ErrActivityActive) {
				return ErrAdventureBusy
			}
			return err
		}
		if zone.HungerCost > 0 && pet.Hunger-zone.HungerCost <= 10 {
			return gameplay.ErrPetTooHungry
		}
		if zone.ReadinessCost > 0 && pet.Readiness < zone.ReadinessCost {
			return ErrInsufficientReadiness
		}
		var encounters []models.AdventureEncounterConfig
		if err := tx.Where("zone_key = ? AND enabled = ? AND weight > 0", zone.Key, true).Order("sort_order asc, id asc").Find(&encounters).Error; err != nil {
			return err
		}
		if len(encounters) == 0 {
			return ErrNoEncounter
		}
		total := 0
		for _, encounter := range encounters {
			total += encounter.Weight
		}
		roll, err := service.RandomIntn(total)
		if err != nil {
			return err
		}
		selected := encounters[0]
		for _, encounter := range encounters {
			roll -= encounter.Weight
			if roll < 0 {
				selected = encounter
				break
			}
		}
		now := service.Now()
		session := models.AdventureExplorationSession{ID: uuid.NewString(), AccountID: accountID, PetID: pet.ID, CommunityID: communityID, MapKey: adventureMap.Key, ZoneKey: zone.Key, EncounterKey: selected.EncounterKey, Status: "active", StartedAt: now}
		pet.Hunger -= zone.HungerCost
		pet.Readiness -= zone.ReadinessCost
		pet.Status = "探索"
		if err := tx.Save(&pet).Error; err != nil {
			return err
		}
		if err := tx.Create(&session).Error; err != nil {
			return err
		}
		if _, err := service.advanceObjectivesTx(tx, accountID, zone.Key, "enter", "", 1); err != nil {
			return err
		}
		result.Session, result.Encounter = session, selected
		if selected.EncounterType == "monster" {
			combat, err := service.startAdventureCombatTx(tx, &session, &pet, selected.TargetKey)
			if err != nil {
				return err
			}
			result.Combat = combat
		} else {
			if selected.EncounterType == "landmark" {
				if _, err := service.advanceObjectivesTx(tx, accountID, zone.Key, "landmark", selected.TargetKey, 1); err != nil {
					return err
				}
			}
			session.Status = "completed"
			session.FinishedAt = &now
			pet.Status = "空闲"
			if err := tx.Save(&session).Error; err != nil {
				return err
			}
			if err := tx.Save(&pet).Error; err != nil {
				return err
			}
		}
		progress, err := service.recomputeZoneProgressTx(tx, accountID, zone)
		if err != nil {
			return err
		}
		result.Progress = progress
		return nil
	})
	return &result, err
}

func (service *Service) startAdventureCombatTx(tx *gorm.DB, exploration *models.AdventureExplorationSession, pet *models.PetProfile, monsterKey string) (*models.AdventureCombatSession, error) {
	var monster models.AdventureMonsterConfig
	if err := tx.First(&monster, "key = ? AND enabled = ?", monsterKey, true).Error; err != nil {
		return nil, err
	}
	stats, err := service.EquippedStatsForPetTx(tx, pet.AccountID, pet.ID)
	if err != nil {
		return nil, err
	}
	difficulty := 1000
	var zone models.AdventureZoneConfig
	if err := tx.First(&zone, "key = ?", exploration.ZoneKey).Error; err == nil && zone.DifficultyPermille > 0 {
		difficulty = zone.DifficultyPermille
	}
	monsterHealth := monster.MaxHealth * int64(difficulty) / 1000
	if monsterHealth < 1 {
		monsterHealth = 1
	}
	combat := models.AdventureCombatSession{
		ID: uuid.NewString(), AccountID: pet.AccountID, PetID: pet.ID, CommunityID: exploration.CommunityID, ExplorationID: exploration.ID, MonsterKey: monster.Key,
		Status: "active", Round: 1, PlayerHealth: pet.Health + stats.Health, MonsterHealth: monsterHealth,
		CooldownsJSON: "{}", ExpiresAt: service.Now().Add(10 * time.Minute), StartedAt: service.Now(),
	}
	if err := tx.Create(&combat).Error; err != nil {
		return nil, err
	}
	pet.Status = "探索战斗"
	if err := tx.Save(pet).Error; err != nil {
		return nil, err
	}
	return &combat, nil
}

func (service *Service) advanceObjectivesTx(tx *gorm.DB, accountID, zoneKey, objectiveType, targetKey string, amount int64) ([]models.AdventureObjectiveConfig, error) {
	query := tx.Where("zone_key = ? AND objective_type = ? AND enabled = ?", zoneKey, objectiveType, true)
	if targetKey != "" {
		query = query.Where("target_key = ?", targetKey)
	} else {
		query = query.Where("target_key = ''")
	}
	var objectives []models.AdventureObjectiveConfig
	if err := query.Find(&objectives).Error; err != nil {
		return nil, err
	}
	completed := make([]models.AdventureObjectiveConfig, 0)
	for _, objective := range objectives {
		var progress models.PlayerObjectiveProgress
		lookup := tx.Limit(1).Find(&progress, "account_id = ? AND objective_key = ?", accountID, objective.Key)
		if lookup.Error != nil {
			return nil, lookup.Error
		}
		wasComplete := progress.CompletedAt != nil
		if lookup.RowsAffected == 0 {
			progress = models.PlayerObjectiveProgress{AccountID: accountID, ObjectiveKey: objective.Key}
		}
		progress.Progress += amount
		if progress.Progress > objective.RequiredCount {
			progress.Progress = objective.RequiredCount
		}
		if progress.Progress >= objective.RequiredCount && progress.CompletedAt == nil {
			now := service.Now()
			progress.CompletedAt = &now
		}
		progress.UpdatedAt = service.Now()
		if err := tx.Save(&progress).Error; err != nil {
			return nil, err
		}
		if !wasComplete && progress.CompletedAt != nil {
			completed = append(completed, objective)
			if objective.CodexCategory != "" && objective.CodexEntry != "" && objective.CodexProgress > 0 {
				entry := models.CodexEntry{AccountID: accountID, Category: objective.CodexCategory, EntryKey: objective.CodexEntry, Progress: objective.CodexProgress, UpdatedAt: service.Now()}
				if err := tx.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "account_id"}, {Name: "category"}, {Name: "entry_key"}}, DoUpdates: clause.Assignments(map[string]any{"progress": gorm.Expr("MIN(100, progress + ?)", objective.CodexProgress), "updated_at": service.Now()})}).Create(&entry).Error; err != nil {
					return nil, err
				}
			}
		}
	}
	return completed, nil
}

func (service *Service) recomputeZoneProgressTx(tx *gorm.DB, accountID string, zone models.AdventureZoneConfig) (models.PlayerZoneProgress, error) {
	var objectives []models.AdventureObjectiveConfig
	if err := tx.Where("zone_key = ? AND enabled = ?", zone.Key, true).Find(&objectives).Error; err != nil {
		return models.PlayerZoneProgress{}, err
	}
	totalWeight, completedWeight := 0, 0
	unlocked := zone.ExpeditionUnlockObjectiveKey == ""
	for _, objective := range objectives {
		totalWeight += objective.Weight
		var progress models.PlayerObjectiveProgress
		if err := tx.Limit(1).Find(&progress, "account_id = ? AND objective_key = ?", accountID, objective.Key).Error; err != nil {
			return models.PlayerZoneProgress{}, err
		}
		if progress.CompletedAt != nil {
			completedWeight += objective.Weight
			if objective.Key == zone.ExpeditionUnlockObjectiveKey {
				unlocked = true
			}
		}
	}
	percent := 0
	if totalWeight > 0 {
		percent = completedWeight * 100 / totalWeight
	}
	var progress models.PlayerZoneProgress
	lookup := tx.Limit(1).Find(&progress, "account_id = ? AND zone_key = ?", accountID, zone.Key)
	if lookup.Error != nil {
		return progress, lookup.Error
	}
	if lookup.RowsAffected == 0 {
		progress = models.PlayerZoneProgress{AccountID: accountID, ZoneKey: zone.Key}
	}
	progress.ExplorationPercent = percent
	progress.ExpeditionUnlocked = unlocked
	if unlocked && progress.FirstClearedAt == nil {
		now := service.Now()
		progress.FirstClearedAt = &now
	}
	progress.UpdatedAt = service.Now()
	return progress, tx.Save(&progress).Error
}

func (service *Service) CombatAction(ctx context.Context, accountID, actionKey, action string) (*AdventureCombatResult, error) {
	var result AdventureCombatResult
	expired := false
	err := gameplay.WithTransactionRetry(ctx, service.DB, func(tx *gorm.DB) error {
		var combat models.AdventureCombatSession
		if err := tx.Where("account_id = ? AND status = ?", accountID, "active").Order("started_at desc").First(&combat).Error; err != nil {
			return ErrNoActiveCombat
		}
		var existing models.AdventureCombatTurn
		if lookup := tx.Limit(1).Find(&existing, "action_key = ?", actionKey); lookup.Error != nil {
			return lookup.Error
		} else if lookup.RowsAffected > 0 {
			combatLookup := tx.First(&combat, "id = ?", existing.SessionID)
			if combatLookup.Error != nil {
				return combatLookup.Error
			}
			result.Session, result.Turn = combat, existing
			return nil
		}
		if !service.Now().Before(combat.ExpiresAt) {
			if err := service.finishCombatTx(tx, &combat, "expired"); err != nil {
				return err
			}
			expired = true
			return nil
		}
		petRow, err := gameplay.PetByIDTx(tx, accountID, combat.PetID)
		if err != nil {
			return err
		}
		if err = gameplay.RefreshPetSkillsTx(tx, petRow); err != nil {
			return err
		}
		pet := *petRow
		var monster models.AdventureMonsterConfig
		if err := tx.First(&monster, "key = ?", combat.MonsterKey).Error; err != nil {
			return err
		}
		if combat.BossInstanceID != "" {
			var instance models.AdventureBossInstance
			if err := tx.First(&instance, "id = ?", combat.BossInstanceID).Error; err != nil {
				return err
			}
			var boss models.AdventureBossConfig
			if err := json.Unmarshal([]byte(instance.SnapshotJSON), &boss); err != nil {
				return err
			}
			monster.Level, monster.MaxHealth, monster.Attack, monster.Defense, monster.Wisdom = boss.RecommendedLevel, boss.MaxHealth, boss.Attack, boss.Defense, boss.Wisdom
		} else if combat.ExplorationID != "" {
			var exploration models.AdventureExplorationSession
			if err := tx.First(&exploration, "id = ?", combat.ExplorationID).Error; err != nil {
				return err
			}
			var zone models.AdventureZoneConfig
			if err := tx.First(&zone, "key = ?", exploration.ZoneKey).Error; err != nil {
				return err
			}
			if zone.DifficultyPermille > 0 {
				monster.Attack = monster.Attack * int64(zone.DifficultyPermille) / 1000
				monster.Defense = monster.Defense * int64(zone.DifficultyPermille) / 1000
				monster.Wisdom = monster.Wisdom * int64(zone.DifficultyPermille) / 1000
			}
		}
		stats, err := service.EquippedStatsForPetTx(tx, accountID, combat.PetID)
		if err != nil {
			return err
		}
		playerAttack := pet.Strength + stats.Attack
		playerDefense := pet.Defense + stats.Defense
		playerWisdom := pet.Wisdom + stats.Wisdom
		turn := models.AdventureCombatTurn{ID: uuid.NewString(), SessionID: combat.ID, Round: combat.Round, ActionKey: actionKey, PlayerAction: action, Result: "ongoing", CreatedAt: service.Now()}
		rolls := map[string]int{}
		combat.PlayerDefending = false
		switch {
		case action == "attack":
			damage, damageRolls, err := service.adventureDamage(playerAttack, playerWisdom, monster.Defense, 1000, 0, stats.CritRate, 0)
			if err != nil {
				return err
			}
			turn.PlayerDamage, rolls = damage, damageRolls
			combat.MonsterHealth -= damage
		case action == "defend":
			combat.PlayerDefending = true
		case strings.HasPrefix(action, "skill:"):
			skillKey := strings.TrimPrefix(action, "skill:")
			var skill models.AdventureSkillConfig
			if err := tx.First(&skill, "key = ? AND enabled = ?", skillKey, true).Error; err != nil {
				return ErrInvalidCombatAction
			}
			if !petHasAdventureSkill(pet.Skills, skillKey, skill.Name) {
				return ErrInvalidCombatAction
			}
			cooldowns := map[string]int{}
			_ = json.Unmarshal([]byte(combat.CooldownsJSON), &cooldowns)
			if cooldowns[skillKey] > 0 {
				return &CombatSkillCooldownError{SkillName: skill.Name, RemainingTurns: cooldowns[skillKey]}
			}
			damage, damageRolls, err := service.adventureDamage(playerAttack, playerWisdom, monster.Defense, skill.PowerPermille, skill.WisdomPermille, stats.CritRate, skill.AccuracyPermille)
			if err != nil {
				return err
			}
			turn.PlayerDamage, rolls = damage, damageRolls
			combat.MonsterHealth -= damage
			cooldowns[skillKey] = skill.CooldownTurns + 1
			raw, _ := json.Marshal(cooldowns)
			combat.CooldownsJSON = string(raw)
		case action == "retreat":
			turn.Result = "retreated"
			if err := service.finishCombatTx(tx, &combat, "retreated"); err != nil {
				return err
			}
		default:
			return ErrInvalidCombatAction
		}
		if combat.BossInstanceID != "" && turn.PlayerDamage > 0 {
			if _, err := service.applyBossDamageTx(tx, &combat, turn.PlayerDamage); err != nil {
				return err
			}
		}
		if combat.MonsterHealth <= 0 {
			combat.MonsterHealth = 0
			turn.Result = "victory"
			if err := service.finishCombatTx(tx, &combat, "victory"); err != nil {
				return err
			}
			if combat.BossInstanceID == "" {
				rewards, xp, zoneProgress, err := service.settleCombatVictoryTx(tx, &combat, monster)
				if err != nil {
					return err
				}
				result.Rewards, result.AdventureXP, result.AdventureLevel = rewards, xp, zoneProgress.Level
				result.ZoneProgress, result.ExpeditionUnlocked = zoneProgress.ExplorationPercent, zoneProgress.ExpeditionUnlocked
			} else {
				adventure, err := service.addAdventureXPTx(tx, accountID, monster.AdventureXP)
				if err != nil {
					return err
				}
				result.AdventureXP, result.AdventureLevel = monster.AdventureXP, adventure.Level
			}
		} else if turn.Result == "ongoing" {
			monsterAction, power, wisdomPower, accuracy, err := service.chooseMonsterActionTx(tx, monster)
			if err != nil {
				return err
			}
			monsterDamage, monsterRolls, err := service.adventureDamage(monster.Attack, monster.Wisdom, playerDefense, power, wisdomPower, 0, accuracy)
			if err != nil {
				return err
			}
			for key, value := range monsterRolls {
				rolls["monster_"+key] = value
			}
			dodge, err := service.RandomIntn(1000)
			if err != nil {
				return err
			}
			rolls["player_dodge"] = dodge
			if int64(dodge) < stats.DodgeRate {
				monsterDamage = 0
			}
			if combat.PlayerDefending {
				monsterDamage = int64(math.Ceil(float64(monsterDamage) * .5))
			}
			if stats.DamageReduction > 0 {
				monsterDamage = monsterDamage * (1000 - minInt64(stats.DamageReduction, 800)) / 1000
			}
			turn.MonsterAction, turn.MonsterDamage = monsterAction, monsterDamage
			combat.PlayerHealth -= monsterDamage
			if combat.PlayerHealth <= 0 {
				combat.PlayerHealth = 0
				turn.Result = "defeat"
				if err := service.finishCombatTx(tx, &combat, "defeat"); err != nil {
					return err
				}
			}
		}
		rawRolls, _ := json.Marshal(rolls)
		turn.RollsJSON = string(rawRolls)
		if turn.Result == "ongoing" {
			combat.Round++
			combat.CooldownsJSON = decrementCooldowns(combat.CooldownsJSON)
		}
		if err := tx.Save(&combat).Error; err != nil {
			return err
		}
		if err := tx.Create(&turn).Error; err != nil {
			return err
		}
		result.Session, result.Turn = combat, turn
		return nil
	})
	if err != nil {
		return nil, err
	}
	if expired {
		return nil, ErrCombatExpired
	}
	return &result, nil
}

func (service *Service) chooseMonsterActionTx(tx *gorm.DB, monster models.AdventureMonsterConfig) (string, int, int, int, error) {
	var rows []models.AdventureMonsterSkillConfig
	if err := tx.Where("monster_key = ? AND weight > 0", monster.Key).Order("sort_order asc, id asc").Find(&rows).Error; err != nil {
		return "", 0, 0, 0, err
	}
	if len(rows) == 0 {
		return "attack", 1000, 0, 1000, nil
	}
	total := 1 // Always retain a basic attack option so a skill table cannot deadlock combat.
	for _, row := range rows {
		total += row.Weight
	}
	roll, err := service.RandomIntn(total)
	if err != nil {
		return "", 0, 0, 0, err
	}
	if roll == 0 {
		return "attack", 1000, 0, 1000, nil
	}
	roll--
	for _, row := range rows {
		if roll < row.Weight {
			var skill models.AdventureSkillConfig
			if err := tx.First(&skill, "key = ? AND enabled = ?", row.SkillKey, true).Error; err != nil {
				return "", 0, 0, 0, err
			}
			return "skill:" + skill.Key, skill.PowerPermille, skill.WisdomPermille, skill.AccuracyPermille, nil
		}
		roll -= row.Weight
	}
	return "attack", 1000, 0, 1000, nil
}

func petHasAdventureSkill(raw, key, name string) bool {
	if strings.TrimSpace(raw) == "" {
		return false
	}
	var values []string
	if json.Unmarshal([]byte(raw), &values) != nil {
		values = strings.FieldsFunc(raw, func(r rune) bool { return r == ',' || r == '，' || r == '#' || r == '/' })
	}
	for _, value := range values {
		if strings.TrimSpace(value) == key || strings.TrimSpace(value) == name {
			return true
		}
	}
	return false
}

func decrementCooldowns(raw string) string {
	values := map[string]int{}
	_ = json.Unmarshal([]byte(raw), &values)
	for key, value := range values {
		if value > 0 {
			values[key] = value - 1
		}
	}
	encoded, _ := json.Marshal(values)
	return string(encoded)
}

func minInt64(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}

func (service *Service) adventureDamage(attack, wisdom, defense int64, powerPermille, wisdomPermille int, critRate int64, accuracyPermille int) (int64, map[string]int, error) {
	rolls := map[string]int{}
	if accuracyPermille > 0 {
		accuracy, err := service.RandomIntn(1000)
		if err != nil {
			return 0, rolls, err
		}
		rolls["accuracy"] = accuracy
		if accuracy >= accuracyPermille {
			return 0, rolls, nil
		}
	}
	variance, err := service.RandomIntn(201)
	if err != nil {
		return 0, rolls, err
	}
	rolls["variance"] = variance + 900
	base := (attack*int64(powerPermille) + wisdom*int64(wisdomPermille)) / 1000
	damage := (base - defense/2) * int64(variance+900) / 1000
	if damage < 1 {
		damage = 1
	}
	if critRate > 0 {
		critical, err := service.RandomIntn(1000)
		if err != nil {
			return 0, rolls, err
		}
		rolls["critical"] = critical
		if int64(critical) < critRate {
			damage = damage * 1500 / 1000
		}
	}
	return damage, rolls, nil
}

func (service *Service) finishCombatTx(tx *gorm.DB, combat *models.AdventureCombatSession, status string) error {
	now := service.Now()
	combat.Status, combat.FinishedAt = status, &now
	petRow, err := gameplay.PetByIDTx(tx, combat.AccountID, combat.PetID)
	if err != nil {
		return err
	}
	pet := *petRow
	if status == "defeat" {
		pet.Health, pet.Status = 1, "受伤"
	} else if status == "expired" {
		// 超时只代表玩家暂时离开，不应把尚未结算的战斗伤害变成濒死惩罚。
		// 宠物档案中的生命值仍是开战前已持久化的状态，安全撤离后直接恢复空闲。
		pet.Status = "空闲"
	} else {
		if combat.PlayerHealth < pet.Health {
			pet.Health = combat.PlayerHealth
		}
		pet.Status = "空闲"
	}
	if err := tx.Save(&pet).Error; err != nil {
		return err
	}
	if err := tx.Save(combat).Error; err != nil {
		return err
	}
	if combat.ExplorationID != "" {
		return tx.Model(&models.AdventureExplorationSession{}).Where("id = ?", combat.ExplorationID).Updates(map[string]any{"status": status, "finished_at": now}).Error
	}
	return nil
}

type combatVictoryProgress struct {
	Level              int
	ExplorationPercent int
	ExpeditionUnlocked bool
}

func (service *Service) settleCombatVictoryTx(tx *gorm.DB, combat *models.AdventureCombatSession, monster models.AdventureMonsterConfig) ([]AdventureReward, int64, combatVictoryProgress, error) {
	var exploration models.AdventureExplorationSession
	if err := tx.First(&exploration, "id = ?", combat.ExplorationID).Error; err != nil {
		return nil, 0, combatVictoryProgress{}, err
	}
	var zone models.AdventureZoneConfig
	if err := tx.First(&zone, "key = ?", exploration.ZoneKey).Error; err != nil {
		return nil, 0, combatVictoryProgress{}, err
	}
	firstClear := false
	if nodeMode(zone) {
		// 节点式区域的通关奖励由闭环事件结算，战斗本身不能被当作
		// "首次区域通关"，否则每场主线战斗都会重复命中 first_clear_only 掉落。
		firstClear = false
	} else {
		objectiveType := "monster_kill"
		if monster.Elite {
			objectiveType = "elite_kill"
		}
		completed, err := service.advanceObjectivesTx(tx, combat.AccountID, exploration.ZoneKey, objectiveType, monster.Key, 1)
		if err != nil {
			return nil, 0, combatVictoryProgress{}, err
		}
		firstClear = len(completed) > 0
	}
	var rewards []AdventureReward
	fixed, err := service.grantLootPoolTx(tx, combat.AccountID, monster.FixedLootPoolKey, "combat:"+combat.ID, firstClear)
	if err != nil {
		return nil, 0, combatVictoryProgress{}, err
	}
	random, err := service.grantLootPoolTx(tx, combat.AccountID, monster.RandomLootPoolKey, "combat:"+combat.ID, firstClear)
	if err != nil {
		return nil, 0, combatVictoryProgress{}, err
	}
	rewards = append(rewards, fixed...)
	rewards = append(rewards, random...)
	if reward, err := service.grantCombatVictoryCurrencyTx(tx, combat); err != nil {
		return nil, 0, combatVictoryProgress{}, err
	} else {
		rewards = append(rewards, reward)
	}
	xpAmount := monster.AdventureXP
	if combat.CommunityID != "" {
		if event, eventErr := currentLiveEventTx(tx, service.Now()); eventErr != nil {
			return nil, 0, combatVictoryProgress{}, eventErr
		} else if event != nil {
			influence, influenceErr := seasonInfluenceTx(tx, event.Key, combat.CommunityID)
			if influenceErr != nil {
				return nil, 0, combatVictoryProgress{}, influenceErr
			}
			if influence.EffectType == "adventure_xp_gain_percent" {
				xpAmount += xpAmount * int64(influence.EffectValue) / 100
			}
		}
	}
	adventure, err := service.addAdventureXPTx(tx, combat.AccountID, xpAmount)
	if err != nil {
		return nil, 0, combatVictoryProgress{}, err
	}
	var zoneProgress models.PlayerZoneProgress
	if nodeMode(zone) {
		var node models.PlayerAdventureNodeProgress
		if err := tx.First(&node, "account_id = ? AND zone_key = ?", combat.AccountID, exploration.ZoneKey).Error; err != nil {
			return nil, 0, combatVictoryProgress{}, err
		}
		var stage models.AdventureExplorationStageConfig
		if node.CompletedAt != nil {
			stage = models.AdventureExplorationStageConfig{Key: "completed", ProgressStart: 100, ProgressEnd: 100}
		} else if err := tx.First(&stage, "key = ? AND enabled = ?", exploration.StageKey, true).Error; err != nil {
			return nil, 0, combatVictoryProgress{}, err
		}
		var encounter models.AdventureEncounterConfig
		if err := tx.First(&encounter, "zone_key = ? AND encounter_key = ?", exploration.ZoneKey, exploration.EncounterKey).Error; err != nil {
			return nil, 0, combatVictoryProgress{}, err
		}
		var advanceErr error
		zoneProgress, _, advanceErr = service.advanceNodeClueTx(tx, combat.AccountID, &node, stage, encounter)
		if advanceErr != nil {
			return nil, 0, combatVictoryProgress{}, advanceErr
		}
	} else {
		var progressErr error
		zoneProgress, progressErr = service.recomputeZoneProgressTx(tx, combat.AccountID, zone)
		if progressErr != nil {
			return nil, 0, combatVictoryProgress{}, progressErr
		}
	}
	return rewards, xpAmount, combatVictoryProgress{Level: adventure.Level, ExplorationPercent: zoneProgress.ExplorationPercent, ExpeditionUnlocked: zoneProgress.ExpeditionUnlocked}, nil
}
