package sky

import (
	"fmt"
	"time"

	"gioui.org/layout"
	"gioui.org/unit"
	"github.com/uija/eqdps/internal/ui"
)

func (m *Module) LayoutStatus(style *ui.Style, gtx layout.Context) layout.Dimensions {
	if m.notification != nil {
		if time.Now().After(m.notification.Ends) {
			m.notification = nil
		} else {
			link := ui.Link(style, &m.status_click, m.notification.Text)
			link.TextColor = style.Palette.Window
			return ui.ColoredRow(gtx, style.Palette.Yes, func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Left: unit.Dp(16), Right: unit.Dp(16)}.Layout(gtx, link.Layout)
			})
		}
	}
	num := 0
	for _, cs := range m.status {
		for _, qs := range cs.Quests {
			if !qs.Done && qs.MissingItems == 0 {
				num++
			}
		}
	}
	link := ui.Link(style, &m.status_click, fmt.Sprintf("%d Quest ready", num))
	link.TextColor = style.Palette.Yes
	return link.Layout(gtx)
}
