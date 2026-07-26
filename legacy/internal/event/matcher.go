package event

import (
	"fmt"
	"regexp"
	"strings"
)

var logPrefix = regexp.MustCompile(`^\[[^\]]+\]\s*`)

type compiledEvent struct {
	event  Event
	regexp *regexp.Regexp
}

type Matcher struct {
	events []compiledEvent
}

func Compile(events []Event) (*Matcher, error) {
	compiled := make([]compiledEvent, 0, len(events))
	ids := make(map[string]bool, len(events))
	for _, configuredEvent := range events {
		if err := configuredEvent.Validate(); err != nil {
			return nil, err
		}
		if ids[configuredEvent.ID] {
			return nil, fmt.Errorf("duplicate event ID %q", configuredEvent.ID)
		}
		ids[configuredEvent.ID] = true

		entry := compiledEvent{event: configuredEvent}
		if configuredEvent.TriggerType == TriggerRegexp {
			matcher, err := regexp.Compile(configuredEvent.Pattern)
			if err != nil {
				return nil, fmt.Errorf("compile event %q regular expression: %w", configuredEvent.Title, err)
			}
			entry.regexp = matcher
		}
		compiled = append(compiled, entry)
	}
	return &Matcher{events: compiled}, nil
}

func (m *Matcher) Matches(logLine string) []Event {
	if m == nil {
		return nil
	}
	message := logPrefix.ReplaceAllString(strings.TrimRight(logLine, "\r\n"), "")
	var matches []Event
	for _, entry := range m.events {
		if !entry.event.Active {
			continue
		}
		matched := false
		switch entry.event.TriggerType {
		case TriggerSpell:
			matched = message == entry.event.Pattern
		case TriggerText:
			if entry.event.ExactMatch {
				matched = message == entry.event.Pattern
			} else {
				matched = strings.Contains(message, entry.event.Pattern)
			}
		case TriggerRegexp:
			matched = entry.regexp.MatchString(message)
		}
		if matched {
			matches = append(matches, entry.event)
		}
	}
	return matches
}
