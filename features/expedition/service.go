package expedition

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"math/big"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"qq-pet-saas/core"
	"qq-pet-saas/gameplay"
	"qq-pet-saas/gameplayrules"
	"qq-pet-saas/models"
)

var (
	ErrPetRequired           = gameplay.ErrPetRequired
	ErrExpeditionActive      = errors.New("已有进行中的远征")
	ErrExpeditionNotReady    = errors.New("远征尚未结束")
	ErrExpeditionUnavailable = errors.New("远征档位未开放")
	ErrInsufficientReadiness = errors.New("宠物准备度不足")
	ErrNothingToClaim        = errors.New("没有可领取的远征奖励")
	ErrInsufficientItem      = gameplay.ErrInsufficientItem
	ErrInvalidBindToken      = errors.New("绑定码无效或已过期")
	ErrBindConflict          = errors.New("目标身份已有独立游戏进度，请先使用“删除我的数据”清理目标存档")
)

type ExpeditionResult struct {
	Name          string
	Item          string
	Quantity      int64
	Records       int64
	Growth        int64
	Currency      int64
	CodexEntry    string
	Progress      int
	BonusText     string
	Image         string
	EventProgress int64
	EventRewards  []EventReward
}

type SeasonInfluence struct {
	Choice      int
	ChoiceKey   string
	Votes       []int64
	EffectType  string
	EffectValue int
	Description string
}

type Service struct {
	DB          *gorm.DB
	Now         func() time.Time
	TokenSource func() (string, error)
	RandomIntn  func(int) (int, error)
}

type SeasonInfo struct {
	Key        string
	Name       string
	Region     string
	StartsAt   time.Time
	EndsAt     time.Time
	Choices    []string
	ChoiceKeys []string
}

func NewService(db *gorm.DB) *Service {
	return &Service{DB: db, Now: time.Now, TokenSource: randomBindToken, RandomIntn: cryptoRandomIntn}
}

func cryptoRandomIntn(limit int) (int, error) {
	if limit <= 0 {
		return 0, errors.New("随机范围无效")
	}
	value, err := rand.Int(rand.Reader, big.NewInt(int64(limit)))
	if err != nil {
		return 0, err
	}
	return int(value.Int64()), nil
}

func identityScope(event core.InboundEvent) string {
	return gameplay.IdentityScope(event)
}

func (service *Service) ResolveAccount(ctx context.Context, event core.InboundEvent) (*models.PlayerAccount, error) {
	return gameplay.NewAccountService(service.DB).Resolve(ctx, event)
}

func (service *Service) Adopt(ctx context.Context, accountID, petType, name string) (*models.PetProfile, error) {
	return gameplay.NewPetService(service.DB).Adopt(ctx, accountID, petType, name)
}

func (service *Service) AdoptWithStarter(ctx context.Context, accountID, petType, name, currencyKey string, starterBalance int64) (*models.PetProfile, error) {
	return gameplay.NewPetService(service.DB).AdoptWithStarter(ctx, accountID, petType, name, currencyKey, starterBalance)
}

func (service *Service) RenamePet(ctx context.Context, accountID, name, currencyKey string, cost int64) (*models.PetProfile, error) {
	return gameplay.NewPetService(service.DB).RenameWithCost(ctx, accountID, name, currencyKey, cost)
}

func (service *Service) StartExpedition(ctx context.Context, accountID string, tierNumber int) (*models.ExpeditionRun, error) {
	if tierNumber <= 0 {
		return nil, fmt.Errorf("远征档位无效")
	}
	var run models.ExpeditionRun
	err := gameplay.WithTransactionRetry(ctx, service.DB, func(tx *gorm.DB) error {
		template, err := service.expeditionTemplateTx(tx, tierNumber)
		if err != nil {
			return err
		}
		petRow, petErr := gameplay.ActivePetTx(tx, accountID)
		if petErr != nil {
			return petErr
		}
		pet := *petRow
		if err = gameplay.ReservePetRunTx(tx, accountID, pet.ID); err != nil {
			if errors.Is(err, gameplay.ErrTooManyConcurrentRuns) || errors.Is(err, gameplay.ErrActivityActive) {
				return ErrExpeditionActive
			}
			return err
		}
		hungerCost := template.HungerCost
		readinessCost := template.ReadinessCost
		if pet.Stance == "守护" {
			hungerCost = int64(math.Ceil(float64(hungerCost) * 0.75))
			readinessCost = int(math.Ceil(float64(readinessCost) * 0.75))
		}
		if hungerCost > 0 && pet.Hunger-hungerCost <= 10 {
			return gameplay.ErrPetTooHungry
		}
		if readinessCost > 0 && pet.Readiness < readinessCost {
			return ErrInsufficientReadiness
		}
		if template.RequiredItem != "" && template.RequiredQuantity > 0 {
			if err = gameplay.NewInventoryService(service.DB).DebitTx(tx, accountID, template.RequiredItem, template.RequiredQuantity); err != nil {
				return err
			}
		}
		averageAttribute := (pet.Wisdom + pet.Strength + pet.Defense) / 3
		bonusPercent := int(averageAttribute / 4)
		if bonusPercent > 25 {
			bonusPercent = 25
		}
		records := expeditionRewardWithBonus(template.RewardRecords, bonusPercent)
		growth := expeditionRewardWithBonus(template.RewardGrowth, bonusPercent)
		codexProgress := template.CodexProgress
		bonusParts := []string{fmt.Sprintf("属性效率 +%d%%", bonusPercent)}
		switch pet.Stance {
		case "探索":
			codexProgress += 5
			bonusParts = append(bonusParts, "探索姿态：图鉴进度 +5")
		case "守护":
			bonusParts = append(bonusParts, "守护姿态：行动消耗 -25%")
		case "支援":
			records = expeditionRewardWithBonus(records, 20)
			bonusParts = append(bonusParts, "支援姿态：调查记录 +20%")
		case "攻击":
			growth = expeditionRewardWithBonus(growth, 20)
			bonusParts = append(bonusParts, "攻击姿态：成长 +20%")
		}
		now := service.Now()
		run = models.ExpeditionRun{
			ID: uuid.NewString(), AccountID: accountID, PetID: pet.ID, Tier: tierNumber, Name: template.Name, Stance: pet.Stance,
			Status: "running", RewardItem: template.RewardItem, RewardQuantity: template.RewardQuantity,
			RewardRecords: records, RewardGrowth: growth, RewardCurrency: template.RewardCurrency,
			CodexCategory: template.CodexCategory, CodexEntry: template.CodexEntry, CodexProgress: codexProgress,
			HungerCost: hungerCost, ReadinessCost: readinessCost, RequiredItem: template.RequiredItem, RequiredQty: template.RequiredQuantity,
			BonusPercent: bonusPercent, BonusText: strings.Join(bonusParts, "；"), StartImage: template.StartImage, EndImage: template.EndImage,
			StartedAt: now, EndsAt: now.Add(time.Duration(template.DurationMinutes) * time.Minute),
		}
		pet.Hunger -= hungerCost
		pet.Readiness -= readinessCost
		pet.Status = "远征"
		if err = tx.Save(&pet).Error; err != nil {
			return err
		}
		if err = tx.Create(&run).Error; err != nil {
			if isUniqueConstraintError(err) {
				return ErrExpeditionActive
			}
			return err
		}
		return nil
	})
	return &run, err
}

