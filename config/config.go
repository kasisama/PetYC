package config

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/transform"
	"gopkg.in/ini.v1"
	"gorm.io/gorm"
	"qq-pet-saas/models"
)

// GlobalConfigPath is the controlled runtime resource root. Historical INI
// archives are deliberately outside this path and never used by startup.
var GlobalConfigPath = "."

// Global variables to hold configurations
var (
	configMu      sync.RWMutex
	Core          CoreConfig
	Interaction   InteractionConfig
	Commands      map[string]string // Key: E-language function key, Value: User custom command
	Images        map[string]string // Key: Image function name, Value: Relative image path
	Pets          map[string]PetSpecies
	Items         map[string]ItemConfig
	Shop          map[string]ShopItem
	AffectionShop map[string]ShopItem
	NewbieCheckin map[string]CheckinReward // Key: day "1"-"7"
	WeeklyCheckin map[string]CheckinReward // Key: weekday "1"-"7"
	WorkSettings  map[string]WorkSetting   // Key: work type (洗碗, 搬砖 etc.), Value: config
	Menus         map[string]string        // Key: Menu Name (Section Name), Value: Menu Reply content
)

// LockForRead protects command processing while an in-memory configuration
// snapshot is being replaced.
func LockForRead() { configMu.RLock() }

// UnlockForRead releases a read lock acquired by LockForRead.
func UnlockForRead() { configMu.RUnlock() }

// LockForWrite protects a full in-memory configuration reload.
func LockForWrite() { configMu.Lock() }

// UnlockForWrite releases a write lock acquired by LockForWrite.
func UnlockForWrite() { configMu.Unlock() }

// WorkSetting 对应 打工配置.ini 中的各个打工类型
type WorkSetting struct {
	Name        string
	Time        int64 // 分钟
	HungerCost  int64
	RewardCoin  int64
	RewardItems string
	ReplyQuotes string
	StartImage  string
	EndImage    string
}

// CoreConfig 对应 核心配置.ini [核心]
type CoreConfig struct {
	InitialPets         []string
	CoinName            string
	InitialCoin         int64
	RenameCost          int64
	RenameBlocklist     []string
	TreatCost           int64
	DyingSaveTime       int64 // 分钟
	DyingProtectTime    int64 // 分钟
	EscapeFindTime      int64 // 分钟
	LostCooldown        int64 // 分钟
	CheckinLike         bool
	CurrencySync        int
	CurrencySyncPath    string
	CurrencySyncSection string
	CurrencySyncKey     string
	TxFee               int64
	MasterQQ            int64
	NotifyQQ            int64
	ImageHost           string // HTTP 图片服务域名/IP (如 http://127.0.0.1:8080)，供远程 SaaS 部署使用
}

// SplitConfigList accepts the historical '#' seed delimiter and the admin
// editor's comma / Chinese separators so list settings round-trip.
func SplitConfigList(raw string) []string {
	parts := strings.FieldsFunc(raw, func(r rune) bool {
		switch r {
		case '#', ',', '，', '、', ';', '；':
			return true
		default:
			return false
		}
	})
	result := make([]string, 0, len(parts))
	seen := make(map[string]struct{}, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if _, exists := seen[part]; exists {
			continue
		}
		seen[part] = struct{}{}
		result = append(result, part)
	}
	return result
}

func StarterPets() []string {
	names := make([]string, 0, len(Core.InitialPets))
	seen := make(map[string]struct{}, len(Core.InitialPets))
	for _, name := range Core.InitialPets {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		if _, exists := seen[name]; exists {
			continue
		}
		seen[name] = struct{}{}
		names = append(names, name)
	}
	if len(names) == 0 {
		return []string{"光芽兽"}
	}
	return names
}

// InteractionConfig 对应 核心配置.ini [互动]
type InteractionConfig struct {
	TrainGrowth               int64
	TrainLimit                int64
	TrainHungerCost           int64
	StudyGrowth               int64
	StudyLimit                int64
	StudyHungerCost           int64
	WashGrowth                int64
	WashAffection             int64
	WashHungerCost            int64
	WalkGrowth                int64
	WalkAffection             int64
	WalkGrowthLimit           int64
	WalkAffectLimit           int64
	WalkInterval              int64
	WalkHungerCost            int64
	TouchGrowth               int64
	TouchAffection            int64
	TouchGrowthLimit          int64
	TouchAffectLimit          int64
	TouchInterval             int64
	TouchHungerCost           int64
	RpsAffection              int64
	RpsAffectLimit            int64
	RpsHungerCost             int64
	RpsInterval               int64
	SnackHungerCost           int64
	SnackSuccess              int64
	SnackInterval             int64
	SnackProtect              int64
	CounterHunger             int64
	CounterSuccess            int64
	CreateFamilyCoin          int64
	CreateFamilyItem          string
	FamilySizeLimit           int
	FishHungerCost            int64
	FishSuccessRate           int64
	GiftLimit                 int64
	FishSpecies               []string
	HungerMoodFlush           int64
	LotteryItem               string
	LotteryRewardStr          string
	WorkTime                  int64
	WorkRewardCoin            int64
	WorkRewardItems           string
	WorkHungerCost            int64
	FitnessGrowth             int64
	FitnessLimit              int64
	FitnessHungerCost         int64
	SellNoPriceGrowth         int64
	CommunityBuildGoal        int64
	CommunityBuildRewardItems string
	BuyLimit                  int
}

