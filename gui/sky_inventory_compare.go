package main

import (
	"fmt"
	"image"
	"image/color"
	"sort"
	"strings"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/op/paint"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"github.com/uija/eqdps/internal/inventorysync"
	"github.com/uija/eqdps/internal/skyquest"
)

type skyInventoryDifference struct {
	Item     string
	Tracked  int
	Exported int
	Selected widget.Bool
}

type skyInventoryCompareValue struct {
	Item     string
	Tracked  int
	Exported int
}

type skyInventoryCompareResult struct {
	id          uint64
	logPath     string
	differences []skyInventoryCompareValue
	err         error
}

func (s *shell) compareSkyInventoryExport(request inventorysync.Request) {
	if !s.skyCompareEnabled.Value {
		return
	}
	s.skyMu.RLock()
	tracker := s.skyTracker
	s.skyMu.RUnlock()
	if tracker == nil {
		s.statusText = "Inventory comparison skipped because Plane of Sky tracking is not active"
		return
	}
	tracked := tracker.Inventory()
	logPath := s.currentLog
	s.skyCompareID++
	id := s.skyCompareID
	go func() {
		exported, err := skyquest.ReadInventoryExport(request.Path, s.skyDatabase)
		result := skyInventoryCompareResult{id: id, logPath: logPath, err: err}
		if err == nil {
			result.differences = skyInventoryDifferences(tracked, exported)
		}
		select {
		case s.skyCompareResults <- result:
		default:
			select {
			case <-s.skyCompareResults:
			default:
			}
			s.skyCompareResults <- result
		}
		if s.window != nil {
			s.window.Invalidate()
		}
	}()
}

func skyInventoryDifferences(tracked, exported map[string]int) []skyInventoryCompareValue {
	items := make(map[string]struct{}, len(tracked)+len(exported))
	for item := range tracked {
		if !strings.HasPrefix(item, "Wind Rune ") {
			items[item] = struct{}{}
		}
	}
	for item := range exported {
		if !strings.HasPrefix(item, "Wind Rune ") {
			items[item] = struct{}{}
		}
	}
	names := make([]string, 0, len(items))
	for item := range items {
		names = append(names, item)
	}
	sort.Strings(names)
	differences := make([]skyInventoryCompareValue, 0, len(names))
	for _, item := range names {
		if tracked[item] != exported[item] {
			differences = append(differences, skyInventoryCompareValue{Item: item, Tracked: tracked[item], Exported: exported[item]})
		}
	}
	return differences
}

func (s *shell) updateSkyInventoryComparison(gtx layout.Context) {
	select {
	case result := <-s.skyCompareResults:
		if result.id == s.skyCompareID && result.logPath == s.currentLog {
			if result.err != nil {
				s.statusText = "Inventory comparison: " + result.err.Error()
			} else if len(result.differences) == 0 {
				s.statusText = "Inventory export matches Plane of Sky progress"
			} else {
				s.skyCompareRows = make([]skyInventoryDifference, len(result.differences))
				for index, difference := range result.differences {
					s.skyCompareRows[index] = skyInventoryDifference{Item: difference.Item, Tracked: difference.Tracked, Exported: difference.Exported}
					s.skyCompareRows[index].Selected.Value = true
				}
				s.skyCompareList.ScrollTo(0)
				s.skyComparePath = result.logPath
				s.skyCompareOpen = true
			}
		}
	default:
	}
	if s.skyCompareCancel.Clicked(gtx) {
		s.skyCompareOpen = false
		s.skyCompareRows = nil
		s.skyComparePath = ""
	}
	if s.skyCompareApply.Clicked(gtx) {
		s.applySkyInventoryComparison()
	}
}

