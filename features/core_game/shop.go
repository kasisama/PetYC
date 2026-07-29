package core_game

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/gorilla/websocket"
	"gorm.io/gorm"
	"qq-pet-saas/config"
	"qq-pet-saas/core"
	"qq-pet-saas/database"
	"qq-pet-saas/models"
	"qq-pet-saas/utils"
)

// PendingSell 挂起待确认出售的商品
type PendingSell struct {
	ItemName string
	Quantity int64
}

var (
	pendingSellMap sync.Map   // Key: int64 (UserID), Value: PendingSell
	shopMu         sync.Mutex // 保证商店库存增减的并发安全
)

func init() {
	// 动态注册指令处理器
	core.RegisterHandler(config.GetCommand("商店"), showShop)
	core.RegisterHandler(config.GetCommand("好感商店"), showAffectionShop)
	core.RegisterHandler(config.GetCommand("查看商品"), viewGoods)
	core.RegisterHandler(config.GetCommand("查看物品"), viewItem)
	core.RegisterHandler(config.GetCommand("购买"), buyItem)
	core.RegisterHandler(config.GetCommand("出售"), sellItem)
	core.RegisterHandler(config.GetCommand("继续出售"), continueSell)
	core.RegisterHandler(config.GetCommand("取消出售"), cancelSell)
}

// 1. 展示普通商店
func showShop(conn *websocket.Conn, event *core.OneBotEvent) {
	pet, ok := GetPetOrReply(conn, event)
	if !ok {
		return
	}

	chkMsg := CheckPlayerStatus(pet, "忽略检测")
	if chkMsg != "" {
		core.SendGroupMessage(conn, event.GroupID, chkMsg)
		return
	}

	prefix := config.GetCommand("商店")
	args := strings.TrimSpace(strings.TrimPrefix(event.RawMessage, prefix))

	page := 1
	if args != "" {
		if p, err := strconv.Atoi(args); err == nil {
			page = p
		}
	}
	if page <= 0 {
		page = 1
	}

	// 收集并排序所有商店商品
	shopMu.Lock()
	var itemNames []string
	for name := range config.Shop {
		itemNames = append(itemNames, name)
	}
	shopMu.Unlock()

	sort.Strings(itemNames)
	totalItems := len(itemNames)
	if totalItems == 0 {
		core.SendGroupMessage(conn, event.GroupID, fmt.Sprintf("%s\n%s商店内没有任何物品出售。", AtSender(event.UserID), utils.Emoji("E29D84")))
		return
	}

	totalPages := (totalItems + 4) / 5
	startIndex := (page - 1) * 5
	if startIndex >= totalItems {
		core.SendGroupMessage(conn, event.GroupID, fmt.Sprintf("%s\n%s阁下，商店页数输入错误啦！", AtSender(event.UserID), utils.Emoji("E29D84")))
		return
	}

	var listText strings.Builder
	shopMu.Lock()
	for i := startIndex; i < startIndex+5 && i < totalItems; i++ {
		name := itemNames[i]
		goods := config.Shop[name]
		listText.WriteString(fmt.Sprintf("[%s]\n%s价格：%d%s\n", name, utils.Emoji("F09F92B0"), goods.Price, config.Core.CoinName))
	}
	shopMu.Unlock()

	replyText := fmt.Sprintf("%s宠物商店%s\n%s- - - - - - - - - - - - - - - - -\n当前页数：[%d/%d]\n%s翻页指令：%s 页数\n%s商品详细：%s 商品名\n%s购买指令：%s 商品名*数量",
		utils.Emoji("F09F8FAA"), utils.Emoji("F09F8FAA"),
		listText.String(),
		page, totalPages,
		utils.Emoji("F09F92A1"), config.GetCommand("商店"),
		utils.Emoji("F09F92A1"), config.GetCommand("查看商品"),
		utils.Emoji("F09F948D"), config.GetCommand("购买"))

	core.SendGroupMessage(conn, event.GroupID, AtSender(event.UserID)+"\n"+replyText)
}