// PetSpecies 对应 宠物配置.ini 中的各个宠物种类
type PetSpecies struct {
	Name            string
	Image           string
	AdoptImage      string
	TrainStartImg   string
	TrainEndImg     string
	StudyStartImg   string
	StudyEndImg     string
	FitnessStartImg string
	FitnessEndImg   string
	FavoriteFood    string
	FavoriteGift    string
	Health          int64
	Wisdom          int64
	Strength        int64
	Defense         int64
	Hunger          int64
	Description     string
	EvolutionBranch int
	Evolution       string
	EvolutionGrowth int64
	EvolutionAffect int64
	EvolutionImage  string
	Awaken          string
	AwakenGrowth    int64
	AwakenAffect    int64
	AwakenItems     string
	AwakenImage     string
	HealthMax       int64
	WisdomMax       int64
	StrengthMax     int64
	DefenseMax      int64
	HungerMax       int64
	AffectionBonus  int
	GrowthBonus     int
	AttributeBonus  int
	CurrencyBonus   int
}

// ItemConfig 对应 物品配置.ini 中的各个物品
type ItemConfig struct {
	Name        string
	Status      string
	Type        string
	RewardType  string // 礼包类型 (固定/随机)
	ObtainType  int    // 获得类型
	OpenReq     string // 打开所需
	Effect      string
	Time        int64
	Image       string
	Description string
	SellPrice   int64
}

// ShopItem 对应 商店配置.ini / 好感商店配置.ini
type ShopItem struct {
	Name          string
	Image         string
	Stock         int64
	RestockTarget int64
	Price         int64
	Description   string
}

// CheckinReward 对应 签到配置.ini / 新手签到配置.ini
type CheckinReward struct {
	Day       string // 天数/星期数
	Currency  int64
	Affection int64
	Items     string
	Image     string
}

// Decode GBK bytes to UTF-8
func decodeGBK(b []byte) ([]byte, error) {
	r := transform.NewReader(bytes.NewReader(b), simplifiedchinese.GBK.NewDecoder())
	return io.ReadAll(r)
}

// GetCommand 返回功能名对应的自定义指令名称，如果未配置则返回功能名本身
func GetCommand(funcName string) string {
	if cmd, exists := Commands[funcName]; exists && cmd != "" {
		return cmd
	}
	return funcName
}

