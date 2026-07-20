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
	"github.com/uija/eqdps/internal/engine"
	"github.com/uija/eqdps/internal/skyquest"
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

func TestTableLayoutGivesRemainingWidthToCombatantColumn(t *testing.T) {
	layout := tableLayoutForWidth(120)
	if layout.combatantWidth != 59 {
		t.Fatalf("expected remaining width in combatant column, got %d", layout.combatantWidth)
	}
}

func TestCombatantCellKeepsTextForTviewExpansion(t *testing.T) {
	layout := tableLayoutForWidth(100)
	name := "a very long combatant name that can use spare terminal width"
	cell := tableCell(name, 0, layout)
	if cell.Text != name {
		t.Fatalf("combatant text was truncated before layout: %q", cell.Text)
	}
	if cell.MaxWidth != layout.combatantWidth || cell.Expansion != 1 {
		t.Fatalf("combatant cell does not participate in responsive layout: max=%d expansion=%d", cell.MaxWidth, cell.Expansion)
	}
}

func TestRestoreTablePositionFollowsLogicalRowAndKeepsViewport(t *testing.T) {
	table := tview.NewTable().SetSelectable(true, false)
	table.Select(7, 0)
	table.SetOffset(4, 2)

	restoreTablePosition(table, map[int]string{25: "selected", 7: "child"}, "selected", 4, 2, false)
	row, _ := table.GetSelection()
	rowOffset, columnOffset := table.GetOffset()
	if row != 25 || rowOffset != 4 || columnOffset != 2 {
		t.Fatalf("unexpected restored position: row=%d offset=%d,%d", row, rowOffset, columnOffset)
	}
}

func TestRestoreTablePositionTracksNewRowsWhenViewWasAtEnd(t *testing.T) {
	table := tview.NewTable().SetSelectable(true, false)
	table.SetRect(0, 0, 80, 10)
	for row := 0; row < 30; row++ {
		table.SetCellSimple(row, 0, "row")
	}
	table.Select(20, 0)
	table.SetOffset(20, 0)

	for row := 30; row < 40; row++ {
		table.SetCellSimple(row, 0, "expanded row")
	}
	restoreTablePosition(table, map[int]string{20: "selected"}, "selected", 20, 0, true)

	row, _ := table.GetSelection()
	rowOffset, _ := table.GetOffset()
	if row != 20 || rowOffset != table.GetRowCount() {
		t.Fatalf("expected selection to remain at row 20 and viewport to track end, got row=%d offset=%d", row, rowOffset)
	}
}

func TestTableViewAtEnd(t *testing.T) {
	table := tview.NewTable()
	table.SetRect(0, 0, 80, 10)
	for row := 0; row < 30; row++ {
		table.SetCellSimple(row, 0, "row")
	}

	if !tableViewAtEnd(table, 20) {
		t.Fatal("expected viewport ending at the last row to be detected")
	}
	if tableViewAtEnd(table, 19) {
		t.Fatal("viewport before the last row was detected as being at the end")
	}
}

