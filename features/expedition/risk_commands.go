package expedition

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"

	"qq-pet-saas/core"
	"qq-pet-saas/gameplay"
	"qq-pet-saas/models"
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
			return riskErrorMessageFor(service.DB, rulesErr, "lottery"), nil
		}
		return withKeyboard(chanceRulesMessage(service.DB, rules, "发送“抽奖一次”即可抽取。"), []core.KeyboardButton{{Label: "抽取一次", Command: "抽奖一次"}}), nil
	}
	if argument != "1" && argument != "一次" && argument != "抽取一次" {
		return text("请发送“抽奖”查看奖池，或发送“抽奖一次”参与。"), nil
	}
	result, playErr := service.PlayLottery(ctx, account.ID, riskSourceKey(event))
	if playErr != nil {
		return riskErrorMessageFor(service.DB, playErr, "lottery"), nil
	}
	lines := []string{"🎰【幸运时刻】", "奖池的光点飞快旋转，最后在你面前停了下来——", fmt.Sprintf("获得：%s", result.Outcome.RewardName)}
	if result.Outcome.ItemName != "" {
		lines[1] = fmt.Sprintf("获得：%s ×%d", result.Outcome.ItemName, result.Outcome.Quantity)
	}
	if result.Outcome.Currency > 0 {
		lines = append(lines, fmt.Sprintf("%s：+%d", currencyName(service.DB), result.Outcome.Currency))
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
		return riskErrorMessageFor(service.DB, err, "fishing"), nil
	}
	return chanceRulesMessage(service.DB, rules, "发送“抛竿”开始，等待后发送“收竿”。"), nil
}

func handleCastFishing(ctx context.Context, event core.InboundEvent, service *Service) (core.OutboundMessage, error) {
	account, err := resolve(ctx, service, event)
	if err != nil {
		return core.OutboundMessage{}, err
	}
	run, attempts, limit, castErr := service.StartFishing(ctx, account.ID, riskSourceKey(event))
	if castErr != nil {
		var busy *FishingBusyError
		if errors.As(castErr, &busy) {
			if message, occupied := petBusyMessage(ctx, service, account.ID); occupied {
				return message, nil
			}
		}
		return riskErrorMessageFor(service.DB, castErr, "fishing"), nil
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
		return riskErrorMessageFor(service.DB, claimErr, "fishing"), nil
	}
	lines := []string{"🐟【收竿成功】", "浮漂猛地一沉——你抓住时机收紧鱼线，把今天的收获稳稳带上了岸！", fmt.Sprintf("获得：%s ×%d", run.ItemName, run.Quantity)}
	if run.Currency > 0 {
		lines = append(lines, fmt.Sprintf("%s：+%d", currencyName(service.DB), run.Currency))
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
		coin := currencyName(service.DB)
		return text(fmt.Sprintf("【宠物猜拳】\n对手出石头、剪刀、布的概率各为 1/3。\n胜利奖励5%s，平局奖励1%s，每日最多20次。\n\n发送“猜拳 石头／剪刀／布”。", coin, coin)), nil
	}
	result, battleErr := service.PlayRockPaperScissors(ctx, account.ID, riskSourceKey(event), choice)
	if battleErr != nil {
		return riskErrorMessageFor(service.DB, battleErr, "guess"), nil
	}
	lines := []string{
		"✊【猜拳结果】",
		"你和宠物同时把手藏到身后，倒数三声一起亮出了选择！",
		fmt.Sprintf("你出了：%s", result.Record.PlayerChoice),
		fmt.Sprintf("对手出了：%s", result.Record.OpponentChoice),
		fmt.Sprintf("结果：%s", result.Record.Result),
	}
	if result.Record.RewardCurrency > 0 {
		lines = append(lines, fmt.Sprintf("%s：+%d", currencyName(service.DB), result.Record.RewardCurrency))
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
		coin := currencyName(service.DB)
		return text(fmt.Sprintf("【安全交易】\n发布后物品会先进入托管；其他玩家接受时，物品与%s在同一事务交换。\n交易单24小时有效。\n\n发布：宠物交易 物品 数量 价格\n查看：交易列表\n接受：接受交易 编号\n取消：取消交易 编号", coin)), nil
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
		return riskErrorMessageFor(service.DB, createErr, "trade"), nil
	}
	return text(fmt.Sprintf("📜【交易委托已发布】\n营地的交易员已经收好物品，并把委托挂上了公告板。\n\n编号：%s\n托管：%s ×%d\n售价：%d%s\n有效期：24小时\n\n其他玩家发送“接受交易 %s”。", offer.Code, offer.ItemName, offer.Quantity, offer.Price, currencyName(service.DB), offer.Code)), nil
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
		lines = append(lines, fmt.Sprintf("%s｜%s ×%d｜%d%s", offer.Code, offer.ItemName, offer.Quantity, offer.Price, currencyName(service.DB)))
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
		return riskErrorMessageFor(service.DB, err, "trade"), nil
	}
	return text(fmt.Sprintf("【交易 %s】\n物品：%s ×%d\n价格：%d%s\n状态：%s\n到期：%s", offer.Code, offer.ItemName, offer.Quantity, offer.Price, currencyName(service.DB), tradeStatusName(offer.Status), offer.ExpiresAt.Format("01-02 15:04"))), nil
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
		return riskErrorMessageFor(service.DB, acceptErr, "trade"), nil
	}
	coin := currencyName(service.DB)
	return text(fmt.Sprintf("🤝【交易完成】\n交易员核对双方物品后，郑重盖下了成交印章！\n\n获得：%s ×%d\n支付：%d%s\n物品与%s已经同时结算。", offer.ItemName, offer.Quantity, offer.Price, coin, coin)), nil
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
		return riskErrorMessageFor(service.DB, cancelErr, "trade"), nil
	}
	return text(fmt.Sprintf("【交易已取消】\n公告板上的委托已经取下。\n%s ×%d 已完整退回背包。", offer.ItemName, offer.Quantity)), nil
}

