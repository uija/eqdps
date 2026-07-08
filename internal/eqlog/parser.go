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

	damageRE   = regexp.MustCompile(`^(.+?) (backstab|backstabs|bash|bashes|bite|bites|cleave|cleaves|claw|claws|crush|crushes|frenzies on|hit|hits|kick|kicks|maul|mauls|pierce|pierces|punch|punches|shoot|shoots|slash|slashes|slice|slices|smite|smites|strike|strikes) (.+?) for ([0-9]+) points? of ((?:[A-Za-z-]+ )?damage)(?: by ([^.]+))?\.(?: \(([^)]+)\))?$`)
	dotRE      = regexp.MustCompile(`^(.+?) has taken ([0-9]+) damage from (.+?) by ([^.]+)\.$`)
	thornsRE   = regexp.MustCompile(`^(.+?) is .+? by YOUR (.+?) for ([0-9]+) points? of ((?:[A-Za-z-]+ )?damage)\.(?: \(([^)]+)\))?$`)
	youSlainRE = regexp.MustCompile(`^You have slain (.+)!$`)
	slainByRE  = regexp.MustCompile(`^(.+) has been slain by (.+)!$`)
)

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
			Critical: damage[7] == "Critical",
		}, true
	}

	thorns := thornsRE.FindStringSubmatch(message)
	if thorns != nil {
		amount, err := strconv.Atoi(thorns[3])
		if err != nil {
			return combat.Event{}, false
		}

		return combat.Event{
			Time:     timestamp,
			Source:   "You",
			Target:   normalizeTarget(strings.TrimSpace(thorns[1])),
			Amount:   amount,
			Kind:     strings.TrimSpace(thorns[4]),
			Ability:  strings.TrimSpace(thorns[2]),
			Critical: thorns[5] == "Critical",
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
		Time:    timestamp,
		Source:  normalizeSource(source),
		Target:  normalizeTarget(target),
		Amount:  amount,
		Kind:    "damage",
		Ability: strings.TrimSpace(dot[3]),
	}, true
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
	if target == "YOU" {
		return target
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
