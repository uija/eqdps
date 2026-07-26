package main

import (
	"image"
	"image/color"
	"log"
	"os"

	"gioui.org/app"
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

var colors = struct {
	window color.NRGBA
	chrome color.NRGBA
	text   color.NRGBA
	muted  color.NRGBA
	hover  color.NRGBA
	panel  color.NRGBA
	shadow color.NRGBA
}{
	window: color.NRGBA{R: 18, G: 20, B: 22, A: 255},
	chrome: color.NRGBA{R: 27, G: 30, B: 33, A: 255},
	text:   color.NRGBA{R: 225, G: 226, B: 222, A: 255},
	muted:  color.NRGBA{R: 150, G: 154, B: 151, A: 255},
	hover:  color.NRGBA{R: 42, G: 46, B: 50, A: 255},
	panel:  color.NRGBA{R: 31, G: 34, B: 37, A: 255},
	shadow: color.NRGBA{A: 190},
}

type shell struct {
	theme        *material.Theme
	window       *app.Window
	menus        []menu
	activeMenu   int
	status       string
	helpOpen     bool
	helpClose    widget.Clickable
	helpBackdrop widget.Clickable
}

type menu struct {
	label string
	click widget.Clickable
	items []menuItem
}

type menuItem struct {
	label   string
	action  string
	enabled bool
	click   widget.Clickable
}

func main() {
	go func() {
		window := new(app.Window)
		window.Option(
			app.Title("eqdps"),
			app.Size(unit.Dp(960), unit.Dp(640)),
			app.MinSize(unit.Dp(640), unit.Dp(400)),
		)
		if err := run(window); err != nil {
			log.Print(err)
		}
		os.Exit(0)
	}()
	app.Main()
}

func run(window *app.Window) error {
	ui := newShell(window)
	var ops op.Ops
	for {
		switch event := window.Event().(type) {
		case app.DestroyEvent:
			return event.Err
		case app.FrameEvent:
			gtx := app.NewContext(&ops, event)
			ui.layout(gtx)
			event.Frame(gtx.Ops)
		}
	}
}

func newShell(window *app.Window) *shell {
	theme := material.NewTheme()
	theme.Palette.Bg = colors.window
	theme.Palette.Fg = colors.text
	return &shell{
		theme:      theme,
		window:     window,
		activeMenu: -1,
		menus: []menu{
			{
				label: "File",
				items: []menuItem{{label: "Quit", action: "quit", enabled: true}},
			},
			{
				label: "View",
				items: []menuItem{{label: "No views available", enabled: false}},
			},
			{label: "Help"},
		},
		status: "Ready",
	}
}

func (s *shell) layout(gtx layout.Context) layout.Dimensions {
	paint.Fill(gtx.Ops, colors.window)
	s.update(gtx)
	return layout.Stack{}.Layout(gtx,
		layout.Expanded(func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(s.layoutMenu),
				layout.Flexed(1, s.layoutMain),
				layout.Rigid(s.layoutStatus),
			)
		}),
		layout.Stacked(s.layoutOpenMenu),
		layout.Expanded(s.layoutHelp),
	)
}

func (s *shell) update(gtx layout.Context) {
	for index := range s.menus {
		if s.menus[index].click.Clicked(gtx) {
			if s.menus[index].label == "Help" {
				s.activeMenu = -1
				s.helpOpen = true
				continue
			}
			if s.activeMenu == index {
				s.activeMenu = -1
			} else {
				s.activeMenu = index
			}
		}
		for itemIndex := range s.menus[index].items {
			item := &s.menus[index].items[itemIndex]
			if !item.enabled || !item.click.Clicked(gtx) {
				continue
			}
			s.activeMenu = -1
			if item.action == "quit" {
				s.window.Perform(system.ActionClose)
			}
		}
	}
	if s.helpClose.Clicked(gtx) || s.helpBackdrop.Clicked(gtx) {
		s.helpOpen = false
	}
}

func (s *shell) layoutMenu(gtx layout.Context) layout.Dimensions {
	gtx.Constraints.Min.Y = gtx.Dp(unit.Dp(38))
	gtx.Constraints.Max.Y = gtx.Constraints.Min.Y
	fill(gtx, colors.chrome)

	children := make([]layout.FlexChild, 0, len(s.menus)+1)
	for index := range s.menus {
		index := index
		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return s.menuButton(gtx, &s.menus[index])
		}))
	}
	children = append(children, layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
		label := material.Label(s.theme, unit.Sp(14), "EVERQUEST LEGENDS")
		label.Color = colors.muted
		label.Alignment = text.End
		return layout.E.Layout(gtx, label.Layout)
	}))

	return layout.UniformInset(unit.Dp(8)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx, children...)
	})
}

