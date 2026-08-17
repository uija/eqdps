package statistics

import (
	"fmt"
	"log"
	"path/filepath"
	"sort"
	"sync"
	"sync/atomic"

	"gioui.org/layout"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"github.com/uija/eqdps/internal/data"
	"github.com/uija/eqdps/internal/module"
	"github.com/uija/eqdps/internal/ui"
)

type Module struct {
	mu     sync.RWMutex
	ctx    *module.Context
	list   widget.List
	replay atomic.Bool

	configPath  string
	readyToRead bool

	config Config
}

func NewModule() *Module {
	return &Module{}
}
func (m *Module) Init(ctx *module.Context, _ func()) error {
	ctx.RegisterLogRow(m.OnLogRow)
	ctx.RegisterUpdate(m.Update)
	ctx.RegisterReplayStart(m.OnReplayStart)
	ctx.RegisterReplayEnd(m.OnReplayEnd)
	ctx.RegisterLogOpen(m.OnLogOpen)
	ctx.AddSidebarItem("Stats", func() {
		ctx.SetMainView(m.Layout)
	})
	m.ctx = ctx
	m.readyToRead = false
	m.list.Axis = layout.Vertical
	return nil
}

func (m *Module) Update(gtx layout.Context) {
}
func (m *Module) OnLogRow(e *data.LogRowEvent) {
	if e.Offset < m.config.Log.Offset || !m.readyToRead {
		return
	}
	if !m.readyToRead && e.Offset >= m.config.Log.Offset {
		m.readyToRead = true
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	switch e.Type {
	case data.LogRowEventTypeYouSlain,
		data.LogRowEventTypeSlainBy:
		who := e.Data[1]
		m.config.Mobs[who]++
	}
}
func (m *Module) OnLogOpen(characterName string, serverName string, size int64, path string) bool {
	// Extract path
	base_path := filepath.Dir(path)
	m.configPath = fmt.Sprintf("%s/eqdps_%s_%s_Stats.json", base_path, characterName, serverName)
	config, err := LoadConfig(m.configPath)
	if err != nil {
		log.Printf("Error loading pos file at %s, %v", m.configPath, err)
		return false
	}
	m.config = config
	m.readyToRead = true
	return true
}
func (m *Module) OnReplayStart() {
	m.replay.Store(true)
}
func (m *Module) OnReplayEnd() {
	m.replay.Store(false)
}
func (m *Module) Shutdown() {
	m.config.Save()
}

func (m *Module) Layout(style *ui.Style, gtx layout.Context) layout.Dimensions {
	if m.replay.Load() {
		return layout.Dimensions{}
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	names := make([]string, 0)
	for name := range m.config.Mobs {
		names = append(names, name)
	}
	sort.Slice(names, func(i, j int) bool {
		if m.config.Mobs[names[i]] == m.config.Mobs[names[j]] {
			return names[i] < names[j]
		}
		return m.config.Mobs[names[i]] > m.config.Mobs[names[j]]
	})
	list := material.List(style.Theme, &m.list)
	return list.Layout(
		gtx,
		len(names),
		func(gtx layout.Context, index int) layout.Dimensions {
			color := style.Palette.Window
			if index%2 == 0 {
				color = style.Palette.Panel
			}
			return ui.ColoredRow(gtx, color, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
					layout.Flexed(1, material.Body1(style.Theme, names[index]).Layout),
					layout.Rigid(
						material.Body1(
							style.Theme,
							fmt.Sprintf("%d", m.config.Mobs[names[index]]),
						).Layout,
					),
				)
			})
		},
	)
}
