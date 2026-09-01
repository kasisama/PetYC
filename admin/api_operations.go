package admin

import (
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"qq-pet-saas/models"
	"qq-pet-saas/security"
)

func creditUnifiedItemTx(tx *gorm.DB, accountID, itemName string, quantity int64, now time.Time) error {
	itemName = strings.TrimSpace(itemName)
	if accountID == "" || itemName == "" || quantity <= 0 {
		return errors.New("物品发放参数无效")
	}
	itemKey, displayName := itemName, itemName
	var configured models.ItemConfig
	if result := tx.Limit(1).Find(&configured, "key = ? OR name = ?", itemName, itemName); result.Error == nil && result.RowsAffected > 0 {
		if strings.TrimSpace(configured.Key) != "" {
			itemKey = configured.Key
		}
		if strings.TrimSpace(configured.Name) != "" {
			displayName = configured.Name
		}
	}
	item := models.GlobalInventoryItem{AccountID: accountID, ItemKey: itemKey, ItemName: displayName, Quantity: quantity, UpdatedAt: now}
	return tx.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "account_id"}, {Name: "item_key"}},
		DoUpdates: clause.Assignments(map[string]interface{}{
			"quantity":   gorm.Expr("quantity + ?", quantity),
			"updated_at": now,
			"item_name":  displayName,
		}),
	}).Create(&item).Error
}

var errConcurrentChange = errors.New("数据已发生变化，请刷新后重试")

func auditOperator() string {
	credentials, err := security.LoadCredentials()
	if err == nil && strings.TrimSpace(credentials.AdminUsername) != "" {
		return credentials.AdminUsername
	}
	return "admin"
}

func encodeAudit(value interface{}) string {
	if value == nil {
		return ""
	}
	data, err := json.Marshal(value)
	if err != nil {
		return ""
	}
	return string(data)
}

func requiredReason(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", errors.New("必须填写操作原因")
	}
	if len([]rune(value)) > 500 {
		return "", errors.New("操作原因不能超过 500 字")
	}
	return value, nil
}

func writeAudit(tx *gorm.DB, action, targetType, targetID, reason string, before, after interface{}, success bool, operationError error) error {
	message := ""
	if operationError != nil {
		message = operationError.Error()
	}
	return tx.Create(&models.AdminAuditLog{Operator: auditOperator(), Action: action, TargetType: targetType, TargetID: targetID, Reason: reason, BeforeJSON: encodeAudit(before), AfterJSON: encodeAudit(after), Success: success, ErrorMessage: message, CreatedAt: time.Now()}).Error
}

func (api *EcosystemAPI) auditedMutation(c *gin.Context, action, targetType, targetID, reason string, before interface{}, mutate func(*gorm.DB) (interface{}, error)) {
	markAuditRecorded(c)
	resultReason, err := requiredReason(reason)
	if err != nil {
		Error(c, 4000, err.Error())
		return
	}
	var after interface{}
	err = api.DB.Transaction(func(tx *gorm.DB) error {
		var mutationErr error
		after, mutationErr = mutate(tx)
		if mutationErr != nil {
			return mutationErr
		}
		return writeAudit(tx, action, targetType, targetID, resultReason, before, after, true, nil)
	})
	if err != nil {
		_ = writeAudit(api.DB, action, targetType, targetID, resultReason, before, nil, false, err)
		if errors.Is(err, gorm.ErrRecordNotFound) {
			Error(c, 4040, "目标记录不存在")
			return
		}
		if errors.Is(err, errConcurrentChange) {
			Error(c, 4090, err.Error())
			return
		}
		Error(c, 4000, err.Error())
		return
	}
	Success(c, after)
}

type grantRequest struct {
	ItemName       string `json:"item_name"`
	Quantity       int64  `json:"quantity"`
	Reason         string `json:"reason"`
	IdempotencyKey string `json:"idempotency_key"`
}

