package view

import (
	"image"
	"time"

	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"github.com/uija/eqdps/internal/eqlog"
	"github.com/uija/eqdps/internal/ui"
)

type LoadHistoryCallback func(eqlog.Loopback)
type historyEntry struct {
	caption   string
	clickable widget.Clickable
	loopback  eqlog.Loopback
}

type historySelection struct {
	visible  bool
	style    *ui.Style
	callback LoadHistoryCallback
	entries  []historyEntry
	close    widget.Clickable
}

func NewhistorySelection(style *ui.Style, callback LoadHistoryCallback) *historySelection {
	result := &historySelection{
		visible:  false,
		style:    style,
		callback: callback,
		entries:  make([]historyEntry, 0),
	}

	result.entries = append(result.entries, historyEntry{caption: "live", loopback: eqlog.Loopback{Skip: true}})
	result.entries = append(result.entries, historyEntry{caption: "last Hour", loopback: eqlog.Loopback{TimeOffset: time.Hour}})
	result.entries = append(result.entries, historyEntry{caption: "last 4 Hours", loopback: eqlog.Loopback{TimeOffset: 4 * time.Hour}})
	result.entries = append(result.entries, historyEntry{caption: "last 8 Hours", loopback: eqlog.Loopback{TimeOffset: 8 * time.Hour}})
	result.entries = append(result.entries, historyEntry{caption: "last Day", loopback: eqlog.Loopback{TimeOffset: 24 * time.Hour}})
	result.entries = append(result.entries, historyEntry{caption: "All", loopback: eqlog.Loopback{}})

	return result
}
func (h *historySelection) ResetOffsetEntries() {
	list := make([]historyEntry, 0)
	for _, e := range h.entries {
		if e.loopback.ByteOffset > 0 {
			list = append(list, e)
		}
	}
	h.entries = list
}
func (h *historySelection) AddOffsetEntry(caption string, offset int64) {
	h.entries = append(h.entries, historyEntry{caption: caption, loopback: eqlog.Loopback{ByteOffset: offset}})
}
func (h *historySelection) Update(gtx layout.Context) {
	if h.close.Clicked(gtx) {
		h.visible = false
	}
	for idx, e := range h.entries {
		if h.entries[idx].clickable.Clicked(gtx) {
			h.callback(e.loopback)
			h.visible = false
		}
	}
}
func (h *historySelection) Show() {
	h.visible = true
}
func (h *historySelection) Layout(gtx layout.Context) layout.Dimensions {
	if !h.visible {
		return layout.Dimensions{}
	}
	ui.Fill(gtx, h.style.Palette.Shadow)

	return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		width := min(gtx.Dp(unit.Dp(420)), gtx.Constraints.Max.X)
		height := min(gtx.Dp(unit.Dp(320)), gtx.Constraints.Max.Y)
		gtx.Constraints = layout.Exact(image.Pt(width, height))

		ui.Fill(gtx, h.style.Palette.Panel)

		return layout.Flex{
			Axis: layout.Vertical,
		}.Layout(gtx,
			layout.Rigid(ui.TitleBar(gtx, *h.style, "Select Combat History Timeframe")),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Top: unit.Dp(20)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					items := make([]layout.FlexChild, 0, len(h.entries))
					for index, item := range h.entries {
						items = append(items, layout.Rigid(h.layoutLink(index, item)))
					}
					return layout.Flex{Axis: layout.Vertical}.Layout(gtx, items...)
				})
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Top: unit.Dp(20), Left: unit.Dp(16), Right: unit.Dp(16)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return h.close.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return material.Button(h.style.Theme, &h.close, "Close").Layout(gtx)
					})
				})
			}),
		)
	})
}
func (h *historySelection) layoutLink(idx int, item historyEntry) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		return h.entries[idx].clickable.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Top: unit.Dp(6), Bottom: unit.Dp(6), Left: unit.Dp(16)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				label := material.Label(h.style.Theme, ui.Sp(15), "Load: "+item.caption)
				label.Color = h.style.Palette.Text
				return label.Layout(gtx)
			})
		})
	}
}