func expeditionRewardWithBonus(base int64, percent int) int64 {
	if base <= 0 || percent <= 0 {
		return base
	}
	return base + int64(math.Ceil(float64(base)*float64(percent)/100))
}

func (service *Service) ListExpeditionTemplates(ctx context.Context) ([]models.ExpeditionTemplateConfig, error) {
	var templates []models.ExpeditionTemplateConfig
	if err := service.DB.WithContext(ctx).Where("enabled = ?", true).Order("tier asc").Find(&templates).Error; err != nil {
		return nil, err
	}
	return templates, nil
}

func (service *Service) expeditionTemplateTx(tx *gorm.DB, tier int) (models.ExpeditionTemplateConfig, error) {
	var template models.ExpeditionTemplateConfig
	find := tx.Limit(1).Find(&template, "tier = ?", tier)
	if find.Error != nil {
		return template, find.Error
	}
	if find.RowsAffected > 0 {
		if !template.Enabled {
			return template, fmt.Errorf("%w: %d", ErrExpeditionUnavailable, tier)
		}
		if template.DurationMinutes <= 0 || template.RewardItem == "" || template.RewardQuantity <= 0 {
			return template, fmt.Errorf("远征模板 %d 配置不完整", tier)
		}
		return template, nil
	}
	return template, fmt.Errorf("%w: %d", ErrExpeditionUnavailable, tier)
}

func isUniqueConstraintError(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "unique constraint") || strings.Contains(message, "constraint failed")
}

func (service *Service) ActiveExpedition(ctx context.Context, accountID string) (*models.ExpeditionRun, error) {
	var run models.ExpeditionRun
	err := service.DB.WithContext(ctx).Where("account_id = ? AND status = ?", accountID, "running").Order("started_at DESC").First(&run).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNothingToClaim
	}
	return &run, err
}

func (service *Service) ClaimExpedition(ctx context.Context, accountID string) (*ExpeditionResult, error) {
	var result ExpeditionResult
	err := gameplay.WithTransactionRetry(ctx, service.DB, func(tx *gorm.DB) error {
		var run models.ExpeditionRun
		if err := tx.Where("account_id = ? AND status = ?", accountID, "running").Order("started_at DESC").First(&run).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrNothingToClaim
			}
			return err
		}
		now := service.Now()
		if now.Before(run.EndsAt) {
			return ErrExpeditionNotReady
		}
		update := tx.Model(&models.ExpeditionRun{}).Where("id = ? AND status = ?", run.ID, "running").Updates(map[string]interface{}{"status": "claimed", "claimed_at": now})
		if update.Error != nil {
			return update.Error
		}
		if update.RowsAffected != 1 {
			return ErrNothingToClaim
		}
		if run.RewardQuantity > 0 {
			if err := addInventoryTx(tx, accountID, run.RewardItem, run.RewardQuantity); err != nil {
				return err
			}
		}
		if run.RewardRecords > 0 {
			if err := addInventoryTx(tx, accountID, "调查记录", run.RewardRecords); err != nil {
				return err
			}
		}
		petRow, err := gameplay.PetByIDTx(tx, accountID, run.PetID)
		if err != nil {
			return err
		}
		pet := *petRow
		pet.Growth += run.RewardGrowth
		pet.Status = "空闲"
		if err := tx.Save(&pet).Error; err != nil {
			return err
		}
		if run.RewardCurrency > 0 {
			if err := gameplay.NewWalletService(service.DB).CreditTxWithReason(tx, accountID, gameplay.DefaultCurrencyKey, run.RewardCurrency, "expedition_reward", run.ID); err != nil {
				return err
			}
		}
		if err := recordBehaviorTx(tx, accountID, "explore", int64(run.Tier), now); err != nil {
			return err
		}
		entry := models.CodexEntry{}
		if run.CodexCategory != "" && run.CodexEntry != "" && run.CodexProgress > 0 {
			var catalog models.CodexCatalogConfig
			catalogLookup := tx.Limit(1).Find(&catalog, "category = ? AND entry_key = ? AND enabled = ?", run.CodexCategory, run.CodexEntry, true)
			if catalogLookup.Error != nil {
				return catalogLookup.Error
			}
			if catalogLookup.RowsAffected > 0 {
				entry = models.CodexEntry{AccountID: accountID, Category: catalog.Category, EntryKey: catalog.EntryKey, Progress: run.CodexProgress}
				if err := tx.Clauses(clause.OnConflict{
					Columns:   []clause.Column{{Name: "account_id"}, {Name: "category"}, {Name: "entry_key"}},
					DoUpdates: clause.Assignments(map[string]interface{}{"progress": gorm.Expr("MIN(100, progress + ?)", run.CodexProgress), "updated_at": now}),
				}).Create(&entry).Error; err != nil {
					return err
				}
				if err := tx.Where("account_id = ? AND category = ? AND entry_key = ?", accountID, catalog.Category, catalog.EntryKey).First(&entry).Error; err != nil {
					return err
				}
			}
		}
		eventDelta := run.RewardRecords
		if eventDelta <= 0 {
			eventDelta = 1
		}
		eventProgress, eventRewards, err := service.addEventProgressTx(tx, accountID, "expedition:"+run.ID, eventDelta, now)
		if err != nil {
			return err
		}
		result = ExpeditionResult{Name: run.Name, Item: run.RewardItem, Quantity: run.RewardQuantity, Records: run.RewardRecords, Growth: run.RewardGrowth, Currency: run.RewardCurrency, CodexEntry: entry.EntryKey, Progress: entry.Progress, BonusText: run.BonusText, Image: run.EndImage, EventProgress: eventProgress, EventRewards: eventRewards}
		return nil
	})
	return &result, err
}

