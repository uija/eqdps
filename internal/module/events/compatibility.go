package events

import (
	"github.com/uija/eqdps/internal/data"
	"github.com/uija/eqdps/internal/spellicon"
	"regexp"
)

// repairLegacySpellEvents recovers Others events saved by the old "Other"
// versus "Others" typo and upgrades old generated patterns. Custom expressions
// are preserved.
func repairLegacySpellEvents(events []data.EventConfig, spells []spellicon.Spell) bool {
	changed := false
	for i := range events {
		event := &events[i]
		if event.Type != data.EventTypeSpell || event.Spell == "" {
			continue
		}
		for _, spell := range spells {
			// This is a catalogue identity lookup, not a rank-insensitive spell
			// comparison: preserve the exact configured spell and fade pattern.
			if spell.Name == event.Spell && spell.FadeMessageOthers != "" {
				legacy := `^Your ` + regexp.QuoteMeta(spell.Name) + ` spell has worn off of .+\.$`
				empty := event.Expression == "" && event.ExpressionOthers == ""
				if !empty && (event.ExpressionOthers != legacy || event.ExpressionOthers == spell.FadeMessageOthers) {
					break
				}
				event.ExpressionOthers = spell.FadeMessageOthers
				event.RegExp = nil
				event.RegExpOthers = nil
				changed = true
				break
			}
		}
	}
	return changed
}
