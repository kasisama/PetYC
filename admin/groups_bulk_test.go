package admin

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
	"qq-pet-saas/database"
	"qq-pet-saas/models"
)

func newGroupTestRouter(t *testing.T) *gin.Engine {
	t.Helper()
	db := newConfigTestDB(t)
	if err := db.AutoMigrate(&models.GroupSwitch{}); err != nil {
		t.Fatalf("迁移群组表失败: %v", err)
	}
	database.DB = db
	router := gin.New()
	router.PUT("/api/admin/groups/bulk-state", BulkUpdateGroups)
	return router
}

func TestBulkUpdateGroupsUpdatesSelectedRows(t *testing.T) {
	router := newGroupTestRouter(t)
	database.DB.Create(&[]models.GroupSwitch{
		{GroupID: 1001, GroupName: "一群", IsActive: true},
		{GroupID: 1002, GroupName: "二群", IsActive: true},
	})

	response := doConfigRequest(t, router, http.MethodPut, "/api/admin/groups/bulk-state", []byte(`{"group_ids":[1002],"is_active":false}`))
	if response.Code != 0 {
		t.Fatalf("期望 code 0，实际 %d，msg=%s", response.Code, response.Msg)
	}
	var payload struct {
		Updated int64                `json:"updated"`
		Groups  []models.GroupSwitch `json:"groups"`
	}
	if err := json.Unmarshal(response.Data, &payload); err != nil {
		t.Fatalf("解析批量更新响应失败: %v", err)
	}
	if payload.Updated != 1 || len(payload.Groups) != 2 || payload.Groups[1].IsActive {
		t.Fatalf("批量更新结果不正确: %+v", payload)
	}
}

func TestBulkUpdateGroupsNullMeansAll(t *testing.T) {
	router := newGroupTestRouter(t)
	database.DB.Create(&[]models.GroupSwitch{
		{GroupID: 2001, IsActive: false},
		{GroupID: 2002, IsActive: false},
	})

	response := doConfigRequest(t, router, http.MethodPut, "/api/admin/groups/bulk-state", []byte(`{"group_ids":null,"is_active":true}`))
	if response.Code != 0 {
		t.Fatalf("期望 code 0，实际 %d，msg=%s", response.Code, response.Msg)
	}
	var activeCount int64
	database.DB.Model(&models.GroupSwitch{}).Where("is_active = ?", true).Count(&activeCount)
	if activeCount != 2 {
		t.Fatalf("全部开启后期望 2 个启用群，实际 %d", activeCount)
	}
}

func TestBulkUpdateGroupsRejectsEmptySelection(t *testing.T) {
	response := doConfigRequest(t, newGroupTestRouter(t), http.MethodPut, "/api/admin/groups/bulk-state", []byte(`{"group_ids":[],"is_active":true}`))
	if response.Code != codeInvalidPayload {
		t.Fatalf("空数组期望 code %d，实际 %d", codeInvalidPayload, response.Code)
	}
}

func TestBulkUpdateGroupsRejectsMissingSelection(t *testing.T) {
	response := doConfigRequest(t, newGroupTestRouter(t), http.MethodPut, "/api/admin/groups/bulk-state", []byte(`{"is_active":true}`))
	if response.Code != codeInvalidPayload {
		t.Fatalf("缺少 group_ids 期望 code %d，实际 %d", codeInvalidPayload, response.Code)
	}
}
