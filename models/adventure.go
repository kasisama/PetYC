package models

import "time"

// AdventureMapConfig is the top-level, administrator-authored world map.
type AdventureMapConfig struct {
	Key              string `gorm:"primaryKey;size:64" json:"key"`
	Name             string `gorm:"size:64;not null" json:"name"`
	Region           string `gorm:"size:64;not null" json:"region"`
	Description      string `gorm:"type:text" json:"description"`
	Image            string `gorm:"size:255" json:"image"`
	RecommendedLevel int    `gorm:"not null;default:1" json:"recommended_level"`
	Enabled          bool   `gorm:"not null;default:true;index" json:"enabled"`
	SortOrder        int    `gorm:"not null;default:0;index" json:"sort_order"`
}

// AdventureZoneConfig is an explorable area inside a map. ExpeditionUnlockObjectiveKey
// identifies the permanent objective that unlocks unattended expeditions for the zone.
type AdventureZoneConfig struct {
	Key                          string `gorm:"primaryKey;size:64" json:"key"`
	MapKey                       string `gorm:"size:64;not null;index" json:"map_key"`
	Name                         string `gorm:"size:64;not null" json:"name"`
	Description                  string `gorm:"type:text" json:"description"`
	Image                        string `gorm:"size:255" json:"image"`
	RecommendedLevel             int    `gorm:"not null;default:1" json:"recommended_level"`
	DifficultyPermille           int    `gorm:"not null;default:1000" json:"difficulty_permille"`
	HungerCost                   int64  `gorm:"not null;default:0" json:"hunger_cost"`
	ReadinessCost                int    `gorm:"not null;default:0" json:"readiness_cost"`
	ExpeditionUnlockObjectiveKey string `gorm:"size:64" json:"expedition_unlock_objective_key"`
	Enabled                      bool   `gorm:"not null;default:true;index" json:"enabled"`
	SortOrder                    int    `gorm:"not null;default:0;index" json:"sort_order"`
}

type AdventureZonePrerequisiteConfig struct {
	ID                  uint   `gorm:"primaryKey" json:"id"`
	ZoneKey             string `gorm:"size:64;not null;uniqueIndex:idx_adventure_zone_prerequisite" json:"zone_key"`
	PrerequisiteZoneKey string `gorm:"size:64;not null;uniqueIndex:idx_adventure_zone_prerequisite" json:"prerequisite_zone_key"`
}

// ObjectiveType supports enter, monster_kill, elite_kill, landmark and boss_kill.
type AdventureObjectiveConfig struct {
	Key           string `gorm:"primaryKey;size:64" json:"key"`
	ZoneKey       string `gorm:"size:64;not null;index" json:"zone_key"`
	Name          string `gorm:"size:96;not null" json:"name"`
	ObjectiveType string `gorm:"size:32;not null" json:"objective_type"`
	TargetKey     string `gorm:"size:64" json:"target_key"`
	RequiredCount int64  `gorm:"not null;default:1" json:"required_count"`
	Weight        int    `gorm:"not null;default:1" json:"weight"`
	CodexCategory string `gorm:"size:64" json:"codex_category"`
	CodexEntry    string `gorm:"size:64" json:"codex_entry"`
	CodexProgress int    `gorm:"not null;default:0" json:"codex_progress"`
	Enabled       bool   `gorm:"not null;default:true;index" json:"enabled"`
	SortOrder     int    `gorm:"not null;default:0;index" json:"sort_order"`
}

