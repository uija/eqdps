package skyquest

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadInventoryExportCountsKnownVariantsAndIgnoresRunes(t *testing.T) {
	path := filepath.Join(t.TempDir(), "Wyrmberg_rivervale-Inventory.txt")
	content := "Location\tName\tID\tCount\tSlots\n" +
		"General 1\tLight Woolen Mask +3\t1\t1\t10\n" +
		"Bank1\tLight Woolen Mask (Exaltation)\t1\t2\t10\n" +
		"General 2\tWind Rune Caza\t2\t9\t10\n" +
		"General 3\tUnrelated Item\t3\t4\t10\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	items, err := ReadInventoryExport(path, testDatabase())
	if err != nil {
		t.Fatal(err)
	}
	if got := items["Light Woolen Mask"]; got != 1 {
		t.Fatalf("mask quantity = %d, want 1", got)
	}
	if _, found := items["Wind Rune Caza"]; found {
		t.Fatal("Wind Rune was included")
	}
}
