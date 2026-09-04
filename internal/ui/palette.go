package ui

import "image/color"

type Palette struct {
	Window     color.NRGBA `json:"window" label:"Window background"`
	Chrome     color.NRGBA `json:"chrome" label:"Window chrome"`
	Text       color.NRGBA `json:"text" label:"Text"`
	Muted      color.NRGBA `json:"muted" label:"Muted text"`
	Hover      color.NRGBA `json:"hover" label:"Hovered elements"`
	Panel      color.NRGBA `json:"panel" label:"Panel background"`
	LightPanel color.NRGBA `json:"light_panel" label:"Light panel background"`
	Shadow     color.NRGBA `json:"shadow" label:"Overlay shadow"`
	Accent     color.NRGBA `json:"accent" label:"Accent"`
	Border     color.NRGBA `json:"border" label:"Borders"`

	Active   color.NRGBA `json:"active" label:"Active"`
	Inactive color.NRGBA `json:"inactive" label:"Inactive"`
	Done     color.NRGBA `json:"done" label:"Completed"`

	Yes color.NRGBA `json:"yes" label:"Positive"`
	No  color.NRGBA `json:"no" label:"Negative"`

	Link        color.NRGBA `json:"link" label:"Links"`
	LinkHover   color.NRGBA `json:"link_hover" label:"Hovered links"`
	LinkClicked color.NRGBA `json:"link_clicked" label:"Visited links"`
}
