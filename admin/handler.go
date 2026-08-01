package admin

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"qq-pet-saas/config"
	"qq-pet-saas/database"
	"qq-pet-saas/models"
	"qq-pet-saas/security"
)

// BroadcastMessageFunc is a callback function set by the core package to send messages.
// This decouples the admin package from the core package, avoiding import cycles.
var BroadcastMessageFunc func(groupID int64, text string)

// RegisterRoutes registers the admin dashboard APIs and serves the static files
const (
	maxUploadBytes = 10 * 1024 * 1024
	maxImagePixels = 40_000_000
)

func RegisterRoutes(r *gin.Engine) {
	// Serve static files from embedded filesystem
	// Using fs.Sub to strip the "dist" prefix so /admin serves index.html directly
	subFS, err := fs.Sub(Assets, "dist")
	if err != nil {
		log.Fatalf("Failed to create sub-FS for embedded assets: %v", err)
	}
	r.StaticFS("/admin", http.FS(subFS))

	_, err = security.LoadCredentials()
	if err != nil {
		log.Fatalf("加载管理员凭据失败: %v", err)
	}

	sessions := NewSessionManager()
	auth := NewAuthHandler(sessions)
	authRoutes := r.Group("/api/admin/auth")
	{
		authRoutes.POST("/login", auth.Login)
		authRoutes.GET("/session", auth.Session)
	}

	protected := r.Group("/api/admin", RequireAdminSession(sessions))
	{
		protected.POST("/auth/logout", auth.Logout)
		protected.PUT("/auth/password", auth.ChangePassword)
	}

	// 管理页面可公开加载，但所有数据接口必须持有管理员登录会话。
	api := protected.Group("", ProtectConfigReads())
	{
		// Stats API
		api.GET("/stats", GetStats)

		// Pets APIs
		api.GET("/pets", GetPets)
		api.PUT("/pets/:id", UpdatePet)
		api.DELETE("/pets/:id", DeletePet)
		api.POST("/pets/:id/operate", OperatePet)

		// Inventory API
		api.POST("/items/give", GiveItem)

		// Group Switches APIs
		api.GET("/groups", GetGroups)
		api.PUT("/groups/:id", UpdateGroup)
		api.DELETE("/groups/:id", DeleteGroup)
		api.POST("/groups/sync", SyncGroups)

		// Compensation API
		api.POST("/compensation", DistributeCompensation)

		// Configs APIs
		api.GET("/configs", GetConfigs)
		api.PUT("/configs/system", UpdateSystemConfigs)
		api.PUT("/configs/commands", UpdateCommandConfigs)
		api.PUT("/configs/pet_species", UpdatePetSpeciesConfigs)
		api.PUT("/configs/items", UpdateItemConfigs)
		api.PUT("/configs/menus", UpdateMenuConfigs)
		api.PUT("/configs/shop_items", UpdateShopItemConfigs)
		api.PUT("/configs/work_settings", UpdateWorkSettingConfigs)
		api.PUT("/configs/images", UpdateImageConfigs)
		api.PUT("/configs/checkin_rewards", UpdateCheckinRewardConfigs)
		api.DELETE("/configs/:type/:key", DeleteConfig)
		api.POST("/configs/reload", ReloadConfigs)
		api.POST("/configs/reset", ResetConfigs)
		api.POST("/upload", UploadImage)

		// 配置中心 REST 接口（新版 Vue 后台使用，统一 {code,msg,data} 响应格式）。
		// 与上面的旧 /configs/* 接口并存，待前端迁移完成后再移除旧接口。
		RegisterConfigRoutes(api, NewConfigAPI(database.DB))
	}
}

