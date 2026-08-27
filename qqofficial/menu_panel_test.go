package qqofficial

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync"
	"testing"
)

func TestClientSyncDiscoveryPublishesMenuAndAllPanelScopes(t *testing.T) {
	var mu sync.Mutex
	createdScopes := make([]string, 0, 4)
	menuUpdated := false
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Header.Get("Authorization") != "QQBot access-token" {
			t.Errorf("authorization = %q", request.Header.Get("Authorization"))
		}
		writer.Header().Set("Content-Type", "application/json")
		switch {
		case request.Method == http.MethodGet && request.URL.Path == "/v2/menu":
			_, _ = writer.Write([]byte(`{"version":3}`))
		case request.Method == http.MethodPut && request.URL.Path == "/v2/menu":
			var body struct {
				Menu Menu `json:"menu"`
			}
			if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
				t.Error(err)
			}
			if len(body.Menu.Items) != 2 || body.Menu.Items[0].SendMessage != "宠物菜单" {
				t.Errorf("menu = %#v", body.Menu)
			}
			menuUpdated = true
			_, _ = writer.Write([]byte(`{"version":4}`))
		case request.Method == http.MethodGet && request.URL.Path == "/v2/panels":
			if !validPanelScope(request.URL.Query().Get("scope")) || request.URL.Query().Get("limit") != "50" {
				t.Errorf("panel query = %s", request.URL.RawQuery)
			}
			_, _ = writer.Write([]byte(`{"records":[],"next_cursor":"","is_end":true}`))
		case request.Method == http.MethodPost && request.URL.Path == "/v2/panels":
			var body CreatePanelRequest
			if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
				t.Error(err)
			}
			if body.TargetType != "all" || body.Panel.Remark != managedPanelRemarkPrefix+body.Scope || len(body.Panel.Items) != 2 {
				t.Errorf("panel request = %#v", body)
			}
			mu.Lock()
			createdScopes = append(createdScopes, body.Scope)
			mu.Unlock()
			_ = json.NewEncoder(writer).Encode(CreatePanelResponse{PanelID: "panel-" + body.Scope})
		default:
			http.Error(writer, "unexpected request", http.StatusNotFound)
		}
	}))
	defer server.Close()

	client := NewClient("app", staticToken("access-token"), server.URL, server.Client())
	result, err := client.SyncDiscovery(context.Background(), []DiscoveryCommand{
		{Key: "help", Command: "玩法帮助", DisplayName: "帮助", Description: "查看玩法说明", SortOrder: 50},
		{Key: "pet_menu", Command: "宠物菜单", DisplayName: "宠物菜单", Description: "查看全部功能", SortOrder: 0},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !menuUpdated || result.MenuVersion != 4 || result.MenuAction != "updated" {
		t.Fatalf("menu result = %#v", result)
	}
	if !reflect.DeepEqual(createdScopes, []string{"c2c", "group", "channel", "dm"}) {
		t.Fatalf("created scopes = %#v", createdScopes)
	}
	for _, scope := range createdScopes {
		if result.Panels[scope].Action != "created" || result.Panels[scope].PanelID != "panel-"+scope {
			t.Fatalf("panel result[%s] = %#v", scope, result.Panels[scope])
		}
	}
}

func TestClientSyncDiscoverySkipsUnchangedManagedContent(t *testing.T) {
	commands := []DiscoveryCommand{{Key: "pet_menu", Command: "宠物菜单", DisplayName: "宠物菜单", Description: "查看全部功能"}}
	menu, basePanel := buildDiscoveryContent(commands)
	writes := 0
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		if request.Method != http.MethodGet {
			writes++
			http.Error(writer, "unexpected write", http.StatusConflict)
			return
		}
		if request.URL.Path == "/v2/menu" {
			_ = json.NewEncoder(writer).Encode(MenuResponse{Version: 7, Menu: &menu})
			return
		}
		scope := request.URL.Query().Get("scope")
		panel := basePanel
		panel.Remark = managedPanelRemarkPrefix + scope
		_ = json.NewEncoder(writer).Encode(PanelListResponse{Records: []PanelRecord{{PanelID: "p-" + scope, Scope: scope, TargetType: "all", Panel: panel, Version: 9}}, IsEnd: true})
	}))
	defer server.Close()

	client := NewClient("app", staticToken("token"), server.URL, server.Client())
	result, err := client.SyncDiscovery(context.Background(), commands)
	if err != nil {
		t.Fatal(err)
	}
	if writes != 0 || result.MenuAction != "unchanged" {
		t.Fatalf("writes = %d, result = %#v", writes, result)
	}
	for scope, item := range result.Panels {
		if item.Action != "unchanged" || item.Version != 9 {
			t.Fatalf("panel result[%s] = %#v", scope, item)
		}
	}
}

