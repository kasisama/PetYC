package admin

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	configstate "qq-pet-saas/config"
	"qq-pet-saas/models"
)

// configResponse mirrors the standard {code, msg, data} contract.
type configResponse struct {
	Code int             `json:"code"`
	Msg  string          `json:"msg"`
	Data json.RawMessage `json:"data"`
}

func TestEveryConfigSchemaDeclaresRuntimeConsumers(t *testing.T) {
	for schema := range configSchemas {
		consumers := configConsumers[schema]
		if len(consumers) == 0 {
			t.Errorf("配置 schema %q 没有声明运行时消费者", schema)
		}
	}
}

// newConfigTestDB builds an isolated in-memory database carrying the config tables.
func newConfigTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("打开内存数据库失败: %v", err)
	}
	err = db.AutoMigrate(
		&models.SystemConfig{},
		&models.CommandConfig{},
		&models.PetSpeciesConfig{},
		&models.PetEvolutionRuleConfig{},
		&models.PetEvolutionCostConfig{},
		&models.PetSkillUnlockConfig{},
		&models.AdventureLevelConfig{},
		&models.ItemConfig{},
		&models.CurrencyConfig{},
		&models.RewardTrackConfig{},
		&models.LiveEventConfig{},
		&models.ShopItemConfig{},
		&models.WorkSettingConfig{},
		&models.MenuConfig{},
		&models.ImageConfig{},
		&models.CheckinRewardConfig{},
		&models.GrowthRoleConfig{},
		&models.GrowthStanceConfig{},
		&models.PersonalityRuleConfig{},
		&models.CodexCatalogConfig{},
		&models.ExpeditionTemplateConfig{},
		&models.AdminConfigState{},
	)
	if err != nil {
		t.Fatalf("迁移测试表结构失败: %v", err)
	}
	return db
}

// newConfigTestRouter registers the config routes without the auth middleware.
func newConfigTestRouter(db *gorm.DB) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	RegisterConfigRoutes(router.Group("/api/admin"), NewConfigAPI(db))
	return router
}

func doConfigRequest(t *testing.T, router *gin.Engine, method, target string, body []byte) configResponse {
	t.Helper()
	var reader *bytes.Reader
	if body == nil {
		reader = bytes.NewReader(nil)
	} else {
		reader = bytes.NewReader(body)
	}
	request, _ := http.NewRequest(method, target, reader)
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	var response configResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("解析 JSON 失败: %v，响应体 %s", err, recorder.Body.String())
	}
	return response
}

func TestGetConfigReturnsRowsForKnownSchema(t *testing.T) {
	db := newConfigTestDB(t)
	db.Create(&models.SystemConfig{Key: "pet_max_level", Value: "60"})
	db.Create(&models.SystemConfig{Key: "adopt_cost", Value: "100"})

	response := doConfigRequest(t, newConfigTestRouter(db), http.MethodGet, "/api/admin/config/system", nil)
	if response.Code != 0 {
		t.Fatalf("期望 code 0，实际 %d，msg=%s", response.Code, response.Msg)
	}

	var rows []models.SystemConfig
	if err := json.Unmarshal(response.Data, &rows); err != nil {
		t.Fatalf("data 无法解析为 SystemConfig 列表: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("期望 2 条记录，实际 %d 条", len(rows))
	}
	// 断言按 key 升序排列
	if rows[0].Key != "adopt_cost" || rows[1].Key != "pet_max_level" {
		t.Fatalf("排序错误，实际顺序为 %s, %s", rows[0].Key, rows[1].Key)
	}
}

func TestLiveEventConfigRejectsInvalidStoryChoices(t *testing.T) {
	db := newConfigTestDB(t)
	if err := db.AutoMigrate(&models.LiveEventConfig{}); err != nil {
		t.Fatal(err)
	}
	body := []byte(`[{"key":"forest-week","name":"森林周","region":"森林","story_choices":"[\"只有一个\"]","starts_at":"2026-08-01T00:00:00Z","ends_at":"2026-08-08T00:00:00Z","active":true}]`)
	response := doConfigRequest(t, newConfigTestRouter(db), http.MethodPut, "/api/admin/config/live_events", body)
	if response.Code != codeInvalidPayload {
		t.Fatalf("expected validation error, got code=%d msg=%s", response.Code, response.Msg)
	}
}

func TestRewardTrackAllowsMultipleItemsAtSameMilestone(t *testing.T) {
	db := newConfigTestDB(t)
	if err := db.AutoMigrate(&models.RewardTrackConfig{}); err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&models.ItemConfig{Key: "wood", Name: "木材", Status: "active"}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&models.ItemConfig{Key: "survey_log", Name: "调查记录", Status: "active"}).Error; err != nil {
		t.Fatal(err)
	}
	body := []byte(`[{"event_key":"forest","milestone":100,"reward_type":"item","reward_key":"wood","reward_name":"木材","quantity":5},{"event_key":"forest","milestone":100,"reward_type":"item","reward_key":"survey_log","reward_name":"调查记录","quantity":2}]`)
	response := doConfigRequest(t, newConfigTestRouter(db), http.MethodPut, "/api/admin/config/reward_tracks", body)
	if response.Code != 0 {
		t.Fatalf("同一里程碑应允许多个不同物品，实际 code=%d msg=%s", response.Code, response.Msg)
	}
	var count int64
	db.Model(&models.RewardTrackConfig{}).Where("event_key = ? AND milestone = ?", "forest", 100).Count(&count)
	if count != 2 {
		t.Fatalf("期望保存 2 个奖励物品，实际 %d", count)
	}
}

