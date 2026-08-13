package admin

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	appconfig "qq-pet-saas/config"
	"qq-pet-saas/models"
)

type configValidationError struct{ message string }

func (err configValidationError) Error() string { return err.message }

func validateDomainConfig(name string, payload json.RawMessage) error {
	switch name {
	case "commands":
		var rows []models.CommandConfig
		if err := json.Unmarshal(payload, &rows); err != nil {
			return err
		}
		seen := make(map[string]string)
		for index, row := range rows {
			if strings.TrimSpace(row.FuncName) == "" || strings.TrimSpace(row.Command) == "" || strings.TrimSpace(row.DisplayName) == "" || strings.TrimSpace(row.Category) == "" || strings.TrimSpace(row.Description) == "" {
				return configValidationError{fmt.Sprintf("第 %d 个命令的信息不完整", index+1)}
			}
			if !row.Enabled {
				continue
			}
			trigger := strings.TrimSpace(row.Command)
			if previous, exists := seen[trigger]; exists {
				return configValidationError{fmt.Sprintf("触发词“%s”同时用于%s和%s", trigger, previous, row.DisplayName)}
			}
			seen[trigger] = row.DisplayName
		}
	case "live_events":
		var rows []models.LiveEventConfig
		if err := json.Unmarshal(payload, &rows); err != nil {
			return err
		}
		for index := range rows {
			row := rows[index]
			if strings.TrimSpace(row.Key) == "" || strings.TrimSpace(row.Name) == "" || strings.TrimSpace(row.Region) == "" || !row.StartsAt.Before(row.EndsAt) {
				return configValidationError{fmt.Sprintf("第 %d 个活动的名称、区域或时间无效", index+1)}
			}
			var choices []string
			if err := json.Unmarshal([]byte(row.StoryChoices), &choices); err != nil || len(choices) != 3 {
				return configValidationError{fmt.Sprintf("活动 %s 必须配置三个故事选项", row.Key)}
			}
			for _, choice := range choices {
				if strings.TrimSpace(choice) == "" {
					return configValidationError{fmt.Sprintf("活动 %s 的故事选项不能为空", row.Key)}
				}
			}
			for otherIndex := 0; otherIndex < index; otherIndex++ {
				other := rows[otherIndex]
				if row.Active && other.Active && row.StartsAt.Before(other.EndsAt) && other.StartsAt.Before(row.EndsAt) {
					return configValidationError{fmt.Sprintf("活动 %s 与 %s 时间重叠", row.Key, other.Key)}
				}
			}
		}
	case "reward_tracks":
		var rows []models.RewardTrackConfig
		if err := json.Unmarshal(payload, &rows); err != nil {
			return err
		}
		seen := make(map[string]struct{}, len(rows))
		for index, row := range rows {
			if strings.TrimSpace(row.EventKey) == "" || strings.TrimSpace(row.ItemName) == "" || row.Milestone <= 0 || row.Quantity <= 0 {
				return configValidationError{fmt.Sprintf("第 %d 个奖励里程碑无效", index+1)}
			}
			key := fmt.Sprintf("%s:%d:%s", row.EventKey, row.Milestone, row.ItemName)
			if _, exists := seen[key]; exists {
				return configValidationError{fmt.Sprintf("同一里程碑不能重复配置同一物品: %s", key)}
			}
			seen[key] = struct{}{}
		}
	case "growth_roles":
		var rows []models.GrowthRoleConfig
		if err := json.Unmarshal(payload, &rows); err != nil {
			return err
		}
		seen := make(map[string]struct{}, len(rows))
		for index, row := range rows {
			if strings.TrimSpace(row.Name) == "" || strings.TrimSpace(row.Description) == "" || strings.TrimSpace(row.Skill1) == "" || strings.TrimSpace(row.Skill2) == "" || strings.TrimSpace(row.Skill3) == "" {
				return configValidationError{fmt.Sprintf("第 %d 个宠物定位信息不完整", index+1)}
			}
			if _, exists := seen[row.Name]; exists {
				return configValidationError{fmt.Sprintf("宠物定位名称重复: %s", row.Name)}
			}
			seen[row.Name] = struct{}{}
		}
	case "growth_stances":
		var rows []models.GrowthStanceConfig
		if err := json.Unmarshal(payload, &rows); err != nil {
			return err
		}
		for index, row := range rows {
			if strings.TrimSpace(row.Name) == "" || strings.TrimSpace(row.Description) == "" {
				return configValidationError{fmt.Sprintf("第 %d 个远征姿态信息不完整", index+1)}
			}
		}
	case "personality_rules":
		var rows []models.PersonalityRuleConfig
		if err := json.Unmarshal(payload, &rows); err != nil {
			return err
		}
		validDimensions := map[string]bool{"care": true, "explore": true, "support": true}
		for index, row := range rows {
			if strings.TrimSpace(row.Name) == "" || strings.TrimSpace(row.Description) == "" || !validDimensions[row.Dimension] || row.MinThreshold <= 0 {
				return configValidationError{fmt.Sprintf("第 %d 个性格形成规则无效", index+1)}
			}
		}
	case "codex_catalog":
		var rows []models.CodexCatalogConfig
		if err := json.Unmarshal(payload, &rows); err != nil {
			return err
		}
		seen := make(map[string]struct{}, len(rows))
		for index, row := range rows {
			if strings.TrimSpace(row.Category) == "" || strings.TrimSpace(row.EntryKey) == "" || strings.TrimSpace(row.Region) == "" {
				return configValidationError{fmt.Sprintf("第 %d 个图鉴条目信息不完整", index+1)}
			}
			key := row.Category + ":" + row.EntryKey
			if _, exists := seen[key]; exists {
				return configValidationError{fmt.Sprintf("图鉴条目重复: %s", key)}
			}
			seen[key] = struct{}{}
		}
	}
	return nil
}

