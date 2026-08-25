package view

import (
	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"github.com/uija/eqdps/internal/module"
	"github.com/uija/eqdps/internal/ui"
)

type helpView struct {
	context *module.Context
	style   *ui.Style
	open    bool

	close widget.Clickable
	back  widget.Clickable

	backdrop widget.Clickable

	links []widget.Clickable

	helpItem *module.HelpItem
}

func newHelpView(style *ui.Style, context *module.Context) *helpView {
	return &helpView{style: style, helpItem: nil, context: context}
}

func (v *helpView) Open() {
	v.open = true
}

func (v *helpView) Update(gtx layout.Context) {
	if v.close.Clicked(gtx) || v.backdrop.Clicked(gtx) {
		v.open = false
	}
	for index := range v.links {
		if v.links[index].Clicked(gtx) {
			v.helpItem = &v.context.HelpItems[index]
		}
	}
	if v.back.Clicked(gtx) {
		v.helpItem = nil
	}
}

func (v *helpView) Layout(gtx layout.Context) layout.Dimensions {
	if !v.open {
		return layout.Dimensions{}
	}

	return layout.Stack{}.Layout(gtx,
		layout.Expanded(func(gtx layout.Context) layout.Dimensions {
			return v.backdrop.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				ui.Fill(gtx, v.style.Palette.Shadow)
				return layout.Dimensions{Size: gtx.Constraints.Min}
			})
		}),
		layout.Expanded(func(gtx layout.Context) layout.Dimensions {
			return layout.UniformInset(unit.Dp(32)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				gtx.Constraints.Min = gtx.Constraints.Max
				ui.Fill(gtx, v.style.Palette.Panel)
				if v.helpItem == nil {
					return layout.UniformInset(unit.Dp(24)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
									layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
										label := material.Label(v.style.Theme, ui.Sp(24), "Help")
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
								return layout.Inset{Top: unit.Dp(24)}.Layout(gtx, v.layoutContent)
							}),
						)
					})
				} else {
					return layout.UniformInset(unit.Dp(24)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
									layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
										label := material.Label(v.style.Theme, ui.Sp(24), v.helpItem.Name)
										label.Color = v.style.Palette.Text
										return label.Layout(gtx)
									}),
									layout.Rigid(func(gtx layout.Context) layout.Dimensions {
										button := material.Button(v.style.Theme, &v.back, "Back")
										return button.Layout(gtx)
									}),
								)
							}),
							layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
								return layout.Inset{Top: unit.Dp(24)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
									return v.helpItem.Layout(v.style, gtx)
								})
							}),
						)
					})
				}
			})
		}),
	)
}

func (v *helpView) layoutContent(gtx layout.Context) layout.Dimensions {
	if len(v.links) != len(v.context.HelpItems) {
		v.links = make([]widget.Clickable, len(v.context.HelpItems))
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			label := material.Label(
				v.style.Theme,
				ui.Sp(15),
				"This is placeholder help text. Replace this paragraph with an introduction that explains where users can find more information.",
			)
			label.Color = v.style.Palette.Muted
			return label.Layout(gtx)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Top: unit.Dp(20)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				items := make([]layout.FlexChild, 0, len(v.context.HelpItems))
				for index, helpItem := range v.context.HelpItems {
					items = append(items, layout.Rigid(v.layoutLink(index, helpItem)))
				}
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx, items...)
			})
		}),
	)
}

func (v *helpView) layoutLink(index int, item module.HelpItem) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		return layout.Inset{Top: unit.Dp(12), Bottom: unit.Dp(12)}.Layout(gtx, ui.IconLink(v.style, &v.links[index], ui.Help, item.Name).Layout)
	}
}
