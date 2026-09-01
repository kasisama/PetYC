package notifications

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"qq-pet-saas/models"
)

const (
	StatusQueued    = "queued"
	StatusSending   = "sending"
	StatusSent      = "sent"
	StatusDead      = "dead"
	StatusCancelled = "cancelled"
)

type EnqueueRequest struct {
	AccountID      string
	IdempotencyKey string
	Kind           string
	Platform       string
	SceneType      string
	AppID          string
	SpaceID        string
	RoomID         string
	ActorID        string
	ActorName      string
	MessageKey     string
	Message        string
	DueAt          time.Time
	MaxAttempts    int
}

type Service struct {
	DB  *gorm.DB
	Now func() time.Time
}

func NewService(db *gorm.DB) *Service {
	return &Service{DB: db, Now: time.Now}
}

func (service *Service) Enqueue(ctx context.Context, request EnqueueRequest) (bool, error) {
	if service == nil || service.DB == nil {
		return false, errors.New("notification database unavailable")
	}
	var created bool
	err := service.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var err error
		created, err = service.EnqueueTx(tx, request)
		return err
	})
	return created, err
}

func (service *Service) EnqueueTx(tx *gorm.DB, request EnqueueRequest) (bool, error) {
	request.AccountID = strings.TrimSpace(request.AccountID)
	request.IdempotencyKey = strings.TrimSpace(request.IdempotencyKey)
	request.Kind = strings.TrimSpace(request.Kind)
	request.Message = strings.TrimSpace(request.Message)
	if request.AccountID == "" || request.IdempotencyKey == "" || request.Kind == "" || request.Platform == "" || request.Message == "" {
		return false, errors.New("notification request is incomplete")
	}
	var preference models.NotificationPreference
	err := tx.First(&preference, "account_id = ?", request.AccountID).Error
	if err == nil && !preference.Enabled {
		return false, nil
	}
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return false, err
	}
	now := service.now()
	if request.DueAt.IsZero() || request.DueAt.Before(now) {
		request.DueAt = now
	}
	if request.MaxAttempts <= 0 {
		request.MaxAttempts = 8
	}
	job := models.NotificationJob{
		ID: uuid.NewString(), AccountID: request.AccountID, IdempotencyKey: request.IdempotencyKey,
		Kind: request.Kind, Platform: request.Platform, SceneType: request.SceneType, AppID: request.AppID,
		SpaceID: request.SpaceID, RoomID: request.RoomID, ActorID: request.ActorID, ActorName: request.ActorName,
		MessageKey: request.MessageKey, Message: request.Message,
		Status: StatusQueued, MaxAttempts: request.MaxAttempts, NextAttemptAt: request.DueAt,
		CreatedAt: now, UpdatedAt: now,
	}
	result := tx.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "idempotency_key"}}, DoNothing: true}).Create(&job)
	return result.RowsAffected == 1, result.Error
}

// ClaimDue leases one due job. A stale sending lease can be reclaimed after
// a process crash; the idempotency key still prevents a second job row.
func (service *Service) ClaimDue(ctx context.Context, lease time.Duration) (*models.NotificationJob, error) {
	if service == nil || service.DB == nil {
		return nil, errors.New("notification database unavailable")
	}
	if lease <= 0 {
		lease = 2 * time.Minute
	}
	now := service.now()
	staleBefore := now.Add(-lease)
	var claimed models.NotificationJob
	err := service.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var candidate models.NotificationJob
		// Use Find instead of First: an empty queue is the idle state, not a
		// GORM error, and First() logs "record not found" on every poll.
		result := tx.Where("(status = ? AND next_attempt_at <= ?) OR (status = ? AND locked_at < ?)",
			StatusQueued, now, StatusSending, staleBefore).
			Order("next_attempt_at asc, created_at asc").Limit(1).Find(&candidate)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return gorm.ErrRecordNotFound
		}
		result = tx.Model(&models.NotificationJob{}).
			Where("id = ? AND ((status = ? AND next_attempt_at <= ?) OR (status = ? AND locked_at < ?))",
				candidate.ID, StatusQueued, now, StatusSending, staleBefore).
			Updates(map[string]interface{}{"status": StatusSending, "locked_at": now, "updated_at": now})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return gorm.ErrRecordNotFound
		}
		candidate.Status = StatusSending
		candidate.LockedAt = &now
		claimed = candidate
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &claimed, nil
}

