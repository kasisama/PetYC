package entertainment

import (
	"fmt"
	"strconv"
	"strings"
	"sync"

	"github.com/gorilla/websocket"
	"gorm.io/gorm"
	"qq-pet-saas/config"
	"qq-pet-saas/core"
	"qq-pet-saas/database"
	"qq-pet-saas/features/core_game"
	"qq-pet-saas/models"
	"qq-pet-saas/utils"
)

type TradeSession struct {
	UserAID     int64
	UserBID     int64
	UserAStatus int              // 0: Proposed, 1: Active, 2: Agreed
	UserBStatus int              // 0: Proposed, 1: Active, 2: Agreed
	UserAItems  map[string]int64 // ItemName -> Qty
	UserBItems  map[string]int64 // ItemName -> Qty
	CancelA     bool
	CancelB     bool
}

var (
	tradeMu  sync.Mutex
	sessions = make(map[int64]*TradeSession) // UserID -> Session
)

func getTradeSession(userID int64) *TradeSession {
	tradeMu.Lock()
	defer tradeMu.Unlock()
	return sessions[userID]
}

func init() {
	core.RegisterHandler(config.GetCommand("宠物交易"), startTrade)
	core.RegisterHandler(config.GetCommand("接受交易"), acceptTrade)
	core.RegisterHandler(config.GetCommand("拒绝交易"), rejectTrade)
	core.RegisterHandler(config.GetCommand("添加交易"), addToTrade)
	core.RegisterHandler(config.GetCommand("删除交易"), removeFromTrade)
	core.RegisterHandler(config.GetCommand("交易信息"), showTradeInfo)
	core.RegisterHandler(config.GetCommand("取消交易"), cancelTrade)
	core.RegisterHandler(config.GetCommand("同意交易"), agreeTrade)
}

// 1. 发起交易
func startTrade(conn *websocket.Conn, event *core.OneBotEvent) {
	lockKey := utils.GetUserLockKey(event.GroupID, event.UserID)
	mutex := utils.GetLock(lockKey)
	mutex.Lock()
	defer mutex.Unlock()

	pet, ok := core_game.GetPetOrReply(conn, event)
	if !ok {
		return
	}

	chkMsg := core_game.CheckPlayerStatus(pet, "")
	if chkMsg != "" {
		core.SendGroupMessage(conn, event.GroupID, chkMsg)
		return
	}

	prefix := config.GetCommand("宠物交易")
	args := strings.TrimSpace(strings.TrimPrefix(event.RawMessage, prefix))
	targetQQ := parseAtQQ(args)
	if targetQQ == 0 {
		core.SendGroupMessage(conn, event.GroupID, fmt.Sprintf("%s\n%s阁下，想和别人交易的话，首先确认背包里是否有充足物品，然后发送【%s @QQ】等待对方同意交易哦！",
			core_game.AtSender(event.UserID), utils.Emoji("E29D84"), prefix))
		return
	}

	if targetQQ == event.UserID {
		core.SendGroupMessage(conn, event.GroupID, core_game.AtSender(event.UserID)+"\n"+utils.Emoji("E29D84")+"阁下不可以和自己交易哟！")
		return
	}

	// 检查双方状态
	tradeMu.Lock()
	sessA := sessions[event.UserID]
	sessB := sessions[targetQQ]
	tradeMu.Unlock()

	if sessA != nil {
		core.SendGroupMessage(conn, event.GroupID, core_game.AtSender(event.UserID)+"\n"+utils.Emoji("E29D84")+"阁下现在已经在交易中了哦！")
		return
	}
	if sessB != nil {
		core.SendGroupMessage(conn, event.GroupID, core_game.AtSender(event.UserID)+"\n"+utils.Emoji("E29D84")+"阁下，对方现在已经在交易中了哦！")
		return
	}

	var targetPet models.UserPet
	err := database.DB.Where("user_id = ? AND group_id = ?", targetQQ, event.GroupID).First(&targetPet).Error
	if err == gorm.ErrRecordNotFound || targetPet.Status == "失去宠物" {
		core.SendGroupMessage(conn, event.GroupID, core_game.AtSender(event.UserID)+"\n"+utils.Emoji("E29D84")+"对方还没有宠物呢！")
		return
	}

	// 手续费检查
	txFee := config.Core.TxFee
	if pet.Currency < txFee {
		core.SendGroupMessage(conn, event.GroupID, fmt.Sprintf("%s\n%s阁下包包里的%s不够交易哦！交易需要%d%s的手续费哦！",
			core_game.AtSender(event.UserID), utils.Emoji("E29D84"), config.Core.CoinName, txFee, config.Core.CoinName))
		return
	}
	if targetPet.Currency < txFee {
		core.SendGroupMessage(conn, event.GroupID, fmt.Sprintf("%s\n%s阁下，对方包包里的%s不够交易哦！交易需要%d%s的手续费哦！\n快去催对方爆肝%s吧！",
			core_game.AtSender(event.UserID), utils.Emoji("E29D84"), config.Core.CoinName, txFee, config.Core.CoinName, config.Core.CoinName))
		return
	}

	// 创建交易会话
	session := &TradeSession{
		UserAID:     event.UserID,
		UserBID:     targetQQ,
		UserAStatus: 0,
		UserBStatus: 0,
		UserAItems:  make(map[string]int64),
		UserBItems:  make(map[string]int64),
	}

	tradeMu.Lock()
	sessions[event.UserID] = session
	sessions[targetQQ] = session
	tradeMu.Unlock()

	replyText := fmt.Sprintf("%s申请向%s发起交易！\n%s想要继续交易请发送【接受交易】\n%s如果不想交易请发送【拒绝交易】",
		core_game.AtSender(event.UserID), core_game.AtSender(targetQQ),
		utils.Emoji("F09F918C"), utils.Emoji("F09FA49A"))

	core.SendGroupMessage(conn, event.GroupID, core_game.AtSender(event.UserID)+"\n"+utils.Emoji("F09FA49D")+replyText)
}

