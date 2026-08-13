package admin

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	appconfig "qq-pet-saas/config"
	"qq-pet-saas/models"
)

type ContentAPI struct{ DB *gorm.DB }

func RegisterContentRoutes(group *gin.RouterGroup, db *gorm.DB) {
	api := &ContentAPI{DB: db}
	group.POST("/content/shop-items/bulk", api.bulkShopItems)
	group.POST("/content/items/bulk", api.bulkItems)
	group.GET("/settings/game", api.getGameSettings)
	group.PUT("/settings/game", api.saveGameSettings)
	group.PUT("/content/events/:key", api.saveEventBundle)
	group.DELETE("/content/events/:key", api.deleteEventBundle)
}

type eventBundleRequest struct {
	Event   models.LiveEventConfig     `json:"event"`
	Rewards []models.RewardTrackConfig `json:"rewards"`
}

func (api *ContentAPI) saveEventBundle(c *gin.Context) {
	var request eventBundleRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		Error(c, codeInvalidPayload, "活动配置格式无效")
		return
	}
	key := strings.TrimSpace(c.Param("key"))
	if key == "" || request.Event.Key != key {
		Error(c, codeInvalidPayload, "活动键与请求地址不一致")
		return
	}
	for index := range request.Rewards {
		request.Rewards[index].EventKey = key
	}
	eventPayload, _ := json.Marshal([]models.LiveEventConfig{request.Event})
	if err := validateDomainConfig("live_events", eventPayload); err != nil {
		Error(c, codeInvalidPayload, err.Error())
		return
	}
	rewardPayload, _ := json.Marshal(request.Rewards)
	if err := validateDomainConfig("reward_tracks", rewardPayload); err != nil {
		Error(c, codeInvalidPayload, err.Error())
		return
	}
	err := api.DB.Transaction(func(tx *gorm.DB) error {
		if err := validateRewardItems(tx, rewardPayload); err != nil {
			return err
		}
		if request.Event.Active {
			var conflicting models.LiveEventConfig
			err := tx.Where("key <> ? AND active = ? AND starts_at < ? AND ends_at > ?", key, true, request.Event.EndsAt, request.Event.StartsAt).First(&conflicting).Error
			if err == nil {
				return configValidationError{fmt.Sprintf("活动 %s 与 %s 时间重叠", key, conflicting.Key)}
			}
			if err != nil && err != gorm.ErrRecordNotFound {
				return err
			}
		}
		if err := tx.Save(&request.Event).Error; err != nil {
			return err
		}
		if err := tx.Where("event_key = ?", key).Delete(&models.RewardTrackConfig{}).Error; err != nil {
			return err
		}
		for index := range request.Rewards {
			request.Rewards[index].ID = 0
			if err := tx.Create(&request.Rewards[index]).Error; err != nil {
				return err
			}
		}
		return appconfig.MarkConfigSaved(tx)
	})
	if err != nil {
		if validation, ok := err.(configValidationError); ok {
			Error(c, codeInvalidPayload, validation.Error())
			return
		}
		Error(c, codeInternalError, "保存活动失败: "+err.Error())
		return
	}
	Success(c, gin.H{"event": request.Event, "rewards": request.Rewards})
}

func (api *ContentAPI) deleteEventBundle(c *gin.Context) {
	key := strings.TrimSpace(c.Param("key"))
	if key == "" {
		Error(c, codeInvalidPayload, "活动键不能为空")
		return
	}
	var deletedRewards int64
	err := api.DB.Transaction(func(tx *gorm.DB) error {
		result := tx.Where("event_key = ?", key).Delete(&models.RewardTrackConfig{})
		if result.Error != nil {
			return result.Error
		}
		deletedRewards = result.RowsAffected
		result = tx.Where("key = ?", key).Delete(&models.LiveEventConfig{})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return configValidationError{"活动不存在"}
		}
		return appconfig.MarkConfigSaved(tx)
	})
	if err != nil {
		if validation, ok := err.(configValidationError); ok {
			Error(c, codeInvalidPayload, validation.Error())
			return
		}
		Error(c, codeInternalError, "删除活动失败: "+err.Error())
		return
	}
	Success(c, gin.H{"deleted_rewards": deletedRewards})
}

