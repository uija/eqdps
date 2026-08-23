package overlay

import (
	"fmt"
	"sort"
	"sync"
	"time"

	"gioui.org/app"
	"gioui.org/io/system"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"github.com/uija/eqdps/internal/data"
	"github.com/uija/eqdps/internal/ui"
)

type OverlayUpdate struct {
	Timers []*data.TimerTracker
}

type Overlay struct {
	style  *ui.Style
	window *app.Window
	list   widget.List

	config *data.Config

	fightMu sync.RWMutex
	fight   *data.Fight
	timers  []data.TimerTracker
	done    chan struct{}

	updates chan any

	decorations widget.Decorations
}

func NewOverlay(source *ui.Style, cfg *data.Config) *Overlay {
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
		updates: make(chan any, 1),
		style:   &overlayStyle,
		done:    make(chan struct{}),
		config:  cfg,
	}
	o.list.Axis = layout.Vertical
	return o
}
func (o *Overlay) Invalidate() {
	o.window.Invalidate()
}
func (o *Overlay) Run(onEnd func()) {
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
func (o *Overlay) Close() {
	o.window.Perform(system.ActionClose)
}
func (o *Overlay) Send(update any) bool {
	select {
	case o.updates <- update:
		return true
	case <-o.done:
		return false
	}
}
func (o *Overlay) handleUpdates() {
	for {
		select {
		case d := <-o.updates:
			o.fightMu.Lock()
			switch a := d.(type) {
			case *data.Fight:
				o.fight = a
			case []data.TimerTracker:
				o.timers = a
			}
			o.fightMu.Unlock()
			o.window.Invalidate()
		case <-o.done:
			return
		}
	}
}

func (o *Overlay) Layout(gtx layout.Context) layout.Dimensions {
	ui.Fill(gtx, o.style.Palette.Window)
	return layout.Stack{}.Layout(
		gtx,
		layout.Expanded(func(gtx layout.Context) layout.Dimensions {
			return o.RenderMainView(gtx)
		}),
		layout.Expanded(func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min = gtx.Constraints.Max
			return layout.NE.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return o.decorations.LayoutMove(gtx, func(gtx layout.Context) layout.Dimensions {
					return layout.UniformInset(unit.Dp(4)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return ui.Arrows.Layout(gtx, o.style.Palette.Text)
					})
				})
			})
		}),
	)
}
func (o *Overlay) RenderMainView(gtx layout.Context) layout.Dimensions {
	o.fightMu.RLock()
	fight := o.fight
	o.fightMu.RUnlock()

	children := make([]layout.FlexChild, 0)
	if len(o.timers) > 0 {

		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return ui.ColoredRow(gtx, o.style.Palette.Panel, func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Top: unit.Dp(4), Bottom: unit.Dp(4), Left: unit.Dp(12)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					rows := make([]layout.FlexChild, 0)
					for _, tt := range o.timers {
						dur := time.Until(tt.StopsAt)
						minutes := int(dur.Minutes())
						seconds := int(dur.Seconds()) % 60
						rows = append(rows, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
								layout.Rigid(material.Label(o.style.Theme, unit.Sp(o.ScaleFont(13)), fmt.Sprintf("%02d:%02d", minutes, seconds)).Layout),
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									return layout.Inset{Left: unit.Dp(8)}.Layout(gtx, material.Label(o.style.Theme, unit.Sp(o.ScaleFont(13)), tt.Event.Spell).Layout)
								}),
							)
						}))
					}
					return layout.Flex{Axis: layout.Vertical}.Layout(gtx, rows...)
				})
			})
		}))
	}

	children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
		return ui.ColoredRow(gtx, o.style.Palette.Panel, func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Top: unit.Dp(4), Bottom: unit.Dp(4), Left: unit.Dp(12)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				gtx.Constraints.Min.X = gtx.Constraints.Max.X
				if fight == nil {
					return material.Label(o.style.Theme, unit.Sp(o.ScaleFont(13)), "Waiting for data").Layout(gtx)
				} else {
					return material.Label(o.style.Theme, unit.Sp(o.ScaleFont(13)), fight.Name).Layout(gtx)
				}
			})
		})
	}))
	children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
		return ui.ColoredRow(gtx, o.style.Palette.Panel, func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Top: unit.Dp(4), Bottom: unit.Dp(4), Left: unit.Dp(12)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
					layout.Flexed(3, func(gtx layout.Context) layout.Dimensions {
						return material.Label(o.style.Theme, unit.Sp(o.ScaleFont(13)), "Combatant").Layout(gtx)
					}),
					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
						label := material.Label(o.style.Theme, unit.Sp(o.ScaleFont(13)), "Dmg")
						label.Alignment = text.End
						return layout.Inset{Right: unit.Dp(8)}.Layout(gtx, label.Layout)
					}),
					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
						label := material.Label(o.style.Theme, unit.Sp(o.ScaleFont(13)), "DPS")
						label.Alignment = text.End
						return layout.Inset{Right: unit.Dp(8)}.Layout(gtx, label.Layout)
					}),
					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
						label := material.Label(o.style.Theme, unit.Sp(o.ScaleFont(13)), "Time")
						label.Alignment = text.End
						return layout.Inset{Right: unit.Dp(8)}.Layout(gtx, label.Layout)
					}),
				)
			})
		})
	}))
	if fight != nil {
		cb := make([]*data.Combatant, 0)
		for _, c := range fight.Combatants {
			cb = append(cb, c)
		}
		sort.Slice(cb, func(i, j int) bool {
			if cb[i].Overall.DPS() == cb[j].Overall.DPS() {
				if cb[i].Overall.Damage == cb[j].Overall.Damage {
					return cb[i].Name > cb[j].Name
				}
				return cb[i].Overall.Damage > cb[j].Overall.Damage
			}
			return cb[i].Overall.DPS() > cb[j].Overall.DPS()
		})
		children = append(children,
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				list := material.List(o.style.Theme, &o.list)
				size := len(fight.Combatants)
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
func (o *Overlay) RenderCombatRow(cb *data.Combatant, gtx layout.Context) layout.Dimensions {
	return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
		layout.Flexed(3, func(gtx layout.Context) layout.Dimensions {
			return material.Label(o.style.Theme, unit.Sp(o.ScaleFont(13)), cb.Name).Layout(gtx)
		}),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			label := material.Label(o.style.Theme, unit.Sp(o.ScaleFont(13)), fmt.Sprintf("%d", cb.Overall.Damage))
			label.Alignment = text.End
			return label.Layout(gtx)
		}),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			label := material.Label(o.style.Theme, unit.Sp(o.ScaleFont(13)), fmt.Sprintf("%.0f", cb.Overall.DPS()))
			label.Alignment = text.End
			return label.Layout(gtx)
		}),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			dur := cb.Overall.LastUpdate.Sub(cb.Overall.Start)
			minutes := int(dur.Minutes())
			seconds := int(dur.Seconds()) % 60
			label := material.Label(o.style.Theme, unit.Sp(o.ScaleFont(13)), fmt.Sprintf("%02d:%02d", minutes, seconds))
			label.Alignment = text.End
			return label.Layout(gtx)
		}),
	)
}

func (o *Overlay) ScaleFont(size float32) float32 {
	if o.config != nil && o.config.UIConfig.OverlayFontScale >= 0.5 {
		return size * o.config.UIConfig.OverlayFontScale
	}
	return size
}
