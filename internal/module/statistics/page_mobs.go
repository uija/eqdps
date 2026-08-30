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

type MobRow struct {
	Statistic MobStatistics
	Clickable widget.Clickable
	Open      bool
	Details   *MobDetails
}

type MobsPage struct {
	db           *sql.DB
	tabClick     widget.Clickable
	list         widget.List
	all_mobs     []MobRow
	mobs         []*MobRow
	filter       widget.Editor
	filter_clear widget.Clickable

	loading atomic.Bool

	nameClick           widget.Clickable
	killedByPlayerClick widget.Clickable
	killedPlayerClick   widget.Clickable
	moneyClick          widget.Clickable
	itemsClick          widget.Clickable
	differentItemsClick widget.Clickable

	invalidateFunc func()
}

func NewMobsPage(iv func()) *MobsPage {
	p := &MobsPage{
		invalidateFunc: iv,
	}
	p.list.Axis = layout.Vertical
	p.filter.SingleLine = true
	return p
}

func (p *MobsPage) Title() string                { return "Mobs" }
func (p *MobsPage) Clickable() *widget.Clickable { return &p.tabClick }
func (p *MobsPage) SetDb(db *sql.DB)             { p.db = db }
func (p *MobsPage) Reset() {
	p.all_mobs = nil
	p.mobs = nil
	p.filter.SetText("")
}
func (p *MobsPage) ApplyFilter() {
	if p.all_mobs == nil {
		return
	}
	if p.mobs == nil {
		p.mobs = make([]*MobRow, 0)
	} else {
		p.mobs = p.mobs[:0]
	}
	lsearch := strings.ToLower(p.filter.Text())
	for idx, m := range p.all_mobs {
		if lsearch == "" || strings.Contains(strings.ToLower(m.Statistic.Name), lsearch) {
			p.mobs = append(p.mobs, &p.all_mobs[idx])
		}
	}
}
func (p *MobsPage) Update(gtx layout.Context) {
	if p.mobs == nil {
		return
	}
	for {
		event, ok := p.filter.Update(gtx)
		if !ok {
			break
		}
		if _, ok := event.(widget.ChangeEvent); ok {
			p.ApplyFilter()
		}
	}
	if p.filter_clear.Clicked(gtx) {
		p.filter.SetText("")
		p.ApplyFilter()
	}
	switch {
	case p.nameClick.Clicked(gtx):
		sort.Slice(p.mobs, func(i, j int) bool {
			return p.mobs[i].Statistic.Name < p.mobs[j].Statistic.Name
		})
	case p.killedByPlayerClick.Clicked(gtx):
		sort.Slice(p.mobs, func(i, j int) bool {
			return p.mobs[i].Statistic.KilledByPlayer > p.mobs[j].Statistic.KilledByPlayer
		})
	case p.killedPlayerClick.Clicked(gtx):
		sort.Slice(p.mobs, func(i, j int) bool {
			return p.mobs[i].Statistic.KilledPlayer > p.mobs[j].Statistic.KilledPlayer
		})
	case p.moneyClick.Clicked(gtx):
		sort.Slice(p.mobs, func(i, j int) bool {
			return p.mobs[i].Statistic.MoneyLooted > p.mobs[j].Statistic.MoneyLooted
		})
	case p.itemsClick.Clicked(gtx):
		sort.Slice(p.mobs, func(i, j int) bool {
			return p.mobs[i].Statistic.ItemsLooted > p.mobs[j].Statistic.ItemsLooted
		})
	case p.differentItemsClick.Clicked(gtx):
		sort.Slice(p.mobs, func(i, j int) bool {
			return p.mobs[i].Statistic.DifferentItems > p.mobs[j].Statistic.DifferentItems
		})
	}
	for idx := range p.mobs {
		if p.mobs[idx].Clickable.Clicked(gtx) {
			if p.mobs[idx].Details == nil {
				details, err := GetMobDetails(p.db, p.mobs[idx].Statistic.Name)
				if err == nil {
					sort.Slice(details.Items, func(i, j int) bool {
						return details.Items[i].Quantity > details.Items[j].Quantity
					})
					p.mobs[idx].Details = &details
					p.mobs[idx].Open = true
				}
			} else {
				p.mobs[idx].Open = !p.mobs[idx].Open
			}
			p.invalidateFunc()
		}
	}
}

func (p *MobsPage) Layout(style *ui.Style, gtx layout.Context) layout.Dimensions {
	if p.mobs == nil && !p.loading.Load() {
		p.loading.Store(true)
		go func() {
			defer func() {
				p.loading.Store(false)
				p.invalidateFunc()
			}()
			stats, err := GetMobStatistics(p.db)
			if err != nil {
				log.Printf("Unable to load mob statistics. %v", err)
				return
			}
			p.all_mobs = make([]MobRow, 0)
			for _, s := range stats {
				p.all_mobs = append(p.all_mobs, MobRow{
					Statistic: s,
				})
			}
			p.ApplyFilter()
		}()
	}
	if p.mobs == nil {
		str := "No data available"
		if p.loading.Load() {
			str = "Loading please wait..."
		}
		width := gtx.Constraints.Max.X
		height := gtx.Constraints.Max.Y
		gtx.Constraints = layout.Exact(image.Pt(width, height))
		return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return material.Label(style.Theme, unit.Sp(15), str).Layout(gtx)
		})
	}
	list := material.List(style.Theme, &p.list)
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Inset{Top: unit.Dp(8), Right: unit.Dp(8)}.Layout(gtx, material.Label(style.Theme, ui.Sp(15), "Filter: ").Layout)
				}),
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					return ui.MaxedTextField(&p.filter, "Filter mob list", style, gtx)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Inset{Right: unit.Dp(8), Top: unit.Dp(8), Left: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return ui.IconLink(style, &p.filter_clear, ui.Close, "Clear").Layout(gtx)
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
			return list.Layout(gtx, len(p.mobs), func(gtx layout.Context, index int) layout.Dimensions {
				return p.renderRow(p.mobs[index], index%2 == 0, style, gtx)
			})
		}),
	)
}