func (api *EcosystemAPI) GrantItem(c *gin.Context) {
	accountID := c.Param("account_id")
	var request grantRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		Error(c, 4000, "请求格式错误")
		return
	}
	request.ItemName = strings.TrimSpace(request.ItemName)
	request.IdempotencyKey = strings.TrimSpace(request.IdempotencyKey)
	if request.ItemName == "" || request.Quantity < 1 || request.Quantity > 9999 {
		Error(c, 4000, "物品名称不能为空，数量必须为 1 到 9999")
		return
	}
	if len(request.IdempotencyKey) < 8 || len(request.IdempotencyKey) > 128 {
		Error(c, 4000, "幂等键长度必须为 8 到 128")
		return
	}
	var account models.PlayerAccount
	if err := api.DB.First(&account, "id = ?", accountID).Error; err != nil {
		Error(c, 4040, "玩家不存在")
		return
	}
	var existing models.AdminOperationKey
	if err := api.DB.First(&existing, "key = ?", request.IdempotencyKey).Error; err == nil {
		if existing.Action != "grant_item" || existing.TargetID != accountID {
			Error(c, 4090, "幂等键已被其他操作使用")
			return
		}
		var item models.GlobalInventoryItem
		api.DB.First(&item, "account_id = ? AND item_name = ?", accountID, request.ItemName)
		Success(c, gin.H{"item": item, "replayed": true})
		return
	}
	var configuredItem models.ItemConfig
	if err := api.DB.Where("name = ?", request.ItemName).First(&configuredItem).Error; err == nil {
		status := strings.ToLower(strings.TrimSpace(configuredItem.Status))
		if status == "" {
			status = "active"
		}
		if status != "active" {
			Error(c, 4090, "该物品当前不可补发，请先在物品库调整状态")
			return
		}
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		Error(c, 5000, "读取物品状态失败")
		return
	}
	before := gin.H{"item_name": request.ItemName}
	api.auditedMutation(c, "grant_item", "player", accountID, request.Reason, before, func(tx *gorm.DB) (interface{}, error) {
		if err := tx.Create(&models.AdminOperationKey{Key: request.IdempotencyKey, Action: "grant_item", TargetID: accountID, CreatedAt: time.Now()}).Error; err != nil {
			return nil, errors.New("幂等键冲突，请刷新结果")
		}
		if err := creditUnifiedItemTx(tx, accountID, request.ItemName, request.Quantity, time.Now()); err != nil {
			return nil, err
		}
		var item models.GlobalInventoryItem
		if err := tx.First(&item, "account_id = ? AND item_name = ?", accountID, request.ItemName).Error; err != nil {
			return nil, err
		}
		return gin.H{"item": item, "replayed": false}, nil
	})
}

type currencyAdjustRequest struct {
	CurrencyKey    string `json:"currency_key"`
	Amount         int64  `json:"amount"`
	Direction      string `json:"direction"`
	Reason         string `json:"reason"`
	IdempotencyKey string `json:"idempotency_key"`
}

func (api *EcosystemAPI) AdjustCurrency(c *gin.Context) {
	accountID := c.Param("account_id")
	var request currencyAdjustRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		Error(c, 4000, "请求格式错误")
		return
	}
	request.CurrencyKey = strings.TrimSpace(request.CurrencyKey)
	request.Direction = strings.ToLower(strings.TrimSpace(request.Direction))
	request.IdempotencyKey = strings.TrimSpace(request.IdempotencyKey)
	if request.CurrencyKey == "" || request.Amount < 1 || request.Amount > 1000000 {
		Error(c, 4000, "货币键不能为空，数量必须为 1 到 1000000")
		return
	}
	if request.Direction != "grant" && request.Direction != "debit" {
		Error(c, 4000, "direction 只能是 grant 或 debit")
		return
	}
	if len(request.IdempotencyKey) < 8 || len(request.IdempotencyKey) > 128 {
		Error(c, 4000, "幂等键长度必须为 8 到 128")
		return
	}
	action := "grant_currency"
	if request.Direction == "debit" {
		action = "debit_currency"
	}
	var account models.PlayerAccount
	if err := api.DB.First(&account, "id = ?", accountID).Error; err != nil {
		Error(c, 4040, "玩家不存在")
		return
	}
	if !currencyKeyAllowed(api.DB, request.CurrencyKey) {
		Error(c, 4000, "未知或未启用的货币")
		return
	}
	var existing models.AdminOperationKey
	if err := api.DB.First(&existing, "key = ?", request.IdempotencyKey).Error; err == nil {
		if existing.Action != action || existing.TargetID != accountID {
			Error(c, 4090, "幂等键已被其他操作使用")
			return
		}
		var wallet models.PlayerWallet
		api.DB.Limit(1).Find(&wallet, "account_id = ? AND currency_key = ?", accountID, request.CurrencyKey)
		Success(c, gin.H{"wallet": wallet, "replayed": true})
		return
	}
	before := gin.H{"currency_key": request.CurrencyKey, "amount": request.Amount, "direction": request.Direction}
	api.auditedMutation(c, action, "player", accountID, request.Reason, before, func(tx *gorm.DB) (interface{}, error) {
		if err := tx.Create(&models.AdminOperationKey{Key: request.IdempotencyKey, Action: action, TargetID: accountID, CreatedAt: time.Now()}).Error; err != nil {
			return nil, errors.New("幂等键冲突，请刷新结果")
		}
		if err := adjustWalletTx(tx, accountID, request.CurrencyKey, request.Amount, request.Direction == "debit", action, request.IdempotencyKey); err != nil {
			return nil, err
		}
		var wallet models.PlayerWallet
		if err := tx.First(&wallet, "account_id = ? AND currency_key = ?", accountID, request.CurrencyKey).Error; err != nil {
			return nil, err
		}
		return gin.H{"wallet": wallet, "replayed": false}, nil
	})
}

