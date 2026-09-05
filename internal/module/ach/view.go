package ach

import (
	"fmt"
	"reflect"

	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget/material"
	achievments "github.com/uija/eqdps/internal/achievements"
	"github.com/uija/eqdps/internal/ui"
)

func (m *Module) Layout(style *ui.Style, gtx layout.Context) layout.Dimensions {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.data == nil {
		if m.loading.Load() {
			return m.RenderLoadingMessage(style, gtx)
		}
		return m.RenderNoDataMessage(style, gtx)
	}
	if m.filtered_data == nil {
		m.FilterData()
	}
	return m.RenderList(m.filtered_data, style, gtx)
}
func (m *Module) RenderList(data *achievments.Export, style *ui.Style, gtx layout.Context) layout.Dimensions {
	items := make([]any, 0)
	for mci, mc := range data.Categories {
		items = append(items, &data.Categories[mci])
		for sci, sc := range mc.Subcategories {
			items = append(items, &data.Categories[mci].Subcategories[sci])
			for ai, a := range sc.Achievements {
				items = append(items, &data.Categories[mci].Subcategories[sci].Achievements[ai])
				if a.Open {
					for i := range a.Objectives {
						items = append(items, &data.Categories[mci].Subcategories[sci].Achievements[ai].Objectives[i])
					}
				}
			}
		}
	}
	return layout.UniformInset(unit.Dp(8)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
						return layout.Inset{Bottom: unit.Dp(16)}.Layout(gtx, ui.HeaderLabel(style, "Achievements").Layout)
					}),
					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
						text := ""
						if m.data != nil && !m.data.Created.IsZero() {
							text = "Exported on " + m.data.Created.Format("2006-02-01 15:04:05")
						}
						return layout.Inset{Top: unit.Dp(1), Bottom: unit.Dp(16)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							return layout.E.Layout(gtx, ui.Label(style, text).Layout)
						})
					}),
				)
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return layout.Inset{Top: unit.Dp(8), Right: unit.Dp(16)}.Layout(gtx, ui.Label(style, "Filter:").Layout)
					}),
					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
						return layout.Inset{Bottom: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							return ui.MaxedTextField(&m.filter, "Filter achievements", style, gtx)
						})
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return layout.Inset{Top: unit.Dp(8), Left: unit.Dp(16)}.Layout(gtx, ui.IconLink(style, &m.filter_clear, ui.Close, "Clear").Layout)
					}),
				)
			}),
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				list := material.List(style.Theme, &m.list)
				return list.Layout(gtx, len(items), func(gtx layout.Context, index int) layout.Dimensions {
					switch d := items[index].(type) {
					case *achievments.Category:
						return m.RenderCategory(d, style, gtx)
					case *achievments.Subcategory:
						return m.RenderSubCategory(d, style, gtx)
					case *achievments.Achievement:
						return m.RenderAchievement(d, style, gtx)
					case *achievments.Objective:
						return m.RenderObjective(d, style, gtx)
					default:
						return ui.Label(style, fmt.Sprintf("Wrong type %v", reflect.TypeOf(items[index]))).Layout(gtx)
					}
				})
			}),
		)
	})
}
func (m *Module) RenderCategory(c *achievments.Category, style *ui.Style, gtx layout.Context) layout.Dimensions {
	return ui.ColoredRow(gtx, style.Palette.LightPanel, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{}.Layout(gtx,
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				return layout.UniformInset(unit.Dp(8)).Layout(gtx, ui.HeaderLabel(style, c.Name).Layout)
			}),
		)
	})
}
func (m *Module) RenderSubCategory(c *achievments.Subcategory, style *ui.Style, gtx layout.Context) layout.Dimensions {
	return ui.ColoredRow(gtx, style.Palette.Panel, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{}.Layout(gtx,
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				return layout.UniformInset(unit.Dp(8)).Layout(gtx, ui.Label(style, c.Name).Layout)
			}),
		)
	})
}
func (m *Module) RenderAchievement(c *achievments.Achievement, style *ui.Style, gtx layout.Context) layout.Dimensions {
	col := style.Palette.Text
	if c.Complete {
		col = style.Palette.Yes
	}
	return layout.UniformInset(unit.Dp(8)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		link := ui.IconLink(style, &c.Click, ui.AddBox, c.Name)
		link.TextColor = col
		return link.Layout(gtx)
	})
}
func (m *Module) RenderObjective(c *achievments.Objective, style *ui.Style, gtx layout.Context) layout.Dimensions {
	return layout.Inset{Top: unit.Dp(8), Bottom: unit.Dp(8), Left: unit.Dp(24)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				col := style.Palette.Text
				if c.Complete {
					col = style.Palette.Yes
				}
				return ui.ColorLabel(col, ui.Label(style, c.Text)).Layout(gtx)
			}),
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				return layout.E.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					txt := ""
					if c.Progress != nil {
						txt = fmt.Sprintf("%d / %d", c.Progress.Current, c.Progress.Required)
					}
					label := ui.Label(style, txt)
					return label.Layout(gtx)
				})
			}),
		)
	})
}
func (m *Module) RenderLoadingMessage(style *ui.Style, gtx layout.Context) layout.Dimensions {
	return layout.Stack{}.Layout(gtx,
		layout.Expanded(func(gtx layout.Context) layout.Dimensions {
			ui.Fill(gtx, style.Palette.Window)
			return layout.Dimensions{Size: gtx.Constraints.Max}
		}),
		layout.Expanded(func(gtx layout.Context) layout.Dimensions {
			return ui.Overlay(gtx, 420, style.Palette.Panel, style.Palette.Border, func(gtx layout.Context) layout.Dimensions {
				return layout.UniformInset(unit.Dp(16)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return ui.Label(style, "Loading data, please wait...").Layout(gtx)
				})
			})
		}),
	)
}
func (m *Module) RenderNoDataMessage(style *ui.Style, gtx layout.Context) layout.Dimensions {
	return layout.Stack{}.Layout(gtx,
		layout.Expanded(func(gtx layout.Context) layout.Dimensions {
			ui.Fill(gtx, style.Palette.Window)
			return layout.Dimensions{Size: gtx.Constraints.Max}
		}),
		layout.Expanded(func(gtx layout.Context) layout.Dimensions {
			return ui.Overlay(gtx, 420, style.Palette.Panel, style.Palette.Border, func(gtx layout.Context) layout.Dimensions {
				return layout.UniformInset(unit.Dp(16)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
						layout.Rigid(ui.Label(style, "There is no data available yet.").Layout),
						layout.Rigid(ui.Label(style, "Use the following command in game:").Layout),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
								layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
									return layout.Inset{Top: unit.Dp(8), Bottom: unit.Dp(18)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
										label := ui.ColorLabel(style.Palette.Accent, material.Label(style.Theme, ui.Sp(16), "/outputfile achievements"))
										return label.Layout(gtx)
									})
								}),
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									return layout.Inset{Top: unit.Dp(3)}.Layout(gtx, ui.IconLink(style, &m.macro_copy_click, ui.Copy, "").Layout)
								}),
							)
						}),
					)
				})
			})
		}),
	)
}
