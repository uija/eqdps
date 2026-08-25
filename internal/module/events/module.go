package events

import (
	"context"
	"fmt"
	"log"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"gioui.org/layout"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"github.com/gen2brain/beeep"
	"github.com/uija/eqdps/internal/audio"
	"github.com/uija/eqdps/internal/data"
	"github.com/uija/eqdps/internal/module"
	"github.com/uija/eqdps/internal/spellicon"
	"github.com/uija/eqdps/internal/ui"
	"github.com/uija/eqdps/internal/ui/form"
)

type Module struct {
	ctx    *module.Context
	replay atomic.Bool

	events_list      widget.List
	add_spell_click  widget.Clickable
	add_timer_click  widget.Clickable
	add_text_click   widget.Clickable
	add_regexp_click widget.Clickable

	edit_index    int
	create_type   data.EventType
	overlay_close widget.Clickable
	delete_id     int

	spells            []spellicon.Spell
	spell_icon_sets   []string
	spell_icon_select *form.SelectBox

	row_click      []widget.Clickable
	activate_click []widget.Clickable

	event_form         *form.Form
	title_field        *widget.Editor
	text_field         *widget.Editor
	duration_field     *widget.Editor
	notification_field *widget.Editor
	full_message_check *widget.Bool
	persistent_check   *widget.Bool
	sound_select       *form.SelectBox
	class_select       *form.SelectBox
	spell_select       *form.SelectBox
	target_select      *form.SelectBox

	close_button_click  widget.Clickable
	save_button_click   widget.Clickable
	delete_button_click widget.Clickable

	do_delete_click     widget.Clickable
	cancel_delete_click widget.Clickable

	validate_regexp_click widget.Clickable
	validation_state      int

	volume           widget.Float
	play_sound_click widget.Clickable

	stop              chan struct{}
	gracePeriodTimers map[string]data.TimerTracker
	runningTimers     map[string]data.TimerTracker
	tmu               sync.RWMutex
}

func NewModule() *Module {
	m := &Module{
		event_form:         form.New(),
		title_field:        &widget.Editor{SingleLine: true},
		text_field:         &widget.Editor{SingleLine: true},
		duration_field:     &widget.Editor{SingleLine: true},
		notification_field: &widget.Editor{SingleLine: true},
		full_message_check: &widget.Bool{},
		persistent_check:   &widget.Bool{},
		sound_select:       form.NewSelectBox([]string{}, 0),
		class_select:       form.NewSelectBox([]string{}, 0),
		spell_select:       form.NewSelectBox([]string{}, 0),
		target_select:      form.NewSelectBox([]string{"Self", "Others", "Both"}, 0),
		spell_icon_select:  form.NewSelectBox([]string{"default"}, 0),
		edit_index:         -1,
		delete_id:          -1,
		create_type:        data.EventTypeUndefined,
		stop:               make(chan struct{}),
		gracePeriodTimers:  make(map[string]data.TimerTracker),
		runningTimers:      make(map[string]data.TimerTracker),
		spell_icon_sets:    make([]string, 0),
	}
	mustRegister := func(err error) {
		if err != nil {
			panic(err)
		}
	}
	mustRegister(m.event_form.AddEditor("title", m.title_field, func(ee widget.EditorEvent) {}))
	mustRegister(m.event_form.AddSelectBox("class", m.class_select))
	mustRegister(m.event_form.AddSelectBox("spell", m.spell_select))
	mustRegister(m.event_form.AddSelectBox("target", m.target_select))
	mustRegister(m.event_form.AddEditor("text", m.text_field, func(ee widget.EditorEvent) {
		m.validation_state = 0
	}))
	mustRegister(m.event_form.AddCheckbox("full", m.full_message_check, func(b bool) {}))
	mustRegister(m.event_form.AddEditor("duration", m.duration_field, func(ee widget.EditorEvent) {}))
	mustRegister(m.event_form.AddEditor("notification", m.notification_field, func(ee widget.EditorEvent) {}))
	mustRegister(m.event_form.AddCheckbox("persistent", m.persistent_check, func(b bool) {}))
	mustRegister(m.event_form.AddSelectBox("sound", m.sound_select))
	mustRegister(m.event_form.AddButton("save", &m.save_button_click, m.OnSave))
	mustRegister(m.event_form.AddButton("cancel", &m.close_button_click, m.OnCancel))

	sounds, err := audio.EmbeddedSounds()
	if err == nil {
		list := make([]string, 0)
		list = append(list, "")
		for _, s := range sounds {
			list = append(list, s.ID)
		}
		m.sound_select.SetOptions(list)
	}
	return m
}

