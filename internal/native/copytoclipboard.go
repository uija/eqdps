package native

import (
	"io"
	"strings"

	"gioui.org/io/clipboard"
	"gioui.org/layout"
)

func CopyTextToClipboard(gtx layout.Context, text string) {
	gtx.Execute(clipboard.WriteCmd{
		Type: "text/plain",
		Data: io.NopCloser(strings.NewReader(text)),
	})
}