// 2. 接受交易
func acceptTrade(conn *websocket.Conn, event *core.OneBotEvent) {
	lockKey := utils.GetUserLockKey(event.GroupID, event.UserID)
	mutex := utils.GetLock(lockKey)
	mutex.Lock()
	defer mutex.Unlock()

	session := getTradeSession(event.UserID)
	if session == nil {
		core.SendGroupMessage(conn, event.GroupID, core_game.AtSender(event.UserID)+"\n"+utils.Emoji("E29D84")+"还没有人和阁下交易呐！")
		return
	}

	if event.UserID == session.UserAID {
		core.SendGroupMessage(conn, event.GroupID, core_game.AtSender(event.UserID)+"\n"+utils.Emoji("E29D84")+"阁下是交易发起人，不能自己接受哦！")
		return
	}

	if session.UserBStatus > 0 {
		core.SendGroupMessage(conn, event.GroupID, core_game.AtSender(event.UserID)+"\n"+utils.Emoji("E29D84")+"阁下正在一场交易中哦！")
		return
	}

	session.UserAStatus = 1
	session.UserBStatus = 1

	replyText := fmt.Sprintf("阁下同意了%s的交易！\n%s来开始一场愉快的交易吧！\n%s发送【交易信息】来查看本场交易的信息。",
		core_game.AtSender(session.UserAID),
		utils.Emoji("F09F9889"), utils.Emoji("F09F92A1"))

	core.SendGroupMessage(conn, event.GroupID, core_game.AtSender(event.UserID)+"\n"+utils.Emoji("E29CA8")+replyText)
}

