package view

import (
	"fmt"

	"gioui.org/layout"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget/material"
	"github.com/uija/eqdps/internal/ui"
)

func (s *Shell) layoutProgressOverlay(gtx layout.Context) layout.Dimensions {
	if s.progress == nil {
		return layout.Dimensions{}
	}

	ui.Fill(gtx, s.Style.Palette.Shadow)
	return ui.Overlay(gtx, 420, s.Style.Palette.Panel, s.Style.Palette.Border, func(gtx layout.Context) layout.Dimensions {
		return layout.UniformInset(unit.Dp(32)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{
				Axis: layout.Vertical,
			}.Layout(gtx,
				layout.Rigid(ui.TitleBar(gtx, *s.Style, s.progress.title)),
				layout.Rigid(s.layoutProgressContent),
			)
		})
	})
}

func (s *Shell) layoutProgressContent(gtx layout.Context) layout.Dimensions {
	value := float32(0)
	if s.progress.max > 0 {
		value = float32(s.progress.value) / float32(s.progress.max)
	}
	percent := int(value*100 + 0.5)
	detail := fmt.Sprintf("%d / %d (%d%%", s.progress.value, s.progress.max, percent)

	return layout.UniformInset(unit.Dp(16)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{
			Axis: layout.Vertical,
		}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				bar := material.ProgressBar(s.Style.Theme, value)
				bar.Height = unit.Dp(8)
				bar.Radius = unit.Dp(0)
				bar.Color = s.Style.Palette.Text
				bar.TrackColor = s.Style.Palette.Hover
				return bar.Layout(gtx)
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{
					Top: unit.Dp(12),
				}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					label := ui.ColorLabel(s.Style.Palette.Muted, material.Label(
						s.Style.Theme,
						ui.Sp(14),
						detail,
					))
					label.Alignment = text.Middle
					return label.Layout(gtx)
				})
			}),
		)
	})
}
