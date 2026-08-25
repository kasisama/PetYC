package expedition

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"
	"qq-pet-saas/config"
	"qq-pet-saas/core"
	"qq-pet-saas/gameplay"
	"qq-pet-saas/models"
)

func handleStartActivity(ctx context.Context, event core.InboundEvent, service *Service) (core.OutboundMessage, error) {
	account, err := resolve(ctx, service, event)
	if err != nil {
		return core.OutboundMessage{}, err
	}
	kind := startActivityKind(event.Text)
	request, listMessage, err := activityStartRequest(service.DB, event.Text, kind)
	if err != nil {
		return activityBusinessError(err)
	}
	if listMessage != "" {
		return text(listMessage), nil
	}
	activity := gameplay.NewActivityService(service.DB)
	activity.Now = service.Now
	result, err := activity.Start(ctx, account.ID, request)
	if err != nil {
		return activityBusinessError(err)
	}
	lines := []string{
		fmt.Sprintf("🚩【%s开始】", kind),
		activityStartScene(kind, result.PetName),
	}
	if result.Run.InputItem != "" {
		lines = append(lines, fmt.Sprintf("使用：%s ×1", result.Run.InputItem))
	} else if result.Run.ConfigKey != "" {
		lines = append(lines, fmt.Sprintf("安排：%s", result.Run.ConfigKey))
	}
	if result.Run.HungerCost > 0 {
		lines = append(lines, fmt.Sprintf("🍖 饱食：%d → %d", result.HungerBefore, result.HungerAfter))
	}
	lines = append(lines,
		fmt.Sprintf("预计完成：%s", result.Run.EndsAt.Format("15:04")),
		"",
		fmt.Sprintf("完成后发送“完成%s”领取成果。", kind),
	)
	message := text(strings.Join(lines, "\n"))
	message.Image = result.Image
	enqueueTimedNotification(ctx, service, event, account.ID, "activity_done", "activity:"+result.Run.ID+":done", result.Run.EndsAt,
		fmt.Sprintf("✨ %s 已完成%s，发送“完成%s”领取成果吧。", result.PetName, kind, kind))
	return message, nil
}

func handleFinishActivity(ctx context.Context, event core.InboundEvent, service *Service) (core.OutboundMessage, error) {
	account, err := resolve(ctx, service, event)
	if err != nil {
		return core.OutboundMessage{}, err
	}
	kind := strings.TrimSpace(strings.TrimPrefix(event.Text, "完成"))
	activity := gameplay.NewActivityService(service.DB)
	activity.Now = service.Now
	result, err := activity.Complete(ctx, account.ID, kind, currencyName())
	if err != nil {
		return activityBusinessError(err)
	}
	lines := []string{fmt.Sprintf("🎊【%s完成】", kind), activityFinishScene(kind, result.PetName)}
	if result.Attribute != "" {
		lines = append(lines, fmt.Sprintf("%s：%d → %d", result.Attribute, result.AttributeBefore, result.AttributeAfter))
	}
	if result.GrowthDelta > 0 {
		lines = append(lines, fmt.Sprintf("🌱 成长 +%d", result.GrowthDelta))
	}
	if result.CurrencyDelta > 0 {
		lines = append(lines, fmt.Sprintf("💰 %s +%d｜余额 %d", result.CurrencyKey, result.CurrencyDelta, result.RemainingBalance))
	}
	for _, item := range result.Items {
		lines = append(lines, fmt.Sprintf("🎁 %s ×%d", item.Name, item.Quantity))
	}
	lines = append(lines, "", "这份努力已经变成了实实在在的成长！", "发送“我的宠物”查看最新状态。")
	message := text(strings.Join(lines, "\n"))
	message.Image = result.Image
	return message, nil
}

func activityStartScene(kind, petName string) string {
	switch kind {
	case gameplay.ActivityStudy:
		return fmt.Sprintf("%s 摊开学习用品，认真地坐到了书桌前。", petName)
	case gameplay.ActivityTrain:
		return fmt.Sprintf("%s 活动了一下筋骨，斗志满满地开始锻炼。", petName)
	case gameplay.ActivityFitness:
		return fmt.Sprintf("%s 调整好呼吸，稳稳地开始今天的健身计划。", petName)
	case gameplay.ActivityWork:
		return fmt.Sprintf("%s 整理好行装，精神十足地去上班啦。", petName)
	default:
		return fmt.Sprintf("%s 已经出发，准备认真完成这次行动。", petName)
	}
}

func activityFinishScene(kind, petName string) string {
	switch kind {
	case gameplay.ActivityStudy:
		return fmt.Sprintf("%s 合上书本，兴奋地与你分享刚刚学会的新知识。", petName)
	case gameplay.ActivityTrain:
		return fmt.Sprintf("%s 擦了擦汗，骄傲地展示这次锻炼的成果。", petName)
	case gameplay.ActivityFitness:
		return fmt.Sprintf("%s 完成最后一组动作，状态看起来比以前更稳健了。", petName)
	case gameplay.ActivityWork:
		return fmt.Sprintf("%s 带着一天的见闻和报酬顺利回家啦。", petName)
	default:
		return fmt.Sprintf("%s 顺利完成了%s，正等着你的夸奖。", petName, kind)
	}
}

func startActivityKind(message string) string {
	message = strings.TrimSpace(message)
	for _, kind := range []string{gameplay.ActivityStudy, gameplay.ActivityTrain, gameplay.ActivityFitness, gameplay.ActivityWork} {
		if strings.HasPrefix(message, kind) {
			return kind
		}
	}
	return ""
}

