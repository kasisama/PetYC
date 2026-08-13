package admin

import "testing"

func TestConfigSchemasExposeLiveEventsAndRewardTracks(t *testing.T) {
	schemas := knownConfigSchemas()
	found := map[string]bool{}
	for _, schema := range schemas {
		found[schema] = true
	}
	for _, required := range []string{"live_events", "reward_tracks"} {
		if !found[required] {
			t.Fatalf("configuration center is missing %s", required)
		}
	}
}
