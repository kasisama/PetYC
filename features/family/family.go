package family

import (
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"gorm.io/gorm"
	"qq-pet-saas/config"
	"qq-pet-saas/core"
	"qq-pet-saas/features/core_game"
	"qq-pet-saas/database"
	"qq-pet-saas/models"
	"qq-pet-saas/utils"
)

var (
	lastWaterTimeMap sync.Map // Key: int64 (UserID), Value: time.Time
)

func init() {
	core.RegisterHandler(config.GetCommand("创建家族"), createFamily)
	core.RegisterHandler(config.GetCommand("加入家族"), joinFamily)
	core.RegisterHandler(config.GetCommand("注销家族"), disbandFamily)
	core.RegisterHandler(config.GetCommand("退出家族"), leaveFamily)
	core.RegisterHandler(config.GetCommand("我的家族"), myFamily)
	core.RegisterHandler(config.GetCommand("神树浇水"), waterTree)
	core.RegisterHandler(config.GetCommand("家族列表"), listFamilies)
	core.RegisterHandler(config.GetCommand("家族成员"), showFamilyMembers)
	core.RegisterHandler(config.GetCommand("踢出成员"), kickMember)
}

// 解析 At QQ号
func parseAtQQ(msg string) int64 {
	idx := strings.Index(msg, "[CQ:at,qq=")
	if idx == -1 {
		return 0
	}
	sub := msg[idx+len("[CQ:at,qq="):]
	end := strings.Index(sub, "]")
	if end == -1 {
		return 0
	}
	qqStr := sub[:end]
	qq, _ := strconv.ParseInt(qqStr, 10, 64)
	return qq
}

// 1. 创建家族
func createFamily(conn *websocket.Conn, event *core.OneBotEvent) {
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

	prefix := config.GetCommand("创建家族")
	familyName := strings.TrimSpace(strings.TrimPrefix(event.RawMessage, prefix))
	if familyName == "" {
		core.SendGroupMessage(conn, event.GroupID, fmt.Sprintf("%s\n%s阁下如果想创建家族的话，请发送【%s 家族名】哦！", core_game.AtSender(event.UserID), utils.Emoji("E29D84"), prefix))
		return
	}

	if pet.Family != "" {
		core.SendGroupMessage(conn, event.GroupID, core_game.AtSender(event.UserID)+"\n"+utils.Emoji("E29D84")+"阁下已经有家族了！")
		return
	}

	// 货币校验
	reqCoin := config.Interaction.CreateFamilyCoin
	if pet.Currency < reqCoin {
		core.SendGroupMessage(conn, event.GroupID, fmt.Sprintf("%s\n%s阁下还需要准备%d%s才能创建家族哦！",
			core_game.AtSender(event.UserID), utils.Emoji("E29D84"), reqCoin-pet.Currency, config.Core.CoinName))
		return
	}

	// 物品校验
	reqItemStr := config.Interaction.CreateFamilyItem
	if reqItemStr == "" {
		reqItemStr = "家族商标*1"
	}

	reqItemName := reqItemStr
	var reqItemQty int64 = 1
	if strings.Contains(reqItemStr, "*") {
		parts := strings.Split(reqItemStr, "*")
		reqItemName = strings.TrimSpace(parts[0])
		reqItemQty, _ = strconv.ParseInt(strings.TrimSpace(parts[1]), 10, 64)
	}

	var backpackItem models.BackpackItem
	err := database.DB.Where("user_id = ? AND group_id = ? AND item_name = ?", event.UserID, event.GroupID, reqItemName).First(&backpackItem).Error
	if err == gorm.ErrRecordNotFound || backpackItem.Quantity < reqItemQty {
		core.SendGroupMessage(conn, event.GroupID, fmt.Sprintf("%s\n%s阁下要想创建家族还需要%d个%s哦！",
			core_game.AtSender(event.UserID), utils.Emoji("E29D84"), reqItemQty-backpackItem.Quantity, reqItemName))
		return
	}

	// 重名校验
	var count int64
	database.DB.Model(&models.Family{}).Where("name = ?", familyName).Count(&count)
	if count > 0 {
		core.SendGroupMessage(conn, event.GroupID, fmt.Sprintf("%s\n%s目标[%s]已存在，请换个名字创建家族！",
			core_game.AtSender(event.UserID), utils.Emoji("E29D84"), familyName))
		return
	}

	tx := database.DB.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// 扣减资源
	if err := core_game.AddBackpackItem(tx, event.UserID, event.GroupID, reqItemName, -reqItemQty); err != nil {
		tx.Rollback()
		return
	}

	pet.Currency -= reqCoin
	pet.Family = familyName
	pet.FamilyScore = 0

	if err := tx.Save(pet).Error; err != nil {
		tx.Rollback()
		return
	}

	// 创建家族表记录
	maxSize := config.Interaction.FamilySizeLimit
	if maxSize <= 0 {
		maxSize = 10
	}

	familyObj := models.Family{
		Name:          familyName,
		LeaderID:      event.UserID,
		CurrentSize:   1,
		MaxSize:       maxSize,
		TreeNutrients: 0,
	}

	if err := tx.Create(&familyObj).Error; err != nil {
		tx.Rollback()
		return
	}

	tx.Commit()

	core.SendGroupMessage(conn, event.GroupID, fmt.Sprintf("%s\n%s创建[%s]家族成功！\n创建家族消耗了阁下%d个%s与%d%s！",
		core_game.AtSender(event.UserID), utils.Emoji("E29C89"), familyName, reqItemQty, reqItemName, reqCoin, config.Core.CoinName))
}

