package page

import (
	"gioui.org/layout"
	"gioui.org/widget"
	"github.com/uija/eqdps/internal/ui"
)

type StatsPage interface {
	Title() string
	Clickable() *widget.Clickable
	Layout(*ui.Style, layout.Context) layout.Dimensions
}
