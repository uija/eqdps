package main

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

const maxRecentLogs = 8

type guiSettings struct {
	LastLogfile    string   `json:"last_logfile,omitempty"`
	RecentLogfiles []string `json:"recent_logfiles,omitempty"`
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
	if err := temporary.Close(); err != nil {
		return err
	}
	return os.Rename(temporaryPath, path)
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
