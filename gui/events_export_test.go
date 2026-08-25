package main

import (
	"encoding/json"
	"testing"

	"github.com/uija/eqdps/internal/event"
)

func TestMarshalEventsForRewrite(t *testing.T) {
	data, err := marshalEventsForRewrite([]event.Event{
		{
			Title: "Clarity", Active: true, TriggerType: event.TriggerSpell,
			Pattern: "The cool breeze fades.", SpellName: "Clarity",
			Notification: "%s faded.", RequestPersistence: true,
			Sound: "embedded:Chord.mp3",
		},
		{
			Title: "Incoming", Active: false, TriggerType: event.TriggerText,
			Pattern: "You are under attack", ExactMatch: true,
		},
		{
			Title: "Named", Active: true, TriggerType: event.TriggerRegexp,
			Pattern: `^(.+) shouts,$`,
		},
		{
			Title: "Charm", Active: true, TriggerType: event.TriggerSpellTimer,
			Pattern: "Charm", SpellName: "Charm", TimerSeconds: 90,
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	var exported rewriteEventExport
	if err := json.Unmarshal(data, &exported); err != nil {
		t.Fatal(err)
	}
	if exported.Format != "eqdps-events" || exported.Version != 1 || len(exported.Events) != 4 {
		t.Fatalf("unexpected export header: %#v", exported)
	}
	spell := exported.Events[0]
	if spell.Type != rewriteEventSpell || spell.Spell != "Clarity" || spell.Expression != "The cool breeze fades." ||
		!spell.PersistNotification || spell.Sound != "embedded:Chord.mp3" || !spell.Active {
		t.Fatalf("unexpected spell export: %#v", spell)
	}
	text := exported.Events[1]
	if text.Type != rewriteEventString || !text.FullExpression || text.Active {
		t.Fatalf("unexpected text export: %#v", text)
	}
	if exported.Events[2].Type != rewriteEventRegexp {
		t.Fatalf("unexpected regexp export: %#v", exported.Events[2])
	}
	timer := exported.Events[3]
	if timer.Type != rewriteEventTimer || timer.Spell != "Charm" || timer.Duration != 90 {
		t.Fatalf("unexpected timer export: %#v", timer)
	}
}

func TestEventForRewriteRejectsUnknownType(t *testing.T) {
	if _, err := eventForRewrite(event.Event{Title: "broken", TriggerType: "unknown"}); err == nil {
		t.Fatal("unknown trigger type was accepted")
	}
}
