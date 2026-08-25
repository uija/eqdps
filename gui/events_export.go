package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"github.com/ncruces/zenity"
	"github.com/uija/eqdps/internal/event"
)

const rewriteEventsFormat = "eqdps-events"

const (
	rewriteEventString = iota
	rewriteEventRegexp
	rewriteEventSpell
	rewriteEventTimer
)

type rewriteEventExport struct {
	Format  string         `json:"format"`
	Version int            `json:"version"`
	Events  []rewriteEvent `json:"events"`
}

type rewriteEvent struct {
	Type                int    `json:"type"`
	Title               string `json:"title"`
	Class               string `json:"class,omitempty"`
	Spell               string `json:"spell,omitempty"`
	Expression          string `json:"expression"`
	ExpressionOthers    string `json:"expression_others"`
	FullExpression      bool   `json:"full_expression"`
	Duration            int    `json:"duration,omitempty"`
	Notification        string `json:"notification"`
	PersistNotification bool   `json:"persist_notification"`
	Sound               string `json:"sound"`
	Active              bool   `json:"active"`
}

type eventExportResult struct {
	path string
	err  error
}

func exportEventsForRewrite(events []event.Event) eventExportResult {
	path, err := zenity.SelectFileSave(
		zenity.Title("Export Events"),
		zenity.Filename("eqdps-events.json"),
		zenity.ConfirmOverwrite(),
		zenity.FileFilters{{
			Name:     "eqdps Events",
			Patterns: []string{"*.json"},
		}},
	)
	if errors.Is(err, zenity.ErrCanceled) {
		return eventExportResult{}
	}
	if err != nil {
		return eventExportResult{err: fmt.Errorf("select event export file: %w", err)}
	}

	data, err := marshalEventsForRewrite(events)
	if err != nil {
		return eventExportResult{err: err}
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return eventExportResult{err: fmt.Errorf("write event export: %w", err)}
	}
	return eventExportResult{path: path}
}

func marshalEventsForRewrite(events []event.Event) ([]byte, error) {
	exported := rewriteEventExport{
		Format:  rewriteEventsFormat,
		Version: 1,
		Events:  make([]rewriteEvent, 0, len(events)),
	}
	for _, configured := range events {
		converted, err := eventForRewrite(configured)
		if err != nil {
			return nil, err
		}
		exported.Events = append(exported.Events, converted)
	}
	data, err := json.Marshal(exported)
	if err != nil {
		return nil, fmt.Errorf("encode event export: %w", err)
	}
	return data, nil
}

func eventForRewrite(configured event.Event) (rewriteEvent, error) {
	exported := rewriteEvent{
		Title:               configured.Title,
		Spell:               configured.SpellName,
		Expression:          configured.Pattern,
		FullExpression:      configured.ExactMatch,
		Duration:            configured.TimerSeconds,
		Notification:        configured.Notification,
		PersistNotification: configured.RequestPersistence,
		Sound:               configured.Sound,
		Active:              configured.Active,
	}
	switch configured.TriggerType {
	case event.TriggerText:
		exported.Type = rewriteEventString
	case event.TriggerRegexp:
		exported.Type = rewriteEventRegexp
	case event.TriggerSpell:
		exported.Type = rewriteEventSpell
	case event.TriggerSpellTimer:
		exported.Type = rewriteEventTimer
	default:
		return rewriteEvent{}, fmt.Errorf("event %q has unsupported trigger type %q", configured.Title, configured.TriggerType)
	}
	return exported, nil
}
