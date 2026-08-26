package expedition

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"qq-pet-saas/gameplay"
	"qq-pet-saas/models"
)

var (
	ErrChanceGameDisabled = errors.New("该玩法暂未开放")
	ErrDailyLimitReached  = errors.New("今天的参与次数已经用完")
	ErrFishingActive      = errors.New("已经有一根鱼竿在等待收获")
	ErrFishingNotReady    = errors.New("现在收竿还太早")
	ErrNoFishingRun       = errors.New("当前没有等待收竿的鱼竿")
)

var defaultChanceGames = map[string]models.ChanceGameConfig{
	"lottery": {GameKey: "lottery", Name: "幸运抽奖", Enabled: true, CostCurrency: 20, DailyLimit: 10, PityThreshold: 10, PityRewardKey: "light-stone", Rules: "每抽消耗20金币；每日10次；连续9次未获得珍稀奖励时，第10次必得光之石。"},
	"fishing": {GameKey: "fishing", Name: "水域垂钓", Enabled: true, CostCurrency: 5, DailyLimit: 20, PityThreshold: 5, PityRewardKey: "water-sample", DurationSecond: 60, Rules: "每次抛竿消耗5金币；每日20次；连续4次未获得珍稀收获时，第5次必得水域样本。"},
}

var defaultChanceRewards = map[string][]models.ChanceRewardConfig{
	"lottery": {
		{GameKey: "lottery", RewardKey: "companion-mark", Name: "陪伴印记", Weight: 60, ItemName: "陪伴印记", Quantity: 1, Enabled: true, SortOrder: 10},
		{GameKey: "lottery", RewardKey: "forest-sample", Name: "林地样本", Weight: 30, ItemName: "林地样本", Quantity: 1, Enabled: true, SortOrder: 20},
		{GameKey: "lottery", RewardKey: "eco-sample", Name: "生态样本", Weight: 9, ItemName: "生态样本", Quantity: 1, Enabled: true, SortOrder: 30},
		{GameKey: "lottery", RewardKey: "light-stone", Name: "光之石", Weight: 1, ItemName: "光之石", Quantity: 1, Rare: true, Enabled: true, SortOrder: 40},
	},
	"fishing": {
		{GameKey: "fishing", RewardKey: "small-fish", Name: "小鱼", Weight: 60, ItemName: "小鱼", Quantity: 1, Enabled: true, SortOrder: 10},
		{GameKey: "fishing", RewardKey: "shell", Name: "贝壳", Weight: 30, ItemName: "贝壳", Quantity: 1, Enabled: true, SortOrder: 20},
		{GameKey: "fishing", RewardKey: "pearl", Name: "珍珠", Weight: 9, ItemName: "珍珠", Quantity: 1, Enabled: true, SortOrder: 30},
		{GameKey: "fishing", RewardKey: "water-sample", Name: "水域样本", Weight: 1, ItemName: "水域样本", Quantity: 1, Rare: true, Enabled: true, SortOrder: 40},
	},
}

type ChanceRate struct {
	Reward models.ChanceRewardConfig
	Rate   float64
}

type ChanceRules struct {
	Game    models.ChanceGameConfig
	Rewards []ChanceRate
}

type ChanceResult struct {
	Outcome       models.ChanceOutcome
	Attempts      int
	DailyLimit    int
	PityCount     int
	PityThreshold int
	Repeated      bool
}

func (service *Service) GetChanceRules(ctx context.Context, gameKey string) (*ChanceRules, error) {
	game, err := chanceGameTx(service.DB.WithContext(ctx), gameKey)
	if err != nil {
		return nil, err
	}
	rewards, total, err := chanceRewardsTx(service.DB.WithContext(ctx), gameKey)
	if err != nil {
		return nil, err
	}
	rates := make([]ChanceRate, 0, len(rewards))
	for _, reward := range rewards {
		rates = append(rates, ChanceRate{Reward: reward, Rate: float64(reward.Weight) * 100 / float64(total)})
	}
	return &ChanceRules{Game: game, Rewards: rates}, nil
}

