package main

import (
	"image"
	"reflect"
	"strings"
	"testing"

	"gioui.org/f32"
	"gioui.org/io/input"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/widget"
	"github.com/uija/eqdps/internal/audio"
	"github.com/uija/eqdps/internal/catalog"
	"github.com/uija/eqdps/internal/event"
	"github.com/uija/eqdps/internal/eventstore"
)

func TestEventsRailOrder(t *testing.T) {
	items := eventRailItems()
	short := make([]string, len(items))
	for index := range items {
		short[index] = items[index].short
	}
	if want := []string{"DPS", "SKY", "EVENTS", "SET"}; !reflect.DeepEqual(short, want) {
		t.Fatalf("rail order = %#v, want %#v", short, want)
	}
}

func TestGUIIconPromptIsDeferredUntilEventsEntryWithLog(t *testing.T) {
	tests := []struct {
		state eventstore.IconSetup
		log   string
		want  bool
	}{
		{state: eventstore.IconSetupUnknown, log: "/eq/Logs/eqlog.txt", want: true},
		{state: eventstore.IconSetupUnknown, log: "", want: false},
		{state: eventstore.IconSetupEnabled, log: "/eq/Logs/eqlog.txt", want: false},
		{state: eventstore.IconSetupDeclined, log: "/eq/Logs/eqlog.txt", want: false},
	}
	for _, test := range tests {
		if got := shouldPromptGUIEventIcons(test.state, test.log); got != test.want {
			t.Errorf("shouldPromptGUIEventIcons(%q, %q) = %v, want %v", test.state, test.log, got, test.want)
		}
	}
}

func TestGUIIconPromptUsesExplicitChoices(t *testing.T) {
	ui := eventsGUI{iconAutomatic: true}
	actions := ui.iconActions()
	labels := make([]string, len(actions))
	for index, action := range actions {
		labels[index] = action.label
	}
	want := []string{"Extract icons", "Ask next time", "Don't ask again"}
	if !reflect.DeepEqual(labels, want) {
		t.Fatalf("automatic icon actions = %#v, want %#v", labels, want)
	}

	ui.iconAutomatic = false
	actions = ui.iconActions()
	labels = labels[:len(actions)]
	for index, action := range actions {
		labels[index] = action.label
	}
	want = []string{"Extract icons", "Close"}
	if !reflect.DeepEqual(labels, want) {
		t.Fatalf("manual icon actions = %#v, want %#v", labels, want)
	}
}

func TestSpellSelectorFiltersByClass(t *testing.T) {
	ui := eventsGUI{
		spells: []catalog.Spell{
			{Name: "Bard Song", Classes: []string{"Bard"}},
			{Name: "Wizard Spell", Classes: []string{"Wizard"}},
		},
		classes:       []string{"Bard", "Wizard"},
		classSelected: 1,
	}
	ui.updateVisibleSpells()
	if len(ui.visibleSpells) != 1 || ui.visibleSpells[0].Name != "Bard Song" {
		t.Fatalf("visible spells = %#v", ui.visibleSpells)
	}
}

func TestOpenPickerDoesNotAddEditorRows(t *testing.T) {
	ui := eventsGUI{editingKind: event.TriggerSpell, picker: "class"}
	want := "title,class,spell,notification,persistence,sound"
	if got := strings.Join(ui.editorItems(), ","); got != want {
		t.Fatalf("editor items with class picker = %q, want %q", got, want)
	}
	ui.picker = "sound"
	if got := strings.Join(ui.editorItems(), ","); got != want {
		t.Fatalf("editor items with sound picker = %q, want %q", got, want)
	}
}

func TestSelectorBackdropClosesWithoutChangingSelection(t *testing.T) {
	ui := eventsGUI{
		picker:          "spell",
		iconSetOpen:     true,
		classSelected:   2,
		spellSelected:   3,
		soundSelected:   1,
		iconSetSelected: 1,
	}
	ui.selectorBackdrop.Click()
	ui.dismissOpenSelector(layout.Context{})

	if ui.picker != "" || ui.iconSetOpen {
		t.Fatalf("selector remained open: picker=%q iconSetOpen=%v", ui.picker, ui.iconSetOpen)
	}
	if ui.classSelected != 2 || ui.spellSelected != 3 || ui.soundSelected != 1 || ui.iconSetSelected != 1 {
		t.Fatalf(
			"closing backdrop changed a selection: class=%d spell=%d sound=%d icons=%d",
			ui.classSelected,
			ui.spellSelected,
			ui.soundSelected,
			ui.iconSetSelected,
		)
	}
}

func TestSelectorBackdropConsumesOutsideClick(t *testing.T) {
	var (
		router     input.Router
		underlying widget.Clickable
		ui         eventsGUI
	)
	gtx := layout.Context{
		Ops:         new(op.Ops),
		Source:      router.Source(),
		Constraints: layout.Exact(image.Pt(200, 200)),
	}
	ui.deferSelectorBackdrop(gtx)
	underlying.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Dimensions{Size: gtx.Constraints.Max}
	})
	router.Frame(gtx.Ops)
	router.Queue(
		pointer.Event{Source: pointer.Touch, Kind: pointer.Press, Position: f32.Pt(20, 20)},
		pointer.Event{Source: pointer.Touch, Kind: pointer.Release, Position: f32.Pt(20, 20)},
	)

	if underlying.Clicked(gtx) {
		t.Fatal("outside click reached the form beneath the selector backdrop")
	}
	if !ui.selectorBackdrop.Clicked(gtx) {
		t.Fatal("selector backdrop did not receive outside click")
	}
}

func TestRegexpEditorRejectsInvalidPatternBeforePersistence(t *testing.T) {
	ui := eventsGUI{
		editingKind:   event.TriggerRegexp,
		editingActive: true,
		sounds:        []audio.Sound{{Label: "[No Sound]"}},
	}
	ui.titleEditor.SetText("Invalid")
	ui.patternEditor.SetText("[")
	ui.saveEditor()
	if !strings.Contains(ui.error, "Invalid regular expression") {
		t.Fatalf("editor error = %q", ui.error)
	}
}
