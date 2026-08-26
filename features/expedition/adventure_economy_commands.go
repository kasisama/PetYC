package expedition

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"qq-pet-saas/core"
	"qq-pet-saas/models"
)

func handleAdventureInventory(ctx context.Context, event core.InboundEvent, service *Service) (core.OutboundMessage, error) {
	argument := strings.TrimSpace(strings.TrimPrefix(event.Text, "远征背包"))
	switch argument {
	case "材料":
		return handleAdventureMaterials(ctx, core.InboundEvent{Platform: event.Platform, SceneType: event.SceneType, AppID: event.AppID, SpaceID: event.SpaceID, RoomID: event.RoomID, ActorID: event.ActorID, ActorName: event.ActorName, MessageID: event.MessageID, EventID: event.EventID, Text: "材料背包", Timestamp: event.Timestamp}, service)
	case "装备":
		return handleEquipment(ctx, event, service)
	case "蓝图":
		return handleBlueprints(ctx, event, service)
	}
	account, err := resolve(ctx, service, event)
	if err != nil {
		return core.OutboundMessage{}, err
	}
	wallet, err := adventureWalletBalanceTx(service.DB.WithContext(ctx), account.ID)
	if err != nil {
		return core.OutboundMessage{}, err
	}
	var materials, equipment, blueprints int64
	if err = service.DB.WithContext(ctx).Model(&models.GlobalInventoryItem{}).Where("account_id = ? AND quantity > 0", account.ID).Count(&materials).Error; err != nil {
		return core.OutboundMessage{}, err
	}
	if err = service.DB.WithContext(ctx).Model(&models.PlayerEquipment{}).Where("account_id = ?", account.ID).Count(&equipment).Error; err != nil {
		return core.OutboundMessage{}, err
	}
	if err = service.DB.WithContext(ctx).Model(&models.PlayerBlueprintProgress{}).Where("account_id = ?", account.ID).Count(&blueprints).Error; err != nil {
		return core.OutboundMessage{}, err
	}
	message := text(fmt.Sprintf("🎒【远征背包】\n旅途徽章：%d\n远征材料：%d 类\n装备：%d 件\n蓝图：%d 项\n\n选择要查看的分类。", wallet.Balance, materials, equipment, blueprints))
	return withKeyboard(message, []core.KeyboardButton{{Label: "材料背包", Command: "材料背包"}, {Label: "装备背包", Command: "装备背包"}, {Label: "蓝图背包", Command: "蓝图背包"}, {Label: "远征商店", Command: "远征商店"}}), nil
}

func handleAdventureMaterials(ctx context.Context, event core.InboundEvent, service *Service) (core.OutboundMessage, error) {
	account, err := resolve(ctx, service, event)
	if err != nil {
		return core.OutboundMessage{}, err
	}
	var rows []models.GlobalInventoryItem
	if err = service.DB.WithContext(ctx).Where("account_id = ? AND quantity > 0", account.ID).Order("item_key asc").Find(&rows).Error; err != nil {
		return core.OutboundMessage{}, err
	}
	if len(rows) == 0 {
		return text("🎒【材料背包】\n还没有远征材料。完成地图探索、远征或首领挑战后会获得材料。"), nil
	}
	keys := make([]string, 0, len(rows))
	for _, row := range rows {
		keys = append(keys, row.ItemKey)
	}
	var definitions []models.ItemConfig
	if err = service.DB.WithContext(ctx).Where("key IN ?", keys).Find(&definitions).Error; err != nil {
		return core.OutboundMessage{}, err
	}
	byKey := make(map[string]models.ItemConfig, len(definitions))
	for _, row := range definitions {
		byKey[row.Key] = row
	}
	lines := []string{"🎒【材料背包】"}
	for _, row := range rows {
		definition := byKey[row.ItemKey]
		name := definition.Name
		if name == "" {
			name = row.ItemKey
		}
		usage := strings.TrimSpace(definition.Usage)
		if usage != "" {
			lines = append(lines, fmt.Sprintf("%s ×%d｜%s｜%s", name, row.Quantity, rarityName(definition.Rarity), usage))
		} else {
			lines = append(lines, fmt.Sprintf("%s ×%d｜%s", name, row.Quantity, rarityName(definition.Rarity)))
		}
	}
	return text(strings.Join(lines, "\n")), nil
}