func (m *Module) Init(ctx *module.Context, _ func()) error {
	m.ctx = ctx
	ctx.AddViewMenuItem("Events", m.OpenMainView)
	ctx.AddSidebarItem("Events", m.OpenMainView)
	//ctx.SetMainView(m.Layout)
	ctx.RegisterLogOpen(m.OnLogOpen)
	ctx.RegisterLogRow(m.OnLogRow)
	ctx.RegisterStatusWidget(m.LayoutStatus)
	ctx.RegisterUpdate(m.Update)
	ctx.RegisterReplayStart(m.OnReplayStart)
	ctx.RegisterReplayEnd(m.OnReplayEnd)
	spells, err := spellicon.Load()
	if err != nil {
		panic("Uanble to load spells")
	}
	m.spells = spells
	m.events_list.Axis = layout.Vertical
	m.UpdateSpellsAndClasses()
	m.volume.Value = m.ctx.Config.Volume

	go m.TimerRun()

	return nil
}
func (m *Module) TimerRun() {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			m.tmu.Lock()
			// move spells that are not interrupted anymore
			now := time.Now()
			for name, tt := range m.gracePeriodTimers {
				if now.After(tt.CancelableUntil) {
					m.runningTimers[name] = tt
					delete(m.gracePeriodTimers, name)
				}
			}
			for name, tt := range m.runningTimers {
				if tt.StopsAt.Before(now) {
					m.Notify(tt.Event)
					delete(m.runningTimers, name)
				}
			}
			if m.ctx.Overlay != nil {
				timers := make([]data.TimerTracker, 0)
				for _, tt := range m.runningTimers {
					timers = append(timers, tt)
				}
				if len(timers) > 0 {
					sort.Slice(timers, func(i, j int) bool {
						return timers[i].Event.Spell < timers[j].Event.Spell
					})
				}
				if m.ctx.Overlay != nil {
					m.ctx.Overlay.Send(timers)
				}
			}
			m.tmu.Unlock()
		case <-m.stop:
			return
		}
	}
}
func (m *Module) OpenMainView() {
	m.ctx.SetMainView(m.Layout)
}
func (m *Module) UpdateSpellsAndClasses() {
	if len(m.class_select.Options()) == 0 {
		classes := make([]string, 0)
		classes = append(classes, "ALL")
		for _, spell := range m.spells {
			for _, c := range spell.Classes {
				if !slices.Contains(classes, c) {
					classes = append(classes, c)
				}
			}
		}
		sort.Slice(classes, func(i, j int) bool {
			return classes[i] < classes[j]
		})
		m.class_select.SetOptions(classes)
		m.class_select.SetSelected(0)
	}
	cl := m.class_select.Value()
	current_selection := m.spell_select.Value()
	spells := make([]string, 0)
	ignore := []string{"Illusion:", "Portal"}
	isIgnored := func(name string) bool {
		for _, i := range ignore {
			if strings.Contains(name, i) {
				return true
			}
		}
		return false
	}
	for _, spell := range m.spells {
		if !isIgnored(spell.Name) && (cl == "ALL" || slices.Contains(spell.Classes, cl)) {
			spells = append(spells, spell.Name)
		}
	}
	m.spell_select.SetOptions(spells)
	m.spell_select.Select(current_selection)
}
func (m *Module) Update(gtx layout.Context) {
	m.event_form.Update(gtx)
	m.spell_icon_select.Update(gtx)
	if m.class_select.Changed() {
		m.UpdateSpellsAndClasses()
	}
	if m.spell_select.Changed() {
		m.title_field.SetText(m.spell_select.Value())
	}
	if m.spell_icon_select.Changed() {
		m.ctx.Config.SpellIconSet = m.spell_icon_select.Value()
		m.ctx.Config.Save()
	}
	if m.add_spell_click.Clicked(gtx) {
		m.PrepareToCreate(data.EventTypeSpell)
		m.event_form.Focus(gtx, "title")
	}
	if m.add_regexp_click.Clicked(gtx) {
		m.PrepareToCreate(data.EventTypeRegexp)
		m.event_form.Focus(gtx, "title")
	}
	if m.add_text_click.Clicked(gtx) {
		m.PrepareToCreate(data.EventTypeString)
		m.event_form.Focus(gtx, "title")
	}
	if m.add_timer_click.Clicked(gtx) {
		m.PrepareToCreate(data.EventTypeTimer)
		m.event_form.Focus(gtx, "title")
	}
	if m.overlay_close.Clicked(gtx) {
		m.OnCancel()
	}
	for idx := range m.row_click {
		if m.row_click[idx].Clicked(gtx) {
			m.SelectToEdit(idx)
			m.create_type = data.EventTypeUndefined
			m.event_form.Focus(gtx, "title")
		}
		if m.activate_click[idx].Clicked(gtx) {
			m.ctx.Config.Events[idx].Active = !m.ctx.Config.Events[idx].Active
			m.ctx.Config.Save()
		}
	}
	if m.do_delete_click.Clicked(gtx) {
		if m.delete_id < 0 || m.delete_id >= len(m.ctx.Config.Events) {
			m.delete_id = -1
			m.edit_index = -1
			return
		}
		m.ctx.Config.Events = slices.Delete(m.ctx.Config.Events, m.delete_id, m.delete_id+1)
		m.delete_id = -1
		m.edit_index = -1
		m.ctx.Config.Save()
	}
	if m.cancel_delete_click.Clicked(gtx) {
		m.delete_id = -1
		m.edit_index = -1
	}
	if m.delete_button_click.Clicked(gtx) {
		m.delete_id = m.edit_index
	}
	if m.play_sound_click.Clicked(gtx) {
		sound := m.sound_select.Value()
		if sound != "" {
			m.ctx.Playback.Play(context.Background(), sound, float64(m.ctx.Config.Volume), func(err error) {
				log.Printf("Unable to play sound: %v", err)
			})
		}
	}
	if m.volume.Dragging() {
		m.ctx.Config.Volume = m.volume.Value
		m.ctx.Config.Save()
	}
	if m.validate_regexp_click.Clicked(gtx) {
		if m.text_field.Text() == "" {
			return
		}
		_, err := regexp.Compile(m.text_field.Text())
		if err == nil {
			m.validation_state = 1
		} else {
			m.validation_state = -1
		}
	}
}
func (m *Module) PrepareToCreate(t data.EventType) {
	m.edit_index = -1
	m.create_type = t
	m.title_field.SetText("")
	m.class_select.SetSelected(0)
	m.UpdateSpellsAndClasses()
	m.spell_select.SetSelected(0)
	m.target_select.SetSelected(0)
	m.text_field.SetText("")
	m.full_message_check.Value = false
	if t == data.EventTypeSpell {
		m.notification_field.SetText("%s faded.")
	}
	m.persistent_check.Value = false
	m.sound_select.SetSelected(0)
}
func (m *Module) SelectToEdit(idx int) {
	m.edit_index = idx
	m.create_type = data.EventTypeUndefined
	if m.edit_index >= 0 && m.edit_index < len(m.ctx.Config.Events) {
		val := &m.ctx.Config.Events[m.edit_index]
		m.title_field.SetText(val.Title)
		if val.Class != "" {
			m.class_select.Select(val.Class)
		} else {
			m.class_select.SetSelected(0)
		}
		m.UpdateSpellsAndClasses()
		if val.Spell != "" {
			m.spell_select.Select(val.Spell)
		} else {
			m.class_select.SetSelected(0)
		}
		if val.Type == data.EventTypeSpell || val.Type == data.EventTypeTimer {
			if val.Expression != "" && val.ExpressionOthers != "" {
				m.target_select.Select("Both")
			} else if val.Expression != "" {
				m.target_select.Select("Self")
			} else {
				m.target_select.Select("Others")
			}
		}
		m.text_field.SetText(val.Expression)
		m.full_message_check.Value = val.FullExpression
		if val.Duration >= 0 {
			m.duration_field.SetText(fmt.Sprintf("%d", val.Duration))
		} else {
			m.duration_field.SetText("")
		}
		m.notification_field.SetText(val.Notification)
		m.persistent_check.Value = val.PersistNotification
		if val.Sound != "" {
			m.sound_select.Select(val.Sound)
		} else {
			m.sound_select.SetSelected(0)
		}
	}
}
func (m *Module) OnSave() {
	var val *data.EventConfig = nil
	if m.edit_index >= 0 && len(m.ctx.Config.Events) > m.edit_index {
		val = &m.ctx.Config.Events[m.edit_index]
	} else {
		val = &data.EventConfig{
			Active: true,
			Type:   m.create_type,
		}
	}
	val.Title = m.title_field.Text()
	if val.Type == data.EventTypeSpell || val.Type == data.EventTypeTimer {
		val.Class = m.class_select.Value()
		val.Spell = m.spell_select.Value()
		val.Expression = ""
		val.ExpressionOthers = ""
		for _, spell := range m.spells {
			if spell.Name == val.Spell {
				if m.target_select.Value() == "Self" || m.target_select.Value() == "Both" {
					val.Expression = spell.FadeMessage
				}
				if m.target_select.Value() == "Other" || m.target_select.Value() == "Both" {
					val.ExpressionOthers = spell.FadeMessageOthers
				}
			}
		}
	} else {
		val.Expression = m.text_field.Text()
		val.FullExpression = m.full_message_check.Value
	}
	text := strings.TrimSpace(m.duration_field.Text())
	if text == "" {
		val.Duration = -1
	} else {
		dur, err := strconv.Atoi(text)
		if err != nil {
			val.Duration = -1
		} else {
			val.Duration = dur
		}
	}
	val.Notification = m.notification_field.Text()
	val.PersistNotification = m.persistent_check.Value
	val.Sound = m.sound_select.Value()
	if m.edit_index >= 0 {
		m.ctx.Config.Events[m.edit_index] = *val
	} else {
		m.ctx.Config.Events = append(m.ctx.Config.Events, *val)
	}
	m.edit_index = -1
	m.create_type = data.EventTypeUndefined
	m.ctx.Config.Save()
}
func (m *Module) OnCancel() {
	m.edit_index = -1
	m.create_type = data.EventTypeUndefined
}
func (m *Module) LayoutStatus(style *ui.Style, gtx layout.Context) layout.Dimensions {
	return material.Label(style.Theme, 14, "No events active").Layout(gtx)
}

