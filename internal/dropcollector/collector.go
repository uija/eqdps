// Package dropcollector records player-related kills and personal-loot
// observations in the durable EQLDB queue. It owns no presentation or network
// code and can therefore be shared by every frontend.
package dropcollector

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
	"strings"
	"sync"
	"time"

	"github.com/uija/eqdps/internal/eqldbqueue"
	"github.com/uija/eqdps/internal/eqlog"
	"github.com/uija/eqdps/internal/skyquest"
)

const (
	stateVersion   = 1
	pendingTimeout = 5 * time.Minute
	rewardWindow   = 2 * time.Second
)

type Observation struct {
	ID         string    `json:"id"`
	Kind       string    `json:"kind"`
	Character  string    `json:"character"`
	Server     string    `json:"server"`
	Zone       string    `json:"zone,omitempty"`
	Mob        string    `json:"mob"`
	Item       string    `json:"item,omitempty"`
	Quantity   int       `json:"quantity,omitempty"`
	LootAction string    `json:"loot_action,omitempty"`
	Timestamp  time.Time `json:"timestamp"`
}

type pendingKill struct {
	Mob       string    `json:"mob"`
	Zone      string    `json:"zone,omitempty"`
	Timestamp time.Time `json:"timestamp"`
	Offset    int64     `json:"offset"`
}

type state struct {
	Version        int                  `json:"version"`
	LogFile        string               `json:"log_file"`
	Offset         int64                `json:"offset"`
	Zone           string               `json:"zone,omitempty"`
	LastReward     time.Time            `json:"last_reward,omitempty"`
	Engaged        map[string]time.Time `json:"engaged,omitempty"`
	Pending        []pendingKill        `json:"pending_kills,omitempty"`
	LinesSinceSave int                  `json:"-"`
}

type settings struct {
	Enabled bool `json:"enabled"`
}

type Collector struct {
	mu           sync.Mutex
	logPath      string
	character    string
	server       string
	settingsPath string
	statePath    string
	queue        *eqldbqueue.Queue
	enabled      bool
	state        state
	seen         map[string]struct{}
}

func Open(logPath string) (*Collector, error) {
	character, server, err := skyquest.CharacterIdentity(logPath)
	if err != nil {
		return nil, err
	}
	absolute, err := filepath.Abs(logPath)
	if err != nil {
		return nil, fmt.Errorf("resolve drop-collection logfile: %w", err)
	}
	config, err := os.UserConfigDir()
	if err != nil {
		return nil, fmt.Errorf("locate drop-collection settings: %w", err)
	}
	directory := filepath.Join(config, "eqdps", "drop-collection")
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return nil, fmt.Errorf("create drop-collection directory: %w", err)
	}
	queue, err := eqldbqueue.Default()
	if err != nil {
		return nil, err
	}
	key := safeName(character + "_" + server)
	pathHash := sha256.Sum256([]byte(absolute))
	stateKey := key + "-" + hex.EncodeToString(pathHash[:6])
	collector := &Collector{
		logPath:      absolute,
		character:    character,
		server:       server,
		settingsPath: filepath.Join(config, "eqdps", "drop-collection-settings.json"),
		statePath:    filepath.Join(directory, stateKey+"-state.json"),
		queue:        queue,
		seen:         make(map[string]struct{}),
		state: state{
			Version: stateVersion,
			LogFile: filepath.Base(absolute),
			Engaged: make(map[string]time.Time),
		},
	}
	if err := collector.load(); err != nil {
		return nil, err
	}
	return collector, nil
}

// ResetState removes the logfile checkpoint and pending beta-era observations
// for one character logfile. The global opt-in setting is preserved.
func ResetState(logPath string) error {
	character, server, err := skyquest.CharacterIdentity(logPath)
	if err != nil {
		return err
	}
	absolute, err := filepath.Abs(logPath)
	if err != nil {
		return fmt.Errorf("resolve drop-collection logfile: %w", err)
	}
	config, err := os.UserConfigDir()
	if err != nil {
		return fmt.Errorf("locate drop-collection settings: %w", err)
	}
	key := safeName(character + "_" + server)
	pathHash := sha256.Sum256([]byte(absolute))
	stateKey := key + "-" + hex.EncodeToString(pathHash[:6])
	path := filepath.Join(config, "eqdps", "drop-collection", stateKey+"-state.json")
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("remove beta drop-collection state: %w", err)
	}
	return nil
}

