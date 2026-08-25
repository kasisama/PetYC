package admin

import (
	"errors"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	appconfig "qq-pet-saas/config"
	"qq-pet-saas/models"
)

type AdventureAPI struct{ DB *gorm.DB }

func RegisterAdventureRoutes(group *gin.RouterGroup, db *gorm.DB) {
	api := &AdventureAPI{DB: db}
	group.GET("/adventure/catalog", api.getCatalog)
	group.POST("/adventure/catalog/validate", api.validateCatalog)
	group.PUT("/adventure/catalog", api.saveCatalog)
	group.GET("/adventure/runtime", api.getRuntime)
}

func (api *AdventureAPI) getCatalog(c *gin.Context) {
	catalog, err := appconfig.CaptureAdventureCatalog(api.DB)
	if err != nil {
		Error(c, codeInternalError, "读取冒险配置失败: "+err.Error())
		return
	}
	Success(c, catalog)
}

func (api *AdventureAPI) validateCatalog(c *gin.Context) {
	var payload appconfig.AdventureCatalog
	if err := c.ShouldBindJSON(&payload); err != nil {
		Error(c, codeInvalidPayload, "冒险配置格式无效: "+err.Error())
		return
	}
	if err := appconfig.ValidateAdventureCatalog(api.DB, payload); err != nil {
		Error(c, codeInvalidPayload, err.Error())
		return
	}
	Success(c, gin.H{"valid": true, "summary": adventureCatalogSummary(payload)})
}

func (api *AdventureAPI) saveCatalog(c *gin.Context) {
	var payload appconfig.AdventureCatalog
	if err := c.ShouldBindJSON(&payload); err != nil {
		Error(c, codeInvalidPayload, "冒险配置格式无效: "+err.Error())
		return
	}
	if err := api.DB.Transaction(func(tx *gorm.DB) error {
		if err := appconfig.ReplaceAdventureCatalog(tx, payload); err != nil {
			return err
		}
		return appconfig.MarkConfigSaved(tx)
	}); err != nil {
		Error(c, codeInvalidPayload, "保存冒险配置失败: "+err.Error())
		return
	}
	Success(c, gin.H{"saved": true, "summary": adventureCatalogSummary(payload)})
}

func adventureCatalogSummary(payload appconfig.AdventureCatalog) gin.H {
	return gin.H{"maps": len(payload.Maps), "zones": len(payload.Zones), "monsters": len(payload.Monsters), "bosses": len(payload.Bosses), "equipment": len(payload.EquipmentTemplates), "objectives": len(payload.Objectives)}
}

func (api *AdventureAPI) getRuntime(c *gin.Context) {
	now := time.Now()
	counts := gin.H{}
	queries := []struct {
		key   string
		model any
		where string
		args  []any
	}{
		{"active_explorations", &models.AdventureExplorationSession{}, "status = ?", []any{"active"}},
		{"active_combats", &models.AdventureCombatSession{}, "status = ? AND expires_at > ?", []any{"active", now}},
		{"running_expeditions", &models.AdventureExpeditionRun{}, "status = ?", []any{"running"}},
		{"active_bosses", &models.AdventureBossInstance{}, "status = ? AND expires_at > ?", []any{"active", now}},
	}
	for _, query := range queries {
		var count int64
		if err := api.DB.Model(query.model).Where(query.where, query.args...).Count(&count).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				count = 0
			} else {
				Error(c, codeInternalError, "读取冒险运行状态失败: "+err.Error())
				return
			}
		}
		counts[query.key] = count
	}
	Success(c, gin.H{"as_of": now, "counts": counts})
}