// GetStats returns general statistics for the admin dashboard
func GetStats(c *gin.Context) {
	var totalPets int64
	database.DB.Model(&models.UserPet{}).Count(&totalPets)

	var totalUsers int64
	database.DB.Model(&models.UserPet{}).Distinct("user_id").Count(&totalUsers)

	// Status distribution
	type StatusCount struct {
		Status string `json:"status"`
		Count  int64  `json:"count"`
	}
	var statusCounts []StatusCount
	database.DB.Model(&models.UserPet{}).Select("status, count(*) as count").Group("status").Find(&statusCounts)

	statusDist := make(map[string]int64)
	for _, sc := range statusCounts {
		if sc.Status == "" {
			statusDist["未知"] = sc.Count
		} else {
			statusDist[sc.Status] = sc.Count
		}
	}

	// Top wealth (top 10)
	var topWealth []models.UserPet
	database.DB.Order("currency desc").Limit(10).Find(&topWealth)

	// Top growth (top 10)
	var topGrowth []models.UserPet
	database.DB.Order("growth desc").Limit(10).Find(&topGrowth)

	// Top affection (top 10)
	var topAffection []models.UserPet
	database.DB.Order("affection desc").Limit(10).Find(&topAffection)

	c.JSON(http.StatusOK, gin.H{
		"total_pets":    totalPets,
		"total_users":   totalUsers,
		"status_dist":   statusDist,
		"top_wealth":    topWealth,
		"top_growth":    topGrowth,
		"top_affection": topAffection,
	})
}

// GetPets returns a paginated and filtered list of user pets
func GetPets(c *gin.Context) {
	query := database.DB.Model(&models.UserPet{})

	if userIDStr := c.Query("user_id"); userIDStr != "" {
		if uid, err := strconv.ParseInt(userIDStr, 10, 64); err == nil {
			query = query.Where("user_id = ?", uid)
		}
	}
	if groupIDStr := c.Query("group_id"); groupIDStr != "" {
		if gid, err := strconv.ParseInt(groupIDStr, 10, 64); err == nil {
			query = query.Where("group_id = ?", gid)
		}
	}
	if name := c.Query("name"); name != "" {
		query = query.Where("name LIKE ?", "%"+name+"%")
	}
	if status := c.Query("status"); status != "" {
		query = query.Where("status = ?", status)
	}
	if petType := c.Query("pet_type"); petType != "" {
		query = query.Where("pet_type = ?", petType)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "统计宠物数据失败: " + err.Error()})
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 10
	}
	if limit > 100 {
		limit = 100
	}
	offset := (page - 1) * limit

	var pets []models.UserPet
	if err := query.Order("id desc").Offset(offset).Limit(limit).Find(&pets).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询宠物数据失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"total": total,
		"page":  page,
		"limit": limit,
		"data":  pets,
	})
}

// PetUpdateRequest holds fields that can be updated inline
type PetUpdateRequest struct {
	Name        *string `json:"name"`
	Status      *string `json:"status"`
	Mood        *string `json:"mood"`
	MoodPoints  *int    `json:"mood_points"`
	Affection   *int64  `json:"affection"`
	Growth      *int64  `json:"growth"`
	Health      *int64  `json:"health"`
	Wisdom      *int64  `json:"wisdom"`
	Strength    *int64  `json:"strength"`
	Defense     *int64  `json:"defense"`
	Hunger      *int64  `json:"hunger"`
	Currency    *int64  `json:"currency"`
	Family      *string `json:"family"`
	FamilyScore *int64  `json:"family_score"`
}

// UpdatePet updates the attributes of a specific pet
func UpdatePet(c *gin.Context) {
	id := c.Param("id")
	var pet models.UserPet
	if err := database.DB.First(&pet, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "未找到对应的宠物"})
		return
	}

	var req PetUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Update fields selectively if they are not nil
	if req.Name != nil {
		pet.Name = *req.Name
	}
	if req.Status != nil {
		pet.Status = *req.Status
	}
	if req.Mood != nil {
		pet.Mood = *req.Mood
	}
	if req.MoodPoints != nil {
		pet.MoodPoints = *req.MoodPoints
	}
	if req.Affection != nil {
		pet.Affection = *req.Affection
	}
	if req.Growth != nil {
		pet.Growth = *req.Growth
	}
	if req.Health != nil {
		pet.Health = *req.Health
	}
	if req.Wisdom != nil {
		pet.Wisdom = *req.Wisdom
	}
	if req.Strength != nil {
		pet.Strength = *req.Strength
	}
	if req.Defense != nil {
		pet.Defense = *req.Defense
	}
	if req.Hunger != nil {
		pet.Hunger = *req.Hunger
	}
	if req.Currency != nil {
		pet.Currency = *req.Currency
	}
	if req.Family != nil {
		pet.Family = *req.Family
	}
	if req.FamilyScore != nil {
		pet.FamilyScore = *req.FamilyScore
	}

	// Handle time fields if status transitions back to idle
	if req.Status != nil && *req.Status == "空闲" {
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
	}

	if err := database.DB.Save(&pet).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "保存数据失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "宠物属性更新成功", "data": pet})
}

