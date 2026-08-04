package main

import (
	"fmt"
	"image"
	"image/color"
	"net/url"
	"strings"

	"gioui.org/font"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"github.com/uija/eqdps/internal/platform"
	"github.com/uija/eqdps/internal/skyquest"
)

type skyRow struct {
	kind                     string
	name, status, have, need string
	detail                   string
	reward                   string
	rewardClick              *widget.Clickable
	questName                string
	watched                  bool
	watchClick               *widget.Clickable
	toggleClick              *widget.Clickable
	foreground               color.NRGBA
}

var (
	skyReadyColor   = color.NRGBA{R: 180, G: 220, B: 187, A: 255}
	skyMissingColor = color.NRGBA{R: 225, G: 186, B: 186, A: 255}
	skyDoneColor    = color.NRGBA{R: 135, G: 140, B: 138, A: 255}
)

func (s *shell) loadSkyState(logPath string) {
	tracker := skyquest.NewTracker(s.skyDatabase)
	s.skyProgress = tracker.QuestProgress()
	s.skyInventory = tracker.Inventory()
	s.skyWatched = make(map[string]bool)
	s.skyIdentity = ""
	s.skyMessage = "Select an EverQuest logfile to load character progress."
	s.rebuildSkyRows()
	s.startSkyForLog(logPath)
}

func (s *shell) rebuildSkyRows() {
	toggleStatus := "HIDE"
	if s.skyReadyCollapsed {
		toggleStatus = "SHOW"
	}
	rows := []skyRow{{kind: "ready-section", name: fmt.Sprintf("READY TO TURN IN (%d)", s.skyReadyCount()), status: toggleStatus, toggleClick: &s.skyReadyToggle, foreground: skyReadyColor}}
	if !s.skyReadyCollapsed {
		for _, progress := range s.skyProgress {
			if progress.Ready {
				rows = append(rows, s.skyQuestRows(progress, true)...)
			}
		}
	}
	watchedStatus := "HIDE"
	if s.skyWatchClosed {
		watchedStatus = "SHOW"
	}
	rows = append(rows, skyRow{kind: "spacer"}, skyRow{kind: "watched-section", name: fmt.Sprintf("WATCHED (%d)", len(s.skyWatched)), status: watchedStatus, toggleClick: &s.skyWatchToggle, foreground: palette.accent})
	if !s.skyWatchClosed {
		for _, progress := range s.skyProgress {
			if s.skyWatched[progress.Quest.Name] {
				rows = append(rows, s.skyQuestRows(progress, true)...)
			}
		}
	}
	rows = append(rows, skyRow{kind: "spacer"}, skyRow{kind: "section", name: "ALL CLASSES", foreground: palette.accent})
	for index := 0; index < len(s.skyProgress); {
		end := index + 1
		for end < len(s.skyProgress) && s.skyProgress[end].Class == s.skyProgress[index].Class {
			end++
		}
		visible := make([]skyquest.QuestProgress, 0, end-index)
		completed, ready := 0, 0
		for _, progress := range s.skyProgress[index:end] {
			if progress.Completed {
				completed++
			}
			if progress.Ready {
				ready++
			}
			if !s.skyHideEmpty || progress.Completed || skyQuestHasOwnedNonRuneItem(progress.Quest, s.skyInventory) {
				visible = append(visible, progress)
			}
		}
		if len(visible) > 0 {
			className := s.skyProgress[index].Class
			classStatus := "HIDE"
			if s.skyClassClosed[className] {
				classStatus = "SHOW"
			}
			rows = append(rows, skyRow{kind: "class", name: className, status: fmt.Sprintf("%d/%d done · %d ready · %s", completed, end-index, ready, classStatus), toggleClick: s.skyClassToggle(className), foreground: palette.accent})
			if !s.skyClassClosed[className] {
				for _, progress := range visible {
					rows = append(rows, s.skyQuestRows(progress, false)...)
				}
			}
		}
		index = end
	}
	s.skyRows = rows
}

func (s *shell) skyClassToggle(className string) *widget.Clickable {
	if s.skyClassClosed == nil {
		s.skyClassClosed = make(map[string]bool)
	}
	if s.skyClassClicks == nil {
		s.skyClassClicks = make(map[string]*widget.Clickable)
	}
	if s.skyClassClicks[className] == nil {
		s.skyClassClicks[className] = &widget.Clickable{}
	}
	return s.skyClassClicks[className]
}