type AdventureMonsterConfig struct {
	Key               string `gorm:"primaryKey;size:64" json:"key"`
	Name              string `gorm:"size:64;not null" json:"name"`
	Description       string `gorm:"type:text" json:"description"`
	Image             string `gorm:"size:255" json:"image"`
	Level             int    `gorm:"not null;default:1" json:"level"`
	MaxHealth         int64  `gorm:"not null" json:"max_health"`
	Attack            int64  `gorm:"not null" json:"attack"`
	Defense           int64  `gorm:"not null" json:"defense"`
	Wisdom            int64  `gorm:"not null;default:0" json:"wisdom"`
	AdventureXP       int64  `gorm:"not null;default:0" json:"adventure_xp"`
	AIProfile         string `gorm:"size:32;not null;default:'balanced'" json:"ai_profile"`
	FixedLootPoolKey  string `gorm:"size:64" json:"fixed_loot_pool_key"`
	RandomLootPoolKey string `gorm:"size:64" json:"random_loot_pool_key"`
	Elite             bool   `gorm:"not null;default:false;index" json:"elite"`
	Enabled           bool   `gorm:"not null;default:true;index" json:"enabled"`
}

// AdventureSkillConfig is shared by pet and monster battle actors.
type AdventureSkillConfig struct {
	Key              string `gorm:"primaryKey;size:64" json:"key"`
	Name             string `gorm:"size:64;not null" json:"name"`
	Description      string `gorm:"type:text" json:"description"`
	PowerPermille    int    `gorm:"not null;default:1000" json:"power_permille"`
	WisdomPermille   int    `gorm:"not null;default:0" json:"wisdom_permille"`
	AccuracyPermille int    `gorm:"not null;default:1000" json:"accuracy_permille"`
	CooldownTurns    int    `gorm:"not null;default:0" json:"cooldown_turns"`
	EffectType       string `gorm:"size:32" json:"effect_type"`
	EffectValue      int    `gorm:"not null;default:0" json:"effect_value"`
	Enabled          bool   `gorm:"not null;default:true;index" json:"enabled"`
}

type AdventureMonsterSkillConfig struct {
	ID         uint   `gorm:"primaryKey" json:"id"`
	MonsterKey string `gorm:"size:64;not null;uniqueIndex:idx_adventure_monster_skill" json:"monster_key"`
	SkillKey   string `gorm:"size:64;not null;uniqueIndex:idx_adventure_monster_skill" json:"skill_key"`
	Weight     int    `gorm:"not null;default:1" json:"weight"`
	SortOrder  int    `gorm:"not null;default:0" json:"sort_order"`
}

// EncounterType supports monster, landmark and safe.
type AdventureEncounterConfig struct {
	ID            uint   `gorm:"primaryKey" json:"id"`
	ZoneKey       string `gorm:"size:64;not null;uniqueIndex:idx_adventure_encounter" json:"zone_key"`
	EncounterKey  string `gorm:"size:64;not null;uniqueIndex:idx_adventure_encounter" json:"encounter_key"`
	EncounterType string `gorm:"size:32;not null" json:"encounter_type"`
	TargetKey     string `gorm:"size:64" json:"target_key"`
	Name          string `gorm:"size:96;not null" json:"name"`
	Description   string `gorm:"type:text" json:"description"`
	Weight        int    `gorm:"not null;default:1" json:"weight"`
	Enabled       bool   `gorm:"not null;default:true;index" json:"enabled"`
	SortOrder     int    `gorm:"not null;default:0" json:"sort_order"`
}

// AdventureEncounterEffectConfig turns landmarks and safe encounters into
// auditable gameplay effects instead of display-only text.
type AdventureEncounterEffectConfig struct {
	ID           uint   `gorm:"primaryKey" json:"id"`
	EncounterKey string `gorm:"size:64;not null;uniqueIndex:idx_adventure_encounter_effect" json:"encounter_key"`
	EffectType   string `gorm:"size:32;not null;uniqueIndex:idx_adventure_encounter_effect" json:"effect_type"`
	TargetKey    string `gorm:"size:64;uniqueIndex:idx_adventure_encounter_effect" json:"target_key"`
	MinValue     int64  `gorm:"not null;default:0" json:"min_value"`
	MaxValue     int64  `gorm:"not null;default:0" json:"max_value"`
	Weight       int    `gorm:"not null;default:1" json:"weight"`
	Enabled      bool   `gorm:"not null;default:true;index" json:"enabled"`
}

