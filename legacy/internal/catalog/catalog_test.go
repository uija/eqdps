package catalog

import "testing"

func TestLoadEmbeddedCatalogue(t *testing.T) {
	spells, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(spells) < 100 {
		t.Fatalf("catalogue contains only %d spells", len(spells))
	}
	for _, spell := range spells {
		if spell.Name == "Alacrity" {
			if spell.FadeMessage == "" {
				t.Fatal("Alacrity has no fade message")
			}
			return
		}
	}
	t.Fatal("embedded catalogue does not contain Alacrity")
}
