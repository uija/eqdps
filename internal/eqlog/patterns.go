package eqlog

import (
	"regexp"

	"github.com/uija/eqdps/internal/data"
)

const timestampLayout = "Mon Jan 02 15:04:05 2006"

var envelopeRE = regexp.MustCompile(`^\[([^\]]+)\] (.*)$`)
var levelUpExpression = regexp.MustCompile(`^You have gained a level! Welcome to level ([0-9]+)!$`)
var zoneChangeExpression = regexp.MustCompile(`^You have entered (.+)\.$`)

type eventPattern struct {
	eventType  data.LogRowEventType
	expression *regexp.Regexp
}

var eventPatterns = []eventPattern{
	{data.LogRowEventTypeCast, regexp.MustCompile(`^(.+?) (?:begin|begins) (?:casting|to cast) (.+)\.$`)},
	{data.LogRowEventTypeDamage, regexp.MustCompile(`^(.+?) (backstab|backstabs|bash|bashes|bite|bites|cleave|cleaves|claw|claws|crush|crushes|frenzy on|frenzies on|hit|hits|kick|kicks|maul|mauls|pierce|pierces|punch|punches|reave|reaves|shoot|shoots|slash|slashes|slice|slices|smash|smashes|smite|smites|strike|strikes) (.+?) for ([0-9]+) points? of ((?:[A-Za-z-]+ )?damage)(?: by ([^.]+))?\.(?: \(([^)]+)\))?$`)},
	{data.LogRowEventTypeYourDamageShield, regexp.MustCompile(`^(.+?) is .+? by YOUR (.+?) for ([0-9]+) points? of ((?:[A-Za-z-]+ )?damage)\.(?: \(([^)]+)\))?$`)},
	{data.LogRowEventTypeDamageShield, regexp.MustCompile("^(.+?) (?:is|are) .+? by (.+)(?:'s|`s) (.+?) for ([0-9]+) points? of ((?:[A-Za-z-]+ )?damage)[.!](?: \\(([^)]+)\\))?$")},
	{data.LogRowEventTypeYourDamageOverTime, regexp.MustCompile(`^(.+?) has taken ([0-9]+) damage from your (.+?)\.(?: \(([^)]+)\))?$`)},
	{data.LogRowEventTypeDamageOverTime, regexp.MustCompile(`^(.+?) (?:has|have) taken ([0-9]+) damage from (.+?) by ([^.]+)\.(?: \(([^)]+)\))?$`)},
	{data.LogRowEventTypeExperience, regexp.MustCompile(`^You gain experience! \(([0-9]+(?:\.[0-9]+)?)%\)$`)},
	{data.LogRowEventTypeKillExperienceReward, regexp.MustCompile(`^You gain (?:party )?experience!(?: \([0-9]+(?:\.[0-9]+)?%\))?$`)},
	{data.LogRowEventTypeCorpseCoinReward, regexp.MustCompile(`^You receive .+ from the corpse\.$`)},
	{data.LogRowEventTypeLevelUp, levelUpExpression},
	{data.LogRowEventTypeAggroClear, regexp.MustCompile(`^Your enemies have forgotten you!$`)},
	{data.LogRowEventTypeYouSlain, regexp.MustCompile(`^You have slain (.+)!$`)},
	{data.LogRowEventTypeSlainBy, regexp.MustCompile(`^(.+) has been slain by (.+)!$`)},
	{data.LogRowEventTypeZoneChange, zoneChangeExpression},
	{data.LogRowEventTypeLoot, regexp.MustCompile(`^--You have looted ((?:a|an|[0-9]+) .+) from (.+)'s corpse\.--$`)},
	{data.LogRowEventTypeLootResult, regexp.MustCompile(`^You looted ((?:a|an|[0-9]+) .+) from (.+)'s corpse (and sold it for .+\.|and stored it in .+|to create (.+))$`)},
	{data.LogRowEventTypeItemDestroyed, regexp.MustCompile(`^You successfully destroyed ([0-9]+) (.+)\.$`)},
	{data.LogRowEventTypeTradeOffer, regexp.MustCompile(`^You offered ([0-9]+) (.+) to (.+)\.$`)},
	{data.LogRowEventTypeTradeComplete, regexp.MustCompile(`^You complete the trade with (.+)\.$`)},
	{data.LogRowEventTypeTradeCancel, regexp.MustCompile(`^(You have|(.+) has) cancelled the trade\.$`)},
	{data.LogRowEventTypeWho, regexp.MustCompile(`^\s*(?:AFK\s+)?\[([0-9]+) ([A-Z]{3}(?:/[A-Z]{3}){0,2})\]\s+([^(]+?)\s+\(([^)]+)\)(?:\s+<([^>]+)>)?\s+ZONE:`)},
	{data.LogRowEventTypeAnonymousWho, regexp.MustCompile(`^\s*(?:AFK\s+)?\[ANONYMOUS\]\s+(.+?)\s*$`)},
	{data.LogRowEventTypeInventoryExport, regexp.MustCompile(`^Outputfile Complete:\s+(.+-Inventory\.txt)\s*$`)},
}

func classify(message string) (data.LogRowEventType, []string) {
	for _, pattern := range eventPatterns {
		if matches := pattern.expression.FindStringSubmatch(message); matches != nil {
			return pattern.eventType, matches
		}
	}
	return data.LogRowEventTypeUnknown, nil
}