func currencyKeyAllowed(db *gorm.DB, currencyKey string) bool {
	switch currencyKey {
	case "primary_coin", "journey_badge", "season_token":
		return true
	}
	if db == nil || !db.Migrator().HasTable(&models.CurrencyConfig{}) {
		return false
	}
	var configured models.CurrencyConfig
	result := db.Limit(1).Find(&configured, "key = ? AND enabled = ?", currencyKey, true)
	return result.Error == nil && result.RowsAffected > 0
}

func adjustWalletTx(tx *gorm.DB, accountID, currencyKey string, amount int64, debit bool, reason, referenceKey string) error {
	now := time.Now()
	if debit {
		result := tx.Model(&models.PlayerWallet{}).
			Where("account_id = ? AND currency_key = ? AND balance >= ?", accountID, currencyKey, amount).
			Updates(map[string]interface{}{"balance": gorm.Expr("balance - ?", amount), "updated_at": now})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return errors.New("余额不足，无法扣减")
		}
	} else {
		wallet := models.PlayerWallet{AccountID: accountID, CurrencyKey: currencyKey, Balance: amount, UpdatedAt: now}
		if err := tx.Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "account_id"}, {Name: "currency_key"}},
			DoUpdates: clause.Assignments(map[string]interface{}{
				"balance":    gorm.Expr("balance + ?", amount),
				"updated_at": now,
			}),
		}).Create(&wallet).Error; err != nil {
			return err
		}
	}
	var wallet models.PlayerWallet
	if err := tx.First(&wallet, "account_id = ? AND currency_key = ?", accountID, currencyKey).Error; err != nil {
		return err
	}
	delta := amount
	if debit {
		delta = -amount
	}
	return tx.Create(&models.WalletLedger{
		ID: uuid.NewString(), AccountID: accountID, CurrencyKey: currencyKey, Delta: delta,
		BalanceAfter: wallet.Balance, Reason: reason, ReferenceKey: referenceKey, CreatedAt: now,
	}).Error
}

func (api *EcosystemAPI) SetActivePet(c *gin.Context) {
	accountID := c.Param("account_id")
	var request struct {
		PetID  string `json:"pet_id"`
		Reason string `json:"reason"`
	}
	if err := c.ShouldBindJSON(&request); err != nil || strings.TrimSpace(request.PetID) == "" {
		Error(c, 4000, "请指定要切换的宠物")
		return
	}
	var account models.PlayerAccount
	if err := api.DB.First(&account, "id = ?", accountID).Error; err != nil {
		Error(c, 4040, "玩家不存在")
		return
	}
	var pet models.PetProfile
	if err := api.DB.First(&pet, "id = ? AND account_id = ?", request.PetID, accountID).Error; err != nil {
		Error(c, 4040, "宠物不存在或不属于该账号")
		return
	}
	api.auditedMutation(c, "set_active_pet", "player", accountID, request.Reason, gin.H{"active_pet_id": account.ActivePetID}, func(tx *gorm.DB) (interface{}, error) {
		if err := tx.Model(&models.PlayerAccount{}).Where("id = ?", accountID).Update("active_pet_id", pet.ID).Error; err != nil {
			return nil, err
		}
		return gin.H{"active_pet_id": pet.ID, "pet": pet}, nil
	})
}

