package main

import (
	"image"
	"image/color"
	"os"
	"time"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget"
	"github.com/uija/eqdps/internal/engine"
)

type xpStartResult struct {
	id        uint64
	landmarks engine.ReplayLandmarks
	err       error
}

func (s *shell) updateXPStart(gtx layout.Context) {
	if s.xpStatusClick.Clicked(gtx) && !s.loading {
		if s.currentLog == "" {
			s.statusText = "Open a logfile before choosing an XP starting point"
		} else if !s.xpStartOpen {
			s.openXPStartOverlay()
		}
	}

	select {
	case result := <-s.xpStartResults:
		if s.xpStartOpen && result.id == s.xpStartScanID {
			s.xpStartScanning = false
			if result.err != nil {
				s.xpStartError = result.err.Error()
			} else {
				s.xpLastLevelUp = result.landmarks.LastLevelUp
				s.xpLastZoneChange = result.landmarks.LastZoneChange
				s.xpZoneLevelXP = result.landmarks.LastZoneLevelPercent
				s.xpZoneLevelKnown = result.landmarks.LastZoneProgressKnown
			}
		}
	default:
	}

	if s.xpStartCancel.Clicked(gtx) {
		s.xpStartOpen = false
	}
	if s.xpSinceLevel.Clicked(gtx) && !s.xpLastLevelUp.IsZero() {
		s.xpStartOpen = false
		s.loadLogSince(s.xpStartPath, s.xpLastLevelUp, "since last level up", 0, false)
	}
	if s.xpSinceZone.Clicked(gtx) && !s.xpLastZoneChange.IsZero() {
		s.xpStartOpen = false
		s.loadLogSince(s.xpStartPath, s.xpLastZoneChange, "since last zoning", s.xpZoneLevelXP, s.xpZoneLevelKnown)
	}
}

func (s *shell) openXPStartOverlay() {
	s.xpStartOpen = true
	s.xpStartScanning = true
	s.xpStartPath = s.currentLog
	s.xpStartError = ""
	s.xpLastLevelUp = time.Time{}
	s.xpLastZoneChange = time.Time{}
	s.xpZoneLevelXP = 0
	s.xpZoneLevelKnown = false
	s.xpStartScanID++
	id, path := s.xpStartScanID, s.xpStartPath
	go func() {
		info, err := os.Stat(path)
		var landmarks engine.ReplayLandmarks
		if err == nil {
			landmarks, err = engine.FindReplayLandmarks(path, info.Size())
		}
		s.xpStartResults <- xpStartResult{id: id, landmarks: landmarks, err: err}
		if s.window != nil {
			s.window.Invalidate()
		}
	}()
}

func (s *shell) layoutXPStartOverlay(gtx layout.Context) layout.Dimensions {
	if !s.xpStartOpen {
		return layout.Dimensions{}
	}
	paint.Fill(gtx.Ops, color.NRGBA{A: 165})
	return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		width := min(gtx.Constraints.Max.X, gtx.Dp(unit.Dp(520)))
		gtx.Constraints.Min = image.Pt(width, 0)
		gtx.Constraints.Max.X = width
		return widget.Border{Color: palette.line, Width: unit.Dp(1)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			macro := op.Record(gtx.Ops)
			dimensions := layout.UniformInset(unit.Dp(22)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return labelWeight(gtx, s.theme, "Measure XP", unit.Sp(20), palette.text, text.Start, font.SemiBold)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						message := "Choose the point from which XP and combat history should be measured."
						if s.xpStartScanning {
							message = "Finding the latest level up and zoning entries in the logfile…"
						} else if s.xpStartError != "" {
							message = s.xpStartError
						}
						return inset(0, unit.Dp(14)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							return label(gtx, s.theme, message, unit.Sp(15), palette.muted, text.Start)
						})
					}),
					layout.Rigid(s.layoutXPStartChoices),
				)
			})
			content := macro.Stop()
			paint.FillShape(gtx.Ops, palette.panel, clip.Rect{Max: dimensions.Size}.Op())
			content.Add(gtx.Ops)
			return dimensions
		})
	})
}

func (s *shell) layoutXPStartChoices(gtx layout.Context) layout.Dimensions {
	if s.xpStartScanning {
		return layout.E.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return skySetupButton(gtx, s.theme, &s.xpStartCancel, "Cancel", false)
		})
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			if s.xpLastLevelUp.IsZero() {
				return label(gtx, s.theme, "No level-up entry found", unit.Sp(15), palette.muted, text.Start)
			}
			return skySetupButton(gtx, s.theme, &s.xpSinceLevel, "Since last level up", true)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			if s.xpLastZoneChange.IsZero() {
				return inset(0, unit.Dp(6)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return label(gtx, s.theme, "No zoning entry found", unit.Sp(15), palette.muted, text.Start)
				})
			}
			return skySetupButton(gtx, s.theme, &s.xpSinceZone, "Since last zoning", true)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.E.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return skySetupButton(gtx, s.theme, &s.xpStartCancel, "Cancel", false)
			})
		}),
	)
}
