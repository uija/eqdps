package sky

import (
	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget/material"
	"github.com/uija/eqdps/internal/data"
	"github.com/uija/eqdps/internal/module"
	"github.com/uija/eqdps/internal/ui"
)

type Module struct {
	ctx *module.Context
}

func (m *Module) Init(ctx *module.Context) error {
	ctx.AddViewMenuItem("Plane of Sky Quest Tracker", m.OpenMainView)
	ctx.OnLogOpen(m.OnLogOpen)
	ctx.OnLogRow(m.OnLogRow)
	ctx.AddHelpItem("Plane of Sky Quest Tracker", m.LayoutHelp)
	m.ctx = ctx
	return nil
}

func (m *Module) OpenMainView() {
	m.ctx.SetMainView(m.MainView)
}

func (m *Module) Shutdown() {

}

func (m *Module) OnLogOpen(characterName string, serverName string, size int64) {

}
func (m *Module) OnLogRow(event *data.LogEvent) {

}
func (m *Module) LayoutHelp(style *ui.Style, gtx layout.Context) layout.Dimensions {
	label := material.Label(
		style.Theme,
		unit.Sp(15),
		"This is awesome Plane of Sky Quest Tracker help content!",
	)
	label.Color = style.Palette.Muted
	return label.Layout(gtx)
}

func (m *Module) MainView(style *ui.Style, gtx layout.Context) layout.Dimensions {
	return layout.Inset{Top: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		label := material.Label(style.Theme, unit.Sp(15), "Plane of Sky")
		//label.Color = palette.muted
		return label.Layout(gtx)
	})
}
