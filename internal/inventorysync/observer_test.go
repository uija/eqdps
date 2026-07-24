package inventorysync

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/uija/eqdps/internal/eqlog"
)

func TestObserverCorrelatesWhoAndExport(t *testing.T) {
	root := t.TempDir()
	logPath := filepath.Join(root, "Logs", "eqlog_Wyrmberg_rivervale.txt")
	observer, err := NewObserver(logPath)
	if err != nil {
		t.Fatal(err)
	}
	whoTime := time.Date(2026, 7, 22, 5, 21, 0, 0, time.UTC)
	observer.Observe(eqlog.Record{
		Kind: eqlog.RecordWho,
		Who: eqlog.WhoResult{
			Time:    whoTime,
			Level:   50,
			Classes: []string{"PAL", "DRU", "MNK"},
			Name:    "Wyrmberg",
			Race:    "Ancient Wolf",
		},
	})
	request, ok := observer.Observe(eqlog.Record{
		Kind: eqlog.RecordInventoryExport,
		Export: eqlog.InventoryExport{
			Time:     whoTime.Add(27 * time.Second),
			Filename: "Wyrmberg_rivervale-Inventory.txt",
		},
	})
	if !ok {
		t.Fatal("expected export request")
	}
	wantPath := filepath.Join(root, "Wyrmberg_rivervale-Inventory.txt")
	if request.Path != wantPath || request.Metadata == nil {
		t.Fatalf("unexpected request: %#v", request)
	}
	if request.Metadata.Level != 50 || request.Metadata.Race != "Ancient Wolf" {
		t.Fatalf("unexpected metadata: %#v", request.Metadata)
	}
	if got := request.Metadata.Classes; len(got) != 3 || got[0] != "PAL" || got[2] != "MNK" {
		t.Fatalf("unexpected classes: %v", got)
	}
}

func TestObserverIgnoresOtherWhoResults(t *testing.T) {
	observer, err := NewObserver(filepath.Join(t.TempDir(), "Logs", "eqlog_Wyrmberg_rivervale.txt"))
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	observer.Observe(eqlog.Record{
		Kind: eqlog.RecordWho,
		Who: eqlog.WhoResult{
			Time: now, Level: 50, Classes: []string{"WAR", "CLR", "BRD"}, Name: "Karthar", Race: "Dwarf",
		},
	})
	request, ok := observer.Observe(eqlog.Record{
		Kind:   eqlog.RecordInventoryExport,
		Export: eqlog.InventoryExport{Time: now.Add(time.Second), Filename: "Wyrmberg_rivervale-Inventory.txt"},
	})
	if !ok || request.Metadata != nil {
		t.Fatalf("unexpected request: %#v, %t", request, ok)
	}
}

func TestObserverRejectsStaleWho(t *testing.T) {
	observer, err := NewObserver(filepath.Join(t.TempDir(), "Logs", "eqlog_Wyrmberg_rivervale.txt"))
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	observer.Observe(eqlog.Record{
		Kind: eqlog.RecordWho,
		Who:  eqlog.WhoResult{Time: now, Level: 50, Classes: []string{"PAL", "DRU", "MNK"}, Name: "Wyrmberg", Race: "Dwarf"},
	})
	request, ok := observer.Observe(eqlog.Record{
		Kind:   eqlog.RecordInventoryExport,
		Export: eqlog.InventoryExport{Time: now.Add(WhoMaxAge + time.Second), Filename: "Wyrmberg_rivervale-Inventory.txt"},
	})
	if !ok || request.Metadata != nil {
		t.Fatalf("unexpected request: %#v, %t", request, ok)
	}
}

func TestObserverAnonymousWhoClearsVisibleMetadata(t *testing.T) {
	observer, err := NewObserver(filepath.Join(t.TempDir(), "Logs", "eqlog_Wyrmberg_rivervale.txt"))
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	observer.Observe(eqlog.Record{
		Kind: eqlog.RecordWho,
		Who:  eqlog.WhoResult{Time: now, Level: 50, Classes: []string{"PAL", "DRU", "MNK"}, Name: "Wyrmberg", Race: "Dwarf"},
	})
	observer.Observe(eqlog.Record{
		Kind: eqlog.RecordWho,
		Who:  eqlog.WhoResult{Time: now.Add(time.Second), Name: "Wyrmberg", Anonymous: true},
	})
	request, ok := observer.Observe(eqlog.Record{
		Kind:   eqlog.RecordInventoryExport,
		Export: eqlog.InventoryExport{Time: now.Add(2 * time.Second), Filename: "Wyrmberg_rivervale-Inventory.txt"},
	})
	if !ok || request.Metadata != nil {
		t.Fatalf("unexpected request: %#v, %t", request, ok)
	}
}

func TestObserverOnlyAcceptsMatchingDefaultExport(t *testing.T) {
	observer, err := NewObserver(filepath.Join(t.TempDir(), "Logs", "eqlog_Wyrmberg_rivervale.txt"))
	if err != nil {
		t.Fatal(err)
	}
	for _, filename := range []string{
		"Karthar_rivervale-Inventory.txt",
		"Wyrmberg_rivervale-Inventory_1.txt",
		"../Wyrmberg_rivervale-Inventory.txt",
	} {
		if request, ok := observer.Observe(eqlog.Record{
			Kind:   eqlog.RecordInventoryExport,
			Export: eqlog.InventoryExport{Time: time.Now(), Filename: filename},
		}); ok {
			t.Fatalf("accepted %q: %#v", filename, request)
		}
	}
}
