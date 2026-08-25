package events

import (
	"fmt"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget/material"
	"github.com/uija/eqdps/internal/ui"
)

func (m *Module) RenderDeleteOverlay(style *ui.Style, gtx layout.Context) layout.Dimensions {
	m.mu.Lock()
	title := ""
	if m.delete_id >= 0 && m.delete_id < len(m.ctx.Config.Events) {
		title = m.ctx.Config.Events[m.delete_id].Title
	}
	m.mu.Unlock()

	return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		width := min(gtx.Dp(unit.Dp(400)), gtx.Constraints.Max.X)
		gtx.Constraints.Max.X = width
		gtx.Constraints.Min.X = width

		ui.FillOverlay(gtx, style.Palette.Accent, style.Palette.Border)

		return layout.Stack{}.Layout(gtx,
			layout.Expanded(func(gtx layout.Context) layout.Dimensions {
				ui.FillOverlay(gtx, style.Palette.Panel, style.Palette.Border)
				return layout.Dimensions{Size: gtx.Constraints.Min}
			}),
			layout.Stacked(func(gtx layout.Context) layout.Dimensions {
				return layout.UniformInset(unit.Dp(16)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							label := material.Label(style.Theme, 18, "Delete Event")
							label.Font.Weight = font.SemiBold
							label.Alignment = text.Middle
							return label.Layout(gtx)
						}),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							str := fmt.Sprintf("Do you really want to delete the event \"%s\"", title)
							return layout.Inset{Top: unit.Dp(16), Bottom: unit.Dp(16)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
								return material.Label(style.Theme, 14, str).Layout(gtx)
							})
						}),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
								layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
									return layout.Inset{Right: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
										return layout.E.Layout(gtx, ui.IconLink(style, &m.do_delete_click, ui.Check, "Delete it!").Layout)
									})
								}),
								layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
									return layout.Inset{Left: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
										return layout.W.Layout(gtx, ui.IconLink(style, &m.cancel_delete_click, ui.Close, "Cancel").Layout)
									})
								}),
							)
						}),
					)
				})
			}),
		)
	})
}
