package events

import (
	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget/material"
	"github.com/uija/eqdps/internal/ui"
)

type eventsHelpExample struct {
	caption string
	text    string
}

type eventsHelpSection struct {
	title    string
	text     string
	examples []eventsHelpExample
}

var eventsHelpSections = []eventsHelpSection{
	{
		title: "When events run",
		text:  "Events react only to new live logfile messages. They do not trigger while eqdps is replaying older logfile content. Disabling an event keeps its configuration but prevents its notification and sound.",
	},
	{
		title: "Spell events",
		text: "Spell events use fade messages from the included spell catalogue. Self watches for an effect fading from you, Others watches for it fading from another target, and Both accepts either message.\n\n" +
			"If the server uses a fade message that differs from the catalogue, use a Text or RegExp event instead.",
	},
	{
		title: "Timer events",
		text: "A timer begins when the logfile reports that you start casting the selected spell. Its duration is entered in seconds. Recasting the same configured spell restarts the timer.\n\n" +
			"Timers do not verify whether the spell landed or faded early. Active timers are also shown in the DPS overlay.",
	},
	{
		title: "Text events",
		text: "A normal Text event performs a case-sensitive search anywhere in the logfile message. Full message match requires the entire message but ignores letter case. The timestamp at the start of the logfile line is not included.\n\n" +
			"The example matches every message containing these words:",
		examples: []eventsHelpExample{
			{caption: "Example text", text: "Your target resisted"},
		},
	},
	{
		title: "Regular-expression events",
		text: "Regular expressions are matched against the message without its timestamp. Validate an expression before saving it; an invalid expression cannot trigger.\n\n" +
			"This example matches ordinary personal-loot messages:",
		examples: []eventsHelpExample{
			{caption: "Example regular expression", text: "^You have looted .+ from .+'s corpse\\.$"},
		},
	},
	{
		title: "Notification text",
		text: "Use %s in the notification message to insert the event title. For an event titled Clarity, the following input produces the displayed result below. Leave the notification empty when an event should only play a sound.\n\n" +
			"The persistent-notification setting is saved with the event, but the current notification backend does not support changing notification lifetime.",
		examples: []eventsHelpExample{
			{caption: "Notification text", text: "%s faded."},
			{caption: "Displayed message", text: "Clarity faded."},
		},
	},
	{
		title: "Custom sounds and spell icons",
		text: "MP3 files placed in the eqdps audio directory are offered as user sounds after restarting the application. The volume setting applies to all event sounds.\n\n" +
			"Spell icons and available icon sets are extracted from the EverQuest installation associated with the open logfile. If that installation cannot be detected, spell notifications may have no icon.",
	},
	{
		title: "Import and export",
		text:  "Import adds events without removing your current configuration. An imported event is skipped when another event already has the same title. Exported files can be used as backups or transferred to another installation.",
	},
	{
		title: "Troubleshooting",
		text:  "Event matching depends on the exact text written by EverQuest. If an enabled event does not trigger, compare its expected text with the actual logfile message. Server changes, different client languages and catalogue differences can all change the message.",
	},
}

func (m *Module) LayoutHelp(style *ui.Style, gtx layout.Context) layout.Dimensions {
	list := material.List(style.Theme, &m.helpList)
	return list.Layout(gtx, len(eventsHelpSections), func(gtx layout.Context, index int) layout.Dimensions {
		section := eventsHelpSections[index]
		return layout.Inset{Bottom: unit.Dp(24), Right: unit.Dp(16)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			children := make([]layout.FlexChild, 0, 2+len(section.examples))
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
			for _, example := range section.examples {
				example := example
				children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Inset{Top: unit.Dp(12)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								label := ui.ColorLabel(style.Palette.Muted, material.Label(style.Theme, ui.Sp(13), example.caption))
								return layout.Inset{Bottom: unit.Dp(4)}.Layout(gtx, label.Layout)
							}),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								return ui.ColorLabel(style.Palette.Accent, ui.Label(style, example.text)).Layout(gtx)
							}),
						)
					})
				}))
			}
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
		})
	})
}