func TestRewardTrackRejectsDuplicateItemAtSameMilestone(t *testing.T) {
	db := newConfigTestDB(t)
	if err := db.AutoMigrate(&models.RewardTrackConfig{}); err != nil {
		t.Fatal(err)
	}
	body := []byte(`[{"event_key":"forest","milestone":100,"reward_type":"item","reward_key":"wood","reward_name":"木材","quantity":5},{"event_key":"forest","milestone":100,"reward_type":"item","reward_key":"wood","reward_name":"木材","quantity":2}]`)
	response := doConfigRequest(t, newConfigTestRouter(db), http.MethodPut, "/api/admin/config/reward_tracks", body)
	if response.Code != codeInvalidPayload {
		t.Fatalf("同一里程碑的同一物品应被拒绝，实际 code=%d msg=%s", response.Code, response.Msg)
	}
}

func TestChanceGameAndRewardConfigsAreValidatedAndReplaceable(t *testing.T) {
	db := newConfigTestDB(t)
	if err := db.AutoMigrate(&models.ChanceGameConfig{}, &models.ChanceRewardConfig{}); err != nil {
		t.Fatal(err)
	}
	gameBody := []byte(`[{"game_key":"lottery","name":"幸运抽奖","enabled":true,"cost_currency":20,"daily_limit":10,"pity_threshold":10,"pity_reward_key":"rare"}]`)
	if response := doConfigRequest(t, newConfigTestRouter(db), http.MethodPut, "/api/admin/config/chance_games", gameBody); response.Code != 0 {
		t.Fatalf("概率玩法配置保存失败: %s", response.Msg)
	}
	rewardBody := []byte(`[{"game_key":"lottery","reward_key":"common","name":"普通奖励","weight":99,"item_name":"木材","quantity":1,"enabled":true},{"game_key":"lottery","reward_key":"rare","name":"珍稀奖励","weight":1,"item_name":"光之石","quantity":1,"rare":true,"enabled":true}]`)
	if response := doConfigRequest(t, newConfigTestRouter(db), http.MethodPut, "/api/admin/config/chance_rewards", rewardBody); response.Code != 0 {
		t.Fatalf("概率奖励配置保存失败: %s", response.Msg)
	}
	invalid := []byte(`[{"game_key":"lottery","reward_key":"broken","name":"错误奖励","weight":0,"quantity":0,"enabled":true}]`)
	if response := doConfigRequest(t, newConfigTestRouter(db), http.MethodPut, "/api/admin/config/chance_rewards", invalid); response.Code != codeInvalidPayload {
		t.Fatalf("无效概率奖励应被拒绝: code=%d msg=%s", response.Code, response.Msg)
	}
}

