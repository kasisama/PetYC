package admin

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	appconfig "qq-pet-saas/config"
	"qq-pet-saas/models"
)

type AdventureAPI struct{ DB *gorm.DB }

var errAdventureRevisionConflict = errors.New("adventure catalog revision conflict")

type adventureCatalogRequest struct {
	ExpectedRevision uint64                     `json:"expected_revision"`
	Catalog          appconfig.AdventureCatalog `json:"catalog"`
}

type adventureValidationIssue struct {
	Module    string `json:"module"`
	Entity    string `json:"entity_key,omitempty"`
	Field     string `json:"field,omitempty"`
	Code      string `json:"code"`
	Message   string `json:"message"`
	Reference string `json:"reference,omitempty"`
}

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
	status, err := appconfig.GetConfigStatus(api.DB)
	if err != nil {
		Error(c, codeInternalError, "读取配置版本失败: "+err.Error())
		return
	}
	Success(c, gin.H{"revision": status.DBRevision, "catalog": catalog})
}

func (api *AdventureAPI) validateCatalog(c *gin.Context) {
	var payload appconfig.AdventureCatalog
	if err := c.ShouldBindJSON(&payload); err != nil {
		Error(c, codeInvalidPayload, "冒险配置格式无效: "+err.Error())
		return
	}
	if err := appconfig.ValidateAdventureCatalog(api.DB, payload); err != nil {
		Success(c, gin.H{"valid": false, "issues": []adventureValidationIssue{adventureIssue(err)}, "summary": adventureCatalogSummary(payload)})
		return
	}
	Success(c, gin.H{"valid": true, "issues": []adventureValidationIssue{}, "summary": adventureCatalogSummary(payload)})
}

func (api *AdventureAPI) saveCatalog(c *gin.Context) {
	var request adventureCatalogRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		Error(c, codeInvalidPayload, "冒险配置格式无效: "+err.Error())
		return
	}
	if err := api.DB.Transaction(func(tx *gorm.DB) error {
		status, err := appconfig.GetConfigStatus(tx)
		if err != nil {
			return err
		}
		if status.DBRevision != request.ExpectedRevision {
			return errAdventureRevisionConflict
		}
		if err := appconfig.ReplaceAdventureCatalog(tx, request.Catalog); err != nil {
			return err
		}
		if err := appconfig.MarkConfigSaved(tx); err != nil {
			return err
		}
		// 冒险业务直接从 SQL 查询配置，没有进程内旧副本；在同一事务中
		// 将新版本标记为已加载，避免出现“保存成功但仍待重载”的假状态。
		return appconfig.MarkConfigLoaded(tx, request.ExpectedRevision+1)
	}); err != nil {
		if errors.Is(err, errAdventureRevisionConflict) {
			c.JSON(http.StatusConflict, gin.H{"code": 4093, "msg": "冒险配置已被其他管理员更新，请刷新后重试", "data": gin.H{"conflict": "revision"}, "request_id": responseRequestID(c)})
			return
		}
		Error(c, codeInvalidPayload, "保存冒险配置失败: "+err.Error())
		return
	}
	status, _ := appconfig.GetConfigStatus(api.DB)
	Success(c, gin.H{"saved": true, "revision": status.DBRevision, "summary": adventureCatalogSummary(request.Catalog)})
}

func adventureCatalogSummary(payload appconfig.AdventureCatalog) gin.H {
	return gin.H{"maps": len(payload.Maps), "zones": len(payload.Zones), "monsters": len(payload.Monsters), "bosses": len(payload.Bosses), "equipment": len(payload.EquipmentTemplates), "objectives": len(payload.Objectives), "stages": len(payload.Stages), "story_events": len(payload.StoryEvents), "story_choices": len(payload.StoryChoices), "items": len(payload.Items), "shop_items": len(payload.ShopItems)}
}

func adventureIssue(err error) adventureValidationIssue {
	message := err.Error()
	issue := adventureValidationIssue{Module: "catalog", Code: "invalid_reference", Message: message}
	mappings := []struct{ Prefix, Module string }{
		{"大地图", "maps"}, {"区域", "exploration"}, {"探索目标", "exploration"}, {"节点探索", "exploration"}, {"探索故事", "exploration"}, {"主线探索", "exploration"}, {"遭遇", "exploration"},
		{"怪物", "monsters"}, {"战斗技能", "skills"}, {"奖励池", "loot"}, {"远征物品", "inventory"},
		{"远征货币", "inventory"}, {"远征商店", "shop"}, {"区域远征", "expeditions"}, {"地图首领", "bosses"},
		{"装备", "equipment"},
	}
	for _, mapping := range mappings {
		if strings.HasPrefix(message, mapping.Prefix) {
			issue.Module = mapping.Module
			break
		}
	}
	return issue
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