func chanceGameTx(tx *gorm.DB, gameKey string) (models.ChanceGameConfig, error) {
	var configured models.ChanceGameConfig
	result := tx.Limit(1).Find(&configured, "game_key = ?", gameKey)
	if result.Error != nil {
		return configured, result.Error
	}
	if result.RowsAffected > 0 {
		if !configured.Enabled {
			return configured, ErrChanceGameDisabled
		}
		return configured, nil
	}
	fallback, exists := defaultChanceGames[gameKey]
	if !exists || !fallback.Enabled {
		return configured, ErrChanceGameDisabled
	}
	return fallback, nil
}

func chanceRewardsTx(tx *gorm.DB, gameKey string) ([]models.ChanceRewardConfig, int, error) {
	var rows []models.ChanceRewardConfig
	if err := tx.Where("game_key = ?", gameKey).Order("sort_order asc, id asc").Find(&rows).Error; err != nil {
		return nil, 0, err
	}
	if len(rows) == 0 {
		rows = append([]models.ChanceRewardConfig(nil), defaultChanceRewards[gameKey]...)
	}
	enabled := make([]models.ChanceRewardConfig, 0, len(rows))
	total := 0
	for _, reward := range rows {
		if !reward.Enabled || reward.Weight <= 0 {
			continue
		}
		enabled = append(enabled, reward)
		total += reward.Weight
	}
	if total <= 0 {
		return nil, 0, errors.New("奖励概率尚未配置")
	}
	return enabled, total, nil
}

func chanceActionKey(gameKey, accountID, sourceKey string) string {
	sourceKey = strings.TrimSpace(sourceKey)
	if sourceKey == "" {
		sourceKey = uuid.NewString()
	}
	return gameKey + ":" + accountID + ":" + sourceKey
}

func (service *Service) PlayLottery(ctx context.Context, accountID, sourceKey string) (*ChanceResult, error) {
	actionKey := chanceActionKey("lottery", accountID, sourceKey)
	if existing, found, err := service.findChanceOutcome(ctx, actionKey); err != nil {
		return nil, err
	} else if found {
		return &ChanceResult{Outcome: existing, Repeated: true}, nil
	}
	var result ChanceResult
	err := gameplay.WithTransactionRetry(ctx, service.DB, func(tx *gorm.DB) error {
		game, err := chanceGameTx(tx, "lottery")
		if err != nil {
			return err
		}
		rewards, total, err := chanceRewardsTx(tx, "lottery")
		if err != nil {
			return err
		}
		attempts, pity, err := service.consumeChanceAttemptTx(tx, accountID, game, service.Now())
		if err != nil {
			return err
		}
		reward, roll, forced, err := service.pickChanceReward(rewards, total, game, pity.PityCount)
		if err != nil {
			return err
		}
		outcome := models.ChanceOutcome{
			ID: uuid.NewString(), AccountID: accountID, GameKey: game.GameKey, ActionKey: actionKey,
			RewardKey: reward.RewardKey, RewardName: reward.Name, ItemName: reward.ItemName,
			Quantity: reward.Quantity, Currency: reward.Currency, Roll: roll, TotalWeight: total,
			PityTriggered: forced, CreatedAt: service.Now(),
		}
		if err = tx.Create(&outcome).Error; err != nil {
			return err
		}
		if err = grantChanceRewardTx(tx, accountID, outcome, "lottery_reward"); err != nil {
			return err
		}
		pityCount, err := updateChancePityTx(tx, pity, reward.Rare || forced, service.Now())
		if err != nil {
			return err
		}
		result = ChanceResult{Outcome: outcome, Attempts: attempts, DailyLimit: game.DailyLimit, PityCount: pityCount, PityThreshold: game.PityThreshold}
		return nil
	})
	if err != nil && isUniqueConstraintError(err) {
		if existing, found, lookupErr := service.findChanceOutcome(ctx, actionKey); lookupErr == nil && found {
			return &ChanceResult{Outcome: existing, Repeated: true}, nil
		}
	}
	return &result, err
}

func (service *Service) findChanceOutcome(ctx context.Context, actionKey string) (models.ChanceOutcome, bool, error) {
	var outcome models.ChanceOutcome
	result := service.DB.WithContext(ctx).Limit(1).Find(&outcome, "action_key = ?", actionKey)
	return outcome, result.RowsAffected > 0, result.Error
}

