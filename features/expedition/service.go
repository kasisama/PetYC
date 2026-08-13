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
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"qq-pet-saas/core"
	"qq-pet-saas/gameplayrules"
	"qq-pet-saas/models"
)

var (
	ErrPetRequired        = errors.New("请先领养宠物")
	ErrExpeditionActive   = errors.New("已有进行中的远征")
	ErrExpeditionNotReady = errors.New("远征尚未结束")
	ErrNothingToClaim     = errors.New("没有可领取的远征奖励")
	ErrInsufficientItem   = errors.New("物品数量不足")
	ErrInvalidBindToken   = errors.New("绑定码无效或已过期")
	ErrBindConflict       = errors.New("目标身份已有独立游戏进度，请先使用“删除我的数据”清理目标存档")
)

type ExpeditionResult struct {
	Name       string
	Item       string
	Quantity   int64
	Records    int64
	Growth     int64
	CodexEntry string
	Progress   int
}

type expeditionTier struct {
	Name       string
	Duration   time.Duration
	Item       string
	Quantity   int64
	Records    int64
	Growth     int64
	CodexEntry string
	Progress   int
}

var expeditionTiers = map[int]expeditionTier{
	1: {Name: "林间巡查", Duration: 10 * time.Minute, Item: "林地样本", Quantity: 1, Records: 6, Growth: 4, CodexEntry: "林间足迹", Progress: 15},
	2: {Name: "遗迹调查", Duration: 2 * time.Hour, Item: "古代零件", Quantity: 3, Records: 12, Growth: 10, CodexEntry: "遗迹守卫", Progress: 15},
	3: {Name: "深层生态勘察", Duration: 8 * time.Hour, Item: "生态样本", Quantity: 2, Records: 25, Growth: 24, CodexEntry: "深层生态", Progress: 20},
}

type Service struct {
	DB          *gorm.DB
	Now         func() time.Time
	TokenSource func() (string, error)
}

type SeasonInfo struct {
	Key      string
	Name     string
	Region   string
	StartsAt time.Time
	EndsAt   time.Time
	Choices  []string
}

func NewService(db *gorm.DB) *Service {
	return &Service{DB: db, Now: time.Now, TokenSource: randomBindToken}
}

func identityScope(event core.InboundEvent) string {
	if event.Platform == core.PlatformOneBot {
		return "*"
	}
	return event.SpaceID
}

func (service *Service) ResolveAccount(ctx context.Context, event core.InboundEvent) (*models.PlayerAccount, error) {
	var account models.PlayerAccount
	err := service.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var identity models.PlayerIdentity
		query := tx.Where("platform = ? AND app_id = ? AND scene_type = ? AND scope_id = ? AND subject_id = ?",
			string(event.Platform), event.AppID, string(event.SceneType), identityScope(event), event.ActorID)
		if err := query.First(&identity).Error; err == nil {
			return tx.First(&account, "id = ?", identity.AccountID).Error
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		account = models.PlayerAccount{ID: uuid.NewString()}
		if err := tx.Create(&account).Error; err != nil {
			return err
		}
		identity = models.PlayerIdentity{
			AccountID: account.ID, Platform: string(event.Platform), AppID: event.AppID,
			SceneType: string(event.SceneType), ScopeID: identityScope(event), SubjectID: event.ActorID,
		}
		return tx.Create(&identity).Error
	})
	return &account, err
}

func (service *Service) Adopt(ctx context.Context, accountID, petType, name string) (*models.PetProfile, error) {
	pet := models.PetProfile{AccountID: accountID, PetType: petType, Name: name, Role: "探索者", Stance: "探索", Mood: "期待", Readiness: 100, BondLevel: 1}
	err := service.DB.WithContext(ctx).Create(&pet).Error
	return &pet, err
}

