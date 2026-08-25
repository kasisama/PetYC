package notifications

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"qq-pet-saas/models"
)

func notificationTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err = db.AutoMigrate(&models.NotificationPreference{}, &models.NotificationJob{}); err != nil {
		t.Fatal(err)
	}
	return db
}

func TestEnqueueIsIdempotentAndHonorsPreference(t *testing.T) {
	db := notificationTestDB(t)
	now := time.Date(2026, 8, 21, 12, 0, 0, 0, time.UTC)
	service := NewService(db)
	service.Now = func() time.Time { return now }
	request := EnqueueRequest{AccountID: "a1", IdempotencyKey: "expedition:r1:done", Kind: "expedition_done", Platform: "onebot", ActorName: "小满", MessageKey: "notification.expedition_done", Message: "回来啦", DueAt: now}
	created, err := service.Enqueue(context.Background(), request)
	if err != nil || !created {
		t.Fatalf("first enqueue = %v, %v", created, err)
	}
	var saved models.NotificationJob
	if err = db.First(&saved, "idempotency_key = ?", request.IdempotencyKey).Error; err != nil {
		t.Fatal(err)
	}
	if saved.ActorName != "小满" || saved.MessageKey != "notification.expedition_done" {
		t.Fatalf("notification target metadata was not preserved: %#v", saved)
	}
	created, err = service.Enqueue(context.Background(), request)
	if err != nil || created {
		t.Fatalf("duplicate enqueue = %v, %v", created, err)
	}
	if err = db.Create(&models.NotificationPreference{AccountID: "a2", Enabled: false}).Error; err != nil {
		t.Fatal(err)
	}
	request.AccountID, request.IdempotencyKey = "a2", "disabled"
	created, err = service.Enqueue(context.Background(), request)
	if err != nil || created {
		t.Fatalf("disabled enqueue = %v, %v", created, err)
	}
}

func TestClaimDueTreatsEmptyQueueAsIdle(t *testing.T) {
	db := notificationTestDB(t)
	service := NewService(db)
	job, err := service.ClaimDue(context.Background(), time.Minute)
	if job != nil {
		t.Fatalf("expected no job, got %#v", job)
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("empty queue error = %v", err)
	}
	worker := &Worker{Service: service, Send: func(context.Context, models.NotificationJob) error {
		t.Fatal("send should not run on an empty queue")
		return nil
	}}
	processed, err := worker.ProcessOne(context.Background())
	if err != nil || processed {
		t.Fatalf("idle process = %v, %v", processed, err)
	}
}

func TestWorkerRetriesWithBackoffThenMarksSent(t *testing.T) {
	db := notificationTestDB(t)
	now := time.Date(2026, 8, 21, 12, 0, 0, 0, time.UTC)
	service := NewService(db)
	service.Now = func() time.Time { return now }
	if _, err := service.Enqueue(context.Background(), EnqueueRequest{AccountID: "a1", IdempotencyKey: "job", Kind: "test", Platform: "onebot", Message: "hi", DueAt: now, MaxAttempts: 3}); err != nil {
		t.Fatal(err)
	}
	calls := 0
	worker := &Worker{Service: service, Send: func(context.Context, models.NotificationJob) error {
		calls++
		if calls == 1 {
			return errors.New("offline")
		}
		return nil
	}, Lease: time.Minute}
	processed, err := worker.ProcessOne(context.Background())
	if err != nil || !processed {
		t.Fatalf("first process = %v, %v", processed, err)
	}
	var job models.NotificationJob
	if err = db.First(&job, "idempotency_key = ?", "job").Error; err != nil {
		t.Fatal(err)
	}
	if job.Status != StatusQueued || job.Attempts != 1 || !job.NextAttemptAt.Equal(now.Add(time.Minute)) {
		t.Fatalf("retried job = %#v", job)
	}
	now = now.Add(time.Minute)
	processed, err = worker.ProcessOne(context.Background())
	if err != nil || !processed {
		t.Fatalf("second process = %v, %v", processed, err)
	}
	if err = db.First(&job, "idempotency_key = ?", "job").Error; err != nil {
		t.Fatal(err)
	}
	if job.Status != StatusSent || job.SentAt == nil {
		t.Fatalf("sent job = %#v", job)
	}
}
