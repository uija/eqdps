package statistics

import (
	"fmt"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget/material"
	"github.com/uija/eqdps/internal/ui"
)

func (m *Module) Layout(style *ui.Style, gtx layout.Context) layout.Dimensions {
	return layout.Stack{}.Layout(gtx,
		layout.Expanded(func(gtx layout.Context) layout.Dimensions {
			return layout.UniformInset(unit.Dp(16)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions { return m.RenderMainPageHeader(style, gtx) }),
					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
						if m.currentPage != nil && m.activeImport == nil {
							return m.currentPage.Layout(style, gtx)
						}
						return material.Label(style.Theme, ui.Sp(15), "Content").Layout(gtx)
					}),
				)
			})
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
			bytesMissing := m.lastLogfileOffset - m.lastKnownOffset
			return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Inset{}.Layout(gtx, ui.IconLink(style, &m.updateClick, ui.Refresh, fmt.Sprintf("Stats is %s behind.", FormatBytes(bytesMissing))).Layout)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Inset{Left: unit.Dp(16)}.Layout(gtx, ui.IconLink(style, &m.reloadClick, ui.Refresh, fmt.Sprintf("Full reload (%s)", FormatBytes(m.lastLogfileOffset))).Layout)
				}),
			)
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
func (m *Module) RenderTab(p StatsPage, style *ui.Style, gtx layout.Context) layout.Dimensions {
	return layout.Inset{Left: unit.Dp(4), Right: unit.Dp(4)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return ui.RenderLinkAsButton(style, p.Clickable(), p.GetIcon(), p.Title())(gtx)
	})
}
