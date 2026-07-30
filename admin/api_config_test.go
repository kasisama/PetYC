package admin

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

// ConfigResponse simulates standard API responses
type ConfigResponse struct {
	Code int         `json:"code"`
	Msg  string      `json:"msg"`
	Data interface{} `json:"data"`
}

func TestGetGlobalConfig(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	
	// Create a dummy session manager to bypass auth middleware in test
	// Let's manually register the routes without auth for this simple unit test
	// In reality we should test with auth, but focusing on the controller logic here
	
	api := &ConfigAPI{}
	router.GET("/api/admin/config/:schema", api.GetConfig)

	req, _ := http.NewRequest(http.MethodGet, "/api/admin/config/global_parameters", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	// Since we mock it, we want 200 OK and a standard response with code 0.
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", recorder.Code)
	}

	var resp ConfigResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode json: %v", err)
	}

	if resp.Code != 0 {
		t.Errorf("expected code 0, got %d. msg: %s", resp.Code, resp.Msg)
	}
}
