package dps

import (
	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget/material"
	"github.com/uija/eqdps/internal/ui"
)

var dpsHelpSections = []struct {
	title string
	text  string
}{
	{
		text: "The DPS Meter automatically detects fights from your EverQuest logfile and records the damage dealt by everyone involved.\n\nEach fight shows its target, duration and how the fight ended. Active fights continue updating as new damage appears. Completed fights remain available so you can review them afterward.",
	},
	{
		title: "Reading the table",
		text: "• Combatant — The player, pet or NPC that dealt damage.\n" +
			"• Damage — Total damage dealt during the fight.\n" +
			"• DPS — Average damage per second over the full duration of the fight.\n" +
			"• SDPS — Damage per second from the point at which you actively joined the fight. This can be useful when a fight was already underway before you attacked.\n" +
			"• Hits — Number of successful damaging attacks.\n" +
			"• Crits — Number of critical hits.\n" +
			"• Active — Time between the combatant’s first and last damaging action.\n\n" +
			"The percentage next to a combatant shows their share of the fight’s total damage. For melee attacks, additional information about misses and hit chance may also be shown.",
	},
	{
		title: "Damage details",
		text: "Click a combatant to expand their damage breakdown. Damage is divided into:\n\n" +
			"• Melee — Normal attacks and combat abilities.\n" +
			"• Spells — Spells actively cast by the player.\n" +
			"• DoTs — Damage-over-time effects.\n" +
			"• Procs — Automatically triggered spell effects.\n" +
			"• Damage Shield — Damage caused by damage shields.\n\n" +
			"Expand these categories to see individual attacks, spells and abilities, including their total damage, DPS, number of hits, critical hits and active time.\n\nFor proc effects, the details can also show an estimated number of procs per minute.",
	},
	{
		title: "Fight history",
		text:  "Use the search field to filter the fight list by target name. This is useful when reviewing a long logfile containing many completed fights.\n\nNew fights are added automatically. If you scroll down to inspect an older fight, the list keeps your current position instead of jumping back to the newest entry.",
	},
	{
		title: "DPS overlay",
		text:  "The overlay provides a compact view of the fight that is most relevant to you. During combat, it prefers the active fight in which you most recently participated. When no fight is active, it keeps showing your most recent completed fight.\n\nYou can open or close the overlay from the DPS Meter. Its visibility is remembered when the application is restarted.",
	},
	{
		title: "Notes",
		text:  "Damage and fight timing are calculated entirely from messages found in the logfile. Results can differ slightly from the game or another parser when messages are missing, fights overlap, or combat begins before logging starts.",
	},
}

func (m *Module) LayoutHelp(style *ui.Style, gtx layout.Context) layout.Dimensions {
	list := material.List(style.Theme, &m.helpList)
	return list.Layout(gtx, len(dpsHelpSections), func(gtx layout.Context, index int) layout.Dimensions {
		section := dpsHelpSections[index]
		return layout.Inset{Bottom: unit.Dp(24), Right: unit.Dp(16)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			children := make([]layout.FlexChild, 0, 2)
			if section.title != "" {
				children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					label := material.Label(style.Theme, ui.Sp(18), section.title)
					label.Font.Weight = font.SemiBold
					return layout.Inset{Bottom: unit.Dp(8)}.Layout(gtx, label.Layout)
				}))
			}
			children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				label := material.Label(style.Theme, ui.Sp(15), section.text)
				label.Color = style.Palette.Muted
				return label.Layout(gtx)
			}))
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
		})
	})
}
