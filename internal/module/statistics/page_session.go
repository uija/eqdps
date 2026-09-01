package statistics

import (
	"database/sql"
	"fmt"
	"image"
	"log"
	"sort"
	"strings"
	"sync/atomic"
	"time"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"github.com/uija/eqdps/internal/ui"
)

type SessionRow struct {
	Statistic SessionStatistics
	Clickable widget.Clickable
	Open      bool
	Details   *SessionDetails
}

type SessionsPage struct {
	db       *sql.DB
	tabClick widget.Clickable
	list     widget.List

	allSessions []SessionRow
	sessions    []*SessionRow
	filter      widget.Editor
	filterClear widget.Clickable
	loading     atomic.Bool

	zoneClick     widget.Clickable
	enteredClick  widget.Clickable
	durationClick widget.Clickable
	killsClick    widget.Clickable
	xpClick       widget.Clickable
	motesClick    widget.Clickable

	invalidateFunc func()
}

func NewSessionsPage(invalidate func()) *SessionsPage {
	p := &SessionsPage{invalidateFunc: invalidate}
	p.list.Axis = layout.Vertical
	p.filter.SingleLine = true
	return p
}

func (p *SessionsPage) Title() string                { return "Sessions" }
func (p *SessionsPage) GetIcon() *widget.Icon        { return ui.StatisticsSessions }
func (p *SessionsPage) Clickable() *widget.Clickable { return &p.tabClick }
func (p *SessionsPage) SetDb(db *sql.DB)             { p.db = db }

func (p *SessionsPage) Reset() {
	p.allSessions = nil
	p.sessions = nil
	p.filter.SetText("")
}

func (p *SessionsPage) applyFilter() {
	if p.allSessions == nil {
		return
	}
	if p.sessions == nil {
		p.sessions = make([]*SessionRow, 0, len(p.allSessions))
	} else {
		p.sessions = p.sessions[:0]
	}
	search := strings.ToLower(strings.TrimSpace(p.filter.Text()))
	for index := range p.allSessions {
		session := &p.allSessions[index]
		if search == "" || strings.Contains(strings.ToLower(session.Statistic.Zone), search) {
			p.sessions = append(p.sessions, session)
		}
	}
}

func (p *SessionsPage) Update(gtx layout.Context) {
	if p.sessions == nil {
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
	case p.zoneClick.Clicked(gtx):
		sort.Slice(p.sessions, func(i, j int) bool { return p.sessions[i].Statistic.Zone < p.sessions[j].Statistic.Zone })
	case p.enteredClick.Clicked(gtx):
		sort.Slice(p.sessions, func(i, j int) bool { return p.sessions[i].Statistic.EnteredAt.After(p.sessions[j].Statistic.EnteredAt) })
	case p.durationClick.Clicked(gtx):
		sort.Slice(p.sessions, func(i, j int) bool { return p.sessions[i].Statistic.Duration > p.sessions[j].Statistic.Duration })
	case p.killsClick.Clicked(gtx):
		sort.Slice(p.sessions, func(i, j int) bool { return p.sessions[i].Statistic.Kills > p.sessions[j].Statistic.Kills })
	case p.xpClick.Clicked(gtx):
		sort.Slice(p.sessions, func(i, j int) bool {
			return p.sessions[i].Statistic.ExperienceGained > p.sessions[j].Statistic.ExperienceGained
		})
	case p.motesClick.Clicked(gtx):
		sort.Slice(p.sessions, func(i, j int) bool { return p.sessions[i].Statistic.Motes > p.sessions[j].Statistic.Motes })
	}
	for _, session := range p.sessions {
		if !session.Clickable.Clicked(gtx) {
			continue
		}
		if session.Details == nil {
			details, err := GetSessionDetails(p.db, session.Statistic)
			if err != nil {
				log.Printf("Unable to load details for session %d. %v", session.Statistic.VisitID, err)
				continue
			}
			session.Details = &details
			session.Open = true
		} else {
			session.Open = !session.Open
		}
		p.invalidateFunc()
	}
}

func (p *SessionsPage) Layout(style *ui.Style, gtx layout.Context) layout.Dimensions {
	if p.sessions == nil && !p.loading.Load() {
		p.loading.Store(true)
		go func() {
			defer func() {
				p.loading.Store(false)
				p.invalidateFunc()
			}()
			statistics, err := GetSessionStatistics(p.db)
			if err != nil {
				log.Printf("Unable to load session statistics. %v", err)
				return
			}
			p.allSessions = make([]SessionRow, 0, len(statistics))
			for _, statistic := range statistics {
				p.allSessions = append(p.allSessions, SessionRow{Statistic: statistic})
			}
			p.applyFilter()
		}()
	}
	if p.sessions == nil {
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
					return ui.MaxedTextField(&p.filter, "Filter session zones", style, gtx)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Inset{Right: unit.Dp(8), Top: unit.Dp(8), Left: unit.Dp(8)}.Layout(gtx, ui.IconLink(style, &p.filterClear, ui.Close, "Clear").Layout)
				}),
			)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Bottom: unit.Dp(4)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return p.renderHeader(style, gtx)
			})
		}),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return list.Layout(gtx, len(p.sessions), func(gtx layout.Context, index int) layout.Dimensions {
				return p.renderRow(p.sessions[index], index%2 == 0, style, gtx)
			})
		}),
	)
}