type gameSettingMeta struct {
	Key         string `json:"key"`
	Label       string `json:"label"`
	Group       string `json:"group"`
	Type        string `json:"type"`
	Unit        string `json:"unit,omitempty"`
	Description string `json:"description"`
	Value       any    `json:"value"`
}

var gameSettingCatalog = []gameSettingMeta{
	{Key: "Core.InitialPets", Label: "初始可领养宠物", Group: "基础设置", Type: "list", Description: "新玩家可以选择的初始宠物名称。"},
	{Key: "Core.CoinName", Label: "货币名称", Group: "基础设置", Type: "text", Description: "聊天回复和商店中展示的货币称呼。"},
	{Key: "Core.InitialCoin", Label: "初始货币", Group: "基础设置", Type: "number", Unit: "枚", Description: "新玩家建立存档时获得的基础货币。"},
	{Key: "Core.RenameCost", Label: "宠物改名费用", Group: "基础设置", Type: "number", Unit: "枚", Description: "玩家修改宠物名称时扣除的货币。"},
	{Key: "Core.ImageHost", Label: "图片服务地址", Group: "高级对接", Type: "text", Description: "OneBot 访问图片时使用的公网地址，留空使用本地文件。"},
	{Key: "Core.CheckinLike", Label: "每日陪伴点赞", Group: "通知能力", Type: "boolean", Description: "每日陪伴完成后是否请求平台点赞能力。"},
	{Key: "Interaction.WashGrowth", Label: "洗澡成长", Group: "陪伴互动", Type: "number", Unit: "点", Description: "完成洗澡互动获得的成长值。"},
	{Key: "Interaction.WashAffection", Label: "洗澡羁绊", Group: "陪伴互动", Type: "number", Unit: "点", Description: "完成洗澡互动获得的羁绊值。"},
	{Key: "Interaction.WalkGrowth", Label: "散步成长", Group: "陪伴互动", Type: "number", Unit: "点", Description: "完成散步互动获得的成长值。"},
	{Key: "Interaction.WalkAffection", Label: "散步羁绊", Group: "陪伴互动", Type: "number", Unit: "点", Description: "完成散步互动获得的羁绊值。"},
	{Key: "Interaction.TouchGrowth", Label: "摸头成长", Group: "陪伴互动", Type: "number", Unit: "点", Description: "摸头互动获得的成长值。"},
	{Key: "Interaction.TouchAffection", Label: "摸头羁绊", Group: "陪伴互动", Type: "number", Unit: "点", Description: "摸头互动获得的羁绊值。"},
	{Key: "Interaction.GiftLimit", Label: "每日送礼次数", Group: "陪伴互动", Type: "number", Unit: "次", Description: "单个玩家每天可以送礼的次数。"},
	{Key: "Interaction.BuyLimit", Label: "单次购买上限", Group: "商店背包", Type: "number", Unit: "件", Description: "一次商店命令允许购买的最大数量。"},
	{Key: "Interaction.SellNoPriceGrowth", Label: "无售价物品回收成长", Group: "商店背包", Type: "number", Unit: "点", Description: "回收没有售价的物品时给予的成长补偿。"},
}

func settingMetaByKey(key string) (gameSettingMeta, bool) {
	for _, meta := range gameSettingCatalog {
		if meta.Key == key {
			return meta, true
		}
	}
	return gameSettingMeta{}, false
}

func parseSettingValue(meta gameSettingMeta, raw string) any {
	switch meta.Type {
	case "boolean":
		value := strings.ToLower(strings.TrimSpace(raw))
		return value == "1" || value == "true" || value == "yes" || value == "on"
	case "number":
		value, _ := strconv.ParseFloat(strings.TrimSpace(raw), 64)
		return value
	case "list":
		parts := strings.FieldsFunc(raw, func(r rune) bool { return r == ',' || r == '，' || r == '、' })
		result := make([]string, 0, len(parts))
		for _, part := range parts {
			if value := strings.TrimSpace(part); value != "" {
				result = append(result, value)
			}
		}
		return result
	default:
		return raw
	}
}

