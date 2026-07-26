package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/uija/eqdps/legacy/internal/event"
	"github.com/uija/eqdps/legacy/internal/eventstore"
)

func TestSpellIconPromptIsContextualToEnteringEvents(t *testing.T) {
	tests := []struct {
		name    string
		state   eventstore.IconSetup
		logPath string
		want    bool
	}{
		{name: "unknown with log", state: eventstore.IconSetupUnknown, logPath: "/eq/Logs/eqlog.txt", want: true},
		{name: "unknown without log", state: eventstore.IconSetupUnknown, want: false},
		{name: "enabled", state: eventstore.IconSetupEnabled, logPath: "/eq/Logs/eqlog.txt", want: false},
		{name: "declined", state: eventstore.IconSetupDeclined, logPath: "/eq/Logs/eqlog.txt", want: false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := shouldPromptSpellIcons(test.state, test.logPath); got != test.want {
				t.Fatalf("shouldPromptSpellIcons() = %v, want %v", got, test.want)
			}
		})
	}
}

func TestEventSoundListIncludesUserAndMissingSounds(t *testing.T) {
	store, err := eventstore.OpenAt(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(store.AudioDir(), "custom.wav"), []byte("test"), 0o600); err != nil {
		t.Fatal(err)
	}
	sounds, err := eventSoundList(store, []event.Event{{Sound: "user:missing.mp3"}})
	if err != nil {
		t.Fatal(err)
	}
	var custom, missing bool
	for _, sound := range sounds {
		custom = custom || sound.ID == "user:custom.wav"
		missing = missing || sound.ID == "user:missing.mp3"
	}
	if !custom || !missing {
		t.Fatalf("sound list missing entries: custom=%v missing=%v", custom, missing)
	}
}

func TestDPSShortcutsExposeEventsPage(t *testing.T) {
	if got := shortcutsText(); !strings.Contains(got, "[gray]n[::-]") || !strings.Contains(got, "Events") {
		t.Fatalf("shortcut bar does not expose Events page: %q", got)
	}
}

func TestEventsEscapeClosesModalBeforePage(t *testing.T) {
	ui := &eventsTUI{
		app: tview.NewApplication(), pages: tview.NewPages(), table: tview.NewTable(),
		open: true, modalOpen: true,
	}
	if !ui.HandleGlobal(tcell.NewEventKey(tcell.KeyEscape, 0, tcell.ModNone)) {
		t.Fatal("escape was not handled")
	}
	if ui.modalOpen {
		t.Fatal("escape did not close modal")
	}
	if !ui.open {
		t.Fatal("closing modal also closed Events page")
	}
}

func TestEventSettingsVolumeArrowKeysAdjustByFiveAndAreHandled(t *testing.T) {
	tests := []struct {
		name string
		key  tcell.Key
		text string
		want int
	}{
		{name: "up", key: tcell.KeyUp, text: "50", want: 55},
		{name: "right", key: tcell.KeyRight, text: "50", want: 55},
		{name: "down", key: tcell.KeyDown, text: "50", want: 45},
		{name: "left", key: tcell.KeyLeft, text: "50", want: 45},
		{name: "upper bound", key: tcell.KeyUp, text: "100", want: 100},
		{name: "lower bound", key: tcell.KeyLeft, text: "0", want: 0},
		{name: "empty uses current volume", key: tcell.KeyRight, text: "", want: 40},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, handled := adjustVolumeWithArrow(test.key, test.text, .35)
			if !handled {
				t.Fatal("arrow key was not consumed")
			}
			if got != test.want {
				t.Fatalf("adjusted volume = %d, want %d", got, test.want)
			}
		})
	}
	if _, handled := adjustVolumeWithArrow(tcell.KeyEnter, "50", .35); handled {
		t.Fatal("non-arrow key was consumed")
	}
}