func (service *Service) StartExpedition(ctx context.Context, accountID string, tierNumber int) (*models.ExpeditionRun, error) {
	tier, ok := expeditionTiers[tierNumber]
	if !ok {
		return nil, fmt.Errorf("远征档位只能是 1、2 或 3")
	}
	var pet models.PetProfile
	if err := service.DB.WithContext(ctx).First(&pet, "account_id = ?", accountID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrPetRequired
		}
		return nil, err
	}
	var count int64
	if err := service.DB.WithContext(ctx).Model(&models.ExpeditionRun{}).Where("account_id = ? AND status = ?", accountID, "running").Count(&count).Error; err != nil {
		return nil, err
	}
	if count > 0 {
		return nil, ErrExpeditionActive
	}
	now := service.Now()
	run := models.ExpeditionRun{
		ID: uuid.NewString(), AccountID: accountID, Tier: tierNumber, Name: tier.Name, Stance: pet.Stance,
		Status: "running", RewardItem: tier.Item, RewardQuantity: tier.Quantity,
		RewardRecords: tier.Records, RewardGrowth: tier.Growth, StartedAt: now, EndsAt: now.Add(tier.Duration),
	}
	return &run, service.DB.WithContext(ctx).Create(&run).Error
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
	err := service.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
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
		if err := addInventoryTx(tx, accountID, run.RewardItem, run.RewardQuantity); err != nil {
			return err
		}
		if err := addInventoryTx(tx, accountID, "调查记录", run.RewardRecords); err != nil {
			return err
		}
		if err := tx.Model(&models.PetProfile{}).Where("account_id = ?", accountID).UpdateColumn("growth", gorm.Expr("growth + ?", run.RewardGrowth)).Error; err != nil {
			return err
		}
		if err := recordBehaviorTx(tx, accountID, "explore", int64(run.Tier), now); err != nil {
			return err
		}
		tier := expeditionTiers[run.Tier]
		entry := models.CodexEntry{AccountID: accountID, Category: "地区", EntryKey: tier.CodexEntry, Progress: tier.Progress}
		if err := tx.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "account_id"}, {Name: "category"}, {Name: "entry_key"}},
			DoUpdates: clause.Assignments(map[string]interface{}{"progress": gorm.Expr("MIN(100, progress + ?)", tier.Progress), "updated_at": now}),
		}).Create(&entry).Error; err != nil {
			return err
		}
		if err := tx.Where("account_id = ? AND category = ? AND entry_key = ?", accountID, "地区", tier.CodexEntry).First(&entry).Error; err != nil {
			return err
		}
		result = ExpeditionResult{Name: run.Name, Item: run.RewardItem, Quantity: run.RewardQuantity, Records: run.RewardRecords, Growth: run.RewardGrowth, CodexEntry: tier.CodexEntry, Progress: entry.Progress}
		return nil
	})
	return &result, err
}

func addInventoryTx(tx *gorm.DB, accountID, itemName string, quantity int64) error {
	item := models.GlobalInventoryItem{AccountID: accountID, ItemName: itemName, Quantity: quantity}
	return tx.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "account_id"}, {Name: "item_name"}},
		DoUpdates: clause.Assignments(map[string]interface{}{"quantity": gorm.Expr("quantity + ?", quantity), "updated_at": time.Now()}),
	}).Create(&item).Error
}

