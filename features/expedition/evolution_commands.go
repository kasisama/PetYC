package expedition

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"qq-pet-saas/core"
	"qq-pet-saas/gameplay"
)

func handleEvolutionPreview(ctx context.Context, event core.InboundEvent, service *Service) (core.OutboundMessage, error) {
	return handleFormPreview(ctx, event, service, "进化")
}

func handleAwakenPreview(ctx context.Context, event core.InboundEvent, service *Service) (core.OutboundMessage, error) {
	return handleFormPreview(ctx, event, service, "觉醒")
}

func handleFormPreview(ctx context.Context, event core.InboundEvent, service *Service, stage string) (core.OutboundMessage, error) {
	account, err := resolve(ctx, service, event)
	if err != nil {
		return core.OutboundMessage{}, err
	}
	preview, err := gameplay.NewEvolutionService(service.DB).Preview(ctx, account.ID, stage)
	if err != nil {
		return evolutionBusinessError(err, stage)
	}
	return renderEvolutionPreview(preview), nil
}

func handleEvolutionConfirm(ctx context.Context, event core.InboundEvent, service *Service) (core.OutboundMessage, error) {
	return handleFormConfirm(ctx, event, service, "进化")
}

func handleAwakenConfirm(ctx context.Context, event core.InboundEvent, service *Service) (core.OutboundMessage, error) {
	return handleFormConfirm(ctx, event, service, "觉醒")
}

func handleFormConfirm(ctx context.Context, event core.InboundEvent, service *Service, stage string) (core.OutboundMessage, error) {
	account, err := resolve(ctx, service, event)
	if err != nil {
		return core.OutboundMessage{}, err
	}
	evolution := gameplay.NewEvolutionService(service.DB)
	var preview *gameplay.EvolutionPreview
	if stage == "进化" {
		preview, err = evolution.Evolve(ctx, account.ID)
	} else {
		preview, err = evolution.Awaken(ctx, account.ID)
	}
	if err != nil {
		return evolutionBusinessError(err, stage)
	}
	message := text(fmt.Sprintf("🌟【%s成功】\n耀眼的光芒将 %s 轻轻包围——熟悉的身影正在发生奇妙变化！\n\n%s 从「%s」成长为「%s」！\n一路积累的陪伴与努力，在这一刻终于开花结果。\n\n发送“我的宠物”欣赏它崭新的模样。", stage, preview.PetName, preview.PetName, preview.CurrentForm, preview.TargetForm))
	message.Image = preview.TargetImage
	return message, nil
}

func renderEvolutionPreview(preview *gameplay.EvolutionPreview) core.OutboundMessage {
	lines := []string{
		fmt.Sprintf("🔮【%s预览】", preview.Stage),
		fmt.Sprintf("%s：%s → %s", preview.PetName, preview.CurrentForm, preview.TargetForm),
		"它体内的力量正在悄悄积蓄，下一种姿态已经若隐若现。",
		"",
		"所需条件：",
	}
	for _, requirement := range preview.Requirements {
		mark := "❌"
		if requirement.Met {
			mark = "✅"
		}
		lines = append(lines, fmt.Sprintf("%s %s %d/%d", mark, requirement.Label, requirement.Current, requirement.Needed))
	}
	if preview.Ready {
		lines = append(lines, "", fmt.Sprintf("条件已经满足，发送“确认%s”即可完成。", preview.Stage))
	} else {
		lines = append(lines, "", "继续陪伴和成长，满足全部条件后再来看看吧。")
	}
	message := text(strings.Join(lines, "\n"))
	message.Image = preview.TargetImage
	return message
}

func evolutionBusinessError(err error, stage string) (core.OutboundMessage, error) {
	switch {
	case errors.Is(err, gameplay.ErrPetRequired):
		return text("你还没有宠物。\n发送“领养宠物”选择第一位伙伴。"), nil
	case errors.Is(err, gameplay.ErrEvolutionUnavailable):
		return text(fmt.Sprintf("当前没有可进行的%s。\n发送“我的宠物”查看现在的形态。", stage)), nil
	case errors.Is(err, gameplay.ErrEvolutionRequirements):
		return text(fmt.Sprintf("%s条件还没有全部满足。\n发送“%s”查看还差哪些条件。", stage, stage)), nil
	case errors.Is(err, gameplay.ErrInsufficientItem):
		return text("觉醒所需物品还不够。\n发送“我的背包”查看现有数量。"), nil
	case errors.Is(err, gameplay.ErrActivityActive):
		return text("宠物正在进行其他行动，结束当前行动后再改变形态吧。"), nil
	default:
		return core.OutboundMessage{}, err
	}
}
