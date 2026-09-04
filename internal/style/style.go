package style

import (
	"image/color"

	"gioui.org/widget/material"
	"github.com/uija/eqdps/internal/ui"
)

var Style = ui.Style{
	Theme: material.NewTheme(),
	Palette: ui.Palette{
		Window:     color.NRGBA{R: 18, G: 20, B: 22, A: 255},
		Chrome:     color.NRGBA{R: 27, G: 30, B: 33, A: 255},
		Text:       color.NRGBA{R: 225, G: 226, B: 222, A: 255},
		Muted:      color.NRGBA{R: 150, G: 154, B: 151, A: 255},
		Hover:      color.NRGBA{R: 42, G: 46, B: 50, A: 255},
		Panel:      color.NRGBA{R: 31, G: 34, B: 37, A: 255},
		LightPanel: color.NRGBA{R: 41, G: 44, B: 47, A: 255},
		Shadow:     color.NRGBA{A: 190},
		Accent:     color.NRGBA{R: 190, G: 155, B: 74, A: 255},
		Border:     color.NRGBA{R: 200, G: 200, B: 200, A: 255},

		Active:   color.NRGBA{R: 109, G: 178, B: 124, A: 255},
		Inactive: color.NRGBA{R: 190, G: 155, B: 74, A: 255},
		Done:     color.NRGBA{R: 120, G: 120, B: 120, A: 255},

		Yes: color.NRGBA{R: 190, G: 242, B: 199, A: 255},
		No:  color.NRGBA{R: 242, G: 190, B: 191, A: 255},

		Link:        color.NRGBA{R: 225, G: 226, B: 222, A: 255},
		LinkHover:   color.NRGBA{R: 63, G: 189, B: 224, A: 255},
		LinkClicked: color.NRGBA{R: 149, G: 123, B: 189, A: 255},
	},
}