func (api *EcosystemAPI) BanPlayer(c *gin.Context) {
	accountID := c.Param("account_id")
	var request struct {
		Reason        string `json:"reason"`
		Confirmation  string `json:"confirmation"`
		DurationHours int    `json:"duration_hours"`
	}
	if err := c.ShouldBindJSON(&request); err != nil {
		Error(c, 4000, "请求格式错误")
		return
	}
	if strings.TrimSpace(request.Confirmation) != "封禁" {
		Error(c, 4000, "请输入「封禁」以确认操作")
		return
	}
	if request.DurationHours < 0 || request.DurationHours > 8760 {
		Error(c, 4000, "封禁时长必须在 0 到 8760 小时之间")
		return
	}
	var account models.PlayerAccount
	if err := api.DB.First(&account, "id = ?", accountID).Error; err != nil {
		Error(c, 4040, "玩家不存在")
		return
	}
	api.auditedMutation(c, "ban_player", "player", accountID, request.Reason, gin.H{"banned_at": account.BannedAt}, func(tx *gorm.DB) (interface{}, error) {
		now := time.Now()
		account.BannedAt = &now
		account.BanReason = strings.TrimSpace(request.Reason)
		if request.DurationHours > 0 {
			expires := now.Add(time.Duration(request.DurationHours) * time.Hour)
			account.BanExpiresAt = &expires
		} else {
			account.BanExpiresAt = nil
		}
		if err := tx.Save(&account).Error; err != nil {
			return nil, err
		}
		return gin.H{"banned": true, "banned_at": account.BannedAt, "ban_expires_at": account.BanExpiresAt}, nil
	})
}

func (api *EcosystemAPI) UnbanPlayer(c *gin.Context) {
	accountID := c.Param("account_id")
	var request confirmRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		Error(c, 4000, "请求格式错误")
		return
	}
	if strings.TrimSpace(request.Confirmation) != "解封" {
		Error(c, 4000, "请输入「解封」以确认操作")
		return
	}
	var account models.PlayerAccount
	if err := api.DB.First(&account, "id = ?", accountID).Error; err != nil {
		Error(c, 4040, "玩家不存在")
		return
	}
	api.auditedMutation(c, "unban_player", "player", accountID, request.Reason, gin.H{"banned_at": account.BannedAt}, func(tx *gorm.DB) (interface{}, error) {
		if err := tx.Exec("UPDATE player_accounts SET banned_at = NULL, ban_expires_at = NULL, ban_reason = '' WHERE id = ?", accountID).Error; err != nil {
			return nil, err
		}
		return gin.H{"banned": false}, nil
	})
}

type reasonRequest struct {
	Reason string `json:"reason"`
}
type confirmRequest struct {
	Reason       string `json:"reason"`
	Confirmation string `json:"confirmation"`
}

const seasonTokenCurrencyKey = "season_token"

