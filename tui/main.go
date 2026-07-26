package main

import (
	"fmt"
	"os"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	application := tview.NewApplication()
	application.EnableMouse(true)

	topStatus := tview.NewTextView()
	topStatus.SetBackgroundColor(tcell.NewHexColor(0x1b1e21))
	topStatus.
		SetDynamicColors(true).
		SetText(" [::b]eqdps[::-]  [gray]•  No logfile open[-]").
		SetTextColor(tcell.ColorWhite)

	mainArea := tview.NewTextView()
	mainArea.SetBackgroundColor(tcell.NewHexColor(0x121416))
	mainArea.
		SetDynamicColors(true).
		SetTextAlign(tview.AlignCenter).
		SetText("\n\n[::b]eqdps[::-]\n\n[gray]No modules registered[-]").
		SetTextColor(tcell.ColorWhite)

	hotkeys := tview.NewTextView()
	hotkeys.SetBackgroundColor(tcell.NewHexColor(0x121416))
	hotkeys.
		SetDynamicColors(true).
		SetText(" [yellow]q[-] Quit   [yellow]?[-] Help").
		SetTextColor(tcell.ColorWhite)

	status := tview.NewTextView()
	status.SetBackgroundColor(tcell.NewHexColor(0x1b1e21))
	status.
		SetDynamicColors(true).
		SetText(" Ready").
		SetTextColor(tcell.ColorWhite)

	mainPage := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(topStatus, 1, 0, false).
		AddItem(mainArea, 0, 1, true).
		AddItem(hotkeys, 1, 0, false).
		AddItem(status, 1, 0, false)

	helpPage := tview.NewTextView()
	helpPage.SetBackgroundColor(tcell.NewHexColor(0x121416))
	helpPage.
		SetDynamicColors(true).
		SetTextAlign(tview.AlignCenter).
		SetText("\n[::b]Help[::-]\n\n[gray]No help content available[-]\n\n[yellow]Esc[-] / [yellow]q[-] Back").
		SetTextColor(tcell.ColorWhite)

	pages := tview.NewPages().
		AddPage("main", mainPage, true, true).
		AddPage("help", helpPage, true, false)

	helpOpen := false
	application.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if helpOpen && (event.Rune() == 'q' || event.Key() == tcell.KeyEscape) {
			helpOpen = false
			pages.SwitchToPage("main")
			application.SetFocus(mainArea)
			return nil
		}
		if !helpOpen && event.Rune() == '?' {
			helpOpen = true
			pages.SwitchToPage("help")
			application.SetFocus(helpPage)
			return nil
		}
		if !helpOpen && event.Rune() == 'q' {
			application.Stop()
			return nil
		}
		return event
	})

	return application.SetRoot(pages, true).Run()
}
