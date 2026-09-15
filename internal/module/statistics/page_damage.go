package statistics

import (
	"database/sql"
	"log"
	"sort"
	"strings"

	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"github.com/uija/eqdps/internal/data"
	"github.com/uija/eqdps/internal/module"
	"github.com/uija/eqdps/internal/ui"
)

type DamageRow struct {
	Statistic DamageStatistics
	Clickable widget.Clickable
	Open      bool
}

type damageLoadResult struct {
	stats []DamageStatistics
	err   error
}

type DamagePage struct {
	ctx          *module.Context
	db           *sql.DB
	tabClick     widget.Clickable
	list         widget.List
	allRows      []DamageRow
	rows         []*DamageRow
	filter       widget.Editor
	filterClear  widget.Clickable
	headers      [7]widget.Clickable
	sortColumn   int
	loaded       bool
	loadError    bool
	pending      chan damageLoadResult
	invalidateFn func()
}

func NewDamagePage(ctx *module.Context, invalidate func()) *DamagePage {
	if invalidate == nil {
		invalidate = func() {}
	}
	p := &DamagePage{ctx: ctx, invalidateFn: invalidate}
	p.list.Axis = layout.Vertical
	p.filter.SingleLine = true
	return p
}

func (p *DamagePage) Title() string                { return "Damage" }
func (p *DamagePage) GetIcon() *widget.Icon        { return ui.StatisticsDamage }
func (p *DamagePage) Clickable() *widget.Clickable { return &p.tabClick }
func (p *DamagePage) SetDb(db *sql.DB)             { p.db = db; p.Reset() }
func (p *DamagePage) Reset() {
	p.allRows, p.rows, p.pending = nil, nil, nil
	p.loaded, p.loadError = false, false
	p.filter.SetText("")
	p.list.Position = layout.Position{}
}

func (p *DamagePage) receiveData() {
	if p.pending == nil {
		return
	}
	select {
	case result := <-p.pending:
		p.pending = nil
		p.loaded = true
		p.loadError = result.err != nil
		if result.err != nil {
			log.Printf("Unable to load damage statistics. %v", result.err)
			return
		}
		p.allRows = make([]DamageRow, len(result.stats))
		for i, s := range result.stats {
			p.allRows[i].Statistic = s
		}
		p.applyFilter()
	default:
	}
}

func (p *DamagePage) applyFilter() {
	p.rows = make([]*DamageRow, 0, len(p.allRows))
	search := strings.ToLower(strings.TrimSpace(p.filter.Text()))
	for i := range p.allRows {
		r := &p.allRows[i]
		if strings.Contains(strings.ToLower(r.Statistic.Name), search) || strings.Contains(strings.ToLower(r.Statistic.Category), search) {
			p.rows = append(p.rows, r)
		}
	}
	p.sortRows()
	p.list.Position = layout.Position{}
}

func (p *DamagePage) sortRows() {
	sort.SliceStable(p.rows, func(i, j int) bool {
		a, b := p.rows[i].Statistic, p.rows[j].Statistic
		switch p.sortColumn {
		case 1:
			if a.Category != b.Category {
				return a.Category < b.Category
			}
		case 2:
			if a.Hits() != b.Hits() {
				return a.Hits() > b.Hits()
			}
		case 3:
			if a.Normal.Average() != b.Normal.Average() {
				return a.Normal.Average() > b.Normal.Average()
			}
		case 4:
			if a.Normal.Max.Valid != b.Normal.Max.Valid {
				return a.Normal.Max.Valid
			}
			if a.Normal.Max.Int64 != b.Normal.Max.Int64 {
				return a.Normal.Max.Int64 > b.Normal.Max.Int64
			}
		case 5:
			if a.Critical.Max.Valid != b.Critical.Max.Valid {
				return a.Critical.Max.Valid
			}
			if a.Critical.Max.Int64 != b.Critical.Max.Int64 {
				return a.Critical.Max.Int64 > b.Critical.Max.Int64
			}
		case 6:
			if a.CritPercent() != b.CritPercent() {
				return a.CritPercent() > b.CritPercent()
			}
		}
		if strings.ToLower(a.Name) != strings.ToLower(b.Name) {
			return strings.ToLower(a.Name) < strings.ToLower(b.Name)
		}
		return a.Category < b.Category
	})
}

func (p *DamagePage) Update(gtx layout.Context) {
	p.receiveData()
	for {
		e, ok := p.filter.Update(gtx)
		if !ok {
			break
		}
		if _, ok := e.(widget.ChangeEvent); ok {
			p.applyFilter()
		}
	}
	if p.filterClear.Clicked(gtx) {
		p.filter.SetText("")
		p.applyFilter()
	}
	for i := range p.headers {
		if p.headers[i].Clicked(gtx) {
			p.sortColumn = i
			p.sortRows()
		}
	}
	for _, row := range p.rows {
		if row.Clickable.Clicked(gtx) {
			row.Open = !row.Open
			p.invalidateFn()
		}
	}
}

