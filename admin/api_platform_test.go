package admin

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"qq-pet-saas/security"
)

func newPlatformTestRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	RegisterPlatformRoutes(router.Group("/api/admin"), nil)
	return router
}

func platformRequest(t *testing.T, router http.Handler, method, target, body string) map[string]interface{} {
	t.Helper()
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(method, target, strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, request)
	var response map[string]interface{}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return response
}

func TestPlatformConfigGetAndPutNeverEchoSecretsAndReconnectQQ(t *testing.T) {
	t.Setenv("QQPET_DATA_DIR", t.TempDir())
	t.Setenv("PORT", "8080")
	t.Setenv("QQPET_WS_TOKEN", "initial-onebot-token")
	if _, err := security.LoadRuntimeConfig(); err != nil {
		t.Fatal(err)
	}

	reconnects := 0
	previousApply := QQOfficialApplyConfigFunc
	QQOfficialApplyConfigFunc = func() error { reconnects++; return nil }
	t.Cleanup(func() { QQOfficialApplyConfigFunc = previousApply })
	router := newPlatformTestRouter()

	response := platformRequest(t, router, http.MethodPut, "/api/admin/platforms/config", `{
		"onebot":{"token":"new-onebot-secret"},
		"qq_official":{"app_id":"new-app-id","app_secret":"new-app-secret","markdown_enabled":true}
	}`)
	encoded, _ := json.Marshal(response)
	if bytes.Contains(encoded, []byte("new-onebot-secret")) || bytes.Contains(encoded, []byte("new-app-secret")) {
		t.Fatalf("PUT response leaked a secret: %s", encoded)
	}
	if reconnects != 1 {
		t.Fatalf("QQ reconnect count = %d, want 1", reconnects)
	}

	stored, err := security.LoadRuntimeConfig()
	if err != nil {
		t.Fatal(err)
	}
	if stored.OneBotToken != "new-onebot-secret" || stored.QQOfficial.AppSecret != "new-app-secret" {
		t.Fatalf("stored runtime config = %#v", stored)
	}

	response = platformRequest(t, router, http.MethodGet, "/api/admin/platforms/config", "")
	encoded, _ = json.Marshal(response)
	if bytes.Contains(encoded, []byte("new-onebot-secret")) || bytes.Contains(encoded, []byte("new-app-secret")) {
		t.Fatalf("GET response leaked a secret: %s", encoded)
	}
}

func TestPlatformConfigBlankSecretFieldsKeepStoredValues(t *testing.T) {
	t.Setenv("QQPET_DATA_DIR", t.TempDir())
	t.Setenv("PORT", "8080")
	t.Setenv("QQPET_WS_TOKEN", "stored-onebot-token")
	t.Setenv("QQBOT_APP_ID", "stored-app-id")
	t.Setenv("QQBOT_APP_SECRET", "stored-app-secret")
	if _, err := security.LoadRuntimeConfig(); err != nil {
		t.Fatal(err)
	}

	response := platformRequest(t, newPlatformTestRouter(), http.MethodPut, "/api/admin/platforms/config", `{
		"onebot":{"token":""},
		"qq_official":{"app_secret":"","markdown_enabled":true}
	}`)
	if response["code"].(float64) != 0 {
		t.Fatalf("blank write-only fields should keep existing values: %#v", response)
	}
	stored, err := security.LoadRuntimeConfig()
	if err != nil {
		t.Fatal(err)
	}
	if stored.OneBotToken != "stored-onebot-token" || stored.QQOfficial.AppSecret != "stored-app-secret" {
		t.Fatalf("blank write-only fields cleared secrets: %#v", stored)
	}
}

func TestPlatformConfigPersistsEventSubscriptions(t *testing.T) {
	t.Setenv("QQPET_DATA_DIR", t.TempDir())
	t.Setenv("PORT", "8080")
	t.Setenv("QQPET_WS_TOKEN", "stable-token")
	if _, err := security.LoadRuntimeConfig(); err != nil {
		t.Fatal(err)
	}
	response := platformRequest(t, newPlatformTestRouter(), http.MethodPut, "/api/admin/platforms/config", `{
		"qq_official":{"group_events_enabled":true,"guild_events_enabled":false}
	}`)
	if response["code"].(float64) != 0 {
		t.Fatalf("event subscription update failed: %#v", response)
	}
	stored, err := security.LoadRuntimeConfig()
	if err != nil {
		t.Fatal(err)
	}
	if !stored.QQOfficial.GroupEventsEnabled || stored.QQOfficial.GuildEventsEnabled {
		t.Fatalf("event subscriptions were not stored: %#v", stored.QQOfficial)
	}
}

func TestPlatformConfigListenAddressUsesEndpointHandoff(t *testing.T) {
	t.Setenv("QQPET_DATA_DIR", t.TempDir())
	t.Setenv("PORT", "8080")
	t.Setenv("LISTEN_ADDRESS", "0.0.0.0")
	t.Setenv("QQPET_WS_TOKEN", "stable-token")
	if _, err := security.LoadRuntimeConfig(); err != nil {
		t.Fatal(err)
	}
	previousEndpoint := PlatformEndpointChangeFunc
	PlatformEndpointChangeFunc = func(address string, port int) (PortHandoffResult, error) {
		if address != "127.0.0.1" || port != 9091 {
			t.Fatalf("endpoint handoff = %s:%d", address, port)
		}
		return PortHandoffResult{Address: "http://127.0.0.1:9091", ConfirmationToken: "endpoint-token"}, nil
	}
	t.Cleanup(func() { PlatformEndpointChangeFunc = previousEndpoint })

	response := platformRequest(t, newPlatformTestRouter(), http.MethodPut, "/api/admin/platforms/config", `{"listen_address":"127.0.0.1","port":9091}`)
	if response["code"].(float64) != 0 {
		t.Fatalf("endpoint handoff failed: %#v", response)
	}
	data := response["data"].(map[string]interface{})
	if data["port_handoff"].(map[string]interface{})["confirmation_token"] != "endpoint-token" {
		t.Fatalf("missing endpoint handoff: %#v", data)
	}
}

