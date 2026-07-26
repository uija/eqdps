package main

import (
	"fmt"
	"image"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"gioui.org/app"
	"gioui.org/font"
	"gioui.org/io/pointer"
	"gioui.org/io/system"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
)

type combatOverlay struct {
	window           *app.Window
	theme            *material.Theme
	updates          chan overlayUpdate
	closed           chan<- *combatOverlay
	owner            *app.Window
	list             widget.List
	decorations      widget.Decorations
	fights           []fakeFightSection
	idleTimeout      time.Duration
	completedAt      time.Time
	lastWidth        int
	lastHeight       int
	lastX            int
	lastY            int
	nativeMu         sync.Mutex
	nativeHandle     uintptr
	nativeOpacity    float32
	positionKnown    bool
	positionRestored bool
	savedX           int
	savedY           int
	hasSavedPosition bool
	focusOrder       uint64
	focusCandidate   uint64
	focusCandidateAt time.Time
}

const overlayFocusDelay = 200 * time.Millisecond

type overlayUpdate struct {
	fights      []fakeFightSection
	fontScale   float32
	idleTimeout time.Duration
}

func (s *shell) openOverlay() {
	s.overlayMu.Lock()
	if s.overlay != nil {
		window := s.overlay.window
		s.overlayMu.Unlock()
		window.Perform(system.ActionRaise)
		return
	}
	window := new(app.Window)
	window.Option(
		app.Title("eqdps — Current Fight"),
		app.Size(unit.Dp(s.settings.OverlayWidth), unit.Dp(s.settings.OverlayHeight)),
		app.MinSize(unit.Dp(380), unit.Dp(180)),
		app.Decorated(false),
		app.TopMost(true),
	)
	// Gio text shapers maintain mutable caches and must not be shared by
	// independently rendered top-level windows.
	theme := material.NewTheme()
	theme.Palette.Fg = palette.text
	theme.Palette.Bg = palette.window
	overlay := &combatOverlay{
		window:           window,
		theme:            theme,
		updates:          make(chan overlayUpdate, 1),
		closed:           s.overlayClosed,
		owner:            s.window,
		list:             widget.List{List: layout.List{Axis: layout.Vertical}},
		savedX:           s.settings.OverlayX,
		savedY:           s.settings.OverlayY,
		nativeOpacity:    s.settings.DPSOpacity,
		hasSavedPosition: s.settings.OverlayPlaced,
	}
	s.overlay = overlay
	s.overlayMu.Unlock()
	s.pushOverlay(s.fights)
	go func() {
		if err := overlay.run(); err != nil {
			log.Printf("DPS overlay: %v", err)
		}
	}()
}

func (s *shell) applyOverlayOpacity(opacity float32) {
	s.overlayMu.RLock()
	overlay := s.overlay
	s.overlayMu.RUnlock()
	if overlay != nil {
		setNativeOverlayOpacity(overlay, opacity)
	}
}

func (s *shell) toggleOverlay() {
	s.overlayMu.RLock()
	overlay := s.overlay
	s.overlayMu.RUnlock()
	if overlay != nil {
		overlay.window.Perform(system.ActionClose)
		s.setOverlayVisible(false)
		return
	}
	if s.showWaylandHelpOnce() {
		s.openAfterHelp = true
		return
	}
	s.openOverlay()
	s.setOverlayVisible(true)
}

func isWaylandSession() bool {
	return strings.EqualFold(os.Getenv("XDG_SESSION_TYPE"), "wayland") || os.Getenv("WAYLAND_DISPLAY") != ""
}

func (s *shell) showWaylandHelpOnce() bool {
	if !isWaylandSession() || s.settings.WaylandNotice {
		return false
	}
	s.waylandHelp = true
	s.rememberHelp = true
	return true
}

func (s *shell) showWaylandHelp() {
	s.waylandHelp = true
}

func (s *shell) setOverlayVisible(visible bool) {
	s.settings.OverlayVisible = visible
	if visible {
		s.menus[2].items[3].name = "Hide DPS overlay"
	} else {
		s.menus[2].items[3].name = "Show DPS overlay"
	}
	if err := saveSettings(s.settings); err != nil {
		s.statusText = "Overlay preference could not be saved"
	}
}

