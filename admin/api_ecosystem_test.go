package admin

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"qq-pet-saas/models"
)

func newEcosystemTestRouter(t *testing.T) (*gin.Engine, *gorm.DB) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err = db.AutoMigrate(&models.PlayerAccount{}, &models.PlayerIdentity{}, &models.PetProfile{}, &models.GlobalInventoryItem{}, &models.PlayerWallet{}, &models.WalletLedger{}, &models.CompanionActionDaily{}, &models.ActivityRun{}, &models.ItemUseRecord{}, &models.ExpeditionRun{}, &models.EventProgress{}, &models.EventProgressGrant{}, &models.EventRewardClaim{}, &models.LiveEventConfig{}, &models.ChanceDailyState{}, &models.ChancePlayerState{}, &models.ChanceOutcome{}, &models.FishingRun{}, &models.BattleRecord{}, &models.TradeOffer{}, &models.TradeAudit{}, &models.CodexEntry{}, &models.Community{}, &models.CommunityMember{}, &models.ExpeditionSquad{}, &models.SquadMember{}, &models.IdentityBindToken{}, &models.NotificationPreference{}, &models.NotificationJob{}, &models.CommunityBoss{}, &models.BossContribution{}, &models.CommunityFacility{}, &models.SeasonVote{}, &models.CommunityHelpRequest{}, &models.HelpGiftLog{}, &models.HelpGiftDailyQuota{}, &models.PetBehaviorProfile{}, &models.GameplayMetric{}, &models.AdminAuditLog{}, &models.AdminOperationKey{}, &models.GrowthRoleConfig{}, &models.GrowthStanceConfig{}, &models.PersonalityRuleConfig{}, &models.CodexCatalogConfig{}, &models.ItemConfig{}, &models.PetSpeciesConfig{}); err != nil {
		t.Fatal(err)
	}
	if err = db.AutoMigrate(&models.AdminConfigState{}, &models.AdventureMapConfig{}, &models.AdventureZoneConfig{}, &models.AdventureZonePrerequisiteConfig{}, &models.AdventureObjectiveConfig{}, &models.AdventureExplorationStageConfig{}, &models.AdventureStoryEventConfig{}, &models.AdventureStoryEventChoiceConfig{}, &models.AdventureMonsterConfig{}, &models.AdventureSkillConfig{}, &models.AdventureMonsterSkillConfig{}, &models.AdventureEncounterConfig{}, &models.AdventureLootPoolConfig{}, &models.AdventureLootEntryConfig{}, &models.CurrencyConfig{}, &models.ItemConfig{}, &models.AdventureShopItemConfig{}, &models.AdventureExpeditionConfig{}, &models.AdventureBossConfig{}, &models.AdventureBossRewardTierConfig{}, &models.EquipmentTemplateConfig{}, &models.EquipmentAffixConfig{}, &models.EquipmentRecipeConfig{}, &models.EquipmentRecipeMaterialConfig{}, &models.AdventureShopPurchase{}, &models.PlayerAdventureProgress{}, &models.PlayerZoneProgress{}, &models.PlayerObjectiveProgress{}, &models.PlayerAdventureNodeProgress{}, &models.PlayerAdventureEventState{}, &models.AdventureExplorationSession{}, &models.AdventureCombatSession{}, &models.AdventureCombatTurn{}, &models.PlayerEquipment{}, &models.PlayerBlueprintProgress{}, &models.AdventureExpeditionRun{}, &models.AdventureBossInstance{}, &models.AdventureBossContribution{}, &models.AdventureBossRewardClaim{}, &models.EquipmentCraftRecord{}); err != nil {
		t.Fatal(err)
	}
	router := gin.New()
	RegisterEcosystemRoutes(router.Group("/api/admin"), db)
	return router, db
}

