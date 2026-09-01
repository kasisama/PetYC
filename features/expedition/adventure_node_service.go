package expedition

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"qq-pet-saas/gameplay"
	"qq-pet-saas/models"
)

var (
	ErrNoPendingAdventureEvent     = errors.New("当前没有待处理的探索事件")
	ErrInvalidAdventureEventChoice = errors.New("当前事件选择无效")
)

type AdventureStoryEventView struct {
	Event   models.AdventureStoryEventConfig         `json:"event"`
	Choices []models.AdventureStoryEventChoiceConfig `json:"choices"`
}

type NodeEventResolution struct {
	Event    models.AdventureStoryEventConfig       `json:"event"`
	Choice   models.AdventureStoryEventChoiceConfig `json:"choice"`
	Progress models.PlayerZoneProgress              `json:"progress"`
	Stage    models.AdventureExplorationStageConfig `json:"stage"`
}

type adventureEventSnapshot struct {
	Event   models.AdventureStoryEventConfig         `json:"event"`
	Choices []models.AdventureStoryEventChoiceConfig `json:"choices"`
}

func nodeMode(zone models.AdventureZoneConfig) bool { return zone.ExplorationMode == "node" }

func (service *Service) loadNodeStagesTx(tx *gorm.DB, zoneKey string) ([]models.AdventureExplorationStageConfig, error) {
	var stages []models.AdventureExplorationStageConfig
	if err := tx.Where("zone_key = ? AND enabled = ?", zoneKey, true).Order("sort_order asc, key asc").Find(&stages).Error; err != nil {
		return nil, err
	}
	if len(stages) == 0 {
		return nil, ErrNoEncounter
	}
	return stages, nil
}

func stageByKey(stages []models.AdventureExplorationStageConfig, key string) (models.AdventureExplorationStageConfig, bool) {
	for _, stage := range stages {
		if stage.Key == key {
			return stage, true
		}
	}
	return models.AdventureExplorationStageConfig{}, false
}

func (service *Service) ensureNodeProgressTx(tx *gorm.DB, accountID string, zone models.AdventureZoneConfig, stages []models.AdventureExplorationStageConfig) (models.PlayerAdventureNodeProgress, error) {
	var state models.PlayerAdventureNodeProgress
	lookup := tx.Limit(1).Find(&state, "account_id = ? AND zone_key = ?", accountID, zone.Key)
	if lookup.Error != nil {
		return state, lookup.Error
	}
	if lookup.RowsAffected > 0 {
		return state, nil
	}
	now := service.Now()
	state = models.PlayerAdventureNodeProgress{AccountID: accountID, ZoneKey: zone.Key, StageKey: stages[0].Key, UpdatedAt: now}

	// Preserve rather than replay legacy progress when an operator enables the
	// node flow for an already explored zone.
	var legacy models.PlayerZoneProgress
	legacyLookup := tx.Limit(1).Find(&legacy, "account_id = ? AND zone_key = ?", accountID, zone.Key)
	if legacyLookup.Error != nil {
		return state, legacyLookup.Error
	}
	if legacyLookup.RowsAffected > 0 && (legacy.ExpeditionUnlocked || legacy.ExplorationPercent >= 100) {
		state.StageKey, state.CompletedAt = "completed", &now
		// 旧流程的 25% 仅表示已经抵达区域。节点流程的入口事件正是要
		// 让玩家在这个节点作出第一条路线选择，因此不能把这批玩家直接
		// 跳进线索阶段而错过分支。
	} else if legacyLookup.RowsAffected > 0 && legacy.ExplorationPercent > 25 {
		for _, stage := range stages {
			if stage.ProgressStart >= legacy.ExplorationPercent {
				state.StageKey = stage.Key
				break
			}
		}
		// Legacy target kills become only already-found clues; they never unlock
		// a node zone without its configured story events.
		var objectives []models.AdventureObjectiveConfig
		if err := tx.Where("zone_key = ? AND objective_type = ?", zone.Key, "monster_kill").Find(&objectives).Error; err != nil {
			return state, err
		}
		if current, ok := stageByKey(stages, state.StageKey); ok && current.RequiredClues > 0 {
			for _, objective := range objectives {
				var old models.PlayerObjectiveProgress
				oldLookup := tx.Limit(1).Find(&old, "account_id = ? AND objective_key = ?", accountID, objective.Key)
				if oldLookup.Error != nil {
					return state, oldLookup.Error
				}
				if oldLookup.RowsAffected > 0 && old.Progress > 0 {
					state.ClueProgress = minInt(current.RequiredClues, int(old.Progress))
					break
				}
			}
		}
	}
	if err := tx.Create(&state).Error; err != nil {
		return state, err
	}
	return state, nil
}

