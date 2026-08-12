package sky

import (
	"fmt"
	"log"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"github.com/uija/eqdps/internal/module"
	"github.com/uija/eqdps/internal/ui"
)

var itemUpgradeSuffixRE = regexp.MustCompile(` \+[0-9]+$`)

const HeaderSize = 15
const RowSize = 14

type Module struct {
	ctx    *module.Context
	db     Database
	config Config
	status []ClassStatus

	replay bool

	configPath string

	tradeIn *TradeInTracker

	questlist widget.List
}

func NewModule() *Module {
	return &Module{
		config:  Config{},
		status:  make([]ClassStatus, 0),
		tradeIn: nil,
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
	QuestGiver string
	Reward     string
	Done       bool
	Watched    bool

	MissingItems int
	Items        []ItemStatus

	WatchClick   widget.Clickable
	UnwatchClick widget.Clickable
	RewardClick  widget.Clickable
}
type ClassStatus struct {
	Name        string
	QuestsDone  int
	QuestsReady int
	Visible     bool
	ToggleClick widget.Clickable

	Quests []QuestStatus
}

func (m *Module) Init(ctx *module.Context) error {
	ctx.AddViewMenuItem("Plane of Sky Quest Tracker", m.OpenMainView)
	ctx.RegisterLogOpen(m.OnLogOpen)
	ctx.RegisterLogRow(m.OnLogRow)
	ctx.RegisterReplayStart(m.OnReplayStart)
	ctx.RegisterReplayEnd(m.OnReplayEnd)
	ctx.RegisterUpdate(m.Update)
	ctx.SetMainView(m.MainView)
	ctx.AddHelpItem("Plane of Sky Quest Tracker", m.LayoutHelp)
	m.ctx = ctx
	m.db, _ = LoadDatabase()
	m.questlist.Axis = layout.Vertical
	m.BuildStatusFromDatabase()
	return nil
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
			}
			qs.MissingItems = len(qs.Items)

			cs.Quests = append(cs.Quests, qs)
		}
		m.status = append(m.status, cs)
	}
}
func (m *Module) RecalculateStatus() {
	for cidx := range m.status {
		c := &m.status[cidx]
		c.QuestsDone = 0
		c.QuestsReady = 0
		for qidx := range c.Quests {
			q := &c.Quests[qidx]
			if m.config.Quests[q.Name] > 0 {
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
}

func (m *Module) OpenMainView() {
	m.ctx.SetMainView(m.MainView)
}

func (m *Module) Shutdown() {

}
func (m *Module) Update(gtx layout.Context) {
	for cidx := range m.status {
		if m.status[cidx].ToggleClick.Clicked(gtx) {
			m.status[cidx].Visible = !m.status[cidx].Visible
		}
		for qidx := range m.status[cidx].Quests {
			if m.status[cidx].Quests[qidx].RewardClick.Clicked(gtx) {
				log.Printf("%s clicked", m.status[cidx].Quests[qidx].Reward)
			}
			if m.status[cidx].Quests[qidx].WatchClick.Clicked(gtx) {
				m.status[cidx].Quests[qidx].Watched = true
			}
			if m.status[cidx].Quests[qidx].UnwatchClick.Clicked(gtx) {
				m.status[cidx].Quests[qidx].Watched = false
			}
		}
	}
}
func (m *Module) OnLogOpen(characterName string, serverName string, size int64, path string) {
	// Extract path
	base_path := filepath.Dir(path)
	m.configPath = fmt.Sprintf("%s/eqdps_%s_%s_PoS.json", base_path, characterName, serverName)
	config, err := LoadConfig(m.configPath)
	if err != nil {
		log.Printf("Error loading pos file at %s, %v", m.configPath, err)
		return
	}
	m.config = config
	m.RecalculateStatus()
}
func (m *Module) OnReplayStart() {
	m.replay = true
}
func (m *Module) OnReplayEnd() {
	m.replay = false
	if err := m.config.Save(); err != nil {
		log.Printf("Unable to save config. %v", err)
	}
	m.RecalculateStatus()
}
func (m *Module) LayoutHelp(style *ui.Style, gtx layout.Context) layout.Dimensions {
	label := material.Label(
		style.Theme,
		unit.Sp(15),
		"This is awesome Plane of Sky Quest Tracker help content!",
	)
	label.Color = style.Palette.Muted
	return label.Layout(gtx)
}