func (api *ContentAPI) getGameSettings(c *gin.Context) {
	rows := make([]models.SystemConfig, 0)
	if err := api.DB.Find(&rows).Error; err != nil {
		Error(c, codeInternalError, "读取游戏参数失败: "+err.Error())
		return
	}
	values := make(map[string]string, len(rows))
	for _, row := range rows {
		values[row.Key] = row.Value
	}
	result := make([]gameSettingMeta, 0, len(gameSettingCatalog))
	for _, catalog := range gameSettingCatalog {
		raw, exists := values[catalog.Key]
		if !exists {
			continue
		}
		catalog.Value = parseSettingValue(catalog, raw)
		result = append(result, catalog)
	}
	Success(c, result)
}

type gameSettingUpdate struct {
	Key   string          `json:"key"`
	Value json.RawMessage `json:"value"`
}

func serializeSettingValue(meta gameSettingMeta, raw json.RawMessage) (string, error) {
	switch meta.Type {
	case "boolean":
		var value bool
		if err := json.Unmarshal(raw, &value); err != nil {
			return "", fmt.Errorf("%s必须使用开关值", meta.Label)
		}
		return strconv.FormatBool(value), nil
	case "number":
		var value float64
		if err := json.Unmarshal(raw, &value); err != nil {
			return "", fmt.Errorf("%s必须是数字", meta.Label)
		}
		return strconv.FormatFloat(value, 'f', -1, 64), nil
	case "list":
		var values []string
		if err := json.Unmarshal(raw, &values); err != nil {
			return "", fmt.Errorf("%s必须是文本列表", meta.Label)
		}
		clean := make([]string, 0, len(values))
		for _, value := range values {
			if value = strings.TrimSpace(value); value != "" {
				clean = append(clean, value)
			}
		}
		return strings.Join(clean, ","), nil
	default:
		var value string
		if err := json.Unmarshal(raw, &value); err != nil {
			return "", fmt.Errorf("%s必须是文本", meta.Label)
		}
		return strings.TrimSpace(value), nil
	}
}

func (api *ContentAPI) saveGameSettings(c *gin.Context) {
	var request []gameSettingUpdate
	if err := c.ShouldBindJSON(&request); err != nil || len(request) == 0 {
		Error(c, codeInvalidPayload, "请提交至少一个游戏参数")
		return
	}
	err := api.DB.Transaction(func(tx *gorm.DB) error {
		for _, row := range request {
			meta, exists := settingMetaByKey(row.Key)
			if !exists {
				return configValidationError{"不允许修改的游戏参数: " + row.Key}
			}
			value, err := serializeSettingValue(meta, row.Value)
			if err != nil {
				return configValidationError{err.Error()}
			}
			if err = tx.Save(&models.SystemConfig{Key: row.Key, Value: value}).Error; err != nil {
				return err
			}
		}
		return appconfig.MarkConfigSaved(tx)
	})
	if err != nil {
		if validation, ok := err.(configValidationError); ok {
			Error(c, codeInvalidPayload, validation.Error())
			return
		}
		Error(c, codeInternalError, "保存游戏参数失败: "+err.Error())
		return
	}
	api.getGameSettings(c)
}

type bulkShopRequest struct {
	IDs    []uint `json:"ids"`
	Action string `json:"action"`
	Value  *int64 `json:"value"`
}