// 2. 展示好感商店
func showAffectionShop(conn *websocket.Conn, event *core.OneBotEvent) {
	pet, ok := GetPetOrReply(conn, event)
	if !ok {
		return
	}

	chkMsg := CheckPlayerStatus(pet, "忽略检测")
	if chkMsg != "" {
		core.SendGroupMessage(conn, event.GroupID, chkMsg)
		return
	}

	prefix := config.GetCommand("好感商店")
	args := strings.TrimSpace(strings.TrimPrefix(event.RawMessage, prefix))

	page := 1
	if args != "" {
		if p, err := strconv.Atoi(args); err == nil {
			page = p
		}
	}
	if page <= 0 {
		page = 1
	}

	// 收集并排序所有好感商店商品
	shopMu.Lock()
	var itemNames []string
	for name := range config.AffectionShop {
		itemNames = append(itemNames, name)
	}
	shopMu.Unlock()

	sort.Strings(itemNames)
	totalItems := len(itemNames)
	if totalItems == 0 {
		core.SendGroupMessage(conn, event.GroupID, fmt.Sprintf("%s\n%s好感商店内没有任何物品出售。", AtSender(event.UserID), utils.Emoji("E29D84")))
		return
	}

	totalPages := (totalItems + 4) / 5
	startIndex := (page - 1) * 5
	if startIndex >= totalItems {
		core.SendGroupMessage(conn, event.GroupID, fmt.Sprintf("%s\n%s阁下，好感商店页数输入错误啦！", AtSender(event.UserID), utils.Emoji("E29D84")))
		return
	}

	var listText strings.Builder
	shopMu.Lock()
	for i := startIndex; i < startIndex+5 && i < totalItems; i++ {
		name := itemNames[i]
		goods := config.AffectionShop[name]
		listText.WriteString(fmt.Sprintf("[%s]\n%s价格：%d好感\n", name, utils.Emoji("F09F9296"), goods.Price))
	}
	shopMu.Unlock()

	replyText := fmt.Sprintf("%s宠物好感商店%s\n%s- - - - - - - - - - - - - - - - -\n当前页数：[%d/%d]\n%s翻页指令：%s 页数\n%s商品详细：%s 商品名\n%s购买指令：%s 商品名*数量",
		utils.Emoji("F09F929D"), utils.Emoji("F09F929D"),
		listText.String(),
		page, totalPages,
		utils.Emoji("F09F92A1"), config.GetCommand("好感商店"),
		utils.Emoji("F09F92A1"), config.GetCommand("查看商品"),
		utils.Emoji("F09F948D"), config.GetCommand("购买"))

	core.SendGroupMessage(conn, event.GroupID, AtSender(event.UserID)+"\n"+replyText)
}

// 3. 查看具体商品
func viewGoods(conn *websocket.Conn, event *core.OneBotEvent) {
	pet, ok := GetPetOrReply(conn, event)
	if !ok {
		return
	}

	chkMsg := CheckPlayerStatus(pet, "忽略检测")
	if chkMsg != "" {
		core.SendGroupMessage(conn, event.GroupID, chkMsg)
		return
	}

	prefix := config.GetCommand("查看商品")
	target := strings.TrimSpace(strings.TrimPrefix(event.RawMessage, prefix))
	if target == "" {
		core.SendGroupMessage(conn, event.GroupID, fmt.Sprintf("%s\n%s阁下请说要查看什么商品呀，正确姿势【%s 商品名】！", AtSender(event.UserID), utils.Emoji("E29D84"), prefix))
		return
	}

	shopMu.Lock()
	goods, inShop := config.Shop[target]
	affGoods, inAffShop := config.AffectionShop[target]
	shopMu.Unlock()

	if !inShop && !inAffShop {
		core.SendGroupMessage(conn, event.GroupID, fmt.Sprintf("%s\n%s没有找到阁下说的这个[%s]商品呀！", AtSender(event.UserID), utils.Emoji("E29D84"), target))
		return
	}

	var priceText string
	var shopType string
	var imgPath string
	var stock int64
	var desc string

	if inShop {
		priceText = fmt.Sprintf("%s价格：%d%s", utils.Emoji("F09F92B0"), goods.Price, config.Core.CoinName)
		shopType = "普通商店"
		imgPath = goods.Image
		stock = goods.Stock
		desc = goods.Description
	} else {
		priceText = fmt.Sprintf("%s价格：%d好感", utils.Emoji("F09F9296"), affGoods.Price)
		shopType = "好感商店"
		imgPath = affGoods.Image
		stock = affGoods.Stock
		desc = affGoods.Description
	}

	imageCQ := GetImageCQ(imgPath)

	replyText := fmt.Sprintf("%s%s\n【%s】\n%s库存：%d\n%s\n%s描述：%s\n注：此商品为%s里的哦！",
		AtSender(event.UserID), imageCQ, target,
		utils.Emoji("F09F93A6"), stock,
		priceText,
		utils.Emoji("F09F92AC"), desc,
		shopType)

	core.SendGroupMessage(conn, event.GroupID, replyText)
}

