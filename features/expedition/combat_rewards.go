package expedition

import (
	"fmt"

	"gorm.io/gorm"
	"qq-pet-saas/config"
	"qq-pet-saas/gameplay"
	"qq-pet-saas/models"
)

const (
	combatVictoryCurrencyMinKey = "Adventure.CombatVictoryCurrencyMin"
	combatVictoryCurrencyMaxKey = "Adventure.CombatVictoryCurrencyMax"
)

// grantCombatVictoryCurrencyTx adds a small, predictable baseline reward to a
// normal map-combat victory. It is deliberately excluded from boss combat and
// shares the combat ID with the rest of the victory transaction for tracing.
func (service *Service) grantCombatVictoryCurrencyTx(tx *gorm.DB, combat *models.AdventureCombatSession) (AdventureReward, error) {
	minimum := config.LiveInt64(tx, combatVictoryCurrencyMinKey, 3)
	maximum := config.LiveInt64(tx, combatVictoryCurrencyMaxKey, 8)
	if minimum < 1 {
		minimum = 1
	}
	if maximum < minimum {
		maximum = minimum
	}
	amount := minimum
	if maximum > minimum {
		roll, err := service.RandomIntn(int(maximum - minimum + 1))
		if err != nil {
			return AdventureReward{}, err
		}
		amount += int64(roll)
	}
	wallet := gameplay.NewWalletService(tx)
	wallet.Now = service.Now
	if err := wallet.CreditTxWithReason(tx, combat.AccountID, gameplay.DefaultCurrencyKey, amount, "adventure_combat_victory", fmt.Sprintf("combat:%s", combat.ID)); err != nil {
		return AdventureReward{}, err
	}
	return AdventureReward{Type: "currency", Key: gameplay.DefaultCurrencyKey, Name: currencyLabel(tx, gameplay.DefaultCurrencyKey), Quantity: amount}, nil
}