func (service *Service) consumeChanceAttemptTx(tx *gorm.DB, accountID string, game models.ChanceGameConfig, now time.Time) (int, models.ChancePlayerState, error) {
	day := now.Format("2006-01-02")
	daily := models.ChanceDailyState{AccountID: accountID, GameKey: game.GameKey, Day: day, UpdatedAt: now}
	if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&daily).Error; err != nil {
		return 0, models.ChancePlayerState{}, err
	}
	query := tx.Model(&models.ChanceDailyState{}).Where("account_id = ? AND game_key = ? AND day = ?", accountID, game.GameKey, day)
	if game.DailyLimit > 0 {
		query = query.Where("attempts < ?", game.DailyLimit)
	}
	updated := query.Updates(map[string]interface{}{"attempts": gorm.Expr("attempts + 1"), "updated_at": now})
	if updated.Error != nil {
		return 0, models.ChancePlayerState{}, updated.Error
	}
	if updated.RowsAffected != 1 {
		return 0, models.ChancePlayerState{}, ErrDailyLimitReached
	}
	if err := tx.Where("account_id = ? AND game_key = ? AND day = ?", accountID, game.GameKey, day).First(&daily).Error; err != nil {
		return 0, models.ChancePlayerState{}, err
	}
	pity := models.ChancePlayerState{AccountID: accountID, GameKey: game.GameKey, UpdatedAt: now}
	if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&pity).Error; err != nil {
		return 0, pity, err
	}
	if err := tx.Where("account_id = ? AND game_key = ?", accountID, game.GameKey).First(&pity).Error; err != nil {
		return 0, pity, err
	}
	if game.CostCurrency > 0 {
		if err := gameplay.NewWalletService(tx).DebitTxWithReason(tx, accountID, gameplay.DefaultCurrencyKey, game.CostCurrency, game.GameKey+"_cost", day); err != nil {
			return 0, pity, err
		}
	}
	if game.CostItem != "" && game.CostQuantity > 0 {
		if err := gameplay.NewInventoryService(tx).DebitTx(tx, accountID, game.CostItem, game.CostQuantity); err != nil {
			return 0, pity, err
		}
	}
	return daily.Attempts, pity, nil
}

func (service *Service) pickChanceReward(rewards []models.ChanceRewardConfig, total int, game models.ChanceGameConfig, pityCount int) (models.ChanceRewardConfig, int, bool, error) {
	if game.PityThreshold > 0 && pityCount+1 >= game.PityThreshold {
		for _, reward := range rewards {
			if reward.RewardKey == game.PityRewardKey {
				return reward, -1, true, nil
			}
		}
		return models.ChanceRewardConfig{}, 0, false, errors.New("保底奖励未配置")
	}
	roll, err := service.RandomIntn(total)
	if err != nil {
		return models.ChanceRewardConfig{}, 0, false, err
	}
	cursor := 0
	for _, reward := range rewards {
		cursor += reward.Weight
		if roll < cursor {
			return reward, roll, false, nil
		}
	}
	return models.ChanceRewardConfig{}, roll, false, errors.New("奖励概率计算失败")
}

func updateChancePityTx(tx *gorm.DB, pity models.ChancePlayerState, reset bool, now time.Time) (int, error) {
	next := pity.PityCount + 1
	if reset {
		next = 0
	}
	result := tx.Model(&models.ChancePlayerState{}).Where("id = ?", pity.ID).Updates(map[string]interface{}{"pity_count": next, "updated_at": now})
	return next, result.Error
}

func grantChanceRewardTx(tx *gorm.DB, accountID string, outcome models.ChanceOutcome, reason string) error {
	if outcome.ItemName != "" && outcome.Quantity > 0 {
		if err := gameplay.NewInventoryService(tx).CreditTx(tx, accountID, outcome.ItemName, outcome.Quantity); err != nil {
			return err
		}
	}
	if outcome.Currency > 0 {
		if err := gameplay.NewWalletService(tx).CreditTxWithReason(tx, accountID, gameplay.DefaultCurrencyKey, outcome.Currency, reason, outcome.ID); err != nil {
			return err
		}
	}
	return nil
}

