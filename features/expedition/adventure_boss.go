package expedition

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"qq-pet-saas/gameplay"
	"qq-pet-saas/models"
)

var (
	ErrBossUnavailable       = errors.New("地图首领当前没有出现")
	ErrBossCommunityOnly     = errors.New("地图首领只能在群或频道中挑战")
	ErrBossChallengeLimit    = errors.New("本次首领的挑战次数已用完")
	ErrBossChallengeCooldown = errors.New("地图首领挑战仍在冷却")
	ErrBossRewardUnavailable = errors.New("地图首领奖励尚不可领取")
)

type AdventureBossView struct {
	Config   models.AdventureBossConfig   `json:"config"`
	Instance models.AdventureBossInstance `json:"instance"`
}

type AdventureBossRewardResult struct {
	Instance     models.AdventureBossInstance     `json:"instance"`
	Contribution models.AdventureBossContribution `json:"contribution"`
	Rewards      []AdventureReward                `json:"rewards"`
}

func bossWindow(config models.AdventureBossConfig, now time.Time) (time.Time, time.Time, bool) {
	if now.Before(config.ScheduleAnchor) || config.SpawnIntervalMinutes <= 0 || config.ActiveDurationMinutes <= 0 {
		return time.Time{}, time.Time{}, false
	}
	interval := time.Duration(config.SpawnIntervalMinutes) * time.Minute
	cycles := int64(now.Sub(config.ScheduleAnchor) / interval)
	startsAt := config.ScheduleAnchor.Add(time.Duration(cycles) * interval)
	endsAt := startsAt.Add(time.Duration(config.ActiveDurationMinutes) * time.Minute)
	return startsAt, endsAt, !now.Before(startsAt) && now.Before(endsAt)
}

func bossInstanceID(bossKey, communityID string, startsAt time.Time) string {
	return fmt.Sprintf("%s:%d:%s", bossKey, startsAt.Unix(), communityID)
}

func (service *Service) adventureBossInstanceTx(tx *gorm.DB, config models.AdventureBossConfig, communityID string, now time.Time) (*models.AdventureBossInstance, error) {
	startsAt, endsAt, active := bossWindow(config, now)
	if !active {
		return nil, ErrBossUnavailable
	}
	raw, err := json.Marshal(config)
	if err != nil {
		return nil, err
	}
	instance := models.AdventureBossInstance{ID: bossInstanceID(config.Key, communityID, startsAt), BossKey: config.Key, CommunityID: communityID, WindowKey: startsAt.UTC().Format(time.RFC3339), Status: "active", MaxHealth: config.MaxHealth, CurrentHealth: config.MaxHealth, SnapshotJSON: string(raw), SpawnedAt: startsAt, ExpiresAt: endsAt}
	if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&instance).Error; err != nil {
		return nil, err
	}
	if err := tx.First(&instance, "id = ?", instance.ID).Error; err != nil {
		return nil, err
	}
	if instance.Status == "active" && !now.Before(instance.ExpiresAt) {
		instance.Status = "expired"
		if err := tx.Save(&instance).Error; err != nil {
			return nil, err
		}
	}
	return &instance, nil
}

func (service *Service) ListActiveAdventureBosses(ctx context.Context, communityID string) ([]AdventureBossView, error) {
	if communityID == "" {
		return nil, ErrBossCommunityOnly
	}
	var configs []models.AdventureBossConfig
	if err := service.DB.WithContext(ctx).Where("enabled = ?", true).Order("map_key asc, zone_key asc, key asc").Find(&configs).Error; err != nil {
		return nil, err
	}
	result := make([]AdventureBossView, 0)
	for _, config := range configs {
		if _, _, active := bossWindow(config, service.Now()); !active {
			continue
		}
		var instance *models.AdventureBossInstance
		err := service.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			var err error
			instance, err = service.adventureBossInstanceTx(tx, config, communityID, service.Now())
			return err
		})
		if err != nil {
			return nil, err
		}
		result = append(result, AdventureBossView{Config: config, Instance: *instance})
	}
	return result, nil
}

