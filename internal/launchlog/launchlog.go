// Package launchlog separates pre-launch beta data from live EverQuest logs.
package launchlog

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/uija/eqdps/internal/dropcollector"
	"github.com/uija/eqdps/internal/eqldbqueue"
	"github.com/uija/eqdps/internal/eqlog"
	"github.com/uija/eqdps/internal/skyquest"
)

const (
	defaultCutoffText = "2026-07-27"
	cutoffEnvironment = "EQDPS_LAUNCH_CUTOFF"
)

type Check struct {
	NeedsAction bool
	Cutoff      time.Time
}

type decisionState struct {
	Ignored map[string]bool `json:"ignored,omitempty"`
}

// Cutoff returns the launch boundary. The environment override exists so the
// one-time behavior can be tested without changing release code.
func Cutoff() (time.Time, error) {
	value := strings.TrimSpace(os.Getenv(cutoffEnvironment))
	if value == "" {
		value = defaultCutoffText
	}
	cutoff, err := time.Parse("2006-01-02", value)
	if err != nil {
		return time.Time{}, fmt.Errorf("%s must use YYYY-MM-DD: %w", cutoffEnvironment, err)
	}
	return cutoff, nil
}

// Inspect reports whether the first timestamped entry predates launch and the
// user has not already chosen to keep this logfile unchanged.
func Inspect(logPath string) (Check, error) {
	cutoff, err := Cutoff()
	if err != nil {
		return Check{}, err
	}
	ignored, err := isIgnored(logPath, cutoff)
	if err != nil {
		return Check{}, err
	}
	if ignored {
		return Check{Cutoff: cutoff}, nil
	}
	first, found, err := firstTimestamp(logPath)
	if err != nil {
		return Check{}, err
	}
	return Check{NeedsAction: found && first.Before(cutoff), Cutoff: cutoff}, nil
}

// RememberIgnored suppresses the beta warning for this logfile and cutoff.
func RememberIgnored(logPath string) error {
	cutoff, err := Cutoff()
	if err != nil {
		return err
	}
	path, err := decisionsPath()
	if err != nil {
		return err
	}
	state, err := loadDecisions(path)
	if err != nil {
		return err
	}
	state.Ignored[decisionKey(logPath, cutoff)] = true
	return saveDecisions(path, state)
}

// Fix archives the beta logfile, creates a new logfile containing only launch
// entries, and removes persisted state derived from beta data.
func Fix(logPath string) (string, error) {
	cutoff, err := Cutoff()
	if err != nil {
		return "", err
	}
	start, err := launchOffset(logPath, cutoff)
	if err != nil {
		return "", err
	}
	// Reset sidecars before changing the logfile. If cleanup is interrupted,
	// the unchanged beta logfile will be detected again and the operation can
	// be retried instead of leaving a shortened log with stale checkpoints.
	if err := resetDerivedState(logPath, cutoff); err != nil {
		return "", err
	}
	archivePath, err := availableArchivePath(strings.TrimSuffix(logPath, filepath.Ext(logPath)) + ".beta")
	if err != nil {
		return "", err
	}
	if err := os.Rename(logPath, archivePath); err != nil {
		return "", fmt.Errorf("archive beta logfile: %w", err)
	}
	if err := restoreLaunchLog(logPath, archivePath, start); err != nil {
		_ = os.Remove(logPath)
		_ = os.Rename(archivePath, logPath)
		return "", err
	}
	return archivePath, nil
}

func firstTimestamp(logPath string) (time.Time, bool, error) {
	file, err := os.Open(logPath)
	if err != nil {
		return time.Time{}, false, fmt.Errorf("open logfile for launch check: %w", err)
	}
	defer file.Close()
	reader := bufio.NewReader(file)
	for {
		line, readErr := reader.ReadString('\n')
		if timestamp, ok := eqlog.ParseTimestamp(line); ok {
			return timestamp, true, nil
		}
		if readErr != nil {
			if errors.Is(readErr, io.EOF) {
				return time.Time{}, false, nil
			}
			return time.Time{}, false, fmt.Errorf("read logfile for launch check: %w", readErr)
		}
	}
}

