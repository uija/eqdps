package events

import (
	"fmt"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"github.com/uija/eqdps/internal/data"
	"github.com/uija/eqdps/internal/ui"
)

func (m *Module) Layout(style *ui.Style, gtx layout.Context) layout.Dimensions {
	m.mu.Lock()
	eventCount := len(m.ctx.Config.Events)
	m.mu.Unlock()

	stacks := make([]layout.StackChild, 0)
	stacks = append(stacks, layout.Expanded(func(gtx layout.Context) layout.Dimensions { return m.RenderMainPage(style, gtx) }))
	if m.edit_index >= 0 && m.edit_index < eventCount || m.create_type != data.EventTypeUndefined {
		stacks = append(stacks, layout.Expanded(func(gtx layout.Context) layout.Dimensions { return m.RenderOverlay(style, gtx) }))
	}
	if m.delete_id >= 0 && m.delete_id < eventCount {
		stacks = append(stacks, layout.Expanded(func(gtx layout.Context) layout.Dimensions {
			return m.RenderDeleteOverlay(style, gtx)
		}))
	}
	return layout.Stack{}.Layout(gtx,
		stacks...,
	)
}
func (m *Module) RenderMainPage(style *ui.Style, gtx layout.Context) layout.Dimensions {
	children := make([]layout.FlexChild, 0)
	children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions { return m.RenderPageHeader(style, gtx) }))
	children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions { return m.RenderVolumeRow(style, gtx) }))
	children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions { return m.RenderSpellIconRow(style, gtx) }))
	children = append(children, layout.Flexed(1, func(gtx layout.Context) layout.Dimensions { return m.RenderEventsTable(style, gtx) }))
	children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions { return m.RenderImExport(style, gtx) }))
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
}
func (m *Module) RenderImExport(style *ui.Style, gtx layout.Context) layout.Dimensions {
	return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
		layout.Flexed(1, material.Label(style.Theme, ui.Sp(15), "").Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.UniformInset(unit.Dp(16)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {

				return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return ui.IconLink(style, &m.export_click, ui.Export, "Export Events").Layout(gtx)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return layout.Inset{Left: unit.Dp(16)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							return ui.IconLink(style, &m.import_click, ui.Import, "Import Events").Layout(gtx)
						})
					}),
				)
			})
		}),
	)
}
func (m *Module) RenderVolumeRow(style *ui.Style, gtx layout.Context) layout.Dimensions {
	return layout.UniformInset(unit.Dp(16)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				gtx.Constraints.Min.X = min(400, gtx.Constraints.Max.X)
				return layout.Inset{Top: unit.Dp(6), Right: unit.Dp(16)}.Layout(gtx, material.Label(style.Theme, ui.Sp(15), "Notification Volume:").Layout)
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				gtx.Constraints.Min.X = min(300, gtx.Constraints.Max.X)
				return material.Slider(style.Theme, &m.volume).Layout(gtx)
			}),
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Top: unit.Dp(6)}.Layout(gtx, material.Label(style.Theme, ui.Sp(15), fmt.Sprintf("%.02f%%", m.ctx.Config.Volume)).Layout)
			}),
		)
	})
}
func (m *Module) RenderSpellIconRow(style *ui.Style, gtx layout.Context) layout.Dimensions {
	return layout.UniformInset(unit.Dp(16)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				gtx.Constraints.Min.X = min(400, gtx.Constraints.Max.X)
				return layout.Inset{Top: unit.Dp(8), Right: unit.Dp(16)}.Layout(gtx, material.Label(style.Theme, ui.Sp(15), "Spell Icon Set:").Layout)
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return m.spell_icon_select.Layout(style, gtx, unit.Dp(gtx.Dp(200)))
			}),
			layout.Flexed(1, material.Body1(style.Theme, "").Layout),
		)
	})
}
func (m *Module) RenderPageHeader(style *ui.Style, gtx layout.Context) layout.Dimensions {
	return ui.ColoredRow(gtx, style.Palette.Window, func(gtx layout.Context) layout.Dimensions {
		return layout.UniformInset(unit.Dp(16)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			label := material.Label(style.Theme, ui.Sp(18), "Events")
			label.Font.Weight = font.SemiBold
			return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					return layout.UniformInset(unit.Dp(8)).Layout(gtx, label.Layout)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					children := make([]layout.FlexChild, 0)
					children = append(children,
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return layout.Inset{Left: unit.Dp(8)}.Layout(gtx, RenderLinkAsButton(style, &m.add_spell_click, ui.Book, "Add Spell"))
						}),
					)
					children = append(children,
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return layout.Inset{Left: unit.Dp(8)}.Layout(gtx, RenderLinkAsButton(style, &m.add_timer_click, ui.Timer, "Add Timer"))
						}),
					)
					children = append(children,
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return layout.Inset{Left: unit.Dp(8)}.Layout(gtx, RenderLinkAsButton(style, &m.add_text_click, ui.Text, "Add Text"))
						}),
					)
					children = append(children,
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return layout.Inset{Left: unit.Dp(8)}.Layout(gtx, RenderLinkAsButton(style, &m.add_regexp_click, ui.RegExp, "Add RegExp"))
						}),
					)

					return layout.Flex{Axis: layout.Horizontal}.Layout(gtx, children...)
				}),
			)
		})
	})
}
func RenderLinkAsButton(style *ui.Style, clickable *widget.Clickable, icon *widget.Icon, text string) layout.Widget {
	link := ui.IconLink(style, clickable, icon, text)
	link.Padding[ui.PADDING_TOP] = 4
	link.Padding[ui.PADDING_BOTTOM] = 4
	link.Padding[ui.PADDING_LEFT] = 8
	link.Padding[ui.PADDING_RIGHT] = 8
	return func(gtx layout.Context) layout.Dimensions {
		return ui.ColoredBorderedRow(gtx, style.Palette.Panel, link.Layout)
	}
}
