package statistics

import (
	"database/sql"
	"testing"
	"time"
)

func TestDeathTotals(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	if err := PrepareDb(db); err != nil {
		t.Fatal(err)
	}
	zone := sessionInsertID(t, db, `INSERT INTO zones(name) VALUES ('Guk')`)
	otherZone := sessionInsertID(t, db, `INSERT INTO zones(name) VALUES ('Oggok')`)
	mob := sessionInsertID(t, db, `INSERT INTO mobs(zone_id, name) VALUES (?, 'a ghoul')`, zone)
	otherMob := sessionInsertID(t, db, `INSERT INTO mobs(zone_id, name) VALUES (?, 'a guard')`, otherZone)
	start := time.Date(2026, 9, 10, 15, 0, 0, 0, time.Local)
	session := SessionStatistics{ZoneID: zone, EnteredAt: start, Duration: time.Hour}
	check := func(wantSession, wantOverall int64) {
		t.Helper()
		normal, err := GetSessionDetails(db, session)
		if err != nil {
			t.Fatal(err)
		}
		crawl, err := GetDungeonCrawlDetails(db, session)
		if err != nil {
			t.Fatal(err)
		}
		overview, err := GetOverviewStatistics(db)
		if err != nil {
			t.Fatal(err)
		}
		if normal.DeathCount != wantSession || crawl.DeathCount != wantSession || overview.DeathCount != wantOverall {
			t.Fatalf("death totals: normal=%d crawl=%d overview=%d; want %d/%d/%d", normal.DeathCount, crawl.DeathCount, overview.DeathCount, wantSession, wantSession, wantOverall)
		}
	}
	check(0, 0)
	for _, at := range []time.Time{start.Add(-time.Second), start, start.Add(time.Minute), start.Add(time.Hour)} {
		sessionInsertID(t, db, `INSERT INTO player_deaths(zone_id, mob_id, died_at) VALUES (?, ?, ?)`, zone, mob, at)
	}
	sessionInsertID(t, db, `INSERT INTO player_deaths(zone_id, mob_id, died_at) VALUES (?, ?, ?)`, otherZone, otherMob, start)
	check(2, 5)
}
