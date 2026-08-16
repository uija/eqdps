package dps

import (
	"fmt"
	"math"
	"sort"
	"strings"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget/material"
	"github.com/uija/eqdps/internal/ui"
)

func (m *Module) MainView(style *ui.Style, gtx layout.Context) layout.Dimensions {
	combat := m.combat
	combat.mu.RLock()
	defer combat.mu.RUnlock()

	children := make([]layout.FlexChild, 0)
	children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions { return m.RenderPageHeader(style, gtx) }))
	children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions { return m.RenderTableHeader(style, gtx) }))
	if !m.replay.Load() {
		children = append(children,
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				list := material.List(style.Theme, &m.table)
				size := len(m.combat.history)
				return list.Layout(
					gtx,
					len(m.combat.history),
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
	return ui.ColoredRow(gtx, style.Palette.Panel, func(gtx layout.Context) layout.Dimensions {
		return layout.UniformInset(unit.Dp(8)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			icon := ui.CheckBoxOutline
			if m.overlay != nil {
				icon = ui.CheckBox
			}
			return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					return material.Label(style.Theme, unit.Sp(15), "DPS Tracker").Layout(gtx)
				}),
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					link := ui.IconLink(style, &m.overlayClick, icon, "Show Overlay")
					link.TextAlign = text.End
					return link.Layout(gtx)
				}),
			)
		})
	})
}
func (m *Module) RenderTableHeader(style *ui.Style, gtx layout.Context) layout.Dimensions {
	columns := make([]layout.FlexChild, 0)
	for i, col := range m.columns {
		columns = append(columns, layout.Flexed(float32(col.weight), func(gtx layout.Context) layout.Dimensions {
			return layout.UniformInset(unit.Dp(4)).Layout(
				gtx,
				func(gtx layout.Context) layout.Dimensions {
					label := material.Body1(style.Theme, strings.ToUpper(col.title))
					label.Color = style.Palette.Muted
					label.TextSize = 16
					label.Font.Weight = font.SemiBold
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
	fight := m.combat.history[index]

	rows := make([]layout.FlexChild, 0)
	rows = append(rows, layout.Rigid(func(gtx layout.Context) layout.Dimensions { return m.RenderFightHeader(fight, style, gtx) }))
	rows = append(rows, m.GenerateFightCombatantRows(fight, style, gtx)...)
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, rows...)
}
func (m *Module) RenderFightHeader(fight *Fight, style *ui.Style, gtx layout.Context) layout.Dimensions {
	return ui.ColoredRow(gtx, style.Palette.Panel, func(gtx layout.Context) layout.Dimensions {
		return layout.Inset{Top: unit.Dp(8), Bottom: unit.Dp(8), Left: unit.Dp(16)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal}.Layout(
				gtx,
				layout.Flexed(8, func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							label := material.Body1(style.Theme, fight.name)
							label.TextSize = unit.Sp(18)
							label.Font.Weight = font.SemiBold
							return layout.Inset{Top: unit.Dp(5)}.Layout(gtx, label.Layout)
						}),
						layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
							cnt := ""
							color := style.Palette.Inactive
							if fight.endReason != "" {
								switch fight.endReason {
								case END_REASON_ZONED:
									cnt = "zoned out"
								case END_REASON_TIMEOUT:
									cnt = "timeout"
								case END_REASON_FD:
									cnt = "feign death"
								case END_REASON_DEATH:
									cnt = "you died"
								default:
									cnt = fmt.Sprintf("Killed by %s", fight.endReason)
								}
							} else {
								color = style.Palette.Active
								cnt = "Active fight"
							}

							label := material.Body2(style.Theme, cnt)
							label.Color = color
							return layout.Inset{Left: unit.Dp(8), Top: unit.Dp(4)}.Layout(gtx, label.Layout)
						}),
					)
				}),
				layout.Flexed(4, func(gtx layout.Context) layout.Dimensions {
					cnt := ""
					if fight.endReason != "" {
						cnt = fmt.Sprintf("Killed %s", fight.end.Format("2006-01-02 15:04"))
					} else {
						cnt = fmt.Sprintf("Started %s", fight.start.Format("2006-01-02 15:04"))
					}
					gtx.Constraints.Min.X = gtx.Constraints.Max.X
					label := material.Body1(style.Theme, cnt)
					label.Alignment = text.End
					label.TextSize = 13
					label.Color = style.Palette.Muted
					return layout.Inset{Top: unit.Dp(4), Right: unit.Dp(8)}.Layout(gtx, label.Layout)
				}),
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					dur := fight.end.Sub(fight.start)
					minutes := int(dur.Minutes())
					seconds := int(dur.Seconds()) % 60
					label := material.Body1(style.Theme, fmt.Sprintf("%02d:%02d", minutes, seconds))
					return ui.RightAlignLabel(gtx, label)
				}),
			)
		})
	})
}
func (m *Module) GenerateFightCombatantRows(fight *Fight, style *ui.Style, gtx layout.Context) []layout.FlexChild {
	rows := make([]layout.FlexChild, 0)
	names := make([]string, 0)
	for name := range fight.combatants {
		names = append(names, name)
	}
	sort.Slice(names, func(i, j int) bool {
		return fight.combatants[names[i]].overall.damage > fight.combatants[names[j]].overall.damage
	})
	for idx, name := range names {
		rows = append(rows, m.GenerateFightCombatantDetails(fight.combatants[name], idx, style, gtx)...)
	}
	return rows
}
func (m *Module) GenerateFightCombatantDetails(c *Combatant, idx int, style *ui.Style, gtx layout.Context) []layout.FlexChild {
	rows := make([]layout.FlexChild, 0)
	pre := " + "
	if c.open {
		pre = " - "
	}
	color := style.Palette.Panel
	if idx%2 == 0 {
		color = style.Palette.Window
	}
	rows = append(rows,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return c.click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return ui.ColoredAccentedRow(gtx, color, style.Palette.Accent, strings.ToLower(c.name) == "you", func(gtx layout.Context) layout.Dimensions {
					return m.GenerateFightDetailsRow(0, fmt.Sprintf("%s%s", pre, c.name), c.overall, style, gtx)
				})
			})
		}),
	)

	if c.open {
		for _, catname := range DamageCategories {
			if cat, ok := c.categories[catname]; ok {
				rows = append(rows,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						// Spacing to the top, so Category names dont directly connect to Combatant rows
						return layout.Inset{Top: unit.Dp(4)}.Layout(gtx, func(layout.Context) layout.Dimensions {
							return ui.ColoredRow(gtx, style.Palette.Panel, func(gtx layout.Context) layout.Dimensions {
								return m.GenerateFightDetailsRow(1, catname, cat.overall, style, gtx)
							})
						})
					}),
				)
				names := make([]string, 0)
				for name := range cat.abilities {
					names = append(names, name)
				}
				sort.Strings(names)
				for _, name := range names {
					rows = append(rows,
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return m.GenerateFightDetailsRow(2, name, cat.abilities[name], style, gtx)
						}),
					)
				}
			}
		}
	}
	return rows
}
func (m *Module) GenerateFightDetailsRow(intent int, name string, d *CombatDamageData, style *ui.Style, gtx layout.Context) layout.Dimensions {
	cells := make([]layout.FlexChild, 0)

	cells = append(cells, layout.Flexed(float32(m.columns[0].weight), func(gtx layout.Context) layout.Dimensions {
		return layout.Inset{Left: unit.Dp(intent * 24)}.Layout(gtx, material.Body1(style.Theme, name).Layout)
	}))
	cells = append(cells, layout.Flexed(float32(m.columns[1].weight), func(gtx layout.Context) layout.Dimensions {
		return ui.RightAlignLabel(gtx, material.Body1(style.Theme, fmt.Sprintf("%d", d.damage)))
	}))
	cells = append(cells, layout.Flexed(float32(m.columns[2].weight), func(gtx layout.Context) layout.Dimensions {
		return ui.RightAlignLabel(gtx, material.Body1(style.Theme, fmt.Sprintf("%d", int(math.Round(d.DPS())))))
	}))
	cells = append(cells, layout.Flexed(float32(m.columns[3].weight), func(gtx layout.Context) layout.Dimensions {
		return ui.RightAlignLabel(gtx, material.Body1(style.Theme, fmt.Sprintf("%d", d.hits)))
	}))
	cells = append(cells, layout.Flexed(float32(m.columns[4].weight), func(gtx layout.Context) layout.Dimensions {
		return ui.RightAlignLabel(gtx, material.Body1(style.Theme, fmt.Sprintf("%d", d.crits)))
	}))
	cells = append(cells, layout.Flexed(float32(m.columns[5].weight), func(gtx layout.Context) layout.Dimensions {
		dur := d.lastUpdate.Sub(d.start)
		minutes := int(dur.Minutes())
		seconds := int(dur.Seconds()) % 60
		label := material.Body1(style.Theme, fmt.Sprintf("%02d:%02d", minutes, seconds))
		label.Color = style.Palette.Accent
		return ui.RightAlignLabel(gtx, label)
	}))
	// each row gets its own padding
	return layout.Inset{Left: unit.Dp(8), Top: unit.Dp(4), Bottom: unit.Dp(4)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Horizontal}.Layout(gtx, cells...)
	})
}