func seedSystemConfig(tx *gorm.DB) {
	coreFields := map[string]string{
		"Core.InitialPets":         strings.Join(Core.InitialPets, "#"),
		"Core.CoinName":            Core.CoinName,
		"Core.InitialCoin":         strconv.FormatInt(Core.InitialCoin, 10),
		"Core.RenameCost":          strconv.FormatInt(Core.RenameCost, 10),
		"Core.RenameBlocklist":     strings.Join(Core.RenameBlocklist, "#"),
		"Core.TreatCost":           strconv.FormatInt(Core.TreatCost, 10),
		"Core.DyingSaveTime":       strconv.FormatInt(Core.DyingSaveTime, 10),
		"Core.DyingProtectTime":    strconv.FormatInt(Core.DyingProtectTime, 10),
		"Core.EscapeFindTime":      strconv.FormatInt(Core.EscapeFindTime, 10),
		"Core.LostCooldown":        strconv.FormatInt(Core.LostCooldown, 10),
		"Core.CheckinLike":         strconv.FormatBool(Core.CheckinLike),
		"Core.CurrencySync":        strconv.Itoa(Core.CurrencySync),
		"Core.CurrencySyncPath":    Core.CurrencySyncPath,
		"Core.CurrencySyncSection": Core.CurrencySyncSection,
		"Core.CurrencySyncKey":     Core.CurrencySyncKey,
		"Core.TxFee":               strconv.FormatInt(Core.TxFee, 10),
		"Core.MasterQQ":            strconv.FormatInt(Core.MasterQQ, 10),
		"Core.NotifyQQ":            strconv.FormatInt(Core.NotifyQQ, 10),
		"Core.ImageHost":           Core.ImageHost,
	}

	interactionFields := map[string]string{
		"Interaction.TrainGrowth":               strconv.FormatInt(Interaction.TrainGrowth, 10),
		"Interaction.TrainLimit":                strconv.FormatInt(Interaction.TrainLimit, 10),
		"Interaction.TrainHungerCost":           strconv.FormatInt(Interaction.TrainHungerCost, 10),
		"Interaction.StudyGrowth":               strconv.FormatInt(Interaction.StudyGrowth, 10),
		"Interaction.StudyLimit":                strconv.FormatInt(Interaction.StudyLimit, 10),
		"Interaction.StudyHungerCost":           strconv.FormatInt(Interaction.StudyHungerCost, 10),
		"Interaction.WashGrowth":                strconv.FormatInt(Interaction.WashGrowth, 10),
		"Interaction.WashAffection":             strconv.FormatInt(Interaction.WashAffection, 10),
		"Interaction.WashHungerCost":            strconv.FormatInt(Interaction.WashHungerCost, 10),
		"Interaction.WalkGrowth":                strconv.FormatInt(Interaction.WalkGrowth, 10),
		"Interaction.WalkAffection":             strconv.FormatInt(Interaction.WalkAffection, 10),
		"Interaction.WalkGrowthLimit":           strconv.FormatInt(Interaction.WalkGrowthLimit, 10),
		"Interaction.WalkAffectLimit":           strconv.FormatInt(Interaction.WalkAffectLimit, 10),
		"Interaction.WalkInterval":              strconv.FormatInt(Interaction.WalkInterval, 10),
		"Interaction.WalkHungerCost":            strconv.FormatInt(Interaction.WalkHungerCost, 10),
		"Interaction.TouchGrowth":               strconv.FormatInt(Interaction.TouchGrowth, 10),
		"Interaction.TouchAffection":            strconv.FormatInt(Interaction.TouchAffection, 10),
		"Interaction.TouchGrowthLimit":          strconv.FormatInt(Interaction.TouchGrowthLimit, 10),
		"Interaction.TouchAffectLimit":          strconv.FormatInt(Interaction.TouchAffectLimit, 10),
		"Interaction.TouchInterval":             strconv.FormatInt(Interaction.TouchInterval, 10),
		"Interaction.TouchHungerCost":           strconv.FormatInt(Interaction.TouchHungerCost, 10),
		"Interaction.RpsAffection":              strconv.FormatInt(Interaction.RpsAffection, 10),
		"Interaction.RpsAffectLimit":            strconv.FormatInt(Interaction.RpsAffectLimit, 10),
		"Interaction.RpsHungerCost":             strconv.FormatInt(Interaction.RpsHungerCost, 10),
		"Interaction.RpsInterval":               strconv.FormatInt(Interaction.RpsInterval, 10),
		"Interaction.SnackHungerCost":           strconv.FormatInt(Interaction.SnackHungerCost, 10),
		"Interaction.SnackSuccess":              strconv.FormatInt(Interaction.SnackSuccess, 10),
		"Interaction.SnackInterval":             strconv.FormatInt(Interaction.SnackInterval, 10),
		"Interaction.SnackProtect":              strconv.FormatInt(Interaction.SnackProtect, 10),
		"Interaction.CounterHunger":             strconv.FormatInt(Interaction.CounterHunger, 10),
		"Interaction.CounterSuccess":            strconv.FormatInt(Interaction.CounterSuccess, 10),
		"Interaction.CreateFamilyCoin":          strconv.FormatInt(Interaction.CreateFamilyCoin, 10),
		"Interaction.CreateFamilyItem":          Interaction.CreateFamilyItem,
		"Interaction.FamilySizeLimit":           strconv.Itoa(Interaction.FamilySizeLimit),
		"Interaction.FishHungerCost":            strconv.FormatInt(Interaction.FishHungerCost, 10),
		"Interaction.FishSuccessRate":           strconv.FormatInt(Interaction.FishSuccessRate, 10),
		"Interaction.GiftLimit":                 strconv.FormatInt(Interaction.GiftLimit, 10),
		"Interaction.FishSpecies":               strings.Join(Interaction.FishSpecies, "#"),
		"Interaction.HungerMoodFlush":           strconv.FormatInt(Interaction.HungerMoodFlush, 10),
		"Interaction.LotteryItem":               Interaction.LotteryItem,
		"Interaction.LotteryRewardStr":          Interaction.LotteryRewardStr,
		"Interaction.WorkTime":                  strconv.FormatInt(Interaction.WorkTime, 10),
		"Interaction.WorkRewardCoin":            strconv.FormatInt(Interaction.WorkRewardCoin, 10),
		"Interaction.WorkRewardItems":           Interaction.WorkRewardItems,
		"Interaction.WorkHungerCost":            strconv.FormatInt(Interaction.WorkHungerCost, 10),
		"Interaction.FitnessGrowth":             strconv.FormatInt(Interaction.FitnessGrowth, 10),
		"Interaction.FitnessLimit":              strconv.FormatInt(Interaction.FitnessLimit, 10),
		"Interaction.FitnessHungerCost":         strconv.FormatInt(Interaction.FitnessHungerCost, 10),
		"Interaction.SellNoPriceGrowth":         strconv.FormatInt(Interaction.SellNoPriceGrowth, 10),
		"Interaction.CommunityBuildGoal":        strconv.FormatInt(Interaction.CommunityBuildGoal, 10),
		"Interaction.CommunityBuildRewardItems": Interaction.CommunityBuildRewardItems,
		"Interaction.BuyLimit":                  strconv.Itoa(Interaction.BuyLimit),
	}

	for k, v := range coreFields {
		tx.Create(&models.SystemConfig{Key: k, Value: v})
	}
	for k, v := range interactionFields {
		tx.Create(&models.SystemConfig{Key: k, Value: v})
	}
}