func (service *Service) AddInventory(ctx context.Context, accountID, itemName string, quantity int64) error {
	if quantity <= 0 {
		return errors.New("数量必须大于零")
	}
	return addInventoryTx(service.DB.WithContext(ctx), accountID, itemName, quantity)
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
	err = service.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		consume := tx.Model(&models.GlobalInventoryItem{}).Where("account_id = ? AND item_name = ? AND quantity >= ?", accountID, itemName, quantity).UpdateColumn("quantity", gorm.Expr("quantity - ?", quantity))
		if consume.Error != nil {
			return consume.Error
		}
		if consume.RowsAffected != 1 {
			return ErrInsufficientItem
		}
		if err := tx.Model(&models.Community{}).Where("id = ?", community.ID).Updates(map[string]interface{}{"materials": gorm.Expr("materials + ?", quantity), "level": gorm.Expr("1 + (materials + ?) / 100", quantity)}).Error; err != nil {
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
		&models.PetProfile{}, &models.GlobalInventoryItem{}, &models.CompanionJournal{},
		&models.ExpeditionRun{}, &models.CodexEntry{}, &models.CommunityMember{},
		&models.SquadMember{}, &models.PetBehaviorProfile{}, &models.SeasonVote{},
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
	var streak int64
	rewarded := false
	now := service.Now()
	err := service.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		entry := models.CompanionJournal{AccountID: accountID, Day: now.Format("2006-01-02"), Action: action, CreatedAt: now}
		result := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&entry)
		if result.Error != nil {
			return result.Error
		}
		rewarded = result.RowsAffected == 1
		if rewarded {
			if err := tx.Model(&models.PetProfile{}).Where("account_id = ?", accountID).Updates(map[string]interface{}{
				"affection": gorm.Expr("affection + 1"),
				"readiness": gorm.Expr("MIN(100, readiness + 10)"),
				"mood":      "愉快",
			}).Error; err != nil {
				return err
			}
			if err := addInventoryTx(tx, accountID, "陪伴印记", 1); err != nil {
				return err
			}
			if err := recordBehaviorTx(tx, accountID, "care", 1, now); err != nil {
				return err
			}
		}
		return tx.Model(&models.CompanionJournal{}).
			Where("account_id = ? AND day >= ? AND day <= ?", accountID, now.AddDate(0, 0, -6).Format("2006-01-02"), now.Format("2006-01-02")).
			Count(&streak).Error
	})
	return streak, rewarded, err
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
			result := service.DB.WithContext(ctx).Model(&models.PetProfile{}).Where("account_id = ?", accountID).Update("stance", stance)
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
		if err := tx.Where("requester_id = ?", accountID).Delete(&models.CommunityHelpRequest{}).Error; err != nil {
			return err
		}
		deletions := []interface{}{
			&models.NotificationPreference{}, &models.IdentityBindToken{}, &models.SquadMember{},
			&models.CommunityMember{}, &models.CodexEntry{}, &models.ExpeditionRun{},
			&models.CompanionJournal{}, &models.GlobalInventoryItem{}, &models.PetBehaviorProfile{},
			&models.BossContribution{}, &models.SeasonVote{}, &models.PetProfile{},
			&models.PlayerIdentity{},
		}
		for _, model := range deletions {
			if err := tx.Where("account_id = ?", accountID).Delete(model).Error; err != nil {
				return err
			}
		}
		if err := tx.Where("leader_id = ?", accountID).Delete(&models.ExpeditionSquad{}).Error; err != nil {
			return err
		}
		return tx.Delete(&models.PlayerAccount{}, "id = ?", accountID).Error
	})
}

func (service *Service) SetNotifications(ctx context.Context, accountID string, enabled bool) error {
	preference := models.NotificationPreference{AccountID: accountID, Enabled: enabled, UpdatedAt: service.Now()}
	return service.DB.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "account_id"}},
		DoUpdates: clause.Assignments(map[string]interface{}{"enabled": enabled, "updated_at": service.Now()}),
	}).Create(&preference).Error
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
	var pet models.PetProfile
	if err := service.DB.WithContext(ctx).First(&pet, "account_id = ?", accountID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, 0, ErrPetRequired
		}
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
	err = service.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		consume := tx.Model(&models.GlobalInventoryItem{}).Where("account_id = ? AND item_name = ? AND quantity >= ?", accountID, "调查记录", records).UpdateColumn("quantity", gorm.Expr("quantity - ?", records))
		if consume.Error != nil {
			return consume.Error
		}
		if consume.RowsAffected != 1 {
			return ErrInsufficientItem
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
			var choices []string
			if json.Unmarshal([]byte(configured.StoryChoices), &choices) == nil && len(choices) == 3 {
				return SeasonInfo{Key: configured.Key, Name: configured.Name, Region: configured.Region, StartsAt: configured.StartsAt, EndsAt: configured.EndsAt, Choices: choices}
			}
		}
	}
	epoch := time.Date(2026, 1, 1, 0, 0, 0, 0, service.Now().Location())
	now := service.Now()
	cycle := int(now.Sub(epoch) / (56 * 24 * time.Hour))
	if cycle < 0 {
		cycle = 0
	}
	startsAt := epoch.Add(time.Duration(cycle) * 56 * 24 * time.Hour)
	themes := []struct{ name, region string }{
		{"林海来信", "森林"}, {"潮汐手记", "水域"}, {"遗迹回声", "遗迹"}, {"城市微光", "城市"},
	}
	theme := themes[cycle%len(themes)]
	return SeasonInfo{
		Key: fmt.Sprintf("S%03d", cycle+1), Name: theme.name, Region: theme.region,
		StartsAt: startsAt, EndsAt: startsAt.Add(56 * 24 * time.Hour),
		Choices: []string{"优先调查未知区域", "优先建设救助设施", "优先支援社区首领"},
	}
}

