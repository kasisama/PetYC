package expedition

import (
	"context"
	"errors"
	"testing"
	"time"

	"qq-pet-saas/gameplay"
	"qq-pet-saas/models"
)

func seedAdventurePlayer(t *testing.T, service *Service, dbAccount, name string) {
	t.Helper()
	if err := service.DB.Create(&models.PlayerAccount{ID: dbAccount}).Error; err != nil {
		t.Fatal(err)
	}
	pet := models.PetProfile{AccountID: dbAccount, PetType: "诺诺", CurrentForm: "诺诺", Name: name, Status: "空闲", Health: 100, HealthMax: 100, Hunger: 100, HungerMax: 100, Readiness: 100, Strength: 20, Defense: 10, Wisdom: 10}
	if err := service.DB.Create(&pet).Error; err != nil {
		t.Fatal(err)
	}
}

func TestExploreCombatUnlocksZoneAndGrantsEquipment(t *testing.T) {
	service, db, _ := newTestService(t)
	service.RandomIntn = func(int) (int, error) { return 0, nil }
	seedAdventurePlayer(t, service, "adventure-player", "探险诺诺")
	rows := []any{
		&models.AdventureMapConfig{Key: "starter", Name: "初始探索区", Region: "原野", RecommendedLevel: 1, Enabled: true},
		&models.AdventureZoneConfig{Key: "forest", MapKey: "starter", Name: "森林边缘", RecommendedLevel: 1, DifficultyPermille: 1000, HungerCost: 1, ReadinessCost: 1, ExpeditionUnlockObjectiveKey: "forest-guardian", Enabled: true},
		&models.AdventureMonsterConfig{Key: "slime", Name: "露珠团", Level: 1, MaxHealth: 5, Attack: 1, AdventureXP: 120, FixedLootPoolKey: "slime-fixed", Enabled: true},
		&models.AdventureEncounterConfig{ZoneKey: "forest", EncounterKey: "slime-encounter", EncounterType: "monster", TargetKey: "slime", Name: "露珠团", Weight: 1, Enabled: true},
		&models.AdventureObjectiveConfig{Key: "forest-guardian", ZoneKey: "forest", Name: "击败露珠团", ObjectiveType: "monster_kill", TargetKey: "slime", RequiredCount: 1, Weight: 100, Enabled: true},
		&models.AdventureLootPoolConfig{Key: "slime-fixed", Name: "露珠团固定奖励", Rolls: 0},
		&models.EquipmentTemplateConfig{Key: "twig-sword", Name: "嫩枝短剑", Slot: "weapon", Rarity: "common", RequiredLevel: 1, BaseAttack: 3, Enabled: true},
		&models.AdventureLootEntryConfig{PoolKey: "slime-fixed", RewardType: "equipment", RewardKey: "twig-sword", MinQuantity: 1, MaxQuantity: 1, Guaranteed: true},
	}
	for _, row := range rows {
		if err := db.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}

	exploration, err := service.ExploreZone(context.Background(), "adventure-player", "forest")
	if err != nil {
		t.Fatal(err)
	}
	if exploration.Combat == nil || exploration.Encounter.TargetKey != "slime" {
		t.Fatalf("expected monster combat, got %#v", exploration)
	}
	result, err := service.CombatAction(context.Background(), "adventure-player", "message-1", "attack")
	if err != nil {
		t.Fatal(err)
	}
	if result.Turn.Result != "victory" || !result.ExpeditionUnlocked || result.ZoneProgress != 100 {
		t.Fatalf("unexpected combat result: %#v", result)
	}
	if result.AdventureLevel != 2 {
		t.Fatalf("expected level 2, got %d", result.AdventureLevel)
	}
	var equipment models.PlayerEquipment
	if err = db.First(&equipment, "account_id = ?", "adventure-player").Error; err != nil {
		t.Fatal(err)
	}
	if equipment.TemplateKey != "twig-sword" {
		t.Fatalf("unexpected equipment: %#v", equipment)
	}
	var pet models.PetProfile
	if err = db.First(&pet, "account_id = ?", "adventure-player").Error; err != nil {
		t.Fatal(err)
	}
	if pet.Status != "空闲" {
		t.Fatalf("pet should be idle after victory, got %s", pet.Status)
	}
}

