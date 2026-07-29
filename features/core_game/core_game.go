package core_game

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"gorm.io/gorm"
	"qq-pet-saas/config"
	"qq-pet-saas/core"
	"qq-pet-saas/database"
	"qq-pet-saas/models"
	"qq-pet-saas/utils"
)

// releaseConfirmMap 用于追踪玩家“确认放生”的二级确认状态
var releaseConfirmMap sync.Map

func init() {
	// 动态注册所有核心玩法指令处理器
	core.RegisterHandler(config.GetCommand("领养宠物"), adoptPetList)
	core.RegisterHandler(config.GetCommand("领养"), adoptPet)
	core.RegisterHandler(config.GetCommand("我的宠物"), showPetStatus)
	core.RegisterHandler(config.GetCommand("查看宠物"), viewPetConfig)
	core.RegisterHandler(config.GetCommand("改名"), renamePet)
	core.RegisterHandler(config.GetCommand("签到"), checkin)
	core.RegisterHandler(config.GetCommand("我的背包"), showBackpack)
	core.RegisterHandler(config.GetCommand("放生"), releasePet)
	core.RegisterHandler(config.GetCommand("确认放生"), confirmRelease)
	core.RegisterHandler(config.GetCommand("找回"), findBackPet)
	core.RegisterHandler(config.GetCommand("治疗"), treatPet)
}

// CheckAndApplyPetRules 检查宠物状态，执行濒死和逃跑超时逻辑的惰性应用
func CheckAndApplyPetRules(pet *models.UserPet) {
	now := time.Now()
	// 1. 检查濒死状态
	if pet.Status == "濒死" {
		if pet.DyingTime != nil {
			limit := time.Duration(config.Core.DyingSaveTime) * time.Minute
			if now.Sub(*pet.DyingTime) > limit {
				pet.Status = "失去宠物"
				lostTime := pet.DyingTime.Add(limit)
				pet.LostTime = &lostTime
				database.DB.Save(pet)
			}
		}
	}
	// 2. 检查逃跑状态
	if pet.Status == "逃跑" {
		if pet.EscapeTime != nil {
			limit := time.Duration(config.Core.EscapeFindTime) * time.Minute
			if now.Sub(*pet.EscapeTime) > limit {
				pet.Status = "失去宠物"
				lostTime := pet.EscapeTime.Add(limit)
				pet.LostTime = &lostTime
				database.DB.Save(pet)
			}
		}
	}
}

// CheckPlayerStatus 封装 E 语言的 驱动_玩家检测 机制
func CheckPlayerStatus(pet *models.UserPet, cmdKey string) string {
	if pet.Status == "失去宠物" {
		return fmt.Sprintf("%s\n%s阁下已经失去宠物了，没有宠物能和阁下互动了。", AtSender(pet.UserID), utils.Emoji("E29D84"))
	}

	if pet.Status != "空闲" {
		if cmdKey == "忽略检测" {
			return ""
		}
		if pet.Status == "逃跑" && cmdKey == "找回" {
			return ""
		}
		if pet.Status == "濒死" && (cmdKey == "治疗" || cmdKey == "血量") {
			return ""
		}
		return fmt.Sprintf("%s\n%s阁下，宠物不在空闲中，他目前正在：%s，让它先把手中事做完吧！",
			AtSender(pet.UserID), utils.Emoji("E29D84"), pet.Status)
	}

	if pet.Health <= 0 {
		if cmdKey == "忽略检测" || cmdKey == "治疗" || cmdKey == "血量" {
			return ""
		}
		return fmt.Sprintf("%s\n%s阁下的宠物血量已经到底了，赶紧给它治疗下吧！", AtSender(pet.UserID), utils.Emoji("E29D84"))
	}

	return ""
}

// GetPetOrReply 获取宠物信息并执行自动冷却检测，若无宠物则直接回复提示
func GetPetOrReply(conn *websocket.Conn, event *core.OneBotEvent) (*models.UserPet, bool) {
	var pet models.UserPet
	err := database.DB.Where("user_id = ? AND group_id = ?", event.UserID, event.GroupID).First(&pet).Error
	if err == gorm.ErrRecordNotFound {
		core.SendGroupMessage(conn, event.GroupID, fmt.Sprintf("%s\n%s嘿！阁下还没有宠物呢，发送'领养宠物'来选一只心仪的伙伴吧！", AtSender(event.UserID), utils.Emoji("E29D84")))
		return nil, false
	} else if err != nil {
		core.SendGroupMessage(conn, event.GroupID, fmt.Sprintf("系统错误: %v", err))
		return nil, false
	}

	oldStatus := pet.Status
	CheckAndApplyPetRules(&pet)
	if pet.Status != oldStatus {
		database.DB.Save(&pet)
	}

	return &pet, true
}

