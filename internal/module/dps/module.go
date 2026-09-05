package dps

import (
	"sort"
	"sync/atomic"
	"time"

	"gioui.org/layout"
	"gioui.org/widget"
	"github.com/uija/eqdps/internal/data"
	"github.com/uija/eqdps/internal/module"
	"github.com/uija/eqdps/internal/overlay"
	"github.com/uija/eqdps/internal/style"
)

var categories = []string{}

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
	ctx.AddModuleNavigation("DPS", "DPS Meter", "DPS", m.MainView)
	ctx.RegisterLogOpen(m.OnLogOpen)
	ctx.RegisterLogRow(m.OnLogRow)
	ctx.RegisterReplayStart(m.OnReplayStart)
	ctx.RegisterReplayEnd(m.OnReplayEnd)
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
					m.combat.endTimedOutFights(row.Timestamp, time.Duration(m.ctx.Config.CombatTimeout)*time.Second)
				}
				if m.combat.AddEvent(row) {
					if !m.replay.Load() {
						m.publishOverlayFight()
					}
				}
			case now := <-ticker.C:
				if !m.replay.Load() && m.combat.endTimedOutFights(now, time.Duration(m.ctx.Config.CombatTimeout)*time.Second) {
					m.publishOverlayFight()
					m.invalidateFunc()
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
	m.ctx.Overlay = overlay.NewOverlay(&style.Style, m.ctx.Config)
	go m.ctx.Overlay.Run(func() {
		m.overlayClosed <- struct{}{}
	})
}
