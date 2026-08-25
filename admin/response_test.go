package admin

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestErrorMapsBusinessConflictToHTTP409(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodPost, "/api/admin/test", nil)

	Error(context, 4090, "操作冲突")

	if recorder.Code != http.StatusConflict {
		t.Fatalf("期望 HTTP %d，实际 %d", http.StatusConflict, recorder.Code)
	}
	if recorder.Header().Get("X-Request-ID") == "" {
		t.Fatal("标准错误响应必须返回 request_id")
	}
}
