package expedition

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"qq-pet-saas/gameplay"
	"qq-pet-saas/models"
)

var ErrNoActiveEvent = errors.New("当前没有进行中的活动")

type EventReward struct {
	Milestone  int64
	RewardType string
	RewardKey  string
	RewardName string
	Quantity   int64
}

type EventStatus struct {
	Event    models.LiveEventConfig
	Progress int64
	Track    []models.RewardTrackConfig
}

func (service *Service) CurrentLiveEvent(ctx context.Context) (*models.LiveEventConfig, error) {
	if service == nil || service.DB == nil {
		return nil, gameplay.ErrDatabaseUnavailable
	}
	event, err := currentLiveEventTx(service.DB.WithContext(ctx), service.Now())
	if err != nil {
		return nil, err
	}
	if event == nil {
		return nil, ErrNoActiveEvent
	}
	return event, nil
}

func currentLiveEventTx(tx *gorm.DB, now time.Time) (*models.LiveEventConfig, error) {
	var event models.LiveEventConfig
	result := tx.Where("active = ? AND starts_at <= ? AND ends_at > ?", true, now, now).
		Order("starts_at DESC").Limit(1).Find(&event)
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected == 0 {
		return nil, nil
	}
	return &event, nil
}

func (service *Service) GetEventStatus(ctx context.Context, accountID string) (*EventStatus, error) {
	event, err := service.CurrentLiveEvent(ctx)
	if err != nil {
		return nil, err
	}
	var progress models.EventProgress
	lookup := service.DB.WithContext(ctx).Limit(1).Find(&progress, "event_key = ? AND account_id = ?", event.Key, accountID)
	if lookup.Error != nil {
		return nil, lookup.Error
	}
	var track []models.RewardTrackConfig
	if err = service.DB.WithContext(ctx).Where("event_key = ?", event.Key).Order("milestone asc, reward_type asc, reward_key asc").Find(&track).Error; err != nil {
		return nil, err
	}
	return &EventStatus{Event: *event, Progress: progress.Progress, Track: track}, nil
}

func (service *Service) AddEventProgress(ctx context.Context, accountID, sourceKey string, delta int64) (int64, []EventReward, error) {
	var progress int64
	var rewards []EventReward
	err := gameplay.WithTransactionRetry(ctx, service.DB, func(tx *gorm.DB) error {
		var err error
		progress, rewards, err = service.addEventProgressTx(tx, accountID, sourceKey, delta, service.Now())
		return err
	})
	return progress, rewards, err
}

func (service *Service) addEventProgressTx(tx *gorm.DB, accountID, sourceKey string, delta int64, now time.Time) (int64, []EventReward, error) {
	if accountID == "" || sourceKey == "" || delta <= 0 {
		return 0, nil, gameplay.ErrInvalidQuantity
	}
	event, err := currentLiveEventTx(tx, now)
	if err != nil {
		return 0, nil, err
	}
	if event == nil {
		return 0, nil, nil
	}
	return service.addEventProgressForEventTx(tx, event.Key, accountID, sourceKey, delta, now)
}

func (service *Service) addEventProgressForEventTx(tx *gorm.DB, eventKey, accountID, sourceKey string, delta int64, now time.Time) (int64, []EventReward, error) {
	if eventKey == "" || accountID == "" || sourceKey == "" || delta <= 0 {
		return 0, nil, gameplay.ErrInvalidQuantity
	}
	var event models.LiveEventConfig
	if err := tx.First(&event, "key = ?", eventKey).Error; err != nil {
		return 0, nil, err
	}
	var err error
	grant := models.EventProgressGrant{ID: uuid.NewString(), EventKey: event.Key, AccountID: accountID, SourceKey: sourceKey, Delta: delta, CreatedAt: now}
	created := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&grant)
	if created.Error != nil {
		return 0, nil, created.Error
	}
	if created.RowsAffected == 0 {
		var existing models.EventProgress
		if err = tx.Limit(1).Find(&existing, "event_key = ? AND account_id = ?", event.Key, accountID).Error; err != nil {
			return 0, nil, err
		}
		return existing.Progress, nil, nil
	}
	progress := models.EventProgress{EventKey: event.Key, AccountID: accountID, Progress: delta, UpdatedAt: now}
	if err = tx.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "event_key"}, {Name: "account_id"}},
		DoUpdates: clause.Assignments(map[string]interface{}{
			"progress":   gorm.Expr("progress + ?", delta),
			"updated_at": now,
		}),
	}).Create(&progress).Error; err != nil {
		return 0, nil, err
	}
	if err = tx.Where("event_key = ? AND account_id = ?", event.Key, accountID).First(&progress).Error; err != nil {
		return 0, nil, err
	}
	rewards, err := service.claimEventRewardsTx(tx, event.Key, accountID, progress.Progress, now)
	return progress.Progress, rewards, err
}

func (service *Service) ClaimEventRewards(ctx context.Context, accountID string) (int64, []EventReward, error) {
	var progress int64
	var rewards []EventReward
	err := gameplay.WithTransactionRetry(ctx, service.DB, func(tx *gorm.DB) error {
		event, err := currentLiveEventTx(tx, service.Now())
		if err != nil {
			return err
		}
		if event == nil {
			return ErrNoActiveEvent
		}
		var state models.EventProgress
		if err = tx.Limit(1).Find(&state, "event_key = ? AND account_id = ?", event.Key, accountID).Error; err != nil {
			return err
		}
		progress = state.Progress
		rewards, err = service.claimEventRewardsTx(tx, event.Key, accountID, progress, service.Now())
		return err
	})
	return progress, rewards, err
}

func (service *Service) claimEventRewardsTx(tx *gorm.DB, eventKey, accountID string, progress int64, now time.Time) ([]EventReward, error) {
	var track []models.RewardTrackConfig
	if err := tx.Where("event_key = ? AND milestone <= ?", eventKey, progress).Order("milestone asc, reward_type asc, reward_key asc").Find(&track).Error; err != nil {
		return nil, err
	}
	rewards := make([]EventReward, 0, len(track))
	for _, configured := range track {
		claim := models.EventRewardClaim{
			ID: uuid.NewString(), EventKey: eventKey, AccountID: accountID,
			Milestone: configured.Milestone, RewardType: configured.RewardType, RewardKey: configured.RewardKey,
			RewardName: configured.RewardName, Quantity: configured.Quantity, ClaimedAt: now,
		}
		created := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&claim)
		if created.Error != nil {
			return nil, created.Error
		}
		if created.RowsAffected == 0 {
			continue
		}
		switch configured.RewardType {
		case "item":
			if err := gameplay.NewInventoryService(tx).CreditTx(tx, accountID, configured.RewardKey, configured.Quantity); err != nil {
				return nil, err
			}
		case "currency":
			wallet := gameplay.NewWalletService(tx)
			wallet.Now = func() time.Time { return now }
			if err := wallet.CreditTxWithReason(tx, accountID, configured.RewardKey, configured.Quantity, "event_milestone", eventKey); err != nil {
				return nil, err
			}
		default:
			return nil, errors.New("活动奖励类型无效")
		}
		rewards = append(rewards, EventReward{Milestone: configured.Milestone, RewardType: configured.RewardType, RewardKey: configured.RewardKey, RewardName: configured.RewardName, Quantity: configured.Quantity})
	}
	return rewards, nil
}
