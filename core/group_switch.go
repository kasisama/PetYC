package core

import (
	"errors"
	"hash/fnv"
	"strconv"

	"gorm.io/gorm"
	"qq-pet-saas/database"
	"qq-pet-saas/models"
)

// GroupEnabled applies the same persisted switch to OneBot groups, QQ official
// groups and QQ guilds. Direct messages deliberately do not use a group switch.
// Official scene IDs are opaque strings, so their stable key lives in the
// negative int64 range and cannot collide with normal positive OneBot group IDs.
func GroupEnabled(event InboundEvent) bool {
	if event.SceneType == SceneDirect || database.DB == nil {
		return true
	}
	if event.Platform == PlatformOneBot {
		groupID, err := strconv.ParseInt(event.SpaceID, 10, 64)
		if err != nil || groupID == 0 {
			return true
		}
		return oneBotGroupEnabled(groupID)
	}
	if event.Platform != PlatformQQGroup && event.Platform != PlatformQQGuild {
		return true
	}
	key := officialSceneKey(event.Platform, event.SpaceID)
	var groupSwitch models.GroupSwitch
	err := database.DB.First(&groupSwitch, "group_id = ?", key).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		groupSwitch = models.GroupSwitch{
			GroupID: key, Platform: string(event.Platform), SpaceID: event.SpaceID,
			GroupName: officialSceneLabel(event), IsActive: true,
		}
		if createErr := database.DB.Create(&groupSwitch).Error; createErr != nil {
			// A concurrent first message may have created the same deterministic row.
			if retryErr := database.DB.First(&groupSwitch, "group_id = ?", key).Error; retryErr != nil {
				return true
			}
		}
		return groupSwitch.IsActive
	}
	if err != nil || groupSwitch.Platform != string(event.Platform) || groupSwitch.SpaceID != event.SpaceID {
		return true
	}
	return groupSwitch.IsActive
}

func officialSceneKey(platform Platform, spaceID string) int64 {
	hash := fnv.New64a()
	_, _ = hash.Write([]byte(string(platform) + "\x00" + spaceID))
	// Keep the key within JavaScript's exact integer range because the admin UI
	// sends it back through JSON when toggling an official scene.
	value := int64(hash.Sum64() & ((1 << 52) - 1))
	if value == 0 {
		value = 1
	}
	return -value
}

func officialSceneLabel(event InboundEvent) string {
	if event.Platform == PlatformQQGuild {
		return "QQ 官方频道 " + event.SpaceID
	}
	return "QQ 官方群 " + event.SpaceID
}

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
