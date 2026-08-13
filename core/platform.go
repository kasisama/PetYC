package core

import (
	"context"
	"errors"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"qq-pet-saas/database"
	"qq-pet-saas/models"
)

type Platform string

const (
	PlatformOneBot  Platform = "onebot"
	PlatformQQGroup Platform = "qq_group"
	PlatformQQGuild Platform = "qq_guild"
)

type SceneType string

const (
	SceneGroup  SceneType = "group"
	SceneGuild  SceneType = "guild"
	SceneDirect SceneType = "direct"
)

type InboundEvent struct {
	Platform   Platform
	SceneType  SceneType
	AppID      string
	SpaceID    string
	RoomID     string
	ActorID    string
	MessageID  string
	EventID    string
	MessageSeq int
	Text       string
	Timestamp  time.Time
}

type MarkdownPayload struct {
	Content string `json:"content"`
}

type KeyboardButton struct {
	Label   string `json:"label"`
	Command string `json:"command"`
}

type KeyboardPayload struct {
	Rows [][]KeyboardButton `json:"rows"`
}

type OutboundMessage struct {
	Text     string
	Markdown *MarkdownPayload
	Keyboard *KeyboardPayload
	ReplyTo  string
	Urgency  string
}

func (message OutboundMessage) Render(markdownEnabled, keyboardEnabled bool) OutboundMessage {
	rendered := message
	if !markdownEnabled {
		rendered.Markdown = nil
	}
	if !keyboardEnabled {
		rendered.Keyboard = nil
	}
	return rendered
}

type UnifiedHandler func(context.Context, InboundEvent) (OutboundMessage, error)

type UnifiedFeature struct {
	FuncName       string
	DefaultCommand string
	DisplayName    string
	Category       string
	Description    string
	Enabled        bool
	SortOrder      int
	Hidden         bool
	handler        UnifiedHandler
}

type CommandRouter struct {
	mu       sync.RWMutex
	handlers map[string]UnifiedHandler
}

func NewCommandRouter() *CommandRouter {
	return &CommandRouter{handlers: make(map[string]UnifiedHandler)}
}

func (router *CommandRouter) Register(command string, handler UnifiedHandler) error {
	command = strings.TrimSpace(command)
	if command == "" {
		return errors.New("command cannot be empty")
	}
	if handler == nil {
		return errors.New("handler cannot be nil")
	}
	router.mu.Lock()
	defer router.mu.Unlock()
	router.handlers[command] = handler
	return nil
}

func (router *CommandRouter) Route(ctx context.Context, event InboundEvent) (OutboundMessage, bool, error) {
	router.mu.RLock()
	commands := make([]string, 0, len(router.handlers))
	for command := range router.handlers {
		commands = append(commands, command)
	}
	sort.Slice(commands, func(left, right int) bool { return len(commands[left]) > len(commands[right]) })
	text := strings.TrimSpace(event.Text)
	var handler UnifiedHandler
	var matchedCommand string
	for _, command := range commands {
		if strings.HasPrefix(text, command) {
			handler = router.handlers[command]
			matchedCommand = command
			break
		}
	}
	router.mu.RUnlock()
	if handler == nil {
		return OutboundMessage{}, false, nil
	}
	event.Text = text
	message, err := handler(ctx, event)
	recordGameplayMetric(event, matchedCommand, err == nil)
	return message, true, err
}

func recordGameplayMetric(event InboundEvent, command string, success bool) {
	if database.DB == nil || command == "" || !database.DB.Migrator().HasTable(&models.GameplayMetric{}) {
		return
	}
	now := time.Now()
	metric := models.GameplayMetric{
		Day: now.Format("2006-01-02"), Platform: string(event.Platform), SceneType: string(event.SceneType),
		Command: command, Success: success, Count: 1, UpdatedAt: now,
	}
	_ = database.DB.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "day"}, {Name: "platform"}, {Name: "scene_type"}, {Name: "command"}, {Name: "success"}},
		DoUpdates: clause.Assignments(map[string]interface{}{"count": gorm.Expr("count + 1"), "updated_at": now}),
	}).Create(&metric).Error
}

var (
	unifiedRouter    = NewCommandRouter()
	unifiedRouterMu  sync.RWMutex
	unifiedFeatures  = make(map[string]UnifiedFeature)
	unifiedFeatureMu sync.RWMutex
)

func RegisterUnifiedHandler(command string, handler UnifiedHandler) error {
	unifiedRouterMu.RLock()
	defer unifiedRouterMu.RUnlock()
	return unifiedRouter.Register(command, handler)
}

func RegisterUnifiedFeature(feature UnifiedFeature, handler UnifiedHandler) error {
	feature.FuncName = strings.TrimSpace(feature.FuncName)
	feature.DefaultCommand = strings.TrimSpace(feature.DefaultCommand)
	if feature.FuncName == "" || feature.DefaultCommand == "" || handler == nil {
		return errors.New("unified feature metadata is incomplete")
	}
	feature.handler = handler
	unifiedFeatureMu.Lock()
	unifiedFeatures[feature.FuncName] = feature
	unifiedFeatureMu.Unlock()
	unifiedRouterMu.RLock()
	defer unifiedRouterMu.RUnlock()
	return unifiedRouter.Register(feature.DefaultCommand, handler)
}

