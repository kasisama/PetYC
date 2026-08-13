package qqofficial

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
	"qq-pet-saas/core"
)

type GatewayInfo struct {
	URL               string `json:"url"`
	Shards            int    `json:"shards"`
	SessionStartLimit struct {
		Total          int `json:"total"`
		Remaining      int `json:"remaining"`
		ResetAfter     int `json:"reset_after"`
		MaxConcurrency int `json:"max_concurrency"`
	} `json:"session_start_limit"`
}

type OfficialSender interface {
	Send(context.Context, core.InboundEvent, core.OutboundMessage) (*SendResult, error)
	AcknowledgeInteraction(context.Context, string, string) error
}

type Gateway struct {
	Config     Config
	Tokens     TokenSource
	Sender     OfficialSender
	Deduper    *Deduplicator
	HTTPClient *http.Client
	Dialer     *websocket.Dialer
	Route      func(context.Context, core.InboundEvent) (core.OutboundMessage, bool, error)
	Status     *RuntimeStatus
}

func NewGateway(config Config, tokens TokenSource, sender OfficialSender) *Gateway {
	return &Gateway{
		Config: config, Tokens: tokens, Sender: sender, Deduper: NewDeduplicator(10 * time.Minute),
		HTTPClient: &http.Client{Timeout: 15 * time.Second}, Dialer: websocket.DefaultDialer,
		Route: core.RouteInbound,
	}
}

func FetchGatewayInfo(ctx context.Context, apiBase string, tokens TokenSource, httpClient *http.Client) (*GatewayInfo, error) {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 15 * time.Second}
	}
	token, err := tokens.Token(ctx)
	if err != nil {
		return nil, err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(apiBase, "/")+"/gateway/bot", nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("Authorization", "QQBot "+token)
	response, err := httpClient.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, fmt.Errorf("获取 QQ Gateway 失败: HTTP %d", response.StatusCode)
	}
	var info GatewayInfo
	if err = json.NewDecoder(response.Body).Decode(&info); err != nil {
		return nil, err
	}
	if info.URL == "" {
		return nil, errors.New("QQ Gateway 响应缺少 url")
	}
	if info.Shards < 1 {
		info.Shards = 1
	}
	return &info, nil
}

func (gateway *Gateway) HandleDispatch(ctx context.Context, payload GatewayPayload) error {
	if gateway.Status != nil {
		gateway.Status.markEvent()
	}
	event, mapped, err := MapDispatch(gateway.Config.AppID, payload)
	if err != nil || !mapped {
		return err
	}
	if err = validateEvent(event); err != nil {
		return err
	}
	if !gateway.Deduper.Accept(event) {
		return nil
	}
	if payload.Type == "INTERACTION_CREATE" {
		if err = gateway.Sender.AcknowledgeInteraction(ctx, event.EventID, "0"); err != nil {
			return err
		}
	}
	message, handled, err := gateway.Route(ctx, event)
	if err != nil || !handled {
		return err
	}
	_, err = gateway.Sender.Send(ctx, event, message)
	return err
}

func (gateway *Gateway) Run(ctx context.Context) error {
	info, err := FetchGatewayInfo(ctx, gateway.Config.APIBase, gateway.Tokens, gateway.HTTPClient)
	if err != nil {
		if gateway.Status != nil {
			gateway.Status.markError(err)
		}
		return err
	}
	if gateway.Status != nil {
		gateway.Status.update(func(snapshot *RuntimeSnapshot) {
			snapshot.RecommendedShards = info.Shards
			snapshot.Connected = true
			snapshot.SessionState = "running"
		})
	}
	defer func() {
		if gateway.Status != nil {
			gateway.Status.update(func(snapshot *RuntimeSnapshot) {
				snapshot.Connected = false
				snapshot.ConnectedShards = 0
				snapshot.SessionState = "stopped"
			})
		}
	}()
	shardCount, err := resolveShardCount(info.Shards, gateway.Config.ShardCount)
	if err != nil {
		return err
	}
	if gateway.Status != nil {
		gateway.Status.update(func(snapshot *RuntimeSnapshot) { snapshot.ConnectedShards = shardCount })
	}
	var waitGroup sync.WaitGroup
	for shardID := 0; shardID < shardCount; shardID++ {
		waitGroup.Add(1)
		go func(id int) {
			defer waitGroup.Done()
			gateway.runShard(ctx, info.URL, id, shardCount)
		}(shardID)
	}
	<-ctx.Done()
	waitGroup.Wait()
	return ctx.Err()
}

func resolveShardCount(recommended, configured int) (int, error) {
	if recommended < 1 {
		recommended = 1
	}
	if configured == 0 || configured == recommended {
		return recommended, nil
	}
	return 0, fmt.Errorf("QQBOT_SHARD_COUNT=%d 与平台推荐分片数 %d 不一致，必须连接全部分片", configured, recommended)
}

