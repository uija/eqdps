package sky

import (
	"image"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"github.com/uija/eqdps/internal/ui"
)

func (m *Module) RenderRunesOverlay(style *ui.Style, gtx layout.Context) layout.Dimensions {
	return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		width := min(gtx.Dp(unit.Dp(420)), gtx.Constraints.Max.X)
		height := min(gtx.Dp(unit.Dp(870)), gtx.Constraints.Max.Y)
		gtx.Constraints = layout.Exact(image.Pt(width, height))

		ui.FillOverlay(gtx, style.Palette.Panel, style.Palette.Border)

		return layout.UniformInset(unit.Dp(32)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Inset{Bottom: unit.Dp(16)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						label := ui.HeaderLabel(style, "Edit your runes")
						label.Font.Weight = font.SemiBold
						return label.Layout(gtx)
					})
				}),
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					list := material.List(style.Theme, &m.runeEditList)
					return list.Layout(
						gtx,
						len(m.runeNames),
						func(gtx layout.Context, idx int) layout.Dimensions {
							return m.TextFieldRow(m.runeEditors[idx], m.runeNames[idx], "", style, gtx)(gtx)
						},
					)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Inset{Top: unit.Dp(16)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								return material.Button(style.Theme, &m.runeSaveClick, "Save").Layout(gtx)
							}),
							layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
								return layout.E.Layout(gtx, material.Button(style.Theme, &m.runeCancelClick, "Cancel").Layout)
							}),
						)
					})
				}),
			)
		})
	})
}

func (m *Module) TextFieldRow(field *widget.Editor, title string, hint string, style *ui.Style, gtx layout.Context) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		return layout.Inset{Left: unit.Dp(0)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					return layout.UniformInset(unit.Dp(10)).Layout(gtx, ui.Label(style, title).Layout)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.UniformInset(unit.Dp(4)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return ui.TextFieldSized(field, hint, 80, style, gtx)
					})
				}),
			)
		})
	}
}
