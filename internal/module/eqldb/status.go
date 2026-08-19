package eqldb

import (
	"gioui.org/layout"
	"gioui.org/unit"
	"github.com/uija/eqdps/internal/ui"
)

func (m *Module) LayoutStatus(style *ui.Style, gtx layout.Context) layout.Dimensions {
	if m.upload_status >= 0 {
		switch m.upload_status {
		case upload_detected:
			return layout.Inset{Left: unit.Dp(16)}.Layout(gtx, ui.ColoredLabel(style.Theme, 15, style.Palette.Yes, "Inventory file detected").Layout)
		case upload_uploading:
			return layout.Inset{Left: unit.Dp(16)}.Layout(gtx, ui.ColoredLabel(style.Theme, 15, style.Palette.Yes, "Uploading inventory file...").Layout)
		case upload_success:
			return layout.Inset{Left: unit.Dp(16)}.Layout(gtx, ui.ColoredLabel(style.Theme, 15, style.Palette.Yes, "Upload success!").Layout)
		case upload_error:
			return layout.Inset{Left: unit.Dp(16)}.Layout(gtx, ui.ColoredLabel(style.Theme, 15, style.Palette.No, "Upload failed!").Layout)
		default:
		}
	}
	return layout.Dimensions{}
}
