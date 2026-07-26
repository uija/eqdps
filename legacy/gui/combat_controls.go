package main

import (
	"fmt"
	"image/color"
	"strings"

	"gioui.org/font"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget/material"
	"github.com/uija/eqdps/legacy/internal/xp"
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

func (s *shell) layoutFightFilterBar(gtx layout.Context) layout.Dimensions {
	gtx.Constraints.Min.Y = gtx.Dp(unit.Dp(42))
	gtx.Constraints.Max.Y = gtx.Constraints.Min.Y
	fill(gtx, palette.panel)
	return centerContent(gtx, func(gtx layout.Context) layout.Dimensions {
		return inset(unit.Dp(14), 0).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return labelWeight(gtx, s.theme, "FILTER", unit.Sp(14), palette.muted, text.Start, font.SemiBold)
				}),
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					return inset(unit.Dp(14), 0).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						editor := material.Editor(s.theme, &s.filterEditor, "Type a mob name…")
						editor.TextSize = unit.Sp(16) * s.theme.TextSize / 16
						editor.Color = palette.text
						editor.HintColor = palette.muted
						dimensions := editor.Layout(gtx)
						s.previewFightFilter()
						return dimensions
					})
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return s.filterClear.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						pointer.CursorPointer.Add(gtx.Ops)
						foreground := palette.muted
						if s.fightFilter != "" {
							foreground = palette.accent
						}
						return labelWeight(gtx, s.theme, "Clear", unit.Sp(14), foreground, text.End, font.SemiBold)
					})
				}),
			)
		})
	})
}

func (s *shell) previewFightFilter() {
	filter := strings.TrimSpace(s.filterEditor.Text())
	if filter == s.fightFilter {
		return
	}
	s.fightFilter = filter
	s.applyFightFilter()
	s.fightList.ScrollTo(0)
	if s.window != nil {
		s.window.Invalidate()
	}
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