// DeletePet deletes a pet's save record from database
func DeletePet(c *gin.Context) {
	id := c.Param("id")
	if err := database.DB.Delete(&models.UserPet{}, id).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "删除宠物失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "宠物存档已成功删除"})
}

// PetOperateRequest holds parameters for operational actions
type PetOperateRequest struct {
	Action string `json:"action"` // "revive", "recall", "clear_cooldown"
}

// OperatePet performs bulk operational actions on a pet
func OperatePet(c *gin.Context) {
	id := c.Param("id")
	var pet models.UserPet
	if err := database.DB.First(&pet, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "未找到对应的宠物"})
		return
	}

	var req PetOperateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	switch req.Action {
	case "revive":
		// 复活：恢复为空闲，生命值恢复到最大（配表最大值或缺省100），清空相关超时计时器
		pet.Status = "空闲"
		maxHealth := int64(100)
		if sp, exists := config.Pets[pet.PetType]; exists {
			maxHealth = sp.HealthMax
		}
		pet.Health = maxHealth
		pet.DyingTime = nil
		pet.EscapeTime = nil
		pet.LostTime = nil

	case "recall":
		// 召回：打断学习、工作、训练等各种耗时异步状态，直接置为空闲
		pet.Status = "空闲"
		pet.StudyTime = nil
		pet.StudyItem = ""
		pet.TrainTime = nil
		pet.TrainItem = ""
		pet.WorkTime = nil
		pet.WorkType = ""
		pet.FitnessTime = nil
		pet.FitnessItem = ""

	case "clear_cooldown":
		// 清除冷却：只清除计时器本身
		pet.StudyTime = nil
		pet.TrainTime = nil
		pet.WorkTime = nil
		pet.FitnessTime = nil
		pet.DyingTime = nil
		pet.EscapeTime = nil
		pet.LostTime = nil

	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "非法的操作指令"})
		return
	}

	if err := database.DB.Save(&pet).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "保存数据失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "操作执行成功", "data": pet})
}

// ItemGiveRequest holds inputs for sending backpack items
type ItemGiveRequest struct {
	UserID   int64  `json:"user_id"`
	GroupID  int64  `json:"group_id"`
	ItemName string `json:"item_name"`
	Quantity int64  `json:"quantity"`
}

// GiveItem distributes items to players or reduces item counts
func GiveItem(c *gin.Context) {
	var req ItemGiveRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.UserID == 0 || req.ItemName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "玩家QQ号 user_id 与 物品名称 item_name 不能为空"})
		return
	}

	var item models.BackpackItem
	err := database.DB.Where("user_id = ? AND group_id = ? AND item_name = ?", req.UserID, req.GroupID, req.ItemName).First(&item).Error
	if err == gorm.ErrRecordNotFound {
		if req.Quantity <= 0 {
			c.JSON(http.StatusOK, gin.H{"message": "数量未变动", "quantity": 0})
			return
		}
		item = models.BackpackItem{
			UserID:   req.UserID,
			GroupID:  req.GroupID,
			ItemName: req.ItemName,
			Quantity: req.Quantity,
		}
		if err := database.DB.Create(&item).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "创建背包记录失败: " + err.Error()})
			return
		}
	} else if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询背包失败: " + err.Error()})
		return
	} else {
		item.Quantity += req.Quantity
		if item.Quantity <= 0 {
			database.DB.Delete(&item)
			item.Quantity = 0
		} else {
			database.DB.Save(&item)
		}
	}

	c.JSON(http.StatusOK, gin.H{"message": "物品分发更新成功", "data": item})
}