func nodeExplorationPercent(state models.PlayerAdventureNodeProgress, stage models.AdventureExplorationStageConfig) int {
	if state.CompletedAt != nil {
		return 100
	}
	if stage.RequiredClues <= 0 {
		return stage.ProgressStart
	}
	clues := minInt(state.ClueProgress, stage.RequiredClues)
	return stage.ProgressStart + (stage.ProgressEnd-stage.ProgressStart)*clues/stage.RequiredClues
}

func (service *Service) syncNodeZoneProgressTx(tx *gorm.DB, accountID string, state models.PlayerAdventureNodeProgress, stage models.AdventureExplorationStageConfig) (models.PlayerZoneProgress, error) {
	var progress models.PlayerZoneProgress
	lookup := tx.Limit(1).Find(&progress, "account_id = ? AND zone_key = ?", accountID, state.ZoneKey)
	if lookup.Error != nil {
		return progress, lookup.Error
	}
	if lookup.RowsAffected == 0 {
		progress = models.PlayerZoneProgress{AccountID: accountID, ZoneKey: state.ZoneKey}
	}
	progress.ExplorationPercent = nodeExplorationPercent(state, stage)
	progress.ExpeditionUnlocked = state.CompletedAt != nil
	if progress.ExpeditionUnlocked && progress.FirstClearedAt == nil {
		now := service.Now()
		progress.FirstClearedAt = &now
	}
	progress.UpdatedAt = service.Now()
	return progress, tx.Save(&progress).Error
}

func (service *Service) pendingNodeEventTx(tx *gorm.DB, accountID, zoneKey string) (models.PlayerAdventureEventState, bool, error) {
	var state models.PlayerAdventureEventState
	query := tx.Where("account_id = ? AND status = ?", accountID, "pending")
	if strings.TrimSpace(zoneKey) != "" {
		query = query.Where("zone_key = ?", zoneKey)
	}
	lookup := query.Order("updated_at desc").Limit(1).Find(&state)
	if lookup.Error != nil {
		return state, false, lookup.Error
	}
	return state, lookup.RowsAffected > 0, nil
}

func decodeStoryEventSnapshot(state models.PlayerAdventureEventState) (AdventureStoryEventView, error) {
	var snapshot adventureEventSnapshot
	if err := json.Unmarshal([]byte(state.SnapshotJSON), &snapshot); err != nil {
		return AdventureStoryEventView{}, err
	}
	if snapshot.Event.Key == "" || len(snapshot.Choices) == 0 {
		return AdventureStoryEventView{}, fmt.Errorf("探索事件快照不完整")
	}
	return AdventureStoryEventView{Event: snapshot.Event, Choices: snapshot.Choices}, nil
}

func (service *Service) createPendingNodeEventTx(tx *gorm.DB, accountID string, stage models.AdventureExplorationStageConfig) (*AdventureStoryEventView, error) {
	if strings.TrimSpace(stage.EventKey) == "" {
		return nil, nil
	}
	var event models.AdventureStoryEventConfig
	if err := tx.First(&event, "key = ? AND enabled = ?", stage.EventKey, true).Error; err != nil {
		return nil, err
	}
	var choices []models.AdventureStoryEventChoiceConfig
	if err := tx.Where("event_key = ? AND enabled = ?", event.Key, true).Order("sort_order asc, choice_key asc").Find(&choices).Error; err != nil {
		return nil, err
	}
	if len(choices) == 0 {
		return nil, fmt.Errorf("探索事件 %s 没有可用选项", event.Key)
	}
	raw, err := json.Marshal(adventureEventSnapshot{Event: event, Choices: choices})
	if err != nil {
		return nil, err
	}
	now := service.Now()
	state := models.PlayerAdventureEventState{AccountID: accountID, ZoneKey: event.ZoneKey, EventKey: event.Key, Status: "pending", SnapshotJSON: string(raw), UpdatedAt: now}
	if err := tx.Create(&state).Error; err != nil {
		return nil, err
	}
	return &AdventureStoryEventView{Event: event, Choices: choices}, nil
}

