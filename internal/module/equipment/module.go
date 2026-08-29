package equipment

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
	"github.com/uija/eqdps/internal/data"
	"github.com/uija/eqdps/internal/inventory"
	"github.com/uija/eqdps/internal/module"
	"github.com/uija/eqdps/internal/ui/form"
)

type ClickableItems struct {
	Item  inventory.ItemData
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
	}
}
func (m *Module) Init(ctx *module.Context, invFunc func()) error {
	m.ctx = ctx
	m.ctx.RegisterLogOpen(m.OnLogOpen)
	m.ctx.RegisterLogRow(m.OnLogRow)
	m.ctx.AddSidebarItem("Equip", func() {
		ctx.SetMainView(m.Layout)
	})
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
func (m *Module) Update(gtx layout.Context) {
	m.class1.Update(gtx)
	m.class2.Update(gtx)
	m.class3.Update(gtx)
	m.slots.Update(gtx)
	if m.class1.Changed() || m.class2.Changed() || m.class3.Changed() || m.slots.Changed() {
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
	if !m.importRunning.CompareAndSwap(false, true) {
		return
	}
	switch event.Type {
	case data.LogRowEventTypeInventoryExport:
		exportPath := filepath.Join(
			filepath.Dir(filepath.Dir(m.configPath)),
			event.Data[1],
		)
		log.Printf("Inventory export found at %s", exportPath)
		go func() {
			defer m.importRunning.Store(false)

			log.Printf("Calling ImportInventory")
			inv, err := m.ImportInventory(exportPath, m.databasePath)
			if err != nil {
				log.Printf("Unable to import Inventory. %v", err)
				return
			}
			select {
			case m.importDone <- inv:
				log.Printf("Sent it!")
			case <-m.stop:
			}
			log.Printf("done!")
		}()
	}
}
func (m *Module) OnReplayStart() { m.replay.Store(true) }
func (m *Module) OnReplayEnd()   { m.replay.Store(false) }
