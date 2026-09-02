package sky

import (
	"fmt"
	"log"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/unit"
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

//var itemUpgradeSuffixRE = regexp.MustCompile(` \+[0-9]+$`)

const HeaderSize = 15
const RowSize = 14

var skyHelpSections = []struct {
	title string
	text  string
}{
	{
		text: "The Plane of Sky Quest Tracker helps you keep track of quest progression, collected quest items and completed armor quests.\n\nIt watches your EverQuest logfile and automatically records recognized Plane of Sky loot, destroyed items and completed quest turn-ins.",
	},
	{
		title: "Progression",
		text: "The Progression page groups quests by class. Each quest shows the required items and how many of them you currently have.\n\n" +
			"Quests are marked according to their current state:\n\n" +
			"• Ready to turn in — You have all required items.\n" +
			"• In progress — You have collected some, but not all, required items.\n" +
			"• Empty — You have not collected any required items.\n" +
			"• Finished — The quest has already been completed.\n\n" +
			"Use the controls at the top to hide finished or empty quests and keep the list focused on relevant progression.",
	},
	{
		title: "Watching quests",
		text:  "You can mark individual quests as watched. Watched quests are collected in a separate section near the top of the page, making it easier to follow the quests you are currently working on.\n\nWatching a quest does not change its progress or consume any items.",
	},
	{
		title: "Collected items",
		text:  "Recognized quest items are added automatically when they appear in loot messages. Items are removed when the logfile reports that they were destroyed or handed in as part of a recognized quest.\n\nThe tracker also understands item quantities, including stacks containing more than one item.\n\nSelect Show Inventory to see all currently recorded Plane of Sky quest items grouped by their location or purpose.",
	},
	{
		title: "Inventory exports",
		text: "An EverQuest inventory export can be used to compare the tracker with the items your character currently owns. This is useful when items were collected before eqdps was running or when the stored amounts no longer match your inventory.\n\n" +
			"Create an inventory export in EverQuest with:\n\n/outputfile inventory\n\n" +
			"eqdps detects the completed export from the logfile and updates the recorded Plane of Sky items.\n\nAn inventory export can only report items currently owned by the character. It cannot restore information about completed quests.",
	},
	{
		title: "Missing or incorrect runes",
		text: "Select Missing Runes? to adjust the recorded rune amounts manually.\n\n" +
			"This can be useful when:\n\n" +
			"• an item was looted before logging started;\n" +
			"• an inventory export did not include every relevant storage location;\n" +
			"• an item was moved or consumed without a recognizable logfile message;\n" +
			"• the tracker’s stored amount differs from your actual inventory.\n\n" +
			"Manual corrections remain stored until they are changed again or the tracker is fully reset.",
	},
	{
		title: "Quest turn-ins",
		text:  "When you complete a recognized Plane of Sky quest, the tracker records the completed quest and removes the required items from its inventory.\n\nA turn-in can only be recognized when the quest giver and handed-in items match a known quest. Unusual, rejected or incomplete trades may not be counted automatically.",
	},
	{
		title: "Reset and Reload",
		text:  "Reset & Reload deletes the collected Plane of Sky progress and rebuilds it from the current logfile.\n\nUse this when the stored data appears incorrect or when the tracker’s data format has changed. A full reload can take a moment for large logfiles.\n\nManual rune corrections and information obtained only from inventory exports may need to be entered again afterward.",
	},
	{
		title: "EQLDB contribution",
		text:  "If Plane of Sky contribution is enabled in the EQLDB settings, recognized rune changes and completed quests are sent to eqldb.org.\n\nOnly the relevant Plane of Sky events are contributed. Disabling this option does not affect local quest tracking.",
	},
}

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
	helpList      widget.List

	progression_click widget.Clickable
	inventory_click   widget.Clickable

	reset_reload_click         widget.Clickable
	show_reset_reload_overlay  bool
	confirm_reset_reload_click widget.Clickable
	cancel_reset_reload_click  widget.Clickable

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
	m.helpList.Axis = layout.Vertical
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
	go func() {
		if err := beeep.Notify("Plane of Sky", fmt.Sprintf("%d new Quests ready to turn in", num), "dialog-information"); err != nil {
			log.Printf("Unable to send Notification: %v", err)
		}
	}()
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
	if m.cancel_reset_reload_click.Clicked(gtx) {
		m.show_reset_reload_overlay = false
		m.invalidFunc()
	}
	if m.reset_reload_click.Clicked(gtx) {
		m.show_reset_reload_overlay = true
		m.invalidFunc()
	}
	if m.confirm_reset_reload_click.Clicked(gtx) {
		defer m.invalidFunc()
		m.show_reset_reload_overlay = false
		if m.configPath == "" {
			return
		}
		m.config = EmptyConfig(m.configPath)
		m.config.Save()
		m.ctx.RequestReplay(eqlog.Loopback{ByteOffset: m.config.Log.Offset})
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
	sectionCount := len(skyHelpSections) + 1
	list := material.List(style.Theme, &m.helpList)
	return list.Layout(gtx, sectionCount, func(gtx layout.Context, index int) layout.Dimensions {
		section := struct {
			title string
			text  string
		}{}
		if index < len(skyHelpSections) {
			section = skyHelpSections[index]
		} else {
			section.title = "Notes"
			section.text = "The tracker can only use information available in the logfile and inventory exports. Events that occurred while logging was disabled may be missing until you perform an inventory export or correct the amounts manually.\n\nYour Plane of Sky progress is stored separately for each character and server.\n\nData file: " + m.configPath
		}

		return layout.Inset{Bottom: unit.Dp(24), Right: unit.Dp(16)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			children := make([]layout.FlexChild, 0, 2)
			if section.title != "" {
				children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					label := material.Label(style.Theme, ui.Sp(18), section.title)
					label.Font.Weight = font.SemiBold
					return layout.Inset{Bottom: unit.Dp(8)}.Layout(gtx, label.Layout)
				}))
			}
			children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				label := material.Label(style.Theme, ui.Sp(15), section.text)
				label.Color = style.Palette.Muted
				return label.Layout(gtx)
			}))
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
		})
	})
}