func validateRewardItems(db *gorm.DB, payload json.RawMessage) error {
	var rows []models.RewardTrackConfig
	if err := json.Unmarshal(payload, &rows); err != nil {
		return err
	}
	modernItems := map[string]bool{"调查记录": true, "木材": true, "林地样本": true, "古代零件": true, "生态样本": true, "陪伴印记": true}
	for _, row := range rows {
		if modernItems[row.ItemName] {
			continue
		}
		var item models.ItemConfig
		if err := db.First(&item, "name = ?", row.ItemName).Error; err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if strings.TrimSpace(item.Name) == "" {
			return configValidationError{fmt.Sprintf("奖励物品不存在: %s", row.ItemName)}
		}
		if item.Status == "hidden" || item.Status == "disabled" {
			return configValidationError{fmt.Sprintf("奖励物品当前不可发放: %s", row.ItemName)}
		}
	}
	return nil
}

// 配置接口使用的业务错误码。HTTP 状态码始终为 200，错误通过 code 字段表达。
const (
	codeInvalidPayload = 4000
	codeSchemaNotFound = 4004
	codeInternalError  = 5000
)

// configSchema 描述一张可被后台管理的配置表。
//
// fetch 与 save 都是强类型闭包：每个 schema 绑定到具体的 models 结构体，
// 因此表名与列名永远来自代码而不是请求参数，杜绝了拼接表名带来的注入面。
type configSchema struct {
	// orderBy 是列表返回时的排序列，保证前端展示顺序稳定。
	orderBy string
	fetch   func(db *gorm.DB, orderBy string) (interface{}, error)
	save    func(tx *gorm.DB, payload json.RawMessage) error
}

// newConfigSchema 为某个配置模型生成读写闭包。
func newConfigSchema[T any](orderBy string) configSchema {
	return configSchema{
		orderBy: orderBy,
		fetch: func(db *gorm.DB, orderBy string) (interface{}, error) {
			// 初始化为空切片而不是 nil，确保空表序列化为 [] 而不是 null。
			rows := make([]T, 0)
			if err := db.Order(orderBy).Find(&rows).Error; err != nil {
				return nil, err
			}
			return rows, nil
		},
		save: func(tx *gorm.DB, payload json.RawMessage) error {
			var rows []T
			if err := json.Unmarshal(payload, &rows); err != nil {
				return err
			}
			for index := range rows {
				if err := tx.Save(&rows[index]).Error; err != nil {
					return err
				}
			}
			return nil
		},
	}
}

// configSchemas 是配置中心的白名单。键与旧接口 /api/admin/configs/<key> 保持一致，
// 便于前端逐步迁移；值绑定到 database.InitDB 中迁移的 9 张配置表。
var configSchemas = map[string]configSchema{
	"system":            newConfigSchema[models.SystemConfig]("key asc"),
	"commands":          newConfigSchema[models.CommandConfig]("func_name asc"),
	"pet_species":       newConfigSchema[models.PetSpeciesConfig]("name asc"),
	"items":             newConfigSchema[models.ItemConfig]("name asc"),
	"shop_items":        newConfigSchema[models.ShopItemConfig]("id asc"),
	"menus":             newConfigSchema[models.MenuConfig]("name asc"),
	"images":            newConfigSchema[models.ImageConfig]("name asc"),
	"live_events":       newConfigSchema[models.LiveEventConfig]("starts_at desc"),
	"reward_tracks":     newConfigSchema[models.RewardTrackConfig]("event_key asc, milestone asc"),
	"growth_roles":      newConfigSchema[models.GrowthRoleConfig]("sort_order asc, name asc"),
	"growth_stances":    newConfigSchema[models.GrowthStanceConfig]("sort_order asc, name asc"),
	"personality_rules": newConfigSchema[models.PersonalityRuleConfig]("sort_order asc, name asc"),
	"codex_catalog":     newConfigSchema[models.CodexCatalogConfig]("sort_order asc, category asc, entry_key asc"),
}

