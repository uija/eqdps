package sky

import (
	"fmt"
	"strings"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget/material"
	"github.com/uija/eqdps/internal/ui"
)

func (m *Module) Layout(style *ui.Style, gtx layout.Context) layout.Dimensions {
	return m.mainView(style, gtx)
}
func (m *Module) RenderTopRow(active string, style *ui.Style, gtx layout.Context, tools layout.FlexChild) layout.FlexChild {
	return layout.Rigid(func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
			layout.Flexed(1, material.Label(style.Theme, unit.Sp(17), "Plane of Sky - "+active).Layout),
			layout.Flexed(1,
				func(gtx layout.Context) layout.Dimensions {
					children := make([]layout.FlexChild, 0)
					if active != "Progression" {
						children = append(children,
							layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
								return m.progression_click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
									return ui.IconLabel(gtx, style.Theme, 15, ui.ActionVisibility, "Show Progression")
								})
								//return m.progression_click.Layout(gtx, material.Label(style.Theme, unit.Sp(15), "Show Progression").Layout)
							}),
						)
					}
					if active != "Inventory" {
						children = append(children,
							layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
								return m.inventory_click.Layout(gtx, material.Label(style.Theme, unit.Sp(15), "Show Inventory").Layout)
							}),
						)
					}
					return layout.Flex{Axis: layout.Horizontal}.Layout(gtx, children...)
				},
			),
			tools,
		)
	})
}

func (m *Module) MainView(style *ui.Style, gtx layout.Context) layout.Dimensions {
	finished_icon := ui.CheckBox
	if m.config.HideFinished {
		finished_icon = ui.CheckBoxOutline
	}
	empty_icon := ui.CheckBox
	if m.config.HideEmpty {
		empty_icon = ui.CheckBoxOutline
	}
	children := make([]layout.FlexChild, 0)
	children = append(children, m.RenderTopRow("Progression", style, gtx,
		layout.Flexed(1,
			func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
						return m.hide_finished.Layout(gtx,
							func(gtx layout.Context) layout.Dimensions {
								return ui.IconLabel(gtx, style.Theme, 15, finished_icon, "Show finished Quests")
							},
						)
					}),
					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
						return m.hide_empty.Layout(gtx,
							func(gtx layout.Context) layout.Dimensions {
								return ui.IconLabel(gtx, style.Theme, 15, empty_icon, "Show empty Quests")
							},
						)
					}),
				)
			},
		),
	))
	if !m.replay {
		children = append(children,
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				list := material.List(style.Theme, &m.questlist)
				return list.Layout(
					gtx,
					len(m.status),
					func(gtx layout.Context, index int) layout.Dimensions {
						return layout.Inset{Top: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							return m.RenderClassSection(index, style, gtx)
						})
					},
				)
			}),
		)
	}

	return layout.UniformInset(unit.Dp(8)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
	})
}
func (m *Module) RenderClassSection(index int, style *ui.Style, gtx layout.Context) layout.Dimensions {
	rows := make([]layout.FlexChild, 0)
	cl := &m.status[index]
	rows = append(rows, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
		return cl.ToggleClick.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return ui.ColoredRow(gtx, style.Palette.Panel, func(layout.Context) layout.Dimensions {
				return layout.UniformInset(unit.Dp(8)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
						layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
							icon := ui.AddBox
							if cl.Visible {
								icon = ui.DelBox
							}
							return ui.ColoredIconLabel(gtx, style.Theme, HeaderSize, icon, style.Palette.Accent, cl.Name)
							//return ui.ColoredLabel(style.Theme, HeaderSize, style.Palette.Accent, cl.Name).Layout(gtx)
						}),
						layout.Flexed(2, func(gtx layout.Context) layout.Dimensions {
							numQuests := len(cl.Quests)
							return ui.ColoredLabel(style.Theme, HeaderSize, style.Palette.Accent, fmt.Sprintf("%d/%d done - %d ready", cl.QuestsDone, numQuests, cl.QuestsReady)).Layout(gtx)
						}),
					)
				})
			})
		})
	}))
	if cl.Visible {
		rows = append(rows, m.RenderClassQuests(index, style, gtx)...)
	}

	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, rows...)
}

