package models

import "time"

type PlayerAccount struct {
	ID        string `gorm:"primaryKey;size:36"`
	CreatedAt time.Time
	UpdatedAt time.Time
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
	AccountID string `gorm:"primaryKey;size:36"`
	PetType   string `gorm:"size:64;not null"`
	Name      string `gorm:"size:64;not null"`
	Role      string `gorm:"size:24;not null;default:'探索者'"`
	Stance    string `gorm:"size:24;not null;default:'探索'"`
	Mood      string `gorm:"size:24;not null;default:'安稳'"`
	Readiness int    `gorm:"not null;default:100"`
	Affection int64  `gorm:"not null;default:0"`
	Growth    int64  `gorm:"not null;default:0"`
	BondLevel int    `gorm:"not null;default:1"`
	Traits    string `gorm:"type:text"`
	Skills    string `gorm:"type:text"`
	CreatedAt time.Time
	UpdatedAt time.Time
}

type GlobalInventoryItem struct {
	ID        uint   `gorm:"primaryKey"`
	AccountID string `gorm:"size:36;not null;uniqueIndex:idx_global_inventory"`
	ItemName  string `gorm:"size:64;not null;uniqueIndex:idx_global_inventory"`
	Quantity  int64  `gorm:"not null;default:0"`
	UpdatedAt time.Time
}

type CompanionJournal struct {
	ID        uint   `gorm:"primaryKey"`
	AccountID string `gorm:"size:36;not null;uniqueIndex:idx_companion_journal"`
	Day       string `gorm:"size:10;not null;uniqueIndex:idx_companion_journal"`
	Action    string `gorm:"size:32;not null"`
	CreatedAt time.Time
}

type ExpeditionRun struct {
	ID             string `gorm:"primaryKey;size:36"`
	AccountID      string `gorm:"size:36;not null;index:idx_expedition_account_status"`
	Tier           int    `gorm:"not null"`
	Name           string `gorm:"size:64;not null"`
	Stance         string `gorm:"size:24;not null"`
	Status         string `gorm:"size:24;not null;index:idx_expedition_account_status"`
	RewardItem     string `gorm:"size:64;not null"`
	RewardQuantity int64  `gorm:"not null"`
	RewardRecords  int64  `gorm:"not null"`
	RewardGrowth   int64  `gorm:"not null"`
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
	Enabled   bool   `gorm:"not null;default:true"`
	UpdatedAt time.Time
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

type PetBehaviorProfile struct {
	AccountID string `gorm:"primaryKey;size:36"`
	Explore   int64  `gorm:"not null;default:0"`
	Care      int64  `gorm:"not null;default:0"`
	Support   int64  `gorm:"not null;default:0"`
	Trait     string `gorm:"size:32"`
	UpdatedAt time.Time
}

type LiveEventConfig struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	Key          string    `gorm:"size:64;not null;uniqueIndex" json:"key"`
	Name         string    `gorm:"size:64;not null" json:"name"`
	Region       string    `gorm:"size:32;not null" json:"region"`
	StoryChoices string    `gorm:"type:text;not null" json:"story_choices"`
	StartsAt     time.Time `gorm:"not null;index" json:"starts_at"`
	EndsAt       time.Time `gorm:"not null;index" json:"ends_at"`
	Active       bool      `gorm:"not null;default:true;index" json:"active"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type RewardTrackConfig struct {
	ID          uint   `gorm:"primaryKey" json:"id"`
	EventKey    string `gorm:"size:64;not null;uniqueIndex:idx_reward_track_item" json:"event_key"`
	Milestone   int64  `gorm:"not null;uniqueIndex:idx_reward_track_item" json:"milestone"`
	ItemName    string `gorm:"size:64;not null;uniqueIndex:idx_reward_track_item" json:"item_name"`
	Quantity    int64  `gorm:"not null" json:"quantity"`
	Description string `gorm:"size:255" json:"description"`
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
}

type GameplayMetric struct {
	ID        uint   `gorm:"primaryKey"`
	Day       string `gorm:"size:10;not null;uniqueIndex:idx_gameplay_metric"`
	Platform  string `gorm:"size:24;not null;uniqueIndex:idx_gameplay_metric"`
	SceneType string `gorm:"size:24;not null;uniqueIndex:idx_gameplay_metric"`
	Command   string `gorm:"size:64;not null;uniqueIndex:idx_gameplay_metric"`
	Success   bool   `gorm:"not null;uniqueIndex:idx_gameplay_metric"`
	Count     int64  `gorm:"not null;default:0"`
	UpdatedAt time.Time
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
