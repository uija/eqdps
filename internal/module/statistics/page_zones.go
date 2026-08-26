package statistics

import (
	"database/sql"
	"fmt"
	"image"
	"sort"
	"sync/atomic"

	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"github.com/uija/eqdps/internal/ui"
)

type ZonesPage struct {
	tabClick widget.Clickable
	list     widget.List
	db       *sql.DB
	loading  atomic.Bool

	zonestats []ZoneStatistics

	name_click       widget.Clickable
	visit_click      widget.Clickable
	time_click       widget.Clickable
	kills_click      widget.Clickable
	motes_click      widget.Clickable
	perhour_click    widget.Clickable
	dropchance_click widget.Clickable
}

func NewZonesPage() *ZonesPage {
	p := ZonesPage{}
	p.list.Axis = layout.Vertical
	return &p
}

func (p *ZonesPage) Title() string {
	return "Zones"
}
func (p *ZonesPage) Clickable() *widget.Clickable {
	return &p.tabClick
}
func (p *ZonesPage) Reset() {
	p.zonestats = nil
}
func (p *ZonesPage) Update(gtx layout.Context) {
	if p.zonestats == nil {
		return
	}
	if p.name_click.Clicked(gtx) {
		sort.Slice(p.zonestats, func(i, j int) bool {
			return p.zonestats[i].Name < p.zonestats[j].Name
		})
	}
	if p.visit_click.Clicked(gtx) {
		sort.Slice(p.zonestats, func(i, j int) bool {
			return p.zonestats[i].Visits > p.zonestats[j].Visits
		})
	}
	if p.time_click.Clicked(gtx) {
		sort.Slice(p.zonestats, func(i, j int) bool {
			return p.zonestats[i].TimeSpent > p.zonestats[j].TimeSpent
		})
	}

	if p.kills_click.Clicked(gtx) {
		sort.Slice(p.zonestats, func(i, j int) bool {
			return p.zonestats[i].MobsKilled > p.zonestats[j].MobsKilled
		})
	}
	if p.motes_click.Clicked(gtx) {
		sort.Slice(p.zonestats, func(i, j int) bool {
			return p.zonestats[i].MotesDropped > p.zonestats[j].MotesDropped
		})
	}
	if p.perhour_click.Clicked(gtx) {
		sort.Slice(p.zonestats, func(i, j int) bool {
			return p.zonestats[i].MotesPerHour > p.zonestats[j].MotesPerHour
		})
	}
	if p.dropchance_click.Clicked(gtx) {
		sort.Slice(p.zonestats, func(i, j int) bool {
			return p.zonestats[i].MoteDropChance > p.zonestats[j].MoteDropChance
		})
	}
}
func (p *ZonesPage) Layout(style *ui.Style, gtx layout.Context) layout.Dimensions {
	if p.zonestats == nil && !p.loading.Load() {
		p.loading.Store(true)
		go func() {
			defer p.loading.Store(false)
			stats, err := GetZoneStatistics(p.db)
			if err == nil {
				p.zonestats = stats
				p.name_click.Click()
			}
		}()
	}
	if p.zonestats == nil {
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
			return layout.Inset{Bottom: unit.Dp(4)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return p.RenderHeader(style, gtx)
			})
		}),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return list.Layout(gtx, len(p.zonestats), func(gtx layout.Context, index int) layout.Dimensions {
				return p.RenderZoneRow(p.zonestats[index], index%2 == 0, style, gtx)
			})
		}),
	)
}
func (p *ZonesPage) SetDb(db *sql.DB) {
	p.db = db
}
func (p *ZonesPage) RenderHeader(style *ui.Style, gtx layout.Context) layout.Dimensions {
	return ui.ColoredRow(gtx, style.Palette.Panel, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
			layout.Flexed(3, func(gtx layout.Context) layout.Dimensions {
				return layout.UniformInset(unit.Dp(ROW_PADDING)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return ui.IconLink(style, &p.name_click, ui.Sort, "Zone").Layout(gtx)
				})
			}),
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				return layout.E.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return layout.UniformInset(unit.Dp(ROW_PADDING)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return ui.IconLink(style, &p.visit_click, ui.Sort, "Visits").Layout(gtx)
					})
				})
			}),
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				return layout.E.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return layout.UniformInset(unit.Dp(ROW_PADDING)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return ui.IconLink(style, &p.time_click, ui.Sort, "Time").Layout(gtx)
					})
				})
			}),
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				return layout.E.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return layout.UniformInset(unit.Dp(ROW_PADDING)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return ui.IconLink(style, &p.kills_click, ui.Sort, "Kills").Layout(gtx)
					})
				})
			}),
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				return layout.E.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return layout.UniformInset(unit.Dp(ROW_PADDING)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return ui.IconLink(style, &p.motes_click, ui.Sort, "Motes").Layout(gtx)
					})
				})
			}),
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				return layout.E.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return layout.UniformInset(unit.Dp(ROW_PADDING)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return ui.IconLink(style, &p.perhour_click, ui.Sort, "Motes/h").Layout(gtx)
					})
				})
			}),
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				return layout.E.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return layout.UniformInset(unit.Dp(ROW_PADDING)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return ui.IconLink(style, &p.dropchance_click, ui.Sort, "%/kill").Layout(gtx)
					})
				})
			}),
		)
	})
}
func (p *ZonesPage) RenderZoneRow(zone ZoneStatistics, odd bool, style *ui.Style, gtx layout.Context) layout.Dimensions {
	color := style.Palette.Window
	if odd {
		color = style.Palette.Panel
	}
	return ui.ColoredRow(gtx, color, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
			layout.Flexed(3, func(gtx layout.Context) layout.Dimensions {
				return layout.UniformInset(unit.Dp(ROW_PADDING)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return material.Label(style.Theme, ui.Sp(15), zone.Name).Layout(gtx)
				})
			}),
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				return layout.E.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return layout.UniformInset(unit.Dp(ROW_PADDING)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return material.Label(style.Theme, ui.Sp(15), fmt.Sprintf("%d", zone.Visits)).Layout(gtx)
					})
				})
			}),
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				return layout.E.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return layout.UniformInset(unit.Dp(ROW_PADDING)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return material.Label(style.Theme, ui.Sp(15), fmt.Sprintf("%v", zone.TimeSpent)).Layout(gtx)
					})
				})
			}),
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				return layout.E.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return layout.UniformInset(unit.Dp(ROW_PADDING)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return material.Label(style.Theme, ui.Sp(15), fmt.Sprintf("%d", zone.MobsKilled)).Layout(gtx)
					})
				})
			}),
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				return layout.E.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return layout.UniformInset(unit.Dp(ROW_PADDING)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return material.Label(style.Theme, ui.Sp(15), fmt.Sprintf("%d", zone.MotesDropped)).Layout(gtx)
					})
				})
			}),
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				return layout.E.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return layout.UniformInset(unit.Dp(ROW_PADDING)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return material.Label(style.Theme, ui.Sp(15), fmt.Sprintf("%.01f", zone.MotesPerHour)).Layout(gtx)
					})
				})
			}),
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				return layout.E.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return layout.UniformInset(unit.Dp(ROW_PADDING)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return material.Label(style.Theme, ui.Sp(15), fmt.Sprintf("%.01f%%", zone.MoteDropChance)).Layout(gtx)
					})
				})
			}),
		)
	})
}
