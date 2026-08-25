package expedition

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"qq-pet-saas/config"
	"qq-pet-saas/core"
	"qq-pet-saas/gameplay"
)

func handleRenamePet(ctx context.Context, event core.InboundEvent, service *Service) (core.OutboundMessage, error) {
	account, err := resolve(ctx, service, event)
	if err != nil {
		return core.OutboundMessage{}, err
	}
	name := strings.TrimSpace(strings.TrimPrefix(event.Text, "改名"))
	if name == "" {
		return text("请告诉我想改成什么名字。\n例如：改名 小星星"), nil
	}
	if err = gameplay.ValidatePetName(name); err != nil {
		return text("这个名字暂时不能使用。\n名字需要 2～12 个字符，且不能包含链接或联系方式。"), nil
	}
	pet, err := service.RenamePet(ctx, account.ID, name, currencyName(), config.Core.RenameCost)
	if err != nil {
		switch {
		case errors.Is(err, gameplay.ErrPetRequired):
			return text("你还没有宠物。\n发送“领养宠物”选择第一位伙伴。"), nil
		case errors.Is(err, gameplay.ErrInsufficientFunds):
			return text(fmt.Sprintf("余额还不够，改名需要 %d %s。\n可以先完成签到或远征。", config.Core.RenameCost, currencyName())), nil
		default:
			return core.OutboundMessage{}, err
		}
	}
	message := text(fmt.Sprintf("🎀【改名成功·新名字诞生啦】\n你轻轻叫了一声「%s」，它马上回过头来——看来已经记住这个新名字了！", pet.Name))
	if config.Core.RenameCost > 0 {
		message.Text += fmt.Sprintf("\n消耗：%s ×%d", currencyName(), config.Core.RenameCost)
		message.Markdown = &core.MarkdownPayload{Content: message.Text}
	}
	return message, nil
}

func handleShop(ctx context.Context, event core.InboundEvent, service *Service) (core.OutboundMessage, error) {
	return handleShopPage(ctx, event, service, gameplay.ShopTypeNormal, "商店", "🛍️ 宠物商店", currencyName())
}

func handleAffectionShop(ctx context.Context, event core.InboundEvent, service *Service) (core.OutboundMessage, error) {
	return handleShopPage(ctx, event, service, gameplay.ShopTypeAffection, "好感商店", "💝 好感商店", "好感")
}

func handleShopPage(ctx context.Context, event core.InboundEvent, service *Service, shopType, command, title, currency string) (core.OutboundMessage, error) {
	account, err := resolve(ctx, service, event)
	if err != nil {
		return core.OutboundMessage{}, err
	}
	if _, err = gameplay.NewPetService(service.DB).Get(ctx, account.ID); err != nil {
		return shopBusinessError(err)
	}
	page := 1
	argument := strings.TrimSpace(strings.TrimPrefix(event.Text, command))
	if argument != "" {
		parsed, parseErr := strconv.Atoi(argument)
		if parseErr != nil || parsed < 1 {
			return text(fmt.Sprintf("页码需要是正整数。\n例如：%s 2", command)), nil
		}
		page = parsed
	}
	shop := gameplay.NewShopService(service.DB)
	shop.Now = service.Now
	result, err := shop.List(ctx, shopType, page, 5)
	if err != nil {
		return core.OutboundMessage{}, err
	}
	if result.Total == 0 {
		return text(title + "\n今天还没有商品上架，晚些时候再来看看吧。"), nil
	}
	lines := []string{title, "欢迎光临！货架上摆满了为宠物准备的小礼物。", fmt.Sprintf("第 %d/%d 页", result.Page, result.TotalPages), ""}
	for _, listing := range result.Items {
		stock := "库存不限"
		if listing.Stock >= 0 {
			stock = fmt.Sprintf("库存 %d", listing.Stock)
		}
		lines = append(lines, fmt.Sprintf("• %s｜%d %s｜%s", listing.Name, listing.Price, currency, stock))
	}
	lines = append(lines, "", "发送“查看商品 商品名”查看图片和详情。", "购买示例：购买 小饼干*2")
	return text(strings.Join(lines, "\n")), nil
}