func (c *Collector) load() error {
	var configured settings
	if data, err := os.ReadFile(c.settingsPath); err == nil {
		if err := json.Unmarshal(data, &configured); err != nil {
			return fmt.Errorf("decode drop-collection settings: %w", err)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("read drop-collection settings: %w", err)
	}
	c.enabled = configured.Enabled

	if data, err := os.ReadFile(c.statePath); err == nil {
		if err := json.Unmarshal(data, &c.state); err != nil {
			return fmt.Errorf("decode drop-collection state: %w", err)
		}
		if c.state.Version != stateVersion || c.state.LogFile != filepath.Base(c.logPath) {
			return fmt.Errorf("drop-collection checkpoint does not match %s", filepath.Base(c.logPath))
		}
		if c.state.Engaged == nil {
			c.state.Engaged = make(map[string]time.Time)
		}
		info, err := os.Stat(c.logPath)
		if err != nil {
			return fmt.Errorf("stat drop-collection logfile: %w", err)
		}
		if c.state.Offset > info.Size() {
			return fmt.Errorf("drop-collection checkpoint exceeds logfile size")
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("read drop-collection state: %w", err)
	} else {
		info, err := os.Stat(c.logPath)
		if err != nil {
			return fmt.Errorf("stat drop-collection logfile: %w", err)
		}
		c.state.Offset = info.Size()
		if c.enabled {
			if err := c.findLatestZoneLocked(info.Size()); err != nil {
				return err
			}
		}
		if err := c.saveStateLocked(); err != nil {
			return err
		}
	}
	return c.loadSeen()
}

func (c *Collector) loadSeen() error {
	for _, name := range []string{eqldbqueue.Kills, eqldbqueue.Drops} {
		ids, err := c.queue.RecentIDs(name, 16*1024*1024)
		if err != nil {
			return fmt.Errorf("read recent %s observations: %w", name, err)
		}
		for id := range ids {
			c.seen[id] = struct{}{}
		}
	}
	return nil
}

func CollectionEnabled() (bool, error) {
	config, err := os.UserConfigDir()
	if err != nil {
		return false, err
	}
	path := filepath.Join(config, "eqdps", "drop-collection-settings.json")
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	var configured settings
	if err := json.Unmarshal(data, &configured); err != nil {
		return false, err
	}
	return configured.Enabled, nil
}

func (c *Collector) Enabled() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.enabled
}

func (c *Collector) LogPath() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.logPath
}

// SetEnabled changes the shared opt-in. Both transitions move the checkpoint
// to the current end of the logfile, so activity from an opted-out period is
// never collected later.
func (c *Collector) SetEnabled(enabled bool) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if enabled == c.enabled {
		return nil
	}
	info, err := os.Stat(c.logPath)
	if err != nil {
		return fmt.Errorf("stat drop-collection logfile: %w", err)
	}
	c.enabled = enabled
	c.state.Offset = info.Size()
	c.state.Pending = nil
	c.state.Engaged = make(map[string]time.Time)
	c.state.LastReward = time.Time{}
	if err := c.findLatestZoneLocked(info.Size()); err != nil {
		return err
	}
	if err := writeJSONAtomic(c.settingsPath, settings{Enabled: enabled}); err != nil {
		return err
	}
	return c.saveStateLocked()
}