func weightedNodeEncounter(service *Service, encounters []models.AdventureEncounterConfig) (models.AdventureEncounterConfig, error) {
	if len(encounters) == 0 {
		return models.AdventureEncounterConfig{}, ErrNoEncounter
	}
	total := 0
	for _, encounter := range encounters {
		total += encounter.Weight
	}
	roll, err := service.RandomIntn(total)
	if err != nil {
		return models.AdventureEncounterConfig{}, err
	}
	for _, encounter := range encounters {
		roll -= encounter.Weight
		if roll < 0 {
			return encounter, nil
		}
	}
	return encounters[len(encounters)-1], nil
}

func (service *Service) selectNodeEncounterTx(tx *gorm.DB, state models.PlayerAdventureNodeProgress, stage models.AdventureExplorationStageConfig) (models.AdventureEncounterConfig, error) {
	query := tx.Where("zone_key = ? AND enabled = ?", state.ZoneKey, true)
	if state.CompletedAt != nil {
		query = query.Where("node_role = ?", "repeat")
	} else {
		query = query.Where("stage_key = ?", stage.Key)
		if state.SideMisses >= 1 {
			query = query.Where("node_role = ?", "mainline")
		} else {
			query = query.Where("node_role IN ?", []string{"mainline", "side"})
		}
	}
	var encounters []models.AdventureEncounterConfig
	if err := query.Order("sort_order asc, id asc").Find(&encounters).Error; err != nil {
		return models.AdventureEncounterConfig{}, err
	}
	if state.CompletedAt != nil && len(encounters) == 0 {
		if err := tx.Where("zone_key = ? AND enabled = ? AND node_role IN ?", state.ZoneKey, true, []string{"mainline", "side"}).Order("sort_order asc, id asc").Find(&encounters).Error; err != nil {
			return models.AdventureEncounterConfig{}, err
		}
	}
	return weightedNodeEncounter(service, encounters)
}

func (service *Service) advanceNodeClueTx(tx *gorm.DB, accountID string, state *models.PlayerAdventureNodeProgress, stage models.AdventureExplorationStageConfig, encounter models.AdventureEncounterConfig) (models.PlayerZoneProgress, models.AdventureExplorationStageConfig, error) {
	if state.CompletedAt != nil {
		progress, err := service.syncNodeZoneProgressTx(tx, accountID, *state, stage)
		return progress, stage, err
	}
	if encounter.NodeRole == "mainline" {
		state.ClueProgress += maxInt(1, encounter.ClueValue)
		state.SideMisses = 0
	} else if encounter.NodeRole == "side" {
		state.SideMisses++
	}
	state.ActionSequence++
	state.UpdatedAt = service.Now()
	if stage.RequiredClues > 0 && state.ClueProgress >= stage.RequiredClues {
		if stage.NextStageKey == "" {
			now := service.Now()
			state.CompletedAt = &now
		} else {
			var next models.AdventureExplorationStageConfig
			if err := tx.First(&next, "key = ? AND enabled = ?", stage.NextStageKey, true).Error; err != nil {
				return models.PlayerZoneProgress{}, stage, err
			}
			state.StageKey, state.ClueProgress, state.SideMisses = next.Key, 0, 0
			stage = next
		}
	}
	if err := tx.Save(state).Error; err != nil {
		return models.PlayerZoneProgress{}, stage, err
	}
	progress, err := service.syncNodeZoneProgressTx(tx, accountID, *state, stage)
	return progress, stage, err
}