func (service *Service) VoteSeason(ctx context.Context, event core.InboundEvent, accountID string, choice int) error {
	if choice < 1 || choice > 3 {
		return errors.New("故事选择只能是 1、2 或 3")
	}
	season := service.CurrentSeason()
	vote := models.SeasonVote{SeasonKey: season.Key, CommunityID: communityID(event), AccountID: accountID, Choice: choice, UpdatedAt: service.Now()}
	return service.DB.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "season_key"}, {Name: "community_id"}, {Name: "account_id"}},
		DoUpdates: clause.Assignments(map[string]interface{}{"choice": choice, "updated_at": service.Now()}),
	}).Create(&vote).Error
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
	var facility models.CommunityFacility
	err := service.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("community_id = ? AND name = ?", communityID(event), name).First(&facility).Error; err != nil {
			return err
		}
		cost := int64(facility.Level * 100)
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
	err := service.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
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
		var donated int64
		if err := tx.Model(&models.HelpGiftLog{}).Where("donor_id = ? AND day = ?", donorID, service.Now().Format("2006-01-02")).Select("COALESCE(SUM(quantity), 0)").Scan(&donated).Error; err != nil {
			return err
		}
		if donated+quantity > 20 {
			return errors.New("每天最多通过求助单赠送 20 件物品")
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
		newFulfilled := request.Fulfilled + quantity
		status := "open"
		if newFulfilled >= request.Quantity {
			status = "fulfilled"
		}
		if err := tx.Model(&request).Updates(map[string]interface{}{"fulfilled": newFulfilled, "status": status, "updated_at": service.Now()}).Error; err != nil {
			return err
		}
		logEntry := models.HelpGiftLog{RequestCode: request.Code, CommunityID: request.CommunityID, DonorID: donorID, ItemName: request.ItemName, Quantity: quantity, Day: service.Now().Format("2006-01-02"), CreatedAt: service.Now()}
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
	profile := models.PetBehaviorProfile{AccountID: accountID, UpdatedAt: now}
	switch behavior {
	case "explore":
		profile.Explore = amount
	case "care":
		profile.Care = amount
	case "support":
		profile.Support = amount
	}
	if err := tx.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "account_id"}},
		DoUpdates: clause.Assignments(map[string]interface{}{column: gorm.Expr(column+" + ?", amount), "updated_at": now}),
	}).Create(&profile).Error; err != nil {
		return err
	}
	if err := tx.First(&profile, "account_id = ?", accountID).Error; err != nil {
		return err
	}
	trait := gameplayrules.ResolveTrait(tx, profile)
	if trait != "" && trait != profile.Trait {
		if err := tx.Model(&profile).Update("trait", trait).Error; err != nil {
			return err
		}
		return tx.Model(&models.PetProfile{}).Where("account_id = ?", accountID).Update("traits", trait).Error
	}
	return nil
}

func (service *Service) SetRole(ctx context.Context, accountID, role string) (*models.PetProfile, error) {
	available := make([]string, 0)
	skills := ""
	for _, configured := range gameplayrules.EnabledRoles(service.DB.WithContext(ctx)) {
		available = append(available, configured.Name)
		if configured.Name == role {
			skills = gameplayrules.Skills(configured)
		}
	}
	if skills == "" {
		return nil, fmt.Errorf("定位只能是%s", humanList(available))
	}
	result := service.DB.WithContext(ctx).Model(&models.PetProfile{}).Where("account_id = ?", accountID).Updates(map[string]interface{}{"role": role, "skills": skills})
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected == 0 {
		return nil, ErrPetRequired
	}
	var pet models.PetProfile
	return &pet, service.DB.WithContext(ctx).First(&pet, "account_id = ?", accountID).Error
}

func humanList(values []string) string {
	if len(values) < 2 {
		return strings.Join(values, "")
	}
	return strings.Join(values[:len(values)-1], "、") + "或" + values[len(values)-1]
}
