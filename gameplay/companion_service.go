package gameplay

import (
	"context"
	"errors"
	"math"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"qq-pet-saas/models"
)

const (
	ActionFeed  = "喂养"
	ActionTouch = "摸头"
	ActionWalk  = "散步"
	ActionGift  = "送礼"
	ActionWash  = "洗澡"
)

type CompanionRules struct {
	Configured bool

	WashGrowth     int64
	WashAffection  int64
	WashHungerCost int64

	TouchGrowth         int64
	TouchAffection      int64
	TouchGrowthLimit    int64
	TouchAffectionLimit int64
	TouchHungerCost     int64
	TouchInterval       time.Duration

	WalkGrowth         int64
	WalkAffection      int64
	WalkGrowthLimit    int64
	WalkAffectionLimit int64
	WalkHungerCost     int64
	WalkInterval       time.Duration

	GiftLimit int64
	Images    map[string]string
}

type CompanionResult struct {
	Action          string
	PetName         string
	ItemName        string
	ItemQuantity    int64
	Image           string
	HungerBefore    int64
	HungerAfter     int64
	AffectionDelta  int64
	GrowthDelta     int64
	MoodPointsAfter int
	FavoriteBonus   bool
	Rescued         bool
}

type CompanionService struct {
	DB        *gorm.DB
	Now       func() time.Time
	Inventory *InventoryService
}

func NewCompanionService(db *gorm.DB) *CompanionService {
	return &CompanionService{DB: db, Now: time.Now, Inventory: NewInventoryService(db)}
}

func (service *CompanionService) Interact(ctx context.Context, accountID, action, itemName string, quantity int64, rules CompanionRules) (*CompanionResult, error) {
	if service == nil || service.DB == nil {
		return nil, ErrDatabaseUnavailable
	}
	if !isCompanionAction(action) {
		return nil, errors.New("未知陪伴操作")
	}
	if quantity <= 0 {
		quantity = 1
	}
	if quantity > 9999 {
		return nil, ErrInvalidQuantity
	}
	rules = normalizedCompanionRules(rules)
	result := &CompanionResult{Action: action}
	err := WithTransactionRetry(ctx, service.DB, func(tx *gorm.DB) error {
		*result = CompanionResult{Action: action}
		activePet, err := ActivePetTx(tx, accountID)
		if err != nil {
			return err
		}
		pet := *activePet
		if pet.Status != "" && pet.Status != "空闲" && !(action == ActionFeed && pet.Status == "濒死") {
			return ErrActionCooldown
		}
		var species models.PetSpeciesConfig
		if lookup := tx.Limit(1).Find(&species, "key = ? OR name = ?", pet.CurrentForm, pet.CurrentForm); lookup.Error != nil {
			return lookup.Error
		}
		now := service.now()
		day := now.Format("2006-01-02")
		daily := models.CompanionActionDaily{AccountID: accountID, PetID: pet.ID, Day: day, Action: action, LastAt: time.Time{}, UpdatedAt: now}
		if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&daily).Error; err != nil {
			return err
		}
		if err := tx.First(&daily, "account_id = ? AND pet_id = ? AND day = ? AND action = ?", accountID, pet.ID, day, action).Error; err != nil {
			return err
		}

		result.PetName = pet.Name
		result.HungerBefore = pet.Hunger
		result.MoodPointsAfter = pet.MoodPoints
		if image := rules.Images[action]; image != "" {
			result.Image = image
		}

		switch action {
		case ActionFeed:
			if err := service.applyFeed(tx, &pet, species, itemName, quantity, result); err != nil {
				return err
			}
		case ActionGift:
			if daily.Count+quantity > rules.GiftLimit {
				return ErrDailyLimit
			}
			if err := service.applyGift(tx, &pet, species, itemName, quantity, result); err != nil {
				return err
			}
		case ActionWash:
			if daily.Count >= 1 {
				return ErrDailyLimit
			}
			if err := applySimpleCompanion(&pet, species, rules.WashHungerCost, rules.WashGrowth, rules.WashAffection, 5, 0, 0, &daily, result); err != nil {
				return err
			}
		case ActionTouch:
			if !daily.LastAt.IsZero() && rules.TouchInterval > 0 && now.Sub(daily.LastAt) < rules.TouchInterval {
				return ErrActionCooldown
			}
			if err := applySimpleCompanion(&pet, species, rules.TouchHungerCost, rules.TouchGrowth, rules.TouchAffection, 4, rules.TouchGrowthLimit, rules.TouchAffectionLimit, &daily, result); err != nil {
				return err
			}
		case ActionWalk:
			if !daily.LastAt.IsZero() && rules.WalkInterval > 0 && now.Sub(daily.LastAt) < rules.WalkInterval {
				return ErrActionCooldown
			}
			if err := applySimpleCompanion(&pet, species, rules.WalkHungerCost, rules.WalkGrowth, rules.WalkAffection, 5, rules.WalkGrowthLimit, rules.WalkAffectionLimit, &daily, result); err != nil {
				return err
			}
		}

		daily.Count += quantity
		daily.GrowthGranted += result.GrowthDelta
		daily.AffectionGiven += result.AffectionDelta
		daily.LastAt = now
		daily.UpdatedAt = now
		if err := tx.Save(&daily).Error; err != nil {
			return err
		}
		if err := tx.Save(&pet).Error; err != nil {
			return err
		}
		if err := recordCareTx(tx, accountID, now); err != nil {
			return err
		}
		result.HungerAfter = pet.Hunger
		result.MoodPointsAfter = pet.MoodPoints
		return nil
	})
	return result, err
}

