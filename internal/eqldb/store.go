package eqldb

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

type State struct {
	IntroductionShown bool   `json:"introduction_shown,omitempty"`
	AccessToken       string `json:"access_token,omitempty"`
	ConnectionID      string `json:"connection_id,omitempty"`
}

type Store struct {
	Path string
}

func (s Store) AcquireUploadLease(now time.Time, duration time.Duration) (bool, time.Duration, error) {
	lockPath := filepath.Join(filepath.Dir(s.Path), "eqldb-upload.lock")
	if err := os.MkdirAll(filepath.Dir(lockPath), 0o700); err != nil {
		return false, 0, fmt.Errorf("create EQLDB settings directory: %w", err)
	}
	for attempt := 0; attempt < 2; attempt++ {
		file, err := os.OpenFile(lockPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
		if err == nil {
			if _, writeErr := fmt.Fprintf(file, "%d\n", now.Unix()); writeErr != nil {
				file.Close()
				os.Remove(lockPath)
				return false, 0, fmt.Errorf("write EQLDB upload lease: %w", writeErr)
			}
			if closeErr := file.Close(); closeErr != nil {
				os.Remove(lockPath)
				return false, 0, fmt.Errorf("close EQLDB upload lease: %w", closeErr)
			}
			return true, duration, nil
		}
		if !errors.Is(err, os.ErrExist) {
			return false, 0, fmt.Errorf("create EQLDB upload lease: %w", err)
		}
		info, statErr := os.Stat(lockPath)
		if statErr != nil {
			if errors.Is(statErr, os.ErrNotExist) {
				continue
			}
			return false, 0, fmt.Errorf("inspect EQLDB upload lease: %w", statErr)
		}
		remaining := duration - now.Sub(info.ModTime())
		if remaining > 0 {
			return false, remaining, nil
		}
		if removeErr := os.Remove(lockPath); removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
			return false, 0, fmt.Errorf("replace expired EQLDB upload lease: %w", removeErr)
		}
	}
	return false, duration, nil
}

func DefaultStore() (Store, error) {
	directory, err := os.UserConfigDir()
	if err != nil {
		return Store{}, fmt.Errorf("find user config directory: %w", err)
	}
	return Store{Path: filepath.Join(directory, "eqdps", "eqldb.json")}, nil
}

func (s Store) Load() (State, error) {
	data, err := os.ReadFile(s.Path)
	if errors.Is(err, os.ErrNotExist) {
		return State{}, nil
	}
	if err != nil {
		return State{}, fmt.Errorf("read EQLDB settings: %w", err)
	}
	var state State
	if err := json.Unmarshal(data, &state); err != nil {
		return State{}, fmt.Errorf("decode EQLDB settings: %w", err)
	}
	return state, nil
}

func (s Store) Save(state State) error {
	if err := os.MkdirAll(filepath.Dir(s.Path), 0o700); err != nil {
		return fmt.Errorf("create EQLDB settings directory: %w", err)
	}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("encode EQLDB settings: %w", err)
	}
	temporary, err := os.CreateTemp(filepath.Dir(s.Path), "eqldb-*.json")
	if err != nil {
		return fmt.Errorf("create temporary EQLDB settings: %w", err)
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if err := temporary.Chmod(0o600); err != nil {
		temporary.Close()
		return fmt.Errorf("protect temporary EQLDB settings: %w", err)
	}
	if _, err := temporary.Write(data); err != nil {
		temporary.Close()
		return fmt.Errorf("write EQLDB settings: %w", err)
	}
	if err := temporary.Sync(); err != nil {
		temporary.Close()
		return fmt.Errorf("sync EQLDB settings: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("close EQLDB settings: %w", err)
	}
	if err := replaceFile(temporaryPath, s.Path); err != nil {
		return fmt.Errorf("replace EQLDB settings: %w", err)
	}
	return nil
}

func replaceFile(source, destination string) error {
	if err := os.Rename(source, destination); err != nil {
		if removeErr := os.Remove(destination); removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
			return err
		}
		return os.Rename(source, destination)
	}
	return nil
}
