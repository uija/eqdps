package ui

import (
	"image"
	"image/color"

	"gioui.org/font"
	"gioui.org/layout"
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
			return label.Layout(gtx)
		}
		return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
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
	})
}
func Link(style *Style, click *widget.Clickable, text string) LinkStyle {
	return LinkStyle{
		style: style,
		click: click,
		icon:  nil,
		Text:  text,

		Size:       14,
		FontWeight: font.Normal,

		TextColor:    style.Palette.Link,
		HoverColor:   style.Palette.LinkHover,
		ClickedColor: style.Palette.LinkClicked,
	}
}

func IconLink(style *Style, click *widget.Clickable, icon *widget.Icon, text string) LinkStyle {
	return LinkStyle{
		style: style,
		click: click,
		icon:  icon,
		Text:  text,

		Size:       14,
		FontWeight: font.Normal,

		TextColor:    style.Palette.Link,
		HoverColor:   style.Palette.LinkHover,
		ClickedColor: style.Palette.LinkClicked,
	}
}
