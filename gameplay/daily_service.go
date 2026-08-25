package gameplay

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"qq-pet-saas/gameplayrules"
	"qq-pet-saas/models"
)

type DailyRewardItem struct {
	Name     string
	Quantity int64
}

type DailyCheckinResult struct {
	Awarded      bool
	RecentDays   int64
	CheckinCount int64
	CurrencyKey  string
	Currency     int64
	Affection    int64
	Items        []DailyRewardItem
	Image        string
}

type DailyService struct {
	DB        *gorm.DB
	Now       func() time.Time
	Inventory *InventoryService
	Wallet    *WalletService
}

func NewDailyService(db *gorm.DB) *DailyService {
	return &DailyService{
		DB: db, Now: time.Now,
		Inventory: NewInventoryService(db), Wallet: NewWalletService(db),
	}
}

func (service *DailyService) CheckIn(ctx context.Context, accountID string) (*DailyCheckinResult, error) {
	if service == nil || service.DB == nil {
		return nil, ErrDatabaseUnavailable
	}
	result := &DailyCheckinResult{CurrencyKey: DefaultCurrencyKey}
	now := service.now()
	err := WithTransactionRetry(ctx, service.DB, func(tx *gorm.DB) error {
		*result = DailyCheckinResult{CurrencyKey: DefaultCurrencyKey}
		var pet models.PetProfile
		if err := tx.First(&pet, "account_id = ?", accountID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrPetRequired
			}
			return err
		}

		var previousCount int64
		if err := tx.Model(&models.CompanionJournal{}).Where("account_id = ?", accountID).Count(&previousCount).Error; err != nil {
			return err
		}
		entry := models.CompanionJournal{AccountID: accountID, Day: now.Format("2006-01-02"), Action: "签到", CreatedAt: now}
		created := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&entry)
		if created.Error != nil {
			return created.Error
		}
		result.Awarded = created.RowsAffected == 1
		result.CheckinCount = previousCount
		if result.Awarded {
			result.CheckinCount++
			reward, rewardErr := selectDailyReward(tx, result.CheckinCount, now)
			if rewardErr != nil {
				return rewardErr
			}
			result.Currency = reward.Currency
			result.Affection = reward.Affection
			result.Image = reward.Image
			result.Items, rewardErr = parseRewardItems(reward.Items)
			if rewardErr != nil {
				return rewardErr
			}

			pet.Affection += reward.Affection
			pet.Readiness = minInt(pet.Readiness+10, 100)
			pet.MoodPoints = clampInt(pet.MoodPoints+5, 0, 100)
			pet.Mood = moodFromPoints(pet.MoodPoints)
			updateBondLevel(&pet)
			if err := tx.Save(&pet).Error; err != nil {
				return err
			}
			if reward.Currency > 0 {
				if err := service.wallet().CreditTxWithReason(tx, accountID, result.CurrencyKey, reward.Currency, "daily_checkin", entry.Day); err != nil {
					return err
				}
			}
			for _, item := range result.Items {
				if err := service.inventory().CreditTx(tx, accountID, item.Name, item.Quantity); err != nil {
					return err
				}
			}
			if err := recordCareTx(tx, accountID, now); err != nil {
				return err
			}
		}
		return tx.Model(&models.CompanionJournal{}).
			Where("account_id = ? AND day >= ? AND day <= ?", accountID, now.AddDate(0, 0, -6).Format("2006-01-02"), now.Format("2006-01-02")).
			Count(&result.RecentDays).Error
	})
	return result, err
}

func minInt(left, right int) int {
	if left < right {
		return left
	}
	return right
}

func selectDailyReward(tx *gorm.DB, checkinCount int64, now time.Time) (models.CheckinRewardConfig, error) {
	rewardType := "checkin_weekly"
	day := strconv.Itoa(weekdayNumber(now))
	if checkinCount <= 7 {
		rewardType = "checkin_newbie"
		day = strconv.FormatInt(checkinCount, 10)
	}
	var reward models.CheckinRewardConfig
	lookup := tx.Limit(1).Find(&reward, "type = ? AND day = ?", rewardType, day)
	if lookup.Error != nil {
		return reward, lookup.Error
	}
	if lookup.RowsAffected == 0 {
		return models.CheckinRewardConfig{Type: rewardType, Day: day, Affection: 1, Items: "陪伴印记*1"}, nil
	}
	return reward, nil
}

func weekdayNumber(now time.Time) int {
	day := int(now.Weekday())
	if day == 0 {
		return 7
	}
	return day
}

func parseRewardItems(value string) ([]DailyRewardItem, error) {
	result := make([]DailyRewardItem, 0)
	for _, raw := range strings.Split(value, "#") {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		normalized := strings.ReplaceAll(raw, "×", "*")
		parts := strings.SplitN(normalized, "*", 2)
		name := strings.TrimSpace(parts[0])
		quantity := int64(1)
		if len(parts) == 2 {
			parsed, err := strconv.ParseInt(strings.TrimSpace(parts[1]), 10, 64)
			if err != nil || parsed <= 0 {
				return nil, fmt.Errorf("签到奖励物品 %q 的数量无效", raw)
			}
			quantity = parsed
		}
		if name == "" {
			return nil, fmt.Errorf("签到奖励物品 %q 的名称为空", raw)
		}
		result = append(result, DailyRewardItem{Name: name, Quantity: quantity})
	}
	return result, nil
}

func recordCareTx(tx *gorm.DB, accountID string, now time.Time) error {
	profile := models.PetBehaviorProfile{AccountID: accountID, Care: 1, UpdatedAt: now}
	if err := tx.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "account_id"}},
		DoUpdates: clause.Assignments(map[string]interface{}{
			"care": gorm.Expr("care + 1"), "updated_at": now,
		}),
	}).Create(&profile).Error; err != nil {
		return err
	}
	if err := tx.First(&profile, "account_id = ?", accountID).Error; err != nil {
		return err
	}
	trait := gameplayrules.ResolveTrait(tx, profile)
	if trait == "" || trait == profile.Trait {
		return nil
	}
	if err := tx.Model(&profile).Update("trait", trait).Error; err != nil {
		return err
	}
	return tx.Model(&models.PetProfile{}).Where("account_id = ?", accountID).Update("traits", trait).Error
}

func (service *DailyService) inventory() *InventoryService {
	if service.Inventory == nil {
		service.Inventory = NewInventoryService(service.DB)
	}
	service.Inventory.Now = service.Now
	return service.Inventory
}

func (service *DailyService) wallet() *WalletService {
	if service.Wallet == nil {
		service.Wallet = NewWalletService(service.DB)
	}
	service.Wallet.Now = service.Now
	return service.Wallet
}

func (service *DailyService) now() time.Time {
	if service.Now != nil {
		return service.Now()
	}
	return time.Now()
}