func TestBlueprintUnlockCraftEquipAndSalvage(t *testing.T) {
	service, db, _ := newTestService(t)
	service.RandomIntn = func(int) (int, error) { return 0, nil }
	seedAdventurePlayer(t, service, "crafter", "工匠诺诺")
	if err := db.Create(&[]models.ItemConfig{{Name: "短剑蓝图碎片", Status: "active"}, {Name: "铁矿", Status: "active"}, {Name: "装备粉尘", Status: "active"}}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&models.EquipmentTemplateConfig{Key: "iron-sword", Name: "铁剑", Slot: "weapon", Rarity: "rare", RequiredLevel: 1, BaseAttack: 8, SalvageItem: "装备粉尘", SalvageQuantity: 2, Enabled: true}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&models.EquipmentRecipeConfig{EquipmentKey: "iron-sword", BlueprintFragmentItem: "短剑蓝图碎片", BlueprintFragments: 2, Enabled: true}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&models.EquipmentRecipeMaterialConfig{EquipmentKey: "iron-sword", ItemName: "铁矿", Quantity: 3}).Error; err != nil {
		t.Fatal(err)
	}
	if err := service.grantBlueprintFragmentsTx(db, "crafter", "短剑蓝图碎片", 2); err != nil {
		t.Fatal(err)
	}
	var blueprint models.PlayerBlueprintProgress
	if err := db.First(&blueprint, "account_id = ? AND equipment_key = ?", "crafter", "iron-sword").Error; err != nil {
		t.Fatal(err)
	}
	if !blueprint.Unlocked || blueprint.Fragments != 2 {
		t.Fatalf("unexpected blueprint progress: %#v", blueprint)
	}
	if err := gameplay.NewInventoryService(db).Credit(context.Background(), "crafter", "铁矿", 3); err != nil {
		t.Fatal(err)
	}
	crafted, err := service.CraftEquipment(context.Background(), "crafter", "iron-sword")
	if err != nil {
		t.Fatal(err)
	}
	equipped, err := service.Equip(context.Background(), "crafter", crafted.ID)
	if err != nil {
		t.Fatal(err)
	}
	if equipped.EquippedSlot != "weapon" {
		t.Fatalf("unexpected equipped slot: %s", equipped.EquippedSlot)
	}
	stats, err := service.EquippedStatsTx(db, "crafter")
	if err != nil || stats.Attack != 8 {
		t.Fatalf("unexpected equipment stats: %#v err=%v", stats, err)
	}
	if err = service.Unequip(context.Background(), "crafter", crafted.ID); err != nil {
		t.Fatal(err)
	}
	if err = service.SalvageEquipment(context.Background(), "crafter", crafted.ID); err != nil {
		t.Fatal(err)
	}
	var dust models.GlobalInventoryItem
	if err = db.First(&dust, "account_id = ? AND item_name = ?", "crafter", "装备粉尘").Error; err != nil {
		t.Fatal(err)
	}
	if dust.Quantity != 2 {
		t.Fatalf("expected salvage dust, got %d", dust.Quantity)
	}
}

func TestAdventureActionIsIdempotent(t *testing.T) {
	service, db, _ := newTestService(t)
	service.RandomIntn = func(int) (int, error) { return 0, nil }
	seedAdventurePlayer(t, service, "repeat-player", "重复诺诺")
	for _, row := range []any{
		&models.AdventureMapConfig{Key: "map", Name: "地图", Region: "区域", RecommendedLevel: 1, Enabled: true},
		&models.AdventureZoneConfig{Key: "zone", MapKey: "map", Name: "区域", RecommendedLevel: 1, DifficultyPermille: 1000, Enabled: true},
		&models.AdventureMonsterConfig{Key: "monster", Name: "怪物", Level: 1, MaxHealth: 100, Attack: 1, Enabled: true},
		&models.AdventureEncounterConfig{ZoneKey: "zone", EncounterKey: "encounter", EncounterType: "monster", TargetKey: "monster", Name: "怪物", Weight: 1, Enabled: true},
	} {
		if err := db.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}
	if _, err := service.ExploreZone(context.Background(), "repeat-player", "zone"); err != nil {
		t.Fatal(err)
	}
	first, err := service.CombatAction(context.Background(), "repeat-player", "same-action", "attack")
	if err != nil {
		t.Fatal(err)
	}
	second, err := service.CombatAction(context.Background(), "repeat-player", "same-action", "attack")
	if err != nil {
		t.Fatal(err)
	}
	if first.Turn.ID != second.Turn.ID || first.Session.MonsterHealth != second.Session.MonsterHealth {
		t.Fatalf("duplicate action changed combat: first=%#v second=%#v", first, second)
	}
}

