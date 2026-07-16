package skyquest

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
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

func TestLoadPersistentTrackerDoesNotCatchUpUntilSync(t *testing.T) {
	directory := t.TempDir()
	logPath := filepath.Join(directory, "eqlog_Wyrmberg_rivervale.txt")
	initial := "[Thu Jul 16 10:40:00 2026] You have entered The Plane of Sky.\n"
	if err := os.WriteFile(logPath, []byte(initial), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := OpenPersistentTracker(logPath, testDatabase()); err != nil {
		t.Fatal(err)
	}
	appended := "[Thu Jul 16 10:40:01 2026] --You have looted a Wind Rune Caza from Protector of Sky's corpse.--\n"
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

	loaded, err := LoadPersistentTracker(logPath, testDatabase())
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Offset() != int64(len(initial)) || loaded.Inventory()["Wind Rune Caza"] != 0 {
		t.Fatalf("load unexpectedly caught up: offset %d, inventory %#v", loaded.Offset(), loaded.Inventory())
	}
	if err := loaded.SyncLogWithProgress(logPath, int64(len(initial)+len(appended)), nil, nil); err != nil {
		t.Fatal(err)
	}
	if loaded.Offset() != int64(len(initial)+len(appended)) || loaded.Inventory()["Wind Rune Caza"] != 1 {
		t.Fatalf("explicit sync did not catch up: offset %d, inventory %#v", loaded.Offset(), loaded.Inventory())
	}
}

func TestCancelledCatchupLeavesSavedCheckpointUnchanged(t *testing.T) {
	directory := t.TempDir()
	logPath := filepath.Join(directory, "eqlog_Wyrmberg_rivervale.txt")
	initial := "[Thu Jul 16 10:40:00 2026] You have entered The Plane of Sky.\n"
	if err := os.WriteFile(logPath, []byte(initial), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := OpenPersistentTracker(logPath, testDatabase()); err != nil {
		t.Fatal(err)
	}
	appended := strings.Repeat("[Thu Jul 16 10:40:01 2026] ignored catch-up line\n", 6001)
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
	loaded, err := LoadPersistentTracker(logPath, testDatabase())
	if err != nil {
		t.Fatal(err)
	}
	cancel := make(chan struct{})
	err = loaded.SyncLogWithProgress(logPath, int64(len(initial)+len(appended)), func(progress ScanProgress) {
		if progress.Lines >= 5000 {
			select {
			case <-cancel:
			default:
				close(cancel)
			}
		}
	}, cancel)
	if !errors.Is(err, ErrScanCancelled) {
		t.Fatalf("catch-up error = %v, want ErrScanCancelled", err)
	}
	reloaded, err := LoadPersistentTracker(logPath, testDatabase())
	if err != nil {
		t.Fatal(err)
	}
	if reloaded.Offset() != int64(len(initial)) {
		t.Fatalf("cancelled catch-up saved offset %d, want %d", reloaded.Offset(), len(initial))
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

func TestPersistentTrackerSavesCompletedQuest(t *testing.T) {
	directory := t.TempDir()
	logPath := filepath.Join(directory, "eqlog_Wyrmberg_rivervale.txt")
	content := "[Thu Jul 16 13:08:00 2026] You have entered The Plane of Sky.\n" +
		"[Thu Jul 16 13:08:01 2026] --You have looted a Wind Rune Caza from Protector of Sky's corpse.--\n" +
		"[Thu Jul 16 13:08:02 2026] --You have looted a Light Woolen Mask from Gorgalosk's corpse.--\n" +
		"[Thu Jul 16 13:08:59 2026] You offered 1 Light Woolen Mask to Cilin Spellsinger.\n" +
		"[Thu Jul 16 13:09:16 2026] You offered 1 Wind Rune Caza to Cilin Spellsinger.\n" +
		"[Thu Jul 16 13:09:20 2026] You complete the trade with Cilin Spellsinger.\n"
	if err := os.WriteFile(logPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	tracker, err := OpenPersistentTracker(logPath, testDatabase())
	if err != nil {
		t.Fatal(err)
	}
	progress := tracker.QuestProgress()[0]
	if !progress.Completed || len(tracker.Inventory()) != 0 {
		t.Fatalf("completed quest was not restored from log: %#v, inventory %#v", progress, tracker.Inventory())
	}
	data, err := os.ReadFile(filepath.Join(directory, "Wyrmberg_rivervale_PoS.json"))
	if err != nil {
		t.Fatal(err)
	}
	var state CharacterState
	if err := json.Unmarshal(data, &state); err != nil {
		t.Fatal(err)
	}
	if !state.Completed["Bard Test of Tone"] {
		t.Fatalf("completed quest missing from state: %#v", state.Completed)
	}
}

func TestCancelledInitialScanCreatesNoStateFile(t *testing.T) {
	directory := t.TempDir()
	logPath := filepath.Join(directory, "eqlog_Wyrmberg_rivervale.txt")
	if err := os.WriteFile(logPath, []byte("[Thu Jul 16 10:40:00 2026] You have entered The Plane of Sky.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cancel := make(chan struct{})
	close(cancel)
	if _, err := InitializePersistentTracker(logPath, testDatabase(), 0, nil, cancel); !errors.Is(err, ErrScanCancelled) {
		t.Fatalf("error = %v, want ErrScanCancelled", err)
	}
	exists, err := StateExists(logPath)
	if err != nil {
		t.Fatal(err)
	}
	if exists {
		t.Fatal("cancelled scan created a state file")
	}
}

func TestInitialScanReportsProgress(t *testing.T) {
	directory := t.TempDir()
	logPath := filepath.Join(directory, "eqlog_Wyrmberg_rivervale.txt")
	content := "[Thu Jul 16 10:40:00 2026] You have entered The Plane of Sky.\n"
	if err := os.WriteFile(logPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	var latest ScanProgress
	if _, err := InitializePersistentTracker(logPath, testDatabase(), int64(len(content)), func(progress ScanProgress) {
		latest = progress
	}, nil); err != nil {
		t.Fatal(err)
	}
	if latest.Bytes != int64(len(content)) || latest.Total != int64(len(content)) || latest.Lines != 1 {
		t.Fatalf("unexpected progress: %#v", latest)
	}
}