// AtSender 获取艾特玩家的 CQ 码
func AtSender(userID int64) string {
	return fmt.Sprintf("[CQ:at,qq=%d]", userID)
}

// GetImageCQ 获取图片的一体化 CQ 码
func GetImageCQ(relPath string) string {
	if relPath == "" {
		return ""
	}
	// 如果配置了图片服务域名，直接返回 HTTP URL 形式的 CQ 码 (供远程/SaaS Bot使用)
	if config.Core.ImageHost != "" {
		urlPath := strings.ReplaceAll(relPath, "\\", "/")
		return fmt.Sprintf("\n[CQ:image,file=%s/images/%s]", config.Core.ImageHost, urlPath)
	}

	// 否则默认使用本地文件路径，并必须转换为绝对路径，以防止 Napcat 客户端从其自身的工作目录相对路径中寻找文件
	absPath := filepath.Join(config.GlobalConfigPath, "图片", relPath)
	if _, err := os.Stat(absPath); err == nil {
		if abs, err := filepath.Abs(absPath); err == nil {
			uriPath := filepath.ToSlash(abs)
			return fmt.Sprintf("\n[CQ:image,file=file:///%s]", uriPath)
		}
	}

	// 后备路径：如果配置路径不存在，尝试查找当前可执行文件目录下的 ./图片
	workPath := filepath.Join(".", "图片", relPath)
	if _, err := os.Stat(workPath); err == nil {
		if abs, err := filepath.Abs(workPath); err == nil {
			uriPath := filepath.ToSlash(abs)
			return fmt.Sprintf("\n[CQ:image,file=file:///%s]", uriPath)
		}
	}
	return ""
}

// AddBackpackItem 增加玩家背包内物品数量
func AddBackpackItem(tx *gorm.DB, userID, groupID int64, itemName string, qty int64) error {
	var item models.BackpackItem
	err := tx.Where("user_id = ? AND group_id = ? AND item_name = ?", userID, groupID, itemName).First(&item).Error
	if err == gorm.ErrRecordNotFound {
		item = models.BackpackItem{
			UserID:   userID,
			GroupID:  groupID,
			ItemName: itemName,
			Quantity: qty,
		}
		return tx.Create(&item).Error
	} else if err != nil {
		return err
	}
	item.Quantity += qty
	return tx.Save(&item).Error
}

// ------------------- 指令处理器 -------------------

// 1. 领养宠物列表
func adoptPetList(conn *websocket.Conn, event *core.OneBotEvent) {
	prefix := config.GetCommand("领养宠物")
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

	initialPets := config.Core.InitialPets
	totalPets := len(initialPets)
	if totalPets == 0 {
		core.SendGroupMessage(conn, event.GroupID, fmt.Sprintf("%s\n%s当前没有宠物可以领养哦，请联系管理员！", AtSender(event.UserID), utils.Emoji("E29D84")))
		return
	}

	totalPages := (totalPets + 4) / 5
	startIndex := (page - 1) * 5
	if startIndex >= totalPets {
		core.SendGroupMessage(conn, event.GroupID, fmt.Sprintf("%s\n%s阁下，页数输入错误啦！", AtSender(event.UserID), utils.Emoji("E29D84")))
		return
	}

	var listText strings.Builder
	for i := startIndex; i < startIndex+5 && i < totalPets; i++ {
		listText.WriteString(fmt.Sprintf("%s%s\n", utils.Emoji("F09F91BE"), initialPets[i]))
	}

	var footer strings.Builder
	footer.WriteString("- - - - - - - - - - - - - - - - -\n")
	footer.WriteString(fmt.Sprintf("当前页数：[%d/%d]\n", page, totalPages))

	replyText := fmt.Sprintf("%s%s\n请问您要哪个可爱的小家伙呢？\n%s%s%s发送\"领养+宠物名\"来领养一只宠物。",
		AtSender(event.UserID), GetImageCQ(config.Images["领养"]), listText.String(), footer.String(), utils.Emoji("F09F92A1"))

	core.SendGroupMessage(conn, event.GroupID, replyText)
}

