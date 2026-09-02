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
		text: "Events let eqdps watch your EverQuest logfile and alert you when something important happens. An event can display a desktop notification, play a sound, or do both.\n\nEvents only react to new live logfile messages. They do not trigger while an older part of the logfile is being replayed.",
	},
	{
		title: "Event list",
		text:  "The Events page shows all configured events and their type, notification status and selected sound.\n\nUse the icon beside an event to enable or disable it. A disabled event remains saved but does not produce notifications or sounds.\n\nClick an event’s name to edit or delete it.",
	},
	{
		title: "Spell events",
		text: "A Spell event notifies you when a selected spell effect fades.\n\nSelect the class and spell from the available spell catalogue. You can choose whose fade message should be detected:\n\n" +
			"• Self — Detect the message when the effect fades from you.\n" +
			"• Others — Detect the message when the effect fades from another player or target.\n" +
			"• Both — Detect either message.\n\n" +
			"Spell events use the known fade messages from the spell catalogue, so you do not need to enter the logfile text yourself.",
	},
	{
		title: "Timer events",
		text:  "A Timer event starts when the logfile reports that you begin casting the selected spell.\n\nEnter the timer duration in seconds. When the timer expires, eqdps displays the configured notification and/or plays the selected sound.\n\nRunning timers are also shown in the DPS overlay when it is open.\n\nTimers use the configured duration rather than checking for an actual fade message. Recasting the same configured spell restarts its timer.",
	},
	{
		title: "Text events",
		text:  "A Text event triggers when a logfile message contains the configured text. The example below matches any logfile message containing those words.\n\nEnable Full message match if the entire message must match. Timestamps are not part of the message being checked.\n\nText matching is case-sensitive unless Full message match is enabled.",
		examples: []eventsHelpExample{
			{caption: "Example text", text: "Your target resisted"},
		},
	},
	{
		title: "Regular-expression events",
		text:  "Regular-expression events provide more flexible matching for users familiar with regular expressions. The example below matches ordinary personal-loot messages.\n\nUse the validation control before saving to check whether the expression is valid. An invalid regular expression cannot trigger an event.\n\nRegular expressions are matched against the logfile message without its timestamp.",
		examples: []eventsHelpExample{
			{caption: "Example regular expression", text: "^You have looted .+ from .+'s corpse\\.$"},
		},
	},
	{
		title: "Notifications",
		text:  "The Title is used as the title of the desktop notification.\n\nThe Notification field contains its message. You can use %s as a placeholder for the event title. The examples below show the notification text and its result for an event titled Clarity.\n\nLeave the Notification field empty if the event should only play a sound.\n\nDesktop notifications are displayed by your operating system and may look or behave differently depending on your desktop environment. The persistent-notification option is saved with the event, but notification lifetime is ultimately controlled by the operating system.",
		examples: []eventsHelpExample{
			{caption: "Notification text", text: "%s faded."},
			{caption: "Displayed message", text: "Clarity faded."},
		},
	},
	{
		title: "Sounds",
		text:  "Select a sound to play when the event triggers. Use the Play button while editing an event to preview it.\n\nThe Notification Volume control at the top of the Events page changes the volume for all event sounds.\n\nLeave the Sound selection empty if the event should only display a notification.",
	},
	{
		title: "Spell icons",
		text:  "Spell notifications can use the icon belonging to the selected spell. If EverQuest is installed in the expected location, eqdps detects compatible spell-icon sets from the game’s UI folders.\n\nUse Spell Icon Set to choose which available EverQuest UI style should be used for notifications.",
	},
	{
		title: "Import and export",
		text:  "Use Export Events to save your event configuration to a JSON file. This is useful for backups, moving events to another computer, or sharing them with another player.\n\nUse Import Events to add events from an exported file. Events whose title already exists are skipped, while new events are added to the current configuration.\n\nImporting does not remove your existing events.",
	},
	{
		title: "Notes",
		text:  "Events depend on the exact text written to the EverQuest logfile. A language change, different server message, or modified spell message can prevent an event from matching.\n\nIf an event does not trigger, first verify that it is enabled and that the expected message appears in the logfile.",
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
				label := material.Label(style.Theme, ui.Sp(15), section.text)
				label.Color = style.Palette.Muted
				return label.Layout(gtx)
			}))
			for _, example := range section.examples {
				example := example
				children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Inset{Top: unit.Dp(12)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								label := material.Label(style.Theme, ui.Sp(13), example.caption)
								label.Color = style.Palette.Muted
								return layout.Inset{Bottom: unit.Dp(4)}.Layout(gtx, label.Layout)
							}),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								label := material.Label(style.Theme, ui.Sp(15), example.text)
								label.Color = style.Palette.Accent
								return label.Layout(gtx)
							}),
						)
					})
				}))
			}
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
		})
	})
}
