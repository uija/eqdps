package module

import (
	"log"
	"os"
	"path/filepath"
	"regexp"
	"sync/atomic"
	"time"

	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"github.com/uija/eqdps/internal/audio"
	"github.com/uija/eqdps/internal/data"
	"github.com/uija/eqdps/internal/eqlog"
	"github.com/uija/eqdps/internal/overlay"
	"github.com/uija/eqdps/internal/ui"
)

type MenuItem struct {
	Name   string
	Action UIActionFunc
}
type HelpItem struct {
	Name   string
	Layout ui.Widget
}
type SidebarItem struct {
	Name   string
	Action UIActionFunc
	Click  widget.Clickable
}

type UIActionFunc func()
type LogOpenListener func(characterName string, serverName string, filesize int64, path string) bool
type LogRowListener func(event *data.LogRowEvent)
type ReplayStartListener func()
type ReplayEndListener func()

type ProgressHandler func(title string, current int64, max int64)
type UpdateListener func(layout.Context)

type Context struct {
	Parser          *eqlog.Parser
	ParserSession   uint64
	currentMainView ui.Widget
	ViewMenuItems   []MenuItem
	ToolsMenuItems  []MenuItem
	SideBarItems    []SidebarItem

	progressHandler ProgressHandler

	logOpenListener       []LogOpenListener
	logRowListener        []LogRowListener
	replayStartListener   []ReplayStartListener
	replayEndListener     []ReplayEndListener
	statusWidgetProvider  []ui.Widget
	overlayWidgetProvider []ui.Widget
	HelpItems             []HelpItem
	updateListener        []UpdateListener

	Config *data.Config

	lastLevelUp    data.LogLandmark
	lastZoneChange data.LogLandmark

	replayLoopback eqlog.Loopback

	invalidateFunc func()

	parserPath      string
	isReplay        atomic.Bool
	readyForFollow  chan struct{}
	requestedReplay chan eqlog.Loopback
	indexingDone    chan struct{}

	Playback *audio.Playback

	Overlay *overlay.Overlay
}

type ReplayRequest struct {
	ByteOffset int64
	TimeOffset time.Duration
	LastLevel  bool
	LastZoning bool
}