// 3. 拒绝交易
func rejectTrade(conn *websocket.Conn, event *core.OneBotEvent) {
	lockKey := utils.GetUserLockKey(event.GroupID, event.UserID)
	mutex := utils.GetLock(lockKey)
	mutex.Lock()
	defer mutex.Unlock()

	session := getTradeSession(event.UserID)
	if session == nil {
		core.SendGroupMessage(conn, event.GroupID, core_game.AtSender(event.UserID)+"\n"+utils.Emoji("E29D84")+"还没有人和阁下交易呐！")
		return
	}

	if event.UserID == session.UserAID {
		core.SendGroupMessage(conn, event.GroupID, core_game.AtSender(event.UserID)+"\n"+utils.Emoji("E29D84")+"阁下是交易发起人，不能自己拒绝的辣，可以发送【取消交易】！")
		return
	}

	if session.UserBStatus == 1 || session.UserBStatus == 2 {
		core.SendGroupMessage(conn, event.GroupID, core_game.AtSender(event.UserID)+"\n"+utils.Emoji("E29D84")+"阁下正在一场交易中，如果不想交易了请发送【取消交易】哦！")
		return
	}

	// 销毁会话
	tradeMu.Lock()
	delete(sessions, session.UserAID)
	delete(sessions, session.UserBID)
	tradeMu.Unlock()

	core.SendGroupMessage(conn, event.GroupID, fmt.Sprintf("%s\n%s阁下拒绝了%s的交易！",
		core_game.AtSender(event.UserID), utils.Emoji("E29B94"), core_game.AtSender(session.UserAID)))
}

// 4. 添加物品到交易 (内存暂存，最终原子提交)
func addToTrade(conn *websocket.Conn, event *core.OneBotEvent) {
	lockKey := utils.GetUserLockKey(event.GroupID, event.UserID)
	mutex := utils.GetLock(lockKey)
	mutex.Lock()
	defer mutex.Unlock()

	session := getTradeSession(event.UserID)
	if session == nil {
		core.SendGroupMessage(conn, event.GroupID, core_game.AtSender(event.UserID)+"\n"+utils.Emoji("E29D84")+"还没有人和阁下交易呐！")
		return
	}

	if session.UserAStatus == 0 || session.UserBStatus == 0 {
		core.SendGroupMessage(conn, event.GroupID, core_game.AtSender(event.UserID)+"\n"+utils.Emoji("E29D84")+"阁下请等待对方接受交易后，再添加交易物品哦！")
		return
	}

	prefix := config.GetCommand("添加交易")
	args := strings.TrimSpace(strings.TrimPrefix(event.RawMessage, prefix))
	if args == "" {
		core.SendGroupMessage(conn, event.GroupID, core_game.AtSender(event.UserID)+"\n"+utils.Emoji("E29D84")+"请指定要添加的物品与数量，格式: 【添加交易 物品*数量】")
		return
	}

	// 解析物品列表 (支持 # 拼接多物品)
	parts := strings.Split(args, "#")
	var successItems []string
	var failItems []string

	pet, _ := core_game.GetPetOrReply(conn, event)
	txFee := config.Core.TxFee

	// 临时合并当前玩家已经放入交易的物品
	userItems := session.UserAItems
	if event.UserID == session.UserBID {
		userItems = session.UserBItems
	}

	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		itemName := part
		qty := int64(1)

		if strings.Contains(part, "*") {
			sub := strings.Split(part, "*")
			itemName = strings.TrimSpace(sub[0])
			qty, _ = strconv.ParseInt(strings.TrimSpace(sub[1]), 10, 64)
		} else if strings.Contains(part, "×") {
			sub := strings.Split(part, "×")
			itemName = strings.TrimSpace(sub[0])
			qty, _ = strconv.ParseInt(strings.TrimSpace(sub[1]), 10, 64)
		}

		if qty <= 0 {
			failItems = append(failItems, itemName)
			continue
		}

		if itemName == "货币" || itemName == config.Core.CoinName {
			currentAdded := userItems["[货币]"]
			if pet.Currency < currentAdded+qty+txFee {
				failItems = append(failItems, config.Core.CoinName)
				continue
			}
			userItems["[货币]"] = currentAdded + qty
			successItems = append(successItems, fmt.Sprintf("%s×%d", config.Core.CoinName, qty))
		} else {
			// 普通物品
			_, hasConfig := config.Items[itemName]
			if !hasConfig {
				failItems = append(failItems, itemName)
				continue
			}

			// 检查背包
			var backpackItem models.BackpackItem
			err := database.DB.Where("user_id = ? AND group_id = ? AND item_name = ?", event.UserID, event.GroupID, itemName).First(&backpackItem).Error
			if err != nil {
				failItems = append(failItems, itemName)
				continue
			}

			currentAdded := userItems[itemName]
			if backpackItem.Quantity < currentAdded+qty {
				failItems = append(failItems, itemName)
				continue
			}

			userItems[itemName] = currentAdded + qty
			successItems = append(successItems, fmt.Sprintf("%s×%d", itemName, qty))
		}
	}

	var reply strings.Builder
	if len(successItems) > 0 {
		reply.WriteString(fmt.Sprintf("%s阁下添加了以下要交易的物品：\n%s\n", utils.Emoji("F09F9A9A"), strings.Join(successItems, "、")))
	}
	if len(failItems) > 0 {
		reply.WriteString(fmt.Sprintf("%s以下物品添加失败，请检查添加数量或背包数量：%s\n", utils.Emoji("E29D97"), strings.Join(failItems, "、")))
	}

	reply.WriteString(utils.Emoji("F09F92A1") + "如果想要查看当前交易的物品，请发送【交易信息】！")
	core.SendGroupMessage(conn, event.GroupID, core_game.AtSender(event.UserID)+"\n"+reply.String())
}

