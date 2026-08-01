package admin

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"qq-pet-saas/models"
)

// configResponse mirrors the standard {code, msg, data} contract.
type configResponse struct {
	Code int             `json:"code"`
	Msg  string          `json:"msg"`
	Data json.RawMessage `json:"data"`
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
		&models.ItemConfig{},
		&models.ShopItemConfig{},
		&models.WorkSettingConfig{},
		&models.MenuConfig{},
		&models.ImageConfig{},
		&models.CheckinRewardConfig{},
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

	if recorder.Code != http.StatusOK {
		t.Fatalf("%s %s 期望 HTTP 200，实际 %d，响应体 %s", method, target, recorder.Code, recorder.Body.String())
	}
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
		"system", "commands", "pet_species", "items", "shop_items",
		"work_settings", "menus", "images", "checkin_rewards",
	}
	for _, schema := range schemas {
		response := doConfigRequest(t, router, http.MethodGet, "/api/admin/config/"+schema, nil)
		if response.Code != 0 {
			t.Errorf("schema %q 期望 code 0，实际 %d，msg=%s", schema, response.Code, response.Msg)
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
	db.Create(&models.MenuConfig{Name: "主菜单", Reply: "旧内容"})

	body := []byte(`[{"Name":"主菜单","Reply":"新内容"},{"Name":"帮助","Reply":"帮助内容"}]`)
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