func (s *shell) skyQuestRows(progress skyquest.QuestProgress, readySummary bool) []skyRow {
	foreground := palette.text
	status := fmt.Sprintf("missing %d", len(progress.Missing))
	if progress.Completed {
		foreground, status = skyDoneColor, "DONE"
	} else if progress.Ready {
		foreground, status = skyReadyColor, "READY"
	}
	name := skyQuestDisplayName(progress.Class, progress.Quest.Name)
	if readySummary {
		name = progress.Class + " — " + name
	}
	reward := ""
	if len(progress.Quest.Rewards) > 0 {
		reward = progress.Quest.Rewards[0]
	}

	rows := []skyRow{{
		kind:        "quest",
		name:        name,
		status:      status,
		detail:      progress.Quest.QuestGiver + " — Reward: " + reward,
		reward:      reward,
		rewardClick: &widget.Clickable{},
		questName:   progress.Quest.Name,
		watched:     s.skyWatched[progress.Quest.Name],
		watchClick:  &widget.Clickable{},
		foreground:  foreground,
	}}
	for _, requirement := range progress.Quest.Requirements {
		owned := s.skyInventory[requirement.Name]
		mark, requirementColor := "–", skyMissingColor
		have, need := fmt.Sprint(owned), fmt.Sprint(requirement.Quantity)
		if progress.Completed {
			mark, requirementColor, have, need = "+", skyDoneColor, "—", "—"
		} else if owned >= requirement.Quantity {
			mark, requirementColor = "+", skyReadyColor
		}
		rows = append(rows, skyRow{kind: "requirement", name: mark + " " + requirement.Name, have: have, need: need, detail: skyRequirementSource(requirement), foreground: requirementColor})
	}
	return rows
}

func (s *shell) skyReadyCount() int {
	count := 0
	for _, progress := range s.skyProgress {
		if progress.Ready {
			count++
		}
	}
	return count
}

func (s *shell) layoutSkyWorkspace(gtx layout.Context) layout.Dimensions {
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					title := "Plane of Sky Quest Tracker"
					if s.skyIdentity != "" {
						title += "  ·  " + s.skyIdentity
					}
					return labelWeight(gtx, s.theme, title, unit.Sp(23), palette.text, text.Start, font.SemiBold)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return s.skyHideClick.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						pointer.CursorPointer.Add(gtx.Ops)
						value := "Hide quests with no items"
						if s.skyHideEmpty {
							value = "Show all quests"
						}
						return labelWeight(gtx, s.theme, value, unit.Sp(14), palette.accent, text.End, font.SemiBold)
					})
				}),
			)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return inset(0, unit.Dp(8)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return label(gtx, s.theme, s.skyMessage, unit.Sp(14), palette.muted, text.Start)
			})
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions { return s.layoutSkyRow(gtx, skyRow{}, true) }),
		layout.Rigid(separator),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			list := material.List(s.theme, &s.skyList)
			list.AnchorStrategy = material.Occupy
			list.Indicator.Color = palette.muted
			return list.Layout(gtx, len(s.skyRows), func(gtx layout.Context, index int) layout.Dimensions {
				return s.layoutSkyRow(gtx, s.skyRows[index], false)
			})
		}),
	)
}

