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
	if err = db.AutoMigrate(&models.PlayerAccount{}, &models.PlayerIdentity{}, &models.PetProfile{}, &models.GlobalInventoryItem{}, &models.ExpeditionRun{}, &models.CodexEntry{}, &models.Community{}, &models.CommunityMember{}, &models.ExpeditionSquad{}, &models.SquadMember{}, &models.IdentityBindToken{}, &models.NotificationPreference{}, &models.CommunityBoss{}, &models.BossContribution{}, &models.CommunityFacility{}, &models.SeasonVote{}, &models.CommunityHelpRequest{}, &models.HelpGiftLog{}, &models.PetBehaviorProfile{}, &models.GameplayMetric{}, &models.AdminAuditLog{}, &models.AdminOperationKey{}, &models.GrowthRoleConfig{}, &models.GrowthStanceConfig{}, &models.PersonalityRuleConfig{}, &models.CodexCatalogConfig{}, &models.ItemConfig{}); err != nil {
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
}
