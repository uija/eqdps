package macros

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync/atomic"

	"gioui.org/layout"
	"gioui.org/widget"
	"github.com/uija/eqdps/internal/module"
	"github.com/uija/eqdps/internal/native"
)

var macroLineRE = regexp.MustCompile(
	`^Page([0-9]+)Button([0-9]+)(Name|Color|Line)([0-9]*)=(.*)$`,
)

type Macro struct {
	Name     string
	Location string
	Loadout  int
	Rows     [5]string
	Click    widget.Clickable
}

type Module struct {
	ctx         *module.Context
	loading     atomic.Bool
	macros      []Macro
	list_macros []Macro
	config_path string
	config_name string
	stop        chan struct{}

	overlay_click      widget.Clickable
	shadow_click       widget.Clickable
	line_copy_click    []widget.Clickable
	filter_editor      widget.Editor
	filter_clear_click widget.Clickable
	selected_index     int

	list widget.List
}

func NewModule() *Module {
	m := Module{
		stop:            make(chan struct{}, 1),
		line_copy_click: make([]widget.Clickable, 5),
	}
	return &m
}

func (m *Module) Init(ctx *module.Context, invalidate func()) error {
	m.ctx = ctx
	ctx.AddViewMenuItem("Macros", m.OpenMainView)
	ctx.AddSidebarItem("Macros", m.OpenMainView)
	ctx.RegisterLogOpen(m.OnLogOpen)
	ctx.RegisterUpdate(m.Update)
	m.list.Axis = layout.Vertical
	m.selected_index = -1
	m.filter_editor.SingleLine = true
	return nil
}
func (m *Module) OpenMainView() {
	m.ctx.SetMainView(m.Layout)
}
func (m *Module) Update(gtx layout.Context) {
	for idx := range m.list_macros {
		if m.list_macros[idx].Click.Clicked(gtx) {
			m.selected_index = idx
		}
	}
	if event, ok := m.filter_editor.Update(gtx); ok {
		if _, changed := event.(widget.ChangeEvent); changed {
			m.FilterList()
		}
	}
	if m.filter_clear_click.Clicked(gtx) {
		m.filter_editor.SetText("")
		m.FilterList()
	}
	if m.overlay_click.Clicked(gtx) || m.shadow_click.Clicked(gtx) {
		m.selected_index = -1
	}
	for i := range 5 {
		if m.line_copy_click[i].Clicked(gtx) {
			if m.selected_index >= 0 && m.selected_index < len(m.list_macros) {
				native.CopyTextToClipboard(gtx, m.list_macros[m.selected_index].Rows[i])
			}
		}
	}
}
func (m *Module) FilterList() {
	m.list_macros = append([]Macro{}, m.macros...)
	sort.Slice(m.list_macros, func(i, j int) bool {
		if m.list_macros[i].Name == m.list_macros[j].Name {
			if m.list_macros[i].Loadout == m.list_macros[j].Loadout {
				return m.list_macros[i].Location < m.list_macros[j].Location
			}
			return m.list_macros[i].Loadout < m.list_macros[j].Loadout
		}
		return m.list_macros[i].Name < m.list_macros[j].Name
	})

	type macroKey struct {
		Name string
		Rows [5]string
	}
	seen := make(map[macroKey]struct{}, len(m.list_macros))
	filtered := m.list_macros[:0]

	searchText := strings.ToLower(m.filter_editor.Text())
	isFound := func(mac Macro) bool {
		if searchText == "" {
			return true
		}
		if strings.Contains(strings.ToLower(mac.Name), searchText) {
			return true
		}
		found := false
		for i := range 5 {
			if strings.Contains(strings.ToLower(mac.Rows[i]), searchText) {
				found = true
			}
		}
		return found
	}

	for _, macro := range m.list_macros {
		key := macroKey{Name: macro.Name, Rows: macro.Rows}
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		if isFound(macro) {
			filtered = append(filtered, macro)
		}
	}
	m.list_macros = filtered
}
func (m *Module) OnLogOpen(characterName string, serverName string, filesize int64, path string) bool {
	if m.loading.Load() {
		select {
		case m.stop <- struct{}{}:
		default:
		}
		m.loading.Store(false)
	}
	logs_dir := filepath.Dir(path)
	m.config_path = filepath.Dir(logs_dir)
	m.config_name = fmt.Sprintf("%s_%s_LO%%d.ini", characterName, serverName)

	go m.LoadConfigs()

	return true
}

type Page struct {
	buttons map[string]Macro
}

func (m *Module) ReadConfig(fp *os.File, loadout int) error {
	scanner := bufio.NewScanner(fp)
	socials := false
	pages := make(map[string]Page)
	for scanner.Scan() {
		line := scanner.Text()
		if line != "" && strings.HasPrefix(line, "[") {
			if line == "[Socials]" {
				socials = true
			} else {
				socials = false
			}
		} else if socials {
			matches := macroLineRE.FindStringSubmatch(line)
			if matches != nil {
				pn := matches[1]
				if _, ok := pages[pn]; !ok {
					pages[pn] = Page{
						buttons: make(map[string]Macro),
					}
				}
				bn := matches[2]
				macro := Macro{Location: fmt.Sprintf("%s/%s", pn, bn), Loadout: loadout}
				if _, ok := pages[pn].buttons[bn]; ok {
					macro = pages[pn].buttons[bn]
				}

				switch matches[3] {
				case "Name":
					macro.Name = matches[5]
				case "Line":
					ln, err := strconv.Atoi(matches[4])
					if err != nil {
						log.Printf("Unable to convert line number: %s. %v", matches[4], err)
					} else {
						macro.Rows[ln-1] = matches[5]
					}
				}
				pages[pn].buttons[bn] = macro
			}
		}
	}
	for _, p := range pages {
		for _, b := range p.buttons {
			if b.Name != "" {
				m.macros = append(m.macros, b)
			}
		}
	}
	return scanner.Err()
}
func (m *Module) LoadConfigs() {
	if !m.loading.CompareAndSwap(false, true) {
		log.Printf("We are already loading...")
		return
	}
	m.macros = make([]Macro, 0)
	defer m.loading.Store(false)
	i := 1
	for i <= 16 {
		filename := fmt.Sprintf(m.config_name, i)
		path := filepath.Join(m.config_path, filename)

		fp, err := os.OpenFile(path, os.O_RDONLY, 0o644)
		if err != nil {
			break
		}
		if err := m.ReadConfig(fp, i); err != nil {
			log.Printf("Unable to read Loadout %d. %v", i, err)
		}
		i++
	}
	m.FilterList()
}
func (m *Module) Shutdown() {
	select {
	case m.stop <- struct{}{}:
	default:
	}
}