func (api *ContentAPI) bulkShopItems(c *gin.Context) {
	var request bulkShopRequest
	if err := c.ShouldBindJSON(&request); err != nil || len(request.IDs) == 0 {
		Error(c, codeInvalidPayload, "请选择至少一个商品")
		return
	}
	if request.Action != "delete" && request.Action != "restock" && request.Action != "set_target" {
		Error(c, codeInvalidPayload, "未知的批量操作")
		return
	}
	if request.Action == "set_target" && (request.Value == nil || *request.Value < -1) {
		Error(c, codeInvalidPayload, "目标库存必须大于等于 -1")
		return
	}
	var updated int64
	err := api.DB.Transaction(func(tx *gorm.DB) error {
		query := tx.Model(&models.ShopItemConfig{}).Where("id IN ?", request.IDs)
		var result *gorm.DB
		switch request.Action {
		case "delete":
			result = query.Delete(&models.ShopItemConfig{})
		case "restock":
			result = query.Update("stock", gorm.Expr("restock_target"))
		case "set_target":
			result = query.Update("restock_target", *request.Value)
		}
		if result.Error != nil {
			return result.Error
		}
		updated = result.RowsAffected
		return appconfig.MarkConfigSaved(tx)
	})
	if err != nil {
		Error(c, codeInternalError, "批量更新商品失败: "+err.Error())
		return
	}
	var rows []models.ShopItemConfig
	if err = api.DB.Order("id asc").Find(&rows).Error; err != nil {
		Error(c, codeInternalError, "商品已更新，但刷新列表失败")
		return
	}
	Success(c, gin.H{"updated": updated, "items": rows})
}

type bulkItemRequest struct {
	Names  []string `json:"names"`
	Action string   `json:"action"`
	Status string   `json:"status"`
}

func (api *ContentAPI) bulkItems(c *gin.Context) {
	var request bulkItemRequest
	if err := c.ShouldBindJSON(&request); err != nil || len(request.Names) == 0 {
		Error(c, codeInvalidPayload, "请选择至少一个物品")
		return
	}
	for index := range request.Names {
		request.Names[index] = strings.TrimSpace(request.Names[index])
		if request.Names[index] == "" {
			Error(c, codeInvalidPayload, "物品名称不能为空")
			return
		}
	}
	validStatus := map[string]bool{"active": true, "limited": true, "hidden": true, "disabled": true}
	if request.Action != "delete" && request.Action != "set_status" {
		Error(c, codeInvalidPayload, "未知的批量操作")
		return
	}
	if request.Action == "set_status" && !validStatus[request.Status] {
		Error(c, codeInvalidPayload, "物品状态必须为 active、limited、hidden 或 disabled")
		return
	}
	var updated int64
	err := api.DB.Transaction(func(tx *gorm.DB) error {
		if request.Action == "delete" {
			if reason, err := itemDeleteBlocker(tx, request.Names); err != nil {
				return err
			} else if reason != "" {
				return configValidationError{reason}
			}
			result := tx.Where("name IN ?", request.Names).Delete(&models.ItemConfig{})
			updated = result.RowsAffected
			if result.Error != nil {
				return result.Error
			}
		} else {
			result := tx.Model(&models.ItemConfig{}).Where("name IN ?", request.Names).Update("status", request.Status)
			updated = result.RowsAffected
			if result.Error != nil {
				return result.Error
			}
		}
		return appconfig.MarkConfigSaved(tx)
	})
	if err != nil {
		if validation, ok := err.(configValidationError); ok {
			Error(c, codeInvalidPayload, validation.Error())
			return
		}
		Error(c, codeInternalError, "批量更新物品失败: "+err.Error())
		return
	}
	var rows []models.ItemConfig
	if err = api.DB.Order("name asc").Find(&rows).Error; err != nil {
		Error(c, codeInternalError, "物品已更新，但刷新列表失败")
		return
	}
	Success(c, gin.H{"updated": updated, "items": rows})
}

func itemDeleteBlocker(tx *gorm.DB, names []string) (string, error) {
	checks := []struct {
		model  interface{}
		column string
		label  string
	}{
		{&models.ShopItemConfig{}, "name", "商店"},
		{&models.RewardTrackConfig{}, "item_name", "活动奖励"},
		{&models.BackpackItem{}, "item_name", "旧版玩家背包"},
		{&models.GlobalInventoryItem{}, "item_name", "玩家背包"},
	}
	for _, check := range checks {
		if !tx.Migrator().HasTable(check.model) {
			continue
		}
		var count int64
		if err := tx.Model(check.model).Where(fmt.Sprintf("%s IN ?", check.column), names).Count(&count).Error; err != nil {
			return "", err
		}
		if count > 0 {
			return fmt.Sprintf("所选物品仍被%s引用，请改为隐藏或停用", check.label), nil
		}
	}
	return "", nil
}
