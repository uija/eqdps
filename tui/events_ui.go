package main

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/uija/eqdps/internal/audio"
	"github.com/uija/eqdps/internal/catalog"
	"github.com/uija/eqdps/internal/event"
	"github.com/uija/eqdps/internal/eventruntime"
	"github.com/uija/eqdps/internal/eventstore"
	"github.com/uija/eqdps/internal/spellicon"
)

const (
	eventsPageName  = "events"
	eventsModalName = "events-modal"
)

type eventsTUI struct {
	app         *tview.Application
	pages       *tview.Pages
	mainFocus   tview.Primitive
	store       *eventstore.Store
	runtime     *eventruntime.Runtime
	logPath     string
	spells      []catalog.Spell
	events      []event.Event
	sounds      []audio.Sound
	table       *tview.Table
	status      *tview.TextView
	layout      tview.Primitive
	open        bool
	modalOpen   bool
	iconSetup   eventstore.IconSetup
	iconSets    []string
	iconSet     string
	audioVolume float64
}

func newEventsTUI(
	app *tview.Application,
	pages *tview.Pages,
	mainFocus tview.Primitive,
	store *eventstore.Store,
	runtime *eventruntime.Runtime,
	logPath string,
) (*eventsTUI, error) {
	spells, err := catalog.Load()
	if err != nil {
		return nil, err
	}
	events, err := store.Load()
	if err != nil {
		return nil, err
	}
	sounds, err := eventSoundList(store, events)
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
	ui := &eventsTUI{
		app:         app,
		pages:       pages,
		mainFocus:   mainFocus,
		store:       store,
		runtime:     runtime,
		logPath:     logPath,
		spells:      spells,
		events:      events,
		sounds:      sounds,
		table:       tview.NewTable().SetBorders(false).SetSelectable(true, false).SetFixed(1, 0),
		status:      tview.NewTextView().SetDynamicColors(true),
		iconSetup:   iconSetup,
		audioVolume: volume,
	}
	if err := ui.reloadIconSets(); err != nil {
		return nil, err
	}
	ui.runtime.SetAudioVolume(volume)
	ui.table.SetBorder(true).SetTitle(" Configured Events ")
	ui.table.SetInputCapture(ui.captureTableInput)
	help := tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignCenter).
		SetText("[gray]a[::-] active   [gray]Enter[::-] edit   [gray]d[::-] delete   [gray]s/t/r[::-] add spell/text/regexp   [gray]v[::-] settings   [gray]i[::-] extract icons   [gray]q/Esc[::-] DPS")
	ui.layout = tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(ui.table, 0, 1, true).
		AddItem(help, 1, 0, false).
		AddItem(ui.status, 1, 0, false)
	ui.render()
	return ui, nil
}

func eventSoundList(store *eventstore.Store, events []event.Event) ([]audio.Sound, error) {
	embedded, err := audio.EmbeddedSounds()
	if err != nil {
		return nil, err
	}
	userFiles, err := store.Sounds()
	if err != nil {
		return nil, err
	}
	sounds := []audio.Sound{{Label: "[No Sound]"}}
	sounds = append(sounds, embedded...)
	for _, filename := range userFiles {
		sounds = append(sounds, audio.Sound{ID: audio.UserPrefix + filename, Label: filename + " (user)"})
	}
	for _, configured := range events {
		if configured.Sound != "" && !eventSoundExists(sounds, configured.Sound) {
			sounds = append(sounds, audio.Sound{ID: configured.Sound, Label: configured.Sound + " (missing)"})
		}
	}
	return sounds, nil
}

