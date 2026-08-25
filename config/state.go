package config

import (
	"errors"
	"time"

	"gorm.io/gorm"
	"qq-pet-saas/models"
)

const configStateID uint = 1

type ConfigStatus struct {
	DBRevision      uint64     `json:"db_revision"`
	LoadedRevision  uint64     `json:"loaded_revision"`
	PendingReload   bool       `json:"pending_reload"`
	SavedAt         *time.Time `json:"saved_at"`
	LoadedAt        *time.Time `json:"loaded_at"`
	ActiveProfileID string     `json:"active_profile_id"`
	ProfileDirty    bool       `json:"profile_dirty"`
}

func getOrCreateConfigState(tx *gorm.DB) (*models.AdminConfigState, error) {
	if tx == nil {
		return nil, errors.New("数据库未初始化")
	}
	var state models.AdminConfigState
	result := tx.Limit(1).Find(&state, configStateID)
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected > 0 {
		return &state, nil
	}
	state = models.AdminConfigState{ID: configStateID}
	if err := tx.Create(&state).Error; err != nil {
		return nil, err
	}
	return &state, nil
}

func MarkConfigSaved(tx *gorm.DB) error {
	state, err := getOrCreateConfigState(tx)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	return tx.Model(state).Updates(map[string]interface{}{
		"db_revision":   gorm.Expr("db_revision + 1"),
		"saved_at":      now,
		"profile_dirty": true,
	}).Error
}

func MarkConfigLoaded(db *gorm.DB, revisions ...uint64) error {
	return db.Transaction(func(tx *gorm.DB) error {
		state, err := getOrCreateConfigState(tx)
		if err != nil {
			return err
		}
		now := time.Now().UTC()
		dbRevision := state.DBRevision
		if dbRevision == 0 {
			dbRevision = 1
		}
		loadedRevision := dbRevision
		if len(revisions) > 0 {
			loadedRevision = revisions[0]
			if loadedRevision == 0 {
				loadedRevision = 1
			}
		}
		if loadedRevision < state.LoadedRevision {
			loadedRevision = state.LoadedRevision
		}
		if loadedRevision > dbRevision {
			loadedRevision = dbRevision
		}
		updates := map[string]interface{}{
			"db_revision":     dbRevision,
			"loaded_revision": loadedRevision,
			"loaded_at":       now,
		}
		if state.SavedAt == nil {
			updates["saved_at"] = now
		}
		return tx.Model(state).Updates(updates).Error
	})
}

func MarkConfigReset(db *gorm.DB) error {
	return db.Transaction(func(tx *gorm.DB) error {
		state, err := getOrCreateConfigState(tx)
		if err != nil {
			return err
		}
		now := time.Now().UTC()
		nextRevision := state.DBRevision + 1
		if nextRevision == 0 {
			nextRevision = 1
		}
		return tx.Model(state).Updates(map[string]interface{}{
			"db_revision":     nextRevision,
			"loaded_revision": nextRevision,
			"saved_at":        now,
			"loaded_at":       now,
		}).Error
	})
}

func GetConfigStatus(db *gorm.DB) (ConfigStatus, error) {
	state, err := getOrCreateConfigState(db)
	if err != nil {
		return ConfigStatus{}, err
	}
	return ConfigStatus{
		DBRevision:      state.DBRevision,
		LoadedRevision:  state.LoadedRevision,
		PendingReload:   state.DBRevision != state.LoadedRevision,
		SavedAt:         state.SavedAt,
		LoadedAt:        state.LoadedAt,
		ActiveProfileID: state.ActiveProfileID,
		ProfileDirty:    state.ProfileDirty,
	}, nil
}
