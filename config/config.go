package config

import (
	"bytes"
	"embed"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"unicode/utf16"

	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/transform"
	"gopkg.in/ini.v1"
	"gorm.io/gorm"
	"qq-pet-saas/models"
)

// GlobalConfigPath 配置文件存放路径，默认为根目录下的“初始数据”
var GlobalConfigPath = "./初始数据"

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

// InteractionConfig 对应 核心配置.ini [互动]
type InteractionConfig struct {
	TrainGrowth       int64
	TrainLimit        int64
	TrainHungerCost   int64
	StudyGrowth       int64
	StudyLimit        int64
	StudyHungerCost   int64
	WashGrowth        int64
	WashAffection     int64
	WashHungerCost    int64
	WalkGrowth        int64
	WalkAffection     int64
	WalkGrowthLimit   int64
	WalkAffectLimit   int64
	WalkInterval      int64
	WalkHungerCost    int64
	TouchGrowth       int64
	TouchAffection    int64
	TouchGrowthLimit  int64
	TouchAffectLimit  int64
	TouchInterval     int64
	TouchHungerCost   int64
	RpsAffection      int64
	RpsAffectLimit    int64
	RpsHungerCost     int64
	RpsInterval       int64
	SnackHungerCost   int64
	SnackSuccess      int64
	SnackInterval     int64
	SnackProtect      int64
	CounterHunger     int64
	CounterSuccess    int64
	CreateFamilyCoin  int64
	CreateFamilyItem  string
	FamilySizeLimit   int
	FishHungerCost    int64
	FishSuccessRate   int64
	GiftLimit         int64
	FishSpecies       []string
	HungerMoodFlush   int64
	LotteryItem       string
	LotteryRewardStr  string
	WorkTime          int64
	WorkRewardCoin    int64
	WorkRewardItems   string
	WorkHungerCost    int64
	FitnessGrowth     int64
	FitnessLimit      int64
	FitnessHungerCost int64
	SellNoPriceGrowth int64
	TreeResultNutri   int64
	TreeRewardItems   string
	BuyLimit          int
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

// LoadIniFile loads an INI file safely handles GBK encoding on Windows
func LoadIniFile(path string) (*ini.File, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	utf8Data, err := decodeGBK(data)
	if err != nil {
		return ini.Load(data) // Fallback to raw load if conversion fails
	}
	return ini.Load(utf8Data)
}

func init() {
	LoadAllConfigs()

	models.OnPetFind = func(pet *models.UserPet) {
		if Core.CurrencySync != 0 {
			pet.Currency = GetCurrency(pet.UserID, pet.GroupID, pet.Currency)
		}
	}

	models.OnPetSave = func(pet *models.UserPet) {
		if Core.CurrencySync != 0 {
			_ = SetCurrency(pet.UserID, pet.GroupID, pet.Currency)
		}
	}
}

// LoadAllConfigs loads all game configs from GlobalConfigPath
func LoadAllConfigs() {
	// 1. 加载核心配置
	corePath := filepath.Join(GlobalConfigPath, "核心配置.ini")
	if cfg, err := LoadIniFile(corePath); err == nil {
		secCore := cfg.Section("核心")
		Core = CoreConfig{
			InitialPets:         strings.Split(secCore.Key("初始宠物").String(), "#"),
			CoinName:            secCore.Key("货币名称").MustString("咔币"),
			InitialCoin:         secCore.Key("初始货币").MustInt64(100),
			RenameCost:          secCore.Key("改名消耗").MustInt64(1000),
			RenameBlocklist:     strings.Split(secCore.Key("改名屏蔽").String(), "#"),
			TreatCost:           secCore.Key("治疗消耗").MustInt64(500),
			DyingSaveTime:       secCore.Key("濒死救治时间").MustInt64(2000),
			DyingProtectTime:    secCore.Key("成功救治保护").MustInt64(60),
			EscapeFindTime:      secCore.Key("逃跑找回时间").MustInt64(2000),
			LostCooldown:        secCore.Key("失去宠物冷却").MustInt64(720),
			CheckinLike:         secCore.Key("签到点赞").MustInt(1) == 1,
			CurrencySync:        secCore.Key("货币对接").MustInt(0),
			CurrencySyncPath:    secCore.Key("货币对接路径").MustString(""),
			CurrencySyncSection: secCore.Key("货币配置节").MustString(""),
			CurrencySyncKey:     secCore.Key("货币配置项").MustString(""),
			TxFee:               secCore.Key("交易手续费").MustInt64(30),
			MasterQQ:            secCore.Key("主人QQ").MustInt64(1347993953),
			NotifyQQ:            secCore.Key("通知QQ").MustInt64(3214412864),
			ImageHost:           secCore.Key("图片服务").MustString(""),
		}

		secInt := cfg.Section("互动")
		Interaction = InteractionConfig{
			TrainGrowth:       secInt.Key("锻炼每次成长").MustInt64(5),
			TrainLimit:        secInt.Key("锻炼次数限制").MustInt64(3),
			TrainHungerCost:   secInt.Key("锻炼消耗饱食").MustInt64(10),
			StudyGrowth:       secInt.Key("学习每次成长").MustInt64(5),
			StudyLimit:        secInt.Key("学习次数限制").MustInt64(5),
			StudyHungerCost:   secInt.Key("学习消耗饱食").MustInt64(10),
			WashGrowth:        secInt.Key("洗澡获得成长").MustInt64(8),
			WashAffection:     secInt.Key("洗澡获得好感").MustInt64(10),
			WashHungerCost:    secInt.Key("洗澡消耗饱食").MustInt64(5),
			WalkGrowth:        secInt.Key("散步获得成长").MustInt64(5),
			WalkAffection:     secInt.Key("散步获得好感").MustInt64(8),
			WalkGrowthLimit:   secInt.Key("散步成长上限").MustInt64(20),
			WalkAffectLimit:   secInt.Key("散步好感上限").MustInt64(24),
			WalkInterval:      secInt.Key("散步间隔时间").MustInt64(10) * 60, // 存秒
			WalkHungerCost:    secInt.Key("散步消耗饱食").MustInt64(15),
			TouchGrowth:       secInt.Key("摸头获得成长").MustInt64(8),
			TouchAffection:    secInt.Key("摸头获得好感").MustInt64(10),
			TouchGrowthLimit:  secInt.Key("摸头成长上限").MustInt64(24),
			TouchAffectLimit:  secInt.Key("摸头好感上限").MustInt64(30),
			TouchInterval:     secInt.Key("摸头间隔时间").MustInt64(10) * 60, // 存秒
			TouchHungerCost:   secInt.Key("摸头消耗饱食").MustInt64(5),
			RpsAffection:      secInt.Key("猜拳获得好感").MustInt64(8),
			RpsAffectLimit:    secInt.Key("猜拳好感上限").MustInt64(24),
			RpsHungerCost:     secInt.Key("猜拳消耗饱食").MustInt64(5),
			RpsInterval:       secInt.Key("猜拳间隔时间").MustInt64(3) * 60,
			SnackHungerCost:   secInt.Key("偷袭消耗饱食").MustInt64(30),
			SnackSuccess:      secInt.Key("偷袭成功几率").MustInt64(99),
			SnackInterval:     secInt.Key("偷袭冷却时间").MustInt64(5) * 60,
			SnackProtect:      secInt.Key("偷袭保护时间").MustInt64(5) * 60,
			CounterHunger:     secInt.Key("回击消耗饱食").MustInt64(15),
			CounterSuccess:    secInt.Key("回击成功几率").MustInt64(80),
			CreateFamilyCoin:  secInt.Key("创建家族货币").MustInt64(500),
			CreateFamilyItem:  secInt.Key("创建家族物品").MustString("家族商标*1"),
			FamilySizeLimit:   secInt.Key("家族人数上限").MustInt(10),
			FishHungerCost:    secInt.Key("钓鱼消耗饱食").MustInt64(5),
			FishSuccessRate:   secInt.Key("钓鱼上钩几率").MustInt64(80),
			GiftLimit:         secInt.Key("送礼次数限制").MustInt64(5),
			FishSpecies:       strings.Split(secInt.Key("鱼类").String(), "#"),
			HungerMoodFlush:   secInt.Key("饱食心情刷新").MustInt64(60),
			LotteryItem:       secInt.Key("抽奖所需物品").MustString("抽奖券*10"),
			LotteryRewardStr:  secInt.Key("抽奖奖励设置").String(),
			WorkTime:          secInt.Key("打工时间").MustInt64(10) * 60,
			WorkRewardCoin:    secInt.Key("打工奖励货币").MustInt64(300),
			WorkRewardItems:   secInt.Key("打工奖励物品").String(),
			WorkHungerCost:    secInt.Key("打工消耗饱食").MustInt64(20),
			FitnessGrowth:     secInt.Key("健身奖励成长").MustInt64(5),
			FitnessLimit:      secInt.Key("健身次数限制").MustInt64(5),
			FitnessHungerCost: secInt.Key("健身消耗饱食").MustInt64(12),
			SellNoPriceGrowth: secInt.Key("出售无价成长").MustInt64(10),
			TreeResultNutri:   secInt.Key("神树结果养分").MustInt64(155),
			TreeRewardItems:   secInt.Key("神树奖励物品").MustString("神树果实*1#抽奖券*10"),
			BuyLimit:          secInt.Key("单次购买上限").MustInt(10),
		}

		Images = make(map[string]string)
		secImg := cfg.Section("图片")
		for _, key := range secImg.Keys() {
			Images[key.Name()] = key.String()
		}

	} else {
		log.Printf("[配置] 警告：加载核心配置失败：%v", err)
	}

	// 2. 加载指令配置
	Commands = make(map[string]string)
	cmdPath := filepath.Join(GlobalConfigPath, "指令配置.ini")
	if cfg, err := LoadIniFile(cmdPath); err == nil {
		for _, sec := range cfg.Sections() {
			if sec.Name() == "DEFAULT" {
				continue
			}
			if sec.HasKey("指令") {
				Commands[sec.Name()] = sec.Key("指令").String()
			}
		}
	} else {
		log.Printf("[配置] 警告：加载指令配置失败：%v", err)
	}

	// 3. 加载宠物配置
	Pets = make(map[string]PetSpecies)
	petPath := filepath.Join(GlobalConfigPath, "宠物配置.ini")
	if cfg, err := LoadIniFile(petPath); err == nil {
		for _, sec := range cfg.Sections() {
			if sec.Name() == "DEFAULT" {
				continue
			}
			Pets[sec.Name()] = PetSpecies{
				Name:            sec.Name(),
				Image:           sec.Key("图片").String(),
				AdoptImage:      sec.Key("领养图片").String(),
				TrainStartImg:   sec.Key("开始锻炼图片").String(),
				TrainEndImg:     sec.Key("完成锻炼图片").String(),
				StudyStartImg:   sec.Key("开始学习图片").String(),
				StudyEndImg:     sec.Key("完成学习图片").String(),
				FitnessStartImg: sec.Key("开始健身图片").String(),
				FitnessEndImg:   sec.Key("完成健身图片").String(),
				FavoriteFood:    sec.Key("喜欢食物").String(),
				FavoriteGift:    sec.Key("喜欢礼物").String(),
				Health:          sec.Key("血量").MustInt64(100),
				Wisdom:          sec.Key("智慧").MustInt64(10),
				Strength:        sec.Key("力量").MustInt64(10),
				Defense:         sec.Key("防御").MustInt64(10),
				Hunger:          sec.Key("饱食").MustInt64(100),
				Description:     sec.Key("描述").String(),
				EvolutionBranch: sec.Key("进化分支").MustInt(0),
				Evolution:       sec.Key("进化").String(),
				EvolutionGrowth: sec.Key("进化成长").MustInt64(0),
				EvolutionAffect: sec.Key("进化好感").MustInt64(0),
				EvolutionImage:  sec.Key("进化图片").String(),
				Awaken:          sec.Key("觉醒").String(),
				AwakenGrowth:    sec.Key("觉醒成长").MustInt64(0),
				AwakenAffect:    sec.Key("觉醒好感").MustInt64(0),
				AwakenItems:     sec.Key("觉醒物品").String(),
				AwakenImage:     sec.Key("觉醒图片").String(),
				HealthMax:       sec.Key("血量上限").MustInt64(100),
				WisdomMax:       sec.Key("智慧上限").MustInt64(100),
				StrengthMax:     sec.Key("力量上限").MustInt64(100),
				DefenseMax:      sec.Key("防御上限").MustInt64(100),
				HungerMax:       sec.Key("饱食上限").MustInt64(100),
				AffectionBonus:  sec.Key("好感加成").MustInt(0),
				GrowthBonus:     sec.Key("成长加成").MustInt(0),
				AttributeBonus:  sec.Key("属性加成").MustInt(0),
				CurrencyBonus:   sec.Key("货币加成").MustInt(0),
			}
		}
	} else {
		log.Printf("[配置] 警告：加载宠物配置失败：%v", err)
	}

	// 4. 加载物品配置
	Items = make(map[string]ItemConfig)
	itemPath := filepath.Join(GlobalConfigPath, "物品配置.ini")
	if cfg, err := LoadIniFile(itemPath); err == nil {
		for _, sec := range cfg.Sections() {
			if sec.Name() == "DEFAULT" {
				continue
			}
			Items[sec.Name()] = ItemConfig{
				Name:        sec.Name(),
				Type:        sec.Key("类型").String(),
				RewardType:  sec.Key("礼包类型").String(),
				ObtainType:  sec.Key("获得类型").MustInt(0),
				OpenReq:     sec.Key("打开所需").String(),
				Effect:      sec.Key("效果").String(),
				Time:        sec.Key("时间").MustInt64(0),
				Image:       sec.Key("图片").String(),
				Description: sec.Key("描述").String(),
				SellPrice:   sec.Key("出售价格").MustInt64(0),
			}
		}
	} else {
		log.Printf("[配置] 警告：加载物品配置失败：%v", err)
	}

	// 5. 加载商店配置
	Shop = make(map[string]ShopItem)
	shopPath := filepath.Join(GlobalConfigPath, "商店配置.ini")
	if cfg, err := LoadIniFile(shopPath); err == nil {
		for _, sec := range cfg.Sections() {
			if sec.Name() == "DEFAULT" {
				continue
			}
			Shop[sec.Name()] = ShopItem{
				Name:        sec.Name(),
				Image:       sec.Key("图片").String(),
				Stock:       sec.Key("库存").MustInt64(-1),
				Price:       sec.Key("价格").MustInt64(0),
				Description: sec.Key("描述").String(),
			}
		}
	} else {
		log.Printf("[配置] 警告：加载商店配置失败：%v", err)
	}

	// 6. 加载好感商店配置
	AffectionShop = make(map[string]ShopItem)
	affShopPath := filepath.Join(GlobalConfigPath, "好感商店配置.ini")
	if cfg, err := LoadIniFile(affShopPath); err == nil {
		for _, sec := range cfg.Sections() {
			if sec.Name() == "DEFAULT" {
				continue
			}
			AffectionShop[sec.Name()] = ShopItem{
				Name:        sec.Name(),
				Image:       sec.Key("图片").String(),
				Stock:       sec.Key("库存").MustInt64(-1),
				Price:       sec.Key("价格").MustInt64(0),
				Description: sec.Key("描述").String(),
			}
		}
	} else {
		log.Printf("[配置] 警告：加载好感商店配置失败：%v", err)
	}

	// 7. 加载新手签到配置
	NewbieCheckin = make(map[string]CheckinReward)
	newbiePath := filepath.Join(GlobalConfigPath, "新手签到配置.ini")
	if cfg, err := LoadIniFile(newbiePath); err == nil {
		for _, sec := range cfg.Sections() {
			if sec.Name() == "DEFAULT" {
				continue
			}
			NewbieCheckin[sec.Name()] = CheckinReward{
				Day:       sec.Name(),
				Currency:  sec.Key("货币").MustInt64(0),
				Affection: sec.Key("好感").MustInt64(0),
				Items:     sec.Key("物品").String(),
				Image:     sec.Key("图片").String(),
			}
		}
	} else {
		log.Printf("[配置] 警告：加载新手签到配置失败：%v", err)
	}

	// 8. 加载签到配置
	WeeklyCheckin = make(map[string]CheckinReward)
	weeklyPath := filepath.Join(GlobalConfigPath, "签到配置.ini")
	if cfg, err := LoadIniFile(weeklyPath); err == nil {
		for _, sec := range cfg.Sections() {
			if sec.Name() == "DEFAULT" {
				continue
			}
			WeeklyCheckin[sec.Name()] = CheckinReward{
				Day:       sec.Name(),
				Currency:  sec.Key("货币").MustInt64(0),
				Affection: sec.Key("好感").MustInt64(0),
				Items:     sec.Key("物品").String(),
				Image:     sec.Key("图片").String(),
			}
		}
	} else {
		log.Printf("[配置] 警告：加载周签到配置失败：%v", err)
	}

	// 9. 加载打工配置
	WorkSettings = make(map[string]WorkSetting)
	workPath := filepath.Join(GlobalConfigPath, "打工配置.ini")
	if cfg, err := LoadIniFile(workPath); err == nil {
		for _, sec := range cfg.Sections() {
			if sec.Name() == "DEFAULT" {
				continue
			}
			WorkSettings[sec.Name()] = WorkSetting{
				Name:        sec.Name(),
				Time:        sec.Key("时间").MustInt64(10),
				HungerCost:  sec.Key("消耗饱食").MustInt64(10),
				RewardCoin:  sec.Key("奖励货币").MustInt64(50),
				RewardItems: sec.Key("奖励物品").String(),
				ReplyQuotes: sec.Key("回复语句").String(),
				StartImage:  sec.Key("开始图片").String(),
				EndImage:    sec.Key("结束图片").String(),
			}
		}
	} else {
		log.Printf("[配置] 警告：加载打工配置失败：%v", err)
	}

	// 10. 加载菜单配置
	Menus = make(map[string]string)
	menuPath := filepath.Join(GlobalConfigPath, "菜单配置.ini")
	if cfg, err := LoadIniFile(menuPath); err == nil {
		for _, sec := range cfg.Sections() {
			if sec.Name() == "DEFAULT" {
				continue
			}
			if sec.HasKey("回复") {
				rawReply := sec.Key("回复").String()
				Menus[sec.Name()] = unescapeMenuText(rawReply)
			}
		}
	} else {
		log.Printf("[配置] 警告：加载菜单配置失败：%v", err)
	}
	log.Printf("[配置] 本地配置已加载：指令 %d · 宠物 %d · 物品 %d · 商店 %d · 菜单 %d", len(Commands), len(Pets), len(Items), len(Shop)+len(AffectionShop), len(Menus))
}

// unescapeMenuText 解密菜单配置文本中包含的 \t, \n, 以及 \uXXXX 格式的表情符号
func unescapeMenuText(s string) string {
	s = strings.ReplaceAll(s, `\t`, "  ")
	s = strings.ReplaceAll(s, `\n`, "\n")

	var result strings.Builder
	runes := []rune(s)
	for i := 0; i < len(runes); i++ {
		if runes[i] == '\\' && i+1 < len(runes) && runes[i+1] == 'u' {
			if i+5 < len(runes) {
				hexStr := string(runes[i+2 : i+6])
				code, err := strconv.ParseUint(hexStr, 16, 16)
				if err == nil {
					// 检查是否为 UTF-16 代理对
					if utf16.IsSurrogate(rune(code)) {
						if i+11 < len(runes) && runes[i+6] == '\\' && runes[i+7] == 'u' {
							hexStr2 := string(runes[i+8 : i+12])
							code2, err2 := strconv.ParseUint(hexStr2, 16, 16)
							if err2 == nil {
								r := utf16.DecodeRune(rune(code), rune(code2))
								result.WriteRune(r)
								i += 11
								continue
							}
						}
					}
					result.WriteRune(rune(code))
					i += 5
					continue
				}
			}
		}
		result.WriteRune(runes[i])
	}
	return result.String()
}

// GetCommand 返回功能名对应的自定义指令名称，如果未配置则返回功能名本身
func GetCommand(funcName string) string {
	if cmd, exists := Commands[funcName]; exists && cmd != "" {
		return cmd
	}
	return funcName
}

// SeedFSHolder 存储嵌入的文件系统，用于在后台执行配置重置
var SeedFSHolder embed.FS

// SyncWithDB 进行数据库同步：若数据库配置表为空，则从嵌入文件系统导入种子；若不为空，则从DB读取最新数据覆盖内存。
func SyncWithDB(db *gorm.DB, seedFS embed.FS) error {
	SeedFSHolder = seedFS
	var count int64
	if err := db.Model(&models.SystemConfig{}).Count(&count).Error; err != nil {
		return fmt.Errorf("统计系统配置失败: %w", err)
	}
	if count == 0 {
		log.Println("[配置] 首次启动，正在初始化 SQLite 配置…")
		if err := LoadAllConfigsFromFS(seedFS); err != nil {
			return fmt.Errorf("从嵌入资源加载初始配置失败: %w", err)
		}

		tx := db.Begin()
		if tx.Error != nil {
			return fmt.Errorf("开始种子配置事务失败: %w", tx.Error)
		}
		seedSystemConfig(tx)
		seedCommandConfig(tx)
		seedPetSpeciesConfig(tx)
		seedItemConfig(tx)
		seedShopItemConfig(tx)
		seedCheckinRewardConfig(tx)
		seedWorkSettingConfig(tx)
		seedMenuConfig(tx)
		seedImageConfig(tx)
		if err := tx.Commit().Error; err != nil {
			return fmt.Errorf("提交种子配置事务失败: %w", err)
		}
		log.Println("[配置] 默认配置已写入 SQLite")
	}

	if err := LoadAllConfigsFromDB(db); err != nil {
		return fmt.Errorf("从数据库加载配置失败: %w", err)
	}
	if count != 0 {
		log.Println("[配置] 游戏配置已从 SQLite 加载")
	}
	return nil
}

// ResetConfigsFromSeed 清空 SQLite 中所有配置表并重新运行 SyncWithDB 导入默认种子数据
func ResetConfigsFromSeed(db *gorm.DB) error {
	log.Println("[Config] 收到管理员配置重置请求，开始清空数据库配置表...")
	err := db.Transaction(func(tx *gorm.DB) error {
		tables := []string{
			"system_configs",
			"command_configs",
			"pet_species_configs",
			"item_configs",
			"shop_item_configs",
			"checkin_reward_configs",
			"work_setting_configs",
			"menu_configs",
			"image_configs",
		}
		for _, table := range tables {
			if err := tx.Exec(fmt.Sprintf("DELETE FROM %s", table)).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("清空配置表失败: %w", err)
	}

	return SyncWithDB(db, SeedFSHolder)
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
		"Interaction.TrainGrowth":       strconv.FormatInt(Interaction.TrainGrowth, 10),
		"Interaction.TrainLimit":        strconv.FormatInt(Interaction.TrainLimit, 10),
		"Interaction.TrainHungerCost":   strconv.FormatInt(Interaction.TrainHungerCost, 10),
		"Interaction.StudyGrowth":       strconv.FormatInt(Interaction.StudyGrowth, 10),
		"Interaction.StudyLimit":        strconv.FormatInt(Interaction.StudyLimit, 10),
		"Interaction.StudyHungerCost":   strconv.FormatInt(Interaction.StudyHungerCost, 10),
		"Interaction.WashGrowth":        strconv.FormatInt(Interaction.WashGrowth, 10),
		"Interaction.WashAffection":     strconv.FormatInt(Interaction.WashAffection, 10),
		"Interaction.WashHungerCost":    strconv.FormatInt(Interaction.WashHungerCost, 10),
		"Interaction.WalkGrowth":        strconv.FormatInt(Interaction.WalkGrowth, 10),
		"Interaction.WalkAffection":     strconv.FormatInt(Interaction.WalkAffection, 10),
		"Interaction.WalkGrowthLimit":   strconv.FormatInt(Interaction.WalkGrowthLimit, 10),
		"Interaction.WalkAffectLimit":   strconv.FormatInt(Interaction.WalkAffectLimit, 10),
		"Interaction.WalkInterval":      strconv.FormatInt(Interaction.WalkInterval, 10),
		"Interaction.WalkHungerCost":    strconv.FormatInt(Interaction.WalkHungerCost, 10),
		"Interaction.TouchGrowth":       strconv.FormatInt(Interaction.TouchGrowth, 10),
		"Interaction.TouchAffection":    strconv.FormatInt(Interaction.TouchAffection, 10),
		"Interaction.TouchGrowthLimit":  strconv.FormatInt(Interaction.TouchGrowthLimit, 10),
		"Interaction.TouchAffectLimit":  strconv.FormatInt(Interaction.TouchAffectLimit, 10),
		"Interaction.TouchInterval":     strconv.FormatInt(Interaction.TouchInterval, 10),
		"Interaction.TouchHungerCost":   strconv.FormatInt(Interaction.TouchHungerCost, 10),
		"Interaction.RpsAffection":      strconv.FormatInt(Interaction.RpsAffection, 10),
		"Interaction.RpsAffectLimit":    strconv.FormatInt(Interaction.RpsAffectLimit, 10),
		"Interaction.RpsHungerCost":     strconv.FormatInt(Interaction.RpsHungerCost, 10),
		"Interaction.RpsInterval":       strconv.FormatInt(Interaction.RpsInterval, 10),
		"Interaction.SnackHungerCost":   strconv.FormatInt(Interaction.SnackHungerCost, 10),
		"Interaction.SnackSuccess":      strconv.FormatInt(Interaction.SnackSuccess, 10),
		"Interaction.SnackInterval":     strconv.FormatInt(Interaction.SnackInterval, 10),
		"Interaction.SnackProtect":      strconv.FormatInt(Interaction.SnackProtect, 10),
		"Interaction.CounterHunger":     strconv.FormatInt(Interaction.CounterHunger, 10),
		"Interaction.CounterSuccess":    strconv.FormatInt(Interaction.CounterSuccess, 10),
		"Interaction.CreateFamilyCoin":  strconv.FormatInt(Interaction.CreateFamilyCoin, 10),
		"Interaction.CreateFamilyItem":  Interaction.CreateFamilyItem,
		"Interaction.FamilySizeLimit":   strconv.Itoa(Interaction.FamilySizeLimit),
		"Interaction.FishHungerCost":    strconv.FormatInt(Interaction.FishHungerCost, 10),
		"Interaction.FishSuccessRate":   strconv.FormatInt(Interaction.FishSuccessRate, 10),
		"Interaction.GiftLimit":         strconv.FormatInt(Interaction.GiftLimit, 10),
		"Interaction.FishSpecies":       strings.Join(Interaction.FishSpecies, "#"),
		"Interaction.HungerMoodFlush":   strconv.FormatInt(Interaction.HungerMoodFlush, 10),
		"Interaction.LotteryItem":       Interaction.LotteryItem,
		"Interaction.LotteryRewardStr":  Interaction.LotteryRewardStr,
		"Interaction.WorkTime":          strconv.FormatInt(Interaction.WorkTime, 10),
		"Interaction.WorkRewardCoin":    strconv.FormatInt(Interaction.WorkRewardCoin, 10),
		"Interaction.WorkRewardItems":   Interaction.WorkRewardItems,
		"Interaction.WorkHungerCost":    strconv.FormatInt(Interaction.WorkHungerCost, 10),
		"Interaction.FitnessGrowth":     strconv.FormatInt(Interaction.FitnessGrowth, 10),
		"Interaction.FitnessLimit":      strconv.FormatInt(Interaction.FitnessLimit, 10),
		"Interaction.FitnessHungerCost": strconv.FormatInt(Interaction.FitnessHungerCost, 10),
		"Interaction.SellNoPriceGrowth": strconv.FormatInt(Interaction.SellNoPriceGrowth, 10),
		"Interaction.TreeResultNutri":   strconv.FormatInt(Interaction.TreeResultNutri, 10),
		"Interaction.TreeRewardItems":   Interaction.TreeRewardItems,
		"Interaction.BuyLimit":          strconv.Itoa(Interaction.BuyLimit),
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
		InitialPets:         strings.Split(readStr("Core.InitialPets", "诺诺"), "#"),
		CoinName:            readStr("Core.CoinName", "咔币"),
		InitialCoin:         readInt64("Core.InitialCoin", 100),
		RenameCost:          readInt64("Core.RenameCost", 1000),
		RenameBlocklist:     strings.Split(readStr("Core.RenameBlocklist", ""), "#"),
		TreatCost:           readInt64("Core.TreatCost", 500),
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
		MasterQQ:            readInt64("Core.MasterQQ", 1347993953),
		NotifyQQ:            readInt64("Core.NotifyQQ", 3214412864),
		ImageHost:           readStr("Core.ImageHost", ""),
	}

	// 填充 Interaction
	Interaction = InteractionConfig{
		TrainGrowth:       readInt64("Interaction.TrainGrowth", 5),
		TrainLimit:        readInt64("Interaction.TrainLimit", 3),
		TrainHungerCost:   readInt64("Interaction.TrainHungerCost", 10),
		StudyGrowth:       readInt64("Interaction.StudyGrowth", 5),
		StudyLimit:        readInt64("Interaction.StudyLimit", 5),
		StudyHungerCost:   readInt64("Interaction.StudyHungerCost", 10),
		WashGrowth:        readInt64("Interaction.WashGrowth", 8),
		WashAffection:     readInt64("Interaction.WashAffection", 10),
		WashHungerCost:    readInt64("Interaction.WashHungerCost", 5),
		WalkGrowth:        readInt64("Interaction.WalkGrowth", 5),
		WalkAffection:     readInt64("Interaction.WalkAffection", 8),
		WalkGrowthLimit:   readInt64("Interaction.WalkGrowthLimit", 20),
		WalkAffectLimit:   readInt64("Interaction.WalkAffectLimit", 24),
		WalkInterval:      readInt64("Interaction.WalkInterval", 600),
		WalkHungerCost:    readInt64("Interaction.WalkHungerCost", 15),
		TouchGrowth:       readInt64("Interaction.TouchGrowth", 8),
		TouchAffection:    readInt64("Interaction.TouchAffection", 10),
		TouchGrowthLimit:  readInt64("Interaction.TouchGrowthLimit", 24),
		TouchAffectLimit:  readInt64("Interaction.TouchAffectLimit", 30),
		TouchInterval:     readInt64("Interaction.TouchInterval", 600),
		TouchHungerCost:   readInt64("Interaction.TouchHungerCost", 5),
		RpsAffection:      readInt64("Interaction.RpsAffection", 8),
		RpsAffectLimit:    readInt64("Interaction.RpsAffectLimit", 24),
		RpsHungerCost:     readInt64("Interaction.RpsHungerCost", 5),
		RpsInterval:       readInt64("Interaction.RpsInterval", 180),
		SnackHungerCost:   readInt64("Interaction.SnackHungerCost", 30),
		SnackSuccess:      readInt64("Interaction.SnackSuccess", 99),
		SnackInterval:     readInt64("Interaction.SnackInterval", 300),
		SnackProtect:      readInt64("Interaction.SnackProtect", 300),
		CounterHunger:     readInt64("Interaction.CounterHunger", 15),
		CounterSuccess:    readInt64("Interaction.CounterSuccess", 80),
		CreateFamilyCoin:  readInt64("Interaction.CreateFamilyCoin", 500),
		CreateFamilyItem:  readStr("Interaction.CreateFamilyItem", "家族商标*1"),
		FamilySizeLimit:   readInt("Interaction.FamilySizeLimit", 10),
		FishHungerCost:    readInt64("Interaction.FishHungerCost", 5),
		FishSuccessRate:   readInt64("Interaction.FishSuccessRate", 80),
		GiftLimit:         readInt64("Interaction.GiftLimit", 5),
		FishSpecies:       strings.Split(readStr("Interaction.FishSpecies", ""), "#"),
		HungerMoodFlush:   readInt64("Interaction.HungerMoodFlush", 60),
		LotteryItem:       readStr("Interaction.LotteryItem", "抽奖券*10"),
		LotteryRewardStr:  readStr("Interaction.LotteryRewardStr", ""),
		WorkTime:          readInt64("Interaction.WorkTime", 600),
		WorkRewardCoin:    readInt64("Interaction.WorkRewardCoin", 300),
		WorkRewardItems:   readStr("Interaction.WorkRewardItems", ""),
		WorkHungerCost:    readInt64("Interaction.WorkHungerCost", 20),
		FitnessGrowth:     readInt64("Interaction.FitnessGrowth", 5),
		FitnessLimit:      readInt64("Interaction.FitnessLimit", 5),
		FitnessHungerCost: readInt64("Interaction.FitnessHungerCost", 12),
		SellNoPriceGrowth: readInt64("Interaction.SellNoPriceGrowth", 10),
		TreeResultNutri:   readInt64("Interaction.TreeResultNutri", 155),
		TreeRewardItems:   readStr("Interaction.TreeRewardItems", "神树果实*1#抽奖券*10"),
		BuyLimit:          readInt("Interaction.BuyLimit", 10),
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

// LoadIniFileFromFS 从 embed.FS 读取并解析 INI 文件，支持 GBK 到 UTF-8 解码
func LoadIniFileFromFS(fs embed.FS, path string) (*ini.File, error) {
	data, err := fs.ReadFile(path)
	if err != nil {
		return nil, err
	}
	utf8Data, err := decodeGBK(data)
	if err != nil {
		return ini.Load(data)
	}
	return ini.Load(utf8Data)
}

// ExtractImages 本地无图片目录时，从嵌入的 SeedFS 中自动释出图片素材
func ExtractImages(fs embed.FS) error {
	localDir := "./图片"
	if _, err := os.Stat(localDir); err == nil {
		// 目录已存在，跳过释放
		return nil
	}

	log.Println("[Config] 本地未检测到 '图片' 资源目录，开始从嵌入数据中自动释放图片资源...")
	return walkAndExtract(fs, "初始数据/图片", localDir)
}

func walkAndExtract(fs embed.FS, srcDir, destDir string) error {
	entries, err := fs.ReadDir(srcDir)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(destDir, 0755); err != nil {
		return err
	}

	for _, entry := range entries {
		srcPath := filepath.ToSlash(filepath.Join(srcDir, entry.Name()))
		destPath := filepath.Join(destDir, entry.Name())

		if entry.IsDir() {
			if err := walkAndExtract(fs, srcPath, destPath); err != nil {
				return err
			}
		} else {
			data, err := fs.ReadFile(srcPath)
			if err != nil {
				return err
			}
			if err := os.WriteFile(destPath, data, 0644); err != nil {
				return err
			}
		}
	}
	return nil
}

// LoadAllConfigsFromFS 从 embedded.FS 中读取 INI 文件并填充到内存全局变量中
func LoadAllConfigsFromFS(fs embed.FS) error {
	// 1. 核心配置
	if cfg, err := LoadIniFileFromFS(fs, "初始数据/核心配置.ini"); err == nil {
		secCore := cfg.Section("核心")
		Core = CoreConfig{
			InitialPets:         strings.Split(secCore.Key("初始宠物").String(), "#"),
			CoinName:            secCore.Key("货币名称").MustString("咔币"),
			InitialCoin:         secCore.Key("初始货币").MustInt64(100),
			RenameCost:          secCore.Key("改名消耗").MustInt64(1000),
			RenameBlocklist:     strings.Split(secCore.Key("改名屏蔽").String(), "#"),
			TreatCost:           secCore.Key("治疗消耗").MustInt64(500),
			DyingSaveTime:       secCore.Key("濒死救治时间").MustInt64(2000),
			DyingProtectTime:    secCore.Key("成功救治保护").MustInt64(60),
			EscapeFindTime:      secCore.Key("逃跑找回时间").MustInt64(2000),
			LostCooldown:        secCore.Key("失去宠物冷却").MustInt64(720),
			CheckinLike:         secCore.Key("签到点赞").MustInt(1) == 1,
			CurrencySync:        secCore.Key("货币对接").MustInt(0),
			CurrencySyncPath:    secCore.Key("货币对接路径").MustString(""),
			CurrencySyncSection: secCore.Key("货币配置节").MustString(""),
			CurrencySyncKey:     secCore.Key("货币配置项").MustString(""),
			TxFee:               secCore.Key("交易手续费").MustInt64(30),
			MasterQQ:            secCore.Key("主人QQ").MustInt64(1347993953),
			NotifyQQ:            secCore.Key("通知QQ").MustInt64(3214412864),
			ImageHost:           secCore.Key("图片服务").MustString(""),
		}

		secInt := cfg.Section("互动")
		Interaction = InteractionConfig{
			TrainGrowth:       secInt.Key("锻炼每次成长").MustInt64(5),
			TrainLimit:        secInt.Key("锻炼次数限制").MustInt64(3),
			TrainHungerCost:   secInt.Key("锻炼消耗饱食").MustInt64(10),
			StudyGrowth:       secInt.Key("学习每次成长").MustInt64(5),
			StudyLimit:        secInt.Key("学习次数限制").MustInt64(5),
			StudyHungerCost:   secInt.Key("学习消耗饱食").MustInt64(10),
			WashGrowth:        secInt.Key("洗澡获得成长").MustInt64(8),
			WashAffection:     secInt.Key("洗澡获得好感").MustInt64(10),
			WashHungerCost:    secInt.Key("洗澡消耗饱食").MustInt64(5),
			WalkGrowth:        secInt.Key("散步获得成长").MustInt64(5),
			WalkAffection:     secInt.Key("散步获得好感").MustInt64(8),
			WalkGrowthLimit:   secInt.Key("散步成长上限").MustInt64(20),
			WalkAffectLimit:   secInt.Key("散步好感上限").MustInt64(24),
			WalkInterval:      secInt.Key("散步间隔时间").MustInt64(10) * 60,
			WalkHungerCost:    secInt.Key("散步消耗饱食").MustInt64(15),
			TouchGrowth:       secInt.Key("摸头获得成长").MustInt64(8),
			TouchAffection:    secInt.Key("摸头获得好感").MustInt64(10),
			TouchGrowthLimit:  secInt.Key("摸头成长上限").MustInt64(24),
			TouchAffectLimit:  secInt.Key("摸头好感上限").MustInt64(30),
			TouchInterval:     secInt.Key("摸头间隔时间").MustInt64(10) * 60,
			TouchHungerCost:   secInt.Key("摸头消耗饱食").MustInt64(5),
			RpsAffection:      secInt.Key("猜拳获得好感").MustInt64(8),
			RpsAffectLimit:    secInt.Key("猜拳好感上限").MustInt64(24),
			RpsHungerCost:     secInt.Key("猜拳消耗饱食").MustInt64(5),
			RpsInterval:       secInt.Key("猜拳间隔时间").MustInt64(3) * 60,
			SnackHungerCost:   secInt.Key("偷袭消耗饱食").MustInt64(30),
			SnackSuccess:      secInt.Key("偷袭成功几率").MustInt64(99),
			SnackInterval:     secInt.Key("偷袭冷却时间").MustInt64(5) * 60,
			SnackProtect:      secInt.Key("偷袭保护时间").MustInt64(5) * 60,
			CounterHunger:     secInt.Key("回击消耗饱食").MustInt64(15),
			CounterSuccess:    secInt.Key("回击成功几率").MustInt64(80),
			CreateFamilyCoin:  secInt.Key("创建家族货币").MustInt64(500),
			CreateFamilyItem:  secInt.Key("创建家族物品").MustString("家族商标*1"),
			FamilySizeLimit:   secInt.Key("家族人数上限").MustInt(10),
			FishHungerCost:    secInt.Key("钓鱼消耗饱食").MustInt64(5),
			FishSuccessRate:   secInt.Key("钓鱼上钩几率").MustInt64(80),
			GiftLimit:         secInt.Key("送礼次数限制").MustInt64(5),
			FishSpecies:       strings.Split(secInt.Key("鱼类").String(), "#"),
			HungerMoodFlush:   secInt.Key("饱食心情刷新").MustInt64(60),
			LotteryItem:       secInt.Key("抽奖所需物品").MustString("抽奖券*10"),
			LotteryRewardStr:  secInt.Key("抽奖奖励设置").String(),
			WorkTime:          secInt.Key("打工时间").MustInt64(10) * 60,
			WorkRewardCoin:    secInt.Key("打工奖励货币").MustInt64(300),
			WorkRewardItems:   secInt.Key("打工奖励物品").String(),
			WorkHungerCost:    secInt.Key("打工消耗饱食").MustInt64(20),
			FitnessGrowth:     secInt.Key("健身奖励成长").MustInt64(5),
			FitnessLimit:      secInt.Key("健身次数限制").MustInt64(5),
			FitnessHungerCost: secInt.Key("健身消耗饱食").MustInt64(12),
			SellNoPriceGrowth: secInt.Key("出售无价成长").MustInt64(10),
			TreeResultNutri:   secInt.Key("神树结果养分").MustInt64(155),
			TreeRewardItems:   secInt.Key("神树奖励物品").MustString("神树果实*1#抽奖券*10"),
			BuyLimit:          secInt.Key("单次购买上限").MustInt(10),
		}

		Images = make(map[string]string)
		secImg := cfg.Section("图片")
		for _, key := range secImg.Keys() {
			Images[key.Name()] = key.String()
		}
	} else {
		return fmt.Errorf("加载嵌入核心配置失败: %w", err)
	}

	// 2. 指令配置
	Commands = make(map[string]string)
	if cfg, err := LoadIniFileFromFS(fs, "初始数据/指令配置.ini"); err == nil {
		for _, sec := range cfg.Sections() {
			if sec.Name() == "DEFAULT" {
				continue
			}
			if sec.HasKey("指令") {
				Commands[sec.Name()] = sec.Key("指令").String()
			}
		}
	} else {
		return fmt.Errorf("加载嵌入指令配置失败: %w", err)
	}

	// 3. 宠物配置
	Pets = make(map[string]PetSpecies)
	if cfg, err := LoadIniFileFromFS(fs, "初始数据/宠物配置.ini"); err == nil {
		for _, sec := range cfg.Sections() {
			if sec.Name() == "DEFAULT" {
				continue
			}
			Pets[sec.Name()] = PetSpecies{
				Name:            sec.Name(),
				Image:           sec.Key("图片").String(),
				AdoptImage:      sec.Key("领养图片").String(),
				TrainStartImg:   sec.Key("开始锻炼图片").String(),
				TrainEndImg:     sec.Key("完成锻炼图片").String(),
				StudyStartImg:   sec.Key("开始学习图片").String(),
				StudyEndImg:     sec.Key("完成学习图片").String(),
				FitnessStartImg: sec.Key("开始健身图片").String(),
				FitnessEndImg:   sec.Key("完成健身图片").String(),
				FavoriteFood:    sec.Key("喜欢食物").String(),
				FavoriteGift:    sec.Key("喜欢礼物").String(),
				Health:          sec.Key("血量").MustInt64(100),
				Wisdom:          sec.Key("智慧").MustInt64(10),
				Strength:        sec.Key("力量").MustInt64(10),
				Defense:         sec.Key("防御").MustInt64(10),
				Hunger:          sec.Key("饱食").MustInt64(100),
				Description:     sec.Key("描述").String(),
				EvolutionBranch: sec.Key("进化分支").MustInt(0),
				Evolution:       sec.Key("进化").String(),
				EvolutionGrowth: sec.Key("进化成长").MustInt64(0),
				EvolutionAffect: sec.Key("进化好感").MustInt64(0),
				EvolutionImage:  sec.Key("进化图片").String(),
				Awaken:          sec.Key("觉醒").String(),
				AwakenGrowth:    sec.Key("觉醒成长").MustInt64(0),
				AwakenAffect:    sec.Key("觉醒好感").MustInt64(0),
				AwakenItems:     sec.Key("觉醒物品").String(),
				AwakenImage:     sec.Key("觉醒图片").String(),
				HealthMax:       sec.Key("血量上限").MustInt64(100),
				WisdomMax:       sec.Key("智慧上限").MustInt64(100),
				StrengthMax:     sec.Key("力量上限").MustInt64(100),
				DefenseMax:      sec.Key("防御上限").MustInt64(100),
				HungerMax:       sec.Key("饱食上限").MustInt64(100),
				AffectionBonus:  sec.Key("好感加成").MustInt(0),
				GrowthBonus:     sec.Key("成长加成").MustInt(0),
				AttributeBonus:  sec.Key("属性加成").MustInt(0),
				CurrencyBonus:   sec.Key("货币加成").MustInt(0),
			}
		}
	} else {
		return fmt.Errorf("加载嵌入宠物配置失败: %w", err)
	}

	// 4. 物品配置
	Items = make(map[string]ItemConfig)
	if cfg, err := LoadIniFileFromFS(fs, "初始数据/物品配置.ini"); err == nil {
		for _, sec := range cfg.Sections() {
			if sec.Name() == "DEFAULT" {
				continue
			}
			Items[sec.Name()] = ItemConfig{
				Name:        sec.Name(),
				Type:        sec.Key("类型").String(),
				RewardType:  sec.Key("礼包类型").String(),
				ObtainType:  sec.Key("获得类型").MustInt(0),
				OpenReq:     sec.Key("打开所需").String(),
				Effect:      sec.Key("效果").String(),
				Time:        sec.Key("时间").MustInt64(0),
				Image:       sec.Key("图片").String(),
				Description: sec.Key("描述").String(),
				SellPrice:   sec.Key("出售价格").MustInt64(0),
			}
		}
	} else {
		return fmt.Errorf("加载嵌入物品配置失败: %w", err)
	}

	// 5. 商店配置
	Shop = make(map[string]ShopItem)
	if cfg, err := LoadIniFileFromFS(fs, "初始数据/商店配置.ini"); err == nil {
		for _, sec := range cfg.Sections() {
			if sec.Name() == "DEFAULT" {
				continue
			}
			Shop[sec.Name()] = ShopItem{
				Name:        sec.Name(),
				Image:       sec.Key("图片").String(),
				Stock:       sec.Key("库存").MustInt64(-1),
				Price:       sec.Key("价格").MustInt64(0),
				Description: sec.Key("描述").String(),
			}
		}
	} else {
		return fmt.Errorf("加载嵌入商店配置失败: %w", err)
	}

	// 6. 好感商店配置
	AffectionShop = make(map[string]ShopItem)
	if cfg, err := LoadIniFileFromFS(fs, "初始数据/好感商店配置.ini"); err == nil {
		for _, sec := range cfg.Sections() {
			if sec.Name() == "DEFAULT" {
				continue
			}
			AffectionShop[sec.Name()] = ShopItem{
				Name:        sec.Name(),
				Image:       sec.Key("图片").String(),
				Stock:       sec.Key("库存").MustInt64(-1),
				Price:       sec.Key("价格").MustInt64(0),
				Description: sec.Key("描述").String(),
			}
		}
	} else {
		return fmt.Errorf("加载嵌入好感商店配置失败: %w", err)
	}

	// 7. 新手签到配置
	NewbieCheckin = make(map[string]CheckinReward)
	if cfg, err := LoadIniFileFromFS(fs, "初始数据/新手签到配置.ini"); err == nil {
		for _, sec := range cfg.Sections() {
			if sec.Name() == "DEFAULT" {
				continue
			}
			NewbieCheckin[sec.Name()] = CheckinReward{
				Day:       sec.Name(),
				Currency:  sec.Key("货币").MustInt64(0),
				Affection: sec.Key("好感").MustInt64(0),
				Items:     sec.Key("物品").String(),
				Image:     sec.Key("图片").String(),
			}
		}
	} else {
		return fmt.Errorf("加载嵌入新手签到配置失败: %w", err)
	}

	// 8. 签到配置
	WeeklyCheckin = make(map[string]CheckinReward)
	if cfg, err := LoadIniFileFromFS(fs, "初始数据/签到配置.ini"); err == nil {
		for _, sec := range cfg.Sections() {
			if sec.Name() == "DEFAULT" {
				continue
			}
			WeeklyCheckin[sec.Name()] = CheckinReward{
				Day:       sec.Name(),
				Currency:  sec.Key("货币").MustInt64(0),
				Affection: sec.Key("好感").MustInt64(0),
				Items:     sec.Key("物品").String(),
				Image:     sec.Key("图片").String(),
			}
		}
	} else {
		return fmt.Errorf("加载嵌入周签到配置失败: %w", err)
	}

	// 9. 打工配置
	WorkSettings = make(map[string]WorkSetting)
	if cfg, err := LoadIniFileFromFS(fs, "初始数据/打工配置.ini"); err == nil {
		for _, sec := range cfg.Sections() {
			if sec.Name() == "DEFAULT" {
				continue
			}
			WorkSettings[sec.Name()] = WorkSetting{
				Name:        sec.Name(),
				Time:        sec.Key("时间").MustInt64(10),
				HungerCost:  sec.Key("消耗饱食").MustInt64(10),
				RewardCoin:  sec.Key("奖励货币").MustInt64(50),
				RewardItems: sec.Key("奖励物品").String(),
				ReplyQuotes: sec.Key("回复语句").String(),
				StartImage:  sec.Key("开始图片").String(),
				EndImage:    sec.Key("结束图片").String(),
			}
		}
	} else {
		return fmt.Errorf("加载嵌入打工配置失败: %w", err)
	}

	// 10. 菜单配置
	Menus = make(map[string]string)
	if cfg, err := LoadIniFileFromFS(fs, "初始数据/菜单配置.ini"); err == nil {
		for _, sec := range cfg.Sections() {
			if sec.Name() == "DEFAULT" {
				continue
			}
			if sec.HasKey("回复") {
				rawReply := sec.Key("回复").String()
				Menus[sec.Name()] = unescapeMenuText(rawReply)
			}
		}
	} else {
		return fmt.Errorf("加载嵌入菜单配置失败: %w", err)
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
