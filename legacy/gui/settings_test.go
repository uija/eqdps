package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestOverlayVisibilityRoundTripsThroughSettingsJSON(t *testing.T) {
	settings := guiSettings{OverlayVisible: true, OverlayX: 100, OverlayY: 200, OverlayPlaced: true}
	data, err := json.Marshal(settings)
	if err != nil {
		t.Fatal(err)
	}
	var decoded guiSettings
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	if !decoded.OverlayVisible {
		t.Fatal("expected overlay visibility to survive settings round trip")
	}
	if !decoded.OverlayPlaced || decoded.OverlayX != 100 || decoded.OverlayY != 200 {
		t.Fatalf("expected overlay position to survive settings round trip: %#v", decoded)
	}
}

func TestSettingsNormalizationAddsPreferenceDefaults(t *testing.T) {
	settings := guiSettings{}
	settings.normalizeForGOOS("linux")
	if settings.MainFontScale != 1 || settings.DPSFontScale != 1 || settings.DPSOpacity != .8 || settings.IdleTimeoutSec != 15 {
		t.Fatalf("unexpected preference defaults: %#v", settings)
	}
	if settings.MainWidth != 1050 || settings.MainHeight != 700 || settings.OverlayWidth != 520 || settings.OverlayHeight != 310 {
		t.Fatalf("unexpected window size defaults: %#v", settings)
	}
}

func TestSettingsNormalizationUsesSmallerWindowsDefaults(t *testing.T) {
	settings := guiSettings{}
	settings.normalizeForGOOS("windows")
	if settings.MainFontScale != .85 || settings.DPSFontScale != .8 {
		t.Fatalf("unexpected Windows font defaults: %#v", settings)
	}
	if settings.MainWidth != 1050 || settings.MainHeight != 700 || settings.OverlayWidth != 430 || settings.OverlayHeight != 240 {
		t.Fatalf("unexpected Windows window size defaults: %#v", settings)
	}
}

func TestEffectiveFontScaleShrinksWindowsRendering(t *testing.T) {
	if got := effectiveFontScaleForGOOS("linux", 1); got != 1 {
		t.Fatalf("expected Linux 100%% setting to render at scale 1, got %v", got)
	}
	if got := effectiveFontScaleForGOOS("windows", 1); got != .75 {
		t.Fatalf("expected Windows 100%% setting to render smaller, got %v", got)
	}
}

func TestSettingsNormalizationClampsIdleTimeoutAndPreservesWindowSizes(t *testing.T) {
	settings := guiSettings{IdleTimeoutSec: 100, MainWidth: 1200, MainHeight: 800, OverlayWidth: 600, OverlayHeight: 400}
	settings.normalizeForGOOS("windows")
	if settings.IdleTimeoutSec != 60 {
		t.Fatalf("expected timeout clamp, got %d", settings.IdleTimeoutSec)
	}
	if settings.MainWidth != 1200 || settings.MainHeight != 800 || settings.OverlayWidth != 600 || settings.OverlayHeight != 400 {
		t.Fatalf("expected saved window sizes, got %#v", settings)
	}
}

func TestSettingsNormalizationClampsPreferenceRanges(t *testing.T) {
	settings := guiSettings{MainFontScale: .1, DPSFontScale: 3, DPSOpacity: .1}
	settings.normalizeForGOOS("windows")
	if settings.MainFontScale != .75 || settings.DPSFontScale != 1.5 || settings.DPSOpacity != .35 {
		t.Fatalf("unexpected clamped preferences: %#v", settings)
	}
}

func TestOverlayFontScaleAllowsFiftyPercent(t *testing.T) {
	settings := guiSettings{DPSFontScale: .5}
	settings.normalizeForGOOS("windows")
	if settings.DPSFontScale != .5 {
		t.Fatalf("expected 50%% overlay scale, got %v", settings.DPSFontScale)
	}
}

func TestReplaceFileOverwritesExistingDestination(t *testing.T) {
	directory := t.TempDir()
	source := filepath.Join(directory, "source.json")
	destination := filepath.Join(directory, "gui.json")
	if err := os.WriteFile(source, []byte(`{"new":true}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(destination, []byte(`{"old":true}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := replaceFile(source, destination); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(destination)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != `{"new":true}` {
		t.Fatalf("unexpected destination content: %s", data)
	}
	if _, err := os.Stat(source); !os.IsNotExist(err) {
		t.Fatalf("expected source to be consumed, got %v", err)
	}
}

func TestReplaceFileCreatesNewDestination(t *testing.T) {
	directory := t.TempDir()
	source := filepath.Join(directory, "source.json")
	destination := filepath.Join(directory, "gui.json")
	if err := os.WriteFile(source, []byte(`{"new":true}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := replaceFile(source, destination); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(destination); err != nil {
		t.Fatal(err)
	}
}

func TestRememberLogMovesPathToFrontWithoutDuplicates(t *testing.T) {
	one := filepath.Join("root", "one")
	two := filepath.Join("root", "two")
	three := filepath.Join("root", "three")
	settings := guiSettings{RecentLogfiles: []string{one, two, three}}
	settings.rememberLog(two)
	if settings.LastLogfile != two || len(settings.RecentLogfiles) != 3 || settings.RecentLogfiles[0] != two || settings.RecentLogfiles[1] != one {
		t.Fatalf("unexpected settings: %#v", settings)
	}
}

func TestRememberLogLimitsRecentPaths(t *testing.T) {
	settings := guiSettings{RecentLogfiles: []string{"1", "2", "3", "4", "5", "6", "7", "8"}}
	settings.rememberLog("new")
	if len(settings.RecentLogfiles) != maxRecentLogs || settings.RecentLogfiles[0] != "new" {
		t.Fatalf("unexpected recent paths: %#v", settings.RecentLogfiles)
	}
}
