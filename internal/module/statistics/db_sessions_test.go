package statistics

import (
	"database/sql"
	"testing"
	"time"
)

func TestGetSessionStatisticsFiltersAndAggregatesVisits(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	db.SetMaxOpenConns(1)
	defer db.Close()
	if err := PrepareDb(db); err != nil {
		t.Fatalf("prepare database: %v", err)
	}

	zoneID := sessionInsertID(t, db, `INSERT INTO zones (name) VALUES (?)`, "Befallen")
	mobID := sessionInsertID(t, db, `INSERT INTO mobs (zone_id, name) VALUES (?, ?)`, zoneID, "a rat")
	otherMobID := sessionInsertID(t, db, `INSERT INTO mobs (zone_id, name) VALUES (?, ?)`, zoneID, "another rat")
	itemID := sessionInsertID(t, db, `INSERT INTO items (name) VALUES (?)`, "Mote of Lesser Potential")

	start := sessionTestTime(t, "2026-08-29 20:00:00")
	sessionInsertID(t, db, `
		INSERT INTO zone_visits (zone_id, entered_at, left_at, raw_zone_name)
		VALUES (?, ?, ?, ?)
	`, zoneID, start, start.Add(30*time.Second), "Befallen")
	sessionInsertID(t, db, `
		INSERT INTO kills (zone_id, mob_id, killed_at, kill_type)
		VALUES (?, ?, ?, 'player')
	`, zoneID, mobID, start.Add(10*time.Second))

	start = start.Add(time.Hour)
	sessionInsertID(t, db, `
		INSERT INTO zone_visits (zone_id, entered_at, left_at, raw_zone_name)
		VALUES (?, ?, ?, ?)
	`, zoneID, start, start.Add(10*time.Minute), "Befallen 4 (Refined)")
	killID := sessionInsertID(t, db, `
		INSERT INTO kills (zone_id, mob_id, killed_at, kill_type)
		VALUES (?, ?, ?, 'player')
	`, zoneID, mobID, start.Add(time.Minute))
	sessionInsertID(t, db, `
		INSERT INTO kills (zone_id, mob_id, killed_at, kill_type)
		VALUES (?, ?, ?, 'other')
	`, zoneID, mobID, start.Add(2*time.Minute))
	sessionInsertID(t, db, `
		INSERT INTO experience (zone_id, received_at, percent)
		VALUES (?, ?, ?)
	`, zoneID, start.Add(3*time.Minute), 1.25)
	sessionInsertID(t, db, `
		INSERT INTO loot (
			zone_id, mob_id, kill_id, item_id, raw_item_name, quantity,
			looted_at, destination
		) VALUES (?, ?, ?, ?, ?, ?, ?, 'inventory')
	`, zoneID, mobID, killID, itemID, "a Mote of Lesser Potential", 3, start.Add(4*time.Minute))
	sessionInsertID(t, db, `
		INSERT INTO loot (
			zone_id, mob_id, item_id, raw_item_name, quantity,
			looted_at, destination
		) VALUES (?, ?, ?, ?, ?, ?, 'inventory')
	`, zoneID, otherMobID, itemID, "a Mote of Lesser Potential", 2, start.Add(4*time.Minute))
	sessionInsertID(t, db, `
		INSERT INTO money (zone_id, received_at, amount_copper, source)
		VALUES (?, ?, ?, 'corpse')
	`, zoneID, start.Add(5*time.Minute), 1234)
	sessionInsertID(t, db, `
		INSERT INTO player_deaths (zone_id, mob_id, died_at)
		VALUES (?, ?, ?)
	`, zoneID, mobID, start.Add(6*time.Minute))

	start = start.Add(time.Hour)
	sessionInsertID(t, db, `
		INSERT INTO zone_visits (zone_id, entered_at, left_at, raw_zone_name)
		VALUES (?, ?, ?, ?)
	`, zoneID, start, start.Add(10*time.Minute), "Befallen")
	sessionInsertID(t, db, `
		INSERT INTO kills (zone_id, mob_id, killed_at, kill_type)
		VALUES (?, ?, ?, 'other')
	`, zoneID, mobID, start.Add(time.Minute))

	statistics, err := GetSessionStatistics(db)
	if err != nil {
		t.Fatalf("get session statistics: %v", err)
	}
	if len(statistics) != 1 {
		t.Fatalf("got %d sessions, want 1", len(statistics))
	}
	got := statistics[0]
	if got.Zone != "Befallen 4 (Refined)" || got.Duration != 10*time.Minute || got.Kills != 2 || got.ExperienceGained != 1.25 || got.Motes != 5 {
		t.Fatalf("unexpected session statistics: %#v", got)
	}

	details, err := GetSessionDetails(db, got)
	if err != nil {
		t.Fatalf("get session details: %v", err)
	}
	if details.Money != 1234 {
		t.Fatalf("got %d session money, want 1234", details.Money)
	}
	if len(details.Mobs) != 1 || details.Mobs[0].Kills != 2 || details.Mobs[0].KilledByYou != 1 {
		t.Fatalf("unexpected session mobs: %#v", details.Mobs)
	}
	if len(details.Loot) != 1 || details.Loot[0].Item != "Mote of Lesser Potential" || details.Loot[0].Mob != "a rat, another rat" || details.Loot[0].Quantity != 5 {
		t.Fatalf("unexpected session loot: %#v", details.Loot)
	}
	if len(details.Deaths) != 1 || details.Deaths[0].Mob != "a rat" || details.Deaths[0].Deaths != 1 {
		t.Fatalf("unexpected session deaths: %#v", details.Deaths)
	}
}

func sessionInsertID(t *testing.T, db *sql.DB, query string, values ...any) int64 {
	t.Helper()
	result, err := db.Exec(query, values...)
	if err != nil {
		t.Fatalf("insert session test data: %v", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		t.Fatalf("get inserted session test ID: %v", err)
	}
	return id
}
