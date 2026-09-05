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
		title: "DPS and SDPS",
		text: "DPS uses the complete fight duration, including time before you joined the fight.\n\n" +
			"SDPS starts when you actively participate. It is useful when a fight was already underway before you attacked. SDPS is only shown for your own combatant row and only when it differs meaningfully from DPS.",
	},
	{
		title: "Percentages and misses",
		text: "The percentage beside a combatant or damage category is its share of the parent row’s total damage.\n\n" +
			"Your melee data can also include misses and hit chance. Avoided attacks such as dodges, parries and ripostes count as unsuccessful attempts rather than zero-damage hits.",
	},
	{
		title: "Damage categories",
		text: "Direct spell damage is listed under Spells only when it can be matched to a spell the character actively cast. Other spell-like damage is treated as a Proc. This prevents automatic effects from being mistaken for deliberate participation.\n\n" +
			"For proc effects, the details also estimate procs per minute over the fight duration.",
	},
	{
		title: "DPS overlay",
		text: "During combat, the overlay prefers the active fight in which you participated most recently. When no fight is active, it retains your most recent completed fight. This prevents unrelated nearby combat from immediately replacing your own target.\n\n" +
			"The overlay also displays active spell timers created by the Events module.",
	},
	{
		title: "Accuracy",
		text:  "All results are reconstructed from logfile messages. Missing messages, overlapping fights, delayed damage-over-time effects, or combat that began before logging started can produce small differences from the game or another parser.",
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
				return ui.ColorLabel(style.Palette.Muted, ui.Label(style, section.text)).Layout(gtx)
			}))
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
		})
	})
}
