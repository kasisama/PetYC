package config

import "testing"

func TestSplitConfigListAcceptsSeedAndAdminSeparators(t *testing.T) {
	got := SplitConfigList("诺诺#呱呱，菀菀、团子,团子")
	want := []string{"诺诺", "呱呱", "菀菀", "团子"}
	if len(got) != len(want) {
		t.Fatalf("got %v", got)
	}
	for index, name := range want {
		if got[index] != name {
			t.Fatalf("got %v", got)
		}
	}
}

func TestStarterPetsFallsBackToNono(t *testing.T) {
	previous := Core.InitialPets
	t.Cleanup(func() { Core.InitialPets = previous })
	Core.InitialPets = []string{"", "  "}
	got := StarterPets()
	if len(got) != 1 || got[0] != "诺诺" {
		t.Fatalf("got %v", got)
	}
	Core.InitialPets = []string{"叶伊布", "叶伊布", "冰伊布"}
	got = StarterPets()
	if len(got) != 2 || got[0] != "叶伊布" || got[1] != "冰伊布" {
		t.Fatalf("got %v", got)
	}
}