// GetGroups returns list of all group switches
func GetGroups(c *gin.Context) {
	var list []models.GroupSwitch
	database.DB.Order("group_id asc").Find(&list)
	c.JSON(http.StatusOK, list)
}

// UpdateGroup updates or creates a group switch state
func UpdateGroup(c *gin.Context) {
	id := c.Param("id")
	gid, err := strconv.ParseInt(id, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "群号 ID 格式不正确"})
		return
	}

	var gSwitch models.GroupSwitch
	err = database.DB.Where("group_id = ?", gid).First(&gSwitch).Error
	if err == gorm.ErrRecordNotFound {
		gSwitch = models.GroupSwitch{GroupID: gid, IsActive: true}
	}

	type GroupUpdateRequest struct {
		GroupName *string `json:"group_name"`
		IsActive  *bool   `json:"is_active"`
	}
	var req GroupUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.GroupName != nil {
		gSwitch.GroupName = *req.GroupName
	}
	if req.IsActive != nil {
		gSwitch.IsActive = *req.IsActive
	}

	if err := database.DB.Save(&gSwitch).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "保存群组开关失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "群开关状态更新成功", "data": gSwitch})
}

// DeleteGroup deletes a group switch record
func DeleteGroup(c *gin.Context) {
	id := c.Param("id")
	if err := database.DB.Delete(&models.GroupSwitch{}, id).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "删除群组记录失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "群组记录已删除，将恢复默认全部开启状态"})
}

// SyncGroups syncs active groups from pet records to initialize the GroupSwitch list
func SyncGroups(c *gin.Context) {
	var groupIDs []int64
	database.DB.Model(&models.UserPet{}).Distinct("group_id").Pluck("group_id", &groupIDs)

	var addedCount int64
	for _, gid := range groupIDs {
		if gid == 0 {
			continue
		}
		var count int64
		database.DB.Model(&models.GroupSwitch{}).Where("group_id = ?", gid).Count(&count)
		if count == 0 {
			g := models.GroupSwitch{
				GroupID:   gid,
				GroupName: fmt.Sprintf("QQ群 (%d)", gid),
				IsActive:  true,
			}
			database.DB.Create(&g)
			addedCount++
		}
	}
	c.JSON(http.StatusOK, gin.H{"message": fmt.Sprintf("群组列表同步完成，自动搜集并初始化了 %d 个活跃群组", addedCount)})
}

// CompensationRequest holds fields for bulk compensations
type CompensationRequest struct {
	GroupID int64   `json:"group_id"` // 0 represents all groups
	UserIDs []int64 `json:"user_ids"` // Empty represents all players in the target group
	Coins   int64   `json:"coins"`
	Items   string  `json:"items"` // e.g. "饼干*5#抽奖券*10"
	Notice  string  `json:"notice"`
}

type compensationItem struct {
	Name     string
	Quantity int64
}

func parseCompensationItems(raw string) ([]compensationItem, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, nil
	}
	items := make([]compensationItem, 0)
	for _, part := range strings.Split(raw, "#") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		name := part
		quantity := int64(1)
		if strings.Contains(part, "*") {
			parts := strings.SplitN(part, "*", 2)
			name = strings.TrimSpace(parts[0])
			parsed, err := strconv.ParseInt(strings.TrimSpace(parts[1]), 10, 64)
			if err != nil || parsed == 0 {
				return nil, fmt.Errorf("物品 %q 的数量无效", part)
			}
			quantity = parsed
		}
		if name == "" {
			return nil, fmt.Errorf("物品名称不能为空")
		}
		items = append(items, compensationItem{Name: name, Quantity: quantity})
	}
	return items, nil
}