func RebuildUnifiedRouter(db *gorm.DB) error {
	if db == nil {
		return errors.New("command database is nil")
	}
	rows := make([]models.CommandConfig, 0)
	if err := db.Find(&rows).Error; err != nil {
		return err
	}
	configs := make(map[string]models.CommandConfig, len(rows))
	for _, row := range rows {
		configs[row.FuncName] = row
	}
	next := NewCommandRouter()
	unifiedFeatureMu.RLock()
	defer unifiedFeatureMu.RUnlock()
	for name, feature := range unifiedFeatures {
		command := feature.DefaultCommand
		enabled := feature.Enabled
		if feature.Hidden {
			enabled = true
		} else if row, exists := configs[name]; exists {
			command, enabled = strings.TrimSpace(row.Command), row.Enabled
		}
		if !enabled || command == "" {
			continue
		}
		if err := next.Register(command, feature.handler); err != nil {
			return err
		}
	}
	unifiedRouterMu.Lock()
	unifiedRouter = next
	unifiedRouterMu.Unlock()
	return nil
}

func SyncUnifiedCommandConfigs(db *gorm.DB) error {
	if db == nil {
		return errors.New("command database is nil")
	}
	const catalogVersion = "2"
	return db.Transaction(func(tx *gorm.DB) error {
		var state models.SystemConfig
		_ = tx.First(&state, "key = ?", "Internal.CommandCatalogVersion").Error
		if state.Value != catalogVersion {
			if err := tx.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&models.CommandConfig{}).Error; err != nil {
				return err
			}
			unifiedFeatureMu.RLock()
			features := make([]UnifiedFeature, 0, len(unifiedFeatures))
			for _, feature := range unifiedFeatures {
				if !feature.Hidden {
					features = append(features, feature)
				}
			}
			unifiedFeatureMu.RUnlock()
			sort.Slice(features, func(i, j int) bool { return features[i].SortOrder < features[j].SortOrder })
			for _, feature := range features {
				row := models.CommandConfig{FuncName: feature.FuncName, Command: feature.DefaultCommand, DisplayName: feature.DisplayName, Category: feature.Category, Description: feature.Description, Enabled: feature.Enabled, SortOrder: feature.SortOrder}
				if err := tx.Create(&row).Error; err != nil {
					return err
				}
			}
			if err := tx.Save(&models.SystemConfig{Key: "Internal.CommandCatalogVersion", Value: catalogVersion}).Error; err != nil {
				return err
			}
		}
		return RebuildUnifiedRouter(tx)
	})
}

var retiredLegacyPrefixes = []string{
	"抽奖", "宠物抽奖", "猜拳", "宠物猜拳", "偷袭", "宠物偷袭", "回击", "宠物回击",
	"宠物交易", "接受交易", "拒绝交易", "添加交易", "删除交易", "交易信息", "取消交易", "确认取消", "同意交易",
	"学习", "宠物学习", "完成学习", "锻炼", "宠物锻炼", "完成锻炼", "健身", "宠物健身", "完成健身", "打工", "宠物打工", "完成打工",
	"钓鱼", "宠物钓鱼", "抛竿", "收竿", "创建家族", "加入家族", "注销家族", "退出家族", "我的家族", "家族列表", "家族成员", "踢出成员", "神树浇水",
}

func IsRetiredLegacyCommand(text string) bool {
	text = strings.TrimSpace(text)
	for _, prefix := range retiredLegacyPrefixes {
		if strings.HasPrefix(text, prefix) {
			return true
		}
	}
	return false
}

func RouteInbound(ctx context.Context, event InboundEvent) (OutboundMessage, bool, error) {
	unifiedRouterMu.RLock()
	router := unifiedRouter
	unifiedRouterMu.RUnlock()
	return router.Route(ctx, event)
}

func routeOneBotMessage(ctx context.Context, event OneBotEvent) (OutboundMessage, bool, error) {
	return RouteInbound(ctx, event.ToInbound("onebot"))
}

func (event OneBotEvent) ToInbound(appID string) InboundEvent {
	scene := SceneDirect
	spaceID := strconv.FormatInt(event.UserID, 10)
	roomID := spaceID
	if event.MessageType == "group" {
		scene = SceneGroup
		spaceID = strconv.FormatInt(event.GroupID, 10)
		roomID = spaceID
	}
	return InboundEvent{
		Platform:  PlatformOneBot,
		SceneType: scene,
		AppID:     appID,
		SpaceID:   spaceID,
		RoomID:    roomID,
		ActorID:   strconv.FormatInt(event.UserID, 10),
		Text:      strings.TrimSpace(event.RawMessage),
		Timestamp: time.Now(),
	}
}
