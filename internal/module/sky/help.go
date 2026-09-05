package sky

import (
	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget/material"
	"github.com/uija/eqdps/internal/ui"
)

var skyHelpSections = []struct {
	title string
	text  string
}{
	{
		title: "Automatic tracking",
		text: "Recognized quest items are added from loot messages and removed when they are destroyed or used in a recognized turn-in. Item quantities and stacks are taken into account.\n\n" +
			"A turn-in is recorded only when its quest giver and handed-in items match a known quest. Rejected, incomplete or unusual trades may require a manual correction.",
	},
	{
		title: "Inventory exports",
		text: "Use /outputfile inventory in EverQuest to compare the tracker with the Plane of Sky items your character currently owns. This can recover items looted before eqdps was running and correct amounts that drifted out of sync.\n\n" +
			"An inventory export cannot recover completed quest turn-ins because those items are no longer present.",
	},
	{
		title: "Manual rune corrections",
		text: "Missing Runes? lets you correct rune amounts when the logfile or inventory export is incomplete. Typical causes are disabled logging, storage locations absent from an export, or items consumed without a recognized message.\n\n" +
			"Manual values remain until they are changed again or Reset & Reload is used.",
	},
	{
		title: "Reset and Reload",
		text:  "Reset & Reload deletes the locally collected Plane of Sky progress and rebuilds it from the current logfile. Manual rune corrections and information obtained only from inventory exports must be restored afterward.",
	},
	{
		title: "EQLDB contribution",
		text:  "When Plane of Sky contribution is enabled, eqdps sends recognized rune changes and completed quest names to eqldb.org. It does not upload unrelated logfile messages. Disabling contribution does not affect local tracking.",
	},
}

func (m *Module) LayoutHelp(style *ui.Style, gtx layout.Context) layout.Dimensions {
	sectionCount := len(skyHelpSections) + 1
	list := material.List(style.Theme, &m.helpList)
	return list.Layout(gtx, sectionCount, func(gtx layout.Context, index int) layout.Dimensions {
		section := struct {
			title string
			text  string
		}{}
		if index < len(skyHelpSections) {
			section = skyHelpSections[index]
		} else {
			section.title = "Data and limitations"
			section.text = "The tracker can only reconstruct events present in the logfile or current inventory. Its data is stored separately for each character and server.\n\nData file: " + m.configPath
		}

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
