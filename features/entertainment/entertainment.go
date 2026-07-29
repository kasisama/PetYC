package entertainment

import (
	"fmt"
	"math"
	"math/rand"
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

type DailyStats struct {
	Day            string
	WashDone       bool
	GiftCount      int64
	TouchGrowth    int64
	TouchAffection int64
	TouchTime      time.Time
	RpsTime        time.Time
	RpsAffection   int64
	WalkTime       time.Time
	WalkGrowth     int64
	WalkAffection  int64
	WorkCount      int64
	StudyCount     int64
	TrainCount     int64
	FitnessCount   int64
	FishTime       time.Time
	FishMinSec     int
	FishMaxSec     int
}

var (
	dailyStatsMu sync.Mutex
	dailyStats   = make(map[int64]*DailyStats)
)

func getDailyStats(userID int64) *DailyStats {
	dailyStatsMu.Lock()
	defer dailyStatsMu.Unlock()

	today := time.Now().Format("2006-01-02")
	stats, exists := dailyStats[userID]
	if !exists || stats.Day != today {
		stats = &DailyStats{
			Day: today,
		}
		dailyStats[userID] = stats
	}
	return stats
}

func init() {
	// 注册指令处理器
	core.RegisterHandler(config.GetCommand("洗澡"), washPet)
	core.RegisterHandler(config.GetCommand("摸头"), touchPet)
	core.RegisterHandler(config.GetCommand("抛竿"), castRod)
	core.RegisterHandler(config.GetCommand("收竿"), pullRod)
	core.RegisterHandler(config.GetCommand("学习"), studyPet)
	core.RegisterHandler(config.GetCommand("完成学习"), finishStudy)
	core.RegisterHandler(config.GetCommand("锻炼"), trainPet)
	core.RegisterHandler(config.GetCommand("完成锻炼"), finishTrain)
	core.RegisterHandler(config.GetCommand("健身"), fitnessPet)
	core.RegisterHandler(config.GetCommand("完成健身"), finishFitness)
	core.RegisterHandler(config.GetCommand("打工"), workPet)
	core.RegisterHandler(config.GetCommand("完成打工"), finishWork)
	core.RegisterHandler(config.GetCommand("送礼"), giftPet)
	core.RegisterHandler(config.GetCommand("猜拳"), rpsPet)
	core.RegisterHandler(config.GetCommand("喂养"), feedPet)
	core.RegisterHandler(config.GetCommand("散步"), walkPet)
}

// ---------------- Helpers ----------------

func getMoodMultiplier(mood string) float64 {
	switch mood {
	case "非常开心":
		return 1.5
	case "开心":
		return 1.25
	case "一般":
		return 1.0
	case "难过":
		return 0.75
	case "非常难过":
		return 0.5
	default:
		return 1.0
	}
}

func updateMood(pet *models.UserPet, delta int) {
	pet.MoodPoints += delta
	if pet.MoodPoints > 100 {
		pet.MoodPoints = 100
	}
	if pet.MoodPoints < 0 {
		pet.MoodPoints = 0
	}

	if pet.MoodPoints > 80 {
		pet.Mood = "非常开心"
	} else if pet.MoodPoints > 60 {
		pet.Mood = "开心"
	} else if pet.MoodPoints > 40 {
		pet.Mood = "一般"
	} else if pet.MoodPoints > 20 {
		pet.Mood = "难过"
	} else {
		pet.Mood = "非常难过"
	}
}

func applyGrowthBonus(val int64, species *config.PetSpecies) (int64, string) {
	if species.GrowthBonus != 0 {
		bonus := int64(math.Round(float64(val) * float64(species.GrowthBonus) / 100.0))
		if bonus <= 0 {
			bonus = 1
		}
		return val + bonus, "成长" + utils.Emoji("E2AC86EFB88F")
	}
	return val, ""
}

func applyAffectionBonus(val int64, species *config.PetSpecies) (int64, string) {
	if species.AffectionBonus != 0 {
		bonus := int64(math.Round(float64(val) * float64(species.AffectionBonus) / 100.0))
		if bonus <= 0 {
			bonus = 1
		}
		return val + bonus, "好感" + utils.Emoji("E2AC86EFB88F")
	}
	return val, ""
}

func applyAttributeBonus(val int64, species *config.PetSpecies) (int64, string) {
	if species.AttributeBonus != 0 {
		bonus := int64(math.Round(float64(val) * float64(species.AttributeBonus) / 100.0))
		if bonus <= 0 {
			bonus = 1
		}
		return val + bonus, "属性" + utils.Emoji("E2AC86EFB88F")
	}
	return val, ""
}

func applyCurrencyBonus(val int64, species *config.PetSpecies) (int64, string) {
	if species.CurrencyBonus != 0 {
		bonus := int64(math.Round(float64(val) * float64(species.CurrencyBonus) / 100.0))
		if bonus <= 0 {
			bonus = 1
		}
		return val + bonus, "货币" + utils.Emoji("E2AC86EFB88F")
	}
	return val, ""
}

// ----------------洗澡 (Wash) ----------------
func washPet(conn *websocket.Conn, event *core.OneBotEvent) {
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

	stats := getDailyStats(event.UserID)
	if stats.WashDone {
		core.SendGroupMessage(conn, event.GroupID, core_game.AtSender(event.UserID)+"\n"+utils.Emoji("E29D84")+"阁下今天给宠物洗过澡了哦！")
		return
	}

	stats.WashDone = true
	imageCQ := core_game.GetImageCQ(config.Images["洗澡"])

	// 饱食消耗
	washCost := config.Interaction.WashHungerCost
	if pet.Hunger < washCost {
		core.SendGroupMessage(conn, event.GroupID, core_game.AtSender(event.UserID)+"\n"+utils.Emoji("E29D84")+"宠物太饿了，洗不动澡啦！")
		return
	}
	pet.Hunger -= washCost

	moodMultiplier := getMoodMultiplier(pet.Mood)
	growth := int64(math.Round(float64(config.Interaction.WashGrowth) * moodMultiplier))
	affection := int64(math.Round(float64(config.Interaction.WashAffection) * moodMultiplier))

	species := config.Pets[pet.PetType]
	growth, bonusG := applyGrowthBonus(growth, &species)
	affection, bonusA := applyAffectionBonus(affection, &species)

	var bonusText string
	var bonusList []string
	if bonusG != "" {
		bonusList = append(bonusList, bonusG)
	}
	if bonusA != "" {
		bonusList = append(bonusList, bonusA)
	}
	if len(bonusList) > 0 {
		bonusText = "\n获得加成：" + strings.Join(bonusList, "、")
	}

	pet.Growth += growth
	pet.Affection += affection

	// 洗澡提升心情
	updateMood(pet, 5)

	database.DB.Save(pet)

	replyText := fmt.Sprintf("%s给'%s'洗澡澡，它感觉好舒服呀！\n%s洗澡消耗了%d点饱食，增加了%d好感和%d成长！%s",
		utils.Emoji("F09F929A"), pet.Name,
		utils.Emoji("E29CA8"), washCost, affection, growth, bonusText)

	core.SendGroupMessage(conn, event.GroupID, core_game.AtSender(event.UserID)+imageCQ+"\n"+replyText)
}

// ----------------摸头 (Touch) ----------------
func touchPet(conn *websocket.Conn, event *core.OneBotEvent) {
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

	stats := getDailyStats(event.UserID)

	// 摸头间隔
	touchInterval := config.Interaction.TouchInterval
	if !stats.TouchTime.IsZero() && time.Since(stats.TouchTime) < time.Duration(touchInterval)*time.Second {
		core.SendGroupMessage(conn, event.GroupID, core_game.AtSender(event.UserID)+"\n"+utils.Emoji("E29D84")+"阁下等会再摸头吧，宠物的头都要秃啦！")
		return
	}

	touchCost := config.Interaction.TouchHungerCost
	if pet.Hunger < touchCost {
		core.SendGroupMessage(conn, event.GroupID, core_game.AtSender(event.UserID)+"\n"+utils.Emoji("E29D84")+"宠物饿得直哼哼，不愿意让你摸头。")
		return
	}

	stats.TouchTime = time.Now()
	pet.Hunger -= touchCost

	moodMultiplier := getMoodMultiplier(pet.Mood)
	growth := int64(math.Round(float64(config.Interaction.TouchGrowth) * moodMultiplier))
	affection := int64(math.Round(float64(config.Interaction.TouchAffection) * moodMultiplier))

	// 上限检查
	var addG, addA int64
	if stats.TouchGrowth < config.Interaction.TouchGrowthLimit {
		addG = growth
		if stats.TouchGrowth+addG > config.Interaction.TouchGrowthLimit {
			addG = config.Interaction.TouchGrowthLimit - stats.TouchGrowth
		}
		stats.TouchGrowth += addG
	}
	if stats.TouchAffection < config.Interaction.TouchAffectLimit {
		addA = affection
		if stats.TouchAffection+addA > config.Interaction.TouchAffectLimit {
			addA = config.Interaction.TouchAffectLimit - stats.TouchAffection
		}
		stats.TouchAffection += addA
	}

	species := config.Pets[pet.PetType]
	var bonusList []string
	if addG > 0 {
		addG, bonusG := applyGrowthBonus(addG, &species)
		pet.Growth += addG
		if bonusG != "" {
			bonusList = append(bonusList, bonusG)
		}
	}
	if addA > 0 {
		addA, bonusA := applyAffectionBonus(addA, &species)
		pet.Affection += addA
		if bonusA != "" {
			bonusList = append(bonusList, bonusA)
		}
	}

	var bonusText string
	if len(bonusList) > 0 {
		bonusText = "\n获得加成：" + strings.Join(bonusList, "、")
	}

	updateMood(pet, 4)
	database.DB.Save(pet)

	imageCQ := core_game.GetImageCQ(config.Images["摸头"])

	replyText := fmt.Sprintf("%s阁下摸了摸'%s'的头，它感到很舒服安心。\n好感＋%d、成长＋%d%s",
		utils.Emoji("F09F929A"), pet.Name,
		addA, addG, bonusText)

	core.SendGroupMessage(conn, event.GroupID, core_game.AtSender(event.UserID)+imageCQ+"\n"+replyText)
}

// ----------------学习 (Study) ----------------
func studyPet(conn *websocket.Conn, event *core.OneBotEvent) {
	lockKey := utils.GetUserLockKey(event.GroupID, event.UserID)
	mutex := utils.GetLock(lockKey)
	mutex.Lock()
	defer mutex.Unlock()

	pet, ok := core_game.GetPetOrReply(conn, event)
	if !ok {
		return
	}

	chkMsg := core_game.CheckPlayerStatus(pet, "学习")
	if chkMsg != "" {
		core.SendGroupMessage(conn, event.GroupID, chkMsg)
		return
	}

	prefix := config.GetCommand("学习")
	itemName := strings.TrimSpace(strings.TrimPrefix(event.RawMessage, prefix))
	if itemName == "" {
		core.SendGroupMessage(conn, event.GroupID, fmt.Sprintf("%s\n%s阁下，想让宠物学习的话，需要一个智慧类型的物品，然后再发送【%s 物品名】哦！", core_game.AtSender(event.UserID), utils.Emoji("E29D84"), prefix))
		return
	}

	item, exists := config.Items[itemName]
	if !exists || item.Type != "智慧" {
		core.SendGroupMessage(conn, event.GroupID, core_game.AtSender(event.UserID)+"\n"+utils.Emoji("E29D84")+"阁下这件物品不能拿来学习哦！")
		return
	}

	// 背包校验
	var backpackItem models.BackpackItem
	err := database.DB.Where("user_id = ? AND group_id = ? AND item_name = ?", event.UserID, event.GroupID, itemName).First(&backpackItem).Error
	if err == gorm.ErrRecordNotFound || backpackItem.Quantity <= 0 {
		core.SendGroupMessage(conn, event.GroupID, core_game.AtSender(event.UserID)+"\n"+utils.Emoji("E29D84")+"阁下背包里没有这个物品哟！")
		return
	}

	// 每日次数限制
	stats := getDailyStats(event.UserID)
	studyLimit := config.Interaction.StudyLimit
	if studyLimit <= 0 {
		studyLimit = 5
	}
	if stats.StudyCount >= studyLimit {
		core.SendGroupMessage(conn, event.GroupID, core_game.AtSender(event.UserID)+"\n"+utils.Emoji("E29D84")+"阁下今天给宠物学习的次数用光了哦，让它放松放松吧！")
		return
	}

	// 智慧上限校验
	species := config.Pets[pet.PetType]
	if pet.Wisdom >= species.WisdomMax {
		core.SendGroupMessage(conn, event.GroupID, fmt.Sprintf("%s\n%s阁下的'%s'的智慧已经达到上限了哦，已经不需要学习啦！", core_game.AtSender(event.UserID), utils.Emoji("E29D84"), pet.Name))
		return
	}

	// 饱食度校验
	cost := config.Interaction.StudyHungerCost
	if pet.Hunger-cost <= 10 {
		core.SendGroupMessage(conn, event.GroupID, core_game.AtSender(event.UserID)+"\n"+utils.Emoji("E29D84")+"阁下，宠物的肚子已经饥肠辘辘了，先给它填饱肚子再让它学习吧！")
		return
	}

	tx := database.DB.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// 扣物品
	if err := core_game.AddBackpackItem(tx, event.UserID, event.GroupID, itemName, -1); err != nil {
		tx.Rollback()
		return
	}

	now := time.Now()
	pet.StudyTime = &now
	pet.StudyItem = itemName
	pet.Status = "学习"
	pet.Hunger -= cost

	if err := tx.Save(pet).Error; err != nil {
		tx.Rollback()
		return
	}

	tx.Commit()
	stats.StudyCount++

	img := species.StudyStartImg
	if img == "" {
		img = config.Images["开始学习"]
	}
	imageCQ := core_game.GetImageCQ(img)

	replyText := fmt.Sprintf("%s阁下让'%s'学习[%s]，消耗了%d饱食，预计学习时间要%d分钟，学习完成后记得发送【完成学习】哦！",
		utils.Emoji("F09F939A"), pet.Name, itemName, cost, item.Time)

	core.SendGroupMessage(conn, event.GroupID, core_game.AtSender(event.UserID)+imageCQ+"\n"+replyText)
}

func finishStudy(conn *websocket.Conn, event *core.OneBotEvent) {
	lockKey := utils.GetUserLockKey(event.GroupID, event.UserID)
	mutex := utils.GetLock(lockKey)
	mutex.Lock()
	defer mutex.Unlock()

	pet, ok := core_game.GetPetOrReply(conn, event)
	if !ok {
		return
	}

	if pet.Status != "学习" || pet.StudyItem == "" || pet.StudyTime == nil {
		core.SendGroupMessage(conn, event.GroupID, core_game.AtSender(event.UserID)+"\n"+utils.Emoji("E29D84")+"阁下的宠物没有在学习哦！")
		return
	}

	item, exists := config.Items[pet.StudyItem]
	if !exists {
		core.SendGroupMessage(conn, event.GroupID, core_game.AtSender(event.UserID)+"\n"+utils.Emoji("E29D84")+"没有找到所学习物品的配置！")
		return
	}

	elapsed := time.Since(*pet.StudyTime)
	required := time.Duration(item.Time) * time.Minute
	if elapsed < required {
		rem := int(math.Ceil((required - elapsed).Minutes()))
		if rem <= 0 {
			rem = 1
		}
		core.SendGroupMessage(conn, event.GroupID, fmt.Sprintf("%s\n%s阁下的宠物还没有学习完成哦，大约还有%d分钟学习完毕！", core_game.AtSender(event.UserID), utils.Emoji("E29D84"), rem))
		return
	}

	moodMultiplier := getMoodMultiplier(pet.Mood)
	effectVal, _ := strconv.ParseInt(item.Effect, 10, 64)
	addWisdom := int64(math.Round(float64(effectVal) * moodMultiplier))
	if addWisdom <= 0 {
		addWisdom = 1
	}

	species := config.Pets[pet.PetType]
	addWisdom, bonusW := applyAttributeBonus(addWisdom, &species)

	pet.Wisdom += addWisdom
	if pet.Wisdom > species.WisdomMax {
		pet.Wisdom = species.WisdomMax
	}

	growth := config.Interaction.StudyGrowth
	pet.Growth += growth

	// 清理学习状态
	pet.Status = "空闲"
	pet.StudyItem = ""
	pet.StudyTime = nil

	updateMood(pet, 5)
	database.DB.Save(pet)

	img := species.StudyEndImg
	if img == "" {
		img = config.Images["完成学习"]
	}
	imageCQ := core_game.GetImageCQ(img)

	var bonusText string
	if bonusW != "" {
		bonusText = "\n获得加成：" + bonusW
	}

	replyText := fmt.Sprintf("%s'%s'非常用功并完成了[%s]的学习，增加了%d点智慧和%d点成长！%s",
		utils.Emoji("F09F939A"), pet.Name, item.Name, addWisdom, growth, bonusText)

	core.SendGroupMessage(conn, event.GroupID, core_game.AtSender(event.UserID)+imageCQ+"\n"+replyText)
}

// ----------------锻炼 (Train) ----------------
func trainPet(conn *websocket.Conn, event *core.OneBotEvent) {
	lockKey := utils.GetUserLockKey(event.GroupID, event.UserID)
	mutex := utils.GetLock(lockKey)
	mutex.Lock()
	defer mutex.Unlock()

	pet, ok := core_game.GetPetOrReply(conn, event)
	if !ok {
		return
	}

	chkMsg := core_game.CheckPlayerStatus(pet, "锻炼")
	if chkMsg != "" {
		core.SendGroupMessage(conn, event.GroupID, chkMsg)
		return
	}

	prefix := config.GetCommand("锻炼")
	itemName := strings.TrimSpace(strings.TrimPrefix(event.RawMessage, prefix))
	if itemName == "" {
		core.SendGroupMessage(conn, event.GroupID, fmt.Sprintf("%s\n%s阁下，想让宠物去锻炼的话，需要一个力量类型的物品，然后再发送【%s 物品名】哦！", core_game.AtSender(event.UserID), utils.Emoji("E29D84"), prefix))
		return
	}

	item, exists := config.Items[itemName]
	if !exists || item.Type != "力量" {
		core.SendGroupMessage(conn, event.GroupID, core_game.AtSender(event.UserID)+"\n"+utils.Emoji("E29D84")+"阁下这件物品不能拿来锻炼哦！")
		return
	}

	// 背包校验
	var backpackItem models.BackpackItem
	err := database.DB.Where("user_id = ? AND group_id = ? AND item_name = ?", event.UserID, event.GroupID, itemName).First(&backpackItem).Error
	if err == gorm.ErrRecordNotFound || backpackItem.Quantity <= 0 {
		core.SendGroupMessage(conn, event.GroupID, core_game.AtSender(event.UserID)+"\n"+utils.Emoji("E29D84")+"阁下背包里没有这个物品哟！")
		return
	}

	// 每日次数限制
	stats := getDailyStats(event.UserID)
	trainLimit := config.Interaction.TrainLimit
	if trainLimit <= 0 {
		trainLimit = 3
	}
	if stats.TrainCount >= trainLimit {
		core.SendGroupMessage(conn, event.GroupID, core_game.AtSender(event.UserID)+"\n"+utils.Emoji("E29D84")+"阁下今天给宠物锻炼的次数用光了哦，让他放松放松吧！")
		return
	}

	// 力量上限校验
	species := config.Pets[pet.PetType]
	if pet.Strength >= species.StrengthMax {
		core.SendGroupMessage(conn, event.GroupID, fmt.Sprintf("%s\n%s阁下的'%s'的力量已经达到上限了哦，已经不需要锻炼啦！", core_game.AtSender(event.UserID), utils.Emoji("E29D84"), pet.Name))
		return
	}

	// 饱食度校验
	cost := config.Interaction.TrainHungerCost
	if pet.Hunger-cost <= 10 {
		core.SendGroupMessage(conn, event.GroupID, core_game.AtSender(event.UserID)+"\n"+utils.Emoji("E29D84")+"阁下，宠物的肚子已经饥肠辘辘了，先给它填饱肚子再让它锻炼吧！")
		return
	}

	tx := database.DB.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// 扣物品
	if err := core_game.AddBackpackItem(tx, event.UserID, event.GroupID, itemName, -1); err != nil {
		tx.Rollback()
		return
	}

	now := time.Now()
	pet.TrainTime = &now
	pet.TrainItem = itemName
	pet.Status = "锻炼"
	pet.Hunger -= cost

	if err := tx.Save(pet).Error; err != nil {
		tx.Rollback()
		return
	}

	tx.Commit()
	stats.TrainCount++

	img := species.TrainStartImg
	if img == "" {
		img = config.Images["开始锻炼"]
	}
	imageCQ := core_game.GetImageCQ(img)

	replyText := fmt.Sprintf("%s阁下让'%s'去锻炼[%s]，消耗了%d饱食，预计锻炼时间要%d分钟，完成后记得发送【完成锻炼】哦！",
		utils.Emoji("F09F92AA"), pet.Name, itemName, cost, item.Time)

	core.SendGroupMessage(conn, event.GroupID, core_game.AtSender(event.UserID)+imageCQ+"\n"+replyText)
}

func finishTrain(conn *websocket.Conn, event *core.OneBotEvent) {
	lockKey := utils.GetUserLockKey(event.GroupID, event.UserID)
	mutex := utils.GetLock(lockKey)
	mutex.Lock()
	defer mutex.Unlock()

	pet, ok := core_game.GetPetOrReply(conn, event)
	if !ok {
		return
	}

	if pet.Status != "锻炼" || pet.TrainItem == "" || pet.TrainTime == nil {
		core.SendGroupMessage(conn, event.GroupID, core_game.AtSender(event.UserID)+"\n"+utils.Emoji("E29D84")+"阁下的宠物没有在锻炼哦！")
		return
	}

	item, exists := config.Items[pet.TrainItem]
	if !exists {
		core.SendGroupMessage(conn, event.GroupID, core_game.AtSender(event.UserID)+"\n"+utils.Emoji("E29D84")+"没有找到锻炼物品的配置！")
		return
	}

	elapsed := time.Since(*pet.TrainTime)
	required := time.Duration(item.Time) * time.Minute
	if elapsed < required {
		rem := int(math.Ceil((required - elapsed).Minutes()))
		if rem <= 0 {
			rem = 1
		}
		core.SendGroupMessage(conn, event.GroupID, fmt.Sprintf("%s\n%s阁下的宠物还没有锻炼完成哦，大约还有%d分钟锻炼完毕！", core_game.AtSender(event.UserID), utils.Emoji("E29D84"), rem))
		return
	}

	moodMultiplier := getMoodMultiplier(pet.Mood)
	effectVal, _ := strconv.ParseInt(item.Effect, 10, 64)
	addStrength := int64(math.Round(float64(effectVal) * moodMultiplier))
	if addStrength <= 0 {
		addStrength = 1
	}

	species := config.Pets[pet.PetType]
	addStrength, bonusS := applyAttributeBonus(addStrength, &species)

	pet.Strength += addStrength
	if pet.Strength > species.StrengthMax {
		pet.Strength = species.StrengthMax
	}

	growth := config.Interaction.TrainGrowth
	pet.Growth += growth

	// 清理锻炼状态
	pet.Status = "空闲"
	pet.TrainItem = ""
	pet.TrainTime = nil

	updateMood(pet, 5)
	database.DB.Save(pet)

	img := species.TrainEndImg
	if img == "" {
		img = config.Images["完成锻炼"]
	}
	imageCQ := core_game.GetImageCQ(img)

	var bonusText string
	if bonusS != "" {
		bonusText = "\n获得加成：" + bonusS
	}

	replyText := fmt.Sprintf("%s'%s'完成了[%s]的锻炼，增加了%d点力量和%d点成长！%s",
		utils.Emoji("F09F92AA"), pet.Name, item.Name, addStrength, growth, bonusText)

	core.SendGroupMessage(conn, event.GroupID, core_game.AtSender(event.UserID)+imageCQ+"\n"+replyText)
}

// ----------------健身 (Fitness) ----------------
func fitnessPet(conn *websocket.Conn, event *core.OneBotEvent) {
	lockKey := utils.GetUserLockKey(event.GroupID, event.UserID)
	mutex := utils.GetLock(lockKey)
	mutex.Lock()
	defer mutex.Unlock()

	pet, ok := core_game.GetPetOrReply(conn, event)
	if !ok {
		return
	}

	chkMsg := core_game.CheckPlayerStatus(pet, "健身")
	if chkMsg != "" {
		core.SendGroupMessage(conn, event.GroupID, chkMsg)
		return
	}

	prefix := config.GetCommand("健身")
	itemName := strings.TrimSpace(strings.TrimPrefix(event.RawMessage, prefix))
	if itemName == "" {
		core.SendGroupMessage(conn, event.GroupID, fmt.Sprintf("%s\n%s阁下，想让宠物去健身的话，需要一个防御类型的物品，然后再发送【%s 物品名】哦！", core_game.AtSender(event.UserID), utils.Emoji("E29D84"), prefix))
		return
	}

	item, exists := config.Items[itemName]
	if !exists || item.Type != "防御" {
		core.SendGroupMessage(conn, event.GroupID, core_game.AtSender(event.UserID)+"\n"+utils.Emoji("E29D84")+"阁下这件物品不能拿来健身哦！")
		return
	}

	// 背包校验
	var backpackItem models.BackpackItem
	err := database.DB.Where("user_id = ? AND group_id = ? AND item_name = ?", event.UserID, event.GroupID, itemName).First(&backpackItem).Error
	if err == gorm.ErrRecordNotFound || backpackItem.Quantity <= 0 {
		core.SendGroupMessage(conn, event.GroupID, core_game.AtSender(event.UserID)+"\n"+utils.Emoji("E29D84")+"阁下背包里没有这个物品哟！")
		return
	}

	// 每日次数限制
	stats := getDailyStats(event.UserID)
	fitnessLimit := config.Interaction.FitnessLimit
	if fitnessLimit <= 0 {
		fitnessLimit = 5
	}
	if stats.FitnessCount >= fitnessLimit {
		core.SendGroupMessage(conn, event.GroupID, core_game.AtSender(event.UserID)+"\n"+utils.Emoji("E29D84")+"阁下今天给宠物健身的次数用光了哦，让他放松放松吧！")
		return
	}

	// 防御上限校验
	species := config.Pets[pet.PetType]
	if pet.Defense >= species.DefenseMax {
		core.SendGroupMessage(conn, event.GroupID, fmt.Sprintf("%s\n%s阁下的'%s'的防御已经达到上限了哦，已经不需要健身啦！", core_game.AtSender(event.UserID), utils.Emoji("E29D84"), pet.Name))
		return
	}

	// 饱食度校验
	cost := config.Interaction.FitnessHungerCost
	if pet.Hunger-cost <= 10 {
		core.SendGroupMessage(conn, event.GroupID, core_game.AtSender(event.UserID)+"\n"+utils.Emoji("E29D84")+"阁下，宠物的肚子已经饥肠辘辘了，先给它填饱肚子再让它健身吧！")
		return
	}

	tx := database.DB.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// 扣物品
	if err := core_game.AddBackpackItem(tx, event.UserID, event.GroupID, itemName, -1); err != nil {
		tx.Rollback()
		return
	}

	now := time.Now()
	pet.FitnessTime = &now
	pet.FitnessItem = itemName
	pet.Status = "健身"
	pet.Hunger -= cost

	if err := tx.Save(pet).Error; err != nil {
		tx.Rollback()
		return
	}

	tx.Commit()
	stats.FitnessCount++

	img := species.FitnessStartImg
	if img == "" {
		img = config.Images["开始健身"]
	}
	imageCQ := core_game.GetImageCQ(img)

	replyText := fmt.Sprintf("%s阁下让'%s'去健身[%s]，消耗了%d饱食，预计健身时间要%d分钟，完成后记得发送【完成健身】哦！",
		utils.Emoji("F09F8F80"), pet.Name, itemName, cost, item.Time)

	core.SendGroupMessage(conn, event.GroupID, core_game.AtSender(event.UserID)+imageCQ+"\n"+replyText)
}

func finishFitness(conn *websocket.Conn, event *core.OneBotEvent) {
	lockKey := utils.GetUserLockKey(event.GroupID, event.UserID)
	mutex := utils.GetLock(lockKey)
	mutex.Lock()
	defer mutex.Unlock()

	pet, ok := core_game.GetPetOrReply(conn, event)
	if !ok {
		return
	}

	if pet.Status != "健身" || pet.FitnessItem == "" || pet.FitnessTime == nil {
		core.SendGroupMessage(conn, event.GroupID, core_game.AtSender(event.UserID)+"\n"+utils.Emoji("E29D84")+"阁下的宠物没有在健身哦！")
		return
	}

	item, exists := config.Items[pet.FitnessItem]
	if !exists {
		core.SendGroupMessage(conn, event.GroupID, core_game.AtSender(event.UserID)+"\n"+utils.Emoji("E29D84")+"没有找到健身物品的配置！")
		return
	}

	elapsed := time.Since(*pet.FitnessTime)
	required := time.Duration(item.Time) * time.Minute
	if elapsed < required {
		rem := int(math.Ceil((required - elapsed).Minutes()))
		if rem <= 0 {
			rem = 1
		}
		core.SendGroupMessage(conn, event.GroupID, fmt.Sprintf("%s\n%s阁下的宠物还没有健身完成哦，大约还有%d分钟健身完毕！", core_game.AtSender(event.UserID), utils.Emoji("E29D84"), rem))
		return
	}

	moodMultiplier := getMoodMultiplier(pet.Mood)
	effectVal, _ := strconv.ParseInt(item.Effect, 10, 64)
	addDefense := int64(math.Round(float64(effectVal) * moodMultiplier))
	if addDefense <= 0 {
		addDefense = 1
	}

	species := config.Pets[pet.PetType]
	addDefense, bonusD := applyAttributeBonus(addDefense, &species)

	pet.Defense += addDefense
	if pet.Defense > species.DefenseMax {
		pet.Defense = species.DefenseMax
	}

	growth := config.Interaction.FitnessGrowth
	pet.Growth += growth

	// 清理健身状态
	pet.Status = "空闲"
	pet.FitnessItem = ""
	pet.FitnessTime = nil

	updateMood(pet, 5)
	database.DB.Save(pet)

	img := species.FitnessEndImg
	if img == "" {
		img = config.Images["完成健身"]
	}
	imageCQ := core_game.GetImageCQ(img)

	var bonusText string
	if bonusD != "" {
		bonusText = "\n获得加成：" + bonusD
	}

	replyText := fmt.Sprintf("%s'%s'完成了[%s]的健身，增加了%d点防御和%d点成长！%s",
		utils.Emoji("F09F8F80"), pet.Name, item.Name, addDefense, growth, bonusText)

	core.SendGroupMessage(conn, event.GroupID, core_game.AtSender(event.UserID)+imageCQ+"\n"+replyText)
}

// ----------------打工 (Work) ----------------
func workPet(conn *websocket.Conn, event *core.OneBotEvent) {
	lockKey := utils.GetUserLockKey(event.GroupID, event.UserID)
	mutex := utils.GetLock(lockKey)
	mutex.Lock()
	defer mutex.Unlock()

	pet, ok := core_game.GetPetOrReply(conn, event)
	if !ok {
		return
	}

	chkMsg := core_game.CheckPlayerStatus(pet, "打工")
	if chkMsg != "" {
		core.SendGroupMessage(conn, event.GroupID, chkMsg)
		return
	}

	stats := getDailyStats(event.UserID)
	workLimit := 5 // 默认 5
	// 我们不用另外查，打工配置直接读 config.WorkSettings
	if len(config.WorkSettings) == 0 {
		core.SendGroupMessage(conn, event.GroupID, core_game.AtSender(event.UserID)+"\n"+utils.Emoji("E29D84")+"没有打工岗位可以打工！")
		return
	}

	if stats.WorkCount >= int64(workLimit) {
		core.SendGroupMessage(conn, event.GroupID, core_game.AtSender(event.UserID)+"\n"+utils.Emoji("E29D84")+"阁下今天的打工次数已经用完了哦！")
		return
	}

	// 随机挑选一种工作
	var jobKeys []string
	for k := range config.WorkSettings {
		jobKeys = append(jobKeys, k)
	}
	rand.Seed(time.Now().UnixNano())
	jobName := jobKeys[rand.Intn(len(jobKeys))]
	job := config.WorkSettings[jobName]

	// 饱食度校验
	cost := job.HungerCost
	if pet.Hunger-cost <= 10 {
		core.SendGroupMessage(conn, event.GroupID, core_game.AtSender(event.UserID)+"\n"+utils.Emoji("E29D84")+"阁下，宠物的肚子已经饥肠辘辘了，先给它填饱肚子再让它打工吧！")
		return
	}

	now := time.Now()
	pet.Status = "打工"
	pet.WorkTime = &now
	pet.WorkType = jobName
	// 饱食度是打工完成时扣除还是开始时扣除？E-语言是在完成打工时扣的，但饱食是在开始时检查。
	// 让我们遵循E-语言，在完成时才真正扣除！
	database.DB.Save(pet)

	imageCQ := core_game.GetImageCQ(job.StartImage)

	replyQuotes := job.ReplyQuotes
	replyQuotes = strings.ReplaceAll(replyQuotes, "[宠物名]", pet.Name)

	replyText := fmt.Sprintf("%s\n%s\n预计打工时间要%d分钟，打工完成后记得发送【完成打工】哦！",
		imageCQ, replyQuotes, job.Time)

	core.SendGroupMessage(conn, event.GroupID, core_game.AtSender(event.UserID)+replyText)
}

func finishWork(conn *websocket.Conn, event *core.OneBotEvent) {
	lockKey := utils.GetUserLockKey(event.GroupID, event.UserID)
	mutex := utils.GetLock(lockKey)
	mutex.Lock()
	defer mutex.Unlock()

	pet, ok := core_game.GetPetOrReply(conn, event)
	if !ok {
		return
	}

	if pet.Status != "打工" || pet.WorkType == "" || pet.WorkTime == nil {
		core.SendGroupMessage(conn, event.GroupID, core_game.AtSender(event.UserID)+"\n"+utils.Emoji("E29D84")+"阁下的宠物没有在打工哦！")
		return
	}

	job, exists := config.WorkSettings[pet.WorkType]
	if !exists {
		core.SendGroupMessage(conn, event.GroupID, core_game.AtSender(event.UserID)+"\n"+utils.Emoji("E29D84")+"没有找到当前工作的配置！")
		return
	}

	elapsed := time.Since(*pet.WorkTime)
	required := time.Duration(job.Time) * time.Minute
	if elapsed < required {
		rem := int(math.Ceil((required - elapsed).Minutes()))
		if rem <= 0 {
			rem = 1
		}
		core.SendGroupMessage(conn, event.GroupID, fmt.Sprintf("%s\n%s阁下的宠物目前还在'%s'哦，大约还有%d分钟打工完毕！", core_game.AtSender(event.UserID), utils.Emoji("E29D84"), pet.WorkType, rem))
		return
	}

	moodMultiplier := getMoodMultiplier(pet.Mood)
	rewardCoin := int64(math.Round(float64(job.RewardCoin) * moodMultiplier))

	species := config.Pets[pet.PetType]
	rewardCoin, bonusC := applyCurrencyBonus(rewardCoin, &species)

	pet.Currency += rewardCoin

	// 饱食度扣除
	hungerCost := job.HungerCost
	pet.Hunger -= hungerCost
	if pet.Hunger < 0 {
		pet.Hunger = 0
	}

	// 扣除心情
	var moodDelta int
	switch pet.Mood {
	case "非常开心":
		moodDelta = -20
	case "开心":
		moodDelta = -15
	case "一般":
		moodDelta = -10
	case "难过":
		moodDelta = -5
	case "非常难过":
		moodDelta = 10
	}
	updateMood(pet, moodDelta)

	// 清理打工状态
	pet.Status = "空闲"
	pet.WorkType = ""
	pet.WorkTime = nil

	tx := database.DB.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// 奖励物品发放
	var awardText string
	if job.RewardItems != "" {
		parts := strings.Split(job.RewardItems, "#")
		var itemsAwarded []string
		for _, part := range parts {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			itemName := part
			qty := int64(1)
			if strings.Contains(part, "*") {
				subParts := strings.Split(part, "*")
				itemName = strings.TrimSpace(subParts[0])
				if q, err := strconv.ParseInt(strings.TrimSpace(subParts[1]), 10, 64); err == nil {
					qty = q
				}
			}

			itemConfig, hasConfig := config.Items[itemName]
			if hasConfig && itemConfig.ObtainType == 1 {
				// 一生一次
				var count int64
				tx.Model(&models.BackpackItem{}).Where("user_id = ? AND group_id = ? AND item_name = ?", event.UserID, event.GroupID, itemName).Count(&count)
				if count > 0 {
					continue // 已经获得了，跳过
				}
			}

			if err := core_game.AddBackpackItem(tx, event.UserID, event.GroupID, itemName, qty); err == nil {
				itemsAwarded = append(itemsAwarded, fmt.Sprintf("%s×%d", itemName, qty))
			}
		}
		if len(itemsAwarded) > 0 {
			awardText = strings.Join(itemsAwarded, "、")
		} else {
			awardText = "未获得奖励QAQ"
		}
	} else {
		awardText = "未获得奖励QAQ"
	}

	if err := tx.Save(pet).Error; err != nil {
		tx.Rollback()
		return
	}

	tx.Commit()

	// 累加打工次数
	stats := getDailyStats(event.UserID)
	stats.WorkCount++

	imageCQ := core_game.GetImageCQ(job.EndImage)

	var bonusText string
	if bonusC != "" {
		bonusText = "\n获得加成：" + bonusC
	}

	replyText := fmt.Sprintf("%s'%s'完成了工作！\n%s奖励物品：%s\n消耗了%d点饱食，增加了%d%s！%s",
		utils.Emoji("E29A93"), pet.Name,
		utils.Emoji("F09F8E81"), awardText,
		hungerCost, rewardCoin, config.Core.CoinName, bonusText)

	core.SendGroupMessage(conn, event.GroupID, core_game.AtSender(event.UserID)+imageCQ+"\n"+replyText)
}

// ----------------送礼 (Gift) ----------------
func giftPet(conn *websocket.Conn, event *core.OneBotEvent) {
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

	prefix := config.GetCommand("送礼")
	args := strings.TrimSpace(strings.TrimPrefix(event.RawMessage, prefix))
	if args == "" {
		core.SendGroupMessage(conn, event.GroupID, fmt.Sprintf("%s\n%s阁下，想给宠物送礼物呀，需要一个好感类型的物品，然后再发送【%s 物品名】就可以了哦！", core_game.AtSender(event.UserID), utils.Emoji("E29D84"), prefix))
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

	item, exists := config.Items[itemName]
	if !exists || item.Type != "好感" {
		core.SendGroupMessage(conn, event.GroupID, core_game.AtSender(event.UserID)+"\n"+utils.Emoji("F09F929D")+"阁下，此物品不是增加好感度的呀！")
		return
	}

	// 数量防溢出
	if quantity > 99999999 {
		core.SendGroupMessage(conn, event.GroupID, core_game.AtSender(event.UserID)+"\n"+utils.Emoji("E29D84")+"阁下输入的数值过大，请检查！")
		return
	}

	// 背包校验
	var backpackItem models.BackpackItem
	err := database.DB.Where("user_id = ? AND group_id = ? AND item_name = ?", event.UserID, event.GroupID, itemName).First(&backpackItem).Error
	if err == gorm.ErrRecordNotFound || backpackItem.Quantity < quantity {
		core.SendGroupMessage(conn, event.GroupID, core_game.AtSender(event.UserID)+"\n"+utils.Emoji("E29D84")+"阁下没有这么多物品啦！")
		return
	}

	// 每日限制校验
	stats := getDailyStats(event.UserID)
	giftLimit := config.Interaction.GiftLimit
	if giftLimit <= 0 {
		giftLimit = 5
	}
	if stats.GiftCount >= giftLimit {
		core.SendGroupMessage(conn, event.GroupID, core_game.AtSender(event.UserID)+"\n"+utils.Emoji("E29D84")+"阁下今天给宠物送礼的次数用光了哦，再送就要不高兴了哦！")
		return
	}
	if stats.GiftCount+quantity > giftLimit {
		core.SendGroupMessage(conn, event.GroupID, fmt.Sprintf("%s\n%s阁下今天最多只能再给宠物送%d次礼物哦！", core_game.AtSender(event.UserID), utils.Emoji("E29D84"), giftLimit-stats.GiftCount))
		return
	}

	// 喜欢的礼物翻倍
	species := config.Pets[pet.PetType]
	effectVal, _ := strconv.ParseInt(item.Effect, 10, 64)
	var finalAffection int64
	if itemName == species.FavoriteGift {
		finalAffection = effectVal * quantity * 2
	} else {
		finalAffection = effectVal * quantity
	}

	finalAffection, bonusA := applyAffectionBonus(finalAffection, &species)

	tx := database.DB.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	if err := core_game.AddBackpackItem(tx, event.UserID, event.GroupID, itemName, -quantity); err != nil {
		tx.Rollback()
		return
	}

	pet.Affection += finalAffection
	if err := tx.Save(pet).Error; err != nil {
		tx.Rollback()
		return
	}

	tx.Commit()
	stats.GiftCount += quantity

	imageCQ := core_game.GetImageCQ(item.Image)

	var bonusText string
	if bonusA != "" {
		bonusText = "\n获得加成：" + bonusA
	}

	replyText := fmt.Sprintf("%s阁下送给'%s' %d 个 %s！它对阁下的好感度＋%d！%s",
		utils.Emoji("F09F8E81"), pet.Name, quantity, itemName, finalAffection, bonusText)

	core.SendGroupMessage(conn, event.GroupID, core_game.AtSender(event.UserID)+imageCQ+"\n"+replyText)
}

// ----------------猜拳 (Rps) ----------------
func rpsPet(conn *websocket.Conn, event *core.OneBotEvent) {
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

	prefix := config.GetCommand("猜拳")
	target := strings.TrimSpace(strings.TrimPrefix(event.RawMessage, prefix))
	if target == "" {
		core.SendGroupMessage(conn, event.GroupID, fmt.Sprintf("%s\n%s阁下，想和宠物猜拳的话，直接发送【%s 石头/剪刀/布】哦！", core_game.AtSender(event.UserID), utils.Emoji("E29D84"), prefix))
		return
	}

	if target != "石头" && target != "剪刀" && target != "布" {
		core.SendGroupMessage(conn, event.GroupID, core_game.AtSender(event.UserID)+"\n"+utils.Emoji("E29D84")+"阁下你这是什么手势呀！只能出【剪刀、石头、布】哦！")
		return
	}

	stats := getDailyStats(event.UserID)

	// 冷却检查
	rpsInterval := config.Interaction.RpsInterval
	if !stats.RpsTime.IsZero() && time.Since(stats.RpsTime) < time.Duration(rpsInterval)*time.Second {
		rem := int(math.Ceil((time.Duration(rpsInterval)*time.Second - time.Since(stats.RpsTime)).Minutes()))
		if rem <= 0 {
			rem = 1
		}
		core.SendGroupMessage(conn, event.GroupID, fmt.Sprintf("%s\n%s阁下等 %d 分钟后再来找宠物猜拳吧！", core_game.AtSender(event.UserID), utils.Emoji("E29D84"), rem))
		return
	}

	// 饱食消耗
	rpsCost := config.Interaction.RpsHungerCost
	if pet.Hunger-rpsCost <= 10 {
		core.SendGroupMessage(conn, event.GroupID, core_game.AtSender(event.UserID)+"\n"+utils.Emoji("E29D84")+"阁下的宠物饿了，嘟着嘴巴不愿意和阁下猜拳。")
		return
	}

	pet.Hunger -= rpsCost
	stats.RpsTime = time.Now()

	// 系统随机出手
	rand.Seed(time.Now().UnixNano())
	sysVal := rand.Intn(3)
	var sysHand string
	switch sysVal {
	case 0:
		sysHand = "石头"
	case 1:
		sysHand = "剪刀"
	case 2:
		sysHand = "布"
	}

	var gameResult string
	if sysHand == target {
		gameResult = "阁下和对方出的都是一样的，所以是平局哦！"
	} else if (sysHand == "石头" && target == "剪刀") || (sysHand == "剪刀" && target == "布") || (sysHand == "布" && target == "石头") {
		var emoji string
		switch sysHand {
		case "石头":
			emoji = utils.Emoji("E29C8A")
		case "剪刀":
			emoji = utils.Emoji("E29C8C")
		case "布":
			emoji = utils.Emoji("E29C8B")
		}
		gameResult = fmt.Sprintf("%s'%s'出的是【%s】，阁下输了哦！", emoji, pet.Name, sysHand)
	} else {
		var emoji string
		switch target {
		case "石头":
			emoji = utils.Emoji("E29C8A")
		case "剪刀":
			emoji = utils.Emoji("E29C8C")
		case "布":
			emoji = utils.Emoji("E29C8B")
		}
		gameResult = fmt.Sprintf("%s阁下出的是【%s】，'%s'出的是【%s】所以它输了哦！", emoji, target, pet.Name, sysHand)
	}

	// 好感结算
	var addAffection int64
	var bonusAff string
	if stats.RpsAffection < config.Interaction.RpsAffectLimit {
		moodMultiplier := getMoodMultiplier(pet.Mood)
		addAffection = int64(math.Round(float64(config.Interaction.RpsAffection) * moodMultiplier))
		if addAffection <= 0 {
			addAffection = 1
		}

		if stats.RpsAffection+addAffection > config.Interaction.RpsAffectLimit {
			addAffection = config.Interaction.RpsAffectLimit - stats.RpsAffection
		}

		stats.RpsAffection += addAffection

		species := config.Pets[pet.PetType]
		addAffection, bonusAff = applyAffectionBonus(addAffection, &species)
		pet.Affection += addAffection
	}

	updateMood(pet, 5)
	database.DB.Save(pet)

	imageCQ := core_game.GetImageCQ(config.Images["猜拳"])

	var affectionDesc string
	if addAffection > 0 {
		var bonusText string
		if bonusAff != "" {
			bonusText = "\n获得加成：" + bonusAff
		}
		affectionDesc = fmt.Sprintf("\n%s好感＋%d点！%s", utils.Emoji("F09F9296"), addAffection, bonusText)
	}

	replyText := fmt.Sprintf("%s\n%s%s", gameResult, imageCQ, affectionDesc)
	core.SendGroupMessage(conn, event.GroupID, core_game.AtSender(event.UserID)+"\n"+replyText)
}

// ----------------喂养 (Feed) ----------------
func feedPet(conn *websocket.Conn, event *core.OneBotEvent) {
	lockKey := utils.GetUserLockKey(event.GroupID, event.UserID)
	mutex := utils.GetLock(lockKey)
	mutex.Lock()
	defer mutex.Unlock()

	pet, ok := core_game.GetPetOrReply(conn, event)
	if !ok {
		return
	}

	chkMsg := core_game.CheckPlayerStatus(pet, "治疗") // 允许濒死时喂养以救治
	if chkMsg != "" {
		core.SendGroupMessage(conn, event.GroupID, chkMsg)
		return
	}

	prefix := config.GetCommand("喂养")
	args := strings.TrimSpace(strings.TrimPrefix(event.RawMessage, prefix))
	if args == "" {
		core.SendGroupMessage(conn, event.GroupID, fmt.Sprintf("%s\n%s阁下，想喂宠物吃东西的话，需要一个饱食类型的物品，然后再发送【%s 物品名】哦！", core_game.AtSender(event.UserID), utils.Emoji("E29D84"), prefix))
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

	item, exists := config.Items[itemName]
	if !exists || item.Type != "饱食" {
		core.SendGroupMessage(conn, event.GroupID, core_game.AtSender(event.UserID)+"\n"+utils.Emoji("E29D84")+"阁下，此物品不是喂给宠物吃的呀，乱吃东西会西内哒！")
		return
	}

	// 数量防溢出
	if quantity > 99999999 {
		core.SendGroupMessage(conn, event.GroupID, core_game.AtSender(event.UserID)+"\n"+utils.Emoji("E29D84")+"阁下输入的数值过大，请检查！")
		return
	}

	// 背包校验
	var backpackItem models.BackpackItem
	err := database.DB.Where("user_id = ? AND group_id = ? AND item_name = ?", event.UserID, event.GroupID, itemName).First(&backpackItem).Error
	if err == gorm.ErrRecordNotFound || backpackItem.Quantity < quantity {
		core.SendGroupMessage(conn, event.GroupID, core_game.AtSender(event.UserID)+"\n"+utils.Emoji("E29D84")+"阁下没有这么多食物啦！")
		return
	}

	species := config.Pets[pet.PetType]
	if pet.Hunger >= species.HungerMax {
		core.SendGroupMessage(conn, event.GroupID, core_game.AtSender(event.UserID)+"\n"+utils.Emoji("E29D84")+"阁下的宠物一点都不饿哦！")
		return
	}

	effectVal, _ := strconv.ParseInt(item.Effect, 10, 64)
	var addHunger int64
	if itemName == species.FavoriteFood {
		addHunger = effectVal * quantity * 2
	} else {
		addHunger = effectVal * quantity
	}

	tx := database.DB.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// 扣背包
	if err := core_game.AddBackpackItem(tx, event.UserID, event.GroupID, itemName, -quantity); err != nil {
		tx.Rollback()
		return
	}

	pet.Hunger += addHunger
	isFull := false
	if pet.Hunger >= species.HungerMax {
		pet.Hunger = species.HungerMax
		isFull = true
	}

	// 如果处于濒死状态，喂饱可以救活
	saved := false
	if pet.Status == "濒死" {
		pet.Status = "空闲"
		pet.DyingTime = nil
		pet.Health = species.Health / 2 // 喂养恢复一半基础血量
		if pet.Health <= 0 {
			pet.Health = 10
		}
		saved = true
	}

	if err := tx.Save(pet).Error; err != nil {
		tx.Rollback()
		return
	}

	tx.Commit()

	imageCQ := core_game.GetImageCQ(item.Image)

	var replyText string
	if saved {
		if isFull {
			replyText = fmt.Sprintf("%s阁下喂给宠物 %d 个 %s，宠物的饱食度回满啦，并且成功脱离了濒死状态！",
				utils.Emoji("F09F928A"), quantity, itemName)
		} else {
			replyText = fmt.Sprintf("%s阁下喂给宠物 %d 个 %s，宠物的饱食恢复到了 %d 点，并且脱离了濒死状态！",
				utils.Emoji("F09F8D99"), quantity, itemName, pet.Hunger)
		}
	} else {
		if isFull {
			replyText = fmt.Sprintf("%s阁下喂给宠物 %d 个 %s，宠物的饱食度回满啦！",
				utils.Emoji("F09F8D99"), quantity, itemName)
		} else {
			replyText = fmt.Sprintf("%s阁下喂给宠物 %d 个 %s，宠物的饱食恢复到了 %d 点！",
				utils.Emoji("F09F8D99"), quantity, itemName, pet.Hunger)
		}
	}

	core.SendGroupMessage(conn, event.GroupID, core_game.AtSender(event.UserID)+imageCQ+"\n"+replyText)
}

// ----------------散步 (Walk) ----------------
func walkPet(conn *websocket.Conn, event *core.OneBotEvent) {
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

	stats := getDailyStats(event.UserID)

	// 散步间隔
	walkInterval := config.Interaction.WalkInterval
	if !stats.WalkTime.IsZero() && time.Since(stats.WalkTime) < time.Duration(walkInterval)*time.Second {
		core.SendGroupMessage(conn, event.GroupID, core_game.AtSender(event.UserID)+"\n"+utils.Emoji("E29D84")+"阁下等会再散步吧，让宠物歇歇嘛！")
		return
	}

	walkCost := config.Interaction.WalkHungerCost
	if pet.Hunger < walkCost {
		core.SendGroupMessage(conn, event.GroupID, core_game.AtSender(event.UserID)+"\n"+utils.Emoji("E29D84")+"宠物腿发软，太饿了走不动道了！")
		return
	}

	stats.WalkTime = time.Now()
	pet.Hunger -= walkCost

	moodMultiplier := getMoodMultiplier(pet.Mood)
	growth := int64(math.Round(float64(config.Interaction.WalkGrowth) * moodMultiplier))
	affection := int64(math.Round(float64(config.Interaction.WalkAffection) * moodMultiplier))

	// 上限检查
	var addG, addA int64
	if stats.WalkGrowth < config.Interaction.WalkGrowthLimit {
		addG = growth
		if stats.WalkGrowth+addG > config.Interaction.WalkGrowthLimit {
			addG = config.Interaction.WalkGrowthLimit - stats.WalkGrowth
		}
		stats.WalkGrowth += addG
	}
	if stats.WalkAffection < config.Interaction.WalkAffectLimit {
		addA = affection
		if stats.WalkAffection+addA > config.Interaction.WalkAffectLimit {
			addA = config.Interaction.WalkAffectLimit - stats.WalkAffection
		}
		stats.WalkAffection += addA
	}

	species := config.Pets[pet.PetType]
	var bonusList []string
	if addG > 0 {
		addG, bonusG := applyGrowthBonus(addG, &species)
		pet.Growth += addG
		if bonusG != "" {
			bonusList = append(bonusList, bonusG)
		}
	}
	if addA > 0 {
		addA, bonusA := applyAffectionBonus(addA, &species)
		pet.Affection += addA
		if bonusA != "" {
			bonusList = append(bonusList, bonusA)
		}
	}

	var bonusText string
	if len(bonusList) > 0 {
		bonusText = "\n获得加成：" + strings.Join(bonusList, "、")
	}

	updateMood(pet, 5)
	database.DB.Save(pet)

	imageCQ := core_game.GetImageCQ(config.Images["散步"])

	replyText := fmt.Sprintf("%s阁下带着'%s'一起散步，它享受着和阁下在一起的时光。\n好感＋%d、成长＋%d%s",
		utils.Emoji("F09F8C9F"), pet.Name,
		addA, addG, bonusText)

	core.SendGroupMessage(conn, event.GroupID, core_game.AtSender(event.UserID)+imageCQ+"\n"+replyText)
}

// ----------------钓鱼 (Fish) ----------------
func castRod(conn *websocket.Conn, event *core.OneBotEvent) {
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

	stats := getDailyStats(event.UserID)
	if !stats.FishTime.IsZero() {
		core.SendGroupMessage(conn, event.GroupID, core_game.AtSender(event.UserID)+"\n"+utils.Emoji("E29D84")+"阁下已经在钓鱼了哦，集中注意力，别让鱼跑啦！")
		return
	}

	stats.FishTime = time.Now()
	rand.Seed(time.Now().UnixNano())
	stats.FishMinSec = rand.Intn(5) + 7   // 7 到 11 秒
	stats.FishMaxSec = rand.Intn(6) + 15  // 15 到 20 秒

	imageCQ := core_game.GetImageCQ(config.Images["抛竿"])

	replyText := fmt.Sprintf("%s阁下和'%s'远远的抛去鱼竿！\n%s请在 %d 到 %d 秒之间收回鱼竿(发送【收竿】)哦！",
		utils.Emoji("F09F908B"), pet.Name,
		utils.Emoji("F09F958E"), stats.FishMinSec, stats.FishMaxSec)

	core.SendGroupMessage(conn, event.GroupID, core_game.AtSender(event.UserID)+imageCQ+"\n"+replyText)
}

func pullRod(conn *websocket.Conn, event *core.OneBotEvent) {
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

	stats := getDailyStats(event.UserID)
	if stats.FishTime.IsZero() {
		core.SendGroupMessage(conn, event.GroupID, core_game.AtSender(event.UserID)+"\n"+utils.Emoji("E29D84")+"阁下还没有抛竿哦！")
		return
	}

	fishTime := stats.FishTime
	minSec := stats.FishMinSec
	maxSec := stats.FishMaxSec

	// 重置钓鱼状态
	stats.FishTime = time.Time{}
	stats.FishMinSec = 0
	stats.FishMaxSec = 0

	elapsed := time.Since(fishTime).Seconds()

	if elapsed < float64(minSec) || elapsed > float64(maxSec) {
		core.SendGroupMessage(conn, event.GroupID, core_game.AtSender(event.UserID)+"\n"+utils.Emoji("E29D84")+"很遗憾，阁下收竿时机不对，错过了最佳时机，让小鱼溜走了！")
		return
	}

	// 饱食度扣除
	fishCost := config.Interaction.FishHungerCost
	if pet.Hunger < fishCost {
		core.SendGroupMessage(conn, event.GroupID, core_game.AtSender(event.UserID)+"\n"+utils.Emoji("E29D84")+"宠物太饿啦，钓起鱼来没力气，鱼又挣脱跑掉了！")
		return
	}
	pet.Hunger -= fishCost

	// 随机鱼类
	fishSpecies := config.Interaction.FishSpecies
	if len(fishSpecies) == 0 {
		core.SendGroupMessage(conn, event.GroupID, core_game.AtSender(event.UserID)+"\n"+utils.Emoji("E29D84")+"没有鱼类配置，无法钓鱼！")
		return
	}

	tx := database.DB.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	var finalFish string
	rand.Seed(time.Now().UnixNano())

	// 查找并校验“获得类型=1”
	attempts := 0
	for attempts < 50 {
		attempts++
		fishName := fishSpecies[rand.Intn(len(fishSpecies))]
		item, exists := config.Items[fishName]
		if !exists {
			continue
		}

		if item.ObtainType == 1 {
			var count int64
			tx.Model(&models.BackpackItem{}).Where("user_id = ? AND group_id = ? AND item_name = ?", event.UserID, event.GroupID, fishName).Count(&count)
			if count > 0 {
				continue // 重新选择
			}
		}
		finalFish = fishName
		break
	}

	if finalFish == "" {
		tx.Rollback()
		core.SendGroupMessage(conn, event.GroupID, core_game.AtSender(event.UserID)+"\n"+utils.Emoji("E29D84")+"运气不好，没有任何合适的鱼咬钩！")
		return
	}

	fishItem := config.Items[finalFish]
	imageCQ := core_game.GetImageCQ(fishItem.Image)

	successRate := config.Interaction.FishSuccessRate
	if successRate <= 0 {
		successRate = 80
	}

	success := rand.Intn(100) < int(successRate)

	var replyText string
	if success {
		if err := core_game.AddBackpackItem(tx, event.UserID, event.GroupID, finalFish, 1); err != nil {
			tx.Rollback()
			return
		}
		replyText = fmt.Sprintf("%s恭喜阁下和'%s'成功钓到了一条'%s'！并妥善收入背包。", utils.Emoji("F09F90AC"), pet.Name, finalFish)
	} else {
		replyText = fmt.Sprintf("%s阁下钓到了一条'%s'，啊！它突然挣脱了阁下回到了水里！", utils.Emoji("F09F9980"), finalFish)
	}

	if err := tx.Save(pet).Error; err != nil {
		tx.Rollback()
		return
	}

	tx.Commit()

	core.SendGroupMessage(conn, event.GroupID, core_game.AtSender(event.UserID)+imageCQ+"\n"+replyText)
}
