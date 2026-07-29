package entertainment

import (
	"fmt"
	"math"
	"math/rand"
	"strconv"
	"strings"
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

func init() {
	core.RegisterHandler(config.GetCommand("偷袭"), sneakAttack)
	core.RegisterHandler(config.GetCommand("回击"), counterAttack)
}

// 解析群消息中的 At QQ号
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

// 1. 偷袭
func sneakAttack(conn *websocket.Conn, event *core.OneBotEvent) {
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

	// 偷袭冷却时间校验
	lastSneak := utils.GetLastSneakTime(event.UserID)
	sneakCooldown := config.Interaction.SnackInterval // 冷却时间 (秒)
	if !lastSneak.IsZero() && time.Since(lastSneak) < time.Duration(sneakCooldown)*time.Second {
		rem := int(math.Ceil((time.Duration(sneakCooldown)*time.Second - time.Since(lastSneak)).Minutes()))
		if rem <= 0 {
			rem = 1
		}
		core.SendGroupMessage(conn, event.GroupID, fmt.Sprintf("%s\n%s阁下积点德吧，宠物现在还在偷袭冷却时间里，大概还有%d分钟冷却完毕！",
			core_game.AtSender(event.UserID), utils.Emoji("E29D84"), rem))
		return
	}

	// 目标校验
	prefix := config.GetCommand("偷袭")
	args := strings.TrimSpace(strings.TrimPrefix(event.RawMessage, prefix))
	targetQQ := parseAtQQ(args)
	if targetQQ == 0 {
		core.SendGroupMessage(conn, event.GroupID, fmt.Sprintf("%s\n%s阁下要攻击谁呢？正确食用方法：%s@QQ",
			core_game.AtSender(event.UserID), utils.Emoji("E29D84"), prefix))
		return
	}

	if targetQQ == event.UserID {
		core.SendGroupMessage(conn, event.GroupID, core_game.AtSender(event.UserID)+"\n"+utils.Emoji("E29D84")+"阁下为什么要让宠物打自己呢？")
		return
	}

	// 获取对方宠物
	var targetPet models.UserPet
	err := database.DB.Where("user_id = ? AND group_id = ?", targetQQ, event.GroupID).First(&targetPet).Error
	if err == gorm.ErrRecordNotFound || targetPet.Status == "失去宠物" {
		core.SendGroupMessage(conn, event.GroupID, core_game.AtSender(event.UserID)+"\n"+utils.Emoji("E29D84")+"阁下，对方还没有宠物呢！")
		return
	} else if err != nil {
		core.SendGroupMessage(conn, event.GroupID, "数据库错误: "+err.Error())
		return
	}

	// 偷袭保护校验
	targetProtect := utils.GetLastProtectTime(targetQQ)
	protectTime := config.Interaction.SnackProtect
	if !targetProtect.IsZero() && time.Since(targetProtect) < time.Duration(protectTime)*time.Second {
		rem := int(math.Ceil((time.Duration(protectTime)*time.Second - time.Since(targetProtect)).Minutes()))
		if rem <= 0 {
			rem = 1
		}
		core.SendGroupMessage(conn, event.GroupID, fmt.Sprintf("%s\n%s阁下，对方的宠物现在还有%d分钟偷袭保护哦！",
			core_game.AtSender(event.UserID), utils.Emoji("E29D84"), rem))
		return
	}

	// 治疗保护校验
	targetTreat := utils.GetLastTreatTime(targetQQ)
	treatProtectTime := config.Core.DyingProtectTime
	if !targetTreat.IsZero() && time.Since(targetTreat) < time.Duration(treatProtectTime)*time.Minute {
		rem := int(math.Ceil((time.Duration(treatProtectTime)*time.Minute - time.Since(targetTreat)).Minutes()))
		if rem <= 0 {
			rem = 1
		}
		core.SendGroupMessage(conn, event.GroupID, fmt.Sprintf("%s\n%s阁下，对方的宠物现在还有%d分钟治疗保护哦！",
			core_game.AtSender(event.UserID), utils.Emoji("E29D84"), rem))
		return
	}

	// 饱食度校验
	cost := config.Interaction.SnackHungerCost
	if pet.Hunger-cost <= 10 {
		core.SendGroupMessage(conn, event.GroupID, fmt.Sprintf("%s\n%s阁下，'%s'的肚子已经饿的咕咕叫了，先给宠物喂点食物吃吧！",
			core_game.AtSender(event.UserID), utils.Emoji("E29D84"), pet.Name))
		return
	}

	// 扣减自己饱食度并更新偷袭时间
	pet.Hunger -= cost
	now := time.Now()
	utils.SetLastSneakTime(event.UserID, now)

	// 随机成功概率
	successRate := config.Interaction.SnackSuccess
	if successRate <= 0 {
		successRate = 99
	}

	rand.Seed(time.Now().UnixNano())
	if rand.Intn(100)+1 > int(successRate) {
		// 失败了
		database.DB.Save(pet)
		imageCQ := core_game.GetImageCQ(config.Images["偷袭失败"])
		core.SendGroupMessage(conn, event.GroupID, fmt.Sprintf("%s%s\n%s'%s'消耗了%d饱食去偷袭%s家的'%s',但是宠物一不小心失手了......",
			core_game.AtSender(event.UserID), imageCQ,
			utils.Emoji("E29D84"), pet.Name, cost, core_game.AtSender(targetQQ), targetPet.Name))
		return
	}

	// 成功了，计算伤害与暴击
	species := config.Pets[pet.PetType]

	wisdomMax := species.WisdomMax
	if wisdomMax <= 0 {
		wisdomMax = 100
	}

	critPct := float64(pet.Wisdom) / float64(wisdomMax) * 100.0
	var critRate int = 0
	if critPct > 80 {
		critRate = 25
	} else if critPct > 60 {
		critRate = 18
	} else if critPct > 50 {
		critRate = 12
	} else if critPct > 30 {
		critRate = 8
	} else if critPct > 10 {
		critRate = 5
	}
	if critPct >= 100 {
		critRate = 30
	}

	isCrit := rand.Intn(100) < critRate
	var damage int64
	var targetDefense int64 = targetPet.Defense

	if isCrit {
		targetDefense = int64(math.Round(float64(targetDefense) * 0.8))
		if targetDefense <= 0 {
			targetDefense = 1
		}
		damage = pet.Strength*2 - targetDefense
	} else {
		// 普通偷袭
		myStrength := int64(math.Round(float64(pet.Strength) * 0.8))
		targetDefense = int64(math.Round(float64(targetDefense) * 0.5))
		if targetDefense <= 0 {
			targetDefense = 1
		}
		damage = myStrength - targetDefense
	}

	if damage <= 0 {
		damage = 1
	}

	targetPet.Health -= damage

	tx := database.DB.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	isDying := false
	if targetPet.Health <= 0 {
		targetPet.Health = 0
		targetPet.Status = "濒死"
		targetPet.DyingTime = &now
		isDying = true
	}

	if err := tx.Save(pet).Error; err != nil {
		tx.Rollback()
		return
	}
	if err := tx.Save(&targetPet).Error; err != nil {
		tx.Rollback()
		return
	}

	tx.Commit()

	// 更新保护时间及攻击者
	utils.SetLastProtectTime(targetQQ, now)
	utils.SetAttackerID(targetQQ, event.UserID)

	imageCQ := core_game.GetImageCQ(config.Images["偷袭成功"])

	if isDying {
		// 发送私聊消息给受害者
		pm := fmt.Sprintf("阁下的宠物被%d偷袭了！\n'%s'的血量－%d\n现在宠物已经濒死了，快去为宠物疗伤，使用恢复血量的物品或者发送'宠物治疗'！",
			event.UserID, targetPet.Name, damage)
		core.SendPrivateMessage(conn, targetQQ, pm)

		replyText := fmt.Sprintf("%s%s\n%s'%s'消耗了%d饱食，趁%s的'%s'不注意，对它造成了%d点伤害（暴击：%t），对方已进入濒死状态！",
			core_game.AtSender(event.UserID), imageCQ,
			utils.Emoji("F09F92A0"), pet.Name, cost, core_game.AtSender(targetQQ), targetPet.Name, damage, isCrit)
		core.SendGroupMessage(conn, event.GroupID, replyText)
	} else {
		// 发送私聊消息给受害者
		pm := fmt.Sprintf("阁下的宠物被%d偷袭了！\n'%s'的血量－%d\n不甘心的话，就发送【%s】来还手吧！",
			event.UserID, targetPet.Name, damage, config.GetCommand("回击"))
		core.SendPrivateMessage(conn, targetQQ, pm)

		replyText := fmt.Sprintf("%s%s\n%s'%s'消耗了%d饱食，趁%s的'%s'不注意，对它造成了%d点伤害（防御力抵扣：%d），对方最終受到了%d点伤害！",
			core_game.AtSender(event.UserID), imageCQ,
			utils.Emoji("F09F92A0"), pet.Name, cost, core_game.AtSender(targetQQ), targetPet.Name, pet.Strength, targetDefense, damage)
		core.SendGroupMessage(conn, event.GroupID, replyText)
	}
}

// 2. 回击
func counterAttack(conn *websocket.Conn, event *core.OneBotEvent) {
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

	attackerID := utils.GetAttackerID(event.UserID)
	if attackerID == 0 {
		core.SendGroupMessage(conn, event.GroupID, core_game.AtSender(event.UserID)+"\n"+utils.Emoji("E29D84")+"没有人偷袭阁下的宠物哦，不要打草惊蛇哦！")
		return
	}

	// 获取对方宠物
	var targetPet models.UserPet
	err := database.DB.Where("user_id = ? AND group_id = ?", attackerID, event.GroupID).First(&targetPet).Error
	if err == gorm.ErrRecordNotFound || targetPet.Status == "失去宠物" {
		core.SendGroupMessage(conn, event.GroupID, core_game.AtSender(event.UserID)+"\n"+utils.Emoji("E29D84")+"对方宠物似乎已经不存在了，无法回击。")
		return
	} else if err != nil {
		core.SendGroupMessage(conn, event.GroupID, "数据库错误: "+err.Error())
		return
	}

	// 饱食度校验
	cost := config.Interaction.CounterHunger
	if pet.Hunger-cost <= 10 {
		core.SendGroupMessage(conn, event.GroupID, fmt.Sprintf("%s\n%s阁下，'%s'的肚子已经饿的咕咕叫了，先给宠物喂点食物吃吧！",
			core_game.AtSender(event.UserID), utils.Emoji("E29D84"), pet.Name))
		return
	}

	// 扣饱食度并清除攻击者关联
	pet.Hunger -= cost
	utils.SetAttackerID(event.UserID, 0)

	// 回击成功概率
	successRate := config.Interaction.CounterSuccess
	if successRate <= 0 {
		successRate = 80
	}

	rand.Seed(time.Now().UnixNano())
	if rand.Intn(100)+1 > int(successRate) {
		database.DB.Save(pet)
		imageCQ := core_game.GetImageCQ(config.Images["回击失败"])
		core.SendGroupMessage(conn, event.GroupID, fmt.Sprintf("%s%s\n%s'%s'消耗了%d饱食去回击[@%d]家的'%s'，但由于精力不集中打偏了......",
			core_game.AtSender(event.UserID), imageCQ,
			utils.Emoji("E29D84"), pet.Name, cost, attackerID, targetPet.Name))
		return
	}

	// 成功了，计算伤害
	var critRate int = 0
	wisdom := pet.Wisdom

	if wisdom >= 245 {
		critRate = 32
	} else if wisdom >= 120 {
		critRate = 23
	} else if wisdom >= 70 {
		critRate = 18
	} else if wisdom >= 30 {
		critRate = 13
	} else if wisdom >= 1 {
		critRate = 10
	}

	isCrit := rand.Intn(100) < critRate
	var damage int64
	var targetDefense int64 = targetPet.Defense

	if isCrit {
		targetDefense = int64(math.Round(float64(targetDefense) * 0.7))
		if targetDefense <= 0 {
			targetDefense = 1
		}
		damage = pet.Strength*2 - targetDefense
	} else {
		myStrength := int64(math.Round(float64(pet.Strength) * 1.2))
		targetDefense = int64(math.Round(float64(targetDefense) * 0.4))
		if targetDefense <= 0 {
			targetDefense = 1
		}
		damage = myStrength - targetDefense
	}

	if damage <= 0 {
		damage = 1
	}

	targetPet.Health -= damage

	tx := database.DB.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	isDying := false
	now := time.Now()
	if targetPet.Health <= 0 {
		targetPet.Health = 0
		targetPet.Status = "濒死"
		targetPet.DyingTime = &now
		isDying = true
	}

	if err := tx.Save(pet).Error; err != nil {
		tx.Rollback()
		return
	}
	if err := tx.Save(&targetPet).Error; err != nil {
		tx.Rollback()
		return
	}

	tx.Commit()

	imageCQ := core_game.GetImageCQ(config.Images["回击成功"])

	if isDying {
		pm := fmt.Sprintf("阁下的宠物被%d回击啦！\n'%s'的血量－%d\n宠物现在已经濒死了，快去为宠物疗伤，使用恢复血量的物品或者发送'宠物治疗'！",
			event.UserID, targetPet.Name, damage)
		core.SendPrivateMessage(conn, attackerID, pm)

		replyText := fmt.Sprintf("%s%s\n%s'%s'消耗了%d饱食，去回击[@%d]的'%s'，对它造成了%d点伤害（暴击：%t），对方已进入濒死状态！",
			core_game.AtSender(event.UserID), imageCQ,
			utils.Emoji("F09F92A0"), pet.Name, cost, attackerID, targetPet.Name, damage, isCrit)
		core.SendGroupMessage(conn, event.GroupID, replyText)
	} else {
		pm := fmt.Sprintf("阁下的宠物被%d回击咯！\n'%s'的血量－%d",
			event.UserID, targetPet.Name, damage)
		core.SendPrivateMessage(conn, attackerID, pm)

		replyText := fmt.Sprintf("%s%s\n%s'%s'消耗了%d饱食，去回击[@%d]的'%s'对它造成了%d点伤害，格挡了%d点伤害，最终受到了%d点伤害！",
			core_game.AtSender(event.UserID), imageCQ,
			utils.Emoji("F09F92A0"), pet.Name, cost, attackerID, targetPet.Name, pet.Strength, targetDefense, damage)
		core.SendGroupMessage(conn, event.GroupID, replyText)
	}
}
