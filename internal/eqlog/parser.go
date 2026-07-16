package eqlog

import (
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/uija/eqdps/internal/combat"
)

const timestampLayout = "Mon Jan 02 15:04:05 2006"

var (
	lineRE = regexp.MustCompile(`^\[([^\]]+)\] (.*)$`)

	damageRE      = regexp.MustCompile(`^(.+?) (backstab|backstabs|bash|bashes|bite|bites|cleave|cleaves|claw|claws|crush|crushes|frenzy on|frenzies on|hit|hits|kick|kicks|maul|mauls|pierce|pierces|punch|punches|reave|reaves|shoot|shoots|slash|slashes|slice|slices|smash|smashes|smite|smites|strike|strikes) (.+?) for ([0-9]+) points? of ((?:[A-Za-z-]+ )?damage)(?: by ([^.]+))?\.(?: \(([^)]+)\))?$`)
	dotRE         = regexp.MustCompile(`^(.+?) (?:has|have) taken ([0-9]+) damage from (.+?) by ([^.]+)\.(?: \(([^)]+)\))?$`)
	yourDotRE     = regexp.MustCompile(`^(.+?) has taken ([0-9]+) damage from your (.+?)\.(?: \(([^)]+)\))?$`)
	yourShieldRE  = regexp.MustCompile(`^(.+?) is .+? by YOUR (.+?) for ([0-9]+) points? of ((?:[A-Za-z-]+ )?damage)\.(?: \(([^)]+)\))?$`)
	shieldRE      = regexp.MustCompile("^(.+?) (?:is|are) .+? by (.+)(?:'s|`s) (.+?) for ([0-9]+) points? of ((?:[A-Za-z-]+ )?damage)[.!](?: \\(([^)]+)\\))?$")
	youSlainRE    = regexp.MustCompile(`^You have slain (.+)!$`)
	slainByRE     = regexp.MustCompile(`^(.+) has been slain by (.+)!$`)
	experienceRE  = regexp.MustCompile(`^You gain experience! \(([0-9]+(?:\.[0-9]+)?)%\)$`)
	levelUpRE     = regexp.MustCompile(`^You have gained a level! Welcome to level ([0-9]+)!$`)
	aggroClearRE  = regexp.MustCompile(`^Your enemies have forgotten you!$`)
	castRE        = regexp.MustCompile(`^(.+?) (?:begin|begins) (?:casting|to cast) (.+)\.$`)
	zoneRE        = regexp.MustCompile(`^You have entered (.+)\.$`)
	lootRE        = regexp.MustCompile(`^--You have looted ((?:a|an|[0-9]+) .+) from (.+)'s corpse\.--$`)
	lootResultRE  = regexp.MustCompile(`^You looted ((?:a|an|[0-9]+) .+) from (.+)'s corpse (and sold it for .+\.|and stored it in .+|to create (.+))$`)
	destroyRE     = regexp.MustCompile(`^You successfully destroyed ([0-9]+) (.+)\.$`)
	tradeOfferRE  = regexp.MustCompile(`^You offered ([0-9]+) (.+) to (.+)\.$`)
	tradeDoneRE   = regexp.MustCompile(`^You complete the trade with (.+)\.$`)
	tradeCancelRE = regexp.MustCompile(`^(You have|(.+) has) cancelled the trade\.$`)
)

type ExperienceGain struct {
	Time    time.Time
	Percent float64
}

type LevelUp struct {
	Time  time.Time
	Level int
}

type ZoneChange struct {
	Time time.Time
	Name string
}

type LootOutcome uint8

const (
	LootRetained LootOutcome = iota + 1
	LootStored
	LootSold
	LootConverted
)

type Loot struct {
	Time     time.Time
	Item     string
	Corpse   string
	Quantity int
	Outcome  LootOutcome
	Created  string
}

type ItemRemoval struct {
	Time     time.Time
	Item     string
	Quantity int
}

type TradeOffer struct {
	Time     time.Time
	Item     string
	Quantity int
	NPC      string
}

type TradeComplete struct {
	Time time.Time
	NPC  string
}

type TradeCancel struct {
	Time time.Time
	NPC  string
}

type RecordKind uint8

const (
	RecordUnknown RecordKind = iota
	RecordCast
	RecordDamage
	RecordExperience
	RecordLevelUp
	RecordAggroClear
	RecordDeath
	RecordZoneChange
	RecordLoot
	RecordItemRemoval
	RecordTradeOffer
	RecordTradeComplete
	RecordTradeCancel
)

// Record is one timestamped EverQuest log entry. Unknown records retain their
// timestamp so session timing can observe log activity without reparsing lines.
type Record struct {
	Time        time.Time
	Kind        RecordKind
	Cast        combat.Cast
	Damage      combat.Event
	Experience  ExperienceGain
	LevelUp     LevelUp
	Death       combat.Death
	ZoneChange  ZoneChange
	Loot        Loot
	Removal     ItemRemoval
	TradeOffer  TradeOffer
	TradeDone   TradeComplete
	TradeCancel TradeCancel
}

