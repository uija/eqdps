package faction

import (
	"slices"
	"strconv"
	"strings"

	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget"
	"github.com/uija/eqdps/internal/ui"
)

func (m *Module) hasCharacter() bool {
	return m.logPath != "" && m.characterName != "" && m.serverName != ""
}

func (m *Module) Update(gtx layout.Context) {
	if !m.hasCharacter() || m.replay.Load() {
		m.closeAddFaction()
		m.closeDeleteFaction()
		return
	}
	if m.deleteIndex >= 0 {
		m.deleteForm.Update(gtx)
		return
	}
	if m.addingFaction {
		m.addForm.Update(gtx)
		return
	}
	if m.add_faction_click.Clicked(gtx) {
		options := slices.Clone(m.ctx.Config.KnownFactions)
		slices.Sort(options)
		m.factionSelect.SetOptions(options)
		m.factionSelect.SetSelected(0)
		m.startEditor.SetText("")
		m.endEditor.SetText("")
		m.formError = ""
		m.addingFaction = true
		m.addForm.Focus(gtx, "faction")
		return
	}
	for i, click := range m.deleteClicks {
		if click.Clicked(gtx) {
			m.deleteIndex = i
			m.deleteForm.Focus(gtx, "cancel")
			return
		}
	}
}

func (m *Module) closeAddFaction() {
	m.addingFaction = false
	m.factionSelect.Close()
	m.formError = ""
}

func (m *Module) saveNewFaction() {
	if !m.addingFaction || !m.hasCharacter() || m.replay.Load() {
		return
	}
	name := m.factionSelect.Value()
	if name == "" {
		m.formError = "Select a faction first."
		return
	}
	start, err := strconv.Atoi(strings.TrimSpace(m.startEditor.Text()))
	if err != nil {
		m.formError = "Start must be a whole number."
		return
	}
	end, err := strconv.Atoi(strings.TrimSpace(m.endEditor.Text()))
	if err != nil {
		m.formError = "End must be a whole number."
		return
	}
	m.factions = append(m.factions, Faction{Name: name, Start: start, End: end})
	m.deleteClicks = append(m.deleteClicks, new(widget.Clickable))
	m.saveFactions()
	m.closeAddFaction()
}

func (m *Module) layoutAddFaction(style *ui.Style, gtx layout.Context) layout.Dimensions {
	ui.Fill(gtx, style.Palette.Shadow)
	m.addForm.LayoutModalInputLayer(gtx)
	return ui.Overlay(gtx, 440, style.Palette.Panel, style.Palette.Border, func(gtx layout.Context) layout.Dimensions {
		return layout.UniformInset(unit.Dp(16)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			row := func(label string, field layout.Widget) layout.FlexChild {
				return layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Inset{Top: unit.Dp(12)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
							layout.Rigid(ui.Label(style, label).Layout),
							layout.Rigid(field),
						)
					})
				})
			}
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(ui.HeaderLabel(style, "Add Faction").Layout),
				layout.Rigid(ui.ColorLabel(style.Palette.Muted, ui.Label(style, "The Logfile only contains changes to faction, not the actual value, so you need to provide the starting value yourself.\nIf you don't find the faction you are looking for, get at least one faction hit for that")).Layout),
				row("Faction", func(gtx layout.Context) layout.Dimensions {
					return m.factionSelect.Layout(style, gtx, unit.Dp(408))
				}),
				row("Start", func(gtx layout.Context) layout.Dimensions {
					return ui.TextField(&m.startEditor, "Starting value", style, gtx)
				}),
				row("End", func(gtx layout.Context) layout.Dimensions {
					return ui.TextField(&m.endEditor, "Target value", style, gtx)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					message := m.formError
					if len(m.factionSelect.Options()) == 0 {
						message = "No factions discovered yet. Parse a log containing faction messages first."
					}
					if message == "" {
						return layout.Dimensions{}
					}
					return layout.Inset{Top: unit.Dp(12)}.Layout(gtx, ui.ColorLabel(style.Palette.Muted, ui.Label(style, message)).Layout)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Inset{Top: unit.Dp(16)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return layout.Flex{}.Layout(gtx,
							layout.Rigid(ui.IconLink(style, &m.saveClick, ui.Check, "Save").Layout),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								return layout.Inset{Left: unit.Dp(8)}.Layout(gtx, ui.IconLink(style, &m.cancelClick, ui.Close, "Cancel").Layout)
							}),
						)
					})
				}),
			)
		})
	})
}
