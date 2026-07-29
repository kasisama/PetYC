package entertainment

import (
	"fmt"
	"math/rand"
	"sort"
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

type LotteryReward struct {
	Percentage int
	ItemName   string
	MinQty     int64
	MaxQty     int64
}

func init() {
	core.RegisterHandler(config.GetCommand("抽奖"), drawLottery)
}

// 解析抽奖奖励设置字符串，如: 2%成长激素*1~2#100%[货币]*100~300#5%爱心礼包*10~20
func parseLotteryRewards(rewardStr string) []LotteryReward {
	var rewards []LotteryReward
	if rewardStr == "" {
		return rewards
	}

	parts := strings.Split(rewardStr, "#")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		// 提取几率 (百分比)
		pctIndex := strings.Index(part, "%")
		if pctIndex == -1 {
			continue
		}
		pctStr := part[:pctIndex]
		pct, err := strconv.Atoi(pctStr)
		if err != nil {
			continue
		}

		rest := part[pctIndex+1:]
		itemName := rest
		var minQty, maxQty int64 = 1, 1

		if strings.Contains(rest, "*") {
			subParts := strings.Split(rest, "*")
			itemName = strings.TrimSpace(subParts[0])
			qtyStr := strings.TrimSpace(subParts[1])

			if strings.Contains(qtyStr, "~") {
				qtyParts := strings.Split(qtyStr, "~")
				if minVal, err := strconv.ParseInt(strings.TrimSpace(qtyParts[0]), 10, 64); err == nil {
					minQty = minVal
				}
				if maxVal, err := strconv.ParseInt(strings.TrimSpace(qtyParts[1]), 10, 64); err == nil {
					maxQty = maxVal
				}
			} else {
				if qty, err := strconv.ParseInt(qtyStr, 10, 64); err == nil {
					minQty = qty
					maxQty = qty
				}
			}
		}

		rewards = append(rewards, LotteryReward{
			Percentage: pct,
			ItemName:   itemName,
			MinQty:     minQty,
			MaxQty:     maxQty,
		})
	}

	// 冒泡排序，从小到大
	sort.Slice(rewards, func(i, j int) bool {
		return rewards[i].Percentage < rewards[j].Percentage
	})

	return rewards
}

// 辅助抽奖逻辑
func rollReward(rewards []LotteryReward) (string, int64) {
	if len(rewards) == 0 {
		return "", 0
	}

	rand.Seed(time.Now().UnixNano())
	for _, reward := range rewards {
		roll := rand.Intn(100) + 1 // 1 到 100
		if roll <= reward.Percentage {
			// 命中
			qty := reward.MinQty
			if reward.MaxQty > reward.MinQty {
				qty = reward.MinQty + int64(rand.Intn(int(reward.MaxQty-reward.MinQty+1)))
			}
			return reward.ItemName, qty
		}
	}

	// 兜底返回最后一个（通常是 100% 概率）
	last := rewards[len(rewards)-1]
	qty := last.MinQty
	if last.MaxQty > last.MinQty {
		qty = last.MinQty + int64(rand.Intn(int(last.MaxQty-last.MinQty+1)))
	}
	return last.ItemName, qty
}

func drawLottery(conn *websocket.Conn, event *core.OneBotEvent) {
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

	lotteryItemsStr := config.Interaction.LotteryItem
	if lotteryItemsStr == "" {
		lotteryItemsStr = "抽奖券*10"
	}

	costParts := strings.Split(lotteryItemsStr, "#")
	if len(costParts) == 0 {
		core.SendGroupMessage(conn, event.GroupID, core_game.AtSender(event.UserID)+"\n"+utils.Emoji("E29D84")+"不好意思阁下，抽奖池暂未开放哦！")
		return
	}

	// 校验背包资源
	var missingList []string
	type CostItem struct {
		Name string
		Qty  int64
	}
	var costs []CostItem

	for _, part := range costParts {
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

		var backpackItem models.BackpackItem
		err := database.DB.Where("user_id = ? AND group_id = ? AND item_name = ?", event.UserID, event.GroupID, itemName).First(&backpackItem).Error
		if err == gorm.ErrRecordNotFound || backpackItem.Quantity < qty {
			has := int64(0)
			if err == nil {
				has = backpackItem.Quantity
			}
			missingList = append(missingList, fmt.Sprintf("%d个%s", qty-has, itemName))
		}
		costs = append(costs, CostItem{Name: itemName, Qty: qty})
	}

	if len(missingList) > 0 {
		core.SendGroupMessage(conn, event.GroupID, fmt.Sprintf("%s\n%s阁下的物品不足哟，还需要%s哦！",
			core_game.AtSender(event.UserID), utils.Emoji("E29D84"), strings.Join(missingList, "、")))
		return
	}

	// 解析奖励
	rewards := parseLotteryRewards(config.Interaction.LotteryRewardStr)
	if len(rewards) == 0 {
		core.SendGroupMessage(conn, event.GroupID, core_game.AtSender(event.UserID)+"\n"+utils.Emoji("E29D84")+"不好意思阁下，抽奖池里暂时没有任何奖励哦！")
		return
	}

	tx := database.DB.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// 扣减消耗
	var costDescParts []string
	for _, c := range costs {
		if err := core_game.AddBackpackItem(tx, event.UserID, event.GroupID, c.Name, -c.Qty); err != nil {
			tx.Rollback()
			return
		}
		costDescParts = append(costDescParts, fmt.Sprintf("%d个%s", c.Qty, c.Name))
	}
	costDesc := strings.Join(costDescParts, "、")

	// 抽奖并考虑“获得类型=1”
	var rewardItem string
	var rewardQty int64

	attempts := 0
	for attempts < 50 {
		attempts++
		item, qty := rollReward(rewards)
		if item == "" {
			continue
		}

		if item == "[货币]" {
			rewardItem = item
			rewardQty = qty
			break
		}

		// 检查特有物品
		itemConfig, hasConfig := config.Items[item]
		if hasConfig && itemConfig.ObtainType == 1 {
			var count int64
			tx.Model(&models.BackpackItem{}).Where("user_id = ? AND group_id = ? AND item_name = ?", event.UserID, event.GroupID, item).Count(&count)
			if count > 0 {
				continue // 已经获得过，重新抽
			}
		}

		rewardItem = item
		rewardQty = qty
		break
	}

	if rewardItem == "" {
		tx.Rollback()
		core.SendGroupMessage(conn, event.GroupID, core_game.AtSender(event.UserID)+"\n"+utils.Emoji("E29D84")+"抽奖出错了，未抽中有效物品！")
		return
	}

	// 发放奖励
	var rewardDesc string
	if rewardItem == "[货币]" {
		pet.Currency += rewardQty
		rewardDesc = fmt.Sprintf("%s×%d", config.Core.CoinName, rewardQty)
	} else {
		if err := core_game.AddBackpackItem(tx, event.UserID, event.GroupID, rewardItem, rewardQty); err != nil {
			tx.Rollback()
			return
		}
		rewardDesc = fmt.Sprintf("%s×%d", rewardItem, rewardQty)
	}

	if err := tx.Save(pet).Error; err != nil {
		tx.Rollback()
		return
	}

	tx.Commit()

	imageCQ := core_game.GetImageCQ(config.Images["抽奖"])

	replyText := fmt.Sprintf("%s阁下使用了%s，抽到了%s！",
		utils.Emoji("F09F8E81"), costDesc, rewardDesc)

	core.SendGroupMessage(conn, event.GroupID, core_game.AtSender(event.UserID)+imageCQ+"\n"+replyText)
}
