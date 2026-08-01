package main

import "testing"

func TestSkyInventoryDifferencesIncludesMissingItemsAndIgnoresRunes(t *testing.T) {
	differences := skyInventoryDifferences(
		map[string]int{"Brass Knuckles": 1, "Light Woolen Mask": 2, "Wind Rune Caza": 4},
		map[string]int{"Brass Knuckles": 2, "Wind Rune Caza": 1},
	)
	if len(differences) != 2 {
		t.Fatalf("differences = %#v", differences)
	}
	if differences[0] != (skyInventoryCompareValue{Item: "Brass Knuckles", Tracked: 1, Exported: 2}) {
		t.Fatalf("first difference = %#v", differences[0])
	}
	if differences[1] != (skyInventoryCompareValue{Item: "Light Woolen Mask", Tracked: 2, Exported: 0}) {
		t.Fatalf("second difference = %#v", differences[1])
	}
}
