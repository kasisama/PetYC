package models

import (
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type PlayerAccount struct {
	ID          string `gorm:"primaryKey;size:36"`
	ActivePetID string `gorm:"size:36;index" json:"active_pet_id"`
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type PlayerIdentity struct {
	ID        uint   `gorm:"primaryKey"`
	AccountID string `gorm:"size:36;not null;index"`
	Platform  string `gorm:"size:24;not null;uniqueIndex:idx_player_identity"`
	AppID     string `gorm:"size:64;not null;uniqueIndex:idx_player_identity"`
	SceneType string `gorm:"size:24;not null;uniqueIndex:idx_player_identity"`
	ScopeID   string `gorm:"size:128;not null;uniqueIndex:idx_player_identity"`
	SubjectID string `gorm:"size:128;not null;uniqueIndex:idx_player_identity"`
	CreatedAt time.Time
}

type PetProfile struct {
	ID          string `gorm:"primaryKey;size:36" json:"pet_id"`
	AccountID   string `gorm:"size:36;not null;index" json:"account_id"`
	PetType     string `gorm:"size:64;not null"`
	Name        string `gorm:"size:64;not null"`
	CurrentForm string `gorm:"size:64;not null"`
	Role        string `gorm:"size:24;not null;default:'探索者'"`
	Stance      string `gorm:"size:24;not null;default:'探索'"`
	Status      string `gorm:"size:24;not null;default:'空闲';index"`
	Mood        string `gorm:"size:24;not null;default:'一般'"`
	MoodPoints  int    `gorm:"not null;default:50"`
	Readiness   int    `gorm:"not null;default:100"`
	Affection   int64  `gorm:"not null;default:0"`
	Growth      int64  `gorm:"not null;default:0"`
	BondLevel   int    `gorm:"not null;default:1"`
	Health      int64  `gorm:"not null;default:100"`
	HealthMax   int64  `gorm:"not null;default:100"`
	Hunger      int64  `gorm:"not null;default:100"`
	HungerMax   int64  `gorm:"not null;default:100"`
	Wisdom      int64  `gorm:"not null;default:10"`
	Strength    int64  `gorm:"not null;default:10"`
	Defense     int64  `gorm:"not null;default:10"`
	Traits      string `gorm:"type:text"`
	Skills      string `gorm:"type:text"`
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func (pet *PetProfile) BeforeCreate(_ *gorm.DB) error {
	if strings.TrimSpace(pet.ID) == "" {
		pet.ID = uuid.NewString()
	}
	return nil
}

type GlobalInventoryItem struct {
	ID        uint   `gorm:"primaryKey"`
	AccountID string `gorm:"size:36;not null;uniqueIndex:idx_global_inventory"`
	ItemKey   string `gorm:"size:64;not null;uniqueIndex:idx_global_inventory;index" json:"item_key"`
	ItemName  string `gorm:"size:64;not null;index"`
	Quantity  int64  `gorm:"not null;default:0"`
	UpdatedAt time.Time
}

// PlayerWallet is the single currency balance store for a player. Currency is
// deliberately kept out of PetProfile because it belongs to the account, not
// to a particular pet form.
type PlayerWallet struct {
	ID          uint   `gorm:"primaryKey"`
	AccountID   string `gorm:"size:36;not null;uniqueIndex:idx_player_wallet"`
	CurrencyKey string `gorm:"size:64;not null;uniqueIndex:idx_player_wallet"`
	Balance     int64  `gorm:"not null;default:0"`
	UpdatedAt   time.Time
}

type WalletLedger struct {
	ID           string `gorm:"primaryKey;size:36"`
	AccountID    string `gorm:"size:36;not null;index"`
	CurrencyKey  string `gorm:"size:64;not null;index"`
	Delta        int64  `gorm:"not null"`
	BalanceAfter int64  `gorm:"not null"`
	Reason       string `gorm:"size:64;not null;index"`
	ReferenceKey string `gorm:"size:128;index"`
	CreatedAt    time.Time
}

type CompanionJournal struct {
	ID        uint   `gorm:"primaryKey"`
	AccountID string `gorm:"size:36;not null;uniqueIndex:idx_companion_journal"`
	PetID     string `gorm:"size:36;not null;uniqueIndex:idx_companion_journal;index"`
	Day       string `gorm:"size:10;not null;uniqueIndex:idx_companion_journal"`
	Action    string `gorm:"size:32;not null"`
	CreatedAt time.Time
}

// CompanionActionDaily persists daily limits and cooldowns that used to live
// in process memory. It keeps behavior consistent after restart and across all
// platform adapters.
type CompanionActionDaily struct {
	ID             uint   `gorm:"primaryKey"`
	AccountID      string `gorm:"size:36;not null;uniqueIndex:idx_companion_action_day"`
	PetID          string `gorm:"size:36;not null;uniqueIndex:idx_companion_action_day;index"`
	Day            string `gorm:"size:10;not null;uniqueIndex:idx_companion_action_day"`
	Action         string `gorm:"size:32;not null;uniqueIndex:idx_companion_action_day"`
	Count          int64  `gorm:"not null;default:0"`
	GrowthGranted  int64  `gorm:"not null;default:0"`
	AffectionGiven int64  `gorm:"not null;default:0"`
	LastAt         time.Time
	UpdatedAt      time.Time
}

// ActivityRun is the single timed-activity record for learning, training,
// fitness and work. Reward inputs are snapshotted when an activity starts so
// a later config reload cannot change an already running result.
type ActivityRun struct {
	ID              string `gorm:"primaryKey;size:36"`
	AccountID       string `gorm:"size:36;not null;index:idx_activity_account_status"`
	PetID           string `gorm:"size:36;not null;index"`
	Kind            string `gorm:"size:24;not null;index"`
	ConfigKey       string `gorm:"size:64;not null"`
	InputItem       string `gorm:"size:64"`
	Status          string `gorm:"size:24;not null;index:idx_activity_account_status"`
	HungerCost      int64  `gorm:"not null;default:0"`
	RewardAttribute string `gorm:"size:24"`
	RewardAmount    int64  `gorm:"not null;default:0"`
	RewardGrowth    int64  `gorm:"not null;default:0"`
	RewardCurrency  int64  `gorm:"not null;default:0"`
	RewardItems     string `gorm:"size:255"`
	StartImage      string `gorm:"size:255"`
	EndImage        string `gorm:"size:255"`
	StartsAt        time.Time
	EndsAt          time.Time `gorm:"index"`
	ClaimedAt       *time.Time
	CreatedAt       time.Time
}

// ItemUseRecord makes consumable effects auditable and idempotent when a
// platform retries delivery of the same inbound message.
type ItemUseRecord struct {
	ID             string `gorm:"primaryKey;size:36"`
	AccountID      string `gorm:"size:36;not null;uniqueIndex:idx_item_use_idempotency"`
	PetID          string `gorm:"size:36;not null;index"`
	IdempotencyKey string `gorm:"size:255;not null;uniqueIndex:idx_item_use_idempotency"`
	ItemName       string `gorm:"size:64;not null"`
	Quantity       int64  `gorm:"not null"`
	EffectType     string `gorm:"size:24;not null"`
	AppliedAmount  int64  `gorm:"not null"`
	BeforeValue    int64  `gorm:"not null"`
	AfterValue     int64  `gorm:"not null"`
	CreatedAt      time.Time
}

type ExpeditionTemplateConfig struct {
	Tier             int    `gorm:"primaryKey"`
	Name             string `gorm:"size:64;not null"`
	Enabled          bool   `gorm:"not null;default:true;index"`
	DurationMinutes  int64  `gorm:"not null"`
	HungerCost       int64  `gorm:"not null;default:0"`
	ReadinessCost    int    `gorm:"not null;default:0"`
	RequiredItem     string `gorm:"size:64"`
	RequiredQuantity int64  `gorm:"not null;default:0"`
	RewardItem       string `gorm:"size:64;not null"`
	RewardQuantity   int64  `gorm:"not null"`
	RewardRecords    int64  `gorm:"not null"`
	RewardGrowth     int64  `gorm:"not null"`
	RewardCurrency   int64  `gorm:"not null;default:0"`
	CodexCategory    string `gorm:"size:32"`
	CodexEntry       string `gorm:"size:64"`
	CodexProgress    int    `gorm:"not null;default:0"`
	StartImage       string `gorm:"size:255"`
	EndImage         string `gorm:"size:255"`
	Description      string `gorm:"type:text"`
}

type ExpeditionRun struct {
	ID             string `gorm:"primaryKey;size:36"`
	AccountID      string `gorm:"size:36;not null;index:idx_expedition_account_status"`
	PetID          string `gorm:"size:36;not null;index"`
	Tier           int    `gorm:"not null"`
	Name           string `gorm:"size:64;not null"`
	Stance         string `gorm:"size:24;not null"`
	Status         string `gorm:"size:24;not null;index:idx_expedition_account_status"`
	RewardItem     string `gorm:"size:64;not null"`
	RewardQuantity int64  `gorm:"not null"`
	RewardRecords  int64  `gorm:"not null"`
	RewardGrowth   int64  `gorm:"not null"`
	RewardCurrency int64  `gorm:"not null;default:0"`
	CodexCategory  string `gorm:"size:32"`
	CodexEntry     string `gorm:"size:64"`
	CodexProgress  int    `gorm:"not null;default:0"`
	HungerCost     int64  `gorm:"not null;default:0"`
	ReadinessCost  int    `gorm:"not null;default:0"`
	RequiredItem   string `gorm:"size:64"`
	RequiredQty    int64  `gorm:"not null;default:0"`
	BonusPercent   int    `gorm:"not null;default:0"`
	BonusText      string `gorm:"size:255"`
	StartImage     string `gorm:"size:255"`
	EndImage       string `gorm:"size:255"`
	StartedAt      time.Time
	EndsAt         time.Time
	ClaimedAt      *time.Time
}

type CodexEntry struct {
	ID        uint   `gorm:"primaryKey"`
	AccountID string `gorm:"size:36;not null;uniqueIndex:idx_codex_entry"`
	Category  string `gorm:"size:32;not null;uniqueIndex:idx_codex_entry"`
	EntryKey  string `gorm:"size:64;not null;uniqueIndex:idx_codex_entry"`
	Progress  int    `gorm:"not null;default:0"`
	UpdatedAt time.Time
}

type Community struct {
	ID                   string `gorm:"primaryKey;size:320"`
	Platform             string `gorm:"size:24;not null"`
	AppID                string `gorm:"size:64;not null"`
	SceneType            string `gorm:"size:24;not null"`
	SpaceID              string `gorm:"size:128;not null"`
	Level                int    `gorm:"not null;default:1"`
	Materials            int64  `gorm:"not null;default:0"`
	NotificationsEnabled bool   `gorm:"not null;default:true"`
	CreatedAt            time.Time
	UpdatedAt            time.Time
}

type CommunityMember struct {
	ID           uint   `gorm:"primaryKey"`
	CommunityID  string `gorm:"size:320;not null;uniqueIndex:idx_community_member"`
	AccountID    string `gorm:"size:36;not null;uniqueIndex:idx_community_member"`
	Contribution int64  `gorm:"not null;default:0"`
	RescueCount  int64  `gorm:"not null;default:0"`
	JoinedAt     time.Time
}

type ExpeditionSquad struct {
	ID          string `gorm:"primaryKey;size:36"`
	CommunityID string `gorm:"size:320;not null;uniqueIndex:idx_squad_name"`
	Name        string `gorm:"size:64;not null;uniqueIndex:idx_squad_name"`
	LeaderID    string `gorm:"size:36;not null"`
	Research    int64  `gorm:"not null;default:0"`
	MaxMembers  int    `gorm:"not null;default:12"`
	CreatedAt   time.Time
}

type SquadMember struct {
	ID          uint   `gorm:"primaryKey"`
	SquadID     string `gorm:"size:36;not null;index"`
	CommunityID string `gorm:"size:320;not null;uniqueIndex:idx_squad_member"`
	AccountID   string `gorm:"size:36;not null;uniqueIndex:idx_squad_member"`
	JoinedAt    time.Time
}

type IdentityBindToken struct {
	TokenHash string `gorm:"primaryKey;size:64"`
	AccountID string `gorm:"size:36;not null;index"`
	ExpiresAt time.Time
	UsedAt    *time.Time
	CreatedAt time.Time
}

type NotificationPreference struct {
	AccountID string `gorm:"primaryKey;size:36"`
	Enabled   bool   `gorm:"not null"`
	UpdatedAt time.Time
}

// NotificationJob is a durable outbox entry for proactive player messages.
// IdempotencyKey prevents duplicated notifications when a platform retries an
// inbound command or a transaction is replayed.
type NotificationJob struct {
	ID             string     `gorm:"primaryKey;size:36"`
	AccountID      string     `gorm:"size:36;not null;index"`
	IdempotencyKey string     `gorm:"size:255;not null;uniqueIndex"`
	Kind           string     `gorm:"size:64;not null;index"`
	Platform       string     `gorm:"size:24;not null"`
	SceneType      string     `gorm:"size:24;not null"`
	AppID          string     `gorm:"size:64"`
	SpaceID        string     `gorm:"size:128"`
	RoomID         string     `gorm:"size:128"`
	ActorID        string     `gorm:"size:128"`
	ActorName      string     `gorm:"size:128"`
	MessageKey     string     `gorm:"size:128"`
	Message        string     `gorm:"type:text;not null"`
	Status         string     `gorm:"size:24;not null;index:idx_notification_due"`
	Attempts       int        `gorm:"not null;default:0"`
	MaxAttempts    int        `gorm:"not null;default:8"`
	NextAttemptAt  time.Time  `gorm:"not null;index:idx_notification_due"`
	LockedAt       *time.Time `gorm:"index"`
	LastError      string     `gorm:"type:text"`
	SentAt         *time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type CommunityBoss struct {
	ID          string `gorm:"primaryKey;size:360"`
	CommunityID string `gorm:"size:320;not null;index"`
	WeekKey     string `gorm:"size:16;not null"`
	Name        string `gorm:"size:64;not null"`
	MaxHP       int64  `gorm:"not null"`
	CurrentHP   int64  `gorm:"not null"`
	Defeated    bool   `gorm:"not null;default:false"`
	UpdatedAt   time.Time
}

type BossContribution struct {
	ID        uint   `gorm:"primaryKey"`
	BossID    string `gorm:"size:360;not null;uniqueIndex:idx_boss_contribution"`
	AccountID string `gorm:"size:36;not null;uniqueIndex:idx_boss_contribution"`
	Damage    int64  `gorm:"not null;default:0"`
	UpdatedAt time.Time
}

type CommunityFacility struct {
	ID          uint   `gorm:"primaryKey"`
	CommunityID string `gorm:"size:320;not null;uniqueIndex:idx_community_facility"`
	Name        string `gorm:"size:64;not null;uniqueIndex:idx_community_facility"`
	Level       int    `gorm:"not null;default:1"`
	Progress    int64  `gorm:"not null;default:0"`
	UpdatedAt   time.Time
}

type SeasonVote struct {
	ID          uint   `gorm:"primaryKey"`
	SeasonKey   string `gorm:"size:32;not null;uniqueIndex:idx_season_vote"`
	CommunityID string `gorm:"size:320;not null;uniqueIndex:idx_season_vote"`
	AccountID   string `gorm:"size:36;not null;uniqueIndex:idx_season_vote"`
	Choice      int    `gorm:"not null"`
	ChoiceKey   string `gorm:"size:64;index"`
	UpdatedAt   time.Time
}

type CommunityHelpRequest struct {
	Code        string `gorm:"primaryKey;size:8"`
	CommunityID string `gorm:"size:320;not null;index"`
	RequesterID string `gorm:"size:36;not null;index"`
	ItemName    string `gorm:"size:64;not null"`
	Quantity    int64  `gorm:"not null"`
	Fulfilled   int64  `gorm:"not null;default:0"`
	Status      string `gorm:"size:16;not null;default:'open';index"`
	ExpiresAt   time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type HelpGiftLog struct {
	ID          uint   `gorm:"primaryKey"`
	RequestCode string `gorm:"size:8;not null;index"`
	CommunityID string `gorm:"size:320;not null;index"`
	DonorID     string `gorm:"size:36;not null;index:idx_help_gift_day"`
	ItemName    string `gorm:"size:64;not null"`
	Quantity    int64  `gorm:"not null"`
	Day         string `gorm:"size:10;not null;index:idx_help_gift_day"`
	CreatedAt   time.Time
}

// HelpGiftDailyQuota is the atomic daily allowance counter. It prevents two
// concurrent support requests from both passing a read-then-write limit check.
type HelpGiftDailyQuota struct {
	ID        uint   `gorm:"primaryKey"`
	DonorID   string `gorm:"size:36;not null;uniqueIndex:idx_help_gift_daily_quota"`
	Day       string `gorm:"size:10;not null;uniqueIndex:idx_help_gift_daily_quota"`
	Quantity  int64  `gorm:"not null;default:0"`
	UpdatedAt time.Time
}

func (HelpGiftDailyQuota) TableName() string {
	return "help_gift_daily_quotas"
}

type PetBehaviorProfile struct {
	PetID     string `gorm:"primaryKey;size:36"`
	AccountID string `gorm:"size:36;not null;index"`
	Explore   int64  `gorm:"not null;default:0"`
	Care      int64  `gorm:"not null;default:0"`
	Support   int64  `gorm:"not null;default:0"`
	Trait     string `gorm:"size:32"`
	UpdatedAt time.Time
}

// BeforeCreate keeps transitional callers safe while pet_id is being rolled
// through every command path. New code should always set PetID explicitly.
func (profile *PetBehaviorProfile) BeforeCreate(tx *gorm.DB) error {
	if strings.TrimSpace(profile.PetID) != "" || strings.TrimSpace(profile.AccountID) == "" {
		return nil
	}
	var account PlayerAccount
	if err := tx.Select("active_pet_id").First(&account, "id = ?", profile.AccountID).Error; err == nil && account.ActivePetID != "" {
		profile.PetID = account.ActivePetID
		return nil
	}
	var pet PetProfile
	if err := tx.Select("id").Where("account_id = ?", profile.AccountID).Order("created_at asc, id asc").First(&pet).Error; err != nil {
		return err
	}
	profile.PetID = pet.ID
	return nil
}

type LiveEventConfig struct {
	ID           uint   `gorm:"primaryKey" json:"id"`
	Key          string `gorm:"size:64;not null;uniqueIndex" json:"key"`
	Name         string `gorm:"size:64;not null" json:"name"`
	Region       string `gorm:"size:32;not null" json:"region"`
	StoryChoices string `gorm:"type:text;not null" json:"story_choices"`
	// ProgressSourceMode is all_expeditions or selected. The legacy
	// StoryChoices column remains during the v0.0.1 migration and is converted
	// into LiveEventChoiceConfig rows.
	ProgressSourceMode string    `gorm:"size:32;not null;default:'all_expeditions'" json:"progress_source_mode"`
	StartsAt           time.Time `gorm:"not null;index" json:"starts_at"`
	EndsAt             time.Time `gorm:"not null;index" json:"ends_at"`
	Active             bool      `gorm:"not null;default:true;index" json:"active"`
	UpdatedAt          time.Time `json:"updated_at"`
}

type RewardTrackConfig struct {
	ID          uint   `gorm:"primaryKey" json:"id"`
	EventKey    string `gorm:"size:64;not null;uniqueIndex:idx_reward_track_reward" json:"event_key"`
	Milestone   int64  `gorm:"not null;uniqueIndex:idx_reward_track_reward" json:"milestone"`
	RewardType  string `gorm:"size:24;not null;uniqueIndex:idx_reward_track_reward" json:"reward_type"`
	RewardKey   string `gorm:"size:64;not null;uniqueIndex:idx_reward_track_reward" json:"reward_key"`
	RewardName  string `gorm:"size:64;not null" json:"reward_name"`
	Quantity    int64  `gorm:"not null" json:"quantity"`
	Description string `gorm:"size:255" json:"description"`
}

// EventProgress stores the account's current milestone progress for a
// published event. Progress sources are deduplicated separately so retries do
// not move the track twice.
type EventProgress struct {
	ID        uint   `gorm:"primaryKey"`
	EventKey  string `gorm:"size:64;not null;uniqueIndex:idx_event_progress"`
	AccountID string `gorm:"size:36;not null;uniqueIndex:idx_event_progress"`
	Progress  int64  `gorm:"not null;default:0"`
	UpdatedAt time.Time
}

type EventProgressGrant struct {
	ID        string `gorm:"primaryKey;size:36"`
	EventKey  string `gorm:"size:64;not null;uniqueIndex:idx_event_progress_source"`
	AccountID string `gorm:"size:36;not null;uniqueIndex:idx_event_progress_source"`
	SourceKey string `gorm:"size:128;not null;uniqueIndex:idx_event_progress_source"`
	Delta     int64  `gorm:"not null"`
	CreatedAt time.Time
}

type EventRewardClaim struct {
	ID         string `gorm:"primaryKey;size:36"`
	EventKey   string `gorm:"size:64;not null;uniqueIndex:idx_event_reward_claim"`
	AccountID  string `gorm:"size:36;not null;uniqueIndex:idx_event_reward_claim"`
	Milestone  int64  `gorm:"not null;uniqueIndex:idx_event_reward_claim"`
	RewardType string `gorm:"size:24;not null;uniqueIndex:idx_event_reward_claim"`
	RewardKey  string `gorm:"size:64;not null;uniqueIndex:idx_event_reward_claim"`
	RewardName string `gorm:"size:64;not null"`
	Quantity   int64  `gorm:"not null"`
	ClaimedAt  time.Time
}

type ChanceGameConfig struct {
	GameKey        string `gorm:"primaryKey;size:32" json:"game_key"`
	Name           string `gorm:"size:64;not null" json:"name"`
	Enabled        bool   `gorm:"not null;default:true;index" json:"enabled"`
	CostCurrency   int64  `gorm:"not null;default:0" json:"cost_currency"`
	CostItem       string `gorm:"size:64" json:"cost_item"`
	CostQuantity   int64  `gorm:"not null;default:0" json:"cost_quantity"`
	DailyLimit     int    `gorm:"not null;default:0" json:"daily_limit"`
	PityThreshold  int    `gorm:"not null;default:0" json:"pity_threshold"`
	PityRewardKey  string `gorm:"size:64" json:"pity_reward_key"`
	DurationSecond int64  `gorm:"not null;default:0" json:"duration_second"`
	Rules          string `gorm:"type:text" json:"rules"`
}

type ChanceRewardConfig struct {
	ID        uint   `gorm:"primaryKey" json:"id"`
	GameKey   string `gorm:"size:32;not null;uniqueIndex:idx_chance_reward" json:"game_key"`
	RewardKey string `gorm:"size:64;not null;uniqueIndex:idx_chance_reward" json:"reward_key"`
	Name      string `gorm:"size:64;not null" json:"name"`
	Weight    int    `gorm:"not null" json:"weight"`
	ItemName  string `gorm:"size:64" json:"item_name"`
	Quantity  int64  `gorm:"not null;default:0" json:"quantity"`
	Currency  int64  `gorm:"not null;default:0" json:"currency"`
	Rare      bool   `gorm:"not null;default:false" json:"rare"`
	Enabled   bool   `gorm:"not null;default:true;index" json:"enabled"`
	SortOrder int    `gorm:"not null;default:0;index" json:"sort_order"`
}

type ChanceDailyState struct {
	ID        uint   `gorm:"primaryKey"`
	AccountID string `gorm:"size:36;not null;uniqueIndex:idx_chance_daily"`
	GameKey   string `gorm:"size:32;not null;uniqueIndex:idx_chance_daily"`
	Day       string `gorm:"size:10;not null;uniqueIndex:idx_chance_daily"`
	Attempts  int    `gorm:"not null;default:0"`
	UpdatedAt time.Time
}

type ChancePlayerState struct {
	ID        uint   `gorm:"primaryKey"`
	AccountID string `gorm:"size:36;not null;uniqueIndex:idx_chance_player"`
	GameKey   string `gorm:"size:32;not null;uniqueIndex:idx_chance_player"`
	PityCount int    `gorm:"not null;default:0"`
	UpdatedAt time.Time
}

type ChanceOutcome struct {
	ID            string `gorm:"primaryKey;size:36"`
	AccountID     string `gorm:"size:36;not null;index"`
	GameKey       string `gorm:"size:32;not null;index"`
	ActionKey     string `gorm:"size:160;not null;uniqueIndex:idx_chance_action"`
	RewardKey     string `gorm:"size:64;not null"`
	RewardName    string `gorm:"size:64;not null"`
	ItemName      string `gorm:"size:64"`
	Quantity      int64  `gorm:"not null;default:0"`
	Currency      int64  `gorm:"not null;default:0"`
	Roll          int    `gorm:"not null"`
	TotalWeight   int    `gorm:"not null"`
	PityTriggered bool   `gorm:"not null;default:false"`
	CreatedAt     time.Time
}

type FishingRun struct {
	ID          string `gorm:"primaryKey;size:36"`
	AccountID   string `gorm:"size:36;not null;index:idx_fishing_account_status"`
	PetID       string `gorm:"size:36;not null;index"`
	Status      string `gorm:"size:24;not null;index:idx_fishing_account_status"`
	ActionKey   string `gorm:"size:160;not null;uniqueIndex"`
	RewardKey   string `gorm:"size:64;not null"`
	RewardName  string `gorm:"size:64;not null"`
	ItemName    string `gorm:"size:64"`
	Quantity    int64  `gorm:"not null;default:0"`
	Currency    int64  `gorm:"not null;default:0"`
	Roll        int    `gorm:"not null"`
	TotalWeight int    `gorm:"not null"`
	Pity        bool   `gorm:"not null;default:false"`
	StartedAt   time.Time
	ReadyAt     time.Time
	ClaimedAt   *time.Time
}

type BattleRecord struct {
	ID             string `gorm:"primaryKey;size:36"`
	AccountID      string `gorm:"size:36;not null;index"`
	PetID          string `gorm:"size:36;not null;index"`
	ActionKey      string `gorm:"size:160;not null;uniqueIndex"`
	Mode           string `gorm:"size:32;not null"`
	PlayerChoice   string `gorm:"size:16;not null"`
	OpponentChoice string `gorm:"size:16;not null"`
	Result         string `gorm:"size:16;not null"`
	RewardCurrency int64  `gorm:"not null;default:0"`
	Roll           int    `gorm:"not null"`
	CreatedAt      time.Time
}

type TradeOffer struct {
	Code            string `gorm:"primaryKey;size:12"`
	SellerAccountID string `gorm:"size:36;not null;index"`
	BuyerAccountID  string `gorm:"size:36;index"`
	ItemName        string `gorm:"size:64;not null"`
	Quantity        int64  `gorm:"not null"`
	Price           int64  `gorm:"not null"`
	CurrencyKey     string `gorm:"size:64;not null"`
	Status          string `gorm:"size:24;not null;index"`
	CreatedAt       time.Time
	ExpiresAt       time.Time `gorm:"index"`
	CompletedAt     *time.Time
}

type TradeAudit struct {
	ID        string `gorm:"primaryKey;size:36"`
	OfferCode string `gorm:"size:12;not null;index"`
	AccountID string `gorm:"size:36;not null;index"`
	Action    string `gorm:"size:32;not null"`
	Detail    string `gorm:"size:255"`
	CreatedAt time.Time
}

type GrowthRoleConfig struct {
	Name        string `gorm:"primaryKey;size:64" json:"name"`
	Description string `gorm:"size:255;not null" json:"description"`
	Skill1      string `gorm:"size:64;not null" json:"skill_1"`
	Skill2      string `gorm:"size:64;not null" json:"skill_2"`
	Skill3      string `gorm:"size:64;not null" json:"skill_3"`
	Enabled     bool   `gorm:"not null;default:true;index" json:"enabled"`
	SortOrder   int    `gorm:"not null;default:0;index" json:"sort_order"`
}

type GrowthStanceConfig struct {
	Name        string `gorm:"primaryKey;size:64" json:"name"`
	Description string `gorm:"size:255;not null" json:"description"`
	Enabled     bool   `gorm:"not null;default:true;index" json:"enabled"`
	SortOrder   int    `gorm:"not null;default:0;index" json:"sort_order"`
}

type PersonalityRuleConfig struct {
	Name         string `gorm:"primaryKey;size:64" json:"name"`
	Dimension    string `gorm:"size:24;not null;index" json:"dimension"`
	MinThreshold int64  `gorm:"not null;default:3" json:"min_threshold"`
	Description  string `gorm:"size:255;not null" json:"description"`
	Enabled      bool   `gorm:"not null;default:true;index" json:"enabled"`
	SortOrder    int    `gorm:"not null;default:0;index" json:"sort_order"`
}

type CodexCatalogConfig struct {
	ID          uint   `gorm:"primaryKey" json:"id"`
	Category    string `gorm:"size:64;not null;uniqueIndex:idx_codex_catalog" json:"category"`
	EntryKey    string `gorm:"size:64;not null;uniqueIndex:idx_codex_catalog" json:"entry_key"`
	Region      string `gorm:"size:64;not null;index" json:"region"`
	Description string `gorm:"size:255" json:"description"`
	Enabled     bool   `gorm:"not null;default:true;index" json:"enabled"`
	SortOrder   int    `gorm:"not null;default:0;index" json:"sort_order"`
	SourceType  string `gorm:"size:32" json:"source_type"`
	SourceKey   string `gorm:"size:64;index" json:"source_key"`
}

type GameplayMetric struct {
	ID              uint   `gorm:"primaryKey"`
	Day             string `gorm:"size:10;not null;uniqueIndex:idx_gameplay_metric"`
	Platform        string `gorm:"size:24;not null;uniqueIndex:idx_gameplay_metric"`
	SceneType       string `gorm:"size:24;not null;uniqueIndex:idx_gameplay_metric"`
	Command         string `gorm:"size:64;not null;uniqueIndex:idx_gameplay_metric"`
	BusinessResult  string `gorm:"size:24;not null;uniqueIndex:idx_gameplay_metric"`
	TechnicalResult string `gorm:"size:24;not null;uniqueIndex:idx_gameplay_metric"`
	Count           int64  `gorm:"not null;default:0"`
	UpdatedAt       time.Time
}

type AdminAuditLog struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	Operator     string    `gorm:"size:64;not null;index" json:"operator"`
	Action       string    `gorm:"size:64;not null;index" json:"action"`
	TargetType   string    `gorm:"size:64;not null;index" json:"target_type"`
	TargetID     string    `gorm:"size:320;not null;index" json:"target_id"`
	Reason       string    `gorm:"size:500;not null" json:"reason"`
	BeforeJSON   string    `gorm:"type:text" json:"before_json"`
	AfterJSON    string    `gorm:"type:text" json:"after_json"`
	Success      bool      `gorm:"not null;index" json:"success"`
	ErrorMessage string    `gorm:"size:500" json:"error_message"`
	CreatedAt    time.Time `gorm:"index" json:"created_at"`
}

type AdminOperationKey struct {
	Key       string `gorm:"primaryKey;size:128"`
	Action    string `gorm:"size:64;not null"`
	TargetID  string `gorm:"size:320;not null"`
	CreatedAt time.Time
}

// ImageAsset is the single registry for administrator-uploaded image files.
// StoredPath is relative to the public image root and never points at a second
// mirrored copy.
type ImageAsset struct {
	ID           string    `gorm:"primaryKey;size:36" json:"id"`
	OriginalName string    `gorm:"size:255;not null" json:"original_name"`
	StoredName   string    `gorm:"size:255;not null;uniqueIndex" json:"stored_name"`
	StoredPath   string    `gorm:"size:500;not null;uniqueIndex" json:"path"`
	URL          string    `gorm:"size:500;not null" json:"url"`
	MIMEType     string    `gorm:"size:64;not null" json:"mime_type"`
	Size         int64     `gorm:"not null" json:"size"`
	Width        int       `gorm:"not null" json:"width"`
	Height       int       `gorm:"not null" json:"height"`
	CreatedAt    time.Time `gorm:"index" json:"created_at"`
}
