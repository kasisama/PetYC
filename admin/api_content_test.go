package admin

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"qq-pet-saas/models"
)

func newContentTestRouter(t *testing.T) (*gin.Engine, *gorm.DB) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err = db.AutoMigrate(&models.SystemConfig{}, &models.ItemConfig{}, &models.ShopItemConfig{}, &models.LiveEventConfig{}, &models.LiveEventChoiceConfig{}, &models.LiveEventExpeditionSourceConfig{}, &models.RewardTrackConfig{}, &models.GlobalInventoryItem{}, &models.AdminConfigState{}); err != nil {
		t.Fatal(err)
	}
	router := gin.New()
	RegisterContentRoutes(router.Group("/api/admin"), db)
	return router, db
}

func TestBulkRestockUsesPerProductTarget(t *testing.T) {
	router, db := newContentTestRouter(t)
	rows := []models.ShopItemConfig{
		{ShopType: "shop_normal", Name: "绷带", Stock: 1, RestockTarget: 50, Price: 10},
		{ShopType: "shop_normal", Name: "书本", Stock: 2, RestockTarget: 80, Price: 20},
	}
	db.Create(&rows)
	body, _ := json.Marshal(map[string]interface{}{"ids": []uint{rows[0].ID, rows[1].ID}, "action": "restock"})
	response := doConfigRequest(t, router, http.MethodPost, "/api/admin/content/shop-items/bulk", body)
	if response.Code != 0 {
		t.Fatalf("批量补货失败: %s", response.Msg)
	}
	var saved []models.ShopItemConfig
	db.Order("id").Find(&saved)
	if saved[0].Stock != 50 || saved[1].Stock != 80 {
		t.Fatalf("补货未恢复目标库存: %+v", saved)
	}
}

func TestBulkItemStatusAndReferencedDeleteProtection(t *testing.T) {
	router, db := newContentTestRouter(t)
	db.Create(&models.ItemConfig{Name: "冰之石", Status: "active"})
	db.Create(&models.ShopItemConfig{ShopType: "shop_normal", Name: "冰之石", Stock: 3, RestockTarget: 10, Price: 100})

	statusBody := []byte(`{"names":["冰之石"],"action":"set_status","status":"disabled"}`)
	response := doConfigRequest(t, router, http.MethodPost, "/api/admin/content/items/bulk", statusBody)
	if response.Code != 0 {
		t.Fatalf("批量停用失败: %s", response.Msg)
	}
	var item models.ItemConfig
	db.First(&item, "name = ?", "冰之石")
	if item.Status != "disabled" {
		t.Fatalf("物品状态未更新: %+v", item)
	}

	deleteBody := []byte(`{"names":["冰之石"],"action":"delete"}`)
	response = doConfigRequest(t, router, http.MethodPost, "/api/admin/content/items/bulk", deleteBody)
	if response.Code == 0 {
		t.Fatal("被商店引用的物品不应允许硬删除")
	}
	if err := db.First(&item, "name = ?", "冰之石").Error; err != nil {
		t.Fatalf("删除失败后物品应保留: %v", err)
	}
}

func TestGameSettingsParseHashSeparatedStarterPets(t *testing.T) {
	router, db := newContentTestRouter(t)
	db.Create(&models.SystemConfig{Key: "Core.InitialPets", Value: "光芽兽#苔须灵#烬爪兽"})
	response := doConfigRequest(t, router, http.MethodGet, "/api/admin/settings/game", nil)
	if response.Code != 0 {
		t.Fatalf("读取游戏参数失败: %s", response.Msg)
	}
	var rows []struct {
		Key   string      `json:"key"`
		Value interface{} `json:"value"`
	}
	if err := json.Unmarshal(response.Data, &rows); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("unexpected rows: %s", response.Data)
	}
	values, ok := rows[0].Value.([]interface{})
	if !ok || len(values) != 3 || values[0] != "光芽兽" || values[2] != "烬爪兽" {
		t.Fatalf("初始宠物未按列表解析: %+v", rows[0].Value)
	}
}