func seedCommandConfig(tx *gorm.DB) {
	for funcName, command := range Commands {
		tx.Create(&models.CommandConfig{FuncName: funcName, Command: command})
	}
}

func seedPetSpeciesConfig(tx *gorm.DB) {
	for _, pet := range Pets {
		tx.Create(&models.PetSpeciesConfig{
			Name:            pet.Name,
			Image:           pet.Image,
			AdoptImage:      pet.AdoptImage,
			TrainStartImg:   pet.TrainStartImg,
			TrainEndImg:     pet.TrainEndImg,
			StudyStartImg:   pet.StudyStartImg,
			StudyEndImg:     pet.StudyEndImg,
			FitnessStartImg: pet.FitnessStartImg,
			FitnessEndImg:   pet.FitnessEndImg,
			FavoriteFood:    pet.FavoriteFood,
			FavoriteGift:    pet.FavoriteGift,
			Health:          pet.Health,
			Wisdom:          pet.Wisdom,
			Strength:        pet.Strength,
			Defense:         pet.Defense,
			Hunger:          pet.Hunger,
			Description:     pet.Description,
			EvolutionBranch: pet.EvolutionBranch,
			Evolution:       pet.Evolution,
			EvolutionGrowth: pet.EvolutionGrowth,
			EvolutionAffect: pet.EvolutionAffect,
			EvolutionImage:  pet.EvolutionImage,
			Awaken:          pet.Awaken,
			AwakenGrowth:    pet.AwakenGrowth,
			AwakenAffect:    pet.AwakenAffect,
			AwakenItems:     pet.AwakenItems,
			AwakenImage:     pet.AwakenImage,
			HealthMax:       pet.HealthMax,
			WisdomMax:       pet.WisdomMax,
			StrengthMax:     pet.StrengthMax,
			DefenseMax:      pet.DefenseMax,
			HungerMax:       pet.HungerMax,
			AffectionBonus:  pet.AffectionBonus,
			GrowthBonus:     pet.GrowthBonus,
			AttributeBonus:  pet.AttributeBonus,
			CurrencyBonus:   pet.CurrencyBonus,
		})
	}
}

func seedItemConfig(tx *gorm.DB) {
	for _, item := range Items {
		tx.Create(&models.ItemConfig{
			Name:        item.Name,
			Status:      "active",
			Type:        item.Type,
			RewardType:  item.RewardType,
			ObtainType:  item.ObtainType,
			OpenReq:     item.OpenReq,
			Effect:      item.Effect,
			Time:        item.Time,
			Image:       item.Image,
			Description: item.Description,
			SellPrice:   item.SellPrice,
		})
	}
}

func seedShopItemConfig(tx *gorm.DB) {
	for _, shopItem := range Shop {
		tx.Create(&models.ShopItemConfig{
			ShopType:      "shop_normal",
			Name:          shopItem.Name,
			Image:         shopItem.Image,
			Stock:         shopItem.Stock,
			RestockTarget: shopItem.Stock,
			Price:         shopItem.Price,
			Description:   shopItem.Description,
		})
	}
	for _, affItem := range AffectionShop {
		tx.Create(&models.ShopItemConfig{
			ShopType:      "shop_affection",
			Name:          affItem.Name,
			Image:         affItem.Image,
			Stock:         affItem.Stock,
			RestockTarget: affItem.Stock,
			Price:         affItem.Price,
			Description:   affItem.Description,
		})
	}
}

func seedCheckinRewardConfig(tx *gorm.DB) {
	for _, reward := range NewbieCheckin {
		tx.Create(&models.CheckinRewardConfig{
			Type:      "checkin_newbie",
			Day:       reward.Day,
			Currency:  reward.Currency,
			Affection: reward.Affection,
			Items:     reward.Items,
			Image:     reward.Image,
		})
	}
	for _, reward := range WeeklyCheckin {
		tx.Create(&models.CheckinRewardConfig{
			Type:      "checkin_weekly",
			Day:       reward.Day,
			Currency:  reward.Currency,
			Affection: reward.Affection,
			Items:     reward.Items,
			Image:     reward.Image,
		})
	}
}

func seedWorkSettingConfig(tx *gorm.DB) {
	for _, job := range WorkSettings {
		tx.Create(&models.WorkSettingConfig{
			Name:        job.Name,
			Time:        job.Time,
			HungerCost:  job.HungerCost,
			RewardCoin:  job.RewardCoin,
			RewardItems: job.RewardItems,
			ReplyQuotes: job.ReplyQuotes,
			StartImage:  job.StartImage,
			EndImage:    job.EndImage,
		})
	}
}

