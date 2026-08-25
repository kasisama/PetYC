package gameplay

import (
	"context"
	"strings"
	"time"

	"gorm.io/gorm"
)

const sqliteTransactionAttempts = 6

// WithTransactionRetry retries the whole unit of work after SQLite releases a
// write lock. Retrying individual statements would be unsafe because an
// earlier statement in the transaction may already have changed state.
func WithTransactionRetry(ctx context.Context, db *gorm.DB, work func(*gorm.DB) error) error {
	var lastErr error
	for attempt := 0; attempt < sqliteTransactionAttempts; attempt++ {
		lastErr = db.WithContext(ctx).Transaction(work)
		if !isSQLiteBusy(lastErr) {
			return lastErr
		}
		if attempt == sqliteTransactionAttempts-1 {
			break
		}
		delay := time.Duration(1<<attempt) * 2 * time.Millisecond
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
	return lastErr
}

func isSQLiteBusy(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "database is locked") ||
		strings.Contains(message, "database table is locked") ||
		strings.Contains(message, "database is deadlocked") ||
		strings.Contains(message, "sqlite_busy")
}