// ResetSeason removes only season-scoped player state. Permanent pets,
// equipment, codex, blueprints, adventure level and map progress are kept.
func (api *EcosystemAPI) ResetSeason(c *gin.Context) {
	eventKey := strings.TrimSpace(c.Param("event_key"))
	var request struct {
		SeasonKey    string `json:"season_key"`
		Reason       string `json:"reason"`
		Confirmation string `json:"confirmation"`
	}
	if err := c.ShouldBindJSON(&request); err != nil || eventKey == "" {
		Error(c, 4000, "请求格式错误")
		return
	}
	request.SeasonKey = strings.TrimSpace(request.SeasonKey)
	if request.SeasonKey == "" {
		request.SeasonKey = eventKey
	}
	if request.Confirmation != "重置赛季:"+eventKey {
		Error(c, 4000, "确认文字不正确")
		return
	}
	var event models.LiveEventConfig
	if err := api.DB.First(&event, "key = ?", eventKey).Error; err != nil {
		Error(c, 4040, "赛季活动不存在")
		return
	}
	before := gin.H{"event_key": eventKey, "season_key": request.SeasonKey}
	api.auditedMutation(c, "reset_season", "live_event", eventKey, request.Reason, before, func(tx *gorm.DB) (interface{}, error) {
		now := time.Now()
		var wallets []models.PlayerWallet
		if err := tx.Where("currency_key = ? AND balance <> 0", seasonTokenCurrencyKey).Find(&wallets).Error; err != nil {
			return nil, err
		}
		for _, wallet := range wallets {
			ledger := models.WalletLedger{
				ID: uuid.NewString(), AccountID: wallet.AccountID, CurrencyKey: wallet.CurrencyKey,
				Delta: -wallet.Balance, BalanceAfter: 0, Reason: "season_reset", ReferenceKey: eventKey, CreatedAt: now,
			}
			if err := tx.Create(&ledger).Error; err != nil {
				return nil, err
			}
		}
		if err := tx.Model(&models.PlayerWallet{}).Where("currency_key = ?", seasonTokenCurrencyKey).Updates(map[string]any{"balance": 0, "updated_at": now}).Error; err != nil {
			return nil, err
		}
		deleted := map[string]int64{}
		for key, result := range map[string]*gorm.DB{
			"event_progress": tx.Where("event_key = ?", eventKey).Delete(&models.EventProgress{}),
			"event_grants":   tx.Where("event_key = ?", eventKey).Delete(&models.EventProgressGrant{}),
			"event_claims":   tx.Where("event_key = ?", eventKey).Delete(&models.EventRewardClaim{}),
			"season_votes":   tx.Where("season_key = ?", request.SeasonKey).Delete(&models.SeasonVote{}),
		} {
			if result.Error != nil {
				return nil, result.Error
			}
			deleted[key] = result.RowsAffected
		}
		return gin.H{"event_key": eventKey, "season_key": request.SeasonKey, "wallets_reset": len(wallets), "deleted": deleted, "permanent_progress_preserved": true}, nil
	})
}

func (api *EcosystemAPI) SetPlayerNotifications(c *gin.Context) {
	accountID := c.Param("account_id")
	var request struct {
		Enabled bool   `json:"enabled"`
		Reason  string `json:"reason"`
	}
	if err := c.ShouldBindJSON(&request); err != nil {
		Error(c, 4000, "请求格式错误")
		return
	}
	var before models.NotificationPreference
	if err := api.DB.First(&before, "account_id = ?", accountID).Error; errors.Is(err, gorm.ErrRecordNotFound) {
		before = models.NotificationPreference{AccountID: accountID, Enabled: true}
	}
	api.auditedMutation(c, "set_notifications", "player", accountID, request.Reason, before, func(tx *gorm.DB) (interface{}, error) {
		var count int64
		if err := tx.Model(&models.PlayerAccount{}).Where("id = ?", accountID).Count(&count).Error; err != nil || count == 0 {
			return nil, gorm.ErrRecordNotFound
		}
		preference := models.NotificationPreference{AccountID: accountID, Enabled: request.Enabled, UpdatedAt: time.Now()}
		if err := tx.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "account_id"}}, DoUpdates: clause.Assignments(map[string]interface{}{"enabled": request.Enabled, "updated_at": time.Now()})}).Create(&preference).Error; err != nil {
			return nil, err
		}
		return preference, nil
	})
}

func (api *EcosystemAPI) DeleteIdentity(c *gin.Context) {
	accountID, identityID := c.Param("account_id"), c.Param("identity_id")
	var request reasonRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		Error(c, 4000, "请求格式错误")
		return
	}
	var identity models.PlayerIdentity
	if err := api.DB.First(&identity, "id = ? AND account_id = ?", identityID, accountID).Error; err != nil {
		Error(c, 4040, "身份不存在")
		return
	}
	before := gin.H{"identity_id": identity.ID, "platform": identity.Platform, "scene_type": identity.SceneType, "subject_id": maskIdentifier(identity.SubjectID)}
	api.auditedMutation(c, "unbind_identity", "player_identity", identityID, request.Reason, before, func(tx *gorm.DB) (interface{}, error) {
		var count int64
		if err := tx.Model(&models.PlayerIdentity{}).Where("account_id = ?", accountID).Count(&count).Error; err != nil {
			return nil, err
		}
		if count <= 1 {
			return nil, errors.New("不能解绑玩家的最后一个身份")
		}
		if result := tx.Delete(&models.PlayerIdentity{}, identity.ID); result.Error != nil || result.RowsAffected != 1 {
			if result.Error != nil {
				return nil, result.Error
			}
			return nil, gorm.ErrRecordNotFound
		}
		return gin.H{"deleted": true, "identity_id": identity.ID}, nil
	})
}