func seedMenuConfig(tx *gorm.DB) {
	for name, reply := range Menus {
		tx.Create(&models.MenuConfig{
			Name:  name,
			Reply: reply,
		})
	}
}

func seedImageConfig(tx *gorm.DB) {
	for name, path := range Images {
		tx.Create(&models.ImageConfig{
			Name: name,
			Path: path,
		})
	}
}

func seedChanceGameConfig(tx *gorm.DB) {
	games := []models.ChanceGameConfig{
		{GameKey: "lottery", Name: "幸运抽奖", Enabled: true, CostCurrency: 20, DailyLimit: 10, PityThreshold: 10, PityRewardKey: "light-stone", Rules: "每抽消耗20星砂；每日10次；连续9次未获得珍稀奖励时，第10次必得光之石。"},
		{GameKey: "fishing", Name: "水域垂钓", Enabled: true, CostCurrency: 5, DailyLimit: 20, PityThreshold: 5, PityRewardKey: "water-sample", DurationSecond: 60, Rules: "每次抛竿消耗5星砂；每日20次；连续4次未获得珍稀收获时，第5次必得水域样本。"},
	}
	for index := range games {
		tx.Where("game_key = ?", games[index].GameKey).FirstOrCreate(&games[index])
	}
	rewards := []models.ChanceRewardConfig{
		{GameKey: "lottery", RewardKey: "companion-mark", Name: "陪伴印记", Weight: 60, ItemName: "陪伴印记", Quantity: 1, Enabled: true, SortOrder: 10},
		{GameKey: "lottery", RewardKey: "forest-sample", Name: "林地样本", Weight: 30, ItemName: "林地样本", Quantity: 1, Enabled: true, SortOrder: 20},
		{GameKey: "lottery", RewardKey: "eco-sample", Name: "生态样本", Weight: 9, ItemName: "生态样本", Quantity: 1, Enabled: true, SortOrder: 30},
		{GameKey: "lottery", RewardKey: "light-stone", Name: "光之石", Weight: 1, ItemName: "光之石", Quantity: 1, Rare: true, Enabled: true, SortOrder: 40},
		{GameKey: "fishing", RewardKey: "small-fish", Name: "小鱼", Weight: 60, ItemName: "小鱼", Quantity: 1, Enabled: true, SortOrder: 10},
		{GameKey: "fishing", RewardKey: "shell", Name: "贝壳", Weight: 30, ItemName: "贝壳", Quantity: 1, Enabled: true, SortOrder: 20},
		{GameKey: "fishing", RewardKey: "pearl", Name: "珍珠", Weight: 9, ItemName: "珍珠", Quantity: 1, Enabled: true, SortOrder: 30},
		{GameKey: "fishing", RewardKey: "water-sample", Name: "水域样本", Weight: 1, ItemName: "水域样本", Quantity: 1, Rare: true, Enabled: true, SortOrder: 40},
	}
	for index := range rewards {
		tx.Where("game_key = ? AND reward_key = ?", rewards[index].GameKey, rewards[index].RewardKey).FirstOrCreate(&rewards[index])
	}
	for _, name := range []string{"陪伴印记", "林地样本", "生态样本", "光之石", "小鱼", "贝壳", "珍珠", "水域样本"} {
		item := models.ItemConfig{Name: name, Status: "active", Type: "材料", Description: "可用于成长、收藏或玩家交易的探索物品。"}
		tx.Where("name = ?", name).FirstOrCreate(&item)
	}
}

