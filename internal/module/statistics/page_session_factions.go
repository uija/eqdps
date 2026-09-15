package statistics

import (
	"gioui.org/layout"
	"github.com/uija/eqdps/internal/ui"
)

func (p *SessionsPage) factionDetailsRows(factions []SessionFactionDetails, style *ui.Style) []layout.FlexChild {
	if len(factions) == 0 {
		return nil
	}
	children := []layout.FlexChild{
		layout.Rigid(titleRow("Faction changes", style)),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return ui.ColoredRow(gtx, style.Palette.HeadlineBGMuted, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{}.Layout(gtx,
					sessionTextCell(4, "Faction", false, style),
					sessionTextCell(1, "Gained", true, style),
					sessionTextCell(1, "Lost", true, style),
					sessionTextCell(1, "Net change", true, style),
				)
			})
		}),
	}
	format := func(value int64) string {
		if value > 0 {
			return "+" + p.ctx.Sprintf("%d", value)
		}
		return p.ctx.Sprintf("%d", value)
	}
	for index, faction := range factions {
		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return statisticsDetailsRow(index, style, gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{}.Layout(gtx,
					sessionTextCell(4, faction.Name, false, style),
					sessionTextCell(1, format(faction.Gained), true, style),
					sessionTextCell(1, format(faction.Lost), true, style),
					sessionTextCell(1, format(faction.NetChange()), true, style))
			})
		}))
	}
	return children
}
