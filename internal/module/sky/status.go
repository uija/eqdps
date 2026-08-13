package sky

import (
	"fmt"

	"gioui.org/layout"
	"github.com/uija/eqdps/internal/ui"
)

func (m *Module) LayoutStatus(style *ui.Style, gtx layout.Context) layout.Dimensions {
	num := 0
	for _, cs := range m.status {
		for _, qs := range cs.Quests {
			if !qs.Done && qs.MissingItems == 0 {
				num++
			}
		}
	}
	if num > 0 {
		link := ui.Link(style, &m.status_click, fmt.Sprintf("%d Quest ready", num))
		link.TextColor = style.Palette.Yes
		return link.Layout(gtx)
	} else {
		return layout.Dimensions{}
	}
}