// LoadAllConfigsFromDB 从数据库重新拉取所有配置覆盖内存中全局变量
func LoadAllConfigsFromDB(db *gorm.DB) error {
	LockForWrite()
	defer UnlockForWrite()

	// 1. 加载 SystemConfig
	var sysConfigs []models.SystemConfig
	if err := db.Find(&sysConfigs).Error; err != nil {
		return err
	}
	sysMap := make(map[string]string)
	for _, cfg := range sysConfigs {
		sysMap[cfg.Key] = cfg.Value
	}

	readStr := func(k, def string) string {
		if val, exists := sysMap[k]; exists {
			return val
		}
		return def
	}
	readInt64 := func(k string, def int64) int64 {
		if val, exists := sysMap[k]; exists {
			if v, err := strconv.ParseInt(val, 10, 64); err == nil {
				return v
			}
		}
		return def
	}
	readInt := func(k string, def int) int {
		if val, exists := sysMap[k]; exists {
			if v, err := strconv.Atoi(val); err == nil {
				return v
			}
		}
		return def
	}
	readBool := func(k string, def bool) bool {
		if val, exists := sysMap[k]; exists {
			if v, err := strconv.ParseBool(val); err == nil {
				return v
			}
		}
		return def
	}

	// 填充 Core
	Core = CoreConfig{
		InitialPets:         SplitConfigList(readStr("Core.InitialPets", "光芽兽")),
		CoinName:            readStr("Core.CoinName", "星砂"),
		InitialCoin:         readInt64("Core.InitialCoin", 240),
		RenameCost:          readInt64("Core.RenameCost", 120),
		RenameBlocklist:     SplitConfigList(readStr("Core.RenameBlocklist", "")),
		TreatCost:           readInt64("Core.TreatCost", 80),
		DyingSaveTime:       readInt64("Core.DyingSaveTime", 2000),
		DyingProtectTime:    readInt64("Core.DyingProtectTime", 60),
		EscapeFindTime:      readInt64("Core.EscapeFindTime", 2000),
		LostCooldown:        readInt64("Core.LostCooldown", 720),
		CheckinLike:         readBool("Core.CheckinLike", true),
		CurrencySync:        readInt("Core.CurrencySync", 0),
		CurrencySyncPath:    readStr("Core.CurrencySyncPath", ""),
		CurrencySyncSection: readStr("Core.CurrencySyncSection", ""),
		CurrencySyncKey:     readStr("Core.CurrencySyncKey", ""),
		TxFee:               readInt64("Core.TxFee", 30),
		MasterQQ:            readInt64("Core.MasterQQ", 0),
		NotifyQQ:            readInt64("Core.NotifyQQ", 0),
		ImageHost:           readStr("Core.ImageHost", ""),
	}

	// 填充 Interaction
	Interaction = InteractionConfig{
		TrainGrowth:               readInt64("Interaction.TrainGrowth", 5),
		TrainLimit:                readInt64("Interaction.TrainLimit", 3),
		TrainHungerCost:           readInt64("Interaction.TrainHungerCost", 10),
		StudyGrowth:               readInt64("Interaction.StudyGrowth", 5),
		StudyLimit:                readInt64("Interaction.StudyLimit", 5),
		StudyHungerCost:           readInt64("Interaction.StudyHungerCost", 10),
		WashGrowth:                readInt64("Interaction.WashGrowth", 8),
		WashAffection:             readInt64("Interaction.WashAffection", 10),
		WashHungerCost:            readInt64("Interaction.WashHungerCost", 5),
		WalkGrowth:                readInt64("Interaction.WalkGrowth", 5),
		WalkAffection:             readInt64("Interaction.WalkAffection", 8),
		WalkGrowthLimit:           readInt64("Interaction.WalkGrowthLimit", 20),
		WalkAffectLimit:           readInt64("Interaction.WalkAffectLimit", 24),
		WalkInterval:              readInt64("Interaction.WalkInterval", 600),
		WalkHungerCost:            readInt64("Interaction.WalkHungerCost", 15),
		TouchGrowth:               readInt64("Interaction.TouchGrowth", 8),
		TouchAffection:            readInt64("Interaction.TouchAffection", 10),
		TouchGrowthLimit:          readInt64("Interaction.TouchGrowthLimit", 24),
		TouchAffectLimit:          readInt64("Interaction.TouchAffectLimit", 30),
		TouchInterval:             readInt64("Interaction.TouchInterval", 600),
		TouchHungerCost:           readInt64("Interaction.TouchHungerCost", 5),
		RpsAffection:              readInt64("Interaction.RpsAffection", 8),
		RpsAffectLimit:            readInt64("Interaction.RpsAffectLimit", 24),
		RpsHungerCost:             readInt64("Interaction.RpsHungerCost", 5),
		RpsInterval:               readInt64("Interaction.RpsInterval", 180),
		SnackHungerCost:           readInt64("Interaction.SnackHungerCost", 30),
		SnackSuccess:              readInt64("Interaction.SnackSuccess", 99),
		SnackInterval:             readInt64("Interaction.SnackInterval", 300),
		SnackProtect:              readInt64("Interaction.SnackProtect", 300),
		CounterHunger:             readInt64("Interaction.CounterHunger", 15),
		CounterSuccess:            readInt64("Interaction.CounterSuccess", 80),
		CreateFamilyCoin:          readInt64("Interaction.CreateFamilyCoin", 500),
		CreateFamilyItem:          readStr("Interaction.CreateFamilyItem", "家族商标*1"),
		FamilySizeLimit:           readInt("Interaction.FamilySizeLimit", 10),
		FishHungerCost:            readInt64("Interaction.FishHungerCost", 5),
		FishSuccessRate:           readInt64("Interaction.FishSuccessRate", 80),
		GiftLimit:                 readInt64("Interaction.GiftLimit", 5),
		FishSpecies:               strings.Split(readStr("Interaction.FishSpecies", ""), "#"),
		HungerMoodFlush:           readInt64("Interaction.HungerMoodFlush", 60),
		LotteryItem:               readStr("Interaction.LotteryItem", "抽奖券*10"),
		LotteryRewardStr:          readStr("Interaction.LotteryRewardStr", ""),
		WorkTime:                  readInt64("Interaction.WorkTime", 600),
		WorkRewardCoin:            readInt64("Interaction.WorkRewardCoin", 300),
		WorkRewardItems:           readStr("Interaction.WorkRewardItems", ""),
		WorkHungerCost:            readInt64("Interaction.WorkHungerCost", 20),
		FitnessGrowth:             readInt64("Interaction.FitnessGrowth", 5),
		FitnessLimit:              readInt64("Interaction.FitnessLimit", 5),
		FitnessHungerCost:         readInt64("Interaction.FitnessHungerCost", 12),
		SellNoPriceGrowth:         readInt64("Interaction.SellNoPriceGrowth", 10),
		CommunityBuildGoal:        readInt64("Interaction.CommunityBuildGoal", 155),
		CommunityBuildRewardItems: readStr("Interaction.CommunityBuildRewardItems", "晨露果*1"),
		BuyLimit:                  readInt("Interaction.BuyLimit", 10),
	}

	// 2. 加载 Commands
	var cmdConfigs []models.CommandConfig
	if err := db.Find(&cmdConfigs).Error; err != nil {
		return err
	}
	Commands = make(map[string]string)
	for _, cmd := range cmdConfigs {
		Commands[cmd.FuncName] = cmd.Command
	}

	// 3. 加载 Pets
	var petConfigs []models.PetSpeciesConfig
	if err := db.Find(&petConfigs).Error; err != nil {
		return err
	}
	Pets = make(map[string]PetSpecies)
	for _, p := range petConfigs {
		Pets[p.Name] = PetSpecies{
			Name:            p.Name,
			Image:           p.Image,
			AdoptImage:      p.AdoptImage,
			TrainStartImg:   p.TrainStartImg,
			TrainEndImg:     p.TrainEndImg,
			StudyStartImg:   p.StudyStartImg,
			StudyEndImg:     p.StudyEndImg,
			FitnessStartImg: p.FitnessStartImg,
			FitnessEndImg:   p.FitnessEndImg,
			FavoriteFood:    p.FavoriteFood,
			FavoriteGift:    p.FavoriteGift,
			Health:          p.Health,
			Wisdom:          p.Wisdom,
			Strength:        p.Strength,
			Defense:         p.Defense,
			Hunger:          p.Hunger,
			Description:     p.Description,
			EvolutionBranch: p.EvolutionBranch,
			Evolution:       p.Evolution,
			EvolutionGrowth: p.EvolutionGrowth,
			EvolutionAffect: p.EvolutionAffect,
			EvolutionImage:  p.EvolutionImage,
			Awaken:          p.Awaken,
			AwakenGrowth:    p.AwakenGrowth,
			AwakenAffect:    p.AwakenAffect,
			AwakenItems:     p.AwakenItems,
			AwakenImage:     p.AwakenImage,
			HealthMax:       p.HealthMax,
			WisdomMax:       p.WisdomMax,
			StrengthMax:     p.StrengthMax,
			DefenseMax:      p.DefenseMax,
			HungerMax:       p.HungerMax,
			AffectionBonus:  p.AffectionBonus,
			GrowthBonus:     p.GrowthBonus,
			AttributeBonus:  p.AttributeBonus,
			CurrencyBonus:   p.CurrencyBonus,
		}
	}

	// 4. 加载 Items
	var itemConfigs []models.ItemConfig
	if err := db.Find(&itemConfigs).Error; err != nil {
		return err
	}
	Items = make(map[string]ItemConfig)
	for _, it := range itemConfigs {
		Items[it.Name] = ItemConfig{
			Name:        it.Name,
			Status:      it.Status,
			Type:        it.Type,
			RewardType:  it.RewardType,
			ObtainType:  it.ObtainType,
			OpenReq:     it.OpenReq,
			Effect:      it.Effect,
			Time:        it.Time,
			Image:       it.Image,
			Description: it.Description,
			SellPrice:   it.SellPrice,
		}
	}

	// 5. 加载 Shop & AffectionShop
	var shopItemConfigs []models.ShopItemConfig
	if err := db.Find(&shopItemConfigs).Error; err != nil {
		return err
	}
	Shop = make(map[string]ShopItem)
	AffectionShop = make(map[string]ShopItem)
	for _, s := range shopItemConfigs {
		sitem := ShopItem{
			Name:          s.Name,
			Image:         s.Image,
			Stock:         s.Stock,
			RestockTarget: s.RestockTarget,
			Price:         s.Price,
			Description:   s.Description,
		}
		if s.ShopType == "shop_normal" {
			Shop[s.Name] = sitem
		} else if s.ShopType == "shop_affection" {
			AffectionShop[s.Name] = sitem
		}
	}

	// 6. 加载 Checkin奖励
	var checkinConfigs []models.CheckinRewardConfig
	if err := db.Find(&checkinConfigs).Error; err != nil {
		return err
	}
	NewbieCheckin = make(map[string]CheckinReward)
	WeeklyCheckin = make(map[string]CheckinReward)
	for _, c := range checkinConfigs {
		item := CheckinReward{
			Day:       c.Day,
			Currency:  c.Currency,
			Affection: c.Affection,
			Items:     c.Items,
			Image:     c.Image,
		}
		if c.Type == "checkin_newbie" {
			NewbieCheckin[c.Day] = item
		} else if c.Type == "checkin_weekly" {
			WeeklyCheckin[c.Day] = item
		}
	}

	// 7. 加载 WorkSettings
	var workConfigs []models.WorkSettingConfig
	if err := db.Find(&workConfigs).Error; err != nil {
		return err
	}
	WorkSettings = make(map[string]WorkSetting)
	for _, w := range workConfigs {
		WorkSettings[w.Name] = WorkSetting{
			Name:        w.Name,
			Time:        w.Time,
			HungerCost:  w.HungerCost,
			RewardCoin:  w.RewardCoin,
			RewardItems: w.RewardItems,
			ReplyQuotes: w.ReplyQuotes,
			StartImage:  w.StartImage,
			EndImage:    w.EndImage,
		}
	}

	// 8. 加载 Menus
	var menuConfigs []models.MenuConfig
	if err := db.Find(&menuConfigs).Error; err != nil {
		return err
	}
	Menus = make(map[string]string)
	for _, m := range menuConfigs {
		Menus[m.Name] = m.Reply
	}

	// 9. 加载 Images
	var imgConfigs []models.ImageConfig
	if err := db.Find(&imgConfigs).Error; err != nil {
		return err
	}
	Images = make(map[string]string)
	for _, img := range imgConfigs {
		Images[img.Name] = img.Path
	}

	return nil
}

