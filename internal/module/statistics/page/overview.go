package page

import (
	"gioui.org/layout"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"github.com/uija/eqdps/internal/ui"
)

type OverviewPage struct {
	tabClick widget.Clickable
}

func NewOverviewPage() *OverviewPage {
	return &OverviewPage{}
}

func (p *OverviewPage) Title() string {
	return "Overview"
}
func (p *OverviewPage) Clickable() *widget.Clickable {
	return &p.tabClick
}
func (p OverviewPage) Layout(style *ui.Style, gtx layout.Context) layout.Dimensions {
	return material.Label(style.Theme, ui.Sp(15), "Hallo Welt").Layout(gtx)
}