func (p *DamagePage) Layout(style *ui.Style, gtx layout.Context) layout.Dimensions {
	p.receiveData()
	if !p.loaded && p.pending == nil && p.db != nil {
		// The worker only publishes a result. Widget state stays on the UI
		// thread; Reset discards this channel if the character/data changes.
		result := make(chan damageLoadResult, 1)
		p.pending = result
		db, invalidate := p.db, p.invalidateFn
		go func() {
			stats, err := GetDamageStatistics(db)
			result <- damageLoadResult{stats, err}
			invalidate()
		}()
	}
	if !p.loaded || p.loadError {
		message := "No data available"
		if p.pending != nil {
			message = "Loading please wait..."
		}
		if p.loadError {
			message = "Unable to load damage statistics."
		}
		return layout.Center.Layout(gtx, ui.Label(style, message).Layout)
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Inset{Top: 8, Right: 8}.Layout(gtx, ui.Label(style, "Filter:").Layout)
				}),
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					return ui.MaxedTextField(&p.filter, "Filter attack name or category", style, gtx)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.UniformInset(8).Layout(gtx, ui.IconLink(style, &p.filterClear, ui.Close, "Clear").Layout)
				}),
			)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return ui.ColoredRow(gtx, style.Palette.HeadlineBGMuted, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{}.Layout(gtx,
					itemHeaderCell(3, "Name", &p.headers[0], false, style),
					itemHeaderCell(1, "Category", &p.headers[1], false, style),
					itemHeaderCell(1, "Hits", &p.headers[2], true, style),
					itemHeaderCell(1, "Average", &p.headers[3], true, style),
					itemHeaderCell(1, "Max", &p.headers[4], true, style),
					itemHeaderCell(1, "Max crit", &p.headers[5], true, style),
					itemHeaderCell(1, "Crit chance", &p.headers[6], true, style),
				)
			})
		}),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			if len(p.rows) == 0 {
				return layout.Center.Layout(gtx, ui.Label(style, "No matching attacks. Rebuild statistics to include previously imported combat.").Layout)
			}
			return material.List(style.Theme, &p.list).Layout(gtx, len(p.rows), func(gtx layout.Context, i int) layout.Dimensions { return p.renderRow(p.rows[i], i, style, gtx) })
		}),
	)
}

func (p *DamagePage) damageBound(value sql.NullInt64) string {
	if !value.Valid {
		return "—"
	}
	return p.ctx.Sprintf("%d", value.Int64)
}

func (p *DamagePage) average(s DamageRangeStatistics) string {
	if s.Count == 0 {
		return "—"
	}
	return p.ctx.Sprintf("%.1f", s.Average())
}

func (p *DamagePage) renderRow(row *DamageRow, index int, style *ui.Style, gtx layout.Context) layout.Dimensions {
	s := row.Statistic
	return statisticsDetailsRow(index, style, gtx, func(gtx layout.Context) layout.Dimensions {
		children := []layout.FlexChild{layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			crit := "—"
			if s.Hits() > 0 {
				crit = p.ctx.Sprintf("%.1f%%", s.CritPercent())
			}
			return layout.Flex{}.Layout(gtx,
				layout.Flexed(3, func(gtx layout.Context) layout.Dimensions {
					icon := ui.AddBox
					if row.Open {
						icon = ui.DelBox
					}
					return layout.UniformInset(unit.Dp(ROW_PADDING)).Layout(gtx, ui.IconLink(style, &row.Clickable, icon, s.Name).Layout)
				}),
				itemTextCell(1, s.Category, false, style),
				itemTextCell(1, p.ctx.Sprintf("%d", s.Hits()), true, style),
				itemTextCell(1, p.average(s.Normal), true, style),
				itemTextCell(1, p.damageBound(s.Normal.Max), true, style),
				itemTextCell(1, p.damageBound(s.Critical.Max), true, style),
				itemTextCell(1, crit, true, style),
			)
		})}
		if row.Open {
			children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions { return p.renderDetails(s, style, gtx) }))
		}
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
	})
}

func (p *DamagePage) renderDetails(s DamageStatistics, style *ui.Style, gtx layout.Context) layout.Dimensions {
	return statisticsDetailsLayout(style, gtx, func(gtx layout.Context) layout.Dimensions {
		children := []layout.FlexChild{
			layout.Rigid(titleRow("Damage", style)),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return ui.ColoredRow(gtx, style.Palette.HeadlineBGMuted, func(gtx layout.Context) layout.Dimensions {

					return layout.Flex{}.Layout(gtx,
						itemTextCell(2, "Action", false, style), itemTextCell(1, "Count", true, style),
						itemTextCell(1, "Min", true, style), itemTextCell(1, "Max", true, style), itemTextCell(1, "Average", true, style),
					)
				})
			}),
		}
		for i, r := range []DamageRangeStatistics{s.Normal, s.Critical} {
			children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				name := "Normal"
				if i == 1 {
					name = "Critical"
				}
				min, max := "—", "—"
				if r.Min.Valid {
					min = p.ctx.Sprintf("%d", r.Min.Int64)
				}
				if r.Max.Valid {
					max = p.ctx.Sprintf("%d", r.Max.Int64)
				}
				return statisticsDetailsRow(i, style, gtx, func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{}.Layout(gtx, itemTextCell(2, name, false, style),
						itemTextCell(1, p.ctx.Sprintf("%d", r.Count), true, style), itemTextCell(1, min, true, style),
						itemTextCell(1, max, true, style), itemTextCell(1, p.average(r), true, style))
				})
			}))
		}
		if s.Category == data.CATEGORY_MELEE {
			children = append(children, layout.Rigid(titleRow("Failures", style)))
			for i, f := range []struct {
				name  string
				count int64
			}{
				{"Miss", s.Miss}, {"Dodge", s.Dodge}, {"Parry", s.Parry}, {"Block", s.Block}, {"Riposte", s.Riposte}, {"Absorb", s.Absorb},
			} {
				children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return statisticsDetailsRow(i, style, gtx, func(gtx layout.Context) layout.Dimensions {
						return layout.Flex{}.Layout(gtx,
							itemTextCell(5, f.name, false, style), itemTextCell(1, p.ctx.Sprintf("%d", f.count), true, style))
					})
				}))
			}
		}
		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Top: 8}.Layout(gtx, ui.ColorLabel(style.Palette.Muted, ui.Label(style, "Hits include critical hits. DoTs count ticks, not casts. Crit % excludes failures.")).Layout)
		}))
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
	})
}
