package expedition

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"qq-pet-saas/gameplay"
	"qq-pet-saas/models"
)

var (
	ErrEquipmentNotFound  = errors.New("没有找到这件装备")
	ErrEquipmentLocked    = errors.New("装备已锁定")
	ErrEquipmentEquipped  = errors.New("请先卸下装备再分解")
	ErrRecipeLocked       = errors.New("装备蓝图尚未解锁")
	ErrInvalidLootPool    = errors.New("奖励池配置无效")
)

type EquipmentLevelTooLowError struct {
	RequiredLevel int
	CurrentLevel  int
}

func (err *EquipmentLevelTooLowError) Error() string {
	return fmt.Sprintf("需要冒险等级 %d 才能穿戴，当前等级 %d", err.RequiredLevel, err.CurrentLevel)
}

type EquipmentAffixRoll struct {
	Key       string `json:"key"`
	Name      string `json:"name"`
	Attribute string `json:"attribute"`
	Value     int64  `json:"value"`
}

type EquipmentStats struct {
	Attack          int64 `json:"attack"`
	Defense         int64 `json:"defense"`
	Health          int64 `json:"health"`
	Wisdom          int64 `json:"wisdom"`
	CritRate        int64 `json:"crit_rate"`
	DodgeRate       int64 `json:"dodge_rate"`
	DamageBonus     int64 `json:"damage_bonus"`
	DamageReduction int64 `json:"damage_reduction"`
}

type AdventureReward struct {
	Type      string                  `json:"type"`
	Key       string                  `json:"key"`
	Name      string                  `json:"name"`
	Quantity  int64                   `json:"quantity"`
	Equipment *models.PlayerEquipment `json:"equipment,omitempty"`
}

func (service *Service) createEquipmentTx(tx *gorm.DB, accountID, templateKey, source string) (*models.PlayerEquipment, error) {
	var template models.EquipmentTemplateConfig
	if err := tx.First(&template, "key = ? AND enabled = ?", templateKey, true).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrEquipmentNotFound
		}
		return nil, err
	}
	affixes, err := service.rollEquipmentAffixesTx(tx, template)
	if err != nil {
		return nil, err
	}
	rawAffixes, err := json.Marshal(affixes)
	if err != nil {
		return nil, err
	}
	item := models.PlayerEquipment{
		ID: uuid.NewString(), AccountID: accountID, TemplateKey: template.Key, Rarity: template.Rarity,
		AffixesJSON: string(rawAffixes), Source: strings.TrimSpace(source), CreatedAt: service.Now(), UpdatedAt: service.Now(),
	}
	if err = tx.Create(&item).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func equipmentTemplateNameTx(tx *gorm.DB, key string) string {
	var template models.EquipmentTemplateConfig
	if err := tx.Select("name").Limit(1).Find(&template, "key = ?", key).Error; err != nil || template.Name == "" {
		return key
	}
	return template.Name
}

func zoneDisplayNameTx(tx *gorm.DB, key string) string {
	var zone models.AdventureZoneConfig
	if err := tx.Select("name").Limit(1).Find(&zone, "key = ?", key).Error; err != nil || zone.Name == "" {
		return key
	}
	return zone.Name
}

func equipmentSlotLabel(slot string) string {
	switch slot {
	case "weapon":
		return "武器"
	case "armor":
		return "防具"
	case "treasure":
		return "秘宝"
	default:
		return slot
	}
}

