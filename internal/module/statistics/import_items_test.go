package statistics

import (
	"testing"

	"github.com/uija/eqdps/internal/data"
)

func TestImportTracksMerchantSalesAndDestroyedItems(t *testing.T) {
	m, db := newSessionTestModule(t)
	observedAt := sessionTestTime(t, "2026-08-28 12:00:00")

	sessionTestRow(
		t,
		m,
		observedAt,
		"You receive 2 gold 2 silver 4 copper from Grop for the Rusty Dagger +3(s).",
		data.LogRowEventTypeMerchantSale,
		"",
		"2 gold 2 silver 4 copper",
		"Grop",
		"Rusty Dagger +3",
	)
	sessionTestRow(
		t,
		m,
		observedAt,
		"You successfully destroyed 2 Rusty Dagger +2.",
		data.LogRowEventTypeItemDestroyed,
		"",
		"2",
		"Rusty Dagger +2",
	)
	if err := m.activeImport.Commit(0, observedAt); err != nil {
		t.Fatalf("commit import: %v", err)
	}

	items, err := GetItemStatistics(db)
	if err != nil {
		t.Fatalf("get item statistics: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("got %d item rows, want 1", len(items))
	}
	if items[0].Name != "Rusty Dagger" || items[0].Sold != 1 || items[0].Destroyed != 2 {
		t.Fatalf("unexpected item statistics: %#v", items[0])
	}

	var merchantCopper int64
	if err := db.QueryRow(`
		SELECT COALESCE(SUM(amount_copper), 0)
		FROM money
		WHERE source = 'merchant'
	`).Scan(&merchantCopper); err != nil {
		t.Fatalf("read merchant proceeds: %v", err)
	}
	if merchantCopper != 224 {
		t.Fatalf("got %d merchant copper, want 224", merchantCopper)
	}
}
