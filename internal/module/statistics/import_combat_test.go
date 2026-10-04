package statistics

import (
	"database/sql"
	"reflect"
	"testing"
	"time"

	"github.com/uija/eqdps/internal/eqlog"
)

func combatTestRow(t *testing.T, m *Module, seconds int, message string) {
	t.Helper()
	at := sessionTestTime(t, "2026-10-04 12:00:00").Add(time.Duration(seconds) * time.Second)
	e, ok := eqlog.NewParser(1).ParseRow("["+at.Format("Mon Jan 02 15:04:05 2006")+"] "+message, 0, true)
	if !ok {
		t.Fatalf("cannot parse %q", message)
	}
	if err := m.OnLogRow(e); err != nil {
		t.Fatalf("import %q: %v", message, err)
	}
}

func resumeCombatTestImport(t *testing.T, db *sql.DB) *Module {
	t.Helper()
	i, err := BeginImport(db)
	if err != nil {
		t.Fatal(err)
	}
	// Read using the transaction: the test DB uses one connection.
	var visit, zone int64
	var entered, last time.Time
	if err := i.tx.QueryRow(`SELECT id,zone_id,entered_at FROM zone_visits WHERE left_at IS NULL ORDER BY id DESC LIMIT 1`).Scan(&visit, &zone, &entered); err != nil {
		t.Fatal(err)
	}
	if err := i.tx.QueryRow(`SELECT last_timestamp FROM log_state WHERE id=1`).Scan(&last); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { i.Rollback() })
	return &Module{activeImport: i, currentVisit: visit, currentZone: zone, currentVisitAt: entered, lastImportRow: last, characterName: "Tester"}
}

func TestSessionCombatQueriesAndRestart(t *testing.T) {
	var reference SessionCombatStatistics
	for _, split := range []bool{false, true} {
		m, db := newSessionTestModule(t)
		m.characterName = "Tester"
		for _, r := range []struct {
			at      int
			message string
		}{
			{0, "You have entered Befallen."},
			{10, "You try to slash a rat, but miss!"},
			{11, "You slash a rat for 100 points of damage."},
			{12, "Tester hits a bat for 50 points of magic damage by Ignite. (Critical)"},
			{13, "A bat tries to hit YOU, but misses!"},
		} {
			combatTestRow(t, m, r.at, r.message)
		}
		if split {
			if err := m.activeImport.Commit(100, m.lastImportRow); err != nil {
				t.Fatal(err)
			}
			m = resumeCombatTestImport(t, db)
		}
		for _, r := range []struct {
			at      int
			message string
		}{
			{14, "a rat has taken 15 damage from your Ignite V."},
			{15, "A rat is pierced by YOUR thorns for 7 points of non-melee damage."},
			{16, "A rat is pierced by Tester's thorns for 9 points of non-melee damage."},
			{17, "Other hits a rat for 999 points of damage."},
			{18, "Tester's pet hits a rat for 999 points of damage."},
			{19, "You hit Tester for 999 points of magic damage by Cannibalization V."},
			{20, "You have slain a rat!"},
			{25, "a bat has been slain by Other!"},
			{60, "You have entered Befallen."},
			{70, "You punch a skeleton for 30 points of damage."},
			{80, "You have slain a skeleton!"},
			{120, "You have entered Oggok."},
		} {
			combatTestRow(t, m, r.at, r.message)
		}
		if err := m.activeImport.Commit(200, m.lastImportRow); err != nil {
			t.Fatal(err)
		}
		sessions, err := GetSessionStatistics(db)
		if err != nil {
			t.Fatal(err)
		}
		if len(sessions) != 1 {
			t.Fatalf("sessions: %+v", sessions)
		}
		s := sessions[0]
		want := SessionCombatStatistics{TotalDamage: 211, CombatTime: 27 * time.Second, SessionDPS: 211.0 / 120, CombatDPS: 211.0 / 27}
		if s.Combat != want {
			t.Fatalf("split=%v combat=%+v want=%+v", split, s.Combat, want)
		}
		details, err := GetSessionDetails(db, s)
		if err != nil {
			t.Fatal(err)
		}
		crawl, err := GetDungeonCrawlDetails(db, s)
		if err != nil {
			t.Fatal(err)
		}
		if details.Combat != want || crawl.Combat != want {
			t.Fatal("detail query combat totals differ")
		}
		if split && !reflect.DeepEqual(reference, s.Combat) {
			t.Fatal("restart changed totals")
		}
		reference = s.Combat
		// An empty incremental import must not count checkpoint totals again.
		m = resumeCombatTestImport(t, db)
		if err := m.activeImport.Commit(200, m.lastImportRow); err != nil {
			t.Fatal(err)
		}
		again, err := GetSessionStatistics(db)
		if err != nil {
			t.Fatal(err)
		}
		if again[0].Combat != want {
			t.Fatal("empty import duplicated totals")
		}
	}
}