func (s *shell) layoutSkyRow(gtx layout.Context, row skyRow, header bool) layout.Dimensions {
	height := unit.Dp(32)
	if row.kind == "section" || row.kind == "ready-section" || row.kind == "watched-section" || row.kind == "class" {
		height = 38
	}
	if row.kind == "spacer" {
		return layout.Dimensions{Size: image.Pt(gtx.Constraints.Max.X, gtx.Dp(unit.Dp(12)))}
	}
	gtx.Constraints.Min.Y = gtx.Dp(height)
	gtx.Constraints.Max.Y = gtx.Constraints.Min.Y
	if header {
		fill(gtx, palette.chrome)
	} else if row.kind == "section" || row.kind == "ready-section" || row.kind == "watched-section" || row.kind == "class" {
		fill(gtx, palette.panelAlt)
	}
	foreground := row.foreground
	if foreground.A == 0 {
		foreground = palette.text
	}
	values := []string{row.name, row.status, row.have, row.need, row.detail}
	if header {
		values = []string{"QUEST / REQUIRED ITEM", "STATUS", "HAVE", "NEED", "SOURCE / REWARD"}
		foreground = palette.muted
	}
	if row.kind == "quest" {
		values[0] = "  " + values[0]
	} else if row.kind == "requirement" {
		values[0] = "      " + values[0]
	}
	content := func(gtx layout.Context) layout.Dimensions {
		return centerContent(gtx, func(gtx layout.Context) layout.Dimensions {
			return inset(unit.Dp(10), 0).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				labelStyle := func(gtx layout.Context, value string, alignment text.Alignment) layout.Dimensions {
					style := material.Label(s.theme, unit.Sp(14)*s.theme.TextSize/16, value)
					style.Color = foreground
					style.Alignment = alignment
					style.MaxLines = 1
					style.Truncator = "…"
					if header || row.kind == "section" || row.kind == "ready-section" || row.kind == "watched-section" || row.kind == "class" {
						style.Font.Weight = font.SemiBold
					}
					return style.Layout(gtx)
				}
				cell := func(value string, weight float32, alignment text.Alignment) layout.FlexChild {
					return layout.Flexed(weight, func(gtx layout.Context) layout.Dimensions {
						return inset(unit.Dp(5), 0).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							return labelStyle(gtx, value, alignment)
						})
					})
				}
				nameCell := layout.Flexed(3.1, func(gtx layout.Context) layout.Dimensions {
					return inset(unit.Dp(5), 0).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						if row.kind != "quest" || row.watchClick == nil {
							return labelStyle(gtx, values[0], text.Start)
						}
						action := "Watch"
						if row.watched {
							action = "Unwatch"
						}
						return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
							layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
								return labelStyle(gtx, values[0], text.Start)
							}),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								return row.watchClick.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
									pointer.CursorPointer.Add(gtx.Ops)
									return inset(unit.Dp(6), 0).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
										return labelWeight(gtx, s.theme, action, unit.Sp(12), palette.accent, text.End, font.SemiBold)
									})
								})
							}),
						)
					})
				})
				if row.kind == "class" {
					return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
						nameCell,
						layout.Flexed(6.25, func(gtx layout.Context) layout.Dimensions {
							return inset(unit.Dp(5), 0).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
								return labelStyle(gtx, values[1], text.Start)
							})
						}),
					)
				}
				return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
					nameCell,
					cell(values[1], 1.25, text.End),
					cell(values[2], .8, text.End),
					cell(values[3], .8, text.End),

					layout.Flexed(3.4, func(gtx layout.Context) layout.Dimensions {
						if row.kind == "quest" && row.reward != "" {

							for row.rewardClick != nil && row.rewardClick.Clicked(gtx) {
								if err := platform.OpenURL(skyRewardURL(row.reward)); err != nil {
									s.statusText = "Could not open reward link: " + err.Error()
								}
							}

							return row.rewardClick.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
								pointer.CursorPointer.Add(gtx.Ops)

								style := material.Label(
									s.theme,
									unit.Sp(14)*s.theme.TextSize/16,
									values[4],
								)

								style.Color = palette.accent
								style.Alignment = text.Start
								style.MaxLines = 1
								style.Truncator = "…"

								return style.Layout(gtx)
							})
						}

						return inset(unit.Dp(5), 0).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							style := material.Label(
								s.theme,
								unit.Sp(14)*s.theme.TextSize/16,
								values[4],
							)

							style.Color = foreground
							style.Alignment = text.Start
							style.MaxLines = 1
							style.Truncator = "…"

							return style.Layout(gtx)
						})
					}),
				)
			})
		})
	}
	if row.toggleClick != nil {
		return row.toggleClick.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			pointer.CursorPointer.Add(gtx.Ops)
			return content(gtx)
		})
	}
	return content(gtx)
}

func skyQuestHasOwnedNonRuneItem(quest skyquest.Quest, inventory map[string]int) bool {
	for _, requirement := range quest.Requirements {
		if requirement.Kind == "rune" || strings.HasPrefix(requirement.Name, "Wind Rune") {
			continue
		}
		if inventory[requirement.Name] > 0 {
			return true
		}
	}
	return false
}

func skyQuestDisplayName(className, questName string) string {
	return strings.TrimPrefix(questName, className+" ")
}

func skyRequirementSource(requirement skyquest.Requirement) string {
	if requirement.Island > 0 && requirement.DropsFrom != "" {
		return fmt.Sprintf("Island %d — %s", requirement.Island, requirement.DropsFrom)
	}
	if requirement.Island > 0 {
		return fmt.Sprintf("Island %d", requirement.Island)
	}
	if requirement.Kind == "rune" {
		return "Plane of Sky random drop"
	}
	if requirement.DropsFrom != "" {
		return requirement.DropsFrom
	}
	return "Plane of Sky"
}

func skyRewardURL(reward string) string {
	return "https://eqlwiki.com/" + url.PathEscape(strings.ReplaceAll(reward, " ", "_"))
}