// DistributeCompensation distributes coins/items to multiple players and broadcasts group notification
func DistributeCompensation(c *gin.Context) {
	var req CompensationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	items, err := parseCompensationItems(req.Items)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 1. Query target pets
	query := database.DB.Model(&models.UserPet{})
	if req.GroupID != 0 {
		query = query.Where("group_id = ?", req.GroupID)
	}
	if len(req.UserIDs) > 0 {
		query = query.Where("user_id IN ?", req.UserIDs)
	}
	var targets []models.UserPet
	if err := query.Find(&targets).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取补偿目标用户失败: " + err.Error()})
		return
	}

	if len(targets) == 0 {
		c.JSON(http.StatusOK, gin.H{"message": "没有找到符合条件的补偿目标玩家，补偿未发放", "count": 0})
		return
	}
	groupRecipients := make(map[int64][]string)
	for _, target := range targets {
		groupRecipients[target.GroupID] = append(groupRecipients[target.GroupID], fmt.Sprintf("[CQ:at,qq=%d]", target.UserID))
	}

	// 2. Perform updates in a GORM transaction
	err = database.DB.Transaction(func(tx *gorm.DB) error {
		for _, target := range targets {
			// Update coins
			if req.Coins != 0 {
				target.Currency += req.Coins
				if target.Currency < 0 {
					target.Currency = 0
				}
			}

			// Save pet (this also triggers GORM hooks to sync currency to external INI if enabled!)
			if err := tx.Save(&target).Error; err != nil {
				return err
			}

			// Add items to backpack
			if len(items) > 0 {
				for _, item := range items {
					itemName := item.Name
					quantity := item.Quantity
					{
						var backpackItem models.BackpackItem
						err := tx.Where("user_id = ? AND group_id = ? AND item_name = ?", target.UserID, target.GroupID, itemName).First(&backpackItem).Error
						if err == gorm.ErrRecordNotFound {
							if quantity > 0 {
								backpackItem = models.BackpackItem{
									UserID:   target.UserID,
									GroupID:  target.GroupID,
									ItemName: itemName,
									Quantity: quantity,
								}
								if err := tx.Create(&backpackItem).Error; err != nil {
									return err
								}
							}
						} else if err == nil {
							backpackItem.Quantity += quantity
							if backpackItem.Quantity <= 0 {
								if err := tx.Delete(&backpackItem).Error; err != nil {
									return err
								}
							} else {
								if err := tx.Save(&backpackItem).Error; err != nil {
									return err
								}
							}
						} else {
							return err
						}
					}
				}
			}
		}
		return nil
	})

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "分发补偿过程中数据库写入失败: " + err.Error()})
		return
	}

	// 3. Broadcast Notice Group-by-Group
	if req.Notice != "" {
		for gid, compensatedPlayers := range groupRecipients {
			if gid == 0 {
				continue
			}

			playersStr := strings.Join(compensatedPlayers, " ")
			if len(req.UserIDs) == 0 {
				playersStr = "本群全体宠物玩家"
			}

			coinsStr := ""
			if req.Coins > 0 {
				coinsStr = fmt.Sprintf("\n- %d %s", req.Coins, config.Core.CoinName)
			}

			itemsStr := ""
			if req.Items != "" {
				itemsStr = fmt.Sprintf("\n- %s", strings.ReplaceAll(req.Items, "#", "、"))
			}

			noticeText := req.Notice
			noticeText = strings.ReplaceAll(noticeText, "[奖励玩家]", playersStr)
			noticeText = strings.ReplaceAll(noticeText, "[奖励货币]", coinsStr)
			noticeText = strings.ReplaceAll(noticeText, "[奖励物品]", itemsStr)
			noticeText = strings.ReplaceAll(noticeText, "[换行]", "\n")

			// Broadcast
			if BroadcastMessageFunc != nil {
				BroadcastMessageFunc(gid, noticeText)
			}
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"message": fmt.Sprintf("成功为 %d 名玩家宠物分发补偿，并已发送群消息广播", len(targets)),
		"count":   len(targets),
	})
}

