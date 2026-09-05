package sky

import (
	"fmt"
	"strings"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget/material"
	"gioui.org/x/component"
	"github.com/uija/eqdps/internal/ui"
)

func (m *Module) Layout(style *ui.Style, gtx layout.Context) layout.Dimensions {
	return layout.Stack{}.Layout(gtx,
		layout.Expanded(func(gtx layout.Context) layout.Dimensions { return m.mainView(style, gtx) }),
		layout.Expanded(func(gtx layout.Context) layout.Dimensions { return m.showOverlay(style, gtx) }),
	)
}
func (m *Module) showOverlay(style *ui.Style, gtx layout.Context) layout.Dimensions {
	if m.show_reset_reload_overlay {
		ui.Fill(gtx, style.Palette.Shadow)
		return ui.Overlay(gtx, 420, style.Palette.Panel, style.Palette.Border,
			func(gtx layout.Context) layout.Dimensions {
				return layout.UniformInset(unit.Dp(32)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return layout.Inset{}.Layout(gtx, ui.HeaderLabel(style, "Reset and Reload").Layout)
						}),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							label := ui.ColorLabel(style.Palette.Muted, ui.Label(style, "This will reset the collected Plane of Sky database and trigger a full reparse of the logfile.\nIf you needed to use inventory updates and/or edited runes manually, you will have to do that again."))
							return layout.Inset{Top: unit.Dp(16), Bottom: unit.Dp(16)}.Layout(gtx, label.Layout)
						}),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
								layout.Flexed(1, ui.RenderLinkAsButton(style, &m.cancel_reset_reload_click, ui.Close, "Cancel")),
								layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
									return layout.E.Layout(gtx, ui.RenderLinkAsButton(style, &m.confirm_reset_reload_click, ui.Refresh, " OK "))
								}),
							)
						}),
					)
				})
			},
		)
	}
	return layout.Dimensions{}
}
func (m *Module) RenderTopRow(active string, style *ui.Style, gtx layout.Context, tools layout.FlexChild) layout.FlexChild {
	return layout.Rigid(func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
			layout.Flexed(1, ui.HeaderLabel(style, "Plane of Sky - "+active).Layout),
			layout.Flexed(1,
				func(gtx layout.Context) layout.Dimensions {
					children := make([]layout.FlexChild, 0)
					if active != "Progression" {
						children = append(children, layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
							return material.Label(style.Theme, ui.Sp(14), "").Layout(gtx)
						}))
						children = append(children,
							layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
								return ui.Link(style, &m.progression_click, "Show Progression").Layout(gtx)
							}),
						)
					}
					if active != "Inventory" {
						children = append(children, layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
							return ui.Link(style, &m.edit_runes_click, "Missing Runes?").Layout(gtx)
						}))
						children = append(children,
							layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
								return ui.Link(style, &m.inventory_click, "Show Inventory").Layout(gtx)
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
		layout.Flexed(2,
			func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
						return ui.IconLink(style, &m.hide_finished, finished_icon, "Show finished Quest").Layout(gtx)
					}),
					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
						return ui.IconLink(style, &m.hide_empty, empty_icon, "Show empty Quest").Layout(gtx)
					}),
					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
						return ui.IconLink(style, &m.reset_reload_click, ui.Refresh, "Reset & Reload").Layout(gtx)
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
					len(m.status)+2,
					func(gtx layout.Context, index int) layout.Dimensions {
						if index == 0 {
							return layout.Inset{Top: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
								return m.RenderReadyToTurnIn(style, gtx)
							})
						} else if index == 1 {
							return layout.Inset{Top: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
								return m.RenderWatched(style, gtx)
							})
						} else {
							return layout.Inset{Top: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
								return m.RenderClassSection(index-2, style, gtx)
							})

						}
					},
				)
			}),
		)
	}
	if m.edit_runes {
		return layout.Stack{}.Layout(gtx,
			layout.Expanded(func(gtx layout.Context) layout.Dimensions {
				return layout.UniformInset(unit.Dp(8)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
				})
			}),
			layout.Expanded(func(gtx layout.Context) layout.Dimensions {
				return layout.UniformInset(unit.Dp(64)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return m.RenderRunesOverlay(style, gtx)
				})
			}),
		)
	} else {
		return layout.UniformInset(unit.Dp(8)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
		})
	}
}
func (m *Module) RenderWatched(style *ui.Style, gtx layout.Context) layout.Dimensions {
	icon := ui.DelBox
	if m.config.HideWatched {
		icon = ui.AddBox
	}
	rows := make([]layout.FlexChild, 0)
	num := 0
	for index := range m.status {
		for qidx, q := range m.status[index].Quests {
			if q.Watched {
				num++
				if !m.config.HideWatched {
					rows = append(rows, m.RenderQuest(index, qidx, true, style, gtx)...)
				}
			}
		}
	}
	if num > 0 {
		ele := layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return ui.ColoredRow(gtx, style.Palette.Panel, func(layout.Context) layout.Dimensions {
				return layout.UniformInset(unit.Dp(8)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					link := ui.IconLink(style, &m.watched_click, icon, fmt.Sprintf("WATCHED (%d)", num))
					link.TextColor = style.Palette.Accent
					return link.Layout(gtx)
				})
			})
		})

		rows = append([]layout.FlexChild{ele}, rows...)
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, rows...)
}
func (m *Module) RenderReadyToTurnIn(style *ui.Style, gtx layout.Context) layout.Dimensions {
	num := 0
	for _, cl := range m.status {
		num += cl.QuestsReady
	}
	icon := ui.DelBox
	if m.config.HideReady {
		icon = ui.AddBox
	}
	rows := make([]layout.FlexChild, 0)
	rows = append(rows, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
		return ui.ColoredRow(gtx, style.Palette.Panel, func(layout.Context) layout.Dimensions {
			return layout.UniformInset(unit.Dp(8)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				link := ui.IconLink(style, &m.ready_turnin_click, icon, fmt.Sprintf("READY TO TURN IN (%d)", num))
				link.TextColor = style.Palette.Yes
				return link.Layout(gtx)
			})
		})
	}))
	if !m.config.HideReady {
		for cidx, cl := range m.status {
			if cl.QuestsReady > 0 {
				for qidx, qs := range cl.Quests {
					if !qs.Done && qs.MissingItems == 0 {
						rows = append(rows, m.RenderQuest(cidx, qidx, true, style, gtx)...)
					}
				}
			}
		}
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, rows...)
}
func (m *Module) RenderClassSection(index int, style *ui.Style, gtx layout.Context) layout.Dimensions {
	rows := make([]layout.FlexChild, 0)
	cl := &m.status[index]
	if index == 0 {
		rows = append(rows, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Bottom: unit.Dp(4)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return ui.ColoredRow(gtx, style.Palette.Panel, func(gtx layout.Context) layout.Dimensions {
					return layout.UniformInset(unit.Dp(8)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						gtx.Constraints.Min.X = gtx.Constraints.Max.X
						return ui.HeaderLabel(style, "Class Quests").Layout(gtx)
					})
				})
			})
		}))
	}
	rows = append(rows, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
		return cl.ToggleClick.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return ui.ColoredRow(gtx, style.Palette.Panel, func(gtx layout.Context) layout.Dimensions {
				return layout.UniformInset(unit.Dp(8)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
						layout.Flexed(2, func(gtx layout.Context) layout.Dimensions {
							icon := ui.AddBox
							if cl.Visible {
								icon = ui.DelBox
							}
							col := style.Palette.Accent
							if cl.ToggleClick.Hovered() {
								col = style.Palette.LinkHover
							}
							return ui.ColoredIconLabel(gtx, style.Theme, HeaderSize, icon, col, cl.Name)
						}),
						layout.Flexed(2, func(gtx layout.Context) layout.Dimensions {
							numQuests := len(cl.Quests)
							col := style.Palette.Accent
							if cl.QuestsReady > 0 {
								col = style.Palette.Yes
							}
							return ui.ColorLabel(col, material.Label(style.Theme, ui.Sp(HeaderSize), fmt.Sprintf("%d/%d done - %d ready", cl.QuestsDone, numQuests, cl.QuestsReady))).Layout(gtx)
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
	for qidx := range m.status[index].Quests {
		rows = append(rows, m.RenderQuest(index, qidx, false, style, gtx)...)
	}
	return rows
}

func (m *Module) RenderQuest(index int, qidx int, fullname bool, style *ui.Style, gtx layout.Context) []layout.FlexChild {
	rows := make([]layout.FlexChild, 0)
	quest := m.status[index].Quests[qidx]

	title_color := style.Theme.Fg
	highlight_color := style.Palette.Accent
	missing_text := fmt.Sprintf("Missing %d", quest.MissingItems)
	show := true
	if quest.Done && m.config.RedoQuests[quest.Key].IsZero() {
		title_color = style.Palette.Done
		highlight_color = style.Palette.Done
		missing_text = "Done"
		if m.config.HideFinished {
			show = false
		}
	} else if quest.MissingItems == len(quest.Items) && m.config.HideEmpty {
		show = false
	}
	quest_name := quest.Name
	if fullname {
		quest_name = fmt.Sprintf("%s - %s", m.status[index].Name, quest.Name)
	}
	if show {
		rows = append(rows,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Top: unit.Dp(4)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
						layout.Flexed(3, func(gtx layout.Context) layout.Dimensions {
							label := ui.ColorLabel(title_color, material.Label(style.Theme, ui.Sp(RowSize), quest_name))
							label.Font.Weight = font.SemiBold
							return layout.Inset{Top: unit.Dp(2), Left: unit.Dp(16)}.Layout(gtx, label.Layout)
						}),
						layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
							children := make([]layout.FlexChild, 0)
							children = append(children, layout.Flexed(1, ui.Label(style, "").Layout))
							if quest.Done || !m.config.RedoQuests[quest.Key].IsZero() {
								if m.config.RedoQuests[quest.Key].IsZero() {
									children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
										link := ui.IconLink(style, &m.status[index].Quests[qidx].RedoQuestClick, ui.Refresh, "")
										link.TextColor = highlight_color
										tip := component.PlatformTooltip(style.Theme, "Redo the quest")
										return layout.Inset{Right: unit.Dp(4)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
											return ui.LayoutTooltip(gtx, &m.status[index].Quests[qidx].RedoQuestTooptip, tip, link.Layout)
										})
									}))
								} else {
									children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
										link := ui.IconLink(style, &m.status[index].Quests[qidx].RedoQuestClick, ui.Check, "")
										link.TextColor = highlight_color
										tip := component.PlatformTooltip(style.Theme, "Cancel redo")
										return layout.Inset{Right: unit.Dp(4)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
											return ui.LayoutTooltip(gtx, &m.status[index].Quests[qidx].RedoQuestTooptip, tip, link.Layout)
										})
									}))
								}
							}
							if !quest.Watched {
								children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									link := ui.IconLink(style, &m.status[index].Quests[qidx].WatchClick, ui.ActionVisibility, "")
									link.TextColor = highlight_color
									tip := component.PlatformTooltip(style.Theme, "Add to watchlist")
									return layout.Inset{Right: unit.Dp(4)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
										return ui.LayoutTooltip(gtx, &m.status[index].Quests[qidx].WatchTooltip, tip, link.Layout)
									})
								}))
							} else if fullname {
								children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									link := ui.IconLink(style, &m.status[index].Quests[qidx].UnwatchClick, ui.ActionVisibilityOff, "")
									link.TextColor = highlight_color
									tip := component.PlatformTooltip(style.Theme, "Remove from watchlist")
									return layout.Inset{Right: unit.Dp(4)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
										return ui.LayoutTooltip(gtx, &m.status[index].Quests[qidx].WatchTooltip, tip, link.Layout)
									})
								}))
							}
							return layout.Flex{Axis: layout.Horizontal}.Layout(gtx, children...)
						}),
						layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
							// TODO Evaluate value
							return ui.ColorLabel(title_color, material.Label(style.Theme, ui.Sp(RowSize), missing_text)).Layout(gtx)
						}),
						layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
							return material.Label(style.Theme, ui.Sp(RowSize), "").Layout(gtx)
						}),
						layout.Flexed(3, func(gtx layout.Context) layout.Dimensions {
							txt := fmt.Sprintf("%s - %s", quest.QuestGiver, quest.Reward)
							link := ui.Link(style, &m.status[index].Quests[qidx].RewardClick, txt)
							link.Size = RowSize
							link.TextColor = highlight_color
							return link.Layout(gtx)
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
						if quest.Done && m.config.RedoQuests[quest.Key].IsZero() {
							amount_text = "-"
							color = style.Palette.Done
						} else if item.Amount > 0 {
							color = style.Palette.Yes
						} else {
							prefix = "-"
						}
						return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
							layout.Flexed(5, func(gtx layout.Context) layout.Dimensions {
								return layout.Inset{Left: unit.Dp(32)}.Layout(gtx,
									ui.ColorLabel(color, material.Label(style.Theme, ui.Sp(RowSize), fmt.Sprintf("%s %s", prefix, item.Name))).Layout,
								)
							}),
							layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
								return ui.ColorLabel(color, material.Label(style.Theme, ui.Sp(RowSize), amount_text)).Layout(gtx)
							}),
							layout.Flexed(3, func(gtx layout.Context) layout.Dimensions {
								return ui.ColorLabel(color, material.Label(style.Theme, ui.Sp(RowSize), item.Hint)).Layout(gtx)
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
func (m *Module) Missing(quest Quest) int {
	missing := 0

	for _, r := range quest.Requirements {
		if m.config.QuestItems[r.Name] < 1 {
			missing++
		}
	}

	return missing
}
