package menu

import (
	"image"
	"image/color"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"github.com/uija/eqdps/internal/ui"
)

const SEPARATOR_TEXT = "-- SEPARATOR --"

// Bar is an application menu bar with overlay dropdown menus.
type Bar struct {
	style     *ui.Style
	title     string
	entries   []*Menu
	active    *Menu
	barHeight unit.Dp
	backdrop  widget.Clickable
}

// Menu is one dropdown menu in a Bar.
type Menu struct {
	label    string
	click    widget.Clickable
	items    []*Item
	action   func()
	dropdown bool
	anchorX  int
	openItem *Item
}

// Item is one actionable entry in a Menu.
type Item struct {
	label   string
	click   widget.Clickable
	action  func()
	submenu *Menu
	enabled bool
}

const MenuDropDownWidth = 300
const MenuEntryHeight = 38
const MenuItemHeight = 32

// NewBar returns an empty application menu bar.
func NewBar(style *ui.Style, title string) *Bar {
	return &Bar{
		style:     style,
		title:     title,
		barHeight: unit.Dp(MenuEntryHeight),
	}
}

// AddMenu adds a dropdown menu and returns it for item registration.
func (b *Bar) AddMenu(label string) *Menu {
	entry := &Menu{label: label, dropdown: true}
	b.entries = append(b.entries, entry)
	return entry
}

// AddAction adds a top-level action without a dropdown.
func (b *Bar) AddAction(label string, action func()) {
	b.entries = append(b.entries, &Menu{label: label, action: action})
}

// AddItem adds an enabled item to a dropdown menu.
func (m *Menu) AddItem(label string, action func()) *Item {
	item := &Item{label: label, action: action, enabled: true}
	m.items = append(m.items, item)
	return item
}

// AddMenu adds a submenu and returns it for item registration.
func (m *Menu) AddMenu(label string) *Menu {
	submenu := &Menu{label: label, dropdown: true}
	m.items = append(m.items, &Item{
		label:   label,
		submenu: submenu,
		enabled: true,
	})
	return submenu
}

// Clear removes every item and open submenu from the menu.
func (m *Menu) Clear() {
	m.items = nil
	m.openItem = nil
}

func (m *Menu) AddSeparator() {
	m.items = append(m.items, &Item{
		label:   SEPARATOR_TEXT,
		action:  func() {},
		enabled: false,
	})
}

// Update processes menu input events.
func (b *Bar) Update(gtx layout.Context) {
	for _, entry := range b.entries {
		if entry.click.Clicked(gtx) {
			if entry.dropdown {
				if b.active == entry {
					b.closeMenus()
				} else {
					b.closeMenus()
					b.active = entry
				}
			} else {
				b.closeMenus()
				if entry.action != nil {
					entry.action()
				}
			}
		}

	}
	if b.active != nil {
		b.updateMenu(gtx, b.active)
	}
	if b.backdrop.Clicked(gtx) {
		b.closeMenus()
	}
}

func (b *Bar) updateMenu(gtx layout.Context, menu *Menu) {
	for _, item := range menu.items {
		if !item.enabled {
			continue
		}

		clicked := item.click.Clicked(gtx)
		if item.submenu != nil {
			if clicked || item.click.Hovered() {
				menu.openItem = item
			}
			continue
		}

		if item.click.Hovered() {
			menu.openItem = nil
		}
		if clicked {
			b.closeMenus()
			if item.action != nil {
				item.action()
			}
			return
		}
	}

	if menu.openItem != nil && menu.openItem.submenu != nil {
		b.updateMenu(gtx, menu.openItem.submenu)
	}
}

func (b *Bar) closeMenus() {
	if b.active != nil {
		closeSubmenus(b.active)
	}
	b.active = nil
}

func closeSubmenus(menu *Menu) {
	if menu.openItem != nil && menu.openItem.submenu != nil {
		closeSubmenus(menu.openItem.submenu)
	}
	menu.openItem = nil
}

// LayoutBar renders the fixed menu-bar row.
func (b *Bar) LayoutBar(gtx layout.Context) layout.Dimensions {
	height := gtx.Dp(b.barHeight)
	gtx.Constraints.Min.Y = height
	gtx.Constraints.Max.Y = height
	fill(gtx, b.style.Palette.Chrome)

	anchorX := gtx.Dp(unit.Dp(8))
	children := make([]layout.FlexChild, 0, len(b.entries)+1)
	for _, entry := range b.entries {
		entry := entry
		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			entry.anchorX = anchorX
			dimensions := b.layoutButton(gtx, entry)
			anchorX += dimensions.Size.X
			return dimensions
		}))
	}
	children = append(children, layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
		label := ui.ColorLabel(b.style.Palette.Muted, material.Label(b.style.Theme, ui.Sp(14), b.title))
		label.Alignment = text.End
		return layout.E.Layout(gtx, label.Layout)
	}))

	return layout.UniformInset(unit.Dp(8)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx, children...)
	})
}

