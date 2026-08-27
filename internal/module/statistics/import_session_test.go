package statistics

import (
	"database/sql"
	"testing"
	"time"

	"github.com/uija/eqdps/internal/data"
)

func TestLoginClosesVisitAtConfirmedCampTimeAcrossBackwardClockJump(t *testing.T) {
	m, db := newSessionTestModule(t)

	enteredAt := sessionTestTime(t, "2026-08-19 20:47:48")
	campStartedAt := sessionTestTime(t, "2026-08-19 20:48:29")
	adventureAt := sessionTestTime(t, "2026-08-19 17:08:30")
	loginAt := sessionTestTime(t, "2026-08-19 17:08:35")

	sessionTestRow(t, m, enteredAt, "You have entered North Kaladim.", data.LogRowEventTypeZoneChange, "", "North Kaladim")
	sessionTestRow(t, m, campStartedAt, campStartMessage, data.LogRowEventTypeUnknown)
	sessionTestRow(t, m, adventureAt, loginAdventureMessage, data.LogRowEventTypeUnknown)
	sessionTestRow(t, m, loginAt, loginMessage, data.LogRowEventTypeUnknown)

	if err := m.activeImport.Commit(0, loginAt); err != nil {
		t.Fatalf("commit import: %v", err)
	}

	var leftAt time.Time
	if err := db.QueryRow(`SELECT left_at FROM zone_visits LIMIT 1`).Scan(&leftAt); err != nil {
		t.Fatalf("read closed visit: %v", err)
	}
	want := campStartedAt.Add(campCompletionTime)
	if !leftAt.Equal(want) {
		t.Fatalf("visit closed at %v, want %v", leftAt, want)
	}
}

func TestLoginNeverLeavesVisitOpenWhenClockMovesBackwards(t *testing.T) {
	m, db := newSessionTestModule(t)

	enteredAt := sessionTestTime(t, "2026-08-21 20:04:28")
	lastSessionRow := sessionTestTime(t, "2026-08-21 20:54:20")
	adventureAt := sessionTestTime(t, "2026-08-21 17:00:52")
	loginAt := sessionTestTime(t, "2026-08-21 17:00:57")

	sessionTestRow(t, m, enteredAt, "You have entered The Castle of Mistmoore 4 (Refined).", data.LogRowEventTypeZoneChange, "", "The Castle of Mistmoore 4 (Refined)")
	sessionTestRow(t, m, lastSessionRow, "A final message from the previous session.", data.LogRowEventTypeUnknown)
	sessionTestRow(t, m, adventureAt, loginAdventureMessage, data.LogRowEventTypeUnknown)
	sessionTestRow(t, m, loginAt, loginMessage, data.LogRowEventTypeUnknown)

	if err := m.activeImport.Commit(0, loginAt); err != nil {
		t.Fatalf("commit import: %v", err)
	}

	var leftAt time.Time
	if err := db.QueryRow(`SELECT left_at FROM zone_visits LIMIT 1`).Scan(&leftAt); err != nil {
		t.Fatalf("read closed visit: %v", err)
	}
	if !leftAt.Equal(lastSessionRow) {
		t.Fatalf("visit closed at %v, want %v", leftAt, lastSessionRow)
	}
}

func TestZoneStatisticsOnlyExtendsNewestVisitWhenItIsOpen(t *testing.T) {
	m, db := newSessionTestModule(t)
	zoneID, err := m.activeImport.GetOrCreateZone("North Kaladim")
	if err != nil {
		t.Fatalf("create zone: %v", err)
	}
	if _, err := m.activeImport.InsertZoneVisit(zoneID, "North Kaladim", sessionTestTime(t, "2026-08-19 20:47:48")); err != nil {
		t.Fatalf("insert stale open visit: %v", err)
	}
	newestEnteredAt := sessionTestTime(t, "2026-08-26 19:00:00")
	if _, err := m.activeImport.InsertZoneVisit(zoneID, "North Kaladim", newestEnteredAt); err != nil {
		t.Fatalf("insert newest open visit: %v", err)
	}
	lastTimestamp := newestEnteredAt.Add(time.Hour)
	if err := m.activeImport.Commit(0, lastTimestamp); err != nil {
		t.Fatalf("commit import: %v", err)
	}

	stats, err := GetZoneStatistics(db)
	if err != nil {
		t.Fatalf("get zone statistics: %v", err)
	}
	if len(stats) != 1 {
		t.Fatalf("got %d zone rows, want 1", len(stats))
	}
	if stats[0].Visits != 2 {
		t.Fatalf("got %d visits, want 2", stats[0].Visits)
	}
	if stats[0].TimeSpent != time.Hour {
		t.Fatalf("got %v spent in zone, want %v", stats[0].TimeSpent, time.Hour)
	}
}

func TestStaleOpenVisitIsNotRestoredOrExtended(t *testing.T) {
	m, db := newSessionTestModule(t)
	zoneID, err := m.activeImport.GetOrCreateZone("North Kaladim")
	if err != nil {
		t.Fatalf("create zone: %v", err)
	}
	if _, err := m.activeImport.InsertZoneVisit(zoneID, "North Kaladim", sessionTestTime(t, "2026-08-19 20:47:48")); err != nil {
		t.Fatalf("insert stale open visit: %v", err)
	}
	closedAt := sessionTestTime(t, "2026-08-26 20:00:00")
	latestVisitID, err := m.activeImport.InsertZoneVisit(zoneID, "North Kaladim", closedAt.Add(-time.Hour))
	if err != nil {
		t.Fatalf("insert latest visit: %v", err)
	}
	if err := m.activeImport.CloseZoneVisit(latestVisitID, closedAt); err != nil {
		t.Fatalf("close latest visit: %v", err)
	}
	if err := m.activeImport.Commit(0, closedAt.Add(time.Hour)); err != nil {
		t.Fatalf("commit import: %v", err)
	}

	_, _, _, found, err := GetOpenZoneVisit(db)
	if err != nil {
		t.Fatalf("get open visit: %v", err)
	}
	if found {
		t.Fatal("stale open visit was restored")
	}
	stats, err := GetZoneStatistics(db)
	if err != nil {
		t.Fatalf("get zone statistics: %v", err)
	}
	if len(stats) != 1 || stats[0].TimeSpent != time.Hour {
		t.Fatalf("stale visit affected time spent: %#v", stats)
	}
}

func newSessionTestModule(t *testing.T) (*Module, *sql.DB) {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open test database: %v", err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { db.Close() })
	if err := PrepareDb(db); err != nil {
		t.Fatalf("prepare test database: %v", err)
	}
	activeImport, err := BeginImport(db)
	if err != nil {
		t.Fatalf("begin test import: %v", err)
	}
	t.Cleanup(func() { activeImport.Rollback() })
	return &Module{activeImport: activeImport}, db
}

func sessionTestRow(t *testing.T, m *Module, timestamp time.Time, message string, eventType data.LogRowEventType, values ...string) {
	t.Helper()
	if err := m.OnLogRow(&data.LogRowEvent{
		Timestamp: timestamp,
		Message:   message,
		Type:      eventType,
		Data:      values,
	}); err != nil {
		t.Fatalf("import %q: %v", message, err)
	}
}

func sessionTestTime(t *testing.T, value string) time.Time {
	t.Helper()
	result, err := time.ParseInLocation("2006-01-02 15:04:05", value, time.Local)
	if err != nil {
		t.Fatalf("parse test time %q: %v", value, err)
	}
	return result
}
