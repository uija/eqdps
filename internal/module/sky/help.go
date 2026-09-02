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
		text: "The Plane of Sky Quest Tracker helps you keep track of quest progression, collected quest items and completed armor quests.\n\nIt watches your EverQuest logfile and automatically records recognized Plane of Sky loot, destroyed items and completed quest turn-ins.",
	},
	{
		title: "Progression",
		text: "The Progression page groups quests by class. Each quest shows the required items and how many of them you currently have.\n\n" +
			"Quests are marked according to their current state:\n\n" +
			"• Ready to turn in — You have all required items.\n" +
			"• In progress — You have collected some, but not all, required items.\n" +
			"• Empty — You have not collected any required items.\n" +
			"• Finished — The quest has already been completed.\n\n" +
			"Use the controls at the top to hide finished or empty quests and keep the list focused on relevant progression.",
	},
	{
		title: "Watching quests",
		text:  "You can mark individual quests as watched. Watched quests are collected in a separate section near the top of the page, making it easier to follow the quests you are currently working on.\n\nWatching a quest does not change its progress or consume any items.",
	},
	{
		title: "Collected items",
		text:  "Recognized quest items are added automatically when they appear in loot messages. Items are removed when the logfile reports that they were destroyed or handed in as part of a recognized quest.\n\nThe tracker also understands item quantities, including stacks containing more than one item.\n\nSelect Show Inventory to see all currently recorded Plane of Sky quest items grouped by their location or purpose.",
	},
	{
		title: "Inventory exports",
		text: "An EverQuest inventory export can be used to compare the tracker with the items your character currently owns. This is useful when items were collected before eqdps was running or when the stored amounts no longer match your inventory.\n\n" +
			"Create an inventory export in EverQuest with:\n\n/outputfile inventory\n\n" +
			"eqdps detects the completed export from the logfile and updates the recorded Plane of Sky items.\n\nAn inventory export can only report items currently owned by the character. It cannot restore information about completed quests.",
	},
	{
		title: "Missing or incorrect runes",
		text: "Select Missing Runes? to adjust the recorded rune amounts manually.\n\n" +
			"This can be useful when:\n\n" +
			"• an item was looted before logging started;\n" +
			"• an inventory export did not include every relevant storage location;\n" +
			"• an item was moved or consumed without a recognizable logfile message;\n" +
			"• the tracker’s stored amount differs from your actual inventory.\n\n" +
			"Manual corrections remain stored until they are changed again or the tracker is fully reset.",
	},
	{
		title: "Quest turn-ins",
		text:  "When you complete a recognized Plane of Sky quest, the tracker records the completed quest and removes the required items from its inventory.\n\nA turn-in can only be recognized when the quest giver and handed-in items match a known quest. Unusual, rejected or incomplete trades may not be counted automatically.",
	},
	{
		title: "Reset and Reload",
		text:  "Reset & Reload deletes the collected Plane of Sky progress and rebuilds it from the current logfile.\n\nUse this when the stored data appears incorrect or when the tracker’s data format has changed. A full reload can take a moment for large logfiles.\n\nManual rune corrections and information obtained only from inventory exports may need to be entered again afterward.",
	},
	{
		title: "EQLDB contribution",
		text:  "If Plane of Sky contribution is enabled in the EQLDB settings, recognized rune changes and completed quests are sent to eqldb.org.\n\nOnly the relevant Plane of Sky events are contributed. Disabling this option does not affect local quest tracking.",
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
			section.title = "Notes"
			section.text = "The tracker can only use information available in the logfile and inventory exports. Events that occurred while logging was disabled may be missing until you perform an inventory export or correct the amounts manually.\n\nYour Plane of Sky progress is stored separately for each character and server.\n\nData file: " + m.configPath
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
				label := material.Label(style.Theme, ui.Sp(15), section.text)
				label.Color = style.Palette.Muted
				return label.Layout(gtx)
			}))
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
		})
	})
}
