package sky

import (
	"fmt"
	"log"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"gioui.org/layout"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"gioui.org/x/component"
	"github.com/gen2brain/beeep"
	"github.com/uija/eqdps/internal/eqldb"
	"github.com/uija/eqdps/internal/eqlog"
	"github.com/uija/eqdps/internal/module"
	"github.com/uija/eqdps/internal/native"
	"github.com/uija/eqdps/internal/ui"
	"github.com/uija/eqdps/internal/ui/form"
)

var itemUpgradeSuffixRE = regexp.MustCompile(` \+[0-9]+$`)

const HeaderSize = 15
const RowSize = 14

type InventoryRow struct {
	Name string
	Need int
	Have int
	Hint string
}
type Notification struct {
	Text string
	Ends time.Time
}
type Module struct {
	ctx       *module.Context
	db        Database
	config    Config
	status    []ClassStatus
	inventory []InventoryRow

	mainView ui.Widget

	replay      bool
	readyToRead bool

	configPath string

	tradeIn *TradeInTracker

	questlist     widget.List
	inventorylist widget.List

	progression_click widget.Clickable
	inventory_click   widget.Clickable

	status_click       widget.Clickable
	ready_turnin_click widget.Clickable
	watched_click      widget.Clickable

	current_charactername string
	current_server        string

	hide_finished widget.Clickable
	hide_empty    widget.Clickable

	notification *Notification

	runeNames       []string
	runeEditors     []*widget.Editor
	runeForm        *form.Form
	runeSaveClick   widget.Clickable
	runeCancelClick widget.Clickable
	runeEditList    widget.List

	mu           sync.Mutex
	eqldb_events []eqldb.PlaneOfSkyEvent
	stop         chan struct{}

	inventoryDiff chan map[string]int

	edit_runes_click widget.Clickable
	edit_runes       bool

	invalidFunc func()
}

func NewModule() *Module {
	return &Module{
		config:        Config{},
		status:        make([]ClassStatus, 0),
		tradeIn:       nil,
		mainView:      func(*ui.Style, layout.Context) layout.Dimensions { return layout.Dimensions{} },
		inventory:     make([]InventoryRow, 0),
		invalidFunc:   func() {},
		eqldb_events:  make([]eqldb.PlaneOfSkyEvent, 0),
		stop:          make(chan struct{}, 1),
		inventoryDiff: make(chan map[string]int, 1),
		runeNames:     make([]string, 0),
	}
}

type TradeInTracker struct {
	Npc        string
	Given      map[string]int
	LastUpdate time.Time
	Denied     time.Time
	Experience time.Time
}
type ItemStatus struct {
	Name   string
	Hint   string
	Amount int
}

type QuestStatus struct {
	Name       string
	Key        string
	QuestGiver string
	Reward     string
	Done       bool
	Watched    bool

	MissingItems int
	Items        []ItemStatus

	WatchClick       widget.Clickable
	WatchTooltip     component.TipArea
	UnwatchClick     widget.Clickable
	RedoQuestClick   widget.Clickable
	RedoQuestTooptip component.TipArea
	RewardClick      widget.Clickable
}
type ClassStatus struct {
	Name        string
	QuestsDone  int
	QuestsReady int
	Visible     bool
	ToggleClick widget.Clickable

	Quests []QuestStatus
}

