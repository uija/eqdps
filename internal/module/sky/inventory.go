package sky

import (
	"fmt"
	"image/color"
	"sort"
	"strings"

	"gioui.org/layout"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"github.com/uija/eqdps/internal/ui"
)

type Container struct {
	missing []InventoryRow
	others  []InventoryRow
}

func (m *Module) InventoryView(style *ui.Style, gtx layout.Context) layout.Dimensions {
	children := make([]layout.FlexChild, 0)
	children = append(children, m.RenderTopRow("Inventory", style, gtx,
		layout.Flexed(1,
			func(gtx layout.Context) layout.Dimensions {
				return layout.Dimensions{}
			},
		),
	))
	if !m.replay {
		container := Container{
			missing: make([]InventoryRow, 0),
			others:  make([]InventoryRow, 0),
		}
		for _, ir := range m.inventory {
			if ir.Need > ir.Have {
				container.missing = append(container.missing, ir)
			} else if ir.Have > 0 && ir.Need <= ir.Have && !strings.Contains(ir.Name, "Wind Rune") {
				container.others = append(container.others, ir)
			}
		}
		sort.Slice(container.missing, func(i, j int) bool {
			if m.inventory_sort_missing == 0 {
				if container.missing[i].Name == container.missing[j].Name {
					return container.missing[i].Hint < container.missing[j].Hint
				}
				return container.missing[i].Name < container.missing[j].Name
			} else {
				if container.missing[i].Hint == container.missing[j].Hint {
					return container.missing[i].Name < container.missing[j].Name
				}
				return container.missing[i].Hint < container.missing[j].Hint
			}
		})
		sort.Slice(container.others, func(i, j int) bool {
			if m.inventory_sort_exess == 0 {
				return container.others[i].Name < container.others[j].Name
			}
			return container.others[i].Hint < container.others[j].Hint
		})
		// collect all the itemsa
		children = append(children,
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				list := material.List(style.Theme, &m.inventorylist)
				return list.Layout(
					gtx,
					len(container.missing)+len(container.others)+2,
					func(gtx layout.Context, index int) layout.Dimensions {
						return layout.Inset{Top: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							return m.RenderListRow(&container, index, style, gtx)
						})
					},
				)
			}),
		)
	}
	return layout.UniformInset(unit.Dp(8)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
	})
}
func (m *Module) RenderListRow(container *Container, index int, style *ui.Style, gtx layout.Context) layout.Dimensions {
	if index == 0 {
		return m.RenderHeader("Missing Items", &m.inventory_sort_missing_item, &m.inventory_sort_missing_mob, style, gtx)
	} else if index <= len(container.missing) {
		idx := index - 1
		return RenderContainerRow(container.missing, idx, style.Palette.No, style, gtx, func(item *InventoryRow) string {
			return fmt.Sprintf("%d / %d", item.Have, item.Need)
		})
	} else if index == len(container.missing)+1 {
		return m.RenderHeader("Exess Items", &m.inventory_sort_exess_item, &m.inventory_sort_exess_mob, style, gtx)
	} else {
		idx := index - (len(container.missing) + 2)
		return RenderContainerRow(container.others, idx, style.Palette.Yes, style, gtx, func(item *InventoryRow) string {
			return fmt.Sprintf("%d / %d +%d", item.Have, item.Need, item.Have-item.Need)
		})
	}
}
func (m *Module) RenderHeader(caption string, item_click *widget.Clickable, mob_click *widget.Clickable, style *ui.Style, gtx layout.Context) layout.Dimensions {
	return layout.Inset{Top: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return ui.ColoredRow(gtx, style.Palette.Panel, func(gtx layout.Context) layout.Dimensions {
			return layout.UniformInset(unit.Dp(8)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
					layout.Flexed(2, func(gtx layout.Context) layout.Dimensions {
						link := ui.IconLink(style, item_click, ui.Sort, caption)
						link.TextColor = style.Palette.Accent
						return link.Layout(gtx)
					}),
					layout.Flexed(1, ui.ColorLabel(style.Palette.Accent, ui.Label(style, "Inventory")).Layout),
					layout.Flexed(2, func(gtx layout.Context) layout.Dimensions {
						link := ui.IconLink(style, mob_click, ui.Sort, "Drops from")
						link.TextColor = style.Palette.Accent
						return link.Layout(gtx)
					}),
				)
			})
		})
	})
}
func RenderContainerRow(container []InventoryRow, index int, col color.NRGBA, style *ui.Style, gtx layout.Context, f func(*InventoryRow) string) layout.Dimensions {
	item := container[index]
	bg := style.Palette.Panel
	if index%2 == 0 {
		bg = style.Palette.Window
	}
	return ui.ColoredRow(gtx, bg, func(gtx layout.Context) layout.Dimensions {
		return layout.Inset{Left: unit.Dp(16), Top: unit.Dp(4), Bottom: unit.Dp(4)}.Layout(gtx, func(layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
				layout.Flexed(2, func(gtx layout.Context) layout.Dimensions {
					return material.Label(style.Theme, ui.Sp(14), item.Name).Layout(gtx)
				}),
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					label := ui.ColorLabel(col, material.Label(style.Theme, ui.Sp(14), f(&item)))
					label.Alignment = text.End
					return layout.Inset{Right: unit.Dp(32)}.Layout(gtx, label.Layout)
				}),
				layout.Flexed(2, func(gtx layout.Context) layout.Dimensions {
					return material.Label(style.Theme, ui.Sp(14), item.Hint).Layout(gtx)
				}),
			)
		})
	})
}

func RenderOthersRow(container *Container, index int, style *ui.Style, gtx layout.Context) layout.Dimensions {
	item := &container.others[index]
	bg := style.Palette.Panel
	if index%2 == 0 {
		bg = style.Palette.Window
	}
	return ui.ColoredRow(gtx, bg, func(gtx layout.Context) layout.Dimensions {
		return layout.Inset{Left: unit.Dp(16), Top: unit.Dp(4), Bottom: unit.Dp(4)}.Layout(gtx, func(layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
				layout.Flexed(2, func(gtx layout.Context) layout.Dimensions {
					return material.Label(style.Theme, ui.Sp(14), item.Name).Layout(gtx)
				}),
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					return ui.ColorLabel(style.Palette.Yes, material.Label(style.Theme, ui.Sp(14), fmt.Sprintf("%d/%d +%d", item.Have, item.Need, item.Have-item.Need))).Layout(gtx)
				}),
				layout.Flexed(2, func(gtx layout.Context) layout.Dimensions {
					return material.Label(style.Theme, ui.Sp(14), item.Hint).Layout(gtx)
				}),
			)
		})
	})
}
