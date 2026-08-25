package admin

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	appconfig "qq-pet-saas/config"
	"qq-pet-saas/models"
)

type EcosystemAPI struct {
	DB *gorm.DB
}

func RegisterEcosystemRoutes(group *gin.RouterGroup, db *gorm.DB) {
	api := &EcosystemAPI{DB: db}
	group.GET("/overview", api.Overview)
	group.GET("/metrics/commands", api.CommandMetrics)
	group.GET("/players", api.Players)
	group.GET("/players/:account_id", api.PlayerDetail)
	group.POST("/players/:account_id/grants", api.GrantItem)
	group.PUT("/players/:account_id/notifications", api.SetPlayerNotifications)
	group.DELETE("/players/:account_id/identities/:identity_id", api.DeleteIdentity)
	group.DELETE("/players/:account_id", api.DeletePlayer)
	group.POST("/expeditions/:id/cancel", api.CancelExpedition)
	group.POST("/expeditions/:id/reconcile", api.ReconcileExpedition)
	group.GET("/expeditions", api.Expeditions)
	group.GET("/gameplay/distributions", api.GameplayDistributions)
	group.GET("/gameplay/growth", api.GameplayGrowth)
	group.GET("/gameplay/codex", api.GameplayCodex)
	group.GET("/communities", api.Communities)
	group.GET("/communities/:id", api.CommunityDetail)
	group.PUT("/communities/:id/facilities/:facility_id", api.UpdateFacility)
	group.PUT("/communities/:id/notifications", api.SetCommunityNotifications)
	group.POST("/communities/:id/boss/reset", api.ResetBoss)
	group.POST("/help-requests/:code/close", api.CloseHelpRequest)
	group.POST("/squads/:id/disband", api.DisbandSquad)
	group.GET("/audit-logs", api.AuditLogs)
	RegisterPlatformRoutes(group, db)
}

func (api *EcosystemAPI) Expeditions(c *gin.Context) {
	page, limit := pageParams(c)
	query := api.DB.Model(&models.ExpeditionRun{})
	now := time.Now()
	if value := strings.TrimSpace(c.Query("status")); value != "" {
		if value == "overdue" {
			query = query.Where("status = ? AND ends_at < ?", "running", time.Now())
		} else {
			query = query.Where("status = ?", value)
		}
	}
	if value := strings.TrimSpace(c.Query("tier")); value != "" {
		query = query.Where("tier = ?", value)
	}
	if value := strings.TrimSpace(c.Query("account_id")); value != "" {
		query = query.Where("account_id = ?", value)
	}
	if value := strings.TrimSpace(c.Query("platform")); value != "" {
		query = query.Where("EXISTS (SELECT 1 FROM player_identities pi WHERE pi.account_id = expedition_runs.account_id AND pi.platform = ?)", value)
	}
	if value := strings.TrimSpace(c.Query("from")); value != "" {
		if parsed, err := time.Parse(time.RFC3339, value); err == nil {
			query = query.Where("started_at >= ?", parsed)
		}
	}
	if value := strings.TrimSpace(c.Query("to")); value != "" {
		if parsed, err := time.Parse(time.RFC3339, value); err == nil {
			query = query.Where("started_at <= ?", parsed)
		}
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		Error(c, 5000, "远征统计失败")
		return
	}
	items := make([]models.ExpeditionRun, 0)
	if err := query.Order("started_at DESC").Offset((page - 1) * limit).Limit(limit).Find(&items).Error; err != nil {
		Error(c, 5000, "远征列表读取失败")
		return
	}
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	Success(c, gin.H{"items": items, "total": total, "page": page, "limit": limit, "summary": expeditionSummary(api.DB, now, today)})
}

func expeditionSummary(db *gorm.DB, now, today time.Time) gin.H {
	var running, completed, overdue, cancelled int64
	db.Model(&models.ExpeditionRun{}).Where("status = ?", "running").Count(&running)
	db.Model(&models.ExpeditionRun{}).Where("status = ? AND claimed_at >= ?", "claimed", today).Count(&completed)
	db.Model(&models.ExpeditionRun{}).Where("status = ? AND ends_at < ?", "running", now).Count(&overdue)
	db.Model(&models.ExpeditionRun{}).Where("status = ?", "cancelled").Count(&cancelled)
	return gin.H{"running": running, "today_completed": completed, "overdue": overdue, "cancelled": cancelled}
}