func chanceRulesMessage(db *gorm.DB, rules *ChanceRules, next string) core.OutboundMessage {
	name := strings.TrimSpace(rules.Game.Name)
	if name == "" {
		name = "机缘"
	}
	plain := []string{"🎲【" + name + "】"}
	markdown := []string{"# " + name}
	if flavor := chanceFlavor(rules.Game.GameKey); flavor != "" {
		plain = append(plain, flavor)
		markdown = append(markdown, "", flavor)
	}
	if cost := chanceCostLine(db, rules.Game); cost != "" {
		plain = append(plain, cost)
		markdown = append(markdown, "", cost)
	}
	if rules.Game.GameKey == "lottery" && strings.Contains(rules.Game.CostItem, "抽签券") {
		line := "探索「石环牧径」及之后的区域，有机会获得抽签券。"
		plain = append(plain, line)
		markdown = append(markdown, line)
	}
	if rules.Game.DailyLimit > 0 {
		line := fmt.Sprintf("每日最多 %d 次", rules.Game.DailyLimit)
		plain = append(plain, line)
		markdown = append(markdown, line)
	}
	if rules.Game.PityThreshold > 0 {
		pityName := chancePityName(rules)
		line := fmt.Sprintf("连续 %d 次未出珍稀时触发保底", rules.Game.PityThreshold)
		if pityName != "" {
			line = fmt.Sprintf("连续 %d 次未出珍稀时，保底获得%s", rules.Game.PityThreshold, pityName)
		}
		plain = append(plain, line)
		markdown = append(markdown, line)
	}
	plain = append(plain, "", "奖池")
	markdown = append(markdown, "", "**奖池**")
	for _, rate := range rules.Rewards {
		label := strings.TrimSpace(rate.Reward.Name)
		if label == "" {
			label = strings.TrimSpace(rate.Reward.ItemName)
		}
		if label == "" {
			continue
		}
		rare := ""
		if rate.Reward.Rare {
			rare = " · 珍稀"
		}
		plain = append(plain, fmt.Sprintf("· %s  %s%s", label, FormatChanceRate(rate.Rate), rare))
		markdown = append(markdown, fmt.Sprintf("- %s  %s%s", label, FormatChanceRate(rate.Rate), rare))
	}
	if strings.TrimSpace(next) != "" {
		plain = append(plain, "", next)
		markdown = append(markdown, "", next)
	}
	return menuText(strings.TrimSpace(strings.Join(plain, "\n")), strings.TrimSpace(strings.Join(markdown, "\n")))
}

