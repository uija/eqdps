package main

import (
	"fmt"
	"image"
	"image/color"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"gioui.org/font"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"github.com/uija/eqdps/internal/audio"
	"github.com/uija/eqdps/internal/catalog"
	"github.com/uija/eqdps/internal/event"
	"github.com/uija/eqdps/internal/eventruntime"
	"github.com/uija/eqdps/internal/eventstore"
	"github.com/uija/eqdps/internal/spellicon"
)

type eventsGUI struct {
	window  appWindow
	store   *eventstore.Store
	runtime *eventruntime.Runtime
	logPath string

	events  []event.Event
	spells  []catalog.Spell
	sounds  []audio.Sound
	classes []string

	screen string
	error  string
	notice string

	list       widget.List
	editorList widget.List
	pickerList widget.List

	addSpellClick widget.Clickable
	addTimerClick widget.Clickable
	addTextClick  widget.Clickable
	addRegexClick widget.Clickable
	iconClick     widget.Clickable
	activeClicks  []widget.Clickable
	editClicks    []widget.Clickable

	titleEditor        widget.Editor
	patternEditor      widget.Editor
	notificationEditor widget.Editor
	timerSecondsEditor widget.Editor
	exactCheck         widget.Bool
	persistenceCheck   widget.Bool
	saveClick          widget.Clickable
	deleteClick        widget.Clickable
	cancelClick        widget.Clickable

	editingID     string
	editingActive bool
	editingKind   event.TriggerType
	classSelected int
	spellSelected int
	soundSelected int
	visibleSpells []catalog.Spell

	classClick widget.Clickable
	spellClick widget.Clickable
	soundClick widget.Clickable
	playClick  widget.Clickable
	picker     string
	choices    []widget.Clickable

	deleteConfirm widget.Clickable
	deleteCancel  widget.Clickable

	iconExtract   widget.Clickable
	iconDecline   widget.Clickable
	iconCancel    widget.Clickable
	iconBackdrop  widget.Clickable
	iconSetup     eventstore.IconSetup
	iconSource    spellicon.Source
	iconAutomatic bool
	iconBusy      bool
	iconResults   chan iconExtractionResult

	iconSets         []string
	iconSet          string
	iconSetSelected  int
	iconSetClick     widget.Clickable
	iconSetChoices   []widget.Clickable
	iconSetList      widget.List
	iconSetOpen      bool
	selectorBackdrop widget.Clickable

	volume      widget.Float
	audioVolume float32
	volumeDirty bool
}

type iconExtractionResult struct {
	names []string
	err   error
}

func newEventsGUI(window appWindow, store *eventstore.Store, runtime *eventruntime.Runtime, logPath string) (*eventsGUI, error) {
	spells, err := catalog.Load()
	if err != nil {
		return nil, err
	}
	events, err := store.Load()
	if err != nil {
		return nil, err
	}
	sounds, err := guiEventSoundList(store, events)
	if err != nil {
		return nil, err
	}
	iconSetup, err := store.SpellIconSetup()
	if err != nil {
		return nil, err
	}
	volume, err := store.AudioVolume()
	if err != nil {
		return nil, err
	}
	ui := &eventsGUI{
		window:      window,
		store:       store,
		runtime:     runtime,
		logPath:     logPath,
		events:      events,
		spells:      spells,
		sounds:      sounds,
		classes:     guiEventClasses(spells),
		screen:      "list",
		list:        widget.List{List: layout.List{Axis: layout.Vertical}},
		editorList:  widget.List{List: layout.List{Axis: layout.Vertical}},
		pickerList:  widget.List{List: layout.List{Axis: layout.Vertical}},
		iconSetup:   iconSetup,
		iconResults: make(chan iconExtractionResult, 1),
		iconSetList: widget.List{List: layout.List{Axis: layout.Vertical}},
		audioVolume: float32(volume),
	}
	if err := ui.reloadIconSets(); err != nil {
		return nil, err
	}
	ui.volume.Value = float32(volume)
	ui.runtime.SetAudioVolume(volume)
	ui.titleEditor.SingleLine = true
	ui.patternEditor.SingleLine = true
	ui.notificationEditor.SingleLine = true
	ui.timerSecondsEditor.SingleLine = true
	ui.rebuildRowControls()
	return ui, nil
}

func guiEventSoundList(store *eventstore.Store, events []event.Event) ([]audio.Sound, error) {
	embedded, err := audio.EmbeddedSounds()
	if err != nil {
		return nil, err
	}
	user, err := store.Sounds()
	if err != nil {
		return nil, err
	}
	sounds := []audio.Sound{{Label: "[No Sound]"}}
	sounds = append(sounds, embedded...)
	for _, filename := range user {
		sounds = append(sounds, audio.Sound{ID: audio.UserPrefix + filename, Label: filename + " (user)"})
	}
	for _, configured := range events {
		if configured.Sound != "" && guiEventSoundIndex(sounds, configured.Sound) < 0 {
			sounds = append(sounds, audio.Sound{ID: configured.Sound, Label: configured.Sound + " (missing)"})
		}
	}
	return sounds, nil
}

func (ui *eventsGUI) SetLog(path string) {
	ui.logPath = path
}