// 4. 查看物品（背包内物品详情）
func viewItem(conn *websocket.Conn, event *core.OneBotEvent) {
	pet, ok := GetPetOrReply(conn, event)
	if !ok {
		return
	}

	chkMsg := CheckPlayerStatus(pet, "忽略检测")
	if chkMsg != "" {
		core.SendGroupMessage(conn, event.GroupID, chkMsg)
		return
	}

	prefix := config.GetCommand("查看物品")
	target := strings.TrimSpace(strings.TrimPrefix(event.RawMessage, prefix))
	if target == "" {
		core.SendGroupMessage(conn, event.GroupID, fmt.Sprintf("%s\n%s阁下请说要查看什么物品呀，正确姿势【%s 物品名】！", AtSender(event.UserID), utils.Emoji("E29D84"), prefix))
		return
	}

	item, exists := config.Items[target]
	if !exists {
		core.SendGroupMessage(conn, event.GroupID, fmt.Sprintf("%s\n%s没有找到阁下说的这个[%s]物品呀！", AtSender(event.UserID), utils.Emoji("E29D84"), target))
		return
	}

	imageCQ := GetImageCQ(item.Image)

	var details string
	switch item.Type {
	case "礼包":
		var previewRewards []string
		if item.Effect != "" {
			parts := strings.Split(item.Effect, "#")
			for _, part := range parts {
				part = strings.TrimSpace(part)
				if part == "" {
					continue
				}
				subParts := strings.Split(part, "*")
				left := strings.TrimSpace(subParts[0])
				right := "1"
				if len(subParts) > 1 {
					right = strings.ReplaceAll(strings.TrimSpace(subParts[1]), "~", "-")
				}
				previewRewards = append(previewRewards, fmt.Sprintf("%s×%s", left, right))
			}
		}
		openReqText := ""
		if item.OpenReq != "" {
			openReqText = fmt.Sprintf("\n%s打开所需：%s", utils.Emoji("F09F9491"), item.OpenReq)
		}
		details = fmt.Sprintf("%s类型：%s%s礼包%s\n%s预览奖励：%s\n%s描述：%s\n%s出售价格：%d",
			utils.Emoji("F09F938C"), item.RewardType, utils.Emoji("F09F8E81"), openReqText,
			utils.Emoji("F09F8C9F"), strings.Join(previewRewards, "、"),
			utils.Emoji("F09F92AC"), item.Description,
			utils.Emoji("F09F92B2"), item.SellPrice)

	case "血量":
		details = fmt.Sprintf("%s类型：血量\n%s恢复效果：%s\n%s描述：%s\n%s出售价格：%d",
			utils.Emoji("F09F938C"), utils.Emoji("F09F929F"), item.Effect,
			utils.Emoji("F09F92AC"), item.Description,
			utils.Emoji("F09F92B2"), item.SellPrice)

	case "饱食":
		details = fmt.Sprintf("%s类型：饱食\n%s恢复饱食：%s\n%s描述：%s\n%s出售价格：%d",
			utils.Emoji("F09F938C"), utils.Emoji("F09F8D94"), item.Effect,
			utils.Emoji("F09F92AC"), item.Description,
			utils.Emoji("F09F92B2"), item.SellPrice)

	case "好感":
		details = fmt.Sprintf("%s类型：礼物\n%s增加好感：%s\n%s描述：%s\n%s出售价格：%d",
			utils.Emoji("F09F938C"), utils.Emoji("F09F9296"), item.Effect,
			utils.Emoji("F09F92AC"), item.Description,
			utils.Emoji("F09F92B2"), item.SellPrice)

	case "智慧":
		details = fmt.Sprintf("%s类型：学习\n%s增加智慧：%s\n%s描述：%s\n%s出售价格：%d",
			utils.Emoji("F09F938C"), utils.Emoji("E29B8E"), item.Effect,
			utils.Emoji("F09F92AC"), item.Description,
			utils.Emoji("F09F92B2"), item.SellPrice)

	case "力量":
		details = fmt.Sprintf("%s类型：力量\n%s增加力量：%s\n%s描述：%s\n%s出售价格：%d",
			utils.Emoji("F09F938C"), utils.Emoji("F09F92AA"), item.Effect,
			utils.Emoji("F09F92AC"), item.Description,
			utils.Emoji("F09F92B2"), item.SellPrice)

	case "鱼类":
		details = fmt.Sprintf("%s类型：鱼类\n%s描述：%s\n%s出售价格：%d",
			utils.Emoji("F09F938C"), utils.Emoji("F09F92AC"), item.Description,
			utils.Emoji("F09F92B2"), item.SellPrice)

	case "家族":
		details = fmt.Sprintf("%s类型：家族\n%s描述：%s\n%s出售价格：%d",
			utils.Emoji("F09F938C"), utils.Emoji("F09F92AC"), item.Description,
			utils.Emoji("F09F92B2"), item.SellPrice)

	case "神树":
		details = fmt.Sprintf("%s类型：神树\n%s增加全属性：%s\n%s描述：%s\n%s出售价格：%d",
			utils.Emoji("F09F938C"), utils.Emoji("F09F92AC"), item.Effect,
			utils.Emoji("F09F92AC"), item.Description,
			utils.Emoji("F09F92B2"), item.SellPrice)

	case "觉醒", "进化觉醒":
		details = fmt.Sprintf("%s类型：觉醒\n%s描述：%s\n%s出售价格：%d",
			utils.Emoji("F09F938C"), utils.Emoji("F09F92AC"), item.Description,
			utils.Emoji("F09F92B2"), item.SellPrice)

	case "抽奖":
		details = fmt.Sprintf("%s类型：抽奖\n%s描述：%s\n%s出售价格：%d",
			utils.Emoji("F09F938C"), utils.Emoji("F09F92AC"), item.Description,
			utils.Emoji("F09F92B2"), item.SellPrice)

	case "防御":
		details = fmt.Sprintf("%s类型：防御\n%s增加防御：%s\n%s描述：%s\n%s出售价格：%d",
			utils.Emoji("F09F938C"), utils.Emoji("F09F94B0"), item.Effect,
			utils.Emoji("F09F92AC"), item.Description,
			utils.Emoji("F09F92B2"), item.SellPrice)

	case "成长":
		details = fmt.Sprintf("%s类型：成长\n%s增加成长：%s\n%s描述：%s\n%s出售价格：%d",
			utils.Emoji("F09F938C"), utils.Emoji("F09F8C9F"), item.Effect,
			utils.Emoji("F09F92AC"), item.Description,
			utils.Emoji("F09F92B2"), item.SellPrice)

	default:
		details = fmt.Sprintf("%s数据类型错误，请去控制台修改。", utils.Emoji("E29D8C"))
	}

	replyText := fmt.Sprintf("%s%s\n【%s】\n%s", AtSender(event.UserID), imageCQ, target, details)
	core.SendGroupMessage(conn, event.GroupID, replyText)
}

