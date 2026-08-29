package form

import (
	"image"
	"slices"

	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
	appui "github.com/uija/eqdps/internal/ui"
	"golang.org/x/exp/shiny/materialdesign/icons"
)

const (
	selectBoxRowHeight = unit.Dp(42)
	selectBoxMaxHeight = unit.Dp(230)
)

var (
	selectBoxArrowDown = mustSelectBoxIcon(icons.NavigationArrowDropDown)
	selectBoxArrowUp   = mustSelectBoxIcon(icons.NavigationArrowDropUp)
)

// SelectBox is a reusable dropdown selector. Create one with NewSelectBox.
//
// Call Update during the form's input phase before laying out any state that
// depends on the selection. Layout renders the control and defers its dropdown
// so it appears above later content.
type SelectBox struct {
	button  widget.Clickable
	list    widget.List
	choices []widget.Clickable
	options []string

	selected    int
	highlighted int
	open        bool
	changed     bool
}

// NewSelectBox creates a selector with the supplied options and selection.
func NewSelectBox(options []string, selected int) *SelectBox {
	s := new(SelectBox)
	s.list.Axis = layout.Vertical
	s.SetOptions(options)
	s.SetSelected(selected)
	return s
}

// SetOptions replaces the available labels. The current selection is clamped
// to the new option range.
func (s *SelectBox) SetOptions(options []string) {
	if slices.Equal(s.options, options) {
		return
	}
	s.options = slices.Clone(options)
	s.choices = make([]widget.Clickable, len(options))
	s.selected = clampSelectIndex(s.selected, len(options))
	s.highlighted = clampSelectIndex(s.highlighted, len(options))
}

// Options returns a copy of the option labels.
func (s *SelectBox) Options() []string {
	return slices.Clone(s.options)
}

// SetSelected changes the selected option without reporting a user change.
func (s *SelectBox) SetSelected(index int) {
	s.selected = clampSelectIndex(index, len(s.options))
	s.highlighted = s.selected
}

// Select selects the first option exactly matching value. It returns false and
// leaves the current selection unchanged when value is not present.
func (s *SelectBox) Select(value string) bool {
	for index, option := range s.options {
		if option == value {
			s.SetSelected(index)
			return true
		}
	}
	return false
}

// Selected returns the selected option index, or -1 when there are no options.
func (s *SelectBox) Selected() int {
	return s.selected
}

// Value returns the selected label, or an empty string when there is no
// selection.
func (s *SelectBox) Value() string {
	if s.selected < 0 || s.selected >= len(s.options) {
		return ""
	}
	return s.options[s.selected]
}

// Changed reports and clears whether the user selected a different option.
func (s *SelectBox) Changed() bool {
	changed := s.changed
	s.changed = false
	return changed
}

// Open reports whether the option popup is open.
func (s *SelectBox) Open() bool {
	return s.open
}

// Close dismisses the option popup.
func (s *SelectBox) Close() {
	s.open = false
	s.highlighted = s.selected
}

// FocusTag returns the Gio tag representing this selector in a Form.
func (s *SelectBox) FocusTag() event.Tag {
	return &s.button
}

// Update consumes input and applies selection changes. Call it once per frame
// before reading Changed and before laying out state that depends on Value.
func (s *SelectBox) Update(gtx layout.Context) {
	s.updateKeyboard(gtx)

	for index := range s.choices {
		if !s.choices[index].Clicked(gtx) {
			continue
		}
		if index != s.selected {
			s.selected = index
			s.changed = true
		}
		s.Close()
		gtx.Execute(key.FocusCmd{Tag: s.FocusTag()})
	}

	if s.button.Clicked(gtx) {
		gtx.Execute(key.FocusCmd{Tag: s.FocusTag()})
		if s.open {
			s.Close()
		} else {
			s.openPopup()
		}
	}
}