func launchOffset(logPath string, cutoff time.Time) (int64, error) {
	file, err := os.Open(logPath)
	if err != nil {
		return 0, fmt.Errorf("open beta logfile: %w", err)
	}
	defer file.Close()
	reader := bufio.NewReader(file)
	var offset int64
	for {
		line, readErr := reader.ReadString('\n')
		if timestamp, ok := eqlog.ParseTimestamp(line); ok && !timestamp.Before(cutoff) {
			return offset, nil
		}
		offset += int64(len(line))
		if readErr != nil {
			if errors.Is(readErr, io.EOF) {
				return offset, nil
			}
			return 0, fmt.Errorf("scan beta logfile: %w", readErr)
		}
	}
}

func restoreLaunchLog(logPath, archivePath string, start int64) error {
	source, err := os.Open(archivePath)
	if err != nil {
		return fmt.Errorf("open archived beta logfile: %w", err)
	}
	defer source.Close()
	info, err := source.Stat()
	if err != nil {
		return fmt.Errorf("stat archived beta logfile: %w", err)
	}
	if _, err := source.Seek(start, io.SeekStart); err != nil {
		return fmt.Errorf("seek to launch data: %w", err)
	}
	target, err := os.OpenFile(logPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, info.Mode().Perm())
	if err != nil {
		return fmt.Errorf("create launch logfile: %w", err)
	}
	ok := false
	defer func() {
		target.Close()
		if !ok {
			os.Remove(logPath)
		}
	}()
	if _, err := io.Copy(target, source); err != nil {
		return fmt.Errorf("copy launch logfile entries: %w", err)
	}
	if err := target.Sync(); err != nil {
		return fmt.Errorf("sync launch logfile: %w", err)
	}
	if err := target.Close(); err != nil {
		return fmt.Errorf("close launch logfile: %w", err)
	}
	ok = true
	return nil
}

func resetDerivedState(logPath string, cutoff time.Time) error {
	if _, err := skyquest.ArchiveState(logPath, ".beta"); err != nil {
		return err
	}
	if err := skyquest.ResetUploadHistory(logPath); err != nil {
		return err
	}
	if err := dropcollector.ResetState(logPath); err != nil {
		return err
	}
	queue, err := eqldbqueue.Default()
	if err != nil {
		return err
	}
	return queue.DiscardBefore(cutoff)
}

func availableArchivePath(preferred string) (string, error) {
	for index := 0; ; index++ {
		candidate := preferred
		if index > 0 {
			candidate = fmt.Sprintf("%s.%d", preferred, index)
		}
		_, err := os.Stat(candidate)
		if errors.Is(err, os.ErrNotExist) {
			return candidate, nil
		}
		if err != nil {
			return "", fmt.Errorf("check beta archive path: %w", err)
		}
	}
}

func decisionsPath() (string, error) {
	config, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("locate launch decisions: %w", err)
	}
	return filepath.Join(config, "eqdps", "launch-log.json"), nil
}

func isIgnored(logPath string, cutoff time.Time) (bool, error) {
	path, err := decisionsPath()
	if err != nil {
		return false, err
	}
	state, err := loadDecisions(path)
	if err != nil {
		return false, err
	}
	return state.Ignored[decisionKey(logPath, cutoff)], nil
}

func decisionKey(logPath string, cutoff time.Time) string {
	absolute, err := filepath.Abs(logPath)
	if err != nil {
		absolute = filepath.Clean(logPath)
	}
	return absolute + "\x00" + cutoff.Format("2006-01-02")
}

func loadDecisions(path string) (decisionState, error) {
	state := decisionState{Ignored: make(map[string]bool)}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return state, nil
	}
	if err != nil {
		return state, fmt.Errorf("read launch decisions: %w", err)
	}
	if err := json.Unmarshal(data, &state); err != nil {
		return state, fmt.Errorf("decode launch decisions: %w", err)
	}
	if state.Ignored == nil {
		state.Ignored = make(map[string]bool)
	}
	return state, nil
}

func saveDecisions(path string, state decisionState) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create launch-decision directory: %w", err)
	}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	temporary, err := os.CreateTemp(filepath.Dir(path), ".launch-log-*.tmp")
	if err != nil {
		return err
	}
	name := temporary.Name()
	defer os.Remove(name)
	if err := temporary.Chmod(0o600); err != nil {
		temporary.Close()
		return err
	}
	if _, err := temporary.Write(append(data, '\n')); err != nil {
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
	if err := os.Rename(name, path); err != nil {
		if removeErr := os.Remove(path); removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
			return err
		}
		return os.Rename(name, path)
	}
	return nil
}