func (ui *eventsTUI) Open() {
	if ui == nil {
		return
	}
	if events, err := ui.store.Load(); err != nil {
		ui.status.SetText("[red]" + tview.Escape(err.Error()) + "[::-]")
	} else {
		ui.events = events
		_ = ui.runtime.Replace(events)
		if sounds, soundErr := eventSoundList(ui.store, events); soundErr == nil {
			ui.sounds = sounds
		}
	}
	if iconSetup, err := ui.store.SpellIconSetup(); err == nil {
		ui.iconSetup = iconSetup
	}
	if volume, err := ui.store.AudioVolume(); err != nil {
		ui.setError(err.Error())
	} else {
		ui.audioVolume = volume
		ui.runtime.SetAudioVolume(volume)
	}
	if err := ui.reloadIconSets(); err != nil {
		ui.setError(err.Error())
	}
	ui.render()
	ui.open = true
	ui.pages.AddAndSwitchToPage(eventsPageName, ui.layout, true)
	ui.app.SetFocus(ui.table)
	ui.table.ScrollToBeginning()
	if len(ui.events) > 0 {
		ui.table.Select(1, 0)
	}
	if shouldPromptSpellIcons(ui.iconSetup, ui.logPath) {
		ui.showIconPrompt(false)
	}
}

func shouldPromptSpellIcons(state eventstore.IconSetup, logPath string) bool {
	return state == eventstore.IconSetupUnknown && logPath != ""
}

func (ui *eventsTUI) Opened() bool {
	return ui != nil && ui.open
}

func (ui *eventsTUI) HandleGlobal(key *tcell.EventKey) bool {
	if ui == nil || !ui.open || key.Key() != tcell.KeyEscape {
		return false
	}
	if ui.modalOpen {
		ui.closeModal()
	} else {
		ui.close()
	}
	return true
}

func (ui *eventsTUI) close() {
	if ui.modalOpen {
		return
	}
	ui.pages.RemovePage(eventsPageName)
	ui.open = false
	ui.app.SetFocus(ui.mainFocus)
}

func (ui *eventsTUI) captureTableInput(key *tcell.EventKey) *tcell.EventKey {
	if key.Key() == tcell.KeyEscape {
		ui.close()
		return nil
	}
	if key.Key() == tcell.KeyEnter {
		ui.editSelected()
		return nil
	}
	switch key.Rune() {
	case 'q', 'Q':
		ui.close()
	case 'a', 'A':
		ui.toggleSelected()
	case 'd', 'D':
		ui.confirmDelete()
	case 's', 'S':
		ui.showSpellEditor(nil)
	case 't', 'T':
		ui.showPatternEditor(event.TriggerText, nil)
	case 'r', 'R':
		ui.showPatternEditor(event.TriggerRegexp, nil)
	case 'i', 'I':
		ui.showIconPrompt(true)
	case 'v', 'V':
		ui.showSettings()
	default:
		return key
	}
	return nil
}