func NewContext(invalidateFunc func()) *Context {
	config, err := data.GetConfig()
	if err != nil {
		log.Printf("Unable to create config. %v", err)
		config = &data.Config{}
	}
	ctx := &Context{
		ParserSession:         0,
		Parser:                nil,
		ViewMenuItems:         make([]MenuItem, 0),
		progressHandler:       func(title string, current int64, max int64) {},
		logOpenListener:       make([]LogOpenListener, 0),
		logRowListener:        make([]LogRowListener, 0),
		replayStartListener:   make([]ReplayStartListener, 0),
		replayEndListener:     make([]ReplayEndListener, 0),
		statusWidgetProvider:  make([]ui.Widget, 0),
		overlayWidgetProvider: make([]ui.Widget, 0),
		HelpItems:             make([]HelpItem, 0),
		SideBarItems:          make([]SidebarItem, 0),
		updateListener:        make([]UpdateListener, 0),

		readyForFollow:  make(chan struct{}, 1),
		requestedReplay: make(chan eqlog.Loopback, 1),
		indexingDone:    make(chan struct{}, 1),
		invalidateFunc:  invalidateFunc,
		Config:          config,
	}
	p, err := audio.NewPlayback("")
	if err != nil {
		panic("Unable to initialize audio playback")
	}
	ctx.Playback = p
	return ctx
}
func (c *Context) ParserLogFileOpened(path string) {
	// Extract Character and Servername
	c.parserPath = path
	c.startParser(path, c.runIndexFile)
}
func (c *Context) RegisterProgressHandler(h ProgressHandler) {
	c.progressHandler = h
}
func (c *Context) RequestReplay(request eqlog.Loopback) {
	c.requestedReplay <- request
}
func (c *Context) runIndexFile() {
	c.Parser.IndexFile(c.ParserOnReplayProgress, func(lm data.LogLandmark) {
		switch lm.Type {
		case data.LogRowEventTypeLevelUp:
			c.lastLevelUp = lm
		case data.LogRowEventTypeZoneChange:
			c.lastZoneChange = lm
		default:
		}
	})
	c.indexingDone <- struct{}{}
}
func (c *Context) CompactStatusElements(style *ui.Style, gtx layout.Context) []layout.FlexChild {
	items := make([]layout.FlexChild, 0)
	items = append(items,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			var label material.LabelStyle
			if c.parserPath == "" {
				label = ui.ColoredLabel(style.Theme, 14, style.Palette.No, "No log")
			} else if c.isReplay.Load() {
				label = ui.ColoredLabel(style.Theme, 14, style.Palette.Accent, "Replay")
			} else {
				label = ui.ColoredLabel(style.Theme, 14, style.Palette.Yes, "Live")
			}
			return layout.Inset{Right: unit.Dp(8)}.Layout(gtx, label.Layout)
		}),
	)
	if c.parserPath != "" {
		items = append(items,
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				_, filename := filepath.Split(c.parserPath)
				return material.Label(style.Theme, unit.Sp(14), filename).Layout(gtx)
			}),
		)
	}
	for _, f := range c.statusWidgetProvider {
		items = append(items,
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				return f(style, gtx)
			}),
		)
	}
	return items
}
func (c *Context) runFollow() {
	c.Parser.Follow(c.ParserNewLogEvent)
}
func (c *Context) runReplay() {
	c.isReplay.Store(true)
	started := time.Now()
	for _, f := range c.replayStartListener {
		f()
	}
	if !c.replayLoopback.Skip {
		c.Parser.Replay(c.replayLoopback, c.ParserNewLogEvent, c.ParserOnReplayProgress)
	}
	for _, f := range c.replayEndListener {
		f()
	}
	log.Printf("Replay took: %v", time.Since(started))
	c.isReplay.Store(false)
	c.readyForFollow <- struct{}{}
}
func (c *Context) Update(gtx layout.Context) {
	select {
	case rr := <-c.requestedReplay:
		if c.parserPath == "" {
			break
		}
		c.replayLoopback = rr
		c.startParser(c.parserPath, c.runReplay)
	case <-c.indexingDone:
		info, err := os.Stat(c.parserPath)
		if err != nil {
			return
		}
		exp := regexp.MustCompile(`^(.*)/eqlog_(.*)_(.*).txt$`)
		follow := true
		if fields := exp.FindStringSubmatch(c.parserPath); fields != nil {
			// find size of file
			for _, f := range c.logOpenListener {
				if !f(fields[2], fields[3], info.Size(), c.parserPath) {
					follow = false
				}
			}
		}
		if follow {
			c.readyForFollow <- struct{}{}
		}
		c.invalidateFunc()
	case <-c.readyForFollow:
		c.startParser(c.parserPath, c.runFollow)
		c.invalidateFunc()
	default:
	}
	for _, f := range c.updateListener {
		f(gtx)
	}
}
func (c *Context) startParser(path string, runFunc func()) {
	c.stopParser()

	c.Parser = eqlog.NewParser(c.ParserSession)
	err := c.Parser.Open(path)
	if err != nil {
		log.Printf("Unable to upen log. %v", err)
		c.stopParser()
		return
	}
	go runFunc()
}
func (c *Context) stopParser() {
	if c.Parser != nil {
		c.ParserSession++
		c.Parser.Close()
		c.Parser = nil
	}
}
func (c *Context) ParserOnReplayProgress(progress eqlog.ReplayProgress) {
	c.progressHandler("Parsing Logfile...", progress.Bytes, progress.Total)
}
func (c *Context) ParserNewLogEvent(row *data.LogRowEvent) {
	switch row.Type {
	case data.LogRowEventTypeZoneChange:
		if row.Timestamp.After(c.lastZoneChange.Timestamp) {
			c.lastZoneChange.Zone = row.Metadata.Zone
			c.lastZoneChange.Timestamp = row.Timestamp
			c.lastZoneChange.Offset = row.Offset
		}
	case data.LogRowEventTypeLevelUp:
		if row.Timestamp.After(c.lastLevelUp.Timestamp) {
			c.lastLevelUp.Timestamp = row.Timestamp
			c.lastLevelUp.Offset = row.Offset
			c.lastLevelUp.Level = row.Metadata.Level
		}
	default:
	}
	for _, f := range c.logRowListener {
		f(row)
	}
	if !c.isReplay.Load() {
		c.invalidateFunc()
	}
}
func (c *Context) AddViewMenuItem(name string, action UIActionFunc) {
	c.ViewMenuItems = append(c.ViewMenuItems, MenuItem{Name: name, Action: action})
}
func (c *Context) AddSidebarItem(name string, action UIActionFunc) {
	c.SideBarItems = append(c.SideBarItems, SidebarItem{Name: name, Action: action})
}
func (c *Context) AddToolsMenuItem(name string, action UIActionFunc) {
	c.ToolsMenuItems = append(c.ToolsMenuItems, MenuItem{Name: name, Action: action})
}
func (c *Context) RegisterLogOpen(f LogOpenListener) {
	c.logOpenListener = append(c.logOpenListener, f)
}
func (c *Context) RegisterLogRow(f LogRowListener) {
	c.logRowListener = append(c.logRowListener, f)
}
func (c *Context) RegisterUpdate(f UpdateListener) {
	c.updateListener = append(c.updateListener, f)
}
func (c *Context) RegisterReplayStart(f ReplayStartListener) {
	c.replayStartListener = append(c.replayStartListener, f)
}
func (c *Context) RegisterReplayEnd(f ReplayEndListener) {
	c.replayEndListener = append(c.replayEndListener, f)
}
func (c *Context) RegisterStatusWidget(f ui.Widget) {
	c.statusWidgetProvider = append(c.statusWidgetProvider, f)
}
func (c *Context) RegisterOverlayWidget(f ui.Widget) {
	c.overlayWidgetProvider = append(c.overlayWidgetProvider, f)
}
func (c *Context) SetMainView(f ui.Widget) {
	c.currentMainView = f
}
func (c *Context) AddHelpItem(name string, layout ui.Widget) {
	c.HelpItems = append(c.HelpItems, HelpItem{Name: name, Layout: layout})
}
func (c *Context) RegisterModule(m Module) error {
	return m.Init(c, c.invalidateFunc)
}
func (c *Context) Layout(style *ui.Style, gtx layout.Context) layout.Dimensions {
	if c.currentMainView != nil {
		return c.currentMainView(style, gtx)
	} else {
		return layout.Inset{Top: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			label := material.Label(style.Theme, unit.Sp(15), "No modules registered")
			//label.Color = palette.muted
			return label.Layout(gtx)
		})
	}
}
func (c *Context) GetLastLevelOffset() time.Time {
	return c.lastLevelUp.Timestamp
}
func (c *Context) GetLastZoningOffset() time.Time {
	return c.lastZoneChange.Timestamp
}