func (api *EcosystemAPI) GameplayDistributions(c *gin.Context) {
	type distribution struct {
		Name  string `json:"name"`
		Count int64  `json:"count"`
	}
	roles := make([]distribution, 0)
	stances := make([]distribution, 0)
	traits := make([]distribution, 0)
	codex := make([]distribution, 0)
	api.DB.Model(&models.PetProfile{}).Select("role name, COUNT(*) count").Group("role").Order("count DESC").Scan(&roles)
	api.DB.Model(&models.PetProfile{}).Select("stance name, COUNT(*) count").Group("stance").Order("count DESC").Scan(&stances)
	api.DB.Model(&models.PetBehaviorProfile{}).Select("trait name, COUNT(*) count").Where("trait <> ''").Group("trait").Order("count DESC").Scan(&traits)
	api.DB.Model(&models.CodexEntry{}).Select("category || ' · ' || entry_key name, COUNT(*) count").Where("progress > 0").Group("category, entry_key").Order("count DESC").Limit(100).Scan(&codex)
	Success(c, gin.H{"roles": roles, "stances": stances, "traits": traits, "codex": codex})
}

func parseRange(value string) (time.Time, string, bool) {
	switch value {
	case "", "7d":
		return time.Now().AddDate(0, 0, -7), "7d", true
	case "30d":
		return time.Now().AddDate(0, 0, -30), "30d", true
	default:
		return time.Time{}, "", false
	}
}

func (api *EcosystemAPI) Overview(c *gin.Context) {
	cutoff, selectedRange, ok := parseRange(c.Query("range"))
	if !ok {
		Error(c, 4000, "range 仅支持 7d 或 30d")
		return
	}
	now := time.Now()
	data := gin.H{"range": selectedRange, "generated_at": now}
	counts := []struct {
		key   string
		model interface{}
		where string
		args  []interface{}
	}{
		{"players", &models.PlayerAccount{}, "", nil},
		{"pets", &models.PetProfile{}, "", nil},
		{"active_expeditions", &models.ExpeditionRun{}, "status = ?", []interface{}{"running"}},
		{"completed_expeditions", &models.ExpeditionRun{}, "status = ? AND claimed_at >= ?", []interface{}{"claimed", cutoff}},
		{"active_communities", &models.Community{}, "updated_at >= ?", []interface{}{cutoff}},
		{"boss_participants", &models.BossContribution{}, "updated_at >= ?", []interface{}{cutoff}},
		{"overdue_expeditions", &models.ExpeditionRun{}, "status = ? AND ends_at < ?", []interface{}{"running", now}},
	}
	for _, item := range counts {
		var count int64
		query := api.DB.Model(item.model)
		if item.where != "" {
			query = query.Where(item.where, item.args...)
		}
		if item.key == "boss_participants" {
			query = query.Distinct("account_id")
		}
		if err := query.Count(&count).Error; err != nil {
			Error(c, 5000, "统计数据读取失败")
			return
		}
		data[item.key] = count
	}
	var successCount, failureCount int64
	api.DB.Model(&models.GameplayMetric{}).Where("day >= ?", cutoff.Format("2006-01-02")).Where("technical_result = ?", "ok").Select("COALESCE(SUM(count), 0)").Scan(&successCount)
	api.DB.Model(&models.GameplayMetric{}).Where("day >= ?", cutoff.Format("2006-01-02")).Where("technical_result <> ?", "ok").Select("COALESCE(SUM(count), 0)").Scan(&failureCount)
	total := successCount + failureCount
	data["command_success_rate"] = float64(0)
	if total > 0 {
		data["command_success_rate"] = float64(successCount) / float64(total)
	}
	data["command_total"] = total
	var todayCompleted int64
	localNow := time.Now()
	today := time.Date(localNow.Year(), localNow.Month(), localNow.Day(), 0, 0, 0, 0, localNow.Location())
	api.DB.Model(&models.ExpeditionRun{}).Where("status = ? AND claimed_at >= ?", "claimed", today).Count(&todayCompleted)
	data["today_completed_expeditions"] = todayCompleted
	if status, err := appconfig.GetConfigStatus(api.DB); err == nil {
		data["config_pending_reload"] = status.PendingReload
	}
	data["platform_error_count"] = 0
	if QQOfficialStatusFunc != nil {
		if encoded, err := json.Marshal(QQOfficialStatusFunc()); err == nil {
			var snapshot map[string]interface{}
			if json.Unmarshal(encoded, &snapshot) == nil && strings.TrimSpace(fmt.Sprint(snapshot["last_error"])) != "" {
				data["platform_error_count"] = 1
			}
		}
	}
	Success(c, data)
}