func TestGrowthEndpointReturnsRulesAndZeroSummaryWithoutPlayers(t *testing.T) {
	router, _ := newEcosystemTestRouter(t)
	response := doConfigRequest(t, router, http.MethodGet, "/api/admin/gameplay/growth", nil)
	if response.Code != 0 {
		t.Fatalf("growth endpoint failed: %s", response.Msg)
	}
	var data struct {
		Summary struct {
			PlayerCount              int64   `json:"player_count"`
			RoleCoverageRate         float64 `json:"role_coverage_rate"`
			PersonalityFormationRate float64 `json:"personality_formation_rate"`
			ConfiguredRuleCount      int     `json:"configured_rule_count"`
		} `json:"summary"`
		Roles         []json.RawMessage `json:"roles"`
		Stances       []json.RawMessage `json:"stances"`
		Personalities []json.RawMessage `json:"personalities"`
		Skills        []json.RawMessage `json:"skills"`
		Warnings      []string          `json:"warnings"`
	}
	if err := json.Unmarshal(response.Data, &data); err != nil {
		t.Fatal(err)
	}
	if data.Summary.PlayerCount != 0 || data.Summary.RoleCoverageRate != 0 || data.Summary.PersonalityFormationRate != 0 {
		t.Fatalf("zero-player summary is incorrect: %+v", data.Summary)
	}
	if data.Summary.ConfiguredRuleCount == 0 || len(data.Roles) == 0 || len(data.Stances) == 0 || len(data.Personalities) == 0 || data.Skills == nil || data.Warnings == nil {
		t.Fatalf("growth response must contain editable rules and non-null arrays: %s", response.Data)
	}
}

func TestCodexEndpointUsesCatalogWhenNoPlayerHasProgress(t *testing.T) {
	router, db := newEcosystemTestRouter(t)
	if err := db.Create(&models.CodexCatalogConfig{Category: "鱼类", EntryKey: "银鳞鱼", Region: "水域", Description: "浅水常见鱼类", Enabled: true}).Error; err != nil {
		t.Fatal(err)
	}
	response := doConfigRequest(t, router, http.MethodGet, "/api/admin/gameplay/codex?region="+url.QueryEscape("水域"), nil)
	if response.Code != 0 {
		t.Fatalf("codex endpoint failed: %s", response.Msg)
	}
	var data struct {
		Items []struct {
			EntryKey          string  `json:"entry_key"`
			DiscoveredPlayers int64   `json:"discovered_players"`
			CompletedPlayers  int64   `json:"completed_players"`
			AverageProgress   float64 `json:"average_progress"`
		} `json:"items"`
	}
	if err := json.Unmarshal(response.Data, &data); err != nil {
		t.Fatal(err)
	}
	if len(data.Items) != 1 || data.Items[0].EntryKey != "银鳞鱼" || data.Items[0].DiscoveredPlayers != 0 || data.Items[0].CompletedPlayers != 0 || data.Items[0].AverageProgress != 0 {
		t.Fatalf("catalog entry missing from zero-player response: %s", response.Data)
	}
}

func TestGrantItemIsIdempotentAndAudited(t *testing.T) {
	router, db := newEcosystemTestRouter(t)
	accountID := "00000000-0000-0000-0000-000000123456"
	db.Create(&models.PlayerAccount{ID: accountID})
	body := []byte(`{"item_name":"调查记录","quantity":10,"reason":"补偿异常结算","idempotency_key":"grant-test-123456"}`)
	for index := 0; index < 2; index++ {
		response := doConfigRequest(t, router, http.MethodPost, "/api/admin/players/"+accountID+"/grants", body)
		if response.Code != 0 {
			t.Fatalf("grant failed: %s", response.Msg)
		}
	}
	var item models.GlobalInventoryItem
	if err := db.First(&item, "account_id = ? AND item_name = ?", accountID, "调查记录").Error; err != nil || item.Quantity != 10 {
		t.Fatalf("expected idempotent quantity 10, got %+v err=%v", item, err)
	}
	var auditCount int64
	db.Model(&models.AdminAuditLog{}).Where("action = ? AND success = ?", "grant_item", true).Count(&auditCount)
	if auditCount != 1 {
		t.Fatalf("expected one successful audit, got %d", auditCount)
	}
}

func TestGrantItemRejectsUnavailableConfiguredItem(t *testing.T) {
	router, db := newEcosystemTestRouter(t)
	accountID := "00000000-0000-0000-0000-000000123457"
	db.Create(&models.PlayerAccount{ID: accountID})
	db.Create(&models.ItemConfig{Name: "故障物品", Status: "disabled"})

	response := doConfigRequest(t, router, http.MethodPost, "/api/admin/players/"+accountID+"/grants", []byte(`{"item_name":"故障物品","quantity":1,"reason":"验证状态限制","idempotency_key":"grant-disabled-123"}`))
	if response.Code == 0 {
		t.Fatal("expected disabled item grant to be rejected")
	}

	var count int64
	db.Model(&models.GlobalInventoryItem{}).Where("account_id = ? AND item_name = ?", accountID, "故障物品").Count(&count)
	if count != 0 {
		t.Fatalf("disabled item must not be granted, got %d inventory rows", count)
	}
}