func addInventoryTx(tx *gorm.DB, accountID, itemName string, quantity int64) error {
	return gameplay.NewInventoryService(tx).CreditTx(tx, accountID, itemName, quantity)
}

func (service *Service) AddInventory(ctx context.Context, accountID, itemName string, quantity int64) error {
	return gameplay.NewInventoryService(service.DB).Credit(ctx, accountID, itemName, quantity)
}

func communityID(event core.InboundEvent) string {
	return strings.Join([]string{string(event.Platform), event.AppID, string(event.SceneType), event.SpaceID}, ":")
}

func (service *Service) GetCommunity(ctx context.Context, event core.InboundEvent, accountID string) (*models.Community, error) {
	community := models.Community{ID: communityID(event), Platform: string(event.Platform), AppID: event.AppID, SceneType: string(event.SceneType), SpaceID: event.SpaceID, Level: 1}
	err := service.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&community).Error; err != nil {
			return err
		}
		if err := tx.First(&community, "id = ?", community.ID).Error; err != nil {
			return err
		}
		member := models.CommunityMember{CommunityID: community.ID, AccountID: accountID, JoinedAt: service.Now()}
		return tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&member).Error
	})
	return &community, err
}

func (service *Service) Contribute(ctx context.Context, event core.InboundEvent, accountID, itemName string, quantity int64) (*models.Community, error) {
	if quantity <= 0 {
		return nil, errors.New("贡献数量必须大于零")
	}
	community, err := service.GetCommunity(ctx, event, accountID)
	if err != nil {
		return nil, err
	}
	season := service.CurrentSeason()
	err = service.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var catalog models.ItemConfig
		catalogLookup := tx.Limit(1).Find(&catalog, "name = ? AND status IN ?", itemName, []string{"active", "limited"})
		if catalogLookup.Error != nil {
			return catalogLookup.Error
		}
		if catalogLookup.RowsAffected == 0 {
			return errors.New("该物品不能用于社区共建")
		}
		if err := gameplay.NewInventoryService(tx).DebitTx(tx, accountID, itemName, quantity); err != nil {
			return err
		}
		materialGain := quantity
		influence, influenceErr := seasonInfluenceTx(tx, season.Key, community.ID)
		if influenceErr != nil {
			return influenceErr
		}
		if influence.EffectType == "community_material_gain_percent" {
			materialGain = expeditionRewardWithBonus(materialGain, influence.EffectValue)
		}
		if err := tx.Model(&models.Community{}).Where("id = ?", community.ID).Updates(map[string]interface{}{"materials": gorm.Expr("materials + ?", materialGain), "level": gorm.Expr("1 + (materials + ?) / 100", materialGain)}).Error; err != nil {
			return err
		}
		return tx.Model(&models.CommunityMember{}).Where("community_id = ? AND account_id = ?", community.ID, accountID).UpdateColumn("contribution", gorm.Expr("contribution + ?", quantity)).Error
	})
	if err != nil {
		return nil, err
	}
	return service.GetCommunity(ctx, event, accountID)
}

func randomBindToken() (string, error) {
	const alphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"
	buffer := make([]byte, 8)
	random := make([]byte, len(buffer))
	if _, err := rand.Read(random); err != nil {
		return "", err
	}
	for index := range buffer {
		buffer[index] = alphabet[int(random[index])%len(alphabet)]
	}
	return string(buffer), nil
}

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(strings.ToUpper(strings.TrimSpace(token))))
	return hex.EncodeToString(sum[:])
}

func (service *Service) GenerateBindToken(ctx context.Context, accountID string) (string, error) {
	token, err := service.TokenSource()
	if err != nil {
		return "", err
	}
	record := models.IdentityBindToken{TokenHash: hashToken(token), AccountID: accountID, ExpiresAt: service.Now().Add(10 * time.Minute)}
	if err := service.DB.WithContext(ctx).Create(&record).Error; err != nil {
		return "", err
	}
	return token, nil
}

func (service *Service) RedeemBindToken(ctx context.Context, event core.InboundEvent, token string) (*models.PlayerAccount, error) {
	var account models.PlayerAccount
	err := service.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var record models.IdentityBindToken
		if err := tx.Where("token_hash = ? AND used_at IS NULL AND expires_at > ?", hashToken(token), service.Now()).First(&record).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrInvalidBindToken
			}
			return err
		}
		identity := models.PlayerIdentity{
			AccountID: record.AccountID, Platform: string(event.Platform), AppID: event.AppID,
			SceneType: string(event.SceneType), ScopeID: identityScope(event), SubjectID: event.ActorID,
		}
		var existing models.PlayerIdentity
		query := tx.Where("platform = ? AND app_id = ? AND scene_type = ? AND scope_id = ? AND subject_id = ?", identity.Platform, identity.AppID, identity.SceneType, identity.ScopeID, identity.SubjectID)
		if err := query.First(&existing).Error; err == nil {
			previousAccountID := existing.AccountID
			if previousAccountID != record.AccountID {
				hasProgress, progressErr := accountHasIndependentProgress(tx, previousAccountID)
				if progressErr != nil {
					return progressErr
				}
				if hasProgress {
					return ErrBindConflict
				}
			}
			if err := tx.Model(&existing).Update("account_id", record.AccountID).Error; err != nil {
				return err
			}
			if previousAccountID != record.AccountID {
				if err := tx.Delete(&models.PlayerAccount{}, "id = ?", previousAccountID).Error; err != nil {
					return err
				}
			}
		} else if errors.Is(err, gorm.ErrRecordNotFound) {
			if err := tx.Create(&identity).Error; err != nil {
				return err
			}
		} else {
			return err
		}
		now := service.Now()
		if result := tx.Model(&models.IdentityBindToken{}).Where("token_hash = ? AND used_at IS NULL", record.TokenHash).Update("used_at", now); result.Error != nil || result.RowsAffected != 1 {
			if result.Error != nil {
				return result.Error
			}
			return ErrInvalidBindToken
		}
		return tx.First(&account, "id = ?", record.AccountID).Error
	})
	return &account, err
}