func TestScrollBarMetrics(t *testing.T) {
	if _, _, visible := scrollBarMetrics(10, 10, 0); visible {
		t.Fatal("scrollbar should be hidden when all rows fit")
	}

	start, height, visible := scrollBarMetrics(100, 20, 40)
	if !visible || height != 4 || start != 8 {
		t.Fatalf("unexpected middle scrollbar: start=%d height=%d visible=%t", start, height, visible)
	}

	start, height, visible = scrollBarMetrics(100, 20, 80)
	if !visible || height != 4 || start != 16 {
		t.Fatalf("unexpected end scrollbar: start=%d height=%d visible=%t", start, height, visible)
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

func TestReplayCutoffCanBeCancelledDuringLatestTimestampScan(t *testing.T) {
	path := filepath.Join(t.TempDir(), "eqlog_Test_server.txt")
	var log strings.Builder
	for index := 0; index < 2000; index++ {
		log.WriteString("[Thu Jul 16 12:00:00 2026] You say, 'test'\n")
	}
	if err := os.WriteFile(path, []byte(log.String()), 0o600); err != nil {
		t.Fatal(err)
	}
	cancel := make(chan struct{})
	close(cancel)

	_, err := engine.ReplayCutoffWithCancel(path, time.Hour, time.Time{}, cancel)
	if !errors.Is(err, engine.ErrReplayCancelled) {
		t.Fatalf("expected replay cancellation, got %v", err)
	}
}

func TestFullHistoryUsesBeginningOfLogCutoff(t *testing.T) {
	full, ok := historyDuration("Full")
	if !ok {
		t.Fatal("expected Full history selection")
	}
	cutoff, err := engine.ReplayCutoff("unused", full, time.Time{})
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

	var latest engine.ReplayProgress
	tracker, _, err := engine.ReplayWithProgress(logPath, combat.DefaultIdleTimeout, -time.Nanosecond, time.Time{}, 0, int64(len(log)), func(progress engine.ReplayProgress) {
		latest = progress
	}, nil)
	if err != nil || tracker == nil || latest.Bytes != latest.Total || latest.Lines != 2 {
		t.Fatalf("unexpected completed replay: progress=%#v tracker=%#v err=%v", latest, tracker, err)
	}

	cancel := make(chan struct{})
	close(cancel)
	if _, _, err := engine.ReplayWithProgress(logPath, combat.DefaultIdleTimeout, -time.Nanosecond, time.Time{}, 0, int64(len(log)), nil, cancel); !errors.Is(err, engine.ErrReplayCancelled) {
		t.Fatalf("expected replay cancellation, got %v", err)
	}
}

func TestProgressTextShowsPercentageAndLineCount(t *testing.T) {
	got := operationProgressText("Loading combat history…", 512*1024, 1024*1024, 12345)
	for _, want := range []string{"Loading combat history…", "50%", "512.0 KiB / 1.0 MiB", "12345 lines processed", "████"} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected progress text %q to contain %q", got, want)
		}
	}
}

func TestOperationsShareDetailedProgressText(t *testing.T) {
	got := operationProgressText("Scanning existing loot history…", 512*1024, 1024*1024, 4321)
	for _, want := range []string{"Scanning existing loot history…", "50%", "512.0 KiB / 1.0 MiB", "4321 lines processed", "████"} {
		if !strings.Contains(got, want) {
			t.Fatalf("progress text %q does not contain %q", got, want)
		}
	}
}

func TestLargeSkyCatchupUsesOverlayOnlyInTUI(t *testing.T) {
	if needsSkyCatchupOverlay(skyCatchupOverlayThreshold, false) {
		t.Fatal("threshold-sized catch-up should remain silent")
	}
	if !needsSkyCatchupOverlay(skyCatchupOverlayThreshold+1, false) {
		t.Fatal("catch-up above threshold should use overlay")
	}
	if needsSkyCatchupOverlay(skyCatchupOverlayThreshold+1, true) {
		t.Fatal("text mode cannot use TUI overlay")
	}
}

func TestCatchupBoundarySuppressesHistoricalCombatButKeepsCompletedPartialLine(t *testing.T) {
	const target = int64(1000)
	if isLiveLineAfterCatchup(target, target) {
		t.Fatal("line ending at catch-up boundary is historical")
	}
	if !isLiveLineAfterCatchup(target+1, target) {
		t.Fatal("line completed after catch-up boundary must remain live")
	}
}

