package sky

import (
	"fmt"
	"log"
	"regexp"
	"strconv"
	"strings"

	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"github.com/uija/eqdps/internal/data"
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

	questlist          widget.List
	class_closed       map[string]bool
	class_click        []widget.Clickable
	quest_reward_click map[string]*widget.Clickable
}

func NewModule() *Module {
	return &Module{
		class_closed: make(map[string]bool),
	}
}

func (m *Module) Init(ctx *module.Context) error {
	ctx.AddViewMenuItem("Plane of Sky Quest Tracker", m.OpenMainView)
	ctx.RegisterLogOpen(m.OnLogOpen)
	ctx.RegisterLogRow(m.OnLogRow)
	ctx.RegisterUpdate(m.Update)
	ctx.SetMainView(m.MainView)
	ctx.AddHelpItem("Plane of Sky Quest Tracker", m.LayoutHelp)
	m.ctx = ctx
	m.db, _ = LoadDatabase()
	m.class_click = make([]widget.Clickable, len(m.db.Classes))
	m.quest_reward_click = make(map[string]*widget.Clickable)
	m.questlist.Axis = layout.Vertical
	return nil
}

func (m *Module) OpenMainView() {
	m.ctx.SetMainView(m.MainView)
}

func (m *Module) Shutdown() {

}
func (m *Module) Update(gtx layout.Context) {
	if len(m.db.Classes) == len(m.class_click) {
		for idx, c := range m.db.Classes {
			if m.class_click[idx].Clicked(gtx) {
				m.class_closed[c.Name] = !m.IsClassClosed(c.Name)
			}
		}
	}
	for reward, clickable := range m.quest_reward_click {
		if clickable.Clicked(gtx) {
			log.Printf("%s clicked", reward)
		}
	}
}

func (m *Module) OnLogOpen(characterName string, serverName string, size int64, path string) {

}
func (m *Module) IsClassClosed(name string) bool {
	v, ok := m.class_closed[name]
	return v && ok
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
	return layout.UniformInset(unit.Dp(8)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return material.Label(style.Theme, unit.Sp(15), "Plane of Sky").Layout(gtx)
			}),
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				list := material.List(style.Theme, &m.questlist)
				return list.Layout(
					gtx,
					len(m.db.Classes),
					func(gtx layout.Context, index int) layout.Dimensions {
						return layout.Inset{Top: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							return m.RenderClassSection(index, style, gtx)
						})
					},
				)
			}),
		)
	})
}
func (m *Module) RenderClassSection(index int, style *ui.Style, gtx layout.Context) layout.Dimensions {
	rows := make([]layout.FlexChild, 0)
	rows = append(rows, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
		return m.class_click[index].Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return ui.ColoredRow(gtx, style.Palette.Panel, func(layout.Context) layout.Dimensions {
				return layout.UniformInset(unit.Dp(8)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
						layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
							return ui.ColoredLabel(style.Theme, HeaderSize, style.Palette.Accent, m.db.Classes[index].Name).Layout(gtx)
						}),
						layout.Flexed(2, func(gtx layout.Context) layout.Dimensions {
							numQuests := len(m.db.Classes[index].Quests)
							return ui.ColoredLabel(style.Theme, HeaderSize, style.Palette.Accent, fmt.Sprintf("%d/%d done - %d ready", 0, numQuests, 0)).Layout(gtx)
						}),
					)
				})
			})
		})
	}))
	if !m.IsClassClosed(m.db.Classes[index].Name) {
		rows = append(rows, m.RenderClassQuests(index, style, gtx)...)
	}

	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, rows...)
}
func (m *Module) RenderClassQuests(index int, style *ui.Style, gtx layout.Context) []layout.FlexChild {
	rows := make([]layout.FlexChild, 0)
	for _, quest := range m.db.Classes[index].Quests {
		rows = append(rows,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Top: unit.Dp(4)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
						layout.Flexed(2, func(gtx layout.Context) layout.Dimensions {
							return layout.Inset{Left: unit.Dp(16)}.Layout(gtx, material.Label(style.Theme, unit.Sp(RowSize), QuestName(quest.Name, m.db.Classes[index].Name)).Layout)
						}),
						layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
							return ui.ColoredLabel(style.Theme, RowSize, style.Palette.Accent, "Watch").Layout(gtx)
						}),
						layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
							return material.Label(style.Theme, unit.Sp(RowSize), "Missing 2").Layout(gtx)
						}),
						layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
							return material.Label(style.Theme, unit.Sp(RowSize), "").Layout(gtx)
						}),
						layout.Flexed(3, func(gtx layout.Context) layout.Dimensions {
							txt := fmt.Sprintf("%s - %s", quest.QuestGiver, quest.Rewards[0])

							cl, ok := m.quest_reward_click[quest.Rewards[0]]
							if !ok {
								cl = &widget.Clickable{}
								m.quest_reward_click[quest.Rewards[0]] = cl
							}
							return cl.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
								return ui.ColoredLabel(style.Theme, RowSize, style.Palette.Accent, txt).Layout(gtx)
							})
						}),
					)
				})
			}),
		)
		for _, item := range quest.Requirements {
			rows = append(rows,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Inset{Top: unit.Dp(2), Bottom: unit.Dp(2)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						color := style.Palette.Yes
						return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
							layout.Flexed(4, func(gtx layout.Context) layout.Dimensions {
								return layout.Inset{Left: unit.Dp(32)}.Layout(gtx, ui.ColoredLabel(style.Theme, RowSize, color, item.Name).Layout)
							}),
							layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
								return ui.ColoredLabel(style.Theme, RowSize, color, "0").Layout(gtx)
							}),
							layout.Flexed(3, func(gtx layout.Context) layout.Dimensions {
								txt := item.DropsFrom
								if txt == "" {
									txt = "Plane of Sky random drop"
								}
								return ui.ColoredLabel(style.Theme, RowSize, color, txt).Layout(gtx)
							}),
						)
					})
				}),
			)
		}
	}
	return rows
}
func QuestName(quest string, class string) string {
	return strings.TrimSpace(strings.TrimPrefix(quest, class))
}
