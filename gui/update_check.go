package main

import (
	"context"
	"image"
	"image/color"
	"net/http"
	"time"

	"gioui.org/font"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget/material"
	"github.com/uija/eqdps/internal/notify"
	"github.com/uija/eqdps/internal/platform"
	"github.com/uija/eqdps/internal/updatecheck"
)

type updateCheckResult struct {
	release updatecheck.Release
}

func (s *shell) startUpdateCheck() {
	if !s.settings.updateChecksEnabled() {
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
		defer cancel()
		release, err := updatecheck.Latest(ctx, &http.Client{Timeout: 10 * time.Second}, updatecheck.LatestReleaseURL)
		if err != nil {
			return
		}
		s.updateResults <- updateCheckResult{release: release}
		s.window.Invalidate()
	}()
}

func (s *shell) updateReleaseCheck(gtx layout.Context) {
	select {
	case result := <-s.updateResults:
		release := result.release
		if release.TagName == s.settings.LastSeenReleaseTag {
			break
		}
		firstCheck := s.settings.LastSeenReleaseTag == ""
		s.settings.LastSeenReleaseTag = release.TagName
		if err := saveSettings(s.settings); err != nil {
			s.statusText = "Latest release tag could not be saved"
		}
		if !firstCheck {
			s.updateTag = release.TagName
			s.updateURL = release.HTMLURL
			s.updateBody = release.Body
			s.updateOpen = true
			go func() {
				_ = (notify.Desktop{}).Send(context.Background(), "eqdps update available", "A new eqdps release is available: "+release.TagName, "", false)
			}()
		}
	default:
	}

	if s.updateOpenClick.Clicked(gtx) {
		if err := platform.OpenURL(s.updateURL); err != nil {
			s.statusText = "Could not open the release page: " + err.Error()
		} else {
			s.updateOpen = false
		}
	}
	if s.updateCloseClick.Clicked(gtx) {
		s.updateOpen = false
	}
}

func (s *shell) layoutUpdateAvailable(gtx layout.Context) layout.Dimensions {
	if !s.updateOpen {
		return layout.Dimensions{}
	}
	paintOverlay(gtx)
	return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		width := min(gtx.Constraints.Max.X, gtx.Dp(unit.Dp(600)))
		height := min(gtx.Constraints.Max.Y, gtx.Dp(unit.Dp(380)))
		gtx.Constraints.Min = image.Pt(width, height)
		gtx.Constraints.Max = gtx.Constraints.Min
		return outline(gtx, palette.line, func(gtx layout.Context) layout.Dimensions {
			fill(gtx, palette.panel)
			return layout.UniformInset(unit.Dp(24)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				body := s.updateBody
				if body == "" {
					body = "No release notes were provided."
				}
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return labelWeight(gtx, s.theme, "Update available", unit.Sp(23), palette.text, text.Start, font.SemiBold)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return inset(0, unit.Dp(8)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							return labelWeight(gtx, s.theme, s.updateTag, unit.Sp(16), palette.accent, text.Start, font.SemiBold)
						})
					}),
					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
						return inset(0, unit.Dp(10)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							list := material.List(s.theme, &s.updateBodyList)
							list.AnchorStrategy = material.Occupy
							list.Indicator.Color = palette.muted
							return list.Layout(gtx, 1, func(gtx layout.Context, _ int) layout.Dimensions {
								return label(gtx, s.theme, body, unit.Sp(15), palette.text, text.Start)
							})
						})
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return inset(0, unit.Dp(10)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							return label(gtx, s.theme, "You can disable automatic update checks in Preferences.", unit.Sp(14), palette.muted, text.Start)
						})
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return layout.E.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									return updateDialogButton(gtx, s, &s.updateOpenClick, "Open release on GitHub", true)
								}),
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									return inset(unit.Dp(8), 0).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
										return updateDialogButton(gtx, s, &s.updateCloseClick, "Close", false)
									})
								}),
							)
						})
					}),
				)
			})
		})
	})
}

func paintOverlay(gtx layout.Context) {
	fill(gtx, color.NRGBA{A: 175})
}

func updateDialogButton(gtx layout.Context, s *shell, click interface {
	Layout(layout.Context, layout.Widget) layout.Dimensions
}, title string, primary bool) layout.Dimensions {
	background := palette.panelAlt
	foreground := palette.text
	if primary {
		background = palette.accent
		foreground = palette.window
	}
	return click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		pointer.CursorPointer.Add(gtx.Ops)
		fill(gtx, background)
		return layout.Inset{Top: unit.Dp(9), Bottom: unit.Dp(9), Left: unit.Dp(13), Right: unit.Dp(13)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return labelWeight(gtx, s.theme, title, unit.Sp(15), foreground, text.Middle, font.SemiBold)
		})
	})
}