// GetConfigs returns all table configurations in SQLite
func GetConfigs(c *gin.Context) {
	var sysConfigs []models.SystemConfig
	var cmdConfigs []models.CommandConfig
	var petConfigs []models.PetSpeciesConfig
	var itemConfigs []models.ItemConfig
	var shopConfigs []models.ShopItemConfig
	var workConfigs []models.WorkSettingConfig
	var menuConfigs []models.MenuConfig
	var imgConfigs []models.ImageConfig
	var checkinConfigs []models.CheckinRewardConfig

	database.DB.Order("key asc").Find(&sysConfigs)
	database.DB.Order("func_name asc").Find(&cmdConfigs)
	database.DB.Order("name asc").Find(&petConfigs)
	database.DB.Order("name asc").Find(&itemConfigs)
	database.DB.Order("id asc").Find(&shopConfigs)
	database.DB.Order("name asc").Find(&workConfigs)
	database.DB.Order("name asc").Find(&menuConfigs)
	database.DB.Order("name asc").Find(&imgConfigs)
	database.DB.Order("id asc").Find(&checkinConfigs)

	c.JSON(http.StatusOK, gin.H{
		"system_configs":         sysConfigs,
		"command_configs":        cmdConfigs,
		"pet_species_configs":    petConfigs,
		"item_configs":           itemConfigs,
		"shop_item_configs":      shopConfigs,
		"work_setting_configs":   workConfigs,
		"menu_configs":           menuConfigs,
		"image_configs":          imgConfigs,
		"checkin_reward_configs": checkinConfigs,
	})
}

// UpdateSystemConfigs bulk updates system key-value configurations
func UpdateSystemConfigs(c *gin.Context) {
	var req []models.SystemConfig
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	err := database.DB.Transaction(func(tx *gorm.DB) error {
		for _, item := range req {
			if err := tx.Save(&item).Error; err != nil {
				return err
			}
		}
		return nil
	})

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新配置失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "系统核心参数已更新至数据库"})
}

// UpdateCommandConfigs bulk updates customized game commands
func UpdateCommandConfigs(c *gin.Context) {
	var req []models.CommandConfig
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	err := database.DB.Transaction(func(tx *gorm.DB) error {
		for _, item := range req {
			if err := tx.Save(&item).Error; err != nil {
				return err
			}
		}
		return nil
	})

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新配置失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "自定义指令设置已更新至数据库"})
}

// UpdatePetSpeciesConfigs bulk updates pet species and evolution configurations
func UpdatePetSpeciesConfigs(c *gin.Context) {
	var req []models.PetSpeciesConfig
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	err := database.DB.Transaction(func(tx *gorm.DB) error {
		for _, item := range req {
			if err := tx.Save(&item).Error; err != nil {
				return err
			}
		}
		return nil
	})

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新配置失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "宠物演变配置已更新至数据库"})
}

// UpdateItemConfigs bulk updates in-game item configurations
func UpdateItemConfigs(c *gin.Context) {
	var req []models.ItemConfig
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	err := database.DB.Transaction(func(tx *gorm.DB) error {
		for _, item := range req {
			if err := tx.Save(&item).Error; err != nil {
				return err
			}
		}
		return nil
	})

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新配置失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "道具信息已更新至数据库"})
}

// UpdateMenuConfigs bulk updates game commands menus and replies
func UpdateMenuConfigs(c *gin.Context) {
	var req []models.MenuConfig
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	err := database.DB.Transaction(func(tx *gorm.DB) error {
		for _, item := range req {
			if err := tx.Save(&item).Error; err != nil {
				return err
			}
		}
		return nil
	})

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新配置失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "菜单回复文案已更新至数据库"})
}

// UpdateShopItemConfigs bulk updates shop shelves items
func UpdateShopItemConfigs(c *gin.Context) {
	var req []models.ShopItemConfig
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	err := database.DB.Transaction(func(tx *gorm.DB) error {
		for _, item := range req {
			if err := tx.Save(&item).Error; err != nil {
				return err
			}
		}
		return nil
	})

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新配置失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "商店货架配置已更新至数据库"})
}

// UpdateWorkSettingConfigs bulk updates in-game挂机打工 settings
func UpdateWorkSettingConfigs(c *gin.Context) {
	var req []models.WorkSettingConfig
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	err := database.DB.Transaction(func(tx *gorm.DB) error {
		for _, item := range req {
			if err := tx.Save(&item).Error; err != nil {
				return err
			}
		}
		return nil
	})

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新配置失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "挂机打工配置已更新至数据库"})
}