// 5. 从交易删除物品
func removeFromTrade(conn *websocket.Conn, event *core.OneBotEvent) {
	lockKey := utils.GetUserLockKey(event.GroupID, event.UserID)
	mutex := utils.GetLock(lockKey)
	mutex.Lock()
	defer mutex.Unlock()

	session := getTradeSession(event.UserID)
	if session == nil {
		core.SendGroupMessage(conn, event.GroupID, core_game.AtSender(event.UserID)+"\n"+utils.Emoji("E29D84")+"还没有人和阁下交易呐！")
		return
	}

	if session.UserAStatus == 0 || session.UserBStatus == 0 {
		core.SendGroupMessage(conn, event.GroupID, core_game.AtSender(event.UserID)+"\n"+utils.Emoji("E29D84")+"阁下请等待对方接受交易后，再进行交易操作哦！")
		return
	}

	prefix := config.GetCommand("删除交易")
	args := strings.TrimSpace(strings.TrimPrefix(event.RawMessage, prefix))
	if args == "" {
		core.SendGroupMessage(conn, event.GroupID, core_game.AtSender(event.UserID)+"\n"+utils.Emoji("E29D84")+"请指定要删除的交易物品与数量，格式: 【删除交易 物品*数量】")
		return
	}

	parts := strings.Split(args, "#")
	var successItems []string
	var failItems []string

	userItems := session.UserAItems
	if event.UserID == session.UserBID {
		userItems = session.UserBItems
	}

	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		itemName := part
		qty := int64(1)

		if strings.Contains(part, "*") {
			sub := strings.Split(part, "*")
			itemName = strings.TrimSpace(sub[0])
			qty, _ = strconv.ParseInt(strings.TrimSpace(sub[1]), 10, 64)
		} else if strings.Contains(part, "×") {
			sub := strings.Split(part, "×")
			itemName = strings.TrimSpace(sub[0])
			qty, _ = strconv.ParseInt(strings.TrimSpace(sub[1]), 10, 64)
		}

		key := itemName
		if itemName == "货币" || itemName == config.Core.CoinName {
			key = "[货币]"
		}

		currentVal := userItems[key]
		if qty <= 0 || currentVal < qty {
			failItems = append(failItems, itemName)
			continue
		}

		userItems[key] = currentVal - qty
		if userItems[key] <= 0 {
			delete(userItems, key)
		}
		successItems = append(successItems, fmt.Sprintf("%s×%d", itemName, qty))
	}

	var reply strings.Builder
	if len(successItems) > 0 {
		reply.WriteString(fmt.Sprintf("%s阁下删除了以下要交易的物品：\n%s\n", utils.Emoji("F09F9791"), strings.Join(successItems, "、")))
	}
	if len(failItems) > 0 {
		reply.WriteString(fmt.Sprintf("%s以下物品删除失败，请检查交易状态或删除数量：%s\n", utils.Emoji("E29D97"), strings.Join(failItems, "、")))
	}

	reply.WriteString(utils.Emoji("F09F92A1") + "如果想要查看当前交易的物品，请发送【交易信息】！")
	core.SendGroupMessage(conn, event.GroupID, core_game.AtSender(event.UserID)+"\n"+reply.String())
}