func (m *Module) RenderClassQuests(index int, style *ui.Style, gtx layout.Context) []layout.FlexChild {
	rows := make([]layout.FlexChild, 0)
	for qidx, quest := range m.status[index].Quests {
		title_color := style.Theme.Fg
		highlight_color := style.Palette.Accent
		missing_text := fmt.Sprintf("Missing %d", quest.MissingItems)
		show := true
		if quest.Done {
			title_color = style.Palette.Done
			highlight_color = style.Palette.Done
			missing_text = "Done"
			if m.config.HideFinished {
				show = false
			}
		} else if quest.MissingItems == len(quest.Items) && m.config.HideEmpty {
			show = false
		}
		if show {
			rows = append(rows,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Inset{Top: unit.Dp(4)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
							layout.Flexed(2, func(gtx layout.Context) layout.Dimensions {
								label := ui.ColoredLabel(style.Theme, RowSize, title_color, quest.Name)
								label.Font.Weight = font.SemiBold
								return layout.Inset{Left: unit.Dp(16)}.Layout(gtx, label.Layout)
							}),
							layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
								if !quest.Watched {
									return m.status[index].Quests[qidx].WatchClick.Layout(gtx,
										ui.ColoredLabel(style.Theme, RowSize, highlight_color, "Watch").Layout,
									)
								} else {
									return ui.ColoredLabel(style.Theme, RowSize, highlight_color, "   ").Layout(gtx)
								}
							}),
							layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
								// TODO Evaluate value
								return ui.ColoredLabel(style.Theme, RowSize, title_color, missing_text).Layout(gtx)
							}),
							layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
								return material.Label(style.Theme, unit.Sp(RowSize), "").Layout(gtx)
							}),
							layout.Flexed(3, func(gtx layout.Context) layout.Dimensions {
								txt := fmt.Sprintf("%s - %s", quest.QuestGiver, quest.Reward)

								return m.status[index].Quests[qidx].RewardClick.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
									return ui.ColoredLabel(style.Theme, RowSize, highlight_color, txt).Layout(gtx)
								})
							}),
						)
					})
				}),
			)
			for _, item := range quest.Items {
				rows = append(rows,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return layout.Inset{Top: unit.Dp(2), Bottom: unit.Dp(2)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							color := style.Palette.No

							prefix := "+"
							amount_text := fmt.Sprintf("%d", item.Amount)
							if quest.Done {
								amount_text = "-"
								color = style.Palette.Done
							} else if item.Amount > 0 {
								color = style.Palette.Yes
							} else {
								prefix = "-"
							}
							return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
								layout.Flexed(4, func(gtx layout.Context) layout.Dimensions {
									return layout.Inset{Left: unit.Dp(32)}.Layout(gtx,
										ui.ColoredLabel(style.Theme, RowSize, color, fmt.Sprintf("%s %s", prefix, item.Name)).Layout,
									)
								}),
								layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
									return ui.ColoredLabel(style.Theme, RowSize, color, amount_text).Layout(gtx)
								}),
								layout.Flexed(3, func(gtx layout.Context) layout.Dimensions {
									return ui.ColoredLabel(style.Theme, RowSize, color, item.Hint).Layout(gtx)
								}),
							)
						})
					}),
				)
			}
		}
	}
	return rows
}

func QuestName(quest string, class string) string {
	return strings.TrimSpace(strings.TrimPrefix(quest, class))
}
func (m *Module) Missing(quest Quest) int {
	missing := 0

	for _, r := range quest.Requirements {
		if m.config.QuestItems[r.Name] < 1 {
			missing++
		}
	}

	return missing
}