func (api *EcosystemAPI) CommandMetrics(c *gin.Context) {
	cutoff, selectedRange, ok := parseRange(c.Query("range"))
	if !ok {
		Error(c, 4000, "range 仅支持 7d 或 30d")
		return
	}
	type row struct {
		Day             string `json:"day"`
		Platform        string `json:"platform"`
		SceneType       string `json:"scene_type"`
		Command         string `json:"command"`
		BusinessResult  string `json:"business_result"`
		TechnicalResult string `json:"technical_result"`
		Count           int64  `json:"count"`
	}
	query := api.DB.Model(&models.GameplayMetric{}).Select("day, platform, scene_type, command, business_result, technical_result, SUM(count) count").Where("day >= ?", cutoff.Format("2006-01-02"))
	if value := strings.TrimSpace(c.Query("platform")); value != "" {
		query = query.Where("platform = ?", value)
	}
	if value := strings.TrimSpace(c.Query("scene_type")); value != "" {
		query = query.Where("scene_type = ?", value)
	}
	var rows []row
	if err := query.Group("day, platform, scene_type, command, business_result, technical_result").Order("day, platform, command").Scan(&rows).Error; err != nil {
		Error(c, 5000, "命令指标读取失败")
		return
	}
	Success(c, gin.H{"range": selectedRange, "items": rows})
}

func pageParams(c *gin.Context) (int, int) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	return page, limit
}

type playerSummary struct {
	AccountID        string     `json:"account_id"`
	PetName          string     `json:"pet_name"`
	PetType          string     `json:"pet_type"`
	PetImage         string     `json:"pet_image"`
	Role             string     `json:"role"`
	Growth           int64      `json:"growth"`
	BondLevel        int        `json:"bond_level"`
	IdentityCount    int64      `json:"identity_count"`
	CommunityCount   int64      `json:"community_count"`
	ExpeditionID     string     `json:"expedition_id"`
	ExpeditionStatus string     `json:"expedition_status"`
	LastActiveAt     *time.Time `json:"last_active_at"`
}

func (api *EcosystemAPI) Players(c *gin.Context) {
	page, limit := pageParams(c)
	query := api.DB.Table("player_accounts pa").Joins("LEFT JOIN pet_profiles pp ON pp.account_id = pa.id")
	if value := strings.TrimSpace(c.Query("query")); value != "" {
		like := "%" + value + "%"
		query = query.Where("pa.id LIKE ? OR pp.name LIKE ? OR EXISTS (SELECT 1 FROM player_identities pi WHERE pi.account_id = pa.id AND pi.subject_id LIKE ?)", like, like, like)
	}
	if value := strings.TrimSpace(c.Query("platform")); value != "" {
		query = query.Where("EXISTS (SELECT 1 FROM player_identities pi WHERE pi.account_id = pa.id AND pi.platform = ?)", value)
	}
	if value := strings.TrimSpace(c.Query("role")); value != "" {
		query = query.Where("pp.role = ?", value)
	}
	if value := strings.TrimSpace(c.Query("expedition_status")); value != "" {
		query = query.Where("EXISTS (SELECT 1 FROM expedition_runs er WHERE er.account_id = pa.id AND er.status = ?)", value)
	}
	if value := strings.TrimSpace(c.Query("community_id")); value != "" {
		query = query.Where("EXISTS (SELECT 1 FROM community_members cm WHERE cm.account_id = pa.id AND cm.community_id = ?)", value)
	}
	var total int64
	if err := query.Distinct("pa.id").Count(&total).Error; err != nil {
		Error(c, 5000, "玩家列表统计失败")
		return
	}
	selectSQL := `pa.id account_id, COALESCE(pp.name, '') pet_name, COALESCE(pp.pet_type, '') pet_type,
		COALESCE((SELECT psc.image FROM pet_species_configs psc WHERE psc.name = pp.pet_type LIMIT 1), '') pet_image, COALESCE(pp.role, '') role,
		COALESCE(pp.growth, 0) growth, COALESCE(pp.bond_level, 0) bond_level,
		(SELECT COUNT(*) FROM player_identities pi WHERE pi.account_id = pa.id) identity_count,
		(SELECT COUNT(*) FROM community_members cm WHERE cm.account_id = pa.id) community_count,
		COALESCE((SELECT er.id FROM expedition_runs er WHERE er.account_id = pa.id ORDER BY er.started_at DESC LIMIT 1), '') expedition_id,
		COALESCE((SELECT er.status FROM expedition_runs er WHERE er.account_id = pa.id ORDER BY er.started_at DESC LIMIT 1), '') expedition_status,
		(SELECT MAX(er.started_at) FROM expedition_runs er WHERE er.account_id = pa.id) last_active_at`
	items := make([]playerSummary, 0)
	if err := query.Select(selectSQL).Order("COALESCE(pp.updated_at, pa.updated_at) DESC").Offset((page - 1) * limit).Limit(limit).Scan(&items).Error; err != nil {
		Error(c, 5000, "玩家列表读取失败")
		return
	}
	Success(c, gin.H{"items": items, "total": total, "page": page, "limit": limit})
}

