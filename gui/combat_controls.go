package main

import (
	"fmt"
	"image"
	"image/color"
	"strings"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"github.com/uija/eqdps/internal/xp"
)

func (s *shell) applyFightFilter() {
	query := strings.ToLower(strings.TrimSpace(s.fightFilter))
	if query == "" {
		s.fights = s.allFights
		return
	}
	s.fights = make([]fakeFightSection, 0, len(s.allFights))
	for _, fight := range s.allFights {
		if strings.Contains(strings.ToLower(fight.name), query) {
			s.fights = append(s.fights, fight)
		}
	}
}

func (s *shell) showCurrentFight() {
	s.fightFilter = ""
	s.applyFightFilter()
	newest := -1
	for index := range s.fights {
		if s.fights[index].current && (newest < 0 || s.fights[index].started.After(s.fights[newest].started)) {
			newest = index
		}
	}
	if newest < 0 {
		newest = 0
	}
	s.fightList.ScrollTo(newest)
}

func (s *shell) updateFilterDialog(gtx layout.Context) {
	switch {
	case s.filterApply.Clicked(gtx):
		s.fightFilter = strings.TrimSpace(s.filterEditor.Text())
		s.applyFightFilter()
		s.fightList.ScrollTo(0)
		s.filterOpen = false
	case s.filterClear.Clicked(gtx):
		s.fightFilter = ""
		s.filterEditor.SetText("")
		s.applyFightFilter()
		s.fightList.ScrollTo(0)
		s.filterOpen = false
	case s.filterCancel.Clicked(gtx):
		s.filterOpen = false
	}
}

func (s *shell) layoutFilterDialog(gtx layout.Context) layout.Dimensions {
	if !s.filterOpen {
		return layout.Dimensions{}
	}
	return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Min = image.Pt(gtx.Dp(unit.Dp(520)), gtx.Dp(unit.Dp(210)))
		gtx.Constraints.Max = gtx.Constraints.Min
		return outline(gtx, palette.line, func(gtx layout.Context) layout.Dimensions {
			fill(gtx, palette.panel)
			return layout.UniformInset(unit.Dp(22)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return labelWeight(gtx, s.theme, "Filter fights", unit.Sp(21), palette.text, text.Start, font.SemiBold)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return inset(0, unit.Dp(16)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							editor := material.Editor(s.theme, &s.filterEditor, "Mob name contains…")
							editor.TextSize = unit.Sp(17) * s.theme.TextSize / 16
							editor.Color = palette.text
							editor.HintColor = palette.muted
							return outline(gtx, palette.line, func(gtx layout.Context) layout.Dimensions {
								return layout.UniformInset(unit.Dp(9)).Layout(gtx, editor.Layout)
							})
						})
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return layout.E.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									return dialogButton(gtx, s.theme, &s.filterClear, "Clear", false)
								}),
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									return dialogButton(gtx, s.theme, &s.filterCancel, "Cancel", false)
								}),
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									return dialogButton(gtx, s.theme, &s.filterApply, "Apply", true)
								}),
							)
						})
					}),
				)
			})
		})
	})
}

func dialogButton(gtx layout.Context, theme *material.Theme, click *widget.Clickable, value string, accent bool) layout.Dimensions {
	return inset(unit.Dp(4), 0).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			fill(gtx, palette.panelAlt)
			color := palette.text
			if accent {
				color = palette.accent
			}
			return layout.UniformInset(unit.Dp(10)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return labelWeight(gtx, theme, value, unit.Sp(15), color, text.Middle, font.SemiBold)
			})
		})
	})
}

func xpStatusText(snapshot xp.Snapshot, filter string) string {
	filterText := ""
	if filter != "" {
		filterText = " · filter: " + filter
	}
	if snapshot.Gains == 0 {
		return "XP: waiting for data" + filterText
	}
	prefix := "~"
	if snapshot.ProgressKnown {
		prefix = ""
	}
	return fmt.Sprintf("XP %s%.1f%% · %.1f%%/h%s", prefix, snapshot.LevelPercent, snapshot.PercentPerHour, filterText)
}

func parserStatus(state string, hasLog bool) (string, color.NRGBA) {
	switch state {
	case "loading":
		return "●  LOADING", palette.accent
	case "live":
		return "●  LIVE", palette.success
	case "error":
		return "●  ERROR", color.NRGBA{R: 220, G: 135, B: 135, A: 255}
	default:
		if hasLog {
			return "●  OPEN", palette.muted
		}
		return "●  NO LOG", palette.muted
	}
}
