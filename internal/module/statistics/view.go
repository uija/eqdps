package statistics

import (
	"fmt"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget/material"
	"github.com/uija/eqdps/internal/module/statistics/page"
	"github.com/uija/eqdps/internal/ui"
)

func (m *Module) Layout(style *ui.Style, gtx layout.Context) layout.Dimensions {
	return layout.Stack{}.Layout(gtx,
		layout.Expanded(func(gtx layout.Context) layout.Dimensions {
			return layout.UniformInset(unit.Dp(16)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions { return m.RenderMainPageHeader(style, gtx) }),
					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
						return ui.ColoredRow(gtx, style.Palette.Panel, func(gtx layout.Context) layout.Dimensions {

							return material.Label(style.Theme, ui.Sp(15), "Content").Layout(gtx)
						})
					}),
				)
			})
		}),
		layout.Expanded(func(gtx layout.Context) layout.Dimensions {
			if m.replayProgress != nil {
				return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					ui.FillOverlay(gtx, style.Palette.Panel, style.Palette.Border)
					return material.Label(style.Theme, ui.Sp(15), fmt.Sprintf("%d/%d", m.replayProgress.Bytes, m.replayProgress.Total)).Layout(gtx)
				})
			}
			return layout.Dimensions{}
		}),
	)
}
func (m *Module) RenderMainPageHeader(style *ui.Style, gtx layout.Context) layout.Dimensions {
	return layout.Flex{}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			label := material.Label(style.Theme, ui.Sp(17), "Statistics")
			label.Font.Weight = font.SemiBold
			return label.Layout(gtx)
		}),
		layout.Flexed(1, material.Label(style.Theme, ui.Sp(15), "").Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return ui.IconLink(style, &m.updateClick, ui.Refresh, "Stats is 14MB and 11324 messages behind,").Layout(gtx)
		}),
		layout.Flexed(1, material.Label(style.Theme, ui.Sp(15), "").Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			tabs := make([]layout.FlexChild, 0)
			for _, p := range m.Pages {
				tabs = append(tabs, layout.Rigid(func(gtx layout.Context) layout.Dimensions { return m.RenderTab(p, style, gtx) }))
			}
			return layout.Flex{}.Layout(gtx, tabs...)
		}),
	)
}
func (m *Module) RenderTab(p page.StatsPage, style *ui.Style, gtx layout.Context) layout.Dimensions {
	return layout.Inset{Left: unit.Dp(4), Right: unit.Dp(4)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return ui.RenderLinkAsButton(style, p.Clickable(), ui.ActionVisibility, p.Title())(gtx)
		/*
			return ui.ColoredRow(gtx, style.Palette.Panel, func(gtx layout.Context) layout.Dimensions {
				return p.Clickable().Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return layout.UniformInset(unit.Dp(8)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return material.Label(style.Theme, ui.Sp(15), p.Title()).Layout(gtx)
					})
				})
			})
		*/
	})
}
