package gameplay

import (
	"context"
	"strings"

	"gorm.io/gorm"
	"qq-pet-saas/models"
)

type EvolutionRequirement struct {
	Label           string
	Current, Needed int64
	Met             bool
}
type EvolutionPreview struct {
	Stage, RuleKey, BranchLabel, PetName, CurrentForm, TargetFormKey, TargetForm, TargetImage string
	Requirements                                                                              []EvolutionRequirement
	RequiredItems                                                                             []DailyRewardItem
	Ready                                                                                     bool
}
type EvolutionService struct {
	DB        *gorm.DB
	Inventory *InventoryService
}

func NewEvolutionService(db *gorm.DB) *EvolutionService {
	return &EvolutionService{DB: db, Inventory: NewInventoryService(db)}
}
func (s *EvolutionService) Preview(ctx context.Context, accountID, stage string) (*EvolutionPreview, error) {
	return s.PreviewTo(ctx, accountID, stage, "")
}
func (s *EvolutionService) PreviewTo(ctx context.Context, accountID, stage, target string) (*EvolutionPreview, error) {
	if s == nil || s.DB == nil {
		return nil, ErrDatabaseUnavailable
	}
	pet, err := ActivePet(ctx, s.DB, accountID)
	if err != nil {
		return nil, err
	}
	return s.previewForTx(s.DB.WithContext(ctx), *pet, stage, target)
}
func (s *EvolutionService) Evolve(ctx context.Context, accountID string) (*EvolutionPreview, error) {
	return s.changeForm(ctx, accountID, "进化", "")
}
func (s *EvolutionService) Awaken(ctx context.Context, accountID string) (*EvolutionPreview, error) {
	return s.changeForm(ctx, accountID, "觉醒", "")
}

// EvolveTo makes the final branch a player choice instead of a random result.
func (s *EvolutionService) EvolveTo(ctx context.Context, accountID, target string) (*EvolutionPreview, error) {
	return s.changeForm(ctx, accountID, "觉醒", target)
}

func (s *EvolutionService) changeForm(ctx context.Context, accountID, stage, target string) (*EvolutionPreview, error) {
	if s == nil || s.DB == nil {
		return nil, ErrDatabaseUnavailable
	}
	var completed *EvolutionPreview
	err := WithTransactionRetry(ctx, s.DB, func(tx *gorm.DB) error {
		pet, err := ActivePetTx(tx, accountID)
		if err != nil {
			return err
		}
		if pet.Status != "" && pet.Status != "空闲" {
			return ErrActivityActive
		}
		preview, err := s.previewForTx(tx, *pet, stage, target)
		if err != nil {
			return err
		}
		if !preview.Ready {
			return ErrEvolutionRequirements
		}
		var costs []models.PetEvolutionCostConfig
		if err = tx.Where("evolution_key = ?", preview.RuleKey).Find(&costs).Error; err != nil {
			return err
		}
		for _, cost := range costs {
			if err = s.inventory().DebitTx(tx, accountID, cost.ItemKey, cost.Quantity); err != nil {
				return err
			}
		}
		pet.CurrentForm = preview.TargetFormKey
		if err = tx.Save(pet).Error; err != nil {
			return err
		}
		completed = preview
		return nil
	})
	return completed, err
}

func (s *EvolutionService) previewForTx(tx *gorm.DB, pet models.PetProfile, stage, target string) (*EvolutionPreview, error) {
	current := strings.TrimSpace(pet.CurrentForm)
	if current == "" {
		current = strings.TrimSpace(pet.PetType)
	}
	var rules []models.PetEvolutionRuleConfig
	if err := tx.Where("from_form_key = ? AND enabled = ?", current, true).Order("sort_order asc, key asc").Find(&rules).Error; err != nil {
		return nil, err
	}
	if len(rules) == 0 {
		return nil, ErrEvolutionUnavailable
	}
	target = strings.TrimSpace(target)
	var rule *models.PetEvolutionRuleConfig
	var form models.PetSpeciesConfig
	for i := range rules {
		candidate := models.PetSpeciesConfig{}
		if err := tx.First(&candidate, "key = ?", rules[i].ToFormKey).Error; err != nil {
			return nil, err
		}
		if (stage == "觉醒") != (candidate.Stage == "awakened") {
			continue
		}
		if target == "" || target == rules[i].Key || target == candidate.Key || target == rules[i].BranchLabel || target == candidate.Name {
			rule, form = &rules[i], candidate
			break
		}
	}
	if rule == nil {
		return nil, ErrEvolutionUnavailable
	}
	preview := &EvolutionPreview{Stage: stage, RuleKey: rule.Key, BranchLabel: rule.BranchLabel, PetName: pet.Name, CurrentForm: current, TargetFormKey: form.Key, TargetForm: form.Name, TargetImage: form.Image, Ready: true}
	preview.Requirements = []EvolutionRequirement{{Label: "成长", Current: pet.Growth, Needed: rule.RequiredGrowth, Met: pet.Growth >= rule.RequiredGrowth}, {Label: "好感", Current: pet.Affection, Needed: rule.RequiredAffection, Met: pet.Affection >= rule.RequiredAffection}}
	var costs []models.PetEvolutionCostConfig
	if err := tx.Where("evolution_key = ?", rule.Key).Order("item_key asc").Find(&costs).Error; err != nil {
		return nil, err
	}
	for _, cost := range costs {
		var definition models.ItemConfig
		if err := tx.First(&definition, "key = ?", cost.ItemKey).Error; err != nil {
			return nil, err
		}
		var inventory models.GlobalInventoryItem
		lookup := tx.Limit(1).Find(&inventory, "account_id = ? AND item_key = ?", pet.AccountID, cost.ItemKey)
		if lookup.Error != nil {
			return nil, lookup.Error
		}
		preview.RequiredItems = append(preview.RequiredItems, DailyRewardItem{Name: definition.Name, Quantity: cost.Quantity})
		preview.Requirements = append(preview.Requirements, EvolutionRequirement{Label: definition.Name, Current: inventory.Quantity, Needed: cost.Quantity, Met: inventory.Quantity >= cost.Quantity})
	}
	for _, requirement := range preview.Requirements {
		if !requirement.Met {
			preview.Ready = false
		}
	}
	return preview, nil
}

func ResolvePetImage(_ models.PetProfile, species models.PetSpeciesConfig) string {
	return species.Image
}
func (s *EvolutionService) inventory() *InventoryService {
	if s.Inventory == nil {
		s.Inventory = NewInventoryService(s.DB)
	}
	return s.Inventory
}