func TestBanPlayerRequiresConfirmationAndBlocksUntilUnban(t *testing.T) {
	router, db := newEcosystemTestRouter(t)
	accountID := "00000000-0000-0000-0000-000000123470"
	if err := db.Create(&models.PlayerAccount{ID: accountID}).Error; err != nil {
		t.Fatal(err)
	}
	response := doConfigRequest(t, router, http.MethodPost, "/api/admin/players/"+accountID+"/ban", []byte(`{"reason":"刷奖","confirmation":"错词"}`))
	if response.Code != 4000 {
		t.Fatalf("错确认词应拒绝，code=%d msg=%s", response.Code, response.Msg)
	}
	response = doConfigRequest(t, router, http.MethodPost, "/api/admin/players/"+accountID+"/ban", []byte(`{"reason":"刷奖","confirmation":"封禁"}`))
	if response.Code != 0 {
		t.Fatalf("封禁失败: %s", response.Msg)
	}
	var account models.PlayerAccount
	if err := db.First(&account, "id = ?", accountID).Error; err != nil || account.BannedAt == nil {
		t.Fatalf("封禁未写入: %+v %v", account, err)
	}
	detail := doConfigRequest(t, router, http.MethodGet, "/api/admin/players/"+accountID, nil)
	if detail.Code != 0 || !strings.Contains(string(detail.Data), `"banned":true`) {
		t.Fatalf("详情应标记 banned: %s", detail.Data)
	}
	response = doConfigRequest(t, router, http.MethodPost, "/api/admin/players/"+accountID+"/unban", []byte(`{"reason":"误封","confirmation":"解封"}`))
	if response.Code != 0 {
		t.Fatalf("解封失败: %s", response.Msg)
	}
	var after models.PlayerAccount
	if err := db.First(&after, "id = ?", accountID).Error; err != nil || after.BannedAt != nil {
		t.Fatalf("解封后 BannedAt 应清空: %+v %v", after, err)
	}
}

func TestGrantCurrencyIsIdempotentAndAudited(t *testing.T) {
	router, db := newEcosystemTestRouter(t)
	accountID := "00000000-0000-0000-0000-000000123458"
	if err := db.Create(&models.PlayerAccount{ID: accountID}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&models.CurrencyConfig{Key: "primary_coin", Name: "星砂", Enabled: true, Builtin: true}).Error; err != nil {
		t.Fatal(err)
	}
	body := []byte(`{"currency_key":"primary_coin","amount":88,"direction":"grant","reason":"补偿漏发签到","idempotency_key":"currency-grant-123456"}`)
	for range 2 {
		response := doConfigRequest(t, router, http.MethodPost, "/api/admin/players/"+accountID+"/currency", body)
		if response.Code != 0 {
			t.Fatalf("currency grant failed: %s", response.Msg)
		}
	}
	var wallet models.PlayerWallet
	if err := db.First(&wallet, "account_id = ? AND currency_key = ?", accountID, "primary_coin").Error; err != nil || wallet.Balance != 88 {
		t.Fatalf("expected idempotent balance 88, got %+v err=%v", wallet, err)
	}
	var auditCount int64
	db.Model(&models.AdminAuditLog{}).Where("action = ? AND success = ?", "grant_currency", true).Count(&auditCount)
	if auditCount != 1 {
		t.Fatalf("expected one successful currency grant audit, got %d", auditCount)
	}
}

func TestDebitCurrencyRejectsInsufficientBalance(t *testing.T) {
	router, db := newEcosystemTestRouter(t)
	accountID := "00000000-0000-0000-0000-000000123459"
	if err := db.Create(&models.PlayerAccount{ID: accountID}).Error; err != nil {
		t.Fatal(err)
	}
	body := []byte(`{"currency_key":"primary_coin","amount":10,"direction":"debit","reason":"回滚误发","idempotency_key":"currency-debit-123456"}`)
	response := doConfigRequest(t, router, http.MethodPost, "/api/admin/players/"+accountID+"/currency", body)
	if response.Code == 0 {
		t.Fatal("expected insufficient debit to be rejected")
	}
	var wallet models.PlayerWallet
	lookup := db.Limit(1).Find(&wallet, "account_id = ? AND currency_key = ?", accountID, "primary_coin")
	if lookup.Error != nil {
		t.Fatal(lookup.Error)
	}
	if lookup.RowsAffected == 1 && wallet.Balance != 0 {
		t.Fatalf("failed debit must not change balance: %+v", wallet)
	}
}