func TestMenuAndPanelValidationRejectsOfficialLimitViolations(t *testing.T) {
	menu := Menu{Items: make([]MenuItem, 11)}
	if err := validateMenu(menu); err == nil {
		t.Fatal("expected menu item limit error")
	}
	panel := Panel{Items: make([]PanelItem, 21)}
	if err := validatePanel(panel); err == nil {
		t.Fatal("expected panel item limit error")
	}
	if err := validatePanel(Panel{Items: []PanelItem{{Name: "官网", Type: "link", Link: "http://example.com"}}}); err == nil {
		t.Fatal("expected https validation error")
	}
}

func TestBuildDiscoveryContentFillsAvailableOfficialLimits(t *testing.T) {
	commands := make([]DiscoveryCommand, 0, 25)
	for index := 0; index < 25; index++ {
		commands = append(commands, DiscoveryCommand{
			Key: fmt.Sprintf("custom_%02d", index), Command: fmt.Sprintf("cmd%02d", index),
			DisplayName: fmt.Sprintf("入口%02d", index), Description: "自定义指令", SortOrder: index,
		})
	}
	menu, panel := buildDiscoveryContent(commands)
	if len(menu.Items) != 10 || len(panel.Items) != 20 {
		t.Fatalf("menu items = %d, panel items = %d", len(menu.Items), len(panel.Items))
	}
}

func TestSyncDiscoveryRejectsManagedPanelScopeDrift(t *testing.T) {
	commands := []DiscoveryCommand{{Key: "pet_menu", Command: "宠物菜单", DisplayName: "宠物菜单", Description: "查看全部功能"}}
	menu, panel := buildDiscoveryContent(commands)
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		if request.URL.Path == "/v2/menu" {
			_ = json.NewEncoder(writer).Encode(MenuResponse{Version: 1, Menu: &menu})
			return
		}
		panel.Remark = managedPanelRemarkPrefix + "c2c"
		_ = json.NewEncoder(writer).Encode(PanelListResponse{Records: []PanelRecord{{PanelID: "specific-panel", Scope: "c2c", TargetType: "specific", Panel: panel}}, IsEnd: true})
	}))
	defer server.Close()

	client := NewClient("app", staticToken("token"), server.URL, server.Client())
	_, err := client.SyncDiscovery(context.Background(), commands)
	if err == nil || !strings.Contains(err.Error(), "恢复为全局面板") {
		t.Fatalf("scope drift error = %v", err)
	}
}

func TestPanelRequestValidationRejectsMismatchedTargets(t *testing.T) {
	panel := Panel{Items: []PanelItem{{Name: "帮助", Type: "command"}}}
	cases := []CreatePanelRequest{
		{Scope: "c2c", TargetType: "all", UserOpenIDs: []string{"user"}, Panel: panel},
		{Scope: "c2c", TargetType: "specific", GroupOpenIDs: []string{"group"}, Panel: panel},
		{Scope: "group", TargetType: "specific", UserOpenIDs: []string{"user"}, Panel: panel},
		{Scope: "channel", TargetType: "all", GroupOpenIDs: []string{"group"}, Panel: panel},
	}
	for _, request := range cases {
		if err := validatePanelRequest(request); err == nil {
			t.Fatalf("expected target validation error for %#v", request)
		}
	}
}