func (service *Service) rollEquipmentAffixesTx(tx *gorm.DB, template models.EquipmentTemplateConfig) ([]EquipmentAffixRoll, error) {
	if template.MaxAffixes <= 0 || template.AffixPoolKey == "" {
		return []EquipmentAffixRoll{}, nil
	}
	var candidates []models.EquipmentAffixConfig
	if err := tx.Where("pool_key = ? AND enabled = ?", template.AffixPoolKey, true).Order("key asc").Find(&candidates).Error; err != nil {
		return nil, err
	}
	if len(candidates) < template.MinAffixes {
		return nil, fmt.Errorf("装备 %s 的可用词条不足", template.Key)
	}
	count := template.MinAffixes
	if spread := template.MaxAffixes - template.MinAffixes; spread > 0 {
		roll, err := service.RandomIntn(spread + 1)
		if err != nil {
			return nil, err
		}
		count += roll
	}
	if count > len(candidates) {
		count = len(candidates)
	}
	result := make([]EquipmentAffixRoll, 0, count)
	for len(result) < count {
		total := 0
		for _, row := range candidates {
			if row.Weight > 0 {
				total += row.Weight
			}
		}
		if total <= 0 {
			return nil, fmt.Errorf("装备词条池 %s 没有有效权重", template.AffixPoolKey)
		}
		roll, err := service.RandomIntn(total)
		if err != nil {
			return nil, err
		}
		selected := 0
		for index, row := range candidates {
			roll -= row.Weight
			if roll < 0 {
				selected = index
				break
			}
		}
		row := candidates[selected]
		value := row.MinValue
		if spread := row.MaxValue - row.MinValue; spread > 0 {
			rolled, rollErr := service.RandomIntn(int(spread + 1))
			if rollErr != nil {
				return nil, rollErr
			}
			value += int64(rolled)
		}
		result = append(result, EquipmentAffixRoll{Key: row.Key, Name: row.Name, Attribute: row.Attribute, Value: value})
		candidates = append(candidates[:selected], candidates[selected+1:]...)
	}
	return result, nil
}

func equipmentStats(template models.EquipmentTemplateConfig, affixes []EquipmentAffixRoll) EquipmentStats {
	result := EquipmentStats{Attack: template.BaseAttack, Defense: template.BaseDefense, Health: template.BaseHealth, Wisdom: template.BaseWisdom}
	for _, affix := range affixes {
		switch affix.Attribute {
		case "attack":
			result.Attack += affix.Value
		case "defense":
			result.Defense += affix.Value
		case "health":
			result.Health += affix.Value
		case "wisdom":
			result.Wisdom += affix.Value
		case "crit_rate":
			result.CritRate += affix.Value
		case "dodge_rate":
			result.DodgeRate += affix.Value
		case "damage_bonus":
			result.DamageBonus += affix.Value
		case "damage_reduction":
			result.DamageReduction += affix.Value
		}
	}
	return result
}

func (service *Service) EquippedStatsTx(tx *gorm.DB, accountID string) (EquipmentStats, error) {
	pet, err := gameplay.ActivePetTx(tx, accountID)
	if err != nil {
		return EquipmentStats{}, err
	}
	return service.EquippedStatsForPetTx(tx, accountID, pet.ID)
}

func (service *Service) EquippedStatsForPetTx(tx *gorm.DB, accountID, petID string) (EquipmentStats, error) {
	var rows []models.PlayerEquipment
	if err := tx.Where("account_id = ? AND equipped_pet_id = ? AND equipped_slot <> ''", accountID, petID).Find(&rows).Error; err != nil {
		return EquipmentStats{}, err
	}
	result := EquipmentStats{}
	for _, row := range rows {
		var template models.EquipmentTemplateConfig
		if err := tx.First(&template, "key = ?", row.TemplateKey).Error; err != nil {
			return result, err
		}
		var affixes []EquipmentAffixRoll
		if err := json.Unmarshal([]byte(row.AffixesJSON), &affixes); err != nil {
			return result, fmt.Errorf("装备 %s 的词条数据损坏: %w", row.ID, err)
		}
		stats := equipmentStats(template, affixes)
		result.Attack += stats.Attack
		result.Defense += stats.Defense
		result.Health += stats.Health
		result.Wisdom += stats.Wisdom
		result.CritRate += stats.CritRate
		result.DodgeRate += stats.DodgeRate
		result.DamageBonus += stats.DamageBonus
		result.DamageReduction += stats.DamageReduction
	}
	if result.CritRate > 500 {
		result.CritRate = 500
	}
	if result.DodgeRate > 350 {
		result.DodgeRate = 350
	}
	return result, nil
}

