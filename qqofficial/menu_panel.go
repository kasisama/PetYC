package qqofficial

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"reflect"
	"sort"
	"strings"
	"sync"
	"unicode/utf8"
)

const managedPanelRemarkPrefix = "qq-pet-saas:auto:"

var discoverySyncMu sync.Mutex

type Menu struct {
	Items []MenuItem `json:"items,omitempty"`
}

type MenuItem struct {
	Name         string        `json:"name,omitempty"`
	Type         string        `json:"type,omitempty"`
	SubMenuItems []SubMenuItem `json:"sub_menu_items,omitempty"`
	SendMessage  string        `json:"send_message,omitempty"`
	Link         string        `json:"link,omitempty"`
	Switch       *MenuSwitch   `json:"switch,omitempty"`
}

type SubMenuItem struct {
	Name        string `json:"name,omitempty"`
	Type        string `json:"type,omitempty"`
	SendMessage string `json:"send_message,omitempty"`
	Link        string `json:"link,omitempty"`
}

type MenuSwitch struct {
	SwitchID string `json:"switch_id,omitempty"`
	Default  bool   `json:"default"`
}

type MenuResponse struct {
	Version int   `json:"version"`
	Menu    *Menu `json:"menu,omitempty"`
}

type Panel struct {
	Items   []PanelItem `json:"items,omitempty"`
	Remark  string      `json:"remark,omitempty"`
	Version int         `json:"version,omitempty"`
}

type PanelItem struct {
	Name      string `json:"name,omitempty"`
	Desc      string `json:"desc,omitempty"`
	Type      string `json:"type,omitempty"`
	OnlyAdmin bool   `json:"only_admin"`
	Link      string `json:"link,omitempty"`
}

type PanelRecord struct {
	PanelID      string   `json:"panel_id"`
	Scope        string   `json:"scope"`
	TargetType   string   `json:"target_type"`
	Panel        Panel    `json:"panel"`
	CreatedAt    string   `json:"created_at,omitempty"`
	UpdatedAt    string   `json:"updated_at,omitempty"`
	Version      int      `json:"version"`
	UserOpenIDs  []string `json:"user_openids,omitempty"`
	GroupOpenIDs []string `json:"group_openids,omitempty"`
}

type PanelListResponse struct {
	Records    []PanelRecord `json:"records"`
	NextCursor string        `json:"next_cursor"`
	IsEnd      bool          `json:"is_end"`
}

type CreatePanelRequest struct {
	Scope        string   `json:"scope"`
	TargetType   string   `json:"target_type,omitempty"`
	UserOpenIDs  []string `json:"user_openids,omitempty"`
	GroupOpenIDs []string `json:"group_openids,omitempty"`
	Panel        Panel    `json:"panel"`
}

type CreatePanelResponse struct {
	PanelID string `json:"panel_id"`
}

type PanelVersionResponse struct {
	Version int `json:"version"`
}

type UpdatePanelTargetsRequest struct {
	Operation    string   `json:"op"`
	UserOpenIDs  []string `json:"user_openids,omitempty"`
	GroupOpenIDs []string `json:"group_openids,omitempty"`
}

type DiscoveryCommand struct {
	Key         string
	Command     string
	DisplayName string
	Description string
	SortOrder   int
}

type DiscoverySyncItem struct {
	PanelID string `json:"panel_id"`
	Version int    `json:"version"`
	Action  string `json:"action"`
}

type DiscoverySyncResult struct {
	MenuVersion int                          `json:"menu_version"`
	MenuAction  string                       `json:"menu_action"`
	Panels      map[string]DiscoverySyncItem `json:"panels"`
}

type ControlAPIError struct {
	StatusCode int
	Code       int
	Message    string
}

func (err *ControlAPIError) Error() string {
	if err.Code != 0 {
		return fmt.Sprintf("QQ 控制接口失败: HTTP %d code=%d %s", err.StatusCode, err.Code, err.Message)
	}
	return fmt.Sprintf("QQ 控制接口失败: HTTP %d %s", err.StatusCode, err.Message)
}

func (client *Client) GetMenu(ctx context.Context) (MenuResponse, error) {
	var result MenuResponse
	err := client.doControlJSON(ctx, http.MethodGet, "/v2/menu", nil, nil, &result)
	return result, err
}

func (client *Client) PutMenu(ctx context.Context, menu Menu) (MenuResponse, error) {
	if err := validateMenu(menu); err != nil {
		return MenuResponse{}, err
	}
	var result MenuResponse
	err := client.doControlJSON(ctx, http.MethodPut, "/v2/menu", nil, map[string]Menu{"menu": menu}, &result)
	return result, err
}

