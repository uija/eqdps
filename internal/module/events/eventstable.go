package events

import (
	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"github.com/uija/eqdps/internal/data"
	"github.com/uija/eqdps/internal/ui"
)

func (m *Module) RenderEventsTable(style *ui.Style, gtx layout.Context) layout.Dimensions {
	return layout.UniformInset(unit.Dp(8)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return m.RenderEventsTableHeader(style, gtx)
			}),
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				return m.RenderEventsTableList(style, gtx)
			}),
		)
	})
}

func (m *Module) RenderEventsTableList(style *ui.Style, gtx layout.Context) layout.Dimensions {
	m.mu.Lock()
	defer m.mu.Unlock()
	size := len(m.ctx.Config.Events)
	if size != len(m.row_click) {
		m.row_click = make([]widget.Clickable, size)
		m.activate_click = make([]widget.Clickable, size)
	}
	list := material.List(style.Theme, &m.events_list)
	return list.Layout(
		gtx,
		size,
		func(gtx layout.Context, index int) layout.Dimensions {
			return m.RenderEventsTableRow(index, style, gtx)
		},
	)
}
func (m *Module) RenderEventsTableRow(index int, style *ui.Style, gtx layout.Context) layout.Dimensions {
	event := m.ctx.Config.Events[index]
	icon := ui.Close
	icon_color := style.Palette.No
	if event.Active {
		icon = ui.Check
		icon_color = style.Palette.Yes
	}
	not_icon := ui.Close
	if event.Notification != "" {
		not_icon = ui.Check
	}
	sound := "No sound"
	if event.Sound != "" {
		sound = event.Sound
	}
	return layout.UniformInset(unit.Dp(8)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,

			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				return layout.W.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return layout.Inset{Left: unit.Dp(16)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						link := ui.IconLink(style, &m.activate_click[index], icon, "")
						link.TextColor = icon_color
						return link.Layout(gtx)
					})
				})
			}),
			layout.Flexed(4, func(gtx layout.Context) layout.Dimensions {
				link := ui.Link(style, &m.row_click[index], event.Title)
				return layout.Inset{Left: unit.Dp(0)}.Layout(gtx, link.Layout)
			}),
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				str := "Unknown"
				switch event.Type {
				case data.EventTypeRegexp:
					str = "RegExp"
				case data.EventTypeSpell:
					str = "Spell"
				case data.EventTypeString:
					str = "Text"
				case data.EventTypeTimer:
					str = "Timer"
				}
				return layout.Inset{Left: unit.Dp(8)}.Layout(gtx, material.Label(style.Theme, ui.Sp(14), str).Layout)
			}),
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				return layout.W.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return layout.Inset{Left: unit.Dp(16)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return ui.Icon(gtx, style.Palette.Text, not_icon)
					})
				})
			}),
			layout.Flexed(2, func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Left: unit.Dp(8)}.Layout(gtx, material.Label(style.Theme, ui.Sp(14), sound).Layout)
			}),
		)
	})
}
func (m *Module) RenderEventsTableHeader(style *ui.Style, gtx layout.Context) layout.Dimensions {
	return ui.ColoredRow(gtx, style.Palette.Panel, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
			layout.Flexed(1,
				func(gtx layout.Context) layout.Dimensions {
					return layout.Inset{Top: unit.Dp(8), Bottom: unit.Dp(8), Left: unit.Dp(16)}.Layout(gtx,
						material.Label(style.Theme, ui.Sp(14), "ACTIVE").Layout,
					)
				},
			),
			layout.Flexed(4,
				func(gtx layout.Context) layout.Dimensions {
					return layout.Inset{Top: unit.Dp(8), Bottom: unit.Dp(8)}.Layout(gtx,
						material.Label(style.Theme, ui.Sp(14), "TITLE").Layout,
					)
				},
			),
			layout.Flexed(1,
				func(gtx layout.Context) layout.Dimensions {
					return layout.Inset{Top: unit.Dp(8), Bottom: unit.Dp(8)}.Layout(gtx,
						material.Label(style.Theme, ui.Sp(14), "TYPE").Layout,
					)
				},
			),
			layout.Flexed(1,
				func(gtx layout.Context) layout.Dimensions {
					return layout.Inset{Top: unit.Dp(8), Bottom: unit.Dp(8)}.Layout(gtx,
						material.Label(style.Theme, ui.Sp(14), "NOTIFY").Layout,
					)
				},
			),
			layout.Flexed(2,
				func(gtx layout.Context) layout.Dimensions {
					return layout.Inset{Top: unit.Dp(8), Bottom: unit.Dp(8)}.Layout(gtx,
						material.Label(style.Theme, ui.Sp(14), "SOUND").Layout,
					)
				},
			),
		)
	})

}
