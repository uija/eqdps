package dps

import (
	"fmt"
	"image"
	"math"
	"sort"
	"strings"
	"time"

	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget/material"
	"github.com/uija/eqdps/internal/data"
	"github.com/uija/eqdps/internal/ui"
)

func (m *Module) GenerateFightCombatantDetails(c *data.Combatant, idx int, fightDuration time.Duration, style *ui.Style, gtx layout.Context) []layout.FlexChild {
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
					return m.GenerateFightDetailsRow(0, false, "", fmt.Sprintf("%s%s", pre, c.Name), fightDuration, c, c.Overall, style, gtx)
				})
			})
		}),
	)

	if c.Open {
		if m.ctx.Config.ShowDpsAsCharts {
			rows = append(rows, m.GenerateSortedDetails(c, combatantIsYou, fightDuration, style, gtx)...)
		} else {
			rows = append(rows, m.GenerateCategorizedDetails(c, combatantIsYou, fightDuration, style, gtx)...)
		}
	}
	return rows
}

type SortedContainer struct {
	Category   string
	DamageData *data.CombatDamageData
}
type SortedHealingContainer struct {
	Category    string
	HealingData *data.HealingData
}

func (m *Module) GenerateSortedDetails(c *data.Combatant, combatantIsYou bool, fightDuration time.Duration, style *ui.Style, gtx layout.Context) []layout.FlexChild {
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
		showDetails := strings.EqualFold(details.Category, "melee")
		rows = append(rows, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Stack{}.Layout(gtx,
				layout.Expanded(func(gtx layout.Context) layout.Dimensions {
					gtx.Constraints.Min.X = int(float32(gtx.Constraints.Max.X) * factor)
					ui.Fill(gtx, barcolor)
					return layout.Dimensions{Size: image.Pt(gtx.Constraints.Min.X, gtx.Constraints.Min.Y)}
				}),
				layout.Stacked(func(gtx layout.Context) layout.Dimensions {
					return m.GenerateFightDetailsRow(2, showDetails, details.Category, details.DamageData.Name, fightDuration, c, details.DamageData, style, gtx)
				}),
			)
		}))
	}
	hdd := make([]SortedHealingContainer, 0)
	var oheal int64 = 0
	for name, cat := range c.HealingCategories {
		for _, ability := range cat.Abilities {
			oheal += ability.Amount
			hdd = append(hdd, SortedHealingContainer{Category: name, HealingData: ability})
		}
	}
	if oheal > 0 {
		hps := float64(oheal) / fightDuration.Seconds()
		rows = append(rows, layout.Rigid(m.GenerateHealingHeader(style, hps)))
		sort.Slice(hdd, func(i, j int) bool {
			if hdd[i].HealingData.Amount == hdd[j].HealingData.Amount {
				return hdd[i].HealingData.Name < hdd[j].HealingData.Name
			}
			return hdd[i].HealingData.Amount > hdd[j].HealingData.Amount
		})
		for idx, details := range hdd {
			barcolor := style.Palette.BarOne
			if idx%2 == 0 {
				barcolor = style.Palette.BarTwo
			}
			factor := 1.0 / float32(oheal) * float32(details.HealingData.Amount)
			rows = append(rows, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Stack{}.Layout(gtx,
					layout.Expanded(func(gtx layout.Context) layout.Dimensions {
						gtx.Constraints.Min.X = int(float32(gtx.Constraints.Max.X) * factor)
						ui.Fill(gtx, barcolor)
						return layout.Dimensions{Size: image.Pt(gtx.Constraints.Min.X, gtx.Constraints.Min.Y)}
					}),
					layout.Stacked(func(gtx layout.Context) layout.Dimensions {
						return m.GenerateHealDetailsRow(2, details.Category, details.HealingData.Name, fightDuration, c, details.HealingData, style, gtx)
					}),
				)
			}))
		}
	}

	return rows
}
func (m *Module) GenerateHealingHeader(style *ui.Style, hps float64) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		return ui.ColoredRow(gtx, style.Palette.LightPanel, func(gtx layout.Context) layout.Dimensions {
			cells := make([]layout.FlexChild, 0)
			cells = append(cells, layout.Flexed(float32(m.columns[0].weight), func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Left: unit.Dp(24)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return ui.ColorLabel(style.Palette.Headline, ui.Label(style, "Healing")).Layout(gtx)
				})
			}))
			cells = append(cells, layout.Flexed(float32(m.columns[1].weight), func(gtx layout.Context) layout.Dimensions {
				return ui.RightAlignLabel(gtx, ui.ColorLabel(style.Palette.Headline, ui.Label(style, "Amount")))
			}))
			cells = append(cells, layout.Flexed(float32(m.columns[2].weight), func(gtx layout.Context) layout.Dimensions {
				return ui.RightAlignLabel(gtx, ui.ColorLabel(style.Palette.Headline, ui.Label(style, "HPS")))
			}))
			cells = append(cells, layout.Flexed(float32(m.columns[3].weight), layout.Spacer{}.Layout))
			cells = append(cells, layout.Flexed(float32(m.columns[4].weight), func(gtx layout.Context) layout.Dimensions {
				return ui.RightAlignLabel(gtx, ui.ColorLabel(style.Palette.Headline, ui.Label(style, "Heals")))
			}))
			cells = append(cells, layout.Flexed(float32(m.columns[5].weight), func(gtx layout.Context) layout.Dimensions {
				return ui.RightAlignLabel(gtx, ui.ColorLabel(style.Palette.Headline, ui.Label(style, "Crits")))
			}))
			cells = append(cells, layout.Flexed(float32(m.columns[6].weight), func(gtx layout.Context) layout.Dimensions {
				return ui.RightAlignLabel(gtx, ui.ColorLabel(style.Palette.Headline, ui.Label(style, m.ctx.Sprintf("%.0f", hps))))
			}))
			cells = append(cells, layout.Flexed(float32(m.columns[7].weight), func(gtx layout.Context) layout.Dimensions {
				return ui.RightAlignLabel(gtx, ui.ColorLabel(style.Palette.Headline, ui.Label(style, "Active")))
			}))
			return layout.Inset{Left: unit.Dp(8), Top: unit.Dp(4), Bottom: unit.Dp(4)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Horizontal}.Layout(gtx, cells...)
			})
		})
	}
}
func (m *Module) GenerateCategorizedDetails(c *data.Combatant, combatantIsYou bool, fightDuration time.Duration, style *ui.Style, gtx layout.Context) []layout.FlexChild {
	rows := make([]layout.FlexChild, 0)
	for _, catname := range data.DamageCategories {
		if cat, ok := c.Categories[catname]; ok {
			showDetails := strings.EqualFold(catname, "melee")
			rows = append(rows,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					// Spacing to the top, so Category names dont directly connect to Combatant rows
					return layout.Inset{Top: unit.Dp(4)}.Layout(gtx, func(layout.Context) layout.Dimensions {
						return ui.ColoredRow(gtx, style.Palette.Panel, func(gtx layout.Context) layout.Dimensions {
							return m.GenerateFightDetailsRow(1, showDetails, catname, catname, fightDuration, c, cat.Overall, style, gtx)
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
						return m.GenerateFightDetailsRow(2, showDetails, catname, name, fightDuration, c, cat.Abilities[name], style, gtx)
					}),
				)
			}
		}
	}
	healing := make([]layout.FlexChild, 0)
	for _, catname := range data.HealingCategories {
		if cat, ok := c.HealingCategories[catname]; ok {
			healing = append(healing,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Inset{Top: unit.Dp(4)}.Layout(gtx, func(layout.Context) layout.Dimensions {
						return ui.ColoredRow(gtx, style.Palette.Panel, func(gtx layout.Context) layout.Dimensions {
							return m.GenerateHealDetailsRow(1, catname, catname, fightDuration, c, cat.Overall, style, gtx)
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
				healing = append(healing,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return m.GenerateHealDetailsRow(2, catname, name, fightDuration, c, cat.Abilities[name], style, gtx)
					}),
				)
			}
		}
	}
	if len(healing) > 0 {
		hps := float64(c.OverallHealing.Amount) / fightDuration.Seconds()
		rows = append(rows, layout.Rigid(m.GenerateHealingHeader(style, hps)))
		rows = append(rows, healing...)
	}
	return rows
}
func (m *Module) GenerateHealDetailsRow(intent int, category string, name string, fightDuration time.Duration, combatant *data.Combatant, h *data.HealingData, style *ui.Style, gtx layout.Context) layout.Dimensions {
	cells := make([]layout.FlexChild, 0)
	cells = append(cells, layout.Flexed(float32(m.columns[0].weight), func(gtx layout.Context) layout.Dimensions {
		return layout.Inset{Left: unit.Dp(intent * 24)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return ui.Label(style, name).Layout(gtx)
		})
	}))
	cells = append(cells, layout.Flexed(float32(m.columns[1].weight), func(gtx layout.Context) layout.Dimensions {
		return ui.RightAlignLabel(gtx, ui.Label(style, m.ctx.Sprintf("%d", h.Amount)))
	}))
	hps := float64(h.Amount) / fightDuration.Seconds()
	cells = append(cells, layout.Flexed(float32(m.columns[2].weight), func(gtx layout.Context) layout.Dimensions {
		return ui.RightAlignLabel(gtx, ui.Label(style, m.ctx.Sprintf("%.0f", hps)))
	}))
	cells = append(cells, layout.Flexed(float32(m.columns[3].weight), layout.Spacer{}.Layout))
	cells = append(cells, layout.Flexed(float32(m.columns[4].weight), func(gtx layout.Context) layout.Dimensions {
		return ui.RightAlignLabel(gtx, ui.Label(style, m.ctx.Sprintf("%d", h.Count)))
	}))
	cells = append(cells, layout.Flexed(float32(m.columns[5].weight), func(gtx layout.Context) layout.Dimensions {
		str := m.ctx.Sprintf("%d", h.Crit)
		return ui.RightAlignLabel(gtx, ui.Label(style, str))
	}))
	cells = append(cells, layout.Flexed(float32(m.columns[6].weight), layout.Spacer{}.Layout))
	cells = append(cells, layout.Flexed(float32(m.columns[7].weight), func(gtx layout.Context) layout.Dimensions {
		minutes := int(fightDuration.Minutes())
		seconds := int(fightDuration.Seconds()) % 60
		return ui.RightAlignLabel(gtx, ui.ColorLabel(style.Palette.Accent, ui.Label(style, fmt.Sprintf("%02d:%02d", minutes, seconds))))
	}))
	return layout.Inset{Left: unit.Dp(8), Top: unit.Dp(4), Bottom: unit.Dp(4)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Horizontal}.Layout(gtx, cells...)
	})
}
func (m *Module) GenerateFightDetailsRow(intent int, showDetails bool, category string, name string, fightDuration time.Duration, combatant *data.Combatant, d *data.CombatDamageData, style *ui.Style, gtx layout.Context) layout.Dimensions {
	cells := make([]layout.FlexChild, 0)

	var sdps float64 = 0
	var hps float64 = 0
	dps := d.DPS()
	if strings.Contains(name, "You") {
		sdps = d.SDPS(d.LastUpdate.Sub(combatant.FirstParticipation))
		if sdps > 0 && sdps < dps*1.1 {
			sdps = 0
		}
	}
	if strings.Contains(name, combatant.Name) {
		hps = float64(combatant.OverallHealing.Amount) / fightDuration.Seconds()
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
			str += m.ctx.Sprintf(format, val)
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
			info = m.ctx.Sprintf("%.1f ppm", ppm)
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
		label := ui.Label(style, name)
		if intent == 0 {
			label.TextSize += ui.Sp(1)
		}
		return layout.Inset{Left: unit.Dp(intent * 24)}.Layout(gtx, label.Layout)
	}))
	cells = append(cells, layout.Flexed(float32(m.columns[1].weight), func(gtx layout.Context) layout.Dimensions {
		return ui.RightAlignLabel(gtx, ui.Label(style, m.ctx.Sprintf("%d", d.Damage)))
	}))
	cells = append(cells, layout.Flexed(float32(m.columns[2].weight), func(gtx layout.Context) layout.Dimensions {
		color := style.Palette.Text
		if sdps > 0 {
			color = style.Palette.Muted
		}
		return ui.RightAlignLabel(gtx, ui.ColorLabel(color, material.Label(style.Theme, ui.Sp(16), m.ctx.Sprintf("%d", int(math.Round(dps))))))
	}))
	cells = append(cells, layout.Flexed(float32(m.columns[3].weight), func(gtx layout.Context) layout.Dimensions {
		sdpsstr := ""
		if sdps > 0 {
			sdpsstr = m.ctx.Sprintf("%d", int(math.Round(sdps)))
		}
		return ui.CenterAlignLabel(gtx, ui.ColorLabel(style.Palette.Yes, material.Label(style.Theme, ui.Sp(16), sdpsstr)))
	}))
	cells = append(cells, layout.Flexed(float32(m.columns[4].weight), func(gtx layout.Context) layout.Dimensions {
		val := m.ctx.Sprintf("%d", d.Hits)
		return ui.RightAlignLabel(gtx, ui.Label(style, val))
	}))
	cells = append(cells, layout.Flexed(float32(m.columns[5].weight), func(gtx layout.Context) layout.Dimensions {
		return ui.RightAlignLabel(gtx, ui.Label(style, m.ctx.Sprintf("%d", d.Crits)))
	}))
	cells = append(cells, layout.Flexed(float32(m.columns[6].weight), func(gtx layout.Context) layout.Dimensions {
		str := ""
		if hps >= 1 {
			str = m.ctx.Sprintf("%.0f", hps)
		}
		label := ui.Label(style, str)
		label.Color = style.Palette.LinkHover
		return ui.RightAlignLabel(gtx, label)
	}))
	cells = append(cells, layout.Flexed(float32(m.columns[7].weight), func(gtx layout.Context) layout.Dimensions {
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