func TestUnlockedZoneExpeditionSnapshotsSelectedEventAndSettlesAfterEventEnds(t *testing.T) {
	service, db, now := newTestService(t)
	service.RandomIntn = func(int) (int, error) { return 999, nil }
	seedAdventurePlayer(t, service, "expedition-player", "远征诺诺")
	for _, row := range []any{
		&models.ItemConfig{Name: "林地样本", Status: "active"},
		&models.ItemConfig{Name: "活动徽章", Status: "active"},
		&models.AdventureMapConfig{Key: "map", Name: "地图", Region: "原野", RecommendedLevel: 1, Enabled: true},
		&models.AdventureZoneConfig{Key: "zone", MapKey: "map", Name: "区域", RecommendedLevel: 1, DifficultyPermille: 1000, Enabled: true},
		&models.PlayerZoneProgress{AccountID: "expedition-player", ZoneKey: "zone", ExpeditionUnlocked: true},
		&models.AdventureLootPoolConfig{Key: "expedition-fixed", Name: "固定", Rolls: 0},
		&models.AdventureLootEntryConfig{PoolKey: "expedition-fixed", RewardType: "item", RewardKey: "林地样本", MinQuantity: 1, MaxQuantity: 1, Guaranteed: true},
		&models.AdventureExpeditionConfig{ZoneKey: "zone", Name: "区域远征", DurationMinutes: 10, FixedLootPoolKey: "expedition-fixed", AdventureXP: 10, EventProgressPoints: 7, Enabled: true},
		&models.LiveEventConfig{Key: "forest-week", Name: "森林周", Region: "森林", StoryChoices: `["一","二","三"]`, ProgressSourceMode: "selected", StartsAt: *now, EndsAt: now.Add(30 * time.Minute), Active: true},
		&models.LiveEventExpeditionSourceConfig{EventKey: "forest-week", ZoneKey: "zone"},
		&models.RewardTrackConfig{EventKey: "forest-week", Milestone: 7, ItemName: "活动徽章", Quantity: 1},
	} {
		if err := db.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}
	run, err := service.StartAdventureExpedition(context.Background(), "expedition-player", "zone")
	if err != nil {
		t.Fatal(err)
	}
	if run.EventKey != "forest-week" || run.EventProgressPoints != 7 {
		t.Fatalf("event was not snapshotted: %#v", run)
	}
	*now = now.Add(40 * time.Minute)
	result, err := service.ClaimAdventureExpedition(context.Background(), "expedition-player")
	if err != nil {
		t.Fatal(err)
	}
	if result.EventProgress != 7 || len(result.EventRewards) != 1 {
		t.Fatalf("unexpected event settlement: %#v", result)
	}
	var sample, badge models.GlobalInventoryItem
	if err = db.First(&sample, "account_id = ? AND item_name = ?", "expedition-player", "林地样本").Error; err != nil {
		t.Fatal(err)
	}
	if err = db.First(&badge, "account_id = ? AND item_name = ?", "expedition-player", "活动徽章").Error; err != nil {
		t.Fatal(err)
	}
	if sample.Quantity != 1 || badge.Quantity != 1 {
		t.Fatalf("unexpected inventory: sample=%d badge=%d", sample.Quantity, badge.Quantity)
	}
}

