// Package engine coordinates logfile input with the reusable combat and XP
// models. It deliberately contains no terminal UI dependencies so another
// frontend can consume the same replay and live-tail behavior.
package engine

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/uija/eqdps/internal/combat"
	"github.com/uija/eqdps/internal/eqlog"
	"github.com/uija/eqdps/internal/xp"
)

type ReplayProgress struct {
	Bytes int64
	Total int64
	Lines int
}

var ErrReplayCancelled = errors.New("replay cancelled")

func Replay(logPath string, idleTimeout, back time.Duration, since time.Time, historyLimit int) (*combat.FightTracker, *xp.Session, error) {
	return ReplayWithProgress(logPath, idleTimeout, back, since, historyLimit, 0, nil, nil)
}

func ReplayWithProgress(logPath string, idleTimeout, back time.Duration, since time.Time, historyLimit int, maxBytes int64, onProgress func(ReplayProgress), cancel <-chan struct{}) (*combat.FightTracker, *xp.Session, error) {
	cutoff, err := ReplayCutoffWithCancel(logPath, back, since, cancel)
	if err != nil {
		return nil, nil, err
	}
	file, err := os.Open(logPath)
	if err != nil {
		return nil, nil, fmt.Errorf("open log: %w", err)
	}
	defer file.Close()
	if maxBytes <= 0 {
		info, statErr := file.Stat()
		if statErr != nil {
			return nil, nil, fmt.Errorf("stat log: %w", statErr)
		}
		maxBytes = info.Size()
	}

	tracker := combat.NewFightTrackerWithHistory(historyLimit)
	xpSession := xp.NewSession()
	var latest time.Time
	var bytesRead int64
	linesRead := 0
	scanner := bufio.NewScanner(io.LimitReader(file, maxBytes))
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		if linesRead%1000 == 0 && cancelled(cancel) {
			return nil, nil, ErrReplayCancelled
		}
		line := scanner.Text()
		bytesRead += int64(len(scanner.Bytes()) + 1)
		linesRead++
		record, hasTimestamp := eqlog.ParseRecordAfter(line, cutoff)
		if hasTimestamp && record.Time.After(latest) {
			latest = record.Time
		}
		if onProgress != nil && linesRead%5000 == 0 {
			onProgress(ReplayProgress{Bytes: min(bytesRead, maxBytes), Total: maxBytes, Lines: linesRead})
		}
		if hasTimestamp {
			ProcessRecord(record, tracker, xpSession, idleTimeout)
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, nil, fmt.Errorf("read log: %w", err)
	}
	if !latest.IsZero() {
		tracker.EndIdleAtLogTime(latest, idleTimeout)
	}
	if cancelled(cancel) {
		return nil, nil, ErrReplayCancelled
	}
	if onProgress != nil {
		onProgress(ReplayProgress{Bytes: maxBytes, Total: maxBytes, Lines: linesRead})
	}
	return tracker, xpSession, nil
}

func cancelled(cancel <-chan struct{}) bool {
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

func ReplayCutoff(logPath string, back time.Duration, since time.Time) (time.Time, error) {
	return ReplayCutoffWithCancel(logPath, back, since, nil)
}

func ReplayCutoffWithCancel(logPath string, back time.Duration, since time.Time, cancel <-chan struct{}) (time.Time, error) {
	if !since.IsZero() {
		return since, nil
	}
	if back <= 0 {
		return time.Time{}, nil
	}
	file, err := os.Open(logPath)
	if err != nil {
		return time.Time{}, fmt.Errorf("open log: %w", err)
	}
	defer file.Close()

	var latest time.Time
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	linesRead := 0
	for scanner.Scan() {
		if linesRead%1000 == 0 && cancelled(cancel) {
			return time.Time{}, ErrReplayCancelled
		}
		linesRead++
		if timestamp, ok := eqlog.ParseTime(scanner.Text()); ok && timestamp.After(latest) {
			latest = timestamp
		}
	}
	if err := scanner.Err(); err != nil {
		return time.Time{}, fmt.Errorf("read log: %w", err)
	}
	if latest.IsZero() {
		return time.Time{}, nil
	}
	return latest.Add(-back), nil
}

func ProcessLine(line string, tracker *combat.FightTracker, xpSession *xp.Session, idleTimeout time.Duration) {
	record, ok := eqlog.ParseRecord(line)
	if ok {
		ProcessRecord(record, tracker, xpSession, idleTimeout)
	}
}

func ProcessRecord(record eqlog.Record, tracker *combat.FightTracker, xpSession *xp.Session, idleTimeout time.Duration) {
	xpSession.Observe(record.Time, time.Now())
	switch record.Kind {
	case eqlog.RecordCast:
		tracker.AddCast(record.Cast)
	case eqlog.RecordDamage:
		xpSession.AddCombat(record.Damage.Time)
		tracker.AddDamageWithIdle(record.Damage, idleTimeout)
	case eqlog.RecordExperience:
		xpSession.AddGain(record.Experience.Time, record.Experience.Percent)
	case eqlog.RecordLevelUp:
		xpSession.AddLevelUp(record.LevelUp.Time)
	case eqlog.RecordAggroClear:
		tracker.ForgetEnemies(record.Time)
	case eqlog.RecordDeath:
		tracker.AddDeath(record.Death)
	}
}

func Follow(logPath string, startOffset int64, done <-chan struct{}, onLine func(string, int64)) error {
	file, err := os.Open(logPath)
	if err != nil {
		return fmt.Errorf("open log: %w", err)
	}
	defer file.Close()
	if _, err := file.Seek(startOffset, io.SeekStart); err != nil {
		return fmt.Errorf("seek log checkpoint: %w", err)
	}

	reader := bufio.NewReader(file)
	offset := startOffset
	for {
		select {
		case <-done:
			return nil
		default:
		}
		line, err := reader.ReadString('\n')
		if strings.HasSuffix(line, "\n") {
			offset += int64(len(line))
			onLine(line, offset)
		}
		if err == nil {
			continue
		}
		if errors.Is(err, io.EOF) {
			if len(line) > 0 {
				if _, seekErr := file.Seek(offset, io.SeekStart); seekErr != nil {
					return fmt.Errorf("rewind partial log line: %w", seekErr)
				}
				reader.Reset(file)
			}
			time.Sleep(250 * time.Millisecond)
			continue
		}
		return fmt.Errorf("read log: %w", err)
	}
}
