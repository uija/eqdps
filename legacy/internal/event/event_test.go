package event

import (
	"encoding/json"
	"errors"
	"testing"
)

func TestUnmarshalDefaultsActiveAndMigratesPermanent(t *testing.T) {
	var configured Event
	if err := json.Unmarshal([]byte(`{
		"id":"old","title":"old event","trigger_type":"text","pattern":"ready",
		"permanent":true
	}`), &configured); err != nil {
		t.Fatal(err)
	}
	if !configured.Active {
		t.Fatal("event without active field should default to active")
	}
	if !configured.RequestPersistence {
		t.Fatal("legacy permanent field was not migrated")
	}
}

func TestCompileAndMatch(t *testing.T) {
	matcher, err := Compile([]Event{
		{ID: "spell", Title: "Spell", Active: true, TriggerType: TriggerSpell, Pattern: "Your speed returns to normal.", SpellName: "Alacrity", Notification: "%s faded."},
		{ID: "text", Title: "Text", Active: true, TriggerType: TriggerText, Pattern: "speed returns"},
		{ID: "exact", Title: "Exact", Active: true, TriggerType: TriggerText, Pattern: "Your speed returns to normal.", ExactMatch: true},
		{ID: "regexp", Title: "Regexp", Active: true, TriggerType: TriggerRegexp, Pattern: `speed\s+returns`},
		{ID: "inactive", Title: "Inactive", TriggerType: TriggerText, Pattern: "speed"},
	})
	if err != nil {
		t.Fatal(err)
	}
	matches := matcher.Matches("[Fri Jul 24 20:15:01 2026] Your speed returns to normal.\r\n")
	if len(matches) != 4 {
		t.Fatalf("matched %d events, want 4", len(matches))
	}
	if got := matches[0].NotificationText(); got != "Alacrity faded." {
		t.Fatalf("notification text = %q", got)
	}
}

func TestCompileRejectsInvalidRegexp(t *testing.T) {
	_, err := Compile([]Event{{
		ID: "bad", Title: "Bad", Active: true, TriggerType: TriggerRegexp, Pattern: "[",
	}})
	if err == nil {
		t.Fatal("expected invalid regular expression error")
	}
}

func TestDispatcherIsNonBlockingWhenFull(t *testing.T) {
	var reported error
	dispatcher, err := NewDispatcher([]Event{{
		ID: "text", Title: "Text", Active: true, TriggerType: TriggerText, Pattern: "ready", Notification: "Ready",
	}}, 1, func(err error) {
		reported = err
	})
	if err != nil {
		t.Fatal(err)
	}
	dispatcher.ObserveLiveLine("[Fri Jul 24 20:15:01 2026] ready")
	dispatcher.ObserveLiveLine("[Fri Jul 24 20:15:02 2026] ready")
	if !errors.Is(reported, ErrQueueFull) {
		t.Fatalf("reported error = %v, want %v", reported, ErrQueueFull)
	}
}