func maskIdentifier(value string) string {
	runes := []rune(value)
	if len(runes) <= 4 {
		return "****"
	}
	return "***" + string(runes[len(runes)-4:])
}

func (api *EcosystemAPI) PlayerDetail(c *gin.Context) {
	accountID := c.Param("account_id")
	var account models.PlayerAccount
	if err := api.DB.First(&account, "id = ?", accountID).Error; err != nil {
		Error(c, 4040, "玩家不存在")
		return
	}
	var pet models.PetProfile
	api.DB.First(&pet, "account_id = ?", accountID)
	petImage := ""
	if pet.PetType != "" {
		var species models.PetSpeciesConfig
		if err := api.DB.First(&species, "name = ?", pet.PetType).Error; err == nil {
			petImage = species.Image
			if pet.CurrentForm != "" && pet.CurrentForm == species.Evolution && species.EvolutionImage != "" {
				petImage = species.EvolutionImage
			}
			if pet.CurrentForm != "" && pet.CurrentForm == species.Awaken && species.AwakenImage != "" {
				petImage = species.AwakenImage
			}
		}
	}
	inventory := make([]models.GlobalInventoryItem, 0)
	codex := make([]models.CodexEntry, 0)
	expeditions := make([]models.ExpeditionRun, 0)
	memberships := make([]models.CommunityMember, 0)
	identities := make([]models.PlayerIdentity, 0)
	api.DB.Where("account_id = ?", accountID).Order("item_name").Find(&inventory)
	api.DB.Where("account_id = ?", accountID).Order("category, entry_key").Find(&codex)
	api.DB.Where("account_id = ?", accountID).Order("started_at DESC").Limit(100).Find(&expeditions)
	api.DB.Where("account_id = ?", accountID).Order("joined_at DESC").Find(&memberships)
	api.DB.Where("account_id = ?", accountID).Order("id").Find(&identities)
	masked := make([]gin.H, 0, len(identities))
	for _, identity := range identities {
		masked = append(masked, gin.H{"id": identity.ID, "platform": identity.Platform, "scene_type": identity.SceneType, "app_id": maskIdentifier(identity.AppID), "scope_id": maskIdentifier(identity.ScopeID), "subject_id": maskIdentifier(identity.SubjectID), "created_at": identity.CreatedAt})
	}
	preference := models.NotificationPreference{AccountID: accountID, Enabled: true}
	api.DB.First(&preference, "account_id = ?", accountID)
	Success(c, gin.H{"account": account, "pet": pet, "pet_image": petImage, "inventory": inventory, "codex": codex, "identities": masked, "expeditions": expeditions, "communities": memberships, "notifications": preference})
}

type communitySummary struct {
	models.Community
	MemberCount   int64 `json:"member_count"`
	SquadCount    int64 `json:"squad_count"`
	OpenHelpCount int64 `json:"open_help_count"`
}