func (ui *eventsTUI) showSettings() {
	volume := tview.NewInputField().
		SetText(strconv.Itoa(int(ui.audioVolume*100 + .5))).
		SetFieldWidth(4).
		SetAcceptanceFunc(tview.InputFieldInteger)
	iconOptions := append([]string(nil), ui.iconSets...)
	iconSelection := eventStringIndex(iconOptions, ui.iconSet)
	if iconSelection < 0 {
		iconSelection = 0
	}
	if len(iconOptions) == 0 {
		iconOptions = []string{"Not extracted"}
	}
	iconSet := tview.NewDropDown().SetOptions(iconOptions, nil).SetCurrentOption(iconSelection)
	if len(ui.iconSets) == 0 {
		iconSet.SetDisabled(true)
	}
	const volumeHint = "[gray]Arrow keys adjust volume by 5%.[::-]"
	message := tview.NewTextView().SetDynamicColors(true).SetText(volumeHint)
	save := tview.NewButton("Save")
	cancel := tview.NewButton("Cancel")
	cancel.SetSelectedFunc(ui.closeModal)
	save.SetSelectedFunc(func() {
		percent, err := strconv.Atoi(volume.GetText())
		if err != nil || percent < 0 || percent > 100 {
			message.SetText("[red]Enter a volume from 0 to 100.[::-]")
			return
		}
		value := float64(percent) / 100
		if err := ui.store.SaveAudioVolume(value); err != nil {
			message.SetText("[red]" + tview.Escape(err.Error()) + "[::-]")
			return
		}
		selectedIconSet := ui.iconSet
		if len(ui.iconSets) > 0 {
			index, _ := iconSet.GetCurrentOption()
			if index < 0 || index >= len(ui.iconSets) {
				message.SetText("[red]Select a spell icon set.[::-]")
				return
			}
			selectedIconSet = ui.iconSets[index]
			if err := ui.store.SaveSpellIconSet(selectedIconSet); err != nil {
				message.SetText("[red]" + tview.Escape(err.Error()) + "[::-]")
				return
			}
		}
		ui.audioVolume = value
		ui.runtime.SetAudioVolume(value)
		ui.iconSet = selectedIconSet
		ui.runtime.SetSpellIconSet(selectedIconSet)
		ui.status.SetText("[green]Event settings saved[::-]")
		ui.closeModal()
	})
	body := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(nil, 1, 0, false).
		AddItem(eventFieldRow("Volume (0–100%)", volume), 1, 0, true).
		AddItem(eventFieldRow("Spell icons", iconSet), 1, 0, false).
		AddItem(message, 1, 0, false).
		AddItem(eventButtonRow(save, cancel), 1, 0, false).
		AddItem(nil, 1, 0, false)
	body.SetInputCapture(func(key *tcell.EventKey) *tcell.EventKey {
		if ui.app.GetFocus() != volume {
			return key
		}
		percent, handled := adjustVolumeWithArrow(key.Key(), volume.GetText(), ui.audioVolume)
		if !handled {
			return key
		}
		volume.SetText(strconv.Itoa(percent))
		message.SetText(volumeHint)
		return nil
	})
	focus := []tview.Primitive{volume}
	if len(ui.iconSets) > 0 {
		focus = append(focus, iconSet)
	}
	focus = append(focus, save, cancel)
	ui.openFormModal(" Event Settings ", body, 64, 9, focus, volume)
}

func (ui *eventsTUI) reloadIconSets() error {
	sets, err := ui.store.SpellIconSets()
	if err != nil {
		return err
	}
	selected, err := ui.store.SpellIconSet()
	if err != nil {
		return err
	}
	if !eventContains(sets, selected) {
		selected = ""
		if len(sets) > 0 {
			selected = sets[0]
		}
	}
	ui.iconSets = sets
	ui.iconSet = selected
	ui.runtime.SetSpellIconSet(selected)
	return nil
}

func eventStringIndex(values []string, wanted string) int {
	for index, value := range values {
		if value == wanted {
			return index
		}
	}
	return -1
}

func adjustVolumeWithArrow(key tcell.Key, text string, fallback float64) (int, bool) {
	delta := 0
	switch key {
	case tcell.KeyUp, tcell.KeyRight:
		delta = 5
	case tcell.KeyDown, tcell.KeyLeft:
		delta = -5
	default:
		return 0, false
	}
	percent, err := strconv.Atoi(text)
	if err != nil {
		percent = int(fallback*100 + .5)
	}
	return min(max(percent+delta, 0), 100), true
}

func (ui *eventsTUI) render() {
	row, _ := ui.table.GetSelection()
	ui.table.Clear()
	for column, title := range []string{"Active", "Title", "Type", "Notification", "Sound"} {
		ui.table.SetCell(0, column, tview.NewTableCell(title).
			SetTextColor(tcell.ColorYellow).
			SetAttributes(tcell.AttrBold).
			SetSelectable(false))
	}
	for index, configured := range ui.events {
		active := "No"
		if configured.Active {
			active = "Yes"
		}
		notification := "No"
		if configured.Notification != "" {
			notification = "Yes"
		}
		sound := "No Sound"
		if configured.Sound != "" {
			sound = ui.soundLabel(configured.Sound)
		}
		cells := []*tview.TableCell{
			tview.NewTableCell(active).SetMaxWidth(7),
			tview.NewTableCell(configured.Title).SetExpansion(1),
			tview.NewTableCell(string(configured.TriggerType)).SetMaxWidth(8),
			tview.NewTableCell(notification).SetMaxWidth(12),
			tview.NewTableCell(sound).SetMaxWidth(24),
		}
		if !configured.Active {
			for _, cell := range cells {
				cell.SetTextColor(tcell.ColorGray)
			}
		}
		for column, cell := range cells {
			ui.table.SetCell(index+1, column, cell)
		}
	}
	if len(ui.events) > 0 {
		ui.table.Select(min(max(row, 1), len(ui.events)), 0)
	}
}

