package config

import (
	"encoding/json"
	"os"
	"testing"

	"qq-pet-saas/models"
)

func TestDefaultSnapshotsUseNewbieFriendlyCareCosts(t *testing.T) {
	official, err := LoadOfficialSnapshot()
	if err != nil {
		t.Fatal(err)
	}
	baseRaw, err := os.ReadFile("defaults/base_surface.json")
	if err != nil {
		t.Fatal(err)
	}
	var base ConfigSnapshot
	if err = json.Unmarshal(baseRaw, &base); err != nil {
		t.Fatal(err)
	}
	for name, snapshot := range map[string]ConfigSnapshot{"official": official, "base": base} {
		values := map[string]string{}
		for _, row := range snapshot.System {
			values[row.Key] = row.Value
		}
		if values["Core.InitialCoin"] != "240" || values["Core.TreatCost"] != "80" || values["Core.RenameCost"] != "120" {
			t.Fatalf("%s 默认新手数值不一致: %#v", name, values)
		}
	}
}

func TestOfficialSnapshotMeetsLaunchReadiness(t *testing.T) {
	snapshot, err := LoadOfficialSnapshot()
	if err != nil {
		t.Fatal(err)
	}
	if err = ValidateLaunchReadiness(snapshot); err != nil {
		t.Fatal(err)
	}
}

func TestLaunchReadinessRejectsSeasonTokenItem(t *testing.T) {
	snapshot, err := LoadOfficialSnapshot()
	if err != nil {
		t.Fatal(err)
	}
	snapshot.Items = append(snapshot.Items, snapshot.Items[0])
	snapshot.Items[len(snapshot.Items)-1].Key = "season_token"
	if err = ValidateLaunchReadiness(snapshot); err == nil {
		t.Fatal("expected season_token item to fail")
	}
}

func TestLaunchReadinessRejectsMissingMap(t *testing.T) {
	snapshot, err := LoadOfficialSnapshot()
	if err != nil {
		t.Fatal(err)
	}
	snapshot.AdventureMaps = snapshot.AdventureMaps[:2]
	if err = ValidateLaunchReadiness(snapshot); err == nil {
		t.Fatal("expected missing map to fail")
	}
}

func TestLaunchReadinessRejectsMissingCurrencySource(t *testing.T) {
	snapshot, err := LoadOfficialSnapshot()
	if err != nil {
		t.Fatal(err)
	}
	filtered := make([]models.AdventureLootEntryConfig, 0, len(snapshot.AdventureLootEntries))
	for _, entry := range snapshot.AdventureLootEntries {
		if entry.RewardType == "currency" && entry.RewardKey == "journey_badge" {
			continue
		}
		filtered = append(filtered, entry)
	}
	snapshot.AdventureLootEntries = filtered
	if err = ValidateLaunchReadiness(snapshot); err == nil {
		t.Fatal("expected missing journey_badge source to fail")
	}
}

func TestLaunchReadinessRejectsMissingEliteEncounter(t *testing.T) {
	snapshot, err := LoadOfficialSnapshot()
	if err != nil {
		t.Fatal(err)
	}
	for i := range snapshot.AdventureMonsters {
		snapshot.AdventureMonsters[i].Elite = false
	}
	if err = ValidateLaunchReadiness(snapshot); err == nil {
		t.Fatal("expected missing elite encounter to fail")
	}
}

func TestLaunchReadinessRejectsMissingSeasonShop(t *testing.T) {
	snapshot, err := LoadOfficialSnapshot()
	if err != nil {
		t.Fatal(err)
	}
	filtered := make([]models.AdventureShopItemConfig, 0, len(snapshot.AdventureShopItems))
	for _, listing := range snapshot.AdventureShopItems {
		if listing.CurrencyKey == "season_token" {
			continue
		}
		filtered = append(filtered, listing)
	}
	snapshot.AdventureShopItems = filtered
	if err = ValidateLaunchReadiness(snapshot); err == nil {
		t.Fatal("expected missing season shop to fail")
	}
}

func TestLaunchReadinessRejectsUnsourcedItem(t *testing.T) {
	snapshot, err := LoadOfficialSnapshot()
	if err != nil {
		t.Fatal(err)
	}
	filtered := snapshot.AdventureLootEntries[:0]
	for _, entry := range snapshot.AdventureLootEntries {
		if entry.RewardType == "item" && entry.RewardKey == "clear_dew" {
			continue
		}
		filtered = append(filtered, entry)
	}
	snapshot.AdventureLootEntries = filtered
	if err = ValidateLaunchReadiness(snapshot); err == nil {
		t.Fatal("expected unsourced material to fail")
	}
}

func TestLaunchReadinessRejectsRecipeWithoutBlueprintSource(t *testing.T) {
	snapshot, err := LoadOfficialSnapshot()
	if err != nil {
		t.Fatal(err)
	}
	filtered := snapshot.AdventureLootEntries[:0]
	for _, entry := range snapshot.AdventureLootEntries {
		if entry.RewardType == "blueprint_fragment" && entry.RewardKey == "equipment_19" {
			continue
		}
		filtered = append(filtered, entry)
	}
	snapshot.AdventureLootEntries = filtered
	if err = ValidateLaunchReadiness(snapshot); err == nil {
		t.Fatal("expected recipe without blueprint source to fail")
	}
}

func TestLaunchReadinessRejectsNumberedPlaceholderContent(t *testing.T) {
	snapshot, err := LoadOfficialSnapshot()
	if err != nil {
		t.Fatal(err)
	}
	snapshot.AdventureSkills[0].Name = "调查战技·01"
	if err = ValidateLaunchReadiness(snapshot); err == nil {
		t.Fatal("expected numbered placeholder skill to fail")
	}
}