// UpdateImageConfigs bulk updates image mapping files
func UpdateImageConfigs(c *gin.Context) {
	var req []models.ImageConfig
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	err := database.DB.Transaction(func(tx *gorm.DB) error {
		for _, item := range req {
			if err := tx.Save(&item).Error; err != nil {
				return err
			}
		}
		return nil
	})

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新配置失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "核心图片映射已更新至数据库"})
}

// UpdateCheckinRewardConfigs bulk updates Monday-Sunday / Day 1-7 checkin rewards
func UpdateCheckinRewardConfigs(c *gin.Context) {
	var req []models.CheckinRewardConfig
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	err := database.DB.Transaction(func(tx *gorm.DB) error {
		for _, item := range req {
			if err := tx.Save(&item).Error; err != nil {
				return err
			}
		}
		return nil
	})

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新配置失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "每日签到奖励配置已更新至数据库"})
}

// DeleteConfig deletes a specific entity from configuration tables
func DeleteConfig(c *gin.Context) {
	cfgType := c.Param("type")
	key := c.Param("key")

	var err error
	switch cfgType {
	case "pet_species":
		err = database.DB.Where("name = ?", key).Delete(&models.PetSpeciesConfig{}).Error
	case "item":
		err = database.DB.Where("name = ?", key).Delete(&models.ItemConfig{}).Error
	case "shop_item":
		if id, parseErr := strconv.ParseUint(key, 10, 32); parseErr == nil {
			err = database.DB.Where("id = ?", uint(id)).Delete(&models.ShopItemConfig{}).Error
		} else {
			err = database.DB.Where("name = ?", key).Delete(&models.ShopItemConfig{}).Error
		}
	case "work_setting":
		err = database.DB.Where("name = ?", key).Delete(&models.WorkSettingConfig{}).Error
	case "menu":
		err = database.DB.Where("name = ?", key).Delete(&models.MenuConfig{}).Error
	case "command":
		err = database.DB.Where("func_name = ?", key).Delete(&models.CommandConfig{}).Error
	case "image":
		err = database.DB.Where("name = ?", key).Delete(&models.ImageConfig{}).Error
	case "checkin_reward":
		if id, parseErr := strconv.ParseUint(key, 10, 32); parseErr == nil {
			err = database.DB.Where("id = ?", uint(id)).Delete(&models.CheckinRewardConfig{}).Error
		}
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "未知的配置类型"})
		return
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "删除配置项失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "配置项删除成功"})
}

// ReloadConfigs loads current SQLite config records into in-memory struct fields
func ReloadConfigs(c *gin.Context) {
	if err := config.LoadAllConfigsFromDB(database.DB); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "加载最新配置覆盖内存失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "所有配置热重载同步成功"})
}

// ResetConfigs drops all user modified config records and seeds from SeedFS
func ResetConfigs(c *gin.Context) {
	if err := config.ResetConfigsFromSeed(database.DB); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "恢复出厂种子设置失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "配置数据已重置为系统默认出厂配置并重载成功"})
}

