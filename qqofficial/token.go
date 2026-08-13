package qqofficial

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"time"
)

const DefaultTokenURL = "https://api.bot.qq.com/app/getAppAccessToken"

type TokenSource interface {
	Token(context.Context) (string, error)
}

type TokenProvider struct {
	appID      string
	secret     string
	tokenURL   string
	httpClient *http.Client
	mu         sync.Mutex
	token      string
	expiresAt  time.Time
	Now        func() time.Time
}

func NewTokenProvider(appID, secret, tokenURL string, httpClient *http.Client) *TokenProvider {
	if tokenURL == "" {
		tokenURL = DefaultTokenURL
	}
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 15 * time.Second}
	}
	return &TokenProvider{appID: appID, secret: secret, tokenURL: tokenURL, httpClient: httpClient, Now: time.Now}
}

func (provider *TokenProvider) Token(ctx context.Context) (string, error) {
	provider.mu.Lock()
	defer provider.mu.Unlock()
	now := provider.Now()
	if provider.token != "" && now.Add(60*time.Second).Before(provider.expiresAt) {
		return provider.token, nil
	}
	body, err := json.Marshal(map[string]string{"appId": provider.appID, "clientSecret": provider.secret})
	if err != nil {
		return "", err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, provider.tokenURL, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := provider.httpClient.Do(request)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return "", fmt.Errorf("获取 QQ Access Token 失败: HTTP %d", response.StatusCode)
	}
	var payload struct {
		AccessToken string      `json:"access_token"`
		ExpiresIn   interface{} `json:"expires_in"`
	}
	if err = json.NewDecoder(response.Body).Decode(&payload); err != nil {
		return "", err
	}
	if payload.AccessToken == "" {
		return "", errors.New("QQ Access Token 响应缺少 access_token")
	}
	expiresIn, err := parseExpiresIn(payload.ExpiresIn)
	if err != nil {
		return "", err
	}
	provider.token = payload.AccessToken
	provider.expiresAt = now.Add(time.Duration(expiresIn) * time.Second)
	return provider.token, nil
}

func parseExpiresIn(value interface{}) (int64, error) {
	switch typed := value.(type) {
	case string:
		return strconv.ParseInt(typed, 10, 64)
	case float64:
		return int64(typed), nil
	default:
		return 0, fmt.Errorf("无法解析 expires_in: %v", value)
	}
}
