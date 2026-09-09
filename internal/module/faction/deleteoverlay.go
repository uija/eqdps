package faction

import (
	"fmt"
	"slices"

	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget"
	"github.com/uija/eqdps/internal/ui"
	"golang.org/x/exp/shiny/materialdesign/icons"
)

var factionDeleteIcon = func() *widget.Icon {
	icon, err := widget.NewIcon(icons.ActionDelete)
	if err != nil {
		panic(err)
	}
	return icon
}()

func (m *Module) closeDeleteFaction() {
	m.deleteIndex = -1
}

func (m *Module) deleteFaction() {
	if !m.hasCharacter() || m.replay.Load() || m.deleteIndex < 0 || m.deleteIndex >= len(m.factions) {
		return
	}
	m.factions = slices.Delete(m.factions, m.deleteIndex, m.deleteIndex+1)
	m.deleteClicks = slices.Delete(m.deleteClicks, m.deleteIndex, m.deleteIndex+1)
	m.saveFactions()
	m.closeDeleteFaction()
}

func (m *Module) layoutDeleteFaction(style *ui.Style, gtx layout.Context) layout.Dimensions {
	if m.deleteIndex < 0 || m.deleteIndex >= len(m.factions) {
		return layout.Dimensions{}
	}
	ui.Fill(gtx, style.Palette.Shadow)
	m.deleteForm.LayoutModalInputLayer(gtx)
	return ui.Overlay(gtx, 440, style.Palette.Panel, style.Palette.Border, func(gtx layout.Context) layout.Dimensions {
		return layout.UniformInset(unit.Dp(16)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(ui.HeaderLabel(style, fmt.Sprintf("Stop tracking %s?", m.factions[m.deleteIndex].Name)).Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Inset{Top: unit.Dp(12)}.Layout(gtx,
						ui.Label(style, "Its tracked progress will be removed. You can add the faction again later.").Layout)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Inset{Top: unit.Dp(16)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return layout.Flex{}.Layout(gtx,
							layout.Rigid(ui.IconLink(style, &m.confirmDelete, ui.Check, "Delete").Layout),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								return layout.Inset{Left: unit.Dp(8)}.Layout(gtx, ui.IconLink(style, &m.cancelDelete, ui.Close, "Cancel").Layout)
							}),
						)
					})
				}),
			)
		})
	})
}
