package models

import (
	"gorm.io/gorm"
	"time"
)

var (
	OnPetFind func(pet *UserPet)
	OnPetSave func(pet *UserPet)
)

func (p *UserPet) AfterFind(tx *gorm.DB) (err error) {
	if OnPetFind != nil {
		OnPetFind(p)
	}
	return nil
}

func (p *UserPet) AfterSave(tx *gorm.DB) (err error) {
	if OnPetSave != nil {
		OnPetSave(p)
	}
	return nil
}

func (p *UserPet) AfterUpdate(tx *gorm.DB) (err error) {
	if OnPetSave != nil {
		OnPetSave(p)
	}
	return nil
}

func (p *UserPet) AfterCreate(tx *gorm.DB) (err error) {
	if OnPetSave != nil {
		OnPetSave(p)
	}
	return nil
}

// UserPet 玩家宠物模型
type UserPet struct {
	ID          uint       `gorm:"primaryKey"`
	UserID      int64      `gorm:"uniqueIndex:idx_user_group;comment:玩家QQ号"`
	GroupID     int64      `gorm:"uniqueIndex:idx_user_group;index:idx_user_pets_group;comment:所在QQ群号"`
	PetType     string     `gorm:"size:64;index:idx_user_pets_type;comment:宠物种类/类型"`
	Name        string     `gorm:"size:64;default:'无名小萌宠';comment:宠物名字"`
	Image       string     `gorm:"size:255;comment:当前图片"`
	Status      string     `gorm:"size:32;default:'空闲';index:idx_user_pets_status;comment:当前状态"`
	Mood        string     `gorm:"size:32;default:'一般';comment:当前心情"`
	MoodPoints  int        `gorm:"default:50;comment:心情点(0-100)"`
	Affection   int64      `gorm:"default:0;comment:好感度"`
	Growth      int64      `gorm:"default:0;comment:成长值"`
	Health      int64      `gorm:"default:100;comment:当前血量"`
	Wisdom      int64      `gorm:"default:10;comment:智慧"`
	Strength    int64      `gorm:"default:10;comment:力量"`
	Defense     int64      `gorm:"default:10;comment:防御"`
	Hunger      int64      `gorm:"default:100;comment:当前饱食度(0-100)"`
	Family      string     `gorm:"size:64;comment:所属家族"`
	FamilyScore int64      `gorm:"default:0;comment:家族积分"`
	NewbieCheck int        `gorm:"default:1;comment:新手签到天数"`
	Currency    int64      `gorm:"default:0;comment:玩家持有货币"`
	LastCheckin *time.Time `gorm:"comment:最近签到时间"`

	// 各种状态的开始时间，用于计算冷却和异步状态时长
	StudyTime   *time.Time `gorm:"comment:开始学习时间"`
	StudyItem   string     `gorm:"size:64;comment:学习消耗的物品"`
	TrainTime   *time.Time `gorm:"comment:开始锻炼时间"`
	TrainItem   string     `gorm:"size:64;comment:锻炼消耗的物品"`
	WorkTime    *time.Time `gorm:"comment:开始打工时间"`
	WorkType    string     `gorm:"size:64;comment:打工类型"`
	FitnessTime *time.Time `gorm:"comment:开始健身时间"`
	FitnessItem string     `gorm:"size:64;comment:健身消耗的物品"`
	DyingTime   *time.Time `gorm:"comment:开始濒死时间"`
	EscapeTime  *time.Time `gorm:"comment:逃跑时间"`
	LostTime    *time.Time `gorm:"comment:失去宠物时间"`
	BindKey     string     `gorm:"size:32;comment:频道绑定密钥"`

	UpdatedAt time.Time
}

// BackpackItem 玩家背包物品模型
type BackpackItem struct {
	ID       uint   `gorm:"primaryKey"`
	UserID   int64  `gorm:"uniqueIndex:idx_user_group_item;comment:玩家QQ号"`
	GroupID  int64  `gorm:"uniqueIndex:idx_user_group_item;comment:所在QQ群号"`
	ItemName string `gorm:"uniqueIndex:idx_user_group_item;size:64;comment:物品名称"`
	Quantity int64  `gorm:"default:0;comment:物品数量"`
}