func (p *SessionsPage) renderHeader(style *ui.Style, gtx layout.Context) layout.Dimensions {
	return ui.ColoredRow(gtx, style.Palette.Panel, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
			sessionHeaderCell(4, "Zone", &p.zoneClick, false, style),
			sessionHeaderCell(2, "Entered", &p.enteredClick, false, style),
			sessionHeaderCell(2, "Duration", &p.durationClick, true, style),
			sessionHeaderCell(1, "Kills", &p.killsClick, true, style),
			sessionHeaderCell(1, "XP", &p.xpClick, true, style),
			sessionHeaderCell(1, "Motes", &p.motesClick, true, style),
		)
	})
}

func (p *SessionsPage) renderRow(session *SessionRow, alternate bool, style *ui.Style, gtx layout.Context) layout.Dimensions {
	color := style.Palette.Window
	if alternate {
		color = style.Palette.Panel
	}
	return ui.ColoredRow(gtx, color, func(gtx layout.Context) layout.Dimensions {
		children := []layout.FlexChild{layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
				sessionLinkCell(4, session, style),
				sessionTextCell(2, session.Statistic.EnteredAt.Format("2006-01-02 15:04"), false, style),
				sessionTextCell(2, session.Statistic.Duration.Round(time.Second).String(), true, style),
				sessionTextCell(1, fmt.Sprintf("%d", session.Statistic.Kills), true, style),
				sessionTextCell(1, fmt.Sprintf("%.2f%%", session.Statistic.ExperienceGained), true, style),
				sessionTextCell(1, fmt.Sprintf("%d", session.Statistic.Motes), true, style),
			)
		})}
		if session.Details != nil && session.Open {
			children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return renderSessionDetails(session, style, gtx)
			}))
		}
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
	})
}

func sessionLinkCell(weight float32, session *SessionRow, style *ui.Style) layout.FlexChild {
	icon := ui.AddBox
	if session.Details != nil && session.Open {
		icon = ui.DelBox
	}
	return layout.Flexed(weight, func(gtx layout.Context) layout.Dimensions {
		return layout.UniformInset(unit.Dp(ROW_PADDING)).Layout(gtx, ui.IconLink(style, &session.Clickable, icon, session.Statistic.Zone).Layout)
	})
}

func renderSessionDetails(session *SessionRow, style *ui.Style, gtx layout.Context) layout.Dimensions {
	details := session.Details
	hours := session.Statistic.Duration.Hours()
	killsPerHour, xpPerHour, motesPerHour := 0.0, 0.0, 0.0
	if hours > 0 {
		killsPerHour = float64(session.Statistic.Kills) / hours
		xpPerHour = session.Statistic.ExperienceGained / hours
		motesPerHour = float64(session.Statistic.Motes) / hours
	}
	children := []layout.FlexChild{
		layout.Rigid(sessionDetailsTitle("Rates", style)),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
				sessionTextCell(1, fmt.Sprintf("Kills/h: %.1f", killsPerHour), false, style),
				sessionTextCell(1, fmt.Sprintf("XP/h: %.2f%%", xpPerHour), false, style),
				sessionTextCell(1, fmt.Sprintf("Motes/h: %.1f", motesPerHour), false, style),
				sessionTextCell(1, "Money: "+FormatMoney(details.Money), false, style),
			)
		}),
	}
	if len(details.Mobs) > 0 {
		children = append(children,
			layout.Rigid(sessionDetailsTitle("Mobs", style)),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
					sessionTextCell(4, "Mob", false, style),
					sessionTextCell(1, "Kills", true, style),
					sessionTextCell(1, "Killed by you", true, style),
				)
			}),
		)
		for index, mob := range details.Mobs {
			children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return statisticsDetailsRow(index, style, gtx, func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
						sessionTextCell(4, mob.Name, false, style),
						sessionTextCell(1, fmt.Sprintf("%d", mob.Kills), true, style),
						sessionTextCell(1, fmt.Sprintf("%d", mob.KilledByYou), true, style),
					)
				})
			}))
		}
	}
	if len(details.Loot) > 0 {
		children = append(children,
			layout.Rigid(sessionDetailsTitle("Loot", style)),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
					sessionTextCell(4, "Item", false, style),
					sessionTextCell(6, "Mobs", false, style),
					sessionTextCell(1, "Drops", true, style),
				)
			}),
		)
		for index, loot := range details.Loot {
			children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return statisticsDetailsRow(index, style, gtx, func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
						sessionTextCell(4, loot.Item, false, style),
						sessionTextCell(6, loot.Mob, false, style),
						sessionTextCell(1, fmt.Sprintf("%d", loot.Quantity), true, style),
					)
				})
			}))
		}
	}
	if len(details.Deaths) > 0 {
		children = append(children,
			layout.Rigid(sessionDetailsTitle("Deaths", style)),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
					sessionTextCell(4, "Killed by", false, style),
					sessionTextCell(1, "Deaths", true, style),
				)
			}),
		)
		for index, death := range details.Deaths {
			children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return statisticsDetailsRow(index, style, gtx, func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
						sessionTextCell(4, death.Mob, false, style),
						sessionTextCell(1, fmt.Sprintf("%d", death.Deaths), true, style),
					)
				})
			}))
		}
	}
	return statisticsDetailsLayout(style, gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
	})
}

func sessionDetailsTitle(title string, style *ui.Style) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		return layout.UniformInset(unit.Dp(8)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			label := material.Label(style.Theme, ui.Sp(17), title)
			label.Font.Weight = font.SemiBold
			return label.Layout(gtx)
		})
	}
}

func sessionHeaderCell(weight float32, title string, click *widget.Clickable, alignEnd bool, style *ui.Style) layout.FlexChild {
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

func sessionTextCell(weight float32, value string, alignEnd bool, style *ui.Style) layout.FlexChild {
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
