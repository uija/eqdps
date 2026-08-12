package xphour

import (
	"fmt"
	"image"
	"strconv"
	"time"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"github.com/uija/eqdps/internal/data"
	"github.com/uija/eqdps/internal/eqlog"
	"github.com/uija/eqdps/internal/module"
	"github.com/uija/eqdps/internal/ui"
)

type Module struct {
	ctx *module.Context

	lastCombat    time.Time
	latestLog     time.Time
	activeBetween time.Duration
	xpReceived    float64

	overlay bool

	statusbar_click widget.Clickable
	overlay_level   widget.Clickable
	overlay_zone    widget.Clickable
	overlay_cancel  widget.Clickable
}

func (m *Module) Init(ctx *module.Context) error {
	ctx.RegisterLogOpen(m.OnLogOpen)
	ctx.RegisterLogRow(m.OnLogRow)
	ctx.RegisterStatusWidget(m.Layout)
	ctx.RegisterOverlayWidget(m.LayoutOverlay)
	ctx.RegisterUpdate(m.Update)
	m.ctx = ctx
	return nil
}
func (m *Module) Update(gtx layout.Context) {
	if m.statusbar_click.Clicked(gtx) {
		m.overlay = !m.overlay
	}
	if m.overlay_cancel.Clicked(gtx) {
		m.overlay = false
	}
	if m.overlay_level.Clicked(gtx) {
		time := m.ctx.GetLastLevelOffset()
		if !time.IsZero() {
			m.ctx.RequestReplay(eqlog.Loopback{Timestamp: time})
		}
		m.overlay = false
	}
	if m.overlay_zone.Clicked(gtx) {
		time := m.ctx.GetLastZoningOffset()
		if !time.IsZero() {
			m.ctx.RequestReplay(eqlog.Loopback{Timestamp: time})
		}
		m.overlay = false
	}
}
func (m *Module) LayoutOverlay(style *ui.Style, gtx layout.Context) layout.Dimensions {
	ui.FillOverlay(gtx, style.Palette.Panel, style.Palette.Border)
	return layout.UniformInset(unit.Dp(8)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return m.overlay_level.Layout(gtx, material.Body1(style.Theme, "Since last levelup").Layout)
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return m.overlay_zone.Layout(gtx, material.Body1(style.Theme, "Since last zoning").Layout)
			}),
		)
	})
}
func (m *Module) Layout(style *ui.Style, gtx layout.Context) layout.Dimensions {
	var dims layout.Dimensions
	if m.latestLog.IsZero() || m.xpReceived == 0 {
		dims = m.statusbar_click.Layout(gtx,
			func(gtx layout.Context) layout.Dimensions {
				return ui.IconLabel(gtx, style.Theme, 13, ui.Timer, "Waiting for data...")
			},
		)
	} else {
		active := m.activeBetween

		if !m.lastCombat.IsZero() && m.latestLog.After(m.lastCombat) {
			gap := m.latestLog.Sub(m.lastCombat)
			active += min(gap, time.Minute)
		}

		xpPerHour := 0.0
		if active > 0 {
			xpPerHour = m.xpReceived / active.Hours()
		}
		dims = m.statusbar_click.Layout(gtx,
			func(gtx layout.Context) layout.Dimensions {
				return ui.IconLabel(gtx, style.Theme, 14, ui.Timer, fmt.Sprintf("%.02f xp/h", xpPerHour))
			},
		)
	}
	if m.overlay {
		width := gtx.Dp(unit.Dp(220))
		height := gtx.Dp(unit.Dp(60))

		recording := op.Record(gtx.Ops)

		offset := op.Offset(image.Pt(
			0,
			-height-8,
		)).Push(gtx.Ops)

		popupGtx := gtx
		popupGtx.Constraints = layout.Exact(image.Pt(width, height))
		m.overlay_cancel.Layout(gtx, func(gtx layout.Context) layout.Dimensions { return m.LayoutOverlay(style, popupGtx) })

		offset.Pop()

		op.Defer(gtx.Ops, recording.Stop())
	}

	return dims
}

func (m *Module) OnLogOpen(characterName string, serverName string, filesize int64, path string) {
	m.latestLog = time.Time{}
	m.lastCombat = time.Time{}
	m.activeBetween = 0
	m.xpReceived = 0
}
func (m *Module) OnLogRow(e *data.LogRowEvent) {
	m.latestLog = e.Timestamp

	switch e.Type {
	case data.LogRowEventTypeDamage,
		data.LogRowEventTypeDamageOverTime,
		data.LogRowEventTypeDamageShield,
		data.LogRowEventTypeYourDamageOverTime,
		data.LogRowEventTypeYourDamageShield:

		if !m.lastCombat.IsZero() && e.Timestamp.After(m.lastCombat) {
			gap := e.Timestamp.Sub(m.lastCombat)
			if gap > time.Minute {
				gap = time.Minute
			}
			m.activeBetween += gap
		}
		m.lastCombat = e.Timestamp

	case data.LogRowEventTypeExperience:
		if xp, err := strconv.ParseFloat(e.Data[1], 64); err == nil {
			m.xpReceived += xp
		}
	}
}

func (m *Module) Shutdown() {

}
