package dps

import (
	"fmt"
	"sort"
	"strings"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget/material"
	"github.com/uija/eqdps/internal/data"
	"github.com/uija/eqdps/internal/ui"
)

func (m *Module) MainView(style *ui.Style, gtx layout.Context) layout.Dimensions {
	combat := m.combat
	combat.mu.RLock()
	defer combat.mu.RUnlock()

	if m.filterEditor.Text() == "" {
		m.displayHistory = m.combat.history
	} else {
		m.displayHistory = make([]*data.Fight, 0)
		search := strings.ToLower(m.filterEditor.Text())
		for _, f := range m.combat.history {
			if strings.Contains(strings.ToLower(f.Name), search) {
				m.displayHistory = append(m.displayHistory, f)
			}
		}
	}

	children := make([]layout.FlexChild, 0)
	children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions { return m.RenderPageHeader(style, gtx) }))
	children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions { return m.RenderFilterRow(style, gtx) }))
	children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions { return m.RenderTableHeader(style, gtx) }))
	if !m.replay.Load() {
		children = append(children,
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				list := material.List(style.Theme, &m.table)
				size := len(m.displayHistory)
				filter := m.filterEditor.Text()
				if m.displayCombat == combat && m.displayFilter == filter && size > m.displaySize {
					added := size - m.displaySize
					atTop := m.table.Position.First == 0 && m.table.Position.Offset == 0
					if atTop {
						m.table.Position.First = 0
						m.table.Position.Offset = 0
					} else {
						m.table.Position.First += added
					}
				}
				m.displayCombat = combat
				m.displayFilter = filter
				m.displaySize = size
				return list.Layout(
					gtx,
					size,
					func(gtx layout.Context, index int) layout.Dimensions {
						return layout.Inset{Top: unit.Dp(8), Right: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							return m.RenderFight(size-index-1, style, gtx)
						})
					},
				)
			}),
		)
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
}
func (m *Module) RenderPageHeader(style *ui.Style, gtx layout.Context) layout.Dimensions {
	//return ui.ColoredRow(gtx, style.Palette.Panel, func(gtx layout.Context) layout.Dimensions {
	return layout.UniformInset(unit.Dp(ui.PAGE_PADDING)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		icon := ui.CheckBoxOutline
		if m.ctx.Overlay != nil {
			icon = ui.CheckBox
		}
		chartIcon := ui.CheckBoxOutline
		if m.ctx.Config.ShowDpsAsCharts {
			chartIcon = ui.CheckBox
		}
		autoIcon := ui.CheckBoxOutline
		if m.ctx.Config.AutoOpenFirstRow {
			autoIcon = ui.CheckBox
		}
		return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				return material.Label(style.Theme, ui.Sp(ui.HEADER), "DPS Tracker").Layout(gtx)
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				link := ui.IconLink(style, &m.autoOpenFirst, autoIcon, "Auto open 'You'")
				return layout.Inset{Right: unit.Dp(16)}.Layout(gtx, link.Layout)
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				link := ui.IconLink(style, &m.showAsChartClick, chartIcon, "Display as chart")
				return layout.Inset{Right: unit.Dp(16)}.Layout(gtx, link.Layout)
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				link := ui.IconLink(style, &m.overlayClick, icon, "Show Overlay")
				return link.Layout(gtx)
			}),
		)
	})
	//})
}
func (m *Module) RenderFilterRow(style *ui.Style, gtx layout.Context) layout.Dimensions {

	return layout.UniformInset(unit.Dp(8)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Top: unit.Dp(7), Right: unit.Dp(16)}.Layout(gtx, material.Label(style.Theme, ui.Sp(16), "Filter:").Layout)
			}),
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				return ui.MaxedTextField(&m.filterEditor, "Filter combat list", style, gtx)
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Top: unit.Dp(7), Left: unit.Dp(16)}.Layout(gtx, ui.IconLink(style, &m.filterReset, ui.Close, "Clear").Layout)
			}),
		)
	})
}
func (m *Module) RenderTableHeader(style *ui.Style, gtx layout.Context) layout.Dimensions {
	columns := make([]layout.FlexChild, 0)
	for i, col := range m.columns {
		columns = append(columns, layout.Flexed(float32(col.weight), func(gtx layout.Context) layout.Dimensions {
			return layout.UniformInset(unit.Dp(4)).Layout(
				gtx,
				func(gtx layout.Context) layout.Dimensions {
					label := ui.ColorLabel(style.Palette.Muted, ui.Label(style, strings.ToUpper(col.title)))
					label.TextSize = 16
					//				label.Font.Weight = font.SemiBold
					if i == 0 {
						return label.Layout(gtx)
					}
					return ui.RightAlignLabel(gtx, label)
				},
			)
		}))
	}
	return layout.Inset{Right: unit.Dp(10), Top: unit.Dp(16)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Horizontal}.Layout(
			gtx, columns...,
		)
	})
}
func (m *Module) RenderFight(index int, style *ui.Style, gtx layout.Context) layout.Dimensions {
	fight := m.displayHistory[index]

	rows := make([]layout.FlexChild, 0)
	rows = append(rows, layout.Rigid(func(gtx layout.Context) layout.Dimensions { return m.RenderFightHeader(fight, style, gtx) }))
	rows = append(rows, m.GenerateFightCombatantRows(fight, style, gtx)...)
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, rows...)
}
func (m *Module) RenderFightHeader(fight *data.Fight, style *ui.Style, gtx layout.Context) layout.Dimensions {
	return ui.ColoredRow(gtx, style.Palette.Panel, func(gtx layout.Context) layout.Dimensions {
		return layout.Inset{Top: unit.Dp(8), Bottom: unit.Dp(8), Left: unit.Dp(16)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal}.Layout(
				gtx,
				layout.Flexed(8, func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							label := ui.Label(style, fight.Name)
							label.TextSize = ui.Sp(18)
							label.Font.Weight = font.SemiBold
							return layout.Inset{Top: unit.Dp(5)}.Layout(gtx, label.Layout)
						}),
						layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
							cnt := ""
							color := style.Palette.Inactive
							if fight.EndReason != "" {
								switch fight.EndReason {
								case data.END_REASON_ZONED:
									cnt = "zoned out"
								case data.END_REASON_TIMEOUT:
									cnt = "timeout"
								case data.END_REASON_FD:
									cnt = "feign death"
								case data.END_REASON_DEATH:
									cnt = "you died"
								default:
									cnt = fmt.Sprintf("Killed by %s", fight.EndReason)
								}
							} else {
								color = style.Palette.Active
								cnt = "Active fight"
							}

							label := ui.ColorLabel(color, material.Body2(style.Theme, cnt))
							return layout.Inset{Left: unit.Dp(8), Top: unit.Dp(4)}.Layout(gtx, label.Layout)
						}),
					)
				}),
				layout.Flexed(5, func(gtx layout.Context) layout.Dimensions {
					cnt := ""
					if fight.EndReason != "" {
						cnt = fmt.Sprintf("Killed %s", fight.End.Format("2006-01-02 15:04"))
					} else {
						cnt = fmt.Sprintf("Started %s", fight.Start.Format("2006-01-02 15:04"))
					}
					gtx.Constraints.Min.X = gtx.Constraints.Max.X
					label := ui.ColorLabel(style.Palette.Muted, ui.Label(style, cnt))
					label.Alignment = text.End
					label.TextSize = 13
					return layout.Inset{Top: unit.Dp(4), Right: unit.Dp(8)}.Layout(gtx, label.Layout)
				}),
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					dur := fight.End.Sub(fight.Start)
					minutes := int(dur.Minutes())
					seconds := int(dur.Seconds()) % 60
					label := ui.Label(style, fmt.Sprintf("%02d:%02d", minutes, seconds))
					return ui.RightAlignLabel(gtx, label)
				}),
			)
		})
	})
}
func (m *Module) GenerateFightCombatantRows(fight *data.Fight, style *ui.Style, gtx layout.Context) []layout.FlexChild {
	rows := make([]layout.FlexChild, 0)
	names := make([]string, 0)
	for name := range fight.Combatants {
		names = append(names, name)
	}
	sort.Slice(names, func(i, j int) bool {
		if fight.Combatants[names[i]].Overall.Damage == fight.Combatants[names[j]].Overall.Damage {
			return names[i] < names[j]
		}
		return fight.Combatants[names[i]].Overall.Damage > fight.Combatants[names[j]].Overall.Damage
	})
	for idx, name := range names {
		rows = append(rows, m.GenerateFightCombatantDetails(fight.Combatants[name], idx, style, gtx)...)
	}
	return rows
}
