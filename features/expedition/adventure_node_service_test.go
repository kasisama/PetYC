package expedition

import (
	"context"
	"errors"
	"strings"
	"testing"

	"qq-pet-saas/models"
)

func seedNodeAdventure(t *testing.T, service *Service, accountID string) {
	t.Helper()
	rows := []any{
		&models.AdventureMapConfig{Key: "node-map", Name: "节点地图", Region: "试炼原野", RecommendedLevel: 1, Enabled: true},
		&models.AdventureZoneConfig{Key: "node-zone", MapKey: "node-map", Name: "节点坡地", RecommendedLevel: 1, DifficultyPermille: 1000, HungerCost: 1, ReadinessCost: 1, ExplorationMode: "node", Enabled: true},
		&models.AdventureMonsterConfig{Key: "node-monster", Name: "线索兽", Level: 1, MaxHealth: 1, Attack: 0, Defense: 0, AdventureXP: 1, Enabled: true},
		&models.AdventureExplorationStageConfig{Key: "survey", ZoneKey: "node-zone", Name: "踏勘", ProgressStart: 0, ProgressEnd: 25, EventKey: "survey-event", Enabled: true, SortOrder: 10},
		&models.AdventureExplorationStageConfig{Key: "clues", ZoneKey: "node-zone", Name: "追踪线索", ProgressStart: 25, ProgressEnd: 60, RequiredClues: 2, NextStageKey: "fate", Enabled: true, SortOrder: 20},
		&models.AdventureExplorationStageConfig{Key: "risk-clues", ZoneKey: "node-zone", Name: "深入萤光带", ProgressStart: 25, ProgressEnd: 60, RequiredClues: 2, NextStageKey: "fate", Enabled: true, SortOrder: 25},
		&models.AdventureExplorationStageConfig{Key: "fate", ZoneKey: "node-zone", Name: "命运所指", ProgressStart: 60, ProgressEnd: 75, EventKey: "fate-event", Enabled: true, SortOrder: 30},
		&models.AdventureExplorationStageConfig{Key: "safe", ZoneKey: "node-zone", Name: "稳妥调查", ProgressStart: 75, ProgressEnd: 90, RequiredClues: 1, NextStageKey: "closure", Enabled: true, SortOrder: 40},
		&models.AdventureExplorationStageConfig{Key: "closure", ZoneKey: "node-zone", Name: "调查收束", ProgressStart: 90, ProgressEnd: 100, EventKey: "closure-event", Enabled: true, SortOrder: 50},
		&models.AdventureStoryEventConfig{Key: "survey-event", ZoneKey: "node-zone", StageKey: "survey", Name: "踏勘事件", Description: "选择路线", EventType: "mainline", Weight: 1, Enabled: true},
		&models.AdventureStoryEventConfig{Key: "fate-event", ZoneKey: "node-zone", StageKey: "fate", Name: "命运事件", Description: "选择路线", EventType: "mainline", Weight: 1, Enabled: true},
		&models.AdventureStoryEventConfig{Key: "closure-event", ZoneKey: "node-zone", StageKey: "closure", Name: "收束事件", Description: "完成调查", EventType: "mainline", Weight: 1, Enabled: true},
		&models.AdventureStoryEventChoiceConfig{EventKey: "survey-event", ChoiceKey: "survey-a", Label: "标记路线", RiskLevel: "low", NextStageKey: "clues", Enabled: true, SortOrder: 10},
		&models.AdventureStoryEventChoiceConfig{EventKey: "survey-event", ChoiceKey: "survey-b", Label: "追随微光", RiskLevel: "medium", NextStageKey: "risk-clues", Enabled: true, SortOrder: 20},
		&models.AdventureStoryEventChoiceConfig{EventKey: "fate-event", ChoiceKey: "safe-a", Label: "走旧路", RiskLevel: "low", NextStageKey: "safe", Enabled: true, SortOrder: 10},
		&models.AdventureStoryEventChoiceConfig{EventKey: "fate-event", ChoiceKey: "safe-b", Label: "绕远路", RiskLevel: "medium", NextStageKey: "safe", Enabled: true, SortOrder: 20},
		&models.AdventureStoryEventChoiceConfig{EventKey: "closure-event", ChoiceKey: "close-a", Label: "归档", RiskLevel: "low", NextStageKey: "", Enabled: true, SortOrder: 10},
		&models.AdventureStoryEventChoiceConfig{EventKey: "closure-event", ChoiceKey: "close-b", Label: "分享", RiskLevel: "low", NextStageKey: "", Enabled: true, SortOrder: 20},
		&models.AdventureEncounterConfig{ZoneKey: "node-zone", EncounterKey: "clue-monster", EncounterType: "monster", TargetKey: "node-monster", Name: "线索兽", Weight: 1, StageKey: "clues", NodeRole: "mainline", ClueValue: 1, Enabled: true},
		&models.AdventureEncounterConfig{ZoneKey: "node-zone", EncounterKey: "risk-clue-monster", EncounterType: "monster", TargetKey: "node-monster", Name: "萤光守卫", Weight: 1, StageKey: "risk-clues", NodeRole: "mainline", ClueValue: 1, Enabled: true},
		&models.AdventureEncounterConfig{ZoneKey: "node-zone", EncounterKey: "safe-monster", EncounterType: "monster", TargetKey: "node-monster", Name: "旧路守卫", Weight: 1, StageKey: "safe", NodeRole: "mainline", ClueValue: 1, Enabled: true},
	}
	for _, row := range rows {
		if err := service.DB.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}
}