func (ui *eventsGUI) Enter() {
	if events, err := ui.store.Load(); err != nil {
		ui.error = err.Error()
	} else {
		ui.events = events
		_ = ui.runtime.Replace(events)
		if sounds, soundErr := guiEventSoundList(ui.store, events); soundErr == nil {
			ui.sounds = sounds
		}
		ui.rebuildRowControls()
	}
	if iconSetup, err := ui.store.SpellIconSetup(); err == nil {
		ui.iconSetup = iconSetup
	}
	if volume, err := ui.store.AudioVolume(); err != nil {
		ui.error = err.Error()
	} else {
		ui.audioVolume = float32(volume)
		ui.volume.Value = float32(volume)
		ui.runtime.SetAudioVolume(volume)
		ui.volumeDirty = false
	}
	if err := ui.reloadIconSets(); err != nil {
		ui.error = err.Error()
	}
	ui.screen = "list"
	ui.picker = ""
	ui.iconSetOpen = false
	if shouldPromptGUIEventIcons(ui.iconSetup, ui.logPath) {
		if source, ok := spellicon.Detect(ui.logPath, ui.spells); ok {
			ui.iconSource = source
			ui.iconAutomatic = true
			ui.screen = "icons"
		}
	}
}

func shouldPromptGUIEventIcons(state eventstore.IconSetup, logPath string) bool {
	return state == eventstore.IconSetupUnknown && logPath != ""
}

func (ui *eventsGUI) Update(gtx layout.Context) {
	select {
	case result := <-ui.iconResults:
		ui.iconBusy = false
		err := result.err
		if err == nil && len(result.names) == 0 {
			err = fmt.Errorf("no distinct spell icon sets found")
		}
		if err == nil {
			selected := ui.iconSet
			if !guiEventContains(result.names, selected) {
				selected = result.names[0]
			}
			err = ui.store.SaveSpellIconSet(selected)
		}
		if err == nil {
			err = ui.store.SaveSpellIconSetup(eventstore.IconSetupEnabled)
		}
		if err == nil {
			err = ui.reloadIconSets()
		}
		if err != nil {
			ui.error = "Spell-icon extraction failed: " + err.Error()
		} else {
			ui.iconSetup = eventstore.IconSetupEnabled
			ui.notice = fmt.Sprintf("Extracted %d spell icon sets.", len(result.names))
			ui.screen = "list"
		}
	default:
	}

	switch ui.screen {
	case "list":
		ui.updateList(gtx)
	case "editor":
		ui.updateEditor(gtx)
	case "delete":
		ui.updateDelete(gtx)
	case "icons":
		ui.updateIcons(gtx)
	}
}

func (ui *eventsGUI) updateList(gtx layout.Context) {
	ui.dismissOpenSelector(gtx)
	if ui.iconSetClick.Clicked(gtx) && len(ui.iconSets) > 0 {
		ui.iconSetOpen = !ui.iconSetOpen
	}
	for index := range ui.iconSetChoices {
		if ui.iconSetChoices[index].Clicked(gtx) {
			ui.selectIconSet(index)
			ui.iconSetOpen = false
		}
	}
	if ui.addSpellClick.Clicked(gtx) {
		ui.iconSetOpen = false
		ui.openEditor(event.TriggerSpell, nil)
	}
	if ui.addTimerClick.Clicked(gtx) {
		ui.iconSetOpen = false
		ui.openEditor(event.TriggerSpellTimer, nil)
	}
	if ui.addTextClick.Clicked(gtx) {
		ui.iconSetOpen = false
		ui.openEditor(event.TriggerText, nil)
	}
	if ui.addRegexClick.Clicked(gtx) {
		ui.iconSetOpen = false
		ui.openEditor(event.TriggerRegexp, nil)
	}
	if ui.iconClick.Clicked(gtx) {
		ui.iconSetOpen = false
		source, ok := spellicon.Detect(ui.logPath, ui.spells)
		if !ok {
			ui.error = "Could not locate EverQuest spell-icon files from the selected logfile."
		} else {
			ui.iconSource = source
			ui.iconAutomatic = false
			ui.screen = "icons"
			ui.error = ""
		}
	}
	for index := range ui.events {
		if ui.activeClicks[index].Clicked(gtx) {
			events := append([]event.Event(nil), ui.events...)
			events[index].Active = !events[index].Active
			ui.save(events)
		}
		if ui.editClicks[index].Clicked(gtx) {
			configured := ui.events[index]
			ui.openEditor(configured.TriggerType, &configured)
		}
	}
}

func (ui *eventsGUI) reloadIconSets() error {
	sets, err := ui.store.SpellIconSets()
	if err != nil {
		return err
	}
	selected, err := ui.store.SpellIconSet()
	if err != nil {
		return err
	}
	index := guiEventStringIndex(sets, selected)
	if index < 0 && len(sets) > 0 {
		index = 0
		selected = sets[0]
	}
	ui.iconSets = sets
	ui.iconSet = selected
	ui.iconSetSelected = max(index, 0)
	ui.iconSetChoices = make([]widget.Clickable, len(sets))
	ui.runtime.SetSpellIconSet(selected)
	return nil
}

func (ui *eventsGUI) selectIconSet(index int) {
	if index < 0 || index >= len(ui.iconSets) {
		return
	}
	selected := ui.iconSets[index]
	if err := ui.store.SaveSpellIconSet(selected); err != nil {
		ui.error = err.Error()
		return
	}
	ui.iconSet = selected
	ui.iconSetSelected = index
	ui.runtime.SetSpellIconSet(selected)
	ui.error = ""
}

