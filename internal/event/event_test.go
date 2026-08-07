package event

import (
	"encoding/json"
	"errors"
	"testing"
	"time"
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

func TestSpellTimerMatchesRankedAndUnrankedSpellNames(t *testing.T) {
	for _, name := range []string{"Mesmerization", "Mesmerization VI", "Mesmerization Rk. II"} {
		if !spellNameMatches(name, "Mesmerization") {
			t.Errorf("%q did not match Mesmerization", name)
		}
	}
	if spellNameMatches("Mesmerization Field", "Mesmerization") {
		t.Fatal("different spell was treated as a rank")
	}
}

func TestSpellTimerExpiresAfterConfiguredDuration(t *testing.T) {
	dispatcher := newTestSpellTimerDispatcher(t)
	dispatcher.ObserveLiveLine("[Fri Aug 07 08:48:41 2026] You begin casting Mesmerization VI.")
	select {
	case delivery := <-dispatcher.Notifications():
		if delivery.NotificationText != "Mesmerization timer expired." {
			t.Fatalf("notification = %q", delivery.NotificationText)
		}
	case <-time.After(200 * time.Millisecond):
		t.Fatal("spell timer did not expire")
	}
}

func TestSpellTimerIsCancelledByInterruptionDuringGrace(t *testing.T) {
	dispatcher := newTestSpellTimerDispatcher(t)
	dispatcher.ObserveLiveLine("[Fri Aug 07 08:48:41 2026] You begin casting Mesmerization VI.")
	dispatcher.ObserveLiveLine("[Fri Aug 07 08:48:41 2026] Your Mesmerization spell is interrupted.")
	select {
	case <-dispatcher.Notifications():
		t.Fatal("interrupted spell timer expired")
	case <-time.After(100 * time.Millisecond):
	}
}

func TestSpellTimerRecastResetsRunningTimer(t *testing.T) {
	dispatcher := newTestSpellTimerDispatcher(t)
	dispatcher.ObserveLiveLine("You begin casting Mesmerization VI.")
	time.Sleep(25 * time.Millisecond)
	dispatcher.ObserveLiveLine("You begin casting Mesmerization VI.")
	select {
	case <-dispatcher.Notifications():
		t.Fatal("spell timer expired from the first cast")
	case <-time.After(25 * time.Millisecond):
	}
	select {
	case <-dispatcher.Notifications():
	case <-time.After(100 * time.Millisecond):
		t.Fatal("reset spell timer did not expire")
	}
}

func TestInterruptedRecastKeepsPreviousTimer(t *testing.T) {
	dispatcher := newTestSpellTimerDispatcher(t)
	dispatcher.ObserveLiveLine("You begin casting Mesmerization VI.")
	time.Sleep(25 * time.Millisecond)
	dispatcher.ObserveLiveLine("You begin casting Mesmerization VI.")
	dispatcher.ObserveLiveLine("Your Mesmerization spell is interrupted.")
	select {
	case <-dispatcher.Notifications():
	case <-time.After(100 * time.Millisecond):
		t.Fatal("interrupted recast cancelled the previous running timer")
	}
}

func TestActiveSpellTimerAppearsOnlyAfterGrace(t *testing.T) {
	dispatcher := newTestSpellTimerDispatcher(t)
	dispatcher.ObserveLiveLine("You begin casting Mesmerization VI.")
	if active := dispatcher.ActiveTimers(); len(active) != 0 {
		t.Fatalf("active timers during grace = %#v", active)
	}
	select {
	case <-dispatcher.TimerChanges():
	case <-time.After(100 * time.Millisecond):
		t.Fatal("successful cast did not announce its active timer")
	}
	active := dispatcher.ActiveTimers()
	if len(active) != 1 || active[0].Title != "Mez" || active[0].SpellName != "Mesmerization" {
		t.Fatalf("active timers = %#v", active)
	}
}

func TestReplacingUnchangedEventsKeepsRunningSpellTimer(t *testing.T) {
	configured := testSpellTimerEvent()
	dispatcher, err := newDispatcher([]Event{configured}, 4, 10*time.Millisecond, nil)
	if err != nil {
		t.Fatal(err)
	}
	dispatcher.timerUnit = 30 * time.Millisecond
	dispatcher.ObserveLiveLine("You begin casting Mesmerization VI.")
	time.Sleep(20 * time.Millisecond)
	if err := dispatcher.Replace([]Event{configured}); err != nil {
		t.Fatal(err)
	}
	select {
	case <-dispatcher.Notifications():
	case <-time.After(100 * time.Millisecond):
		t.Fatal("reloading unchanged events cancelled the running timer")
	}
}

func newTestSpellTimerDispatcher(t *testing.T) *Dispatcher {
	t.Helper()
	dispatcher, err := newDispatcher([]Event{testSpellTimerEvent()}, 4, 10*time.Millisecond, func(err error) { t.Errorf("dispatcher error: %v", err) })
	if err != nil {
		t.Fatal(err)
	}
	dispatcher.timerUnit = 30 * time.Millisecond
	return dispatcher
}

func testSpellTimerEvent() Event {
	return Event{
		ID: "mez", Title: "Mez", Active: true, TriggerType: TriggerSpellTimer,
		Pattern: "Mesmerization", SpellName: "Mesmerization", TimerSeconds: 2,
		Notification: "%s timer expired.",
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

func TestCompileRejectsSpellTimerWithoutPositiveDuration(t *testing.T) {
	_, err := Compile([]Event{{
		ID: "timer", Title: "Timer", Active: true, TriggerType: TriggerSpellTimer,
		Pattern: "Mesmerization", SpellName: "Mesmerization",
	}})
	if err == nil {
		t.Fatal("expected invalid spell timer duration error")
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