// 2. 加入家族
func joinFamily(conn *websocket.Conn, event *core.OneBotEvent) {
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

	if pet.Family != "" {
		core.SendGroupMessage(conn, event.GroupID, core_game.AtSender(event.UserID)+"\n"+utils.Emoji("E29D84")+"阁下已经有家族了！")
		return
	}

	prefix := config.GetCommand("加入家族")
	targetFamily := strings.TrimSpace(strings.TrimPrefix(event.RawMessage, prefix))
	if targetFamily == "" {
		core.SendGroupMessage(conn, event.GroupID, fmt.Sprintf("%s\n%s阁下请指定要加入的家族名称！", core_game.AtSender(event.UserID), utils.Emoji("E29D84")))
		return
	}

	var family models.Family
	err := database.DB.Where("name = ?", targetFamily).First(&family).Error
	if err == gorm.ErrRecordNotFound {
		core.SendGroupMessage(conn, event.GroupID, fmt.Sprintf("%s\n%s没有找到阁下说的[%s]家族哦！", core_game.AtSender(event.UserID), utils.Emoji("E29D84"), targetFamily))
		return
	}

	if family.CurrentSize >= family.MaxSize {
		core.SendGroupMessage(conn, event.GroupID, fmt.Sprintf("%s\n%s[%s]家族人数已达到上限，请换个家族加入吧！", core_game.AtSender(event.UserID), utils.Emoji("E29D84"), targetFamily))
		return
	}

	tx := database.DB.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	family.CurrentSize++
	if err := tx.Save(&family).Error; err != nil {
		tx.Rollback()
		return
	}

	pet.Family = targetFamily
	pet.FamilyScore = 0
	if err := tx.Save(pet).Error; err != nil {
		tx.Rollback()
		return
	}

	tx.Commit()

	core.SendGroupMessage(conn, event.GroupID, fmt.Sprintf("%s\n%s恭喜阁下加入了[%s]家族！", core_game.AtSender(event.UserID), utils.Emoji("E29C89"), targetFamily))
}