func (ui *eventsGUI) openEditor(kind event.TriggerType, existing *event.Event) {
	ui.screen = "editor"
	ui.error = ""
	ui.notice = ""
	ui.picker = ""
	ui.editingKind = kind
	ui.editingID = ""
	ui.editingActive = true
	ui.titleEditor.SetText("")
	ui.patternEditor.SetText("")
	ui.notificationEditor.SetText("")
	ui.timerSecondsEditor.SetText("")
	ui.exactCheck.Value = kind == event.TriggerText
	ui.persistenceCheck.Value = false
	ui.classSelected = 0
	ui.spellSelected = 0
	ui.soundSelected = 0
	ui.updateVisibleSpells()
	if kind == event.TriggerSpell {
		ui.notificationEditor.SetText("%s has faded.")
	} else if kind == event.TriggerSpellTimer {
		ui.notificationEditor.SetText("%s timer expired.")
	}
	if existing != nil {
		ui.editingID = existing.ID
		ui.editingActive = existing.Active
		ui.titleEditor.SetText(existing.Title)
		ui.patternEditor.SetText(existing.Pattern)
		ui.notificationEditor.SetText(existing.Notification)
		ui.exactCheck.Value = existing.ExactMatch
		ui.persistenceCheck.Value = existing.RequestPersistence
		if index := guiEventSoundIndex(ui.sounds, existing.Sound); index >= 0 {
			ui.soundSelected = index
		}
		if kind == event.TriggerSpell || kind == event.TriggerSpellTimer {
			if existing.TimerSeconds > 0 {
				ui.timerSecondsEditor.SetText(strconv.Itoa(existing.TimerSeconds))
			}
			ui.classSelected = 0
			ui.updateVisibleSpells()
			for index := range ui.visibleSpells {
				if ui.visibleSpells[index].Name == existing.SpellName {
					ui.spellSelected = index
					break
				}
			}
		}
	}
	ui.editorList.ScrollTo(0)
}

func (ui *eventsGUI) updateEditor(gtx layout.Context) {
	ui.dismissOpenSelector(gtx)
	if ui.cancelClick.Clicked(gtx) {
		ui.screen = "list"
		ui.picker = ""
	}
	if ui.deleteClick.Clicked(gtx) && ui.editingID != "" {
		ui.screen = "delete"
		ui.picker = ""
	}
	if ui.classClick.Clicked(gtx) {
		ui.openPicker("class", ui.classSelected)
	}
	if ui.spellClick.Clicked(gtx) {
		ui.openPicker("spell", ui.spellSelected)
	}
	if ui.soundClick.Clicked(gtx) {
		ui.openPicker("sound", ui.soundSelected)
	}
	if ui.playClick.Clicked(gtx) && ui.soundSelected > 0 && ui.soundSelected < len(ui.sounds) {
		if err := ui.runtime.PreviewSound(ui.sounds[ui.soundSelected].ID); err != nil {
			ui.error = "Could not preview sound: " + err.Error()
		} else {
			ui.error = ""
		}
	}
	if ui.picker != "" {
		for index := range ui.choices {
			if !ui.choices[index].Clicked(gtx) {
				continue
			}
			switch ui.picker {
			case "class":
				ui.classSelected = index
				ui.spellSelected = 0
				ui.updateVisibleSpells()
			case "spell":
				ui.spellSelected = index
				if ui.titleEditor.Text() == "" && index < len(ui.visibleSpells) {
					ui.titleEditor.SetText(ui.visibleSpells[index].Name)
				}
			case "sound":
				ui.soundSelected = index
			}
			ui.picker = ""
			break
		}
	}
	if ui.saveClick.Clicked(gtx) {
		ui.saveEditor()
	}
}

func (ui *eventsGUI) openPicker(kind string, selected int) {
	if ui.picker == kind {
		ui.picker = ""
		return
	}
	ui.picker = kind
	ui.choices = make([]widget.Clickable, len(ui.pickerOptions()))
	ui.pickerList.ScrollTo(max(selected-2, 0))
}

func (ui *eventsGUI) dismissOpenSelector(gtx layout.Context) {
	if ui.selectorBackdrop.Clicked(gtx) {
		ui.picker = ""
		ui.iconSetOpen = false
	}
}

func (ui *eventsGUI) pickerOptions() []string {
	switch ui.picker {
	case "class":
		return append([]string{"ALL"}, ui.classes...)
	case "spell":
		options := make([]string, len(ui.visibleSpells))
		for index, spell := range ui.visibleSpells {
			options[index] = spell.Name
		}
		return options
	case "sound":
		options := make([]string, len(ui.sounds))
		for index, sound := range ui.sounds {
			options[index] = sound.Label
		}
		return options
	default:
		return nil
	}
}

func (ui *eventsGUI) pickerSelected() int {
	switch ui.picker {
	case "class":
		return ui.classSelected
	case "spell":
		return ui.spellSelected
	case "sound":
		return ui.soundSelected
	default:
		return 0
	}
}

func (ui *eventsGUI) updateVisibleSpells() {
	ui.visibleSpells = ui.visibleSpells[:0]
	className := "ALL"
	if ui.classSelected > 0 && ui.classSelected-1 < len(ui.classes) {
		className = ui.classes[ui.classSelected-1]
	}
	for _, spell := range ui.spells {
		if className == "ALL" || guiEventContains(spell.Classes, className) {
			ui.visibleSpells = append(ui.visibleSpells, spell)
		}
	}
	if ui.spellSelected >= len(ui.visibleSpells) {
		ui.spellSelected = 0
	}
}

