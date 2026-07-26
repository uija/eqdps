package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadSpellsFiltersClassesByMaximumLevel(t *testing.T) {
	fades, err := readFadeMessages(strings.NewReader(
		"#SPELLINDEX^CASTERMETXT^CASTEROTHERTXT^CASTEDMETXT^CASTEDOTHERTXT^SPELLGONE^\n" +
			"42^^^^^The effect fades.^\n",
	))
	if err != nil {
		t.Fatal(err)
	}

	fields := make([]string, requiredSpellFields)
	fields[spellIDField] = "42"
	fields[spellNameField] = "Example Spell"
	fields[newIconField] = "4"
	for i := range classNames {
		fields[firstClassField+i] = "255"
	}
	fields[firstClassField+9] = "50"  // Shaman
	fields[firstClassField+14] = "64" // Beastlord

	spells, err := readSpells(strings.NewReader(strings.Join(fields, "^")+"\n"), fades, 50)
	if err != nil {
		t.Fatal(err)
	}
	if len(spells) != 1 {
		t.Fatalf("got %d spells, want 1", len(spells))
	}
	if got, want := strings.Join(spells[0].Classes, ","), "Shaman"; got != want {
		t.Fatalf("classes = %q, want %q", got, want)
	}
	if got, want := spells[0].IconID, 4; got != want {
		t.Fatalf("icon ID = %d, want %d", got, want)
	}
}

func TestReadSpellsRequiresFadeAndEligibleClass(t *testing.T) {
	fields := make([]string, requiredSpellFields)
	fields[spellIDField] = "42"
	fields[spellNameField] = "Example Spell"
	fields[newIconField] = "4"
	for i := range classNames {
		fields[firstClassField+i] = "255"
	}

	spells, err := readSpells(
		strings.NewReader(strings.Join(fields, "^")+"\n"),
		map[string]string{"42": "The effect fades."},
		50,
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(spells) != 0 {
		t.Fatalf("got %d spells, want 0", len(spells))
	}

	fields[firstClassField] = "1"
	spells, err = readSpells(
		strings.NewReader(strings.Join(fields, "^")+"\n"),
		map[string]string{},
		50,
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(spells) != 0 {
		t.Fatalf("got %d spells without a fade message, want 0", len(spells))
	}
}

func TestRunWritesSharedCatalogueSchema(t *testing.T) {
	gameDir := t.TempDir()
	if err := os.WriteFile(
		filepath.Join(gameDir, "spells_us_str.txt"),
		[]byte("42^^^^^The effect fades.^\n"),
		0o600,
	); err != nil {
		t.Fatal(err)
	}
	fields := make([]string, requiredSpellFields)
	fields[spellIDField] = "42"
	fields[spellNameField] = "Example Spell"
	fields[newIconField] = "7"
	for index := range classNames {
		fields[firstClassField+index] = "255"
	}
	fields[firstClassField+1] = "20"
	if err := os.WriteFile(
		filepath.Join(gameDir, "spells_us.txt"),
		[]byte(strings.Join(fields, "^")+"\n"),
		0o600,
	); err != nil {
		t.Fatal(err)
	}

	target := filepath.Join(t.TempDir(), "spells.json")
	if err := run([]string{"20", gameDir, target}); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	var generated []spell
	if err := json.Unmarshal(data, &generated); err != nil {
		t.Fatal(err)
	}
	if len(generated) != 1 ||
		generated[0].Name != "Example Spell" ||
		generated[0].FadeMessage != "The effect fades." ||
		generated[0].IconID != 7 {
		t.Fatalf("generated catalogue = %#v", generated)
	}
}
