package ui

import (
	"image"
	"image/color"

	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
)

func HeaderLabel(style *Style, text string) material.LabelStyle {
	return material.Label(style.Theme, Sp(17), text)
}
func Label(style *Style, text string) material.LabelStyle {
	return material.Label(style.Theme, Sp(15), text)
}
func ColorLabel(col color.NRGBA, label material.LabelStyle) material.LabelStyle {
	label.Color = col
	return label
}
func IconLabel(gtx layout.Context, th *material.Theme, size float32, icon *widget.Icon, txt string) layout.Dimensions {
	return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints = layout.Exact(image.Pt(gtx.Dp(unit.Dp(18)), gtx.Dp(unit.Dp(18))))
			return layout.Inset{Top: unit.Dp(2)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return icon.Layout(gtx, th.Fg)
			})
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Left: unit.Dp(6)}.Layout(gtx, material.Label(th, Sp(size), txt).Layout)
		}),
	)
}
func ColoredIconLabel(gtx layout.Context, th *material.Theme, size float32, icon *widget.Icon, col color.NRGBA, txt string) layout.Dimensions {
	return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints = layout.Exact(image.Pt(gtx.Dp(unit.Dp(18)), gtx.Dp(unit.Dp(18))))
			return layout.Inset{Top: unit.Dp(2)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return icon.Layout(gtx, col)
			})
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			label := material.Label(th, Sp(size), txt)
			label.Color = col
			return layout.Inset{Left: unit.Dp(6)}.Layout(gtx, label.Layout)
		}),
	)
}
