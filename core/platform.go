package core

import (
	"context"
	"errors"
	"fmt"
	"log"
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
	Platform    Platform
	SceneType   SceneType
	AppID       string
	SpaceID     string
	RoomID      string
	ActorID     string
	ActorName   string
	MessageID   string
	ReferenceID string
	EventID     string
	MessageSeq  int
	Text        string
	Timestamp   time.Time
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
	MessageKey      string
	Text            string
	Image           string
	Markdown        *MarkdownPayload
	Keyboard        *KeyboardPayload
	ReplyTo         string
	Urgency         string
	BusinessResult  string
	TechnicalResult string
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
	Aliases        []string
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
	return router.RegisterAll([]string{command}, handler)
}

func (router *CommandRouter) RegisterAll(commands []string, handler UnifiedHandler) error {
	normalized, err := normalizeCommandTriggers(commands)
	if err != nil {
		return err
	}
	if handler == nil {
		return errors.New("handler cannot be nil")
	}
	router.mu.Lock()
	defer router.mu.Unlock()
	for _, command := range normalized {
		if _, exists := router.handlers[command]; exists {
			return fmt.Errorf("command %q is already registered", command)
		}
	}
	for _, command := range normalized {
		router.handlers[command] = handler
	}
	return nil
}

func normalizeCommandTriggers(commands []string) ([]string, error) {
	normalized := make([]string, 0, len(commands))
	seen := make(map[string]struct{}, len(commands))
	for _, command := range commands {
		command = strings.TrimSpace(command)
		if command == "" {
			return nil, errors.New("command cannot be empty")
		}
		if _, exists := seen[command]; exists {
			continue
		}
		seen[command] = struct{}{}
		normalized = append(normalized, command)
	}
	if len(normalized) == 0 {
		return nil, errors.New("at least one command is required")
	}
	return normalized, nil
}

func (router *CommandRouter) Route(ctx context.Context, event InboundEvent) (OutboundMessage, bool, error) {
	router.mu.RLock()
	commands := make([]string, 0, len(router.handlers))
	for command := range router.handlers {
		commands = append(commands, command)
	}
	sort.Slice(commands, func(left, right int) bool { return len(commands[left]) > len(commands[right]) })
	text := normalizeInboundCommandText(event.Text)
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
	recordGameplayMetric(event, matchedCommand, message, err)
	return message, true, err
}

// normalizeInboundCommandText keeps existing plain-text commands compatible
// with platforms such as QQ Official that send slash-prefixed commands.
func normalizeInboundCommandText(text string) string {
	text = strings.TrimSpace(text)
	text = strings.TrimPrefix(text, "/")
	return strings.TrimSpace(text)
}

func recordGameplayMetric(event InboundEvent, command string, message OutboundMessage, handlerErr error) {
	if database.DB == nil || command == "" || !database.DB.Migrator().HasTable(&models.GameplayMetric{}) {
		return
	}
	now := time.Now()
	businessResult := strings.TrimSpace(message.BusinessResult)
	if businessResult == "" {
		businessResult = "success"
	}
	technicalResult := strings.TrimSpace(message.TechnicalResult)
	if technicalResult == "" {
		technicalResult = "ok"
		if handlerErr != nil {
			technicalResult = "error"
		}
	}
	metric := models.GameplayMetric{
		Day: now.Format("2006-01-02"), Platform: string(event.Platform), SceneType: string(event.SceneType),
		Command: command, BusinessResult: businessResult, TechnicalResult: technicalResult, Count: 1, UpdatedAt: now,
	}
	_ = database.DB.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "day"}, {Name: "platform"}, {Name: "scene_type"}, {Name: "command"}, {Name: "business_result"}, {Name: "technical_result"}},
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
	feature.Aliases = normalizedAliases(feature.DefaultCommand, feature.Aliases)
	feature.handler = handler
	unifiedRouterMu.RLock()
	err := unifiedRouter.RegisterAll(featureTriggers(feature.DefaultCommand, feature.Aliases), handler)
	unifiedRouterMu.RUnlock()
	if err != nil {
		return err
	}
	unifiedFeatureMu.Lock()
	unifiedFeatures[feature.FuncName] = feature
	unifiedFeatureMu.Unlock()
	return nil
}

func normalizedAliases(command string, aliases []string) []string {
	result := make([]string, 0, len(aliases))
	seen := map[string]struct{}{strings.TrimSpace(command): {}}
	for _, alias := range aliases {
		alias = strings.TrimSpace(alias)
		if alias == "" {
			continue
		}
		if _, exists := seen[alias]; exists {
			continue
		}
		seen[alias] = struct{}{}
		result = append(result, alias)
	}
	return result
}