func (ui *eventsGUI) saveEditor() {
	name := strings.TrimSpace(ui.titleEditor.Text())
	notification := ui.notificationEditor.Text()
	configured := event.Event{
		ID: ui.editingID, Title: name, Active: ui.editingActive, TriggerType: ui.editingKind,
		Notification: notification, RequestPersistence: ui.persistenceCheck.Value,
	}
	if ui.soundSelected > 0 && ui.soundSelected < len(ui.sounds) {
		configured.Sound = ui.sounds[ui.soundSelected].ID
	}
	switch ui.editingKind {
	case event.TriggerSpell, event.TriggerSpellTimer:
		if len(ui.visibleSpells) == 0 || ui.spellSelected >= len(ui.visibleSpells) {
			ui.error = "Select a spell."
			return
		}
		spell := ui.visibleSpells[ui.spellSelected]
		configured.SpellName = spell.Name
		if ui.editingKind == event.TriggerSpell {
			configured.Pattern = spell.FadeMessage
		} else {
			seconds, err := strconv.Atoi(strings.TrimSpace(ui.timerSecondsEditor.Text()))
			if err != nil || seconds < 1 {
				ui.error = "Enter a timer duration of at least one second."
				return
			}
			configured.Pattern = spell.Name
			configured.TimerSeconds = seconds
		}
		if configured.Title == "" {
			configured.Title = spell.Name
		}
	case event.TriggerText:
		configured.Pattern = ui.patternEditor.Text()
		configured.ExactMatch = ui.exactCheck.Value
	case event.TriggerRegexp:
		configured.Pattern = ui.patternEditor.Text()
		if _, err := regexp.Compile(configured.Pattern); err != nil {
			ui.error = "Invalid regular expression: " + err.Error()
			return
		}
	}
	if configured.Pattern == "" {
		ui.error = "Enter a pattern to match."
		return
	}
	if configured.Title == "" {
		configured.Title = configured.Pattern
	}
	events := append([]event.Event(nil), ui.events...)
	if configured.ID == "" {
		id, err := event.NewID()
		if err != nil {
			ui.error = err.Error()
			return
		}
		configured.ID = id
		events = append(events, configured)
	} else {
		found := false
		for index := range events {
			if events[index].ID == configured.ID {
				events[index] = configured
				found = true
				break
			}
		}
		if !found {
			ui.error = "Event no longer exists."
			return
		}
	}
	if ui.save(events) {
		ui.notice = fmt.Sprintf("Saved %q.", configured.Title)
		ui.screen = "list"
		ui.picker = ""
	}
}

func (ui *eventsGUI) updateDelete(gtx layout.Context) {
	if ui.deleteCancel.Clicked(gtx) {
		ui.screen = "editor"
	}
	if ui.deleteConfirm.Clicked(gtx) {
		events := make([]event.Event, 0, len(ui.events)-1)
		title := ""
		for _, configured := range ui.events {
			if configured.ID == ui.editingID {
				title = configured.Title
				continue
			}
			events = append(events, configured)
		}
		if ui.save(events) {
			ui.notice = fmt.Sprintf("Deleted %q.", title)
			ui.screen = "list"
		}
	}
}

func (ui *eventsGUI) updateIcons(gtx layout.Context) {
	if ui.iconBusy {
		return
	}
	if ui.iconCancel.Clicked(gtx) {
		ui.screen = "list"
	}
	if ui.iconDecline.Clicked(gtx) {
		if err := ui.store.SaveSpellIconSetup(eventstore.IconSetupDeclined); err != nil {
			ui.error = err.Error()
		} else {
			ui.iconSetup = eventstore.IconSetupDeclined
			ui.screen = "list"
		}
	}
	if ui.iconExtract.Clicked(gtx) {
		ui.iconBusy = true
		ui.error = ""
		source := ui.iconSource
		go func() {
			names, err := spellicon.ExtractAll(source, ui.store.IconDir(), ui.spells)
			ui.iconResults <- iconExtractionResult{names: names, err: err}
			ui.window.Invalidate()
		}()
	}
}

func (ui *eventsGUI) save(events []event.Event) bool {
	if err := ui.store.Save(events); err != nil {
		ui.error = err.Error()
		return false
	}
	if err := ui.runtime.Replace(events); err != nil {
		ui.error = err.Error()
		return false
	}
	ui.events = events
	ui.error = ""
	ui.rebuildRowControls()
	return true
}

func (ui *eventsGUI) rebuildRowControls() {
	ui.activeClicks = make([]widget.Clickable, len(ui.events))
	ui.editClicks = make([]widget.Clickable, len(ui.events))
}

func (ui *eventsGUI) Layout(gtx layout.Context, theme *material.Theme) layout.Dimensions {
	switch ui.screen {
	case "editor":
		return ui.layoutEditor(gtx, theme)
	case "delete":
		return ui.layoutDelete(gtx, theme)
	case "icons":
		return ui.layoutIconsOverlay(gtx, theme)
	default:
		return ui.layoutList(gtx, theme)
	}
}

