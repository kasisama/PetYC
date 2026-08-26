package gameplay

import (
	"context"
	"errors"
	"strings"

	"gorm.io/gorm"
	"qq-pet-saas/models"
)

type PetService struct {
	DB *gorm.DB
}

func NewPetService(db *gorm.DB) *PetService {
	return &PetService{DB: db}
}

func (service *PetService) Get(ctx context.Context, accountID string) (*models.PetProfile, error) {
	if service == nil || service.DB == nil {
		return nil, ErrDatabaseUnavailable
	}
	return ActivePet(ctx, service.DB, accountID)
}

func (service *PetService) List(ctx context.Context, accountID string) ([]models.PetProfile, error) {
	rows := make([]models.PetProfile, 0)
	err := service.DB.WithContext(ctx).Where("account_id = ?", accountID).Order("created_at asc, id asc").Find(&rows).Error
	return rows, err
}

func (service *PetService) SetActive(ctx context.Context, accountID, petID string) (*models.PetProfile, error) {
	var pet *models.PetProfile
	err := WithTransactionRetry(ctx, service.DB, func(tx *gorm.DB) error {
		var err error
		pet, err = PetByIDTx(tx, accountID, petID)
		if err != nil {
			return err
		}
		return tx.Model(&models.PlayerAccount{}).Where("id = ?", accountID).Update("active_pet_id", pet.ID).Error
	})
	return pet, err
}

func (service *PetService) Adopt(ctx context.Context, accountID, petType, name string) (*models.PetProfile, error) {
	return service.AdoptWithStarter(ctx, accountID, petType, name, DefaultCurrencyKey, 0)
}

func (service *PetService) AdoptWithStarter(ctx context.Context, accountID, petType, name, currencyKey string, starterBalance int64) (*models.PetProfile, error) {
	if service == nil || service.DB == nil {
		return nil, ErrDatabaseUnavailable
	}
	petType = strings.TrimSpace(petType)
	name = strings.TrimSpace(name)
	if petType == "" || ValidatePetName(name) != nil {
		return nil, errors.New("宠物名称需要在 2 到 12 个字符之间")
	}
	// Prefer the stable slot-limit result even when a stale client submits a
	// species that is no longer present in the current content profile.
	var existing int64
	if err := service.DB.WithContext(ctx).Model(&models.PetProfile{}).Where("account_id = ?", accountID).Count(&existing).Error; err != nil {
		return nil, err
	}
	if existing >= int64(systemPositiveIntTx(service.DB.WithContext(ctx), MaxPetSlotsConfigKey, 1)) {
		return nil, ErrPetAlreadyExists
	}

	var species models.PetSpeciesConfig
	result := service.DB.WithContext(ctx).Limit(1).Find(&species, "key = ? OR name = ?", petType, petType)
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected == 0 || (species.Key != species.Name && !species.Adoptable) {
		return nil, ErrPetRequired
	}
	pet := initialPet(accountID, species.Key, name, species)
	err := WithTransactionRetry(ctx, service.DB, func(tx *gorm.DB) error {
		var count int64
		if err := tx.Model(&models.PetProfile{}).Where("account_id = ?", accountID).Count(&count).Error; err != nil {
			return err
		}
		if count >= int64(systemPositiveIntTx(tx, MaxPetSlotsConfigKey, 1)) {
			return ErrPetAlreadyExists
		}
		if err := tx.Create(&pet).Error; err != nil {
			return err
		}
		if err := tx.Model(&models.PlayerAccount{}).Where("id = ? AND (active_pet_id = '' OR active_pet_id IS NULL)", accountID).Update("active_pet_id", pet.ID).Error; err != nil {
			return err
		}
		if starterBalance > 0 {
			return NewWalletService(service.DB).CreditTxWithReason(tx, accountID, currencyKey, starterBalance, "adoption_starter", pet.PetType)
		}
		return nil
	})
	if err != nil {
		if isUniqueConstraint(err) {
			return nil, ErrPetAlreadyExists
		}
		return nil, err
	}
	return &pet, nil
}

func (service *PetService) RenameWithCost(ctx context.Context, accountID, name, currencyKey string, cost int64) (*models.PetProfile, error) {
	name = strings.TrimSpace(name)
	if err := ValidatePetName(name); err != nil {
		return nil, err
	}
	if cost < 0 {
		return nil, ErrInvalidQuantity
	}
	var pet models.PetProfile
	err := WithTransactionRetry(ctx, service.DB, func(tx *gorm.DB) error {
		if cost > 0 {
			if err := NewWalletService(service.DB).DebitTxWithReason(tx, accountID, currencyKey, cost, "pet_rename", name); err != nil {
				return err
			}
		}
		active, err := ActivePetTx(tx, accountID)
		if err != nil {
			return err
		}
		result := tx.Model(&models.PetProfile{}).Where("id = ? AND account_id = ?", active.ID, accountID).Update("name", name)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return ErrPetRequired
		}
		return tx.First(&pet, "id = ?", active.ID).Error
	})
	return &pet, err
}

func (service *PetService) Rename(ctx context.Context, accountID, name string) (*models.PetProfile, error) {
	name = strings.TrimSpace(name)
	if err := ValidatePetName(name); err != nil {
		return nil, err
	}
	pet, err := service.Get(ctx, accountID)
	if err != nil {
		return nil, err
	}
	result := service.DB.WithContext(ctx).Model(&models.PetProfile{}).Where("id = ? AND account_id = ?", pet.ID, accountID).Update("name", name)
	if result.Error != nil || result.RowsAffected != 1 {
		if result.Error != nil {
			return nil, result.Error
		}
		return nil, ErrPetRequired
	}
	return service.Get(ctx, accountID)
}

func ValidatePetName(name string) error {
	name = strings.TrimSpace(name)
	length := len([]rune(name))
	if length < 2 || length > 12 {
		return errors.New("名称长度需要在 2 到 12 个字符之间")
	}
	lower := strings.ToLower(name)
	for _, blocked := range []string{"http://", "https://", "www.", "qq", "微信", "vx"} {
		if strings.Contains(lower, blocked) {
			return errors.New("名称不能包含联系方式或链接")
		}
	}
	for _, character := range name {
		if character < 32 || character == 127 {
			return errors.New("名称不能包含控制字符")
		}
	}
	return nil
}

func initialPet(accountID, petType, name string, species models.PetSpeciesConfig) models.PetProfile {
	health := positiveOr(species.Health, 100)
	healthMax := positiveOr(species.HealthMax, health)
	if healthMax < health {
		healthMax = health
	}
	hunger := positiveOr(species.Hunger, 100)
	hungerMax := positiveOr(species.HungerMax, hunger)
	if hungerMax < hunger {
		hungerMax = hunger
	}
	return models.PetProfile{
		AccountID: accountID, PetType: species.FamilyKey, Name: name, CurrentForm: petType,
		Role: "探索者", Stance: "探索", Status: "空闲", Mood: moodFromPoints(70), MoodPoints: 70,
		Readiness: 100, BondLevel: 1, Health: health, HealthMax: healthMax,
		Hunger: hunger, HungerMax: hungerMax, Wisdom: positiveOr(species.Wisdom, 10),
		Strength: positiveOr(species.Strength, 10), Defense: positiveOr(species.Defense, 10),
	}
}

func positiveOr(value, fallback int64) int64 {
	if value > 0 {
		return value
	}
	return fallback
}

func isUniqueConstraint(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "unique constraint") || strings.Contains(message, "constraint failed")
}