func chanceFlavor(gameKey string) string {
	switch gameKey {
	case "lottery":
		return "调查队把签条投入遗迹灯盏，请示今日机缘。"
	case "fishing":
		return "在静水边放下鱼竿，等待遗迹水域给出回应。"
	default:
		return ""
	}
}

func chanceCostLine(db *gorm.DB, game models.ChanceGameConfig) string {
	if item := strings.TrimSpace(game.CostItem); item != "" {
		quantity := game.CostQuantity
		if quantity <= 0 {
			quantity = 1
		}
		return fmt.Sprintf("消耗 %s ×%d", item, quantity)
	}
	if game.CostCurrency > 0 {
		return fmt.Sprintf("消耗 %s %d", currencyName(db), game.CostCurrency)
	}
	return ""
}

func chancePityName(rules *ChanceRules) string {
	key := strings.TrimSpace(rules.Game.PityRewardKey)
	if key == "" {
		return ""
	}
	for _, rate := range rules.Rewards {
		if rate.Reward.RewardKey == key || rate.Reward.ItemName == key || strings.HasSuffix(rate.Reward.RewardKey, key) {
			if name := strings.TrimSpace(rate.Reward.Name); name != "" {
				return name
			}
			return strings.TrimSpace(rate.Reward.ItemName)
		}
	}
	return ""
}

func riskErrorMessage(err error) core.OutboundMessage {
	return riskErrorMessageFor(nil, err, "")
}

func riskErrorMessageFor(db *gorm.DB, err error, gameKey string) core.OutboundMessage {
	var fishingBusy *FishingBusyError
	switch {
	case errors.Is(err, gameplay.ErrInsufficientFunds):
		return text(currencyName(db) + "不足，先签到、打工或完成远征吧。")
	case errors.Is(err, gameplay.ErrInsufficientItem), errors.Is(err, ErrInsufficientItem):
		if gameKey == "lottery" {
			return text("还没有足够的遗迹抽签券。先去探索石环牧径碰碰运气吧。")
		}
		return text("背包里的物品数量不足。")
	case errors.Is(err, ErrPetRequired):
		return text("请先发送“领养宠物”选择伙伴。")
	case errors.Is(err, ErrDailyLimitReached):
		return text("今天的参与次数已经用完，明天再来吧。")
	case errors.Is(err, ErrFishingActive):
		return text("已经抛过竿啦，等待后发送“收竿”。")
	case errors.As(err, &fishingBusy):
		status := strings.TrimSpace(fishingBusy.Status)
		if status == "" || status == "空闲" {
			return text("当前宠物正在进行其他行动，完成后再发送“抛竿”。")
		}
		return text(fmt.Sprintf("当前宠物正在%s，先完成并领取当前行动后再发送“抛竿”。", status))
	case errors.Is(err, ErrFishingCapacity):
		return text("当前账号的宠物行动名额已满，先完成或领取进行中的行动后再抛竿。")
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
		if gameKey == "lottery" {
			return text("抽奖还没有配置奖池，请稍后再试。")
		}
		if gameKey == "fishing" {
			return text("钓鱼还没有配置收获，请稍后再试。")
		}
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