func (client *Client) ListPanels(ctx context.Context, scope, cursor string, limit int) (PanelListResponse, error) {
	if !validPanelScope(scope) {
		return PanelListResponse{}, fmt.Errorf("不支持的指令面板场景: %s", scope)
	}
	query := url.Values{"scope": []string{scope}}
	if strings.TrimSpace(cursor) != "" {
		query.Set("cursor", cursor)
	}
	if limit > 0 {
		if limit > 50 {
			return PanelListResponse{}, errors.New("指令面板分页大小不能超过 50")
		}
		query.Set("limit", fmt.Sprintf("%d", limit))
	}
	var result PanelListResponse
	err := client.doControlJSON(ctx, http.MethodGet, "/v2/panels", query, nil, &result)
	return result, err
}

func (client *Client) GetPanel(ctx context.Context, panelID string) (PanelRecord, error) {
	if strings.TrimSpace(panelID) == "" {
		return PanelRecord{}, errors.New("指令面板 ID 不能为空")
	}
	var result PanelRecord
	err := client.doControlJSON(ctx, http.MethodGet, "/v2/panels/"+url.PathEscape(panelID), nil, nil, &result)
	return result, err
}

func (client *Client) CreatePanel(ctx context.Context, request CreatePanelRequest) (CreatePanelResponse, error) {
	if err := validatePanelRequest(request); err != nil {
		return CreatePanelResponse{}, err
	}
	var result CreatePanelResponse
	err := client.doControlJSON(ctx, http.MethodPost, "/v2/panels", nil, request, &result)
	return result, err
}

func (client *Client) UpdatePanel(ctx context.Context, panelID string, panel Panel) (PanelVersionResponse, error) {
	if strings.TrimSpace(panelID) == "" {
		return PanelVersionResponse{}, errors.New("指令面板 ID 不能为空")
	}
	if err := validatePanel(panel); err != nil {
		return PanelVersionResponse{}, err
	}
	var result PanelVersionResponse
	err := client.doControlJSON(ctx, http.MethodPut, "/v2/panels/"+url.PathEscape(panelID), nil, map[string]Panel{"panel": panel}, &result)
	return result, err
}

func (client *Client) DeletePanel(ctx context.Context, panelID string) error {
	if strings.TrimSpace(panelID) == "" {
		return errors.New("指令面板 ID 不能为空")
	}
	return client.doControlJSON(ctx, http.MethodDelete, "/v2/panels/"+url.PathEscape(panelID), nil, nil, nil)
}

func (client *Client) UpdatePanelTargets(ctx context.Context, panelID string, request UpdatePanelTargetsRequest) error {
	if strings.TrimSpace(panelID) == "" {
		return errors.New("指令面板 ID 不能为空")
	}
	if request.Operation != "add" && request.Operation != "del" {
		return errors.New("关联对象操作仅支持 add 或 del")
	}
	if len(request.UserOpenIDs) == 0 && len(request.GroupOpenIDs) == 0 {
		return errors.New("关联对象不能为空")
	}
	if len(request.UserOpenIDs) > 0 && len(request.GroupOpenIDs) > 0 {
		return errors.New("单次只能修改用户或群中的一种关联对象")
	}
	if len(request.UserOpenIDs) > 20 || len(request.GroupOpenIDs) > 20 {
		return errors.New("单次最多修改 20 个关联对象")
	}
	return client.doControlJSON(ctx, http.MethodPut, "/v2/panels/"+url.PathEscape(panelID)+"/target", nil, request, nil)
}

func (client *Client) SyncDiscovery(ctx context.Context, commands []DiscoveryCommand) (DiscoverySyncResult, error) {
	discoverySyncMu.Lock()
	defer discoverySyncMu.Unlock()
	menu, panel := buildDiscoveryContent(commands)
	if len(menu.Items) == 0 || len(panel.Items) == 0 {
		return DiscoverySyncResult{}, errors.New("没有可同步的已启用指令")
	}
	result := DiscoverySyncResult{Panels: make(map[string]DiscoverySyncItem)}
	currentMenu, err := client.GetMenu(ctx)
	if err != nil {
		return result, discoverySyncError(result, fmt.Errorf("查询自定义菜单失败: %w", err))
	}
	if currentMenu.Menu != nil && reflect.DeepEqual(*currentMenu.Menu, menu) {
		result.MenuVersion, result.MenuAction = currentMenu.Version, "unchanged"
	} else {
		updated, updateErr := client.PutMenu(ctx, menu)
		if updateErr != nil {
			return result, discoverySyncError(result, fmt.Errorf("同步自定义菜单失败: %w", updateErr))
		}
		result.MenuVersion, result.MenuAction = updated.Version, "updated"
	}
	for _, scope := range []string{"c2c", "group", "channel", "dm"} {
		item, syncErr := client.syncManagedPanel(ctx, scope, panel)
		if syncErr != nil {
			return result, discoverySyncError(result, fmt.Errorf("同步 %s 指令面板失败: %w", scope, syncErr))
		}
		result.Panels[scope] = item
	}
	return result, nil
}

