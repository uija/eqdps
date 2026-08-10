package eqlog

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/uija/eqdps/internal/data"
)

const (
	followPollInterval       = 250 * time.Millisecond
	latestTimestampTailBytes = int64(10 * 1024)
)

var logFilenameRE = regexp.MustCompile(`^eqlog_([^_]+)_([^_]+)\.txt$`)

var (
	ErrLogNotOpen  = errors.New("logfile is not open")
	ErrParserInUse = errors.New("parser is already replaying or following")
)

type EventHandler func(*data.LogRowEvent)

type ReplayProgress struct {
	Bytes int64
	Total int64
	Lines int
}

var num_rows = 0
var used_rows = 0

type ReplayProgressHandler func(ReplayProgress)
type LandmarkHandler func(data.LogLandmark)

type Parser struct {
	mu       sync.Mutex
	path     string
	offset   int64
	metadata data.CharacterMetadata
	reading  bool
	closing  bool
	session  uint64
	stop     chan struct{}
}

func NewParser(session uint64) *Parser {
	return &Parser{
		session: session,
	}
}

func (p *Parser) Open(path string) error {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("resolve logfile path: %w", err)
	}
	file, err := os.Open(absolute)
	if err != nil {
		return fmt.Errorf("open logfile: %w", err)
	}
	info, statErr := file.Stat()
	closeErr := file.Close()
	if statErr != nil {
		return fmt.Errorf("inspect logfile: %w", statErr)
	}
	if closeErr != nil {
		return fmt.Errorf("close logfile after inspection: %w", closeErr)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("open logfile: %s is not a regular file", absolute)
	}

	character, server, err := characterIdentity(absolute)
	if err != nil {
		return err
	}

	p.mu.Lock()
	defer p.mu.Unlock()
	if p.reading {
		return ErrParserInUse
	}
	p.closing = false
	p.path = absolute
	p.offset = info.Size()
	p.metadata = data.CharacterMetadata{
		CharacterName: character,
		ServerName:    server,
	}
	return nil
}

func (p *Parser) IndexFile(onProgress ReplayProgressHandler, onLandmark LandmarkHandler) error {
	path, stop, err := p.beginRead()
	if err != nil {
		return err
	}
	defer p.endRead()

	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("Unable to open logfile for indexing. %w", err)
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return fmt.Errorf("inspect logfile for replay: %w", err)
	}

	reader := bufio.NewReader(io.LimitReader(file, info.Size()))

	var offset int64
	if onProgress != nil {
		onProgress(ReplayProgress{Total: info.Size()})
	}
	lines := 0
	for {
		select {
		case <-stop:
			return nil
		default:
		}
		line, readErr := reader.ReadString('\n')
		if len(line) > 0 {
			line = strings.TrimRight(line, "\r\n")
			lines++
			switch {
			case strings.Contains(line, "You have entered "):
				timestampText, message, ok := splitEnvelope(line)
				if !ok {
					continue
				}
				timestamp, err := time.Parse(timestampLayout, timestampText)
				if err != nil {
					log.Printf("Unable to parse timestamp. %v", err)
					continue
				}
				values := zoneChangeExpression.FindStringSubmatch(message)
				if values != nil {
					p.metadata.Zone = values[1]
					onLandmark(data.LogLandmark{
						Type:      data.LogRowEventTypeZoneChange,
						Timestamp: timestamp,
						Offset:    offset,
						Zone:      values[1],
					})
				}
			case strings.Contains(line, "You have gained a level! Welcome to level "):
				timestampText, message, ok := splitEnvelope(line)
				if !ok {
					continue
				}
				timestamp, err := time.Parse(timestampLayout, timestampText)
				if err != nil {
					log.Printf("Unable to parse timestamp. %v", err)
					continue
				}
				values := levelUpExpression.FindStringSubmatch(message)
				if values != nil {
					level, err := strconv.Atoi(values[1])
					if err != nil {
						log.Printf("Unable to parse level. %s", values[1])
						continue
					}
					p.metadata.Level = level
					onLandmark(data.LogLandmark{
						Type:      data.LogRowEventTypeLevelUp,
						Timestamp: timestamp,
						Offset:    offset,
						Level:     level,
					})
				}
			}
			offset += int64(len(line))
			if onProgress != nil && lines%10000 == 0 {
				onProgress(ReplayProgress{Bytes: offset, Total: info.Size(), Lines: lines})
			}
		}
		if readErr == nil {
			continue
		}
		if errors.Is(readErr, io.EOF) {
			if onProgress != nil {
				onProgress(ReplayProgress{Bytes: info.Size(), Total: info.Size(), Lines: lines})
			}
			return nil
		}
		return fmt.Errorf("read logfile replay: %w", readErr)
	}
}

