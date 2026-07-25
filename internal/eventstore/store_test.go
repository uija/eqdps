package eventstore

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/uija/eqdps/internal/event"
)

func TestEventRoundTripAndDirectories(t *testing.T) {
	store, err := OpenAt(filepath.Join(t.TempDir(), "eqdps"))
	if err != nil {
		t.Fatal(err)
	}
	want := []event.Event{{
		ID: "one", Title: "Ready", Active: true, TriggerType: event.TriggerText, Pattern: "ready",
	}}
	if err := store.Save(want); err != nil {
		t.Fatal(err)
	}
	got, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].ID != want[0].ID {
		t.Fatalf("loaded events = %#v", got)
	}
	for _, directory := range []string{store.AudioDir(), store.IconDir()} {
		if info, err := os.Stat(directory); err != nil || !info.IsDir() {
			t.Fatalf("configuration directory %q is unavailable: %v", directory, err)
		}
	}
}

func TestSpellIconSetupRoundTrip(t *testing.T) {
	store, err := OpenAt(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	state, err := store.SpellIconSetup()
	if err != nil {
		t.Fatal(err)
	}
	if state != IconSetupUnknown {
		t.Fatalf("initial state = %q", state)
	}
	if err := store.SaveSpellIconSetup(IconSetupDeclined); err != nil {
		t.Fatal(err)
	}
	state, err = store.SpellIconSetup()
	if err != nil {
		t.Fatal(err)
	}
	if state != IconSetupDeclined {
		t.Fatalf("saved state = %q", state)
	}
}

func TestSpellIconSetRoundTripAndDiscovery(t *testing.T) {
	store, err := OpenAt(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"default_modern", "default"} {
		directory := filepath.Join(store.IconDir(), name)
		if err := os.MkdirAll(directory, 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(directory, "spell_4.png"), []byte("png"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	sets, err := store.SpellIconSets()
	if err != nil {
		t.Fatal(err)
	}
	if len(sets) != 2 || sets[0] != "default" || sets[1] != "default_modern" {
		t.Fatalf("spell icon sets = %#v", sets)
	}
	if err := store.SaveSpellIconSet("default_modern"); err != nil {
		t.Fatal(err)
	}
	selected, err := store.SpellIconSet()
	if err != nil {
		t.Fatal(err)
	}
	if selected != "default_modern" {
		t.Fatalf("selected spell icon set = %q", selected)
	}
	if err := store.SaveSpellIconSet("../outside"); err == nil {
		t.Fatal("unsafe spell icon set name was accepted")
	}
}

func TestAudioVolumeDefaultsAndRoundTrips(t *testing.T) {
	store, err := OpenAt(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	volume, err := store.AudioVolume()
	if err != nil {
		t.Fatal(err)
	}
	if volume != 1 {
		t.Fatalf("initial audio volume = %v, want 1", volume)
	}
	if err := store.SaveAudioVolume(.35); err != nil {
		t.Fatal(err)
	}
	volume, err = store.AudioVolume()
	if err != nil {
		t.Fatal(err)
	}
	if volume != .35 {
		t.Fatalf("saved audio volume = %v, want .35", volume)
	}
}

func TestAudioVolumeRejectsOutOfRangeValues(t *testing.T) {
	store, err := OpenAt(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	for _, volume := range []float64{-.01, 1.01} {
		if err := store.SaveAudioVolume(volume); err == nil {
			t.Fatalf("SaveAudioVolume(%v) succeeded", volume)
		}
	}
}

func TestLoadRejectsInvalidConfiguration(t *testing.T) {
	store, err := OpenAt(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(store.eventFile, []byte(`[{"id":"bad","title":"Bad","active":true,"trigger_type":"regexp","pattern":"["}]`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Load(); err == nil {
		t.Fatal("expected invalid configuration error")
	}
}

func TestSaveRefusesConcurrentWriter(t *testing.T) {
	store, err := OpenAt(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(store.lockFile, []byte("another process\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	eventConfig := []event.Event{{
		ID: "one", Title: "Ready", Active: true, TriggerType: event.TriggerText, Pattern: "ready",
	}}
	if err := store.Save(eventConfig); err == nil {
		t.Fatal("expected busy configuration error")
	}
}