func (service *CompanionService) applyFeed(tx *gorm.DB, pet *models.PetProfile, species models.PetSpeciesConfig, itemName string, quantity int64, result *CompanionResult) error {
	item, err := getItem(tx, itemName)
	if err != nil {
		return err
	}
	if strings.TrimSpace(item.Type) != "饱食" {
		return ErrWrongItemType
	}
	effect, err := strconv.ParseInt(strings.TrimSpace(item.Effect), 10, 64)
	if err != nil || effect <= 0 {
		return ErrWrongItemType
	}
	maxHunger := pet.HungerMax
	if maxHunger <= 0 {
		maxHunger = positiveOr(species.HungerMax, 100)
	}
	if pet.Hunger >= maxHunger {
		return ErrPetNotHungry
	}
	if err = service.inventory().DebitTx(tx, pet.AccountID, item.Name, quantity); err != nil {
		return err
	}
	addition := effect * quantity
	if item.Name == species.FavoriteFood {
		addition *= 2
		result.FavoriteBonus = true
	}
	pet.Hunger = minInt64(maxHunger, pet.Hunger+addition)
	if pet.Status == "濒死" {
		pet.Status = "空闲"
		pet.Health = maxInt64(10, pet.HealthMax/2)
		result.Rescued = true
	}
	result.ItemName = item.Name
	result.ItemQuantity = quantity
	result.Image = item.Image
	return nil
}

func (service *CompanionService) applyGift(tx *gorm.DB, pet *models.PetProfile, species models.PetSpeciesConfig, itemName string, quantity int64, result *CompanionResult) error {
	item, err := getItem(tx, itemName)
	if err != nil {
		return err
	}
	if strings.TrimSpace(item.Type) != "好感" {
		return ErrWrongItemType
	}
	effect, err := strconv.ParseInt(strings.TrimSpace(item.Effect), 10, 64)
	if err != nil || effect <= 0 {
		return ErrWrongItemType
	}
	if err = service.inventory().DebitTx(tx, pet.AccountID, item.Name, quantity); err != nil {
		return err
	}
	affection := effect * quantity
	if item.Name == species.FavoriteGift {
		affection *= 2
		result.FavoriteBonus = true
	}
	affection = applyPercentBonus(affection, species.AffectionBonus)
	pet.Affection += affection
	updateBondLevel(pet)
	pet.MoodPoints = clampInt(pet.MoodPoints+5, 0, 100)
	pet.Mood = moodFromPoints(pet.MoodPoints)
	result.ItemName = item.Name
	result.ItemQuantity = quantity
	result.AffectionDelta = affection
	result.Image = item.Image
	return nil
}

