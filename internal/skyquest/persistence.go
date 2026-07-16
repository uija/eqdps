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
	ErrScanCancelled     = errors.New("Plane of Sky inventory scan cancelled")
)

type ScanProgress struct {
	Bytes int64
	Total int64
	Lines int
}

type CharacterState struct {
	Version    int                       `json:"version"`
	Character  string                    `json:"character"`
	Server     string                    `json:"server"`
	Holdings   map[string]int            `json:"holdings"`
	Completed  map[string]bool           `json:"completed_quests,omitempty"`
	Pending    map[string]map[string]int `json:"pending_offers,omitempty"`
	Checkpoint LogCheckpoint             `json:"log_checkpoint"`
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
	persistent, absoluteLogPath, err := loadPersistentTracker(logPath, database)
	if err != nil {
		return nil, err
	}
	if err := persistent.syncLog(absoluteLogPath, 0, nil, nil); err != nil {
		return nil, err
	}
	return persistent, nil
}

// LoadPersistentTracker loads saved state without reading newer logfile lines.
// Call SyncLogWithProgress before live processing begins.
func LoadPersistentTracker(logPath string, database Database) (*PersistentTracker, error) {
	persistent, _, err := loadPersistentTracker(logPath, database)
	return persistent, err
}

func InitializePersistentTracker(logPath string, database Database, maxBytes int64, onProgress func(ScanProgress), cancel <-chan struct{}) (*PersistentTracker, error) {
	statePath, err := StatePath(logPath)
	if err != nil {
		return nil, err
	}
	if _, err := os.Stat(statePath); err == nil {
		return nil, fmt.Errorf("Plane of Sky state already exists: %s", statePath)
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("stat Plane of Sky state: %w", err)
	}
	return openPersistentTracker(logPath, database, maxBytes, onProgress, cancel)
}

func openPersistentTracker(logPath string, database Database, maxBytes int64, onProgress func(ScanProgress), cancel <-chan struct{}) (*PersistentTracker, error) {
	persistent, absoluteLogPath, err := loadPersistentTracker(logPath, database)
	if err != nil {
		return nil, err
	}
	if err := persistent.syncLog(absoluteLogPath, maxBytes, onProgress, cancel); err != nil {
		return nil, err
	}
	return persistent, nil
}

func loadPersistentTracker(logPath string, database Database) (*PersistentTracker, string, error) {
	character, server, err := CharacterIdentity(logPath)
	if err != nil {
		return nil, "", err
	}
	absoluteLogPath, err := filepath.Abs(logPath)
	if err != nil {
		return nil, "", fmt.Errorf("resolve log path: %w", err)
	}
	statePath := filepath.Join(filepath.Dir(absoluteLogPath), character+"_"+server+"_PoS.json")
	state := CharacterState{
		Version: stateVersion, Character: character, Server: server, Holdings: make(map[string]int),
		Completed:  make(map[string]bool),
		Pending:    make(map[string]map[string]int),
		Checkpoint: LogCheckpoint{LogFile: filepath.Base(absoluteLogPath)},
	}
	if err := loadState(statePath, &state); err != nil {
		return nil, "", err
	}
	if state.Version != stateVersion || state.Character != character || state.Server != server || state.Checkpoint.LogFile != filepath.Base(absoluteLogPath) {
		return nil, "", fmt.Errorf("%w: state identity does not match %s", ErrInvalidCheckpoint, filepath.Base(absoluteLogPath))
	}

	persistent := &PersistentTracker{tracker: NewTracker(database), state: state, statePath: statePath}
	for item, quantity := range state.Holdings {
		if _, known := persistent.tracker.known[item]; known && quantity > 0 {
			persistent.tracker.owned[item] = quantity
		}
	}
	for quest, completed := range state.Completed {
		if completed {
			persistent.tracker.completed[quest] = true
		}
	}
	for npc, offered := range state.Pending {
		persistent.tracker.pending[npc] = make(map[string]int, len(offered))
		for item, quantity := range offered {
			if normalized, known := persistent.tracker.knownItem(item); known && quantity > 0 {
				persistent.tracker.pending[npc][normalized] += quantity
			}
		}
	}
	persistent.tracker.zone = state.Checkpoint.LastZone
	return persistent, absoluteLogPath, nil
}

