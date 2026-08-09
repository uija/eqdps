package view

import (
	"fmt"

	"gioui.org/layout"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget/material"
)

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
				bar := material.ProgressBar(s.style.Theme, value)
				bar.Height = unit.Dp(8)
				bar.Radius = unit.Dp(0)
				bar.Color = s.style.Palette.Text
				bar.TrackColor = s.style.Palette.Hover
				return bar.Layout(gtx)
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{
					Top: unit.Dp(12),
				}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					label := material.Label(
						s.style.Theme,
						unit.Sp(14),
						detail,
					)
					label.Color = s.style.Palette.Muted
					label.Alignment = text.Middle
					return label.Layout(gtx)
				})
			}),
		)
	})
}