func applySimpleCompanion(pet *models.PetProfile, species models.PetSpeciesConfig, hungerCost, growth, affection int64, moodDelta int, growthLimit, affectionLimit int64, daily *models.CompanionActionDaily, result *CompanionResult) error {
	if hungerCost > 0 && pet.Hunger < hungerCost {
		return ErrPetTooHungry
	}
	pet.Hunger -= hungerCost
	multiplier := moodMultiplier(pet.Mood)
	growth = int64(math.Round(float64(growth) * multiplier))
	affection = int64(math.Round(float64(affection) * multiplier))
	growth = applyPercentBonus(growth, species.GrowthBonus)
	affection = applyPercentBonus(affection, species.AffectionBonus)
	if growthLimit > 0 {
		growth = minInt64(growth, maxInt64(0, growthLimit-daily.GrowthGranted))
	}
	if affectionLimit > 0 {
		affection = minInt64(affection, maxInt64(0, affectionLimit-daily.AffectionGiven))
	}
	if growth == 0 && affection == 0 && (growthLimit > 0 || affectionLimit > 0) {
		return ErrDailyLimit
	}
	pet.Growth += growth
	pet.Affection += affection
	updateBondLevel(pet)
	pet.MoodPoints = clampInt(pet.MoodPoints+moodDelta, 0, 100)
	pet.Mood = moodFromPoints(pet.MoodPoints)
	result.GrowthDelta = growth
	result.AffectionDelta = affection
	return nil
}

func normalizedCompanionRules(rules CompanionRules) CompanionRules {
	if rules.Configured {
		if rules.GiftLimit <= 0 {
			rules.GiftLimit = 5
		}
		if rules.Images == nil {
			rules.Images = map[string]string{}
		}
		return rules
	}
	return CompanionRules{
		Configured: true,
		WashGrowth: 8, WashAffection: 10, WashHungerCost: 5,
		TouchGrowth: 8, TouchAffection: 10, TouchGrowthLimit: 24, TouchAffectionLimit: 30, TouchHungerCost: 5, TouchInterval: 10 * time.Minute,
		WalkGrowth: 5, WalkAffection: 8, WalkGrowthLimit: 20, WalkAffectionLimit: 24, WalkHungerCost: 15, WalkInterval: 10 * time.Minute,
		GiftLimit: 5, Images: map[string]string{},
	}
}

func isCompanionAction(action string) bool {
	return action == ActionFeed || action == ActionTouch || action == ActionWalk || action == ActionGift || action == ActionWash
}

func moodMultiplier(mood string) float64 {
	multiplier, exists := map[string]float64{"非常开心": 1.5, "开心": 1.25, "一般": 1, "难过": 0.75, "非常难过": 0.5, "期待": 1.1, "愉快": 1.25}[mood]
	if !exists {
		return 1
	}
	return multiplier
}

func moodFromPoints(points int) string {
	switch {
	case points > 80:
		return "非常开心"
	case points > 60:
		return "开心"
	case points > 40:
		return "一般"
	case points > 20:
		return "难过"
	default:
		return "非常难过"
	}
}

func updateBondLevel(pet *models.PetProfile) {
	if pet == nil {
		return
	}
	level := int(pet.Affection/100) + 1
	if level < 1 {
		level = 1
	}
	if level > pet.BondLevel {
		pet.BondLevel = level
	}
}

func applyPercentBonus(value int64, percent int) int64 {
	if value <= 0 || percent == 0 {
		return value
	}
	bonus := int64(math.Round(float64(value) * float64(percent) / 100))
	if bonus <= 0 {
		bonus = 1
	}
	return value + bonus
}

func clampInt(value, minimum, maximum int) int {
	if value < minimum {
		return minimum
	}
	if value > maximum {
		return maximum
	}
	return value
}

func minInt64(left, right int64) int64 {
	if left < right {
		return left
	}
	return right
}

func maxInt64(left, right int64) int64 {
	if left > right {
		return left
	}
	return right
}

func (service *CompanionService) inventory() *InventoryService {
	if service.Inventory == nil {
		service.Inventory = NewInventoryService(service.DB)
	}
	service.Inventory.Now = service.Now
	return service.Inventory
}

func (service *CompanionService) now() time.Time {
	if service.Now != nil {
		return service.Now()
	}
	return time.Now()
}
