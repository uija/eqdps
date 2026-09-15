package statistics

import (
	"testing"
	"time"
)

func TestOverallFactionStatistics(t *testing.T) {
	m, db := newSessionTestModule(t)
	if _, err := m.activeImport.tx.Exec(`INSERT INTO faction_adjustments (name, observed_at, adjustment)
		VALUES ('Frogloks', ?, 5), ('frogloks', ?, -25), ('Frogloks', ?, 10), ('Other', ?, -2)`,
		time.Now(), time.Now(), time.Now(), time.Now()); err != nil {
		t.Fatal(err)
	}
	if err := m.activeImport.Commit(0, time.Time{}); err != nil {
		t.Fatal(err)
	}
	stats, err := GetFactionStatistics(db)
	if err != nil {
		t.Fatal(err)
	}
	if len(stats) != 2 || stats[0].Gained != 15 || stats[0].Lost != -25 || stats[0].NetChange() != -10 {
		t.Fatalf("unexpected overall totals: %+v", stats)
	}
	if _, err := GetFactionStatistics(nil); err == nil {
		t.Fatal("expected nil database error")
	}
}

func TestFactionsPageFilterSortReset(t *testing.T) {
	p := NewFactionsPage(nil, nil)
	p.allRows = []SessionFactionDetails{{Name: "Alpha", Gained: 5, Lost: -20}, {Name: "Beta", Gained: 10, Lost: -2}, {Name: "Gamma", Gained: 20, Lost: -30}}
	for column, want := range []string{"Alpha", "Gamma", "Gamma", "Beta"} {
		p.sortColumn = column
		p.applyFilter()
		if p.rows[0].Name != want {
			t.Fatalf("column %d: got %s, want %s", column, p.rows[0].Name, want)
		}
	}
	p.filter.SetText("  ALP  ")
	p.applyFilter()
	if len(p.rows) != 1 || p.rows[0].Name != "Alpha" {
		t.Fatal("name filter failed")
	}
	old := make(chan factionLoadResult, 1)
	p.pending = old
	p.Reset()
	old <- factionLoadResult{stats: []SessionFactionDetails{{Name: "stale"}}}
	p.receiveData()
	if p.loaded || p.rows != nil || p.filter.Text() != "" {
		t.Fatal("reset accepted stale data")
	}
	p.pending = make(chan factionLoadResult, 1)
	p.pending <- factionLoadResult{stats: []SessionFactionDetails{}}
	p.receiveData()
	if !p.loaded || p.rows == nil {
		t.Fatal("empty data did not finish loading")
	}
}
