package view

import (
	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"github.com/uija/eqdps/internal/ui"
)

type helpView struct {
	style    *ui.Style
	open     bool
	close    widget.Clickable
	backdrop widget.Clickable
}

func newHelpView(style *ui.Style) *helpView {
	return &helpView{style: style}
}

func (v *helpView) Open() {
	v.open = true
}

func (v *helpView) Update(gtx layout.Context) {
	if v.close.Clicked(gtx) || v.backdrop.Clicked(gtx) {
		v.open = false
	}
}

func (v *helpView) Layout(gtx layout.Context) layout.Dimensions {
	if !v.open {
		return layout.Dimensions{}
	}

	return layout.Stack{}.Layout(gtx,
		layout.Expanded(func(gtx layout.Context) layout.Dimensions {
			return v.backdrop.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				fill(gtx, v.style.Palette.Shadow)
				return layout.Dimensions{Size: gtx.Constraints.Min}
			})
		}),
		layout.Expanded(func(gtx layout.Context) layout.Dimensions {
			return layout.UniformInset(unit.Dp(32)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				gtx.Constraints.Min = gtx.Constraints.Max
				fill(gtx, v.style.Palette.Panel)
				return layout.UniformInset(unit.Dp(24)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
								layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
									label := material.Label(v.style.Theme, unit.Sp(24), "Help")
									label.Color = v.style.Palette.Text
									return label.Layout(gtx)
								}),
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									button := material.Button(v.style.Theme, &v.close, "Close")
									return button.Layout(gtx)
								}),
							)
						}),
						layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
							return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
								label := material.Label(v.style.Theme, unit.Sp(15), "No help content available")
								label.Color = v.style.Palette.Muted
								return label.Layout(gtx)
							})
						}),
					)
				})
			})
		}),
	)
}
