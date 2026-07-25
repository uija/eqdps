package eventstore

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/uija/eqdps/internal/event"
)

type IconSetup string

const (
	IconSetupUnknown  IconSetup = ""
	IconSetupEnabled  IconSetup = "enabled"
	IconSetupDeclined IconSetup = "declined"
)

type Store struct {
	configDir    string
	eventFile    string
	settingsFile string
	audioDir     string
	iconDir      string
	lockFile     string
}

func Open() (*Store, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return nil, fmt.Errorf("find user config directory: %w", err)
	}
	return OpenAt(filepath.Join(base, "eqdps"))
}

func OpenAt(configDir string) (*Store, error) {
	audioDir := filepath.Join(configDir, "audio")
	iconDir := filepath.Join(configDir, "spell-icons")
	for _, directory := range []string{configDir, audioDir, iconDir} {
		if err := os.MkdirAll(directory, 0o700); err != nil {
			return nil, fmt.Errorf("create event configuration directory: %w", err)
		}
	}
	return &Store{
		configDir:    configDir,
		eventFile:    filepath.Join(configDir, "events.json"),
		settingsFile: filepath.Join(configDir, "events-settings.json"),
		audioDir:     audioDir,
		iconDir:      iconDir,
		lockFile:     filepath.Join(configDir, "events.lock"),
	}, nil
}

func (s *Store) ConfigDir() string { return s.configDir }
func (s *Store) AudioDir() string  { return s.audioDir }
func (s *Store) IconDir() string   { return s.iconDir }

