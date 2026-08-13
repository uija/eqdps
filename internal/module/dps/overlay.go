package dps

import (
	"gioui.org/app"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
	"gioui.org/widget/material"
	"github.com/uija/eqdps/internal/ui"
)

type Overlay struct {
	updates chan Fight
	style   *ui.Style
	window  *app.Window

	fight *Fight
}

func newOverlay(source *ui.Style) *Overlay {

	overlayStyle := *source
	overlayStyle.Theme = material.NewTheme()
	overlayStyle.Theme.Palette = source.Theme.Palette
	overlayStyle.Theme.TextSize = source.Theme.TextSize
	overlayStyle.Theme.Face = source.Theme.Face
	overlayStyle.Theme.FingerSize = source.Theme.FingerSize

	window := new(app.Window)
	window.Option(
		app.Title("eqdps — Current Fight"),
		app.Size(unit.Dp(360), unit.Dp(200)),
		app.MinSize(unit.Dp(360), unit.Dp(200)),
		app.Decorated(false),
		app.TopMost(true),
	)
	return &Overlay{
		window:  window,
		updates: make(chan Fight, 1),
		style:   &overlayStyle,
	}
}

func (o *Overlay) run(onEnd func()) {
	var ops op.Ops
	for {
		switch event := o.window.Event().(type) {
		case app.DestroyEvent:
			onEnd()
			return
		case app.FrameEvent:
			gtx := app.NewContext(&ops, event)
			o.Layout(gtx)
			event.Frame(gtx.Ops)
		}
	}
}
func (o *Overlay) Layout(gtx layout.Context) layout.Dimensions {
	ui.Fill(gtx, o.style.Palette.Window)
	children := make([]layout.FlexChild, 0)
	// TODO: Timer

	children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
		return ui.ColoredRow(gtx, o.style.Palette.Panel, func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Top: unit.Dp(4), Bottom: unit.Dp(4), Left: unit.Dp(12)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				gtx.Constraints.Min.X = gtx.Constraints.Max.X
				if o.fight == nil {
					return material.Label(o.style.Theme, unit.Sp(13), "Waiting for data").Layout(gtx)
				} else {
					return material.Label(o.style.Theme, unit.Sp(13), o.fight.name).Layout(gtx)
				}
			})
		})
	}))
	return layout.Stack{}.Layout(gtx,
		layout.Expanded(func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min = gtx.Constraints.Max
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
		}),
	)
}
