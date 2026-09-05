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
	return layout.UniformInset(unit.Dp(8)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		icon := ui.CheckBoxOutline
		if m.ctx.Overlay != nil {
			icon = ui.CheckBox
		}
		return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				return material.Label(style.Theme, ui.Sp(17), "DPS Tracker").Layout(gtx)
			}),
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				link := ui.IconLink(style, &m.overlayClick, icon, "Show Overlay")
				link.TextAlign = text.End
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
					label := material.Body1(style.Theme, strings.ToUpper(col.title))
					label.Color = style.Palette.Muted
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
							label := material.Body1(style.Theme, fight.Name)
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

							label := material.Body2(style.Theme, cnt)
							label.Color = color
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
					label := material.Body1(style.Theme, cnt)
					label.Alignment = text.End
					label.TextSize = 13
					label.Color = style.Palette.Muted
					return layout.Inset{Top: unit.Dp(4), Right: unit.Dp(8)}.Layout(gtx, label.Layout)
				}),
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					dur := fight.End.Sub(fight.Start)
					minutes := int(dur.Minutes())
					seconds := int(dur.Seconds()) % 60
					label := material.Body1(style.Theme, fmt.Sprintf("%02d:%02d", minutes, seconds))
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
func (m *Module) GenerateFightCombatantDetails(c *data.Combatant, idx int, style *ui.Style, gtx layout.Context) []layout.FlexChild {
	rows := make([]layout.FlexChild, 0)
	pre := " + "
	if c.Open {
		pre = " - "
	}
	color := style.Palette.Panel
	if idx%2 == 0 {
		color = style.Palette.Window
	}
	combatantIsYou := strings.EqualFold(c.Name, "you")
	rows = append(rows,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return c.Click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return ui.ColoredAccentedRow(gtx, color, style.Palette.Accent, combatantIsYou, func(gtx layout.Context) layout.Dimensions {
					return m.GenerateFightDetailsRow(0, false, "", fmt.Sprintf("%s%s", pre, c.Name), c, c.Overall, style, gtx)
				})
			})
		}),
	)

	if c.Open {
		for _, catname := range data.DamageCategories {
			if cat, ok := c.Categories[catname]; ok {
				showDetails := strings.EqualFold(catname, "melee") && combatantIsYou
				rows = append(rows,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						// Spacing to the top, so Category names dont directly connect to Combatant rows
						return layout.Inset{Top: unit.Dp(4)}.Layout(gtx, func(layout.Context) layout.Dimensions {
							return ui.ColoredRow(gtx, style.Palette.Panel, func(gtx layout.Context) layout.Dimensions {
								return m.GenerateFightDetailsRow(1, showDetails, catname, catname, c, cat.Overall, style, gtx)
							})
						})
					}),
				)
				names := make([]string, 0)
				for name := range cat.Abilities {
					names = append(names, name)
				}
				sort.Strings(names)
				for _, name := range names {
					rows = append(rows,
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return m.GenerateFightDetailsRow(2, showDetails, catname, name, c, cat.Abilities[name], style, gtx)
						}),
					)
				}
			}
		}
	}
	return rows
}
func (m *Module) GenerateFightDetailsRow(intent int, showDetails bool, category string, name string, combatant *data.Combatant, d *data.CombatDamageData, style *ui.Style, gtx layout.Context) layout.Dimensions {
	cells := make([]layout.FlexChild, 0)

	var sdps float64 = 0
	dps := d.DPS()
	if strings.Contains(name, "You") {
		sdps = d.SDPS(d.LastUpdate.Sub(combatant.FirstParticipation))
		if sdps > 0 && sdps < dps*1.1 {
			sdps = 0
		}
	}
	info := ""
	if showDetails {
		numHits := d.Hits
		numAll := d.NumAttacks()
		percent := 0.0
		if numHits > 0 {
			percent = 100 / float64(numAll) * float64(numHits)
		}
		addValue := func(str string, format string, val any) string {
			if val == 0 {
				return str
			}
			if str != "" {
				str += ", "
			}
			str += fmt.Sprintf(format, val)
			return str
		}
		info = addValue(info, "%d slay undead", d.SlayUndead)
		info = addValue(info, "%d miss", d.Miss)
		info = addValue(info, "%d dodge", d.Dodge)
		info = addValue(info, "%d parry", d.Parry)
		info = addValue(info, "%d block", d.Block)
		info = addValue(info, "%d absorb", d.Absorb)
		info = addValue(info, "%d riposte", d.Riposte)
		info = addValue(info, "%.01f%%", percent)
	} else if category == data.CATEGORY_PROCS && category != name {
		fight_duration := combatant.Overall.LastUpdate.Sub(combatant.FirstParticipation)
		if fight_duration > 0 {
			ppm := float64(d.Hits) / fight_duration.Minutes()
			info = fmt.Sprintf("%.1f ppm", ppm)

		}
	}

	cells = append(cells, layout.Flexed(float32(m.columns[0].weight), func(gtx layout.Context) layout.Dimensions {
		if info != "" {
			return layout.Inset{Left: unit.Dp(intent * 24)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
					layout.Rigid(material.Body1(style.Theme, name).Layout),
					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
						return layout.Inset{Left: unit.Dp(8), Top: unit.Dp(2)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							label := material.Label(style.Theme, ui.Sp(14), info)
							label.Color = style.Palette.Muted
							return layout.E.Layout(gtx, label.Layout)
						})
					}),
				)
			})
		}
		return layout.Inset{Left: unit.Dp(intent * 24)}.Layout(gtx, material.Body1(style.Theme, name).Layout)
	}))
	cells = append(cells, layout.Flexed(float32(m.columns[1].weight), func(gtx layout.Context) layout.Dimensions {
		return ui.RightAlignLabel(gtx, material.Body1(style.Theme, fmt.Sprintf("%d", d.Damage)))
	}))
	cells = append(cells, layout.Flexed(float32(m.columns[2].weight), func(gtx layout.Context) layout.Dimensions {
		color := style.Palette.Text
		if sdps > 0 {
			color = style.Palette.Muted
		}
		return ui.RightAlignLabel(gtx, ui.ColoredLabel(style.Theme, 16, color, fmt.Sprintf("%d", int(math.Round(dps)))))
	}))
	cells = append(cells, layout.Flexed(float32(m.columns[2].weight), func(gtx layout.Context) layout.Dimensions {
		sdpsstr := ""
		if sdps > 0 {
			sdpsstr = fmt.Sprintf("%d", int(math.Round(sdps)))
		}
		return ui.CenterAlignLabel(gtx, ui.ColoredLabel(style.Theme, 16, style.Palette.Yes, sdpsstr))
	}))
	cells = append(cells, layout.Flexed(float32(m.columns[3].weight), func(gtx layout.Context) layout.Dimensions {
		val := fmt.Sprintf("%d", d.Hits)
		return ui.RightAlignLabel(gtx, material.Body1(style.Theme, val))
	}))
	cells = append(cells, layout.Flexed(float32(m.columns[4].weight), func(gtx layout.Context) layout.Dimensions {
		return ui.RightAlignLabel(gtx, material.Body1(style.Theme, fmt.Sprintf("%d", d.Crits)))
	}))
	cells = append(cells, layout.Flexed(float32(m.columns[5].weight), func(gtx layout.Context) layout.Dimensions {
		dur := d.LastUpdate.Sub(d.Start)
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