func ParseRecord(line string) (Record, bool) {
	return ParseRecordAfter(line, time.Time{})
}

// ParseRecordAfter parses the timestamp envelope and only classifies messages
// at or after cutoff. This keeps short history replays from running every old
// log line through all combat and loot expressions.
func ParseRecordAfter(line string, cutoff time.Time) (Record, bool) {
	timestamp, message, ok := parseEnvelope(line)
	if !ok {
		return Record{}, false
	}
	if !cutoff.IsZero() && timestamp.Before(cutoff) {
		return Record{}, false
	}
	record := Record{Time: timestamp}
	if cast, ok := parseCast(timestamp, message); ok {
		record.Kind, record.Cast = RecordCast, cast
	} else if damage, ok := parseDamage(timestamp, message); ok {
		record.Kind, record.Damage = RecordDamage, damage
	} else if gain, ok := parseExperience(timestamp, message); ok {
		record.Kind, record.Experience = RecordExperience, gain
	} else if levelUp, ok := parseLevelUp(timestamp, message); ok {
		record.Kind, record.LevelUp = RecordLevelUp, levelUp
	} else if aggroClearRE.MatchString(message) {
		record.Kind = RecordAggroClear
	} else if death, ok := parseDeath(timestamp, message); ok {
		record.Kind, record.Death = RecordDeath, death
	} else if zone, ok := parseZoneChange(timestamp, message); ok {
		record.Kind, record.ZoneChange = RecordZoneChange, zone
	} else if loot, ok := parseLoot(timestamp, message); ok {
		record.Kind, record.Loot = RecordLoot, loot
	} else if removal, ok := parseItemRemoval(timestamp, message); ok {
		record.Kind, record.Removal = RecordItemRemoval, removal
	} else if offer, ok := parseTradeOffer(timestamp, message); ok {
		record.Kind, record.TradeOffer = RecordTradeOffer, offer
	} else if completed, ok := parseTradeComplete(timestamp, message); ok {
		record.Kind, record.TradeDone = RecordTradeComplete, completed
	} else if cancelled, ok := parseTradeCancel(timestamp, message); ok {
		record.Kind, record.TradeCancel = RecordTradeCancel, cancelled
	}
	return record, true
}

func parseTradeCancel(timestamp time.Time, message string) (TradeCancel, bool) {
	matches := tradeCancelRE.FindStringSubmatch(message)
	if matches == nil {
		return TradeCancel{}, false
	}
	return TradeCancel{Time: timestamp, NPC: strings.TrimSpace(matches[2])}, true
}

func parseTradeOffer(timestamp time.Time, message string) (TradeOffer, bool) {
	matches := tradeOfferRE.FindStringSubmatch(message)
	if matches == nil {
		return TradeOffer{}, false
	}
	quantity, err := strconv.Atoi(matches[1])
	if err != nil || quantity < 1 {
		return TradeOffer{}, false
	}
	return TradeOffer{Time: timestamp, Quantity: quantity, Item: strings.TrimSpace(matches[2]), NPC: strings.TrimSpace(matches[3])}, true
}

func parseTradeComplete(timestamp time.Time, message string) (TradeComplete, bool) {
	matches := tradeDoneRE.FindStringSubmatch(message)
	if matches == nil {
		return TradeComplete{}, false
	}
	return TradeComplete{Time: timestamp, NPC: strings.TrimSpace(matches[1])}, true
}

func ParseZoneChangeLine(line string) (ZoneChange, bool) {
	timestamp, message, ok := parseEnvelope(line)
	if !ok {
		return ZoneChange{}, false
	}
	return parseZoneChange(timestamp, message)
}

func parseZoneChange(timestamp time.Time, message string) (ZoneChange, bool) {
	matches := zoneRE.FindStringSubmatch(message)
	if matches == nil {
		return ZoneChange{}, false
	}
	return ZoneChange{Time: timestamp, Name: strings.TrimSpace(matches[1])}, true
}

func ParseLootLine(line string) (Loot, bool) {
	timestamp, message, ok := parseEnvelope(line)
	if !ok {
		return Loot{}, false
	}
	return parseLoot(timestamp, message)
}