func TestGrantCurrencyRejectsUnknownKey(t *testing.T) {
	router, db := newEcosystemTestRouter(t)
	accountID := "00000000-0000-0000-0000-000000123460"
	if err := db.Create(&models.PlayerAccount{ID: accountID}).Error; err != nil {
		t.Fatal(err)
	}
	response := doConfigRequest(t, router, http.MethodPost, "/api/admin/players/"+accountID+"/currency", []byte(`{"currency_key":"not_a_wallet","amount":1,"direction":"grant","reason":"验证未知货币","idempotency_key":"currency-unknown-123"}`))
	if response.Code == 0 {
		t.Fatal("unknown currency key must be rejected")
	}
}

func TestCannotDeleteLastIdentityAndFailureIsAudited(t *testing.T) {
	router, db := newEcosystemTestRouter(t)
	accountID := "00000000-0000-0000-0000-000000654321"
	db.Create(&models.PlayerAccount{ID: accountID})
	identity := models.PlayerIdentity{AccountID: accountID, Platform: "qq_group", AppID: "app", SceneType: "group", ScopeID: "scope-secret", SubjectID: "openid-secret"}
	db.Create(&identity)
	response := doConfigRequest(t, router, http.MethodDelete, "/api/admin/players/"+accountID+"/identities/1", []byte(`{"reason":"测试唯一身份保护"}`))
	if response.Code == 0 {
		t.Fatal("expected last identity deletion to be rejected")
	}
	var audit models.AdminAuditLog
	if err := db.Where("action = ?", "unbind_identity").First(&audit).Error; err != nil || audit.Success {
		t.Fatalf("expected failed audit, got %+v err=%v", audit, err)
	}
	if strings.Contains(audit.BeforeJSON, "openid-secret") {
		t.Fatal("audit leaked full OpenID")
	}
}

func TestReconcileOverdueExpeditionOnlyAwardsOnce(t *testing.T) {
	router, db := newEcosystemTestRouter(t)
	accountID := "00000000-0000-0000-0000-000000111111"
	db.Create(&models.PlayerAccount{ID: accountID})
	db.Create(&models.PetProfile{AccountID: accountID, Name: "米塔", PetType: "猫", Role: "探索者", Stance: "守护"})
	db.Create(&models.ExpeditionRun{ID: "exp-1", AccountID: accountID, Tier: 2, Name: "遗迹调查", Stance: "守护", Status: "running", RewardItem: "古代零件", RewardQuantity: 3, RewardRecords: 12, RewardGrowth: 8, StartedAt: time.Now().Add(-3 * time.Hour), EndsAt: time.Now().Add(-time.Hour)})
	response := doConfigRequest(t, router, http.MethodPost, "/api/admin/expeditions/exp-1/reconcile", []byte(`{"reason":"处理到期未结算"}`))
	if response.Code != 0 {
		t.Fatalf("reconcile failed: %s", response.Msg)
	}
	response = doConfigRequest(t, router, http.MethodPost, "/api/admin/expeditions/exp-1/reconcile", []byte(`{"reason":"重复请求验证"}`))
	if response.Code == 0 {
		t.Fatal("expected second reconcile to fail")
	}
	var item models.GlobalInventoryItem
	db.First(&item, "account_id = ? AND item_name = ?", accountID, "古代零件")
	if item.Quantity != 3 {
		t.Fatalf("expected reward once, got %d", item.Quantity)
	}
}

func TestPlayerDetailMasksOfficialIdentifiers(t *testing.T) {
	router, db := newEcosystemTestRouter(t)
	accountID := "00000000-0000-0000-0000-000000999999"
	db.Create(&models.PlayerAccount{ID: accountID})
	db.Create(&models.PlayerIdentity{AccountID: accountID, Platform: "qq_guild", AppID: "123456789", SceneType: "guild", ScopeID: "guild-openid-1234", SubjectID: "member-openid-9876"})
	response := doConfigRequest(t, router, http.MethodGet, "/api/admin/players/"+accountID, nil)
	if strings.Contains(string(response.Data), "member-openid-9876") || strings.Contains(string(response.Data), "guild-openid-1234") {
		t.Fatalf("detail leaked identifiers: %s", response.Data)
	}
	var payload map[string]interface{}
	if err := json.Unmarshal(response.Data, &payload); err != nil {
		t.Fatal(err)
	}
}

