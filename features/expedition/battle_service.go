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

var ErrInvalidBattleChoice = errors.New("请选择石头、剪刀或布")

type BattleResult struct {
	Record   models.BattleRecord
	Repeated bool
	Attempts int
	Limit    int
}

func (service *Service) PlayRockPaperScissors(ctx context.Context, accountID, sourceKey, choice string) (*BattleResult, error) {
	choice = normalizeBattleChoice(choice)
	if choice == "" {
		return nil, ErrInvalidBattleChoice
	}
	actionKey := chanceActionKey("rps", accountID, sourceKey)
	var existing models.BattleRecord
	lookup := service.DB.WithContext(ctx).Limit(1).Find(&existing, "action_key = ?", actionKey)
	if lookup.Error != nil {
		return nil, lookup.Error
	}
	if lookup.RowsAffected > 0 {
		return &BattleResult{Record: existing, Repeated: true}, nil
	}
	var result BattleResult
	err := gameplay.WithTransactionRetry(ctx, service.DB, func(tx *gorm.DB) error {
		var pet models.PetProfile
		if err := tx.First(&pet, "account_id = ?", accountID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrPetRequired
			}
			return err
		}
		attempts, err := consumeBattleAttemptTx(tx, accountID, service.Now(), 20)
		if err != nil {
			return err
		}
		roll, err := service.RandomIntn(3)
		if err != nil {
			return err
		}
		opponent := []string{"石头", "剪刀", "布"}[roll]
		battleResult := battleOutcome(choice, opponent)
		reward := int64(0)
		switch battleResult {
		case "胜利":
			reward = 5
		case "平局":
			reward = 1
		}
		record := models.BattleRecord{
			ID: uuid.NewString(), AccountID: accountID, ActionKey: actionKey, Mode: "猜拳",
			PlayerChoice: choice, OpponentChoice: opponent, Result: battleResult,
			RewardCurrency: reward, Roll: roll, CreatedAt: service.Now(),
		}
		if err = tx.Create(&record).Error; err != nil {
			return err
		}
		if reward > 0 {
			if err = gameplay.NewWalletService(tx).CreditTxWithReason(tx, accountID, gameplay.DefaultCurrencyKey, reward, "battle_reward", record.ID); err != nil {
				return err
			}
		}
		behavior := "explore"
		if battleResult == "胜利" {
			behavior = "support"
		}
		if err = recordBehaviorTx(tx, accountID, behavior, 1, service.Now()); err != nil {
			return err
		}
		result = BattleResult{Record: record, Attempts: attempts, Limit: 20}
		return nil
	})
	if err != nil && isUniqueConstraintError(err) {
		if lookupErr := service.DB.WithContext(ctx).First(&existing, "action_key = ?", actionKey).Error; lookupErr == nil {
			return &BattleResult{Record: existing, Repeated: true}, nil
		}
	}
	return &result, err
}

func consumeBattleAttemptTx(tx *gorm.DB, accountID string, now time.Time, limit int) (int, error) {
	day := now.Format("2006-01-02")
	state := models.ChanceDailyState{AccountID: accountID, GameKey: "rps", Day: day, UpdatedAt: now}
	if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&state).Error; err != nil {
		return 0, err
	}
	updated := tx.Model(&models.ChanceDailyState{}).
		Where("account_id = ? AND game_key = ? AND day = ? AND attempts < ?", accountID, "rps", day, limit).
		Updates(map[string]interface{}{"attempts": gorm.Expr("attempts + 1"), "updated_at": now})
	if updated.Error != nil {
		return 0, updated.Error
	}
	if updated.RowsAffected != 1 {
		return 0, ErrDailyLimitReached
	}
	if err := tx.Where("account_id = ? AND game_key = ? AND day = ?", accountID, "rps", day).First(&state).Error; err != nil {
		return 0, err
	}
	return state.Attempts, nil
}

func normalizeBattleChoice(choice string) string {
	switch choice {
	case "石头", "拳头":
		return "石头"
	case "剪刀":
		return "剪刀"
	case "布":
		return "布"
	default:
		return ""
	}
}

func battleOutcome(player, opponent string) string {
	if player == opponent {
		return "平局"
	}
	if (player == "石头" && opponent == "剪刀") || (player == "剪刀" && opponent == "布") || (player == "布" && opponent == "石头") {
		return "胜利"
	}
	return "失败"
}