// 2. 确认领养具体宠物
func adoptPet(conn *websocket.Conn, event *core.OneBotEvent) {
	// 使用并发锁
	lockKey := utils.GetUserLockKey(event.GroupID, event.UserID)
	mutex := utils.GetLock(lockKey)
	mutex.Lock()
	defer mutex.Unlock()

	prefix := config.GetCommand("领养")
	target := strings.TrimSpace(strings.TrimPrefix(event.RawMessage, prefix))
	if target == "" {
		core.SendGroupMessage(conn, event.GroupID, fmt.Sprintf("%s\n%s请输入要领养的宠物名字，例如：领养 诺诺", AtSender(event.UserID), utils.Emoji("E29D84")))
		return
	}

	// 验证宠物是否为初始宠物
	isInitial := false
	for _, p := range config.Core.InitialPets {
		if p == target {
			isInitial = true
			break
		}
	}
	if !isInitial {
		core.SendGroupMessage(conn, event.GroupID, fmt.Sprintf("%s\n%s没有找到阁下说的[%s]这个宠物呀！", AtSender(event.UserID), utils.Emoji("E29D84"), target))
		return
	}

	petSpecies, exists := config.Pets[target]
	if !exists {
		core.SendGroupMessage(conn, event.GroupID, fmt.Sprintf("%s\n%s没有找到该宠物的配置属性，请联系管理员！", AtSender(event.UserID), utils.Emoji("E29D84")))
		return
	}

	var pet models.UserPet
	err := database.DB.Where("user_id = ? AND group_id = ?", event.UserID, event.GroupID).First(&pet).Error

	var replyText string
	if err == nil {
		// 已有宠物，检查是否是“失去宠物”状态且已过冷却
		if pet.Status != "失去宠物" {
			replyText = fmt.Sprintf("%s\n%s阁下已经有宠物了哦！", AtSender(event.UserID), utils.Emoji("E29D84"))
			core.SendGroupMessage(conn, event.GroupID, replyText)
			return
		}

		// 检查冷却
		if pet.LostTime != nil {
			interval := time.Since(*pet.LostTime).Seconds()
			cooldown := float64(config.Core.LostCooldown * 60)
			if interval < cooldown {
				remaining := math.Round((cooldown - interval) / 60)
				if remaining <= 0 {
					remaining = 1
				}
				replyText = fmt.Sprintf("%s\n%s阁下请认真反思你对上个宠物的所作所为，还有%d分钟可以再次领养宠物！",
					AtSender(event.UserID), utils.Emoji("E29D84"), int(remaining))
				core.SendGroupMessage(conn, event.GroupID, replyText)
				return
			}
		}

		// 过了冷却期，重置宠物存档
		pet.PetType = target
		pet.Name = target
		pet.Image = petSpecies.Image
		pet.Status = "空闲"
		pet.Mood = "一般"
		pet.MoodPoints = 50
		pet.Affection = 0
		pet.Growth = 0
		pet.Health = petSpecies.Health
		pet.Wisdom = petSpecies.Wisdom
		pet.Strength = petSpecies.Strength
		pet.Defense = petSpecies.Defense
		pet.Hunger = petSpecies.Hunger
		if pet.Hunger <= 0 {
			pet.Hunger = 100
		}
		pet.NewbieCheck = 1
		pet.LastCheckin = nil
		pet.StudyTime = nil
		pet.StudyItem = ""
		pet.TrainTime = nil
		pet.TrainItem = ""
		pet.WorkTime = nil
		pet.WorkType = ""
		pet.FitnessTime = nil
		pet.FitnessItem = ""
		pet.DyingTime = nil
		pet.EscapeTime = nil
		pet.LostTime = nil
		pet.Currency += config.Core.InitialCoin

		database.DB.Save(&pet)
	} else if err == gorm.ErrRecordNotFound {
		// 创建全新宠物
		hunger := petSpecies.Hunger
		if hunger <= 0 {
			hunger = 100
		}
		pet = models.UserPet{
			UserID:      event.UserID,
			GroupID:     event.GroupID,
			PetType:     target,
			Name:        target,
			Image:       petSpecies.Image,
			Status:      "空闲",
			Mood:        "一般",
			MoodPoints:  50,
			Affection:   0,
			Growth:      0,
			Health:      petSpecies.Health,
			Wisdom:      petSpecies.Wisdom,
			Strength:    petSpecies.Strength,
			Defense:     petSpecies.Defense,
			Hunger:      hunger,
			NewbieCheck: 1,
			Currency:    config.Core.InitialCoin,
		}
		database.DB.Create(&pet)
	} else {
		core.SendGroupMessage(conn, event.GroupID, fmt.Sprintf("数据库错误: %v", err))
		return
	}

	renameCmd := config.GetCommand("改名")
	replyText = fmt.Sprintf("%s恭喜[%s]%s\n%s领养了：<%s>为宠物！\n%s它的一生就托付给阁下了哦，要好好照顾它呀！\n%s发送'%s 新名字'就可以给宠物改名字了哦！(首次免费)",
		utils.Emoji("F09F8E89"), AtSender(event.UserID), GetImageCQ(petSpecies.AdoptImage), utils.Emoji("F09F8DAD"), target, utils.Emoji("E280BC"), utils.Emoji("F09F939D"), renameCmd)

	core.SendGroupMessage(conn, event.GroupID, replyText)
}

