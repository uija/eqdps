package eqlog

import (
	"bufio"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/uija/eqdps/internal/combat"
)

const timestampLayout = "Mon Jan 02 15:04:05 2006"

var (
	lineRE = regexp.MustCompile(`^\[([^\]]+)\] (.*)$`)

	damageRE     = regexp.MustCompile(`^(.+?) (backstab|backstabs|bash|bashes|bite|bites|cleave|cleaves|claw|claws|crush|crushes|frenzy on|frenzies on|hit|hits|kick|kicks|maul|mauls|pierce|pierces|punch|punches|reave|reaves|shoot|shoots|slash|slashes|slice|slices|smash|smashes|smite|smites|strike|strikes) (.+?) for ([0-9]+) points? of ((?:[A-Za-z-]+ )?damage)(?: by ([^.]+))?\.(?: \(([^)]+)\))?$`)
	dotRE        = regexp.MustCompile(`^(.+?) (?:has|have) taken ([0-9]+) damage from (.+?) by ([^.]+)\.(?: \(([^)]+)\))?$`)
	yourDotRE    = regexp.MustCompile(`^(.+?) has taken ([0-9]+) damage from your (.+?)\.(?: \(([^)]+)\))?$`)
	yourShieldRE = regexp.MustCompile(`^(.+?) is .+? by YOUR (.+?) for ([0-9]+) points? of ((?:[A-Za-z-]+ )?damage)\.(?: \(([^)]+)\))?$`)
	shieldRE     = regexp.MustCompile("^(.+?) (?:is|are) .+? by (.+)(?:'s|`s) (.+?) for ([0-9]+) points? of ((?:[A-Za-z-]+ )?damage)[.!](?: \\(([^)]+)\\))?$")
	youSlainRE   = regexp.MustCompile(`^You have slain (.+)!$`)
	slainByRE    = regexp.MustCompile(`^(.+) has been slain by (.+)!$`)
	experienceRE = regexp.MustCompile(`^You gain experience! \(([0-9]+(?:\.[0-9]+)?)%\)$`)
	levelUpRE    = regexp.MustCompile(`^You have gained a level! Welcome to level ([0-9]+)!$`)
)

type ExperienceGain struct {
	Time    time.Time
	Percent float64
}

type LevelUp struct {
	Time  time.Time
	Level int
}

func ParseLine(line string) (combat.Event, bool) {
	timestamp, message, ok := parseEnvelope(line)
	if !ok {
		return combat.Event{}, false
	}
	damage := damageRE.FindStringSubmatch(message)
	if damage != nil {
		source := strings.TrimSpace(damage[1])
		target := strings.TrimSpace(damage[3])

		amount, err := strconv.Atoi(damage[4])
		if err != nil {
			return combat.Event{}, false
		}

		return combat.Event{
			Time:     timestamp,
			Source:   normalizeSource(source),
			Target:   normalizeTarget(target),
			Amount:   amount,
			Kind:     strings.TrimSpace(damage[5]),
			Ability:  strings.TrimSpace(damage[6]),
			Critical: isCritical(damage[7]),
		}, true
	}

	yourShield := yourShieldRE.FindStringSubmatch(message)
	if yourShield != nil {
		amount, err := strconv.Atoi(yourShield[3])
		if err != nil {
			return combat.Event{}, false
		}

		return combat.Event{
			Time:     timestamp,
			Source:   "You",
			Target:   normalizeTarget(strings.TrimSpace(yourShield[1])),
			Amount:   amount,
			Kind:     strings.TrimSpace(yourShield[4]),
			Ability:  strings.TrimSpace(yourShield[2]),
			Critical: isCritical(yourShield[5]),
			Passive:  true,
		}, true
	}

	shield := shieldRE.FindStringSubmatch(message)
	if shield != nil {
		amount, err := strconv.Atoi(shield[4])
		if err != nil {
			return combat.Event{}, false
		}

		return combat.Event{
			Time:     timestamp,
			Source:   normalizeSource(strings.TrimSpace(shield[2])),
			Target:   normalizeTarget(strings.TrimSpace(shield[1])),
			Amount:   amount,
			Kind:     strings.TrimSpace(shield[5]),
			Ability:  strings.TrimSpace(shield[3]),
			Critical: isCritical(shield[6]),
			Passive:  true,
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
			Kind:           "damage",
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
		Kind:           "damage",
		Ability:        strings.TrimSpace(dot[3]),
		Critical:       isCritical(dot[5]),
		Passive:        true,
		DamageOverTime: true,
	}, true
}

func isCritical(marker string) bool {
	return strings.Contains(marker, "Critical")
}

func ParseDeathLine(line string) (combat.Death, bool) {
	timestamp, message, ok := parseEnvelope(line)
	if !ok {
		return combat.Death{}, false
	}

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

func Parse(r io.Reader, meter *combat.Meter) error {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	for scanner.Scan() {
		if event, ok := ParseLine(scanner.Text()); ok {
			meter.Add(event)
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("read log: %w", err)
	}

	return nil
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