func (ui *eventsGUI) layoutList(gtx layout.Context, theme *material.Theme) layout.Dimensions {
	if ui.iconSetOpen {
		ui.deferSelectorBackdrop(gtx)
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					return labelWeight(gtx, theme, "Events", unit.Sp(25), palette.text, text.Start, font.SemiBold)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return guiEventActions(gtx, theme,
						guiEventAction{"Add spell", &ui.addSpellClick},
						guiEventAction{"Add timer", &ui.addTimerClick},
						guiEventAction{"Add text", &ui.addTextClick},
						guiEventAction{"Add regexp", &ui.addRegexClick},
						guiEventAction{"Extract icons", &ui.iconClick},
					)
				}),
			)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return inset(0, unit.Dp(14)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return ui.layoutVolume(gtx, theme)
			})
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return inset(0, unit.Dp(8)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return ui.layoutIconSetSelector(gtx, theme)
			})
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			if ui.error != "" {
				return guiEventMessage(gtx, theme, ui.error, color.NRGBA{R: 220, G: 150, B: 150, A: 255})
			}
			if ui.notice != "" {
				return guiEventMessage(gtx, theme, ui.notice, palette.success)
			}
			return layout.Dimensions{Size: image.Pt(0, gtx.Dp(unit.Dp(12)))}
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return ui.layoutEventRow(gtx, theme, -1)
		}),
		layout.Rigid(separator),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			if len(ui.events) == 0 {
				return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return label(gtx, theme, "No events configured.", unit.Sp(17), palette.muted, text.Middle)
				})
			}
			list := material.List(theme, &ui.list)
			list.AnchorStrategy = material.Occupy
			list.Indicator.Color = palette.muted
			list.Indicator.HoverColor = palette.text
			return list.Layout(gtx, len(ui.events), func(gtx layout.Context, index int) layout.Dimensions {
				return ui.layoutEventRow(gtx, theme, index)
			})
		}),
	)
}

func (ui *eventsGUI) layoutVolume(gtx layout.Context, theme *material.Theme) layout.Dimensions {
	percent := int(ui.volume.Value*100 + .5)
	display := fmt.Sprintf("%d%%", percent)
	if percent == 0 {
		display = "Muted"
	}
	return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			width := min(gtx.Constraints.Max.X, gtx.Dp(unit.Dp(105)))
			gtx.Constraints.Min.X, gtx.Constraints.Max.X = width, width
			return label(gtx, theme, "Sound volume", unit.Sp(15), palette.text, text.Start)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return (layout.Inset{Left: unit.Dp(14)}).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				width := min(gtx.Constraints.Max.X, gtx.Dp(unit.Dp(260)))
				gtx.Constraints.Min.X, gtx.Constraints.Max.X = width, width
				slider := material.Slider(theme, &ui.volume)
				slider.Color = palette.accent
				dimensions := slider.Layout(gtx)
				ui.applyAudioVolume()
				return dimensions
			})
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return (layout.Inset{Left: unit.Dp(12)}).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return labelWeight(gtx, theme, display, unit.Sp(15), palette.accent, text.End, font.SemiBold)
			})
		}),
	)
}

func (ui *eventsGUI) layoutIconSetSelector(gtx layout.Context, theme *material.Theme) layout.Dimensions {
	value := "Not extracted"
	if ui.iconSet != "" {
		value = ui.iconSet
	}
	return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			width := min(gtx.Constraints.Max.X, gtx.Dp(unit.Dp(105)))
			gtx.Constraints.Min.X, gtx.Constraints.Max.X = width, width
			return label(gtx, theme, "Spell icons", unit.Sp(15), palette.text, text.Start)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return (layout.Inset{Left: unit.Dp(14)}).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				width := min(gtx.Constraints.Max.X, gtx.Dp(unit.Dp(260)))
				gtx.Constraints.Min.X, gtx.Constraints.Max.X = width, width
				if len(ui.iconSets) == 0 {
					gtx = gtx.Disabled()
				}
				dimensions := guiEventSelector(theme, &ui.iconSetClick, value, ui.iconSetOpen)(gtx)
				if ui.iconSetOpen {
					ui.deferPicker(gtx, dimensions.Size, func(gtx layout.Context) layout.Dimensions {
						return ui.layoutIconSetOptions(gtx, theme)
					})
				}
				return dimensions
			})
		}),
	)
}

func (ui *eventsGUI) layoutIconSetOptions(gtx layout.Context, theme *material.Theme) layout.Dimensions {
	rowHeight := gtx.Dp(unit.Dp(40))
	height := min(rowHeight*len(ui.iconSets), gtx.Dp(unit.Dp(160)))
	gtx.Constraints.Min.Y, gtx.Constraints.Max.Y = height, height
	return outline(gtx, palette.accent, func(gtx layout.Context) layout.Dimensions {
		fill(gtx, palette.window)
		list := material.List(theme, &ui.iconSetList)
		list.AnchorStrategy = material.Occupy
		return list.Layout(gtx, len(ui.iconSets), func(gtx layout.Context, index int) layout.Dimensions {
			gtx.Constraints.Min.Y, gtx.Constraints.Max.Y = rowHeight, rowHeight
			return ui.iconSetChoices[index].Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				pointer.CursorPointer.Add(gtx.Ops)
				background, foreground := palette.panel, palette.text
				if index == ui.iconSetSelected {
					background, foreground = color.NRGBA{R: 70, G: 60, B: 34, A: 255}, palette.accent
				} else if ui.iconSetChoices[index].Hovered() {
					background = palette.panelAlt
				}
				fill(gtx, background)
				return layout.UniformInset(unit.Dp(9)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return label(gtx, theme, ui.iconSets[index], unit.Sp(15), foreground, text.Start)
				})
			})
		})
	})
}

func (ui *eventsGUI) deferPicker(gtx layout.Context, anchor image.Point, picker layout.Widget) {
	macro := op.Record(gtx.Ops)
	offset := op.Offset(image.Pt(0, anchor.Y)).Push(gtx.Ops)
	popup := gtx
	popup.Constraints.Min = image.Pt(anchor.X, 0)
	popup.Constraints.Max = image.Pt(anchor.X, gtx.Dp(unit.Dp(230)))
	picker(popup)
	offset.Pop()
	op.Defer(gtx.Ops, macro.Stop())
}

