package statistics

import (
	"database/sql"

	"gioui.org/layout"
	"gioui.org/widget"
	"github.com/uija/eqdps/internal/ui"
)

const ROW_PADDING = 8

type StatsPage interface {
	Title() string
	Clickable() *widget.Clickable
	Layout(*ui.Style, layout.Context) layout.Dimensions
	SetDb(db *sql.DB)
	Update(layout.Context)
	Reset()
}