func TestLiveEventSaveReplacesRemovedRows(t *testing.T) {
	db := newConfigTestDB(t)
	if err := db.AutoMigrate(&models.LiveEventConfig{}); err != nil {
		t.Fatal(err)
	}
	db.Create(&models.LiveEventConfig{Key: "old", Name: "旧活动", Region: "森林", StoryChoices: `["一","二","三"]`, StartsAt: time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC), EndsAt: time.Date(2026, 7, 2, 0, 0, 0, 0, time.UTC), Active: true})
	body := []byte(`[{"key":"new","name":"新活动","region":"遗迹","story_choices":"[\"一\",\"二\",\"三\"]","starts_at":"2026-08-01T00:00:00Z","ends_at":"2026-08-08T00:00:00Z","active":true}]`)
	response := doConfigRequest(t, newConfigTestRouter(db), http.MethodPut, "/api/admin/config/live_events", body)
	if response.Code != 0 {
		t.Fatalf("save failed: %s", response.Msg)
	}
	var rows []models.LiveEventConfig
	db.Find(&rows)
	if len(rows) != 1 || rows[0].Key != "new" {
		t.Fatalf("expected only new row, got %+v", rows)
	}
}

func TestGetConfigReturnsEmptyListNotNullForEmptyTable(t *testing.T) {
	response := doConfigRequest(t, newConfigTestRouter(newConfigTestDB(t)), http.MethodGet, "/api/admin/config/menus", nil)
	if response.Code != 0 {
		t.Fatalf("期望 code 0，实际 %d", response.Code)
	}
	if string(response.Data) != "[]" {
		t.Fatalf("空表应返回 []，实际返回 %s", string(response.Data))
	}
}

func TestGetConfigCoversEveryConfigSchema(t *testing.T) {
	router := newConfigTestRouter(newConfigTestDB(t))
	// 后台需要管理的 9 张配置表都必须可读，否则 Vue 前端会缺少数据源。
	schemas := []string{
		"system", "commands", "pet_species", "pet_evolution_rules", "pet_evolution_costs", "pet_skill_unlocks",
		"items", "shop_items", "checkin_rewards", "work_settings", "adventure_levels",
		"menus", "images",
		"growth_roles", "growth_stances", "personality_rules", "codex_catalog",
		"expedition_templates",
	}
	for _, schema := range schemas {
		response := doConfigRequest(t, router, http.MethodGet, "/api/admin/config/"+schema, nil)
		if response.Code != 0 {
			t.Errorf("schema %q 期望 code 0，实际 %d，msg=%s", schema, response.Code, response.Msg)
		}
	}
}

func TestCheckinAndWorkSettingsAreExposed(t *testing.T) {
	router := newConfigTestRouter(newConfigTestDB(t))
	for _, schema := range []string{"work_settings", "checkin_rewards", "adventure_levels", "pet_evolution_rules"} {
		response := doConfigRequest(t, router, http.MethodGet, "/api/admin/config/"+schema, nil)
		if response.Code != 0 {
			t.Fatalf("配置域 %s 应可管理，实际 code=%d msg=%s", schema, response.Code, response.Msg)
		}
	}
}

func TestCommandConfigRejectsDuplicateEnabledTrigger(t *testing.T) {
	body := []byte(`[{"func_name":"status","command":"状态","display_name":"状态","category":"基础","description":"查看状态","enabled":true,"sort_order":1},{"func_name":"daily","command":"状态","display_name":"今日","category":"基础","description":"查看今日","enabled":true,"sort_order":2}]`)
	response := doConfigRequest(t, newConfigTestRouter(newConfigTestDB(t)), http.MethodPut, "/api/admin/config/commands", body)
	if response.Code != codeInvalidPayload {
		t.Fatalf("重复启用触发词应被拒绝，实际 code=%d msg=%s", response.Code, response.Msg)
	}
}

