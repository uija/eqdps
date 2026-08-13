package ui

import (
	"image"
	"image/color"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
)

type LinkStyle struct {
	style *Style
	click *widget.Clickable
	icon  *widget.Icon
	Size  float32
	Text  string

	FontWeight font.Weight
	TextAlign  text.Alignment

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
		label := material.Label(l.style.Theme, unit.Sp(l.Size), l.Text)
		label.Color = col
		label.Font.Weight = l.FontWeight
		if l.icon == nil {
			tgtx := gtx
			if l.TextAlign != text.Start {
				tgtx.Constraints.Min.X = tgtx.Constraints.Max.X
				label.Alignment = l.TextAlign
			}
			return label.Layout(tgtx)
		}
		align := layout.Start
		switch l.TextAlign {
		case text.Middle:
			align = layout.Middle
		case text.End:
			align = layout.End
		}
		content := func(gtx layout.Context) layout.Dimensions {
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
		}
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

		TextColor:    style.Palette.Link,
		HoverColor:   style.Palette.LinkHover,
		ClickedColor: style.Palette.LinkClicked,
	}
}
