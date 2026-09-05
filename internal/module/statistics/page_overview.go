package statistics

import (
	"database/sql"
	"fmt"
	"log"

	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"github.com/uija/eqdps/internal/ui"
)

type OverviewPage struct {
	db       *sql.DB
	tabClick widget.Clickable
	list     widget.List
	stats    *OverviewStatistics
}

func NewOverviewPage() *OverviewPage {
	o := OverviewPage{}
	o.list.Axis = layout.Vertical
	return &o
}

func (p *OverviewPage) Title() string {
	return "Overview"
}
func (p *OverviewPage) GetIcon() *widget.Icon {
	return ui.StatisticsOverview
}
func (p *OverviewPage) Clickable() *widget.Clickable {
	return &p.tabClick
}
func (p *OverviewPage) Layout(style *ui.Style, gtx layout.Context) layout.Dimensions {
	if p.stats == nil {
		stats, err := GetOverviewStatistics(p.db)
		if err != nil {
			log.Printf("Unable to load statistics. %v", err)
		}
		p.stats = &stats
	}
	if p.stats == nil {
		return layout.Dimensions{}
	}
	list := material.List(style.Theme, &p.list)

	return list.Layout(gtx, 8, func(gtx layout.Context, index int) layout.Dimensions {
		switch index {
		case 0:
			return RenderIntStatsRow("Zones visited", p.stats.ZonesVisited, index%2 == 0, style, gtx)
		case 1:
			return RenderIntStatsRow("Mobs killed", p.stats.MobsKilled, index%2 == 0, style, gtx)
		case 2:
			return RenderIntStatsRow("Items looted", p.stats.ItemsLooted, index%2 == 0, style, gtx)
		case 3:
			return RenderStatsRow("Money collected", FormatMoney(p.stats.MoneyCollected), index%2 == 0, style, gtx)
		case 4:
			return RenderFloatStatsRow("Experience gained", p.stats.ExperienceGained, index%2 == 0, style, gtx)
		case 5:
			return RenderFloatStatsRow("Levels gained", float64(p.stats.LevelsGained), index%2 == 0, style, gtx)
		case 6:
			return RenderIntStatsRow("Motes collected", p.stats.MotesCollected, index%2 == 0, style, gtx)
		case 7:
			return RenderIntStatsRow("Chat messages sent", p.stats.ChatMessagesSent, index%2 == 0, style, gtx)
		default:
			return ui.Label(style, "Index missing").Layout(gtx)
		}
	})
}
func (p *OverviewPage) SetDb(db *sql.DB) {
	p.db = db
}
func (p *OverviewPage) Update(layout.Context) {

}
func (p *OverviewPage) Reset() {
	p.stats = nil
}

func RenderIntStatsRow(name string, num int64, odd bool, style *ui.Style, gtx layout.Context) layout.Dimensions {
	return RenderStatsRow(name, fmt.Sprintf("%d", num), odd, style, gtx)
}
func RenderFloatStatsRow(name string, num float64, odd bool, style *ui.Style, gtx layout.Context) layout.Dimensions {
	return RenderStatsRow(name, fmt.Sprintf("%.02f", num), odd, style, gtx)
}
func RenderStatsRow(name string, num string, odd bool, style *ui.Style, gtx layout.Context) layout.Dimensions {
	color := style.Palette.Panel
	if odd {
		color = style.Palette.Window
	}
	return ui.ColoredRow(gtx, color, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
			layout.Flexed(3, func(gtx layout.Context) layout.Dimensions {
				return layout.UniformInset(unit.Dp(ROW_PADDING)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return ui.Label(style, name).Layout(gtx)
				})
			}),
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				return layout.E.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return layout.UniformInset(unit.Dp(ROW_PADDING)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return ui.Label(style, num).Layout(gtx)
					})
				})
			}),
		)
	})
}
