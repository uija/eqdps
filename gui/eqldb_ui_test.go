package main

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"gioui.org/layout"
	"github.com/uija/eqdps/internal/eqldb"
	"github.com/uija/eqdps/internal/inventorysync"
)

func TestEQLDBGUISyncErrorRefreshesClearedConnection(t *testing.T) {
	store := eqldb.Store{Path: filepath.Join(t.TempDir(), "eqldb.json")}
	if err := store.Save(eqldb.State{IntroductionShown: true}); err != nil {
		t.Fatal(err)
	}
	ui := &eqldbGUI{
		store: store,
		state: eqldb.State{IntroductionShown: true, AccessToken: "stale-token"},
	}

	ui.handleSyncError(&eqldb.APIError{Status: 401, Description: "token revoked"})

	if ui.state.AccessToken != "" {
		t.Fatalf("stale access token remained in UI state: %q", ui.state.AccessToken)
	}
	if ui.lastError != "token revoked" {
		t.Fatalf("last error = %q", ui.lastError)
	}
}

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

func TestInventoryExportCallbackRunsWithoutEQLDBConnection(t *testing.T) {
	var received inventorysync.Request
	ui := &eqldbGUI{
		context: context.Background(),
		events:  make(chan eqldbGUIEvent, 1),
		inventoryExport: func(request inventorysync.Request) {
			received = request
		},
	}
	want := inventorysync.Request{Path: "inventory.txt"}
	ui.events <- eqldbGUIEvent{kind: eqldbGUIExportDetected, request: want}

	ui.processEvents()

	if received.Path != want.Path {
		t.Fatalf("callback request = %#v", received)
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