func (service *Service) StartAdventureBossChallenge(ctx context.Context, accountID, communityID, bossKey string) (*models.AdventureCombatSession, error) {
	if communityID == "" {
		return nil, ErrBossCommunityOnly
	}
	var combat models.AdventureCombatSession
	err := gameplay.WithTransactionRetry(ctx, service.DB, func(tx *gorm.DB) error {
		var config models.AdventureBossConfig
		if err := tx.First(&config, "key = ? AND enabled = ?", bossKey, true).Error; err != nil {
			return ErrBossUnavailable
		}
		instance, err := service.adventureBossInstanceTx(tx, config, communityID, service.Now())
		if err != nil {
			return err
		}
		if instance.Status != "active" || instance.CurrentHealth <= 0 {
			return ErrBossUnavailable
		}
		var pet models.PetProfile
		if err := tx.First(&pet, "account_id = ?", accountID).Error; err != nil {
			return ErrPetRequired
		}
		if pet.Status == "受伤" {
			return ErrAdventureInjured
		}
		if pet.Status != "" && pet.Status != "空闲" {
			return ErrAdventureBusy
		}
		var contribution models.AdventureBossContribution
		lookup := tx.Limit(1).Find(&contribution, "boss_instance_id = ? AND account_id = ?", instance.ID, accountID)
		if lookup.Error != nil {
			return lookup.Error
		}
		if lookup.RowsAffected == 0 {
			contribution = models.AdventureBossContribution{BossInstanceID: instance.ID, AccountID: accountID}
		}
		if config.ChallengeLimit > 0 && contribution.Challenges >= config.ChallengeLimit {
			return ErrBossChallengeLimit
		}
		if contribution.LastChallengeAt != nil && config.ChallengeCooldownMinutes > 0 && service.Now().Before(contribution.LastChallengeAt.Add(time.Duration(config.ChallengeCooldownMinutes)*time.Minute)) {
			return ErrBossChallengeCooldown
		}
		stats, err := service.EquippedStatsTx(tx, accountID)
		if err != nil {
			return err
		}
		now := service.Now()
		combat = models.AdventureCombatSession{ID: uuid.NewString(), AccountID: accountID, CommunityID: communityID, BossInstanceID: instance.ID, MonsterKey: config.MonsterKey, Status: "active", Round: 1, PlayerHealth: pet.Health + stats.Health, MonsterHealth: instance.CurrentHealth, CooldownsJSON: "{}", ExpiresAt: minTime(instance.ExpiresAt, now.Add(10*time.Minute)), StartedAt: now}
		if err := tx.Create(&combat).Error; err != nil {
			return err
		}
		contribution.Challenges++
		contribution.LastChallengeAt = &now
		contribution.UpdatedAt = now
		if err := tx.Save(&contribution).Error; err != nil {
			return err
		}
		pet.Status = "首领战斗"
		return tx.Save(&pet).Error
	})
	return &combat, err
}

func minTime(a, b time.Time) time.Time {
	if a.Before(b) {
		return a
	}
	return b
}

func (service *Service) applyBossDamageTx(tx *gorm.DB, combat *models.AdventureCombatSession, damage int64) (bool, error) {
	if combat.BossInstanceID == "" {
		return false, nil
	}
	if combat.CommunityID != "" {
		if event, err := currentLiveEventTx(tx, service.Now()); err != nil {
			return false, err
		} else if event != nil {
			influence, err := seasonInfluenceTx(tx, event.Key, combat.CommunityID)
			if err != nil {
				return false, err
			}
			if influence.EffectType == "boss_damage_gain_percent" {
				damage += damage * int64(influence.EffectValue) / 100
			}
		}
	}
	update := tx.Model(&models.AdventureBossInstance{}).Where("id = ? AND status = ? AND current_health > 0 AND expires_at > ?", combat.BossInstanceID, "active", service.Now()).Updates(map[string]any{"current_health": gorm.Expr("MAX(0, current_health - ?)", damage)})
	if update.Error != nil {
		return false, update.Error
	}
	if update.RowsAffected != 1 {
		return false, ErrBossUnavailable
	}
	var instance models.AdventureBossInstance
	if err := tx.First(&instance, "id = ?", combat.BossInstanceID).Error; err != nil {
		return false, err
	}
	combat.MonsterHealth = instance.CurrentHealth
	contribution := models.AdventureBossContribution{BossInstanceID: instance.ID, AccountID: combat.AccountID, Damage: damage, UpdatedAt: service.Now()}
	if err := tx.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "boss_instance_id"}, {Name: "account_id"}}, DoUpdates: clause.Assignments(map[string]any{"damage": gorm.Expr("damage + ?", damage), "updated_at": service.Now()})}).Create(&contribution).Error; err != nil {
		return false, err
	}
	if instance.CurrentHealth <= 0 {
		now := service.Now()
		if err := tx.Model(&instance).Updates(map[string]any{"status": "defeated", "defeated_at": now}).Error; err != nil {
			return false, err
		}
		return true, nil
	}
	return false, nil
}

