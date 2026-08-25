package qqofficial

import (
	"context"
	"sync"
	"time"

	"qq-pet-saas/core"
)

type RateLimiter struct {
	mu         sync.Mutex
	nextScene  map[string]time.Time
	nextGlobal map[string]time.Time
}

func NewRateLimiter() *RateLimiter {
	return &RateLimiter{nextScene: make(map[string]time.Time), nextGlobal: make(map[string]time.Time)}
}

func (limiter *RateLimiter) Reserve(event core.InboundEvent, now time.Time) time.Duration {
	limiter.mu.Lock()
	defer limiter.mu.Unlock()
	scheduled := now
	var sceneKey, globalKey string
	var sceneInterval, globalInterval time.Duration
	switch event.Platform {
	case core.PlatformQQGroup:
		if event.SceneType == core.SceneDirect {
			sceneKey = "c2c:" + event.AppID + ":" + event.ActorID
			globalKey = "c2c-global:" + event.AppID
		} else {
			sceneKey = "group:" + event.AppID + ":" + event.SpaceID
			globalKey = "group-global:" + event.AppID
		}
		sceneInterval = 3 * time.Second
		globalInterval = 2 * time.Second
	case core.PlatformQQGuild:
		sceneKey = "guild:" + event.AppID + ":" + event.RoomID
		sceneInterval = 200 * time.Millisecond
	default:
		return 0
	}
	if next := limiter.nextScene[sceneKey]; next.After(scheduled) {
		scheduled = next
	}
	if globalKey != "" {
		if next := limiter.nextGlobal[globalKey]; next.After(scheduled) {
			scheduled = next
		}
		limiter.nextGlobal[globalKey] = scheduled.Add(globalInterval)
	}
	limiter.nextScene[sceneKey] = scheduled.Add(sceneInterval)
	return scheduled.Sub(now)
}

func (limiter *RateLimiter) Wait(ctx context.Context, event core.InboundEvent) error {
	delay := limiter.Reserve(event, time.Now())
	if delay <= 0 {
		return nil
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