func accountHasIndependentProgress(tx *gorm.DB, accountID string) (bool, error) {
	modelsWithProgress := []interface{}{
		&models.PetProfile{}, &models.GlobalInventoryItem{}, &models.PlayerWallet{}, &models.WalletLedger{}, &models.CompanionJournal{}, &models.CompanionActionDaily{},
		&models.ActivityRun{}, &models.ItemUseRecord{}, &models.ExpeditionRun{}, &models.CodexEntry{}, &models.CommunityMember{},
		&models.SquadMember{}, &models.PetBehaviorProfile{}, &models.SeasonVote{}, &models.EventProgress{}, &models.EventProgressGrant{}, &models.EventRewardClaim{},
		&models.ChanceDailyState{}, &models.ChancePlayerState{}, &models.ChanceOutcome{}, &models.FishingRun{}, &models.BattleRecord{}, &models.TradeAudit{},
		&models.AdventureShopPurchase{},
		&models.PlayerAdventureProgress{}, &models.PlayerZoneProgress{}, &models.PlayerObjectiveProgress{}, &models.AdventureExplorationSession{},
		&models.AdventureCombatSession{}, &models.PlayerEquipment{}, &models.PlayerBlueprintProgress{}, &models.AdventureExpeditionRun{},
	}
	for _, model := range modelsWithProgress {
		var count int64
		if err := tx.Model(model).Where("account_id = ?", accountID).Count(&count).Error; err != nil {
			return false, err
		}
		if count > 0 {
			return true, nil
		}
	}
	var identities int64
	if err := tx.Model(&models.PlayerIdentity{}).Where("account_id = ?", accountID).Count(&identities).Error; err != nil {
		return false, err
	}
	return identities > 1, nil
}

func (service *Service) RecordDaily(ctx context.Context, accountID, action string) (int64, bool, error) {
	result, err := service.CheckIn(ctx, accountID)
	if err != nil {
		return 0, false, err
	}
	return result.RecentDays, result.Awarded, nil
}

func (service *Service) CheckIn(ctx context.Context, accountID string) (*gameplay.DailyCheckinResult, error) {
	daily := gameplay.NewDailyService(service.DB)
	daily.Now = service.Now
	return daily.CheckIn(ctx, accountID)
}

func ValidatePlayerName(name string) error {
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

func (service *Service) CreateSquad(ctx context.Context, event core.InboundEvent, accountID, name string) (*models.ExpeditionSquad, error) {
	if err := ValidatePlayerName(name); err != nil {
		return nil, err
	}
	community, err := service.GetCommunity(ctx, event, accountID)
	if err != nil {
		return nil, err
	}
	squad := models.ExpeditionSquad{ID: uuid.NewString(), CommunityID: community.ID, Name: strings.TrimSpace(name), LeaderID: accountID, MaxMembers: 12, CreatedAt: service.Now()}
	err = service.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var count int64
		if err := tx.Model(&models.SquadMember{}).Where("community_id = ? AND account_id = ?", community.ID, accountID).Count(&count).Error; err != nil {
			return err
		}
		if count > 0 {
			return errors.New("你已经加入了当前社区的远征小队")
		}
		if err := tx.Create(&squad).Error; err != nil {
			return err
		}
		member := models.SquadMember{SquadID: squad.ID, CommunityID: community.ID, AccountID: accountID, JoinedAt: service.Now()}
		return tx.Create(&member).Error
	})
	return &squad, err
}

func (service *Service) FormatExpeditionStatus(ctx context.Context, accountID string) (string, error) {
	run, err := service.ActiveExpedition(ctx, accountID)
	if err != nil {
		return "", err
	}
	remaining := run.EndsAt.Sub(service.Now())
	if remaining <= 0 {
		return fmt.Sprintf("【%s已完成】\n发送“领取”结算奖励。", run.Name), nil
	}
	minutes := int(math.Ceil(remaining.Minutes()))
	if minutes >= 60 {
		return fmt.Sprintf("【%s进行中】\n姿态：%s\n剩余约：%d小时%d分钟\n结束后发送“领取”。", run.Name, run.Stance, minutes/60, minutes%60), nil
	}
	return fmt.Sprintf("【%s进行中】\n姿态：%s\n剩余约：%d分钟\n结束后发送“领取”。", run.Name, run.Stance, minutes), nil
}

func (service *Service) SetStance(ctx context.Context, accountID, stance string) error {
	allowed := make([]string, 0)
	for _, configured := range gameplayrules.EnabledStances(service.DB.WithContext(ctx)) {
		allowed = append(allowed, configured.Name)
		if configured.Name == stance {
			pet, err := gameplay.ActivePet(ctx, service.DB, accountID)
			if err != nil {
				return err
			}
			result := service.DB.WithContext(ctx).Model(&models.PetProfile{}).Where("id = ? AND account_id = ?", pet.ID, accountID).Update("stance", stance)
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected == 0 {
				return ErrPetRequired
			}
			return nil
		}
	}
	return fmt.Errorf("姿态只能是%s", humanList(allowed))
}