func SyncDefaultDiscovery(ctx context.Context, commands []DiscoveryCommand) (DiscoverySyncResult, error) {
	defaultRuntime.Lock()
	client := defaultRuntime.client
	defaultRuntime.Unlock()
	if client == nil {
		return DiscoverySyncResult{}, errors.New("QQ 官方机器人当前未连接")
	}
	return client.SyncDiscovery(ctx, commands)
}

func (client *Client) syncManagedPanel(ctx context.Context, scope string, base Panel) (DiscoverySyncItem, error) {
	panel := base
	panel.Remark = managedPanelRemarkPrefix + scope
	matches := make([]PanelRecord, 0, 1)
	cursor := ""
	visited := make(map[string]struct{})
	ended := false
	for pageNumber := 0; pageNumber < 20; pageNumber++ {
		if _, exists := visited[cursor]; exists {
			return DiscoverySyncItem{}, errors.New("指令面板分页游标重复")
		}
		visited[cursor] = struct{}{}
		page, err := client.ListPanels(ctx, scope, cursor, 50)
		if err != nil {
			return DiscoverySyncItem{}, err
		}
		for index := range page.Records {
			if page.Records[index].Panel.Remark == panel.Remark {
				matches = append(matches, page.Records[index])
			}
		}
		if page.IsEnd || page.NextCursor == "" {
			ended = true
			break
		}
		cursor = page.NextCursor
	}
	if !ended {
		return DiscoverySyncItem{}, errors.New("指令面板分页超过安全上限")
	}
	if len(matches) > 1 {
		return DiscoverySyncItem{}, fmt.Errorf("发现 %d 个重复的托管面板，请先在 QQ 开放平台删除重复项", len(matches))
	}
	if len(matches) == 0 {
		created, err := client.CreatePanel(ctx, CreatePanelRequest{Scope: scope, TargetType: "all", Panel: panel})
		return DiscoverySyncItem{PanelID: created.PanelID, Action: "created"}, err
	}
	existing := matches[0]
	if existing.TargetType != "all" {
		return DiscoverySyncItem{}, fmt.Errorf("托管面板 %s 已被改为 %s 范围，请先在 QQ 开放平台恢复为全局面板", existing.PanelID, existing.TargetType)
	}
	comparable := existing.Panel
	comparable.Version = 0
	if reflect.DeepEqual(comparable, panel) {
		return DiscoverySyncItem{PanelID: existing.PanelID, Version: existing.Version, Action: "unchanged"}, nil
	}
	updated, err := client.UpdatePanel(ctx, existing.PanelID, panel)
	return DiscoverySyncItem{PanelID: existing.PanelID, Version: updated.Version, Action: "updated"}, err
}

func discoverySyncError(result DiscoverySyncResult, cause error) error {
	completed := make([]string, 0, len(result.Panels)+1)
	if result.MenuAction != "" {
		completed = append(completed, "menu="+result.MenuAction)
	}
	for _, scope := range []string{"c2c", "group", "channel", "dm"} {
		if item, exists := result.Panels[scope]; exists {
			completed = append(completed, scope+"="+item.Action)
		}
	}
	if len(completed) == 0 {
		return cause
	}
	return fmt.Errorf("%w（已完成: %s）", cause, strings.Join(completed, ", "))
}