// 5. 购买商品
func buyItem(conn *websocket.Conn, event *core.OneBotEvent) {
	// 使用并发锁
	lockKey := utils.GetUserLockKey(event.GroupID, event.UserID)
	mutex := utils.GetLock(lockKey)
	mutex.Lock()
	defer mutex.Unlock()

	pet, ok := GetPetOrReply(conn, event)
	if !ok {
		return
	}

	chkMsg := CheckPlayerStatus(pet, "忽略检测")
	if chkMsg != "" {
		core.SendGroupMessage(conn, event.GroupID, chkMsg)
		return
	}

	prefix := config.GetCommand("购买")
	args := strings.TrimSpace(strings.TrimPrefix(event.RawMessage, prefix))
	if args == "" {
		core.SendGroupMessage(conn, event.GroupID, fmt.Sprintf("%s\n%s阁下，如果想购买物品的话，请按照正确姿势【%s 物品名】再来一次哦！", AtSender(event.UserID), utils.Emoji("E29D84"), prefix))
		return
	}

	// 解析物品名与数量 (如: 苹果*5 或 苹果)
	itemName := args
	quantity := int64(1)
	if strings.Contains(args, "*") {
		parts := strings.Split(args, "*")
		itemName = strings.TrimSpace(parts[0])
		if q, err := strconv.ParseInt(strings.TrimSpace(parts[1]), 10, 64); err == nil {
			quantity = q
		}
	} else if strings.Contains(args, "×") {
		parts := strings.Split(args, "×")
		itemName = strings.TrimSpace(parts[0])
		if q, err := strconv.ParseInt(strings.TrimSpace(parts[1]), 10, 64); err == nil {
			quantity = q
		}
	}

	if itemName == "" || quantity <= 0 {
		core.SendGroupMessage(conn, event.GroupID, fmt.Sprintf("%s\n%s阁下输入的购买信息不合法，请检查！", AtSender(event.UserID), utils.Emoji("E29D84")))
		return
	}

	// 限制数值大小防溢出
	if quantity > 99999999 {
		core.SendGroupMessage(conn, event.GroupID, fmt.Sprintf("%s\n%s阁下输入的数值过大，请检查后重新输入哟！", AtSender(event.UserID), utils.Emoji("E29D84")))
		return
	}

	// 限购检查
	buyLimit := config.Interaction.BuyLimit
	if buyLimit <= 0 {
		buyLimit = 10
	}
	if quantity > int64(buyLimit) {
		core.SendGroupMessage(conn, event.GroupID, fmt.Sprintf("%s\n%s商店批量购买一次只能购买%d个哦！", AtSender(event.UserID), utils.Emoji("E29D84"), buyLimit))
		return
	}

	shopMu.Lock()
	goods, inShop := config.Shop[itemName]
	affGoods, inAffShop := config.AffectionShop[itemName]
	shopMu.Unlock()

	if !inShop && !inAffShop {
		core.SendGroupMessage(conn, event.GroupID, fmt.Sprintf("%s\n%s商店和好感商店里都没有找到阁下说的这个[%s]哦！", AtSender(event.UserID), utils.Emoji("E29D84"), itemName))
		return
	}

	var isAffShop bool
	var price int64
	var stock int64
	var imageCQ string

	if inShop {
		isAffShop = false
		price = goods.Price
		stock = goods.Stock
		imageCQ = GetImageCQ(goods.Image)
	} else {
		isAffShop = true
		price = affGoods.Price
		stock = affGoods.Stock
		imageCQ = GetImageCQ(affGoods.Image)
	}

	// 检查库存
	if stock != -1 && stock < quantity {
		core.SendGroupMessage(conn, event.GroupID, fmt.Sprintf("%s\n%s阁下，商店里的'%s'库存只有%d个了，请等待下次进货或者调整数量哦！", AtSender(event.UserID), utils.Emoji("E29D84"), itemName, stock))
		return
	}

	// 检查币种
	var userMoney int64
	if !isAffShop {
		userMoney = pet.Currency
	} else {
		userMoney = pet.Affection
	}

	totalCost := price * quantity
	if userMoney < totalCost {
		if !isAffShop {
			core.SendGroupMessage(conn, event.GroupID, fmt.Sprintf("%s\n%s阁下的%s不足%d个，不能购买%d个'%s'！", AtSender(event.UserID), utils.Emoji("E29D84"), config.Core.CoinName, totalCost, quantity, itemName))
		} else {
			core.SendGroupMessage(conn, event.GroupID, fmt.Sprintf("%s\n%s阁下的好感点数不足%d点，不能购买%d个'%s'哦！", AtSender(event.UserID), utils.Emoji("E29D84"), totalCost, quantity, itemName))
		}
		return
	}

	itemConfig, hasItemConfig := config.Items[itemName]
	if !hasItemConfig {
		core.SendGroupMessage(conn, event.GroupID, fmt.Sprintf("%s\n%s没有找到该物品的基础配置，无法购买！", AtSender(event.UserID), utils.Emoji("E29D84")))
		return
	}

	tx := database.DB.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// 获得类型 == 1 校验：一生仅能获得一次
	if itemConfig.ObtainType == 1 {
		var count int64
		tx.Model(&models.BackpackItem{}).Where("user_id = ? AND group_id = ? AND item_name = ?", event.UserID, event.GroupID, itemName).Count(&count)
		if count > 0 {
			tx.Rollback()
			core.SendGroupMessage(conn, event.GroupID, fmt.Sprintf("%s\n%s阁下，这个物品一个人只能获得一次，所以不能购买哦！", AtSender(event.UserID), utils.Emoji("E29D84")))
			return
		}
		// 只能买 1 个
		if quantity > 1 {
			tx.Rollback()
			core.SendGroupMessage(conn, event.GroupID, fmt.Sprintf("%s\n%s该特种物品单次且终生只能购买1个哦！", AtSender(event.UserID), utils.Emoji("E29D84")))
			return
		}
	}

	// 扣减货币
	if !isAffShop {
		pet.Currency -= totalCost
	} else {
		pet.Affection -= totalCost
	}

	// 保存宠物状态
	if err := tx.Save(pet).Error; err != nil {
		tx.Rollback()
		core.SendGroupMessage(conn, event.GroupID, fmt.Sprintf("数据库保存失败: %v", err))
		return
	}

	// 增加到背包
	if err := AddBackpackItem(tx, event.UserID, event.GroupID, itemName, quantity); err != nil {
		tx.Rollback()
		core.SendGroupMessage(conn, event.GroupID, fmt.Sprintf("添加到背包失败: %v", err))
		return
	}

	// 更新内存库存
	shopMu.Lock()
	if !isAffShop {
		if goods.Stock != -1 {
			goods.Stock -= quantity
			config.Shop[itemName] = goods
		}
	} else {
		if affGoods.Stock != -1 {
			affGoods.Stock -= quantity
			config.AffectionShop[itemName] = affGoods
		}
	}
	shopMu.Unlock()

	tx.Commit()

	var successText string
	if !isAffShop {
		successText = fmt.Sprintf("%s阁下购买了%s×%d，消费了%d%s。", utils.Emoji("F09F92B2"), itemName, quantity, totalCost, config.Core.CoinName)
	} else {
		successText = fmt.Sprintf("%s阁下获得了%s×%d，使用了%d好感点数哦。", utils.Emoji("F09F929D"), itemName, quantity, totalCost)
	}

	core.SendGroupMessage(conn, event.GroupID, AtSender(event.UserID)+imageCQ+"\n"+successText)
}