func TestGrowthRulesRejectIncompleteOrInvalidRows(t *testing.T) {
	db := newConfigTestDB(t)
	router := newConfigTestRouter(db)
	cases := []struct {
		schema string
		body   string
	}{
		{"growth_roles", `[{"name":"守护者","description":"保护伙伴","skill_1":"护盾","skill_2":"","skill_3":"稳固","enabled":true}]`},
		{"growth_stances", `[{"name":"","description":"降低风险","enabled":true}]`},
		{"personality_rules", `[{"name":"温柔","dimension":"unknown","min_threshold":3,"description":"照料形成","enabled":true}]`},
		{"codex_catalog", `[{"category":"生物","entry_key":"林间足迹","region":"","enabled":true}]`},
		{"expedition_templates", `[{"tier":1,"name":"","duration_minutes":0,"reward_item":"","reward_quantity":0}]`},
	}
	for _, item := range cases {
		response := doConfigRequest(t, router, http.MethodPut, "/api/admin/config/"+item.schema, []byte(item.body))
		if response.Code != codeInvalidPayload {
			t.Errorf("schema %s expected validation error, got code=%d msg=%s", item.schema, response.Code, response.Msg)
		}
	}
}

func TestGetConfigRejectsUnknownSchema(t *testing.T) {
	// global_parameters 并不存在于数据库中，必须走业务错误分支而不是查询未知表。
	response := doConfigRequest(t, newConfigTestRouter(newConfigTestDB(t)), http.MethodGet, "/api/admin/config/global_parameters", nil)
	if response.Code != codeSchemaNotFound {
		t.Fatalf("期望 code %d，实际 %d，msg=%s", codeSchemaNotFound, response.Code, response.Msg)
	}
}

func TestSaveConfigPersistsRows(t *testing.T) {
	db := newConfigTestDB(t)
	if err := configstate.MarkConfigLoaded(db); err != nil {
		t.Fatalf("初始化配置状态失败: %v", err)
	}
	db.Create(&models.MenuConfig{Name: "主菜单", Reply: "旧内容"})

	body := []byte(`[{"Name":"主菜单","Reply":"新内容","Markdown":"# 新内容","Image":"上传/main.webp"},{"Name":"帮助","Reply":"帮助内容","Markdown":"","Image":""}]`)
	response := doConfigRequest(t, newConfigTestRouter(db), http.MethodPut, "/api/admin/config/menus", body)
	if response.Code != 0 {
		t.Fatalf("期望 code 0，实际 %d，msg=%s", response.Code, response.Msg)
	}

	var rows []models.MenuConfig
	db.Order("name asc").Find(&rows)
	if len(rows) != 2 {
		t.Fatalf("期望库中有 2 条记录，实际 %d 条", len(rows))
	}
	var updated models.MenuConfig
	db.First(&updated, "name = ?", "主菜单")
	if updated.Reply != "新内容" {
		t.Fatalf("既有记录未被更新，Reply=%q", updated.Reply)
	}
	if updated.Markdown != "# 新内容" {
		t.Fatalf("菜单 Markdown 未被保存，Markdown=%q", updated.Markdown)
	}
	if updated.Image != "上传/main.webp" {
		t.Fatalf("菜单图片未被保存，Image=%q", updated.Image)
	}

	status, err := configstate.GetConfigStatus(db)
	if err != nil {
		t.Fatalf("读取配置状态失败: %v", err)
	}
	if !status.PendingReload || status.DBRevision != status.LoadedRevision+1 {
		t.Fatalf("保存后应进入待重载状态，实际 %+v", status)
	}
	if err := configstate.MarkConfigLoaded(db); err != nil {
		t.Fatalf("标记配置已加载失败: %v", err)
	}
	status, err = configstate.GetConfigStatus(db)
	if err != nil {
		t.Fatalf("重载后读取配置状态失败: %v", err)
	}
	if status.PendingReload || status.DBRevision != status.LoadedRevision {
		t.Fatalf("重载后数据库与内存版本应一致，实际 %+v", status)
	}
}

