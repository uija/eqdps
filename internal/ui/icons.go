package ui

import (
	"gioui.org/widget"
	"golang.org/x/exp/shiny/materialdesign/icons"
)

var ActionVisibility *widget.Icon
var ActionVisibilityOff *widget.Icon
var CheckBox *widget.Icon
var CheckBoxOutline *widget.Icon
var AddBox *widget.Icon
var DelBox *widget.Icon
var Timer *widget.Icon
var Help *widget.Icon
var Priority *widget.Icon
var Arrows *widget.Icon

var NoData *widget.Icon
var Replay *widget.Icon
var Live *widget.Icon

var Book *widget.Icon
var Clock *widget.Icon
var Text *widget.Icon
var RegExp *widget.Icon

var Open *widget.Icon
var Close *widget.Icon
var Check *widget.Icon

var Exclamation *widget.Icon

var Refresh *widget.Icon
var Download *widget.Icon

var Import *widget.Icon
var Export *widget.Icon
var Sort *widget.Icon

var StatisticsOverview *widget.Icon
var StatisticsZones *widget.Icon
var StatisticsSessions *widget.Icon
var StatisticsMobs *widget.Icon
var StatisticsItems *widget.Icon

var Copy *widget.Icon

func Init() {
	ActionVisibility = loadIcon(icons.ActionVisibility)
	ActionVisibilityOff = loadIcon(icons.ActionVisibilityOff)
	CheckBox = loadIcon(icons.ToggleCheckBox)
	CheckBoxOutline = loadIcon(icons.ToggleCheckBoxOutlineBlank)
	AddBox = loadIcon(icons.ContentAddBox)
	DelBox = loadIcon(icons.ToggleIndeterminateCheckBox)
	Timer = loadIcon(icons.ImageTimer)
	Help = loadIcon(icons.ActionHelpOutline)
	Priority = loadIcon(icons.NotificationPriorityHigh)
	Arrows = loadIcon(icons.ActionOpenWith)

	Replay = loadIcon(icons.AVFastForward)
	Live = loadIcon(icons.HardwareKeyboardArrowRight)
	NoData = loadIcon(icons.NavigationClose)

	Book = loadIcon(icons.AVLibraryBooks)
	Clock = loadIcon(icons.ImageTimer10)
	Text = loadIcon(icons.EditorTextFields)
	RegExp = loadIcon(icons.ActionSearch)
	Open = loadIcon(icons.FileFolderOpen)
	Close = loadIcon(icons.NavigationClose)
	Check = loadIcon(icons.NavigationCheck)
	Exclamation = loadIcon(icons.NotificationPriorityHigh)
	Refresh = loadIcon(icons.NavigationRefresh)
	Download = loadIcon(icons.FileFileDownload)

	Import = loadIcon(icons.FileFolderOpen)
	Export = loadIcon(icons.ContentSave)
	Sort = loadIcon(icons.ContentSort)

	StatisticsOverview = loadIcon(icons.ActionDashboard)
	StatisticsZones = loadIcon(icons.MapsMap)
	StatisticsSessions = loadIcon(icons.ActionHistory)
	StatisticsMobs = loadIcon(icons.ActionPets)
	StatisticsItems = loadIcon(icons.ActionLoyalty)

	Copy = loadIcon(icons.ContentContentCopy)
}
func loadIcon(src []byte) *widget.Icon {
	icon, err := widget.NewIcon(src)
	if err != nil {
		panic("Icon loading failed")
	}
	return icon

}
