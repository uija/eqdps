package eqlog

import (
	"slices"
	"testing"

	"github.com/uija/eqdps/internal/data"
)

func TestLootSources(t *testing.T) {
	for _, source := range []struct{ text, capture string }{
		{"Reward Chest", "Reward Chest"},
		{"an essence tamer's corpse", "an essence tamer"},
	} {
		for _, item := range []string{"a Rib-bone Stiletto +4", "an Emerald", "6 Mote of Major Potential"} {
			t.Run(source.text+"/"+item, func(t *testing.T) {
				message := "--You have looted " + item + " from " + source.text + ".--"
				kind, captures, ok := classify(message)
				want := []string{message, item, source.capture}
				if !ok || kind != data.LogRowEventTypeLoot || !slices.Equal(captures, want) {
					t.Fatalf("classify = (%v, %q, %v), want Loot with %q", kind, captures, ok, want)
				}
			})
		}
	}
}

func TestLootResultSources(t *testing.T) {
	for _, source := range []struct{ text, capture string }{
		{"Reward Chest", "Reward Chest"},
		{"an essence tamer's corpse", "an essence tamer"},
	} {
		for _, outcome := range []struct{ text, created string }{
			{"and stored it in your currency", ""},
			{"and stored it in your tradeskill depot", ""},
			{"and sold it for 2 platinum and 5 gold.", ""},
			{"and sold it for free.", ""},
			{"to create a Dark Mail Gauntlets +8", "a Dark Mail Gauntlets +8"},
		} {
			message := "You looted 6 Mote of Major Potential from " + source.text + " " + outcome.text
			t.Run(source.text+"/"+outcome.text, func(t *testing.T) {
				kind, captures, ok := classify(message)
				want := []string{message, "6 Mote of Major Potential", source.capture, outcome.text, outcome.created}
				if !ok || kind != data.LogRowEventTypeLootResult || !slices.Equal(captures, want) {
					t.Fatalf("classify = (%v, %q, %v), want LootResult with %q", kind, captures, ok, want)
				}
			})
		}
	}
}