func TestSessionCombatTimeoutAndBoundaries(t *testing.T) {
	for _, tc := range []struct {
		name string
		rows []struct {
			at      int
			message string
		}
		seconds int64
	}{
		{"timeout", []struct {
			at      int
			message string
		}{{10, "You slash a rat for 10 points of damage."}, {20, "You slash a rat for 10 points of damage."}, {70, "Waiting."}, {100, "You slash a rat for 10 points of damage."}, {105, "You have slain a rat!"}}, 17},
		{"one second", []struct {
			at      int
			message string
		}{{10, "You slash a rat for 10 points of damage."}, {10, "You have slain a rat!"}}, 1},
		{"aggro clear", []struct {
			at      int
			message string
		}{{10, "You slash a rat for 10 points of damage."}, {20, "Your enemies have forgotten you!"}}, 11},
		{"death", []struct {
			at      int
			message string
		}{{10, "A rat hits YOU for 10 points of damage."}, {20, "You have been slain by a rat!"}}, 11},
		{"exit clipping", []struct {
			at      int
			message string
		}{{199, "You slash a rat for 10 points of damage."}}, 1},
		{"backward clock", []struct {
			at      int
			message string
		}{{100, "You slash a rat for 10 points of damage."}, {110, "You slash a rat for 10 points of damage."}, {20, "Clock changed."}, {30, "You slash a bat for 10 points of damage."}, {35, "You have slain a bat!"}}, 17},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m, db := newSessionTestModule(t)
			combatTestRow(t, m, 0, "You have entered Befallen.")
			for _, r := range tc.rows {
				combatTestRow(t, m, r.at, r.message)
			}
			combatTestRow(t, m, 200, "You have entered Oggok.")
			if err := m.activeImport.Commit(200, m.lastImportRow); err != nil {
				t.Fatal(err)
			}
			var seconds int64
			if err := db.QueryRow(`SELECT combat_seconds FROM zone_visits WHERE id=1`).Scan(&seconds); err != nil {
				t.Fatal(err)
			}
			if seconds != tc.seconds {
				t.Fatalf("combat seconds=%d want=%d", seconds, tc.seconds)
			}
		})
	}
}

func TestSessionCombatRollback(t *testing.T) {
	m, db := newSessionTestModule(t)
	combatTestRow(t, m, 0, "You have entered Befallen.")
	combatTestRow(t, m, 10, "You slash a rat for 10 points of damage.")
	if err := m.activeImport.Commit(100, m.lastImportRow); err != nil {
		t.Fatal(err)
	}
	m = resumeCombatTestImport(t, db)
	combatTestRow(t, m, 20, "You slash a rat for 100 points of damage.")
	if err := m.activeImport.flushCombat(); err != nil {
		t.Fatal(err)
	}
	if err := m.activeImport.Rollback(); err != nil {
		t.Fatal(err)
	}
	m = resumeCombatTestImport(t, db)
	combatTestRow(t, m, 20, "You slash a rat for 100 points of damage.")
	combatTestRow(t, m, 25, "You have slain a rat!")
	if err := m.activeImport.Commit(200, m.lastImportRow); err != nil {
		t.Fatal(err)
	}
	var damage, seconds int64
	if err := db.QueryRow(`SELECT total_damage,combat_seconds FROM zone_visits WHERE id=1`).Scan(&damage, &seconds); err != nil {
		t.Fatal(err)
	}
	if damage != 110 || seconds != 16 {
		t.Fatalf("rollback/retry totals=%d/%d", damage, seconds)
	}
}

func TestZeroCombatDPS(t *testing.T) {
	if got := sessionCombatStatistics(0, 0, 0); got != (SessionCombatStatistics{}) {
		t.Fatalf("zero totals: %+v", got)
	}
	// An interrupted ranked cast does not count as an attack or damage.
	m, _ := newSessionTestModule(t)
	combatTestRow(t, m, 0, "You have entered Befallen.")
	combatTestRow(t, m, 10, "Your Ignite V spell is interrupted.")
	combatTestRow(t, m, 11, "Your Ignite spell is interrupted.")
	if m.activeImport.combat.damage != 0 || m.activeImport.combat.seconds != 0 {
		t.Fatal("interruption counted as combat")
	}
}
