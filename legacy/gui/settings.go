package main

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
)

const maxRecentLogs = 8

type guiDefaults struct {
	mainFontScale  float32
	dpsFontScale   float32
	fontMultiplier float32
	mainWidth      int
	mainHeight     int
	overlayWidth   int
	overlayHeight  int
}

type guiSettings struct {
	LastLogfile    string   `json:"last_logfile,omitempty"`
	RecentLogfiles []string `json:"recent_logfiles,omitempty"`
	OverlayVisible bool     `json:"overlay_visible,omitempty"`
	WaylandNotice  bool     `json:"wayland_overlay_notice_shown,omitempty"`
	MainFontScale  float32  `json:"main_font_scale,omitempty"`
	DPSFontScale   float32  `json:"dps_font_scale,omitempty"`
	DPSOpacity     float32  `json:"dps_opacity,omitempty"`
	IdleTimeoutSec int      `json:"idle_timeout_seconds,omitempty"`
	MainWidth      int      `json:"main_width,omitempty"`
	MainHeight     int      `json:"main_height,omitempty"`
	OverlayWidth   int      `json:"overlay_width,omitempty"`
	OverlayHeight  int      `json:"overlay_height,omitempty"`
	OverlayX       int      `json:"overlay_x,omitempty"`
	OverlayY       int      `json:"overlay_y,omitempty"`
	OverlayPlaced  bool     `json:"overlay_placed,omitempty"`
}

func (settings *guiSettings) normalize() {
	settings.normalizeForGOOS(runtime.GOOS)
}

func (settings *guiSettings) normalizeForGOOS(goos string) {
	defaults := settingsDefaultsForGOOS(goos)
	settings.MainFontScale = clampSetting(settings.MainFontScale, .75, 1.5, defaults.mainFontScale)
	settings.DPSFontScale = clampSetting(settings.DPSFontScale, .5, 1.5, defaults.dpsFontScale)
	settings.DPSOpacity = clampSetting(settings.DPSOpacity, .35, 1, .8)
	if settings.IdleTimeoutSec == 0 {
		settings.IdleTimeoutSec = 15
	}
	settings.IdleTimeoutSec = min(max(settings.IdleTimeoutSec, 5), 60)
	settings.MainWidth = normalizedWindowSize(settings.MainWidth, defaults.mainWidth, 720)
	settings.MainHeight = normalizedWindowSize(settings.MainHeight, defaults.mainHeight, 460)
	settings.OverlayWidth = normalizedWindowSize(settings.OverlayWidth, defaults.overlayWidth, 380)
	settings.OverlayHeight = normalizedWindowSize(settings.OverlayHeight, defaults.overlayHeight, 180)
}

func settingsDefaultsForGOOS(goos string) guiDefaults {
	if goos == "windows" {
		return guiDefaults{
			mainFontScale:  .85,
			dpsFontScale:   .8,
			fontMultiplier: .75,
			mainWidth:      1050,
			mainHeight:     700,
			overlayWidth:   430,
			overlayHeight:  240,
		}
	}
	return guiDefaults{
		mainFontScale:  1,
		dpsFontScale:   1,
		fontMultiplier: 1,
		mainWidth:      1050,
		mainHeight:     700,
		overlayWidth:   520,
		overlayHeight:  310,
	}
}

func effectiveFontScale(scale float32) float32 {
	return effectiveFontScaleForGOOS(runtime.GOOS, scale)
}

func effectiveFontScaleForGOOS(goos string, scale float32) float32 {
	return scale * settingsDefaultsForGOOS(goos).fontMultiplier
}

func normalizedWindowSize(value, fallback, minimum int) int {
	if value < minimum {
		return fallback
	}
	return value
}

func clampSetting(value, minimum, maximum, fallback float32) float32 {
	if value == 0 {
		return fallback
	}
	if value < minimum {
		return minimum
	}
	if value > maximum {
		return maximum
	}
	return value
}

func settingsPath() (string, error) {
	directory, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(directory, "eqdps", "gui.json"), nil
}

func loadSettings() (guiSettings, error) {
	path, err := settingsPath()
	if err != nil {
		return guiSettings{}, err
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return guiSettings{}, nil
	}
	if err != nil {
		return guiSettings{}, err
	}
	var settings guiSettings
	if err := json.Unmarshal(data, &settings); err != nil {
		return guiSettings{}, err
	}
	settings.normalize()
	return settings, nil
}

func saveSettings(settings guiSettings) error {
	path, err := settingsPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return err
	}
	temporary, err := os.CreateTemp(filepath.Dir(path), "gui-*.json")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if err := temporary.Chmod(0o600); err != nil {
		temporary.Close()
		return err
	}
	if _, err := temporary.Write(data); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Sync(); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	if err := replaceFile(temporaryPath, path); err != nil {
		return err
	}
	return nil
}

func replaceFile(source, destination string) error {
	if err := os.Rename(source, destination); err != nil {
		if removeErr := os.Remove(destination); removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
			return err
		}
		if renameErr := os.Rename(source, destination); renameErr != nil {
			return renameErr
		}
	}
	return nil
}

func (settings *guiSettings) rememberLog(path string) {
	path = filepath.Clean(path)
	settings.LastLogfile = path
	recent := []string{path}
	for _, candidate := range settings.RecentLogfiles {
		if candidate != path && len(recent) < maxRecentLogs {
			recent = append(recent, candidate)
		}
	}
	settings.RecentLogfiles = recent
}
