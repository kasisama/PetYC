package config

import (
	"testing"
	"time"
)

func TestConfigReadLockBlocksReloadWriter(t *testing.T) {
	LockForRead()
	writerAcquired := make(chan struct{})
	go func() {
		LockForWrite()
		close(writerAcquired)
		UnlockForWrite()
	}()

	select {
	case <-writerAcquired:
		t.Fatal("configuration writer acquired lock while a reader was active")
	case <-time.After(25 * time.Millisecond):
	}

	UnlockForRead()
	select {
	case <-writerAcquired:
	case <-time.After(time.Second):
		t.Fatal("configuration writer did not acquire lock after reader released it")
	}
}
