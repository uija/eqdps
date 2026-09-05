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
	"gioui.org/x/component"
)

const (
	MIN_FONT_SCALING = 0.5
	MAX_FONT_SCALING = 1.4
)

var FontScaling float32 = 1.0

func Sp(val float32) unit.Sp {
	return unit.Sp(val * FontScaling)
}

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
			title := ColorLabel(style.Palette.Text, HeaderLabel(&style, text))
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

/*
	func ColoredLabel(th *material.Theme, size float32, col color.NRGBA, txt string) material.LabelStyle {
		l := material.Label(th, Sp(size), txt)
		l.Color = col
		return l
	}
*/
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

func TextFieldSized(editor *widget.Editor, hint string, width int, style *Style, gtx layout.Context) layout.Dimensions {
	return layout.Stack{}.Layout(gtx,
		layout.Expanded(func(gtx layout.Context) layout.Dimensions {
			Fill(gtx, style.Palette.Border)
			return layout.Dimensions{}
		}),
		layout.Stacked(func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min.X = min(gtx.Dp(unit.Dp(width)), gtx.Constraints.Max.X)
			return layout.UniformInset(unit.Dp(1)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Background{}.Layout(gtx,
					func(gtx layout.Context) layout.Dimensions {
						Fill(gtx, style.Palette.Window)
						return layout.Dimensions{Size: gtx.Constraints.Min}
					},
					func(gtx layout.Context) layout.Dimensions {
						editor := material.Editor(style.Theme, editor, hint)
						editor.TextSize = Sp(15)
						return layout.UniformInset(unit.Dp(7)).Layout(gtx, editor.Layout)
					},
				)
			})
		}),
	)
}
func TextField(editor *widget.Editor, hint string, style *Style, gtx layout.Context) layout.Dimensions {
	return TextFieldSized(editor, hint, 500, style, gtx)
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
						editor := material.Editor(style.Theme, editor, hint)
						editor.TextSize = Sp(15)
						return layout.UniformInset(unit.Dp(7)).Layout(gtx, editor.Layout)
					},
				)
			})
		}),
	)
}

func PageTitle(style *Style, title string, gtx layout.Context) material.LabelStyle {
	label := material.Label(style.Theme, Sp(18), title)
	label.Font.Weight = font.SemiBold
	return label
}

func LayoutTooltip(
	gtx layout.Context,
	area *component.TipArea,
	tip component.Tooltip,
	child layout.Widget,
) layout.Dimensions {
	tipGtx := gtx
	tipGtx.Constraints.Max.X = max(
		tipGtx.Constraints.Max.X,
		gtx.Dp(unit.Dp(300)),
	)

	return area.Layout(tipGtx, tip, child)
}

func RenderLinkAsButton(style *Style, clickable *widget.Clickable, icon *widget.Icon, text string) layout.Widget {
	link := IconLink(style, clickable, icon, text)
	link.Padding[PADDING_TOP] = 4
	link.Padding[PADDING_BOTTOM] = 4
	link.Padding[PADDING_LEFT] = 8
	link.Padding[PADDING_RIGHT] = 8
	return func(gtx layout.Context) layout.Dimensions {
		return ColoredBorderedRow(gtx, style.Palette.Panel, link.Layout)
	}
}
func Overlay(gtx layout.Context, width int, bgcolor color.NRGBA, bordercolor color.NRGBA, widget layout.Widget) layout.Dimensions {
	return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		w := min(gtx.Dp(unit.Dp(width)), gtx.Constraints.Max.X)
		gtx.Constraints.Min.X = w
		gtx.Constraints.Max.X = w
		gtx.Constraints.Min.Y = 0
		return layout.Background{}.Layout(gtx,
			func(gtx layout.Context) layout.Dimensions {
				FillOverlay(gtx, bgcolor, bordercolor)
				return layout.Dimensions{Size: gtx.Constraints.Min}
			},
			widget,
		)
	})
}
