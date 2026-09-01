package config

import (
	"gorm.io/gorm"
	"qq-pet-saas/models"
)

var restoredFishingGame = models.ChanceGameConfig{
	GameKey: "fishing", Name: "水域垂钓", Enabled: true, CostCurrency: 5, DailyLimit: 20,
	PityThreshold: 5, PityRewardKey: "water-sample", DurationSecond: 60,
	Rules: "每次抛竿消耗5星砂；每日20次；连续4次未获得珍稀收获时，第5次必得水域样本。",
}

var restoredFishingRewards = []models.ChanceRewardConfig{
	{GameKey: "fishing", RewardKey: "small-fish", Name: "小鱼", ItemName: "小鱼", Quantity: 1, Weight: 60, Enabled: true, SortOrder: 10},
	{GameKey: "fishing", RewardKey: "shell", Name: "贝壳", ItemName: "贝壳", Quantity: 1, Weight: 30, Enabled: true, SortOrder: 20},
	{GameKey: "fishing", RewardKey: "pearl", Name: "珍珠", ItemName: "珍珠", Quantity: 1, Weight: 9, Enabled: true, SortOrder: 30},
	{GameKey: "fishing", RewardKey: "water-sample", Name: "水域样本", ItemName: "水域样本", Quantity: 1, Weight: 1, Rare: true, Enabled: true, SortOrder: 40},
}

// repairKnownBrokenFishingConfig only repairs the known configuration snapshot
// that copied the lottery pool into fishing. Operator-customized pools are left
// untouched.
func repairKnownBrokenFishingConfig(tx *gorm.DB) error {
	if tx == nil || !tx.Migrator().HasTable(&models.ChanceRewardConfig{}) || !tx.Migrator().HasTable(&models.ChanceGameConfig{}) {
		return nil
	}
	var rewards []models.ChanceRewardConfig
	if err := tx.Where("game_key = ?", "fishing").Order("sort_order asc, id asc").Find(&rewards).Error; err != nil {
		return err
	}
	if !isKnownBrokenFishingPool(rewards) {
		return nil
	}
	if err := tx.Where("game_key = ?", "fishing").Delete(&models.ChanceRewardConfig{}).Error; err != nil {
		return err
	}
	restoredRewards := append([]models.ChanceRewardConfig(nil), restoredFishingRewards...)
	for index := range restoredRewards {
		restoredRewards[index].ID = 0
	}
	if err := tx.Create(&restoredRewards).Error; err != nil {
		return err
	}
	var game models.ChanceGameConfig
	if result := tx.Limit(1).Find(&game, "game_key = ?", "fishing"); result.Error != nil {
		return result.Error
	} else if result.RowsAffected > 0 && isKnownBrokenFishingGame(game) {
		if err := tx.Model(&game).Select("name", "enabled", "cost_currency", "cost_item", "cost_quantity", "daily_limit", "pity_threshold", "pity_reward_key", "duration_second", "rules").Updates(restoredFishingGame).Error; err != nil {
			return err
		}
	}
	if tx.Migrator().HasTable(&models.ItemConfig{}) {
		for _, reward := range restoredFishingRewards {
			item := models.ItemConfig{Name: reward.ItemName, Status: "active", Type: "材料", Description: "可通过水域垂钓获得的收藏物。"}
			if err := tx.Where("name = ?", item.Name).FirstOrCreate(&item).Error; err != nil {
				return err
			}
		}
	}
	return nil
}

func isKnownBrokenFishingPool(rows []models.ChanceRewardConfig) bool {
	want := []models.ChanceRewardConfig{
		{RewardKey: "fishing_meadow_fiber", Name: "原野纤维", ItemName: "原野纤维", Quantity: 1, Weight: 60, Currency: 0, Rare: false, Enabled: true, SortOrder: 0},
		{RewardKey: "fishing_survey_ink", Name: "调查墨水", ItemName: "调查墨水", Quantity: 1, Weight: 28, Currency: 0, Rare: false, Enabled: true, SortOrder: 10},
		{RewardKey: "fishing_pressed_flower", Name: "栖光压花", ItemName: "栖光压花", Quantity: 1, Weight: 10, Currency: 0, Rare: true, Enabled: true, SortOrder: 20},
		{RewardKey: "fishing_star_core", Name: "星辉晶核", ItemName: "星辉晶核", Quantity: 1, Weight: 2, Currency: 0, Rare: true, Enabled: true, SortOrder: 30},
	}
	if len(rows) != len(want) {
		return false
	}
	for index, reward := range rows {
		expected := want[index]
		if reward.GameKey != "fishing" || reward.RewardKey != expected.RewardKey || reward.Name != expected.Name ||
			reward.ItemName != expected.ItemName || reward.Quantity != expected.Quantity || reward.Weight != expected.Weight ||
			reward.Currency != expected.Currency || reward.Rare != expected.Rare || reward.Enabled != expected.Enabled ||
			reward.SortOrder != expected.SortOrder {
			return false
		}
	}
	return true
}

func isKnownBrokenFishingGame(game models.ChanceGameConfig) bool {
	return game.Name == "生态垂钓" && game.CostCurrency == 10 && game.DailyLimit == 5 && game.PityRewardKey == "echo_shell" && game.DurationSecond == 60
}
