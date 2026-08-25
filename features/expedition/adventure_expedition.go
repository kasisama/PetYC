package expedition

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"qq-pet-saas/gameplay"
	"qq-pet-saas/models"
)

type AdventureExpeditionSnapshot struct {
	Config                  models.AdventureExpeditionConfig `json:"config"`
	Map                     models.AdventureMapConfig        `json:"map"`
	Zone                    models.AdventureZoneConfig       `json:"zone"`
	Power                   int64                            `json:"power"`
	Grade                   string                           `json:"grade"`
	BonusRandomRolls        int                              `json:"bonus_random_rolls"`
	InjuryPermille          int                              `json:"injury_permille"`
	AdventureXPBonusPercent int                              `json:"adventure_xp_bonus_percent"`
	RewardBonusPercent      int                              `json:"reward_bonus_percent"`
}

type AdventureExpeditionResult struct {
	Run            models.AdventureExpeditionRun `json:"run"`
	Snapshot       AdventureExpeditionSnapshot   `json:"snapshot"`
	Rewards        []AdventureReward             `json:"rewards"`
	Injured        bool                          `json:"injured"`
	AdventureLevel int                           `json:"adventure_level"`
	EventProgress  int64                         `json:"event_progress"`
	EventRewards   []EventReward                 `json:"event_rewards"`
}

func adventurePower(pet models.PetProfile, progress models.PlayerAdventureProgress, stats EquipmentStats) int64 {
	return int64(progress.Level*10) + pet.Strength + pet.Defense + pet.Wisdom + stats.Attack + stats.Defense + stats.Wisdom + stats.Health/10
}

func expeditionGrade(power, recommended int64) (string, int, int) {
	if recommended <= 0 {
		return "成功", 0, 0
	}
	ratio := power * 1000 / recommended
	switch {
	case ratio >= 1200:
		return "完美", 1, 0
	case ratio >= 900:
		return "成功", 0, 0
	case ratio >= 700:
		return "艰难", 0, 100
	default:
		return "危险", 0, 300
	}
}

func (service *Service) StartAdventureExpedition(ctx context.Context, accountID, zoneKey string) (*models.AdventureExpeditionRun, error) {
	return service.StartAdventureExpeditionInCommunity(ctx, accountID, "", zoneKey)
}

