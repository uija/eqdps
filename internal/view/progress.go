package view

import (
	"fmt"
	"image"

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

	fill(gtx, s.style.Palette.Shadow)

	return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		width := min(gtx.Dp(unit.Dp(420)), gtx.Constraints.Max.X)
		height := min(gtx.Dp(unit.Dp(120)), gtx.Constraints.Max.Y)
		gtx.Constraints = layout.Exact(image.Pt(width, height))

		fill(gtx, s.style.Palette.Panel)

		return layout.Flex{
			Axis: layout.Vertical,
		}.Layout(gtx,
			layout.Rigid(ui.TitleBar(gtx, *s.style, s.progress.title)),
			layout.Flexed(1, s.layoutProgressContent),
		)
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