func (service *Service) StartFishing(ctx context.Context, accountID, sourceKey string) (*models.FishingRun, int, int, error) {
	actionKey := chanceActionKey("fishing", accountID, sourceKey)
	var existing models.FishingRun
	lookup := service.DB.WithContext(ctx).Limit(1).Find(&existing, "action_key = ?", actionKey)
	if lookup.Error != nil {
		return nil, 0, 0, lookup.Error
	}
	if lookup.RowsAffected > 0 {
		return &existing, 0, 0, nil
	}
	var run models.FishingRun
	var attempts, dailyLimit int
	err := gameplay.WithTransactionRetry(ctx, service.DB, func(tx *gorm.DB) error {
		game, err := chanceGameTx(tx, "fishing")
		if err != nil {
			return err
		}
		rewards, total, err := chanceRewardsTx(tx, "fishing")
		if err != nil {
			return err
		}
		petRow, petErr := gameplay.ActivePetTx(tx, accountID)
		if petErr != nil {
			return petErr
		}
		pet := *petRow
		if err = gameplay.ReservePetRunTx(tx, accountID, pet.ID); err != nil {
			if errors.Is(err, gameplay.ErrTooManyConcurrentRuns) || errors.Is(err, gameplay.ErrActivityActive) {
				return ErrFishingActive
			}
			return err
		}
		var pity models.ChancePlayerState
		attempts, pity, err = service.consumeChanceAttemptTx(tx, accountID, game, service.Now())
		if err != nil {
			return err
		}
		reward, roll, forced, err := service.pickChanceReward(rewards, total, game, pity.PityCount)
		if err != nil {
			return err
		}
		if _, err = updateChancePityTx(tx, pity, reward.Rare || forced, service.Now()); err != nil {
			return err
		}
		run = models.FishingRun{
			ID: uuid.NewString(), AccountID: accountID, PetID: pet.ID, Status: "running", ActionKey: actionKey,
			RewardKey: reward.RewardKey, RewardName: reward.Name, ItemName: reward.ItemName,
			Quantity: reward.Quantity, Currency: reward.Currency, Roll: roll, TotalWeight: total, Pity: forced,
			StartedAt: service.Now(), ReadyAt: service.Now().Add(time.Duration(game.DurationSecond) * time.Second),
		}
		if err = tx.Create(&run).Error; err != nil {
			if isUniqueConstraintError(err) {
				return ErrFishingActive
			}
			return err
		}
		pet.Status = "钓鱼"
		if err = tx.Save(&pet).Error; err != nil {
			return err
		}
		dailyLimit = game.DailyLimit
		return nil
	})
	return &run, attempts, dailyLimit, err
}

func (service *Service) ClaimFishing(ctx context.Context, accountID string) (*models.FishingRun, error) {
	var run models.FishingRun
	err := gameplay.WithTransactionRetry(ctx, service.DB, func(tx *gorm.DB) error {
		if err := tx.Where("account_id = ? AND status = ?", accountID, "running").Order("started_at desc").First(&run).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrNoFishingRun
			}
			return err
		}
		if service.Now().Before(run.ReadyAt) {
			return ErrFishingNotReady
		}
		now := service.Now()
		updated := tx.Model(&models.FishingRun{}).Where("id = ? AND status = ?", run.ID, "running").Updates(map[string]interface{}{"status": "claimed", "claimed_at": now})
		if updated.Error != nil {
			return updated.Error
		}
		if updated.RowsAffected != 1 {
			return ErrNoFishingRun
		}
		outcome := models.ChanceOutcome{
			ID: uuid.NewString(), AccountID: accountID, GameKey: "fishing", ActionKey: "fishing-claim:" + run.ID,
			RewardKey: run.RewardKey, RewardName: run.RewardName, ItemName: run.ItemName,
			Quantity: run.Quantity, Currency: run.Currency, Roll: run.Roll, TotalWeight: run.TotalWeight,
			PityTriggered: run.Pity, CreatedAt: now,
		}
		if err := tx.Create(&outcome).Error; err != nil {
			return err
		}
		if err := grantChanceRewardTx(tx, accountID, outcome, "fishing_reward"); err != nil {
			return err
		}
		return tx.Model(&models.PetProfile{}).Where("id = ?", run.PetID).Updates(map[string]interface{}{"status": "空闲", "updated_at": now}).Error
	})
	return &run, err
}

func FormatChanceRate(rate float64) string {
	if rate >= 1 {
		return fmt.Sprintf("%.0f%%", rate)
	}
	return fmt.Sprintf("%.1f%%", rate)
}
