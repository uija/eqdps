package ach

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"

	"gioui.org/layout"
	"gioui.org/widget"
	achievments "github.com/uija/eqdps/internal/achievements"
	"github.com/uija/eqdps/internal/data"
	"github.com/uija/eqdps/internal/module"
	"github.com/uija/eqdps/internal/native"
)

type Module struct {
	ctx     *module.Context
	mu      sync.Mutex
	loading atomic.Bool
	replay  atomic.Bool
	stop    chan struct{}

	characterName string
	serverName    string
	exportPath    string

	data          *achievments.Export
	filtered_data *achievments.Export

	list widget.List

	macro_copy_click widget.Clickable
	filter           widget.Editor
	filter_clear     widget.Clickable
}

func NewModule() *Module {
	m := Module{
		stop: make(chan struct{}, 1),
	}
	return &m
}

func (m *Module) Init(ctx *module.Context, invalidate func()) error {
	m.ctx = ctx
	ctx.AddViewMenuItem("Achievements", m.OpenMainView)
	ctx.AddSidebarItem("Achiev", m.OpenMainView)
	ctx.RegisterLogOpen(m.OnLogOpen)
	ctx.RegisterLogRow(m.OnLogRow)
	ctx.RegisterReplayStart(m.OnReplayStart)
	ctx.RegisterReplayEnd(m.OnReplayEnd)
	ctx.RegisterUpdate(m.Update)
	m.list.Axis = layout.Vertical
	m.filter.SingleLine = true
	return nil
}
func (m *Module) OpenMainView() {
	m.ctx.SetMainView(m.Layout)
}
func (m *Module) OnLogRow(e *data.LogRowEvent) {
	if m.replay.Load() {
		return
	}
	if m.loading.Load() {
		return
	}
	if e.Type == data.LogRowEventTypeAchievementExport {
		go m.ParseExport(e.Data[1])
	}
}
func (m *Module) ParseExport(filename string) {
	if m.exportPath == "" {
		log.Printf("No export path!")
		return
	}
	if !m.loading.CompareAndSwap(false, true) {
		log.Printf("Import already running")
		return
	}
	defer m.loading.Store(false)
	m.mu.Lock()
	m.data = nil
	m.mu.Unlock()
	path := filepath.Join(m.exportPath, filename)
	data, err := achievments.Parse(path)
	if err != nil {
		log.Printf("Unable to parse achievment file. %v", err)
		return
	}
	bytes, err := json.Marshal(data)
	if err != nil {
		log.Printf("Unable to marshal achievement data. %v", err)
		return
	}
	jsonPath := m.AchievementFilepath()
	err = os.WriteFile(jsonPath, bytes, 0o644)
	if err != nil {
		log.Printf("Unable to save json file. %v", err)
		return
	}
	m.mu.Lock()
	m.data = data
	m.mu.Unlock()
}
func (m *Module) Update(gtx layout.Context) {
	if m.macro_copy_click.Clicked(gtx) {
		native.CopyTextToClipboard(gtx, "/outputfile achievements")
	}
	if m.loading.Load() {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.filtered_data == nil {
		return
	}
	for ci, c := range m.filtered_data.Categories {
		for si, s := range c.Subcategories {
			for i := range s.Achievements {
				if m.filtered_data.Categories[ci].Subcategories[si].Achievements[i].Click.Clicked(gtx) {
					m.filtered_data.Categories[ci].Subcategories[si].Achievements[i].Open = !m.filtered_data.Categories[ci].Subcategories[si].Achievements[i].Open
				}
			}
		}
	}
	if m.filter_clear.Clicked(gtx) {
		if m.filter.Text() != "" {
			m.filter.SetText("")
			m.filtered_data = nil
		}
	}
	if event, ok := m.filter.Update(gtx); ok {
		if _, changed := event.(widget.ChangeEvent); changed {
			m.filtered_data = nil
		}
	}
}
func (m *Module) AchievementFilepath() string {
	return filepath.Join(filepath.Join(m.exportPath, "Logs"), fmt.Sprintf("eqdps_%s_%s_Achievements.json", m.characterName, m.serverName))
}
func (m *Module) OnLogOpen(characterName string, serverName string, filesize int64, path string) bool {
	logPath := filepath.Dir(path)
	m.exportPath = filepath.Dir(logPath)
	m.characterName = characterName
	m.serverName = serverName
	jsonPath := m.AchievementFilepath()
	if m.loading.CompareAndSwap(false, true) {
		go func() {
			defer m.loading.Store(false)
			bytes, err := os.ReadFile(jsonPath)
			if err != nil {
				if !os.IsNotExist(err) {
					log.Printf("Unable to read json file. %v", err)
				}
				return
			}
			var data achievments.Export
			err = json.Unmarshal(bytes, &data)
			if err != nil {
				log.Printf("Unable to parse json. %v", err)
				return
			}
			m.mu.Lock()
			m.data = &data
			m.mu.Unlock()
		}()
	}
	return true
}
func (m *Module) OnReplayStart() {
	m.replay.Store(true)
}
func (m *Module) OnReplayEnd() {
	m.replay.Store(false)
}

func (m *Module) Shutdown() {
	select {
	case m.stop <- struct{}{}:
	default:
	}
}
