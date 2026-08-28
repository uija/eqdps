package statistics

import (
	"database/sql"
	"fmt"
	"image"
	"log"
	"sort"
	"strings"
	"sync/atomic"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"github.com/uija/eqdps/internal/ui"
)

type ItemRow struct {
	Statistic ItemStatistics
	Clickable widget.Clickable
	Open      bool
	Details   *ItemDetails
}

type ItemsPage struct {
	db             *sql.DB
	tabClick       widget.Clickable
	list           widget.List
	allItems       []ItemRow
	items          []*ItemRow
	filter         widget.Editor
	filterClear    widget.Clickable
	loading        atomic.Bool
	nameClick      widget.Clickable
	dropsClick     widget.Clickable
	autoSoldClick  widget.Clickable
	soldClick      widget.Clickable
	destroyedClick widget.Clickable
	parceledClick  widget.Clickable
	invalidateFn   func()
}

func NewItemsPage(invalidate func()) *ItemsPage {
	p := &ItemsPage{invalidateFn: invalidate}
	p.list.Axis = layout.Vertical
	p.filter.SingleLine = true
	return p
}

func (p *ItemsPage) Title() string                { return "Items" }
func (p *ItemsPage) Clickable() *widget.Clickable { return &p.tabClick }
func (p *ItemsPage) SetDb(db *sql.DB)             { p.db = db }
func (p *ItemsPage) Reset() {
	p.allItems = nil
	p.items = nil
	p.filter.SetText("")
}

func (p *ItemsPage) applyFilter() {
	if p.allItems == nil {
		return
	}
	if p.items == nil {
		p.items = make([]*ItemRow, 0)
	} else {
		p.items = p.items[:0]
	}
	search := strings.ToLower(p.filter.Text())
	for index := range p.allItems {
		item := &p.allItems[index]
		if search == "" || strings.Contains(strings.ToLower(item.Statistic.Name), search) {
			p.items = append(p.items, item)
		}
	}
}

func (p *ItemsPage) Update(gtx layout.Context) {
	if p.items == nil {
		return
	}
	for {
		event, ok := p.filter.Update(gtx)
		if !ok {
			break
		}
		if _, changed := event.(widget.ChangeEvent); changed {
			p.applyFilter()
		}
	}
	if p.filterClear.Clicked(gtx) {
		p.filter.SetText("")
		p.applyFilter()
	}
	switch {
	case p.nameClick.Clicked(gtx):
		sort.Slice(p.items, func(i, j int) bool {
			return p.items[i].Statistic.Name < p.items[j].Statistic.Name
		})
	case p.dropsClick.Clicked(gtx):
		sort.Slice(p.items, func(i, j int) bool {
			return p.items[i].Statistic.Drops > p.items[j].Statistic.Drops
		})
	case p.autoSoldClick.Clicked(gtx):
		sort.Slice(p.items, func(i, j int) bool {
			return p.items[i].Statistic.AutoSold > p.items[j].Statistic.AutoSold
		})
	case p.soldClick.Clicked(gtx):
		sort.Slice(p.items, func(i, j int) bool {
			return p.items[i].Statistic.Sold > p.items[j].Statistic.Sold
		})
	case p.destroyedClick.Clicked(gtx):
		sort.Slice(p.items, func(i, j int) bool {
			return p.items[i].Statistic.Destroyed > p.items[j].Statistic.Destroyed
		})
	case p.parceledClick.Clicked(gtx):
		sort.Slice(p.items, func(i, j int) bool {
			return p.items[i].Statistic.Parceled > p.items[j].Statistic.Parceled
		})
	}
	for _, item := range p.items {
		if !item.Clickable.Clicked(gtx) {
			continue
		}
		if item.Details == nil {
			details, err := GetItemDetails(p.db, item.Statistic.Name)
			if err != nil {
				log.Printf("Unable to load details for item %q. %v", item.Statistic.Name, err)
				continue
			}
			item.Details = &details
			item.Open = true
		} else {
			item.Open = !item.Open
		}
		p.invalidateFn()
	}
}

func (p *ItemsPage) Layout(style *ui.Style, gtx layout.Context) layout.Dimensions {
	if p.items == nil && !p.loading.Load() {
		p.loading.Store(true)
		go func() {
			defer func() {
				p.loading.Store(false)
				p.invalidateFn()
			}()
			stats, err := GetItemStatistics(p.db)
			if err != nil {
				log.Printf("Unable to load item statistics. %v", err)
				return
			}
			p.allItems = make([]ItemRow, 0, len(stats))
			for _, statistic := range stats {
				p.allItems = append(p.allItems, ItemRow{Statistic: statistic})
			}
			p.applyFilter()
		}()
	}
	if p.items == nil {
		message := "No data available"
		if p.loading.Load() {
			message = "Loading please wait..."
		}
		gtx.Constraints = layout.Exact(image.Pt(gtx.Constraints.Max.X, gtx.Constraints.Max.Y))
		return layout.Center.Layout(gtx, material.Label(style.Theme, ui.Sp(15), message).Layout)
	}

	list := material.List(style.Theme, &p.list)
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Inset{Top: unit.Dp(8), Right: unit.Dp(8)}.Layout(gtx, material.Label(style.Theme, ui.Sp(15), "Filter: ").Layout)
				}),
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					return ui.MaxedTextField(&p.filter, "", style, gtx)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Inset{Right: unit.Dp(8), Top: unit.Dp(8), Left: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return ui.IconLink(style, &p.filterClear, ui.Close, "Clear").Layout(gtx)
					})
				}),
			)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Bottom: unit.Dp(4)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return p.renderHeader(style, gtx)
			})
		}),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return list.Layout(gtx, len(p.items), func(gtx layout.Context, index int) layout.Dimensions {
				return p.renderRow(p.items[index], index%2 == 0, style, gtx)
			})
		}),
	)
}