func (ui *eventsGUI) deferSelectorBackdrop(gtx layout.Context) {
	macro := op.Record(gtx.Ops)
	ui.selectorBackdrop.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Dimensions{Size: gtx.Constraints.Max}
	})
	op.Defer(gtx.Ops, macro.Stop())
}

func (ui *eventsGUI) applyAudioVolume() {
	if ui.volume.Value != ui.audioVolume {
		ui.audioVolume = ui.volume.Value
		ui.runtime.SetAudioVolume(float64(ui.audioVolume))
		ui.volumeDirty = true
	}
	if !ui.volumeDirty || ui.volume.Dragging() {
		return
	}
	ui.volumeDirty = false
	if err := ui.store.SaveAudioVolume(float64(ui.audioVolume)); err != nil {
		ui.error = err.Error()
	}
}

func (ui *eventsGUI) layoutEventRow(gtx layout.Context, theme *material.Theme, index int) layout.Dimensions {
	header := index < 0
	height := gtx.Dp(unit.Dp(42))
	gtx.Constraints.Min.Y, gtx.Constraints.Max.Y = height, height
	if header {
		fill(gtx, palette.chrome)
	}
	values := []string{"ACTIVE", "TITLE", "TYPE", "NOTIFICATION", "SOUND"}
	active := true
	if !header {
		configured := ui.events[index]
		values[0] = "No"
		if configured.Active {
			values[0] = "Yes"
		}
		values[1] = configured.Title
		values[2] = strings.ReplaceAll(string(configured.TriggerType), "_", " ")
		values[3] = "No"
		if configured.Notification != "" {
			values[3] = "Yes"
		}
		values[4] = ui.soundLabel(configured.Sound)
		active = configured.Active
	}
	foreground := palette.text
	if header || !active {
		foreground = palette.muted
	}
	cell := func(value string, weight float32) layout.FlexChild {
		return layout.Flexed(weight, func(gtx layout.Context) layout.Dimensions {
			return label(gtx, theme, value, unit.Sp(15), foreground, text.Start)
		})
	}
	content := func(gtx layout.Context) layout.Dimensions {
		return inset(unit.Dp(10), 0).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
				cell(values[1], 3.1), cell(values[2], 1.1), cell(values[3], 1.5), cell(values[4], 2.1),
			)
		})
	}
	return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			width := min(gtx.Dp(unit.Dp(78)), max(gtx.Constraints.Max.X/7, gtx.Dp(unit.Dp(54))))
			gtx.Constraints.Min.X, gtx.Constraints.Max.X = width, width
			if header {
				return inset(unit.Dp(10), 0).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return label(gtx, theme, values[0], unit.Sp(14), foreground, text.Start)
				})
			}
			return ui.activeClicks[index].Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				pointer.CursorPointer.Add(gtx.Ops)
				return inset(unit.Dp(10), 0).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return label(gtx, theme, values[0], unit.Sp(15), foreground, text.Start)
				})
			})
		}),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			if header {
				return content(gtx)
			}
			return ui.editClicks[index].Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				pointer.CursorPointer.Add(gtx.Ops)
				if ui.editClicks[index].Hovered() {
					fill(gtx, palette.panel)
				}
				return content(gtx)
			})
		}),
	)
}

func (ui *eventsGUI) layoutEditor(gtx layout.Context, theme *material.Theme) layout.Dimensions {
	if ui.picker != "" {
		ui.deferSelectorBackdrop(gtx)
	}
	title := "Add event"
	if ui.editingID != "" {
		title = "Edit event"
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return labelWeight(gtx, theme, title, unit.Sp(25), palette.text, text.Start, font.SemiBold)
		}),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			list := material.List(theme, &ui.editorList)
			list.AnchorStrategy = material.Occupy
			items := ui.editorItems()
			return list.Layout(gtx, len(items), func(gtx layout.Context, index int) layout.Dimensions {
				return ui.layoutEditorItem(gtx, theme, items[index])
			})
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			actions := []guiEventAction{{"Save", &ui.saveClick}}
			if ui.editingID != "" {
				actions = append(actions, guiEventAction{"Delete", &ui.deleteClick})
			}
			actions = append(actions, guiEventAction{"Cancel", &ui.cancelClick})
			return layout.E.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return guiEventActions(gtx, theme, actions...)
			})
		}),
	)
}

func (ui *eventsGUI) editorItems() []string {
	items := []string{"title"}
	if ui.editingKind == event.TriggerSpell || ui.editingKind == event.TriggerSpellTimer {
		items = append(items, "class", "spell")
		if ui.editingKind == event.TriggerSpellTimer {
			items = append(items, "timer-seconds")
		}
	} else {
		items = append(items, "pattern")
		if ui.editingKind == event.TriggerText {
			items = append(items, "exact")
		}
	}
	items = append(items, "notification", "persistence", "sound", "sound-preview")
	if ui.error != "" {
		items = append(items, "error")
	}
	return items
}

