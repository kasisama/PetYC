package gameplay

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"qq-pet-saas/models"
)

const (
	ActivityStudy   = "学习"
	ActivityTrain   = "锻炼"
	ActivityFitness = "健身"
	ActivityWork    = "打工"

	ActivityStatusRunning = "running"
	ActivityStatusClaimed = "claimed"
)

type ActivityStartRequest struct {
	Kind             string
	ConfigKey        string
	ItemName         string
	RequiredItemType string
	RewardAttribute  string
	Duration         time.Duration
	DailyLimit       int64
	HungerCost       int64
	RewardGrowth     int64
	RewardCurrency   int64
	RewardItems      string
	StartImage       string
	EndImage         string
}

type ActivityResult struct {
	Run              models.ActivityRun
	PetName          string
	HungerBefore     int64
	HungerAfter      int64
	Attribute        string
	AttributeBefore  int64
	AttributeAfter   int64
	GrowthDelta      int64
	CurrencyKey      string
	CurrencyDelta    int64
	RemainingBalance int64
	Items            []DailyRewardItem
	Image            string
}

type ActivityNotReadyError struct {
	Remaining time.Duration
}

func (err *ActivityNotReadyError) Error() string {
	return fmt.Sprintf("%s，还需等待 %s", ErrActivityNotReady, err.Remaining.Round(time.Second))
}

func (err *ActivityNotReadyError) Is(target error) bool {
	return target == ErrActivityNotReady
}

type ActivityService struct {
	DB        *gorm.DB
	Now       func() time.Time
	Inventory *InventoryService
	Wallet    *WalletService
}

func NewActivityService(db *gorm.DB) *ActivityService {
	return &ActivityService{DB: db, Now: time.Now, Inventory: NewInventoryService(db), Wallet: NewWalletService(db)}
}

func (service *ActivityService) Start(ctx context.Context, accountID string, request ActivityStartRequest) (*ActivityResult, error) {
	if service == nil || service.DB == nil {
		return nil, ErrDatabaseUnavailable
	}
	request.Kind = strings.TrimSpace(request.Kind)
	if !isTimedActivity(request.Kind) || request.HungerCost < 0 || request.DailyLimit < 0 {
		return nil, ErrInvalidQuantity
	}
	now := service.now()
	result := &ActivityResult{CurrencyKey: DefaultCurrencyKey}
	err := WithTransactionRetry(ctx, service.DB, func(tx *gorm.DB) error {
		*result = ActivityResult{CurrencyKey: DefaultCurrencyKey}
		activePet, err := ActivePetTx(tx, accountID)
		if err != nil {
			return err
		}
		pet := *activePet
		if err := ReservePetRunTx(tx, accountID, pet.ID); err != nil {
			return err
		}
		if request.DailyLimit > 0 {
			dayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
			var used int64
			if err := tx.Model(&models.ActivityRun{}).Where("account_id = ? AND kind = ? AND starts_at >= ?", accountID, request.Kind, dayStart).Count(&used).Error; err != nil {
				return err
			}
			if used >= request.DailyLimit {
				return ErrDailyLimit
			}
		}

		duration := request.Duration
		rewardAmount := int64(0)
		inputItem := ""
		startImage := request.StartImage
		endImage := request.EndImage
		if request.RequiredItemType != "" {
			item, err := getItem(tx, request.ItemName)
			if err != nil {
				return err
			}
			if strings.TrimSpace(item.Type) != request.RequiredItemType {
				return ErrWrongItemType
			}
			rewardAmount, err = strconv.ParseInt(strings.TrimSpace(item.Effect), 10, 64)
			if err != nil || rewardAmount <= 0 {
				return ErrWrongItemType
			}
			if item.Time > 0 {
				duration = time.Duration(item.Time) * time.Minute
			}
			if duration <= 0 {
				duration = time.Minute
			}
			if err := service.inventory().DebitTx(tx, accountID, item.Name, 1); err != nil {
				return err
			}
			inputItem = item.Name
		}
		if duration <= 0 {
			duration = time.Minute
		}
		var species models.PetSpeciesConfig
		if find := tx.Limit(1).Find(&species, "key = ? OR name = ?", pet.CurrentForm, pet.CurrentForm); find.Error != nil {
			return find.Error
		}
		if request.RewardAttribute != "" && attributeAtMax(pet, species, request.RewardAttribute) {
			return ErrAttributeMax
		}
		if request.HungerCost > 0 && pet.Hunger-request.HungerCost <= 10 {
			return ErrPetTooHungry
		}
		result.PetName = pet.Name
		result.HungerBefore = pet.Hunger
		pet.Hunger -= request.HungerCost
		pet.Status = request.Kind
		if err := tx.Save(&pet).Error; err != nil {
			return err
		}
		startImage, endImage = activityImages(species, request.Kind, startImage, endImage)
		run := models.ActivityRun{
			ID: uuid.NewString(), AccountID: accountID, PetID: pet.ID, Kind: request.Kind, ConfigKey: strings.TrimSpace(request.ConfigKey), InputItem: inputItem,
			Status: ActivityStatusRunning, HungerCost: request.HungerCost, RewardAttribute: request.RewardAttribute,
			RewardAmount: rewardAmount, RewardGrowth: request.RewardGrowth, RewardCurrency: request.RewardCurrency,
			RewardItems: request.RewardItems, StartImage: startImage, EndImage: endImage,
			StartsAt: now, EndsAt: now.Add(duration), CreatedAt: now,
		}
		if run.ConfigKey == "" {
			run.ConfigKey = request.Kind
		}
		if err := tx.Create(&run).Error; err != nil {
			if isUniqueConstraint(err) {
				return ErrActivityActive
			}
			return err
		}
		result.Run = run
		result.HungerAfter = pet.Hunger
		result.Image = run.StartImage
		return nil
	})
	return result, err
}