func activityStartRequest(db *gorm.DB, message, kind string) (gameplay.ActivityStartRequest, string, error) {
	argument := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(message), kind))
	switch kind {
	case gameplay.ActivityStudy:
		if argument == "" {
			return gameplay.ActivityStartRequest{}, "请选择背包中的智慧物品。\n例如：学习 专业书本\n\n发送“我的背包”查看已有物品。", nil
		}
		return gameplay.ActivityStartRequest{
			Kind: kind, ItemName: argument, RequiredItemType: "智慧", RewardAttribute: "智慧",
			DailyLimit: config.Interaction.StudyLimit, HungerCost: config.Interaction.StudyHungerCost, RewardGrowth: config.Interaction.StudyGrowth,
			StartImage: config.Images["开始学习"], EndImage: config.Images["完成学习"],
		}, "", nil
	case gameplay.ActivityTrain:
		if argument == "" {
			return gameplay.ActivityStartRequest{}, "请选择背包中的力量物品。\n例如：锻炼 力量哑铃\n\n发送“我的背包”查看已有物品。", nil
		}
		return gameplay.ActivityStartRequest{
			Kind: kind, ItemName: argument, RequiredItemType: "力量", RewardAttribute: "力量",
			DailyLimit: config.Interaction.TrainLimit, HungerCost: config.Interaction.TrainHungerCost, RewardGrowth: config.Interaction.TrainGrowth,
			StartImage: config.Images["开始锻炼"], EndImage: config.Images["完成锻炼"],
		}, "", nil
	case gameplay.ActivityFitness:
		if argument == "" {
			return gameplay.ActivityStartRequest{}, "请选择背包中的防御物品。\n例如：健身 防护软垫\n\n发送“我的背包”查看已有物品。", nil
		}
		return gameplay.ActivityStartRequest{
			Kind: kind, ItemName: argument, RequiredItemType: "防御", RewardAttribute: "防御",
			DailyLimit: config.Interaction.FitnessLimit, HungerCost: config.Interaction.FitnessHungerCost, RewardGrowth: config.Interaction.FitnessGrowth,
			StartImage: config.Images["开始健身"], EndImage: config.Images["完成健身"],
		}, "", nil
	case gameplay.ActivityWork:
		if argument == "" {
			var jobs []models.WorkSettingConfig
			if err := db.Order("name asc").Find(&jobs).Error; err != nil {
				return gameplay.ActivityStartRequest{}, "", err
			}
			if len(jobs) == 0 {
				return gameplay.ActivityStartRequest{}, "目前还没有开放岗位，可以晚些再来看看。", nil
			}
			lines := []string{"【可选岗位】"}
			for _, job := range jobs {
				lines = append(lines, fmt.Sprintf("%s｜%d分钟｜%d %s", job.Name, maxDurationMinute(job.Time), job.RewardCoin, currencyName()))
			}
			lines = append(lines, "", "发送“打工 岗位名”开始。")
			return gameplay.ActivityStartRequest{}, strings.Join(lines, "\n"), nil
		}
		var job models.WorkSettingConfig
		if err := db.First(&job, "name = ?", argument).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return gameplay.ActivityStartRequest{}, "", gameplay.ErrItemNotFound
			}
			return gameplay.ActivityStartRequest{}, "", err
		}
		return gameplay.ActivityStartRequest{
			Kind: kind, ConfigKey: job.Name, Duration: time.Duration(maxDurationMinute(job.Time)) * time.Minute,
			DailyLimit: 5, HungerCost: job.HungerCost, RewardCurrency: job.RewardCoin, RewardItems: job.RewardItems,
			StartImage: job.StartImage, EndImage: job.EndImage,
		}, "", nil
	default:
		return gameplay.ActivityStartRequest{}, "", gameplay.ErrNoActiveActivity
	}
}

func maxDurationMinute(value int64) int64 {
	if value <= 0 {
		return 1
	}
	return value
}

func activityBusinessError(err error) (core.OutboundMessage, error) {
	var notReady *gameplay.ActivityNotReadyError
	switch {
	case errors.As(err, &notReady):
		minutes := int64(notReady.Remaining.Round(time.Minute) / time.Minute)
		if minutes <= 0 {
			minutes = 1
		}
		return text(fmt.Sprintf("还差大约 %d 分钟，宠物就能完成这次行动。", minutes)), nil
	case errors.Is(err, gameplay.ErrPetRequired):
		return text("你还没有宠物。\n发送“领养宠物”选择第一位伙伴。"), nil
	case errors.Is(err, gameplay.ErrActivityActive):
		return text("宠物正在进行其他行动。\n发送“我的宠物”查看当前进度。"), nil
	case errors.Is(err, gameplay.ErrNoActiveActivity):
		return text("宠物当前没有进行这项行动，可以先发送对应命令开始。"), nil
	case errors.Is(err, gameplay.ErrItemNotFound):
		return text("没有找到这个物品或岗位。\n发送“我的背包”或“打工”查看可选内容。"), nil
	case errors.Is(err, gameplay.ErrInsufficientItem):
		return text("背包里没有足够的这个物品。\n发送“我的背包”查看现有物品。"), nil
	case errors.Is(err, gameplay.ErrWrongItemType):
		return text("这个物品不适合本次行动，请按提示选择对应类型。"), nil
	case errors.Is(err, gameplay.ErrPetTooHungry):
		return text("宠物现在太饿了，先喂饱它再安排成长行动吧。"), nil
	case errors.Is(err, gameplay.ErrDailyLimit):
		return text("今天这项行动的次数已经用完，明天再继续吧。"), nil
	case errors.Is(err, gameplay.ErrAttributeMax):
		return text("这项属性已经达到当前形态的上限，可以看看进化或其他成长方向。"), nil
	default:
		return core.OutboundMessage{}, err
	}
}
