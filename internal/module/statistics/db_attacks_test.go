package statistics

import (
	"database/sql"
	"testing"
)

func TestAttackStatisticsSchema(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	defer db.Close()
	for range 2 {
		if err := PrepareDb(db); err != nil {
			t.Fatal(err)
		}
	}

	// A failure-only attack has no damage range, rather than a zero minimum.
	if _, err := db.Exec(`INSERT INTO attack_statistics (name, category, miss_count)
		VALUES ('crush', 'Melee', 3)`); err != nil {
		t.Fatal(err)
	}
	var count, misses int
	var minimum sql.NullInt64
	if err := db.QueryRow(`SELECT normal_count + crit_count, normal_min, miss_count
		FROM attack_statistics WHERE name = 'crush'`).Scan(&count, &minimum, &misses); err != nil {
		t.Fatal(err)
	}
	if count != 0 || minimum.Valid || misses != 3 {
		t.Fatalf("unexpected failure-only totals: hits=%d min=%v misses=%d", count, minimum, misses)
	}

	// The same spell name can have independent direct-hit and DoT totals.
	for _, category := range []string{"Spells", "DoTs", "Procs", "Damage Shield"} {
		if _, err := db.Exec(`INSERT INTO attack_statistics
			(name, category, normal_count, normal_damage, normal_min, normal_max,
			 crit_count, crit_damage, crit_min, crit_max)
			VALUES ('Test Spell', ?, 2, 30, 10, 20, 1, 40, 40, 40)`, category); err != nil {
			t.Fatal(err)
		}
	}
	for _, query := range []string{
		`INSERT INTO attack_statistics (name, category) VALUES ('CRUSH', 'Melee')`,
		`INSERT INTO attack_statistics (name, category) VALUES ('unknown', 'invalid')`,
		`UPDATE attack_statistics SET miss_count = -1 WHERE name = 'crush'`,
		`UPDATE attack_statistics SET normal_count = 1 WHERE name = 'crush'`,
		`UPDATE attack_statistics SET crit_min = 50 WHERE name = 'Test Spell'`,
	} {
		if _, err := db.Exec(query); err == nil {
			t.Errorf("expected constraint error for %s", query)
		}
	}
	// Preparing an existing database preserves its accumulated totals.
	if err := PrepareDb(db); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT count(*) FROM attack_statistics`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 5 {
		t.Fatalf("got %d rows, want 5", count)
	}
	if err := DropTables(db); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT count(*) FROM sqlite_master WHERE name = 'attack_statistics'`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatal("attack_statistics remains after DropTables")
	}
}