func TestGameSettingsUseChineseStructuredFieldsAndHideRetiredKeys(t *testing.T) {
	router, db := newContentTestRouter(t)
	db.Create(&models.SystemConfig{Key: "Core.CoinName", Value: "星砂"})
	db.Create(&models.SystemConfig{Key: "Core.CheckinLike", Value: "true"})
	db.Create(&models.SystemConfig{Key: "Interaction.LotteryItem", Value: "抽奖券"})
	response := doConfigRequest(t, router, http.MethodGet, "/api/admin/settings/game", nil)
	if response.Code != 0 {
		t.Fatalf("读取游戏参数失败: %s", response.Msg)
	}
	var rows []struct {
		Key   string      `json:"key"`
		Label string      `json:"label"`
		Type  string      `json:"type"`
		Value interface{} `json:"value"`
	}
	if err := json.Unmarshal(response.Data, &rows); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 {
		t.Fatalf("应隐藏下线玩法参数，实际返回 %s", response.Data)
	}
	for _, row := range rows {
		if row.Label == "" || row.Label == row.Key {
			t.Fatalf("参数必须提供中文业务标签: %+v", row)
		}
		if row.Key == "Core.CheckinLike" && (row.Type != "boolean" || row.Value != true) {
			t.Fatalf("布尔参数应返回布尔值而不是字符串: %+v", row)
		}
	}
}

func TestSaveEventBundlePersistsEventAndMultipleRewardsAtomically(t *testing.T) {
	router, db := newContentTestRouter(t)
	db.Create(&models.ItemConfig{Key: "wood", Name: "木材", Status: "active"})
	db.Create(&models.ItemConfig{Key: "survey_log", Name: "调查记录", Status: "active"})
	body := []byte(`{"event":{"key":"forest-week","name":"森林调查周","region":"森林","story_choices":"[\"记录线索\",\"继续调查\",\"呼叫支援\"]","starts_at":"2026-08-20T00:00:00Z","ends_at":"2026-08-27T00:00:00Z","active":true},"rewards":[{"event_key":"forest-week","milestone":100,"reward_type":"item","reward_key":"wood","reward_name":"木材","quantity":5},{"event_key":"forest-week","milestone":100,"reward_type":"item","reward_key":"survey_log","reward_name":"调查记录","quantity":2}]}`)
	response := doConfigRequest(t, router, http.MethodPut, "/api/admin/content/events/forest-week", body)
	if response.Code != 0 {
		t.Fatalf("保存活动组合失败: %s", response.Msg)
	}
	var eventCount, rewardCount int64
	db.Model(&models.LiveEventConfig{}).Where("key = ?", "forest-week").Count(&eventCount)
	db.Model(&models.RewardTrackConfig{}).Where("event_key = ?", "forest-week").Count(&rewardCount)
	if eventCount != 1 || rewardCount != 2 {
		t.Fatalf("活动与奖励未同时保存: event=%d reward=%d", eventCount, rewardCount)
	}
}

func TestSaveEventBundleRejectsOverlapWithExistingActiveEvent(t *testing.T) {
	router, db := newContentTestRouter(t)
	db.Create(&models.LiveEventConfig{
		Key: "existing-event", Name: "既有活动", Region: "森林", Active: true,
		StartsAt:     time.Date(2026, 8, 20, 0, 0, 0, 0, time.UTC),
		EndsAt:       time.Date(2026, 8, 22, 0, 0, 0, 0, time.UTC),
		StoryChoices: `["记录线索","继续调查","呼叫支援"]`,
	})
	body := []byte(`{"event":{"key":"overlap-event","name":"重叠活动","region":"遗迹","story_choices":"[\"记录位置\",\"尝试开启\",\"呼叫小队\"]","starts_at":"2026-08-21T00:00:00Z","ends_at":"2026-08-23T00:00:00Z","active":true},"rewards":[]}`)

	response := doConfigRequest(t, router, http.MethodPut, "/api/admin/content/events/overlap-event", body)
	if response.Code == 0 {
		t.Fatal("expected overlapping active event to be rejected")
	}
}

func TestDeleteEventBundleDeletesRelatedRewards(t *testing.T) {
	router, db := newContentTestRouter(t)
	db.Create(&models.LiveEventConfig{Key: "forest-week", Name: "森林周", Region: "森林", StoryChoices: `["一","二","三"]`, StartsAt: time.Now(), EndsAt: time.Now().Add(time.Hour), Active: true})
	db.Create(&models.ItemConfig{Key: "wood", Name: "木材", Status: "active"})
	db.Create(&models.RewardTrackConfig{EventKey: "forest-week", Milestone: 100, RewardType: "item", RewardKey: "wood", RewardName: "木材", Quantity: 5})
	response := doConfigRequest(t, router, http.MethodDelete, "/api/admin/content/events/forest-week", nil)
	if response.Code != 0 {
		t.Fatalf("删除活动组合失败: %s", response.Msg)
	}
	var eventCount, rewardCount int64
	db.Model(&models.LiveEventConfig{}).Where("key = ?", "forest-week").Count(&eventCount)
	db.Model(&models.RewardTrackConfig{}).Where("event_key = ?", "forest-week").Count(&rewardCount)
	if eventCount != 0 || rewardCount != 0 {
		t.Fatalf("关联数据未清理: event=%d reward=%d", eventCount, rewardCount)
	}
}
