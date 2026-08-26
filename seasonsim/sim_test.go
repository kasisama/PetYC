package seasonsim

import (
	"math"
	"testing"

	"qq-pet-saas/config"
)

func official(t *testing.T) config.ConfigSnapshot {
	t.Helper()
	snapshot, err := config.LoadOfficialSnapshot()
	if err != nil {
		t.Fatal(err)
	}
	return snapshot
}

func runSmall(t *testing.T, snapshot config.ConfigSnapshot, seed int64) Report {
	t.Helper()
	return Run(snapshot, Options{Seed: seed, Players: 400, Days: 70}, nil)
}

func TestSameSeedIsDeterministic(t *testing.T) {
	snapshot := official(t)
	left := runSmall(t, snapshot, 20260826)
	right := runSmall(t, snapshot, 20260826)
	if left.Cohorts["standard"].DailyIncomeP50 != right.Cohorts["standard"].DailyIncomeP50 {
		t.Fatalf("income changed for same seed: %v vs %v", left.Cohorts["standard"].DailyIncomeP50, right.Cohorts["standard"].DailyIncomeP50)
	}
	if left.Cohorts["standard"].EvolveDayP50 != right.Cohorts["standard"].EvolveDayP50 {
		t.Fatalf("evolve day changed for same seed")
	}
}

func TestLowerCheckinAndWorkReducesIncome(t *testing.T) {
	high := official(t)
	for i := range high.CheckinRewards {
		high.CheckinRewards[i].Currency *= 3
	}
	for i := range high.WorkSettings {
		high.WorkSettings[i].RewardCoin *= 3
	}
	low := official(t)
	for i := range low.CheckinRewards {
		low.CheckinRewards[i].Currency /= 4
		if low.CheckinRewards[i].Currency == 0 {
			low.CheckinRewards[i].Currency = 1
		}
	}
	for i := range low.WorkSettings {
		low.WorkSettings[i].RewardCoin /= 4
		if low.WorkSettings[i].RewardCoin == 0 {
			low.WorkSettings[i].RewardCoin = 1
		}
	}
	highRep := runSmall(t, high, 20260826)
	lowRep := runSmall(t, low, 20260826)
	if lowRep.Cohorts["standard"].DailyIncomeP50 >= highRep.Cohorts["standard"].DailyIncomeP50*0.7 {
		t.Fatalf("income did not drop with weaker checkin/work: low=%v high=%v", lowRep.Cohorts["standard"].DailyIncomeP50, highRep.Cohorts["standard"].DailyIncomeP50)
	}
	if lowRep.Cohorts["standard"].DailyIncomeP50 >= 250 && lowRep.Cohorts["standard"].DailyIncomeP50 <= 350 && highRep.Cohorts["standard"].DailyIncomeP50 == lowRep.Cohorts["standard"].DailyIncomeP50 {
		t.Fatal("income appears clamped to the launch target band")
	}
}

func TestHigherShopPricesChangeSpend(t *testing.T) {
	cheap := official(t)
	dear := official(t)
	for i := range dear.ShopItems {
		dear.ShopItems[i].Price *= 3
	}
	cheapRep := runSmall(t, cheap, 20260826)
	dearRep := runSmall(t, dear, 20260826)
	spendChanged := math.Abs(cheapRep.Cohorts["standard"].DailySpendP50-dearRep.Cohorts["standard"].DailySpendP50) >= 5
	buysChanged := math.Abs(cheapRep.Cohorts["standard"].ShopBuysP50-dearRep.Cohorts["standard"].ShopBuysP50) >= 0.05
	if !spendChanged && !buysChanged {
		t.Fatalf("shop prices did not change spend or purchase count: cheap spend=%v buys=%v dear spend=%v buys=%v", cheapRep.Cohorts["standard"].DailySpendP50, cheapRep.Cohorts["standard"].ShopBuysP50, dearRep.Cohorts["standard"].DailySpendP50, dearRep.Cohorts["standard"].ShopBuysP50)
	}
}

func TestHigherEvolutionGrowthDelaysEvolve(t *testing.T) {
	base := official(t)
	hard := official(t)
	for i := range hard.PetEvolutionRules {
		if hard.PetEvolutionRules[i].RequiredGrowth > 0 && hard.PetEvolutionRules[i].RequiredGrowth < 3000 {
			hard.PetEvolutionRules[i].RequiredGrowth *= 8
		}
	}
	easy := runSmall(t, base, 20260826)
	late := runSmall(t, hard, 20260826)
	if late.Cohorts["standard"].EvolveRate >= easy.Cohorts["standard"].EvolveRate && late.Cohorts["standard"].EvolveDayP50 <= easy.Cohorts["standard"].EvolveDayP50 {
		t.Fatalf("raising growth requirement did not delay evolution: base rate=%v day=%v hard rate=%v day=%v", easy.Cohorts["standard"].EvolveRate, easy.Cohorts["standard"].EvolveDayP50, late.Cohorts["standard"].EvolveRate, late.Cohorts["standard"].EvolveDayP50)
	}
}

