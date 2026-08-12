package eqlog

import (
	"regexp"
	"strings"

	"github.com/uija/eqdps/internal/data"
)

const timestampLayout = "Mon Jan 02 15:04:05 2006"

var levelUpExpression = regexp.MustCompile(`^You have gained a level! Welcome to level ([0-9]+)!$`)
var zoneChangeExpression = regexp.MustCompile(`^You have entered (.+)\.$`)

type eventPattern struct {
	eventType  data.LogRowEventType
	expression *regexp.Regexp
	contains   []string
}

var eventPatterns = []eventPattern{
	{data.LogRowEventTypeCast, regexp.MustCompile(`^(.+?) (?:begin|begins) (?:casting|to cast) (.+)\.$`), []string{" begin", " cast"}},
	{data.LogRowEventTypeDamage, regexp.MustCompile(`^(.+?) (backstab|backstabs|bash|bashes|bite|bites|cleave|cleaves|claw|claws|crush|crushes|frenzy on|frenzies on|hit|hits|kick|kicks|maul|mauls|pierce|pierces|punch|punches|reave|reaves|shoot|shoots|slash|slashes|slice|slices|smash|smashes|smite|smites|strike|strikes) (.+?) for ([0-9]+) points? of ((?:[A-Za-z-]+ )?damage)(?: by ([^.]+))?\.(?: \(([^)]+)\))?$`), []string{" for ", " point", "damage"}},
	{data.LogRowEventTypeYourDamageShield, regexp.MustCompile(`^(.+?) is .+? by YOUR (.+?) for ([0-9]+) points? of ((?:[A-Za-z-]+ )?damage)\.(?: \(([^)]+)\))?$`), []string{" by YOUR ", " for ", " point", "damage"}},
	{data.LogRowEventTypeDamageShield, regexp.MustCompile("^(.+?) (?:is|are) .+? by (.+)(?:'s|`s) (.+?) for ([0-9]+) points? of ((?:[A-Za-z-]+ )?damage)[.!](?: \\(([^)]+)\\))?$"), []string{" by ", " for ", " point", "damage"}},
	{data.LogRowEventTypeYourDamageOverTime, regexp.MustCompile(`^(.+?) has taken ([0-9]+) damage from your (.+?)\.(?: \(([^)]+)\))?$`), []string{" has taken ", " damage from your "}},
	{data.LogRowEventTypeDamageOverTime, regexp.MustCompile(`^(.+?) (?:has|have) taken ([0-9]+) damage from (.+?) by ([^.]+)\.(?: \(([^)]+)\))?$`), []string{" taken ", " damage from ", " by "}},
	{data.LogRowEventTypeExperience, regexp.MustCompile(`^You gain experience! \(([0-9]+(?:\.[0-9]+)?)%\)$`), []string{"You gain experience!", "%"}},
	{data.LogRowEventTypeKillExperienceReward, regexp.MustCompile(`^You gain (?:party )?experience!(?: \([0-9]+(?:\.[0-9]+)?%\))?$`), []string{"You gain ", "experience!"}},
	{data.LogRowEventTypeCorpseCoinReward, regexp.MustCompile(`^You receive .+ from the corpse\.$`), []string{"You receive ", " from the corpse."}},
	{data.LogRowEventTypeLevelUp, levelUpExpression, []string{"You have gained a level! Welcome to level "}},
	{data.LogRowEventTypeAggroClear, regexp.MustCompile(`^Your enemies have forgotten you!$`), []string{"Your enemies have forgotten you!"}},
	{data.LogRowEventTypeYouSlain, regexp.MustCompile(`^You have slain (.+)!$`), []string{"You have slain ", "!"}},
	{data.LogRowEventTypeSlainBy, regexp.MustCompile(`^(.+) has been slain by (.+)!$`), []string{" has been slain by ", "!"}},
	{data.LogRowEventTypeZoneChange, zoneChangeExpression, []string{"You have entered "}},
	{data.LogRowEventTypeLoot, regexp.MustCompile(`^--You have looted ((?:a|an|[0-9]+) .+) from (.+)'s corpse\.--$`), []string{"--You have looted ", " from ", "'s corpse.--"}},
	{data.LogRowEventTypeLootResult, regexp.MustCompile(`^You looted ((?:a|an|[0-9]+) .+) from (.+)'s corpse (and sold it for .+\.|and stored it in .+|to create (.+))$`), []string{"You looted ", " from ", "'s corpse "}},
	{data.LogRowEventTypeItemDestroyed, regexp.MustCompile(`^You successfully destroyed ([0-9]+) (.+)\.$`), []string{"You successfully destroyed "}},
	{data.LogRowEventTypeTradeOffer, regexp.MustCompile(`^You offered ([0-9]+) (.+) to (.+)\.$`), []string{"You offered ", " to "}},
	{data.LogRowEventTypeTradeComplete, regexp.MustCompile(`^You complete the trade with (.+)\.$`), []string{"You complete the trade with "}},
	{data.LogRowEventTypeTradeCancel, regexp.MustCompile(`^(You have|(.+) has) cancelled the trade\.$`), []string{" cancelled the trade."}},
	{data.LogRowEventTypeTradeRejected, regexp.MustCompile(`^(.+?) says, 'I have no need for this, .+?\. You can have it back\.'$`), []string{" says, 'I have no need for this, ", " You can have it back.'"}},
	{data.LogRowEventTypeWho, regexp.MustCompile(`^\s*(?:AFK\s+)?\[([0-9]+) ([A-Z]{3}(?:/[A-Z]{3}){0,2})\]\s+([^(]+?)\s+\(([^)]+)\)(?:\s+<([^>]+)>)?\s+ZONE:`), []string{"ZONE:"}},
	{data.LogRowEventTypeAnonymousWho, regexp.MustCompile(`^\s*(?:AFK\s+)?\[ANONYMOUS\]\s+(.+?)\s*$`), []string{"[ANONYMOUS]"}},
	{data.LogRowEventTypeInventoryExport, regexp.MustCompile(`^Outputfile Complete:\s+(.+-Inventory\.txt)\s*$`), []string{"Outputfile Complete:", "-Inventory.txt"}},
}

func classify(message string) (data.LogRowEventType, []string, bool) {
	for _, pattern := range eventPatterns {
		containsAll := true
		for _, required := range pattern.contains {
			if !strings.Contains(message, required) {
				containsAll = false
				break
			}
		}
		if !containsAll {
			continue
		}
		if matches := pattern.expression.FindStringSubmatch(message); matches != nil {
			return pattern.eventType, matches, true
		}
	}
	return data.LogRowEventTypeUnknown, nil, false
}
