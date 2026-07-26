package event

import (
	"encoding/json"
	"fmt"
	"strings"
)

type TriggerType string

const (
	TriggerSpell  TriggerType = "spell"
	TriggerText   TriggerType = "text"
	TriggerRegexp TriggerType = "regexp"
)

type Event struct {
	ID                 string      `json:"id"`
	Title              string      `json:"title"`
	Active             bool        `json:"active"`
	TriggerType        TriggerType `json:"trigger_type"`
	Pattern            string      `json:"pattern"`
	ExactMatch         bool        `json:"exact_match,omitempty"`
	SpellName          string      `json:"spell_name,omitempty"`
	Notification       string      `json:"notification,omitempty"`
	RequestPersistence bool        `json:"request_persistence,omitempty"`
	Sound              string      `json:"sound,omitempty"`
}

func (e *Event) UnmarshalJSON(data []byte) error {
	type eventAlias Event
	var decoded struct {
		eventAlias
		Permanent bool `json:"permanent,omitempty"`
	}
	decoded.eventAlias.Active = true
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*e = Event(decoded.eventAlias)
	if decoded.Permanent {
		e.RequestPersistence = true
	}
	return nil
}

func (e Event) Validate() error {
	if strings.TrimSpace(e.ID) == "" {
		return fmt.Errorf("event ID is required")
	}
	if strings.TrimSpace(e.Title) == "" {
		return fmt.Errorf("event %q has no title", e.ID)
	}
	if strings.TrimSpace(e.Pattern) == "" {
		return fmt.Errorf("event %q has no pattern", e.Title)
	}
	switch e.TriggerType {
	case TriggerSpell:
		if strings.TrimSpace(e.SpellName) == "" {
			return fmt.Errorf("spell event %q has no spell name", e.Title)
		}
	case TriggerText, TriggerRegexp:
	default:
		return fmt.Errorf("event %q has unsupported trigger type %q", e.Title, e.TriggerType)
	}
	return nil
}

func (e Event) NotificationText() string {
	if e.TriggerType != TriggerSpell {
		return e.Notification
	}
	return strings.ReplaceAll(e.Notification, "%s", e.SpellName)
}