func TestFillSkyQuestTableShowsReadySummaryAndRequirementSources(t *testing.T) {
	quest := skyquest.Quest{
		Name: "Bard Test of Tone", QuestGiver: "Clarisa Spiritsong", Rewards: []string{"Mask of Song"},
		Requirements: []skyquest.Requirement{
			{Name: "Wind Rune Meda", Kind: "rune", Quantity: 1},
			{Name: "Light Woolen Mask", Kind: "item", Quantity: 1, Island: 3, DropsFrom: "Gorgalosk"},
		},
	}
	progress := []skyquest.QuestProgress{{
		Class: "Bard", Quest: quest, Missing: []skyquest.Requirement{quest.Requirements[1]},
	}}
	table := tview.NewTable()
	fillSkyQuestTable(table, progress, map[string]int{"Wind Rune Meda": 1}, false)
	if table.GetRowCount() != 8 {
		t.Fatalf("row count = %d, want 8", table.GetRowCount())
	}
	contents := ""
	for row := 0; row < table.GetRowCount(); row++ {
		for column := 0; column < table.GetColumnCount(); column++ {
			contents += table.GetCell(row, column).Text + "\n"
		}
	}
	for _, want := range []string{"READY TO TURN IN (0)", "Bard", "Test of Tone", "Clarisa Spiritsong — Reward: Mask of Song", "Wind Rune Meda", "Plane of Sky random drop", "Light Woolen Mask", "Island 3 — Gorgalosk"} {
		if !strings.Contains(contents, want) {
			t.Fatalf("Sky table does not contain %q:\n%s", want, contents)
		}
	}
	if strings.Contains(contents, "Bard Test of Tone") {
		t.Fatalf("Sky table repeats class in quest name:\n%s", contents)
	}
}

func TestFillSkyQuestTableReadySectionIncludesHandInDetailsAndSpacer(t *testing.T) {
	quest := skyquest.Quest{
		Name: "Necromancer Test of Power", QuestGiver: "Drakis Bloodcaster", Rewards: []string{"Cloak of Spiroc Feathers"},
		Requirements: []skyquest.Requirement{
			{Name: "Wind Rune Neza", Kind: "rune", Quantity: 1},
			{Name: "Black Silk Cape", Kind: "item", Quantity: 1, Island: 4, DropsFrom: "Keeper of Souls"},
		},
	}
	table := tview.NewTable()
	fillSkyQuestTable(table, []skyquest.QuestProgress{{Class: "Necromancer", Quest: quest, Ready: true}}, map[string]int{"Wind Rune Neza": 1, "Black Silk Cape": 1}, false)
	contents := ""
	for row := 0; row < table.GetRowCount(); row++ {
		for column := 0; column < table.GetColumnCount(); column++ {
			contents += table.GetCell(row, column).Text + "\n"
		}
	}
	for _, want := range []string{"READY TO TURN IN (1)", "Necromancer — Test of Power", "Quest giver: Drakis Bloodcaster", "Cloak of Spiroc Feathers", "Wind Rune Neza", "Plane of Sky random drop", "Black Silk Cape", "Island 4 — Keeper of Souls"} {
		if !strings.Contains(contents, want) {
			t.Fatalf("ready section does not contain %q:\n%s", want, contents)
		}
	}
	allClassesRow := -1
	for row := 0; row < table.GetRowCount(); row++ {
		if table.GetCell(row, 0).Text == "ALL CLASSES" {
			allClassesRow = row
			break
		}
	}
	if allClassesRow < 1 || table.GetCell(allClassesRow-1, 0).Text != "" {
		t.Fatalf("expected empty spacer before ALL CLASSES, row = %d", allClassesRow)
	}
}

func TestFillSkyQuestTableCanHideUnstartedQuests(t *testing.T) {
	progress := []skyquest.QuestProgress{
		{Class: "Bard", Quest: skyquest.Quest{Name: "Bard Test of Tone", Requirements: []skyquest.Requirement{{Name: "Wind Rune Meda", Quantity: 1}}}},
		{Class: "Bard", Quest: skyquest.Quest{Name: "Bard Test of Voice", Requirements: []skyquest.Requirement{{Name: "Wind Rune Kala", Quantity: 1}}}},
		{Class: "Cleric", Quest: skyquest.Quest{Name: "Cleric Test of Courage", Requirements: []skyquest.Requirement{{Name: "Wind Rune Caza", Quantity: 1}}}, Completed: true},
	}
	table := tview.NewTable()
	fillSkyQuestTable(table, progress, map[string]int{"Wind Rune Meda": 1}, true)
	contents := ""
	for row := 0; row < table.GetRowCount(); row++ {
		contents += table.GetCell(row, 0).Text + "\n"
	}
	for _, want := range []string{"Test of Tone", "Test of Courage"} {
		if !strings.Contains(contents, want) {
			t.Fatalf("filtered table does not contain %q:\n%s", want, contents)
		}
	}
	if strings.Contains(contents, "Test of Voice") {
		t.Fatalf("filtered table contains unstarted quest:\n%s", contents)
	}
}

