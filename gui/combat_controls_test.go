package main

import (
	"strings"
	"testing"

	"github.com/uija/eqdps/internal/xp"
)

func TestFightFilterMatchesMobNamesCaseInsensitively(t *testing.T) {
	shell := shell{
		allFights:   []fakeFightSection{{name: "A Rock Golem"}, {name: "an efreeti"}},
		fightFilter: "rock",
	}
	shell.applyFightFilter()
	if len(shell.fights) != 1 || shell.fights[0].name != "A Rock Golem" {
		t.Fatalf("unexpected filtered fights: %#v", shell.fights)
	}
}

func TestEmptyFightFilterRestoresAllFights(t *testing.T) {
	shell := shell{allFights: []fakeFightSection{{name: "one"}, {name: "two"}}, fightFilter: "  "}
	shell.applyFightFilter()
	if len(shell.fights) != 2 {
		t.Fatalf("expected all fights, got %#v", shell.fights)
	}
}

func TestXPStatusUsesObservedSnapshotAndFilter(t *testing.T) {
	got := xpStatusText(xp.Snapshot{Gains: 2, LevelPercent: 12.5, PercentPerHour: 3.25}, "golem")
	if !strings.Contains(got, "XP ~12.5%") || !strings.Contains(got, "3.2%/h") || !strings.Contains(got, "filter: golem") {
		t.Fatalf("unexpected XP status: %q", got)
	}
}

func TestParserStatusReflectsRuntimeState(t *testing.T) {
	if text, _ := parserStatus("live", true); text != "●  LIVE" {
		t.Fatalf("unexpected live status: %q", text)
	}
	if text, _ := parserStatus("", false); text != "●  NO LOG" {
		t.Fatalf("unexpected empty status: %q", text)
	}
}