func (service *ActivityService) Complete(ctx context.Context, accountID, kind, currencyKey string) (*ActivityResult, error) {
	if service == nil || service.DB == nil {
		return nil, ErrDatabaseUnavailable
	}
	if !isTimedActivity(kind) {
		return nil, ErrNoActiveActivity
	}
	currencyKey = normalizeCurrencyKey(currencyKey)
	now := service.now()
	result := &ActivityResult{CurrencyKey: currencyKey}
	err := WithTransactionRetry(ctx, service.DB, func(tx *gorm.DB) error {
		*result = ActivityResult{CurrencyKey: currencyKey}
		var run models.ActivityRun
		if err := tx.First(&run, "account_id = ? AND kind = ? AND status = ?", accountID, kind, ActivityStatusRunning).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrNoActiveActivity
			}
			return err
		}
		result.Run = run
		result.Image = run.EndImage
		if now.Before(run.EndsAt) {
			return &ActivityNotReadyError{Remaining: run.EndsAt.Sub(now)}
		}
		petRow, err := PetByIDTx(tx, accountID, run.PetID)
		if err != nil {
			return err
		}
		pet := *petRow
		var species models.PetSpeciesConfig
		if find := tx.Limit(1).Find(&species, "key = ? OR name = ?", pet.CurrentForm, pet.CurrentForm); find.Error != nil {
			return find.Error
		}
		result.PetName = pet.Name
		result.Attribute = run.RewardAttribute
		if run.RewardAttribute != "" {
			before, maximum := attributeValueAndMax(pet, species, run.RewardAttribute)
			gain := int64(math.Round(float64(run.RewardAmount) * moodMultiplier(pet.Mood)))
			if gain <= 0 {
				gain = 1
			}
			gain = applyPercentBonus(gain, species.AttributeBonus)
			after := minInt64(maximum, before+gain)
			setPetAttribute(&pet, run.RewardAttribute, after)
			result.AttributeBefore = before
			result.AttributeAfter = after
		}
		pet.Growth += run.RewardGrowth
		result.GrowthDelta = run.RewardGrowth
		if run.RewardCurrency > 0 {
			currency := int64(math.Round(float64(run.RewardCurrency) * moodMultiplier(pet.Mood)))
			currency = applyPercentBonus(currency, species.CurrencyBonus)
			if err := service.wallet().CreditTxWithReason(tx, accountID, currencyKey, currency, "activity_reward", run.ID); err != nil {
				return err
			}
			result.CurrencyDelta = currency
		}
		items, err := parseRewardItems(run.RewardItems)
		if err != nil {
			return err
		}
		for _, item := range items {
			if err := service.inventory().CreditTx(tx, accountID, item.Name, item.Quantity); err != nil {
				return err
			}
		}
		result.Items = items
		if kind == ActivityWork {
			pet.MoodPoints = clampInt(pet.MoodPoints-10, 0, 100)
		} else {
			pet.MoodPoints = clampInt(pet.MoodPoints+5, 0, 100)
		}
		pet.Mood = moodFromPoints(pet.MoodPoints)
		pet.Status = "空闲"
		if err := tx.Save(&pet).Error; err != nil {
			return err
		}
		claimedAt := now
		update := tx.Model(&models.ActivityRun{}).Where("id = ? AND status = ?", run.ID, ActivityStatusRunning).Updates(map[string]any{"status": ActivityStatusClaimed, "claimed_at": &claimedAt})
		if update.Error != nil {
			return update.Error
		}
		if update.RowsAffected != 1 {
			return ErrNoActiveActivity
		}
		result.Run.Status = ActivityStatusClaimed
		result.Run.ClaimedAt = &claimedAt
		if result.CurrencyDelta > 0 {
			var wallet models.PlayerWallet
			if err := tx.First(&wallet, "account_id = ? AND currency_key = ?", accountID, currencyKey).Error; err != nil {
				return err
			}
			result.RemainingBalance = wallet.Balance
		}
		return nil
	})
	return result, err
}