func (service *Service) Equip(ctx context.Context, accountID, equipmentID string) (*models.PlayerEquipment, error) {
	var equipped models.PlayerEquipment
	err := gameplay.WithTransactionRetry(ctx, service.DB, func(tx *gorm.DB) error {
		pet, err := gameplay.ActivePetTx(tx, accountID)
		if err != nil {
			return err
		}
		if err := tx.First(&equipped, "id = ? AND account_id = ?", equipmentID, accountID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrEquipmentNotFound
			}
			return err
		}
		var template models.EquipmentTemplateConfig
		if err := tx.First(&template, "key = ? AND enabled = ?", equipped.TemplateKey, true).Error; err != nil {
			return err
		}
		var progress models.PlayerAdventureProgress
		if err := ensureAdventureProgressTx(tx, accountID, &progress, service.Now()); err != nil {
			return err
		}
		if progress.Level < template.RequiredLevel {
			return &EquipmentLevelTooLowError{RequiredLevel: template.RequiredLevel, CurrentLevel: progress.Level}
		}
		if err := tx.Model(&models.PlayerEquipment{}).Where("account_id = ? AND equipped_pet_id = ? AND equipped_slot = ?", accountID, pet.ID, template.Slot).Updates(map[string]any{"equipped_slot": "", "equipped_pet_id": "", "updated_at": service.Now()}).Error; err != nil {
			return err
		}
		equipped.EquippedSlot = template.Slot
		equipped.EquippedPetID = pet.ID
		equipped.UpdatedAt = service.Now()
		return tx.Save(&equipped).Error
	})
	return &equipped, err
}

func (service *Service) Unequip(ctx context.Context, accountID, equipmentID string) error {
	result := service.DB.WithContext(ctx).Model(&models.PlayerEquipment{}).Where("id = ? AND account_id = ? AND equipped_slot <> ''", equipmentID, accountID).Updates(map[string]any{"equipped_slot": "", "equipped_pet_id": "", "updated_at": service.Now()})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return ErrEquipmentNotFound
	}
	return nil
}

func (service *Service) SalvageEquipment(ctx context.Context, accountID, equipmentID string) error {
	return gameplay.WithTransactionRetry(ctx, service.DB, func(tx *gorm.DB) error {
		var equipment models.PlayerEquipment
		if err := tx.First(&equipment, "id = ? AND account_id = ?", equipmentID, accountID).Error; err != nil {
			return ErrEquipmentNotFound
		}
		if equipment.Locked {
			return ErrEquipmentLocked
		}
		if equipment.EquippedSlot != "" {
			return ErrEquipmentEquipped
		}
		var template models.EquipmentTemplateConfig
		if err := tx.First(&template, "key = ?", equipment.TemplateKey).Error; err != nil {
			return err
		}
		if template.SalvageItem == "" || template.SalvageQuantity <= 0 {
			return errors.New("该装备没有配置分解产物")
		}
		if err := tx.Delete(&equipment).Error; err != nil {
			return err
		}
		return creditAdventureItemTx(tx, accountID, template.SalvageItem, template.SalvageQuantity, service.Now())
	})
}