// encodeGBK 将 UTF-8 字符串编码为 GBK 字节数组，供 Windows 上的易语言外部 INI 使用
func encodeGBK(s string) ([]byte, error) {
	r := transform.NewReader(strings.NewReader(s), simplifiedchinese.GBK.NewEncoder())
	return io.ReadAll(r)
}

func parseSyncPathAndSection(userID, groupID int64) (string, string) {
	path := Core.CurrencySyncPath
	path = strings.ReplaceAll(path, "[qq]", strconv.FormatInt(userID, 10))
	path = strings.ReplaceAll(path, "[QQ]", strconv.FormatInt(userID, 10))
	path = strings.ReplaceAll(path, "[群号]", strconv.FormatInt(groupID, 10))

	section := Core.CurrencySyncSection
	section = strings.ReplaceAll(section, "[qq]", strconv.FormatInt(userID, 10))
	section = strings.ReplaceAll(section, "[QQ]", strconv.FormatInt(userID, 10))
	section = strings.ReplaceAll(section, "[群号]", strconv.FormatInt(groupID, 10))
	return path, section
}

// GetCurrency 从外部对接的 INI 文件中获取代币余额
func GetCurrency(userID, groupID int64, fallbackVal int64) int64 {
	if Core.CurrencySync == 0 || Core.CurrencySyncPath == "" {
		return fallbackVal
	}

	path, section := parseSyncPathAndSection(userID, groupID)
	data, err := os.ReadFile(path)
	if err != nil {
		return fallbackVal
	}

	utf8Data, err := decodeGBK(data)
	if err != nil {
		utf8Data = data
	}

	cfg, err := ini.Load(utf8Data)
	if err != nil {
		return fallbackVal
	}

	sec := cfg.Section(section)
	if sec == nil {
		return fallbackVal
	}

	key := sec.Key(Core.CurrencySyncKey)
	if key == nil {
		return fallbackVal
	}

	val, err := key.Int64()
	if err != nil {
		return fallbackVal
	}

	return val
}

// SetCurrency 将代币余额写入外部对接的 INI 文件中
func SetCurrency(userID, groupID int64, newAmount int64) error {
	if Core.CurrencySync == 0 || Core.CurrencySyncPath == "" {
		return nil
	}

	path, section := parseSyncPathAndSection(userID, groupID)
	var cfg *ini.File
	var err error

	if _, statErr := os.Stat(path); statErr == nil {
		data, readErr := os.ReadFile(path)
		if readErr == nil {
			utf8Data, decodeErr := decodeGBK(data)
			if decodeErr == nil {
				cfg, err = ini.Load(utf8Data)
			} else {
				cfg, err = ini.Load(data)
			}
		}
	}

	if cfg == nil || err != nil {
		cfg = ini.Empty()
	}

	cfg.Section(section).Key(Core.CurrencySyncKey).SetValue(strconv.FormatInt(newAmount, 10))

	var buf bytes.Buffer
	if _, err := cfg.WriteTo(&buf); err != nil {
		return err
	}

	gbkBytes, err := encodeGBK(buf.String())
	if err != nil {
		gbkBytes = buf.Bytes()
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	return os.WriteFile(path, gbkBytes, 0644)
}
