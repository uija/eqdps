package main

import (
	"fmt"
	"image"
	"image/color"
	"log"
	"os"

	"gioui.org/app"
	"gioui.org/font"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
)

var palette = struct {
	window, chrome, rail, panel, panelAlt color.NRGBA
	line, text, muted, accent, success    color.NRGBA
}{
	window:   color.NRGBA{R: 18, G: 20, B: 22, A: 255},
	chrome:   color.NRGBA{R: 27, G: 30, B: 33, A: 255},
	rail:     color.NRGBA{R: 23, G: 26, B: 29, A: 255},
	panel:    color.NRGBA{R: 31, G: 34, B: 37, A: 255},
	panelAlt: color.NRGBA{R: 36, G: 39, B: 42, A: 255},
	line:     color.NRGBA{R: 57, G: 61, B: 65, A: 255},
	text:     color.NRGBA{R: 225, G: 226, B: 222, A: 255},
	muted:    color.NRGBA{R: 150, G: 154, B: 151, A: 255},
	accent:   color.NRGBA{R: 190, G: 155, B: 74, A: 255},
	success:  color.NRGBA{R: 109, G: 178, B: 124, A: 255},
}

type shell struct {
	theme      *material.Theme
	workspace  int
	activeMenu int
	menus      []menu
	rail       []railItem
}

type menu struct {
	name  string
	click widget.Clickable
	items []menuItem
}

type menuItem struct {
	name    string
	detail  string
	click   widget.Clickable
	enabled bool
}

type railItem struct {
	short string
	name  string
	click widget.Clickable
}

type fakeCombatant struct {
	name              string
	damage, dps, sdps int
	hits, crits       int
	active            string
	accent            bool
}

var fakeFight = []fakeCombatant{
	{name: "You", damage: 4789, dps: 165, hits: 56, crits: 5, active: "00:29", accent: true},
	{name: "Gigglemage", damage: 3779, dps: 130, sdps: 126, hits: 57, crits: 1, active: "00:29"},
	{name: "Griz", damage: 3138, dps: 116, sdps: 105, hits: 105, crits: 3, active: "00:27"},
	{name: "Moth", damage: 2918, dps: 112, sdps: 97, hits: 97, crits: 21, active: "00:26"},
	{name: "Zabektik", damage: 571, dps: 19, sdps: 19, hits: 16, active: "00:30"},
	{name: "a rock golem", damage: 1492, dps: 50, hits: 17, active: "00:30"},
}

func main() {
	go func() {
		window := new(app.Window)
		window.Option(
			app.Title("eqdps — Gio preview"),
			app.Size(unit.Dp(1050), unit.Dp(700)),
			app.MinSize(unit.Dp(720), unit.Dp(460)),
		)
		if err := run(window); err != nil {
			log.Print(err)
		}
		os.Exit(0)
	}()
	app.Main()
}

func run(window *app.Window) error {
	ui := newShell()
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

func newShell() *shell {
	theme := material.NewTheme()
	theme.Palette.Fg = palette.text
	theme.Palette.Bg = palette.window
	return &shell{
		theme:      theme,
		activeMenu: -1,
		menus: []menu{
			{name: "File", items: []menuItem{{name: "Open logfile…", detail: "Choose an EverQuest log", enabled: true}, {name: "Recent logfiles", detail: "No recent files", enabled: false}, {name: "Exit", enabled: true}}},
			{name: "Combat", items: []menuItem{{name: "Current fight", enabled: true}, {name: "History…", enabled: true}, {name: "Load last hour", enabled: true}, {name: "Filter…", enabled: true}}},
			{name: "View", items: []menuItem{{name: "Damage meter", enabled: true}, {name: "Plane of Sky", enabled: true}, {name: "DPS overlay", detail: "Not available in this preview", enabled: false}}},
			{name: "Tools", items: []menuItem{{name: "Preferences…", enabled: true}}},
			{name: "Help", items: []menuItem{{name: "About eqdps", enabled: true}}},
		},
		rail: []railItem{{short: "DPS", name: "Damage Meter"}, {short: "LOG", name: "Combat History"}, {short: "SKY", name: "Plane of Sky"}, {short: "SET", name: "Settings"}},
	}
}

func (s *shell) layout(gtx layout.Context) layout.Dimensions {
	paint.Fill(gtx.Ops, palette.window)
	s.update(gtx)
	return layout.Stack{}.Layout(gtx,
		layout.Expanded(func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(s.layoutMenuBar),
				layout.Flexed(1, s.layoutBody),
				layout.Rigid(s.layoutStatus),
			)
		}),
		layout.Stacked(s.layoutOpenMenu),
	)
}