func featureTriggers(command string, aliases []string) []string {
	triggers := make([]string, 0, len(aliases)+1)
	triggers = append(triggers, command)
	triggers = append(triggers, aliases...)
	return triggers
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
	triggerOwners := make(map[string]string)
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
		triggers := featureTriggers(command, feature.Aliases)
		for _, trigger := range triggers {
			trigger = strings.TrimSpace(trigger)
			if owner, exists := triggerOwners[trigger]; exists && owner != name {
				return fmt.Errorf("指令 %q 同时属于功能 %s 和 %s", trigger, owner, name)
			}
			triggerOwners[trigger] = name
		}
		if err := next.RegisterAll(triggers, feature.handler); err != nil {
			return err
		}
	}
	if db.Migrator().HasTable(&models.MenuConfig{}) {
		menus := make([]models.MenuConfig, 0)
		if err := db.Order("name asc").Find(&menus).Error; err != nil {
			return err
		}
		for _, row := range menus {
			trigger := strings.TrimSpace(row.Name)
			if trigger == "" || strings.TrimSpace(row.Reply) == "" {
				continue
			}
			if owner, owned := triggerOwners[trigger]; owned {
				log.Printf("[菜单场景] 名称 %q 与已有指令 %s 冲突，已跳过运行时注册", trigger, owner)
				continue
			}
			menuName := trigger
			if err := next.Register(menuName, func(ctx context.Context, _ InboundEvent) (OutboundMessage, error) {
				var current models.MenuConfig
				if err := db.WithContext(ctx).Where("name = ?", menuName).First(&current).Error; err != nil {
					return OutboundMessage{}, err
				}
				reply := strings.TrimSpace(current.Reply)
				message := OutboundMessage{
					MessageKey: "menu." + menuName,
					Text:       reply,
					Image:      ExistingImageSource(current.Image),
					ReplyTo:    "source",
				}
				if markdown := strings.TrimSpace(current.Markdown); markdown != "" {
					message.Markdown = &MarkdownPayload{Content: markdown}
				}
				return message, nil
			}); err != nil {
				return err
			}
			triggerOwners[menuName] = "menu:" + menuName
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
	const catalogVersion = "6"
	if err := db.Transaction(func(tx *gorm.DB) error {
		var state models.SystemConfig
		if err := tx.Limit(1).Find(&state, "key = ?", "Internal.CommandCatalogVersion").Error; err != nil {
			return err
		}
		if state.Value != catalogVersion {
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
				var row models.CommandConfig
				result := tx.Limit(1).Find(&row, "func_name = ?", feature.FuncName)
				if result.Error != nil {
					return result.Error
				}
				if result.RowsAffected == 0 {
					row = models.CommandConfig{FuncName: feature.FuncName, Command: feature.DefaultCommand, DisplayName: feature.DisplayName, Category: feature.Category, Description: feature.Description, Enabled: feature.Enabled, SortOrder: feature.SortOrder}
					if err := tx.Create(&row).Error; err != nil {
						return err
					}
					continue
				}
				legacyDefaults := map[string]string{"equipment": "装备", "blueprints": "蓝图"}
				if legacy, exists := legacyDefaults[feature.FuncName]; exists && row.Command == legacy {
					row.Command = feature.DefaultCommand
					if err := tx.Model(&row).Update("command", row.Command).Error; err != nil {
						return err
					}
				}
				if err := tx.Model(&row).Updates(map[string]interface{}{
					"display_name": feature.DisplayName, "category": feature.Category,
					"description": feature.Description, "sort_order": feature.SortOrder,
				}).Error; err != nil {
					return err
				}
			}
			if err := tx.Save(&models.SystemConfig{Key: "Internal.CommandCatalogVersion", Value: catalogVersion}).Error; err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return err
	}
	// 路由处理器的生命周期长于上面的事务，必须捕获根 DB，不能持有已提交的 tx。
	return RebuildUnifiedRouter(db)
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
	messageID := ""
	if event.MessageID != 0 {
		messageID = strconv.FormatInt(event.MessageID, 10)
	}
	return InboundEvent{
		Platform:  PlatformOneBot,
		SceneType: scene,
		AppID:     appID,
		SpaceID:   spaceID,
		RoomID:    roomID,
		ActorID:   strconv.FormatInt(event.UserID, 10),
		ActorName: strings.TrimSpace(event.Sender.Nickname),
		MessageID: messageID,
		Text:      strings.TrimSpace(event.RawMessage),
		Timestamp: time.Now(),
	}
}
