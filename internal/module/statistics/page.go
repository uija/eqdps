package statistics

import (
	"database/sql"

	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget"
	"github.com/uija/eqdps/internal/ui"
)

const ROW_PADDING = 8

type StatsPage interface {
	Title() string
	GetIcon() *widget.Icon
	Clickable() *widget.Clickable
	Layout(*ui.Style, layout.Context) layout.Dimensions
	SetDb(db *sql.DB)
	Update(layout.Context)
	Reset()
}

func statisticsDetailsLayout(style *ui.Style, gtx layout.Context, content layout.Widget) layout.Dimensions {
	border := widget.Border{Color: style.Palette.Border, Width: unit.Dp(1)}
	return border.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.UniformInset(unit.Dp(16)).Layout(gtx, content)
	})
}

func statisticsDetailsRow(index int, style *ui.Style, gtx layout.Context, content layout.Widget) layout.Dimensions {
	color := style.Palette.Window
	if index%2 != 0 {
		color = style.Palette.Panel
	}
	return ui.ColoredRow(gtx, color, content)
}
