package admin

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"qq-pet-saas/updater"
)

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