func TestConfigStatusTracksLoadedRevision(t *testing.T) {
	db := newConfigTestDB(t)
	if err := configstate.MarkConfigLoaded(db); err != nil {
		t.Fatalf("初始化配置状态失败: %v", err)
	}

	response := doConfigRequest(t, newConfigTestRouter(db), http.MethodGet, "/api/admin/config/status", nil)
	if response.Code != 0 {
		t.Fatalf("期望 code 0，实际 %d，msg=%s", response.Code, response.Msg)
	}
	var status configstate.ConfigStatus
	if err := json.Unmarshal(response.Data, &status); err != nil {
		t.Fatalf("解析配置状态失败: %v", err)
	}
	if status.PendingReload || status.DBRevision != status.LoadedRevision {
		t.Fatalf("首次加载后数据库与内存版本应一致，实际 %+v", status)
	}
}

func TestMarkConfigLoadedDoesNotHideNewerSavedRevision(t *testing.T) {
	db := newConfigTestDB(t)
	if err := configstate.MarkConfigLoaded(db); err != nil {
		t.Fatalf("初始化配置状态失败: %v", err)
	}
	if err := configstate.MarkConfigSaved(db); err != nil {
		t.Fatalf("第一次保存标记失败: %v", err)
	}
	status, err := configstate.GetConfigStatus(db)
	if err != nil {
		t.Fatalf("读取待加载版本失败: %v", err)
	}
	loadedRevision := status.DBRevision

	if err := configstate.MarkConfigSaved(db); err != nil {
		t.Fatalf("并发保存标记失败: %v", err)
	}
	if err := configstate.MarkConfigLoaded(db, loadedRevision); err != nil {
		t.Fatalf("标记指定版本已加载失败: %v", err)
	}

	status, err = configstate.GetConfigStatus(db)
	if err != nil {
		t.Fatalf("读取最终配置状态失败: %v", err)
	}
	if !status.PendingReload || status.LoadedRevision != loadedRevision || status.DBRevision <= status.LoadedRevision {
		t.Fatalf("较新的保存版本必须保持待重载，实际 %+v", status)
	}

	latestRevision := status.DBRevision
	if err := configstate.MarkConfigLoaded(db, latestRevision); err != nil {
		t.Fatalf("标记最新版本已加载失败: %v", err)
	}
	if err := configstate.MarkConfigLoaded(db, loadedRevision); err != nil {
		t.Fatalf("较旧重载完成标记失败: %v", err)
	}
	status, err = configstate.GetConfigStatus(db)
	if err != nil {
		t.Fatalf("读取单调加载版本失败: %v", err)
	}
	if status.PendingReload || status.LoadedRevision != latestRevision {
		t.Fatalf("较旧重载不应让已加载版本倒退，实际 %+v", status)
	}
}

func TestSaveMenuConfigValidatesRequiredFieldsAndDuplicateNames(t *testing.T) {
	router := newConfigTestRouter(newConfigTestDB(t))
	tests := []struct {
		name string
		body string
	}{
		{name: "empty-name", body: `[{"Name":"  ","Reply":"有效回复","Image":""}]`},
		{name: "empty-reply", body: `[{"Name":"主菜单","Reply":"  ","Image":""}]`},
		{name: "duplicate-name", body: `[{"Name":"主菜单","Reply":"回复一","Image":""},{"Name":" 主菜单 ","Reply":"回复二","Image":""}]`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			response := doConfigRequest(t, router, http.MethodPut, "/api/admin/config/menus", []byte(tc.body))
			if response.Code != codeInvalidPayload {
				t.Fatalf("期望 code %d，实际 %d，msg=%s", codeInvalidPayload, response.Code, response.Msg)
			}
		})
	}
}

func TestSaveMenuConfigAllowsEmptyImage(t *testing.T) {
	db := newConfigTestDB(t)
	response := doConfigRequest(t, newConfigTestRouter(db), http.MethodPut, "/api/admin/config/menus", []byte(`[{"Name":"主菜单","Reply":"纯文字菜单","Image":""}]`))
	if response.Code != 0 {
		t.Fatalf("空图片菜单应允许保存，code=%d msg=%s", response.Code, response.Msg)
	}
	var row models.MenuConfig
	if err := db.First(&row, "name = ?", "主菜单").Error; err != nil {
		t.Fatal(err)
	}
	if row.Reply != "纯文字菜单" || row.Markdown != "" || row.Image != "" {
		t.Fatalf("纯文字菜单保存结果错误: %#v", row)
	}
}