func (s *shell) menuButton(gtx layout.Context, item *menu) layout.Dimensions {
	return item.click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		if item.click.Hovered() {
			fill(gtx, colors.hover)
		}
		return layout.Inset{Left: unit.Dp(10), Right: unit.Dp(10)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			label := material.Label(s.theme, unit.Sp(15), item.label)
			label.Color = colors.text
			return layout.Center.Layout(gtx, label.Layout)
		})
	})
}

func (s *shell) layoutMain(gtx layout.Context) layout.Dimensions {
	return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical, Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				label := material.Label(s.theme, unit.Sp(26), "eqdps")
				label.Color = colors.text
				return label.Layout(gtx)
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Top: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					label := material.Label(s.theme, unit.Sp(15), "No modules registered")
					label.Color = colors.muted
					return label.Layout(gtx)
				})
			}),
		)
	})
}

func (s *shell) layoutStatus(gtx layout.Context) layout.Dimensions {
	gtx.Constraints.Min.Y = gtx.Dp(unit.Dp(30))
	gtx.Constraints.Max.Y = gtx.Constraints.Min.Y
	fill(gtx, colors.chrome)
	return layout.Inset{Left: unit.Dp(14), Right: unit.Dp(14)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		label := material.Label(s.theme, unit.Sp(14), s.status)
		label.Color = colors.muted
		return layout.W.Layout(gtx, label.Layout)
	})
}

func (s *shell) layoutOpenMenu(gtx layout.Context) layout.Dimensions {
	if s.activeMenu < 0 || s.activeMenu >= len(s.menus) {
		return layout.Dimensions{}
	}

	left := unit.Dp(8)
	if s.activeMenu == 1 {
		left = unit.Dp(62)
	}
	return layout.Inset{Top: unit.Dp(38), Left: left}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		width := gtx.Dp(unit.Dp(210))
		gtx.Constraints.Min.X = width
		gtx.Constraints.Max.X = width
		fill(gtx, colors.panel)

		items := s.menus[s.activeMenu].items
		children := make([]layout.FlexChild, 0, len(items))
		for index := range items {
			index := index
			children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				item := &items[index]
				gtx.Constraints.Min.Y = gtx.Dp(unit.Dp(36))
				gtx.Constraints.Max.Y = gtx.Constraints.Min.Y
				return item.click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					if item.enabled && item.click.Hovered() {
						fill(gtx, colors.hover)
					}
					return layout.Inset{Left: unit.Dp(12), Right: unit.Dp(12)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						label := material.Label(s.theme, unit.Sp(15), item.label)
						if item.enabled {
							label.Color = colors.text
						} else {
							label.Color = colors.muted
						}
						return layout.W.Layout(gtx, label.Layout)
					})
				})
			}))
		}
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
	})
}

func (s *shell) layoutHelp(gtx layout.Context) layout.Dimensions {
	if !s.helpOpen {
		return layout.Dimensions{}
	}

	return layout.Stack{}.Layout(gtx,
		layout.Expanded(func(gtx layout.Context) layout.Dimensions {
			return s.helpBackdrop.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				fill(gtx, colors.shadow)
				return layout.Dimensions{Size: gtx.Constraints.Min}
			})
		}),
		layout.Expanded(func(gtx layout.Context) layout.Dimensions {
			return layout.UniformInset(unit.Dp(32)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				gtx.Constraints.Min = gtx.Constraints.Max
				fill(gtx, colors.panel)
				return layout.UniformInset(unit.Dp(24)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
								layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
									label := material.Label(s.theme, unit.Sp(24), "Help")
									label.Color = colors.text
									return label.Layout(gtx)
								}),
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									button := material.Button(s.theme, &s.helpClose, "Close")
									return button.Layout(gtx)
								}),
							)
						}),
						layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
							return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
								label := material.Label(s.theme, unit.Sp(15), "No help content available")
								label.Color = colors.muted
								return label.Layout(gtx)
							})
						}),
					)
				})
			})
		}),
	)
}

func fill(gtx layout.Context, background color.NRGBA) {
	defer clip.Rect(image.Rectangle{Max: gtx.Constraints.Min}).Push(gtx.Ops).Pop()
	paint.ColorOp{Color: background}.Add(gtx.Ops)
	paint.PaintOp{}.Add(gtx.Ops)
}