func (s *shell) pushOverlay(fights []fakeFightSection) {
	s.overlayMu.RLock()
	defer s.overlayMu.RUnlock()
	overlay := s.overlay
	if overlay == nil {
		return
	}
	fontScale := float32(s.dpsFontMilli.Load()) / 1000
	select {
	case overlay.updates <- overlayUpdate{fights: fights, fontScale: fontScale, idleTimeout: time.Duration(s.combatIdleNanos.Load())}:
	default:
		select {
		case <-overlay.updates:
		default:
		}
		overlay.updates <- overlayUpdate{fights: fights, fontScale: fontScale, idleTimeout: time.Duration(s.combatIdleNanos.Load())}
	}
	if overlay.window != nil {
		overlay.window.Invalidate()
	}
}

func (o *combatOverlay) run() error {
	var ops op.Ops
	defer func() {
		o.closed <- o
		o.owner.Invalidate()
	}()
	for {
		event := o.window.Event()
		handleNativeOverlayEvent(o, event)
		switch event := event.(type) {
		case app.DestroyEvent:
			captureNativeOverlayPosition(o)
			return event.Err
		case app.FrameEvent:
			gtx := app.NewContext(&ops, event)
			o.lastWidth = int(gtx.Metric.PxToDp(gtx.Constraints.Max.X))
			o.lastHeight = int(gtx.Metric.PxToDp(gtx.Constraints.Max.Y))
			o.update()
			o.layout(gtx)
			event.Frame(gtx.Ops)
		}
	}
}

func (o *combatOverlay) update() {
	for {
		select {
		case update := <-o.updates:
			hadCurrent := hasCurrentFight(o.fights)
			o.fights = update.fights
			o.theme.TextSize = unit.Sp(16 * update.fontScale)
			o.idleTimeout = update.idleTimeout
			o.observeFocus(time.Now())
			hasCurrent := hasCurrentFight(o.fights)
			switch {
			case hasCurrent:
				o.completedAt = time.Time{}
			case len(o.fights) > 0 && (hadCurrent || o.completedAt.IsZero()):
				o.completedAt = time.Now()
				// An idle-ended fight has already spent the configured timeout
				// without activity; do not retain it for a second timeout.
				if o.fights[0].status == "idle timeout" {
					o.completedAt = o.completedAt.Add(-o.idleTimeout)
				}
			}
		default:
			return
		}
	}
}

func (o *combatOverlay) displayFight() *fakeFightSection {
	return o.displayFightAt(time.Now())
}

func (o *combatOverlay) displayFightAt(now time.Time) *fakeFightSection {
	var newest, latestIntentional *fakeFightSection
	for index := range o.fights {
		fight := &o.fights[index]
		if fight.lastYouIntentionalOrder > 0 && (latestIntentional == nil || fight.lastYouIntentionalOrder > latestIntentional.lastYouIntentionalOrder) {
			latestIntentional = fight
		}
		if !fight.current {
			continue
		}
		if newest == nil || fight.started.After(newest.started) {
			newest = fight
		}
	}
	focused := latestIntentional
	if o.focusOrder > 0 {
		for index := range o.fights {
			if o.fights[index].lastYouIntentionalOrder == o.focusOrder {
				focused = &o.fights[index]
				break
			}
		}
	}
	if focused != nil {
		if newest == nil && o.idleTimeout > 0 && !o.completedAt.IsZero() && !now.Before(o.completedAt.Add(o.idleTimeout)) {
			return nil
		}
		return focused
	}
	if newest != nil {
		return newest
	}
	if o.idleTimeout > 0 && !o.completedAt.IsZero() && !now.Before(o.completedAt.Add(o.idleTimeout)) {
		return nil
	}
	// DisplaySections orders completed history newest first. Keeping its first
	// entry visible avoids blanking the meter between fights.
	if len(o.fights) > 0 {
		return &o.fights[0]
	}
	return nil
}

func (o *combatOverlay) observeFocus(now time.Time) {
	latest := latestIntentionalOrder(o.fights)
	switch {
	case latest == 0:
		return
	case o.focusOrder == 0:
		o.focusOrder = latest
		o.focusCandidate = 0
	case latest == o.focusOrder:
		o.focusCandidate = 0
	case latest != o.focusCandidate:
		o.focusCandidate = latest
		o.focusCandidateAt = now.Add(overlayFocusDelay)
	}
}

func (o *combatOverlay) commitFocus(now time.Time) {
	if o.focusCandidate == 0 || now.Before(o.focusCandidateAt) {
		return
	}
	if latestIntentionalOrder(o.fights) == o.focusCandidate {
		o.focusOrder = o.focusCandidate
	}
	o.focusCandidate = 0
}

func latestIntentionalOrder(fights []fakeFightSection) uint64 {
	var latest uint64
	for _, fight := range fights {
		latest = max(latest, fight.lastYouIntentionalOrder)
	}
	return latest
}

