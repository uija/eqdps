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
}

// Item is one actionable entry in a Menu.
type Item struct {
	label   string
	click   widget.Clickable
	action  func()
	enabled bool
}

// NewBar returns an empty application menu bar.
func NewBar(style *ui.Style, title string) *Bar {
	return &Bar{
		style:     style,
		title:     title,
		barHeight: unit.Dp(38),
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

// Update processes menu input events.
func (b *Bar) Update(gtx layout.Context) {
	for _, entry := range b.entries {
		if entry.click.Clicked(gtx) {
			if entry.dropdown {
				if b.active == entry {
					b.active = nil
				} else {
					b.active = entry
				}
			} else {
				b.active = nil
				if entry.action != nil {
					entry.action()
				}
			}
		}

		for _, item := range entry.items {
			if !item.enabled || !item.click.Clicked(gtx) {
				continue
			}
			b.active = nil
			if item.action != nil {
				item.action()
			}
		}
	}
	if b.backdrop.Clicked(gtx) {
		b.active = nil
	}
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
		label := material.Label(b.style.Theme, unit.Sp(14), b.title)
		label.Color = b.style.Palette.Muted
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

	return layout.Stack{}.Layout(gtx,
		layout.Expanded(b.layoutBackdrop),
		layout.Stacked(b.layoutDropdown),
	)
}

func (b *Bar) layoutBackdrop(gtx layout.Context) layout.Dimensions {
	return layout.Inset{Top: b.barHeight}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Min = gtx.Constraints.Max
		return b.backdrop.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Dimensions{Size: gtx.Constraints.Max}
		})
	})
}

func (b *Bar) layoutDropdown(gtx layout.Context) layout.Dimensions {
	offset := op.Offset(image.Pt(b.active.anchorX, gtx.Dp(b.barHeight))).Push(gtx.Ops)
	defer offset.Pop()

	width := gtx.Dp(unit.Dp(210))
	gtx.Constraints.Min.X = width
	gtx.Constraints.Max.X = width
	itemCount := max(1, len(b.active.items))
	height := gtx.Dp(unit.Dp(36 * itemCount))
	gtx.Constraints.Min.Y = height
	gtx.Constraints.Max.Y = height
	fill(gtx, b.style.Palette.Panel)

	if len(b.active.items) == 0 {
		return b.layoutEmptyItem(gtx)
	}

	children := make([]layout.FlexChild, 0, len(b.active.items))
	for _, item := range b.active.items {
		item := item
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
			label := material.Label(b.style.Theme, unit.Sp(15), entry.label)
			label.Color = b.style.Palette.Text
			return layout.Center.Layout(gtx, label.Layout)
		})
	})
}

func (b *Bar) layoutItem(gtx layout.Context, item *Item) layout.Dimensions {
	height := gtx.Dp(unit.Dp(36))
	gtx.Constraints.Min.Y = height
	gtx.Constraints.Max.Y = height
	return item.click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		if item.enabled && item.click.Hovered() {
			fill(gtx, b.style.Palette.Hover)
		}
		return layout.Inset{Left: unit.Dp(12), Right: unit.Dp(12)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			label := material.Label(b.style.Theme, unit.Sp(15), item.label)
			if item.enabled {
				label.Color = b.style.Palette.Text
			} else {
				label.Color = b.style.Palette.Muted
			}
			return layout.W.Layout(gtx, label.Layout)
		})
	})
}

func (b *Bar) layoutEmptyItem(gtx layout.Context) layout.Dimensions {
	height := gtx.Dp(unit.Dp(36))
	gtx.Constraints.Min.Y = height
	gtx.Constraints.Max.Y = height
	return layout.Inset{Left: unit.Dp(12), Right: unit.Dp(12)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		label := material.Label(b.style.Theme, unit.Sp(15), "No items available")
		label.Color = b.style.Palette.Muted
		return layout.W.Layout(gtx, label.Layout)
	})
}

func fill(gtx layout.Context, background color.NRGBA) {
	defer clip.Rect(image.Rectangle{Max: gtx.Constraints.Min}).Push(gtx.Ops).Pop()
	paint.ColorOp{Color: background}.Add(gtx.Ops)
	paint.PaintOp{}.Add(gtx.Ops)
}