// 3. 注销家族
func disbandFamily(conn *websocket.Conn, event *core.OneBotEvent) {
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

	if pet.Family == "" {
		core.SendGroupMessage(conn, event.GroupID, core_game.AtSender(event.UserID)+"\n"+utils.Emoji("E29D84")+"阁下还没有家族！")
		return
	}

	var family models.Family
	err := database.DB.Where("name = ?", pet.Family).First(&family).Error
	if err != nil {
		return
	}

	if family.LeaderID != event.UserID {
		core.SendGroupMessage(conn, event.GroupID, core_game.AtSender(event.UserID)+"\n"+utils.Emoji("E29D84")+"阁下不是家族的族长，不能注销家族！")
		return
	}

	tx := database.DB.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// 清空全体成员的家族属性
	if err := tx.Model(&models.UserPet{}).Where("family = ?", family.Name).Updates(map[string]interface{}{
		"family":       "",
		"family_score": 0,
	}).Error; err != nil {
		tx.Rollback()
		return
	}

	// 删除家族
	if err := tx.Delete(&family).Error; err != nil {
		tx.Rollback()
		return
	}

	tx.Commit()

	core.SendGroupMessage(conn, event.GroupID, fmt.Sprintf("%s\n%s家族[%s]注销成功！", core_game.AtSender(event.UserID), utils.Emoji("E29C89"), family.Name))
}

// 4. 退出家族
func leaveFamily(conn *websocket.Conn, event *core.OneBotEvent) {
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

	if pet.Family == "" {
		core.SendGroupMessage(conn, event.GroupID, core_game.AtSender(event.UserID)+"\n"+utils.Emoji("E29D84")+"阁下还没有家族！")
		return
	}

	var family models.Family
	err := database.DB.Where("name = ?", pet.Family).First(&family).Error
	if err != nil {
		return
	}

	if family.LeaderID == event.UserID {
		core.SendGroupMessage(conn, event.GroupID, core_game.AtSender(event.UserID)+"\n"+utils.Emoji("E29D84")+"阁下是家族族长哦，不能退出家族！如果非要注销的话，请发送【注销家族】！")
		return
	}

	tx := database.DB.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	family.CurrentSize--
	if err := tx.Save(&family).Error; err != nil {
		tx.Rollback()
		return
	}

	pet.Family = ""
	pet.FamilyScore = 0
	if err := tx.Save(pet).Error; err != nil {
		tx.Rollback()
		return
	}

	tx.Commit()

	core.SendGroupMessage(conn, event.GroupID, fmt.Sprintf("%s\n%s阁下退出了[%s]家族！", core_game.AtSender(event.UserID), utils.Emoji("E29C89"), family.Name))
}

// 5. 我的家族信息
func myFamily(conn *websocket.Conn, event *core.OneBotEvent) {
	pet, ok := core_game.GetPetOrReply(conn, event)
	if !ok {
		return
	}

	chkMsg := core_game.CheckPlayerStatus(pet, "忽略检测")
	if chkMsg != "" {
		core.SendGroupMessage(conn, event.GroupID, chkMsg)
		return
	}

	if pet.Family == "" {
		core.SendGroupMessage(conn, event.GroupID, core_game.AtSender(event.UserID)+"\n"+utils.Emoji("E29D84")+"阁下还没有家族！")
		return
	}

	var family models.Family
	err := database.DB.Where("name = ?", pet.Family).First(&family).Error
	if err != nil {
		return
	}

	replyText := fmt.Sprintf("【%s】\n%s家族族长：%d\n%s家族人数：%d/%d\n%s神树养分：%d",
		family.Name,
		utils.Emoji("F09F91A4"), family.LeaderID,
		utils.Emoji("F09F91A5"), family.CurrentSize, family.MaxSize,
		utils.Emoji("F09F8CBD"), family.TreeNutrients)

	core.SendGroupMessage(conn, event.GroupID, core_game.AtSender(event.UserID)+"\n"+replyText)
}

