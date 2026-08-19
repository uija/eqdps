package eqldb

import (
	"image/color"

	"github.com/uija/eqdps/internal/ui"
)

func (m *Module) GeneralStatus(style *ui.Style) (string, color.NRGBA) {
	if m.upload_status >= 0 {
		switch m.upload_status {
		case upload_detected:
			return "Inventory file detected", style.Palette.Yes
		case upload_uploading:
			return "Uploading inventory file...", style.Palette.Yes
		case upload_success:
			return "Upload success!", style.Palette.Yes
		case upload_error:
			return "Upload failed!", style.Palette.No
		default:
		}
	}
	return "", style.Palette.Yes
}
