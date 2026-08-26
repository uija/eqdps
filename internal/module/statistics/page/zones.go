package page

import (
	"gioui.org/layout"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"github.com/uija/eqdps/internal/ui"
)

type ZonesPage struct {
	tabClick widget.Clickable
}

func NewZonesPage() *ZonesPage {
	return &ZonesPage{}
}

func (p *ZonesPage) Title() string {
	return "Zones"
}
func (p *ZonesPage) Clickable() *widget.Clickable {
	return &p.tabClick
}
func (p ZonesPage) Layout(style *ui.Style, gtx layout.Context) layout.Dimensions {
	return material.Label(style.Theme, ui.Sp(15), "Hallo Welt").Layout(gtx)
}
