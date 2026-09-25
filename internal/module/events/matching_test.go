package events

import (
	"regexp"
	"testing"

	"github.com/uija/eqdps/internal/data"
	"github.com/uija/eqdps/internal/module"
	"github.com/uija/eqdps/internal/spellicon"
)

const allureSelf = "You are no longer charmed."

var allureOthers = spellicon.FadeMessageOthersPattern("Allure")

func TestSpellTargetsSaveAndMatch(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	for _, target := range []string{"Self", "Others", "Both"} {
		t.Run(target, func(t *testing.T) {
			m := NewModule()
			m.ctx = &module.Context{Config: &data.Config{}}
			m.spells = []spellicon.Spell{{Name: "Allure", Classes: []string{"Enchanter"}, FadeMessage: allureSelf, FadeMessageOthers: allureOthers}}
			m.create_type = data.EventTypeSpell
			m.UpdateSpellsAndClasses()
			m.spell_select.Select("Allure")
			m.target_select.Select(target)
			m.OnSave()
			e := m.ctx.Config.Events[0]
			prepareEventPatterns(&e)
			for _, tc := range []struct {
				message string
				want    bool
			}{
				{allureSelf, target != "Others"},
				{"Your Allure spell has worn off of an abhorrent.", target != "Self"},
				{"Unrelated message.", false},
				{"", false},
				{"Your Allure II spell has worn off of an abhorrent.", target != "Self"},
				{"Your Allure V spell has worn off of a desert tarantula.", target != "Self"},
				{"Your Allure V spell is interrupted.", false},
			} {
				if got := matchesEvent(e, tc.message); got != tc.want {
					t.Errorf("match %q = %v, want %v", tc.message, got, tc.want)
				}
			}
			m.SelectToEdit(0)
			if got := m.target_select.Value(); got != target {
				t.Fatalf("reopened target = %q, want %q", got, target)
			}
			m.ctx.Config.Events[0].RegExp = regexp.MustCompile("old")
			m.ctx.Config.Events[0].RegExpOthers = regexp.MustCompile("old")
			m.OnSave()
			if e := m.ctx.Config.Events[0]; e.RegExp != nil || e.RegExpOthers != nil {
				t.Fatal("save retained cached patterns")
			}
		})
	}
}

func TestEmptyExpressionsNeverMatch(t *testing.T) {
	for _, typ := range []data.EventType{data.EventTypeString, data.EventTypeSpell, data.EventTypeRegexp} {
		for _, full := range []bool{false, true} {
			e := data.EventConfig{Type: typ, FullExpression: full, RegExp: regexp.MustCompile(""), RegExpOthers: regexp.MustCompile("")}
			for _, message := range []string{"anything", ""} {
				if matchesEvent(e, message) {
					t.Errorf("empty type %v matched %q", typ, message)
				}
			}
		}
	}
}

func TestPatternChangesAndInvalidPatterns(t *testing.T) {
	for _, typ := range []data.EventType{data.EventTypeRegexp, data.EventTypeSpell} {
		e := data.EventConfig{Type: typ, ExpressionOthers: "old"}
		prepareEventPatterns(&e)
		e.ExpressionOthers = "new"
		prepareEventPatterns(&e)
		if matchesEvent(e, "old") || !matchesEvent(e, "new") {
			t.Fatal("changed pattern not applied")
		}
		for _, expression := range []string{"[", ""} {
			e.ExpressionOthers = expression
			prepareEventPatterns(&e)
			if e.RegExpOthers != nil || matchesEvent(e, "new") {
				t.Fatal("invalid or cleared pattern still matches")
			}
		}
	}
	e := data.EventConfig{Type: data.EventTypeRegexp, Expression: "old"}
	prepareEventPatterns(&e)
	e.Expression = "["
	prepareEventPatterns(&e)
	if e.RegExp != nil || matchesEvent(e, "old") {
		t.Fatal("invalid self regex retained old match")
	}
}

func TestBothPatternsMatchOneDecision(t *testing.T) {
	e := data.EventConfig{Type: data.EventTypeSpell, Expression: "faded", ExpressionOthers: "faded"}
	prepareEventPatterns(&e)
	if !matchesEvent(e, "faded") {
		t.Fatal("both matching targets did not match")
	}
}