// 6. 出售商品
func sellItem(conn *websocket.Conn, event *core.OneBotEvent) {
	lockKey := utils.GetUserLockKey(event.GroupID, event.UserID)
	mutex := utils.GetLock(lockKey)
	mutex.Lock()
	defer mutex.Unlock()

	pet, ok := GetPetOrReply(conn, event)
	if !ok {
		return
	}

	chkMsg := CheckPlayerStatus(pet, "忽略检测")
	if chkMsg != "" {
		core.SendGroupMessage(conn, event.GroupID, chkMsg)
		return
	}

	prefix := config.GetCommand("出售")
	args := strings.TrimSpace(strings.TrimPrefix(event.RawMessage, prefix))
	if args == "" {
		core.SendGroupMessage(conn, event.GroupID, fmt.Sprintf("%s\n%s阁下，如果想出售物品的话，请按照正确姿势【%s 物品名】再来一次哦！", AtSender(event.UserID), utils.Emoji("E29D84"), prefix))
		return
	}

	itemName := args
	quantity := int64(1)
	if strings.Contains(args, "*") {
		parts := strings.Split(args, "*")
		itemName = strings.TrimSpace(parts[0])
		if q, err := strconv.ParseInt(strings.TrimSpace(parts[1]), 10, 64); err == nil {
			quantity = q
		}
	} else if strings.Contains(args, "×") {
		parts := strings.Split(args, "×")
		itemName = strings.TrimSpace(parts[0])
		if q, err := strconv.ParseInt(strings.TrimSpace(parts[1]), 10, 64); err == nil {
			quantity = q
		}
	}

	if itemName == "" || quantity <= 0 {
		core.SendGroupMessage(conn, event.GroupID, fmt.Sprintf("%s\n%s阁下，请指定合法的出售商品名称与数量！", AtSender(event.UserID), utils.Emoji("E29D84")))
		return
	}

	if quantity > 99999999 {
		core.SendGroupMessage(conn, event.GroupID, fmt.Sprintf("%s\n%s阁下输入的数值过大，请检查后重新输入哟！", AtSender(event.UserID), utils.Emoji("E29D84")))
		return
	}

	itemConfig, exists := config.Items[itemName]
	if !exists {
		core.SendGroupMessage(conn, event.GroupID, fmt.Sprintf("%s\n%s没有找到该物品的基础配置，无法出售！", AtSender(event.UserID), utils.Emoji("E29D84")))
		return
	}

	// 查背包内物品数量
	var backpackItem models.BackpackItem
	err := database.DB.Where("user_id = ? AND group_id = ? AND item_name = ?", event.UserID, event.GroupID, itemName).First(&backpackItem).Error
	if err == gorm.ErrRecordNotFound || backpackItem.Quantity <= 0 {
		core.SendGroupMessage(conn, event.GroupID, fmt.Sprintf("%s\n%s阁下没有这个物品哟！", AtSender(event.UserID), utils.Emoji("E29D84")))
		return
	}
	if backpackItem.Quantity < quantity {
		core.SendGroupMessage(conn, event.GroupID, fmt.Sprintf("%s\n%s阁下背包里没有这么多该物品的啦！", AtSender(event.UserID), utils.Emoji("E29D84")))
		return
	}

	// 如果出售价格 <= 0，触发二次确认逻辑 (转成长)
	if itemConfig.SellPrice <= 0 {
		pendingSellMap.Store(event.UserID, PendingSell{
			ItemName: itemName,
			Quantity: quantity,
		})
		replyText := fmt.Sprintf("%s\n%s当前物品出售价格为0，请问阁下是否继续出售？\n【%s】  【%s】",
			AtSender(event.UserID), utils.Emoji("E29D97"), config.GetCommand("继续出售"), config.GetCommand("取消出售"))
		core.SendGroupMessage(conn, event.GroupID, replyText)
		return
	}

	// 出售价格 > 0
	tx := database.DB.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// 减少背包
	if err := AddBackpackItem(tx, event.UserID, event.GroupID, itemName, -quantity); err != nil {
		tx.Rollback()
		core.SendGroupMessage(conn, event.GroupID, fmt.Sprintf("扣减背包物品失败: %v", err))
		return
	}

	// 增加金币
	gainCoin := itemConfig.SellPrice * quantity
	pet.Currency += gainCoin

	if err := tx.Save(pet).Error; err != nil {
		tx.Rollback()
		core.SendGroupMessage(conn, event.GroupID, fmt.Sprintf("保存玩家数据失败: %v", err))
		return
	}

	tx.Commit()

	replyText := fmt.Sprintf("%s\n%s阁下出售了 %d 个 %s，获得了 %d %s！",
		AtSender(event.UserID), utils.Emoji("F09F92B0"), quantity, itemName, gainCoin, config.Core.CoinName)
	core.SendGroupMessage(conn, event.GroupID, replyText)
}

