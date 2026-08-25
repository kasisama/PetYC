package admin

import (
	"errors"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"qq-pet-saas/security"
)

var OneBotStatusFunc func() interface{}
var QQOfficialStatusFunc func() interface{}
var QQOfficialReconnectFunc func() error
var QQOfficialApplyConfigFunc func() error

type PortHandoffResult struct {
	Address           string    `json:"address"`
	ConfirmationToken string    `json:"confirmation_token"`
	ExpiresAt         time.Time `json:"expires_at"`
}

var PlatformPortChangeFunc func(port int) (PortHandoffResult, error)
var PlatformEndpointChangeFunc func(address string, port int) (PortHandoffResult, error)
var PlatformPortConfirmFunc func(token string) error

type PlatformAPI struct{ DB *gorm.DB }

func RegisterPlatformRoutes(group *gin.RouterGroup, db *gorm.DB) {
	api := &PlatformAPI{DB: db}
	group.GET("/platforms/status", getPlatformStatus)
	group.GET("/platforms/config", api.getConfig)
	group.PUT("/platforms/config", api.putConfig)
	group.POST("/platforms/port/confirm", api.confirmPort)
	group.POST("/platforms/qq/reconnect", api.reconnectQQOfficial)
	group.GET("/platforms/qq/env-template", qqOfficialEnvTemplate)
}

type platformConfigUpdate struct {
	ListenAddress *string `json:"listen_address"`
	Port          *int    `json:"port"`
	OneBot        *struct {
		Token *string `json:"token"`
	} `json:"onebot"`
	QQOfficial *struct {
		AppID              *string `json:"app_id"`
		AppSecret          *string `json:"app_secret"`
		APIBase            *string `json:"api_base"`
		TokenURL           *string `json:"token_url"`
		ShardCount         *int    `json:"shard_count"`
		MarkdownEnabled    *bool   `json:"markdown_enabled"`
		KeyboardEnabled    *bool   `json:"keyboard_enabled"`
		InteractionEnabled *bool   `json:"interaction_enabled"`
		AuditEnabled       *bool   `json:"audit_enabled"`
		GroupEventsEnabled *bool   `json:"group_events_enabled"`
		GuildEventsEnabled *bool   `json:"guild_events_enabled"`
	} `json:"qq_official"`
}

func platformConfigView(config security.RuntimeConfig) gin.H {
	return gin.H{
		"listen_address": config.ListenAddress,
		"port":           config.Port,
		"onebot": gin.H{
			"token_configured": config.OneBotToken != "",
		},
		"qq_official": gin.H{
			"app_id": config.QQOfficial.AppID, "app_secret_configured": config.QQOfficial.AppSecret != "",
			"api_base": config.QQOfficial.APIBase, "token_url": config.QQOfficial.TokenURL,
			"shard_count": config.QQOfficial.ShardCount, "markdown_enabled": config.QQOfficial.MarkdownEnabled,
			"keyboard_enabled": config.QQOfficial.KeyboardEnabled, "interaction_enabled": config.QQOfficial.InteractionEnabled,
			"audit_enabled":        config.QQOfficial.AuditEnabled,
			"group_events_enabled": config.QQOfficial.GroupEventsEnabled,
			"guild_events_enabled": config.QQOfficial.GuildEventsEnabled,
		},
	}
}

func (api *PlatformAPI) getConfig(c *gin.Context) {
	config, err := security.LoadRuntimeConfig()
	if err != nil {
		Error(c, 5000, err.Error())
		return
	}
	Success(c, platformConfigView(config))
}

func (api *PlatformAPI) putConfig(c *gin.Context) {
	var request platformConfigUpdate
	if err := c.ShouldBindJSON(&request); err != nil {
		Error(c, 4000, "请求格式错误")
		return
	}
	if request.Port != nil && (*request.Port < 1 || *request.Port > 65535) {
		Error(c, 4000, "端口必须在 1 到 65535 之间")
		return
	}
	current, err := security.LoadRuntimeConfig()
	if err != nil {
		Error(c, 5000, err.Error())
		return
	}
	next := current
	if request.ListenAddress != nil {
		next.ListenAddress = strings.TrimSpace(*request.ListenAddress)
	}
	if request.Port != nil {
		next.Port = *request.Port
	}
	if request.OneBot != nil && request.OneBot.Token != nil {
		if token := strings.TrimSpace(*request.OneBot.Token); token != "" {
			next.OneBotToken = token
		}
	}
	if request.QQOfficial != nil {
		applyQQOfficialUpdate(&next.QQOfficial, request.QQOfficial)
	}

	var handoff *PortHandoffResult
	if next.Port != current.Port || next.ListenAddress != current.ListenAddress {
		var result PortHandoffResult
		var beginErr error
		if PlatformEndpointChangeFunc != nil {
			result, beginErr = PlatformEndpointChangeFunc(next.ListenAddress, next.Port)
		} else if next.ListenAddress == current.ListenAddress && PlatformPortChangeFunc != nil {
			result, beginErr = PlatformPortChangeFunc(next.Port)
		} else {
			Error(c, 5000, "端口交接服务未初始化")
			return
		}
		if beginErr != nil {
			Error(c, 4000, beginErr.Error())
			return
		}
		handoff = &result
		next.Port = current.Port
		next.ListenAddress = current.ListenAddress
	}
	qqChanged := next.QQOfficial != current.QQOfficial
	if err := security.SaveRuntimeConfig(next); err != nil {
		Error(c, 4000, err.Error())
		return
	}
	if qqChanged && QQOfficialApplyConfigFunc != nil {
		if err := QQOfficialApplyConfigFunc(); err != nil {
			Error(c, 5000, "配置已保存，但 QQ 官方机器人重连失败: "+err.Error())
			return
		}
	}
	data := platformConfigView(next)
	if handoff != nil {
		data["port_handoff"] = handoff
	}
	Success(c, data)
}

