package dps

import (
	"fmt"
	"image"
	"math"
	"sort"
	"strings"

	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget/material"
	"github.com/uija/eqdps/internal/data"
	"github.com/uija/eqdps/internal/ui"
)

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
		if m.ctx.Config.ShowDpsAsCharts {
			rows = append(rows, m.GenerateSortedDetails(c, combatantIsYou, style, gtx)...)
		} else {
			rows = append(rows, m.GenerateCategorizedDetails(c, combatantIsYou, style, gtx)...)
		}
	}
	return rows
}

type SortedContainer struct {
	Category   string
	DamageData *data.CombatDamageData
}

func (m *Module) GenerateSortedDetails(c *data.Combatant, combatantIsYou bool, style *ui.Style, gtx layout.Context) []layout.FlexChild {
	cdd := make([]SortedContainer, 0)

	rows := make([]layout.FlexChild, 0)
	overall := 0
	for _, cat := range c.Categories {
		for _, ability := range cat.Abilities {
			overall += ability.Damage
			cdd = append(cdd, SortedContainer{Category: cat.Overall.Name, DamageData: ability})
		}
	}
	sort.Slice(cdd, func(i, j int) bool {
		if cdd[i].DamageData.Damage == cdd[j].DamageData.Damage {
			return cdd[i].DamageData.Name < cdd[j].DamageData.Name
		}
		return cdd[i].DamageData.Damage > cdd[j].DamageData.Damage
	})
	for idx, details := range cdd {
		barcolor := style.Palette.BarOne
		if idx%2 == 0 {
			barcolor = style.Palette.BarTwo
		}
		factor := 1.0 / float32(overall) * float32(details.DamageData.Damage)
		showDetails := strings.EqualFold(details.Category, "melee") && combatantIsYou
		rows = append(rows, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Stack{}.Layout(gtx,
				layout.Expanded(func(gtx layout.Context) layout.Dimensions {
					gtx.Constraints.Min.X = int(float32(gtx.Constraints.Max.X) * factor)
					ui.Fill(gtx, barcolor)
					return layout.Dimensions{Size: image.Pt(gtx.Constraints.Min.X, gtx.Constraints.Min.Y)}
				}),
				layout.Stacked(func(gtx layout.Context) layout.Dimensions {
					return m.GenerateFightDetailsRow(2, showDetails, details.Category, details.DamageData.Name, c, details.DamageData, style, gtx)
				}),
			)
		}))
	}

	return rows
}
func (m *Module) GenerateCategorizedDetails(c *data.Combatant, combatantIsYou bool, style *ui.Style, gtx layout.Context) []layout.FlexChild {
	rows := make([]layout.FlexChild, 0)
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
					layout.Rigid(ui.Label(style, name).Layout),
					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
						return layout.Inset{Left: unit.Dp(8), Top: unit.Dp(2)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							label := ui.ColorLabel(style.Palette.Muted, material.Label(style.Theme, ui.Sp(14), info))
							return layout.E.Layout(gtx, label.Layout)
						})
					}),
				)
			})
		}
		return layout.Inset{Left: unit.Dp(intent * 24)}.Layout(gtx, ui.Label(style, name).Layout)
	}))
	cells = append(cells, layout.Flexed(float32(m.columns[1].weight), func(gtx layout.Context) layout.Dimensions {
		return ui.RightAlignLabel(gtx, ui.Label(style, fmt.Sprintf("%d", d.Damage)))
	}))
	cells = append(cells, layout.Flexed(float32(m.columns[2].weight), func(gtx layout.Context) layout.Dimensions {
		color := style.Palette.Text
		if sdps > 0 {
			color = style.Palette.Muted
		}
		return ui.RightAlignLabel(gtx, ui.ColorLabel(color, material.Label(style.Theme, ui.Sp(16), fmt.Sprintf("%d", int(math.Round(dps))))))
	}))
	cells = append(cells, layout.Flexed(float32(m.columns[2].weight), func(gtx layout.Context) layout.Dimensions {
		sdpsstr := ""
		if sdps > 0 {
			sdpsstr = fmt.Sprintf("%d", int(math.Round(sdps)))
		}
		return ui.CenterAlignLabel(gtx, ui.ColorLabel(style.Palette.Yes, material.Label(style.Theme, ui.Sp(16), sdpsstr)))
	}))
	cells = append(cells, layout.Flexed(float32(m.columns[3].weight), func(gtx layout.Context) layout.Dimensions {
		val := fmt.Sprintf("%d", d.Hits)
		return ui.RightAlignLabel(gtx, ui.Label(style, val))
	}))
	cells = append(cells, layout.Flexed(float32(m.columns[4].weight), func(gtx layout.Context) layout.Dimensions {
		return ui.RightAlignLabel(gtx, ui.Label(style, fmt.Sprintf("%d", d.Crits)))
	}))
	cells = append(cells, layout.Flexed(float32(m.columns[5].weight), func(gtx layout.Context) layout.Dimensions {
		dur := d.LastUpdate.Sub(d.Start)
		minutes := int(dur.Minutes())
		seconds := int(dur.Seconds()) % 60
		return ui.RightAlignLabel(gtx, ui.ColorLabel(style.Palette.Accent, ui.Label(style, fmt.Sprintf("%02d:%02d", minutes, seconds))))
	}))
	// each row gets its own padding
	return layout.Inset{Left: unit.Dp(8), Top: unit.Dp(4), Bottom: unit.Dp(4)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Horizontal}.Layout(gtx, cells...)
	})
}
