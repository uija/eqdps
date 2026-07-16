package main

import "testing"

func TestItemDropsExtractsNPCsAndOmitsZoneLink(t *testing.T) {
	wikitext := `{{Itempage
|dropsfrom =
[[Plane of Sky]]
* [[Eye of Veeshan]]
* [[Noble Dojorn]]
|relatedquests =
* [[Necromancer Plane of Sky Tests]]
}}`
	if got, want := itemDrops(wikitext), "Eye of Veeshan / Noble Dojorn"; got != want {
		t.Fatalf("itemDrops() = %q, want %q", got, want)
	}
}

func TestCanonicalQuestGiverUsesInGameMagicianNPCName(t *testing.T) {
	if got := canonicalQuestGiver("Magi Frinon"); got != "Magus Frinon" {
		t.Fatalf("canonicalQuestGiver() = %q, want Magus Frinon", got)
	}
}

func TestCanonicalQuestGiverUsesInGameBardAndRangerNPCNames(t *testing.T) {
	for input, want := range map[string]string{
		"Clarisa Spiritsong": "Cilin Spellsinger",
		"Denise Songweaver":  "Cilin Spellsinger",
		"the Ranger Spirit":  "Ranger Spirit",
	} {
		if got := canonicalQuestGiver(input); got != want {
			t.Errorf("canonicalQuestGiver(%q) = %q, want %q", input, got, want)
		}
	}
}
