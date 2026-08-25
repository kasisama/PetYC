package qqofficial

import (
	"context"
	"errors"
	"net/http"
	"sync"
	"time"

	"qq-pet-saas/core"
)

type CapabilityStatus struct {
	Group       bool `json:"group"`
	Guild       bool `json:"guild"`
	Markdown    bool `json:"markdown"`
	Keyboard    bool `json:"keyboard"`
	Interaction bool `json:"interaction"`
	Audit       bool `json:"audit"`
}

type RuntimeSnapshot struct {
	Configured          bool             `json:"configured"`
	AppIDConfigured     bool             `json:"app_id_configured"`
	AppSecretConfigured bool             `json:"app_secret_configured"`
	MaskedAppID         string           `json:"masked_app_id"`
	Connected           bool             `json:"connected"`
	SessionState        string           `json:"session_state"`
	RecommendedShards   int              `json:"recommended_shards"`
	ConnectedShards     int              `json:"connected_shards"`
	LastHeartbeatAt     *time.Time       `json:"last_heartbeat_at"`
	LastEventAt         *time.Time       `json:"last_event_at"`
	LastSendAt          *time.Time       `json:"last_send_at"`
	QueueDepth          int              `json:"queue_depth"`
	LastError           string           `json:"last_error"`
	Capabilities        CapabilityStatus `json:"capabilities"`
}

type RuntimeStatus struct {
	mu       sync.RWMutex
	snapshot RuntimeSnapshot
}

func newRuntimeStatus(config Config) *RuntimeStatus {
	return &RuntimeStatus{snapshot: RuntimeSnapshot{
		Configured: true, AppIDConfigured: config.AppID != "", AppSecretConfigured: config.Secret != "",
		MaskedAppID: maskAppID(config.AppID), SessionState: "connecting",
		Capabilities: CapabilityStatus{Group: config.Intents&IntentsGroupAndC2C != 0, Guild: config.Intents&IntentsPublicGuildMessages != 0, Markdown: config.MarkdownEnabled, Keyboard: config.KeyboardEnabled, Interaction: config.Intents&IntentsInteraction != 0, Audit: config.Intents&IntentsMessageAudit != 0},
	}}
}

func maskAppID(value string) string {
	if value == "" {
		return ""
	}
	if len(value) <= 4 {
		return "****"
	}
	return "***" + value[len(value)-4:]
}

func (status *RuntimeStatus) update(update func(*RuntimeSnapshot)) {
	if status == nil {
		return
	}
	status.mu.Lock()
	defer status.mu.Unlock()
	update(&status.snapshot)
}

func (status *RuntimeStatus) Snapshot() RuntimeSnapshot {
	if status == nil {
		return RuntimeSnapshot{}
	}
	status.mu.RLock()
	defer status.mu.RUnlock()
	return status.snapshot
}

func (status *RuntimeStatus) markError(err error) {
	if err != nil {
		status.update(func(snapshot *RuntimeSnapshot) { snapshot.LastError = err.Error() })
	}
}
func (status *RuntimeStatus) markHeartbeat() {
	now := time.Now()
	status.update(func(snapshot *RuntimeSnapshot) { snapshot.LastHeartbeatAt = &now })
}
func (status *RuntimeStatus) markEvent() {
	now := time.Now()
	status.update(func(snapshot *RuntimeSnapshot) { snapshot.LastEventAt = &now })
}
func (status *RuntimeStatus) markSend() {
	now := time.Now()
	status.update(func(snapshot *RuntimeSnapshot) { snapshot.LastSendAt = &now; snapshot.LastError = "" })
}

var defaultRuntime struct {
	sync.Mutex
	status *RuntimeStatus
	client *Client
	cancel context.CancelFunc
}

func SendDefault(ctx context.Context, event core.InboundEvent, message core.OutboundMessage) error {
	defaultRuntime.Lock()
	client := defaultRuntime.client
	defaultRuntime.Unlock()
	if client == nil {
		return errors.New("QQ 官方机器人当前未连接")
	}
	_, err := client.Send(ctx, event, message)
	return err
}

func DefaultRuntimeSnapshot() RuntimeSnapshot {
	defaultRuntime.Lock()
	status := defaultRuntime.status
	defaultRuntime.Unlock()
	if status == nil {
		config, enabled, _ := LoadConfig()
		return RuntimeSnapshot{Configured: enabled, AppIDConfigured: config.AppID != "", AppSecretConfigured: config.Secret != "", MaskedAppID: maskAppID(config.AppID), SessionState: "not_started"}
	}
	return status.Snapshot()
}

func ReconnectDefault() error {
	config, enabled, err := LoadConfig()
	if err != nil {
		return err
	}
	if !enabled {
		return errors.New("QQ 官方机器人环境变量未配置")
	}
	_, err = startConfigured(context.Background(), config)
	return err
}

func ApplyDefaultConfig() error {
	config, enabled, err := LoadConfig()
	if err != nil {
		return err
	}
	if !enabled {
		defaultRuntime.Lock()
		cancel := defaultRuntime.cancel
		defaultRuntime.cancel = nil
		defaultRuntime.status = nil
		defaultRuntime.client = nil
		defaultRuntime.Unlock()
		if cancel != nil {
			cancel()
		}
		return nil
	}
	_, err = startConfigured(context.Background(), config)
	return err
}

func startConfigured(parent context.Context, config Config) (*Gateway, error) {
	ctx, cancel := context.WithCancel(parent)
	status := newRuntimeStatus(config)
	httpClient := &http.Client{Timeout: 15 * time.Second}
	tokens := NewTokenProvider(config.AppID, config.Secret, config.TokenURL, httpClient)
	client := NewClient(config.AppID, tokens, config.APIBase, httpClient)
	client.MarkdownEnabled, client.KeyboardEnabled, client.Status = config.MarkdownEnabled, config.KeyboardEnabled, status
	gateway := NewGateway(config, tokens, client)
	gateway.Status = status
	defaultRuntime.Lock()
	if defaultRuntime.cancel != nil {
		defaultRuntime.cancel()
	}
	defaultRuntime.cancel, defaultRuntime.status, defaultRuntime.client = cancel, status, client
	defaultRuntime.Unlock()
	go func() {
		if runErr := gateway.Run(ctx); runErr != nil && !errors.Is(runErr, context.Canceled) {
			status.markError(runErr)
		}
	}()
	return gateway, nil
}