type AdventureLootPoolConfig struct {
	Key             string `gorm:"primaryKey;size:64" json:"key"`
	Name            string `gorm:"size:96;not null" json:"name"`
	Rolls           int    `gorm:"not null;default:1" json:"rolls"`
	AllowDuplicates bool   `gorm:"not null;default:false" json:"allow_duplicates"`
}

// RewardType supports item, currency, equipment and blueprint_fragment.
// Items and currencies use the account-wide inventory and wallet domains.
type AdventureLootEntryConfig struct {
	ID             uint   `gorm:"primaryKey" json:"id"`
	PoolKey        string `gorm:"size:64;not null;index" json:"pool_key"`
	RewardType     string `gorm:"size:32;not null" json:"reward_type"`
	RewardKey      string `gorm:"size:64;not null" json:"reward_key"`
	MinQuantity    int64  `gorm:"not null;default:1" json:"min_quantity"`
	MaxQuantity    int64  `gorm:"not null;default:1" json:"max_quantity"`
	Weight         int    `gorm:"not null;default:1" json:"weight"`
	Guaranteed     bool   `gorm:"not null;default:false" json:"guaranteed"`
	FirstClearOnly bool   `gorm:"not null;default:false" json:"first_clear_only"`
	SortOrder      int    `gorm:"not null;default:0" json:"sort_order"`
}

// CurrencyConfig is the stable catalog for every account-wide currency.
type CurrencyConfig struct {
	Key         string `gorm:"primaryKey;size:64" json:"key"`
	Name        string `gorm:"size:64;not null" json:"name"`
	Description string `gorm:"type:text" json:"description"`
	Image       string `gorm:"size:255" json:"image"`
	Builtin     bool   `gorm:"not null;default:false" json:"builtin"`
	Enabled     bool   `gorm:"not null;default:true;index" json:"enabled"`
	SortOrder   int    `gorm:"not null;default:0;index" json:"sort_order"`
}

// AdventureShopItemConfig is a fixed listing bought with a configured account currency.
// LimitType supports none, daily, weekly, season and lifetime.
type AdventureShopItemConfig struct {
	Key           string `gorm:"primaryKey;size:64" json:"key"`
	Name          string `gorm:"size:96;not null" json:"name"`
	Description   string `gorm:"type:text" json:"description"`
	Image         string `gorm:"size:255" json:"image"`
	ProductType   string `gorm:"size:32;not null;index" json:"product_type"`
	ProductKey    string `gorm:"size:64;not null;index" json:"product_key"`
	Quantity      int64  `gorm:"not null;default:1" json:"quantity"`
	Price         int64  `gorm:"not null" json:"price"`
	CurrencyKey   string `gorm:"size:64;not null;default:'journey_badge';index" json:"currency_key"`
	LimitType     string `gorm:"size:16;not null;default:'none'" json:"limit_type"`
	LimitQuantity int64  `gorm:"not null;default:0" json:"limit_quantity"`
	Enabled       bool   `gorm:"not null;default:true;index" json:"enabled"`
	SortOrder     int    `gorm:"not null;default:0;index" json:"sort_order"`
}

type AdventureExpeditionConfig struct {
	ZoneKey             string `gorm:"primaryKey;size:64" json:"zone_key"`
	Name                string `gorm:"size:96;not null" json:"name"`
	Description         string `gorm:"type:text" json:"description"`
	DurationMinutes     int64  `gorm:"not null" json:"duration_minutes"`
	HungerCost          int64  `gorm:"not null;default:0" json:"hunger_cost"`
	ReadinessCost       int    `gorm:"not null;default:0" json:"readiness_cost"`
	RequiredItem        string `gorm:"size:64" json:"required_item"`
	RequiredQuantity    int64  `gorm:"not null;default:0" json:"required_quantity"`
	FixedLootPoolKey    string `gorm:"size:64" json:"fixed_loot_pool_key"`
	RandomLootPoolKey   string `gorm:"size:64" json:"random_loot_pool_key"`
	AdventureXP         int64  `gorm:"not null;default:0" json:"adventure_xp"`
	EventProgressPoints int64  `gorm:"not null;default:0" json:"event_progress_points"`
	RecommendedPower    int64  `gorm:"not null;default:0" json:"recommended_power"`
	StartImage          string `gorm:"size:255" json:"start_image"`
	EndImage            string `gorm:"size:255" json:"end_image"`
	Enabled             bool   `gorm:"not null;default:true;index" json:"enabled"`
}