func (service *Service) DeleteAccount(ctx context.Context, accountID string) error {
	return service.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var squads []models.ExpeditionSquad
		if err := tx.Where("leader_id = ?", accountID).Find(&squads).Error; err != nil {
			return err
		}
		for _, squad := range squads {
			if err := tx.Where("squad_id = ?", squad.ID).Delete(&models.SquadMember{}).Error; err != nil {
				return err
			}
		}
		var requestCodes []string
		if err := tx.Model(&models.CommunityHelpRequest{}).Where("requester_id = ?", accountID).Pluck("code", &requestCodes).Error; err != nil {
			return err
		}
		if len(requestCodes) > 0 {
			if err := tx.Where("request_code IN ?", requestCodes).Delete(&models.HelpGiftLog{}).Error; err != nil {
				return err
			}
		}
		if err := tx.Where("donor_id = ?", accountID).Delete(&models.HelpGiftLog{}).Error; err != nil {
			return err
		}
		if err := tx.Where("donor_id = ?", accountID).Delete(&models.HelpGiftDailyQuota{}).Error; err != nil {
			return err
		}
		if err := tx.Where("requester_id = ?", accountID).Delete(&models.CommunityHelpRequest{}).Error; err != nil {
			return err
		}
		deletions := []interface{}{
			&models.NotificationJob{}, &models.NotificationPreference{}, &models.IdentityBindToken{}, &models.SquadMember{},
			&models.CommunityMember{}, &models.CodexEntry{}, &models.ActivityRun{}, &models.ItemUseRecord{}, &models.ExpeditionRun{},
			&models.EventProgress{}, &models.EventProgressGrant{}, &models.EventRewardClaim{},
			&models.ChanceDailyState{}, &models.ChancePlayerState{}, &models.ChanceOutcome{}, &models.FishingRun{}, &models.BattleRecord{}, &models.TradeAudit{},
			&models.CompanionJournal{}, &models.CompanionActionDaily{}, &models.GlobalInventoryItem{}, &models.PlayerWallet{}, &models.WalletLedger{}, &models.PetBehaviorProfile{},
			&models.BossContribution{}, &models.SeasonVote{}, &models.PetProfile{},
			&models.PlayerAdventureProgress{}, &models.PlayerZoneProgress{}, &models.PlayerObjectiveProgress{},
			&models.AdventureExplorationSession{}, &models.AdventureShopPurchase{}, &models.PlayerEquipment{}, &models.PlayerBlueprintProgress{},
			&models.AdventureExpeditionRun{}, &models.AdventureBossContribution{}, &models.AdventureBossRewardClaim{}, &models.EquipmentCraftRecord{},
			&models.PlayerIdentity{},
		}
		for _, model := range deletions {
			if err := tx.Where("account_id = ?", accountID).Delete(model).Error; err != nil {
				return err
			}
		}
		var combatIDs []string
		if err := tx.Model(&models.AdventureCombatSession{}).Where("account_id = ?", accountID).Pluck("id", &combatIDs).Error; err != nil {
			return err
		}
		if len(combatIDs) > 0 {
			if err := tx.Where("session_id IN ?", combatIDs).Delete(&models.AdventureCombatTurn{}).Error; err != nil {
				return err
			}
		}
		if err := tx.Where("account_id = ?", accountID).Delete(&models.AdventureCombatSession{}).Error; err != nil {
			return err
		}
		if err := tx.Where("seller_account_id = ? OR buyer_account_id = ?", accountID, accountID).Delete(&models.TradeOffer{}).Error; err != nil {
			return err
		}
		if err := tx.Where("leader_id = ?", accountID).Delete(&models.ExpeditionSquad{}).Error; err != nil {
			return err
		}
		return tx.Delete(&models.PlayerAccount{}, "id = ?", accountID).Error
	})
}

func (service *Service) SetNotifications(ctx context.Context, accountID string, enabled bool) error {
	preference := models.NotificationPreference{AccountID: accountID, Enabled: enabled, UpdatedAt: service.Now()}
	return service.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "account_id"}},
			DoUpdates: clause.Assignments(map[string]interface{}{"enabled": enabled, "updated_at": service.Now()}),
		}).Create(&preference).Error; err != nil {
			return err
		}
		if !enabled {
			return tx.Model(&models.NotificationJob{}).
				Where("account_id = ? AND status IN ?", accountID, []string{"queued", "sending"}).
				Updates(map[string]interface{}{"status": "cancelled", "locked_at": nil, "updated_at": service.Now()}).Error
		}
		return nil
	})
}

func (service *Service) UnbindIdentity(ctx context.Context, event core.InboundEvent, accountID string) error {
	var count int64
	if err := service.DB.WithContext(ctx).Model(&models.PlayerIdentity{}).Where("account_id = ?", accountID).Count(&count).Error; err != nil {
		return err
	}
	if count <= 1 {
		return errors.New("当前是唯一登录身份，无法解绑；你可以删除我的数据")
	}
	return service.DB.WithContext(ctx).Where("account_id = ? AND platform = ? AND app_id = ? AND scene_type = ? AND scope_id = ? AND subject_id = ?",
		accountID, string(event.Platform), event.AppID, string(event.SceneType), identityScope(event), event.ActorID).Delete(&models.PlayerIdentity{}).Error
}

func (service *Service) JoinSquad(ctx context.Context, event core.InboundEvent, accountID, name string) error {
	if err := ValidatePlayerName(name); err != nil {
		return err
	}
	community, err := service.GetCommunity(ctx, event, accountID)
	if err != nil {
		return err
	}
	return service.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var squad models.ExpeditionSquad
		if err := tx.Where("community_id = ? AND name = ?", community.ID, strings.TrimSpace(name)).First(&squad).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return errors.New("当前社区没有这个远征小队")
			}
			return err
		}
		var existing int64
		if err := tx.Model(&models.SquadMember{}).Where("community_id = ? AND account_id = ?", community.ID, accountID).Count(&existing).Error; err != nil {
			return err
		}
		if existing > 0 {
			return errors.New("你已经加入了当前社区的远征小队")
		}
		var memberCount int64
		if err := tx.Model(&models.SquadMember{}).Where("squad_id = ?", squad.ID).Count(&memberCount).Error; err != nil {
			return err
		}
		if memberCount >= int64(squad.MaxMembers) {
			return errors.New("远征小队成员已满")
		}
		return tx.Create(&models.SquadMember{SquadID: squad.ID, CommunityID: community.ID, AccountID: accountID, JoinedAt: service.Now()}).Error
	})
}

