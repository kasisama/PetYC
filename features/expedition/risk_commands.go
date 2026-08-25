package expedition

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"qq-pet-saas/core"
	"qq-pet-saas/gameplay"
)

func handleLottery(ctx context.Context, event core.InboundEvent, service *Service) (core.OutboundMessage, error) {
	account, err := resolve(ctx, service, event)
	if err != nil {
		return core.OutboundMessage{}, err
	}
	argument := trimAnyPrefix(event.Text, "宠物抽奖", "抽奖")
	if argument == "" || argument == "概率" || argument == "规则" {
		rules, rulesErr := service.GetChanceRules(ctx, "lottery")
		if rulesErr != nil {
			return riskErrorMessage(rulesErr), nil
		}
		return chanceRulesMessage(rules, "发送“抽奖 1”参与一次。"), nil
	}
	if argument != "1" && argument != "一次" {
		return text("抽奖格式不正确。\n发送“抽奖”查看概率，或发送“抽奖 1”参与一次。"), nil
	}
	result, playErr := service.PlayLottery(ctx, account.ID, riskSourceKey(event))
	if playErr != nil {
		return riskErrorMessage(playErr), nil
	}
	lines := []string{"🎰【幸运时刻】", "奖池的光点飞快旋转，最后在你面前停了下来——", fmt.Sprintf("获得：%s", result.Outcome.RewardName)}
	if result.Outcome.ItemName != "" {
		lines[1] = fmt.Sprintf("获得：%s ×%d", result.Outcome.ItemName, result.Outcome.Quantity)
	}
	if result.Outcome.Currency > 0 {
		lines = append(lines, fmt.Sprintf("%s：+%d", currencyName(), result.Outcome.Currency))
	}
	if result.Outcome.PityTriggered {
		lines = append(lines, "✨ 积攒的幸运终于爆发，本次触发保底奖励！")
	}
	if result.DailyLimit > 0 {
		lines = append(lines, fmt.Sprintf("今日次数：%d/%d", result.Attempts, result.DailyLimit))
	}
	if result.PityThreshold > 0 && !result.Outcome.PityTriggered {
		lines = append(lines, fmt.Sprintf("保底进度：%d/%d", result.PityCount, result.PityThreshold))
	}
	return text(strings.Join(lines, "\n")), nil
}

func handleFishingMenu(ctx context.Context, event core.InboundEvent, service *Service) (core.OutboundMessage, error) {
	if _, err := resolve(ctx, service, event); err != nil {
		return core.OutboundMessage{}, err
	}
	rules, err := service.GetChanceRules(ctx, "fishing")
	if err != nil {
		return riskErrorMessage(err), nil
	}
	return chanceRulesMessage(rules, "发送“抛竿”开始，等待后发送“收竿”。"), nil
}

func handleCastFishing(ctx context.Context, event core.InboundEvent, service *Service) (core.OutboundMessage, error) {
	account, err := resolve(ctx, service, event)
	if err != nil {
		return core.OutboundMessage{}, err
	}
	run, attempts, limit, castErr := service.StartFishing(ctx, account.ID, riskSourceKey(event))
	if castErr != nil {
		return riskErrorMessage(castErr), nil
	}
	wait := run.ReadyAt.Sub(service.Now())
	if wait < 0 {
		wait = 0
	}
	lines := []string{"🎣【鱼线入水】", "浮漂轻轻晃了晃，水面又慢慢恢复平静……会有什么悄悄靠近呢？", fmt.Sprintf("约等待：%s", friendlyDuration(wait)), "到时间后发送“收竿”。"}
	if limit > 0 && attempts > 0 {
		lines = append(lines, fmt.Sprintf("今日次数：%d/%d", attempts, limit))
	}
	return text(strings.Join(lines, "\n")), nil
}