func lastSix(value string) string {
	runes := []rune(value)
	if len(runes) <= 6 {
		return value
	}
	return string(runes[len(runes)-6:])
}

func (api *EcosystemAPI) DeletePlayer(c *gin.Context) {
	accountID := c.Param("account_id")
	var request confirmRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		Error(c, 4000, "请求格式错误")
		return
	}
	if request.Confirmation != lastSix(accountID) {
		Error(c, 4000, "确认文字应为内部账号末六位")
		return
	}
	var account models.PlayerAccount
	if err := api.DB.First(&account, "id = ?", accountID).Error; err != nil {
		Error(c, 4040, "玩家不存在")
		return
	}
	api.auditedMutation(c, "delete_player", "player", accountID, request.Reason, gin.H{"account_id": accountID}, func(tx *gorm.DB) (interface{}, error) {
		var squads []models.ExpeditionSquad
		if err := tx.Where("leader_id = ?", accountID).Find(&squads).Error; err != nil {
			return nil, err
		}
		for _, squad := range squads {
			if err := tx.Where("squad_id = ?", squad.ID).Delete(&models.SquadMember{}).Error; err != nil {
				return nil, err
			}
		}
		accountModels := []interface{}{&models.PlayerIdentity{}, &models.PetProfile{}, &models.GlobalInventoryItem{}, &models.PlayerWallet{}, &models.WalletLedger{}, &models.AdventureShopPurchase{}, &models.PlayerEquipment{}, &models.PlayerBlueprintProgress{}, &models.CompanionJournal{}, &models.CompanionActionDaily{}, &models.ActivityRun{}, &models.ItemUseRecord{}, &models.ExpeditionRun{}, &models.EventProgress{}, &models.EventProgressGrant{}, &models.EventRewardClaim{}, &models.ChanceDailyState{}, &models.ChancePlayerState{}, &models.ChanceOutcome{}, &models.FishingRun{}, &models.BattleRecord{}, &models.TradeAudit{}, &models.CodexEntry{}, &models.CommunityMember{}, &models.SquadMember{}, &models.IdentityBindToken{}, &models.NotificationJob{}, &models.NotificationPreference{}, &models.BossContribution{}, &models.SeasonVote{}, &models.PetBehaviorProfile{}}
		for _, model := range accountModels {
			if err := tx.Where("account_id = ?", accountID).Delete(model).Error; err != nil {
				return nil, err
			}
		}
		if err := tx.Where("seller_account_id = ? OR buyer_account_id = ?", accountID, accountID).Delete(&models.TradeOffer{}).Error; err != nil {
			return nil, err
		}
		if err := tx.Where("donor_id = ?", accountID).Delete(&models.HelpGiftLog{}).Error; err != nil {
			return nil, err
		}
		if err := tx.Where("donor_id = ?", accountID).Delete(&models.HelpGiftDailyQuota{}).Error; err != nil {
			return nil, err
		}
		if err := tx.Where("requester_id = ?", accountID).Delete(&models.CommunityHelpRequest{}).Error; err != nil {
			return nil, err
		}
		if err := tx.Where("leader_id = ?", accountID).Delete(&models.ExpeditionSquad{}).Error; err != nil {
			return nil, err
		}
		if result := tx.Delete(&models.PlayerAccount{}, "id = ?", accountID); result.Error != nil || result.RowsAffected != 1 {
			if result.Error != nil {
				return nil, result.Error
			}
			return nil, gorm.ErrRecordNotFound
		}
		return gin.H{"deleted": true}, nil
	})
}

func (api *EcosystemAPI) CancelExpedition(c *gin.Context) {
	id := c.Param("id")
	var request struct {
		Reason         string `json:"reason"`
		ExpectedStatus string `json:"expected_status"`
	}
	if err := c.ShouldBindJSON(&request); err != nil {
		Error(c, 4000, "请求格式错误")
		return
	}
	if request.ExpectedStatus != "running" {
		Error(c, 4000, "expected_status 必须为 running")
		return
	}
	var before models.ExpeditionRun
	if err := api.DB.First(&before, "id = ?", id).Error; err != nil {
		Error(c, 4040, "远征不存在")
		return
	}
	api.auditedMutation(c, "cancel_expedition", "expedition", id, request.Reason, before, func(tx *gorm.DB) (interface{}, error) {
		result := tx.Model(&models.ExpeditionRun{}).Where("id = ? AND status = ?", id, request.ExpectedStatus).Update("status", "cancelled")
		if result.Error != nil {
			return nil, result.Error
		}
		if result.RowsAffected != 1 {
			return nil, errConcurrentChange
		}
		var run models.ExpeditionRun
		if err := tx.First(&run, "id = ?", id).Error; err != nil {
			return nil, err
		}
		return gin.H{"expedition": run, "refunded": []interface{}{}}, nil
	})
}