// Sync catches up only from the collection checkpoint. Calling it during a
// combat history reload is harmless: already processed offsets are skipped.
func (c *Collector) Sync(maxOffset int64) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.enabled || maxOffset <= c.state.Offset {
		return nil
	}
	file, err := os.Open(c.logPath)
	if err != nil {
		return fmt.Errorf("open logfile for drop collection: %w", err)
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return fmt.Errorf("stat logfile for drop collection: %w", err)
	}
	if info.Size() < c.state.Offset {
		return fmt.Errorf("drop-collection logfile is smaller than its checkpoint")
	}
	maxOffset = min(maxOffset, info.Size())
	if _, err := file.Seek(c.state.Offset, io.SeekStart); err != nil {
		return fmt.Errorf("seek drop-collection checkpoint: %w", err)
	}
	reader := bufio.NewReader(io.LimitReader(file, maxOffset-c.state.Offset))
	for {
		line, readErr := reader.ReadString('\n')
		if strings.HasSuffix(line, "\n") {
			if err := c.processLineLocked(line, c.state.Offset+int64(len(line))); err != nil {
				return err
			}
		}
		if readErr == nil {
			continue
		}
		if errors.Is(readErr, io.EOF) {
			break
		}
		return fmt.Errorf("read logfile for drop collection: %w", readErr)
	}
	return c.saveStateLocked()
}

func (c *Collector) ProcessLine(line string, endOffset int64) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.enabled || endOffset <= c.state.Offset {
		return nil
	}
	beforeQueued := len(c.seen)
	if err := c.processLineLocked(line, endOffset); err != nil {
		return err
	}
	c.state.LinesSinceSave++
	if len(c.seen) != beforeQueued || c.state.LinesSinceSave >= 1000 {
		return c.saveStateLocked()
	}
	return nil
}

func (c *Collector) processLineLocked(line string, endOffset int64) error {
	record, ok := eqlog.ParseRecord(line)
	if ok {
		c.expirePendingLocked(record.Time)
		switch record.Kind {
		case eqlog.RecordZoneChange:
			c.state.Zone = record.ZoneChange.Name
			c.state.Engaged = make(map[string]time.Time)
			c.state.Pending = nil
		case eqlog.RecordExperience, eqlog.RecordKillReward:
			c.state.LastReward = record.Time
		case eqlog.RecordDamage:
			if c.isOwnedSource(record.Damage.Source) {
				c.state.Engaged[mobKey(record.Damage.Target)] = record.Time
			}
		case eqlog.RecordDeath:
			if record.Death.Victim != "You" {
				relevant := record.Death.Killer == "You" ||
					wasRecentlyEngaged(c.state.Engaged[mobKey(record.Death.Victim)], record.Time) ||
					within(record.Time, c.state.LastReward, rewardWindow)
				delete(c.state.Engaged, mobKey(record.Death.Victim))
				if relevant {
					if err := c.appendKillLocked(record.Death.Victim, c.state.Zone, record.Time, endOffset); err != nil {
						return err
					}
				} else {
					c.state.Pending = append(c.state.Pending, pendingKill{
						Mob: record.Death.Victim, Zone: c.state.Zone, Timestamp: record.Time, Offset: endOffset,
					})
				}
				c.state.LastReward = time.Time{}
			}
		case eqlog.RecordLoot:
			if err := c.appendLootLocked(record.Loot, endOffset); err != nil {
				return err
			}
			remaining := c.state.Pending[:0]
			for _, pending := range c.state.Pending {
				if mobKey(pending.Mob) == mobKey(record.Loot.Corpse) && pending.Zone == c.state.Zone &&
					within(record.Loot.Time, pending.Timestamp, pendingTimeout) {
					if err := c.appendKillLocked(pending.Mob, pending.Zone, pending.Timestamp, pending.Offset); err != nil {
						return err
					}
					continue
				}
				remaining = append(remaining, pending)
			}
			c.state.Pending = remaining
		}
	}
	c.state.Offset = endOffset
	return nil
}

func (c *Collector) appendKillLocked(mob, zone string, timestamp time.Time, offset int64) error {
	return c.appendLocked(Observation{
		ID: c.observationID("kill", offset), Kind: "kill", Character: c.character,
		Server: c.server, Zone: zone, Mob: mob, Timestamp: timestamp,
	})
}

