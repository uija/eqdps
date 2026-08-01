package engine

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/uija/eqdps/internal/combat"
	"github.com/uija/eqdps/internal/event"
	"github.com/uija/eqdps/internal/xp"
)

func TestProcessLineUpdatesCombatAndXP(t *testing.T) {
	tracker := combat.NewFightTracker()
	session := xp.NewSession()
	ProcessLine("[Mon Jul 13 16:46:18 2026] You pierce an elemental visier for 44 points of damage.", tracker, session, combat.DefaultIdleTimeout)
	ProcessLine("[Mon Jul 13 16:46:49 2026] You gain experience! (1.239%)", tracker, session, combat.DefaultIdleTimeout)

	if len(tracker.DisplaySections()) != 1 {
		t.Fatalf("expected one combat section, got %d", len(tracker.DisplaySections()))
	}
	if got := session.SnapshotAtLatestLog().Percent; got != 1.239 {
		t.Fatalf("XP percent = %v, want 1.239", got)
	}
}

func TestFindReplayLandmarksReturnsLatestEvents(t *testing.T) {
	log := "[Thu Jul 02 05:19:04 2026] You have gained a level! Welcome to level 43!\n" +
		"[Thu Jul 09 08:55:08 2026] You have entered The Plane of Sky.\n" +
		"[Mon Jul 13 15:34:31 2026] You have gained a level! Welcome to level 44!\n" +
		"[Mon Jul 13 16:00:00 2026] You have entered North Ro.\n"
	path := filepath.Join(t.TempDir(), "eqlog_Test_server.txt")
	if err := os.WriteFile(path, []byte(log), 0o600); err != nil {
		t.Fatal(err)
	}

	landmarks, err := FindReplayLandmarks(path, int64(len(log)))
	if err != nil {
		t.Fatal(err)
	}
	if got := landmarks.LastLevelUp.Format("2006-01-02 15:04:05"); got != "2026-07-13 15:34:31" {
		t.Fatalf("last level up = %s", got)
	}
	if got := landmarks.LastZoneChange.Format("2006-01-02 15:04:05"); got != "2026-07-13 16:00:00" {
		t.Fatalf("last zone change = %s", got)
	}
}

func TestReplayFromLandmarkRebuildsXPFromSelectedPoint(t *testing.T) {
	log := "[Mon Jul 13 15:34:31 2026] You have gained a level! Welcome to level 44!\n" +
		"[Mon Jul 13 15:40:00 2026] You gain experience! (1.000%)\n" +
		"[Mon Jul 13 16:00:00 2026] You have entered North Ro.\n" +
		"[Mon Jul 13 16:05:00 2026] You gain experience! (2.000%)\n"
	path := filepath.Join(t.TempDir(), "eqlog_Test_server.txt")
	if err := os.WriteFile(path, []byte(log), 0o600); err != nil {
		t.Fatal(err)
	}
	landmarks, err := FindReplayLandmarks(path, int64(len(log)))
	if err != nil {
		t.Fatal(err)
	}

	_, session, err := Replay(path, combat.DefaultIdleTimeout, 0, landmarks.LastZoneChange, combat.DefaultFightHistory)
	if err != nil {
		t.Fatal(err)
	}
	session.AddPriorLevelProgress(landmarks.LastZoneLevelPercent, landmarks.LastZoneProgressKnown)
	snapshot := session.SnapshotAtLatestLog()
	if snapshot.Percent != 2 {
		t.Fatalf("XP since zoning = %v, want 2", snapshot.Percent)
	}
	if snapshot.LevelPercent != 3 || !snapshot.ProgressKnown {
		t.Fatalf("level progress = %.1f known=%v, want 3.0 and known", snapshot.LevelPercent, snapshot.ProgressKnown)
	}
}

func TestReplayReportsProgressAndSupportsCancellation(t *testing.T) {
	log := "[Mon Jul 13 16:46:18 2026] You pierce an elemental visier for 44 points of damage.\n"
	path := filepath.Join(t.TempDir(), "eqlog_Test_server.txt")
	if err := os.WriteFile(path, []byte(log), 0o600); err != nil {
		t.Fatal(err)
	}

	var progress ReplayProgress
	tracker, _, err := ReplayWithProgress(path, combat.DefaultIdleTimeout, -time.Nanosecond, time.Time{}, 0, int64(len(log)), func(update ReplayProgress) {
		progress = update
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(tracker.DisplaySections()) != 1 || progress.Bytes != int64(len(log)) || progress.Lines != 1 {
		t.Fatalf("unexpected replay result: sections=%d progress=%+v", len(tracker.DisplaySections()), progress)
	}

	cancel := make(chan struct{})
	close(cancel)
	if _, _, err := ReplayWithProgress(path, combat.DefaultIdleTimeout, -time.Nanosecond, time.Time{}, 0, int64(len(log)), nil, cancel); !errors.Is(err, ErrReplayCancelled) {
		t.Fatalf("expected cancellation, got %v", err)
	}
}

func TestFollowReportsCompleteLinesAndOffsets(t *testing.T) {
	line := "[Mon Jul 13 16:46:49 2026] You gain experience! (1.239%)\n"
	path := filepath.Join(t.TempDir(), "eqlog_Test_server.txt")
	if err := os.WriteFile(path, []byte(line), 0o600); err != nil {
		t.Fatal(err)
	}

	done := make(chan struct{})
	var gotLine string
	var gotOffset int64
	err := Follow(path, 0, done, func(update string, endOffset int64) {
		gotLine = update
		gotOffset = endOffset
		close(done)
	})
	if err != nil {
		t.Fatal(err)
	}
	if gotLine != line || gotOffset != int64(len(line)) {
		t.Fatalf("line=%q offset=%d, want %q offset=%d", gotLine, gotOffset, line, len(line))
	}
}

func TestFollowWithPollInvokesIdleCallbackAtEOF(t *testing.T) {
	path := filepath.Join(t.TempDir(), "eqlog_Test_server.txt")
	if err := os.WriteFile(path, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	polled := false
	err := FollowWithPoll(path, 0, done, func(string, int64) {}, func(time.Time) {
		polled = true
		close(done)
	})
	if err != nil {
		t.Fatal(err)
	}
	if !polled {
		t.Fatal("expected EOF poll callback")
	}
}

func TestReplayDoesNotDispatchLiveLineEvents(t *testing.T) {
	line := "[Mon Jul 13 16:46:49 2026] Your speed returns to normal.\n"
	path := filepath.Join(t.TempDir(), "eqlog_Test_server.txt")
	if err := os.WriteFile(path, []byte(line), 0o600); err != nil {
		t.Fatal(err)
	}
	dispatcher, err := event.NewDispatcher([]event.Event{{
		ID: "fade", Title: "Fade", Active: true, TriggerType: event.TriggerText,
		Pattern: "speed returns", Notification: "Buff faded",
	}}, 1, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := Replay(path, combat.DefaultIdleTimeout, -time.Nanosecond, time.Time{}, 0); err != nil {
		t.Fatal(err)
	}
	select {
	case delivery := <-dispatcher.Notifications():
		t.Fatalf("replay dispatched event %#v", delivery)
	default:
	}

	DispatchLiveLine(line, dispatcher)
	select {
	case delivery := <-dispatcher.Notifications():
		if delivery.Event.ID != "fade" {
			t.Fatalf("unexpected delivery %#v", delivery)
		}
	default:
		t.Fatal("live line did not dispatch event")
	}
}
