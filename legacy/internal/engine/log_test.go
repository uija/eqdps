package engine

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/uija/eqdps/legacy/internal/combat"
	"github.com/uija/eqdps/legacy/internal/event"
	"github.com/uija/eqdps/legacy/internal/xp"
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