func (api *EcosystemAPI) ReconcileExpedition(c *gin.Context) {
	id := c.Param("id")
	var request reasonRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		Error(c, 4000, "请求格式错误")
		return
	}
	var before models.ExpeditionRun
	if err := api.DB.First(&before, "id = ?", id).Error; err != nil {
		Error(c, 4040, "远征不存在")
		return
	}
	api.auditedMutation(c, "reconcile_expedition", "expedition", id, request.Reason, before, func(tx *gorm.DB) (interface{}, error) {
		var run models.ExpeditionRun
		if err := tx.First(&run, "id = ?", id).Error; err != nil {
			return nil, err
		}
		if run.Status != "running" || time.Now().Before(run.EndsAt) {
			return nil, errors.New("仅允许结算已到期且仍在进行中的远征")
		}
		now := time.Now()
		result := tx.Model(&models.ExpeditionRun{}).Where("id = ? AND status = ?", id, "running").Updates(map[string]interface{}{"status": "claimed", "claimed_at": now})
		if result.Error != nil {
			return nil, result.Error
		}
		if result.RowsAffected != 1 {
			return nil, errConcurrentChange
		}
		for itemName, quantity := range map[string]int64{run.RewardItem: run.RewardQuantity, "调查记录": run.RewardRecords} {
			if quantity <= 0 {
				continue
			}
			if err := creditUnifiedItemTx(tx, run.AccountID, itemName, quantity, now); err != nil {
				return nil, err
			}
		}
		petQuery := tx.Model(&models.PetProfile{}).Where("account_id = ?", run.AccountID)
		if strings.TrimSpace(run.PetID) != "" {
			petQuery = petQuery.Where("id = ?", run.PetID)
		}
		if err := petQuery.UpdateColumn("growth", gorm.Expr("growth + ?", run.RewardGrowth)).Error; err != nil {
			return nil, err
		}
		if err := tx.First(&run, "id = ?", id).Error; err != nil {
			return nil, err
		}
		return gin.H{"expedition": run, "reward": gin.H{"item_name": run.RewardItem, "quantity": run.RewardQuantity, "records": run.RewardRecords, "growth": run.RewardGrowth}}, nil
	})
}

func (api *EcosystemAPI) UpdateFacility(c *gin.Context) {
	communityID, facilityID := c.Param("id"), c.Param("facility_id")
	var request struct {
		Level             int       `json:"level"`
		Progress          int64     `json:"progress"`
		Reason            string    `json:"reason"`
		ExpectedUpdatedAt time.Time `json:"expected_updated_at"`
	}
	if err := c.ShouldBindJSON(&request); err != nil {
		Error(c, 4000, "请求格式错误")
		return
	}
	if request.Level < 1 || request.Progress < 0 || request.ExpectedUpdatedAt.IsZero() {
		Error(c, 4000, "等级、进度或版本时间无效")
		return
	}
	var before models.CommunityFacility
	if err := api.DB.First(&before, "id = ? AND community_id = ?", facilityID, communityID).Error; err != nil {
		Error(c, 4040, "设施不存在")
		return
	}
	api.auditedMutation(c, "update_facility", "community_facility", facilityID, request.Reason, before, func(tx *gorm.DB) (interface{}, error) {
		now := time.Now()
		result := tx.Model(&models.CommunityFacility{}).Where("id = ? AND community_id = ? AND updated_at = ?", facilityID, communityID, request.ExpectedUpdatedAt).Updates(map[string]interface{}{"level": request.Level, "progress": request.Progress, "updated_at": now})
		if result.Error != nil {
			return nil, result.Error
		}
		if result.RowsAffected != 1 {
			return nil, errConcurrentChange
		}
		var after models.CommunityFacility
		if err := tx.First(&after, "id = ?", facilityID).Error; err != nil {
			return nil, err
		}
		return after, nil
	})
}

