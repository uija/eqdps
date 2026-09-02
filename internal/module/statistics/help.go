package statistics

import (
	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget/material"
	"github.com/uija/eqdps/internal/ui"
)

var statisticsHelpSections = []struct {
	title string
	text  string
}{
	{
		title: "Updating the database",
		text: "Statistics are not updated continuously. Update processes only the logfile data added since the previous import; the displayed byte count shows how far behind the database is.\n\n" +
			"Full reload deletes the collected statistics and rebuilds them from the entire logfile. Use it after an interrupted or incorrect import, or when a newer version collects additional data. Anything not present in the logfile cannot be recovered by a reload.",
	},
	{
		title: "What the totals mean",
		text: "Item totals include quantities, so a stack of three counts as three items. Automatically sold loot still counts as looted.\n\n" +
			"Experience is the sum of the percentages reported by the game, rather than an absolute number of experience points. Money combines the platinum, gold, silver and copper amounts recognized in the logfile.\n\n" +
			"Zone difficulties and group variants are combined under their base zone in the overview and Zones page. Item upgrade suffixes are likewise combined, so Rusty Dagger and Rusty Dagger +3 share one entry.",
	},
	{
		title: "Kills and loot association",
		text: "A kill is counted when the logfile provides enough evidence that your character participated, such as the killing blow, corpse money or loot. Nearby kills belonging only to other players are excluded when they cannot be confirmed.\n\n" +
			"Coin and loot messages do not always identify a unique corpse. When several recently killed mobs have the same name, eqdps associates them using their order and timing. Unusual loot sequences can therefore occasionally be assigned to the wrong kill.",
	},
	{
		title: "Drop chances and Mote rates",
		text: "Drop chance is the percentage of recorded kills on which an item appeared at least once. Multiple copies from one kill increase the Drops total but still count as one successful kill for the percentage.\n\n" +
			"The Mote %/kill value similarly counts kills that produced at least one Mote. Motes/h uses the estimated time spent in the zone. All rates can be misleading when based on only a small number of kills or a short visit.",
	},
	{
		title: "Sessions",
		text: "A session must last at least one minute and contain a kill credited directly to your character. Other confirmed kills seen during that period are then included in its totals. Consecutive entries into the same exact zone, such as returning after an evacuation, are combined when there is no gap.\n\n" +
			"Session rates use the complete estimated duration, including downtime spent in that zone. Expanding a session shows the mobs, grouped loot, deaths and money that could be associated with its time range.",
	},
	{
		title: "Time estimates",
		text:  "EverQuest does not continuously record whether you are online. eqdps estimates visit boundaries from zoning, login, camping and nearby logfile activity. Normal logouts are usually excluded, but crashes, disconnects, incomplete logs and long silent periods can make zone and session durations less precise.",
	},
}

func (m *Module) LayoutHelp(style *ui.Style, gtx layout.Context) layout.Dimensions {
	list := material.List(style.Theme, &m.helpList)
	return list.Layout(gtx, len(statisticsHelpSections), func(gtx layout.Context, index int) layout.Dimensions {
		section := statisticsHelpSections[index]
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
