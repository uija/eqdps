// Package inventorysync correlates live EverQuest /who results with inventory
// export completion messages.
package inventorysync

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/uija/eqdps/internal/eqlog"
)

const WhoMaxAge = time.Minute

var logNameRE = regexp.MustCompile(`^eqlog_([^_]+)_([^_]+)\.txt$`)

type Metadata struct {
	Level      int
	Classes    []string
	Race       string
	ObservedAt time.Time
}

type Request struct {
	Path     string
	Filename string
	Exported time.Time
	Metadata *Metadata
}

type Observer struct {
	mu               sync.Mutex
	logPath          string
	character        string
	expectedFilename string
	lastWho          eqlog.WhoResult
}

func NewObserver(logPath string) (*Observer, error) {
	character, server, err := CharacterIdentity(logPath)
	if err != nil {
		return nil, err
	}
	return &Observer{
		logPath:          filepath.Clean(logPath),
		character:        character,
		expectedFilename: character + "_" + server + "-Inventory.txt",
	}, nil
}

func CharacterIdentity(logPath string) (string, string, error) {
	base := filepath.Base(logPath)
	matches := logNameRE.FindStringSubmatch(base)
	if matches == nil {
		return "", "", fmt.Errorf("derive character and server: expected eqlog_CHARACTER_SERVER.txt, got %q", base)
	}
	return matches[1], matches[2], nil
}

func (o *Observer) Observe(record eqlog.Record) (Request, bool) {
	o.mu.Lock()
	defer o.mu.Unlock()

	switch record.Kind {
	case eqlog.RecordWho:
		if strings.EqualFold(record.Who.Name, o.character) {
			o.lastWho = cloneWho(record.Who)
		}
	case eqlog.RecordInventoryExport:
		return o.exportRequest(record.Export)
	}
	return Request{}, false
}

func (o *Observer) exportRequest(export eqlog.InventoryExport) (Request, bool) {
	if strings.ContainsAny(export.Filename, `/\`) {
		return Request{}, false
	}
	if !strings.EqualFold(export.Filename, o.expectedFilename) {
		return Request{}, false
	}
	root := filepath.Dir(filepath.Dir(o.logPath))
	request := Request{
		Path:     filepath.Join(root, export.Filename),
		Filename: export.Filename,
		Exported: export.Time,
	}
	age := export.Time.Sub(o.lastWho.Time)
	if o.lastWho.Time.IsZero() || o.lastWho.Anonymous || o.lastWho.Level < 1 || len(o.lastWho.Classes) == 0 || o.lastWho.Race == "" || age < 0 || age > WhoMaxAge {
		return request, true
	}
	request.Metadata = &Metadata{
		Level:      o.lastWho.Level,
		Classes:    append([]string(nil), o.lastWho.Classes...),
		Race:       o.lastWho.Race,
		ObservedAt: o.lastWho.Time,
	}
	return request, true
}

func cloneWho(who eqlog.WhoResult) eqlog.WhoResult {
	who.Classes = append([]string(nil), who.Classes...)
	return who
}
