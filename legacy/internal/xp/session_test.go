package xp

import (
	"testing"
	"time"
)

func TestSessionCapsLongCombatGapsAtOneMinute(t *testing.T) {
	started := time.Date(2026, 7, 13, 12, 0, 0, 0, time.UTC)
	session := NewSession()
	session.AddCombat(started)
	session.AddGain(started.Add(10*time.Second), 2)
	session.AddCombat(started.Add(10 * time.Minute))
	session.AddGain(started.Add(10*time.Minute), 3)
	session.Observe(started.Add(10*time.Minute), started.Add(10*time.Minute))

	got := session.SnapshotAtLatestLog()
	if got.ActiveDuration != time.Minute {
		t.Fatalf("expected long gap to count as one minute, got %s", got.ActiveDuration)
	}
	if got.Percent != 5 || got.Gains != 2 {
		t.Fatalf("unexpected XP totals: %#v", got)
	}
	if got.PercentPerHour != 300 {
		t.Fatalf("expected 300%%/h, got %.2f", got.PercentPerHour)
	}
}

func TestSessionCountsShortCombatGapsFully(t *testing.T) {
	started := time.Date(2026, 7, 13, 12, 0, 0, 0, time.UTC)
	session := NewSession()
	session.AddCombat(started)
	session.AddCombat(started.Add(45 * time.Second))
	session.AddGain(started.Add(45*time.Second), 1)
	session.Observe(started.Add(45*time.Second), started.Add(45*time.Second))

	got := session.SnapshotAtLatestLog()
	if got.ActiveDuration != 45*time.Second {
		t.Fatalf("expected short gap in full, got %s", got.ActiveDuration)
	}
}

func TestSessionLiveIdleStopsGrowingAfterOneMinute(t *testing.T) {
	started := time.Date(2026, 7, 13, 12, 0, 0, 0, time.UTC)
	observed := time.Date(2026, 7, 13, 14, 0, 0, 0, time.UTC)
	session := NewSession()
	session.AddCombat(started)
	session.AddGain(started, 1)
	session.Observe(started, observed)

	got := session.SnapshotLive(observed.Add(5 * time.Minute))
	if got.ActiveDuration != time.Minute {
		t.Fatalf("expected live idle to stop at one minute, got %s", got.ActiveDuration)
	}
}

func TestSessionLevelUpResetsProgressButKeepsSessionRateTotal(t *testing.T) {
	started := time.Date(2026, 7, 13, 12, 0, 0, 0, time.UTC)
	session := NewSession()
	session.AddCombat(started)
	session.AddGain(started.Add(time.Minute), 99)
	session.AddLevelUp(started.Add(2 * time.Minute))
	session.AddGain(started.Add(2*time.Minute), 1.2)
	session.AddGain(started.Add(3*time.Minute), 0.8)
	session.Observe(started.Add(3*time.Minute), started.Add(3*time.Minute))

	got := session.SnapshotAtLatestLog()
	if got.Percent != 101 {
		t.Fatalf("expected complete session total, got %.3f", got.Percent)
	}
	if got.LevelPercent != 0.8 {
		t.Fatalf("expected only post-level-up progress, got %.3f", got.LevelPercent)
	}
	if !got.ProgressKnown {
		t.Fatal("expected progress to be known after observing a level-up")
	}
}

func TestSessionProgressIsUnknownWithoutObservedLevelUp(t *testing.T) {
	started := time.Date(2026, 7, 13, 12, 0, 0, 0, time.UTC)
	session := NewSession()
	session.AddGain(started, 2)
	session.Observe(started, started)

	if got := session.SnapshotAtLatestLog(); got.ProgressKnown {
		t.Fatal("expected progress to remain approximate when starting mid-level")
	}
}

func TestSnapshotTimeToLevelRoundsUpToMinute(t *testing.T) {
	snapshot := Snapshot{LevelPercent: 97.085, PercentPerHour: 81.06}
	got, ok := snapshot.TimeToLevel()
	if !ok {
		t.Fatal("expected ETA")
	}
	if got != 3*time.Minute {
		t.Fatalf("expected three-minute ETA, got %s", got)
	}
}

func TestSnapshotTimeToLevelNeedsRate(t *testing.T) {
	if _, ok := (Snapshot{LevelPercent: 50}).TimeToLevel(); ok {
		t.Fatal("expected no ETA without an XP rate")
	}
}
