package events

import (
	"encoding/json"
	"log"
	"os"
	"slices"
	"time"

	"github.com/ncruces/zenity"
	"github.com/uija/eqdps/internal/data"
)

type EventExport struct {
	Format  string             `json:"format"`
	Version int                `json:"version"`
	Events  []data.EventConfig `json:"events"`
}

const EVENTS_FORMAT = "eqdps-events"

func (m *Module) ExportEvents() {
	defer m.export_running.Store(false)
	path, err := zenity.SelectFileSave(
		zenity.Title("Export Events"),
		zenity.Filename("eqdps-events.json"),
		zenity.ConfirmOverwrite(),
		zenity.FileFilters{
			{
				Name:     "eqdps Events",
				Patterns: []string{"*.json"},
			},
		},
	)
	if err != nil {
		log.Printf("Error selecting save file. %v", err)
		return
	}
	log.Printf("Selected file: %s", path)

	m.mu.Lock()
	events := slices.Clone(m.ctx.Config.Events)
	m.mu.Unlock()

	export := EventExport{
		Format:  EVENTS_FORMAT,
		Version: 1,
		Events:  events,
	}
	bytes, err := json.Marshal(export)
	if err != nil {
		log.Printf("Error marshalling json. %v", err)
		return
	}
	err = os.WriteFile(path, bytes, 0644)
	if err != nil {
		log.Printf("Error writing json. %v", err)
		return
	}
	m.export_success = time.Now()
}
func (m *Module) ImportEvents() {
	defer m.export_running.Store(false)
	path, err := zenity.SelectFile(
		zenity.Title("Import Events"),
		zenity.ConfirmOverwrite(),
		zenity.FileFilters{
			{
				Name:     "eqdps Events",
				Patterns: []string{"*.json"},
			},
		},
	)
	if err != nil {
		log.Printf("Error selecting save file. %v", err)
		return
	}
	log.Printf("Selected file: %s", path)
	bytes, err := os.ReadFile(path)
	if err != nil {
		log.Printf("Unable to read file")
		return
	}
	events := EventExport{}
	err = json.Unmarshal(bytes, &events)
	if err != nil {
		log.Printf("Unable to parse json. %v", err)
		return
	}
	if events.Format != EVENTS_FORMAT {
		log.Printf("Wrong file selected. %v", err)
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	importSuccess := ImportSuccess{
		Timestamp: time.Now(),
	}
	for _, e := range events.Events {
		if slices.ContainsFunc(m.ctx.Config.Events, func(ev data.EventConfig) bool {
			if e.Title == ev.Title {
				return true
			}
			return false
		}) {
			importSuccess.Skipped++
		} else {
			m.ctx.Config.Events = append(m.ctx.Config.Events, e)
			importSuccess.Imported++
		}
	}
	if importSuccess.Imported > 0 {
		m.ctx.Config.Save()
	}
	m.import_success = &importSuccess
}