// 3. 我的宠物状态查询
func showPetStatus(conn *websocket.Conn, event *core.OneBotEvent) {
	pet, ok := GetPetOrReply(conn, event)
	if !ok {
		return
	}

	chkMsg := CheckPlayerStatus(pet, "忽略检测")
	if chkMsg != "" {
		core.SendGroupMessage(conn, event.GroupID, chkMsg)
		return
	}

	petSpecies, _ := config.Pets[pet.PetType]

	replyText := fmt.Sprintf("%s%s\n【%s】\n%s种类：%s\n%s状态：%s\n%s心情：%s\n%s好感：%d\n%s成长：%d\n%s血量：%d\n%s智慧：%d\n%s力量：%d\n%s防御：%d\n%s饱食：%d",
		AtSender(event.UserID), GetImageCQ(petSpecies.Image), pet.Name,
		utils.Emoji("F09F90BE"), pet.PetType,
		utils.Emoji("E29B84"), pet.Status,
		utils.Emoji("F09F8E90"), pet.Mood,
		utils.Emoji("F09F9296"), pet.Affection,
		utils.Emoji("F09F8C9F"), pet.Growth,
		utils.Emoji("F09F929F"), pet.Health,
		utils.Emoji("E29B8E"), pet.Wisdom,
		utils.Emoji("F09F92AA"), pet.Strength,
		utils.Emoji("F09F94B0"), pet.Defense,
		utils.Emoji("F09F8D94"), pet.Hunger)

	core.SendGroupMessage(conn, event.GroupID, replyText)
}

// 4. 查看具体宠物配置详情
func viewPetConfig(conn *websocket.Conn, event *core.OneBotEvent) {
	prefix := config.GetCommand("查看宠物")
	target := strings.TrimSpace(strings.TrimPrefix(event.RawMessage, prefix))
	if target == "" {
		core.SendGroupMessage(conn, event.GroupID, fmt.Sprintf("%s\n%s阁下，如果想查看宠物的详细信息，请发送【查看宠物＋宠物名】即可哦！", AtSender(event.UserID), utils.Emoji("E29D84")))
		return
	}

	petSpecies, exists := config.Pets[target]
	if !exists {
		core.SendGroupMessage(conn, event.GroupID, fmt.Sprintf("%s\n%s没有找到阁下说的[%s]这个宠物呀！", AtSender(event.UserID), utils.Emoji("E29D84"), target))
		return
	}

	var evoStr string
	if petSpecies.EvolutionBranch == 0 {
		if petSpecies.Evolution != "" {
			evoStr = fmt.Sprintf("\n%s进化后：%s\n%s进化成长：%d",
				utils.Emoji("F09F94B1"), petSpecies.Evolution, utils.Emoji("F09F8C9F"), petSpecies.EvolutionGrowth)
		} else if petSpecies.Awaken != "" {
			awakenItems := strings.ReplaceAll(petSpecies.AwakenItems, "*", "×")
			evoStr = fmt.Sprintf("\n%s觉醒后：%s\n%s觉醒成长：%d\n%s觉醒所需：%s",
				utils.Emoji("F09F94A5"), petSpecies.Awaken, utils.Emoji("F09F8C9F"), petSpecies.AwakenGrowth, utils.Emoji("F09F928E"), awakenItems)
		}
	} else if petSpecies.EvolutionBranch == 1 {
		branches := strings.Split(petSpecies.Evolution, "#")
		var branchLines []string
		for _, b := range branches {
			b = strings.TrimSpace(b)
			if b == "" {
				continue
			}
			if strings.Contains(b, "*") {
				parts := strings.Split(b, "*")
				evoPetName := parts[0]
				evoReq := parts[1]
				branchLines = append(branchLines, fmt.Sprintf("[%s]\n%s进化所需道具：%s", evoPetName, utils.Emoji("F09F92BF"), evoReq))
			} else {
				branchLines = append(branchLines, fmt.Sprintf("[%s]\n%s进化所需道具：无", b, utils.Emoji("F09F9380")))
			}
		}
		evoStr = fmt.Sprintf("\n%s进化分支：\n%s", utils.Emoji("F09F94B1"), strings.Join(branchLines, "\n"))
	} else if petSpecies.EvolutionBranch == 2 {
		branches := strings.Split(petSpecies.Awaken, "#")
		var branchLines []string
		for _, b := range branches {
			b = strings.TrimSpace(b)
			if b == "" {
				continue
			}
			if strings.Contains(b, "*") {
				parts := strings.Split(b, "*")
				awakenPetName := parts[0]
				evoReq := parts[1]
				branchLines = append(branchLines, fmt.Sprintf("[%s]\n%s觉醒所需道具：%s", awakenPetName, utils.Emoji("F09F92BF"), evoReq))
			} else {
				branchLines = append(branchLines, fmt.Sprintf("[%s]\n%s觉醒所需道具：无", b, utils.Emoji("F09F9380")))
			}
		}
		evoStr = fmt.Sprintf("\n%s觉醒分支：\n%s", utils.Emoji("F09F94A5"), strings.Join(branchLines, "\n"))
	}

	replyText := fmt.Sprintf("%s%s\n【%s】\n%s喜欢食物：%s\n%s喜欢礼物：%s%s\n%s血量上限：%d\n%s智慧上限：%d\n%s力量上限：%d\n%s防御上限：%d\n%s饱食上限：%d\n%s描述：%s",
		AtSender(event.UserID), GetImageCQ(petSpecies.Image), petSpecies.Name,
		utils.Emoji("F09F8DB4"), petSpecies.FavoriteFood,
		utils.Emoji("F09F8E80"), petSpecies.FavoriteGift,
		evoStr,
		utils.Emoji("F09F929F"), petSpecies.HealthMax,
		utils.Emoji("E29B8E"), petSpecies.WisdomMax,
		utils.Emoji("F09F92AA"), petSpecies.StrengthMax,
		utils.Emoji("F09F94B0"), petSpecies.DefenseMax,
		utils.Emoji("F09F8D94"), petSpecies.HungerMax,
		utils.Emoji("F09F92AC"), petSpecies.Description)

	core.SendGroupMessage(conn, event.GroupID, replyText)
}

