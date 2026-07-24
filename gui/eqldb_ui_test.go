package main

import (
	"strings"
	"testing"

	"gioui.org/layout"
	"github.com/uija/eqdps/internal/eqldb"
	"github.com/uija/eqdps/internal/inventorysync"
)

func TestEQLDBGUIMacroGuidanceUsesCharacter(t *testing.T) {
	macro := eqldbGUIMacroText("Wyrmberg")
	for _, expected := range []string{"/who Wyrmberg", "/outputfile inventory"} {
		if !strings.Contains(macro, expected) {
			t.Fatalf("macro %q does not contain %q", macro, expected)
		}
	}
	if !strings.Contains(eqldbGUIMacroExplanation, "level, race, and classes") {
		t.Fatalf("unexpected explanation: %q", eqldbGUIMacroExplanation)
	}
}

func TestEQLDBGUIExportWithoutWhoReplacesOpenDialog(t *testing.T) {
	ui := &eqldbGUI{
		state:         eqldb.State{AccessToken: "token"},
		modal:         "manage",
		pendingExport: &inventorysync.Request{Path: "inventory.txt"},
	}

	ui.processPendingExport()

	if ui.modal != "metadata" {
		t.Fatalf("expected metadata dialog, got %q", ui.modal)
	}
	if ui.pendingExport != nil {
		t.Fatal("pending export was not consumed")
	}
	if ui.levelEditor.Text() != "" {
		t.Fatalf("default level = %q, want an empty field", ui.levelEditor.Text())
	}
}

func TestMetadataPickerOpensTowardAvailableViewportSpace(t *testing.T) {
	tests := []struct {
		name      string
		fieldItem int
		wantAbove bool
		wantFirst int
	}{
		{name: "field near top opens below", fieldItem: 2, wantAbove: false, wantFirst: 2},
		{name: "field near bottom opens above", fieldItem: 6, wantAbove: true, wantFirst: 6},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ui := &eqldbGUI{
				classPicker:    -1,
				pickerItem:     -1,
				metadataFields: [4]int{test.fieldItem},
			}
			ui.metadataList.Position = layout.Position{First: 1, Count: 6}

			ui.openMetadataPicker(-2, 0)

			if ui.pickerAbove != test.wantAbove {
				t.Fatalf("pickerAbove = %v, want %v", ui.pickerAbove, test.wantAbove)
			}
			if ui.metadataList.Position.First != test.wantFirst {
				t.Fatalf("viewport starts at %d, want %d", ui.metadataList.Position.First, test.wantFirst)
			}
		})
	}
}