func (api *EcosystemAPI) SetCommunityNotifications(c *gin.Context) {
	id := c.Param("id")
	var request struct {
		Enabled bool   `json:"enabled"`
		Reason  string `json:"reason"`
	}
	if err := c.ShouldBindJSON(&request); err != nil {
		Error(c, 4000, "请求格式错误")
		return
	}
	var before models.Community
	if err := api.DB.First(&before, "id = ?", id).Error; err != nil {
		Error(c, 4040, "社区不存在")
		return
	}
	api.auditedMutation(c, "set_community_notifications", "community", id, request.Reason, before, func(tx *gorm.DB) (interface{}, error) {
		if result := tx.Model(&models.Community{}).Where("id = ?", id).Update("notifications_enabled", request.Enabled); result.Error != nil || result.RowsAffected != 1 {
			if result.Error != nil {
				return nil, result.Error
			}
			return nil, gorm.ErrRecordNotFound
		}
		var after models.Community
		if err := tx.First(&after, "id = ?", id).Error; err != nil {
			return nil, err
		}
		return after, nil
	})
}

func (api *EcosystemAPI) ResetBoss(c *gin.Context) {
	communityID := c.Param("id")
	var request confirmRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		Error(c, 4000, "请求格式错误")
		return
	}
	if request.Confirmation != "重置首领" {
		Error(c, 4000, "请输入“重置首领”确认")
		return
	}
	var boss models.CommunityBoss
	if err := api.DB.Where("community_id = ?", communityID).Order("updated_at DESC").First(&boss).Error; err != nil {
		Error(c, 4040, "当前社区没有首领记录")
		return
	}
	api.auditedMutation(c, "reset_boss", "community", communityID, request.Reason, boss, func(tx *gorm.DB) (interface{}, error) {
		if err := tx.Where("boss_id = ?", boss.ID).Delete(&models.BossContribution{}).Error; err != nil {
			return nil, err
		}
		boss.CurrentHP, boss.Defeated, boss.UpdatedAt = boss.MaxHP, false, time.Now()
		if err := tx.Save(&boss).Error; err != nil {
			return nil, err
		}
		return boss, nil
	})
}

func (api *EcosystemAPI) CloseHelpRequest(c *gin.Context) {
	code := c.Param("code")
	var request reasonRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		Error(c, 4000, "请求格式错误")
		return
	}
	var before models.CommunityHelpRequest
	if err := api.DB.First(&before, "code = ?", code).Error; err != nil {
		Error(c, 4040, "求助单不存在")
		return
	}
	api.auditedMutation(c, "close_help_request", "help_request", code, request.Reason, before, func(tx *gorm.DB) (interface{}, error) {
		result := tx.Model(&models.CommunityHelpRequest{}).Where("code = ? AND status = ?", code, "open").Updates(map[string]interface{}{"status": "closed", "updated_at": time.Now()})
		if result.Error != nil {
			return nil, result.Error
		}
		if result.RowsAffected != 1 {
			return nil, errConcurrentChange
		}
		var after models.CommunityHelpRequest
		if err := tx.First(&after, "code = ?", code).Error; err != nil {
			return nil, err
		}
		return after, nil
	})
}

func (api *EcosystemAPI) DisbandSquad(c *gin.Context) {
	id := c.Param("id")
	var request confirmRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		Error(c, 4000, "请求格式错误")
		return
	}
	if request.Confirmation != "解散小队" {
		Error(c, 4000, "请输入“解散小队”确认")
		return
	}
	var before models.ExpeditionSquad
	if err := api.DB.First(&before, "id = ?", id).Error; err != nil {
		Error(c, 4040, "小队不存在")
		return
	}
	api.auditedMutation(c, "disband_squad", "squad", id, request.Reason, before, func(tx *gorm.DB) (interface{}, error) {
		if err := tx.Where("squad_id = ?", id).Delete(&models.SquadMember{}).Error; err != nil {
			return nil, err
		}
		if result := tx.Delete(&models.ExpeditionSquad{}, "id = ?", id); result.Error != nil || result.RowsAffected != 1 {
			if result.Error != nil {
				return nil, result.Error
			}
			return nil, gorm.ErrRecordNotFound
		}
		return gin.H{"disbanded": true}, nil
	})
}