func handleAdventureShop(ctx context.Context, event core.InboundEvent, service *Service) (core.OutboundMessage, error) {
	account, err := resolve(ctx, service, event)
	if err != nil {
		return core.OutboundMessage{}, err
	}
	argument := strings.TrimSpace(strings.TrimPrefix(event.Text, "远征商店"))
	query := service.DB.WithContext(ctx).Where("enabled = ?", true)
	if argument == "材料" {
		query = query.Where("product_type = ?", "item")
	} else if argument == "装备" {
		query = query.Where("product_type IN ?", []string{"equipment", "blueprint_fragment"})
	}
	var listings []models.AdventureShopItemConfig
	if err = query.Order("sort_order asc, key asc").Find(&listings).Error; err != nil {
		return core.OutboundMessage{}, err
	}
	wallet, err := adventureWalletBalanceTx(service.DB.WithContext(ctx), account.ID)
	if err != nil {
		return core.OutboundMessage{}, err
	}
	if len(listings) == 0 {
		return text("🧭【远征商店】\n当前分类还没有上架商品。"), nil
	}
	lines := []string{fmt.Sprintf("🧭【远征商店】｜旅途徽章 %d", wallet.Balance)}
	buttons := make([]core.KeyboardButton, 0, len(listings))
	for _, listing := range listings {
		limit := "不限购"
		if listing.LimitType != "none" {
			remaining, remainingErr := adventureShopRemainingTx(service.DB.WithContext(ctx), account.ID, listing, service.Now())
			if remainingErr != nil {
				return core.OutboundMessage{}, remainingErr
			}
			limit = fmt.Sprintf("%s剩余 %d/%d", limitTypeName(listing.LimitType), remaining, listing.LimitQuantity)
		}
		lines = append(lines, fmt.Sprintf("%s｜%s ×%d｜%d 徽章｜%s", listing.Name, productTypeName(listing.ProductType), listing.Quantity, listing.Price, limit))
		buttons = append(buttons, core.KeyboardButton{Label: "购买 " + listing.Name, Command: "远征购买 " + listing.Name})
	}
	return withKeyboard(text(strings.Join(lines, "\n")), buttons), nil
}

func handleAdventurePurchase(ctx context.Context, event core.InboundEvent, service *Service) (core.OutboundMessage, error) {
	account, err := resolve(ctx, service, event)
	if err != nil {
		return core.OutboundMessage{}, err
	}
	argument := strings.TrimSpace(strings.TrimPrefix(event.Text, "远征购买"))
	if argument == "" {
		return text("请输入商品名称，例如：远征购买 林地样本补给 2"), nil
	}
	units := int64(1)
	name := argument
	parts := strings.Fields(argument)
	if len(parts) > 1 {
		if parsed, parseErr := strconv.ParseInt(parts[len(parts)-1], 10, 64); parseErr == nil {
			units = parsed
			name = strings.TrimSpace(strings.TrimSuffix(argument, parts[len(parts)-1]))
		}
	}
	var listing models.AdventureShopItemConfig
	if err = service.DB.WithContext(ctx).Where("enabled = ? AND (name = ? OR key = ?)", true, name, name).First(&listing).Error; err != nil {
		return text("没有找到这个远征商品，发送“远征商店”查看当前商品。"), nil
	}
	idempotencyKey := riskSourceKey(event)
	if idempotencyKey == "" {
		idempotencyKey = fmt.Sprintf("fallback:%s:%d:%s", account.ID, event.Timestamp.UnixNano(), event.Text)
	}
	result, err := service.PurchaseAdventureShop(ctx, account.ID, listing.Key, units, idempotencyKey)
	if err != nil {
		switch {
		case errors.Is(err, ErrJourneyBadgeShort):
			return text("旅途徽章不足。探索地图、完成远征或挑战首领可以获得更多徽章。"), nil
		case errors.Is(err, ErrAdventureShopLimit):
			return text("这个商品本周期的个人限购数量已经用完。"), nil
		case errors.Is(err, ErrAdventureShopItem):
			return text("这个商品已经下架。"), nil
		default:
			return core.OutboundMessage{}, err
		}
	}
	return text(fmt.Sprintf("✅【远征购买成功】\n获得：%s ×%d\n消耗：旅途徽章 ×%d\n余额：%d\n\n发送“远征背包”查看。", result.Listing.Name, result.Purchase.GrantedQuantity, result.Purchase.Cost, result.RemainingBalance)), nil
}

func rarityName(value string) string {
	return map[string]string{"common": "普通", "fine": "优良", "rare": "稀有", "epic": "史诗", "legendary": "传说"}[value]
}

func productTypeName(value string) string {
	return map[string]string{"item": "统一物品", "equipment": "装备", "blueprint_fragment": "蓝图碎片"}[value]
}

func limitTypeName(value string) string {
	return map[string]string{"daily": "每日", "weekly": "每周", "lifetime": "永久"}[value]
}