// LayoutOverlay renders the active dropdown above the application body.
func (b *Bar) LayoutOverlay(gtx layout.Context) layout.Dimensions {
	if b.active == nil {
		return layout.Dimensions{}
	}

	children := []layout.StackChild{
		layout.Expanded(b.layoutBackdrop),
		layout.Stacked(func(gtx layout.Context) layout.Dimensions {
			return b.layoutDropdown(gtx, b.active, b.active.anchorX, gtx.Dp(b.barHeight))
		}),
	}
	b.appendSubmenus(
		&children,
		b.active,
		b.active.anchorX,
		gtx.Dp(b.barHeight),
		gtx.Dp(unit.Dp(MenuDropDownWidth)),
		gtx.Dp(unit.Dp(MenuItemHeight)),
	)
	return layout.Stack{}.Layout(gtx, children...)
}

func (b *Bar) appendSubmenus(children *[]layout.StackChild, menu *Menu, x, y, width, itemHeight int) {
	if menu.openItem == nil || menu.openItem.submenu == nil {
		return
	}

	index := 0
	for i, item := range menu.items {
		if item == menu.openItem {
			index = i
			break
		}
	}
	submenu := menu.openItem.submenu
	submenuX := x + width
	submenuY := y + itemHeight*index
	*children = append(*children, layout.Stacked(func(gtx layout.Context) layout.Dimensions {
		return b.layoutDropdown(gtx, submenu, submenuX, submenuY)
	}))
	b.appendSubmenus(children, submenu, submenuX, submenuY, width, itemHeight)
}

func (b *Bar) layoutBackdrop(gtx layout.Context) layout.Dimensions {
	return layout.Inset{Top: b.barHeight}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Min = gtx.Constraints.Max
		return b.backdrop.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Dimensions{Size: gtx.Constraints.Max}
		})
	})
}

func (b *Bar) layoutDropdown(gtx layout.Context, menu *Menu, x, y int) layout.Dimensions {
	offset := op.Offset(image.Pt(x, y)).Push(gtx.Ops)
	defer offset.Pop()

	width := gtx.Dp(unit.Dp(MenuDropDownWidth))
	gtx.Constraints.Min.X = width
	gtx.Constraints.Max.X = width
	itemCount := max(1, len(menu.items))
	height := gtx.Dp(unit.Dp((MenuItemHeight) * itemCount))
	gtx.Constraints.Min.Y = height
	gtx.Constraints.Max.Y = height
	fill(gtx, b.style.Palette.Panel)

	if len(menu.items) == 0 {
		return b.layoutEmptyItem(gtx)
	}

	children := make([]layout.FlexChild, 0, len(menu.items))
	for _, item := range menu.items {
		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return b.layoutItem(gtx, item)
		}))
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
}

func (b *Bar) layoutButton(gtx layout.Context, entry *Menu) layout.Dimensions {
	return entry.click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		if entry.click.Hovered() {
			fill(gtx, b.style.Palette.Hover)
		}
		return layout.Inset{Left: unit.Dp(10), Right: unit.Dp(10)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			label := ui.ColorLabel(b.style.Palette.Text, ui.Label(b.style, entry.label))
			return layout.Center.Layout(gtx, label.Layout)
		})
	})
}

func (b *Bar) layoutItem(gtx layout.Context, item *Item) layout.Dimensions {
	height := gtx.Dp(unit.Dp(MenuItemHeight))
	gtx.Constraints.Min.Y = height
	gtx.Constraints.Max.Y = height
	return item.click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		if item.enabled && item.click.Hovered() {
			fill(gtx, b.style.Palette.Hover)
		}
		return layout.Inset{Left: unit.Dp(12), Right: unit.Dp(12), Top: unit.Dp(4)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			caption := item.label
			if item.label == SEPARATOR_TEXT {
				caption = "----"
			}
			var label material.LabelStyle
			if item.enabled {
				label = ui.ColorLabel(b.style.Palette.Text, ui.Label(b.style, caption))
			} else {
				label = ui.ColorLabel(b.style.Palette.Muted, ui.Label(b.style, caption))
			}
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					return layout.W.Layout(gtx, label.Layout)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					if item.submenu == nil {
						return layout.Dimensions{}
					}
					arrow := ui.Label(b.style, ">")
					arrow.Color = b.style.Palette.Muted
					return arrow.Layout(gtx)
				}),
			)
		})
	})
}

func (b *Bar) layoutEmptyItem(gtx layout.Context) layout.Dimensions {
	height := gtx.Dp(unit.Dp(MenuEntryHeight - 2))
	gtx.Constraints.Min.Y = height
	gtx.Constraints.Max.Y = height
	return layout.Inset{Left: unit.Dp(12), Right: unit.Dp(12)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		label := ui.ColorLabel(b.style.Palette.Muted, ui.Label(b.style, "No items available"))
		return layout.W.Layout(gtx, label.Layout)
	})
}

func fill(gtx layout.Context, background color.NRGBA) {
	defer clip.Rect(image.Rectangle{Max: gtx.Constraints.Min}).Push(gtx.Ops).Pop()
	paint.ColorOp{Color: background}.Add(gtx.Ops)
	paint.PaintOp{}.Add(gtx.Ops)
}
