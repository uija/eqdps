package sky

import (
	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"github.com/uija/eqdps/internal/ui"
)

func (m *Module) RenderRunesOverlay(style *ui.Style, gtx layout.Context) layout.Dimensions {
	ui.FillOverlay(gtx, style.Palette.Panel, style.Palette.Border)
	return layout.UniformInset(unit.Dp(32)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		children := make([]layout.FlexChild, 0)
		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Bottom: unit.Dp(16)}.Layout(gtx, material.Label(style.Theme, unit.Sp(17), "Edit your runes").Layout)
		}))
		for idx := range m.runeNames {
			children = append(children, layout.Rigid(m.TextFieldRow(m.runeEditors[idx], m.runeNames[idx], "", style, gtx)))
		}
		children = append(children,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return material.Button(style.Theme, &m.runeSaveClick, "Save").Layout(gtx)
					}),
					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
						return layout.E.Layout(gtx, material.Button(style.Theme, &m.runeCancelClick, "Cancel").Layout)
					}),
				)
			}),
		)
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
	})
}

func (m *Module) TextFieldRow(field *widget.Editor, title string, hint string, style *ui.Style, gtx layout.Context) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		return layout.Inset{Left: unit.Dp(0)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					return layout.UniformInset(unit.Dp(10)).Layout(gtx, material.Body1(style.Theme, title).Layout)
				}),
				layout.Flexed(4, func(gtx layout.Context) layout.Dimensions {
					return layout.UniformInset(unit.Dp(4)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return ui.TextFieldSized(field, hint, 80, style, gtx)
					})
				}),
			)
		})
	}
}