// 6. 神树浇水
func waterTree(conn *websocket.Conn, event *core.OneBotEvent) {
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

	if pet.Family == "" {
		core.SendGroupMessage(conn, event.GroupID, core_game.AtSender(event.UserID)+"\n"+utils.Emoji("E29D84")+"阁下还没有家族！")
		return
	}

	// 浇水间隔检查
	now := time.Now()
	val, loaded := lastWaterTimeMap.Load(event.UserID)
	if loaded {
		lastTime := val.(time.Time)
		if now.Sub(lastTime) < 60*time.Minute {
			rem := int(60.0 - now.Sub(lastTime).Minutes())
			if rem <= 0 {
				rem = 1
			}
			core.SendGroupMessage(conn, event.GroupID, fmt.Sprintf("%s\n%s阁下距离下次给神树浇水还差%d分钟！",
				core_game.AtSender(event.UserID), utils.Emoji("E29D84"), rem))
			return
		}
	}

	var family models.Family
	err := database.DB.Where("name = ?", pet.Family).First(&family).Error
	if err != nil {
		return
	}

	lastWaterTimeMap.Store(event.UserID, now)

	tx := database.DB.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	family.TreeNutrients++

	// 结果成熟检查
	treeLimit := config.Interaction.TreeResultNutri
	if treeLimit <= 0 {
		treeLimit = 155
	}

	isMatured := family.TreeNutrients >= treeLimit
	var rewardDesc string

	if isMatured {
		family.TreeNutrients = 0

		// 获取本家族全部成员
		var members []models.UserPet
		if err := tx.Where("family = ?", family.Name).Find(&members).Error; err == nil {
			// 解析奖励
			rewardsStr := config.Interaction.TreeRewardItems
			if rewardsStr == "" {
				rewardsStr = "神树果实*1#抽奖券*10"
			}

			parts := strings.Split(rewardsStr, "#")
			var awardedList []string

			// 发放奖励
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
				}

				// 给所有成员背包都发一份
				for _, mem := range members {
					core_game.AddBackpackItem(tx, mem.UserID, event.GroupID, itemName, qty)
				}
				awardedList = append(awardedList, fmt.Sprintf("%s×%d", itemName, qty))
			}
			rewardDesc = strings.Join(awardedList, "、")
		}
	}

	if err := tx.Save(&family).Error; err != nil {
		tx.Rollback()
		return
	}

	tx.Commit()

	if isMatured {
		core.SendGroupMessage(conn, event.GroupID, fmt.Sprintf("%s\n%s阁下带着宠物来给神树浇水，神树获得了1点养分！\n叮叮：神树养分已成熟，家族每位成员获得奖励：%s",
			core_game.AtSender(event.UserID), utils.Emoji("F09F8CBD"), rewardDesc))
	} else {
		core.SendGroupMessage(conn, event.GroupID, fmt.Sprintf("%s\n%s阁下带着宠物来给神树浇水，神树获得了1点养分！",
			core_game.AtSender(event.UserID), utils.Emoji("F09F8CBD")))
	}
}

// 7. 家族列表
func listFamilies(conn *websocket.Conn, event *core.OneBotEvent) {
	prefix := config.GetCommand("家族列表")
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

	var families []models.Family
	err := database.DB.Order("id ASC").Find(&families).Error
	if err != nil {
		return
	}

	total := len(families)
	if total == 0 {
		core.SendGroupMessage(conn, event.GroupID, core_game.AtSender(event.UserID)+"\n"+utils.Emoji("E29D84")+"阁下，暂时没有人创建家族哦！")
		return
	}

	totalPages := (total + 4) / 5
	startIndex := (page - 1) * 5
	if startIndex >= total {
		core.SendGroupMessage(conn, event.GroupID, core_game.AtSender(event.UserID)+"\n"+utils.Emoji("E29D84")+"阁下，家族列表页数输入错误啦！")
		return
	}

	var listText strings.Builder
	for i := startIndex; i < startIndex+5 && i < total; i++ {
		fam := families[i]
		listText.WriteString(fmt.Sprintf("%s(%d/%d)\n", fam.Name, fam.CurrentSize, fam.MaxSize))
	}

	replyText := fmt.Sprintf("%s【家族列表】%s\n%s- - - - - - - - - - - - - - - - -\n当前页数：[%d/%d]\n%s翻页指令：%s 页数",
		utils.Emoji("F09F91A5"), utils.Emoji("F09F91A5"),
		listText.String(),
		page, totalPages,
		utils.Emoji("F09F92A1"), config.GetCommand("家族列表"))

	core.SendGroupMessage(conn, event.GroupID, core_game.AtSender(event.UserID)+"\n"+replyText)
}

