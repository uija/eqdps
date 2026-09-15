package statistics

import (
	"reflect"
	"testing"
	"time"

	"github.com/uija/eqdps/internal/data"
	"github.com/uija/eqdps/internal/eqlog"
)

func TestSessionFactionAdjustments(t *testing.T) {
	m, db := newSessionTestModule(t)
	start := sessionTestTime(t, "2026-09-15 12:00:00")
	parser := eqlog.NewParser(1)
	observe := func(at time.Time, message string) {
		t.Helper()
		e, ok := parser.ParseRow("["+at.Format("Mon Jan 02 15:04:05 2006")+"] "+message, 0, false)
		if !ok {
			t.Fatalf("not parsed: %s", message)
		}
		if err := m.OnLogRow(e); err != nil {
			t.Fatal(err)
		}
	}
	// Unknown-zone hits remain available for future overall statistics.
	observe(start.Add(-time.Minute), "Your faction standing with Frogloks of Guk has been adjusted by 100.")
	sessionTestRow(t, m, start, "", data.LogRowEventTypeZoneChange, "", "Guk")
	zoneID := m.currentZone
	observe(start, "Your faction standing with Frogloks of Guk has been adjusted by +5.")
	observe(start.Add(time.Second), "Your faction standing with frogloks of guk has been adjusted by 5.")
	observe(start.Add(2*time.Second), "Your faction standing with Frogloks of Guk has been adjusted by -25.")
	observe(start.Add(3*time.Second), "Your faction standing with Frogloks of Guk could not possibly get any better.")
	observe(start.Add(4*time.Second), "Your faction standing with Frogloks of Guk could not possibly get any worse.")
	observe(start.Add(5*time.Second), "Your faction standing with Another Faction has been adjusted by -2.")
	observe(start.Add(6*time.Second), "Your faction standing with Capped Faction could not possibly get any better.")
	// Preserve existing totals across an incremental import.
	if err := m.activeImport.Commit(100, start.Add(6*time.Second)); err != nil {
		t.Fatal(err)
	}
	active, err := BeginImport(db)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { active.Rollback() })
	m.activeImport = active
	observe(start.Add(time.Minute), "Your faction standing with Frogloks of Guk has been adjusted by +5.")
	// End is exclusive, and another zone's adjustments are excluded.
	observe(start.Add(2*time.Minute), "Your faction standing with Frogloks of Guk has been adjusted by 500.")
	sessionTestRow(t, m, start.Add(2*time.Minute), "", data.LogRowEventTypeZoneChange, "", "Elsewhere")
	observe(start.Add(time.Minute), "Your faction standing with Frogloks of Guk has been adjusted by 900.")
	if err := active.Commit(200, start.Add(2*time.Minute)); err != nil {
		t.Fatal(err)
	}
	session := SessionStatistics{ZoneID: zoneID, EnteredAt: start, Duration: 2 * time.Minute}
	normal, err := GetSessionDetails(db, session)
	if err != nil {
		t.Fatal(err)
	}
	crawl, err := GetDungeonCrawlDetails(db, session)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(normal.Factions, crawl.Factions) {
		t.Fatal("detail views disagree")
	}
	if len(normal.Factions) != 2 {
		t.Fatalf("unexpected factions: %+v", normal.Factions)
	}
	f := normal.Factions[1]
	if f.Gained != 15 || f.Lost != -25 || f.NetChange() != -10 {
		t.Fatalf("wrong totals: %+v", f)
	}
	var unknown int
	if err := db.QueryRow(`SELECT COUNT(*) FROM faction_adjustments WHERE zone_id IS NULL`).Scan(&unknown); err != nil || unknown != 1 {
		t.Fatalf("unknown-zone row missing: %d %v", unknown, err)
	}
	// A rolled-back import must not inflate totals or advance the offset.
	active, err = BeginImport(db)
	if err != nil {
		t.Fatal(err)
	}
	m.activeImport, m.currentZone = active, zoneID
	observe(start, "Your faction standing with Frogloks of Guk has been adjusted by 1000.")
	if err := active.Rollback(); err != nil {
		t.Fatal(err)
	}
	after, err := GetSessionFactionDetails(db, session)
	if err != nil || !reflect.DeepEqual(after, normal.Factions) {
		t.Fatalf("rollback changed totals: %+v %v", after, err)
	}
	if offset, err := GetLogOffset(db); err != nil || offset != 200 {
		t.Fatalf("wrong offset: %d %v", offset, err)
	}
	if err := DropTables(db); err != nil {
		t.Fatal(err)
	}
	if err := PrepareDb(db); err != nil {
		t.Fatal(err)
	}
	if err := PrepareDb(db); err != nil {
		t.Fatal(err)
	}
}

func TestMalformedFactionAdjustment(t *testing.T) {
	m, _ := newSessionTestModule(t)
	for _, fields := range [][]string{nil, {"", "", "5"}, {"", "Faction", "invalid"}, {"", "Faction", ""}} {
		if err := m.OnLogRow(&data.LogRowEvent{Type: data.LogRowEventTypeFaction, Data: fields}); err == nil {
			t.Fatalf("accepted invalid data: %v", fields)
		}
	}
}