func handleClaimFishing(ctx context.Context, event core.InboundEvent, service *Service) (core.OutboundMessage, error) {
	account, err := resolve(ctx, service, event)
	if err != nil {
		return core.OutboundMessage{}, err
	}
	run, claimErr := service.ClaimFishing(ctx, account.ID)
	if claimErr != nil {
		return riskErrorMessage(claimErr), nil
	}
	lines := []string{"🐟【收竿成功】", "浮漂猛地一沉——你抓住时机收紧鱼线，把今天的收获稳稳带上了岸！", fmt.Sprintf("获得：%s ×%d", run.ItemName, run.Quantity)}
	if run.Currency > 0 {
		lines = append(lines, fmt.Sprintf("%s：+%d", currencyName(), run.Currency))
	}
	if run.Pity {
		lines = append(lines, "✨ 本次触发保底收获")
	}
	lines = append(lines, "发送“钓鱼”查看完整概率。")
	message := text(strings.Join(lines, "\n"))
	message.Image = core.ExistingImageSource("钓鱼图片/"+run.ItemName+".png", "钓鱼图片/"+run.ItemName+".jpg")
	return message, nil
}

func handleRockPaperScissors(ctx context.Context, event core.InboundEvent, service *Service) (core.OutboundMessage, error) {
	account, err := resolve(ctx, service, event)
	if err != nil {
		return core.OutboundMessage{}, err
	}
	choice := trimAnyPrefix(event.Text, "宠物猜拳", "战斗", "猜拳")
	if choice == "" {
		return text("【宠物猜拳】\n对手出石头、剪刀、布的概率各为 1/3。\n胜利奖励5金币，平局奖励1金币，每日最多20次。\n\n发送“猜拳 石头／剪刀／布”。"), nil
	}
	result, battleErr := service.PlayRockPaperScissors(ctx, account.ID, riskSourceKey(event), choice)
	if battleErr != nil {
		return riskErrorMessage(battleErr), nil
	}
	lines := []string{
		"✊【猜拳结果】",
		"你和宠物同时把手藏到身后，倒数三声一起亮出了选择！",
		fmt.Sprintf("你出了：%s", result.Record.PlayerChoice),
		fmt.Sprintf("对手出了：%s", result.Record.OpponentChoice),
		fmt.Sprintf("结果：%s", result.Record.Result),
	}
	if result.Record.RewardCurrency > 0 {
		lines = append(lines, fmt.Sprintf("%s：+%d", currencyName(), result.Record.RewardCurrency))
	}
	if result.Limit > 0 {
		lines = append(lines, fmt.Sprintf("今日次数：%d/%d", result.Attempts, result.Limit))
	}
	switch result.Record.Result {
	case "胜利":
		lines = append(lines, "🎉 宠物兴奋地跳了起来，这一局配合得漂亮！")
	case "平局":
		lines = append(lines, "彼此对视了一眼——看来默契也太好了。")
	default:
		lines = append(lines, "宠物不服气地晃了晃脑袋，已经准备好下一局啦。")
	}
	return text(strings.Join(lines, "\n")), nil
}

func handleLegacyCombatGuide(context.Context, core.InboundEvent, *Service) (core.OutboundMessage, error) {
	return text("为避免在群内强制骚扰其他玩家，对抗已改为公开规则的宠物猜拳。\n发送“猜拳”查看胜负概率与奖励。"), nil
}

