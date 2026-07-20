package main

import (
	"fmt"
	"log"
	"os"
	"strings"

	"gioui.org/app"
	"gioui.org/font"
	"gioui.org/io/system"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
)

type combatOverlay struct {
	window  *app.Window
	theme   *material.Theme
	updates chan []fakeFightSection
	closed  chan<- *combatOverlay
	owner   *app.Window
	list    widget.List
	fights  []fakeFightSection
}

func (s *shell) openOverlay() {
	if s.overlay != nil {
		s.overlay.window.Perform(system.ActionRaise)
		return
	}
	window := new(app.Window)
	window.Option(
		app.Title("eqdps — Current Fight"),
		app.Size(unit.Dp(520), unit.Dp(310)),
		app.MinSize(unit.Dp(380), unit.Dp(180)),
		app.TopMost(true),
	)
	// Gio text shapers maintain mutable caches and must not be shared by
	// independently rendered top-level windows.
	theme := material.NewTheme()
	theme.Palette.Fg = palette.text
	theme.Palette.Bg = palette.window
	overlay := &combatOverlay{
		window:  window,
		theme:   theme,
		updates: make(chan []fakeFightSection, 1),
		closed:  s.overlayClosed,
		owner:   s.window,
		list:    widget.List{List: layout.List{Axis: layout.Vertical}},
	}
	s.overlay = overlay
	s.pushOverlay(s.fights)
	go func() {
		if err := overlay.run(); err != nil {
			log.Printf("DPS overlay: %v", err)
		}
	}()
}

func (s *shell) toggleOverlay() {
	if s.overlay != nil {
		s.overlay.window.Perform(system.ActionClose)
		s.setOverlayVisible(false)
		return
	}
	s.openOverlay()
	s.setOverlayVisible(true)
	s.showWaylandHelpOnce()
}

func isWaylandSession() bool {
	return strings.EqualFold(os.Getenv("XDG_SESSION_TYPE"), "wayland") || os.Getenv("WAYLAND_DISPLAY") != ""
}

func (s *shell) showWaylandHelpOnce() {
	if !isWaylandSession() || s.settings.WaylandNotice {
		return
	}
	s.settings.WaylandNotice = true
	s.waylandHelp = true
	if err := saveSettings(s.settings); err != nil {
		s.statusText = "Wayland help preference could not be saved"
	}
}

func (s *shell) setOverlayVisible(visible bool) {
	s.settings.OverlayVisible = visible
	if visible {
		s.menus[2].items[2].name = "Hide DPS overlay"
	} else {
		s.menus[2].items[2].name = "Show DPS overlay"
	}
	if err := saveSettings(s.settings); err != nil {
		s.statusText = "Overlay preference could not be saved"
	}
}

func (s *shell) pushOverlay(fights []fakeFightSection) {
	if s.overlay == nil {
		return
	}
	select {
	case s.overlay.updates <- fights:
	default:
		select {
		case <-s.overlay.updates:
		default:
		}
		s.overlay.updates <- fights
	}
	s.overlay.window.Invalidate()
}

func (o *combatOverlay) run() error {
	var ops op.Ops
	defer func() {
		o.closed <- o
		o.owner.Invalidate()
	}()
	for {
		switch event := o.window.Event().(type) {
		case app.DestroyEvent:
			return event.Err
		case app.FrameEvent:
			gtx := app.NewContext(&ops, event)
			o.update()
			o.layout(gtx)
			event.Frame(gtx.Ops)
		}
	}
}

func (o *combatOverlay) update() {
	for {
		select {
		case fights := <-o.updates:
			o.fights = fights
		default:
			return
		}
	}
}

func (o *combatOverlay) displayFight() *fakeFightSection {
	var newest *fakeFightSection
	for index := range o.fights {
		fight := &o.fights[index]
		if fight.current && (newest == nil || fight.started.After(newest.started)) {
			newest = fight
		}
	}
	if newest != nil {
		return newest
	}
	// DisplaySections orders completed history newest first. Keeping its first
	// entry visible avoids blanking the meter between fights.
	if len(o.fights) > 0 {
		return &o.fights[0]
	}
	return nil
}

func (o *combatOverlay) layout(gtx layout.Context) layout.Dimensions {
	fill(gtx, palette.window)
	fight := o.displayFight()
	if fight == nil {
		return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return label(gtx, o.theme, "Waiting for combat…", unit.Sp(18), palette.muted, text.Middle)
		})
	}
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
							return label(gtx, o.theme, fight.duration, unit.Sp(16), palette.accent, text.End)
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
