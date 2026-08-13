package dps

import (
	"fmt"
	"sort"
	"sync"

	"gioui.org/app"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"github.com/uija/eqdps/internal/ui"
)

type Overlay struct {
	updates chan *Fight
	style   *ui.Style
	window  *app.Window
	list    widget.List

	fightMu sync.RWMutex
	fight   *Fight
	done    chan struct{}
}

func newOverlay(source *ui.Style) *Overlay {

	overlayStyle := *source
	overlayStyle.Theme = material.NewTheme()
	overlayStyle.Theme.Palette = source.Theme.Palette
	overlayStyle.Theme.TextSize = source.Theme.TextSize
	overlayStyle.Theme.Face = source.Theme.Face
	overlayStyle.Theme.FingerSize = source.Theme.FingerSize

	window := new(app.Window)
	window.Option(
		app.Title("eqdps — Current Fight"),
		app.Size(unit.Dp(360), unit.Dp(200)),
		app.MinSize(unit.Dp(360), unit.Dp(200)),
		app.Decorated(false),
		app.TopMost(true),
	)
	o := &Overlay{
		window:  window,
		updates: make(chan *Fight, 1),
		style:   &overlayStyle,
		done:    make(chan struct{}),
	}
	o.list.Axis = layout.Vertical
	return o
}

func (o *Overlay) run(onEnd func()) {
	go o.handleUpdates()

	var ops op.Ops
	for {
		switch event := o.window.Event().(type) {
		case app.DestroyEvent:
			close(o.done)
			onEnd()
			return
		case app.FrameEvent:
			gtx := app.NewContext(&ops, event)
			o.Layout(gtx)
			event.Frame(gtx.Ops)
		}
	}
}

func (o *Overlay) handleUpdates() {
	for {
		select {
		case fight := <-o.updates:
			o.fightMu.Lock()
			o.fight = fight
			o.fightMu.Unlock()
			o.window.Invalidate()
		case <-o.done:
			return
		}
	}
}

func (o *Overlay) Layout(gtx layout.Context) layout.Dimensions {
	o.fightMu.RLock()
	fight := o.fight
	o.fightMu.RUnlock()

	ui.Fill(gtx, o.style.Palette.Window)
	children := make([]layout.FlexChild, 0)
	// TODO: Timer

	children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
		return ui.ColoredRow(gtx, o.style.Palette.Panel, func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Top: unit.Dp(4), Bottom: unit.Dp(4), Left: unit.Dp(12)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				gtx.Constraints.Min.X = gtx.Constraints.Max.X
				if fight == nil {
					return material.Label(o.style.Theme, unit.Sp(13), "Waiting for data").Layout(gtx)
				} else {
					return material.Label(o.style.Theme, unit.Sp(13), fight.name).Layout(gtx)
				}
			})
		})
	}))
	children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
		return ui.ColoredRow(gtx, o.style.Palette.Panel, func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Top: unit.Dp(4), Bottom: unit.Dp(4), Left: unit.Dp(12)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
					layout.Flexed(3, func(gtx layout.Context) layout.Dimensions {
						return material.Label(o.style.Theme, unit.Sp(13), "Combatant").Layout(gtx)
					}),
					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
						return material.Label(o.style.Theme, unit.Sp(13), "Damage").Layout(gtx)
					}),
					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
						return material.Label(o.style.Theme, unit.Sp(13), "DPS").Layout(gtx)
					}),
					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
						return material.Label(o.style.Theme, unit.Sp(13), "Time").Layout(gtx)
					}),
				)
			})
		})
	}))
	if fight != nil {
		cb := make([]*Combatant, 0)
		for _, c := range fight.combatants {
			cb = append(cb, c)
		}
		sort.Slice(cb, func(i, j int) bool {
			return cb[i].overall.DPS() < cb[j].overall.DPS()
		})
		children = append(children,
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				list := material.List(o.style.Theme, &o.list)
				size := len(fight.combatants)
				return list.Layout(
					gtx,
					size,
					func(gtx layout.Context, index int) layout.Dimensions {
						return layout.Inset{Top: unit.Dp(8), Left: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							return o.RenderCombatRow(cb[index], gtx)
						})
					},
				)
			}),
		)
	}
	return layout.Stack{}.Layout(gtx,
		layout.Expanded(func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min = gtx.Constraints.Max
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
		}),
	)
}
func (o *Overlay) RenderCombatRow(cb *Combatant, gtx layout.Context) layout.Dimensions {
	return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
		layout.Flexed(3, func(gtx layout.Context) layout.Dimensions {
			return material.Label(o.style.Theme, unit.Sp(13), cb.name).Layout(gtx)
		}),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			label := material.Label(o.style.Theme, unit.Sp(13), fmt.Sprintf("%d", cb.overall.damage))
			label.Alignment = text.End
			return label.Layout(gtx)
		}),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			label := material.Label(o.style.Theme, unit.Sp(13), fmt.Sprintf("%.0f", cb.overall.DPS()))
			label.Alignment = text.End
			return label.Layout(gtx)
		}),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			dur := cb.overall.lastUpdate.Sub(cb.overall.start)
			minutes := int(dur.Minutes())
			seconds := int(dur.Seconds()) % 60
			label := material.Label(o.style.Theme, unit.Sp(13), fmt.Sprintf("%02d:%02d", minutes, seconds))
			label.Alignment = text.End
			return label.Layout(gtx)
		}),
	)
}