func TestSkyQuestDisplayNameRemovesOnlyMatchingClassPrefix(t *testing.T) {
	for _, test := range []struct{ className, questName, want string }{
		{"Berserker", "Berserker Test of Blood", "Test of Blood"},
		{"Shadow Knight", "Shadow Knight Test of Night", "Test of Night"},
		{"Bard", "Songweaver's Test", "Songweaver's Test"},
	} {
		if got := skyQuestDisplayName(test.className, test.questName); got != test.want {
			t.Errorf("skyQuestDisplayName(%q, %q) = %q, want %q", test.className, test.questName, got, test.want)
		}
	}
}

func TestXPInfoTextShowsSessionXP(t *testing.T) {
	got := xpInfoText(xp.Snapshot{
		Percent:        97.085,
		LevelPercent:   97.085,
		PercentPerHour: 81.06,
		ActiveDuration: 71*time.Minute + 15*time.Second,
		Gains:          81,
	}, "")
	for _, want := range []string{"XP ~97.1%", "81.1%/h", "~00:03 to level"} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected status %q to contain %q", got, want)
		}
	}
}

func TestXPInfoTextShowsKnownProgressWithoutApproximationMarker(t *testing.T) {
	got := xpInfoText(xp.Snapshot{
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

func TestXPInfoTextShowsActiveFightFilter(t *testing.T) {
	got := xpInfoText(xp.Snapshot{}, "King Tranix")
	for _, want := range []string{"filter: King Tranix", "XP: waiting for data"} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected status %q to contain %q", got, want)
		}
	}
}

func TestReadyNoticeTextCombinesNewlyReadyQuests(t *testing.T) {
	quests := []skyquest.QuestProgress{
		{Class: "Necromancer", Quest: skyquest.Quest{Name: "Necromancer Test of Power"}},
		{Class: "Bard", Quest: skyquest.Quest{Name: "Bard Test of Tone"}},
	}
	if got, want := readyNoticeText(quests), "✓ READY: Necromancer — Test of Power (+1 more)"; got != want {
		t.Fatalf("readyNoticeText() = %q, want %q", got, want)
	}
}

func TestNewReadyQuestsExcludesAlreadyReadyQuest(t *testing.T) {
	after := []skyquest.QuestProgress{
		{Class: "Bard", Quest: skyquest.Quest{Name: "Bard Test of Tone"}},
		{Class: "Necromancer", Quest: skyquest.Quest{Name: "Necromancer Test of Power"}},
	}
	got := newReadyQuests(map[string]bool{"Bard Test of Tone": true}, after)
	if len(got) != 1 || got[0].Quest.Name != "Necromancer Test of Power" {
		t.Fatalf("unexpected newly ready quests: %#v", got)
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
	engine.ProcessLine("[Mon Jul 13 16:46:18 2026] You pierce an elemental visier for 44 points of damage.", combatTracker, xpSession, combat.DefaultIdleTimeout)
	engine.ProcessLine("[Mon Jul 13 16:46:49 2026] You gain experience! (1.239%)", combatTracker, xpSession, combat.DefaultIdleTimeout)

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
	engine.ProcessLine("[Wed Jul 15 18:53:34 2026] Zonektik begins casting Furor.", combatTracker, xpSession, combat.DefaultIdleTimeout)
	engine.ProcessLine("[Wed Jul 15 18:53:36 2026] Zonektik hit a dar ghoul knight for 33 points of magic damage by Furor.", combatTracker, xpSession, combat.DefaultIdleTimeout)

	fight, _ := combatTracker.DisplayFight()
	player := fight.Meter.Players()[0]
	if player.Breakdown["Magic"].Children["Furor"].Damage != 33 {
		t.Fatalf("expected Furor to be correlated as cast magic: %#v", player)
	}
}

func TestProcessLineClosesCombatWhenEnemiesForgetYou(t *testing.T) {
	combatTracker := combat.NewFightTracker()
	xpSession := xp.NewSession()
	engine.ProcessLine("[Mon Jul 13 14:56:40 2026] A lava guardian hits YOU for 20 points of damage.", combatTracker, xpSession, combat.DefaultIdleTimeout)
	engine.ProcessLine("[Mon Jul 13 14:56:50 2026] Your enemies have forgotten you!", combatTracker, xpSession, combat.DefaultIdleTimeout)

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
	for row := 2; row <= 3; row++ {
		for col := 0; col < 10; col++ {
			cell := table.GetCell(row, col)
			_, got, _ := cell.Style.Decompose()
			if cell.Transparent || got != combatantRowColor {
				t.Fatalf("combatant row %d column %d has background %v, want %v", row, col, got, combatantRowColor)
			}
		}
	}
}

func TestFillTableExplainsEmptyHistory(t *testing.T) {
	table := tview.NewTable()
	actions := fillTable(table, nil, make(map[string]bool), 100)

	if got := table.GetCell(1, 0).Text; got != "No fights found in the selected history." {
		t.Fatalf("unexpected empty-history message: %q", got)
	}
	if table.GetCell(1, 0).NotSelectable {
		t.Fatal("empty-history row must remain selectable so table navigation has a valid target")
	}
	if len(actions) != 0 {
		t.Fatalf("empty history exposed expandable rows: %#v", actions)
	}
}

func TestFillTableCollapsedMobShowsFightSummary(t *testing.T) {
	started := time.Date(2026, 7, 16, 21, 34, 0, 0, time.UTC)
	meter := combat.NewMeter()
	meter.Add(combat.Event{Time: started, Source: "You", Target: "a fire giant warrior", Amount: 100})
	meter.Add(combat.Event{Time: started.Add(9 * time.Second), Source: "You", Target: "a fire giant warrior", Amount: 100})
	section := combat.DisplaySection{Fight: &combat.Fight{
		Mob:   "a fire giant warrior",
		Meter: meter,
		Death: combat.Death{Victim: "a fire giant warrior", Killer: "You"},
	}}
	table := tview.NewTable()
	expanded := map[string]bool{"mob:" + sectionRowKey(section): false}

	fillTable(table, []combat.DisplaySection{section}, expanded, 106)

	if got := table.GetCell(1, 0).Text; got != "▶ a fire giant warrior (slain by You)" {
		t.Fatalf("unexpected mob summary name: %q", got)
	}
	if got := table.GetCell(1, 2).Text; got != "" {
		t.Fatalf("expected no date before DPS, got %q", got)
	}
	if got := table.GetCell(1, 3).Text; got != "20" {
		t.Fatalf("expected local-player DPS, got %q", got)
	}
	wantStarted := []string{"Start", "2026", "07-16", "21:34"}
	for offset, want := range wantStarted {
		if got := table.GetCell(1, 4+offset).Text; got != want {
			t.Fatalf("start column %d = %q, want %q", 4+offset, got, want)
		}
	}
	if got := table.GetCell(1, 9).Text; got != "00:10" {
		t.Fatalf("expected fight duration, got %q", got)
	}
}

func TestFillTableCollapsedMobWithoutLocalPlayerLeavesDPSBlank(t *testing.T) {
	started := time.Date(2026, 7, 16, 21, 34, 0, 0, time.UTC)
	meter := combat.NewMeter()
	meter.Add(combat.Event{Time: started, Source: "Alice", Target: "a fire giant warrior", Amount: 100})
	section := combat.DisplaySection{Fight: &combat.Fight{Mob: "a fire giant warrior", Meter: meter}}
	table := tview.NewTable()
	expanded := map[string]bool{"mob:" + sectionRowKey(section): false}

	fillTable(table, []combat.DisplaySection{section}, expanded, 106)

	if got := table.GetCell(1, 3).Text; got != "" {
		t.Fatalf("expected blank local-player DPS, got %q", got)
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

func TestToggleRowTreeOpensEveryDescendant(t *testing.T) {
	started := time.Date(2026, 7, 16, 12, 0, 0, 0, time.UTC)
	meter := combat.NewMeter()
	meter.Add(combat.Event{Time: started, Source: "You", Target: "King Tranix", Amount: 100, Attack: "slash"})
	meter.Add(combat.Event{Time: started.Add(time.Second), Source: "You", Target: "King Tranix", Amount: 50, Ability: "Smiting Strike"})
	meter.Add(combat.Event{Time: started.Add(2 * time.Second), Source: "King Tranix", Target: "YOU", Amount: 25, Attack: "hits"})
	section := combat.DisplaySection{Fight: &combat.Fight{Mob: "King Tranix", Meter: meter}}
	sectionKey := sectionRowKey(section)
	expanded := make(map[string]bool)

	if !toggleRowTree("mob:"+sectionKey, []combat.DisplaySection{section}, expanded) {
		t.Fatal("expected mob row to be found")
	}
	for _, key := range []string{
		"mob:" + sectionKey,
		"combatant:" + sectionKey + ":You",
		"combatant:" + sectionKey + ":You:category:Melee",
		"combatant:" + sectionKey + ":You:category:Procs",
		"combatant:" + sectionKey + ":King Tranix",
		"combatant:" + sectionKey + ":King Tranix:category:Melee",
	} {
		if !expanded[key] {
			t.Fatalf("expected descendant %q to be expanded: %#v", key, expanded)
		}
	}
}

func TestToggleRowTreeCanOpenOneCombatant(t *testing.T) {
	started := time.Date(2026, 7, 16, 12, 0, 0, 0, time.UTC)
	meter := combat.NewMeter()
	meter.Add(combat.Event{Time: started, Source: "You", Target: "mob", Amount: 10, Attack: "slash"})
	section := combat.DisplaySection{Fight: &combat.Fight{Mob: "mob", Meter: meter}}
	sectionKey := sectionRowKey(section)
	expanded := make(map[string]bool)
	combatantKey := "combatant:" + sectionKey + ":You"

	if !toggleRowTree(combatantKey, []combat.DisplaySection{section}, expanded) || !expanded[combatantKey+":category:Melee"] {
		t.Fatalf("expected combatant categories to expand: %#v", expanded)
	}
	if expanded["mob:"+sectionKey] {
		t.Fatal("expanding a combatant must not change its parent mob")
	}
}

func TestToggleRowTreeClosesEveryDescendantWhenAnythingIsOpen(t *testing.T) {
	started := time.Date(2026, 7, 16, 12, 0, 0, 0, time.UTC)
	meter := combat.NewMeter()
	meter.Add(combat.Event{Time: started, Source: "You", Target: "mob", Amount: 10, Attack: "slash"})
	meter.Add(combat.Event{Time: started.Add(time.Second), Source: "You", Target: "mob", Amount: 20, Ability: "Proc"})
	section := combat.DisplaySection{Fight: &combat.Fight{Mob: "mob", Meter: meter}}
	sectionKey := sectionRowKey(section)
	mobKey := "mob:" + sectionKey
	expanded := map[string]bool{mobKey: true}

	if !toggleRowTree(mobKey, []combat.DisplaySection{section}, expanded) {
		t.Fatal("expected mob tree to be found")
	}
	for _, key := range rowTreeKeys(mobKey, []combat.DisplaySection{section}) {
		if expanded[key] {
			t.Fatalf("expected entire subtree to close, but %q remains open", key)
		}
	}

	if !toggleRowTree(mobKey, []combat.DisplaySection{section}, expanded) {
		t.Fatal("expected mob tree to reopen")
	}
	for _, key := range rowTreeKeys(mobKey, []combat.DisplaySection{section}) {
		if !expanded[key] {
			t.Fatalf("expected entire subtree to reopen, but %q remains closed", key)
		}
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
