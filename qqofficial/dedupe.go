package qqofficial

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"qq-pet-saas/core"
)

type Deduplicator struct {
	mu      sync.Mutex
	ttl     time.Duration
	entries map[string]time.Time
	Now     func() time.Time
}

func NewDeduplicator(ttl time.Duration) *Deduplicator {
	return &Deduplicator{ttl: ttl, entries: make(map[string]time.Time), Now: time.Now}
}

func (deduper *Deduplicator) Accept(event core.InboundEvent) bool {
	key, tracked := dedupeKey(event)
	if !tracked {
		return true
	}
	deduper.mu.Lock()
	defer deduper.mu.Unlock()
	now := deduper.Now()
	for key, expiresAt := range deduper.entries {
		if !expiresAt.After(now) {
			delete(deduper.entries, key)
		}
	}
	if expiresAt, exists := deduper.entries[key]; exists && expiresAt.After(now) {
		return false
	}
	deduper.entries[key] = now.Add(deduper.ttl)
	return true
}

func dedupeKey(event core.InboundEvent) (string, bool) {
	if messageID := strings.TrimSpace(event.MessageID); messageID != "" {
		return fmt.Sprintf("message:%s:%s:%s:%s:%s:%d", event.AppID, event.Platform, event.SceneType, event.SpaceID, messageID, event.MessageSeq), true
	}
	if eventID := strings.TrimSpace(event.EventID); eventID != "" {
		return fmt.Sprintf("event:%s:%s", event.AppID, eventID), true
	}
	return "", false
}
