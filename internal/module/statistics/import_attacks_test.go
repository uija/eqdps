package statistics

import (
	"database/sql"
	"testing"
	"time"

	"github.com/uija/eqdps/internal/data"
	"github.com/uija/eqdps/internal/eqlog"
)

func TestImportAttackRows(t *testing.T) {
	m, db := newSessionTestModule(t)
	m.characterName = "Tester"
	parser := eqlog.NewParser(1)
	importMessage := func(message string) {
		t.Helper()
		event, ok := parser.ParseRow("[Tue Sep 15 12:00:00 2026] "+message, 0, false)
		if !ok {
			t.Fatalf("not parsed: %s", message)
		}
		if err := m.OnLogRow(event); err != nil {
			t.Fatalf("import %s: %v", message, err)
		}
	}
	for _, message := range []string{
		"You try to crush a rat, but miss!",
		"You crush a rat for 10 points of damage.",
		"You crush a rat for 30 points of damage.",
		"You crush a rat for 0 points of damage.",
		"You crush a rat for 50 points of damage. (Critical)",
		"You crush a rat for 40 points of damage. (Riposte Critical)",
		"Tester crushes a rat for 20 points of damage.",
		"You try to crush a rat, but a rat dodges!",
		"You try to crush a rat, but a rat parries!",
		"You try to crush a rat, but a rat blocks!",
		"You try to crush a rat, but a rat ripostes!",
		"You try to crush a rat, but a rat's magical skin absorbs the blow!",
		"You try to kick a rat, but miss!",
		"You hit a rat for 100 points of magic damage by Ignite. (Critical)",
		"You hit a rat for 25 points of magic damage by Ignite.",
		"a rat has taken 12 damage from your Ignite.",
		"a rat has taken 24 damage from your Ignite. (Critical)",
		"a rat has taken 15 damage from Ignite by Tester.",
		"a rat hits YOU for 999 points of damage.",
		"Other crushes a rat for 999 points of damage.",
		"a rat has taken 999 damage from Ignite by Other.",
		"Other tries to crush a rat, but misses!",
	} {
		importMessage(message)
	}
	if err := m.activeImport.Commit(100, time.Time{}); err != nil {
		t.Fatal(err)
	}
	var normalCount, normalDamage, normalMin, normalMax, critCount, critDamage, critMin, critMax int
	if err := db.QueryRow(`SELECT normal_count, normal_damage, normal_min, normal_max,
		crit_count, crit_damage, crit_min, crit_max FROM attack_statistics
		WHERE category = 'Melee' AND name = 'crush'`).Scan(
		&normalCount, &normalDamage, &normalMin, &normalMax, &critCount, &critDamage, &critMin, &critMax); err != nil {
		t.Fatal(err)
	}
	if normalCount != 4 || normalDamage != 60 || normalMin != 0 || normalMax != 30 ||
		critCount != 2 || critDamage != 90 || critMin != 40 || critMax != 50 {
		t.Fatalf("wrong hit totals: normal %d/%d/%d/%d crit %d/%d/%d/%d",
			normalCount, normalDamage, normalMin, normalMax, critCount, critDamage, critMin, critMax)
	}
	var failures [6]int
	if err := db.QueryRow(`SELECT miss_count, dodge_count, parry_count, block_count, riposte_count, absorb_count
		FROM attack_statistics WHERE name = 'crush'`).Scan(
		&failures[0], &failures[1], &failures[2], &failures[3], &failures[4], &failures[5]); err != nil {
		t.Fatal(err)
	}
	if failures != [6]int{1, 1, 1, 1, 1, 1} {
		t.Fatalf("wrong failures: %v", failures)
	}
	var minimum sql.NullInt64
	if err := db.QueryRow(`SELECT normal_min FROM attack_statistics WHERE name = 'kick'`).Scan(&minimum); err != nil || minimum.Valid {
		t.Fatalf("failure-only minimum: %v, err: %v", minimum, err)
	}
	for _, want := range []struct {
		category                              string
		normalCount, normalDamage, critDamage int
	}{{"Spells", 1, 25, 100}, {"DoTs", 2, 27, 24}} {
		if err := db.QueryRow(`SELECT normal_count, normal_damage, crit_count, crit_damage
			FROM attack_statistics WHERE name = 'Ignite' AND category = ?`, want.category).
			Scan(&normalCount, &normalDamage, &critCount, &critDamage); err != nil {
			t.Fatal(err)
		}
		if normalCount != want.normalCount || normalDamage != want.normalDamage || critCount != 1 || critDamage != want.critDamage {
			t.Fatalf("unexpected %s totals: %d/%d/%d/%d", want.category, normalCount, normalDamage, critCount, critDamage)
		}
	}

	// A later import adds to existing totals; rolling it back also leaves the
	// saved offset untouched, allowing the same rows to be safely retried.
	for _, commit := range []bool{false, true} {
		var err error
		m.activeImport, err = BeginImport(db)
		if err != nil {
			t.Fatal(err)
		}
		importMessage("You crush a rat for 5 points of damage.")
		if commit {
			err = m.activeImport.Commit(200, time.Time{})
		} else {
			err = m.activeImport.Rollback()
		}
		if err != nil {
			t.Fatal(err)
		}
	}
	if err := db.QueryRow(`SELECT normal_count FROM attack_statistics WHERE name = 'crush'`).Scan(&normalCount); err != nil || normalCount != 5 {
		t.Fatalf("wrong committed total: %d, %v", normalCount, err)
	}
	if offset, err := GetLogOffset(db); err != nil || offset != 200 {
		t.Fatalf("wrong offset: %d, %v", offset, err)
	}
}

func TestImportMalformedAttack(t *testing.T) {
	m, _ := newSessionTestModule(t)
	for _, event := range []*data.LogRowEvent{
		{Type: data.LogRowEventTypeDamage},
		{Type: data.LogRowEventTypeYourDamageOverTime},
		{Type: data.LogRowEventTypeDamageOverTime},
		{Type: data.LogRowEventTypeFailedMelee},
		{Type: data.LogRowEventTypeFailedMeleeOthers},
		{Type: data.LogRowEventTypeDamage, Data: []string{"", "You", "crush", "rat", "bad", "damage", "", ""}},
		{Type: data.LogRowEventTypeFailedMelee, Data: []string{"", "crush", "rat", "unknown"}},
	} {
		if err := m.OnLogRow(event); err == nil {
			t.Fatalf("expected malformed event error: %#v", event)
		}
	}
}