func (service *Service) StartAdventureExpeditionInCommunity(ctx context.Context, accountID, communityID, zoneKey string) (*models.AdventureExpeditionRun, error) {
	var run models.AdventureExpeditionRun
	err := gameplay.WithTransactionRetry(ctx, service.DB, func(tx *gorm.DB) error {
		var config models.AdventureExpeditionConfig
		if err := tx.First(&config, "zone_key = ? AND enabled = ?", zoneKey, true).Error; err != nil {
			return ErrExpeditionUnavailable
		}
		var zone models.AdventureZoneConfig
		if err := tx.First(&zone, "key = ? AND enabled = ?", zoneKey, true).Error; err != nil {
			return ErrExpeditionUnavailable
		}
		var adventureMap models.AdventureMapConfig
		if err := tx.First(&adventureMap, "key = ? AND enabled = ?", zone.MapKey, true).Error; err != nil {
			return ErrExpeditionUnavailable
		}
		var zoneProgress models.PlayerZoneProgress
		if err := tx.First(&zoneProgress, "account_id = ? AND zone_key = ? AND expedition_unlocked = ?", accountID, zoneKey, true).Error; err != nil {
			return ErrZoneLocked
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
		var active int64
		if err := tx.Model(&models.AdventureExpeditionRun{}).Where("account_id = ? AND status = ?", accountID, "running").Count(&active).Error; err != nil {
			return err
		}
		if active > 0 {
			return ErrAdventureBusy
		}
		if err := tx.Model(&models.ExpeditionRun{}).Where("account_id = ? AND status = ?", accountID, "running").Count(&active).Error; err != nil {
			return err
		}
		if active > 0 {
			return ErrAdventureBusy
		}
		if config.HungerCost > 0 && pet.Hunger-config.HungerCost <= 10 {
			return gameplay.ErrPetTooHungry
		}
		if config.ReadinessCost > 0 && pet.Readiness < config.ReadinessCost {
			return ErrInsufficientReadiness
		}
		if config.RequiredItem != "" && config.RequiredQuantity > 0 {
			if err := gameplay.NewInventoryService(tx).DebitTx(tx, accountID, config.RequiredItem, config.RequiredQuantity); err != nil {
				return err
			}
		}
		var progress models.PlayerAdventureProgress
		if err := ensureAdventureProgressTx(tx, accountID, &progress, service.Now()); err != nil {
			return err
		}
		stats, err := service.EquippedStatsTx(tx, accountID)
		if err != nil {
			return err
		}
		power := adventurePower(pet, progress, stats)
		grade, bonusRolls, injury := expeditionGrade(power, config.RecommendedPower)
		snapshot := AdventureExpeditionSnapshot{Config: config, Map: adventureMap, Zone: zone, Power: power, Grade: grade, BonusRandomRolls: bonusRolls, InjuryPermille: injury}
		now := service.Now()
		run = models.AdventureExpeditionRun{ID: uuid.NewString(), AccountID: accountID, CommunityID: communityID, MapKey: adventureMap.Key, ZoneKey: zone.Key, Status: "running", StartedAt: now, EndsAt: now.Add(time.Duration(config.DurationMinutes) * time.Minute)}
		if event, eventErr := currentLiveEventTx(tx, now); eventErr != nil {
			return eventErr
		} else if event != nil && !run.EndsAt.After(event.EndsAt) {
			eligible := event.ProgressSourceMode == "" || event.ProgressSourceMode == "all_expeditions"
			if event.ProgressSourceMode == "selected" {
				var count int64
				if err := tx.Model(&models.LiveEventExpeditionSourceConfig{}).Where("event_key = ? AND zone_key = ?", event.Key, zone.Key).Count(&count).Error; err != nil {
					return err
				}
				eligible = count > 0
			}
			if eligible && config.EventProgressPoints > 0 {
				run.EventKey, run.EventProgressPoints = event.Key, config.EventProgressPoints
			}
			if communityID != "" {
				influence, influenceErr := seasonInfluenceTx(tx, event.Key, communityID)
				if influenceErr != nil {
					return influenceErr
				}
				if influence.EffectType == "adventure_xp_gain_percent" {
					snapshot.AdventureXPBonusPercent = influence.EffectValue
				}
				if influence.EffectType == "expedition_reward_gain_percent" {
					snapshot.RewardBonusPercent = influence.EffectValue
				}
			}
		}
		raw, err := json.Marshal(snapshot)
		if err != nil {
			return err
		}
		run.SnapshotJSON = string(raw)
		pet.Hunger -= config.HungerCost
		pet.Readiness -= config.ReadinessCost
		pet.Status = "远征"
		if err := tx.Save(&pet).Error; err != nil {
			return err
		}
		if err := tx.Create(&run).Error; err != nil {
			if isUniqueConstraintError(err) {
				return ErrAdventureBusy
			}
			return err
		}
		return nil
	})
	return &run, err
}

func (service *Service) ClaimAdventureExpedition(ctx context.Context, accountID string) (*AdventureExpeditionResult, error) {
	var result AdventureExpeditionResult
	err := gameplay.WithTransactionRetry(ctx, service.DB, func(tx *gorm.DB) error {
		var run models.AdventureExpeditionRun
		if err := tx.Where("account_id = ? AND status = ?", accountID, "running").Order("started_at desc").First(&run).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrNothingToClaim
			}
			return err
		}
		if service.Now().Before(run.EndsAt) {
			return ErrExpeditionNotReady
		}
		update := tx.Model(&models.AdventureExpeditionRun{}).Where("id = ? AND status = ?", run.ID, "running").Updates(map[string]any{"status": "claimed", "claimed_at": service.Now()})
		if update.Error != nil {
			return update.Error
		}
		if update.RowsAffected != 1 {
			return ErrNothingToClaim
		}
		run.Status = "claimed"
		now := service.Now()
		run.ClaimedAt = &now
		var snapshot AdventureExpeditionSnapshot
		if err := json.Unmarshal([]byte(run.SnapshotJSON), &snapshot); err != nil {
			return fmt.Errorf("远征快照损坏: %w", err)
		}
		fixed, err := service.grantLootPoolTx(tx, accountID, snapshot.Config.FixedLootPoolKey, "expedition:"+run.ID, false)
		if err != nil {
			return err
		}
		random, err := service.grantLootPoolTx(tx, accountID, snapshot.Config.RandomLootPoolKey, "expedition:"+run.ID, false)
		if err != nil {
			return err
		}
		rewards := append(fixed, random...)
		if snapshot.BonusRandomRolls > 0 {
			bonus, err := service.grantLootPoolWithRollsTx(tx, accountID, snapshot.Config.RandomLootPoolKey, "expedition:"+run.ID+":bonus", false, snapshot.BonusRandomRolls)
			if err != nil {
				return err
			}
			rewards = append(rewards, bonus...)
		}
		if snapshot.RewardBonusPercent > 0 {
			roll, rollErr := service.RandomIntn(100)
			if rollErr != nil {
				return rollErr
			}
			if roll < snapshot.RewardBonusPercent {
				bonus, bonusErr := service.grantLootPoolWithRollsTx(tx, accountID, snapshot.Config.RandomLootPoolKey, "expedition:"+run.ID+":event-bonus", false, 1)
				if bonusErr != nil {
					return bonusErr
				}
				rewards = append(rewards, bonus...)
			}
		}
		xpAmount := snapshot.Config.AdventureXP + snapshot.Config.AdventureXP*int64(snapshot.AdventureXPBonusPercent)/100
		adventure, err := service.addAdventureXPTx(tx, accountID, xpAmount)
		if err != nil {
			return err
		}
		injured := false
		if snapshot.InjuryPermille > 0 {
			roll, err := service.RandomIntn(1000)
			if err != nil {
				return err
			}
			injured = roll < snapshot.InjuryPermille
		}
		var pet models.PetProfile
		if err := tx.First(&pet, "account_id = ?", accountID).Error; err != nil {
			return err
		}
		if injured {
			pet.Health, pet.Status = 1, "受伤"
		} else {
			pet.Status = "空闲"
		}
		if err := tx.Save(&pet).Error; err != nil {
			return err
		}
		var eventProgress int64
		var eventRewards []EventReward
		if run.EventKey != "" && run.EventProgressPoints > 0 {
			eventProgress, eventRewards, err = service.addEventProgressForEventTx(tx, run.EventKey, accountID, "adventure-expedition:"+run.ID, run.EventProgressPoints, service.Now())
			if err != nil {
				return err
			}
		}
		result = AdventureExpeditionResult{Run: run, Snapshot: snapshot, Rewards: rewards, Injured: injured, AdventureLevel: adventure.Level, EventProgress: eventProgress, EventRewards: eventRewards}
		return nil
	})
	return &result, err
}
