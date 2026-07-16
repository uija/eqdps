package skyquest

import (
	"testing"

	"github.com/uija/eqdps/internal/eqlog"
)

func TestTrackerOnlyAddsRetainedKnownLootInPlaneOfSky(t *testing.T) {
	tracker := NewTracker(testDatabase())
	processTestLine(t, tracker, "[Thu Jul 16 10:40:00 2026] You have entered East Freeport.")
	processTestLine(t, tracker, "[Thu Jul 16 10:40:01 2026] --You have looted a Wind Rune Caza from Protector of Sky's corpse.--")
	if got := tracker.Owned("Wind Rune Caza"); got != 0 {
		t.Fatalf("outside-Sky rune count = %d, want 0", got)
	}

	processTestLine(t, tracker, "[Thu Jul 16 10:40:02 2026] You have entered The Plane of Sky.")
	processTestLine(t, tracker, "[Thu Jul 16 10:40:03 2026] You looted a Wind Rune Caza from Protector of Sky's corpse and sold it for free.")
	processTestLine(t, tracker, "[Thu Jul 16 10:40:04 2026] You looted a Wind Rune Caza from Protector of Sky's corpse to create a Wind Rune Caza +1")
	if got := tracker.Owned("Wind Rune Caza"); got != 0 {
		t.Fatalf("unretained rune count = %d, want 0", got)
	}

	processTestLine(t, tracker, "[Thu Jul 16 10:40:05 2026] --You have looted a Wind Rune Caza from Protector of Sky's corpse.--")
	processTestLine(t, tracker, "[Thu Jul 16 10:40:05 2026] You looted a Wind Rune Caza from Protector of Sky's corpse and stored it in your Dragon Hoard")
	processTestLine(t, tracker, "[Thu Jul 16 10:40:06 2026] --You have looted a Not A Quest Item from Protector of Sky's corpse.--")
	if got := tracker.Owned("Wind Rune Caza"); got != 2 {
		t.Fatalf("retained and directly stored rune count = %d, want 2", got)
	}
	if got := tracker.Owned("Not A Quest Item"); got != 0 {
		t.Fatalf("unknown item count = %d, want 0", got)
	}
}

func TestTrackerNormalizesUpgradedLootInInstancedPlaneOfSky(t *testing.T) {
	tracker := NewTracker(testDatabase())
	processTestLine(t, tracker, "[Thu Jul 16 10:40:00 2026] You have entered The Plane of Sky 2 (Adaptive).")
	processTestLine(t, tracker, "[Thu Jul 16 10:40:01 2026] --You have looted a Light Woolen Mask +3 from Gorgalosk's corpse.--")
	if got := tracker.Owned("Light Woolen Mask"); got != 1 {
		t.Fatalf("normalized upgraded loot count = %d, want 1", got)
	}
}

func TestTrackerRemovesDestroyedItemInAnyZone(t *testing.T) {
	tracker := NewTracker(testDatabase())
	processTestLine(t, tracker, "[Thu Jul 16 10:40:00 2026] You have entered The Plane of Sky.")
	processTestLine(t, tracker, "[Thu Jul 16 10:40:01 2026] --You have looted 2 Wind Rune Caza from Protector of Sky's corpse.--")
	processTestLine(t, tracker, "[Thu Jul 16 10:40:02 2026] You have entered East Freeport.")
	processTestLine(t, tracker, "[Thu Jul 16 10:40:03 2026] You successfully destroyed 1 Wind Rune Caza.")
	if got := tracker.Owned("Wind Rune Caza"); got != 1 {
		t.Fatalf("rune count after destruction = %d, want 1", got)
	}
	processTestLine(t, tracker, "[Thu Jul 16 10:40:04 2026] You successfully destroyed 2 Wind Rune Caza.")
	if got := tracker.Owned("Wind Rune Caza"); got != 0 {
		t.Fatalf("rune count after over-removal = %d, want 0", got)
	}
}

func TestTrackerReportsReadyQuests(t *testing.T) {
	tracker := NewTracker(testDatabase())
	processTestLine(t, tracker, "[Thu Jul 16 10:40:00 2026] You have entered The Plane of Sky.")
	processTestLine(t, tracker, "[Thu Jul 16 10:40:01 2026] --You have looted a Wind Rune Caza from Protector of Sky's corpse.--")
	if got := len(tracker.ReadyQuests()); got != 0 {
		t.Fatalf("ready quests with missing item = %d, want 0", got)
	}
	processTestLine(t, tracker, "[Thu Jul 16 10:40:02 2026] --You have looted a Light Woolen Mask from Gorgalosk's corpse.--")
	ready := tracker.ReadyQuests()
	if len(ready) != 1 || ready[0].Class != "Bard" || ready[0].Quest.Name != "Bard Test of Tone" {
		t.Fatalf("unexpected ready quests: %#v", ready)
	}
}

func TestTrackerCompletesQuestFromExactOfferedItemsAndGiver(t *testing.T) {
	tracker := NewTracker(testDatabase())
	for _, line := range []string{
		"[Thu Jul 16 10:40:00 2026] You have entered The Plane of Sky.",
		"[Thu Jul 16 10:40:01 2026] --You have looted a Wind Rune Caza from Protector of Sky's corpse.--",
		"[Thu Jul 16 10:40:02 2026] --You have looted a Light Woolen Mask from Gorgalosk's corpse.--",
		"[Thu Jul 16 10:40:03 2026] You offered 1 Light Woolen Mask to Clarisa Spiritsong.",
		"[Thu Jul 16 10:40:04 2026] You offered 1 Wind Rune Caza to Clarisa Spiritsong.",
	} {
		processTestLine(t, tracker, line)
	}
	if !tracker.QuestProgress()[0].Ready {
		t.Fatal("offers must not complete or consume the quest before trade confirmation")
	}
	processTestLine(t, tracker, "[Thu Jul 16 10:40:05 2026] You complete the trade with Clarisa Spiritsong.")
	progress := tracker.QuestProgress()[0]
	if !progress.Completed || progress.Ready || tracker.Owned("Wind Rune Caza") != 0 || tracker.Owned("Light Woolen Mask") != 0 {
		t.Fatalf("unexpected completed quest state: %#v, inventory %#v", progress, tracker.Inventory())
	}
}

