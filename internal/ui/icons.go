package ui

import (
	"gioui.org/widget"
	"golang.org/x/exp/shiny/materialdesign/icons"
)

var ActionVisibility *widget.Icon
var CheckBox *widget.Icon
var CheckBoxOutline *widget.Icon
var AddBox *widget.Icon
var DelBox *widget.Icon
var Timer *widget.Icon

func Init() {
	ActionVisibility = loadIcon(icons.ActionVisibility)
	CheckBox = loadIcon(icons.ToggleCheckBox)
	CheckBoxOutline = loadIcon(icons.ToggleCheckBoxOutlineBlank)
	AddBox = loadIcon(icons.ContentAddBox)
	DelBox = loadIcon(icons.ToggleIndeterminateCheckBox)
	Timer = loadIcon(icons.ImageTimer)
}
func loadIcon(src []byte) *widget.Icon {
	icon, err := widget.NewIcon(src)
	if err != nil {
		panic("Icon loading failed")
	}
	return icon

}