// Replay reads rows from the end of the logfile's requested lookback window.
// A zero or negative lookback reads from the beginning.
func (p *Parser) Replay(lookback time.Duration, handler EventHandler, onProgress ReplayProgressHandler) error {
	used_rows = 0
	num_rows = 0
	path, stop, err := p.beginRead()
	if err != nil {
		return err
	}
	defer p.endRead()

	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open logfile for replay: %w", err)
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return fmt.Errorf("inspect logfile for replay: %w", err)
	}

	var cutoff time.Time
	if lookback > 0 {
		latest, stopped, err := latestTimestamp(file, info.Size(), stop)
		if err != nil {
			return err
		}
		if stopped {
			return nil
		}
		if !latest.IsZero() {
			cutoff = latest.Add(-lookback)
		}
		if _, err := file.Seek(0, io.SeekStart); err != nil {
			return fmt.Errorf("rewind logfile for replay: %w", err)
		}
	}
	p.resetReplayState()
	reader := bufio.NewReader(io.LimitReader(file, info.Size()))
	var offset int64
	lines := 0
	if onProgress != nil {
		onProgress(ReplayProgress{Total: info.Size()})
	}
	for {
		select {
		case <-stop:
			return nil
		default:
		}
		line, readErr := reader.ReadString('\n')
		if len(line) > 0 {
			offset += int64(len(line))
			lines++
			/*
				if strings.Contains(line, " tells ") {
					continue
				}
				if strings.Contains(line, " sais ") {
					continue
				}
			*/
			if cutoff.IsZero() {
				p.emit(line, offset, false, handler)
			} else if timestamp, ok := rowTimestamp(line); ok && !timestamp.Before(cutoff) {
				p.emit(line, offset, false, handler)
			} else {
				p.advanceOffset(offset)
			}
			if onProgress != nil && lines%10000 == 0 {
				onProgress(ReplayProgress{Bytes: offset, Total: info.Size(), Lines: lines})
			}
		}
		if readErr == nil {
			continue
		}
		if errors.Is(readErr, io.EOF) {
			if onProgress != nil {
				onProgress(ReplayProgress{Bytes: info.Size(), Total: info.Size(), Lines: lines})
			}
			log.Printf("Used %d out of %d filed. %d skipped", used_rows, num_rows, num_rows-used_rows)
			return nil
		}
		return fmt.Errorf("read logfile replay: %w", readErr)
	}
}

func latestTimestamp(file *os.File, size int64, stop <-chan struct{}) (time.Time, bool, error) {
	start := max(size-latestTimestampTailBytes, 0)
	if _, err := file.Seek(start, io.SeekStart); err != nil {
		return time.Time{}, false, fmt.Errorf("seek to logfile tail: %w", err)
	}
	reader := bufio.NewReader(io.LimitReader(file, size-start))
	if start > 0 {
		select {
		case <-stop:
			return time.Time{}, true, nil
		default:
		}
		if _, err := reader.ReadString('\n'); err != nil {
			if errors.Is(err, io.EOF) {
				return time.Time{}, false, nil
			}
			return time.Time{}, false, fmt.Errorf("discard partial logfile row: %w", err)
		}
	}
	var latest time.Time
	for {
		select {
		case <-stop:
			return time.Time{}, true, nil
		default:
		}
		line, readErr := reader.ReadString('\n')
		if strings.HasSuffix(line, "\n") {
			if timestamp, ok := rowTimestamp(line); ok {
				latest = timestamp
			}
		}
		if readErr == nil {
			continue
		}
		if errors.Is(readErr, io.EOF) {
			return latest, false, nil
		}
		return time.Time{}, false, fmt.Errorf("find latest logfile timestamp: %w", readErr)
	}
}

func rowTimestamp(row string) (time.Time, bool) {
	timestampText, _, ok := splitEnvelope(strings.TrimRight(row, "\r\n"))
	if !ok {
		return time.Time{}, false
	}
	timestamp, err := time.Parse(timestampLayout, timestampText)
	return timestamp, err == nil
}

func splitEnvelope(row string) (timestamp, message string, ok bool) {
	if len(row) == 0 || row[0] != '[' {
		return "", "", false
	}
	timestamp, message, ok = strings.Cut(row[1:], "] ")
	return timestamp, message, ok
}

