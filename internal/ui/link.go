package ui

import (
	"image"
	"image/color"

	"gioui.org/font"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
)

const (
	PADDING_TOP    = 0
	PADDING_RIGHT  = 1
	PADDING_BOTTOM = 2
	PADDING_LEFT   = 3
)

type LinkStyle struct {
	style *Style
	click *widget.Clickable
	icon  *widget.Icon
	Size  float32
	Text  string

	FontWeight font.Weight
	TextAlign  text.Alignment
	Padding    []float32

	TextColor    color.NRGBA
	HoverColor   color.NRGBA
	ClickedColor color.NRGBA
}

func (l LinkStyle) Layout(gtx layout.Context) layout.Dimensions {
	return l.click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		col := l.TextColor
		if l.click.Hovered() {
			col = l.HoverColor
		} else if l.click.Pressed() {
			col = l.ClickedColor
		}
		label := material.Label(l.style.Theme, Sp(l.Size), l.Text)
		label.Color = col
		label.Font.Weight = l.FontWeight
		if l.icon == nil {
			tgtx := gtx
			if l.TextAlign != text.Start {
				tgtx.Constraints.Min.X = tgtx.Constraints.Max.X
				label.Alignment = l.TextAlign
			}
			pointer.CursorPointer.Add(gtx.Ops)
			return layout.Inset{Top: unit.Dp(l.Padding[0]), Right: unit.Dp(l.Padding[1]), Bottom: unit.Dp(l.Padding[2]), Left: unit.Dp(l.Padding[3])}.Layout(gtx, label.Layout)
		}
		align := layout.Start
		switch l.TextAlign {
		case text.Middle:
			align = layout.Middle
		case text.End:
			align = layout.End
		}
		content := func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Top: unit.Dp(l.Padding[0]), Right: unit.Dp(l.Padding[1]), Bottom: unit.Dp(l.Padding[2]), Left: unit.Dp(l.Padding[3])}.Layout(gtx,
				func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Horizontal, Alignment: align}.Layout(gtx,
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							gtx.Constraints = layout.Exact(image.Pt(gtx.Dp(unit.Dp(18)), gtx.Dp(unit.Dp(18))))
							return layout.Inset{Top: unit.Dp(2)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
								return l.icon.Layout(gtx, col)
							})
						}),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return layout.Inset{Left: unit.Dp(6)}.Layout(gtx, label.Layout)
						}),
					)
				},
			)
		}
		pointer.CursorPointer.Add(gtx.Ops)
		switch l.TextAlign {
		case text.End:
			return layout.E.Layout(gtx, content)
		case text.Middle:
			return layout.Center.Layout(gtx, content)
		default:
			return content(gtx)
		}
	})
}
func Link(style *Style, click *widget.Clickable, str string) LinkStyle {
	return LinkStyle{
		style:     style,
		click:     click,
		icon:      nil,
		Text:      str,
		TextAlign: text.Start,

		Size:       14,
		FontWeight: font.Normal,
		Padding:    make([]float32, 4),

		TextColor:    style.Palette.Link,
		HoverColor:   style.Palette.LinkHover,
		ClickedColor: style.Palette.LinkClicked,
	}
}

func IconLink(style *Style, click *widget.Clickable, icon *widget.Icon, str string) LinkStyle {
	return LinkStyle{
		style: style,
		click: click,
		icon:  icon,
		Text:  str,

		Size:       14,
		FontWeight: font.Normal,
		TextAlign:  text.Start,
		Padding:    make([]float32, 4),

		TextColor:    style.Palette.Link,
		HoverColor:   style.Palette.LinkHover,
		ClickedColor: style.Palette.LinkClicked,
	}
}