// 6. 展示交易详情
func showTradeInfo(conn *websocket.Conn, event *core.OneBotEvent) {
	session := getTradeSession(event.UserID)
	if session == nil {
		core.SendGroupMessage(conn, event.GroupID, core_game.AtSender(event.UserID)+"\n"+utils.Emoji("E29D84")+"还没有人和阁下交易呐！")
		return
	}

	if session.UserAStatus == 0 || session.UserBStatus == 0 {
		core.SendGroupMessage(conn, event.GroupID, core_game.AtSender(event.UserID)+"\n"+utils.Emoji("E29D84")+"阁下请等待对方接受交易后，再查看交易信息哦！")
		return
	}

	formatItems := func(items map[string]int64) (string, string) {
		var coinDesc string
		var itemDescParts []string

		coinQty := items["[货币]"]
		if coinQty > 0 {
			coinDesc = fmt.Sprintf("%s交易%s：%d\n", utils.Emoji("F09F92B0"), config.Core.CoinName, coinQty)
		}

		for k, v := range items {
			if k == "[货币]" {
				continue
			}
			itemDescParts = append(itemDescParts, fmt.Sprintf("%s×%d", k, v))
		}

		itemDesc := "暂无\n"
		if len(itemDescParts) > 0 {
			itemDesc = strings.Join(itemDescParts, "、") + "\n"
		}

		return coinDesc, itemDesc
	}

	coinA, itemsA := formatItems(session.UserAItems)
	coinB, itemsB := formatItems(session.UserBItems)

	txFee := config.Core.TxFee

	replyText := fmt.Sprintf("%s交易信息%s\n%s\n%s%s交易物品：%s======================\n%s\n%s%s交易物品：%s======================\n%s【添加交易 物品*数量】添加想交易的物品\n%s【删除交易 物品*数量】删除想交易的物品\n%s本次交易需要双方分别支付%d%s手续费，如果双方同意交易，请双方发送【同意交易】",
		utils.Emoji("F09F939D"), utils.Emoji("F09F939D"),
		core_game.AtSender(session.UserAID), coinA, utils.Emoji("F09F93A6"), itemsA,
		core_game.AtSender(session.UserBID), coinB, utils.Emoji("F09F93A6"), itemsB,
		utils.Emoji("E29E95"), utils.Emoji("E29E96"), utils.Emoji("F09F92A1"),
		txFee, config.Core.CoinName)

	core.SendGroupMessage(conn, event.GroupID, replyText)
}

// 7. 取消交易（支持二级确认）
func cancelTrade(conn *websocket.Conn, event *core.OneBotEvent) {
	lockKey := utils.GetUserLockKey(event.GroupID, event.UserID)
	mutex := utils.GetLock(lockKey)
	mutex.Lock()
	defer mutex.Unlock()

	session := getTradeSession(event.UserID)
	if session == nil {
		core.SendGroupMessage(conn, event.GroupID, core_game.AtSender(event.UserID)+"\n"+utils.Emoji("E29D84")+"还没有人和阁下交易呐！")
		return
	}

	if event.UserID == session.UserAID {
		session.CancelA = true
	} else {
		session.CancelB = true
	}

	// 注册二级确认的临时指令处理器，或者直接引导玩家发送【确认取消】
	core.RegisterHandler("确认取消", confirmCancelTrade)

	replyText := fmt.Sprintf("%s阁下确定要取消交易吗？\n【%s】 【确认取消】",
		core_game.AtSender(event.UserID), utils.Emoji("E29AA0"))

	core.SendGroupMessage(conn, event.GroupID, replyText)
}