func TestNodeSurveyChoicesLeadToDifferentConfiguredRoutes(t *testing.T) {
	service, _, _ := newTestService(t)
	service.RandomIntn = func(int) (int, error) { return 0, nil }
	seedAdventurePlayer(t, service, "branch-player", "分支探险者")
	seedNodeAdventure(t, service, "branch-player")

	first, err := service.ExploreZone(context.Background(), "branch-player", "node-zone")
	if err != nil || first.Event == nil || len(first.Event.Choices) != 2 {
		t.Fatalf("survey must present two choices: %#v, %v", first, err)
	}
	resolved, err := service.ResolveNodeEvent(context.Background(), "branch-player", "2")
	if err != nil || resolved.Stage.Key != "risk-clues" {
		t.Fatalf("risk survey choice must enter its own route: %#v, %v", resolved, err)
	}
	next, err := service.ExploreZone(context.Background(), "branch-player", "node-zone")
	if err != nil || next.Encounter.EncounterKey != "risk-clue-monster" {
		t.Fatalf("risk route must use its own configured encounter: %#v, %v", next, err)
	}
}

func TestNodeEventPlayerFacingOrdinalDoesNotExposeChoiceKey(t *testing.T) {
	service, _, _ := newTestService(t)
	service.RandomIntn = func(int) (int, error) { return 0, nil }
	seedAdventurePlayer(t, service, "ordinal-player", "序号探险者")
	seedNodeAdventure(t, service, "ordinal-player")

	first, err := service.ExploreZone(context.Background(), "ordinal-player", "node-zone")
	if err != nil || first.Event == nil {
		t.Fatalf("expected a pending event: %#v, %v", first, err)
	}
	message := formatAdventureStoryEvent(models.AdventureZoneConfig{Name: "节点坡地"}, *first.Event)
	if strings.Contains(message.Text, "survey-a") || strings.Contains(message.Text, "存在风险") || strings.Contains(message.Text, "高风险") {
		t.Fatalf("event copy leaked internal key or risk hint: %q", message.Text)
	}
	if message.Keyboard == nil || len(message.Keyboard.Rows) != 2 || message.Keyboard.Rows[0][0].Command != "探索选择 1" || message.Keyboard.Rows[1][0].Command != "探索选择 2" {
		t.Fatalf("expected ordinal button commands, got %#v", message.Keyboard)
	}
	resolved, err := service.ResolveNodeEvent(context.Background(), "ordinal-player", "2")
	if err != nil || resolved.Choice.ChoiceKey != "survey-b" || resolved.Stage.Key != "risk-clues" {
		t.Fatalf("ordinal must resolve the snapshot's second choice: %#v, %v", resolved, err)
	}
}

func TestNodeEventAcceptsExistingKeyOnlyAsBackwardCompatibility(t *testing.T) {
	service, _, _ := newTestService(t)
	service.RandomIntn = func(int) (int, error) { return 0, nil }
	seedAdventurePlayer(t, service, "legacy-button-player", "旧按钮探险者")
	seedNodeAdventure(t, service, "legacy-button-player")
	if _, err := service.ExploreZone(context.Background(), "legacy-button-player", "node-zone"); err != nil {
		t.Fatal(err)
	}
	resolved, err := service.ResolveNodeEvent(context.Background(), "legacy-button-player", "survey-a")
	if err != nil || resolved.Stage.Key != "clues" {
		t.Fatalf("already-sent legacy button should remain usable: %#v, %v", resolved, err)
	}
}