func (service *Service) exploreNodeZoneInCommunity(ctx context.Context, accountID, communityID string, zone models.AdventureZoneConfig) (*ExplorationResult, error) {
	var result ExplorationResult
	err := gameplay.WithTransactionRetry(ctx, service.DB, func(tx *gorm.DB) error {
		accessible, missing, err := service.zoneAccessibleTx(tx, accountID, zone.Key)
		if err != nil {
			return err
		}
		if !accessible {
			return fmt.Errorf("%w: 需要先完成 %s", ErrZoneLocked, strings.Join(missing, "、"))
		}
		stages, err := service.loadNodeStagesTx(tx, zone.Key)
		if err != nil {
			return err
		}
		state, err := service.ensureNodeProgressTx(tx, accountID, zone, stages)
		if err != nil {
			return err
		}
		// 事件选项命令不携带区域键；同一玩家同时只能保留一个待选事件，
		// 避免在两个区域之间产生无法判定归属的选择。
		if pending, ok, err := service.pendingNodeEventTx(tx, accountID, ""); err != nil {
			return err
		} else if ok {
			view, err := decodeStoryEventSnapshot(pending)
			if err != nil {
				return err
			}
			result.Event = &view
			return nil
		}
		stage := stages[0]
		if state.CompletedAt == nil {
			var ok bool
			stage, ok = stageByKey(stages, state.StageKey)
			if !ok {
				return fmt.Errorf("当前探索阶段 %s 已不存在", state.StageKey)
			}
			if event, err := service.createPendingNodeEventTx(tx, accountID, stage); err != nil {
				return err
			} else if event != nil {
				result.Event = event
				return nil
			}
		} else {
			stage = models.AdventureExplorationStageConfig{Key: "completed", ProgressStart: 100, ProgressEnd: 100}
		}
		petRow, err := gameplay.ActivePetTx(tx, accountID)
		if err != nil {
			return err
		}
		pet := *petRow
		if pet.Status == "受伤" {
			return ErrAdventureInjured
		}
		if err := gameplay.ReservePetRunTx(tx, accountID, pet.ID); err != nil {
			if errors.Is(err, gameplay.ErrTooManyConcurrentRuns) || errors.Is(err, gameplay.ErrActivityActive) {
				return ErrAdventureBusy
			}
			return err
		}
		if zone.HungerCost > 0 && pet.Hunger-zone.HungerCost <= 10 {
			return gameplay.ErrPetTooHungry
		}
		if zone.ReadinessCost > 0 && pet.Readiness < zone.ReadinessCost {
			return ErrInsufficientReadiness
		}
		encounter, err := service.selectNodeEncounterTx(tx, state, stage)
		if err != nil {
			return err
		}
		var adventureMap models.AdventureMapConfig
		if err := tx.First(&adventureMap, "key = ? AND enabled = ?", zone.MapKey, true).Error; err != nil {
			return ErrZoneLocked
		}
		now := service.Now()
		session := models.AdventureExplorationSession{ID: uuid.NewString(), AccountID: accountID, PetID: pet.ID, CommunityID: communityID, MapKey: adventureMap.Key, ZoneKey: zone.Key, EncounterKey: encounter.EncounterKey, StageKey: state.StageKey, NodeRole: encounter.NodeRole, Status: "active", StartedAt: now}
		pet.Hunger -= zone.HungerCost
		pet.Readiness -= zone.ReadinessCost
		pet.Status = "探索"
		if err := tx.Save(&pet).Error; err != nil {
			return err
		}
		if err := tx.Create(&session).Error; err != nil {
			return err
		}
		result.Session, result.Encounter = session, encounter
		result.StageName = stage.Name
		result.Goal = nodeGoalText(stage, state)
		if encounter.EncounterType == "monster" {
			combat, err := service.startAdventureCombatTx(tx, &session, &pet, encounter.TargetKey)
			if err != nil {
				return err
			}
			result.Combat = combat
			return nil
		}
		session.Status, session.FinishedAt = "completed", &now
		pet.Status = "空闲"
		if err := tx.Save(&session).Error; err != nil {
			return err
		}
		if err := tx.Save(&pet).Error; err != nil {
			return err
		}
		progress, _, err := service.advanceNodeClueTx(tx, accountID, &state, stage, encounter)
		if err != nil {
			return err
		}
		result.Progress = progress
		return nil
	})
	return &result, err
}

func nodeGoalText(stage models.AdventureExplorationStageConfig, state models.PlayerAdventureNodeProgress) string {
	if stage.RequiredClues > 0 {
		return fmt.Sprintf("%s · 线索 %d/%d", stage.Name, minInt(state.ClueProgress, stage.RequiredClues), stage.RequiredClues)
	}
	return stage.Name
}

