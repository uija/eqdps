package ui

import (
	"image/color"

	"gioui.org/widget/material"
)

func HeaderLabel(style *Style, text string) material.LabelStyle {
	return material.Label(style.Theme, Sp(17), text)
}
func Label(style *Style, text string) material.LabelStyle {
	return material.Label(style.Theme, Sp(15), text)
}
func ColorLabel(col color.NRGBA, label material.LabelStyle) material.LabelStyle {
	label.Color = col
	return label
}