func (p *MobsPage) renderHeader(style *ui.Style, gtx layout.Context) layout.Dimensions {
	return ui.ColoredRow(gtx, style.Palette.Panel, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
			mobHeaderCell(4, "Mob", &p.nameClick, false, style),
			mobHeaderCell(1, "Killed", &p.killedByPlayerClick, true, style),
			mobHeaderCell(1, "Killed me", &p.killedPlayerClick, true, style),
			mobHeaderCell(2, "Money", &p.moneyClick, true, style),
			mobHeaderCell(1, "Items", &p.itemsClick, true, style),
			mobHeaderCell(1, "Different", &p.differentItemsClick, true, style),
		)
	})
}

func (p *MobsPage) renderRow(mob *MobRow, alternate bool, style *ui.Style, gtx layout.Context) layout.Dimensions {
	color := style.Palette.Window
	if alternate {
		color = style.Palette.Panel
	}
	return ui.ColoredRow(gtx, color, func(gtx layout.Context) layout.Dimensions {
		children := make([]layout.FlexChild, 0)
		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
				mobLinkCell(4, mob, style),
				mobTextCell(1, fmt.Sprintf("%d", mob.Statistic.KilledByPlayer), true, style),
				mobTextCell(1, fmt.Sprintf("%d", mob.Statistic.KilledPlayer), true, style),
				mobTextCell(2, FormatMoney(mob.Statistic.MoneyLooted), true, style),
				mobTextCell(1, fmt.Sprintf("%d", mob.Statistic.ItemsLooted), true, style),
				mobTextCell(1, fmt.Sprintf("%d", mob.Statistic.DifferentItems), true, style),
			)
		}))
		if mob.Details != nil && mob.Open {
			children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return mobDetails(mob.Details, style, gtx)
			}))
		}
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
	})
}

func mobDetails(details *MobDetails, style *ui.Style, gtx layout.Context) layout.Dimensions {
	children := make([]layout.FlexChild, 0)
	if len(details.Zones) > 0 {
		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.UniformInset(unit.Dp(8)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				label := material.Label(style.Theme, ui.Sp(17), "Zones")
				label.Font.Weight = font.SemiBold
				return label.Layout(gtx)
			})
		}))
		for _, z := range details.Zones {
			children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
						return layout.E.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							return layout.UniformInset(unit.Dp(ROW_PADDING)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
								return material.Label(style.Theme, ui.Sp(15), fmt.Sprintf("%d", z.Kills)).Layout(gtx)
							})
						})
					}),
					layout.Flexed(4, func(gtx layout.Context) layout.Dimensions {
						return layout.UniformInset(unit.Dp(ROW_PADDING)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							return material.Label(style.Theme, ui.Sp(15), z.Name).Layout(gtx)
						})
					}),
				)
			}))
		}
	}
	if len(details.Items) > 0 {
		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.UniformInset(unit.Dp(8)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				label := material.Label(style.Theme, ui.Sp(17), "Items")
				label.Font.Weight = font.SemiBold
				return label.Layout(gtx)
			})
		}))
		for _, i := range details.Items {
			children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
						return layout.E.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							return layout.UniformInset(unit.Dp(ROW_PADDING)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
								return material.Label(style.Theme, ui.Sp(15), fmt.Sprintf("%d", i.Quantity)).Layout(gtx)
							})
						})
					}),
					layout.Flexed(4, func(gtx layout.Context) layout.Dimensions {
						return layout.UniformInset(unit.Dp(ROW_PADDING)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							return material.Label(style.Theme, ui.Sp(15), fmt.Sprintf("%s (%.02f%%)", i.Name, i.DropChance)).Layout(gtx)
						})
					}),
				)
			}))
		}
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
}

func mobHeaderCell(weight float32, title string, click *widget.Clickable, alignEnd bool, style *ui.Style) layout.FlexChild {
	return layout.Flexed(weight, func(gtx layout.Context) layout.Dimensions {
		content := func(gtx layout.Context) layout.Dimensions {
			return layout.UniformInset(unit.Dp(ROW_PADDING)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return ui.IconLink(style, click, ui.Sort, title).Layout(gtx)
			})
		}
		if alignEnd {
			return layout.E.Layout(gtx, content)
		}
		return content(gtx)
	})
}

func mobLinkCell(weight float32, mob *MobRow, style *ui.Style) layout.FlexChild {
	icon := ui.AddBox
	if mob.Details != nil && mob.Open {
		icon = ui.DelBox
	}
	return layout.Flexed(weight, func(gtx layout.Context) layout.Dimensions {
		return layout.UniformInset(unit.Dp(ROW_PADDING)).Layout(gtx, ui.IconLink(style, &mob.Clickable, icon, mob.Statistic.Name).Layout)
	})
}
func mobTextCell(weight float32, value string, alignEnd bool, style *ui.Style) layout.FlexChild {
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