func (service *Service) CraftEquipment(ctx context.Context, accountID, templateKey string) (*models.PlayerEquipment, error) {
	var crafted *models.PlayerEquipment
	err := gameplay.WithTransactionRetry(ctx, service.DB, func(tx *gorm.DB) error {
		var recipe models.EquipmentRecipeConfig
		if err := tx.First(&recipe, "equipment_key = ? AND enabled = ?", templateKey, true).Error; err != nil {
			return err
		}
		var blueprint models.PlayerBlueprintProgress
		if err := tx.First(&blueprint, "account_id = ? AND equipment_key = ? AND unlocked = ?", accountID, templateKey, true).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrRecipeLocked
			}
			return err
		}
		var materials []models.EquipmentRecipeMaterialConfig
		if err := tx.Where("equipment_key = ?", templateKey).Order("item_name asc").Find(&materials).Error; err != nil {
			return err
		}
		for _, material := range materials {
			if err := debitAdventureItemTx(tx, accountID, material.ItemName, material.Quantity, service.Now()); err != nil {
				return fmt.Errorf("制造材料 %s 不足: %w", material.ItemName, err)
			}
		}
		if recipe.CurrencyCost > 0 {
			if _, err := debitAdventureCurrencyTx(tx, accountID, recipe.CurrencyCost, "equipment_craft", templateKey, service.Now()); err != nil {
				return err
			}
		}
		var err error
		crafted, err = service.createEquipmentTx(tx, accountID, templateKey, "craft:"+templateKey)
		if err != nil {
			return err
		}
		rawMaterials, _ := json.Marshal(materials)
		return tx.Create(&models.EquipmentCraftRecord{ID: uuid.NewString(), AccountID: accountID, EquipmentID: crafted.ID, TemplateKey: templateKey, MaterialsJSON: string(rawMaterials), CreatedAt: service.Now()}).Error
	})
	return crafted, err
}

func (service *Service) grantBlueprintFragmentsTx(tx *gorm.DB, accountID, equipmentKey string, quantity int64) error {
	var recipe models.EquipmentRecipeConfig
	if err := tx.First(&recipe, "equipment_key = ? AND enabled = ?", equipmentKey, true).Error; err != nil {
		return fmt.Errorf("装备蓝图 %s 没有关联配方", equipmentKey)
	}
	var progress models.PlayerBlueprintProgress
	lookup := tx.Limit(1).Find(&progress, "account_id = ? AND equipment_key = ?", accountID, equipmentKey)
	if lookup.Error != nil {
		return lookup.Error
	}
	if lookup.RowsAffected == 0 {
		progress = models.PlayerBlueprintProgress{AccountID: accountID, EquipmentKey: equipmentKey}
	}
	progress.Fragments += quantity
	if progress.Fragments >= recipe.BlueprintFragments && !progress.Unlocked {
		now := service.Now()
		progress.Unlocked = true
		progress.UnlockedAt = &now
	}
	progress.UpdatedAt = service.Now()
	return tx.Save(&progress).Error
}

func (service *Service) grantLootPoolTx(tx *gorm.DB, accountID, poolKey, source string, firstClear bool) ([]AdventureReward, error) {
	return service.grantLootPoolWithRollsTx(tx, accountID, poolKey, source, firstClear, -1)
}

func (service *Service) grantLootPoolWithRollsTx(tx *gorm.DB, accountID, poolKey, source string, firstClear bool, rollsOverride int) ([]AdventureReward, error) {
	pool, entries, err := loadLootPoolSnapshotTx(tx, poolKey)
	if err != nil {
		return nil, err
	}
	return service.grantLootSnapshotTx(tx, accountID, pool, entries, source, firstClear, rollsOverride)
}

func loadLootPoolSnapshotTx(tx *gorm.DB, poolKey string) (models.AdventureLootPoolConfig, []models.AdventureLootEntryConfig, error) {
	if poolKey == "" {
		return models.AdventureLootPoolConfig{}, nil, nil
	}
	var pool models.AdventureLootPoolConfig
	if err := tx.First(&pool, "key = ?", poolKey).Error; err != nil {
		return models.AdventureLootPoolConfig{}, nil, ErrInvalidLootPool
	}
	var entries []models.AdventureLootEntryConfig
	if err := tx.Where("pool_key = ?", poolKey).Order("sort_order asc, id asc").Find(&entries).Error; err != nil {
		return models.AdventureLootPoolConfig{}, nil, err
	}
	return pool, entries, nil
}