func (ui *eventsTUI) selected() (*event.Event, bool) {
	row, _ := ui.table.GetSelection()
	index := row - 1
	if index < 0 || index >= len(ui.events) {
		ui.setError("No event selected")
		return nil, false
	}
	return &ui.events[index], true
}

func (ui *eventsTUI) toggleSelected() {
	selected, ok := ui.selected()
	if !ok {
		return
	}
	events := append([]event.Event(nil), ui.events...)
	for index := range events {
		if events[index].ID == selected.ID {
			events[index].Active = !events[index].Active
			if err := ui.saveAll(events); err != nil {
				ui.setError(err.Error())
				return
			}
			state := "inactive"
			if events[index].Active {
				state = "active"
			}
			ui.status.SetText(fmt.Sprintf("[green]%q is now %s[::-]", events[index].Title, state))
			return
		}
	}
}

func (ui *eventsTUI) editSelected() {
	selected, ok := ui.selected()
	if !ok {
		return
	}
	copy := *selected
	switch copy.TriggerType {
	case event.TriggerSpell:
		ui.showSpellEditor(&copy)
	case event.TriggerText, event.TriggerRegexp:
		ui.showPatternEditor(copy.TriggerType, &copy)
	}
}

func (ui *eventsTUI) confirmDelete() {
	selected, ok := ui.selected()
	if !ok {
		return
	}
	id, title := selected.ID, selected.Title
	modal := tview.NewModal().
		SetText(fmt.Sprintf("Delete %q?", title)).
		AddButtons([]string{"Delete", "Cancel"}).
		SetDoneFunc(func(_ int, label string) {
			if label == "Delete" {
				events := make([]event.Event, 0, len(ui.events)-1)
				for _, configured := range ui.events {
					if configured.ID != id {
						events = append(events, configured)
					}
				}
				if err := ui.saveAll(events); err != nil {
					ui.setError(err.Error())
				} else {
					ui.status.SetText(fmt.Sprintf("[green]Deleted %q[::-]", title))
				}
			}
			ui.closeModal()
		})
	modal.SetInputCapture(func(key *tcell.EventKey) *tcell.EventKey {
		if key.Key() == tcell.KeyEscape {
			ui.closeModal()
			return nil
		}
		return key
	})
	ui.openModalPrimitive(" Confirm Delete ", modal, 56, 9, modal)
}

