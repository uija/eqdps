package spellicon

import (
	"regexp"
	"testing"
)

func TestCatalogueFadePatterns(t *testing.T) {
	spells, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, spell := range spells {
		if spell.FadeMessageOthers == "" {
			continue
		}
		count++
		if spell.FadeMessageOthers != FadeMessageOthersPattern(spell.Name) {
			t.Fatalf("outdated catalogue pattern for %s", spell.Name)
		}
		r := regexp.MustCompile(spell.FadeMessageOthers)
		for _, rank := range []string{"", " II", " V", " IX"} {
			for _, message := range []string{
				"Your " + spell.Name + rank + " spell has worn off of a desert tarantula.",
				"Your pet's " + spell.Name + rank + " spell has worn off.",
			} {
				if !r.MatchString(message) {
					t.Errorf("%s does not match %q", spell.Name, message)
				}
			}
			if r.MatchString("Your " + spell.Name + rank + " spell is interrupted.") {
				t.Errorf("%s matched interruption", spell.Name)
			}
		}
		if r.MatchString("Your " + spell.Name + " Other spell has worn off of a mob.") {
			t.Errorf("%s matched a different spell", spell.Name)
		}
	}
	if count == 0 {
		t.Fatal("no patterns checked")
	}
}