func buildDiscoveryContent(commands []DiscoveryCommand) (Menu, Panel) {
	commands = append([]DiscoveryCommand(nil), commands...)
	sort.SliceStable(commands, func(i, j int) bool { return commands[i].SortOrder < commands[j].SortOrder })
	byKey := make(map[string]DiscoveryCommand, len(commands))
	for index := range commands {
		commands[index].Key = strings.TrimSpace(commands[index].Key)
		commands[index].Command = strings.TrimSpace(commands[index].Command)
		if commands[index].Key != "" && commands[index].Command != "" {
			byKey[commands[index].Key] = commands[index]
		}
	}
	menuKeys := []string{"pet_menu", "pet_status", "daily_checkin", "inventory", "shop", "expedition", "adventure_maps", "community", "live_event", "help"}
	panelKeys := []string{"pet_menu", "adopt_list", "pet_status", "pet_list", "daily_checkin", "inventory", "shop", "feed", "touch", "walk", "work", "evolution", "expedition", "adventure_maps", "adventure_boss", "fishing", "lottery", "community", "live_event", "help"}
	menu := Menu{Items: make([]MenuItem, 0, len(menuKeys))}
	menuUsed := make(map[string]bool, len(menuKeys))
	appendMenu := func(command DiscoveryCommand) {
		if len(menu.Items) >= 10 || command.Key == "" || command.Command == "" || menuUsed[command.Key] {
			return
		}
		label := strings.TrimSpace(command.DisplayName)
		if label == "" || weightedLength(label) > 10 {
			label = truncateWeighted(command.Command, 10)
		}
		menu.Items = append(menu.Items, MenuItem{Name: label, Type: "send_message", SendMessage: command.Command})
		menuUsed[command.Key] = true
	}
	for _, key := range menuKeys {
		command, exists := byKey[key]
		if !exists {
			continue
		}
		appendMenu(command)
	}
	for _, command := range commands {
		appendMenu(command)
	}
	panel := Panel{Items: make([]PanelItem, 0, len(panelKeys))}
	panelUsed := make(map[string]bool, len(panelKeys))
	appendPanel := func(command DiscoveryCommand) {
		if len(panel.Items) >= 20 || command.Key == "" || command.Command == "" || panelUsed[command.Key] || weightedLength(command.Command) > 14 {
			return
		}
		panel.Items = append(panel.Items, PanelItem{Name: command.Command, Desc: truncateWeighted(command.Description, 30), Type: "command"})
		panelUsed[command.Key] = true
	}
	for _, key := range panelKeys {
		command, exists := byKey[key]
		if !exists {
			continue
		}
		appendPanel(command)
	}
	for _, command := range commands {
		appendPanel(command)
	}
	return menu, panel
}

func (client *Client) doControlJSON(ctx context.Context, method, path string, query url.Values, payload, result interface{}) error {
	if client == nil || client.Tokens == nil || client.HTTPClient == nil {
		return errors.New("QQ 官方客户端未初始化")
	}
	token, err := client.Tokens.Token(ctx)
	if err != nil {
		return err
	}
	var body io.Reader
	if payload != nil {
		encoded, encodeErr := json.Marshal(payload)
		if encodeErr != nil {
			return encodeErr
		}
		body = bytes.NewReader(encoded)
	}
	endpoint := client.BaseURL + path
	if len(query) > 0 {
		endpoint += "?" + query.Encode()
	}
	request, err := http.NewRequestWithContext(ctx, method, endpoint, body)
	if err != nil {
		return err
	}
	request.Header.Set("Authorization", "QQBot "+token)
	if payload != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	response, err := client.HTTPClient.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		failure, _ := io.ReadAll(io.LimitReader(response.Body, 4096))
		var payload struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
			Msg     string `json:"msg"`
		}
		_ = json.Unmarshal(failure, &payload)
		message := strings.TrimSpace(payload.Message)
		if message == "" {
			message = strings.TrimSpace(payload.Msg)
		}
		if message == "" {
			message = http.StatusText(response.StatusCode)
		}
		message = truncateRunes(message, 256)
		return &ControlAPIError{StatusCode: response.StatusCode, Code: payload.Code, Message: message}
	}
	if result == nil || response.StatusCode == http.StatusNoContent {
		return nil
	}
	if err := json.NewDecoder(response.Body).Decode(result); err != nil && !errors.Is(err, io.EOF) {
		return err
	}
	return nil
}

