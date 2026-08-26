package expedition

import (
	"context"
	"log"
	"time"

	"qq-pet-saas/core"
	"qq-pet-saas/notifications"
)

func enqueueTimedNotification(
	ctx context.Context,
	service *Service,
	event core.InboundEvent,
	accountID, kind, idempotencyKey string,
	dueAt time.Time,
	message string,
) {
	if service == nil || service.DB == nil {
		return
	}
	_, err := notifications.NewService(service.DB).Enqueue(ctx, notifications.EnqueueRequest{
		AccountID: accountID, IdempotencyKey: idempotencyKey, Kind: kind,
		Platform: string(event.Platform), SceneType: string(event.SceneType), AppID: event.AppID,
		SpaceID: event.SpaceID, RoomID: event.RoomID, ActorID: event.ActorID, ActorName: event.ActorName,
		MessageKey: "notification." + kind,
		Message:    message, DueAt: dueAt,
	})
	if err != nil {
		// 通知是附加体验，写入失败不能回滚已成功的玩家操作。
		log.Printf("[通知] 创建 %s 任务失败: %v", kind, err)
	}
}