func parseLoot(timestamp time.Time, message string) (Loot, bool) {
	if matches := lootRE.FindStringSubmatch(message); matches != nil {
		quantity, item, ok := parseLootItem(matches[1])
		if !ok {
			return Loot{}, false
		}
		return Loot{Time: timestamp, Item: item, Corpse: strings.TrimSpace(matches[2]), Quantity: quantity, Outcome: LootRetained}, true
	}

	matches := lootResultRE.FindStringSubmatch(message)
	if matches == nil {
		return Loot{}, false
	}
	quantity, item, ok := parseLootItem(matches[1])
	if !ok {
		return Loot{}, false
	}
	outcome := LootStored
	result := matches[3]
	if strings.HasPrefix(result, "and sold it for ") {
		outcome = LootSold
	} else if strings.HasPrefix(result, "to create ") {
		outcome = LootConverted
	}
	return Loot{
		Time: timestamp, Item: item, Corpse: strings.TrimSpace(matches[2]), Quantity: quantity,
		Outcome: outcome, Created: strings.TrimSpace(matches[4]),
	}, true
}

func parseLootItem(value string) (int, string, bool) {
	prefix, item, ok := strings.Cut(strings.TrimSpace(value), " ")
	if !ok || item == "" {
		return 0, "", false
	}
	quantity := 1
	if prefix != "a" && prefix != "an" {
		var err error
		quantity, err = strconv.Atoi(prefix)
		if err != nil || quantity < 1 {
			return 0, "", false
		}
	}
	return quantity, strings.TrimSpace(item), true
}

func ParseItemRemovalLine(line string) (ItemRemoval, bool) {
	timestamp, message, ok := parseEnvelope(line)
	if !ok {
		return ItemRemoval{}, false
	}
	return parseItemRemoval(timestamp, message)
}

func parseItemRemoval(timestamp time.Time, message string) (ItemRemoval, bool) {
	matches := destroyRE.FindStringSubmatch(message)
	if matches == nil {
		return ItemRemoval{}, false
	}
	quantity, err := strconv.Atoi(matches[1])
	if err != nil || quantity < 1 {
		return ItemRemoval{}, false
	}
	return ItemRemoval{Time: timestamp, Item: strings.TrimSpace(matches[2]), Quantity: quantity}, true
}

func ParseLine(line string) (combat.Event, bool) {
	timestamp, message, ok := parseEnvelope(line)
	if !ok {
		return combat.Event{}, false
	}
	return parseDamage(timestamp, message)
}

func parseDamage(timestamp time.Time, message string) (combat.Event, bool) {
	damage := damageRE.FindStringSubmatch(message)
	if damage != nil {
		source := strings.TrimSpace(damage[1])
		target := strings.TrimSpace(damage[3])

		amount, err := strconv.Atoi(damage[4])
		if err != nil {
			return combat.Event{}, false
		}

		return combat.Event{
			Time:       timestamp,
			Source:     normalizeSource(source),
			Target:     normalizeTarget(target),
			Amount:     amount,
			Attack:     strings.TrimSpace(damage[2]),
			Ability:    strings.TrimSpace(damage[6]),
			Critical:   isCritical(damage[7]),
			Incidental: isIncidentalDamage(damage[2], damage[7]),
		}, true
	}

	yourShield := yourShieldRE.FindStringSubmatch(message)
	if yourShield != nil {
		amount, err := strconv.Atoi(yourShield[3])
		if err != nil {
			return combat.Event{}, false
		}

		return combat.Event{
			Time:       timestamp,
			Source:     "You",
			Target:     normalizeTarget(strings.TrimSpace(yourShield[1])),
			Amount:     amount,
			Ability:    strings.TrimSpace(yourShield[2]),
			Critical:   isCritical(yourShield[5]),
			Passive:    true,
			Incidental: true,
		}, true
	}

	shield := shieldRE.FindStringSubmatch(message)
	if shield != nil {
		amount, err := strconv.Atoi(shield[4])
		if err != nil {
			return combat.Event{}, false
		}

		return combat.Event{
			Time:       timestamp,
			Source:     normalizeSource(strings.TrimSpace(shield[2])),
			Target:     normalizeTarget(strings.TrimSpace(shield[1])),
			Amount:     amount,
			Ability:    strings.TrimSpace(shield[3]),
			Critical:   isCritical(shield[6]),
			Passive:    true,
			Incidental: true,
		}, true
	}

	yourDot := yourDotRE.FindStringSubmatch(message)
	if yourDot != nil {
		amount, err := strconv.Atoi(yourDot[2])
		if err != nil {
			return combat.Event{}, false
		}

		return combat.Event{
			Time:           timestamp,
			Source:         "You",
			Target:         normalizeTarget(strings.TrimSpace(yourDot[1])),
			Amount:         amount,
			Ability:        strings.TrimSpace(yourDot[3]),
			Critical:       isCritical(yourDot[4]),
			Passive:        true,
			DamageOverTime: true,
		}, true
	}

	dot := dotRE.FindStringSubmatch(message)
	if dot == nil {
		return combat.Event{}, false
	}
	source := strings.TrimSpace(dot[4])
	target := strings.TrimSpace(dot[1])
	amount, err := strconv.Atoi(dot[2])
	if err != nil {
		return combat.Event{}, false
	}
	return combat.Event{
		Time:           timestamp,
		Source:         normalizeSource(source),
		Target:         normalizeTarget(target),
		Amount:         amount,
		Ability:        strings.TrimSpace(dot[3]),
		Critical:       isCritical(dot[5]),
		Passive:        true,
		DamageOverTime: true,
	}, true
}