type AdventureBossConfig struct {
	Key                      string    `gorm:"primaryKey;size:64" json:"key"`
	MapKey                   string    `gorm:"size:64;not null;index" json:"map_key"`
	ZoneKey                  string    `gorm:"size:64;not null;index" json:"zone_key"`
	MonsterKey               string    `gorm:"size:64;not null" json:"monster_key"`
	Name                     string    `gorm:"size:96;not null" json:"name"`
	Description              string    `gorm:"type:text" json:"description"`
	ScheduleAnchor           time.Time `gorm:"not null" json:"schedule_anchor"`
	SpawnIntervalMinutes     int64     `gorm:"not null" json:"spawn_interval_minutes"`
	ActiveDurationMinutes    int64     `gorm:"not null" json:"active_duration_minutes"`
	RecommendedLevel         int       `gorm:"not null;default:1" json:"recommended_level"`
	MaxHealth                int64     `gorm:"not null" json:"max_health"`
	Attack                   int64     `gorm:"not null" json:"attack"`
	Defense                  int64     `gorm:"not null" json:"defense"`
	Wisdom                   int64     `gorm:"not null;default:0" json:"wisdom"`
	ChallengeCooldownMinutes int64     `gorm:"not null;default:0" json:"challenge_cooldown_minutes"`
	ChallengeLimit           int       `gorm:"not null;default:0" json:"challenge_limit"`
	MinimumContribution      int64     `gorm:"not null;default:1" json:"minimum_contribution"`
	DefeatedLootPoolKey      string    `gorm:"size:64" json:"defeated_loot_pool_key"`
	ExpiredLootPoolKey       string    `gorm:"size:64" json:"expired_loot_pool_key"`
	Enabled                  bool      `gorm:"not null;default:true;index" json:"enabled"`
}

type AdventureBossRewardTierConfig struct {
	ID          uint   `gorm:"primaryKey" json:"id"`
	BossKey     string `gorm:"size:64;not null;uniqueIndex:idx_adventure_boss_reward_tier" json:"boss_key"`
	Threshold   int64  `gorm:"not null;uniqueIndex:idx_adventure_boss_reward_tier" json:"threshold"`
	LootPoolKey string `gorm:"size:64;not null" json:"loot_pool_key"`
	Description string `gorm:"size:255" json:"description"`
}

// Slot supports weapon, armor and treasure (秘宝).
type EquipmentTemplateConfig struct {
	Key             string `gorm:"primaryKey;size:64" json:"key"`
	Name            string `gorm:"size:96;not null" json:"name"`
	Description     string `gorm:"type:text" json:"description"`
	Image           string `gorm:"size:255" json:"image"`
	Slot            string `gorm:"size:24;not null;index" json:"slot"`
	Rarity          string `gorm:"size:24;not null;index" json:"rarity"`
	RequiredLevel   int    `gorm:"not null;default:1" json:"required_level"`
	BaseAttack      int64  `gorm:"not null;default:0" json:"base_attack"`
	BaseDefense     int64  `gorm:"not null;default:0" json:"base_defense"`
	BaseHealth      int64  `gorm:"not null;default:0" json:"base_health"`
	BaseWisdom      int64  `gorm:"not null;default:0" json:"base_wisdom"`
	AffixPoolKey    string `gorm:"size:64" json:"affix_pool_key"`
	MinAffixes      int    `gorm:"not null;default:0" json:"min_affixes"`
	MaxAffixes      int    `gorm:"not null;default:0" json:"max_affixes"`
	SalvageItem     string `gorm:"size:64" json:"salvage_item"`
	SalvageQuantity int64  `gorm:"not null;default:0" json:"salvage_quantity"`
	Enabled         bool   `gorm:"not null;default:true;index" json:"enabled"`
}

