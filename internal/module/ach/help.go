package ach

import (
	"log"

	_ "embed"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/widget/material"
	"gioui.org/x/markdown"
	"gioui.org/x/richtext"
	"github.com/uija/eqdps/internal/ui"
)

//go:embed help.md
var help []byte

func (m *Module) LayoutHelp(style *ui.Style, gtx layout.Context) layout.Dimensions {
	renderer := markdown.NewRenderer()
	renderer.Config = markdown.Config{
		DefaultFont:      font.Font{Typeface: style.Theme.Face},
		DefaultSize:      ui.Sp(15),
		DefaultColor:     style.Palette.Text,
		InteractiveColor: style.Palette.Accent,
	}
	rows, err := renderer.Render(help)
	if err != nil {
		log.Printf("Error rendering md")
		return layout.Dimensions{}
	}
	list := material.List(style.Theme, &m.helplist)

	return list.Layout(gtx, 1, func(gtx layout.Context, index int) layout.Dimensions {
		return richtext.Text(
			&m.helpText,
			style.Theme.Shaper,
			rows...,
		).Layout(gtx)

	})
}
