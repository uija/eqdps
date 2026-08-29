package statistics

import (
	"database/sql"
	"fmt"
	"log"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"gioui.org/layout"
	"gioui.org/widget"
	_ "github.com/glebarez/go-sqlite"
	"github.com/uija/eqdps/internal/data"
	"github.com/uija/eqdps/internal/eqlog"
	"github.com/uija/eqdps/internal/module"
)

const DATABASE_VERSION = 1

var zoneVariantSuffixRE = regexp.MustCompile(`(?: - (?:Group|Solo)(?: [0-9]+)?(?: \([^)]+\))?| [0-9]+ \([^)]+\))$`)

func normalizeZoneName(name string) string {
	return zoneVariantSuffixRE.ReplaceAllString(strings.TrimSpace(name), "")
}

type Module struct {
	mu   sync.RWMutex
	ctx  *module.Context
	list widget.List
	db   *sql.DB

	replay         atomic.Bool
	importRunning  atomic.Bool
	activeImport   *Import
	currentZone    int64
	currentVisit   int64
	currentVisitAt time.Time
	lastImportRow  time.Time
	pendingCamp    time.Time
	loginSequence  time.Time
	preLoginRow    time.Time
	pendingKills   []pendingStatisticsKill
	lastCoinReward time.Time
	lastCoinID     int64
	importDone     chan struct{}

	logPath     string
	updateClick widget.Clickable
	reloadClick widget.Clickable
	Pages       []StatsPage
	currentPage StatsPage

	replayProgress *eqlog.ReplayProgress

	lastKnownOffset   int64
	lastLogfileOffset int64

	invalidateFunc func()
}

func NewModule() *Module {
	return &Module{
		Pages:          make([]StatsPage, 0),
		invalidateFunc: func() {},
		importDone:     make(chan struct{}, 1),
	}
}
func (m *Module) Init(ctx *module.Context, invalidFunc func()) error {
	ctx.RegisterUpdate(m.Update)
	ctx.AddSidebarItem("Stats", m.OpenMainView)
	ctx.AddViewMenuItem("Statistics", m.OpenMainView)
	ctx.RegisterLogOpen(m.OnLogOpen)
	ctx.RegisterReplayStart(m.OnExternalReplayStart)
	ctx.RegisterReplayEnd(m.OnExternalReplayEnd)
	ctx.RegisterLogRow(m.OnExternalLogRow)
	m.ctx = ctx
	m.invalidateFunc = invalidFunc
	m.list.Axis = layout.Vertical

	m.Pages = append(m.Pages, NewOverviewPage())
	m.Pages = append(m.Pages, NewZonesPage())
	m.Pages = append(m.Pages, NewMobsPage(invalidFunc))
	m.Pages = append(m.Pages, NewItemsPage(invalidFunc))
	m.currentPage = m.Pages[0]

	return nil
}
func (m *Module) OpenMainView() {
	m.ctx.SetMainView(m.Layout)
}
func (m *Module) OnLogOpen(characterName, serverName string, filesize int64, path string) bool {
	m.lastLogfileOffset = filesize
	logdir := filepath.Dir(path)
	database_dir := filepath.Join(logdir, fmt.Sprintf("eqdps_%s_%s.sqlite", characterName, serverName))
	db, err := sql.Open("sqlite", database_dir+"?_pragma=foreign_keys(1)")
	if err != nil {
		log.Printf("Unable to initialize database. %v", err)
	} else {
		if m.db != nil {
			m.db.Close()
		}
		id, err := GetUserVersion(db)
		if err != nil {
			db.Close()
			log.Printf("Unable to lead user version. %v", err)
			m.db = nil
			m.logPath = path
			return true
		}
		if id != DATABASE_VERSION {
			if err = DropTables(db); err != nil {
				db.Close()
				log.Printf("Unable to drop old tables. %v", err)
				m.db = nil
				m.logPath = path
				return true
			}
		}
		if err := PrepareDb(db); err != nil {
			db.Close()
			log.Printf("Unable to prepare statistics database. %v", err)
			m.db = nil
			m.logPath = path
			return true
		}
		m.db = db
		if err := SetUserVersion(m.db, DATABASE_VERSION); err != nil {
			db.Close()
			log.Printf("Unable to prepare statistics database. %v", err)
			m.db = nil
			m.logPath = path
			return true
		}
		offset, err := GetLogOffset(m.db)
		if err == nil {
			m.lastKnownOffset = offset
		}
	}
	for idx := range m.Pages {
		m.Pages[idx].Reset()
		m.Pages[idx].SetDb(db)
	}
	m.logPath = path
	return true
}

func (m *Module) Update(gtx layout.Context) {
	select {
	case <-m.importDone:
		for idx := range m.Pages {
			m.Pages[idx].Reset()
		}
	default:
	}
	if m.updateClick.Clicked(gtx) {
		m.RunImport()
	}
	if m.reloadClick.Clicked(gtx) {
		if m.importRunning.Load() || m.db == nil {
			return
		}
		for idx := range m.Pages {
			m.Pages[idx].Reset()
		}
		if err := DropTables(m.db); err != nil {
			log.Printf("Unable to drop tables. %v", err)
			return
		}
		if err := PrepareDb(m.db); err != nil {
			log.Printf("Unable to prepare db. %v", err)
			return
		}
		m.RunImport()
	}
	for idx := range m.Pages {
		if m.Pages[idx].Clickable().Clicked(gtx) {
			m.currentPage = m.Pages[idx]
		}
		m.Pages[idx].Update(gtx)
	}
}
func (m *Module) OnExternalLogRow(e *data.LogRowEvent) {
	// we try to keep track of the current file size, to estimate backlog
	if m.replay.Load() || e.Offset < m.lastLogfileOffset {
		return
	}
	m.lastLogfileOffset = e.Offset
}
func (m *Module) OnExternalReplayStart() {
	m.replay.Store(true)
}
func (m *Module) OnExternalReplayEnd() {
	m.replay.Store(false)
}
func (m *Module) Shutdown() {
	if m.db != nil {
		m.db.Close()
	}
}

func FormatBytes(bytes int64) string {
	const step = 1024

	units := []string{"B", "KB", "MB", "GB", "TB", "PB", "EB"}
	value := float64(bytes)
	unit := 0

	for value >= step && unit < len(units)-1 {
		value /= step
		unit++
	}

	if unit == 0 {
		return fmt.Sprintf("%dB", bytes)
	}
	return fmt.Sprintf("%.1f%s", value, units[unit])
}