func TestPlatformConfigPortChangeStartsHandoffWithoutPersistingPort(t *testing.T) {
	t.Setenv("QQPET_DATA_DIR", t.TempDir())
	t.Setenv("PORT", "8080")
	t.Setenv("QQPET_WS_TOKEN", "onebot-token")
	if _, err := security.LoadRuntimeConfig(); err != nil {
		t.Fatal(err)
	}

	previousBegin := PlatformPortChangeFunc
	PlatformPortChangeFunc = func(port int) (PortHandoffResult, error) {
		if port != 9090 {
			t.Fatalf("handoff port = %d", port)
		}
		return PortHandoffResult{Address: "http://127.0.0.1:9090", ConfirmationToken: "one-time-token", ExpiresAt: time.Now().Add(time.Minute)}, nil
	}
	t.Cleanup(func() { PlatformPortChangeFunc = previousBegin })

	response := platformRequest(t, newPlatformTestRouter(), http.MethodPut, "/api/admin/platforms/config", `{"port":9090}`)
	data := response["data"].(map[string]interface{})
	handoff := data["port_handoff"].(map[string]interface{})
	if handoff["address"] != "http://127.0.0.1:9090" || handoff["confirmation_token"] != "one-time-token" {
		t.Fatalf("handoff response = %#v", handoff)
	}
	stored, err := security.LoadRuntimeConfig()
	if err != nil {
		t.Fatal(err)
	}
	if stored.Port != 8080 {
		t.Fatalf("port was persisted before confirmation: %d", stored.Port)
	}
}

func TestPlatformConfigPortListenerFailureDoesNotSaveAnyChanges(t *testing.T) {
	t.Setenv("QQPET_DATA_DIR", t.TempDir())
	t.Setenv("PORT", "8080")
	t.Setenv("QQPET_WS_TOKEN", "stable-token")
	original, err := security.LoadRuntimeConfig()
	if err != nil {
		t.Fatal(err)
	}

	previousBegin := PlatformPortChangeFunc
	PlatformPortChangeFunc = func(int) (PortHandoffResult, error) {
		return PortHandoffResult{}, errors.New("address already in use")
	}
	t.Cleanup(func() { PlatformPortChangeFunc = previousBegin })

	response := platformRequest(t, newPlatformTestRouter(), http.MethodPut, "/api/admin/platforms/config", `{"port":9090,"onebot":{"token":"must-not-save"}}`)
	if response["code"].(float64) == 0 {
		t.Fatalf("listener failure returned success: %#v", response)
	}
	stored, err := security.LoadRuntimeConfig()
	if err != nil {
		t.Fatal(err)
	}
	if stored != original {
		t.Fatalf("failed handoff changed config: got %#v, want %#v", stored, original)
	}
}

func TestPlatformConfigRejectsInvalidPortBeforeStartingHandoff(t *testing.T) {
	t.Setenv("QQPET_DATA_DIR", t.TempDir())
	t.Setenv("PORT", "8080")
	t.Setenv("QQPET_WS_TOKEN", "stable-token")
	if _, err := security.LoadRuntimeConfig(); err != nil {
		t.Fatal(err)
	}

	previousBegin := PlatformPortChangeFunc
	called := false
	PlatformPortChangeFunc = func(int) (PortHandoffResult, error) {
		called = true
		return PortHandoffResult{}, nil
	}
	t.Cleanup(func() { PlatformPortChangeFunc = previousBegin })

	response := platformRequest(t, newPlatformTestRouter(), http.MethodPut, "/api/admin/platforms/config", `{"port":0}`)
	if response["code"].(float64) == 0 {
		t.Fatalf("invalid port returned success: %#v", response)
	}
	if called {
		t.Fatal("invalid port reached the listener handoff")
	}
}

func TestPlatformPortConfirmPassesOneTimeToken(t *testing.T) {
	previousConfirm := PlatformPortConfirmFunc
	confirmed := ""
	PlatformPortConfirmFunc = func(token string) error { confirmed = token; return nil }
	t.Cleanup(func() { PlatformPortConfirmFunc = previousConfirm })

	response := platformRequest(t, newPlatformTestRouter(), http.MethodPost, "/api/admin/platforms/port/confirm", `{"confirmation_token":"confirm-me"}`)
	if response["code"].(float64) != 0 || confirmed != "confirm-me" {
		t.Fatalf("confirm response = %#v, token = %q", response, confirmed)
	}
}

func TestRegisterPlatformRoutesIncludesConfigAndPortConfirmation(t *testing.T) {
	router := newPlatformTestRouter()
	wanted := map[string]bool{
		"GET /api/admin/platforms/config":        false,
		"PUT /api/admin/platforms/config":        false,
		"POST /api/admin/platforms/port/confirm": false,
	}
	for _, route := range router.Routes() {
		key := route.Method + " " + route.Path
		if _, ok := wanted[key]; ok {
			wanted[key] = true
		}
	}
	for route, found := range wanted {
		if !found {
			t.Errorf("missing route %s", route)
		}
	}
}
