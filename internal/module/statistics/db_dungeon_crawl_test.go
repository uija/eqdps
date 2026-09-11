package statistics

import (
	"database/sql"
	"testing"
	"time"

	_ "github.com/glebarez/go-sqlite"
)

func TestDungeonCrawlDetails(t *testing.T) {
	sessionInsertID := func(t *testing.T, db *sql.DB, query string, values ...any) int64 {
		t.Helper()
		result, err := db.Exec(query, values...)
		if err != nil {
			t.Fatal(err)
		}
		id, err := result.LastInsertId()
		if err != nil {
			t.Fatal(err)
		}
		return id
	}
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
	mob := sessionInsertID(t, db, `INSERT INTO mobs(zone_id, name) VALUES (?, 'a froglok')`, zone)
	chest := sessionInsertID(t, db, `INSERT INTO mobs(zone_id, name) VALUES (?, 'Reward Chest')`, zone)
	major := sessionInsertID(t, db, `INSERT INTO items(name) VALUES ('Mote of Major Potential')`)
	lesser := sessionInsertID(t, db, `INSERT INTO items(name) VALUES ('Mote of Lesser Potential')`)
	dagger := sessionInsertID(t, db, `INSERT INTO items(name) VALUES ('Rusty Dagger')`)
	start := time.Date(2026, 9, 9, 18, 0, 0, 0, time.Local)
	session := SessionStatistics{ZoneID: zone, EnteredAt: start, Duration: time.Hour}
	for _, kind := range []string{"player", "other", "unknown"} {
		sessionInsertID(t, db, `INSERT INTO kills(zone_id, mob_id, killed_at, kill_type) VALUES (?, ?, ?, ?)`, zone, mob, start, kind)
	}
	sessionInsertID(t, db, `INSERT INTO experience(zone_id, received_at, percent) VALUES (?, ?, 1.25)`, zone, start)
	sessionInsertID(t, db, `INSERT INTO money(zone_id, received_at, amount_copper, source) VALUES (?, ?, 123, 'corpse')`, zone, start)
	sessionInsertID(t, db, `INSERT INTO money(zone_id, received_at, amount_copper, source) VALUES (?, ?, 456, 'loot_sale')`, zone, start)
	addLoot := func(z, source, item int64, raw string, quantity int, at time.Time, destination string) {
		t.Helper()
		sessionInsertID(t, db, `INSERT INTO loot(zone_id, mob_id, item_id, raw_item_name, quantity, looted_at, destination) VALUES (?, ?, ?, ?, ?, ?, ?)`, z, source, item, raw, quantity, at, destination)
	}
	addLoot(zone, mob, major, "Mote of Major Potential", 2, start, "currency")
	addLoot(zone, chest, major, "Mote of Major Potential", 6, start.Add(50*time.Minute), "currency")
	addLoot(zone, mob, lesser, "Mote of Lesser Potential", 3, start, "currency")
	addLoot(zone, chest, dagger, "Rusty Dagger +4", 1, start, "sold")
	addLoot(zone, chest, dagger, "Rusty Dagger +4", 1, start, "inventory")
	addLoot(zone, chest, dagger, "Rusty Dagger +5", 1, start, "inventory")
	// Nearby sessions and other zones must not contribute.
	addLoot(zone, chest, major, "Mote of Major Potential", 100, start.Add(-time.Second), "currency")
	addLoot(zone, chest, major, "Mote of Major Potential", 100, start.Add(time.Hour), "currency")
	addLoot(otherZone, chest, major, "Mote of Major Potential", 100, start, "currency")
	got, err := GetDungeonCrawlDetails(db, session)
	if err != nil {
		t.Fatal(err)
	}
	if got.Kills != 2 || got.ExperienceGained != 1.25 || got.Money != 579 || got.Motes != 11 || got.Motes5Plus != 8 {
		t.Fatalf("unexpected totals: %+v", got)
	}
	if len(got.MoteDetails) != 2 || got.MoteDetails[1].Quantity != 8 || len(got.MoteDetails[1].Sources) != 2 {
		t.Fatalf("unexpected Mote details: %+v", got.MoteDetails)
	}
	if len(got.ChestRewards) != 3 || got.ChestRewards[0].Quantity != 6 || got.ChestRewards[1].Item != "Rusty Dagger +4" || got.ChestRewards[1].Quantity != 2 || got.ChestRewards[2].Item != "Rusty Dagger +5" {
		t.Fatalf("unexpected rewards: %+v", got.ChestRewards)
	}
	_, err = GetDungeonCrawlDetails(nil, session)
	if err == nil {
		t.Fatal("expected nil database error")
	}
	_, err = GetDungeonCrawlDetails(db, SessionStatistics{})
	if err == nil {
		t.Fatal("expected invalid session error")
	}
}
