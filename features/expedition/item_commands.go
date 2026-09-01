package expedition

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"qq-pet-saas/core"
	"qq-pet-saas/gameplay"
)

func handleUseItem(ctx context.Context, event core.InboundEvent, service *Service) (core.OutboundMessage, error) {
	account, err := resolve(ctx, service, event)
	if err != nil {
		return core.OutboundMessage{}, err
	}
	name, quantity, ok := parseItemQuantity(event.Text, "使用")
	if !ok {
		return text("请按“使用 物品名*数量”的格式发送。\n例如：使用 急救包*1"), nil
	}
	result, err := gameplay.NewItemEffectService(service.DB).Use(ctx, account.ID, name, quantity, itemUseIdempotencyKey(event))
	if err != nil {
		if errors.Is(err, gameplay.ErrActivityActive) {
			if message, busy := petBusyMessage(ctx, service, account.ID); busy {
				return message, nil
			}
		}
		return itemUseBusinessError(err)
	}
	if result.Duplicate {
		return text(fmt.Sprintf("这条使用指令已经处理过了，没有重复扣除物品。\n%s 当前%s为 %d。", result.PetName, displayItemType(result.Record.EffectType), result.Record.AfterValue)), nil
	}
	message := text(fmt.Sprintf("✨【物品使用成功】\n你把「%s」交给了 %s，它认真收下并发挥了作用。\n\n使用：%s ×%d\n%s：%d → %d\n\n发送“我的宠物”看看它现在的状态。", result.Record.ItemName, result.PetName, result.Record.ItemName, result.Record.Quantity, displayItemType(result.Record.EffectType), result.Record.BeforeValue, result.Record.AfterValue))
	message.Image = result.Image
	return message, nil
}

func itemUseIdempotencyKey(event core.InboundEvent) string {
	eventKey := strings.TrimSpace(event.EventID)
	if eventKey == "" {
		eventKey = strings.TrimSpace(event.MessageID)
	}
	if eventKey == "" {
		return ""
	}
	return strings.Join([]string{string(event.Platform), event.AppID, eventKey}, ":")
}

func itemUseBusinessError(err error) (core.OutboundMessage, error) {
	switch {
	case errors.Is(err, gameplay.ErrPetRequired):
		return text("你还没有宠物。\n发送“领养宠物”选择第一位伙伴。"), nil
	case errors.Is(err, gameplay.ErrItemNotFound):
		return text("没有找到这个物品。\n发送“我的背包”查看已有物品。"), nil
	case errors.Is(err, gameplay.ErrInsufficientItem):
		return text("背包里的数量不够。\n发送“我的背包”查看现有数量。"), nil
	case errors.Is(err, gameplay.ErrWrongItemType):
		return text("这个物品不能直接使用。\n食物请发送“喂养”，礼物请发送“送礼”；学习用品请用于对应成长行动。"), nil
	case errors.Is(err, gameplay.ErrTreatmentNotNeeded):
		return text("宠物现在体力充足，不需要使用恢复物品。"), nil
	case errors.Is(err, gameplay.ErrActivityActive):
		return text("宠物正在进行其他行动，完成后再使用这个物品吧。"), nil
	case errors.Is(err, gameplay.ErrInvalidQuantity):
		return text("数量需要是大于零的整数，请检查后再试。"), nil
	default:
		return core.OutboundMessage{}, err
	}
}