func TestNodeEventRejectsInvalidOrdinalWithoutResolvingTheEvent(t *testing.T) {
	service, _, _ := newTestService(t)
	service.RandomIntn = func(int) (int, error) { return 0, nil }
	seedAdventurePlayer(t, service, "invalid-ordinal-player", "无效选择探险者")
	seedNodeAdventure(t, service, "invalid-ordinal-player")
	if _, err := service.ExploreZone(context.Background(), "invalid-ordinal-player", "node-zone"); err != nil {
		t.Fatal(err)
	}
	if _, err := service.ResolveNodeEvent(context.Background(), "invalid-ordinal-player", "3"); !errors.Is(err, ErrInvalidAdventureEventChoice) {
		t.Fatalf("out-of-range ordinal should be rejected, got %v", err)
	}
	var pending models.PlayerAdventureEventState
	if err := service.DB.First(&pending, "account_id = ? AND status = ?", "invalid-ordinal-player", "pending").Error; err != nil {
		t.Fatalf("invalid ordinal must not resolve the event: %v", err)
	}
}

func TestNodeMigrationKeepsLegacyQuarterProgressAtSurveyChoice(t *testing.T) {
	service, _, _ := newTestService(t)
	seedAdventurePlayer(t, service, "legacy-quarter-player", "旧进度探险者")
	seedNodeAdventure(t, service, "legacy-quarter-player")
	if err := service.DB.Create(&models.PlayerZoneProgress{AccountID: "legacy-quarter-player", ZoneKey: "node-zone", ExplorationPercent: 25}).Error; err != nil {
		t.Fatal(err)
	}

	result, err := service.ExploreZone(context.Background(), "legacy-quarter-player", "node-zone")
	if err != nil || result.Event == nil || result.Event.Event.Key != "survey-event" {
		t.Fatalf("legacy 25%% must show the first route choice instead of skipping it: %#v, %v", result, err)
	}
}

func finishNodeCombat(t *testing.T, service *Service, accountID, actionKey string) *AdventureCombatResult {
	t.Helper()
	result, err := service.CombatAction(context.Background(), accountID, actionKey, "attack")
	if err != nil {
		t.Fatal(err)
	}
	if result.Turn.Result != "victory" {
		t.Fatalf("expected victory, got %#v", result.Turn)
	}
	return result
}

func TestNodeExplorationRequiresEventsAndDoesNotUnlockFromTargetKills(t *testing.T) {
	service, _, _ := newTestService(t)
	service.RandomIntn = func(int) (int, error) { return 0, nil }
	seedAdventurePlayer(t, service, "node-player", "节点探险者")
	seedNodeAdventure(t, service, "node-player")

	start, err := service.ExploreZone(context.Background(), "node-player", "node-zone")
	if err != nil || start.Event == nil || start.Event.Event.Key != "survey-event" {
		t.Fatalf("first exploration must trigger the survey event: %#v, %v", start, err)
	}
	if _, err = service.ResolveNodeEvent(context.Background(), "node-player", "survey-a"); err != nil {
		t.Fatal(err)
	}
	if _, err = service.ResolveNodeEvent(context.Background(), "node-player", "survey-a"); !errors.Is(err, ErrNoPendingAdventureEvent) {
		t.Fatalf("resolved event must be idempotent, got %v", err)
	}

	for index := 0; index < 2; index++ {
		exploration, err := service.ExploreZone(context.Background(), "node-player", "node-zone")
		if err != nil || exploration.Combat == nil {
			t.Fatalf("expected clue combat %d: %#v, %v", index, exploration, err)
		}
		result := finishNodeCombat(t, service, "node-player", "clue-action-"+string(rune('a'+index)))
		if result.ExpeditionUnlocked || result.ZoneProgress >= 100 {
			t.Fatalf("target combat must not directly unlock the zone: %#v", result)
		}
	}

	fate, err := service.ExploreZone(context.Background(), "node-player", "node-zone")
	if err != nil || fate.Event == nil || fate.Event.Event.Key != "fate-event" {
		t.Fatalf("two clues must lead to the fate event: %#v, %v", fate, err)
	}
	if _, err = service.ResolveNodeEvent(context.Background(), "node-player", "safe-a"); err != nil {
		t.Fatal(err)
	}
	exploration, err := service.ExploreZone(context.Background(), "node-player", "node-zone")
	if err != nil || exploration.Combat == nil {
		t.Fatalf("choice route must lead to its configured combat: %#v, %v", exploration, err)
	}
	result := finishNodeCombat(t, service, "node-player", "safe-action")
	if result.ExpeditionUnlocked || result.ZoneProgress != 90 {
		t.Fatalf("route combat must enter closure rather than unlock: %#v", result)
	}
	closure, err := service.ExploreZone(context.Background(), "node-player", "node-zone")
	if err != nil || closure.Event == nil || closure.Event.Event.Key != "closure-event" {
		t.Fatalf("completion must require a closure event: %#v, %v", closure, err)
	}
	final, err := service.ResolveNodeEvent(context.Background(), "node-player", "close-a")
	if err != nil || !final.Progress.ExpeditionUnlocked || final.Progress.ExplorationPercent != 100 {
		t.Fatalf("closure event must be the only unlock point: %#v, %v", final, err)
	}
}

