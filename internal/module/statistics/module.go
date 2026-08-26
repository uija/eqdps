package statistics

import (
	"database/sql"
	"log"
	"regexp"
	"strings"
	"sync"

	"gioui.org/layout"
	"gioui.org/widget"
	"github.com/uija/eqdps/internal/data"
	"github.com/uija/eqdps/internal/eqlog"
	"github.com/uija/eqdps/internal/module"
	"github.com/uija/eqdps/internal/module/statistics/page"
	//_ "github.com/glebarez/go-sqlite"
)

var zoneVariantSuffixRE = regexp.MustCompile(`(?: - (?:Group|Solo)(?: [0-9]+)?(?: \([^)]+\))?| [0-9]+ \([^)]+\))$`)

func normalizeZoneName(name string) string {
	return zoneVariantSuffixRE.ReplaceAllString(strings.TrimSpace(name), "")
}

type Module struct {
	mu   sync.RWMutex
	ctx  *module.Context
	list widget.List
	db   *sql.DB

	logPath     string
	updateClick widget.Clickable
	Pages       []page.StatsPage

	replayProgress *eqlog.ReplayProgress

	invalidateFunc func()
}

func NewModule() *Module {
	return &Module{
		Pages:          make([]page.StatsPage, 0),
		invalidateFunc: func() {},
	}
}
func (m *Module) Init(ctx *module.Context, invalidFunc func()) error {
	ctx.RegisterUpdate(m.Update)
	ctx.AddSidebarItem("Stats", func() {
		ctx.SetMainView(m.Layout)
	})
	ctx.RegisterLogOpen(m.OnLogOpen)
	m.ctx = ctx
	m.invalidateFunc = invalidFunc
	m.list.Axis = layout.Vertical

	m.Pages = append(m.Pages, page.NewOverviewPage())
	m.Pages = append(m.Pages, page.NewZonesPage())
	return nil
}
func (m *Module) OnLogOpen(characterName, serverName string, filesize int64, path string) bool {
	m.logPath = path
	return true
}

func (m *Module) Update(gtx layout.Context) {
	if m.updateClick.Clicked(gtx) {
		if m.logPath == "" {
			return
		}
		parser := eqlog.NewParser(2)
		parser.Open(m.logPath)
		go func() {
			parser.Replay(eqlog.Loopback{}, m.OnLogRow, m.OnProgress)
			log.Printf("Done with parsing")
		}()
	}
}
func (m *Module) OnLogRow(lre *data.LogRowEvent) {

}
func (m *Module) OnProgress(rp eqlog.ReplayProgress) {
	defer m.invalidateFunc()
	if rp.Bytes == rp.Total {
		m.replayProgress = nil
		return
	}
	m.replayProgress = &rp
}
func (m *Module) Shutdown() {
}
