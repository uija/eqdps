package main

import (
	"errors"
	"os"
	"path/filepath"
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
	if layout.combatantWidth < 20 {
		t.Fatalf("combatant width too small: %d", layout.combatantWidth)
	}
}

func TestHistoryDuration(t *testing.T) {
	tests := map[string]time.Duration{
		"Now":          0,
		"Last Hour":    time.Hour,
		"Last 4 Hours": 4 * time.Hour,
		"Last 8 Hours": 8 * time.Hour,
		"Last Day":     24 * time.Hour,
		"Full":         -time.Nanosecond,
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

func TestFullHistoryUsesBeginningOfLogCutoff(t *testing.T) {
	full, ok := historyDuration("Full")
	if !ok {
		t.Fatal("expected Full history selection")
	}
	cutoff, err := replayCutoff("unused", full, time.Time{})
	if err != nil || !cutoff.IsZero() {
		t.Fatalf("full history must replay from the beginning, cutoff=%v err=%v", cutoff, err)
	}
}

func TestReplayReportsProgressAndSupportsCancellation(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "eqlog.txt")
	log := "[Wed Jul 15 18:53:34 2026] Zonektik begins casting Furor.\n" +
		"[Wed Jul 15 18:53:36 2026] Zonektik hit a dar ghoul knight for 33 points of magic damage by Furor.\n"
	if err := os.WriteFile(logPath, []byte(log), 0o600); err != nil {
		t.Fatal(err)
	}

	var latest replayProgress
	tracker, _, err := replayLogWithProgress(logPath, combat.DefaultIdleTimeout, -time.Nanosecond, time.Time{}, 0, int64(len(log)), func(progress replayProgress) {
		latest = progress
	}, nil)
	if err != nil || tracker == nil || latest.Bytes != latest.Total || latest.Lines != 2 {
		t.Fatalf("unexpected completed replay: progress=%#v tracker=%#v err=%v", latest, tracker, err)
	}

	cancel := make(chan struct{})
	close(cancel)
	if _, _, err := replayLogWithProgress(logPath, combat.DefaultIdleTimeout, -time.Nanosecond, time.Time{}, 0, int64(len(log)), nil, cancel); !errors.Is(err, errReplayCancelled) {
		t.Fatalf("expected replay cancellation, got %v", err)
	}
}

func TestProgressTextShowsPercentageAndLineCount(t *testing.T) {
	got := progressText(replayProgress{Bytes: 50, Total: 100, Lines: 12345})
	for _, want := range []string{"50%", "12345 lines processed", "████"} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected progress text %q to contain %q", got, want)
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
	}, "")
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
	}, "")
	if !strings.Contains(got, "XP 27.0%") {
		t.Fatalf("expected known level progress, got %q", got)
	}
	if strings.Contains(got, "XP ~27.0%") {
		t.Fatalf("did not expect approximation marker, got %q", got)
	}
}

func TestStatusTextShowsActiveFightFilter(t *testing.T) {
	got := statusText(xp.Snapshot{}, "King Tranix")
	for _, want := range []string{"filter: King Tranix", "/", "filter"} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected status %q to contain %q", got, want)
		}
	}
}