func (service *Service) grantExpeditionSnapshotLootTx(tx *gorm.DB, accountID string, pool models.AdventureLootPoolConfig, entries []models.AdventureLootEntryConfig, legacyPoolKey, source string, rollsOverride int) ([]AdventureReward, error) {
	// Snapshots created before v0.0.1 did not contain reward rows. Keep a
	// compatibility path for those already-running records; all new runs settle
	// exclusively from the immutable reward definition captured at departure.
	if pool.Key == "" && legacyPoolKey != "" {
		return service.grantLootPoolWithRollsTx(tx, accountID, legacyPoolKey, source, false, rollsOverride)
	}
	return service.grantLootSnapshotTx(tx, accountID, pool, entries, source, false, rollsOverride)
}

func (service *Service) grantLootSnapshotTx(tx *gorm.DB, accountID string, pool models.AdventureLootPoolConfig, entries []models.AdventureLootEntryConfig, source string, firstClear bool, rollsOverride int) ([]AdventureReward, error) {
	if pool.Key == "" {
		return nil, nil
	}
	selected := make([]models.AdventureLootEntryConfig, 0)
	weighted := make([]models.AdventureLootEntryConfig, 0)
	for _, entry := range entries {
		if entry.FirstClearOnly && !firstClear {
			continue
		}
		if entry.Guaranteed {
			selected = append(selected, entry)
		} else if entry.Weight > 0 {
			weighted = append(weighted, entry)
		}
	}
	rolls := pool.Rolls
	if rollsOverride >= 0 {
		rolls = rollsOverride
	}
	for rollIndex := 0; rollIndex < rolls && len(weighted) > 0; rollIndex++ {
		total := 0
		for _, entry := range weighted {
			total += entry.Weight
		}
		roll, err := service.RandomIntn(total)
		if err != nil {
			return nil, err
		}
		chosen := 0
		for index, entry := range weighted {
			roll -= entry.Weight
			if roll < 0 {
				chosen = index
				break
			}
		}
		selected = append(selected, weighted[chosen])
		if !pool.AllowDuplicates {
			weighted = append(weighted[:chosen], weighted[chosen+1:]...)
		}
	}
	rewards := make([]AdventureReward, 0, len(selected))
	for _, entry := range selected {
		quantity := entry.MinQuantity
		if spread := entry.MaxQuantity - entry.MinQuantity; spread > 0 {
			roll, err := service.RandomIntn(int(spread + 1))
			if err != nil {
				return nil, err
			}
			quantity += int64(roll)
		}
		reward := AdventureReward{Type: entry.RewardType, Key: entry.RewardKey, Name: entry.RewardKey, Quantity: quantity}
		switch entry.RewardType {
		case "item":
			if err := creditAdventureItemTx(tx, accountID, entry.RewardKey, quantity, service.Now()); err != nil {
				return nil, err
			}
			var item models.ItemConfig
			if err := tx.First(&item, "key = ?", entry.RewardKey).Error; err == nil && item.Name != "" {
				reward.Name = item.Name
			}
		case "currency":
			if entry.RewardKey != JourneyBadgeCurrencyKey {
				return nil, ErrInvalidLootPool
			}
			if _, err := creditAdventureCurrencyTx(tx, accountID, quantity, "adventure_reward", source, service.Now()); err != nil {
				return nil, err
			}
			reward.Name = "旅途徽章"
		case "equipment":
			if quantity != 1 {
				return nil, fmt.Errorf("装备奖励 %s 的数量必须为 1", entry.RewardKey)
			}
			equipment, err := service.createEquipmentTx(tx, accountID, entry.RewardKey, source)
			if err != nil {
				return nil, err
			}
			reward.Equipment = equipment
			reward.Name = equipmentTemplateNameTx(tx, entry.RewardKey)
		case "blueprint_fragment":
			if err := service.grantBlueprintFragmentsTx(tx, accountID, entry.RewardKey, quantity); err != nil {
				return nil, err
			}
			reward.Name = equipmentTemplateNameTx(tx, entry.RewardKey) + "蓝图碎片"
		default:
			return nil, ErrInvalidLootPool
		}
		rewards = append(rewards, reward)
	}
	return rewards, nil
}