func TestEmptyPlayerCollectionsAreJSONArrays(t *testing.T) {
	router, db := newEcosystemTestRouter(t)
	accountID := "00000000-0000-0000-0000-000000000001"

	listResponse := doConfigRequest(t, router, http.MethodGet, "/api/admin/players", nil)
	if !strings.Contains(string(listResponse.Data), `"items":[]`) {
		t.Fatalf("empty player list must return [], got %s", listResponse.Data)
	}
	db.Create(&models.PlayerAccount{ID: accountID})
	detailResponse := doConfigRequest(t, router, http.MethodGet, "/api/admin/players/"+accountID, nil)
	for _, field := range []string{"inventory", "codex", "identities", "expeditions", "communities"} {
		if !strings.Contains(string(detailResponse.Data), `"`+field+`":[]`) {
			t.Fatalf("empty %s must return [], got %s", field, detailResponse.Data)
		}
	}
	if !strings.Contains(string(detailResponse.Data), `"notifications":{"AccountID":"`+accountID+`","Enabled":true`) {
		t.Fatalf("missing notification preference should expose the enabled default, got %s", detailResponse.Data)
	}
}

func TestSeasonResetClearsOnlySeasonScopedState(t *testing.T) {
	router, db := newEcosystemTestRouter(t)
	accountID := "season-player"
	now := time.Now()
	rows := []any{
		&models.PlayerAccount{ID: accountID},
		&models.LiveEventConfig{Key: "season-01", Name: "第一调查季", Region: "全域", StoryChoices: `[]`, StartsAt: now.Add(-time.Hour), EndsAt: now.Add(time.Hour), Active: true},
		&models.PlayerWallet{AccountID: accountID, CurrencyKey: "season_token", Balance: 77},
		&models.EventProgress{EventKey: "season-01", AccountID: accountID, Progress: 123},
		&models.EventProgressGrant{ID: "grant-1", EventKey: "season-01", AccountID: accountID, SourceKey: "test", Delta: 123},
		&models.EventRewardClaim{ID: "claim-1", EventKey: "season-01", AccountID: accountID, Milestone: 100, RewardType: "item", RewardKey: "season_memento", RewardName: "首季纪念叶", Quantity: 1},
		&models.SeasonVote{SeasonKey: "S01", CommunityID: "community", AccountID: accountID, Choice: 1},
		&models.CodexEntry{AccountID: accountID, Category: "遗迹", EntryKey: "永久条目", Progress: 100},
		&models.PlayerAdventureProgress{AccountID: accountID, Level: 8, XP: 99},
		&models.PlayerEquipment{ID: "gear-1", AccountID: accountID, TemplateKey: "permanent-gear"},
	}
	for _, row := range rows {
		if err := db.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}
	body := []byte(`{"season_key":"S01","reason":"第一调查季结算","confirmation":"重置赛季:season-01"}`)
	response := doConfigRequest(t, router, http.MethodPost, "/api/admin/seasons/season-01/reset", body)
	if response.Code != 0 {
		t.Fatalf("season reset failed: %s", response.Msg)
	}
	var wallet models.PlayerWallet
	if err := db.First(&wallet, "account_id = ? AND currency_key = ?", accountID, "season_token").Error; err != nil || wallet.Balance != 0 {
		t.Fatalf("season wallet not reset: %#v err=%v", wallet, err)
	}
	var ledger models.WalletLedger
	if err := db.First(&ledger, "account_id = ? AND reason = ?", accountID, "season_reset").Error; err != nil || ledger.Delta != -77 || ledger.BalanceAfter != 0 {
		t.Fatalf("season reset ledger missing: %#v err=%v", ledger, err)
	}
	for _, check := range []struct {
		model any
		where string
		args  []any
	}{
		{&models.EventProgress{}, "event_key = ?", []any{"season-01"}},
		{&models.EventProgressGrant{}, "event_key = ?", []any{"season-01"}},
		{&models.EventRewardClaim{}, "event_key = ?", []any{"season-01"}},
		{&models.SeasonVote{}, "season_key = ?", []any{"S01"}},
	} {
		var count int64
		if err := db.Model(check.model).Where(check.where, check.args...).Count(&count).Error; err != nil || count != 0 {
			t.Fatalf("season row survived reset: %T count=%d err=%v", check.model, count, err)
		}
	}
	for _, check := range []any{&models.CodexEntry{}, &models.PlayerAdventureProgress{}, &models.PlayerEquipment{}} {
		var count int64
		if err := db.Model(check).Where("account_id = ?", accountID).Count(&count).Error; err != nil || count != 1 {
			t.Fatalf("permanent progress was removed: %T count=%d err=%v", check, count, err)
		}
	}
}