func (p *ItemsPage) renderHeader(style *ui.Style, gtx layout.Context) layout.Dimensions {
	return ui.ColoredRow(gtx, style.Palette.Panel, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
			itemHeaderCell(5, "Item", &p.nameClick, false, style),
			itemHeaderCell(1, "Drops", &p.dropsClick, true, style),
			itemHeaderCell(1, "Autosold", &p.autoSoldClick, true, style),
			itemHeaderCell(1, "Sold", &p.soldClick, true, style),
			itemHeaderCell(1, "Destroyed", &p.destroyedClick, true, style),
			itemHeaderCell(1, "Parceled", &p.parceledClick, true, style),
		)
	})
}

func (p *ItemsPage) renderRow(item *ItemRow, alternate bool, style *ui.Style, gtx layout.Context) layout.Dimensions {
	color := style.Palette.Window
	if alternate {
		color = style.Palette.Panel
	}
	return ui.ColoredRow(gtx, color, func(gtx layout.Context) layout.Dimensions {
		children := []layout.FlexChild{
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
					itemLinkCell(5, item, style),
					itemTextCell(1, fmt.Sprintf("%d", item.Statistic.Drops), true, style),
					itemTextCell(1, fmt.Sprintf("%d", item.Statistic.AutoSold), true, style),
					itemTextCell(1, fmt.Sprintf("%d", item.Statistic.Sold), true, style),
					itemTextCell(1, fmt.Sprintf("%d", item.Statistic.Destroyed), true, style),
					itemTextCell(1, fmt.Sprintf("%d", item.Statistic.Parceled), true, style),
				)
			}),
		}
		if item.Details != nil && item.Open {
			children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return itemDetails(item.Details, style, gtx)
			}))
		}
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
	})
}

func itemDetails(details *ItemDetails, style *ui.Style, gtx layout.Context) layout.Dimensions {
	children := make([]layout.FlexChild, 0, len(details.Drops)+1)
	children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
		return layout.UniformInset(unit.Dp(8)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			label := material.Label(style.Theme, ui.Sp(17), "Drops")
			label.Font.Weight = font.SemiBold
			return label.Layout(gtx)
		})
	}))
	children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
			itemDetailTextCell(4, "Mob", false, style),
			itemDetailTextCell(3, "Zone", false, style),
			itemDetailTextCell(1, "Kills", true, style),
			itemDetailTextCell(1, "Drops", true, style),
			itemDetailTextCell(1, "Chance", true, style),
		)
	}))
	for _, drop := range details.Drops {
		drop := drop
		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
				itemDetailTextCell(4, drop.Mob, false, style),
				itemDetailTextCell(3, drop.Zone, false, style),
				itemDetailTextCell(1, fmt.Sprintf("%d", drop.Kills), true, style),
				itemDetailTextCell(1, fmt.Sprintf("%d", drop.Drops), true, style),
				itemDetailTextCell(1, fmt.Sprintf("%.02f%%", drop.DropChance), true, style),
			)
		}))
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
}

func itemHeaderCell(weight float32, title string, click *widget.Clickable, alignEnd bool, style *ui.Style) layout.FlexChild {
	return layout.Flexed(weight, func(gtx layout.Context) layout.Dimensions {
		content := func(gtx layout.Context) layout.Dimensions {
			return layout.UniformInset(unit.Dp(ROW_PADDING)).Layout(gtx, ui.IconLink(style, click, ui.Sort, title).Layout)
		}
		if alignEnd {
			return layout.E.Layout(gtx, content)
		}
		return content(gtx)
	})
}

func itemLinkCell(weight float32, item *ItemRow, style *ui.Style) layout.FlexChild {
	icon := ui.AddBox
	if item.Details != nil && item.Open {
		icon = ui.DelBox
	}
	return layout.Flexed(weight, func(gtx layout.Context) layout.Dimensions {
		return layout.UniformInset(unit.Dp(ROW_PADDING)).Layout(gtx, ui.IconLink(style, &item.Clickable, icon, item.Statistic.Name).Layout)
	})
}

func itemTextCell(weight float32, value string, alignEnd bool, style *ui.Style) layout.FlexChild {
	return itemDetailTextCell(weight, value, alignEnd, style)
}

func itemDetailTextCell(weight float32, value string, alignEnd bool, style *ui.Style) layout.FlexChild {
	return layout.Flexed(weight, func(gtx layout.Context) layout.Dimensions {
		content := func(gtx layout.Context) layout.Dimensions {
			return layout.UniformInset(unit.Dp(ROW_PADDING)).Layout(gtx, material.Label(style.Theme, ui.Sp(15), value).Layout)
		}
		if alignEnd {
			return layout.E.Layout(gtx, content)
		}
		return content(gtx)
	})
}
