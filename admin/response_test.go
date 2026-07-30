package admin

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestSuccessResponse(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	
	Success(ctx, gin.H{"key": "value"})

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", recorder.Code)
	}
	
	// Map serialization order might vary occasionally, but for simple gin.H it's usually stable,
	// or we can just check if required keys exist
	body := recorder.Body.String()
	if !strings.Contains(body, `"code":0`) || !strings.Contains(body, `"msg":"success"`) || !strings.Contains(body, `"key":"value"`) {
		t.Fatalf("unexpected JSON: %s", body)
	}
}

func TestErrorResponse(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	
	Error(ctx, 4004, "not found")

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", recorder.Code)
	}
	body := recorder.Body.String()
	if !strings.Contains(body, `"code":4004`) || !strings.Contains(body, `"msg":"not found"`) {
		t.Fatalf("unexpected JSON: %s", body)
	}
}
