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
			Name: "Bard Test of Tone",
			Requirements: []Requirement{
				{Name: "Wind Rune Caza", Kind: "rune", Quantity: 1},
				{Name: "Light Woolen Mask", Kind: "item", Quantity: 1},
			},
		}},
	}}}
}
