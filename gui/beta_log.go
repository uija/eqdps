package main

import (
	"context"
	"fmt"
	"image"
	"path/filepath"

	"gioui.org/font"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"github.com/uija/eqdps/internal/launchlog"
)

type betaCleanupResult struct {
	archive string
	err     error
}

func (s *shell) chooseLogfile(choice fileChoice) {
	check, err := launchlog.Inspect(choice.path)
	if err != nil {
		s.statusText = "Could not check logfile for beta data: " + err.Error()
		return
	}
	if !check.NeedsAction {
		s.rememberChosenFile(choice)
		return
	}
	s.pauseEQLDBSync()
	s.betaChoice = choice
	s.betaPromptOpen = true
	s.betaWorking = false
	s.betaError = ""
	s.statusText = "Beta data found in " + filepath.Base(choice.path)
}

func (s *shell) updateBetaCleanup(gtx layout.Context) {
	if s.betaIgnore.Clicked(gtx) && s.betaPromptOpen && !s.betaWorking {
		if err := launchlog.RememberIgnored(s.betaChoice.path); err != nil {
			s.betaError = err.Error()
		} else {
			choice := s.betaChoice
			s.closeBetaPrompt()
			s.startEQLDBSync()
			s.rememberChosenFile(choice)
		}
	}
	if s.betaFix.Clicked(gtx) && s.betaPromptOpen && !s.betaWorking {
		s.betaWorking = true
		s.betaError = ""
		done := s.pauseEQLDBSync()
		path := s.betaChoice.path
		go func() {
			<-done
			archive, err := launchlog.Fix(path)
			s.betaResults <- betaCleanupResult{archive: archive, err: err}
			s.window.Invalidate()
		}()
	}
	select {
	case result := <-s.betaResults:
		s.betaWorking = false
		if result.err != nil {
			s.betaError = result.err.Error()
			break
		}
		choice := s.betaChoice
		s.closeBetaPrompt()
		s.startEQLDBSync()
		s.statusText = "Archived Beta logfile as " + result.archive
		s.rememberChosenFile(choice)
	default:
	}
}

func (s *shell) closeBetaPrompt() {
	s.betaPromptOpen = false
	s.betaWorking = false
	s.betaError = ""
	s.betaChoice = fileChoice{}
}

func (s *shell) startEQLDBSync() {
	if s.eqldbRunner == nil || s.eqldbSyncCancel != nil {
		return
	}
	previous := s.eqldbSyncDone
	syncContext, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	s.eqldbSyncCancel = cancel
	s.eqldbSyncDone = done
	go func() {
		if previous != nil {
			<-previous
		}
		s.eqldbRunner.Run(syncContext)
		close(done)
	}()
}

func (s *shell) pauseEQLDBSync() <-chan struct{} {
	if s.eqldbSyncCancel == nil {
		if s.eqldbSyncDone != nil {
			return s.eqldbSyncDone
		}
		done := make(chan struct{})
		close(done)
		return done
	}
	done := s.eqldbSyncDone
	s.eqldbSyncCancel()
	s.eqldbSyncCancel = nil
	return done
}

func (s *shell) layoutBetaCleanup(gtx layout.Context) layout.Dimensions {
	if !s.betaPromptOpen {
		return layout.Dimensions{}
	}
	return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		width := min(gtx.Constraints.Max.X, gtx.Dp(unit.Dp(680)))
		height := min(gtx.Constraints.Max.Y, gtx.Dp(unit.Dp(340)))
		gtx.Constraints.Min = image.Pt(width, height)
		gtx.Constraints.Max = gtx.Constraints.Min
		return outline(gtx, palette.line, func(gtx layout.Context) layout.Dimensions {
			fill(gtx, palette.panel)
			return layout.UniformInset(unit.Dp(24)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				message := fmt.Sprintf(
					"%s contains data from before the EverQuest Legends launch.\n\n"+
						"Beta data can make logfile loading slower and would produce incorrect Plane of Sky progress. "+
						"eqdps can archive the Beta logfile, keep entries from launch onward, and reset all Beta tracking and queued upload data.",
					filepath.Base(s.betaChoice.path),
				)
				if s.betaWorking {
					message += "\n\nArchiving and cleaning the logfile…"
				} else if s.betaError != "" {
					message += "\n\nCleanup failed: " + s.betaError
				}
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return labelWeight(gtx, s.theme, "Beta logfile detected", unit.Sp(21), palette.text, text.Start, font.SemiBold)
					}),
					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
						return inset(0, unit.Dp(16)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							return label(gtx, s.theme, message, unit.Sp(15), palette.text, text.Start)
						})
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						if s.betaWorking {
							return layout.Dimensions{}
						}
						return layout.E.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									return betaCleanupButton(gtx, s.theme, &s.betaIgnore, "Ignore", false)
								}),
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									return betaCleanupButton(gtx, s.theme, &s.betaFix, "Fix logfile", true)
								}),
							)
						})
					}),
				)
			})
		})
	})
}

func betaCleanupButton(gtx layout.Context, theme *material.Theme, clickable *widget.Clickable, caption string, primary bool) layout.Dimensions {
	return clickable.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		pointer.CursorPointer.Add(gtx.Ops)
		background := palette.panelAlt
		foreground := palette.text
		if primary {
			background = palette.accent
			foreground = palette.window
		}
		fill(gtx, background)
		return inset(unit.Dp(14), unit.Dp(9)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return labelWeight(gtx, theme, caption, unit.Sp(15), foreground, text.Middle, font.SemiBold)
		})
	})
}