func TestFilterSectionsMatchesMobNamesCaseInsensitively(t *testing.T) {
	sections := []combat.DisplaySection{
		{Fight: &combat.Fight{Mob: "King Tranix"}},
		{Fight: &combat.Fight{Mob: "a fire giant warrior"}},
		{Fight: &combat.Fight{Mob: "King Ak'Anon"}},
	}

	filtered := filterSections(sections, "  KING ")
	if len(filtered) != 2 || filtered[0].Fight.Mob != "King Tranix" || filtered[1].Fight.Mob != "King Ak'Anon" {
		t.Fatalf("unexpected filtered sections: %#v", filtered)
	}
	if got := filterSections(sections, ""); len(got) != len(sections) {
		t.Fatalf("empty filter should retain every section: %#v", got)
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

func TestProcessLineCorrelatesCastDamage(t *testing.T) {
	combatTracker := combat.NewFightTracker()
	xpSession := xp.NewSession()
	processLine("[Wed Jul 15 18:53:34 2026] Zonektik begins casting Furor.", combatTracker, xpSession, combat.DefaultIdleTimeout)
	processLine("[Wed Jul 15 18:53:36 2026] Zonektik hit a dar ghoul knight for 33 points of magic damage by Furor.", combatTracker, xpSession, combat.DefaultIdleTimeout)

	fight, _ := combatTracker.DisplayFight()
	player := fight.Meter.Players()[0]
	if player.Breakdown["Magic"].Children["Furor"].Damage != 33 {
		t.Fatalf("expected Furor to be correlated as cast magic: %#v", player)
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
		Name:   "You",
		Damage: 100,
		Breakdown: map[string]*combat.BreakdownStats{
			"DoTs": {
				Name: "DoTs", Damage: 40, Hits: 2, MinHit: 20, MaxHit: 20,
				FirstSeen: started, LastSeen: started.Add(4 * time.Second),
				Children: map[string]*combat.BreakdownStats{},
			},
		},
	}
	table := tview.NewTable()

	nextRow := addDamageBreakdownRows(table, 0, player, 10*time.Second, tableLayoutForWidth(100), "player", map[string]bool{}, map[int]string{})
	if nextRow != 1 {
		t.Fatalf("expected one detail row, got next row %d", nextRow)
	}
	if got := table.GetCell(0, 1).Text; got != "40" {
		t.Fatalf("expected rounded percentage, got %q", got)
	}
	if got := table.GetCell(0, 3).Text; got != "8" {
		t.Fatalf("expected ability DPS in DPS column, got %q", got)
	}
	if got := table.GetCell(0, 4).Text; got != "4" {
		t.Fatalf("expected sustained DPS in SDPS column, got %q", got)
	}
	if got := table.GetCell(0, 5).Text; got != "2" {
		t.Fatalf("expected hit count, got %q", got)
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
	if got := table.GetCell(1, 0).Text; got != "▼ Hoptor Thaggelum (active)" {
		t.Fatalf("unexpected mob header: %q", got)
	}
	if _, ok := actions[1]; !ok {
		t.Fatal("expected mob header to be expandable")
	}
	if got := table.GetCell(2, 3).Text; got != "10" {
		t.Fatalf("expected You active DPS, got %q", got)
	}
	if got := table.GetCell(2, 4).Text; got != "" {
		t.Fatalf("expected equal sustained DPS to be hidden, got %q", got)
	}
	if got := table.GetCell(3, 3).Text; got != "50" {
		t.Fatalf("expected Alice active DPS, got %q", got)
	}
	if got := table.GetCell(3, 4).Text; got != "5" {
		t.Fatalf("expected Alice DPS over shared ten-second mob duration, got %q", got)
	}
	if _, ok := actions[3]; !ok {
		t.Fatal("expected non-local combatant to be expandable")
	}
}

func TestFillTableExpandsDetailsForEveryCombatantAndCategory(t *testing.T) {
	started := time.Date(2026, 7, 16, 12, 0, 0, 0, time.UTC)
	meter := combat.NewMeter()
	meter.Add(combat.Event{Time: started, Source: "Alice", Target: "a fire giant", Amount: 40, Ability: "Flame Proc"})
	meter.Add(combat.Event{Time: started.Add(time.Second), Source: "a fire giant", Target: "Alice", Amount: 25, Attack: "crushes"})
	sections := []combat.DisplaySection{{Fight: &combat.Fight{Mob: "a fire giant", Meter: meter}, Current: true}}
	table := tview.NewTable()
	expanded := make(map[string]bool)

	actions := fillTable(table, sections, expanded, 106)
	aliceKey, ok := actions[2]
	if !ok {
		t.Fatal("expected another player to have expandable details")
	}
	expanded[aliceKey] = true
	actions = fillTable(table, sections, expanded, 106)
	if got := table.GetCell(3, 0).Text; got != "    ▶ Procs" {
		t.Fatalf("expected proc category under Alice, got %q", got)
	}
	procKey, ok := actions[3]
	if !ok {
		t.Fatal("expected proc category to be expandable")
	}
	expanded[procKey] = true
	fillTable(table, sections, expanded, 106)
	if got := table.GetCell(4, 0).Text; got != "        Flame Proc" {
		t.Fatalf("expected individual proc detail, got %q", got)
	}

	// The mob is a combatant too and exposes its melee details.
	if got := table.GetCell(5, 0).Text; got != "  ▶ a fire giant" {
		t.Fatalf("expected mob combatant to remain expandable, got %q", got)
	}
}

func TestFormatPlayerDPSShowsEngagedOnlyWhenMateriallyDifferent(t *testing.T) {
	started := time.Date(2026, 7, 14, 12, 0, 0, 0, time.UTC)
	player := combat.PlayerStats{Name: "You", Damage: 100, FirstSeen: started, LastSeen: started.Add(19 * time.Second), EngagedAt: started.Add(10 * time.Second)}

	if dps, sdps := playerDPSColumns(player, started.Add(19*time.Second), 20*time.Second); dps != "10" || sdps != "5" {
		t.Fatalf("expected materially different engaged and sustained DPS, got %q/%q", dps, sdps)
	}
	player.EngagedAt = started.Add(time.Second)
	if dps, sdps := playerDPSColumns(player, started.Add(19*time.Second), 20*time.Second); dps != "5" || sdps != "" {
		t.Fatalf("expected DPS values within ten percent to collapse, got %q/%q", dps, sdps)
	}
}