func TestRemovingAwakenMaterialsStopsAwaken(t *testing.T) {
	base := official(t)
	cut := official(t)
	for i := range cut.AdventureLootEntries {
		if cut.AdventureLootEntries[i].RewardKey == "dawn_core" || cut.AdventureLootEntries[i].RewardKey == "mist_core" {
			cut.AdventureLootEntries[i].RewardKey = "meadow_fiber"
		}
	}
	for i := range cut.RewardTracks {
		if cut.RewardTracks[i].RewardKey == "dawn_core" || cut.RewardTracks[i].RewardKey == "mist_core" {
			cut.RewardTracks[i].RewardKey = "meadow_fiber"
		}
	}
	for i := range cut.AdventureShopItems {
		if cut.AdventureShopItems[i].ProductKey == "dawn_core" || cut.AdventureShopItems[i].ProductKey == "mist_core" {
			cut.AdventureShopItems[i].Enabled = false
		}
	}
	baseRep := runSmall(t, base, 20260826)
	cutRep := runSmall(t, cut, 20260826)
	if cutRep.Cohorts["standard"].AwakenRate >= baseRep.Cohorts["standard"].AwakenRate && cutRep.Cohorts["standard"].AwakenRate > 0.05 {
		t.Fatalf("removing awaken material sources did not reduce awaken rate: base=%v cut=%v", baseRep.Cohorts["standard"].AwakenRate, cutRep.Cohorts["standard"].AwakenRate)
	}
}

func TestFewerEquipmentDropsIncreaseReplaceGap(t *testing.T) {
	base := official(t)
	sparse := official(t)
	for i := range sparse.AdventureLootEntries {
		if sparse.AdventureLootEntries[i].RewardType == "equipment" {
			sparse.AdventureLootEntries[i].Weight = 1
		}
	}
	baseRep := runSmall(t, base, 20260826)
	sparseRep := runSmall(t, sparse, 20260826)
	if sparseRep.EquipmentReplace.EarlyP50+sparseRep.EquipmentReplace.MidP50 <= baseRep.EquipmentReplace.EarlyP50+baseRep.EquipmentReplace.MidP50 {
		t.Fatalf("equipment gaps did not increase: base=%v/%v sparse=%v/%v", baseRep.EquipmentReplace.EarlyP50, baseRep.EquipmentReplace.MidP50, sparseRep.EquipmentReplace.EarlyP50, sparseRep.EquipmentReplace.MidP50)
	}
}

func TestHigherXPCurveDelaysLevels(t *testing.T) {
	base := official(t)
	slow := official(t)
	for i := range slow.AdventureLevels {
		if slow.AdventureLevels[i].XPToNext > 0 {
			slow.AdventureLevels[i].XPToNext *= 5
		}
	}
	baseRep := runSmall(t, base, 20260826)
	slowRep := runSmall(t, slow, 20260826)
	if slowRep.LevelArrival["10"].DayP50 <= baseRep.LevelArrival["10"].DayP50 && slowRep.LevelArrival["10"].Players >= baseRep.LevelArrival["10"].Players {
		t.Fatalf("xp curve change did not delay level 10: base day=%v players=%v slow day=%v players=%v", baseRep.LevelArrival["10"].DayP50, baseRep.LevelArrival["10"].Players, slowRep.LevelArrival["10"].DayP50, slowRep.LevelArrival["10"].Players)
	}
}

func TestOfficialProgressTargetsCoverLevelsAndMaps(t *testing.T) {
	report := runSmall(t, official(t), 20260826)
	if !report.Targets["progress_ok"] {
		t.Fatalf("official progress target failed: lv15=%+v first=%+v second=%+v third=%+v", report.LevelArrival["15"], report.ZoneUnlock["zone_04"], report.ZoneUnlock["zone_05"], report.ZoneUnlock["zone_09"])
	}
	if report.LevelArrival["15"].Players == 0 || report.ZoneUnlock["zone_09"].UnlockedPlayers == 0 {
		t.Fatal("third map progression must be reachable during the season")
	}
}

func TestBossHealthChangesSharedClearRate(t *testing.T) {
	base := official(t)
	hard := official(t)
	for i := range hard.AdventureBosses {
		hard.AdventureBosses[i].MaxHealth *= 100
	}
	baseReport := runSmall(t, base, 20260826)
	hardReport := runSmall(t, hard, 20260826)
	if hardReport.Cohorts["standard"].BossClearRate >= baseReport.Cohorts["standard"].BossClearRate {
		t.Fatalf("boss clear rate ignored configured health: base=%v hard=%v", baseReport.Cohorts["standard"].BossClearRate, hardReport.Cohorts["standard"].BossClearRate)
	}
}

func TestSkillsAffectFamilyCombatMetrics(t *testing.T) {
	base := official(t)
	weak := official(t)
	for i := range weak.AdventureSkills {
		weak.AdventureSkills[i].PowerPermille = 1
		weak.AdventureSkills[i].WisdomPermille = 0
		weak.AdventureSkills[i].EffectValue = 0
	}
	baseReport := runSmall(t, base, 20260826)
	tweakReport := runSmall(t, weak, 20260826)
	if tweakReport.Families["lumisprout"].MeanPower >= baseReport.Families["lumisprout"].MeanPower {
		t.Fatalf("combat metric ignored configured skills: base=%v tweak=%v", baseReport.Families["lumisprout"].MeanPower, tweakReport.Families["lumisprout"].MeanPower)
	}
}

func TestBlueprintSourcesAffectCrafting(t *testing.T) {
	base := official(t)
	cut := official(t)
	for i := range cut.AdventureLootEntries {
		if cut.AdventureLootEntries[i].RewardType == "blueprint_fragment" {
			cut.AdventureLootEntries[i].Weight = 0
			cut.AdventureLootEntries[i].Guaranteed = false
		}
	}
	baseReport := runSmall(t, base, 20260826)
	cutReport := runSmall(t, cut, 20260826)
	if cutReport.Cohorts["standard"].CraftsP50 >= baseReport.Cohorts["standard"].CraftsP50 && baseReport.Cohorts["standard"].CraftsP50 > 0 {
		t.Fatalf("crafting ignored blueprint sources: base=%v cut=%v", baseReport.Cohorts["standard"].CraftsP50, cutReport.Cohorts["standard"].CraftsP50)
	}
}
