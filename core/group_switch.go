package core

import (
	"errors"

	"gorm.io/gorm"
	"qq-pet-saas/database"
	"qq-pet-saas/models"
)

func oneBotGroupEnabled(groupID int64) bool {
	if groupID == 0 || database.DB == nil {
		return true
	}
	var groupSwitch models.GroupSwitch
	err := database.DB.First(&groupSwitch, "group_id = ?", groupID).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return true
	}
	return err == nil && groupSwitch.IsActive
}