func (s *SelectBox) Layout(style *appui.Style, gtx layout.Context, width unit.Dp) layout.Dimensions {
	pixelWidth := min(gtx.Dp(width), gtx.Constraints.Max.X)
	gtx.Constraints.Min.X = pixelWidth
	gtx.Constraints.Max.X = pixelWidth

	dimensions := (widget.Border{
		Color: style.Palette.Border,
		Width: unit.Dp(1),
	}).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return s.layoutControl(style, gtx)
	})

	if s.open && len(s.options) > 0 {
		s.deferPopup(style, gtx, dimensions.Size)
	}

	return dimensions
}

// Layout renders the current selector state and, while open, its anchored
// option popup. It does not consume input or change the selection.
func (s *SelectBox) LayoutNoBorder(style *appui.Style, gtx layout.Context, width unit.Dp) layout.Dimensions {
	pixelWidth := min(gtx.Dp(width), gtx.Constraints.Max.X)
	gtx.Constraints.Min.X = pixelWidth
	gtx.Constraints.Max.X = pixelWidth
	dimensions := s.layoutControl(style, gtx)
	if s.open && len(s.options) > 0 {
		s.deferPopup(style, gtx, dimensions.Size)
	}
	return dimensions
}

func (s *SelectBox) layoutControl(style *appui.Style, gtx layout.Context) layout.Dimensions {
	return s.button.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		pointer.CursorPointer.Add(gtx.Ops)
		background := style.Palette.Panel
		if s.open || s.button.Hovered() || gtx.Focused(s.FocusTag()) {
			background = style.Palette.Hover
		}
		return appui.ColoredRow(gtx, background, func(gtx layout.Context) layout.Dimensions {
			return layout.UniformInset(unit.Dp(9)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
						label := material.Label(style.Theme, appui.Sp(15), s.Value())
						label.Color = style.Palette.Text
						return label.Layout(gtx)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						arrow := selectBoxArrowDown
						if s.open {
							arrow = selectBoxArrowUp
						}
						size := gtx.Dp(unit.Dp(20))
						gtx.Constraints = layout.Exact(image.Pt(size, size))
						return arrow.Layout(gtx, style.Palette.Accent)
					}),
				)
			})
		})
	})
}

func (s *SelectBox) deferPopup(style *appui.Style, gtx layout.Context, anchor image.Point) {
	macro := op.Record(gtx.Ops)
	offset := op.Offset(image.Pt(0, anchor.Y)).Push(gtx.Ops)
	popup := gtx
	popup.Constraints.Min = image.Pt(anchor.X, 0)
	popup.Constraints.Max = image.Pt(anchor.X, gtx.Dp(selectBoxMaxHeight))
	s.layoutPopup(style, popup)
	offset.Pop()
	op.Defer(gtx.Ops, macro.Stop())
}

func (s *SelectBox) layoutPopup(style *appui.Style, gtx layout.Context) layout.Dimensions {
	rowHeight := gtx.Dp(selectBoxRowHeight)
	height := min(rowHeight*len(s.options), gtx.Dp(selectBoxMaxHeight))
	gtx.Constraints.Min.Y = height
	gtx.Constraints.Max.Y = height
	minimum := gtx.Constraints.Min

	return layout.Stack{}.Layout(gtx,
		layout.Expanded(func(gtx layout.Context) layout.Dimensions {
			appui.FillOverlay(gtx, style.Palette.Window, style.Palette.Accent)
			return layout.Dimensions{Size: gtx.Constraints.Min}
		}),
		layout.Stacked(func(gtx layout.Context) layout.Dimensions {
			// Stack clears minimum constraints for Stacked children. Restore
			// the popup minimum so the list establishes at least the anchor's
			// width for the background layer.
			gtx.Constraints.Min = minimum
			return layout.UniformInset(unit.Dp(1)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				list := material.List(style.Theme, &s.list)
				list.AnchorStrategy = material.Occupy
				list.Indicator.Color = style.Palette.Muted
				list.Indicator.HoverColor = style.Palette.Text
				return list.Layout(gtx, len(s.options), func(gtx layout.Context, index int) layout.Dimensions {
					gtx.Constraints.Min.Y = rowHeight
					gtx.Constraints.Max.Y = rowHeight
					return s.layoutOption(style, gtx, index)
				})
			})
		}),
	)
}