func (m *Module) OnLogOpen(characterName string, serverName string, filesize int64, path string) bool {
	source, sets, ok := spellicon.Detect(path, m.spells)
	if !ok {
		return true
	}

	iconDir, err := data.AppDataPath("spell-icons")
	if err != nil {
		log.Printf("Unable to locate spell icon directory: %v", err)
		return true
	}

	for _, set := range sets {
		if !spellicon.Available(iconDir, set, m.spells) {
			if _, err := spellicon.ExtractAll(source, iconDir, m.spells); err != nil {
				log.Printf("Unable to extract spell icons: %v", err)
				return true
			}
			break
		}
	}

	m.spell_icon_sets = sets
	m.spell_icon_select.SetOptions(sets)
	if !slices.Contains(sets, m.ctx.Config.SpellIconSet) {
		m.ctx.Config.SpellIconSet = sets[0]
	}
	m.spell_icon_select.Select(m.ctx.Config.SpellIconSet)
	return true
}
func (m *Module) OnLogRow(e *data.LogRowEvent) {
	if m.replay.Load() {
		return
	}
	if len(m.ctx.Config.Events) == 0 {
		return
	}
	for idx, event := range m.ctx.Config.Events {
		if !event.Active {
			continue
		}
		switch event.Type {
		case data.EventTypeString,
			data.EventTypeSpell:
			if event.FullExpression && strings.EqualFold(event.Expression, e.Message) {
				m.Notify(event)
			} else if strings.Contains(e.Message, event.Expression) {
				m.Notify(event)
			}
		case data.EventTypeRegexp:
			if event.Expression != "" && event.RegExp == nil {
				regexp, err := regexp.Compile(event.Expression)
				if err == nil {
					m.ctx.Config.Events[idx].RegExp = regexp
					event.RegExp = regexp
				}
			}
			if event.ExpressionOthers != "" && event.RegExpOthers == nil {
				regexp, err := regexp.Compile(event.ExpressionOthers)
				if err == nil {
					m.ctx.Config.Events[idx].RegExpOthers = regexp
					event.RegExpOthers = regexp
				}
			}
			if event.RegExp != nil {
				if event.RegExp.Match([]byte(e.Message)) {
					m.Notify(event)
				}
			}
			if event.RegExpOthers != nil {
				if event.RegExpOthers.Match([]byte(e.Message)) {
					m.Notify(event)
				}
			}
		case data.EventTypeTimer:
			switch e.Type {
			case data.LogRowEventTypeCast:
				if e.Data[1] == "You" {
					spell := e.Data[2]
					if strings.Contains(spell, event.Spell) {
						now := time.Now()
						m.tmu.Lock()
						m.gracePeriodTimers[event.Spell] = data.TimerTracker{
							Started:         now,
							CancelableUntil: now.Add(time.Second),
							StopsAt:         now.Add(time.Second * time.Duration(event.Duration)),
							Event:           event,
						}
						m.tmu.Unlock()
					}
				}
			}
		}
	}
}
func (m *Module) Notify(event data.EventConfig) {

	if event.Notification != "" {
		not := event.Notification
		if strings.Contains(not, "%s") {
			not = fmt.Sprintf(not, event.Title)
		}
		iconId := -1
		if event.Spell != "" {
			for _, s := range m.spells {
				if strings.EqualFold(s.Name, event.Spell) {
					iconId = s.IconID
					break
				}
			}
		}
		iconPath := ""

		iconDir, err := data.AppDataPath("spell-icons")
		if err == nil {
			iconPath = spellicon.SetIconPath(
				iconDir,
				m.ctx.Config.SpellIconSet,
				iconId,
			)
		}
		beeep.Notify(event.Title, not, iconPath)
	}
	if event.Sound != "" {
		m.ctx.Playback.Play(context.Background(), event.Sound, float64(m.ctx.Config.Volume), func(err error) {
			log.Printf("Unable to play sound. %v", err)
		})
	}
}
func (m *Module) OnReplayStart() {
	m.replay.Store(true)
}
func (m *Module) OnReplayEnd() {
	m.replay.Store(false)
}
func (m *Module) Shutdown() {
}