func isCritical(marker string) bool {
	return strings.Contains(marker, "Critical")
}

func isIncidentalDamage(verb, marker string) bool {
	switch strings.ToLower(strings.TrimSpace(verb)) {
	case "cleave", "cleaves", "kick", "kicks", "punch", "punches", "reave", "reaves", "strike", "strikes":
		return true
	}
	return strings.Contains(marker, "Riposte")
}

func ParseCastLine(line string) (combat.Cast, bool) {
	timestamp, message, ok := parseEnvelope(line)
	if !ok {
		return combat.Cast{}, false
	}
	return parseCast(timestamp, message)
}

func parseCast(timestamp time.Time, message string) (combat.Cast, bool) {
	cast := castRE.FindStringSubmatch(message)
	if cast == nil {
		return combat.Cast{}, false
	}
	return combat.Cast{
		Time:    timestamp,
		Source:  normalizeSource(strings.TrimSpace(cast[1])),
		Ability: strings.TrimSpace(cast[2]),
	}, true
}

func ParseDeathLine(line string) (combat.Death, bool) {
	timestamp, message, ok := parseEnvelope(line)
	if !ok {
		return combat.Death{}, false
	}
	return parseDeath(timestamp, message)
}

func parseDeath(timestamp time.Time, message string) (combat.Death, bool) {
	if slain := youSlainRE.FindStringSubmatch(message); slain != nil {
		return combat.Death{
			Time:   timestamp,
			Victim: normalizeTarget(strings.TrimSpace(slain[1])),
			Killer: "You",
		}, true
	}

	if slainBy := slainByRE.FindStringSubmatch(message); slainBy != nil {
		return combat.Death{
			Time:   timestamp,
			Victim: normalizeTarget(strings.TrimSpace(slainBy[1])),
			Killer: normalizeSource(strings.TrimSpace(slainBy[2])),
		}, true
	}

	return combat.Death{}, false
}

func ParseExperienceLine(line string) (ExperienceGain, bool) {
	timestamp, message, ok := parseEnvelope(line)
	if !ok {
		return ExperienceGain{}, false
	}
	return parseExperience(timestamp, message)
}

func parseExperience(timestamp time.Time, message string) (ExperienceGain, bool) {
	matches := experienceRE.FindStringSubmatch(message)
	if matches == nil {
		return ExperienceGain{}, false
	}
	percent, err := strconv.ParseFloat(matches[1], 64)
	if err != nil {
		return ExperienceGain{}, false
	}
	return ExperienceGain{Time: timestamp, Percent: percent}, true
}

func ParseLevelUpLine(line string) (LevelUp, bool) {
	timestamp, message, ok := parseEnvelope(line)
	if !ok {
		return LevelUp{}, false
	}
	return parseLevelUp(timestamp, message)
}

func parseLevelUp(timestamp time.Time, message string) (LevelUp, bool) {
	matches := levelUpRE.FindStringSubmatch(message)
	if matches == nil {
		return LevelUp{}, false
	}
	level, err := strconv.Atoi(matches[1])
	if err != nil {
		return LevelUp{}, false
	}
	return LevelUp{Time: timestamp, Level: level}, true
}

func ParseAggroClearLine(line string) (time.Time, bool) {
	timestamp, message, ok := parseEnvelope(line)
	if !ok || !aggroClearRE.MatchString(message) {
		return time.Time{}, false
	}
	return timestamp, true
}

func ParseTime(line string) (time.Time, bool) {
	timestamp, _, ok := parseEnvelope(line)
	return timestamp, ok
}

func parseEnvelope(line string) (time.Time, string, bool) {
	line = strings.TrimSpace(line)
	matches := lineRE.FindStringSubmatch(line)
	if matches == nil {
		return time.Time{}, "", false
	}

	timestamp, err := time.Parse(timestampLayout, matches[1])
	if err != nil {
		return time.Time{}, "", false
	}

	return timestamp, matches[2], true
}

func normalizeSource(source string) string {
	if source == "You" {
		return "You"
	}
	return normalizeCombatantName(source)
}

func normalizeTarget(target string) string {
	if strings.EqualFold(target, "you") {
		return "YOU"
	}
	return normalizeCombatantName(target)
}

func normalizeCombatantName(name string) string {
	for _, article := range []string{"A ", "An ", "The "} {
		if strings.HasPrefix(name, article) {
			return strings.ToLower(article[:len(article)-1]) + name[len(article)-1:]
		}
	}
	return name
}
