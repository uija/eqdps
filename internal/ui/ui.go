package ui

import "gioui.org/layout"

type Widget func(*Style, layout.Context) layout.Dimensions