func TestTrackerCompletesKartharTurnInInAdaptiveSkyWithUpgradedItem(t *testing.T) {
	database := Database{SchemaVersion: 1, Classes: []Class{{
		Name: "Monk", Quests: []Quest{{
			Name: "Monk Test of Fists", QuestGiver: "Holwin",
			Requirements: []Requirement{{Name: "Wind Rune Neza", Quantity: 1}, {Name: "Brass Knuckles", Quantity: 1}, {Name: "Nebulous Sapphire", Quantity: 1}},
		}},
	}}}
	tracker := NewTracker(database)
	for _, line := range []string{
		"[Mon Jul 06 18:31:55 2026] You have entered The Plane of Sky 2 (Adaptive).",
		"[Mon Jul 06 19:51:53 2026] You offered 1 Wind Rune Neza to Holwin.",
		"[Mon Jul 06 19:51:56 2026] You offered 1 Brass Knuckles +1 to Holwin.",
		"[Mon Jul 06 19:51:58 2026] You offered 1 Nebulous Sapphire to Holwin.",
		"[Mon Jul 06 19:51:59 2026] You complete the trade with Holwin.",
	} {
		processTestLine(t, tracker, line)
	}
	if progress := tracker.QuestProgress()[0]; !progress.Completed {
		t.Fatalf("Karthar turn-in was not completed: %#v", progress)
	}
}

func TestEmbeddedDatabaseMatchesKartharSkyTurnIns(t *testing.T) {
	database, err := LoadDatabase()
	if err != nil {
		t.Fatal(err)
	}
	tracker := NewTracker(database)
	lines := []string{
		"[Mon Jul 06 18:31:55 2026] You have entered The Plane of Sky 2 (Adaptive).",
		"[Mon Jul 06 18:54:45 2026] You offered 1 Tear of Quellious to Holwin.",
		"[Mon Jul 06 18:54:50 2026] You offered 1 Wind Rune Lena to Holwin.",
		"[Mon Jul 06 18:54:51 2026] You complete the trade with Holwin.",
		"[Mon Jul 06 19:51:53 2026] You offered 1 Wind Rune Neza to Holwin.",
		"[Mon Jul 06 19:51:56 2026] You offered 1 Brass Knuckles +1 to Holwin.",
		"[Mon Jul 06 19:51:58 2026] You offered 1 Nebulous Sapphire to Holwin.",
		"[Mon Jul 06 19:51:59 2026] You complete the trade with Holwin.",
		"[Mon Jul 06 19:54:40 2026] You offered 1 Efreeti Battle Axe +2 to Torgon Blademaster.",
		"[Mon Jul 06 19:54:42 2026] You offered 1 Ethereal Emerald to Torgon Blademaster.",
		"[Mon Jul 06 19:54:44 2026] You offered 1 Wind Rune Dena to Torgon Blademaster.",
		"[Mon Jul 06 19:54:45 2026] You complete the trade with Torgon Blademaster.",
	}
	for _, line := range lines {
		processTestLine(t, tracker, line)
	}
	completed := tracker.Completed()
	for _, quest := range []string{"Monk Test of Tranquility", "Monk Test of Fists", "Warrior Test of Bash"} {
		if !completed[quest] {
			t.Errorf("Karthar turn-in %q was not completed: %#v", quest, completed)
		}
	}
}

func TestTrackerClearsPendingItemsWhenTradeIsCancelled(t *testing.T) {
	tracker := NewTracker(testDatabase())
	for _, line := range []string{
		"[Thu Jul 16 10:40:00 2026] You have entered The Plane of Sky 1 (Awakened).",
		"[Thu Jul 16 10:40:01 2026] You offered 1 Wind Rune Caza to Clarisa Spiritsong.",
		"[Thu Jul 16 10:40:02 2026] You have cancelled the trade.",
		"[Thu Jul 16 10:40:03 2026] You offered 1 Light Woolen Mask to Clarisa Spiritsong.",
		"[Thu Jul 16 10:40:04 2026] You complete the trade with Clarisa Spiritsong.",
	} {
		processTestLine(t, tracker, line)
	}
	if tracker.QuestProgress()[0].Completed {
		t.Fatal("cancelled offer leaked into a later completed trade")
	}
}

func processTestLine(t *testing.T, tracker *Tracker, line string) {
	t.Helper()
	record, ok := eqlog.ParseRecord(line)
	if !ok || record.Kind == eqlog.RecordUnknown {
		t.Fatalf("line did not produce a recognized record: %q", line)
	}
	tracker.ProcessRecord(record)
}

func testDatabase() Database {
	return Database{SchemaVersion: 1, Classes: []Class{{
		Name: "Bard",
		Quests: []Quest{{
			Name:       "Bard Test of Tone",
			QuestGiver: "Clarisa Spiritsong",
			Requirements: []Requirement{
				{Name: "Wind Rune Caza", Kind: "rune", Quantity: 1},
				{Name: "Light Woolen Mask", Kind: "item", Quantity: 1},
			},
		}},
	}}}
}