func (ui *eventsTUI) showPatternEditor(kind event.TriggerType, existing *event.Event) {
	title := tview.NewInputField()
	pattern := tview.NewInputField()
	exact := tview.NewCheckbox().SetLabel("Full message match").SetChecked(true)
	notification := tview.NewInputField()
	persistence := tview.NewCheckbox().SetLabel("Request persistent notification")
	sound := ui.soundDropDown("")
	message := tview.NewTextView().SetDynamicColors(true)
	active := true
	modalTitle := " Add Text Event "
	eventBindAutomaticTitle(title, pattern)
	if kind == event.TriggerRegexp {
		modalTitle = " Add Regular-Expression Event "
	}
	if existing != nil {
		active = existing.Active
		title.SetText(existing.Title)
		pattern.SetText(existing.Pattern)
		exact.SetChecked(existing.ExactMatch)
		notification.SetText(existing.Notification)
		persistence.SetChecked(existing.RequestPersistence)
		sound = ui.soundDropDown(existing.Sound)
		modalTitle = strings.Replace(modalTitle, "Add", "Edit", 1)
	}
	save := tview.NewButton("Save")
	cancel := tview.NewButton("Cancel")
	cancel.SetSelectedFunc(ui.closeModal)
	save.SetSelectedFunc(func() {
		value := pattern.GetText()
		if value == "" {
			message.SetText("[red]Enter a pattern to match.[::-]")
			return
		}
		if kind == event.TriggerRegexp {
			if _, err := regexp.Compile(value); err != nil {
				message.SetText("[red]Invalid regular expression: " + tview.Escape(err.Error()) + "[::-]")
				return
			}
		}
		name := strings.TrimSpace(title.GetText())
		if name == "" {
			name = value
		}
		configured := event.Event{
			Title: name, Active: active, TriggerType: kind, Pattern: value,
			ExactMatch: exact.IsChecked(), Notification: notification.GetText(),
			RequestPersistence: persistence.IsChecked(), Sound: ui.selectedSound(sound),
		}
		if existing != nil {
			configured.ID = existing.ID
		}
		if err := ui.saveOne(configured); err != nil {
			message.SetText("[red]" + tview.Escape(err.Error()) + "[::-]")
			return
		}
		ui.closeModal()
	})
	rows := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(eventFieldRow("Title", title), 1, 0, false).
		AddItem(eventFieldRow("Pattern", pattern), 1, 0, false)
	focus := []tview.Primitive{title, pattern}
	if kind == event.TriggerText {
		rows.AddItem(eventFieldRow("", exact), 1, 0, false)
		focus = append(focus, exact)
	}
	rows.
		AddItem(eventFieldRow("Notification", notification), 1, 0, false).
		AddItem(eventFieldRow("", persistence), 1, 0, false).
		AddItem(eventFieldRow("Sound", sound), 1, 0, false).
		AddItem(message, 2, 0, false).
		AddItem(eventButtonRow(save, cancel), 1, 0, false)
	focus = append(focus, notification, persistence, sound, save, cancel)
	ui.openFormModal(modalTitle, rows, 76, 12, focus, pattern)
}

func (ui *eventsTUI) showSpellEditor(existing *event.Event) {
	title := tview.NewInputField()
	notification := tview.NewInputField().SetText("%s has faded.")
	persistence := tview.NewCheckbox().SetLabel("Request persistent notification")
	sound := ui.soundDropDown("")
	classes := tview.NewList().ShowSecondaryText(false)
	spells := tview.NewList().ShowSecondaryText(false)
	message := tview.NewTextView().SetDynamicColors(true)
	active := true
	modalTitle := " Add Spell Event "
	if existing != nil {
		active = existing.Active
		title.SetText(existing.Title)
		notification.SetText(existing.Notification)
		persistence.SetChecked(existing.RequestPersistence)
		sound = ui.soundDropDown(existing.Sound)
		modalTitle = " Edit Spell Event "
	}
	classes.AddItem("ALL", "", 0, nil)
	for _, className := range eventAvailableClasses(ui.spells) {
		classes.AddItem(className, "", 0, nil)
	}
	var visible []catalog.Spell
	updateSpells := func(className string) {
		spells.Clear()
		visible = visible[:0]
		for _, spell := range ui.spells {
			if className == "ALL" || eventContains(spell.Classes, className) {
				visible = append(visible, spell)
				spells.AddItem(spell.Name, "", 0, nil)
			}
		}
	}
	classes.SetChangedFunc(func(_ int, name, _ string, _ rune) { updateSpells(name) })
	updateSpells("ALL")
	if existing != nil {
		for index := range visible {
			if visible[index].Name == existing.SpellName {
				spells.SetCurrentItem(index)
				break
			}
		}
	}
	save := tview.NewButton("Save")
	cancel := tview.NewButton("Cancel")
	cancel.SetSelectedFunc(ui.closeModal)
	save.SetSelectedFunc(func() {
		index := spells.GetCurrentItem()
		if index < 0 || index >= len(visible) {
			message.SetText("[red]Select a spell.[::-]")
			return
		}
		spell := visible[index]
		name := strings.TrimSpace(title.GetText())
		if name == "" {
			name = spell.Name
		}
		configured := event.Event{
			Title: name, Active: active, TriggerType: event.TriggerSpell,
			Pattern: spell.FadeMessage, SpellName: spell.Name,
			Notification: notification.GetText(), RequestPersistence: persistence.IsChecked(),
			Sound: ui.selectedSound(sound),
		}
		if existing != nil {
			configured.ID = existing.ID
		}
		if err := ui.saveOne(configured); err != nil {
			message.SetText("[red]" + tview.Escape(err.Error()) + "[::-]")
			return
		}
		ui.closeModal()
	})
	classes.SetBorder(true).SetTitle(" Class ")
	spells.SetBorder(true).SetTitle(" Spell ")
	lists := tview.NewFlex().AddItem(classes, 22, 0, true).AddItem(spells, 0, 1, false)
	body := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(eventFieldRow("Title", title), 1, 0, false).
		AddItem(lists, 0, 1, true).
		AddItem(eventFieldRow("Notification", notification), 1, 0, false).
		AddItem(eventFieldRow("", persistence), 1, 0, false).
		AddItem(eventFieldRow("Sound", sound), 1, 0, false).
		AddItem(message, 1, 0, false).
		AddItem(eventButtonRow(save, cancel), 1, 0, false)
	ui.openFormModal(modalTitle, body, 80, 24,
		[]tview.Primitive{title, classes, spells, notification, persistence, sound, save, cancel}, classes)
}

