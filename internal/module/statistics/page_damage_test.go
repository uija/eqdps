package statistics

import (
	"testing"
	"time"
)

func TestGetDamageStatistics(t *testing.T) {
	m, db := newSessionTestModule(t)
	for _, hit := range []struct {
		amount   int64
		critical bool
	}{{10, false}, {20, false}, {60, true}} {
		if err := m.activeImport.addAttackHit("Ignite", "Spells", hit.amount, hit.critical); err != nil {
			t.Fatal(err)
		}
	}
	if err := m.activeImport.addAttackHit("Ignite", "DoTs", 0, false); err != nil {
		t.Fatal(err)
	}
	if _, err := m.activeImport.tx.Exec(`INSERT INTO attack_statistics (name, category, miss_count) VALUES ('kick', 'Melee', 2)`); err != nil {
		t.Fatal(err)
	}
	if err := m.activeImport.Commit(0, time.Time{}); err != nil {
		t.Fatal(err)
	}
	stats, err := GetDamageStatistics(db)
	if err != nil {
		t.Fatal(err)
	}
	if len(stats) != 3 {
		t.Fatalf("got %d rows", len(stats))
	}
	for _, s := range stats {
		switch s.Category {
		case "Spells":
			if s.Hits() != 3 || s.Normal.Average() != 15 || s.Critical.Average() != 60 || s.CritPercent() < 33.33 || s.CritPercent() > 33.34 {
				t.Fatalf("incorrect spell statistics: %+v", s)
			}
		case "DoTs":
			if !s.Normal.Min.Valid || s.Normal.Min.Int64 != 0 || s.Hits() != 1 {
				t.Fatalf("zero tick lost: %+v", s)
			}
		case "Melee":
			if s.Normal.Min.Valid || s.Critical.Max.Valid || s.Hits() != 0 || s.Miss != 2 || s.CritPercent() != 0 {
				t.Fatalf("incorrect failure-only statistics: %+v", s)
			}
		}
	}
}

func TestDamagePageFilterSortAndReset(t *testing.T) {
	p := NewDamagePage(nil, nil)
	p.allRows = []DamageRow{
		{Statistic: DamageStatistics{Name: "crush", Category: "Melee"}},
		{Statistic: DamageStatistics{Name: "Ignite", Category: "Spells", Normal: DamageRangeStatistics{Count: 2}}, Open: true},
		{Statistic: DamageStatistics{Name: "Ignite", Category: "DoTs", Normal: DamageRangeStatistics{Count: 5}}},
	}
	p.filter.SetText("IGNITE")
	p.sortColumn = 2
	p.applyFilter()
	if len(p.rows) != 2 || p.rows[0].Statistic.Category != "DoTs" || !p.rows[1].Open {
		t.Fatal("filter/sort lost category or expanded state")
	}
	p.filter.SetText("melee")
	p.applyFilter()
	if len(p.rows) != 1 || p.rows[0].Statistic.Name != "crush" {
		t.Fatal("category filter failed")
	}
	old := make(chan damageLoadResult, 1)
	p.pending = old
	p.Reset()
	old <- damageLoadResult{stats: []DamageStatistics{{Name: "stale"}}}
	p.receiveData()
	if p.loaded || p.rows != nil || p.filter.Text() != "" {
		t.Fatal("stale load survived reset")
	}
	p.pending = make(chan damageLoadResult, 1)
	p.pending <- damageLoadResult{stats: []DamageStatistics{}}
	p.receiveData()
	if !p.loaded || p.rows == nil || p.pending != nil {
		t.Fatal("empty database did not finish loading")
	}
}
