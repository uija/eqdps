package eqldb

import "testing"

func TestManualRacesAreUniqueAndNonEmpty(t *testing.T) {
	seen := make(map[string]bool)
	for _, race := range ManualRaces {
		if race == "" || seen[race] {
			t.Fatalf("invalid manual race choice %q", race)
		}
		seen[race] = true
	}
}