func (api *EcosystemAPI) Communities(c *gin.Context) {
	page, limit := pageParams(c)
	query := api.DB.Model(&models.Community{})
	if value := strings.TrimSpace(c.Query("platform")); value != "" {
		query = query.Where("platform = ?", value)
	}
	if value := strings.TrimSpace(c.Query("scene_type")); value != "" {
		query = query.Where("scene_type = ?", value)
	}
	if value := strings.TrimSpace(c.Query("query")); value != "" {
		query = query.Where("id LIKE ?", "%"+value+"%")
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		Error(c, 5000, "社区统计失败")
		return
	}
	var communities []models.Community
	if err := query.Order("updated_at DESC").Offset((page - 1) * limit).Limit(limit).Find(&communities).Error; err != nil {
		Error(c, 5000, "社区读取失败")
		return
	}
	items := make([]communitySummary, 0, len(communities))
	for _, community := range communities {
		item := communitySummary{Community: community}
		api.DB.Model(&models.CommunityMember{}).Where("community_id = ?", community.ID).Count(&item.MemberCount)
		api.DB.Model(&models.ExpeditionSquad{}).Where("community_id = ?", community.ID).Count(&item.SquadCount)
		api.DB.Model(&models.CommunityHelpRequest{}).Where("community_id = ? AND status = ?", community.ID, "open").Count(&item.OpenHelpCount)
		items = append(items, item)
	}
	Success(c, gin.H{"items": items, "total": total, "page": page, "limit": limit})
}

func (api *EcosystemAPI) CommunityDetail(c *gin.Context) {
	id := c.Param("id")
	var community models.Community
	if err := api.DB.First(&community, "id = ?", id).Error; err != nil {
		Error(c, 4040, "社区不存在")
		return
	}
	members := make([]models.CommunityMember, 0)
	facilities := make([]models.CommunityFacility, 0)
	squads := make([]models.ExpeditionSquad, 0)
	bosses := make([]models.CommunityBoss, 0)
	requests := make([]models.CommunityHelpRequest, 0)
	votes := make([]models.SeasonVote, 0)
	api.DB.Where("community_id = ?", id).Order("contribution DESC").Find(&members)
	api.DB.Where("community_id = ?", id).Order("name").Find(&facilities)
	api.DB.Where("community_id = ?", id).Order("created_at DESC").Find(&squads)
	api.DB.Where("community_id = ?", id).Order("updated_at DESC").Find(&bosses)
	api.DB.Where("community_id = ?", id).Order("created_at DESC").Find(&requests)
	api.DB.Where("community_id = ?", id).Order("updated_at DESC").Find(&votes)
	Success(c, gin.H{"community": community, "members": members, "facilities": facilities, "squads": squads, "bosses": bosses, "help_requests": requests, "votes": votes})
}

func (api *EcosystemAPI) AuditLogs(c *gin.Context) {
	page, limit := pageParams(c)
	query := api.DB.Model(&models.AdminAuditLog{})
	for column, param := range map[string]string{"operator": "operator", "action": "action", "target_type": "target_type"} {
		if value := strings.TrimSpace(c.Query(param)); value != "" {
			query = query.Where(fmt.Sprintf("%s = ?", column), value)
		}
	}
	if value := strings.TrimSpace(c.Query("target")); value != "" {
		query = query.Where("target_id LIKE ?", "%"+value+"%")
	}
	if value := strings.TrimSpace(c.Query("success")); value != "" {
		if parsed, err := strconv.ParseBool(value); err == nil {
			query = query.Where("success = ?", parsed)
		}
	}
	if value := strings.TrimSpace(c.Query("from")); value != "" {
		if parsed, err := time.Parse(time.RFC3339, value); err == nil {
			query = query.Where("created_at >= ?", parsed)
		}
	}
	if value := strings.TrimSpace(c.Query("to")); value != "" {
		if parsed, err := time.Parse(time.RFC3339, value); err == nil {
			query = query.Where("created_at <= ?", parsed)
		}
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		Error(c, 5000, "审计日志统计失败")
		return
	}
	items := make([]models.AdminAuditLog, 0)
	if err := query.Order("created_at DESC, id DESC").Offset((page - 1) * limit).Limit(limit).Find(&items).Error; err != nil {
		Error(c, 5000, "审计日志读取失败")
		return
	}
	Success(c, gin.H{"items": items, "total": total, "page": page, "limit": limit})
}
