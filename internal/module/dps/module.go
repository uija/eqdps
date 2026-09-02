package dps

import (
	"sort"
	"sync/atomic"
	"time"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"github.com/uija/eqdps/internal/data"
	"github.com/uija/eqdps/internal/module"
	"github.com/uija/eqdps/internal/overlay"
	"github.com/uija/eqdps/internal/ui"
	"github.com/uija/eqdps/internal/view"
)

var categories = []string{}

var dpsHelpSections = []struct {
	title string
	text  string
}{
	{
		text: "The DPS Meter automatically detects fights from your EverQuest logfile and records the damage dealt by everyone involved.\n\nEach fight shows its target, duration and how the fight ended. Active fights continue updating as new damage appears. Completed fights remain available so you can review them afterward.",
	},
	{
		title: "Reading the table",
		text: "• Combatant — The player, pet or NPC that dealt damage.\n" +
			"• Damage — Total damage dealt during the fight.\n" +
			"• DPS — Average damage per second over the full duration of the fight.\n" +
			"• SDPS — Damage per second from the point at which you actively joined the fight. This can be useful when a fight was already underway before you attacked.\n" +
			"• Hits — Number of successful damaging attacks.\n" +
			"• Crits — Number of critical hits.\n" +
			"• Active — Time between the combatant’s first and last damaging action.\n\n" +
			"The percentage next to a combatant shows their share of the fight’s total damage. For melee attacks, additional information about misses and hit chance may also be shown.",
	},
	{
		title: "Damage details",
		text: "Click a combatant to expand their damage breakdown. Damage is divided into:\n\n" +
			"• Melee — Normal attacks and combat abilities.\n" +
			"• Spells — Spells actively cast by the player.\n" +
			"• DoTs — Damage-over-time effects.\n" +
			"• Procs — Automatically triggered spell effects.\n" +
			"• Damage Shield — Damage caused by damage shields.\n\n" +
			"Expand these categories to see individual attacks, spells and abilities, including their total damage, DPS, number of hits, critical hits and active time.\n\nFor proc effects, the details can also show an estimated number of procs per minute.",
	},
	{
		title: "Fight history",
		text:  "Use the search field to filter the fight list by target name. This is useful when reviewing a long logfile containing many completed fights.\n\nNew fights are added automatically. If you scroll down to inspect an older fight, the list keeps your current position instead of jumping back to the newest entry.",
	},
	{
		title: "DPS overlay",
		text:  "The overlay provides a compact view of the fight that is most relevant to you. During combat, it prefers the active fight in which you most recently participated. When no fight is active, it keeps showing your most recent completed fight.\n\nYou can open or close the overlay from the DPS Meter. Its visibility is remembered when the application is restarted.",
	},
	{
		title: "Notes",
		text:  "Damage and fight timing are calculated entirely from messages found in the logfile. Results can differ slightly from the game or another parser when messages are missing, fights overlap, or combat begins before logging starts.",
	},
}

type column struct {
	title  string
	weight int
}

type Module struct {
	ctx    *module.Context
	combat *Combat
	rows   chan *data.LogRowEvent
	stop   chan struct{}

	table    widget.List
	helpList widget.List
	columns  []column

	replay       atomic.Bool
	startOverlay bool

	filterEditor widget.Editor
	filterReset  widget.Clickable

	overlayClick   widget.Clickable
	overlayClosed  chan struct{}
	overlayTimeout time.Time

	invalidateFunc func()

	displayHistory []*data.Fight
	displaySize    int
	displayFilter  string
	displayCombat  *Combat
}

func NewModule() *Module {
	return &Module{
		combat:         newCombat(),
		rows:           make(chan *data.LogRowEvent, 1024),
		stop:           make(chan struct{}, 1),
		columns:        make([]column, 0),
		overlayClosed:  make(chan struct{}, 1),
		displayHistory: make([]*data.Fight, 0),
	}
}

type CombatInstance struct {
	start      time.Time
	target     string
	lastAction time.Time
	events     []*data.LogRowEvent
}

func (m *Module) Init(ctx *module.Context, invalidateFunc func()) error {
	m.ctx = ctx
	m.table.Axis = layout.Vertical
	m.helpList.Axis = layout.Vertical
	m.columns = append(m.columns, column{title: "Combatant", weight: 6})
	m.columns = append(m.columns, column{title: "Damage", weight: 1})
	m.columns = append(m.columns, column{title: "Dps", weight: 1})
	m.columns = append(m.columns, column{title: "SDps", weight: 1})
	m.columns = append(m.columns, column{title: "Hits", weight: 1})
	m.columns = append(m.columns, column{title: "Crits", weight: 1})
	m.columns = append(m.columns, column{title: "Active", weight: 1})
	m.invalidateFunc = invalidateFunc
	ctx.AddViewMenuItem("DPS Meter", m.OpenMainView)
	ctx.RegisterLogOpen(m.OnLogOpen)
	ctx.RegisterLogRow(m.OnLogRow)
	ctx.RegisterReplayStart(m.OnReplayStart)
	ctx.RegisterReplayEnd(m.OnReplayEnd)
	ctx.AddSidebarItem("DPS", m.OpenMainView)
	ctx.SetMainView(m.MainView)
	ctx.AddHelpItem("DPS Meter", m.LayoutHelp)
	ctx.RegisterUpdate(m.Update)
	m.startOverlay = m.ctx.Config.OpenOverlay
	m.filterEditor.SingleLine = true
	m.combat = newCombat()

	go func() {
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()

		for {
			select {
			case row := <-m.rows:
				if m.replay.Load() {
					m.combat.endTimedOutFights(row.Timestamp)
				}
				if m.combat.AddEvent(row) {
					if !m.replay.Load() {
						m.publishOverlayFight()
					}
				}
			case now := <-ticker.C:
				if !m.replay.Load() && m.combat.endTimedOutFights(now) {
					m.publishOverlayFight()
				}
			case <-m.stop:
				return
			}
		}
	}()
	return nil
}