func (service *Service) GetBoss(ctx context.Context, event core.InboundEvent, accountID string) (*models.CommunityBoss, error) {
	community, err := service.GetCommunity(ctx, event, accountID)
	if err != nil {
		return nil, err
	}
	year, week := service.Now().ISOWeek()
	weekKey := fmt.Sprintf("%04d-W%02d", year, week)
	boss := models.CommunityBoss{ID: community.ID + ":" + weekKey, CommunityID: community.ID, WeekKey: weekKey, Name: "迷雾守卫", MaxHP: 1000, CurrentHP: 1000, UpdatedAt: service.Now()}
	if err := service.DB.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&boss).Error; err != nil {
		return nil, err
	}
	if err := service.DB.WithContext(ctx).First(&boss, "id = ?", boss.ID).Error; err != nil {
		return nil, err
	}
	return &boss, nil
}

func (service *Service) ChallengeBoss(ctx context.Context, event core.InboundEvent, accountID string, records int64) (*models.CommunityBoss, int64, error) {
	if records < 1 || records > 50 {
		return nil, 0, errors.New("每次支援需要投入 1 到 50 个调查记录")
	}
	boss, err := service.GetBoss(ctx, event, accountID)
	if err != nil {
		return nil, 0, err
	}
	if boss.Defeated || boss.CurrentHP <= 0 {
		return boss, 0, errors.New("本周社区首领已经完成调查")
	}
	pet, err := gameplay.ActivePetTx(service.DB.WithContext(ctx), accountID)
	if err != nil {
		return nil, 0, err
	}
	multiplier := int64(100)
	switch pet.Stance {
	case "攻击":
		multiplier = 150
	case "守护":
		multiplier = 120
	case "支援":
		multiplier = 130
	}
	damage := records * 3 * multiplier / 100
	influence, err := service.GetSeasonInfluence(ctx, event)
	if err != nil {
		return nil, 0, err
	}
	if influence.EffectType == "boss_damage_gain_percent" {
		damage = expeditionRewardWithBonus(damage, influence.EffectValue)
	}
	err = service.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := gameplay.NewInventoryService(tx).DebitTx(tx, accountID, "调查记录", records); err != nil {
			return err
		}
		update := tx.Model(&models.CommunityBoss{}).Where("id = ? AND current_hp > 0", boss.ID).Updates(map[string]interface{}{
			"current_hp": gorm.Expr("MAX(0, current_hp - ?)", damage),
			"defeated":   gorm.Expr("current_hp - ? <= 0", damage),
			"updated_at": service.Now(),
		})
		if update.Error != nil {
			return update.Error
		}
		if update.RowsAffected != 1 {
			return errors.New("本周社区首领已经完成调查")
		}
		contribution := models.BossContribution{BossID: boss.ID, AccountID: accountID, Damage: damage, UpdatedAt: service.Now()}
		if err := tx.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "boss_id"}, {Name: "account_id"}},
			DoUpdates: clause.Assignments(map[string]interface{}{"damage": gorm.Expr("damage + ?", damage), "updated_at": service.Now()}),
		}).Create(&contribution).Error; err != nil {
			return err
		}
		if err := recordBehaviorTx(tx, accountID, "support", 1, service.Now()); err != nil {
			return err
		}
		return tx.Model(&models.CommunityMember{}).Where("community_id = ? AND account_id = ?", communityID(event), accountID).Updates(map[string]interface{}{
			"contribution": gorm.Expr("contribution + ?", records),
			"rescue_count": gorm.Expr("rescue_count + 1"),
		}).Error
	})
	if err != nil {
		return nil, 0, err
	}
	if err := service.DB.WithContext(ctx).First(boss, "id = ?", boss.ID).Error; err != nil {
		return nil, 0, err
	}
	return boss, damage, nil
}

func (service *Service) CurrentSeason() SeasonInfo {
	if service.DB != nil {
		var configured models.LiveEventConfig
		now := service.Now()
		if err := service.DB.Where("active = ? AND starts_at <= ? AND ends_at > ?", true, now, now).Order("starts_at DESC").First(&configured).Error; err == nil {
			choices, _ := seasonChoicesTx(service.DB, configured)
			labels, keys := make([]string, 0, len(choices)), make([]string, 0, len(choices))
			for _, choice := range choices {
				labels = append(labels, choice.Label)
				keys = append(keys, choice.ChoiceKey)
			}
			if len(labels) >= 2 {
				return SeasonInfo{Key: configured.Key, Name: configured.Name, Region: configured.Region, StartsAt: configured.StartsAt, EndsAt: configured.EndsAt, Choices: labels, ChoiceKeys: keys}
			}
		}
	}
	return SeasonInfo{}
}

func (service *Service) VoteSeason(ctx context.Context, event core.InboundEvent, accountID string, choice int) error {
	season := service.CurrentSeason()
	if season.Key == "" || choice < 1 || choice > len(season.Choices) {
		return errors.New("故事选择编号不在当前活动选项范围内")
	}
	vote := models.SeasonVote{SeasonKey: season.Key, CommunityID: communityID(event), AccountID: accountID, Choice: choice, ChoiceKey: season.ChoiceKeys[choice-1], UpdatedAt: service.Now()}
	return service.DB.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "season_key"}, {Name: "community_id"}, {Name: "account_id"}},
		DoUpdates: clause.Assignments(map[string]interface{}{"choice": choice, "choice_key": vote.ChoiceKey, "updated_at": service.Now()}),
	}).Create(&vote).Error
}

func (service *Service) GetSeasonInfluence(ctx context.Context, event core.InboundEvent) (SeasonInfluence, error) {
	season := service.CurrentSeason()
	if season.Key == "" {
		return SeasonInfluence{Description: "当前没有进行中的故事活动"}, nil
	}
	return seasonInfluenceTx(service.DB.WithContext(ctx), season.Key, communityID(event))
}

