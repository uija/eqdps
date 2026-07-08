package main

import (
	"testing"
	"time"
)

func TestFitTextTruncatesWithEllipsis(t *testing.T) {
	if got := fitText("an exceptionally long target name", 12); got != "an except..." {
		t.Fatalf("unexpected truncated text: %q", got)
	}
}

func TestTableLayoutForNarrowWidthKeepsTextColumnsUsable(t *testing.T) {
	layout := tableLayoutForWidth(70)
	if layout.combatantWidth < 10 {
		t.Fatalf("combatant width too small: %d", layout.combatantWidth)
	}
	if layout.targetWidth < 8 {
		t.Fatalf("target width too small: %d", layout.targetWidth)
	}
}

func TestHistoryDuration(t *testing.T) {
	tests := map[string]time.Duration{
		"Now":          0,
		"Last Hour":    time.Hour,
		"Last 4 Hours": 4 * time.Hour,
		"Last 8 Hours": 8 * time.Hour,
		"Last Day":     24 * time.Hour,
	}
	for label, expected := range tests {
		got, ok := historyDuration(label)
		if !ok {
			t.Fatalf("expected %q to be recognized", label)
		}
		if got != expected {
			t.Fatalf("expected %q to map to %s, got %s", label, expected, got)
		}
	}
}
