package module

import (
	"log"
	"regexp"
	"time"

	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget/material"
	"github.com/uija/eqdps/internal/data"
	"github.com/uija/eqdps/internal/eqlog"
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

type UIActionFunc func()
type OnLogOpenFunc func(characterName string, serverName string, filesize int64)
type OnLogRowFunc func(event *data.LogRowEvent)

type ProgressHandler func(title string, current int64, max int64)
type UpdateFunc func(layout.Context)

type Context struct {
	Parser          *eqlog.Parser
	ParserSession   uint64
	currentMainView ui.Widget
	ViewMenuItems   []MenuItem
	ToolsMenuItems  []MenuItem
	progressHandler ProgressHandler
	onLogOpenFuncs  []OnLogOpenFunc
	onLogRowFuncs   []OnLogRowFunc
	onStatus        []ui.Widget
	onOverlay       []ui.Widget
	HelpItems       []HelpItem
	updateFuncs     []UpdateFunc

	lastLevelUp    data.LogLandmark
	lastZoneChange data.LogLandmark

	replayLoopback eqlog.Loopback

	invalidateFunc func()

	parserPath      string
	isReplay        bool
	readyForFollow  chan struct{}
	requestedReplay chan eqlog.Loopback
}

type ReplayRequest struct {
	ByteOffset int64
	TimeOffset time.Duration
	LastLevel  bool
	LastZoning bool
}

func NewContext(invalidateFunc func()) *Context {
	return &Context{
		ParserSession:   0,
		Parser:          nil,
		ViewMenuItems:   make([]MenuItem, 0),
		progressHandler: func(title string, current int64, max int64) {},
		onLogOpenFuncs:  make([]OnLogOpenFunc, 0),
		onLogRowFuncs:   make([]OnLogRowFunc, 0),
		onStatus:        make([]ui.Widget, 0),
		onOverlay:       make([]ui.Widget, 0),
		HelpItems:       make([]HelpItem, 0),
		readyForFollow:  make(chan struct{}, 1),
		requestedReplay: make(chan eqlog.Loopback, 1),
		updateFuncs:     make([]UpdateFunc, 0),
		invalidateFunc:  invalidateFunc,
	}
}
func (c *Context) ParserLogFileOpened(path string) {
	// Extract Character and Servername
	exp := regexp.MustCompile(`^(.*)/eqlog_(.*)_(.*).txt$`)
	if fields := exp.FindStringSubmatch(path); fields != nil {
		for _, f := range c.onLogOpenFuncs {
			f(fields[2], fields[3], 0)
		}
	}
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
	c.readyForFollow <- struct{}{}
}
func (c *Context) CompactStatusElements(style *ui.Style, gtx layout.Context) []layout.Widget {
	items := make([]layout.Widget, 0)
	if c.parserPath != "" {
		items = append(items, func(gtx layout.Context) layout.Dimensions {
			return material.Label(style.Theme, unit.Sp(14), c.parserPath).Layout(gtx)
		})
	}
	for _, f := range c.onStatus {
		items = append(items, func(gtx layout.Context) layout.Dimensions {
			return f(style, gtx)
		})
	}
	return items
}
func (c *Context) runFollow() {
	c.Parser.Follow(c.ParserNewLogEvent)
}
func (c *Context) runReplay() {
	c.isReplay = true
	started := time.Now()
	c.Parser.Replay(c.replayLoopback, c.ParserNewLogEvent, c.ParserOnReplayProgress)
	log.Printf("Replay took: %v", time.Since(started))
	c.isReplay = false
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
	case <-c.readyForFollow:
		c.startParser(c.parserPath, c.runFollow)
		c.invalidateFunc()
	default:
	}
	for _, f := range c.updateFuncs {
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
	for _, f := range c.onLogRowFuncs {
		f(row)
	}
	if !c.isReplay {
		c.invalidateFunc()
	}
}
func (c *Context) AddViewMenuItem(name string, action UIActionFunc) {
	c.ViewMenuItems = append(c.ViewMenuItems, MenuItem{Name: name, Action: action})
}
func (c *Context) AddToolsMenuItem(name string, action UIActionFunc) {
	c.ToolsMenuItems = append(c.ToolsMenuItems, MenuItem{Name: name, Action: action})
}
func (c *Context) OnLogOpen(f OnLogOpenFunc) {
	c.onLogOpenFuncs = append(c.onLogOpenFuncs, f)
}
func (c *Context) OnLogRow(f OnLogRowFunc) {
	c.onLogRowFuncs = append(c.onLogRowFuncs, f)
}
func (c *Context) UpdateFunc(f UpdateFunc) {
	c.updateFuncs = append(c.updateFuncs, f)
}
func (c *Context) OnStatus(f ui.Widget) {
	c.onStatus = append(c.onStatus, f)
}
func (c *Context) OnOverlay(f ui.Widget) {
	c.onOverlay = append(c.onOverlay, f)
}
func (c *Context) SetMainView(f ui.Widget) {
	c.currentMainView = f
}
func (c *Context) AddHelpItem(name string, layout ui.Widget) {
	c.HelpItems = append(c.HelpItems, HelpItem{Name: name, Layout: layout})
}

func (c *Context) RegisterModule(m Module) error {
	return m.Init(c)
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
