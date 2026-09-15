package statistics

import (
	"database/sql"
	"log"
	"sort"
	"strings"

	"gioui.org/layout"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"github.com/uija/eqdps/internal/module"
	"github.com/uija/eqdps/internal/ui"
)

type factionLoadResult struct {
	stats []SessionFactionDetails
	err   error
}

type FactionsPage struct {
	ctx               *module.Context
	db                *sql.DB
	tabClick          widget.Clickable
	list              widget.List
	filter            widget.Editor
	filterClear       widget.Clickable
	headers           [4]widget.Clickable
	sortColumn        int
	allRows, rows     []SessionFactionDetails
	pending           chan factionLoadResult
	loaded, loadError bool
	invalidateFn      func()
}

func NewFactionsPage(ctx *module.Context, invalidate func()) *FactionsPage {
	if invalidate == nil {
		invalidate = func() {}
	}
	p := &FactionsPage{ctx: ctx, invalidateFn: invalidate}
	p.list.Axis = layout.Vertical
	p.filter.SingleLine = true
	return p
}

func (p *FactionsPage) Title() string                { return "Faction" }
func (p *FactionsPage) GetIcon() *widget.Icon        { return ui.StatisticsFactions }
func (p *FactionsPage) Clickable() *widget.Clickable { return &p.tabClick }
func (p *FactionsPage) SetDb(db *sql.DB)             { p.db = db; p.Reset() }
func (p *FactionsPage) Reset() {
	p.allRows, p.rows, p.pending = nil, nil, nil
	p.loaded, p.loadError = false, false
	p.filter.SetText("")
	p.list.Position = layout.Position{}
}

func (p *FactionsPage) receiveData() {
	if p.pending == nil {
		return
	}
	select {
	case result := <-p.pending:
		p.pending = nil
		p.loaded, p.loadError = true, result.err != nil
		if result.err != nil {
			log.Printf("Unable to load faction statistics. %v", result.err)
			return
		}
		p.allRows = result.stats
		p.applyFilter()
	default:
	}
}

func (p *FactionsPage) applyFilter() {
	search := strings.ToLower(strings.TrimSpace(p.filter.Text()))
	p.rows = make([]SessionFactionDetails, 0, len(p.allRows))
	for _, row := range p.allRows {
		if strings.Contains(strings.ToLower(row.Name), search) {
			p.rows = append(p.rows, row)
		}
	}
	p.sortRows()
	p.list.Position = layout.Position{}
}

func (p *FactionsPage) sortRows() {
	sort.SliceStable(p.rows, func(i, j int) bool {
		a, b := p.rows[i], p.rows[j]
		switch p.sortColumn {
		case 1:
			if a.Gained != b.Gained {
				return a.Gained > b.Gained
			}
		case 2:
			// Largest loss first; Lost is stored as a negative sum.
			if a.Lost != b.Lost {
				return a.Lost < b.Lost
			}
		case 3:
			if a.NetChange() != b.NetChange() {
				return a.NetChange() > b.NetChange()
			}
		}
		return strings.ToLower(a.Name) < strings.ToLower(b.Name)
	})
}

func (p *FactionsPage) Update(gtx layout.Context) {
	p.receiveData()
	for {
		event, ok := p.filter.Update(gtx)
		if !ok {
			break
		}
		if _, ok := event.(widget.ChangeEvent); ok {
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
}

func (p *FactionsPage) Layout(style *ui.Style, gtx layout.Context) layout.Dimensions {
	p.receiveData()
	if !p.loaded && p.pending == nil && p.db != nil {
		// Publish data to the UI thread; Reset discards stale results.
		result := make(chan factionLoadResult, 1)
		p.pending = result
		db, invalidate := p.db, p.invalidateFn
		go func() {
			stats, err := GetFactionStatistics(db)
			result <- factionLoadResult{stats, err}
			invalidate()
		}()
	}
	if !p.loaded || p.loadError {
		message := "No data available"
		if p.pending != nil {
			message = "Loading please wait..."
		}
		if p.loadError {
			message = "Unable to load faction statistics."
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
					return ui.MaxedTextField(&p.filter, "Filter faction name", style, gtx)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.UniformInset(8).Layout(gtx, ui.IconLink(style, &p.filterClear, ui.Close, "Clear").Layout)
				}),
			)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return ui.ColoredRow(gtx, style.Palette.HeadlineBGMuted, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{}.Layout(gtx,
					itemHeaderCell(4, "Faction", &p.headers[0], false, style),
					itemHeaderCell(1, "Gained", &p.headers[1], true, style),
					itemHeaderCell(1, "Lost", &p.headers[2], true, style),
					itemHeaderCell(1, "Net change", &p.headers[3], true, style))
			})
		}),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			if len(p.rows) == 0 {
				return layout.Center.Layout(gtx, ui.Label(style, "No matching factions.").Layout)
			}
			return material.List(style.Theme, &p.list).Layout(gtx, len(p.rows), func(gtx layout.Context, index int) layout.Dimensions {
				s := p.rows[index]
				return statisticsDetailsRow(index, style, gtx, func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{}.Layout(gtx,
						itemTextCell(4, s.Name, false, style),
						itemTextCell(1, p.formatChange(s.Gained), true, style),
						itemTextCell(1, p.formatChange(s.Lost), true, style),
						itemTextCell(1, p.formatChange(s.NetChange()), true, style))
				})
			})
		}),
	)
}

func (p *FactionsPage) formatChange(value int64) string {
	if value > 0 {
		return "+" + p.ctx.Sprintf("%d", value)
	}
	return p.ctx.Sprintf("%d", value)
}