func (m *Module) publishOverlayFight() {
	if m.ctx.Overlay == nil {
		return
	}
	var fight *data.Fight
	combat := m.combat
	combat.mu.RLock()
	if len(m.combat.history) == 0 {
		combat.mu.RUnlock()
		return
	}
	if m.combat.lastParticipatedFight != nil {
		fight = m.combat.lastParticipatedFight.Clone()
	} else {
		active := make([]*data.Fight, 0)
		for _, f := range combat.history {
			if f.EndReason == "" {
				active = append(active, f)
			}
		}
		if len(active) == 0 {
			fight = combat.history[len(combat.history)-1].Clone()
		} else {
			sort.Slice(active, func(i, j int) bool {
				return active[i].LastParticipate.After(active[j].LastParticipate)
			})
			fight = active[0].Clone()
		}

	}
	combat.mu.RUnlock()
	if fight != nil {
		if m.ctx.Overlay != nil {
			m.ctx.Overlay.Send(fight)
			m.ctx.Overlay.Invalidate()
		}
	}
}

func (m *Module) OpenMainView() {
	m.ctx.SetMainView(m.MainView)
}

func (m *Module) Shutdown() {
	m.stop <- struct{}{}
	if m.ctx.Overlay != nil {
		m.ctx.Overlay.Close()
	}
}

func (m *Module) OnLogOpen(characterName string, serverName string, size int64, path string) bool {
	m.combat = newCombat()
	return true
}
func (m *Module) OnReplayStart() {
	m.replay.Store(true)
	m.combat = newCombat()
}
func (m *Module) OnReplayEnd() {
	m.replay.Store(false)
	m.publishOverlayFight()
}
func (m *Module) OnLogRow(event *data.LogRowEvent) {
	switch event.Type {
	case data.LogRowEventTypeCast,
		data.LogRowEventTypeDamage,
		data.LogRowEventTypeDamageOverTime,
		data.LogRowEventTypeDamageShield,
		data.LogRowEventTypeYourDamageOverTime,
		data.LogRowEventTypeYourDamageShield,
		data.LogRowEventTypeAggroClear,
		data.LogRowEventTypeZoneChange,
		data.LogRowEventTypeSlainBy,
		data.LogRowEventTypeSomeoneDied,
		data.LogRowEventTypeYouSlain,
		data.LogRowEventTypeFailedMelee:

		m.rows <- event
	}
}
func (m *Module) SelectBacklog() {

}
func (m *Module) Update(gtx layout.Context) {
	if !m.replay.Load() {
		combat := m.combat
		combat.mu.RLock()
		for _, f := range combat.history {
			for _, c := range f.Combatants {
				if c.Click.Clicked(gtx) {
					c.Open = !c.Open
				}
			}
		}
		combat.mu.RUnlock()
	}
	if m.overlayClick.Clicked(gtx) {
		if m.ctx.Overlay == nil {
			m.OpenOverlay()
			m.ctx.Config.OpenOverlay = true
			m.ctx.Config.Save()
		} else {
			m.ctx.Overlay.Close()
			m.ctx.Config.OpenOverlay = false
			m.ctx.Config.Save()
		}
	}
	select {
	case <-m.overlayClosed:
		m.ctx.Overlay = nil
		m.invalidateFunc()
	default:
	}
	if m.startOverlay {
		m.startOverlay = false
		m.OpenOverlay()
	}
	if m.filterReset.Clicked(gtx) {
		m.filterEditor.SetText("")
	}
}
func (m *Module) OpenOverlay() {
	if m.ctx.Overlay != nil {
		return
	}
	m.ctx.Overlay = overlay.NewOverlay(&view.Style, m.ctx.Config)
	go m.ctx.Overlay.Run(func() {
		m.overlayClosed <- struct{}{}
	})
}
func (m *Module) LayoutHelp(style *ui.Style, gtx layout.Context) layout.Dimensions {
	list := material.List(style.Theme, &m.helpList)
	return list.Layout(gtx, len(dpsHelpSections), func(gtx layout.Context, index int) layout.Dimensions {
		section := dpsHelpSections[index]
		return layout.Inset{Bottom: unit.Dp(24), Right: unit.Dp(16)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			children := make([]layout.FlexChild, 0, 2)
			if section.title != "" {
				children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					label := material.Label(style.Theme, ui.Sp(18), section.title)
					label.Font.Weight = font.SemiBold
					return layout.Inset{Bottom: unit.Dp(8)}.Layout(gtx, label.Layout)
				}))
			}
			children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				label := material.Label(style.Theme, ui.Sp(15), section.text)
				label.Color = style.Palette.Muted
				return label.Layout(gtx)
			}))
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
		})
	})
}
