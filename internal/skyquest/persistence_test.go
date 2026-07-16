package skyquest

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestCharacterIdentity(t *testing.T) {
	character, server, err := CharacterIdentity(`/logs/eqlog_Wyrmberg_rivervale.txt`)
	if err != nil || character != "Wyrmberg" || server != "rivervale" {
		t.Fatalf("identity = %q, %q, %v", character, server, err)
	}
	if _, _, err := CharacterIdentity(`/logs/not-an-eq-log.txt`); err == nil {
		t.Fatal("expected invalid log name error")
	}
}

func TestPersistentTrackerScansOnceAndResumesFromExactByteOffset(t *testing.T) {
	directory := t.TempDir()
	logPath := filepath.Join(directory, "eqlog_Wyrmberg_rivervale.txt")
	initial := "[Thu Jul 16 10:40:00 2026] You have entered The Plane of Sky.\r\n" +
		"[Thu Jul 16 10:40:01 2026] --You have looted 2 Wind Rune Caza from Protector of Sky's corpse.--\r\n" +
		"[Thu Jul 16 10:40:02 2026] You looted a Wind Rune Caza from Protector of Sky's corpse and sold it for free.\r\n"
	if err := os.WriteFile(logPath, []byte(initial), 0o644); err != nil {
		t.Fatal(err)
	}

	first, err := OpenPersistentTracker(logPath, testDatabase())
	if err != nil {
		t.Fatal(err)
	}
	if got := first.Inventory()["Wind Rune Caza"]; got != 2 {
		t.Fatalf("first scan quantity = %d, want 2", got)
	}
	if got := first.Offset(); got != int64(len(initial)) {
		t.Fatalf("checkpoint offset = %d, want %d", got, len(initial))
	}

	second, err := OpenPersistentTracker(logPath, testDatabase())
	if err != nil {
		t.Fatal(err)
	}
	if got := second.Inventory()["Wind Rune Caza"]; got != 2 {
		t.Fatalf("reopened quantity = %d, want 2; initial loot was counted again", got)
	}

	appended := "[Thu Jul 16 10:40:03 2026] You have entered East Freeport.\r\n" +
		"[Thu Jul 16 10:40:04 2026] You successfully destroyed 1 Wind Rune Caza.\r\n"
	file, err := os.OpenFile(logPath, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := file.WriteString(appended); err != nil {
		file.Close()
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}

	third, err := OpenPersistentTracker(logPath, testDatabase())
	if err != nil {
		t.Fatal(err)
	}
	if got := third.Inventory()["Wind Rune Caza"]; got != 1 {
		t.Fatalf("quantity after resumed destruction = %d, want 1", got)
	}
	if got := third.Offset(); got != int64(len(initial)+len(appended)) {
		t.Fatalf("resumed offset = %d, want %d", got, len(initial)+len(appended))
	}

	stateData, err := os.ReadFile(filepath.Join(directory, "Wyrmberg_rivervale_PoS.json"))
	if err != nil {
		t.Fatal(err)
	}
	var state CharacterState
	if err := json.Unmarshal(stateData, &state); err != nil {
		t.Fatal(err)
	}
	if state.Checkpoint.LastZone != "East Freeport" || state.Holdings["Wind Rune Caza"] != 1 {
		t.Fatalf("unexpected persisted state: %#v", state)
	}
}

func TestPersistentTrackerRejectsTruncatedOrReplacedLog(t *testing.T) {
	directory := t.TempDir()
	logPath := filepath.Join(directory, "eqlog_Wyrmberg_rivervale.txt")
	content := "[Thu Jul 16 10:40:00 2026] You have entered The Plane of Sky.\n" +
		"[Thu Jul 16 10:40:01 2026] --You have looted a Wind Rune Caza from Protector of Sky's corpse.--\n"
	if err := os.WriteFile(logPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := OpenPersistentTracker(logPath, testDatabase()); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(logPath, []byte("replacement\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := OpenPersistentTracker(logPath, testDatabase()); !errors.Is(err, ErrInvalidCheckpoint) {
		t.Fatalf("error = %v, want ErrInvalidCheckpoint", err)
	}
}

func TestPersistentTrackerLeavesPartialFinalLineForNextRead(t *testing.T) {
	directory := t.TempDir()
	logPath := filepath.Join(directory, "eqlog_Wyrmberg_rivervale.txt")
	complete := "[Thu Jul 16 10:40:00 2026] You have entered The Plane of Sky.\n"
	partial := "[Thu Jul 16 10:40:01 2026] --You have looted a Wind Rune Caza"
	if err := os.WriteFile(logPath, []byte(complete+partial), 0o644); err != nil {
		t.Fatal(err)
	}
	tracker, err := OpenPersistentTracker(logPath, testDatabase())
	if err != nil {
		t.Fatal(err)
	}
	if got := tracker.Offset(); got != int64(len(complete)) {
		t.Fatalf("offset = %d, want complete-line boundary %d", got, len(complete))
	}
	if got := tracker.Inventory()["Wind Rune Caza"]; got != 0 {
		t.Fatalf("partial line changed inventory: %d", got)
	}
}