// 5. 宠物改名
func renamePet(conn *websocket.Conn, event *core.OneBotEvent) {
	lockKey := utils.GetUserLockKey(event.GroupID, event.UserID)
	mutex := utils.GetLock(lockKey)
	mutex.Lock()
	defer mutex.Unlock()

	pet, ok := GetPetOrReply(conn, event)
	if !ok {
		return
	}

	chkMsg := CheckPlayerStatus(pet, "")
	if chkMsg != "" {
		core.SendGroupMessage(conn, event.GroupID, chkMsg)
		return
	}

	prefix := config.GetCommand("改名")
	newName := strings.TrimSpace(strings.TrimPrefix(event.RawMessage, prefix))
	if newName == "" {
		core.SendGroupMessage(conn, event.GroupID, fmt.Sprintf("%s\n%s请输入想要修改的新名字！", AtSender(event.UserID), utils.Emoji("E29D84")))
		return
	}

	// 校验名字长度 2-6 个中文字符 (使用 Go 的 rune 切片判断)
	runes := []rune(newName)
	if len(runes) < 2 || len(runes) > 6 {
		core.SendGroupMessage(conn, event.GroupID, fmt.Sprintf("%s\n%s阁下注意啦，名字只能在2~6个字之间！", AtSender(event.UserID), utils.Emoji("E29D84")))
		return
	}

	// 敏感字屏蔽检测
	for _, blockWord := range config.Core.RenameBlocklist {
		if blockWord != "" && strings.Contains(newName, blockWord) {
			core.SendGroupMessage(conn, event.GroupID, fmt.Sprintf("%s\n%s阁下起的名字里有敏感字哦，请换个名字重试！", AtSender(event.UserID), utils.Emoji("E29D84")))
			return
		}
	}

	// 重名检测
	var count int64
	database.DB.Model(&models.UserPet{}).Where("group_id = ? AND name = ?", event.GroupID, newName).Count(&count)
	if count > 0 {
		core.SendGroupMessage(conn, event.GroupID, fmt.Sprintf("%s\n%s阁下给宠物起的名字别人已经占用了哦！", AtSender(event.UserID), utils.Emoji("E29D84")))
		return
	}

	// 改名消耗逻辑: 初始名字等于宠物种类名字时为首次改名 (免费)
	isFirstTime := pet.Name == pet.PetType
	cost := int64(0)
	if !isFirstTime {
		cost = config.Core.RenameCost
		if pet.Currency < cost {
			core.SendGroupMessage(conn, event.GroupID, fmt.Sprintf("%s\n%s阁下兜兜里的%s不够啦，改名需要：%d%s哦！",
				AtSender(event.UserID), utils.Emoji("E29D84"), config.Core.CoinName, cost, config.Core.CoinName))
			return
		}
		pet.Currency -= cost
	}

	pet.Name = newName
	database.DB.Save(pet)

	var feeDesc string
	if !isFirstTime {
		feeDesc = fmt.Sprintf("%s一共消费%d%s，愿宠物会喜欢这个新名字！", utils.Emoji("F09F92B0"), cost, config.Core.CoinName)
	} else {
		feeDesc = fmt.Sprintf("%s阁下第一次给宠物改名，就不收小费啦！", utils.Emoji("E29DA4"))
	}

	replyText := fmt.Sprintf("%s\n%s阁下给宠物改名为[%s]！\n%s", AtSender(event.UserID), utils.Emoji("F09F939D"), newName, feeDesc)
	core.SendGroupMessage(conn, event.GroupID, replyText)
}