func (s *shell) update(gtx layout.Context) {
	for index := range s.menus {
		if s.menus[index].click.Clicked(gtx) {
			if s.activeMenu == index {
				s.activeMenu = -1
			} else {
				s.activeMenu = index
			}
		}
	}
	for index := range s.rail {
		if s.rail[index].click.Clicked(gtx) {
			s.workspace = index
			s.activeMenu = -1
		}
	}
	if s.activeMenu >= 0 {
		for index := range s.menus[s.activeMenu].items {
			item := &s.menus[s.activeMenu].items[index]
			if item.enabled && item.click.Clicked(gtx) {
				s.activeMenu = -1
			}
		}
	}
}

func (s *shell) layoutMenuBar(gtx layout.Context) layout.Dimensions {
	gtx.Constraints.Min.Y = gtx.Dp(unit.Dp(38))
	gtx.Constraints.Max.Y = gtx.Constraints.Min.Y
	fill(gtx, palette.chrome)
	return inset(unit.Dp(14), 0).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		children := make([]layout.FlexChild, 0, len(s.menus)+1)
		for index := range s.menus {
			index := index
			children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return flatTextControl(gtx, s.theme, &s.menus[index].click, s.menus[index].name, s.activeMenu == index)
			}))
		}
		children = append(children, layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return layout.E.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return label(gtx, s.theme, "GUI SHELL PREVIEW", unit.Sp(12), palette.muted, text.End)
			})
		}))
		return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx, children...)
	})
}

func (s *shell) layoutBody(gtx layout.Context) layout.Dimensions {
	return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
		layout.Rigid(s.layoutRail),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return inset(unit.Dp(22), unit.Dp(20)).Layout(gtx, s.layoutWorkspace)
		}),
	)
}

func (s *shell) layoutRail(gtx layout.Context) layout.Dimensions {
	gtx.Constraints.Min.X = gtx.Dp(unit.Dp(68))
	gtx.Constraints.Max.X = gtx.Constraints.Min.X
	fill(gtx, palette.rail)
	items := make([]layout.FlexChild, 0, len(s.rail)+1)
	for index := range s.rail {
		index := index
		if index == len(s.rail)-1 {
			items = append(items, layout.Flexed(1, func(gtx layout.Context) layout.Dimensions { return layout.Dimensions{Size: gtx.Constraints.Min} }))
		}
		items = append(items, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return railControl(gtx, s.theme, &s.rail[index].click, s.rail[index], s.workspace == index)
		}))
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, items...)
}

func (s *shell) layoutWorkspace(gtx layout.Context) layout.Dimensions {
	switch s.workspace {
	case 1:
		return s.layoutPlaceholder(gtx, "Combat History", "Completed fights and history replay will live here.")
	case 2:
		return s.layoutPlaceholder(gtx, "Plane of Sky", "Quest progress will use this dedicated workspace.")
	case 3:
		return s.layoutPlaceholder(gtx, "Settings", "Application, logfile, and overlay preferences will live here.")
	default:
		return s.layoutDamageMeter(gtx)
	}
}

func (s *shell) layoutDamageMeter(gtx layout.Context) layout.Dimensions {
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Baseline}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return label(gtx, s.theme, "a rock golem", unit.Sp(24), palette.text, text.Start)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return inset(unit.Dp(12), 0).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return label(gtx, s.theme, "slain by You", unit.Sp(13), palette.muted, text.Start)
					})
				}),
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					return layout.E.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return label(gtx, s.theme, "00:30", unit.Sp(15), palette.accent, text.End)
					})
				}),
			)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return inset(0, unit.Dp(14)).Layout(gtx, s.layoutFightSummary)
		}),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return s.layoutCombatRow(gtx, fakeCombatant{name: "COMBATANT", damage: -1}, true, false)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions { return separator(gtx) }),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions { return s.layoutCombatRows(gtx) }),
			)
		}),
	)
}

func (s *shell) layoutFightSummary(gtx layout.Context) layout.Dimensions {
	gtx.Constraints.Min.Y = gtx.Dp(unit.Dp(40))
	gtx.Constraints.Max.Y = gtx.Constraints.Min.Y
	fill(gtx, palette.panel)
	return centerContent(gtx, func(gtx layout.Context) layout.Dimensions {
		return inset(unit.Dp(14), 0).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return label(gtx, s.theme, "6 combatants", unit.Sp(14), palette.text, text.Start)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return inset(unit.Dp(22), 0).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return label(gtx, s.theme, "13,388 total damage", unit.Sp(14), palette.muted, text.Start)
					})
				}),
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					return layout.E.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return label(gtx, s.theme, "CURRENT FIGHT", unit.Sp(12), palette.success, text.End)
					})
				}),
			)
		})
	})
}