func (ui *eventsTUI) saveOne(configured event.Event) error {
	events := append([]event.Event(nil), ui.events...)
	if configured.ID == "" {
		id, err := event.NewID()
		if err != nil {
			return err
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
			return fmt.Errorf("event no longer exists")
		}
	}
	if err := ui.saveAll(events); err != nil {
		return err
	}
	ui.status.SetText(fmt.Sprintf("[green]Saved %q[::-]", configured.Title))
	return nil
}

func (ui *eventsTUI) saveAll(events []event.Event) error {
	if err := ui.store.Save(events); err != nil {
		return err
	}
	if err := ui.runtime.Replace(events); err != nil {
		return err
	}
	ui.events = events
	ui.render()
	return nil
}

func (ui *eventsTUI) showIconPrompt(manual bool) {
	source, detected := spellicon.Detect(ui.logPath, ui.spells)
	if !detected {
		if manual {
			ui.setError("Could not locate the EverQuest spell-icon files from this logfile")
		}
		return
	}
	message := tview.NewTextView().
		SetTextAlign(tview.AlignCenter).
		SetWordWrap(true).
		SetText("Extract all distinct EverQuest spell icon sets for notifications?")
	yes := tview.NewButton("Extract")
	noLabel := "Don't ask again"
	if manual {
		noLabel = "Close"
	}
	no := tview.NewButton(noLabel)
	yes.SetSelectedFunc(func() {
		yes.SetDisabled(true)
		no.SetDisabled(true)
		message.SetTextColor(tcell.ColorYellow).SetText("Extracting spell icons…")
		go func() {
			names, err := spellicon.ExtractAll(source, ui.store.IconDir(), ui.spells)
			ui.app.QueueUpdateDraw(func() {
				if err == nil && len(names) == 0 {
					err = fmt.Errorf("no distinct spell icon sets found")
				}
				if err == nil {
					selected := ui.iconSet
					if !eventContains(names, selected) {
						selected = names[0]
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
					yes.SetDisabled(false)
					no.SetDisabled(false)
					message.SetTextColor(tcell.ColorRed).SetText("Extraction failed: " + err.Error())
					return
				}
				ui.iconSetup = eventstore.IconSetupEnabled
				ui.status.SetText(fmt.Sprintf("[green]Extracted %d spell icon sets[::-]", len(names)))
				ui.closeModal()
			})
		}()
	})
	no.SetSelectedFunc(func() {
		if manual {
			ui.closeModal()
			return
		}
		if err := ui.store.SaveSpellIconSetup(eventstore.IconSetupDeclined); err != nil {
			message.SetTextColor(tcell.ColorRed).SetText(err.Error())
			return
		}
		ui.iconSetup = eventstore.IconSetupDeclined
		ui.closeModal()
	})
	body := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(nil, 1, 0, false).
		AddItem(message, 3, 0, false).
		AddItem(eventButtonRow(yes, no), 1, 0, false).
		AddItem(nil, 1, 0, false)
	ui.openFormModal(" Spell Icons ", body, 68, 9, []tview.Primitive{yes, no}, no)
}

func (ui *eventsTUI) openFormModal(title string, body *tview.Flex, width, height int, focus []tview.Primitive, initial tview.Primitive) {
	body.SetBorder(true).SetTitle(title)
	dialogCapture := body.GetInputCapture()
	body.SetInputCapture(func(key *tcell.EventKey) *tcell.EventKey {
		if dialogCapture != nil {
			key = dialogCapture(key)
			if key == nil {
				return nil
			}
		}
		if key.Key() == tcell.KeyEscape {
			ui.closeModal()
			return nil
		}
		if key.Key() != tcell.KeyTab && key.Key() != tcell.KeyBacktab {
			return key
		}
		current := ui.app.GetFocus()
		index := 0
		for i, primitive := range focus {
			if primitive == current {
				index = i
				break
			}
		}
		if key.Key() == tcell.KeyBacktab {
			index = (index - 1 + len(focus)) % len(focus)
		} else {
			index = (index + 1) % len(focus)
		}
		ui.app.SetFocus(focus[index])
		return nil
	})
	ui.openModalPrimitive(title, body, width, height, initial)
}

func (ui *eventsTUI) openModalPrimitive(_ string, primitive tview.Primitive, width, height int, initial tview.Primitive) {
	grid := tview.NewGrid().
		SetColumns(0, width, 0).
		SetRows(0, height, 0).
		AddItem(primitive, 1, 1, 1, 1, 0, 0, true)
	ui.modalOpen = true
	ui.pages.AddAndSwitchToPage(eventsModalName, grid, true)
	ui.app.SetFocus(initial)
}

func (ui *eventsTUI) closeModal() {
	ui.pages.RemovePage(eventsModalName)
	ui.modalOpen = false
	ui.app.SetFocus(ui.table)
}

func (ui *eventsTUI) soundDropDown(selectedID string) *tview.DropDown {
	labels := make([]string, len(ui.sounds))
	selected := 0
	for index, sound := range ui.sounds {
		labels[index] = sound.Label
		if sound.ID == selectedID {
			selected = index
		}
	}
	dropdown := tview.NewDropDown().SetOptions(labels, nil)
	dropdown.SetCurrentOption(selected)
	return dropdown
}

func (ui *eventsTUI) selectedSound(dropdown *tview.DropDown) string {
	index, _ := dropdown.GetCurrentOption()
	if index <= 0 || index >= len(ui.sounds) {
		return ""
	}
	return ui.sounds[index].ID
}

func (ui *eventsTUI) soundLabel(id string) string {
	for _, sound := range ui.sounds {
		if sound.ID == id {
			return sound.Label
		}
	}
	return id
}

func (ui *eventsTUI) setError(message string) {
	ui.status.SetText("[red]" + tview.Escape(message) + "[::-]")
}

func eventFieldRow(label string, field tview.Primitive) *tview.Flex {
	return tview.NewFlex().
		AddItem(tview.NewTextView().SetTextAlign(tview.AlignRight).SetText(label+" "), 15, 0, false).
		AddItem(field, 0, 1, true)
}

func eventButtonRow(buttons ...*tview.Button) *tview.Flex {
	row := tview.NewFlex().AddItem(nil, 0, 1, false)
	for _, button := range buttons {
		row.AddItem(button, 12, 0, false).AddItem(nil, 1, 0, false)
	}
	return row.AddItem(nil, 0, 1, false)
}

func eventBindAutomaticTitle(title, trigger *tview.InputField) {
	custom, updating := false, false
	title.SetChangedFunc(func(string) {
		if !updating {
			custom = true
		}
	})
	trigger.SetChangedFunc(func(value string) {
		if custom {
			return
		}
		updating = true
		title.SetText(value)
		updating = false
	})
}

func eventAvailableClasses(spells []catalog.Spell) []string {
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

func eventContains(values []string, wanted string) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}

func eventSoundExists(sounds []audio.Sound, id string) bool {
	for _, sound := range sounds {
		if sound.ID == id {
			return true
		}
	}
	return false
}
