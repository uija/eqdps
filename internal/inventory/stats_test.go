package inventory

import "testing"

func TestItemDataGetStatsBlockCleansWikiMarkup(t *testing.T) {
	item := ItemData{Metadata: map[string]any{
		"statsblock": "MAGIC ITEM<br>\nEffect: [[Haste|<span class='itemeff'>Haste</span>]] (Combat)<br>\nClass: ALL<br>",
	}}
	want := "MAGIC ITEM\nEffect: Haste (Combat)\nClass: ALL"
	if got := item.GetStatsBlock(0); got != want {
		t.Fatalf("GetStatsBlock(0) = %q, want %q", got, want)
	}
}

func TestItemDataGetStatsBlockAppliesTierValues(t *testing.T) {
	item := ItemData{Metadata: map[string]any{
		"statsblock": "AC: 3<br>HP: +30<br>SV MAGIC: -10<br>DMG: 6<br>Haste: 41%<br>WT: 5.0<br>",
	}}
	want := "AC: 5\nHP: +36\nSV MAGIC: -8\nDMG: 7\nHaste: 43%\nWT: 4.1"
	if got := item.GetStatsBlock(2); got != want {
		t.Fatalf("GetStatsBlock(2) = %q, want %q", got, want)
	}
}

func TestItemDataGetStatsBlockClampsTier(t *testing.T) {
	item := ItemData{Metadata: map[string]any{"statsblock": "AC: 3"}}
	if got := item.GetStatsBlock(20); got != "AC: 13" {
		t.Fatalf("GetStatsBlock(20) = %q, want %q", got, "AC: 13")
	}
}