func handleTrade(ctx context.Context, event core.InboundEvent, service *Service) (core.OutboundMessage, error) {
	account, err := resolve(ctx, service, event)
	if err != nil {
		return core.OutboundMessage{}, err
	}
	argument := trimAnyPrefix(event.Text, "添加交易", "宠物交易", "交易")
	if argument == "" {
		return text("【安全交易】\n发布后物品会先进入托管；其他玩家接受时，物品与金币在同一事务交换。\n交易单24小时有效。\n\n发布：宠物交易 物品 数量 价格\n查看：交易列表\n接受：接受交易 编号\n取消：取消交易 编号"), nil
	}
	parts := strings.Fields(argument)
	if len(parts) != 3 {
		return text("交易格式不正确。\n请发送“宠物交易 物品 数量 价格”。"), nil
	}
	quantity, quantityErr := strconv.ParseInt(parts[1], 10, 64)
	price, priceErr := strconv.ParseInt(parts[2], 10, 64)
	if quantityErr != nil || priceErr != nil {
		return text("交易数量和价格需要是正整数。"), nil
	}
	offer, createErr := service.CreateTradeOffer(ctx, account.ID, parts[0], quantity, price)
	if createErr != nil {
		return riskErrorMessage(createErr), nil
	}
	return text(fmt.Sprintf("📜【交易委托已发布】\n营地的交易员已经收好物品，并把委托挂上了公告板。\n\n编号：%s\n托管：%s ×%d\n售价：%d金币\n有效期：24小时\n\n其他玩家发送“接受交易 %s”。", offer.Code, offer.ItemName, offer.Quantity, offer.Price, offer.Code)), nil
}

func handleTradeList(ctx context.Context, event core.InboundEvent, service *Service) (core.OutboundMessage, error) {
	if _, err := resolve(ctx, service, event); err != nil {
		return core.OutboundMessage{}, err
	}
	offers, err := service.ListTradeOffers(ctx, 10)
	if err != nil {
		return text("交易列表暂时无法读取，请稍后再试。"), nil
	}
	if len(offers) == 0 {
		return text("当前没有可接受的交易。\n发送“宠物交易”查看发布方法。"), nil
	}
	lines := []string{"【交易列表】"}
	for _, offer := range offers {
		lines = append(lines, fmt.Sprintf("%s｜%s ×%d｜%d金币", offer.Code, offer.ItemName, offer.Quantity, offer.Price))
	}
	lines = append(lines, "", "发送“交易信息 编号”查看，或发送“接受交易 编号”。")
	return text(strings.Join(lines, "\n")), nil
}

func handleTradeInfo(ctx context.Context, event core.InboundEvent, service *Service) (core.OutboundMessage, error) {
	if _, err := resolve(ctx, service, event); err != nil {
		return core.OutboundMessage{}, err
	}
	code := trimAnyPrefix(event.Text, "交易信息")
	if code == "" {
		return text("请发送“交易信息 编号”。"), nil
	}
	offer, err := service.GetTradeOffer(ctx, code)
	if err != nil {
		return riskErrorMessage(err), nil
	}
	return text(fmt.Sprintf("【交易 %s】\n物品：%s ×%d\n价格：%d金币\n状态：%s\n到期：%s", offer.Code, offer.ItemName, offer.Quantity, offer.Price, tradeStatusName(offer.Status), offer.ExpiresAt.Format("01-02 15:04"))), nil
}

func handleAcceptTrade(ctx context.Context, event core.InboundEvent, service *Service) (core.OutboundMessage, error) {
	account, err := resolve(ctx, service, event)
	if err != nil {
		return core.OutboundMessage{}, err
	}
	code := trimAnyPrefix(event.Text, "同意交易", "接受交易")
	if code == "" {
		return text("请发送“接受交易 编号”。"), nil
	}
	offer, acceptErr := service.AcceptTradeOffer(ctx, account.ID, code)
	if acceptErr != nil {
		return riskErrorMessage(acceptErr), nil
	}
	return text(fmt.Sprintf("🤝【交易完成】\n交易员核对双方物品后，郑重盖下了成交印章！\n\n获得：%s ×%d\n支付：%d金币\n物品与金币已经同时结算。", offer.ItemName, offer.Quantity, offer.Price)), nil
}