func hasCurrentFight(fights []fakeFightSection) bool {
	for _, fight := range fights {
		if fight.current {
			return true
		}
	}
	return false
}

func (o *combatOverlay) layout(gtx layout.Context) layout.Dimensions {
	fill(gtx, palette.window)
	now := time.Now()
	o.commitFocus(now)
	if o.focusCandidate > 0 {
		gtx.Execute(op.InvalidateCmd{At: o.focusCandidateAt})
	}
	fight := o.displayFightAt(now)
	if fight != nil && !hasCurrentFight(o.fights) && o.idleTimeout > 0 && !o.completedAt.IsZero() {
		gtx.Execute(op.InvalidateCmd{At: o.completedAt.Add(o.idleTimeout)})
	}
	return layout.Stack{}.Layout(gtx,
		layout.Expanded(func(gtx layout.Context) layout.Dimensions {
			if fight == nil {
				return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return label(gtx, o.theme, "Waiting for combat…", unit.Sp(18), palette.muted, text.Middle)
				})
			}
			return o.layoutFight(gtx, fight)
		}),
		layout.Stacked(func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min = gtx.Constraints.Max
			return layout.NE.Layout(gtx, o.layoutDragHandle)
		}),
	)
}

func (o *combatOverlay) layoutFight(gtx layout.Context, fight *fakeFightSection) layout.Dimensions {
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min.Y = gtx.Dp(unit.Dp(42))
			gtx.Constraints.Max.Y = gtx.Constraints.Min.Y
			fill(gtx, palette.panelAlt)
			return centerContent(gtx, func(gtx layout.Context) layout.Dimensions {
				return inset(unit.Dp(12), 0).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
						layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
							return labelWeight(gtx, o.theme, fight.name, unit.Sp(18), palette.text, text.Start, font.SemiBold)
						}),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return (layout.Inset{Right: unit.Dp(40)}).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
								return label(gtx, o.theme, fight.duration, unit.Sp(16), palette.accent, text.End)
							})
						}),
					)
				})
			})
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions { return o.layoutRow(gtx, fakeCombatant{}, true) }),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			list := material.List(o.theme, &o.list)
			list.AnchorStrategy = material.Occupy
			list.Indicator.Color = palette.muted
			return list.Layout(gtx, len(fight.combatants), func(gtx layout.Context, index int) layout.Dimensions {
				return o.layoutRow(gtx, fight.combatants[index], false)
			})
		}),
	)
}

func (o *combatOverlay) layoutDragHandle(gtx layout.Context) layout.Dimensions {
	return inset(unit.Dp(7), unit.Dp(4)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return o.decorations.LayoutMove(gtx, func(gtx layout.Context) layout.Dimensions {
			pointer.CursorPointer.Add(gtx.Ops)
			size := gtx.Dp(unit.Dp(28))
			lineWidth := gtx.Dp(unit.Dp(16))
			lineHeight := gtx.Dp(unit.Dp(2))
			left := (size - lineWidth) / 2
			for _, top := range []int{7, 13, 19} {
				y := gtx.Dp(unit.Dp(top))
				paint.FillShape(gtx.Ops, palette.text, clip.Rect{
					Min: image.Pt(left, y),
					Max: image.Pt(left+lineWidth, y+lineHeight),
				}.Op())
			}
			return layout.Dimensions{Size: image.Pt(size, size)}
		})
	})
}

func (o *combatOverlay) layoutRow(gtx layout.Context, row fakeCombatant, header bool) layout.Dimensions {
	gtx.Constraints.Min.Y = gtx.Dp(unit.Dp(34))
	gtx.Constraints.Max.Y = gtx.Constraints.Min.Y
	if header {
		fill(gtx, palette.chrome)
	}
	values := []string{row.name, fmt.Sprint(row.damage), fmt.Sprint(row.dps), row.active}
	if header {
		values = []string{"COMBATANT", "DAMAGE", "DPS", "ACTIVE"}
	}
	return centerContent(gtx, func(gtx layout.Context) layout.Dimensions {
		return inset(unit.Dp(12), 0).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			cell := func(value string, weight float32, alignment text.Alignment) layout.FlexChild {
				return layout.Flexed(weight, func(gtx layout.Context) layout.Dimensions {
					return label(gtx, o.theme, value, unit.Sp(16), palette.text, alignment)
				})
			}
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
				cell(values[0], 3, text.Start), cell(values[1], 1.4, text.End), cell(values[2], 1, text.End), cell(values[3], 1.2, text.End),
			)
		})
	})
}