// UploadImage handles file uploads and saves the images to the static images directory
func UploadImage(c *gin.Context) {
	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "获取上传文件失败: " + err.Error()})
		return
	}

	if file.Size > maxUploadBytes {
		c.JSON(http.StatusBadRequest, gin.H{"error": "文件大小超出 10MB 限制"})
		return
	}

	source, err := file.Open()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "读取上传文件失败: " + err.Error()})
		return
	}
	ext, validateErr := validateImageUpload(source)
	_ = source.Close()
	if validateErr != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "图片内容校验失败: " + validateErr.Error()})
		return
	}

	// 由真实内容确定扩展名，避免伪造文件名绕过校验。
	if ext != ".jpg" && ext != ".jpeg" && ext != ".png" && ext != ".gif" && ext != ".webp" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "只支持 JPG, JPEG, PNG, GIF, WEBP 格式的图片"})
		return
	}

	// 保证目录存在
	uploadRelDir := filepath.Join("上传")

	// 同时保存到两个地方，确保无论是 go:embed / config 找绝对路径，还是 Static 服务拉取，都能访问到
	destDir1 := filepath.Join(".", "图片", uploadRelDir)
	destDir2 := filepath.Join(config.GlobalConfigPath, "图片", uploadRelDir)

	if err := os.MkdirAll(destDir1, 0755); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建上传目录失败: " + err.Error()})
		return
	}
	if err := os.MkdirAll(destDir2, 0755); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建配置图片目录失败: " + err.Error()})
		return
	}

	// 使用时间戳构造唯一文件名，避免重名覆盖
	uniqueName := fmt.Sprintf("%d%s", time.Now().UnixNano(), ext)

	destPath1 := filepath.Join(destDir1, uniqueName)
	destPath2 := filepath.Join(destDir2, uniqueName)

	// 保存到第一个目录
	if err := c.SaveUploadedFile(file, destPath1); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "保存上传文件失败: " + err.Error()})
		return
	}

	if destPath1 != destPath2 {
		data, readErr := os.ReadFile(destPath1)
		if readErr != nil {
			_ = os.Remove(destPath1)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "读取上传文件失败: " + readErr.Error()})
			return
		}
		if writeErr := os.WriteFile(destPath2, data, 0644); writeErr != nil {
			_ = os.Remove(destPath1)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "同步上传文件失败: " + writeErr.Error()})
			return
		}
	}

	// 返回给前端的路径格式是 "上传\文件名.ext"，以便与原有配置的路径格式一致
	finalPath := filepath.Join(uploadRelDir, uniqueName)

	c.JSON(http.StatusOK, gin.H{
		"message": "图片上传成功",
		"path":    finalPath,
		"url":     "/images/上传/" + uniqueName,
	})
}

func validateImageUpload(reader io.Reader) (string, error) {
	data, err := io.ReadAll(io.LimitReader(reader, maxUploadBytes+1))
	if err != nil {
		return "", err
	}
	if len(data) == 0 || len(data) > maxUploadBytes {
		return "", errors.New("文件为空或超过大小限制")
	}

	if width, height, ok := webpDimensions(data); ok {
		if int64(width)*int64(height) > maxImagePixels {
			return "", errors.New("图片像素超出限制")
		}
		return ".webp", nil
	}

	config, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return "", errors.New("仅支持 JPEG、PNG、GIF、WebP 图片")
	}
	if config.Width <= 0 || config.Height <= 0 || int64(config.Width)*int64(config.Height) > maxImagePixels {
		return "", errors.New("图片尺寸或像素超出限制")
	}
	switch format {
	case "jpeg":
		return ".jpg", nil
	case "png":
		return ".png", nil
	case "gif":
		return ".gif", nil
	default:
		return "", errors.New("不支持的图片格式")
	}
}

func webpDimensions(data []byte) (int, int, bool) {
	if len(data) < 30 || string(data[:4]) != "RIFF" || string(data[8:12]) != "WEBP" {
		return 0, 0, false
	}
	switch string(data[12:16]) {
	case "VP8X":
		width := 1 + int(data[24]) + int(data[25])<<8 + int(data[26])<<16
		height := 1 + int(data[27]) + int(data[28])<<8 + int(data[29])<<16
		return width, height, true
	case "VP8 ":
		if data[23] != 0x9d || data[24] != 0x01 || data[25] != 0x2a {
			return 0, 0, false
		}
		width := int(data[26]) | int(data[27]&0x3f)<<8
		height := int(data[28]) | int(data[29]&0x3f)<<8
		return width, height, width > 0 && height > 0
	case "VP8L":
		if data[20] != 0x2f {
			return 0, 0, false
		}
		bits := uint32(data[21]) | uint32(data[22])<<8 | uint32(data[23])<<16 | uint32(data[24])<<24
		width := int(bits&0x3fff) + 1
		height := int((bits>>14)&0x3fff) + 1
		return width, height, true
	default:
		return 0, 0, false
	}
}