// Family 玩家家族模型
type Family struct {
	ID            uint   `gorm:"primaryKey"`
	Name          string `gorm:"size:64;uniqueIndex;comment:家族名称"`
	LeaderID      int64  `gorm:"comment:族长QQ号"`
	CurrentSize   int    `gorm:"default:1;comment:当前人数"`
	MaxSize       int    `gorm:"default:10;comment:人数上限"`
	TreeNutrients int64  `gorm:"default:0;comment:神树养分"`
	CreatedAt     time.Time
}

// SystemConfig 核心与互动键值配置表
type SystemConfig struct {
	Key   string `gorm:"primaryKey;size:128;comment:配置项名称"`
	Value string `gorm:"type:text;comment:配置值"`
}

// CommandConfig 自定义指令配置表
type CommandConfig struct {
	FuncName    string `gorm:"primaryKey;size:128;comment:稳定功能键" json:"func_name"`
	Command     string `gorm:"size:128;not null;comment:用户自定义触发指令" json:"command"`
	DisplayName string `gorm:"size:64;not null;default:''" json:"display_name"`
	Category    string `gorm:"size:32;not null;default:'基础';index" json:"category"`
	Description string `gorm:"size:255;not null;default:''" json:"description"`
	Enabled     bool   `gorm:"not null;default:true;index" json:"enabled"`
	SortOrder   int    `gorm:"not null;default:0;index" json:"sort_order"`
}

// PetSpeciesConfig 宠物属性与进化配置表
type PetSpeciesConfig struct {
	Name            string `gorm:"primaryKey;size:64;comment:宠物种类"`
	Image           string `gorm:"size:255;comment:图片文件名"`
	AdoptImage      string `gorm:"size:255;comment:领养配图"`
	TrainStartImg   string `gorm:"size:255;comment:锻炼开始图"`
	TrainEndImg     string `gorm:"size:255;comment:锻炼结束图"`
	StudyStartImg   string `gorm:"size:255;comment:学习开始图"`
	StudyEndImg     string `gorm:"size:255;comment:学习结束图"`
	FitnessStartImg string `gorm:"size:255;comment:健身开始图"`
	FitnessEndImg   string `gorm:"size:255;comment:健身结束图"`
	FavoriteFood    string `gorm:"size:255;comment:喜欢食物"`
	FavoriteGift    string `gorm:"size:255;comment:喜欢礼物"`
	Health          int64  `gorm:"comment:血量"`
	Wisdom          int64  `gorm:"comment:智慧"`
	Strength        int64  `gorm:"comment:力量"`
	Defense         int64  `gorm:"comment:防御"`
	Hunger          int64  `gorm:"comment:饱食"`
	Description     string `gorm:"type:text;comment:描述"`
	EvolutionBranch int    `gorm:"comment:进化分支"`
	Evolution       string `gorm:"size:64;comment:进化形态名称"`
	EvolutionGrowth int64  `gorm:"comment:进化所需成长"`
	EvolutionAffect int64  `gorm:"comment:进化所需好感"`
	EvolutionImage  string `gorm:"size:255;comment:进化后图片"`
	Awaken          string `gorm:"size:64;comment:觉醒形态名称"`
	AwakenGrowth    int64  `gorm:"comment:觉醒所需成长"`
	AwakenAffect    int64  `gorm:"comment:觉醒所需好感"`
	AwakenItems     string `gorm:"size:255;comment:觉醒所需物品"`
	AwakenImage     string `gorm:"size:255;comment:觉醒后图片"`
	HealthMax       int64  `gorm:"comment:血量上限"`
	WisdomMax       int64  `gorm:"comment:智慧上限"`
	StrengthMax     int64  `gorm:"comment:力量上限"`
	DefenseMax      int64  `gorm:"comment:防御上限"`
	HungerMax       int64  `gorm:"comment:饱食上限"`
	AffectionBonus  int    `gorm:"comment:好感加成比率"`
	GrowthBonus     int    `gorm:"comment:成长加成比率"`
	AttributeBonus  int    `gorm:"comment:属性加成比率"`
	CurrencyBonus   int    `gorm:"comment:金币加成比率"`
}

