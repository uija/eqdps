package main

import (
	"fmt"
	"image"
	"image/color"
	"net/url"
	"os/exec"
	"strings"

	"gioui.org/font"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"github.com/uija/eqdps/internal/skyquest"
)

type skyRow struct {
	kind                     string
	name, status, have, need string
	detail                   string
	reward                   string
	rewardClick              *widget.Clickable
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
	s.skyIdentity = ""
	s.skyMessage = "Select an EverQuest logfile to load character progress."
	s.rebuildSkyRows()
	s.startSkyForLog(logPath)
}

func (s *shell) rebuildSkyRows() {
	rows := []skyRow{{kind: "section", name: fmt.Sprintf("READY TO TURN IN (%d)", s.skyReadyCount()), foreground: skyReadyColor}}
	for _, progress := range s.skyProgress {
		if progress.Ready {
			rows = append(rows, s.skyQuestRows(progress, true)...)
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
			if !s.skyHideEmpty || progress.Completed || skyQuestHasOwnedItem(progress.Quest, s.skyInventory) {
				visible = append(visible, progress)
			}
		}
		if len(visible) > 0 {
			rows = append(rows, skyRow{kind: "class", name: s.skyProgress[index].Class, status: fmt.Sprintf("%d/%d done · %d ready", completed, end-index, ready), foreground: palette.accent})
			for _, progress := range visible {
				rows = append(rows, s.skyQuestRows(progress, false)...)
			}
		}
		index = end
	}
	s.skyRows = rows
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
		kind:       "quest",
		name:       name,
		status:     status,
		detail:     progress.Quest.QuestGiver + " — Reward: " + reward,
		reward:     reward,
		rewardClick: &widget.Clickable{},
		foreground: foreground,
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
	if row.kind == "section" || row.kind == "class" {
		height = 38
	}
	if row.kind == "spacer" {
		return layout.Dimensions{Size: image.Pt(gtx.Constraints.Max.X, gtx.Dp(unit.Dp(12)))}
	}
	gtx.Constraints.Min.Y = gtx.Dp(height)
	gtx.Constraints.Max.Y = gtx.Constraints.Min.Y
	if header {
		fill(gtx, palette.chrome)
	} else if row.kind == "section" || row.kind == "class" {
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
	return centerContent(gtx, func(gtx layout.Context) layout.Dimensions {
		return inset(unit.Dp(10), 0).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			cell := func(value string, weight float32, alignment text.Alignment) layout.FlexChild {
				return layout.Flexed(weight, func(gtx layout.Context) layout.Dimensions {
					return inset(unit.Dp(5), 0).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						style := material.Label(s.theme, unit.Sp(14)*s.theme.TextSize/16, value)
						style.Color = foreground
						style.Alignment = alignment
						style.MaxLines = 1
						style.Truncator = "…"
						if header || row.kind == "section" || row.kind == "class" {
							style.Font.Weight = font.SemiBold
						}
						return style.Layout(gtx)
					})
				})
			}
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
				cell(values[0], 3.1, text.Start),
				cell(values[1], 1.25, text.End),
				cell(values[2], .8, text.End),
				cell(values[3], .8, text.End),

				layout.Flexed(3.4, func(gtx layout.Context) layout.Dimensions {
					if row.kind == "quest" && row.reward != "" {

						for row.rewardClick != nil && row.rewardClick.Clicked(gtx) {
							link := "https://eqlwiki.com/" + url.PathEscape(strings.ReplaceAll(row.reward, " ", "_"))
							openURL(link)
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

func skyQuestHasOwnedItem(quest skyquest.Quest, inventory map[string]int) bool {
	for _, requirement := range quest.Requirements {
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

func openURL(url string) {
	_ = exec.Command("xdg-open", url).Start()
}