func handleShopItem(ctx context.Context, event core.InboundEvent, service *Service) (core.OutboundMessage, error) {
	account, err := resolve(ctx, service, event)
	if err != nil {
		return core.OutboundMessage{}, err
	}
	if _, err = gameplay.NewPetService(service.DB).Get(ctx, account.ID); err != nil {
		return shopBusinessError(err)
	}
	name := strings.TrimSpace(strings.TrimPrefix(event.Text, "查看商品"))
	if name == "" {
		return text("请告诉我想查看哪个商品。\n例如：查看商品 小饼干"), nil
	}
	listing, err := gameplay.NewShopService(service.DB).GetListing(ctx, name)
	if err != nil {
		return shopBusinessError(err)
	}
	currency := currencyName()
	if listing.ShopType == gameplay.ShopTypeAffection {
		currency = "好感"
	}
	stock := "不限"
	if listing.Stock >= 0 {
		stock = strconv.FormatInt(listing.Stock, 10)
	}
	description := strings.TrimSpace(listing.Description)
	if description == "" {
		description = "这件商品还没有填写介绍。"
	}
	message := text(fmt.Sprintf("【%s】\n价格：%d %s\n库存：%s\n%s\n\n购买示例：购买 %s*1", listing.Name, listing.Price, currency, stock, description, listing.Name))
	message.Image = listing.Image
	return message, nil
}

func handleItemDetail(ctx context.Context, event core.InboundEvent, service *Service) (core.OutboundMessage, error) {
	account, err := resolve(ctx, service, event)
	if err != nil {
		return core.OutboundMessage{}, err
	}
	if _, err = gameplay.NewPetService(service.DB).Get(ctx, account.ID); err != nil {
		return shopBusinessError(err)
	}
	name := strings.TrimSpace(strings.TrimPrefix(event.Text, "查看物品"))
	if name == "" {
		return text("请告诉我想查看哪个物品。\n例如：查看物品 小饼干"), nil
	}
	item, err := gameplay.NewShopService(service.DB).GetItem(ctx, name)
	if err != nil {
		return shopBusinessError(err)
	}
	description := strings.TrimSpace(item.Description)
	if description == "" {
		description = "这个物品还没有填写介绍。"
	}
	effect := itemEffectDescription(item.Type, item.Effect)
	sell := "不可出售"
	if item.SellPrice > 0 {
		sell = fmt.Sprintf("%d %s", item.SellPrice, currencyName())
	}
	message := text(fmt.Sprintf("【%s】\n类型：%s\n%s\n出售价格：%s\n%s", item.Name, displayItemType(item.Type), effect, sell, description))
	message.Image = item.Image
	return message, nil
}

func handleBuy(ctx context.Context, event core.InboundEvent, service *Service) (core.OutboundMessage, error) {
	account, err := resolve(ctx, service, event)
	if err != nil {
		return core.OutboundMessage{}, err
	}
	name, quantity, ok := parseItemQuantity(event.Text, "购买")
	if !ok {
		return text("请按“购买 商品名*数量”的格式发送。\n例如：购买 小饼干*2"), nil
	}
	shop := gameplay.NewShopService(service.DB)
	shop.Now = service.Now
	result, err := shop.Purchase(ctx, account.ID, name, quantity)
	if err != nil {
		return shopBusinessError(err)
	}
	message := text(fmt.Sprintf("🛍️【购买成功】\n店员把「%s」仔细包好，已经放进你的背包啦！\n\n获得：%s ×%d\n消耗：%s ×%d\n余额：%d %s\n\n发送“我的背包”查看，或继续逛逛“商店”。", result.Listing.Name, result.Listing.Name, result.Quantity, result.CurrencyKey, result.Cost, result.RemainingBalance, result.CurrencyKey))
	message.Image = result.Listing.Image
	return message, nil
}