func TestNodeEventUsesSnapshotWhenOperatorsReplaceEventConfig(t *testing.T) {
	service, _, _ := newTestService(t)
	service.RandomIntn = func(int) (int, error) { return 0, nil }
	seedAdventurePlayer(t, service, "snapshot-player", "快照探险者")
	seedNodeAdventure(t, service, "snapshot-player")

	first, err := service.ExploreZone(context.Background(), "snapshot-player", "node-zone")
	if err != nil || first.Event == nil {
		t.Fatalf("expected pending event before replacing config: %#v, %v", first, err)
	}
	// 配置导入会先删除可变的事件和选项。待选事件应继续使用创建时快照，
	// 而不是因运营更新而让玩家卡在半途中。
	if err := service.DB.Where("event_key = ?", "survey-event").Delete(&models.AdventureStoryEventChoiceConfig{}).Error; err != nil {
		t.Fatal(err)
	}
	if err := service.DB.Where("key = ?", "survey-event").Delete(&models.AdventureStoryEventConfig{}).Error; err != nil {
		t.Fatal(err)
	}

	resolved, err := service.ResolveNodeEvent(context.Background(), "snapshot-player", "survey-a")
	if err != nil || resolved.Stage.Key != "clues" || resolved.Progress.ExplorationPercent != 25 {
		t.Fatalf("snapshot choice should still reach the configured next stage: %#v, %v", resolved, err)
	}
}

func TestNodeSideEncounterCannotDelayMainlineTwice(t *testing.T) {
	service, _, _ := newTestService(t)
	service.RandomIntn = func(int) (int, error) { return 0, nil }
	seedAdventurePlayer(t, service, "side-player", "支线探险者")
	rows := []any{
		&models.AdventureMapConfig{Key: "side-map", Name: "支线地图", Region: "试炼原野", RecommendedLevel: 1, Enabled: true},
		&models.AdventureZoneConfig{Key: "side-zone", MapKey: "side-map", Name: "支线区域", RecommendedLevel: 1, DifficultyPermille: 1000, HungerCost: 1, ReadinessCost: 1, ExplorationMode: "node", Enabled: true},
		&models.AdventureExplorationStageConfig{Key: "side-stage", ZoneKey: "side-zone", Name: "搜寻出口", ProgressStart: 0, ProgressEnd: 100, RequiredClues: 1, Enabled: true, SortOrder: 10},
		&models.AdventureEncounterConfig{ZoneKey: "side-zone", EncounterKey: "side-camp", EncounterType: "safe", Name: "临时营地", Weight: 1, StageKey: "side-stage", NodeRole: "side", Enabled: true, SortOrder: 10},
		&models.AdventureEncounterConfig{ZoneKey: "side-zone", EncounterKey: "side-clue", EncounterType: "landmark", Name: "出口标记", Weight: 1, StageKey: "side-stage", NodeRole: "mainline", ClueValue: 1, Enabled: true, SortOrder: 20},
	}
	for _, row := range rows {
		if err := service.DB.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}

	first, err := service.ExploreZone(context.Background(), "side-player", "side-zone")
	if err != nil || first.Encounter.EncounterKey != "side-camp" || first.Progress.ExplorationPercent != 0 {
		t.Fatalf("first roll should be optional and not advance mainline: %#v, %v", first, err)
	}
	second, err := service.ExploreZone(context.Background(), "side-player", "side-zone")
	if err != nil || second.Encounter.EncounterKey != "side-clue" || !second.Progress.ExpeditionUnlocked {
		t.Fatalf("after one side encounter the next roll must be mainline: %#v, %v", second, err)
	}
}