func TestSaveConfigRejectsMalformedBody(t *testing.T) {
	response := doConfigRequest(t, newConfigTestRouter(newConfigTestDB(t)), http.MethodPut, "/api/admin/config/menus", []byte(`{"not":"an array"}`))
	if response.Code != codeInvalidPayload {
		t.Fatalf("期望 code %d，实际 %d", codeInvalidPayload, response.Code)
	}
}

func TestSaveConfigRejectsUnknownSchema(t *testing.T) {
	response := doConfigRequest(t, newConfigTestRouter(newConfigTestDB(t)), http.MethodPut, "/api/admin/config/global_parameters", []byte(`[]`))
	if response.Code != codeSchemaNotFound {
		t.Fatalf("期望 code %d，实际 %d", codeSchemaNotFound, response.Code)
	}
}

func TestSaveConfigIsAtomicOnFailure(t *testing.T) {
	db := newConfigTestDB(t)
	db.Create(&models.MenuConfig{Name: "主菜单", Reply: "旧内容"})

	// SQLite 对空主键并不报错，因此注入一个 GORM 回调来模拟真实的写库故障，
	// 以此验证批量写入确实包在事务里。
	const poison = "触发写库失败"
	failOnPoison := func(tx *gorm.DB) {
		if row, ok := tx.Statement.Dest.(*models.MenuConfig); ok && row.Reply == poison {
			tx.AddError(errors.New("注入的写库故障"))
		}
	}
	if err := db.Callback().Update().Before("gorm:update").Register("test:fail_update", failOnPoison); err != nil {
		t.Fatalf("注册 update 回调失败: %v", err)
	}
	if err := db.Callback().Create().Before("gorm:create").Register("test:fail_create", failOnPoison); err != nil {
		t.Fatalf("注册 create 回调失败: %v", err)
	}

	// 第一条合法、第二条触发故障，整批都不应落库。
	body := []byte(`[{"Name":"主菜单","Reply":"新内容"},{"Name":"帮助","Reply":"` + poison + `"}]`)
	response := doConfigRequest(t, newConfigTestRouter(db), http.MethodPut, "/api/admin/config/menus", body)
	if response.Code != codeInternalError {
		t.Fatalf("期望 code %d，实际 %d，msg=%s", codeInternalError, response.Code, response.Msg)
	}

	var unchanged models.MenuConfig
	db.First(&unchanged, "name = ?", "主菜单")
	if unchanged.Reply != "旧内容" {
		t.Fatalf("事务未回滚，Reply=%q", unchanged.Reply)
	}
	var count int64
	db.Model(&models.MenuConfig{}).Where("name = ?", "帮助").Count(&count)
	if count != 0 {
		t.Fatalf("失败批次中的记录不应落库，实际找到 %d 条", count)
	}
}

func TestConfigRoutesRequireAdminSession(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	// 与 RegisterRoutes 中一致：配置接口必须挂在带鉴权中间件的分组下。
	group := router.Group("/api/admin", RequireAdminSession(NewSessionManager()))
	RegisterConfigRoutes(group, NewConfigAPI(newConfigTestDB(t)))

	request, _ := http.NewRequest(http.MethodGet, "/api/admin/config/system", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("无会话访问期望 401，实际 %d", recorder.Code)
	}
}

func TestConfigAPIWithoutDatabaseReportsError(t *testing.T) {
	// DB 未注入属于配置错误，必须显式报错，不能伪装成成功的空结果。
	response := doConfigRequest(t, newConfigTestRouter(nil), http.MethodGet, "/api/admin/config/system", nil)
	if response.Code != codeInternalError {
		t.Fatalf("期望 code %d，实际 %d", codeInternalError, response.Code)
	}
}
