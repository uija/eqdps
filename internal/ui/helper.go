package ui

import (
	"image"
	"image/color"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
)

func Fill(gtx layout.Context, background color.NRGBA) {
	defer clip.Rect(image.Rectangle{Max: gtx.Constraints.Min}).Push(gtx.Ops).Pop()
	paint.ColorOp{Color: background}.Add(gtx.Ops)
	paint.PaintOp{}.Add(gtx.Ops)
}
func FillOverlay(gtx layout.Context, background color.NRGBA, border color.NRGBA) {
	size := gtx.Constraints.Min
	paint.FillShape(gtx.Ops, border, clip.Rect{Max: size}.Op())
	if size.X <= 2 || size.Y <= 2 {
		return
	}
	paint.FillShape(
		gtx.Ops,
		background,
		clip.Rect{
			Min: image.Pt(1, 1),
			Max: image.Pt(size.X-1, size.Y-1),
		}.Op(),
	)
}
func TitleBar(gtx layout.Context, style Style, text string) func(gtx layout.Context) layout.Dimensions {
	return func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Min.Y = gtx.Dp(unit.Dp(38))
		gtx.Constraints.Max.Y = gtx.Constraints.Min.Y

		Fill(gtx, style.Palette.Chrome)

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
func ColoredBorderedRow(gtx layout.Context, color color.NRGBA, content layout.Widget) layout.Dimensions {
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

func ColoredAccentedRow(gtx layout.Context, color color.NRGBA, accent color.NRGBA, showAccent bool, content layout.Widget) layout.Dimensions {
	return layout.Stack{}.Layout(
		gtx,
		layout.Expanded(func(gtx layout.Context) layout.Dimensions {
			size := gtx.Constraints.Min
			paint.FillShape(gtx.Ops, color, clip.Rect{Max: size}.Op())
			return layout.Dimensions{Size: size}
		}),
		layout.Expanded(func(gtx layout.Context) layout.Dimensions {
			size := gtx.Constraints.Min
			if showAccent {
				paint.FillShape(
					gtx.Ops,
					accent,
					clip.Rect{
						Max: image.Pt(gtx.Dp(unit.Dp(3)), size.Y),
					}.Op(),
				)
			}
			return layout.Dimensions{Size: size}
		}),
		layout.Stacked(content),
	)
}
func RightAlignLabel(gtx layout.Context, label material.LabelStyle) layout.Dimensions {
	return layout.Inset{Right: unit.Dp(2)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		tgtx := gtx
		tgtx.Constraints.Min.X = tgtx.Constraints.Max.X
		label.Alignment = text.End
		return label.Layout(tgtx)
	})
}
func CenterAlignLabel(gtx layout.Context, label material.LabelStyle) layout.Dimensions {
	return layout.Inset{Right: unit.Dp(2)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		tgtx := gtx
		tgtx.Constraints.Min.X = tgtx.Constraints.Max.X
		label.Alignment = text.Middle
		return label.Layout(tgtx)
	})
}
func ColoredLabel(th *material.Theme, size float32, col color.NRGBA, txt string) material.LabelStyle {
	l := material.Label(th, unit.Sp(size), txt)
	l.Color = col
	return l
}
func Icon(gtx layout.Context, col color.NRGBA, icon *widget.Icon) layout.Dimensions {
	gtx.Constraints = layout.Exact(image.Pt(gtx.Dp(unit.Dp(18)), gtx.Dp(unit.Dp(18))))
	return layout.Inset{Top: unit.Dp(2)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return icon.Layout(gtx, col)
	})
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
			return layout.Inset{Left: unit.Dp(6)}.Layout(gtx, material.Label(th, unit.Sp(size), txt).Layout)
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
			label := material.Label(th, unit.Sp(size), txt)
			label.Color = col
			return layout.Inset{Left: unit.Dp(6)}.Layout(gtx, label.Layout)
		}),
	)
}

func TextField(editor *widget.Editor, hint string, style *Style, gtx layout.Context) layout.Dimensions {
	return layout.Stack{}.Layout(gtx,
		layout.Expanded(func(gtx layout.Context) layout.Dimensions {
			Fill(gtx, style.Palette.Border)
			return layout.Dimensions{}
		}),
		layout.Stacked(func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min.X = min(gtx.Dp(unit.Dp(500)), gtx.Constraints.Max.X)
			return layout.UniformInset(unit.Dp(1)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Background{}.Layout(gtx,
					func(gtx layout.Context) layout.Dimensions {
						Fill(gtx, style.Palette.Window)
						return layout.Dimensions{Size: gtx.Constraints.Min}
					},
					func(gtx layout.Context) layout.Dimensions {
						return layout.UniformInset(unit.Dp(7)).Layout(gtx, material.Editor(style.Theme, editor, hint).Layout)
					},
				)
			})
		}),
	)
}
func MaxedTextField(editor *widget.Editor, hint string, style *Style, gtx layout.Context) layout.Dimensions {
	return layout.Stack{}.Layout(gtx,
		layout.Expanded(func(gtx layout.Context) layout.Dimensions {
			Fill(gtx, style.Palette.Border)
			return layout.Dimensions{}
		}),
		layout.Stacked(func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min.X = gtx.Constraints.Max.X
			return layout.UniformInset(unit.Dp(1)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Background{}.Layout(gtx,
					func(gtx layout.Context) layout.Dimensions {
						Fill(gtx, style.Palette.Window)
						return layout.Dimensions{Size: gtx.Constraints.Min}
					},
					func(gtx layout.Context) layout.Dimensions {
						return layout.UniformInset(unit.Dp(7)).Layout(gtx, material.Editor(style.Theme, editor, hint).Layout)
					},
				)
			})
		}),
	)
}

func PageTitle(style *Style, title string, gtx layout.Context) material.LabelStyle {
	label := material.Label(style.Theme, unit.Sp(18), title)
	label.Font.Weight = font.SemiBold
	return label
}