func StatePath(logPath string) (string, error) {
	character, server, err := CharacterIdentity(logPath)
	if err != nil {
		return "", err
	}
	absoluteLogPath, err := filepath.Abs(logPath)
	if err != nil {
		return "", fmt.Errorf("resolve log path: %w", err)
	}
	return filepath.Join(filepath.Dir(absoluteLogPath), character+"_"+server+"_PoS.json"), nil
}

func StateExists(logPath string) (bool, error) {
	path, err := StatePath(logPath)
	if err != nil {
		return false, err
	}
	_, err = os.Stat(path)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	return false, fmt.Errorf("stat Plane of Sky state: %w", err)
}

func CharacterIdentity(logPath string) (string, string, error) {
	matches := logNameRE.FindStringSubmatch(filepath.Base(logPath))
	if matches == nil {
		return "", "", fmt.Errorf("derive character and server: expected eqlog_CHARACTER_SERVER.txt, got %q", filepath.Base(logPath))
	}
	return matches[1], matches[2], nil
}

func (p *PersistentTracker) syncLog(logPath string, maxBytes int64, onProgress func(ScanProgress), cancel <-chan struct{}) error {
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
	if maxBytes <= 0 || maxBytes > info.Size() {
		maxBytes = info.Size()
	}
	if maxBytes < p.state.Checkpoint.Offset {
		return fmt.Errorf("%w: scan limit %d is smaller than saved offset %d", ErrInvalidCheckpoint, maxBytes, p.state.Checkpoint.Offset)
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

	reader := bufio.NewReader(io.LimitReader(file, maxBytes-p.state.Checkpoint.Offset))
	lines := 0
	for {
		if lines%1000 == 0 && scanCancelled(cancel) {
			return ErrScanCancelled
		}
		line, readErr := reader.ReadString('\n')
		if len(line) > 0 && strings.HasSuffix(line, "\n") {
			p.processLineLocked(line, p.state.Checkpoint.Offset+int64(len(line)))
			lines++
			if onProgress != nil && lines%5000 == 0 {
				onProgress(ScanProgress{Bytes: p.state.Checkpoint.Offset, Total: maxBytes, Lines: lines})
			}
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
	if scanCancelled(cancel) {
		return ErrScanCancelled
	}
	if onProgress != nil {
		onProgress(ScanProgress{Bytes: p.state.Checkpoint.Offset, Total: maxBytes, Lines: lines})
	}
	return p.saveLocked()
}

func (p *PersistentTracker) SyncLog(logPath string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.syncLog(logPath, 0, nil, nil)
}

func (p *PersistentTracker) SyncLogWithProgress(logPath string, maxBytes int64, onProgress func(ScanProgress), cancel <-chan struct{}) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.syncLog(logPath, maxBytes, onProgress, cancel)
}

func (p *PersistentTracker) ProcessLine(line string, endOffset int64) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if endOffset <= p.state.Checkpoint.Offset {
		return nil
	}
	changed := p.processLineLocked(line, endOffset)
	if changed {
		return p.saveLocked()
	}
	return nil
}

func scanCancelled(cancel <-chan struct{}) bool {
	if cancel == nil {
		return false
	}
	select {
	case <-cancel:
		return true
	default:
		return false
	}
}

func (p *PersistentTracker) processLineLocked(line string, endOffset int64) bool {
	record, ok := eqlog.ParseRecord(line)
	changed := false
	if ok {
		beforeZone := p.tracker.zone
		beforeCompleted := len(p.tracker.completed)
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
		changed = beforeZone != p.tracker.zone || item != "" && beforeQuantity != p.tracker.Owned(item) || beforeCompleted != len(p.tracker.completed)
		if record.Time.After(p.state.Checkpoint.LastTimestamp) {
			p.state.Checkpoint.LastTimestamp = record.Time
		}
	}
	p.state.Checkpoint.Offset = endOffset
	p.state.Checkpoint.LastZone = p.tracker.zone
	p.state.Holdings = p.tracker.Inventory()
	p.state.Completed = p.tracker.Completed()
	p.state.Pending = p.tracker.PendingOffers()
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

func (p *PersistentTracker) QuestProgress() []QuestProgress {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.tracker.QuestProgress()
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
	if state.Completed == nil {
		state.Completed = make(map[string]bool)
	}
	if state.Pending == nil {
		state.Pending = make(map[string]map[string]int)
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