// ResolveNodeEvent applies a configured choice exactly once. The event body
// and choices come from the stored snapshot, not mutable live configuration.
func (service *Service) ResolveNodeEvent(ctx context.Context, accountID, selector string) (*NodeEventResolution, error) {
	var result NodeEventResolution
	err := gameplay.WithTransactionRetry(ctx, service.DB, func(tx *gorm.DB) error {
		pending, ok, err := service.pendingNodeEventTx(tx, accountID, "")
		if err != nil || !ok {
			if err != nil {
				return err
			}
			return ErrNoPendingAdventureEvent
		}
		view, err := decodeStoryEventSnapshot(pending)
		if err != nil {
			return err
		}
		choice, found := resolveEventChoice(view.Choices, selector)
		if !found {
			return ErrInvalidAdventureEventChoice
		}
		var node models.PlayerAdventureNodeProgress
		if err := tx.First(&node, "account_id = ? AND zone_key = ?", accountID, pending.ZoneKey).Error; err != nil {
			return err
		}
		var stage models.AdventureExplorationStageConfig
		if choice.NextStageKey == "" {
			now := service.Now()
			node.CompletedAt, node.StageKey, node.ClueProgress, node.SideMisses = &now, "completed", 0, 0
			stage = models.AdventureExplorationStageConfig{Key: "completed", Name: "区域收束", ProgressStart: 100, ProgressEnd: 100}
		} else {
			if err := tx.First(&stage, "key = ? AND enabled = ?", choice.NextStageKey, true).Error; err != nil {
				return err
			}
			node.StageKey, node.ClueProgress, node.SideMisses = stage.Key, 0, 0
		}
		node.ActionSequence++
		node.UpdatedAt = service.Now()
		if err := tx.Save(&node).Error; err != nil {
			return err
		}
		now := service.Now()
		pending.Status, pending.SelectedChoiceKey, pending.ResolvedAt, pending.UpdatedAt = "resolved", choice.ChoiceKey, &now, now
		if err := tx.Save(&pending).Error; err != nil {
			return err
		}
		progress, err := service.syncNodeZoneProgressTx(tx, accountID, node, stage)
		if err != nil {
			return err
		}
		result = NodeEventResolution{Event: view.Event, Choice: choice, Progress: progress, Stage: stage}
		return nil
	})
	return &result, err
}

func minInt(left, right int) int {
	if left < right {
		return left
	}
	return right
}

func maxInt(left, right int) int {
	if left > right {
		return left
	}
	return right
}

// nodeProgressForZone is read-only and used by the map view.
func (service *Service) nodeProgressForZone(ctx context.Context, accountID string, zone models.AdventureZoneConfig) (models.PlayerZoneProgress, string, string, error) {
	var progress models.PlayerZoneProgress
	if err := service.DB.WithContext(ctx).Limit(1).Find(&progress, "account_id = ? AND zone_key = ?", accountID, zone.Key).Error; err != nil {
		return progress, "", "", err
	}
	var node models.PlayerAdventureNodeProgress
	lookup := service.DB.WithContext(ctx).Limit(1).Find(&node, "account_id = ? AND zone_key = ?", accountID, zone.Key)
	if lookup.Error != nil || lookup.RowsAffected == 0 || node.CompletedAt != nil {
		if node.CompletedAt != nil {
			return progress, "区域收束", "已完成，可继续探索刷取资源", nil
		}
		return progress, "踏勘准备", "开始探索以触发区域事件", lookup.Error
	}
	var stage models.AdventureExplorationStageConfig
	if err := service.DB.WithContext(ctx).First(&stage, "key = ?", node.StageKey).Error; err != nil {
		return progress, "", "", err
	}
	return progress, stage.Name, nodeGoalText(stage, node), nil
}

// resolveEventChoice accepts the player-facing one-based ordinal first. The
// fallback for ChoiceKey keeps buttons sent before this protocol change usable
// until their pending event is resolved; new cards never expose that key.
func resolveEventChoice(choices []models.AdventureStoryEventChoiceConfig, selector string) (models.AdventureStoryEventChoiceConfig, bool) {
	selector = strings.TrimSpace(selector)
	if ordinal, err := strconv.Atoi(selector); err == nil {
		if ordinal >= 1 && ordinal <= len(choices) {
			return choices[ordinal-1], true
		}
		return models.AdventureStoryEventChoiceConfig{}, false
	}
	for _, choice := range choices {
		if choice.ChoiceKey == selector {
			return choice, true
		}
	}
	return models.AdventureStoryEventChoiceConfig{}, false
}

func eventChoiceCommand(ordinal int) string {
	return "探索选择 " + strconv.Itoa(ordinal)
}