func handleSell(ctx context.Context, event core.InboundEvent, service *Service) (core.OutboundMessage, error) {
	account, err := resolve(ctx, service, event)
	if err != nil {
		return core.OutboundMessage{}, err
	}
	name, quantity, ok := parseItemQuantity(event.Text, "出售")
	if !ok {
		return text("请按“出售 物品名*数量”的格式发送。\n例如：出售 小饼干*2"), nil
	}
	shop := gameplay.NewShopService(service.DB)
	shop.Now = service.Now
	result, err := shop.Sell(ctx, account.ID, name, quantity)
	if err != nil {
		return shopBusinessError(err)
	}
	message := text(fmt.Sprintf("💰【出售成功】\n店员接过「%s」，清点后把报酬交到了你手中。\n\n交出：%s ×%d\n获得：%s ×%d\n余额：%d %s\n\n背包又腾出了一点空间。", result.Item.Name, result.Item.Name, result.Quantity, result.CurrencyKey, result.Revenue, result.RemainingBalance, result.CurrencyKey))
	message.Image = result.Item.Image
	return message, nil
}

func parseItemQuantity(message, command string) (string, int64, bool) {
	argument := strings.TrimSpace(strings.TrimPrefix(message, command))
	if argument == "" {
		return "", 0, false
	}
	normalized := strings.ReplaceAll(argument, "×", "*")
	parts := strings.SplitN(normalized, "*", 2)
	name := strings.TrimSpace(parts[0])
	quantity := int64(1)
	if len(parts) == 2 {
		parsed, err := strconv.ParseInt(strings.TrimSpace(parts[1]), 10, 64)
		if err != nil {
			return "", 0, false
		}
		quantity = parsed
	}
	return name, quantity, name != "" && quantity > 0
}

func shopBusinessError(err error) (core.OutboundMessage, error) {
	switch {
	case errors.Is(err, gameplay.ErrPetRequired):
		return text("你还没有宠物。\n发送“领养宠物”选择第一位伙伴。"), nil
	case errors.Is(err, gameplay.ErrShopItemNotFound):
		return text("商店里没有找到这个商品。\n发送“商店”查看当前货架。"), nil
	case errors.Is(err, gameplay.ErrItemNotFound):
		return text("没有找到这个物品。\n可以发送“我的背包”看看已经拥有的物品。"), nil
	case errors.Is(err, gameplay.ErrItemUnavailable):
		return text("这个物品暂时无法获得，换一件看看吧。"), nil
	case errors.Is(err, gameplay.ErrOutOfStock):
		return text("这件商品的库存不够了，可以减少数量后再试。"), nil
	case errors.Is(err, gameplay.ErrInsufficientFunds):
		return text("余额还不够，可以先完成签到或远征。"), nil
	case errors.Is(err, gameplay.ErrInsufficientItem):
		return text("背包里的数量不够。\n发送“我的背包”查看现有物品。"), nil
	case errors.Is(err, gameplay.ErrOneTimeItem):
		return text("这件特别物品每位玩家只能拥有一个。"), nil
	case errors.Is(err, gameplay.ErrNotSellable):
		return text("这件物品暂时不能出售，可以继续留在背包里。"), nil
	case errors.Is(err, gameplay.ErrInvalidQuantity):
		return text("数量需要是大于零的整数，请检查后再试。"), nil
	default:
		return core.OutboundMessage{}, err
	}
}

func displayItemType(itemType string) string {
	itemType = strings.TrimSpace(itemType)
	if itemType == "" {
		return "普通物品"
	}
	return itemType
}

func itemEffectDescription(itemType, effect string) string {
	effect = strings.TrimSpace(effect)
	if effect == "" {
		return "效果：无直接使用效果"
	}
	label := map[string]string{
		"血量": "恢复体力", "饱食": "恢复饱食", "好感": "增加好感",
		"智慧": "增加智慧", "力量": "增加力量", "防御": "增加防御", "成长": "增加成长",
	}[strings.TrimSpace(itemType)]
	if label == "" {
		label = "效果"
	}
	return label + "：" + strings.ReplaceAll(effect, "*", " ×")
}