// 8. 家族成员展示
func showFamilyMembers(conn *websocket.Conn, event *core.OneBotEvent) {
	pet, ok := core_game.GetPetOrReply(conn, event)
	if !ok {
		return
	}

	chkMsg := core_game.CheckPlayerStatus(pet, "忽略检测")
	if chkMsg != "" {
		core.SendGroupMessage(conn, event.GroupID, chkMsg)
		return
	}

	if pet.Family == "" {
		core.SendGroupMessage(conn, event.GroupID, core_game.AtSender(event.UserID)+"\n"+utils.Emoji("E29D84")+"阁下还没有家族！")
		return
	}

	var family models.Family
	err := database.DB.Where("name = ?", pet.Family).First(&family).Error
	if err != nil {
		return
	}

	var members []models.UserPet
	err = database.DB.Where("family = ?", pet.Family).Find(&members).Error
	if err != nil {
		return
	}

	var memberList []string
	for _, mem := range members {
		if mem.UserID == family.LeaderID {
			continue // 排除族长，单独展示
		}
		memberList = append(memberList, strconv.FormatInt(mem.UserID, 10))
	}

	memberDesc := "阁下的家族还没有其他的成员哦！"
	if len(memberList) > 0 {
		memberDesc = strings.Join(memberList, "\n")
	}

	replyText := fmt.Sprintf("【%s】\n%s家族族长：%d\n%s家族人数：%d/%d\n%s家族成员：\n%s",
		family.Name,
		utils.Emoji("F09F91A4"), family.LeaderID,
		utils.Emoji("F09F91A5"), family.CurrentSize, family.MaxSize,
		utils.Emoji("F09F939D"), memberDesc)

	core.SendGroupMessage(conn, event.GroupID, core_game.AtSender(event.UserID)+"\n"+replyText)
}

// 9. 踢出成员
func kickMember(conn *websocket.Conn, event *core.OneBotEvent) {
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

	if pet.Family == "" {
		core.SendGroupMessage(conn, event.GroupID, core_game.AtSender(event.UserID)+"\n"+utils.Emoji("E29D84")+"阁下还没有家族！")
		return
	}

	var family models.Family
	err := database.DB.Where("name = ?", pet.Family).First(&family).Error
	if err != nil {
		return
	}

	if family.LeaderID != event.UserID {
		core.SendGroupMessage(conn, event.GroupID, core_game.AtSender(event.UserID)+"\n"+utils.Emoji("E29D84")+"阁下不是家族的族长，不能踢出家族成员哦！")
		return
	}

	prefix := config.GetCommand("踢出成员")
	args := strings.TrimSpace(strings.TrimPrefix(event.RawMessage, prefix))
	targetQQ := parseAtQQ(args)
	if targetQQ == 0 {
		// 尝试直接解析纯数字
		if val, err := strconv.ParseInt(args, 10, 64); err == nil {
			targetQQ = val
		}
	}

	if targetQQ == 0 {
		core.SendGroupMessage(conn, event.GroupID, fmt.Sprintf("%s\n%s请指定要踢出的成员QQ或At对方！", core_game.AtSender(event.UserID), utils.Emoji("E29D84")))
		return
	}

	var targetPet models.UserPet
	err = database.DB.Where("user_id = ? AND group_id = ?", targetQQ, event.GroupID).First(&targetPet).Error
	if err == gorm.ErrRecordNotFound || targetPet.Family != family.Name {
		core.SendGroupMessage(conn, event.GroupID, core_game.AtSender(event.UserID)+"\n"+utils.Emoji("E29D84")+"阁下，这位兄弟不在阁下的家族哦！")
		return
	}

	tx := database.DB.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	family.CurrentSize--
	if err := tx.Save(&family).Error; err != nil {
		tx.Rollback()
		return
	}

	targetPet.Family = ""
	targetPet.FamilyScore = 0
	if err := tx.Save(&targetPet).Error; err != nil {
		tx.Rollback()
		return
	}

	tx.Commit()

	core.SendGroupMessage(conn, event.GroupID, fmt.Sprintf("%s\n%s阁下被族长踢出了家族[%s]。",
		core_game.AtSender(targetQQ), utils.Emoji("E29B85"), family.Name))
}
