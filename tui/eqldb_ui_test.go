package main

import (
	"strings"
	"testing"

	"github.com/rivo/tview"
	"github.com/uija/eqdps/internal/eqldb"
	"github.com/uija/eqdps/internal/inventorysync"
)

func TestEQLDBMacroGuidanceUsesCharacter(t *testing.T) {
	macro := eqldbMacroText("Wyrmberg")
	for _, expected := range []string{"/who Wyrmberg", "/outputfile inventory"} {
		if !strings.Contains(macro, expected) {
			t.Fatalf("macro %q does not contain %q", macro, expected)
		}
	}
	if !strings.Contains(eqldbMacroExplanation, "level, race, and classes") {
		t.Fatalf("unexpected explanation: %q", eqldbMacroExplanation)
	}
}

func TestEQLDBExportWithoutWhoReplacesOpenDialog(t *testing.T) {
	app := tview.NewApplication()
	main := tview.NewBox()
	pages := tview.NewPages().
		AddPage("main", main, true, true).
		AddPage("eqldb-manage", tview.NewBox(), true, true)
	ui := &eqldbTUI{
		app:           app,
		pages:         pages,
		mainFocus:     main,
		state:         eqldb.State{AccessToken: "token"},
		modal:         "eqldb-manage",
		character:     "Wyrmberg",
		pendingExport: &inventorysync.Request{Path: "inventory.txt"},
	}

	ui.processPendingExport()

	if ui.modal != "eqldb-metadata" {
		t.Fatalf("expected metadata dialog, got %q", ui.modal)
	}
	if ui.pendingExport != nil {
		t.Fatal("pending export was not consumed")
	}
}

func TestEQLDBIntroductionInteractionStopsCountdown(t *testing.T) {
	button := tview.NewButton("Close (20s)")
	ui := &eqldbTUI{
		modal:      "eqldb-intro",
		introTimer: true,
		introClose: button,
	}
	ui.stopIntroductionTimer()
	if ui.introTimer {
		t.Fatal("introduction timer remained active")
	}
	if button.GetLabel() != "Close" {
		t.Fatalf("unexpected close label: %q", button.GetLabel())
	}
}