func (s *shell) layoutCombatRows(gtx layout.Context) layout.Dimensions {
	children := make([]layout.FlexChild, 0, len(fakeFight))
	for index, combatant := range fakeFight {
		index, combatant := index, combatant
		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return s.layoutCombatRow(gtx, combatant, false, index%2 == 1)
		}))
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
}

func (s *shell) layoutCombatRow(gtx layout.Context, row fakeCombatant, header, alternate bool) layout.Dimensions {
	gtx.Constraints.Min.Y = gtx.Dp(unit.Dp(34))
	gtx.Constraints.Max.Y = gtx.Constraints.Min.Y
	if alternate {
		fill(gtx, palette.panel)
	}
	if row.accent {
		paint.FillShape(gtx.Ops, palette.accent, clip.Rect{Max: image.Pt(gtx.Dp(unit.Dp(3)), gtx.Constraints.Max.Y)}.Op())
	}
	nameColor := palette.text
	fontWeight := font.Normal
	if header {
		nameColor = palette.muted
		fontWeight = font.SemiBold
	}
	cell := func(value string, weight float32, align text.Alignment) layout.FlexChild {
		return layout.Flexed(weight, func(gtx layout.Context) layout.Dimensions {
			return labelWeight(gtx, s.theme, value, unit.Sp(14), nameColor, align, fontWeight)
		})
	}
	values := []string{row.name, formatCell(row.damage, "DAMAGE"), formatCell(row.dps, "DPS"), formatCell(row.sdps, "SDPS"), formatCell(row.hits, "HITS"), formatCell(row.crits, "CRITS"), row.active}
	if header {
		values = []string{"COMBATANT", "DAMAGE", "DPS", "SDPS", "HITS", "CRITS", "ACTIVE"}
	}
	return centerContent(gtx, func(gtx layout.Context) layout.Dimensions {
		return inset(unit.Dp(14), 0).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
				cell(values[0], 3.8, text.Start), cell(values[1], 1.4, text.End), cell(values[2], 1, text.End), cell(values[3], 1, text.End), cell(values[4], 1, text.End), cell(values[5], 1, text.End), cell(values[6], 1.2, text.End),
			)
		})
	})
}

func (s *shell) layoutPlaceholder(gtx layout.Context, title, description string) layout.Dimensions {
	return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical, Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return label(gtx, s.theme, title, unit.Sp(25), palette.text, text.Middle)
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return inset(0, unit.Dp(10)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return label(gtx, s.theme, description, unit.Sp(14), palette.muted, text.Middle)
				})
			}),
		)
	})
}

func (s *shell) layoutStatus(gtx layout.Context) layout.Dimensions {
	gtx.Constraints.Min.Y = gtx.Dp(unit.Dp(34))
	gtx.Constraints.Max.Y = gtx.Constraints.Min.Y
	fill(gtx, palette.chrome)
	return centerContent(gtx, func(gtx layout.Context) layout.Dimensions {
		return inset(unit.Dp(14), 0).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return label(gtx, s.theme, "Wyrmberg · rivervale", unit.Sp(12), palette.text, text.Start)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return inset(unit.Dp(28), 0).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return label(gtx, s.theme, "XP ~42.1% · 18.4%/h", unit.Sp(12), palette.muted, text.Start)
					})
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return label(gtx, s.theme, "PoS: 2 ready", unit.Sp(12), palette.accent, text.Start)
				}),
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					return layout.E.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return label(gtx, s.theme, "●  LIVE", unit.Sp(12), palette.success, text.End)
					})
				}),
			)
		})
	})
}

func (s *shell) layoutOpenMenu(gtx layout.Context) layout.Dimensions {
	if s.activeMenu < 0 {
		return layout.Dimensions{}
	}
	xOffsets := []int{12, 68, 151, 215, 278}
	offset := op.Offset(image.Pt(gtx.Dp(unit.Dp(xOffsets[s.activeMenu])), gtx.Dp(unit.Dp(38)))).Push(gtx.Ops)
	defer offset.Pop()
	gtx.Constraints.Min = image.Pt(gtx.Dp(unit.Dp(230)), 0)
	gtx.Constraints.Max.X = gtx.Constraints.Min.X
	menu := &s.menus[s.activeMenu]
	menuHeight := gtx.Dp(unit.Dp(42*len(menu.items) + 2))
	gtx.Constraints.Min.Y = menuHeight
	gtx.Constraints.Max.Y = menuHeight
	children := make([]layout.FlexChild, 0, len(menu.items))
	for index := range menu.items {
		index := index
		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return menuControl(gtx, s.theme, &menu.items[index])
		}))
	}
	return outline(gtx, palette.line, func(gtx layout.Context) layout.Dimensions {
		fill(gtx, palette.panel)
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
	})
}

