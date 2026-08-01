package admin

import (
	"encoding/json"
	"sort"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"qq-pet-saas/models"
)

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
	"system":          newConfigSchema[models.SystemConfig]("key asc"),
	"commands":        newConfigSchema[models.CommandConfig]("func_name asc"),
	"pet_species":     newConfigSchema[models.PetSpeciesConfig]("name asc"),
	"items":           newConfigSchema[models.ItemConfig]("name asc"),
	"shop_items":      newConfigSchema[models.ShopItemConfig]("id asc"),
	"work_settings":   newConfigSchema[models.WorkSettingConfig]("name asc"),
	"menus":           newConfigSchema[models.MenuConfig]("name asc"),
	"images":          newConfigSchema[models.ImageConfig]("name asc"),
	"checkin_rewards": newConfigSchema[models.CheckinRewardConfig]("id asc"),
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
	group.GET("/config/:schema", api.GetConfig)
	group.PUT("/config/:schema", api.SaveConfig)
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

	if err := api.DB.Transaction(func(tx *gorm.DB) error {
		return schema.save(tx, payload)
	}); err != nil {
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
