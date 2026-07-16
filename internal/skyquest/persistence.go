package skyquest

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/uija/eqdps/internal/eqlog"
)

const (
	stateVersion     = 1
	fingerprintBytes = 64 * 1024
)

var (
	logNameRE            = regexp.MustCompile(`^eqlog_([^_]+)_([^_]+)\.txt$`)
	ErrInvalidCheckpoint = errors.New("Plane of Sky log checkpoint is invalid")
)

type CharacterState struct {
	Version    int            `json:"version"`
	Character  string         `json:"character"`
	Server     string         `json:"server"`
	Holdings   map[string]int `json:"holdings"`
	Checkpoint LogCheckpoint  `json:"log_checkpoint"`
}

type LogCheckpoint struct {
	LogFile       string    `json:"log_file"`
	PrefixSHA256  string    `json:"prefix_sha256"`
	PrefixBytes   int64     `json:"prefix_bytes"`
	Offset        int64     `json:"offset"`
	LastTimestamp time.Time `json:"last_timestamp,omitempty"`
	LastZone      string    `json:"last_zone,omitempty"`
}

type PersistentTracker struct {
	mu        sync.Mutex
	tracker   *Tracker
	state     CharacterState
	statePath string
}

func OpenPersistentTracker(logPath string, database Database) (*PersistentTracker, error) {
	character, server, err := CharacterIdentity(logPath)
	if err != nil {
		return nil, err
	}
	absoluteLogPath, err := filepath.Abs(logPath)
	if err != nil {
		return nil, fmt.Errorf("resolve log path: %w", err)
	}
	statePath := filepath.Join(filepath.Dir(absoluteLogPath), character+"_"+server+"_PoS.json")
	state := CharacterState{
		Version: stateVersion, Character: character, Server: server, Holdings: make(map[string]int),
		Checkpoint: LogCheckpoint{LogFile: filepath.Base(absoluteLogPath)},
	}
	if err := loadState(statePath, &state); err != nil {
		return nil, err
	}
	if state.Version != stateVersion || state.Character != character || state.Server != server || state.Checkpoint.LogFile != filepath.Base(absoluteLogPath) {
		return nil, fmt.Errorf("%w: state identity does not match %s", ErrInvalidCheckpoint, filepath.Base(absoluteLogPath))
	}

	persistent := &PersistentTracker{tracker: NewTracker(database), state: state, statePath: statePath}
	for item, quantity := range state.Holdings {
		if _, known := persistent.tracker.known[item]; known && quantity > 0 {
			persistent.tracker.owned[item] = quantity
		}
	}
	persistent.tracker.zone = state.Checkpoint.LastZone
	if err := persistent.syncLog(absoluteLogPath); err != nil {
		return nil, err
	}
	return persistent, nil
}

func CharacterIdentity(logPath string) (string, string, error) {
	matches := logNameRE.FindStringSubmatch(filepath.Base(logPath))
	if matches == nil {
		return "", "", fmt.Errorf("derive character and server: expected eqlog_CHARACTER_SERVER.txt, got %q", filepath.Base(logPath))
	}
	return matches[1], matches[2], nil
}

func (p *PersistentTracker) syncLog(logPath string) error {
	file, err := os.Open(logPath)
	if err != nil {
		return fmt.Errorf("open log for Plane of Sky inventory: %w", err)
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return fmt.Errorf("stat log for Plane of Sky inventory: %w", err)
	}
	if info.Size() < p.state.Checkpoint.Offset {
		return fmt.Errorf("%w: log size %d is smaller than saved offset %d", ErrInvalidCheckpoint, info.Size(), p.state.Checkpoint.Offset)
	}
	fingerprint, prefixBytes, err := logFingerprint(file, p.state.Checkpoint.PrefixBytes)
	if err != nil {
		return err
	}
	if p.state.Checkpoint.PrefixSHA256 != "" && p.state.Checkpoint.PrefixSHA256 != fingerprint {
		return fmt.Errorf("%w: log prefix changed; refusing to rescan automatically", ErrInvalidCheckpoint)
	}
	if _, err := file.Seek(p.state.Checkpoint.Offset, io.SeekStart); err != nil {
		return fmt.Errorf("seek Plane of Sky log checkpoint: %w", err)
	}

	reader := bufio.NewReader(file)
	for {
		line, readErr := reader.ReadString('\n')
		if len(line) > 0 && strings.HasSuffix(line, "\n") {
			p.processLineLocked(line, p.state.Checkpoint.Offset+int64(len(line)))
		}
		if readErr == nil {
			continue
		}
		if errors.Is(readErr, io.EOF) {
			break
		}
		return fmt.Errorf("read log for Plane of Sky inventory: %w", readErr)
	}
	p.state.Checkpoint.PrefixSHA256 = fingerprint
	p.state.Checkpoint.PrefixBytes = prefixBytes
	return p.saveLocked()
}