func (ui *eventsGUI) layoutEditorItem(gtx layout.Context, theme *material.Theme, item string) layout.Dimensions {
	switch item {
	case "title":
		return guiEventEditorRow(gtx, theme, "Title", guiEventEditor(theme, &ui.titleEditor))
	case "pattern":
		labelText := "Text"
		if ui.editingKind == event.TriggerRegexp {
			labelText = "Regular expression"
		}
		return guiEventEditorRow(gtx, theme, labelText, guiEventEditor(theme, &ui.patternEditor))
	case "notification":
		return guiEventEditorRow(gtx, theme, "Notification", guiEventEditor(theme, &ui.notificationEditor))
	case "timer-seconds":
		return guiEventEditorRow(gtx, theme, "Duration (seconds)", guiEventEditor(theme, &ui.timerSecondsEditor))
	case "exact":
		return guiEventEditorRow(gtx, theme, "", func(gtx layout.Context) layout.Dimensions {
			return material.CheckBox(theme, &ui.exactCheck, "Full message match").Layout(gtx)
		})
	case "persistence":
		return guiEventEditorRow(gtx, theme, "", func(gtx layout.Context) layout.Dimensions {
			return material.CheckBox(theme, &ui.persistenceCheck, "Request persistent notification").Layout(gtx)
		})
	case "class":
		value := "ALL"
		if ui.classSelected > 0 && ui.classSelected-1 < len(ui.classes) {
			value = ui.classes[ui.classSelected-1]
		}
		return guiEventEditorRow(gtx, theme, "Class", ui.editorSelector(theme, &ui.classClick, value, "class"))
	case "spell":
		value := "[No spells]"
		if len(ui.visibleSpells) > 0 && ui.spellSelected < len(ui.visibleSpells) {
			value = ui.visibleSpells[ui.spellSelected].Name
		}
		return guiEventEditorRow(gtx, theme, "Spell", ui.editorSelector(theme, &ui.spellClick, value, "spell"))
	case "sound":
		return guiEventEditorRow(gtx, theme, "Sound", ui.editorSelector(theme, &ui.soundClick, ui.sounds[ui.soundSelected].Label, "sound"))
	case "sound-preview":
		return guiEventEditorRow(gtx, theme, "", func(gtx layout.Context) layout.Dimensions {
			if ui.soundSelected <= 0 || ui.soundSelected >= len(ui.sounds) {
				gtx = gtx.Disabled()
			}
			return layout.W.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return guiEventActions(gtx, theme, guiEventAction{"Play", &ui.playClick})
			})
		})
	case "error":
		return guiEventMessage(gtx, theme, ui.error, color.NRGBA{R: 220, G: 150, B: 150, A: 255})
	default:
		return layout.Dimensions{}
	}
}

func (ui *eventsGUI) editorSelector(theme *material.Theme, click *widget.Clickable, value, kind string) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		dimensions := guiEventSelector(theme, click, value, ui.picker == kind)(gtx)
		if ui.picker == kind {
			ui.deferPicker(gtx, dimensions.Size, func(gtx layout.Context) layout.Dimensions {
				return ui.layoutPicker(gtx, theme)
			})
		}
		return dimensions
	}
}

func (ui *eventsGUI) layoutPicker(gtx layout.Context, theme *material.Theme) layout.Dimensions {
	options := ui.pickerOptions()
	selected := ui.pickerSelected()
	rowHeight := gtx.Dp(unit.Dp(42))
	height := min(rowHeight*len(options), gtx.Dp(unit.Dp(230)))
	height = max(height, min(gtx.Dp(unit.Dp(120)), gtx.Constraints.Max.Y))
	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	gtx.Constraints.Min.Y, gtx.Constraints.Max.Y = height, height
	return outline(gtx, palette.accent, func(gtx layout.Context) layout.Dimensions {
		fill(gtx, palette.window)
		list := material.List(theme, &ui.pickerList)
		list.AnchorStrategy = material.Occupy
		return list.Layout(gtx, len(options), func(gtx layout.Context, index int) layout.Dimensions {
			gtx.Constraints.Min.Y, gtx.Constraints.Max.Y = rowHeight, rowHeight
			return ui.choices[index].Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				pointer.CursorPointer.Add(gtx.Ops)
				background, foreground := palette.panel, palette.text
				if index == selected {
					background, foreground = color.NRGBA{R: 70, G: 60, B: 34, A: 255}, palette.accent
				} else if ui.choices[index].Hovered() {
					background = palette.panelAlt
				}
				fill(gtx, background)
				return layout.UniformInset(unit.Dp(9)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return label(gtx, theme, options[index], unit.Sp(15), foreground, text.Start)
				})
			})
		})
	})
}

func (ui *eventsGUI) layoutDelete(gtx layout.Context, theme *material.Theme) layout.Dimensions {
	return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical, Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return labelWeight(gtx, theme, "Delete this event?", unit.Sp(24), palette.text, text.Middle, font.SemiBold)
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return inset(0, unit.Dp(18)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return guiEventActions(gtx, theme,
						guiEventAction{"Delete", &ui.deleteConfirm},
						guiEventAction{"Cancel", &ui.deleteCancel},
					)
				})
			}),
		)
	})
}

func (ui *eventsGUI) layoutIconsOverlay(gtx layout.Context, theme *material.Theme) layout.Dimensions {
	return layout.Stack{}.Layout(gtx,
		layout.Expanded(func(gtx layout.Context) layout.Dimensions {
			return ui.layoutList(gtx, theme)
		}),
		layout.Expanded(func(gtx layout.Context) layout.Dimensions {
			return ui.iconBackdrop.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				fill(gtx, color.NRGBA{A: 185})
				return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					margin := gtx.Dp(unit.Dp(24))
					gtx.Constraints.Min = image.Point{}
					gtx.Constraints.Max.X = max(0, gtx.Constraints.Max.X-margin*2)
					gtx.Constraints.Max.Y = max(0, gtx.Constraints.Max.Y-margin*2)
					return ui.layoutIconsCard(gtx, theme)
				})
			})
		}),
	)
}

