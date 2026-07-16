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