type EquipmentAffixConfig struct {
	Key       string `gorm:"primaryKey;size:64" json:"key"`
	PoolKey   string `gorm:"size:64;not null;index" json:"pool_key"`
	Name      string `gorm:"size:64;not null" json:"name"`
	Attribute string `gorm:"size:32;not null" json:"attribute"`
	MinValue  int64  `gorm:"not null" json:"min_value"`
	MaxValue  int64  `gorm:"not null" json:"max_value"`
	Weight    int    `gorm:"not null;default:1" json:"weight"`
	Enabled   bool   `gorm:"not null;default:true;index" json:"enabled"`
}

type EquipmentRecipeConfig struct {
	EquipmentKey string `gorm:"primaryKey;size:64" json:"equipment_key"`
	// BlueprintFragmentItem is kept for backwards-compatible decoding of
	// v0.0.1 profiles. New rewards identify blueprints by EquipmentKey.
	BlueprintFragmentItem string `gorm:"size:64;not null" json:"blueprint_fragment_item"`
	BlueprintFragments    int64  `gorm:"not null" json:"blueprint_fragments"`
	CurrencyCost          int64  `gorm:"not null;default:0" json:"currency_cost"`
	Enabled               bool   `gorm:"not null;default:true;index" json:"enabled"`
}

type EquipmentRecipeMaterialConfig struct {
	ID           uint   `gorm:"primaryKey" json:"id"`
	EquipmentKey string `gorm:"size:64;not null;uniqueIndex:idx_equipment_recipe_material" json:"equipment_key"`
	ItemName     string `gorm:"size:64;not null;uniqueIndex:idx_equipment_recipe_material" json:"item_name"`
	Quantity     int64  `gorm:"not null" json:"quantity"`
}

// LiveEventChoiceConfig replaces positional string choices with stable keys.
type LiveEventChoiceConfig struct {
	ID          uint   `gorm:"primaryKey" json:"id"`
	EventKey    string `gorm:"size:64;not null;uniqueIndex:idx_live_event_choice" json:"event_key"`
	ChoiceKey   string `gorm:"size:64;not null;uniqueIndex:idx_live_event_choice" json:"choice_key"`
	Label       string `gorm:"size:128;not null" json:"label"`
	EffectType  string `gorm:"size:64;not null" json:"effect_type"`
	EffectValue int    `gorm:"not null" json:"effect_value"`
	SortOrder   int    `gorm:"not null;default:0" json:"sort_order"`
}

type LiveEventExpeditionSourceConfig struct {
	ID       uint   `gorm:"primaryKey" json:"id"`
	EventKey string `gorm:"size:64;not null;uniqueIndex:idx_live_event_expedition_source" json:"event_key"`
	ZoneKey  string `gorm:"size:64;not null;uniqueIndex:idx_live_event_expedition_source" json:"zone_key"`
}

// Player/runtime models below are intentionally excluded from configuration profiles.
type PlayerAdventureProgress struct {
	AccountID string `gorm:"primaryKey;size:36" json:"account_id"`
	Level     int    `gorm:"not null;default:1" json:"level"`
	XP        int64  `gorm:"not null;default:0" json:"xp"`
	UpdatedAt time.Time
}

type PlayerZoneProgress struct {
	ID                 uint   `gorm:"primaryKey"`
	AccountID          string `gorm:"size:36;not null;uniqueIndex:idx_player_zone_progress"`
	ZoneKey            string `gorm:"size:64;not null;uniqueIndex:idx_player_zone_progress"`
	ExplorationPercent int    `gorm:"not null;default:0"`
	ExpeditionUnlocked bool   `gorm:"not null;default:false"`
	FirstClearedAt     *time.Time
	UpdatedAt          time.Time
}

