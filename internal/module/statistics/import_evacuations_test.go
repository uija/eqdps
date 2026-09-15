package statistics

import (
	"testing"
	"time"

	"github.com/uija/eqdps/internal/data"
)

func TestSessionEvacuations(t *testing.T) {
	for _, scenario := range []struct {
		name                                                           string
		caster                                                         string
		interrupted, noLoading, stale, differentZone, duplicate, split bool
		want                                                           int64
	}{
		{name: "self", caster: "You", want: 1},
		{name: "other", caster: "Gigglemage", want: 1},
		{name: "self interrupted", caster: "You", interrupted: true},
		{name: "other interrupted", caster: "Gigglemage", interrupted: true},
		{name: "no loading", caster: "You", noLoading: true},
		{name: "expired", caster: "You", stale: true},
		{name: "different instance", caster: "You", differentZone: true},
		{name: "simultaneous casts", caster: "You", duplicate: true, want: 1},
		{name: "incremental import", caster: "Gigglemage", split: true, want: 1},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			m, db := newSessionTestModule(t)
			start := sessionTestTime(t, "2026-09-15 12:00:00")
			zone := "The Ruins of Old Guk 4 (Refined)"
			sessionTestRow(t, m, start, "", data.LogRowEventTypeZoneChange, "", zone)
			zoneID := m.currentZone
			sessionTestRow(t, m, start.Add(time.Minute), "", data.LogRowEventTypeCast, "", scenario.caster, "Lesser Evacuate")
			if scenario.duplicate {
				sessionTestRow(t, m, start.Add(time.Minute), "", data.LogRowEventTypeCast, "", "Gigglemage", "Lesser Succor")
			}
			if scenario.interrupted {
				message := scenario.caster + "'s Lesser Evacuate spell is interrupted."
				if scenario.caster == "You" {
					message = "Your Lesser Evacuate spell is interrupted."
				}
				sessionTestRow(t, m, start.Add(62*time.Second), message, data.LogRowEventTypeUnknown)
			}
			if scenario.split {
				if err := m.activeImport.Commit(100, start.Add(time.Minute)); err != nil {
					t.Fatal(err)
				}
				active, err := BeginImport(db)
				if err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() { active.Rollback() })
				m = &Module{activeImport: active, currentZone: zoneID, currentVisit: m.currentVisit, currentVisitAt: start}
			}
			loadingAt := start.Add(65 * time.Second)
			if scenario.stale {
				loadingAt = start.Add(2 * time.Minute)
			}
			if !scenario.noLoading {
				sessionTestRow(t, m, loadingAt, "LOADING, PLEASE WAIT...", data.LogRowEventTypeUnknown)
			}
			if scenario.differentZone {
				zone = "The Ruins of Old Guk 5 (Refined)"
			}
			sessionTestRow(t, m, loadingAt.Add(10*time.Second), "", data.LogRowEventTypeZoneChange, "", zone)
			// A later same-zone entry without a new cast must not count again.
			sessionTestRow(t, m, start.Add(3*time.Minute), "", data.LogRowEventTypeZoneChange, "", zone)
			if err := m.activeImport.Commit(200, start.Add(4*time.Minute)); err != nil {
				t.Fatal(err)
			}
			session := SessionStatistics{ZoneID: zoneID, EnteredAt: start, Duration: 4 * time.Minute}
			details, err := GetSessionDetails(db, session)
			if err != nil {
				t.Fatal(err)
			}
			crawl, err := GetDungeonCrawlDetails(db, session)
			if err != nil {
				t.Fatal(err)
			}
			if details.EvacCount != scenario.want || crawl.EvacCount != scenario.want {
				t.Fatalf("got normal=%d crawl=%d, want %d", details.EvacCount, crawl.EvacCount, scenario.want)
			}
			// The following session must not inherit this session's evac.
			session.EnteredAt = start.Add(4 * time.Minute)
			if count, err := getSessionEvacCount(db, session); err != nil || count != 0 {
				t.Fatalf("wrong next session count: %d, %v", count, err)
			}
		})
	}
}