func (s *shell) applySkyInventoryComparison() {
	quantities := make(map[string]int)
	for index := range s.skyCompareRows {
		row := &s.skyCompareRows[index]
		if row.Selected.Value {
			quantities[row.Item] = row.Exported
		}
	}
	if len(quantities) == 0 {
		s.skyCompareOpen = false
		s.skyCompareRows = nil
		s.skyComparePath = ""
		return
	}
	if s.skyComparePath != s.currentLog {
		s.skyCompareOpen = false
		s.skyCompareRows = nil
		s.skyComparePath = ""
		return
	}
	s.skyMu.RLock()
	tracker := s.skyTracker
	s.skyMu.RUnlock()
	if tracker == nil {
		s.statusText = "Inventory comparison could not update inactive Plane of Sky tracking"
		return
	}
	if err := tracker.SetInventoryQuantities(quantities); err != nil {
		s.statusText = "Inventory comparison: " + err.Error()
		return
	}
	s.applySkySnapshot(tracker, fmt.Sprintf("Updated %d Plane of Sky item quantities from inventory export.", len(quantities)))
	s.skyCompareOpen = false
	s.skyCompareRows = nil
	s.skyComparePath = ""
}

func (s *shell) layoutSkyInventoryComparison(gtx layout.Context) layout.Dimensions {
	if !s.skyCompareOpen {
		return layout.Dimensions{}
	}
	paint.Fill(gtx.Ops, color.NRGBA{A: 175})
	return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		width := min(gtx.Constraints.Max.X, gtx.Dp(unit.Dp(720)))
		height := gtx.Dp(unit.Dp(190 + min(len(s.skyCompareRows), 7)*42))
		height = min(gtx.Constraints.Max.Y, height)
		gtx.Constraints.Min = image.Pt(width, height)
		gtx.Constraints.Max = gtx.Constraints.Min
		return outline(gtx, palette.line, func(gtx layout.Context) layout.Dimensions {
			fill(gtx, palette.panel)
			return layout.UniformInset(unit.Dp(22)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return labelWeight(gtx, s.theme, "Plane of Sky inventory differences", unit.Sp(21), palette.text, text.Start, font.SemiBold)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return inset(0, unit.Dp(10)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							return label(gtx, s.theme, "Choose which saved quantities should be replaced by the inventory export.", unit.Sp(14), palette.muted, text.Start)
						})
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return s.layoutSkyInventoryDifferenceRow(gtx, -1)
					}),
					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
						list := material.List(s.theme, &s.skyCompareList)
						list.AnchorStrategy = material.Occupy
						return list.Layout(gtx, len(s.skyCompareRows), func(gtx layout.Context, index int) layout.Dimensions {
							return s.layoutSkyInventoryDifferenceRow(gtx, index)
						})
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return layout.E.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									return betaCleanupButton(gtx, s.theme, &s.skyCompareCancel, "Cancel", false)
								}),
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									return betaCleanupButton(gtx, s.theme, &s.skyCompareApply, "Apply selected", true)
								}),
							)
						})
					}),
				)
			})
		})
	})
}

func (s *shell) layoutSkyInventoryDifferenceRow(gtx layout.Context, index int) layout.Dimensions {
	header := index < 0
	gtx.Constraints.Min.Y = gtx.Dp(unit.Dp(38))
	gtx.Constraints.Max.Y = gtx.Constraints.Min.Y
	if header {
		fill(gtx, palette.chrome)
	}
	item, tracked, exported := "ITEM", "SAVED", "EXPORT"
	var selected *widget.Bool
	if !header {
		row := &s.skyCompareRows[index]
		item, tracked, exported = row.Item, fmt.Sprint(row.Tracked), fmt.Sprint(row.Exported)
		selected = &row.Selected
	}
	return centerContent(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				if header {
					return layout.Dimensions{Size: image.Pt(gtx.Dp(unit.Dp(40)), 0)}
				}
				gtx.Constraints.Min.X = gtx.Dp(unit.Dp(40))
				return material.CheckBox(s.theme, selected, "").Layout(gtx)
			}),
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				return labelWeight(gtx, s.theme, item, unit.Sp(15), palette.text, text.Start, font.SemiBold)
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				gtx.Constraints.Min.X = gtx.Dp(unit.Dp(90))
				return label(gtx, s.theme, tracked, unit.Sp(15), palette.muted, text.End)
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				gtx.Constraints.Min.X = gtx.Dp(unit.Dp(90))
				return labelWeight(gtx, s.theme, exported, unit.Sp(15), palette.accent, text.End, font.SemiBold)
			}),
		)
	})
}
