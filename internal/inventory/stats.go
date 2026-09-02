package inventory

import (
	"fmt"
	"html"
	"math"
	"regexp"
	"strconv"
	"strings"
)

var (
	wikiLineBreakRE = regexp.MustCompile(`(?i)<br\s*/?>`)
	wikiPipedLinkRE = regexp.MustCompile(`\[\[[^\]|]+\|([^\]]+)\]\]`)
	wikiLinkRE      = regexp.MustCompile(`\[\[([^\]]+)\]\]`)
	wikiTemplateRE  = regexp.MustCompile(`\{\{[^}]+\}\}`)
	htmlTagRE       = regexp.MustCompile(`<[^>]+>`)
	itemStatRE      = regexp.MustCompile(`(?i)\b(HP REGEN|SV DISEASE|SV POISON|SV MAGIC|SV COLD|SV FIRE|DAMAGE|MANA|ENDUR|HASTE|WEIGHT|DMG|AC|HP|MP|END|STR|STA|AGI|DEX|WIS|INT|CHA|MAGIC|FIRE|COLD|POISON|DISEASE|WT)\b(:[ \t]*)([+\-]?)(\d+(?:\.\d+)?)([ \t]*%?)`)
	weaponDelayRE   = regexp.MustCompile(`(?i)\b(?:ATK|ATTACK)[ \t]+DELAY:[ \t]*(\d+(?:\.\d+)?)`)
)

var itemStatAliases = map[string]string{
	"DAMAGE":     "DMG",
	"MANA":       "MP",
	"ENDUR":      "END",
	"SV MAGIC":   "SV_MAGIC",
	"SV FIRE":    "SV_FIRE",
	"SV COLD":    "SV_COLD",
	"SV POISON":  "SV_POISON",
	"SV DISEASE": "SV_DISEASE",
	"MAGIC":      "SV_MAGIC",
	"FIRE":       "SV_FIRE",
	"COLD":       "SV_COLD",
	"POISON":     "SV_POISON",
	"DISEASE":    "SV_DISEASE",
	"HP REGEN":   "HP_REGEN",
	"WEIGHT":     "WT",
}

var primaryItemStats = map[string]bool{
	"AC": true, "HP": true, "MP": true, "END": true,
	"STR": true, "STA": true, "AGI": true, "DEX": true,
	"WIS": true, "INT": true, "CHA": true,
	"SV_MAGIC": true, "SV_FIRE": true, "SV_COLD": true,
	"SV_POISON": true, "SV_DISEASE": true,
}

var filterableItemStats = map[string]bool{
	"AC": true, "HP": true, "MP": true, "END": true,
	"STR": true, "STA": true, "AGI": true, "DEX": true,
	"WIS": true, "INT": true, "CHA": true,
	"SV_MAGIC": true, "SV_FIRE": true, "SV_COLD": true,
	"SV_POISON": true, "SV_DISEASE": true,
	"HP_REGEN": true, "HASTE": true,
}

// GetStats returns the filterable stats at the supplied item level. RATIO is
// level-adjusted weapon damage divided by attack delay. Stats not present in
// the catalogue stats block are omitted from the result.
func (item ItemData) GetStats(level int) map[string]float64 {
	statsBlock := cleanWikiText(metadataString(item.Metadata, "statsblock"))
	if statsBlock == "" {
		return nil
	}
	level = max(0, min(level, 10))
	result := make(map[string]float64)
	var weaponDamage float64
	for _, parts := range itemStatRE.FindAllStringSubmatch(statsBlock, -1) {
		if len(parts) != 6 {
			continue
		}
		key := strings.ToUpper(parts[1])
		if alias, found := itemStatAliases[key]; found {
			key = alias
		}
		base, err := strconv.ParseFloat(parts[4], 64)
		if err != nil {
			continue
		}
		if parts[3] == "-" {
			base *= -1
		}
		value := tieredItemStat(key, base, level)
		if key == "DMG" {
			weaponDamage = value
		}
		if filterableItemStats[key] {
			result[key] = math.Round(value)
		}
	}
	if delayParts := weaponDelayRE.FindStringSubmatch(statsBlock); len(delayParts) == 2 && weaponDamage > 0 {
		if delay, err := strconv.ParseFloat(delayParts[1], 64); err == nil && delay > 0 {
			result["RATIO"] = weaponDamage / delay
		}
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

// GetStatsBlock returns the item's catalogue stats as plain text with the
// EverQuest Legends tier adjustments applied. Level is clamped to 0 through 10.
func (item ItemData) GetStatsBlock(level int) string {
	stats := cleanWikiText(metadataString(item.Metadata, "statsblock"))
	if stats == "" {
		return ""
	}
	level = max(0, min(level, 10))

	return itemStatRE.ReplaceAllStringFunc(stats, func(match string) string {
		parts := itemStatRE.FindStringSubmatch(match)
		if len(parts) != 6 {
			return match
		}

		base, err := strconv.ParseFloat(parts[4], 64)
		if err != nil {
			return match
		}
		if parts[3] == "-" {
			base *= -1
		}

		key := strings.ToUpper(parts[1])
		if alias, found := itemStatAliases[key]; found {
			key = alias
		}
		value := tieredItemStat(key, base, level)
		formatted := formatItemStat(key, value)
		if value > 0 && parts[3] == "+" {
			formatted = "+" + formatted
		}
		return parts[1] + parts[2] + formatted + parts[5]
	})
}

func tieredItemStat(key string, base float64, level int) float64 {
	tier := float64(level)
	switch {
	case primaryItemStats[key]:
		switch {
		case base > 0 && base <= 10:
			return base + tier
		case base > 10:
			increase := math.Floor(base*tier/10 + 0.5)
			return math.Floor(base + increase)
		case base < 0:
			return min(0, base+tier)
		}
	case key == "DMG" && base > 0:
		return base + math.Floor(base*tier/10)
	case (key == "HP_REGEN" || key == "HASTE") && base > 0:
		return base + tier
	case key == "WT" && base > 0.1:
		return max(0, math.Ceil(base*(1-0.09*tier)*10-1e-9)/10)
	}
	return base
}

func formatItemStat(key string, value float64) string {
	if key == "WT" {
		return fmt.Sprintf("%.1f", value)
	}
	if math.Abs(value-math.Round(value)) < 0.0000001 {
		return strconv.FormatInt(int64(math.Round(value)), 10)
	}
	return strings.TrimRight(strings.TrimRight(fmt.Sprintf("%.3f", value), "0"), ".")
}

func cleanWikiText(value string) string {
	value = wikiLineBreakRE.ReplaceAllString(value, "\n")
	value = wikiPipedLinkRE.ReplaceAllString(value, "$1")
	value = wikiLinkRE.ReplaceAllString(value, "$1")
	value = wikiTemplateRE.ReplaceAllString(value, "")
	value = htmlTagRE.ReplaceAllString(value, "")
	value = strings.ReplaceAll(value, "'''", "")
	value = strings.ReplaceAll(value, "''", "")
	value = html.UnescapeString(value)

	lines := strings.Split(strings.ReplaceAll(value, "\r", "\n"), "\n")
	result := make([]string, 0, len(lines))
	for _, line := range lines {
		if line = strings.TrimSpace(line); line != "" {
			result = append(result, line)
		}
	}
	return strings.Join(result, "\n")
}
