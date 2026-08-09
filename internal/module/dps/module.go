package dps

import (
	"time"

	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget/material"
	"github.com/uija/eqdps/internal/data"
	"github.com/uija/eqdps/internal/module"
	"github.com/uija/eqdps/internal/ui"
)

type Module struct {
	ctx    *module.Context
	combat Combat
}

type CombatInstance struct {
	start      time.Time
	target     string
	lastAction time.Time
	events     []*data.LogRowEvent
}

func (m *Module) Init(ctx *module.Context) error {
	m.ctx = ctx
	ctx.AddViewMenuItem("DPS Meter", m.OpenMainView)
	ctx.OnLogOpen(m.OnLogOpen)
	ctx.OnLogRow(m.OnLogRow)
	ctx.SetMainView(m.MainView)
	ctx.AddToolsMenuItem("Load Combat History", m.SelectBacklog)
	ctx.AddHelpItem("DPS Meter", m.LayoutHelp)
	m.combat = newCombat()
	return nil
}

func (m *Module) OpenMainView() {
	m.ctx.SetMainView(m.MainView)
}

func (m *Module) Shutdown() {

}

func (m *Module) OnLogOpen(characterName string, serverName string, size int64) {

}
func (m *Module) OnLogRow(event *data.LogRowEvent) {
	m.combat.AddEvent(event)
}
func (m *Module) SelectBacklog() {

}
func (m *Module) LayoutHelp(style *ui.Style, gtx layout.Context) layout.Dimensions {
	label := material.Label(
		style.Theme,
		unit.Sp(15),
		"This is awesome DPS Meter help content!",
	)
	label.Color = style.Palette.Muted
	return label.Layout(gtx)
}

func (m *Module) MainView(style *ui.Style, gtx layout.Context) layout.Dimensions {
	return layout.Inset{Top: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		label := material.Label(style.Theme, unit.Sp(15), "Dps Meter")
		//label.Color = palette.muted
		return label.Layout(gtx)
	})
}