func handleCancelTrade(ctx context.Context, event core.InboundEvent, service *Service) (core.OutboundMessage, error) {
	account, err := resolve(ctx, service, event)
	if err != nil {
		return core.OutboundMessage{}, err
	}
	code := trimAnyPrefix(event.Text, "确认取消", "拒绝交易", "删除交易", "取消交易")
	if code == "" {
		return text("请发送“取消交易 编号”。"), nil
	}
	offer, cancelErr := service.CancelTradeOffer(ctx, account.ID, code)
	if cancelErr != nil {
		return riskErrorMessage(cancelErr), nil
	}
	return text(fmt.Sprintf("【交易已取消】\n公告板上的委托已经取下。\n%s ×%d 已完整退回背包。", offer.ItemName, offer.Quantity)), nil
}

func chanceRulesMessage(rules *ChanceRules, next string) core.OutboundMessage {
	lines := []string{fmt.Sprintf("【%s规则】", rules.Game.Name), rules.Game.Rules, "", "公开概率："}
	for _, rate := range rules.Rewards {
		lines = append(lines, fmt.Sprintf("- %s：%s", rate.Reward.Name, FormatChanceRate(rate.Rate)))
	}
	lines = append(lines, "", next)
	return text(strings.Join(lines, "\n"))
}

func riskErrorMessage(err error) core.OutboundMessage {
	switch {
	case errors.Is(err, gameplay.ErrInsufficientFunds):
		return text("金币不足，先签到、打工或完成远征吧。")
	case errors.Is(err, gameplay.ErrInsufficientItem), errors.Is(err, ErrInsufficientItem):
		return text("背包里的物品数量不足。")
	case errors.Is(err, ErrPetRequired):
		return text("请先发送“领养宠物”选择伙伴。")
	case errors.Is(err, ErrDailyLimitReached):
		return text("今天的参与次数已经用完，明天再来吧。")
	case errors.Is(err, ErrFishingActive):
		return text("已经抛过竿啦，等待后发送“收竿”。")
	case errors.Is(err, ErrFishingNotReady):
		return text("水面还没有动静，再等一会儿后发送“收竿”。")
	case errors.Is(err, ErrNoFishingRun):
		return text("当前没有等待收竿的鱼竿，发送“抛竿”开始。")
	case errors.Is(err, ErrInvalidBattleChoice):
		return text("请选择石头、剪刀或布，例如“猜拳 石头”。")
	case errors.Is(err, ErrTradeNotFound):
		return text("没有找到这笔交易，请核对编号。")
	case errors.Is(err, ErrTradeNotOpen), errors.Is(err, ErrTradeExpired):
		return text("这笔交易已经结束，发送“交易列表”查看可接受的交易。")
	case errors.Is(err, ErrTradeOwnOffer):
		return text("不能接受自己发布的交易。")
	case errors.Is(err, ErrTradeSellerRequired):
		return text("只有交易发布者可以取消这笔交易。")
	case errors.Is(err, ErrChanceGameDisabled):
		return text("该玩法暂时没有开放。")
	default:
		return text("操作暂时没有完成，请稍后再试。")
	}
}

func riskSourceKey(event core.InboundEvent) string {
	if event.EventID != "" {
		return "event:" + event.EventID
	}
	if event.MessageID != "" {
		return "message:" + event.MessageID
	}
	return ""
}

func trimAnyPrefix(value string, prefixes ...string) string {
	value = strings.TrimSpace(value)
	for _, prefix := range prefixes {
		if strings.HasPrefix(value, prefix) {
			return strings.TrimSpace(strings.TrimPrefix(value, prefix))
		}
	}
	return value
}

func friendlyDuration(duration time.Duration) string {
	seconds := int(duration.Seconds())
	if seconds < 60 {
		if seconds < 1 {
			seconds = 1
		}
		return fmt.Sprintf("%d秒", seconds)
	}
	return fmt.Sprintf("%d分钟", (seconds+59)/60)
}

func tradeStatusName(status string) string {
	switch status {
	case "open":
		return "等待接受"
	case "completed":
		return "已完成"
	case "cancelled":
		return "已取消"
	case "expired":
		return "已过期"
	default:
		return "处理中"
	}
}