func (service *ActivityService) Active(ctx context.Context, accountID string) (*models.ActivityRun, error) {
	var run models.ActivityRun
	err := service.DB.WithContext(ctx).First(&run, "account_id = ? AND status = ?", accountID, ActivityStatusRunning).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &run, err
}

func isTimedActivity(kind string) bool {
	return kind == ActivityStudy || kind == ActivityTrain || kind == ActivityFitness || kind == ActivityWork
}

func activityImages(species models.PetSpeciesConfig, kind, startImage, endImage string) (string, string) {
	switch kind {
	case ActivityStudy:
		if species.StudyStartImg != "" {
			startImage = species.StudyStartImg
		}
		if species.StudyEndImg != "" {
			endImage = species.StudyEndImg
		}
	case ActivityTrain:
		if species.TrainStartImg != "" {
			startImage = species.TrainStartImg
		}
		if species.TrainEndImg != "" {
			endImage = species.TrainEndImg
		}
	case ActivityFitness:
		if species.FitnessStartImg != "" {
			startImage = species.FitnessStartImg
		}
		if species.FitnessEndImg != "" {
			endImage = species.FitnessEndImg
		}
	}
	return startImage, endImage
}

func attributeAtMax(pet models.PetProfile, species models.PetSpeciesConfig, attribute string) bool {
	value, maximum := attributeValueAndMax(pet, species, attribute)
	return maximum > 0 && value >= maximum
}

func attributeValueAndMax(pet models.PetProfile, species models.PetSpeciesConfig, attribute string) (int64, int64) {
	switch attribute {
	case "智慧":
		return pet.Wisdom, positiveOr(species.WisdomMax, 100)
	case "力量":
		return pet.Strength, positiveOr(species.StrengthMax, 100)
	case "防御":
		return pet.Defense, positiveOr(species.DefenseMax, 100)
	default:
		return 0, 0
	}
}

func setPetAttribute(pet *models.PetProfile, attribute string, value int64) {
	switch attribute {
	case "智慧":
		pet.Wisdom = value
	case "力量":
		pet.Strength = value
	case "防御":
		pet.Defense = value
	}
}

func (service *ActivityService) inventory() *InventoryService {
	if service.Inventory == nil {
		service.Inventory = NewInventoryService(service.DB)
	}
	service.Inventory.Now = service.Now
	return service.Inventory
}

func (service *ActivityService) wallet() *WalletService {
	if service.Wallet == nil {
		service.Wallet = NewWalletService(service.DB)
	}
	service.Wallet.Now = service.Now
	return service.Wallet
}

func (service *ActivityService) now() time.Time {
	if service.Now != nil {
		return service.Now()
	}
	return time.Now()
}
