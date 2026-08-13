package qqofficial

import (
	"fmt"
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
	deduper.mu.Lock()
	defer deduper.mu.Unlock()
	now := deduper.Now()
	for key, expiresAt := range deduper.entries {
		if !expiresAt.After(now) {
			delete(deduper.entries, key)
		}
	}
	key := fmt.Sprintf("%s:%s:%s:%d", event.AppID, event.EventID, event.MessageID, event.MessageSeq)
	if expiresAt, exists := deduper.entries[key]; exists && expiresAt.After(now) {
		return false
	}
	deduper.entries[key] = now.Add(deduper.ttl)
	return true
}