func applyQQOfficialUpdate(target *security.QQOfficialRuntimeConfig, update *struct {
	AppID              *string `json:"app_id"`
	AppSecret          *string `json:"app_secret"`
	APIBase            *string `json:"api_base"`
	TokenURL           *string `json:"token_url"`
	ShardCount         *int    `json:"shard_count"`
	MarkdownEnabled    *bool   `json:"markdown_enabled"`
	KeyboardEnabled    *bool   `json:"keyboard_enabled"`
	InteractionEnabled *bool   `json:"interaction_enabled"`
	AuditEnabled       *bool   `json:"audit_enabled"`
	GroupEventsEnabled *bool   `json:"group_events_enabled"`
	GuildEventsEnabled *bool   `json:"guild_events_enabled"`
}) {
	if update.AppID != nil {
		target.AppID = strings.TrimSpace(*update.AppID)
	}
	if update.AppSecret != nil {
		if secret := strings.TrimSpace(*update.AppSecret); secret != "" {
			target.AppSecret = secret
		}
	}
	if update.APIBase != nil {
		target.APIBase = strings.TrimSpace(*update.APIBase)
	}
	if update.TokenURL != nil {
		target.TokenURL = strings.TrimSpace(*update.TokenURL)
	}
	if update.ShardCount != nil {
		target.ShardCount = *update.ShardCount
	}
	if update.MarkdownEnabled != nil {
		target.MarkdownEnabled = *update.MarkdownEnabled
	}
	if update.KeyboardEnabled != nil {
		target.KeyboardEnabled = *update.KeyboardEnabled
	}
	if update.InteractionEnabled != nil {
		target.InteractionEnabled = *update.InteractionEnabled
	}
	if update.AuditEnabled != nil {
		target.AuditEnabled = *update.AuditEnabled
	}
	if update.GroupEventsEnabled != nil {
		target.GroupEventsEnabled = *update.GroupEventsEnabled
	}
	if update.GuildEventsEnabled != nil {
		target.GuildEventsEnabled = *update.GuildEventsEnabled
	}
}

func (api *PlatformAPI) confirmPort(c *gin.Context) {
	var request struct {
		ConfirmationToken string `json:"confirmation_token"`
	}
	if err := c.ShouldBindJSON(&request); err != nil || strings.TrimSpace(request.ConfirmationToken) == "" {
		Error(c, 4000, "确认令牌不能为空")
		return
	}
	if PlatformPortConfirmFunc == nil {
		Error(c, 5000, "端口交接服务未初始化")
		return
	}
	if err := PlatformPortConfirmFunc(strings.TrimSpace(request.ConfirmationToken)); err != nil {
		Error(c, 4000, err.Error())
		return
	}
	Success(c, gin.H{"confirmed": true})
}

func getPlatformStatus(c *gin.Context) {
	oneBot := interface{}(gin.H{"configured": true, "connected": false, "connection_count": 0})
	if OneBotStatusFunc != nil {
		oneBot = OneBotStatusFunc()
	}
	qq := interface{}(gin.H{"configured": false, "app_id_configured": false, "app_secret_configured": false, "connected": false, "capabilities": gin.H{"group": false, "guild": false, "markdown": false, "keyboard": false, "interaction": false}})
	if QQOfficialStatusFunc != nil {
		qq = QQOfficialStatusFunc()
	}
	Success(c, gin.H{"onebot": oneBot, "qq_official": qq})
}

func (api *PlatformAPI) reconnectQQOfficial(c *gin.Context) {
	var request struct {
		Reason string `json:"reason"`
	}
	if err := c.ShouldBindJSON(&request); err != nil {
		Error(c, 4000, "请求格式错误")
		return
	}
	reason, err := requiredReason(request.Reason)
	if err != nil {
		Error(c, 4000, err.Error())
		return
	}
	if QQOfficialReconnectFunc == nil {
		err = errors.New("QQ 官方机器人未配置")
	} else {
		err = QQOfficialReconnectFunc()
	}
	if err != nil {
		_ = writeAudit(api.DB, "reconnect_qq_gateway", "platform", "qq_official", reason, nil, nil, false, err)
		Error(c, 4000, err.Error())
		return
	}
	result := gin.H{"accepted": true}
	if err = writeAudit(api.DB, "reconnect_qq_gateway", "platform", "qq_official", reason, nil, result, true, nil); err != nil {
		Error(c, 5000, "网关已重连，但审计日志写入失败")
		return
	}
	Success(c, result)
}

func qqOfficialEnvTemplate(c *gin.Context) {
	template := strings.Join([]string{
		"LISTEN_ADDRESS=127.0.0.1",
		"PORT=8080",
		"QQPET_WS_TOKEN=your_onebot_token",
		"QQBOT_APP_ID=your_app_id",
		"QQBOT_APP_SECRET=your_app_secret",
		"QQBOT_GROUP_EVENTS_ENABLED=true",
		"QQBOT_GUILD_EVENTS_ENABLED=true",
		"QQBOT_MARKDOWN_ENABLED=false",
		"QQBOT_KEYBOARD_ENABLED=false",
		"QQBOT_INTERACTION_ENABLED=false",
		"QQBOT_AUDIT_ENABLED=false",
	}, "\n")
	Success(c, gin.H{"template": template})
}
