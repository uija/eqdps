package main

import (
	"strings"
	"testing"
	"time"

	"github.com/rivo/tview"
	"github.com/uija/eqdps/internal/combat"
	"github.com/uija/eqdps/internal/xp"
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

func TestStatusTextShowsSessionXP(t *testing.T) {
	got := statusText(xp.Snapshot{
		Percent:        97.085,
		LevelPercent:   97.085,
		PercentPerHour: 81.06,
		ActiveDuration: 71*time.Minute + 15*time.Second,
		Gains:          81,
	})
	for _, want := range []string{"XP ~97.1%", "81.1%/h", "~00:03 to level", "reset"} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected status %q to contain %q", got, want)
		}
	}
}

func TestStatusTextShowsKnownProgressWithoutApproximationMarker(t *testing.T) {
	got := statusText(xp.Snapshot{
		LevelPercent:   27,
		ProgressKnown:  true,
		PercentPerHour: 60,
		Gains:          20,
	})
	if !strings.Contains(got, "XP 27.0%") {
		t.Fatalf("expected known level progress, got %q", got)
	}
	if strings.Contains(got, "XP ~27.0%") {
		t.Fatalf("did not expect approximation marker, got %q", got)
	}
}

func TestProcessLineUpdatesCombatAndXPTrackers(t *testing.T) {
	combatTracker := combat.NewFightTracker()
	xpSession := xp.NewSession()
	processLine("[Mon Jul 13 16:46:18 2026] You pierce an elemental visier for 44 points of damage.", combatTracker, xpSession, combat.DefaultIdleTimeout)
	processLine("[Mon Jul 13 16:46:49 2026] You gain experience! (1.239%)", combatTracker, xpSession, combat.DefaultIdleTimeout)

	snapshot := xpSession.SnapshotAtLatestLog()
	if snapshot.Percent != 1.239 || snapshot.Gains != 1 {
		t.Fatalf("unexpected session XP: %#v", snapshot)
	}
	if snapshot.ActiveDuration != 31*time.Second {
		t.Fatalf("expected combat through XP line to count as active, got %s", snapshot.ActiveDuration)
	}
}

func TestProcessLineClosesCombatWhenEnemiesForgetYou(t *testing.T) {
	combatTracker := combat.NewFightTracker()
	xpSession := xp.NewSession()
	processLine("[Mon Jul 13 14:56:40 2026] A lava guardian hits YOU for 20 points of damage.", combatTracker, xpSession, combat.DefaultIdleTimeout)
	processLine("[Mon Jul 13 14:56:50 2026] Your enemies have forgotten you!", combatTracker, xpSession, combat.DefaultIdleTimeout)

	fight, current := combatTracker.DisplayFight()
	if fight == nil || current || fight.EndReason != "enemies forgot you" {
		t.Fatalf("expected aggro clear to close fight, got fight=%#v current=%v", fight, current)
	}
}

func TestDamageBreakdownShowsDPSAndPercentInExpectedColumns(t *testing.T) {
	started := time.Date(2026, 7, 13, 12, 0, 0, 0, time.UTC)
	player := combat.PlayerStats{
		Name:        "You",
		Damage:      100,
		FirstSeen:   started,
		LastSeen:    started.Add(9 * time.Second),
		DamageTypes: map[string]int{"Tuyen's Chant of Flame": 40},
	}
	table := tview.NewTable()

	nextRow := addDamageBreakdownRows(table, 0, player, 10*time.Second, tableLayoutForWidth(100))
	if nextRow != 1 {
		t.Fatalf("expected one detail row, got next row %d", nextRow)
	}
	if got := table.GetCell(0, 2).Text; got != "4.00" {
		t.Fatalf("expected ability DPS in DPS column, got %q", got)
	}
	if got := table.GetCell(0, 6).Text; got != "40.0%" {
		t.Fatalf("expected percentage in Last Target column, got %q", got)
	}
}

func TestFillTableShowsExpandableMobSectionsWithSharedDPS(t *testing.T) {
	started := time.Date(2026, 7, 13, 12, 0, 0, 0, time.UTC)
	meter := combat.NewMeter()
	meter.Add(combat.Event{Time: started, Source: "You", Target: "Hoptor Thaggelum", Amount: 100})
	meter.Add(combat.Event{Time: started.Add(9 * time.Second), Source: "Alice", Target: "Hoptor Thaggelum", Amount: 50})
	sections := []combat.DisplaySection{{
		Fight:   &combat.Fight{Mob: "Hoptor Thaggelum", Meter: meter},
		Current: true,
	}}
	table := tview.NewTable()
	expanded := make(map[string]bool)

	actions := fillTable(table, sections, expanded, 100)
	if got := table.GetCell(1, 0).Text; got != "▼ Hoptor Thaggelum" {
		t.Fatalf("unexpected mob header: %q", got)
	}
	if _, ok := actions[1]; !ok {
		t.Fatal("expected mob header to be expandable")
	}
	if got := table.GetCell(2, 2).Text; got != "10.00" {
		t.Fatalf("expected You DPS over shared ten-second mob duration, got %q", got)
	}
	if got := table.GetCell(3, 2).Text; got != "5.00" {
		t.Fatalf("expected Alice DPS over shared ten-second mob duration, got %q", got)
	}
}

func TestFormatPlayerDPSShowsEngagedOnlyWhenMateriallyDifferent(t *testing.T) {
	started := time.Date(2026, 7, 14, 12, 0, 0, 0, time.UTC)
	player := combat.PlayerStats{Name: "You", Damage: 100, EngagedAt: started.Add(10 * time.Second)}

	if got := formatPlayerDPS(player, started.Add(19*time.Second), 20*time.Second); got != "5.00/10.00" {
		t.Fatalf("expected materially different engaged DPS, got %q", got)
	}
	player.EngagedAt = started.Add(time.Second)
	if got := formatPlayerDPS(player, started.Add(19*time.Second), 20*time.Second); got != "5.00" {
		t.Fatalf("expected DPS values within ten percent to collapse, got %q", got)
	}
}
