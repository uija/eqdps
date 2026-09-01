package equipment

import (
	"fmt"
	"strings"

	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget/material"
	"github.com/uija/eqdps/internal/inventory"
	"github.com/uija/eqdps/internal/ui"
	"github.com/uija/eqdps/internal/ui/form"
)

func (m *Module) Layout(style *ui.Style, gtx layout.Context) layout.Dimensions {
	return layout.Stack{}.Layout(gtx,
		layout.Expanded(func(gtx layout.Context) layout.Dimensions {
			return layout.UniformInset(unit.Dp(16)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions { return m.RenderHeader(style, gtx) }),
					//layout.Rigid(func(gtx layout.Context) layout.Dimensions { return m.RenderSearchBar(style, gtx) }),
					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
						return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
							layout.Rigid(func(gtx layout.Context) layout.Dimensions { return m.RenderFilter(style, gtx) }),
							layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
								return layout.Inset{Left: unit.Dp(16)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
									return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
										layout.Rigid(func(gtx layout.Context) layout.Dimensions {
											return m.RenderSearchBar(style, gtx)
										}),
										layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
											return m.RenderList(style, gtx)
										}),
									)
								})
							}),
						)
					}),
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
func (m *Module) RenderSearchBar(style *ui.Style, gtx layout.Context) layout.Dimensions {
	return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Top: unit.Dp(8), Right: unit.Dp(16)}.Layout(gtx, material.Label(style.Theme, ui.Sp(15), "Search:").Layout)
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
}
func (m *Module) RenderHeader(style *ui.Style, gtx layout.Context) layout.Dimensions {
	return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Top: unit.Dp(8), Bottom: unit.Dp(16)}.Layout(gtx, material.Label(style.Theme, ui.Sp(17), "Equipment").Layout)
		}),
	)
}
func (m *Module) RenderLabeledSelect(name string, sel *form.SelectBox, style *ui.Style, gtx layout.Context) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Bottom: unit.Dp(4)}.Layout(gtx, material.Label(style.Theme, ui.Sp(15), name).Layout)
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Bottom: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return sel.Layout(style, gtx, unit.Dp(150))
				})
			}),
		)
	}
}
func (m *Module) RenderFilter(style *ui.Style, gtx layout.Context) layout.Dimensions {
	return layout.Inset{Bottom: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(m.RenderLabeledSelect("Class", m.class1, style, gtx)),
			layout.Rigid(m.RenderLabeledSelect("Class", m.class2, style, gtx)),
			layout.Rigid(m.RenderLabeledSelect("Class", m.class3, style, gtx)),
			layout.Rigid(m.RenderLabeledSelect("Slot", m.slots, style, gtx)),
			layout.Rigid(m.RenderLabeledSelect("Stats", m.stats, style, gtx)),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return material.CheckBox(style.Theme, &m.exaltation_checkbox, "Hide Exaltations").Layout(gtx)
			}),
			layout.Flexed(1, material.Body1(style.Theme, "").Layout),
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
	return list.Layout(gtx, len(m.items)+1, func(gtx layout.Context, index int) layout.Dimensions {
		if index == 0 {
			return m.RenderTableHeader(style, gtx)
		}
		return m.RenderRow(index-1, style, gtx)
	})
}
func (m *Module) RenderTableHeader(style *ui.Style, gtx layout.Context) layout.Dimensions {

	return layout.UniformInset(unit.Dp(8)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
			layout.Flexed(2, func(gtx layout.Context) layout.Dimensions {
				return ui.IconLink(style, &m.nameSort, ui.Sort, "Name").Layout(gtx)
			}),
			layout.Flexed(2, func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Left: unit.Dp(4), Right: unit.Dp(4)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return material.Label(style.Theme, ui.Sp(15), "Class").Layout(gtx)
				})
			}),
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Left: unit.Dp(4), Right: unit.Dp(4)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return material.Label(style.Theme, ui.Sp(15), "Slot").Layout(gtx)
				})
			}),
			layout.Flexed(2, func(gtx layout.Context) layout.Dimensions {
				return material.Label(style.Theme, ui.Sp(15), "Location").Layout(gtx)
			}),
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				value := "Stats"
				if selectedStat := m.stats.Value(); selectedStat != "" {
					value = selectedStat
				}
				return layout.E.Layout(gtx, ui.IconLink(style, &m.statSort, ui.Sort, value).Layout)
			}),
		)
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
				layout.Flexed(2, func(gtx layout.Context) layout.Dimensions {
					link := ui.Link(style, &m.items[index].Click, item.Item.Name)
					link.TextColor = tcol
					return link.Layout(gtx)
				}),
				layout.Flexed(2, func(gtx layout.Context) layout.Dimensions {
					return layout.Inset{Left: unit.Dp(4), Right: unit.Dp(4)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return material.Label(style.Theme, ui.Sp(13), classes).Layout(gtx)
					})
				}),
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					return layout.Inset{Left: unit.Dp(4), Right: unit.Dp(4)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return material.Label(style.Theme, ui.Sp(13), slots).Layout(gtx)
					})
				}),
				layout.Flexed(2, func(gtx layout.Context) layout.Dimensions {
					return material.Label(style.Theme, ui.Sp(13), item.Item.Location).Layout(gtx)
				}),
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					value := ""
					if selectedStat := m.stats.Value(); selectedStat != "" {
						if stat, found := item.Stats[selectedStat]; found {
							value = fmt.Sprintf("%d", stat)
						}
					}
					return layout.E.Layout(gtx, material.Label(style.Theme, ui.Sp(13), value).Layout)
				}),
			)
		})
	})
}
func ToList(list []string) string {
	if len(list) == 0 {
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