// 7. 继续出售 (0元物品转换为宠物成长值)
func continueSell(conn *websocket.Conn, event *core.OneBotEvent) {
	lockKey := utils.GetUserLockKey(event.GroupID, event.UserID)
	mutex := utils.GetLock(lockKey)
	mutex.Lock()
	defer mutex.Unlock()

	pet, ok := GetPetOrReply(conn, event)
	if !ok {
		return
	}

	chkMsg := CheckPlayerStatus(pet, "忽略检测")
	if chkMsg != "" {
		core.SendGroupMessage(conn, event.GroupID, chkMsg)
		return
	}

	val, loaded := pendingSellMap.Load(event.UserID)
	if !loaded {
		core.SendGroupMessage(conn, event.GroupID, fmt.Sprintf("%s\n%s阁下没有要出售的东西呀！", AtSender(event.UserID), utils.Emoji("E29D84")))
		return
	}
	pendingSellMap.Delete(event.UserID)

	pending := val.(PendingSell)

	// 双重校验背包数量
	var backpackItem models.BackpackItem
	err := database.DB.Where("user_id = ? AND group_id = ? AND item_name = ?", event.UserID, event.GroupID, pending.ItemName).First(&backpackItem).Error
	if err == gorm.ErrRecordNotFound || backpackItem.Quantity < pending.Quantity {
		core.SendGroupMessage(conn, event.GroupID, fmt.Sprintf("%s\n%s阁下的物品数量不足，无法出售！", AtSender(event.UserID), utils.Emoji("E29D84")))
		return
	}

	// 转换为成长值
	growthRate := config.Interaction.SellNoPriceGrowth
	if growthRate <= 0 {
		growthRate = 10
	}
	gainGrowth := growthRate * pending.Quantity

	tx := database.DB.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// 扣背包
	if err := AddBackpackItem(tx, event.UserID, event.GroupID, pending.ItemName, -pending.Quantity); err != nil {
		tx.Rollback()
		core.SendGroupMessage(conn, event.GroupID, fmt.Sprintf("扣减背包失败: %v", err))
		return
	}

	// 增成长
	pet.Growth += gainGrowth
	if err := tx.Save(pet).Error; err != nil {
		tx.Rollback()
		core.SendGroupMessage(conn, event.GroupID, fmt.Sprintf("保存宠物成长失败: %v", err))
		return
	}

	tx.Commit()

	replyText := fmt.Sprintf("%s\n%s阁下捐出了 %d 个 %s，宠物成长＋ %d！",
		AtSender(event.UserID), utils.Emoji("F09F94A5"), pending.Quantity, pending.ItemName, gainGrowth)
	core.SendGroupMessage(conn, event.GroupID, replyText)
}

// 8. 取消出售
func cancelSell(conn *websocket.Conn, event *core.OneBotEvent) {
	lockKey := utils.GetUserLockKey(event.GroupID, event.UserID)
	mutex := utils.GetLock(lockKey)
	mutex.Lock()
	defer mutex.Unlock()

	pet, ok := GetPetOrReply(conn, event)
	if !ok {
		return
	}

	chkMsg := CheckPlayerStatus(pet, "忽略检测")
	if chkMsg != "" {
		core.SendGroupMessage(conn, event.GroupID, chkMsg)
		return
	}

	val, loaded := pendingSellMap.Load(event.UserID)
	if !loaded {
		core.SendGroupMessage(conn, event.GroupID, fmt.Sprintf("%s\n%s阁下没有要取消出售的东西呀！", AtSender(event.UserID), utils.Emoji("E29D84")))
		return
	}
	pendingSellMap.Delete(event.UserID)

	pending := val.(PendingSell)

	replyText := fmt.Sprintf("%s\n%s阁下取消出售了 %d 个 %s！",
		AtSender(event.UserID), utils.Emoji("F09F9499"), pending.Quantity, pending.ItemName)
	core.SendGroupMessage(conn, event.GroupID, replyText)
}