func flatTextControl(gtx layout.Context, theme *material.Theme, click *widget.Clickable, value string, active bool) layout.Dimensions {
	return click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		pointer.CursorPointer.Add(gtx.Ops)
		if active || click.Hovered() {
			fill(gtx, palette.panelAlt)
		}
		return inset(unit.Dp(12), 0).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return label(gtx, theme, value, unit.Sp(14), palette.text, text.Start)
		})
	})
}

func railControl(gtx layout.Context, theme *material.Theme, click *widget.Clickable, item railItem, selected bool) layout.Dimensions {
	gtx.Constraints.Min.Y = gtx.Dp(unit.Dp(58))
	gtx.Constraints.Max.Y = gtx.Constraints.Min.Y
	return click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		pointer.CursorPointer.Add(gtx.Ops)
		if selected || click.Hovered() {
			fill(gtx, palette.panel)
		}
		if selected {
			paint.FillShape(gtx.Ops, palette.accent, clip.Rect{Max: image.Pt(gtx.Dp(unit.Dp(3)), gtx.Constraints.Max.Y)}.Op())
		}
		return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			valueColor := palette.muted
			if selected {
				valueColor = palette.accent
			}
			return labelWeight(gtx, theme, item.short, unit.Sp(12), valueColor, text.Middle, font.SemiBold)
		})
	})
}

func menuControl(gtx layout.Context, theme *material.Theme, item *menuItem) layout.Dimensions {
	gtx.Constraints.Min.Y = gtx.Dp(unit.Dp(42))
	gtx.Constraints.Max.Y = gtx.Constraints.Min.Y
	return item.click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		if item.enabled {
			pointer.CursorPointer.Add(gtx.Ops)
			if item.click.Hovered() {
				fill(gtx, palette.panelAlt)
			}
		}
		foreground := palette.text
		if !item.enabled {
			foreground = palette.muted
		}
		return inset(unit.Dp(12), 0).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical, Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return label(gtx, theme, item.name, unit.Sp(13), foreground, text.Start)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					if item.detail == "" {
						return layout.Dimensions{}
					}
					return label(gtx, theme, item.detail, unit.Sp(10), palette.muted, text.Start)
				}),
			)
		})
	})
}

func label(gtx layout.Context, theme *material.Theme, value string, size unit.Sp, foreground color.NRGBA, align text.Alignment) layout.Dimensions {
	return labelWeight(gtx, theme, value, size, foreground, align, font.Normal)
}

func labelWeight(gtx layout.Context, theme *material.Theme, value string, size unit.Sp, foreground color.NRGBA, align text.Alignment, weight font.Weight) layout.Dimensions {
	style := material.Label(theme, size, value)
	style.Color = foreground
	style.Alignment = align
	style.Font.Weight = weight
	return style.Layout(gtx)
}

func formatCell(value int, header string) string {
	if value < 0 {
		return header
	}
	if value == 0 {
		return "—"
	}
	return fmt.Sprintf("%d", value)
}

func inset(horizontal, vertical unit.Dp) layout.Inset {
	return layout.Inset{Left: horizontal, Right: horizontal, Top: vertical, Bottom: vertical}
}

func fill(gtx layout.Context, fill color.NRGBA) {
	paint.FillShape(gtx.Ops, fill, clip.Rect{Max: gtx.Constraints.Max}.Op())
}

func separator(gtx layout.Context) layout.Dimensions {
	height := gtx.Dp(unit.Dp(1))
	paint.FillShape(gtx.Ops, palette.line, clip.Rect{Max: image.Pt(gtx.Constraints.Max.X, height)}.Op())
	return layout.Dimensions{Size: image.Pt(gtx.Constraints.Max.X, height)}
}

func outline(gtx layout.Context, border color.NRGBA, content layout.Widget) layout.Dimensions {
	paint.FillShape(gtx.Ops, border, clip.Rect{Max: gtx.Constraints.Max}.Op())
	return layout.UniformInset(unit.Dp(1)).Layout(gtx, content)
}

// centerContent lays out a naturally sized child inside an exact-height area.
// Passing the area's minimum height directly to a label makes its glyphs hug
// the top of the row instead of centering the label itself.
func centerContent(gtx layout.Context, content layout.Widget) layout.Dimensions {
	size := image.Pt(gtx.Constraints.Max.X, gtx.Constraints.Min.Y)
	macro := op.Record(gtx.Ops)
	child := gtx
	child.Constraints.Min = image.Point{}
	child.Constraints.Max = size
	dimensions := content(child)
	call := macro.Stop()
	offset := op.Offset(image.Pt(0, max(0, (size.Y-dimensions.Size.Y)/2))).Push(gtx.Ops)
	call.Add(gtx.Ops)
	offset.Pop()
	return layout.Dimensions{Size: size}
}
