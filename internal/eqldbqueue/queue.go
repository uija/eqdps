// Package eqldbqueue provides small durable FIFO queues for EQLDB uploads.
// Queue cursors are byte offsets; acknowledged history is never rescanned.
package eqldbqueue

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
)

const (
	PlaneOfSky = "plane-of-sky"
	Kills      = "kills"
	Drops      = "drops"

	lockTimeout = 5 * time.Second
	staleLock   = 30 * time.Second
)

type Entry struct {
	ID      string          `json:"id"`
	Payload json.RawMessage `json:"payload"`

	EndOffset int64 `json:"-"`
}

type cursorState struct {
	Offsets map[string]int64 `json:"offsets,omitempty"`
}

type Queue struct {
	directory string
	statePath string
	lockPath  string
}

func Default() (*Queue, error) {
	config, err := os.UserConfigDir()
	if err != nil {
		return nil, fmt.Errorf("locate EQLDB queue: %w", err)
	}
	return Open(filepath.Join(config, "eqdps", "eqldb-queue"))
}

func Open(directory string) (*Queue, error) {
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return nil, fmt.Errorf("create EQLDB queue: %w", err)
	}
	return &Queue{
		directory: directory,
		statePath: filepath.Join(directory, "state.json"),
		lockPath:  filepath.Join(directory, ".lock"),
	}, nil
}

func (q *Queue) Path(name string) string {
	return filepath.Join(q.directory, name+".jsonl")
}

func (q *Queue) Append(name, id string, payload any) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("encode %s queue payload: %w", name, err)
	}
	line, err := json.Marshal(Entry{ID: id, Payload: data})
	if err != nil {
		return fmt.Errorf("encode %s queue entry: %w", name, err)
	}
	return q.withLock(func() error {
		file, err := os.OpenFile(q.Path(name), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
		if err != nil {
			return fmt.Errorf("open %s queue: %w", name, err)
		}
		if _, err := file.Write(append(line, '\n')); err != nil {
			file.Close()
			return fmt.Errorf("append %s queue: %w", name, err)
		}
		if err := file.Close(); err != nil {
			return fmt.Errorf("close %s queue: %w", name, err)
		}
		return nil
	})
}

func (q *Queue) Batch(name string, maximum int) ([]Entry, error) {
	if maximum < 1 {
		return nil, nil
	}
	var entries []Entry
	err := q.withLock(func() error {
		state, err := q.loadState()
		if err != nil {
			return err
		}
		file, err := os.Open(q.Path(name))
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("open %s queue: %w", name, err)
		}
		defer file.Close()
		offset := state.Offsets[name]
		info, err := file.Stat()
		if err != nil {
			return fmt.Errorf("stat %s queue: %w", name, err)
		}
		if offset < 0 || offset > info.Size() {
			return fmt.Errorf("%s queue cursor %d exceeds file size %d", name, offset, info.Size())
		}
		if _, err := file.Seek(offset, io.SeekStart); err != nil {
			return fmt.Errorf("seek %s queue: %w", name, err)
		}
		reader := bufio.NewReader(file)
		for len(entries) < maximum {
			line, readErr := reader.ReadString('\n')
			if strings.HasSuffix(line, "\n") {
				offset += int64(len(line))
				var entry Entry
				if err := json.Unmarshal([]byte(line), &entry); err != nil {
					return fmt.Errorf("decode %s queue entry: %w", name, err)
				}
				entry.EndOffset = offset
				entries = append(entries, entry)
			}
			if readErr == nil {
				continue
			}
			if errors.Is(readErr, io.EOF) {
				return nil
			}
			return fmt.Errorf("read %s queue: %w", name, readErr)
		}
		return nil
	})
	return entries, err
}

func (q *Queue) Acknowledge(name string, endOffset int64) error {
	return q.withLock(func() error {
		state, err := q.loadState()
		if err != nil {
			return err
		}
		if endOffset < state.Offsets[name] {
			return nil
		}
		info, err := os.Stat(q.Path(name))
		if errors.Is(err, os.ErrNotExist) {
			state.Offsets[name] = 0
			return q.saveState(state)
		}
		if err != nil {
			return fmt.Errorf("stat %s queue: %w", name, err)
		}
		if endOffset > info.Size() {
			return fmt.Errorf("acknowledge %s queue beyond file size", name)
		}
		if endOffset == info.Size() {
			if err := os.Truncate(q.Path(name), 0); err != nil {
				return fmt.Errorf("truncate drained %s queue: %w", name, err)
			}
			state.Offsets[name] = 0
		} else {
			state.Offsets[name] = endOffset
		}
		return q.saveState(state)
	})
}

// RecentIDs returns IDs from the tail of a queue. Producers use this bounded
// window to avoid re-enqueueing lines after a crash between queue append and
// logfile-checkpoint persistence.
func (q *Queue) RecentIDs(name string, tailBytes int64) (map[string]struct{}, error) {
	result := make(map[string]struct{})
	err := q.withLock(func() error {
		file, err := os.Open(q.Path(name))
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		if err != nil {
			return err
		}
		defer file.Close()
		info, err := file.Stat()
		if err != nil {
			return err
		}
		start := max(info.Size()-tailBytes, 0)
		if _, err := file.Seek(start, io.SeekStart); err != nil {
			return err
		}
		reader := bufio.NewReader(file)
		if start > 0 {
			if _, err := reader.ReadString('\n'); err != nil && !errors.Is(err, io.EOF) {
				return err
			}
		}
		scanner := bufio.NewScanner(reader)
		scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
		for scanner.Scan() {
			var entry Entry
			if err := json.Unmarshal(scanner.Bytes(), &entry); err != nil {
				return err
			}
			result[entry.ID] = struct{}{}
		}
		return scanner.Err()
	})
	return result, err
}

func (q *Queue) loadState() (cursorState, error) {
	state := cursorState{Offsets: make(map[string]int64)}
	data, err := os.ReadFile(q.statePath)
	if errors.Is(err, os.ErrNotExist) {
		return state, nil
	}
	if err != nil {
		return state, fmt.Errorf("read EQLDB queue state: %w", err)
	}
	if err := json.Unmarshal(data, &state); err != nil {
		return state, fmt.Errorf("decode EQLDB queue state: %w", err)
	}
	if state.Offsets == nil {
		state.Offsets = make(map[string]int64)
	}
	return state, nil
}

func (q *Queue) saveState(state cursorState) error {
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	temporary, err := os.CreateTemp(q.directory, ".state-*.tmp")
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
	if err := temporary.Close(); err != nil {
		return err
	}
	if err := os.Rename(name, q.statePath); err != nil {
		if removeErr := os.Remove(q.statePath); removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
			return err
		}
		return os.Rename(name, q.statePath)
	}
	return nil
}

func (q *Queue) withLock(operation func() error) error {
	deadline := time.Now().Add(lockTimeout)
	for {
		file, err := os.OpenFile(q.lockPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
		if err == nil {
			_, _ = fmt.Fprintf(file, "%d\n", os.Getpid())
			_ = file.Close()
			defer os.Remove(q.lockPath)
			return operation()
		}
		if !errors.Is(err, os.ErrExist) {
			return fmt.Errorf("lock EQLDB queue: %w", err)
		}
		if info, statErr := os.Stat(q.lockPath); statErr == nil && time.Since(info.ModTime()) > staleLock {
			_ = os.Remove(q.lockPath)
			continue
		}
		if time.Now().After(deadline) {
			return errors.New("timed out waiting for EQLDB queue")
		}
		time.Sleep(20 * time.Millisecond)
	}
}