func (c *Collector) appendLootLocked(loot eqlog.Loot, offset int64) error {
	return c.appendLocked(Observation{
		ID: c.observationID("loot", offset), Kind: "loot", Character: c.character,
		Server: c.server, Zone: c.state.Zone, Mob: loot.Corpse, Item: loot.Item,
		Quantity: loot.Quantity, LootAction: lootAction(loot.Outcome), Timestamp: loot.Time,
	})
}

func (c *Collector) appendLocked(observation Observation) error {
	if _, exists := c.seen[observation.ID]; exists {
		return nil
	}
	name := eqldbqueue.Drops
	if observation.Kind == "kill" {
		name = eqldbqueue.Kills
	}
	if err := c.queue.Append(name, observation.ID, observation); err != nil {
		return fmt.Errorf("queue drop observation: %w", err)
	}
	c.seen[observation.ID] = struct{}{}
	return nil
}

func (c *Collector) observationID(kind string, offset int64) string {
	sum := sha256.Sum256([]byte(c.logPath + "\x00" + kind + "\x00" + fmt.Sprint(offset)))
	return hex.EncodeToString(sum[:])
}

func (c *Collector) isOwnedSource(source string) bool {
	normalized := strings.TrimSpace(source)
	return normalized == "You" ||
		strings.EqualFold(normalized, c.character+" pet") ||
		strings.EqualFold(normalized, c.character+"'s pet") ||
		strings.EqualFold(normalized, c.character+"`s pet")
}

func (c *Collector) expirePendingLocked(now time.Time) {
	if now.IsZero() {
		return
	}
	remaining := c.state.Pending[:0]
	for _, pending := range c.state.Pending {
		if now.Before(pending.Timestamp) || now.Sub(pending.Timestamp) <= pendingTimeout {
			remaining = append(remaining, pending)
		}
	}
	c.state.Pending = remaining
	for mob, timestamp := range c.state.Engaged {
		if now.After(timestamp) && now.Sub(timestamp) > pendingTimeout {
			delete(c.state.Engaged, mob)
		}
	}
}

func (c *Collector) findLatestZoneLocked(maxOffset int64) error {
	file, err := os.Open(c.logPath)
	if err != nil {
		return fmt.Errorf("open logfile to find current zone: %w", err)
	}
	defer file.Close()
	scanner := bufio.NewScanner(io.LimitReader(file, maxOffset))
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		if zone, ok := eqlog.ParseZoneChangeLine(scanner.Text()); ok {
			c.state.Zone = zone.Name
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("find current zone: %w", err)
	}
	return nil
}

func (c *Collector) saveStateLocked() error {
	c.state.LinesSinceSave = 0
	return writeJSONAtomic(c.statePath, c.state)
}

func (c *Collector) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.saveStateLocked()
}

func writeJSONAtomic(path string, value any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create settings directory: %w", err)
	}
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return fmt.Errorf("encode %s: %w", filepath.Base(path), err)
	}
	temporary, err := os.CreateTemp(filepath.Dir(path), ".eqdps-drops-*.tmp")
	if err != nil {
		return fmt.Errorf("create temporary %s: %w", filepath.Base(path), err)
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

func mobKey(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func within(later, earlier time.Time, limit time.Duration) bool {
	return !later.IsZero() && !earlier.IsZero() && !later.Before(earlier) && later.Sub(earlier) <= limit
}

func wasRecentlyEngaged(engaged, death time.Time) bool {
	return within(death, engaged, pendingTimeout)
}

func safeName(value string) string {
	return strings.NewReplacer("/", "_", "\\", "_", ":", "_").Replace(value)
}

func lootAction(outcome eqlog.LootOutcome) string {
	switch outcome {
	case eqlog.LootStored:
		return "stored"
	case eqlog.LootSold:
		return "sold"
	case eqlog.LootConverted:
		return "converted"
	default:
		return "retained"
	}
}