func (ui *eventsGUI) layoutIconsCard(gtx layout.Context, theme *material.Theme) layout.Dimensions {
	body := "Extract every distinct spell icon set from this EverQuest installation for use in notifications."
	if ui.iconBusy {
		body = "Extracting spell icons…"
	}
	if ui.iconAutomatic && !ui.iconBusy {
		body += "\n\n“Ask next time” leaves setup undecided. “Don’t ask again” stops automatic prompts; setup remains available from Events."
	}
	maxWidth := min(gtx.Constraints.Max.X, gtx.Dp(unit.Dp(650)))
	gtx.Constraints.Min = image.Point{}
	gtx.Constraints.Max.X = maxWidth
	return widget.Border{Color: palette.line, Width: unit.Dp(1)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Background{}.Layout(gtx,
			func(gtx layout.Context) layout.Dimensions {
				fill(gtx, palette.panel)
				return layout.Dimensions{Size: gtx.Constraints.Max}
			},
			func(gtx layout.Context) layout.Dimensions {
				return layout.UniformInset(unit.Dp(28)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Vertical, Alignment: layout.Middle}.Layout(gtx,
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return labelWeight(gtx, theme, "Spell icons", unit.Sp(24), palette.text, text.Middle, font.SemiBold)
						}),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return inset(0, unit.Dp(18)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
								return label(gtx, theme, body, unit.Sp(16), palette.muted, text.Middle)
							})
						}),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							if ui.error == "" {
								return layout.Dimensions{}
							}
							return guiEventMessage(gtx, theme, ui.error, color.NRGBA{R: 220, G: 150, B: 150, A: 255})
						}),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							if ui.iconBusy {
								return layout.Dimensions{}
							}
							return guiEventActions(gtx, theme, ui.iconActions()...)
						}),
					)
				})
			},
		)
	})
}

func (ui *eventsGUI) iconActions() []guiEventAction {
	actions := []guiEventAction{{"Extract icons", &ui.iconExtract}}
	if ui.iconAutomatic {
		return append(actions,
			guiEventAction{"Ask next time", &ui.iconCancel},
			guiEventAction{"Don't ask again", &ui.iconDecline},
		)
	}
	return append(actions, guiEventAction{"Close", &ui.iconCancel})
}

func (ui *eventsGUI) soundLabel(id string) string {
	if id == "" {
		return "No Sound"
	}
	if index := guiEventSoundIndex(ui.sounds, id); index >= 0 {
		return ui.sounds[index].Label
	}
	return id
}

type guiEventAction struct {
	label string
	click *widget.Clickable
}

func guiEventActions(gtx layout.Context, theme *material.Theme, actions ...guiEventAction) layout.Dimensions {
	children := make([]layout.FlexChild, 0, len(actions))
	for _, action := range actions {
		action := action
		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return inset(unit.Dp(4), 0).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return eqldbDialogButton(gtx, theme, action.click, action.label, action.label == "Save")
			})
		}))
	}
	return layout.Flex{Alignment: layout.Middle}.Layout(gtx, children...)
}

func guiEventEditorRow(gtx layout.Context, theme *material.Theme, name string, control layout.Widget) layout.Dimensions {
	height := gtx.Dp(unit.Dp(56))
	gtx.Constraints.Min.Y, gtx.Constraints.Max.Y = height, height
	return inset(unit.Dp(4), unit.Dp(5)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		labelWidth := min(gtx.Dp(unit.Dp(145)), gtx.Constraints.Max.X/3)
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				gtx.Constraints.Min.X, gtx.Constraints.Max.X = labelWidth, labelWidth
				return label(gtx, theme, name, unit.Sp(15), palette.text, text.Start)
			}),
			layout.Flexed(1, control),
		)
	})
}

func guiEventEditor(theme *material.Theme, editor *widget.Editor) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		style := material.Editor(theme, editor, "")
		style.TextSize = unit.Sp(16)
		style.Color = palette.text
		return outline(gtx, palette.line, func(gtx layout.Context) layout.Dimensions {
			fill(gtx, palette.window)
			return layout.UniformInset(unit.Dp(8)).Layout(gtx, style.Layout)
		})
	}
}

func guiEventSelector(theme *material.Theme, click *widget.Clickable, value string, open bool) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		return eqldbSelector(gtx, theme, click, value, open)
	}
}

func guiEventMessage(gtx layout.Context, theme *material.Theme, message string, foreground color.NRGBA) layout.Dimensions {
	return inset(unit.Dp(4), unit.Dp(8)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return label(gtx, theme, message, unit.Sp(14), foreground, text.Start)
	})
}

func guiEventClasses(spells []catalog.Spell) []string {
	seen := make(map[string]bool)
	for _, spell := range spells {
		for _, className := range spell.Classes {
			seen[className] = true
		}
	}
	classes := make([]string, 0, len(seen))
	for className := range seen {
		classes = append(classes, className)
	}
	sort.Strings(classes)
	return classes
}

func guiEventSoundIndex(sounds []audio.Sound, id string) int {
	if id == "" {
		return 0
	}
	for index, sound := range sounds {
		if sound.ID == id {
			return index
		}
	}
	return -1
}

func guiEventStringIndex(values []string, wanted string) int {
	for index, value := range values {
		if value == wanted {
			return index
		}
	}
	return -1
}

func guiEventContains(values []string, wanted string) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}