func (s *Store) Load() ([]event.Event, error) {
	data, err := os.ReadFile(s.eventFile)
	if errors.Is(err, os.ErrNotExist) {
		return []event.Event{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read events: %w", err)
	}
	var events []event.Event
	if err := json.Unmarshal(data, &events); err != nil {
		return nil, fmt.Errorf("decode %s: %w", s.eventFile, err)
	}
	if _, err := event.Compile(events); err != nil {
		return nil, fmt.Errorf("validate %s: %w", s.eventFile, err)
	}
	return events, nil
}

func (s *Store) Save(events []event.Event) error {
	if _, err := event.Compile(events); err != nil {
		return err
	}
	data, err := json.MarshalIndent(events, "", "  ")
	if err != nil {
		return fmt.Errorf("encode events: %w", err)
	}
	return s.withLock(func() error {
		return writeAtomic(s.eventFile, append(data, '\n'))
	})
}

type settings struct {
	IconSetup   IconSetup `json:"spell_icon_setup,omitempty"`
	IconSet     string    `json:"spell_icon_set,omitempty"`
	AudioVolume *float64  `json:"audio_volume,omitempty"`
}

func (s *Store) SpellIconSetup() (IconSetup, error) {
	configured, err := s.loadSettings()
	if err != nil {
		return IconSetupUnknown, err
	}
	switch configured.IconSetup {
	case IconSetupUnknown, IconSetupEnabled, IconSetupDeclined:
		return configured.IconSetup, nil
	default:
		return IconSetupUnknown, fmt.Errorf("invalid spell icon setup state %q", configured.IconSetup)
	}
}

func (s *Store) SaveSpellIconSetup(state IconSetup) error {
	if state != IconSetupEnabled && state != IconSetupDeclined {
		return fmt.Errorf("invalid spell icon setup state %q", state)
	}
	return s.withLock(func() error {
		configured, err := s.loadSettings()
		if err != nil {
			return err
		}
		configured.IconSetup = state
		data, err := json.MarshalIndent(configured, "", "  ")
		if err != nil {
			return fmt.Errorf("encode event settings: %w", err)
		}
		return writeAtomic(s.settingsFile, append(data, '\n'))
	})
}

func (s *Store) SpellIconSet() (string, error) {
	configured, err := s.loadSettings()
	if err != nil {
		return "", err
	}
	if configured.IconSet != "" && !validIconSetName(configured.IconSet) {
		return "", fmt.Errorf("invalid spell icon set %q", configured.IconSet)
	}
	return configured.IconSet, nil
}

func (s *Store) SaveSpellIconSet(name string) error {
	if !validIconSetName(name) {
		return fmt.Errorf("invalid spell icon set %q", name)
	}
	return s.withLock(func() error {
		configured, err := s.loadSettings()
		if err != nil {
			return err
		}
		configured.IconSet = name
		data, err := json.MarshalIndent(configured, "", "  ")
		if err != nil {
			return fmt.Errorf("encode event settings: %w", err)
		}
		return writeAtomic(s.settingsFile, append(data, '\n'))
	})
}

func (s *Store) SpellIconSets() ([]string, error) {
	entries, err := os.ReadDir(s.iconDir)
	if err != nil {
		return nil, fmt.Errorf("read spell icon directory: %w", err)
	}
	var sets []string
	for _, entry := range entries {
		if !entry.IsDir() || !validIconSetName(entry.Name()) {
			continue
		}
		files, err := os.ReadDir(filepath.Join(s.iconDir, entry.Name()))
		if err != nil {
			return nil, fmt.Errorf("read spell icon set %q: %w", entry.Name(), err)
		}
		for _, file := range files {
			name := strings.ToLower(file.Name())
			if file.Type().IsRegular() && strings.HasPrefix(name, "spell_") && strings.HasSuffix(name, ".png") {
				sets = append(sets, entry.Name())
				break
			}
		}
	}
	sort.Strings(sets)
	return sets, nil
}

func validIconSetName(name string) bool {
	return name != "" && name != "." && name != ".." && filepath.Base(name) == name
}

func (s *Store) AudioVolume() (float64, error) {
	configured, err := s.loadSettings()
	if err != nil {
		return 0, err
	}
	if configured.AudioVolume == nil {
		return 1, nil
	}
	if math.IsNaN(*configured.AudioVolume) || *configured.AudioVolume < 0 || *configured.AudioVolume > 1 {
		return 0, fmt.Errorf("invalid event audio volume %v", *configured.AudioVolume)
	}
	return *configured.AudioVolume, nil
}

func (s *Store) SaveAudioVolume(volume float64) error {
	if math.IsNaN(volume) || volume < 0 || volume > 1 {
		return fmt.Errorf("invalid event audio volume %v", volume)
	}
	return s.withLock(func() error {
		configured, err := s.loadSettings()
		if err != nil {
			return err
		}
		configured.AudioVolume = &volume
		data, err := json.MarshalIndent(configured, "", "  ")
		if err != nil {
			return fmt.Errorf("encode event settings: %w", err)
		}
		return writeAtomic(s.settingsFile, append(data, '\n'))
	})
}

func (s *Store) loadSettings() (settings, error) {
	data, err := os.ReadFile(s.settingsFile)
	if errors.Is(err, os.ErrNotExist) {
		return settings{}, nil
	}
	if err != nil {
		return settings{}, fmt.Errorf("read event settings: %w", err)
	}
	var configured settings
	if err := json.Unmarshal(data, &configured); err != nil {
		return settings{}, fmt.Errorf("decode %s: %w", s.settingsFile, err)
	}
	return configured, nil
}

func (s *Store) Sounds() ([]string, error) {
	entries, err := os.ReadDir(s.audioDir)
	if err != nil {
		return nil, fmt.Errorf("read audio directory: %w", err)
	}
	var sounds []string
	for _, entry := range entries {
		extension := filepath.Ext(entry.Name())
		if entry.IsDir() || (!strings.EqualFold(extension, ".wav") && !strings.EqualFold(extension, ".mp3")) {
			continue
		}
		sounds = append(sounds, entry.Name())
	}
	sort.Strings(sounds)
	return sounds, nil
}

func (s *Store) withLock(action func() error) error {
	const (
		waitFor = 2 * time.Second
		staleAt = 30 * time.Second
	)
	deadline := time.Now().Add(waitFor)
	for {
		lock, err := os.OpenFile(s.lockFile, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
		if err == nil {
			if _, writeErr := fmt.Fprintf(lock, "%d\n", os.Getpid()); writeErr != nil {
				lock.Close()
				os.Remove(s.lockFile)
				return fmt.Errorf("write event configuration lock: %w", writeErr)
			}
			if closeErr := lock.Close(); closeErr != nil {
				os.Remove(s.lockFile)
				return fmt.Errorf("close event configuration lock: %w", closeErr)
			}
			defer os.Remove(s.lockFile)
			return action()
		}
		if !errors.Is(err, os.ErrExist) {
			return fmt.Errorf("acquire event configuration lock: %w", err)
		}
		if info, statErr := os.Stat(s.lockFile); statErr == nil && time.Since(info.ModTime()) > staleAt {
			_ = os.Remove(s.lockFile)
			continue
		}
		if time.Now().After(deadline) {
			return errors.New("event configuration is busy in another eqdps process")
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func writeAtomic(path string, data []byte) error {
	directory := filepath.Dir(path)
	temporary, err := os.CreateTemp(directory, ".events-*.tmp")
	if err != nil {
		return fmt.Errorf("create temporary event configuration: %w", err)
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if err := temporary.Chmod(0o600); err != nil {
		temporary.Close()
		return fmt.Errorf("protect temporary event configuration: %w", err)
	}
	if _, err := temporary.Write(data); err != nil {
		temporary.Close()
		return fmt.Errorf("write temporary event configuration: %w", err)
	}
	if err := temporary.Sync(); err != nil {
		temporary.Close()
		return fmt.Errorf("sync temporary event configuration: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("close temporary event configuration: %w", err)
	}
	if err := replaceFile(temporaryPath, path); err != nil {
		return fmt.Errorf("replace event configuration: %w", err)
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