// ItemConfig 道具配置表
type ItemConfig struct {
	Name        string `gorm:"primaryKey;size:64;comment:道具名称"`
	Status      string `gorm:"size:16;not null;default:'active';index;comment:active/limited/hidden/disabled"`
	Type        string `gorm:"size:64;comment:道具类型"`
	RewardType  string `gorm:"size:64;comment:礼包类型"`
	ObtainType  int    `gorm:"comment:获取类型(1为单次，0为多次)"`
	OpenReq     string `gorm:"size:64;comment:打开需要(钥匙道具)"`
	Effect      string `gorm:"size:255;comment:使用效果"`
	Time        int64  `gorm:"comment:持续时长"`
	Image       string `gorm:"size:255;comment:配图"`
	Description string `gorm:"type:text;comment:道具描述"`
	SellPrice   int64  `gorm:"comment:出售价格"`
}

// ShopItemConfig 商店商品配置表
type ShopItemConfig struct {
	ID            uint   `gorm:"primaryKey"`
	ShopType      string `gorm:"size:32;index;comment:shop_normal或shop_affection"`
	Name          string `gorm:"size:64;index;comment:商品名称"`
	Image         string `gorm:"size:255;comment:商品配图"`
	Stock         int64  `gorm:"comment:库存(-1为无限)"`
	RestockTarget int64  `gorm:"not null;default:0;comment:一键补货目标库存"`
	Price         int64  `gorm:"comment:价格"`
	Description   string `gorm:"type:text;comment:商品描述"`
}

// CheckinRewardConfig 签到奖励配置表
type CheckinRewardConfig struct {
	ID        uint   `gorm:"primaryKey"`
	Type      string `gorm:"size:32;index;comment:checkin_newbie或checkin_weekly"`
	Day       string `gorm:"size:32;index;comment:天数/星期数"`
	Currency  int64  `gorm:"comment:奖励货币"`
	Affection int64  `gorm:"comment:奖励好感"`
	Items     string `gorm:"size:255;comment:奖励物品"`
	Image     string `gorm:"size:255;comment:奖励图片"`
}

// WorkSettingConfig 打工种类配置表
type WorkSettingConfig struct {
	Name        string `gorm:"primaryKey;size:64;comment:工作名称"`
	Time        int64  `gorm:"comment:打工时长(分钟)"`
	HungerCost  int64  `gorm:"comment:消耗饱食"`
	RewardCoin  int64  `gorm:"comment:奖励货币"`
	RewardItems string `gorm:"size:255;comment:概率奖励物品"`
	ReplyQuotes string `gorm:"type:text;comment:打工回复语(逗号分隔)"`
	StartImage  string `gorm:"size:255;comment:开始打工图"`
	EndImage    string `gorm:"size:255;comment:结束打工图"`
}

// MenuConfig 菜单配置表
type MenuConfig struct {
	Name  string `gorm:"primaryKey;size:64;comment:菜单指令名"`
	Reply string `gorm:"type:text;comment:菜单回复内容"`
}

// ImageConfig 游戏核心图片表
type ImageConfig struct {
	Name string `gorm:"primaryKey;size:64;comment:图片标识键"`
	Path string `gorm:"size:255;comment:图片相对路径"`
}

// GroupSwitch 群开启/关闭开关表
type GroupSwitch struct {
	GroupID   int64  `gorm:"primaryKey;comment:群号"`
	GroupName string `gorm:"size:128;comment:群名称"`
	IsActive  bool   `gorm:"default:true;comment:是否开启"`
}

// AdminConfigState 记录数据库配置与机器人内存配置的版本关系。
type AdminConfigState struct {
	ID             uint       `gorm:"primaryKey" json:"-"`
	DBRevision     uint64     `gorm:"not null;default:0" json:"db_revision"`
	LoadedRevision uint64     `gorm:"not null;default:0" json:"loaded_revision"`
	SavedAt        *time.Time `json:"saved_at"`
	LoadedAt       *time.Time `json:"loaded_at"`
}