type PlayerObjectiveProgress struct {
	ID           uint   `gorm:"primaryKey"`
	AccountID    string `gorm:"size:36;not null;uniqueIndex:idx_player_objective_progress"`
	ObjectiveKey string `gorm:"size:64;not null;uniqueIndex:idx_player_objective_progress"`
	Progress     int64  `gorm:"not null;default:0"`
	CompletedAt  *time.Time
	UpdatedAt    time.Time
}

type AdventureExplorationSession struct {
	ID           string `gorm:"primaryKey;size:36"`
	AccountID    string `gorm:"size:36;not null;index:idx_adventure_exploration_account_status"`
	PetID        string `gorm:"size:36;not null;index"`
	CommunityID  string `gorm:"size:320;index"`
	MapKey       string `gorm:"size:64;not null"`
	ZoneKey      string `gorm:"size:64;not null"`
	EncounterKey string `gorm:"size:64"`
	Status       string `gorm:"size:24;not null;index:idx_adventure_exploration_account_status"`
	StartedAt    time.Time
	FinishedAt   *time.Time
}

type AdventureCombatSession struct {
	ID              string    `gorm:"primaryKey;size:36"`
	AccountID       string    `gorm:"size:36;not null;index:idx_adventure_combat_account_status"`
	PetID           string    `gorm:"size:36;not null;index"`
	CommunityID     string    `gorm:"size:320;index"`
	ExplorationID   string    `gorm:"size:36;index"`
	BossInstanceID  string    `gorm:"size:128;index"`
	MonsterKey      string    `gorm:"size:64;not null"`
	Status          string    `gorm:"size:24;not null;index:idx_adventure_combat_account_status"`
	Round           int       `gorm:"not null;default:1"`
	PlayerHealth    int64     `gorm:"not null"`
	MonsterHealth   int64     `gorm:"not null"`
	PlayerDefending bool      `gorm:"not null;default:false"`
	CooldownsJSON   string    `gorm:"type:text;not null;default:'{}'"`
	ExpiresAt       time.Time `gorm:"not null;index"`
	StartedAt       time.Time
	FinishedAt      *time.Time
}

type AdventureCombatTurn struct {
	ID            string `gorm:"primaryKey;size:36"`
	SessionID     string `gorm:"size:36;not null;uniqueIndex:idx_adventure_combat_turn"`
	Round         int    `gorm:"not null;uniqueIndex:idx_adventure_combat_turn"`
	ActionKey     string `gorm:"size:128;not null;uniqueIndex"`
	PlayerAction  string `gorm:"size:64;not null"`
	MonsterAction string `gorm:"size:64"`
	PlayerDamage  int64  `gorm:"not null;default:0"`
	MonsterDamage int64  `gorm:"not null;default:0"`
	Result        string `gorm:"size:24;not null"`
	RollsJSON     string `gorm:"type:text;not null;default:'{}'"`
	CreatedAt     time.Time
}

