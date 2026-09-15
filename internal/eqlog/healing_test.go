package eqlog

import (
	"slices"
	"testing"

	"github.com/uija/eqdps/internal/data"
)

func TestHealingPattern(t *testing.T) {
	for _, test := range []struct {
		message string
		fields  []string
	}{
		{"You healed Wyrmberg for 14 hit points by Minor Healing.", []string{"You", "Wyrmberg", "", "14", "", "Minor Healing", ""}},
		{"You healed Wyrmberg over time for 62 hit points by Sprouting Heal. (Critical)", []string{"You", "Wyrmberg", "over time", "62", "", "Sprouting Heal", "Critical"}},
		{"You healed Wyrmberg for 3 (100) hit points by Sprouting Heal Trigger.", []string{"You", "Wyrmberg", "", "3", "100", "Sprouting Heal Trigger", ""}},
		{"Sesik healed himself over time for 1 (31) hit points by Sprouting Heal.", []string{"Sesik", "himself", "over time", "1", "31", "Sprouting Heal", ""}},
		{"Wyrmberg`s warder healed you for 12 hit points by Minor Healing.", []string{"Wyrmberg`s warder", "you", "", "12", "", "Minor Healing", ""}},
		{"You healed Wyrmberg over time for 25 (271) hit points by Stoicism VI. (Critical)", []string{"You", "Wyrmberg", "over time", "25", "271", "Stoicism VI", "Critical"}},
		{"You healed Wyrmberg for 5250 hit points by Promised Renewal Trigger I.", []string{"You", "Wyrmberg", "", "5250", "", "Promised Renewal Trigger I", ""}},
		{"Gabarn healed itself for 0 (4) hit points by Lifetap.", []string{"Gabarn", "itself", "", "0", "4", "Lifetap", ""}},
		{"Firelor healed herself for 1 hit point by Minor Healing.", []string{"Firelor", "herself", "", "1", "", "Minor Healing", ""}},
	} {
		t.Run(test.message, func(t *testing.T) {
			kind, fields, ok := classify(test.message)
			want := append([]string{test.message}, test.fields...)
			if !ok || kind != data.LogRowEventTypeHealing || !slices.Equal(fields, want) {
				t.Fatalf("got %v %q %v; want Healing %q", kind, fields, ok, want)
			}
		})
	}
	for _, message := range []string{
		"Your Stoicism VI spell is interrupted.",
		"You begin casting Minor Healing II.",
		"You hit a rat for 14 points of damage.",
		"You healed Wyrmberg for nope hit points by Minor Healing.",
	} {
		if kind, _, _ := classify(message); kind == data.LogRowEventTypeHealing {
			t.Errorf("incorrectly classified as healing: %s", message)
		}
	}
}
