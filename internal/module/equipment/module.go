package equipment

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"sync/atomic"

	"gioui.org/layout"
	"gioui.org/widget"
	"github.com/uija/eqdps/internal/data"
	"github.com/uija/eqdps/internal/inventory"
	"github.com/uija/eqdps/internal/module"
	"github.com/uija/eqdps/internal/ui/form"
)

type ClickableItems struct {
	Item  inventory.ItemData
	Stats map[string]float64
	Click widget.Clickable
}
type Module struct {
	ctx          *module.Context
	mu           sync.Mutex
	replay       atomic.Bool
	configPath   string
	databasePath string

	inv       *inventory.Inventory
	items     []ClickableItems
	itemsList widget.List
	nameSort  widget.Clickable
	statSort  widget.Clickable

	hide_worn bool

	importRunning atomic.Bool
	importDone    chan *inventory.Inventory

	stop           chan struct{}
	invalidateFunc func()

	filter              widget.Editor
	filter_clear        widget.Clickable
	class1              *form.SelectBox
	class2              *form.SelectBox
	class3              *form.SelectBox
	slots               *form.SelectBox
	stats               *form.SelectBox
	exaltation_checkbox widget.Bool
	hide_exaltations    bool

	selected_item       *inventory.ItemData
	selected_item_click widget.Clickable
}

func NewModule() *Module {
	return &Module{
		importDone:     make(chan *inventory.Inventory, 1),
		stop:           make(chan struct{}, 1),
		invalidateFunc: func() {},
		class1:         form.NewSelectBox([]string{}, 0),
		class2:         form.NewSelectBox([]string{}, 0),
		class3:         form.NewSelectBox([]string{}, 0),
		slots:          form.NewSelectBox([]string{}, 0),
		stats:          form.NewSelectBox([]string{}, 0),
	}
}
func (m *Module) Init(ctx *module.Context, invFunc func()) error {
	m.ctx = ctx
	m.ctx.RegisterLogOpen(m.OnLogOpen)
	m.ctx.RegisterLogRow(m.OnLogRow)
	m.ctx.RegisterReplayStart(m.OnReplayStart)
	m.ctx.RegisterReplayEnd(m.OnReplayEnd)
	m.ctx.AddSidebarItem("Equip", m.OpenMainView)
	m.ctx.AddViewMenuItem("Equipment", m.OpenMainView)
	m.ctx.RegisterUpdate(m.Update)
	m.itemsList.Axis = layout.Vertical
	m.invalidateFunc = invFunc
	m.exaltation_checkbox.Value = false
	m.hide_exaltations = false

	go func() {
		for {
			select {
			case <-m.stop:
				return
			case ptr := <-m.importDone:
				m.mu.Lock()
				m.inv = ptr
				m.items = nil
				m.mu.Unlock()
				m.invalidateFunc()
			}
		}
	}()

	return nil
}
func (m *Module) OpenMainView() {
	m.ctx.SetMainView(m.Layout)
}
func (m *Module) Update(gtx layout.Context) {
	m.class1.Update(gtx)
	m.class2.Update(gtx)
	m.class3.Update(gtx)
	m.slots.Update(gtx)
	m.stats.Update(gtx)
	if m.stats.Changed() || m.class1.Changed() || m.class2.Changed() || m.class3.Changed() || m.slots.Changed() {
		m.PrepareItems()
		m.invalidateFunc()
	}
	if m.exaltation_checkbox.Update(gtx) {
		if m.hide_exaltations != m.exaltation_checkbox.Value {
			m.hide_exaltations = m.exaltation_checkbox.Value
			m.PrepareItems()
			m.invalidateFunc()
		}
	}
	if event, ok := m.filter.Update(gtx); ok {
		if _, changed := event.(widget.ChangeEvent); changed {
			m.PrepareItems()
			m.invalidateFunc()
		}
	}
	if m.filter_clear.Clicked(gtx) {
		m.filter.SetText("")
		m.PrepareItems()
		m.invalidateFunc()
	}
	if m.nameSort.Clicked(gtx) {
		sort.SliceStable(m.items, func(i, j int) bool {
			return m.items[i].Item.Name < m.items[j].Item.Name
		})
	}
	if m.statSort.Clicked(gtx) {
		if selectedStat := m.stats.Value(); selectedStat != "" {
			sort.SliceStable(m.items, func(i, j int) bool {
				left := m.items[i].Stats[selectedStat]
				right := m.items[j].Stats[selectedStat]
				if left == right {
					return m.items[i].Item.Name < m.items[j].Item.Name
				}
				return left > right
			})
		}
	}
	for idx, item := range m.items {
		if m.items[idx].Click.Clicked(gtx) {
			if item.Item.Metadata != nil && item.Item.Metadata["statsblock"] != "" {
				m.selected_item = &item.Item
				m.invalidateFunc()
			}
		}
	}
	if m.selected_item_click.Clicked(gtx) {
		m.selected_item = nil
		m.invalidateFunc()
	}
}
func (m *Module) Shutdown() {
	m.stop <- struct{}{}
}
func (m *Module) OnLogOpen(characterName, serverName string, filesize int64, path string) bool {
	m.configPath = path
	m.databasePath = filepath.Join(filepath.Dir(path), fmt.Sprintf("eqdps_%s_%s_inventory.json", characterName, serverName))

	m.mu.Lock()
	m.inv = nil
	m.items = nil
	m.mu.Unlock()

	go func() {
		if !m.importRunning.CompareAndSwap(false, true) {
			log.Printf("already running")
			return
		}
		defer m.importRunning.Store(false)
		bytes, err := os.ReadFile(m.databasePath)
		if err != nil {
			if os.IsNotExist(err) {
				log.Printf("File does not exists.")
				return
			}
			log.Printf("Unable to load inventory json. %v", err)
			return
		}
		var inv inventory.Inventory
		err = json.Unmarshal(bytes, &inv)
		if err != nil {
			log.Printf("Unable to unmarshal inventory file. %v", err)
		}
		select {
		case m.importDone <- &inv:
		case <-m.stop:
		}
	}()
	return true
}
func (m *Module) OnLogRow(event *data.LogRowEvent) {
	if m.databasePath == "" {
		return
	}
	if m.replay.Load() {
		return
	}
	switch event.Type {
	case data.LogRowEventTypeInventoryExport:
		if !m.importRunning.CompareAndSwap(false, true) {
			return
		}
		exportPath := filepath.Join(
			filepath.Dir(filepath.Dir(m.configPath)),
			event.Data[1],
		)
		go func() {
			defer m.importRunning.Store(false)
			inv, err := m.ImportInventory(exportPath, m.databasePath)
			if err != nil {
				log.Printf("Unable to import Inventory. %v", err)
				return
			}
			select {
			case m.importDone <- inv:
			case <-m.stop:
			}
		}()
	}
}
func (m *Module) OnReplayStart() { m.replay.Store(true) }
func (m *Module) OnReplayEnd()   { m.replay.Store(false) }