func (m *Module) Init(ctx *module.Context, invalidate func()) error {
	m.invalidFunc = invalidate
	ctx.AddViewMenuItem("Plane of Sky Quest Tracker", m.OpenMainView)
	ctx.AddSidebarItem("PoS", m.OpenMainView)
	ctx.RegisterLogOpen(m.OnLogOpen)
	ctx.RegisterLogRow(m.OnLogRow)
	ctx.RegisterReplayStart(m.OnReplayStart)
	ctx.RegisterReplayEnd(m.OnReplayEnd)
	ctx.RegisterStatusWidget(m.LayoutStatus)
	ctx.RegisterUpdate(m.Update)
	//ctx.SetMainView(m.Layout)
	ctx.AddHelpItem("Plane of Sky Quest Tracker", m.LayoutHelp)
	m.ctx = ctx
	m.db, _ = LoadDatabase()
	m.questlist.Axis = layout.Vertical
	m.inventorylist.Axis = layout.Vertical
	m.BuildStatusFromDatabase()
	m.runeEditors = make([]*widget.Editor, len(m.runeNames))
	m.runeForm = form.New()
	for idx, name := range m.runeNames {
		m.runeEditors[idx] = &widget.Editor{SingleLine: true}
		m.runeForm.AddEditor(name, m.runeEditors[idx], func(ee widget.EditorEvent) {})
	}
	m.runeForm.AddButton("save", &m.runeSaveClick, func() {
		if m.config.QuestItems != nil {
			for idx, name := range m.runeNames {
				lname := strings.ToLower(name)
				i, err := strconv.Atoi(m.runeEditors[idx].Text())
				if err == nil {
					if i == 0 {
						delete(m.config.QuestItems, lname)
					} else {
						m.config.QuestItems[lname] = i
					}
				}
				m.config.Save()
			}
		}
		m.edit_runes = false
	})
	m.runeForm.AddButton("cancel", &m.runeCancelClick, func() {
		m.edit_runes = false
	})
	m.runeEditList.Axis = layout.Vertical
	m.mainView = m.MainView
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-m.stop:
				return
			case <-ticker.C:
				m.TickSkyItemUpload()
			}
		}

	}()

	return nil
}
func (m *Module) TickSkyItemUpload() {
	m.mu.Lock()
	if len(m.eqldb_events) == 0 {
		m.mu.Unlock()
		return
	}
	token := m.ctx.Config.EQLDbConfig.AccessToken
	character_name := m.current_charactername
	server := m.current_server
	events := append([]eqldb.PlaneOfSkyEvent(nil), m.eqldb_events...)
	m.eqldb_events = make([]eqldb.PlaneOfSkyEvent, 0)
	m.mu.Unlock()

	go func() {
		err := eqldb.UploadPlaneOfSkyEvents(token, character_name, server, events...)
		if eqldb.IsUnauthorized(err) {
			m.ctx.Config.EQLDbConfig.AccessToken = ""
			m.ctx.Config.EQLDbConfig.AuthorizationTime = time.Time{}
		}
	}()
}
func (m *Module) BuildStatusFromDatabase() {
	for _, c := range m.db.Classes {
		cs := ClassStatus{
			Name:        c.Name,
			QuestsDone:  0,
			QuestsReady: 0,
			Visible:     true,
			Quests:      make([]QuestStatus, 0),
		}
		for _, q := range c.Quests {
			qs := QuestStatus{
				Name:       QuestName(q.Name, c.Name),
				Key:        q.Name,
				QuestGiver: q.QuestGiver,
				Reward:     q.Rewards[0],
				Done:       false,
				Watched:    false,
				Items:      make([]ItemStatus, 0),
			}

			for _, i := range q.Requirements {
				is := ItemStatus{
					Name:   i.Name,
					Amount: 0,
					Hint:   i.DropsFrom,
				}
				if is.Hint == "" {
					is.Hint = "Random trash drop"
				}
				qs.Items = append(qs.Items, is)
				if strings.Contains(i.Name, "Wind Rune") {
					if !slices.Contains(m.runeNames, i.Name) {
						m.runeNames = append(m.runeNames, i.Name)
					}
				}
			}
			qs.MissingItems = len(qs.Items)

			cs.Quests = append(cs.Quests, qs)
		}
		m.status = append(m.status, cs)
	}
}
func (m *Module) RecalculateStatus() {
	readyBefore := 0
	for _, c := range m.status {
		readyBefore += c.QuestsReady
	}
	for cidx := range m.status {
		c := &m.status[cidx]
		c.QuestsDone = 0
		c.QuestsReady = 0
		for qidx := range c.Quests {
			q := &c.Quests[qidx]
			q.Watched = m.config.Watched[q.Key]
			if m.config.Quests[q.Key] > 0 && m.config.RedoQuests[q.Key].IsZero() {
				q.Done = true
				c.QuestsDone++
			} else {
				q.MissingItems = 0
				for iidx := range q.Items {
					i := &q.Items[iidx]

					i.Amount = m.config.QuestItems[strings.ToLower(i.Name)]
					if i.Amount == 0 {
						q.MissingItems++
					}
				}
				if q.MissingItems == 0 {
					c.QuestsReady++
				}
			}
		}
	}
	readyAfter := 0
	for _, c := range m.status {
		readyAfter += c.QuestsReady
	}
	if readyAfter > readyBefore && !m.replay {
		m.Notify(readyAfter - readyBefore)
	}
	items := make(map[string]InventoryRow)
	for _, c := range m.status {
		for _, q := range c.Quests {
			for _, i := range q.Items {
				ir, ok := items[i.Name]
				if !ok {
					amount := m.config.QuestItems[strings.ToLower(i.Name)]
					ir = InventoryRow{
						Name: i.Name,
						Hint: i.Hint,
						Need: 0,
						Have: amount,
					}
				}
				if !q.Done || !m.config.RedoQuests[q.Key].IsZero() {
					ir.Need++
				}
				items[i.Name] = ir
			}
		}
	}
	m.inventory = make([]InventoryRow, 0)
	for _, ir := range items {
		m.inventory = append(m.inventory, ir)
	}
	sort.Slice(m.inventory, func(i, j int) bool {
		return m.inventory[i].Hint < m.inventory[j].Hint
	})
}

