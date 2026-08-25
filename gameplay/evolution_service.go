package gameplay

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"gorm.io/gorm"
	"qq-pet-saas/models"
)

type EvolutionRequirement struct {
	Label   string
	Current int64
	Needed  int64
	Met     bool
}

type EvolutionPreview struct {
	Stage         string
	PetName       string
	CurrentForm   string
	TargetForm    string
	TargetImage   string
	Requirements  []EvolutionRequirement
	RequiredItems []DailyRewardItem
	Ready         bool
}

type EvolutionService struct {
	DB        *gorm.DB
	Inventory *InventoryService
}

func NewEvolutionService(db *gorm.DB) *EvolutionService {
	return &EvolutionService{DB: db, Inventory: NewInventoryService(db)}
}

func (service *EvolutionService) Preview(ctx context.Context, accountID, stage string) (*EvolutionPreview, error) {
	if service == nil || service.DB == nil {
		return nil, ErrDatabaseUnavailable
	}
	var pet models.PetProfile
	if err := service.DB.WithContext(ctx).First(&pet, "account_id = ?", accountID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrPetRequired
		}
		return nil, err
	}
	var species models.PetSpeciesConfig
	if err := service.DB.WithContext(ctx).First(&species, "name = ?", pet.PetType).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrEvolutionUnavailable
		}
		return nil, err
	}
	return service.previewFor(ctx, pet, species, stage)
}

func (service *EvolutionService) Evolve(ctx context.Context, accountID string) (*EvolutionPreview, error) {
	return service.changeForm(ctx, accountID, "进化")
}

func (service *EvolutionService) Awaken(ctx context.Context, accountID string) (*EvolutionPreview, error) {
	return service.changeForm(ctx, accountID, "觉醒")
}

func (service *EvolutionService) changeForm(ctx context.Context, accountID, stage string) (*EvolutionPreview, error) {
	if service == nil || service.DB == nil {
		return nil, ErrDatabaseUnavailable
	}
	var completed *EvolutionPreview
	err := WithTransactionRetry(ctx, service.DB, func(tx *gorm.DB) error {
		var pet models.PetProfile
		if err := tx.First(&pet, "account_id = ?", accountID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrPetRequired
			}
			return err
		}
		if pet.Status != "" && pet.Status != "空闲" {
			return ErrActivityActive
		}
		var species models.PetSpeciesConfig
		if err := tx.First(&species, "name = ?", pet.PetType).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrEvolutionUnavailable
			}
			return err
		}
		preview, err := service.previewForTx(tx, pet, species, stage)
		if err != nil {
			return err
		}
		if !preview.Ready {
			return ErrEvolutionRequirements
		}
		if stage == "觉醒" {
			for _, item := range preview.RequiredItems {
				if err := service.inventory().DebitTx(tx, accountID, item.Name, item.Quantity); err != nil {
					return err
				}
			}
		}
		pet.CurrentForm = preview.TargetForm
		if err := tx.Save(&pet).Error; err != nil {
			return err
		}
		completed = preview
		return nil
	})
	return completed, err
}

func (service *EvolutionService) previewFor(ctx context.Context, pet models.PetProfile, species models.PetSpeciesConfig, stage string) (*EvolutionPreview, error) {
	return service.previewForTx(service.DB.WithContext(ctx), pet, species, stage)
}

func (service *EvolutionService) previewForTx(tx *gorm.DB, pet models.PetProfile, species models.PetSpeciesConfig, stage string) (*EvolutionPreview, error) {
	current := strings.TrimSpace(pet.CurrentForm)
	if current == "" {
		current = pet.PetType
	}
	preview := &EvolutionPreview{Stage: stage, PetName: pet.Name, CurrentForm: current, Ready: true}
	switch stage {
	case "进化":
		if strings.TrimSpace(species.Evolution) == "" || current != pet.PetType {
			return nil, ErrEvolutionUnavailable
		}
		preview.TargetForm = species.Evolution
		preview.TargetImage = species.EvolutionImage
		preview.Requirements = []EvolutionRequirement{
			{Label: "成长", Current: pet.Growth, Needed: species.EvolutionGrowth, Met: pet.Growth >= species.EvolutionGrowth},
			{Label: "好感", Current: pet.Affection, Needed: species.EvolutionAffect, Met: pet.Affection >= species.EvolutionAffect},
		}
	case "觉醒":
		if strings.TrimSpace(species.Awaken) == "" || current != species.Evolution {
			return nil, ErrEvolutionUnavailable
		}
		preview.TargetForm = species.Awaken
		preview.TargetImage = species.AwakenImage
		preview.Requirements = []EvolutionRequirement{
			{Label: "成长", Current: pet.Growth, Needed: species.AwakenGrowth, Met: pet.Growth >= species.AwakenGrowth},
			{Label: "好感", Current: pet.Affection, Needed: species.AwakenAffect, Met: pet.Affection >= species.AwakenAffect},
		}
		items, err := parseRewardItems(species.AwakenItems)
		if err != nil {
			return nil, fmt.Errorf("觉醒物品配置无效: %w", err)
		}
		preview.RequiredItems = items
		for _, item := range items {
			var inventory models.GlobalInventoryItem
			find := tx.Limit(1).Find(&inventory, "account_id = ? AND item_name = ?", pet.AccountID, item.Name)
			if find.Error != nil {
				return nil, find.Error
			}
			preview.Requirements = append(preview.Requirements, EvolutionRequirement{
				Label: item.Name, Current: inventory.Quantity, Needed: item.Quantity, Met: inventory.Quantity >= item.Quantity,
			})
		}
	default:
		return nil, ErrEvolutionUnavailable
	}
	for _, requirement := range preview.Requirements {
		if !requirement.Met {
			preview.Ready = false
		}
	}
	return preview, nil
}

func ResolvePetImage(pet models.PetProfile, species models.PetSpeciesConfig) string {
	current := strings.TrimSpace(pet.CurrentForm)
	switch {
	case current != "" && current == species.Awaken && species.AwakenImage != "":
		return species.AwakenImage
	case current != "" && current == species.Evolution && species.EvolutionImage != "":
		return species.EvolutionImage
	default:
		return species.Image
	}
}

func (service *EvolutionService) inventory() *InventoryService {
	if service.Inventory == nil {
		service.Inventory = NewInventoryService(service.DB)
	}
	return service.Inventory
}
