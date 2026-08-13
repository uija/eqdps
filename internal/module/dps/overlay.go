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
}

func newOverlay(style *ui.Style) *Overlay {
	window := new(app.Window)
	window.Option(
		app.Title("eqdps - Current Fight"),
		app.Size(unit.Dp(300), unit.Dp(180)),
		app.MinSize(unit.Dp(380), unit.Dp(220)),
		app.Decorated(false),
		app.TopMost(true),
	)
	return &Overlay{
		window:  window,
		updates: make(chan Fight, 1),
		style:   style,
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
			return layout.UniformInset(unit.Dp(4)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				gtx.Constraints.Min.X = gtx.Constraints.Max.X
				return material.Label(o.style.Theme, unit.Sp(13), "Dps").Layout(gtx)
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