func confirmCancelTrade(conn *websocket.Conn, event *core.OneBotEvent) {
	lockKey := utils.GetUserLockKey(event.GroupID, event.UserID)
	mutex := utils.GetLock(lockKey)
	mutex.Lock()
	defer mutex.Unlock()

	session := getTradeSession(event.UserID)
	if session == nil {
		return
	}

	isConfirmed := (event.UserID == session.UserAID && session.CancelA) || (event.UserID == session.UserBID && session.CancelB)
	if !isConfirmed {
		return
	}

	// 销毁交易状态
	tradeMu.Lock()
	delete(sessions, session.UserAID)
	delete(sessions, session.UserBID)
	tradeMu.Unlock()

	replyText := fmt.Sprintf("%s\n%s阁下和%s的交易已取消。",
		core_game.AtSender(event.UserID), utils.Emoji("E299BB"), core_game.AtSender(session.UserAID+session.UserBID-event.UserID))

	core.SendGroupMessage(conn, event.GroupID, replyText)
}

// 8. 同意交易（双方达成一致，触发最终原子交割）
func agreeTrade(conn *websocket.Conn, event *core.OneBotEvent) {
	lockKey := utils.GetUserLockKey(event.GroupID, event.UserID)
	mutex := utils.GetLock(lockKey)
	mutex.Lock()
	defer mutex.Unlock()

	session := getTradeSession(event.UserID)
	if session == nil {
		core.SendGroupMessage(conn, event.GroupID, core_game.AtSender(event.UserID)+"\n"+utils.Emoji("E29D84")+"还没有人和阁下交易呐！")
		return
	}

	if session.UserAStatus == 0 || session.UserBStatus == 0 {
		core.SendGroupMessage(conn, event.GroupID, core_game.AtSender(event.UserID)+"\n"+utils.Emoji("E29D84")+"阁下请等待对方接受交易后，再进行交易操作哦！")
		return
	}

	isUserA := event.UserID == session.UserAID

	if (isUserA && session.UserAStatus == 2) || (!isUserA && session.UserBStatus == 2) {
		core.SendGroupMessage(conn, event.GroupID, core_game.AtSender(event.UserID)+"\n"+utils.Emoji("E29D84")+"阁下已经同意交易了，请勿重复操作哦！")
		return
	}

	// 必须双方都存有要交易的物品才合法（或者至少不是空的）
	if isUserA {
		session.UserAStatus = 2
	} else {
		session.UserBStatus = 2
	}

	// 检查是否双方都同意了
	if session.UserAStatus != 2 || session.UserBStatus != 2 {
		core.SendGroupMessage(conn, event.GroupID, core_game.AtSender(event.UserID)+"\n"+utils.Emoji("E29D84")+"阁下同意了本场交易！但是需要双方同时同意交易才可以完成交易，如需取消交易请发送【取消交易】。")
		return
	}

	// 双方都同意，执行最终数据库原子提交
	tx := database.DB.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// 加锁并获取宠物
	var petA, petB models.UserPet
	if err := tx.Set("gorm:query_option", "FOR UPDATE").Where("user_id = ? AND group_id = ?", session.UserAID, event.GroupID).First(&petA).Error; err != nil {
		tx.Rollback()
		core.SendGroupMessage(conn, event.GroupID, "交易执行失败，玩家A数据获取错误！")
		return
	}
	if err := tx.Set("gorm:query_option", "FOR UPDATE").Where("user_id = ? AND group_id = ?", session.UserBID, event.GroupID).First(&petB).Error; err != nil {
		tx.Rollback()
		core.SendGroupMessage(conn, event.GroupID, "交易执行失败，玩家B数据获取错误！")
		return
	}

	txFee := config.Core.TxFee

	// 1. 最终资产核对与扣减 (A 扣除 A 放入的资产，B 扣除 B 放入的资产)
	// A 货币
	coinA := session.UserAItems["[货币]"]
	if petA.Currency < coinA+txFee {
		tx.Rollback()
		core.SendGroupMessage(conn, event.GroupID, fmt.Sprintf("交易失败！%s的%s不足，无法支付交易金及手续费！", core_game.AtSender(session.UserAID), config.Core.CoinName))
		return
	}
	petA.Currency = petA.Currency - coinA - txFee

	// B 货币
	coinB := session.UserBItems["[货币]"]
	if petB.Currency < coinB+txFee {
		tx.Rollback()
		core.SendGroupMessage(conn, event.GroupID, fmt.Sprintf("交易失败！%s的%s不足，无法支付交易金及手续费！", core_game.AtSender(session.UserBID), config.Core.CoinName))
		return
	}
	petB.Currency = petB.Currency - coinB - txFee

	// A 扣除物品并核对
	for name, qty := range session.UserAItems {
		if name == "[货币]" {
			continue
		}
		var backpackItem models.BackpackItem
		if err := tx.Set("gorm:query_option", "FOR UPDATE").Where("user_id = ? AND group_id = ? AND item_name = ?", session.UserAID, event.GroupID, name).First(&backpackItem).Error; err != nil || backpackItem.Quantity < qty {
			tx.Rollback()
			core.SendGroupMessage(conn, event.GroupID, fmt.Sprintf("交易失败！%s的[%s]数量不足，无法完成交易！", core_game.AtSender(session.UserAID), name))
			return
		}
		backpackItem.Quantity -= qty
		if err := tx.Save(&backpackItem).Error; err != nil {
			tx.Rollback()
			return
		}
	}

	// B 扣除物品并核对
	for name, qty := range session.UserBItems {
		if name == "[货币]" {
			continue
		}
		var backpackItem models.BackpackItem
		if err := tx.Set("gorm:query_option", "FOR UPDATE").Where("user_id = ? AND group_id = ? AND item_name = ?", session.UserBID, event.GroupID, name).First(&backpackItem).Error; err != nil || backpackItem.Quantity < qty {
			tx.Rollback()
			core.SendGroupMessage(conn, event.GroupID, fmt.Sprintf("交易失败！%s的[%s]数量不足，无法完成交易！", core_game.AtSender(session.UserBID), name))
			return
		}
		backpackItem.Quantity -= qty
		if err := tx.Save(&backpackItem).Error; err != nil {
			tx.Rollback()
			return
		}
	}

	// 2. 接收资产发放
	// A 获得 B 放入的资产
	petA.Currency += coinB
	for name, qty := range session.UserBItems {
		if name == "[货币]" {
			continue
		}
		if err := core_game.AddBackpackItem(tx, session.UserAID, event.GroupID, name, qty); err != nil {
			tx.Rollback()
			return
		}
	}

	// B 获得 A 放入的资产
	petB.Currency += coinA
	for name, qty := range session.UserAItems {
		if name == "[货币]" {
			continue
		}
		if err := core_game.AddBackpackItem(tx, session.UserBID, event.GroupID, name, qty); err != nil {
			tx.Rollback()
			return
		}
	}

	if err := tx.Save(&petA).Error; err != nil {
		tx.Rollback()
		return
	}
	if err := tx.Save(&petB).Error; err != nil {
		tx.Rollback()
		return
	}

	tx.Commit()

	// 销毁交易
	tradeMu.Lock()
	delete(sessions, session.UserAID)
	delete(sessions, session.UserBID)
	tradeMu.Unlock()

	// 格式化输出
	formatGains := func(coin int64, items map[string]int64) (string, string) {
		var coinDesc string
		var itemDescParts []string

		if coin > 0 {
			coinDesc = fmt.Sprintf("%s获得%s：%d\n", utils.Emoji("F09F92B0"), config.Core.CoinName, coin)
		}

		for k, v := range items {
			if k == "[货币]" {
				continue
			}
			itemDescParts = append(itemDescParts, fmt.Sprintf("%s×%d", k, v))
		}

		itemDesc := "暂无\n"
		if len(itemDescParts) > 0 {
			itemDesc = strings.Join(itemDescParts, "、") + "\n"
		}

		return coinDesc, itemDesc
	}

	coinAStr, itemsAStr := formatGains(coinB, session.UserBItems)
	coinBStr, itemsBStr := formatGains(coinA, session.UserAItems)

	replyText := fmt.Sprintf("交易完成\n%s\n%s%s获得物品：%s========================\n%s\n%s%s获得物品：%s\n分别扣除了双方%d%s手续费！",
		core_game.AtSender(session.UserAID), coinAStr, utils.Emoji("F09F93A6"), itemsAStr,
		core_game.AtSender(session.UserBID), coinBStr, utils.Emoji("F09F93A6"), itemsBStr,
		txFee, config.Core.CoinName)

	core.SendGroupMessage(conn, event.GroupID, replyText)
}
