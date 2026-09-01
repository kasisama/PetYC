package expedition

import (
	"context"
	"strings"

	"qq-pet-saas/core"
	"qq-pet-saas/gameplay"
	"qq-pet-saas/models"
)

// petBusyMessage turns a persisted pet state into the next action a player can
// actually take. It intentionally falls back to the raw status so newly
// configured timed activities remain understandable without a code deploy.
func petBusyMessage(ctx context.Context, service *Service, accountID string) (core.OutboundMessage, bool) {
	if service == nil || service.DB == nil || strings.TrimSpace(accountID) == "" {
		return core.OutboundMessage{}, false
	}
	var pet models.PetProfile
	lookup := service.DB.WithContext(ctx).Where("account_id = ?", accountID).Order("created_at asc, id asc").First(&pet)
	if lookup.Error != nil {
		return core.OutboundMessage{}, false
	}
	status := strings.TrimSpace(pet.Status)
	if status == "" || status == "空闲" {
		return core.OutboundMessage{}, false
	}

	switch status {
	case gameplay.PetStatusResting:
		return text("🏡【宠物正在休养】\n它正在安心恢复，暂时不能安排其他行动。\n发送“找回”让它重新与你同行。"), true
	case "受伤", "濒死":
		return text("💚【宠物需要治疗】\n它现在不适合继续行动。发送“治疗”恢复后再陪伴或出发。"), true
	case "逃跑":
		return text("宠物暂时不在身边。发送“找回”让它重新同行后再操作。"), true
	case "远征":
		return text("🧭【宠物正在远征】\n它正在执行委托，暂时不能安排其他行动。远征结束后发送“领取”结算收获。"), true
	case "探索战斗":
		return text("⚔️【宠物正在地图战斗】\n请先发送“普攻”“防御”“战斗技能 技能名”或“撤退”完成当前战斗。"), true
	case "首领战斗":
		return text("🐲【宠物正在挑战地图首领】\n请先发送“普攻”“防御”“战斗技能 技能名”或“撤退”完成当前战斗。"), true
	case "探索":
		return text("🧭【宠物正在探索】\n请先完成当前遭遇，再安排其他行动。"), true
	case "钓鱼":
		return text("🎣【宠物正在钓鱼】\n请等待鱼讯后发送“收竿”领取收获。"), true
	default:
		return text("⏳【宠物正在" + status + "】\n请先发送“完成" + status + "”结算当前行动，再安排其他操作。"), true
	}
}
