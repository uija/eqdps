package statistics

import (
	"fmt"
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

	mobs map[string]int
}

func NewModule() *Module {
	return &Module{
		mobs: make(map[string]int),
	}
}
func (m *Module) Init(ctx *module.Context, _ func()) error {
	ctx.RegisterLogRow(m.OnLogRow)
	ctx.RegisterUpdate(m.Update)
	ctx.RegisterReplayStart(m.OnReplayStart)
	ctx.RegisterReplayEnd(m.OnReplayEnd)
	ctx.AddSidebarItem("Stats", func() {
		ctx.SetMainView(m.Layout)
	})
	m.ctx = ctx
	m.list.Axis = layout.Vertical
	return nil
}

func (m *Module) Update(gtx layout.Context) {
}
func (m *Module) OnLogRow(e *data.LogRowEvent) {
	m.mu.Lock()
	defer m.mu.Unlock()
	switch e.Type {
	case data.LogRowEventTypeYouSlain,
		data.LogRowEventTypeSlainBy:
		who := e.Data[1]
		m.mobs[who]++
	}
}
func (m *Module) OnReplayStart() {
	m.mobs = make(map[string]int)
	m.replay.Store(true)
}
func (m *Module) OnReplayEnd() {
	m.replay.Store(false)
}
func (m *Module) Shutdown() {

}

func (m *Module) Layout(style *ui.Style, gtx layout.Context) layout.Dimensions {
	if m.replay.Load() {
		return layout.Dimensions{}
	}
	names := make([]string, len(m.mobs))
	mobs := make(map[string]int, len(m.mobs))
	m.mu.RLock()
	for name, count := range mobs {
		names = append(names, name)
		mobs[name] = count
	}
	m.mu.RUnlock()
	sort.Slice(names, func(i, j int) bool {
		if mobs[names[i]] == mobs[names[j]] {
			return names[i] < names[j]
		}
		return mobs[names[i]] > mobs[names[j]]
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
							fmt.Sprintf("%d", mobs[names[index]]),
						).Layout,
					),
				)
			})
		},
	)
}
