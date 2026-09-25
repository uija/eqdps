package events

import (
	"reflect"
	"testing"

	"github.com/uija/eqdps/internal/data"
	"github.com/uija/eqdps/internal/spellicon"
)

func TestRepairLegacySpellEvents(t *testing.T) {
	spells := []spellicon.Spell{
		{Name: "Allure", FadeMessageOthers: allureOthers},
		{Name: "Allure II", FadeMessageOthers: `^Your Allure II spell has worn off of .+\.$`},
		{Name: "Self only", FadeMessage: "Faded."},
	}
	for _, spell := range spells[:2] {
		events := []data.EventConfig{{Type: data.EventTypeSpell, Spell: spell.Name, Active: false, Title: "My event"}}
		if !repairLegacySpellEvents(events, spells) {
			t.Fatal("missing repair")
		}
		e := events[0]
		if e.ExpressionOthers != spell.FadeMessageOthers || e.Expression != "" || e.Spell != spell.Name || e.Active || e.Title != "My event" {
			t.Fatalf("unexpected repaired event: %+v", e)
		}
		prepareEventPatterns(&e)
		if !matchesEvent(e, "Your "+spell.Name+" spell has worn off of an abhorrent.") {
			t.Fatal("repaired event does not match")
		}
		if repairLegacySpellEvents(events, spells) {
			t.Fatal("repair is not idempotent")
		}
	}
	for _, e := range []data.EventConfig{
		{Type: data.EventTypeSpell, Spell: "Allure", Expression: allureSelf},
		{Type: data.EventTypeSpell, Spell: "Allure", ExpressionOthers: "custom"},
		{Type: data.EventTypeSpell, Spell: "Allure", Expression: allureSelf, ExpressionOthers: allureOthers},
		{Type: data.EventTypeSpell, Spell: "Unknown"},
		{Type: data.EventTypeSpell, Spell: "Self only"},
		{Type: data.EventTypeSpell},
		{Type: data.EventTypeTimer, Spell: "Allure"},
		{Type: data.EventTypeString, Spell: "Allure"},
		{Type: data.EventTypeRegexp, Spell: "Allure"},
	} {
		events := []data.EventConfig{e}
		if repairLegacySpellEvents(events, spells) || !reflect.DeepEqual(events[0], e) {
			t.Fatalf("changed unrelated event: %+v", e)
		}
	}
}

func TestUpgradeSavedUnrankedPattern(t *testing.T) {
	spells, err := spellicon.Load()
	if err != nil {
		t.Fatal(err)
	}
	for _, self := range []string{"", allureSelf} {
		events := []data.EventConfig{{Type: data.EventTypeSpell, Spell: "Allure", Expression: self, ExpressionOthers: `^Your Allure spell has worn off of .+\.$`}}
		if !repairLegacySpellEvents(events, spells) {
			t.Fatal("legacy generated pattern was not upgraded")
		}
		if events[0].Expression != self {
			t.Fatal("changed Self selection")
		}
		prepareEventPatterns(&events[0])
		if !matchesEvent(events[0], "Your Allure V spell has worn off of a desert tarantula.") {
			t.Fatal("ranked log message did not match")
		}
		if repairLegacySpellEvents(events, spells) {
			t.Fatal("upgrade not idempotent")
		}
	}
}
