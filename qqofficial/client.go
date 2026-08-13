package qqofficial

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"qq-pet-saas/core"
)

const DefaultAPIBase = "https://api.bot.qq.com"

type SendResult struct {
	ID      string `json:"id"`
	AuditID string `json:"message_audit,omitempty"`
}

type Client struct {
	AppID           string
	Tokens          TokenSource
	BaseURL         string
	HTTPClient      *http.Client
	MarkdownEnabled bool
	KeyboardEnabled bool
	Limiter         *RateLimiter
	Status          *RuntimeStatus
}

func NewClient(appID string, tokens TokenSource, baseURL string, httpClient *http.Client) *Client {
	if baseURL == "" {
		baseURL = DefaultAPIBase
	}
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 15 * time.Second}
	}
	return &Client{AppID: appID, Tokens: tokens, BaseURL: strings.TrimRight(baseURL, "/"), HTTPClient: httpClient, Limiter: NewRateLimiter()}
}

func (client *Client) Send(ctx context.Context, event core.InboundEvent, message core.OutboundMessage) (*SendResult, error) {
	if client.Status != nil {
		client.Status.update(func(snapshot *RuntimeSnapshot) { snapshot.QueueDepth++ })
		defer client.Status.update(func(snapshot *RuntimeSnapshot) {
			if snapshot.QueueDepth > 0 {
				snapshot.QueueDepth--
			}
		})
	}
	if message.Text == "" {
		return nil, fmt.Errorf("QQ 消息缺少纯文本降级内容")
	}
	if client.Limiter != nil {
		if err := client.Limiter.Wait(ctx, event); err != nil {
			return nil, err
		}
	}
	rendered := message.Render(client.MarkdownEnabled, client.KeyboardEnabled)
	var endpoint string
	payload := make(map[string]interface{})
	switch event.Platform {
	case core.PlatformQQGroup:
		endpoint = fmt.Sprintf("%s/v2/groups/%s/messages", client.BaseURL, url.PathEscape(event.SpaceID))
		payload["content"] = rendered.Text
		payload["msg_type"] = 0
		if event.MessageID != "" {
			payload["msg_id"] = event.MessageID
			payload["msg_seq"] = 1
		}
		if rendered.Markdown != nil {
			payload["msg_type"] = 2
			payload["markdown"] = rendered.Markdown
		}
	case core.PlatformQQGuild:
		endpoint = fmt.Sprintf("%s/channels/%s/messages", client.BaseURL, url.PathEscape(event.RoomID))
		payload["content"] = rendered.Text
		if event.MessageID != "" {
			payload["msg_id"] = event.MessageID
		}
		if rendered.Markdown != nil {
			payload["markdown"] = rendered.Markdown
		}
	default:
		return nil, fmt.Errorf("不支持的平台: %s", event.Platform)
	}
	if rendered.Keyboard != nil {
		payload["keyboard"] = renderCommandKeyboard(rendered.Keyboard)
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	token, err := client.Tokens.Token(ctx)
	if err != nil {
		return nil, err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(encoded))
	if err != nil {
		return nil, err
	}
	request.Header.Set("Authorization", "QQBot "+token)
	request.Header.Set("Content-Type", "application/json")
	response, err := client.HTTPClient.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		var failure map[string]interface{}
		_ = json.NewDecoder(response.Body).Decode(&failure)
		return nil, fmt.Errorf("QQ 消息发送失败: HTTP %d %v", response.StatusCode, failure)
	}
	var result SendResult
	if err = json.NewDecoder(response.Body).Decode(&result); err != nil {
		return nil, err
	}
	if client.Status != nil {
		client.Status.markSend()
	}
	return &result, nil
}

func renderCommandKeyboard(keyboard *core.KeyboardPayload) map[string]interface{} {
	rows := make([]interface{}, 0, len(keyboard.Rows))
	for rowIndex, row := range keyboard.Rows {
		buttons := make([]interface{}, 0, len(row))
		for buttonIndex, button := range row {
			buttons = append(buttons, map[string]interface{}{
				"id":          fmt.Sprintf("pet-%d-%d", rowIndex, buttonIndex),
				"render_data": map[string]interface{}{"label": button.Label, "visited_label": button.Label, "style": 1},
				"action": map[string]interface{}{
					"type": 2, "permission": map[string]interface{}{"type": 2},
					"data": button.Command, "enter": true,
				},
			})
		}
		rows = append(rows, map[string]interface{}{"buttons": buttons})
	}
	return map[string]interface{}{"content": map[string]interface{}{"rows": rows}}
}

func (client *Client) AcknowledgeInteraction(ctx context.Context, interactionID, code string) error {
	if code == "" {
		code = "0"
	}
	body, _ := json.Marshal(map[string]string{"code": code})
	token, err := client.Tokens.Token(ctx)
	if err != nil {
		return err
	}
	endpoint := fmt.Sprintf("%s/interactions/%s", client.BaseURL, url.PathEscape(interactionID))
	request, err := http.NewRequestWithContext(ctx, http.MethodPut, endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	request.Header.Set("Authorization", "QQBot "+token)
	request.Header.Set("Content-Type", "application/json")
	response, err := client.HTTPClient.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("确认互动事件失败: HTTP %d", response.StatusCode)
	}
	return nil
}
