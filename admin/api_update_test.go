package admin

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"qq-pet-saas/updater"
)

func TestInstallUpdateRequiresConfirmation(t *testing.T) {
	previous := UpdateService
	UpdateService = updater.NewService(updater.Config{CurrentVersion: "dev"})
	t.Cleanup(func() { UpdateService = previous })

	gin.SetMode(gin.TestMode)
	router := gin.New()
	RegisterUpdateRoutes(router.Group("/api/admin"))

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/api/admin/updates/install", nil))
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("无确认词不应开始安装，HTTP %d body=%s", recorder.Code, recorder.Body.String())
	}
	if UpdateService.Status().State == "checking" || UpdateService.Status().State == "downloading" {
		t.Fatalf("无确认词时更新服务状态被改成了 %s", UpdateService.Status().State)
	}
}

func TestUpdateRoutesRequireAdminSession(t *testing.T) {
	t.Setenv("QQPET_DATA_DIR", t.TempDir())
	previous := UpdateService
	UpdateService = updater.NewService(updater.Config{CurrentVersion: "dev"})
	t.Cleanup(func() { UpdateService = previous })
	gin.SetMode(gin.TestMode)
	router := gin.New()
	RegisterRoutes(router)

	for _, request := range []struct{ method, path string }{
		{http.MethodGet, "/api/admin/updates/check"},
		{http.MethodGet, "/api/admin/updates/status"},
		{http.MethodPost, "/api/admin/updates/install"},
	} {
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, httptest.NewRequest(request.method, request.path, nil))
		if recorder.Code != http.StatusUnauthorized {
			t.Fatalf("%s %s returned %d, want 401", request.method, request.path, recorder.Code)
		}
	}
}
