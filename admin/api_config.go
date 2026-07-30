package admin

import (
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type ConfigAPI struct {
	DB *gorm.DB
}

func NewConfigAPI(db *gorm.DB) *ConfigAPI {
	return &ConfigAPI{
		DB: db,
	}
}

func (api *ConfigAPI) GetConfig(c *gin.Context) {
	schema := c.Param("schema")

	limit := 100
	results := []map[string]interface{}{}

	if api.DB == nil {
		Success(c, results)
		return
	}

	table := "config_" + schema
	if schema == "global_parameters" {
		table = "global_parameters"
	}

	err := api.DB.Table(table).Limit(limit).Find(&results).Error
	if err != nil {
		Error(c, 500, "failed to query data")
		return
	}

	Success(c, results)
}

func (api *ConfigAPI) SaveConfig(c *gin.Context) {
	schema := c.Param("schema")
	_ = schema
	Success(c, nil)
}