func TestScheduledCommunityBossSharesHealthAndClaimsContributionRewardsOnce(t *testing.T) {
	service, db, now := newTestService(t)
	service.RandomIntn = func(int) (int, error) { return 0, nil }
	seedAdventurePlayer(t, service, "boss-player", "首领诺诺")
	for _, row := range []any{
		&models.ItemConfig{Name: "首领徽记", Status: "active"},
		&models.AdventureMapConfig{Key: "boss-map", Name: "首领地图", Region: "山谷", RecommendedLevel: 1, Enabled: true},
		&models.AdventureZoneConfig{Key: "boss-zone", MapKey: "boss-map", Name: "首领谷地", RecommendedLevel: 1, DifficultyPermille: 1000, Enabled: true},
		&models.AdventureObjectiveConfig{Key: "boss-objective", ZoneKey: "boss-zone", Name: "击败岩甲兽", ObjectiveType: "boss_kill", TargetKey: "rock-boss", RequiredCount: 1, Weight: 100, Enabled: true},
		&models.AdventureMonsterConfig{Key: "map-boss-monster", Name: "岩甲兽", Level: 1, MaxHealth: 5, Attack: 1, AdventureXP: 20, Enabled: true},
		&models.AdventureLootPoolConfig{Key: "boss-reward", Name: "首领奖励", Rolls: 0},
		&models.AdventureLootEntryConfig{PoolKey: "boss-reward", RewardType: "item", RewardKey: "首领徽记", MinQuantity: 1, MaxQuantity: 1, Guaranteed: true},
		&models.AdventureBossConfig{Key: "rock-boss", MapKey: "boss-map", ZoneKey: "boss-zone", MonsterKey: "map-boss-monster", Name: "岩甲兽", ScheduleAnchor: now.Add(-time.Minute), SpawnIntervalMinutes: 60, ActiveDurationMinutes: 30, RecommendedLevel: 1, MaxHealth: 5, Attack: 1, MinimumContribution: 1, DefeatedLootPoolKey: "boss-reward", Enabled: true},
	} {
		if err := db.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}
	bosses, err := service.ListActiveAdventureBosses(context.Background(), "onebot:app:group:100")
	if err != nil || len(bosses) != 1 {
		t.Fatalf("expected active boss, bosses=%#v err=%v", bosses, err)
	}
	combat, err := service.StartAdventureBossChallenge(context.Background(), "boss-player", "onebot:app:group:100", "rock-boss")
	if err != nil {
		t.Fatal(err)
	}
	result, err := service.CombatAction(context.Background(), "boss-player", "boss-hit-1", "attack")
	if err != nil {
		t.Fatal(err)
	}
	if result.Turn.Result != "victory" {
		t.Fatalf("expected boss victory, got %#v", result)
	}
	var instance models.AdventureBossInstance
	if err = db.First(&instance, "id = ?", combat.BossInstanceID).Error; err != nil {
		t.Fatal(err)
	}
	if instance.Status != "defeated" || instance.CurrentHealth != 0 {
		t.Fatalf("boss state not shared: %#v", instance)
	}
	claimed, err := service.ClaimAdventureBossReward(context.Background(), "boss-player", instance.ID)
	if err != nil {
		t.Fatal(err)
	}
	if claimed.Contribution.Damage < 1 || len(claimed.Rewards) != 1 {
		t.Fatalf("unexpected claim: %#v", claimed)
	}
	var objective models.PlayerObjectiveProgress
	if err = db.First(&objective, "account_id = ? AND objective_key = ?", "boss-player", "boss-objective").Error; err != nil || objective.CompletedAt == nil {
		t.Fatalf("boss reward did not complete permanent exploration objective: %#v err=%v", objective, err)
	}
	if _, err = service.ClaimAdventureBossReward(context.Background(), "boss-player", instance.ID); !errors.Is(err, ErrBossRewardUnavailable) {
		t.Fatalf("duplicate claim should fail, got %v", err)
	}
}

func TestZoneDifficultyAndConfiguredMonsterSkillDriveCombat(t *testing.T) {
	service, db, _ := newTestService(t)
	service.RandomIntn = func(limit int) (int, error) {
		if limit > 1 {
			return 1, nil
		}
		return 0, nil
	}
	seedAdventurePlayer(t, service, "difficulty-player", "探索诺诺")
	rows := []any{
		&models.AdventureMapConfig{Key: "difficulty-map", Name: "险境", Region: "山地", RecommendedLevel: 1, Enabled: true},
		&models.AdventureZoneConfig{Key: "difficulty-zone", MapKey: "difficulty-map", Name: "险境入口", RecommendedLevel: 1, DifficultyPermille: 2000, Enabled: true},
		&models.AdventureMonsterConfig{Key: "skilled-monster", Name: "战术兽", Level: 1, MaxHealth: 999, Attack: 10, Defense: 2, Enabled: true},
		&models.AdventureSkillConfig{Key: "monster-smash", Name: "重击", PowerPermille: 1500, AccuracyPermille: 1000, Enabled: true},
		&models.AdventureMonsterSkillConfig{MonsterKey: "skilled-monster", SkillKey: "monster-smash", Weight: 100},
		&models.AdventureEncounterConfig{ZoneKey: "difficulty-zone", EncounterKey: "skilled", EncounterType: "monster", TargetKey: "skilled-monster", Name: "战术兽", Weight: 1, Enabled: true},
	}
	for _, row := range rows {
		if err := db.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}
	exploration, err := service.ExploreZone(context.Background(), "difficulty-player", "difficulty-zone")
	if err != nil {
		t.Fatal(err)
	}
	if exploration.Combat == nil || exploration.Combat.MonsterHealth != 1998 {
		t.Fatalf("zone difficulty did not scale monster health: %#v", exploration.Combat)
	}
	result, err := service.CombatAction(context.Background(), "difficulty-player", "difficulty-action", "defend")
	if err != nil {
		t.Fatal(err)
	}
	if result.Turn.MonsterAction != "skill:monster-smash" || result.Turn.MonsterDamage <= 0 {
		t.Fatalf("configured monster skill did not drive action: %#v", result.Turn)
	}
}