func (s *SelectBox) layoutOption(style *appui.Style, gtx layout.Context, index int) layout.Dimensions {
	return s.choices[index].Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		pointer.CursorPointer.Add(gtx.Ops)
		background := style.Palette.Panel
		foreground := style.Palette.Text
		if index == s.highlighted {
			background = style.Palette.Hover
			foreground = style.Palette.Accent
		} else if s.choices[index].Hovered() {
			background = style.Palette.Hover
		}

		return layout.Background{}.Layout(
			gtx,
			func(gtx layout.Context) layout.Dimensions {
				appui.Fill(gtx, background)
				return layout.Dimensions{Size: gtx.Constraints.Min}
			},
			func(gtx layout.Context) layout.Dimensions {
				return layout.UniformInset(unit.Dp(9)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					label := material.Label(style.Theme, appui.Sp(15), s.options[index])
					label.Color = foreground
					return label.Layout(gtx)
				})
			},
		)
	})
}

func (s *SelectBox) updateKeyboard(gtx layout.Context) {
	for {
		event, ok := gtx.Event(
			key.FocusFilter{Target: s.FocusTag()},
			key.Filter{Focus: s.FocusTag(), Name: key.NameUpArrow},
			key.Filter{Focus: s.FocusTag(), Name: key.NameDownArrow},
			key.Filter{Focus: s.FocusTag(), Name: key.NameEnter},
			key.Filter{Focus: s.FocusTag(), Name: key.NameReturn},
			key.Filter{Focus: s.FocusTag(), Name: key.NameEscape},
		)
		if !ok {
			return
		}
		switch event := event.(type) {
		case key.FocusEvent:
			if !event.Focus {
				s.Close()
			}
		case key.Event:
			if event.State != key.Press {
				continue
			}
			switch event.Name {
			case key.NameUpArrow:
				if s.open {
					s.moveHighlight(-1)
				} else {
					s.openPopup()
				}
			case key.NameDownArrow:
				if s.open {
					s.moveHighlight(1)
				} else {
					s.openPopup()
				}
			case key.NameEnter, key.NameReturn:
				s.commitHighlight()
			case key.NameEscape:
				s.Close()
			}
		}
	}
}

func (s *SelectBox) openPopup() {
	if len(s.options) == 0 {
		return
	}
	s.open = true
	s.highlighted = s.selected
	if s.highlighted < 0 {
		s.highlighted = 0
	}
	s.scrollToHighlight()
}

func (s *SelectBox) moveHighlight(direction int) {
	if len(s.options) == 0 || direction == 0 {
		return
	}
	if s.highlighted < 0 {
		s.highlighted = 0
	} else {
		s.highlighted = min(max(s.highlighted+direction, 0), len(s.options)-1)
	}
	s.scrollToHighlight()
}

func (s *SelectBox) commitHighlight() {
	if !s.open || s.highlighted < 0 || s.highlighted >= len(s.options) {
		return
	}
	if s.highlighted != s.selected {
		s.selected = s.highlighted
		s.changed = true
	}
	s.Close()
}

func (s *SelectBox) scrollToHighlight() {
	if s.highlighted >= 0 {
		s.list.ScrollTo(max(s.highlighted-2, 0))
	}
}

func clampSelectIndex(index, length int) int {
	if length == 0 {
		return -1
	}
	return min(max(index, 0), length-1)
}

func mustSelectBoxIcon(data []byte) *widget.Icon {
	icon, err := widget.NewIcon(data)
	if err != nil {
		panic("select box icon could not be loaded")
	}
	return icon
}