func (m *Module) Notify(num int) {
	beeep.AppName = "eqDps"
	err := beeep.Notify("Plane of Sky", fmt.Sprintf("%d new Quests ready to turn in", num), "dialog-information")
	if err != nil {
		log.Printf("Unable to start notification %v", err)
	}
	m.notification = &Notification{
		Text: fmt.Sprintf("%d new Quests", num),
		Ends: time.Now().Add(5 * time.Second),
	}
	m.invalidFunc()
}

func (m *Module) OpenMainView() {
	m.ctx.SetMainView(m.Layout)
}

func (m *Module) Shutdown() {
	m.stop <- struct{}{}
}
func (m *Module) Update(gtx layout.Context) {
	m.runeForm.Update(gtx)
	for cidx := range m.status {
		if m.status[cidx].ToggleClick.Clicked(gtx) {
			m.status[cidx].Visible = !m.status[cidx].Visible
		}
		for qidx, quest := range m.status[cidx].Quests {
			if m.status[cidx].Quests[qidx].RewardClick.Clicked(gtx) {
				url := fmt.Sprintf("https://www.eqlwiki.com/%s", m.status[cidx].Quests[qidx].Reward)
				native.OpenURL(url)
			}
			if m.status[cidx].Quests[qidx].WatchClick.Clicked(gtx) {
				m.status[cidx].Quests[qidx].WatchTooltip = component.TipArea{}
				m.status[cidx].Quests[qidx].Watched = true
				if m.config.Watched != nil {
					m.config.Watched[quest.Key] = true
					m.config.Save()
				}
			}
			if m.status[cidx].Quests[qidx].UnwatchClick.Clicked(gtx) {
				m.status[cidx].Quests[qidx].WatchTooltip = component.TipArea{}
				m.status[cidx].Quests[qidx].Watched = false
				if m.config.Watched != nil {
					delete(m.config.Watched, quest.Key)
					m.config.Save()
				}
			}

			if m.status[cidx].Quests[qidx].RedoQuestClick.Clicked(gtx) {
				m.status[cidx].Quests[qidx].RedoQuestTooptip = component.TipArea{}
				if m.config.RedoQuests != nil {
					if m.config.RedoQuests[quest.Key].IsZero() {
						m.config.RedoQuests[quest.Key] = time.Now()
					} else {
						delete(m.config.RedoQuests, quest.Key)
					}
				}
				m.RecalculateStatus()
			}
		}
	}
	if m.progression_click.Clicked(gtx) {
		m.mainView = m.MainView
	}
	if m.inventory_click.Clicked(gtx) {
		m.mainView = m.InventoryView
	}
	if m.hide_empty.Clicked(gtx) {
		m.config.HideEmpty = !m.config.HideEmpty
		m.config.Save()
	}
	if m.hide_finished.Clicked(gtx) {
		m.config.HideFinished = !m.config.HideFinished
		m.config.Save()
	}
	if m.status_click.Clicked(gtx) {
		m.OpenMainView()
	}
	if m.ready_turnin_click.Clicked(gtx) {
		m.config.HideReady = !m.config.HideReady
		m.config.Save()
	}
	if m.watched_click.Clicked(gtx) {
		m.config.HideWatched = !m.config.HideWatched
		m.config.Save()
	}
	if m.edit_runes_click.Clicked(gtx) {
		m.edit_runes = !m.edit_runes
		if m.config.QuestItems != nil {
			if m.edit_runes {
				for idx, name := range m.runeNames {
					lname := strings.ToLower(name)
					val := m.config.QuestItems[lname]
					m.runeEditors[idx].SetText(strconv.Itoa(val))
				}
			}
		}
	}
}
func (m *Module) OnLogOpen(characterName string, serverName string, size int64, path string) bool {
	m.TickSkyItemUpload()
	// Extract path
	base_path := filepath.Dir(path)
	m.current_charactername = characterName
	m.current_server = serverName
	m.configPath = fmt.Sprintf("%s/eqdps_%s_%s_PoS.json", base_path, characterName, serverName)
	config, err := LoadConfig(m.configPath)
	if err != nil {
		log.Printf("Error loading pos file at %s, %v", m.configPath, err)
		return false
	}
	m.config = config
	if size > m.config.Log.Offset {
		m.ctx.RequestReplay(eqlog.Loopback{ByteOffset: m.config.Log.Offset})
		return false
	}
	m.readyToRead = true
	m.RecalculateStatus()
	return true
}
func (m *Module) OnReplayStart() {
	m.replay = true
}
func (m *Module) OnReplayEnd() {
	m.replay = false
	m.readyToRead = true
	if err := m.config.Save(); err != nil {
		log.Printf("Unable to save config. %v", err)
	}
	m.RecalculateStatus()
}
func (m *Module) LayoutHelp(style *ui.Style, gtx layout.Context) layout.Dimensions {
	label := material.Label(
		style.Theme,
		ui.Sp(15),
		fmt.Sprintf("You find your PoS config file at: %s", m.configPath),
	)
	label.Color = style.Palette.Muted
	return label.Layout(gtx)
}