// knownConfigSchemas 返回排序后的白名单键，用于错误提示。
func knownConfigSchemas() []string {
	names := make([]string, 0, len(configSchemas))
	for name := range configSchemas {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// ConfigAPI 提供配置中心的 REST 接口。
type ConfigAPI struct {
	DB *gorm.DB
}

func NewConfigAPI(db *gorm.DB) *ConfigAPI {
	return &ConfigAPI{DB: db}
}

// RegisterConfigRoutes 把配置中心接口挂到给定分组上。
// 调用方负责在分组上叠加 RequireAdminSession 等中间件。
func RegisterConfigRoutes(group *gin.RouterGroup, api *ConfigAPI) {
	group.GET("/config/schemas", api.ListSchemas)
	group.GET("/config/status", api.GetStatus)
	group.GET("/config/:schema", api.GetConfig)
	group.PUT("/config/:schema", api.SaveConfig)
}

func (api *ConfigAPI) GetStatus(c *gin.Context) {
	if api.DB == nil {
		Error(c, codeInternalError, "数据库未初始化")
		return
	}
	status, err := appconfig.GetConfigStatus(api.DB)
	if err != nil {
		Error(c, codeInternalError, "查询配置状态失败: "+err.Error())
		return
	}
	Success(c, status)
}

// resolve 校验 schema 参数并确认数据库可用。
func (api *ConfigAPI) resolve(c *gin.Context) (configSchema, bool) {
	if api.DB == nil {
		Error(c, codeInternalError, "数据库未初始化")
		return configSchema{}, false
	}
	name := c.Param("schema")
	schema, exists := configSchemas[name]
	if !exists {
		Error(c, codeSchemaNotFound, "未知的配置类型: "+name+"，可用类型: "+strings.Join(knownConfigSchemas(), ", "))
		return configSchema{}, false
	}
	return schema, true
}

// ListSchemas 返回可管理的配置类型清单，供前端动态生成导航。
func (api *ConfigAPI) ListSchemas(c *gin.Context) {
	Success(c, knownConfigSchemas())
}

// GetConfig 读取某个配置表的全部记录。
func (api *ConfigAPI) GetConfig(c *gin.Context) {
	schema, ok := api.resolve(c)
	if !ok {
		return
	}

	rows, err := schema.fetch(api.DB, schema.orderBy)
	if err != nil {
		Error(c, codeInternalError, "查询配置失败: "+err.Error())
		return
	}
	Success(c, rows)
}

// SaveConfig 批量写入某个配置表。整批记录在单个事务内提交，任一条失败则全部回滚。
func (api *ConfigAPI) SaveConfig(c *gin.Context) {
	schema, ok := api.resolve(c)
	if !ok {
		return
	}

	var payload json.RawMessage
	if err := c.ShouldBindJSON(&payload); err != nil {
		Error(c, codeInvalidPayload, "请求体不是合法的 JSON: "+err.Error())
		return
	}
	if err := validateDomainConfig(c.Param("schema"), payload); err != nil {
		var validationErr configValidationError
		if errors.As(err, &validationErr) {
			Error(c, codeInvalidPayload, validationErr.Error())
			return
		}
		Error(c, codeInvalidPayload, "请求体结构无效: "+err.Error())
		return
	}

	if err := api.DB.Transaction(func(tx *gorm.DB) error {
		if c.Param("schema") == "reward_tracks" {
			if err := validateRewardItems(tx, payload); err != nil {
				return err
			}
		}
		if c.Param("schema") == "live_events" {
			if err := tx.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&models.LiveEventConfig{}).Error; err != nil {
				return err
			}
		}
		if c.Param("schema") == "reward_tracks" {
			if err := tx.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&models.RewardTrackConfig{}).Error; err != nil {
				return err
			}
		}
		switch c.Param("schema") {
		case "growth_roles":
			if err := tx.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&models.GrowthRoleConfig{}).Error; err != nil {
				return err
			}
		case "growth_stances":
			if err := tx.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&models.GrowthStanceConfig{}).Error; err != nil {
				return err
			}
		case "personality_rules":
			if err := tx.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&models.PersonalityRuleConfig{}).Error; err != nil {
				return err
			}
		case "codex_catalog":
			if err := tx.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&models.CodexCatalogConfig{}).Error; err != nil {
				return err
			}
		}
		if err := schema.save(tx, payload); err != nil {
			return err
		}
		return appconfig.MarkConfigSaved(tx)
	}); err != nil {
		var validationErr configValidationError
		if errors.As(err, &validationErr) {
			Error(c, codeInvalidPayload, validationErr.Error())
			return
		}
		// 反序列化失败属于客户端问题，写库失败属于服务端问题，分别用不同错误码。
		if _, isTypeErr := err.(*json.UnmarshalTypeError); isTypeErr {
			Error(c, codeInvalidPayload, "请求体结构与该配置类型不匹配: "+err.Error())
			return
		}
		if _, isSyntaxErr := err.(*json.SyntaxError); isSyntaxErr {
			Error(c, codeInvalidPayload, "请求体不是合法的 JSON: "+err.Error())
			return
		}
		Error(c, codeInternalError, "保存配置失败: "+err.Error())
		return
	}

	Success(c, nil)
}
