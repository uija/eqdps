package ui

import "image/color"

type Palette struct {
	Window color.NRGBA
	Chrome color.NRGBA
	Text   color.NRGBA
	Muted  color.NRGBA
	Hover  color.NRGBA
	Panel  color.NRGBA
	Shadow color.NRGBA
	Accent color.NRGBA
	Border color.NRGBA

	Active   color.NRGBA
	Inactive color.NRGBA
	Done     color.NRGBA

	Yes color.NRGBA
	No  color.NRGBA

	Link        color.NRGBA
	LinkHover   color.NRGBA
	LinkClicked color.NRGBA
}