func validateMenu(menu Menu) error {
	if len(menu.Items) > 10 {
		return errors.New("自定义菜单最多包含 10 个菜单项")
	}
	for _, item := range menu.Items {
		if strings.TrimSpace(item.Name) == "" || weightedLength(item.Name) > 10 {
			return errors.New("菜单项名称不能为空且最多 10 个字符")
		}
		switch item.Type {
		case "send_message":
			if strings.TrimSpace(item.SendMessage) == "" {
				return errors.New("发送消息菜单项缺少消息内容")
			}
		case "link":
			if !validHTTPSURL(item.Link) {
				return errors.New("菜单链接必须使用 https")
			}
		case "switch":
			if item.Switch == nil || strings.TrimSpace(item.Switch.SwitchID) == "" {
				return errors.New("开关菜单项缺少 switch_id")
			}
		case "menu":
			if len(item.SubMenuItems) == 0 || len(item.SubMenuItems) > 5 {
				return errors.New("折叠菜单必须包含 1 到 5 个子菜单项")
			}
			for _, child := range item.SubMenuItems {
				if strings.TrimSpace(child.Name) == "" || weightedLength(child.Name) > 14 {
					return errors.New("子菜单项名称不能为空且最多 14 个字符")
				}
				if child.Type == "send_message" && strings.TrimSpace(child.SendMessage) != "" {
					continue
				}
				if child.Type == "link" && validHTTPSURL(child.Link) {
					continue
				}
				return errors.New("子菜单项仅支持有效的 send_message 或 https link")
			}
		default:
			return fmt.Errorf("不支持的菜单项类型: %s", item.Type)
		}
	}
	return nil
}

func validatePanelRequest(request CreatePanelRequest) error {
	if !validPanelScope(request.Scope) {
		return fmt.Errorf("不支持的指令面板场景: %s", request.Scope)
	}
	if request.TargetType == "" {
		request.TargetType = "all"
	}
	if request.TargetType != "all" && request.TargetType != "specific" {
		return errors.New("指令面板作用范围仅支持 all 或 specific")
	}
	if (request.Scope == "channel" || request.Scope == "dm") && request.TargetType != "all" {
		return errors.New("channel 和 dm 指令面板仅支持全局配置")
	}
	if request.TargetType == "all" && (len(request.UserOpenIDs) > 0 || len(request.GroupOpenIDs) > 0) {
		return errors.New("全局指令面板不能携带指定关联对象")
	}
	if request.Scope == "c2c" && len(request.GroupOpenIDs) > 0 {
		return errors.New("c2c 指令面板不能关联群 openid")
	}
	if request.Scope == "group" && len(request.UserOpenIDs) > 0 {
		return errors.New("group 指令面板不能关联用户 openid")
	}
	if (request.Scope == "channel" || request.Scope == "dm") && (len(request.UserOpenIDs) > 0 || len(request.GroupOpenIDs) > 0) {
		return errors.New("channel 和 dm 指令面板不能关联指定对象")
	}
	if len(request.UserOpenIDs) > 20 || len(request.GroupOpenIDs) > 20 {
		return errors.New("创建面板时最多关联 20 个用户或群")
	}
	return validatePanel(request.Panel)
}

func validatePanel(panel Panel) error {
	if len(panel.Items) > 20 {
		return errors.New("指令面板最多包含 20 个元素")
	}
	if weightedLength(panel.Remark) > 255 {
		return errors.New("指令面板备注最多 255 个字符")
	}
	for _, item := range panel.Items {
		if strings.TrimSpace(item.Name) == "" || weightedLength(item.Name) > 14 {
			return errors.New("面板元素名称不能为空且最多 14 个字符")
		}
		if weightedLength(item.Desc) > 30 {
			return errors.New("面板元素描述最多 30 个字符")
		}
		switch item.Type {
		case "command":
		case "link":
			if !validHTTPSURL(item.Link) {
				return errors.New("面板链接必须使用 https")
			}
		default:
			return fmt.Errorf("不支持的面板元素类型: %s", item.Type)
		}
	}
	return nil
}

func validPanelScope(scope string) bool {
	return scope == "c2c" || scope == "group" || scope == "channel" || scope == "dm"
}

func validHTTPSURL(value string) bool {
	parsed, err := url.Parse(strings.TrimSpace(value))
	return err == nil && parsed.Scheme == "https" && parsed.Host != ""
}

func weightedLength(value string) int {
	length := 0
	for _, character := range value {
		if character <= 127 {
			length++
		} else {
			length += 2
		}
	}
	return length
}

func truncateWeighted(value string, limit int) string {
	value = strings.TrimSpace(value)
	if weightedLength(value) <= limit {
		return value
	}
	var builder strings.Builder
	used := 0
	for len(value) > 0 {
		character, size := utf8.DecodeRuneInString(value)
		weight := 1
		if character > 127 {
			weight = 2
		}
		if used+weight > limit {
			break
		}
		builder.WriteRune(character)
		used += weight
		value = value[size:]
	}
	return builder.String()
}

func truncateRunes(value string, limit int) string {
	runes := []rune(value)
	if len(runes) <= limit {
		return value
	}
	return string(runes[:limit])
}
