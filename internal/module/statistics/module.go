package statistics

import (
	"database/sql"
	"fmt"
	"log"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"

	"gioui.org/layout"
	"gioui.org/widget"
	"github.com/uija/eqdps/internal/data"
	"github.com/uija/eqdps/internal/module"
	"github.com/uija/eqdps/internal/ui"
	//_ "github.com/glebarez/go-sqlite"
)

var itemUpgradeSuffixRE = regexp.MustCompile(` \+[0-9]+$`)
var zoneVariantSuffixRE = regexp.MustCompile(`(?: - (?:Group|Solo)(?: [0-9]+)?(?: \([^)]+\))?| [0-9]+ \([^)]+\))$`)

func normalizeItemName(value string) (int, string) {
	value = strings.TrimSpace(value)
	prefix, name, found := strings.Cut(value, " ")
	if !found {
		return 1, itemUpgradeSuffixRE.ReplaceAllString(value, "")
	}

	quantity := 1
	if !strings.EqualFold(prefix, "a") && !strings.EqualFold(prefix, "an") {
		parsed, err := strconv.Atoi(prefix)
		if err != nil || parsed < 1 {
			return 1, itemUpgradeSuffixRE.ReplaceAllString(value, "")
		}
		quantity = parsed
	}

	name = itemUpgradeSuffixRE.ReplaceAllString(strings.TrimSpace(name), "")
	return quantity, name
}

func normalizeZoneName(name string) string {
	return zoneVariantSuffixRE.ReplaceAllString(strings.TrimSpace(name), "")
}

type Module struct {
	mu     sync.RWMutex
	ctx    *module.Context
	list   widget.List
	replay atomic.Bool

	configPath  string
	readyToRead bool

	currentZone int64
	byteOffset  int64

	db *sql.DB
}

func NewModule() *Module {
	return &Module{}
}
func (m *Module) Init(ctx *module.Context, _ func()) error {
	ctx.RegisterLogRow(m.OnLogRow)
	ctx.RegisterUpdate(m.Update)
	ctx.RegisterReplayStart(m.OnReplayStart)
	ctx.RegisterReplayEnd(m.OnReplayEnd)
	ctx.RegisterLogOpen(m.OnLogOpen)
	ctx.AddSidebarItem("Stats", func() {
		ctx.SetMainView(m.Layout)
	})
	m.ctx = ctx
	m.readyToRead = false
	m.list.Axis = layout.Vertical
	return nil
}

func (m *Module) Update(gtx layout.Context) {
}
func (m *Module) OnLogRow(e *data.LogRowEvent) {
	if e.Offset < m.byteOffset || !m.readyToRead {
		return
	}
	if !m.readyToRead && e.Offset >= m.byteOffset {
		m.readyToRead = true
	}
	m.byteOffset = e.Offset
	if !m.replay.Load() {
		defer func() {
			err := SetLogOffset(m.db, m.byteOffset)
			if err != nil {
				log.Printf("Unable to write byte offset. %v", err)
			}
		}()
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	switch e.Type {
	case data.LogRowEventTypeZoneChange:
		id, err := GetOrCreateZone(m.db, normalizeZoneName(e.Data[1]))
		if err != nil {
			log.Printf("Unable to query database. %v", err)
			return
		}
		m.currentZone = id
		err = IncrementZoneVisits(m.db, id)
		if err != nil {
			log.Printf("Unable to query database. %v", err)
			return
		}
	case data.LogRowEventTypeLoot,
		data.LogRowEventTypeLootResult:
		quantity, item := normalizeItemName(e.Data[1])
		mob := e.Data[2]
		mobid, err := GetOrCreateMob(m.db, m.currentZone, mob)
		if err != nil {
			log.Printf("Unable to query database. %v", err)
			return
		}
		itemid, err := GetOrCreateItem(m.db, mobid, item)
		if err != nil {
			log.Printf("Unable to query database. %v", err)
			return
		}
		err = IncrementItemLooted(m.db, itemid, quantity)
	case data.LogRowEventTypeYouSlain,
		data.LogRowEventTypeSlainBy:
		who := e.Data[1]
		if !strings.EqualFold(who, "you") {
			mobid, err := GetOrCreateMob(m.db, m.currentZone, who)
			if err != nil {
				log.Printf("Unable to query database. %v", err)
				return
			}
			err = IncrementMobKills(m.db, mobid)
			if err != nil {
				log.Printf("Unable to query database. %v", err)
				return
			}
		}
	}
}
func (m *Module) OnLogOpen(characterName string, serverName string, size int64, path string) bool {
	// Extract path
	base_path := filepath.Dir(path)
	m.configPath = fmt.Sprintf("%s/eqdps_%s_%s.sqlite?_pragma=foreign_keys(1)", base_path, characterName, serverName)
	log.Printf("Database: %s", m.configPath)
	db, err := sql.Open("sqlite", m.configPath)
	if err != nil {
		log.Printf("Unable to open database. %v", err)
		return true
	}
	if err := PrepareDb(db); err != nil {
		log.Printf("Unable to prepare database. %v", err)
		return true
	}
	m.db = db
	o, err := GetLogOffset(m.db)
	if err != nil {
		log.Printf("Unable to read byte offset. %v", err)
		return true
	}
	m.byteOffset = o
	m.readyToRead = true
	return true
}
func (m *Module) OnReplayStart() {
	m.replay.Store(true)
}
func (m *Module) OnReplayEnd() {
	err := SetLogOffset(m.db, m.byteOffset)
	if err != nil {
		log.Printf("Error storing offset. %v", err)
	}
	m.replay.Store(false)
}
func (m *Module) Shutdown() {
}

func (m *Module) Layout(style *ui.Style, gtx layout.Context) layout.Dimensions {
	if m.replay.Load() {
		return layout.Dimensions{}
	}
	return layout.Dimensions{}
}