// 6. 每日签到 (支持新手 1-7 天与循环周签到)
func checkin(conn *websocket.Conn, event *core.OneBotEvent) {
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

	now := time.Now()
	// 签到防刷：一天只能签到一次
	if pet.LastCheckin != nil && pet.LastCheckin.Format("2006-01-02") == now.Format("2006-01-02") {
		core.SendGroupMessage(conn, event.GroupID, fmt.Sprintf("%s\n%s阁下今天已经签到过了哦!", AtSender(event.UserID), utils.Emoji("E29D84")))
		return
	}

	tx := database.DB.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	var reward config.CheckinReward
	var isNewbie bool
	var finalReply string

	// 判断是新手签到还是周签到
	if pet.NewbieCheck > 0 && pet.NewbieCheck <= 7 {
		isNewbie = true
		dayStr := strconv.Itoa(pet.NewbieCheck)
		reward = config.NewbieCheckin[dayStr]
	} else {
		isNewbie = false
		weekday := int(now.Weekday())
		if weekday == 0 {
			weekday = 7 // Sunday is 7 in INI
		}
		reward = config.WeeklyCheckin[strconv.Itoa(weekday)]
	}

	// 给于奖励货币与好感
	pet.Currency += reward.Currency
	pet.Affection += reward.Affection
	pet.LastCheckin = &now

	// 奖励物品发放
	itemsText, err := parseAndAwardItems(tx, event.UserID, event.GroupID, reward.Items)
	if err != nil {
		tx.Rollback()
		core.SendGroupMessage(conn, event.GroupID, fmt.Sprintf("发放奖励物品失败: %v", err))
		return
	}

	var itemsDesc string
	if itemsText != "" {
		itemsDesc = fmt.Sprintf("%s奖励物品：%s", utils.Emoji("F09F8E81"), itemsText)
	}

	// 组合输出信息
	imgText := GetImageCQ(reward.Image)

	if isNewbie {
		newbieDay := pet.NewbieCheck
		pet.NewbieCheck++
		if pet.NewbieCheck > 7 {
			pet.NewbieCheck = 8 // 完成新手签到
		}

		var newbieQuotes string
		switch newbieDay {
		case 1:
			newbieQuotes = fmt.Sprintf("阁下好哦，照顾'%s'的第一天！相处的还融洽嘛~", pet.Name)
		case 2:
			newbieQuotes = fmt.Sprintf("阁下好哦，照顾'%s'的第二天！小宠乖不乖呀~", pet.Name)
		case 3:
			newbieQuotes = fmt.Sprintf("阁下好哦，照顾'%s'的第三天！今天有按时吃饭嘛", pet.Name)
		case 4:
			newbieQuotes = fmt.Sprintf("阁下好哦，照顾'%s'的第四天！多一点关心~多点爱哟！", pet.Name)
		case 5:
			newbieQuotes = fmt.Sprintf("阁下好哦，照顾'%s'的第五天！应该渐入佳境了吧！", pet.Name)
		case 6:
			newbieQuotes = fmt.Sprintf("阁下好哦，照顾'%s'的第六天！阁下应该掌握节奏了吧！", pet.Name)
		default:
			newbieQuotes = fmt.Sprintf("阁下好哦，照顾'%s'的第七天！恭喜阁下脱离新手期！", pet.Name)
		}

		finalReply = fmt.Sprintf("%s\n%s%s\n%s快收下阁下第%d天签到的奖励吧：\n%s奖励%s：%d\n%s增加好感：%d\n%s\n阁下明天也要记得来看宠物哦！",
			AtSender(event.UserID), newbieQuotes, imgText,
			utils.Emoji("F09F9592"), newbieDay,
			utils.Emoji("F09F92B0"), config.Core.CoinName, reward.Currency,
			utils.Emoji("F09F9296"), reward.Affection,
			itemsDesc)
	} else {
		weekdayNum := int(now.Weekday())
		if weekdayNum == 0 {
			weekdayNum = 7
		}
		finalReply = fmt.Sprintf("%s\n阁下好呀~来看阁下的'%s'了呀！%s\n%s以下是周%d的奖励哦！\n%s奖励%s：%d\n%s增加好感：%d\n%s\n阁下明天也要记得来看宠物哦！",
			AtSender(event.UserID), pet.Name, imgText,
			utils.Emoji("F09F9592"), weekdayNum,
			utils.Emoji("F09F92B0"), config.Core.CoinName, reward.Currency,
			utils.Emoji("F09F9296"), reward.Affection,
			itemsDesc)
	}

	if err := tx.Save(pet).Error; err != nil {
		tx.Rollback()
		core.SendGroupMessage(conn, event.GroupID, fmt.Sprintf("保存玩家数据失败: %v", err))
		return
	}

	tx.Commit()
	core.SendGroupMessage(conn, event.GroupID, finalReply)
}