func (p *PersistentTracker) ProcessLine(line string, endOffset int64) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	changed := p.processLineLocked(line, endOffset)
	if changed {
		return p.saveLocked()
	}
	return nil
}

func (p *PersistentTracker) processLineLocked(line string, endOffset int64) bool {
	record, ok := eqlog.ParseRecord(line)
	changed := false
	if ok {
		beforeZone := p.tracker.zone
		beforeQuantity := 0
		item := ""
		switch record.Kind {
		case eqlog.RecordLoot:
			item = record.Loot.Item
		case eqlog.RecordItemRemoval:
			item = record.Removal.Item
		}
		if item != "" {
			beforeQuantity = p.tracker.Owned(item)
		}
		p.tracker.ProcessRecord(record)
		changed = beforeZone != p.tracker.zone || item != "" && beforeQuantity != p.tracker.Owned(item)
		if record.Time.After(p.state.Checkpoint.LastTimestamp) {
			p.state.Checkpoint.LastTimestamp = record.Time
		}
	}
	p.state.Checkpoint.Offset = endOffset
	p.state.Checkpoint.LastZone = p.tracker.zone
	p.state.Holdings = p.tracker.Inventory()
	return changed
}

func (p *PersistentTracker) Save() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.saveLocked()
}

func (p *PersistentTracker) saveLocked() error {
	data, err := json.MarshalIndent(p.state, "", "  ")
	if err != nil {
		return fmt.Errorf("encode Plane of Sky state: %w", err)
	}
	data = append(data, '\n')
	directory := filepath.Dir(p.statePath)
	temporary, err := os.CreateTemp(directory, ".eqdps-pos-*.tmp")
	if err != nil {
		return fmt.Errorf("create temporary Plane of Sky state: %w", err)
	}
	temporaryPath := temporary.Name()
	cleanup := func() {
		temporary.Close()
		os.Remove(temporaryPath)
	}
	if _, err := temporary.Write(data); err != nil {
		cleanup()
		return fmt.Errorf("write Plane of Sky state: %w", err)
	}
	if err := temporary.Sync(); err != nil {
		cleanup()
		return fmt.Errorf("sync Plane of Sky state: %w", err)
	}
	if err := temporary.Close(); err != nil {
		os.Remove(temporaryPath)
		return fmt.Errorf("close Plane of Sky state: %w", err)
	}
	if err := os.Rename(temporaryPath, p.statePath); err != nil {
		// Windows cannot replace an existing file with os.Rename.
		if removeErr := os.Remove(p.statePath); removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
			os.Remove(temporaryPath)
			return fmt.Errorf("replace Plane of Sky state: %w", err)
		}
		if renameErr := os.Rename(temporaryPath, p.statePath); renameErr != nil {
			os.Remove(temporaryPath)
			return fmt.Errorf("replace Plane of Sky state: %w", renameErr)
		}
	}
	return nil
}

func (p *PersistentTracker) Offset() int64 {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.state.Checkpoint.Offset
}

func (p *PersistentTracker) StatePath() string {
	return p.statePath
}

func (p *PersistentTracker) Inventory() map[string]int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.tracker.Inventory()
}

func (p *PersistentTracker) ReadyQuests() []QuestProgress {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.tracker.ReadyQuests()
}

func loadState(path string, state *CharacterState) error {
	file, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("open Plane of Sky state: %w", err)
	}
	defer file.Close()
	decoder := json.NewDecoder(file)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(state); err != nil {
		return fmt.Errorf("decode Plane of Sky state %s: %w", path, err)
	}
	if state.Holdings == nil {
		state.Holdings = make(map[string]int)
	}
	return nil
}

func logFingerprint(file *os.File, prefixBytes int64) (string, int64, error) {
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return "", 0, fmt.Errorf("seek log fingerprint: %w", err)
	}
	var data []byte
	if prefixBytes > 0 {
		data = make([]byte, prefixBytes)
		if _, err := io.ReadFull(file, data); err != nil {
			return "", 0, fmt.Errorf("fingerprint log: %w", err)
		}
	} else {
		reader := bufio.NewReader(io.LimitReader(file, fingerprintBytes))
		var err error
		data, err = reader.ReadBytes('\n')
		if err != nil && !errors.Is(err, io.EOF) {
			return "", 0, fmt.Errorf("fingerprint log: %w", err)
		}
	}
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:]), int64(len(data)), nil
}