type shardSession struct {
	ID       string
	Sequence atomic.Int64
}

func (gateway *Gateway) runShard(ctx context.Context, gatewayURL string, shardID, shardCount int) {
	session := shardSession{}
	for ctx.Err() == nil {
		err := gateway.runConnection(ctx, gatewayURL, shardID, shardCount, &session)
		if ctx.Err() != nil {
			return
		}
		log.Printf("[QQOfficial] 分片 %d 连接中断: %v，5 秒后重连", shardID, err)
		select {
		case <-ctx.Done():
			return
		case <-time.After(5 * time.Second):
		}
	}
}

func (gateway *Gateway) runConnection(ctx context.Context, gatewayURL string, shardID, shardCount int, session *shardSession) error {
	connection, _, err := gateway.Dialer.DialContext(ctx, gatewayURL, nil)
	if err != nil {
		return err
	}
	defer connection.Close()
	var hello GatewayPayload
	if err = connection.ReadJSON(&hello); err != nil {
		return err
	}
	if hello.Op != OpHello {
		return fmt.Errorf("QQ Gateway 首包不是 Hello: op=%d", hello.Op)
	}
	var helloData struct {
		HeartbeatInterval int `json:"heartbeat_interval"`
	}
	if err = json.Unmarshal(hello.Data, &helloData); err != nil {
		return err
	}
	if helloData.HeartbeatInterval <= 0 {
		return errors.New("QQ Gateway Hello 缺少心跳周期")
	}
	token, err := gateway.Tokens.Token(ctx)
	if err != nil {
		return err
	}
	if session.ID != "" {
		err = connection.WriteJSON(resumePayload(token, session.ID, int(session.Sequence.Load())))
	} else {
		err = connection.WriteJSON(identifyPayload(token, gateway.Config.Intents, shardID, shardCount))
	}
	if err != nil {
		return err
	}
	heartbeatCtx, cancelHeartbeat := context.WithCancel(ctx)
	defer cancelHeartbeat()
	heartbeatErrors := make(chan error, 1)
	go heartbeatLoop(heartbeatCtx, connection, time.Duration(helloData.HeartbeatInterval)*time.Millisecond, session, heartbeatErrors, gateway.Status)
	for {
		select {
		case heartbeatErr := <-heartbeatErrors:
			return heartbeatErr
		default:
		}
		var payload GatewayPayload
		if err = connection.ReadJSON(&payload); err != nil {
			return err
		}
		if payload.Sequence > 0 {
			session.Sequence.Store(int64(payload.Sequence))
		}
		switch payload.Op {
		case OpDispatch:
			if payload.Type == "READY" {
				var ready struct {
					SessionID string `json:"session_id"`
				}
				if err = json.Unmarshal(payload.Data, &ready); err != nil {
					return err
				}
				session.ID = ready.SessionID
				continue
			}
			if payload.Type == "RESUMED" {
				continue
			}
			if err = gateway.HandleDispatch(ctx, payload); err != nil {
				log.Printf("[QQOfficial] 事件 %s 处理失败: %v", payload.Type, err)
			}
		case OpReconnect:
			return errors.New("QQ Gateway 要求重连")
		case OpInvalidSession:
			session.ID = ""
			session.Sequence.Store(0)
			return errors.New("QQ Gateway Session 已失效")
		}
	}
}

func heartbeatLoop(ctx context.Context, connection *websocket.Conn, interval time.Duration, session *shardSession, errorsChannel chan<- error, status *RuntimeStatus) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if status != nil {
				status.markHeartbeat()
			}
			sequence := session.Sequence.Load()
			payload := map[string]interface{}{"op": OpHeartbeat, "d": sequence}
			if sequence == 0 {
				payload["d"] = nil
			}
			if err := connection.WriteJSON(payload); err != nil {
				select {
				case errorsChannel <- err:
				default:
				}
				return
			}
		}
	}
}

func StartFromEnv(ctx context.Context) (*Gateway, bool, error) {
	config, enabled, err := LoadConfig()
	if err != nil || !enabled {
		return nil, enabled, err
	}
	gateway, startErr := startConfigured(ctx, config)
	if startErr == nil {
		return gateway, true, nil
	}
	// Fallback to the original startup path if runtime initialization fails.
	httpClient := &http.Client{Timeout: 15 * time.Second}
	tokens := NewTokenProvider(config.AppID, config.Secret, config.TokenURL, httpClient)
	client := NewClient(config.AppID, tokens, config.APIBase, httpClient)
	client.MarkdownEnabled = config.MarkdownEnabled
	client.KeyboardEnabled = config.KeyboardEnabled
	legacyGateway := NewGateway(config, tokens, client)
	go func() {
		if runErr := legacyGateway.Run(ctx); runErr != nil && !errors.Is(runErr, context.Canceled) {
			log.Printf("[QQOfficial] 网关退出: %v", runErr)
		}
	}()
	return legacyGateway, true, nil
}