func parseAndAwardItems(tx *gorm.DB, userID, groupID int64, itemsStr string) (string, error) {
	if itemsStr == "" {
		return "", nil
	}
	parts := strings.Split(itemsStr, "#")
	var descParts []string
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		var name string
		qty := int64(1)
		if strings.Contains(part, "*") {
			subParts := strings.Split(part, "*")
			name = strings.TrimSpace(subParts[0])
			if q, err := strconv.ParseInt(strings.TrimSpace(subParts[1]), 10, 64); err == nil {
				qty = q
			}
		} else if strings.Contains(part, "×") {
			subParts := strings.Split(part, "×")
			name = strings.TrimSpace(subParts[0])
			if q, err := strconv.ParseInt(strings.TrimSpace(subParts[1]), 10, 64); err == nil {
				qty = q
			}
		} else {
			name = part
		}

		if name == "" {
			continue
		}

		err := AddBackpackItem(tx, userID, groupID, name, qty)
		if err != nil {
			return "", err
		}
		descParts = append(descParts, fmt.Sprintf("%s×%d", name, qty))
	}
	if len(descParts) == 0 {
		return "", nil
	}
	return strings.Join(descParts, "、"), nil
}

// 7. 我的背包列表查询 (带分页逻辑)
func showBackpack(conn *websocket.Conn, event *core.OneBotEvent) {
	pet, ok := GetPetOrReply(conn, event)
	if !ok {
		return
	}

	chkMsg := CheckPlayerStatus(pet, "忽略检测")
	if chkMsg != "" {
		core.SendGroupMessage(conn, event.GroupID, chkMsg)
		return
	}

	prefix := config.GetCommand("我的背包")
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

	var items []models.BackpackItem
	err := database.DB.Where("user_id = ? AND group_id = ? AND quantity > 0", event.UserID, event.GroupID).Find(&items).Error
	if err != nil {
		core.SendGroupMessage(conn, event.GroupID, fmt.Sprintf("获取背包失败: %v", err))
		return
	}

	totalItems := len(items)
	if totalItems == 0 {
		core.SendGroupMessage(conn, event.GroupID, fmt.Sprintf("%s\n%s%s：%d\n%s阁下的背包里还没有任何物品呐。",
			AtSender(event.UserID), utils.Emoji("F09F92B0"), config.Core.CoinName, pet.Currency, utils.Emoji("E29D84")))
		return
	}

	totalPages := (totalItems + 4) / 5
	startIndex := (page - 1) * 5
	if startIndex >= totalItems {
		core.SendGroupMessage(conn, event.GroupID, fmt.Sprintf("%s\n%s阁下，背包页数输入错误了哦。", AtSender(event.UserID), utils.Emoji("E29D84")))
		return
	}

	var listText strings.Builder
	for i := startIndex; i < startIndex+5 && i < totalItems; i++ {
		listText.WriteString(fmt.Sprintf("\n%s%s×%d", utils.Emoji("F09F93A6"), items[i].ItemName, items[i].Quantity))
	}

	replyText := fmt.Sprintf("%s\n%s%s：%d\n【我的背包】%s\n当前页数：%d/%d\n%s翻页指令：我的背包 页数",
		AtSender(event.UserID), utils.Emoji("F09F92B0"), config.Core.CoinName, pet.Currency, listText.String(), page, totalPages, utils.Emoji("F09F92A1"))

	core.SendGroupMessage(conn, event.GroupID, replyText)
}

// 8. 放生宠物提示
func releasePet(conn *websocket.Conn, event *core.OneBotEvent) {
	pet, ok := GetPetOrReply(conn, event)
	if !ok {
		return
	}

	chkMsg := CheckPlayerStatus(pet, "忽略检测")
	if chkMsg != "" {
		core.SendGroupMessage(conn, event.GroupID, chkMsg)
		return
	}

	releaseConfirmMap.Store(event.UserID, true)

	replyText := fmt.Sprintf("%s\n%s阁下确定要放弃这个可怜的小家伙，并清空阁下的个人数据吗？\n%s如果阁下是家族族长将会注销家族。\n%s如果阁下想好了的话，请发送【确认放生】",
		AtSender(event.UserID), utils.Emoji("F09F9294"), utils.Emoji("E29AA0"), utils.Emoji("F09F92AD"))

	core.SendGroupMessage(conn, event.GroupID, replyText)
}

