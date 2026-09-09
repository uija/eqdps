package faction

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync/atomic"

	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"github.com/uija/eqdps/internal/data"
	"github.com/uija/eqdps/internal/module"
	"github.com/uija/eqdps/internal/ui"
	"github.com/uija/eqdps/internal/ui/form"
)

type Faction struct {
	Name      string
	Start     int
	End       int
	Collected int
}

type Module struct {
	ctx      *module.Context
	factions []Faction
	replay   atomic.Bool

	logPath       string
	characterName string
	serverName    string

	add_faction_click widget.Clickable
	addForm           *form.Form
	factionSelect     *form.SelectBox
	startEditor       widget.Editor
	endEditor         widget.Editor
	saveClick         widget.Clickable
	cancelClick       widget.Clickable
	addingFaction     bool
	formError         string
	deleteClicks      []*widget.Clickable
	deleteIndex       int
	deleteForm        *form.Form
	confirmDelete     widget.Clickable
	cancelDelete      widget.Clickable

	list widget.List
}

func NewModule() *Module {
	m := &Module{
		addForm:       form.New(),
		factionSelect: form.NewSelectBox(nil, -1),
		startEditor:   widget.Editor{SingleLine: true},
		endEditor:     widget.Editor{SingleLine: true},
		deleteIndex:   -1,
		deleteForm:    form.New(),
	}
	for _, err := range []error{
		m.addForm.AddSelectBox("faction", m.factionSelect),
		m.addForm.AddEditor("start", &m.startEditor, nil),
		m.addForm.AddEditor("end", &m.endEditor, nil),
		m.addForm.AddButton("save", &m.saveClick, m.saveNewFaction),
		m.addForm.AddButton("cancel", &m.cancelClick, m.closeAddFaction),
		m.deleteForm.AddButton("delete", &m.confirmDelete, m.deleteFaction),
		m.deleteForm.AddButton("cancel", &m.cancelDelete, m.closeDeleteFaction),
	} {
		if err != nil {
			panic(err)
		}
	}
	return m
}
func (m *Module) Init(ctx *module.Context, invalidate func()) error {
	m.ctx = ctx
	m.list.Axis = layout.Vertical
	ctx.AddModuleNavigation("faction", "Faction", "Faction", m.Layout)
	ctx.RegisterReplayStart(m.OnReplayStart)
	ctx.RegisterReplayEnd(m.OnReplayEnd)
	ctx.RegisterLogOpen(m.OnLogOpen)
	ctx.RegisterLogRow(m.OnLogRow)
	ctx.RegisterUpdate(m.Update)
	return nil
}
func (m *Module) Shutdown() {

}
func (m *Module) OnLogRow(event *data.LogRowEvent) {
	if event.Type != data.LogRowEventTypeFaction {
		return
	}
	if !slices.Contains(m.ctx.Config.KnownFactions, event.Data[1]) {
		m.ctx.Config.KnownFactions = append(m.ctx.Config.KnownFactions, event.Data[1])
		m.ctx.Config.Save()
	}
	if event.Data[2] == "" {
		return
	}
	if m.replay.Load() {
		return
	}
	for idx := range m.factions {
		if strings.EqualFold(m.factions[idx].Name, event.Data[1]) {
			add, err := strconv.Atoi(event.Data[2])
			if err != nil {
				log.Printf("Faction adjustment cannot be parsed. '%s'. %v", event.Data[2], err)
			} else {
				m.factions[idx].Collected += add
				m.saveFactions()
			}
			return
		}
	}
}
func (m *Module) OnLogOpen(characterName, serverName string, filesize int64, path string) bool {
	m.characterName = characterName
	m.serverName = serverName
	m.logPath = filepath.Dir(path)
	if path == "" {
		m.logPath = ""
	}
	m.closeAddFaction()
	m.factions = make([]Faction, 0)
	m.loadFactions()
	m.closeDeleteFaction()
	m.deleteClicks = make([]*widget.Clickable, len(m.factions))
	for i := range m.deleteClicks {
		m.deleteClicks[i] = new(widget.Clickable)
	}
	return true
}

func (m *Module) OnReplayStart() {
	m.replay.Store(true)
}
func (m *Module) OnReplayEnd() {
	m.replay.Store(false)
}

func (m *Module) Layout(style *ui.Style, gtx layout.Context) layout.Dimensions {
	if !m.hasCharacter() {
		return ui.Label(style, "Open a character logfile to track factions.").Layout(gtx)
	}
	if m.replay.Load() || m.factions == nil {
		return layout.Dimensions{}
	}
	return layout.Stack{}.Layout(gtx,
		layout.Expanded(func(gtx layout.Context) layout.Dimensions {
			if m.addingFaction || m.deleteIndex >= 0 {
				gtx = gtx.Disabled()
			}
			return m.layoutFactions(style, gtx)
		}),
		layout.Expanded(func(gtx layout.Context) layout.Dimensions {
			if m.deleteIndex >= 0 {
				return m.layoutDeleteFaction(style, gtx)
			}
			if !m.addingFaction {
				return layout.Dimensions{}
			}
			return m.layoutAddFaction(style, gtx)
		}),
	)
}

func (m *Module) layoutFactions(style *ui.Style, gtx layout.Context) layout.Dimensions {
	return layout.UniformInset(unit.Dp(8)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
					layout.Flexed(1, ui.HeaderLabel(style, "Factions").Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						children := make([]layout.FlexChild, 0)
						children = append(children,
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								return layout.Inset{Left: unit.Dp(8)}.Layout(gtx, ui.RenderLinkAsButton(style, &m.add_faction_click, ui.AddBox, "Add Faction"))
							}),
						)
						return layout.Flex{Axis: layout.Horizontal}.Layout(gtx, children...)
					}),
				)
			}),
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				list := material.List(style.Theme, &m.list)

				return list.Layout(gtx, len(m.factions), func(gtx layout.Context, index int) layout.Dimensions {
					col := style.Palette.Window
					if index%2 != 0 {
						col = style.Palette.Panel
					}
					return ui.ColoredRow(gtx, col, func(gtx layout.Context) layout.Dimensions {
						return layout.UniformInset(unit.Dp(8)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
								layout.Flexed(1, ui.Label(style, m.factions[index].Name).Layout),
								layout.Rigid(ui.Label(style, fmt.Sprintf("%d / %d", m.factions[index].Start+m.factions[index].Collected, m.factions[index].End)).Layout),
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									return layout.Inset{Left: unit.Dp(16)}.Layout(gtx, ui.IconLink(style, m.deleteClicks[index], factionDeleteIcon, "").Layout)
								}),
							)
						})
					})
				})
			}),
		)
	})
}

func (m *Module) loadFactions() {
	if m.logPath == "" || m.characterName == "" || m.serverName == "" {
		return
	}
	bytes, err := os.ReadFile(m.configPath())
	if err != nil {
		if os.IsNotExist(err) {
			return
		}
		log.Printf("Unable to open factions config. %v", err)
		return
	}
	err = json.Unmarshal(bytes, &m.factions)
	if err != nil {
		log.Printf("Unable to unmarshal factions. %v", err)
	}
}
func (m *Module) saveFactions() {
	bytes, err := json.Marshal(m.factions)
	if err != nil {
		log.Printf("Unable to marshal factions. %v", err)
		return
	}
	err = os.WriteFile(m.configPath(), bytes, 0o644)
	if err != nil {
		log.Printf("Unable to write factions file. %v", err)
	}
}
func (m *Module) configPath() string {
	return filepath.Join(m.logPath, fmt.Sprintf("eqdps_%s_%s_Factions.json", m.characterName, m.serverName))
}