// Follow blocks while waiting for new complete logfile rows. Call Close from
// another goroutine to stop it.
func (p *Parser) Follow(handler EventHandler) error {
	path, stop, err := p.beginRead()
	if err != nil {
		return err
	}
	defer p.endRead()

	p.mu.Lock()
	offset := p.offset
	p.mu.Unlock()

	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open logfile for following: %w", err)
	}
	defer file.Close()
	if _, err := file.Seek(offset, io.SeekStart); err != nil {
		return fmt.Errorf("seek logfile: %w", err)
	}

	reader := bufio.NewReader(file)
	ticker := time.NewTicker(followPollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-stop:
			return nil
		default:
		}

		line, readErr := reader.ReadString('\n')
		if strings.HasSuffix(line, "\n") {
			offset += int64(len(line))
			p.emit(line, offset, true, handler)
		}
		if readErr == nil {
			continue
		}
		if !errors.Is(readErr, io.EOF) {
			return fmt.Errorf("follow logfile: %w", readErr)
		}
		if len(line) > 0 {
			if _, err := file.Seek(offset, io.SeekStart); err != nil {
				return fmt.Errorf("rewind partial logfile row: %w", err)
			}
			reader.Reset(file)
		}

		select {
		case <-stop:
			return nil
		case <-ticker.C:
		}
	}
}

func (p *Parser) Close() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.closing = true
	if p.stop != nil {
		close(p.stop)
		p.stop = nil
	}
	if !p.reading {
		p.resetOpenState()
	}
}

func (p *Parser) ParseRow(row string, endOffset int64, live bool) (*data.LogRowEvent, bool) {
	num_rows++
	row = strings.TrimRight(row, "\r\n")
	timestampText, message, validEnvelope := splitEnvelope(row)

	p.mu.Lock()
	defer p.mu.Unlock()
	p.offset = endOffset
	if !validEnvelope {
		return nil, false
	}

	eventType, eventData, known := classify(message)
	if !known {
		return nil, false
	}
	timestamp, err := time.Parse(timestampLayout, timestampText)
	if err != nil {
		return nil, false
	}
	used_rows++
	p.updateMetadata(eventType, eventData, timestamp)
	event := &data.LogRowEvent{
		Session:   p.session,
		Timestamp: timestamp,
		Offset:    endOffset,
		Message:   message,
		Live:      live,
		Type:      eventType,
		Data:      eventData,
		Metadata:  cloneMetadata(p.metadata),
	}
	return event, true
}

func (p *Parser) emit(row string, endOffset int64, live bool, handler EventHandler) {
	event, ok := p.ParseRow(row, endOffset, live)
	if ok && handler != nil {
		handler(event)
	}
}

func (p *Parser) advanceOffset(offset int64) {
	p.mu.Lock()
	p.offset = offset
	p.mu.Unlock()
}

func (p *Parser) beginRead() (string, <-chan struct{}, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.path == "" {
		return "", nil, ErrLogNotOpen
	}
	if p.reading {
		return "", nil, ErrParserInUse
	}
	p.reading = true
	p.stop = make(chan struct{})
	return p.path, p.stop, nil
}

func (p *Parser) endRead() {
	p.mu.Lock()
	p.reading = false
	p.stop = nil
	if p.closing {
		p.resetOpenState()
	}
	p.mu.Unlock()
}

func (p *Parser) resetOpenState() {
	p.path = ""
	p.offset = 0
	p.metadata = data.CharacterMetadata{}
	p.closing = false
}

func (p *Parser) resetReplayState() {
	p.mu.Lock()
	p.offset = 0
	p.metadata.Level = 0
	p.metadata.Classes = nil
	p.metadata.Race = ""
	p.metadata.WhoObservedAt = time.Time{}
	p.mu.Unlock()
}

func (p *Parser) updateMetadata(eventType data.LogRowEventType, eventData []string, timestamp time.Time) {
	switch eventType {
	case data.LogRowEventTypeWho:
		if len(eventData) < 5 || !strings.EqualFold(strings.TrimSpace(eventData[3]), p.metadata.CharacterName) {
			return
		}
		level, err := strconv.Atoi(eventData[1])
		if err != nil || level < 1 {
			return
		}
		p.metadata.Level = level
		p.metadata.Classes = strings.Split(eventData[2], "/")
		p.metadata.Race = strings.TrimSpace(eventData[4])
		p.metadata.WhoObservedAt = timestamp
	case data.LogRowEventTypeLevelUp:
		if len(eventData) < 2 {
			return
		}
		level, err := strconv.Atoi(eventData[1])
		if err == nil && level > 0 {
			p.metadata.Level = level
		}
	}
}

func cloneMetadata(metadata data.CharacterMetadata) data.CharacterMetadata {
	metadata.Classes = append([]string(nil), metadata.Classes...)
	return metadata
}

func characterIdentity(path string) (string, string, error) {
	matches := logFilenameRE.FindStringSubmatch(filepath.Base(path))
	if matches == nil {
		return "", "", fmt.Errorf("derive character and server: expected eqlog_CHARACTER_SERVER.txt, got %q", filepath.Base(path))
	}
	return matches[1], matches[2], nil
}