// 9. 确认放生宠物
func confirmRelease(conn *websocket.Conn, event *core.OneBotEvent) {
	_, confirmed := releaseConfirmMap.Load(event.UserID)
	if !confirmed {
		return // 若没有发送“放生”进行二次保护，则直接忽略该指令
	}
	releaseConfirmMap.Delete(event.UserID)

	pet, ok := GetPetOrReply(conn, event)
	if !ok {
		return
	}

	tx := database.DB.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// 若是家族族长则注销家族，否则若是家族成员则减少人数
	if pet.Family != "" {
		var family models.Family
		err := tx.Where("name = ?", pet.Family).First(&family).Error
		if err == nil {
			if family.LeaderID == pet.UserID {
				// 族长注销家族
				tx.Where("name = ?", pet.Family).Delete(&models.Family{})
				// 解散所有成员家族属性
				tx.Model(&models.UserPet{}).Where("family = ?", pet.Family).Updates(map[string]interface{}{
					"family":       "",
					"family_score": 0,
				})
			} else {
				// 普通成员离开家族
				family.CurrentSize--
				if family.CurrentSize < 1 {
					family.CurrentSize = 1
				}
				tx.Save(&family)
			}
		}
	}

	// 清空玩家的所有宠物数据与背包数据
	if err := tx.Where("user_id = ? AND group_id = ?", event.UserID, event.GroupID).Delete(&models.UserPet{}).Error; err != nil {
		tx.Rollback()
		core.SendGroupMessage(conn, event.GroupID, "清空宠物档案失败，请重试！")
		return
	}
	if err := tx.Where("user_id = ? AND group_id = ?", event.UserID, event.GroupID).Delete(&models.BackpackItem{}).Error; err != nil {
		tx.Rollback()
		core.SendGroupMessage(conn, event.GroupID, "清空背包物品失败，请重试！")
		return
	}

	tx.Commit()
	core.SendGroupMessage(conn, event.GroupID, fmt.Sprintf("%s\n%s阁下的数据已被清空。", AtSender(event.UserID), utils.Emoji("E29D84")))
}

// 10. 宠物找回 (从逃跑状态中找回)
func findBackPet(conn *websocket.Conn, event *core.OneBotEvent) {
	pet, ok := GetPetOrReply(conn, event)
	if !ok {
		return
	}

	if pet.Status != "逃跑" {
		if pet.Status == "失去宠物" {
			core.SendGroupMessage(conn, event.GroupID, fmt.Sprintf("%s\n%s阁下的宠物已经跑的无影无踪了，你真不是个好主人啊！", AtSender(event.UserID), utils.Emoji("E29D84")))
		} else {
			core.SendGroupMessage(conn, event.GroupID, fmt.Sprintf("%s\n%s阁下的宠物没有逃跑呀！", AtSender(event.UserID), utils.Emoji("E29D84")))
		}
		return
	}

	pet.Affection -= 50
	if pet.Affection < 0 {
		pet.Affection = 0
	}
	pet.Mood = "非常难过"
	pet.MoodPoints = 20
	pet.Status = "空闲"
	pet.EscapeTime = nil
	pet.DyingTime = nil

	database.DB.Save(pet)

	core.SendGroupMessage(conn, event.GroupID, fmt.Sprintf("%s\n%s阁下找了好久终于在一个角落里找到了逃跑的'%s'并把它带回了家。宠物好感－50",
		AtSender(event.UserID), utils.Emoji("F09F9294"), pet.Name))
}

// 11. 宠物治疗 (脱离濒死状态)
func treatPet(conn *websocket.Conn, event *core.OneBotEvent) {
	lockKey := utils.GetUserLockKey(event.GroupID, event.UserID)
	mutex := utils.GetLock(lockKey)
	mutex.Lock()
	defer mutex.Unlock()

	pet, ok := GetPetOrReply(conn, event)
	if !ok {
		return
	}

	chkMsg := CheckPlayerStatus(pet, "治疗")
	if chkMsg != "" {
		core.SendGroupMessage(conn, event.GroupID, chkMsg)
		return
	}

	petSpecies, exists := config.Pets[pet.PetType]
	if !exists {
		core.SendGroupMessage(conn, event.GroupID, "找不到您的宠物属性配置，请联系管理员！")
		return
	}

	// 检查治疗费用
	if pet.Currency < config.Core.TreatCost {
		core.SendGroupMessage(conn, event.GroupID, fmt.Sprintf("%s\n%s阁下身上的钱不够了哦，医院也要恰饭的嘛，请阁下攒够钱再来吧！",
			AtSender(event.UserID), utils.Emoji("E29D84")))
		return
	}

	pet.Currency -= config.Core.TreatCost
	pet.Health = petSpecies.HealthMax
	pet.Hunger = petSpecies.HungerMax
	pet.Status = "空闲"
	pet.DyingTime = nil
	pet.EscapeTime = nil
	utils.SetLastTreatTime(event.UserID, time.Now())

	pet.Affection -= 10
	if pet.Affection < 0 {
		pet.Affection = 0
	}

	database.DB.Save(pet)

	replyText := fmt.Sprintf("%s%s\n%s阁下的宠物在医生的帮助下已经恢复正常啦，一共支付了%d%s！宠物好感－10",
		AtSender(event.UserID), GetImageCQ(config.Images["治疗"]), utils.Emoji("F09F9296"), config.Core.TreatCost, config.Core.CoinName)

	core.SendGroupMessage(conn, event.GroupID, replyText)
}