func (service *Service) MarkCancelled(ctx context.Context, jobID string) error {
	now := service.now()
	result := service.DB.WithContext(ctx).Model(&models.NotificationJob{}).
		Where("id = ? AND status = ?", jobID, StatusSending).
		Updates(map[string]interface{}{"status": StatusCancelled, "locked_at": nil, "last_error": "account banned", "updated_at": now})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (service *Service) MarkSent(ctx context.Context, jobID string) error {
	now := service.now()
	result := service.DB.WithContext(ctx).Model(&models.NotificationJob{}).
		Where("id = ? AND status = ?", jobID, StatusSending).
		Updates(map[string]interface{}{"status": StatusSent, "sent_at": now, "locked_at": nil, "last_error": "", "updated_at": now})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (service *Service) MarkFailed(ctx context.Context, job *models.NotificationJob, sendErr error) error {
	if job == nil {
		return errors.New("notification job is nil")
	}
	now := service.now()
	attempts := job.Attempts + 1
	status := StatusQueued
	next := now.Add(backoff(attempts))
	if attempts >= job.MaxAttempts {
		status = StatusDead
		next = now
	}
	message := "notification delivery failed"
	if sendErr != nil {
		message = sendErr.Error()
	}
	if len(message) > 1000 {
		message = message[:1000]
	}
	return service.DB.WithContext(ctx).Model(&models.NotificationJob{}).
		Where("id = ? AND status = ?", job.ID, StatusSending).
		Updates(map[string]interface{}{"status": status, "attempts": attempts, "next_attempt_at": next, "locked_at": nil, "last_error": message, "updated_at": now}).Error
}

func (service *Service) CancelQueued(ctx context.Context, accountID string) error {
	return service.DB.WithContext(ctx).Model(&models.NotificationJob{}).
		Where("account_id = ? AND status IN ?", accountID, []string{StatusQueued, StatusSending}).
		Updates(map[string]interface{}{"status": StatusCancelled, "locked_at": nil, "updated_at": service.now()}).Error
}

func backoff(attempt int) time.Duration {
	if attempt < 1 {
		attempt = 1
	}
	delay := time.Minute
	for index := 1; index < attempt && delay < time.Hour; index++ {
		delay *= 2
	}
	if delay > time.Hour {
		return time.Hour
	}
	return delay
}

func accountNotificationBanned(db *gorm.DB, accountID string, now time.Time) bool {
	if db == nil || strings.TrimSpace(accountID) == "" {
		return false
	}
	var account models.PlayerAccount
	if err := db.Select("banned_at", "ban_expires_at").First(&account, "id = ?", accountID).Error; err != nil {
		return false
	}
	if account.BannedAt == nil {
		return false
	}
	if account.BanExpiresAt != nil && !account.BanExpiresAt.After(now) {
		return false
	}
	return true
}

func (service *Service) now() time.Time {
	if service.Now != nil {
		return service.Now()
	}
	return time.Now()
}

type SendFunc func(context.Context, models.NotificationJob) error

type Worker struct {
	Service      *Service
	Send         SendFunc
	PollInterval time.Duration
	Lease        time.Duration
}

func NewWorker(db *gorm.DB, send SendFunc) *Worker {
	return &Worker{Service: NewService(db), Send: send, PollInterval: 5 * time.Second, Lease: 2 * time.Minute}
}

func (worker *Worker) ProcessOne(ctx context.Context) (bool, error) {
	job, err := worker.Service.ClaimDue(ctx, worker.Lease)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if accountNotificationBanned(worker.Service.DB, job.AccountID, worker.Service.now()) {
		return true, worker.Service.MarkCancelled(ctx, job.ID)
	}
	if worker.Send == nil {
		err = errors.New("notification sender unavailable")
	} else {
		err = worker.Send(ctx, *job)
	}
	if err != nil {
		return true, worker.Service.MarkFailed(ctx, job, fmt.Errorf("send %s: %w", job.Kind, err))
	}
	return true, worker.Service.MarkSent(ctx, job.ID)
}

func (worker *Worker) Run(ctx context.Context) {
	interval := worker.PollInterval
	if interval <= 0 {
		interval = 5 * time.Second
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		for {
			processed, _ := worker.ProcessOne(ctx)
			if !processed {
				break
			}
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
