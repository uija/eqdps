package spellicon

import "regexp"

// FadeMessageOthersPattern accepts an optional upgrade rank while preserving
// the catalogue name, including numerals that are part of that name (Yaulp II).
func FadeMessageOthersPattern(name string) string {
	spell := regexp.QuoteMeta(name) + `(?: [IVXLCDM]+)?`
	return `^Your (?:` + spell + ` spell has worn off of .+|pet's ` + spell + ` spell has worn off)\.$`
}