func seasonInfluenceTx(tx *gorm.DB, seasonKey, communityID string) (SeasonInfluence, error) {
	var event models.LiveEventConfig
	if err := tx.First(&event, "key = ?", seasonKey).Error; err != nil {
		return SeasonInfluence{}, err
	}
	choices, err := seasonChoicesTx(tx, event)
	if err != nil {
		return SeasonInfluence{}, err
	}
	var votes []models.SeasonVote
	if err := tx.Where("season_key = ? AND community_id = ?", seasonKey, communityID).Find(&votes).Error; err != nil {
		return SeasonInfluence{}, err
	}
	indexByKey := map[string]int{}
	for index, choice := range choices {
		indexByKey[choice.ChoiceKey] = index
	}
	influence := SeasonInfluence{Votes: make([]int64, len(choices))}
	for _, vote := range votes {
		index, ok := indexByKey[vote.ChoiceKey]
		if !ok && vote.Choice > 0 && vote.Choice <= len(choices) {
			index = vote.Choice - 1
			ok = true
		}
		if ok {
			influence.Votes[index]++
		}
	}
	var highest int64
	tied := false
	for index, count := range influence.Votes {
		if count > highest {
			highest = count
			influence.Choice = index + 1
			tied = false
		} else if count == highest && highest > 0 {
			tied = true
		}
	}
	if tied {
		influence.Choice = 0
	}
	if influence.Choice > 0 {
		choice := choices[influence.Choice-1]
		influence.ChoiceKey = choice.ChoiceKey
		influence.EffectType = choice.EffectType
		influence.EffectValue = choice.EffectValue
		influence.Description = fmt.Sprintf("%s：%s %d%%", choice.Label, seasonEffectLabel(choice.EffectType), choice.EffectValue)
	} else {
		influence.Description = "票数尚未形成唯一领先选项，当前无社区加成"
	}
	return influence, nil
}

func seasonChoicesTx(tx *gorm.DB, event models.LiveEventConfig) ([]models.LiveEventChoiceConfig, error) {
	var choices []models.LiveEventChoiceConfig
	if err := tx.Where("event_key = ?", event.Key).Order("sort_order asc, id asc").Find(&choices).Error; err != nil {
		return nil, err
	}
	if len(choices) > 0 {
		return choices, nil
	}
	var labels []string
	if json.Unmarshal([]byte(event.StoryChoices), &labels) != nil {
		return choices, nil
	}
	for index, label := range labels {
		choices = append(choices, models.LiveEventChoiceConfig{EventKey: event.Key, ChoiceKey: fmt.Sprintf("choice-%d", index+1), Label: label, SortOrder: (index + 1) * 10})
	}
	return choices, nil
}

func seasonEffectLabel(effect string) string {
	switch effect {
	case "community_material_gain_percent":
		return "社区共建材料增加"
	case "facility_upgrade_cost_reduction_percent":
		return "设施升级消耗降低"
	case "boss_damage_gain_percent":
		return "地图首领伤害增加"
	case "adventure_xp_gain_percent":
		return "冒险经验增加"
	case "expedition_reward_gain_percent":
		return "远征奖励增加"
	default:
		return "无已配置效果"
	}
}

func (service *Service) GetFacilities(ctx context.Context, event core.InboundEvent, accountID string) ([]models.CommunityFacility, error) {
	community, err := service.GetCommunity(ctx, event, accountID)
	if err != nil {
		return nil, err
	}
	for _, name := range []string{"栖息地", "研究站", "救助站"} {
		facility := models.CommunityFacility{CommunityID: community.ID, Name: name, Level: 1, UpdatedAt: service.Now()}
		if err := service.DB.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&facility).Error; err != nil {
			return nil, err
		}
	}
	var facilities []models.CommunityFacility
	err = service.DB.WithContext(ctx).Where("community_id = ?", community.ID).Order("name").Find(&facilities).Error
	return facilities, err
}

func (service *Service) UpgradeFacility(ctx context.Context, event core.InboundEvent, accountID, name string) (*models.CommunityFacility, error) {
	allowed := map[string]bool{"栖息地": true, "研究站": true, "救助站": true}
	if !allowed[name] {
		return nil, errors.New("设施只能是栖息地、研究站或救助站")
	}
	if _, err := service.GetFacilities(ctx, event, accountID); err != nil {
		return nil, err
	}
	influence, err := service.GetSeasonInfluence(ctx, event)
	if err != nil {
		return nil, err
	}
	var facility models.CommunityFacility
	err = service.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("community_id = ? AND name = ?", communityID(event), name).First(&facility).Error; err != nil {
			return err
		}
		reduction := 0
		if influence.EffectType == "facility_upgrade_cost_reduction_percent" {
			reduction = influence.EffectValue
		}
		cost := facilityUpgradeCost(facility.Level, reduction)
		consume := tx.Model(&models.Community{}).Where("id = ? AND materials >= ?", communityID(event), cost).UpdateColumn("materials", gorm.Expr("materials - ?", cost))
		if consume.Error != nil {
			return consume.Error
		}
		if consume.RowsAffected != 1 {
			return errors.New("社区建设材料不足")
		}
		if err := tx.Model(&facility).Updates(map[string]interface{}{"level": gorm.Expr("level + 1"), "updated_at": service.Now()}).Error; err != nil {
			return err
		}
		return tx.First(&facility, facility.ID).Error
	})
	return &facility, err
}

func facilityUpgradeCost(level, reductionPercent int) int64 {
	cost := int64(level * 100)
	if reductionPercent > 0 {
		cost = int64(math.Ceil(float64(cost) * float64(100-reductionPercent) / 100))
	}
	return cost
}

func (service *Service) CreateHelpRequest(ctx context.Context, event core.InboundEvent, accountID, itemName string, quantity int64) (*models.CommunityHelpRequest, error) {
	itemName = strings.TrimSpace(itemName)
	if itemName == "" || quantity < 1 || quantity > 20 {
		return nil, errors.New("求助物品不能为空，数量需要在 1 到 20 之间")
	}
	if _, err := service.GetCommunity(ctx, event, accountID); err != nil {
		return nil, err
	}
	var active int64
	if err := service.DB.WithContext(ctx).Model(&models.CommunityHelpRequest{}).Where("community_id = ? AND requester_id = ? AND status = ? AND expires_at > ?", communityID(event), accountID, "open", service.Now()).Count(&active).Error; err != nil {
		return nil, err
	}
	if active >= 3 {
		return nil, errors.New("你已有 3 条进行中的求助")
	}
	code := strings.ToUpper(strings.ReplaceAll(uuid.NewString(), "-", "")[:6])
	request := models.CommunityHelpRequest{
		Code: code, CommunityID: communityID(event), RequesterID: accountID, ItemName: itemName,
		Quantity: quantity, Status: "open", ExpiresAt: service.Now().Add(24 * time.Hour), CreatedAt: service.Now(), UpdatedAt: service.Now(),
	}
	return &request, service.DB.WithContext(ctx).Create(&request).Error
}

