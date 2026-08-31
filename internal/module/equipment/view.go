package equipment

import (
	"strings"

	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget/material"
	"github.com/uija/eqdps/internal/inventory"
	"github.com/uija/eqdps/internal/ui"
)

func (m *Module) Layout(style *ui.Style, gtx layout.Context) layout.Dimensions {
	return layout.Stack{}.Layout(gtx,
		layout.Expanded(func(gtx layout.Context) layout.Dimensions {
			return layout.UniformInset(unit.Dp(16)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions { return m.RenderHeader(style, gtx) }),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions { return m.RenderFilter(style, gtx) }),
					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions { return m.RenderList(style, gtx) }),
				)
			})
		}),
		layout.Expanded(func(gtx layout.Context) layout.Dimensions {
			if m.selected_item != nil && m.selected_item.Metadata != nil && m.selected_item.Metadata["statsblock"] != "" {
				return ui.Overlay(gtx, 400, style.Palette.Panel, style.Palette.Border, func(gtx layout.Context) layout.Dimensions {
					return layout.UniformInset(unit.Dp(16)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return m.selected_item_click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							_, level := inventory.NormalizeItemName(m.selected_item.Name)
							str := m.selected_item.GetStatsBlock(level)
							str = strings.Replace(str, "<br>", "", -1)
							str = m.selected_item.Name + "\n\n" + str
							return material.Label(style.Theme, ui.Sp(15), str).Layout(gtx)
						})
					})
				})
			}
			return layout.Dimensions{}
		}),
	)
}
func (m *Module) RenderHeader(style *ui.Style, gtx layout.Context) layout.Dimensions {
	return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Top: unit.Dp(8), Bottom: unit.Dp(16)}.Layout(gtx, material.Label(style.Theme, ui.Sp(17), "Equipment").Layout)
		}),
	)
}
func (m *Module) RenderFilter(style *ui.Style, gtx layout.Context) layout.Dimensions {
	return layout.Inset{Bottom: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {

		return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return layout.Inset{Top: unit.Dp(8), Right: unit.Dp(16)}.Layout(gtx, material.Label(style.Theme, ui.Sp(15), "Filter:").Layout)
					}),
					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
						return ui.MaxedTextField(&m.filter, "Filter equipment", style, gtx)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return layout.Inset{Left: unit.Dp(16), Right: unit.Dp(16), Top: unit.Dp(8)}.Layout(gtx,
							ui.IconLink(style, &m.filter_clear, ui.Close, "Clear").Layout,
						)
					}),
				)
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
						return layout.Inset{Left: unit.Dp(4), Right: unit.Dp(4)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							return m.class1.Layout(style, gtx, unit.Dp(80))
						})
					}),
					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
						return layout.Inset{Left: unit.Dp(4), Right: unit.Dp(4)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							return m.class2.Layout(style, gtx, unit.Dp(80))
						})
					}),
					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
						return layout.Inset{Left: unit.Dp(4), Right: unit.Dp(4)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							return m.class3.Layout(style, gtx, unit.Dp(80))
						})
					}),
					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
						return layout.Inset{Left: unit.Dp(4), Right: unit.Dp(4)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							return m.slots.Layout(style, gtx, unit.Dp(180))
						})
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return material.CheckBox(style.Theme, &m.exaltation_checkbox, "Hide Exaltations").Layout(gtx)
					}),
				)
			}),
		)
	})
}
func (m *Module) RenderList(style *ui.Style, gtx layout.Context) layout.Dimensions {
	if m.items == nil {
		m.mu.Lock()
		if m.inv == nil {
			m.mu.Unlock()
			text := ""
			if m.importRunning.Load() {
				text = "Imported running. This may take a second, because we need to load the items from eqldb.org"
			} else {
				text = "No equipment loaded."
				if m.ctx.Config.EQLDbConfig.AccessToken == "" {
					text += "\n\nYou need to be connected to eqldb.org because we need to load item data."
				}
				text += "\n\nExport your inventory to load data into the parser."

			}
			return layout.Center.Layout(gtx, material.Label(style.Theme, ui.Sp(15), text).Layout)
		}
		m.PrepareItems()
		m.mu.Unlock()
	}
	list := material.List(style.Theme, &m.itemsList)
	return list.Layout(gtx, len(m.items), func(gtx layout.Context, index int) layout.Dimensions {
		return m.RenderRow(index, style, gtx)
	})
}
func (m *Module) RenderRow(index int, style *ui.Style, gtx layout.Context) layout.Dimensions {
	col := style.Palette.Window
	if index%2 == 0 {
		col = style.Palette.Panel
	}
	tcol := style.Palette.Text
	item := m.items[index]
	if len(item.Item.Metadata) == 0 {
		tcol = style.Palette.No
	}
	classes := ToList(item.Item.Classes)
	slots := ToList(item.Item.Slots)
	return ui.ColoredRow(gtx, col, func(gtx layout.Context) layout.Dimensions {
		return layout.UniformInset(unit.Dp(8)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					link := ui.Link(style, &m.items[index].Click, item.Item.Name)
					link.TextColor = tcol
					return link.Layout(gtx)
				}),
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					return layout.Inset{Left: unit.Dp(4), Right: unit.Dp(4)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return material.Label(style.Theme, ui.Sp(13), classes).Layout(gtx)
					})
				}),
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					return layout.Inset{Left: unit.Dp(4), Right: unit.Dp(4)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return material.Label(style.Theme, ui.Sp(13), slots).Layout(gtx)
					})
				}),
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					return material.Label(style.Theme, ui.Sp(13), item.Item.Location).Layout(gtx)
				}),
			)
		})
	})
}
func ToList(list []string) string {
	if list == nil || len(list) == 0 {
		return ""
	}
	str := ""
	for _, i := range list {
		if str != "" {
			str += " "
		}
		str += i
	}
	return str
}
