package sky

import (
	"log"
	"regexp"
	"strconv"
	"strings"

	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget/material"
	"github.com/uija/eqdps/internal/data"
	"github.com/uija/eqdps/internal/module"
	"github.com/uija/eqdps/internal/ui"
)

var itemUpgradeSuffixRE = regexp.MustCompile(` \+[0-9]+$`)

type Module struct {
	ctx *module.Context
	db  Database
}

func (m *Module) Init(ctx *module.Context) error {
	ctx.AddViewMenuItem("Plane of Sky Quest Tracker", m.OpenMainView)
	ctx.OnLogOpen(m.OnLogOpen)
	ctx.OnLogRow(m.OnLogRow)
	ctx.AddHelpItem("Plane of Sky Quest Tracker", m.LayoutHelp)
	m.ctx = ctx
	m.db, _ = LoadDatabase()
	return nil
}

func (m *Module) OpenMainView() {
	m.ctx.SetMainView(m.MainView)
}

func (m *Module) Shutdown() {

}

func (m *Module) OnLogOpen(characterName string, serverName string, size int64) {

}
func (m *Module) HandleLootedItems(quantity int, item string) {
	for _, c := range m.db.Classes {
		for _, q := range c.Quests {
			for _, r := range q.Requirements {
				if strings.Contains(item, "Brass") {
					log.Printf("Comparing '%s' to '%s'", item, r.Name)
				}
				if strings.EqualFold(item, r.Name) {
					log.Printf("Found Quest Item for %s Quest %s: %d %s", c.Name, q.Name, quantity, item)
				}
			}
		}
	}

}
func (m *Module) OnLogRow(event *data.LogRowEvent) {
	switch event.Type {
	case data.LogRowEventTypeLoot:
		quantity, item := normalizeItemName(event.Data[1])
		m.HandleLootedItems(quantity, item)
	}
}
func normalizeItemName(value string) (int, string) {
	value = strings.TrimSpace(value)
	prefix, item, ok := strings.Cut(value, " ")
	if !ok {
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

	item = strings.TrimSpace(item)
	return quantity, itemUpgradeSuffixRE.ReplaceAllString(item, "")
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

func (m *Module) MainView(style *ui.Style, gtx layout.Context) layout.Dimensions {
	return layout.Inset{Top: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		label := material.Label(style.Theme, unit.Sp(15), "Plane of Sky")
		//label.Color = palette.muted
		return label.Layout(gtx)
	})
}