func (service *Service) ClaimAdventureBossReward(ctx context.Context, accountID, instanceID string) (*AdventureBossRewardResult, error) {
	var result AdventureBossRewardResult
	err := gameplay.WithTransactionRetry(ctx, service.DB, func(tx *gorm.DB) error {
		var instance models.AdventureBossInstance
		if err := tx.First(&instance, "id = ?", instanceID).Error; err != nil {
			return ErrBossRewardUnavailable
		}
		if instance.Status == "active" && !service.Now().Before(instance.ExpiresAt) {
			instance.Status = "expired"
			if err := tx.Save(&instance).Error; err != nil {
				return err
			}
		}
		if instance.Status != "defeated" && instance.Status != "expired" {
			return ErrBossRewardUnavailable
		}
		var config models.AdventureBossConfig
		if err := json.Unmarshal([]byte(instance.SnapshotJSON), &config); err != nil {
			return err
		}
		var contribution models.AdventureBossContribution
		if err := tx.First(&contribution, "boss_instance_id = ? AND account_id = ?", instance.ID, accountID).Error; err != nil {
			return ErrBossRewardUnavailable
		}
		if contribution.Damage < config.MinimumContribution {
			return ErrBossRewardUnavailable
		}
		var existing int64
		if err := tx.Model(&models.AdventureBossRewardClaim{}).Where("boss_instance_id = ? AND account_id = ?", instance.ID, accountID).Count(&existing).Error; err != nil {
			return err
		}
		if existing > 0 {
			return ErrBossRewardUnavailable
		}
		pool := config.DefeatedLootPoolKey
		if instance.Status == "expired" {
			pool = config.ExpiredLootPoolKey
		}
		rewards, err := service.grantLootPoolTx(tx, accountID, pool, "boss:"+instance.ID, false)
		if err != nil {
			return err
		}
		var tiers []models.AdventureBossRewardTierConfig
		if err := tx.Where("boss_key = ? AND threshold <= ?", config.Key, contribution.Damage).Order("threshold asc").Find(&tiers).Error; err != nil {
			return err
		}
		for _, tier := range tiers {
			tierRewards, err := service.grantLootPoolTx(tx, accountID, tier.LootPoolKey, fmt.Sprintf("boss:%s:tier:%d", instance.ID, tier.Threshold), false)
			if err != nil {
				return err
			}
			rewards = append(rewards, tierRewards...)
		}
		raw, _ := json.Marshal(rewards)
		claim := models.AdventureBossRewardClaim{ID: uuid.NewString(), BossInstanceID: instance.ID, AccountID: accountID, RewardJSON: string(raw), ClaimedAt: service.Now()}
		if err := tx.Create(&claim).Error; err != nil {
			return err
		}
		if instance.Status == "defeated" {
			if _, err := service.advanceObjectivesTx(tx, accountID, config.ZoneKey, "boss_kill", config.Key, 1); err != nil {
				return err
			}
			var zone models.AdventureZoneConfig
			if err := tx.First(&zone, "key = ?", config.ZoneKey).Error; err != nil {
				return err
			}
			if _, err := service.recomputeZoneProgressTx(tx, accountID, zone); err != nil {
				return err
			}
		}
		result = AdventureBossRewardResult{Instance: instance, Contribution: contribution, Rewards: rewards}
		return nil
	})
	return &result, err
}