type PlayerEquipment struct {
	ID            string `gorm:"primaryKey;size:36" json:"id"`
	AccountID     string `gorm:"size:36;not null;index" json:"account_id"`
	TemplateKey   string `gorm:"size:64;not null;index" json:"template_key"`
	Rarity        string `gorm:"size:24;not null" json:"rarity"`
	AffixesJSON   string `gorm:"type:text;not null;default:'[]'" json:"affixes_json"`
	EquippedSlot  string `gorm:"size:24;index" json:"equipped_slot"`
	EquippedPetID string `gorm:"size:36;index" json:"equipped_pet_id"`
	Locked        bool   `gorm:"not null;default:false" json:"locked"`
	Source        string `gorm:"size:128" json:"source"`
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

type PlayerBlueprintProgress struct {
	ID           uint   `gorm:"primaryKey"`
	AccountID    string `gorm:"size:36;not null;uniqueIndex:idx_player_blueprint_progress"`
	EquipmentKey string `gorm:"size:64;not null;uniqueIndex:idx_player_blueprint_progress"`
	Fragments    int64  `gorm:"not null;default:0"`
	Unlocked     bool   `gorm:"not null;default:false"`
	UnlockedAt   *time.Time
	UpdatedAt    time.Time
}

type AdventureShopPurchase struct {
	ID              string    `gorm:"primaryKey;size:36" json:"id"`
	AccountID       string    `gorm:"size:36;not null;index;uniqueIndex:idx_adventure_shop_idempotency" json:"account_id"`
	ShopItemKey     string    `gorm:"size:64;not null;index" json:"shop_item_key"`
	PurchaseUnits   int64     `gorm:"not null" json:"purchase_units"`
	GrantedQuantity int64     `gorm:"not null" json:"granted_quantity"`
	Cost            int64     `gorm:"not null" json:"cost"`
	CurrencyKey     string    `gorm:"size:64;not null" json:"currency_key"`
	PeriodKey       string    `gorm:"size:32;not null;index" json:"period_key"`
	IdempotencyKey  string    `gorm:"size:128;not null;uniqueIndex:idx_adventure_shop_idempotency" json:"idempotency_key"`
	CreatedAt       time.Time `gorm:"not null;index" json:"created_at"`
}

type AdventureExpeditionRun struct {
	ID                  string `gorm:"primaryKey;size:36"`
	AccountID           string `gorm:"size:36;not null;index:idx_adventure_expedition_account_status"`
	PetID               string `gorm:"size:36;not null;index"`
	CommunityID         string `gorm:"size:320;index"`
	MapKey              string `gorm:"size:64;not null"`
	ZoneKey             string `gorm:"size:64;not null"`
	Status              string `gorm:"size:24;not null;index:idx_adventure_expedition_account_status"`
	SnapshotJSON        string `gorm:"type:text;not null"`
	EventKey            string `gorm:"size:64;index"`
	EventProgressPoints int64  `gorm:"not null;default:0"`
	StartedAt           time.Time
	EndsAt              time.Time `gorm:"not null;index"`
	ClaimedAt           *time.Time
}

type AdventureBossInstance struct {
	ID            string `gorm:"primaryKey;size:160"`
	BossKey       string `gorm:"size:64;not null;index"`
	CommunityID   string `gorm:"size:320;not null;index"`
	WindowKey     string `gorm:"size:64;not null"`
	Status        string `gorm:"size:24;not null;index"`
	MaxHealth     int64  `gorm:"not null"`
	CurrentHealth int64  `gorm:"not null"`
	SnapshotJSON  string `gorm:"type:text;not null"`
	SpawnedAt     time.Time
	ExpiresAt     time.Time `gorm:"not null;index"`
	DefeatedAt    *time.Time
}

type AdventureBossContribution struct {
	ID              uint   `gorm:"primaryKey"`
	BossInstanceID  string `gorm:"size:160;not null;uniqueIndex:idx_adventure_boss_contribution"`
	AccountID       string `gorm:"size:36;not null;uniqueIndex:idx_adventure_boss_contribution"`
	Damage          int64  `gorm:"not null;default:0"`
	Challenges      int    `gorm:"not null;default:0"`
	LastChallengeAt *time.Time
	UpdatedAt       time.Time
}

type AdventureBossRewardClaim struct {
	ID             string `gorm:"primaryKey;size:36"`
	BossInstanceID string `gorm:"size:160;not null;uniqueIndex:idx_adventure_boss_reward_claim"`
	AccountID      string `gorm:"size:36;not null;uniqueIndex:idx_adventure_boss_reward_claim"`
	RewardJSON     string `gorm:"type:text;not null"`
	ClaimedAt      time.Time
}

type EquipmentCraftRecord struct {
	ID            string `gorm:"primaryKey;size:36"`
	AccountID     string `gorm:"size:36;not null;index"`
	EquipmentID   string `gorm:"size:36;not null;uniqueIndex"`
	TemplateKey   string `gorm:"size:64;not null"`
	MaterialsJSON string `gorm:"type:text;not null"`
	CreatedAt     time.Time
}