func (service *Service) SupportHelpRequest(ctx context.Context, event core.InboundEvent, donorID, code string, quantity int64) (*models.CommunityHelpRequest, error) {
	if quantity < 1 {
		return nil, errors.New("支援数量必须大于零")
	}
	var request models.CommunityHelpRequest
	err := gameplay.WithTransactionRetry(ctx, service.DB, func(tx *gorm.DB) error {
		request = models.CommunityHelpRequest{}
		if err := tx.Where("code = ? AND community_id = ? AND status = ? AND expires_at > ?", strings.ToUpper(strings.TrimSpace(code)), communityID(event), "open", service.Now()).First(&request).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return errors.New("当前社区没有这条有效求助")
			}
			return err
		}
		if request.RequesterID == donorID {
			return errors.New("不能支援自己发布的求助")
		}
		remaining := request.Quantity - request.Fulfilled
		if quantity > remaining {
			return fmt.Errorf("这条求助还需要 %d 个%s", remaining, request.ItemName)
		}
		now := service.Now()
		quota := tx.Exec(`INSERT INTO help_gift_daily_quotas (donor_id, day, quantity, updated_at)
			VALUES (?, ?, ?, ?)
			ON CONFLICT(donor_id, day) DO UPDATE SET
				quantity = help_gift_daily_quotas.quantity + excluded.quantity,
				updated_at = excluded.updated_at
			WHERE help_gift_daily_quotas.quantity + excluded.quantity <= 20`,
			donorID, now.Format("2006-01-02"), quantity, now)
		if quota.Error != nil {
			return quota.Error
		}
		if quota.RowsAffected != 1 {
			return errors.New("每天最多通过求助单赠送 20 件物品")
		}

		requestUpdate := tx.Model(&models.CommunityHelpRequest{}).
			Where("code = ? AND community_id = ? AND status = ? AND expires_at > ? AND fulfilled + ? <= quantity",
				request.Code, request.CommunityID, "open", now, quantity).
			Updates(map[string]interface{}{
				"fulfilled":  gorm.Expr("fulfilled + ?", quantity),
				"status":     gorm.Expr("CASE WHEN fulfilled + ? >= quantity THEN 'fulfilled' ELSE 'open' END", quantity),
				"updated_at": now,
			})
		if requestUpdate.Error != nil {
			return requestUpdate.Error
		}
		if requestUpdate.RowsAffected != 1 {
			var current models.CommunityHelpRequest
			if err := tx.First(&current, "code = ?", request.Code).Error; err != nil {
				return err
			}
			remaining := current.Quantity - current.Fulfilled
			if remaining < 0 {
				remaining = 0
			}
			return fmt.Errorf("这条求助还需要 %d 个%s", remaining, current.ItemName)
		}
		consume := tx.Model(&models.GlobalInventoryItem{}).Where("account_id = ? AND item_name = ? AND quantity >= ?", donorID, request.ItemName, quantity).UpdateColumn("quantity", gorm.Expr("quantity - ?", quantity))
		if consume.Error != nil {
			return consume.Error
		}
		if consume.RowsAffected != 1 {
			return ErrInsufficientItem
		}
		if err := addInventoryTx(tx, request.RequesterID, request.ItemName, quantity); err != nil {
			return err
		}
		logEntry := models.HelpGiftLog{RequestCode: request.Code, CommunityID: request.CommunityID, DonorID: donorID, ItemName: request.ItemName, Quantity: quantity, Day: now.Format("2006-01-02"), CreatedAt: now}
		if err := tx.Create(&logEntry).Error; err != nil {
			return err
		}
		return tx.First(&request, "code = ?", request.Code).Error
	})
	return &request, err
}

func recordBehaviorTx(tx *gorm.DB, accountID, behavior string, amount int64, now time.Time) error {
	column := map[string]string{"explore": "explore", "care": "care", "support": "support"}[behavior]
	if column == "" {
		return errors.New("未知宠物行为")
	}
	pet, err := gameplay.ActivePetTx(tx, accountID)
	if err != nil {
		return err
	}
	profile := models.PetBehaviorProfile{PetID: pet.ID, AccountID: accountID, UpdatedAt: now}
	switch behavior {
	case "explore":
		profile.Explore = amount
	case "care":
		profile.Care = amount
	case "support":
		profile.Support = amount
	}
	if err := tx.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "pet_id"}},
		DoUpdates: clause.Assignments(map[string]interface{}{column: gorm.Expr(column+" + ?", amount), "updated_at": now}),
	}).Create(&profile).Error; err != nil {
		return err
	}
	if err := tx.First(&profile, "pet_id = ?", pet.ID).Error; err != nil {
		return err
	}
	trait := gameplayrules.ResolveTrait(tx, profile)
	if trait != "" && trait != profile.Trait {
		if err := tx.Model(&profile).Update("trait", trait).Error; err != nil {
			return err
		}
		return tx.Model(&models.PetProfile{}).Where("id = ?", pet.ID).Update("traits", trait).Error
	}
	return nil
}

func (service *Service) SetRole(ctx context.Context, accountID, role string) (*models.PetProfile, error) {
	available := make([]string, 0)
	matched := false
	for _, configured := range gameplayrules.EnabledRoles(service.DB.WithContext(ctx)) {
		available = append(available, configured.Name)
		if configured.Name == role {
			matched = true
		}
	}
	if !matched {
		return nil, fmt.Errorf("定位只能是%s", humanList(available))
	}
	var updated *models.PetProfile
	err := gameplay.WithTransactionRetry(ctx, service.DB, func(tx *gorm.DB) error {
		pet, err := gameplay.ActivePetTx(tx, accountID)
		if err != nil {
			return err
		}
		if err = tx.Model(&models.PetProfile{}).Where("id = ?", pet.ID).Update("role", role).Error; err != nil {
			return err
		}
		pet.Role = role
		if err = gameplay.RefreshPetSkillsTx(tx, pet); err != nil {
			return err
		}
		updated = pet
		return nil
	})
	return updated, err
}

func humanList(values []string) string {
	if len(values) < 2 {
		return strings.Join(values, "")
	}
	return strings.Join(values[:len(values)-1], "、") + "或" + values[len(values)-1]
}
