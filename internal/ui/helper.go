package ui

import (
	"image"
	"image/color"

	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget/material"
)

func fill(gtx layout.Context, background color.NRGBA) {
	defer clip.Rect(image.Rectangle{Max: gtx.Constraints.Min}).Push(gtx.Ops).Pop()
	paint.ColorOp{Color: background}.Add(gtx.Ops)
	paint.PaintOp{}.Add(gtx.Ops)
}
func TitleBar(gtx layout.Context, style Style, text string) func(gtx layout.Context) layout.Dimensions {
	return func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Min.Y = gtx.Dp(unit.Dp(38))
		gtx.Constraints.Max.Y = gtx.Constraints.Min.Y

		fill(gtx, style.Palette.Chrome)

		return layout.Inset{
			Left:  unit.Dp(14),
			Right: unit.Dp(14),
			Top:   unit.Dp(8),
		}.Layout(gtx, func(layout.Context) layout.Dimensions {
			title := material.Label(style.Theme, unit.Sp(17), text)
			title.Color = style.Palette.Text
			return title.Layout(gtx)
		})
	}
}
func ColoredRow(gtx layout.Context, color color.NRGBA, content layout.Widget) layout.Dimensions {
	return layout.Stack{}.Layout(
		gtx,
		layout.Expanded(func(gtx layout.Context) layout.Dimensions {
			size := gtx.Constraints.Min
			paint.FillShape(gtx.Ops, color, clip.Rect{Max: size}.Op())
			return layout.Dimensions{Size: size}
		}),
		layout.Stacked(content),
	)
}
