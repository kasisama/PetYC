package config

import "testing"

func TestSplitConfigListAcceptsSeedAndAdminSeparators(t *testing.T) {
	got := SplitConfigList("光芽兽#苔须灵，烬爪兽、团子,团子")
	want := []string{"光芽兽", "苔须灵", "烬爪兽", "团子"}
	if len(got) != len(want) {
		t.Fatalf("got %v", got)
	}
	for index, name := range want {
		if got[index] != name {
			t.Fatalf("got %v", got)
		}
	}
}

func TestStarterPetsFallsBackToLumisprout(t *testing.T) {
	previous := Core.InitialPets
	t.Cleanup(func() { Core.InitialPets = previous })
	Core.InitialPets = []string{"", "  "}
	got := StarterPets()
	if len(got) != 1 || got[0] != "光芽兽" {
		t.Fatalf("got %v", got)
	}
	Core.InitialPets = []string{"光芽兽", "光芽兽", "烬爪兽"}
	got = StarterPets()
	if len(got) != 2 || got[0] != "光芽兽" || got[1] != "烬爪兽" {
		t.Fatalf("got %v", got)
	}
}
