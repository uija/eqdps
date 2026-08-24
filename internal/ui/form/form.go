// Package form coordinates focus and input for groups of Gio controls.
package form

import (
	"fmt"
	"image"
	"reflect"

	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/widget"
)

type field struct {
	id     string
	tag    event.Tag
	update func(layout.Context)

	visible     bool
	visibleFunc func() bool
	enabled     bool
	enabledFunc func() bool

	internalOnBlur func()
	onFocus        func()
	onBlur         func()

	editorHandler   *editorEventHandler
	checkboxHandler *checkboxChangeHandler
	buttonHandler   *buttonClickHandler
}

func (f *field) isVisible() bool {
	if f.visibleFunc != nil {
		return f.visibleFunc()
	}
	return f.visible
}

func (f *field) isEnabled() bool {
	if f.enabledFunc != nil {
		return f.enabledFunc()
	}
	return f.enabled
}

func (f *field) active() bool {
	return f.isVisible() && f.isEnabled()
}

// FocusChange describes a transition between logical form controls. An empty
// ID represents focus outside the form.
type FocusChange struct {
	From string
	To   string
}

// Form owns the input and focus registration for an ordered group of Gio
// controls. Controls are added in their desired Tab order.
type Form struct {
	fields []*field

	focusedID string
	change    *FocusChange

	outsideTag   struct{}
	modalBlocker widget.Clickable

	// Wrap controls whether Tab on the last field moves to the first field and
	// Shift-Tab on the first moves to the last. New forms enable wrapping.
	Wrap bool
}

// New creates an empty form with wrapping Tab navigation.
func New() *Form {
	return &Form{Wrap: true}
}

func (f *Form) addField(field *field) error {
	if field.id == "" {
		return fmt.Errorf("form control has no ID")
	}
	if field.tag == nil {
		return fmt.Errorf("form control %q has no focus tag", field.id)
	}
	if !reflect.TypeOf(field.tag).Comparable() {
		return fmt.Errorf("form control %q has a non-comparable focus tag", field.id)
	}
	for _, existing := range f.fields {
		if existing.id == field.id {
			return fmt.Errorf("duplicate form control ID %q", field.id)
		}
		if existing.tag == field.tag {
			return fmt.Errorf("form controls share focus tag for %q", field.id)
		}
	}
	field.visible = true
	field.enabled = true
	f.fields = append(f.fields, field)
	return nil
}

// SetVisible changes whether a control participates in form input and focus.
// It replaces any visibility function previously assigned to the control.
func (f *Form) SetVisible(id string, visible bool) error {
	field, err := f.requireField(id)
	if err != nil {
		return err
	}
	field.visible = visible
	field.visibleFunc = nil
	return nil
}

// SetVisibleFunc supplies a function evaluated whenever the form needs the
// control's current visibility. Passing nil restores fixed visibility.
func (f *Form) SetVisibleFunc(id string, visible func() bool) error {
	field, err := f.requireField(id)
	if err != nil {
		return err
	}
	field.visibleFunc = visible
	return nil
}

// IsVisible reports the current visibility of a registered control. It
// returns false for an unknown ID.
func (f *Form) IsVisible(id string) bool {
	field, ok := f.field(id)
	return ok && field.isVisible()
}

// SetEnabled changes whether a control participates in form input and focus.
// It replaces any enabled function previously assigned to the control.
func (f *Form) SetEnabled(id string, enabled bool) error {
	field, err := f.requireField(id)
	if err != nil {
		return err
	}
	field.enabled = enabled
	field.enabledFunc = nil
	return nil
}

// SetEnabledFunc supplies a function evaluated whenever the form needs the
// control's current enabled state. Passing nil restores fixed enabled state.
func (f *Form) SetEnabledFunc(id string, enabled func() bool) error {
	field, err := f.requireField(id)
	if err != nil {
		return err
	}
	field.enabledFunc = enabled
	return nil
}

// IsEnabled reports the current enabled state of a registered control. It
// returns false for an unknown ID.
func (f *Form) IsEnabled(id string) bool {
	field, ok := f.field(id)
	return ok && field.isEnabled()
}

// SetOnFocus replaces the function called when the control gains form focus.
func (f *Form) SetOnFocus(id string, onFocus func()) error {
	field, err := f.requireField(id)
	if err != nil {
		return err
	}
	field.onFocus = onFocus
	return nil
}

// SetOnBlur replaces the function called when the control loses form focus.
func (f *Form) SetOnBlur(id string, onBlur func()) error {
	field, err := f.requireField(id)
	if err != nil {
		return err
	}
	field.onBlur = onBlur
	return nil
}

// Update processes one form input phase. It must run after the previous
// frame's input layer was registered and before any state-dependent layout.
func (f *Form) Update(gtx layout.Context) {
	for f.modalBlocker.Clicked(gtx) {
	}

	if focused, ok := f.field(f.focusedID); ok && !focused.active() {
		gtx.Execute(key.FocusCmd{Tag: nil})
		f.transition("")
	}

	outsidePress := false
	for {
		event, ok := gtx.Event(pointer.Filter{
			Target: &f.outsideTag,
			Kinds:  pointer.Press,
		})
		if !ok {
			break
		}
		if press, ok := event.(pointer.Event); ok && press.Kind == pointer.Press {
			outsidePress = true
		}
	}
	if outsidePress {
		// A control receiving the same pass-through press may request focus
		// again from its update callback below.
		gtx.Execute(key.FocusCmd{Tag: nil})
	}

	for _, field := range f.fields {
		if field.active() && field.update != nil {
			field.update(gtx)
		}
	}

	focusedIndex := f.focusedIndex(gtx)
	if focusedIndex >= 0 {
		if direction := f.tabDirection(gtx, f.fields[focusedIndex].tag); direction != 0 {
			next := f.nextActive(focusedIndex, direction)
			if next >= 0 {
				gtx.Execute(key.FocusCmd{Tag: f.fields[next].tag})
				focusedIndex = next
			} else {
				gtx.Execute(key.FocusCmd{Tag: nil})
				focusedIndex = -1
			}
		}
	}

	nextID := ""
	if focusedIndex >= 0 {
		nextID = f.fields[focusedIndex].id
	}
	f.transition(nextID)
}

// LayoutInputLayer registers the form-wide pass-through pointer observer used
// to clear focus when the user presses outside form controls. Call it once with
// the full view context before laying out the view's controls.
func (f *Form) LayoutInputLayer(gtx layout.Context) layout.Dimensions {
	defer clip.Rect(image.Rectangle{Max: gtx.Constraints.Max}).Push(gtx.Ops).Pop()
	defer pointer.PassOp{}.Push(gtx.Ops).Pop()
	event.Op(gtx.Ops, &f.outsideTag)
	return layout.Dimensions{Size: gtx.Constraints.Max}
}

// LayoutModalInputLayer registers the form input layer above a full-size
// pointer barrier. Form controls rendered afterward remain interactive, while
// controls beneath the modal form cannot receive its pointer events.
func (f *Form) LayoutModalInputLayer(gtx layout.Context) layout.Dimensions {
	return layout.Stack{}.Layout(gtx,
		layout.Expanded(func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min = gtx.Constraints.Max
			return f.modalBlocker.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Dimensions{Size: gtx.Constraints.Max}
			})
		}),
		layout.Expanded(f.LayoutInputLayer),
	)
}

// Focus requests focus for an active control and reports whether it was found.
func (f *Form) Focus(gtx layout.Context, id string) bool {
	field, ok := f.field(id)
	if !ok || !field.active() {
		return false
	}
	gtx.Execute(key.FocusCmd{Tag: field.tag})
	f.transition(id)
	return true
}

// Blur clears keyboard focus and the form's logical focus.
func (f *Form) Blur(gtx layout.Context) {
	gtx.Execute(key.FocusCmd{Tag: nil})
	f.transition("")
}

// Focused returns the focused control ID, or false when focus is outside the
// form.
func (f *Form) Focused() (string, bool) {
	return f.focusedID, f.focusedID != ""
}

// FocusChanged reports and clears the most recent logical focus transition.
func (f *Form) FocusChanged() (FocusChange, bool) {
	if f.change == nil {
		return FocusChange{}, false
	}
	change := *f.change
	f.change = nil
	return change, true
}

func (f *Form) focusedIndex(gtx layout.Context) int {
	for index, field := range f.fields {
		if field.active() && gtx.Focused(field.tag) {
			return index
		}
	}
	return -1
}

func (f *Form) tabDirection(gtx layout.Context, tag event.Tag) int {
	for {
		event, ok := gtx.Event(key.Filter{
			Focus:    tag,
			Name:     key.NameTab,
			Optional: key.ModShift,
		})
		if !ok {
			return 0
		}
		keyEvent, ok := event.(key.Event)
		if !ok || keyEvent.State != key.Press {
			continue
		}
		if keyEvent.Modifiers.Contain(key.ModShift) {
			return -1
		}
		return 1
	}
}

func (f *Form) nextActive(current, direction int) int {
	if len(f.fields) == 0 || direction == 0 {
		return -1
	}
	for step := 1; step <= len(f.fields); step++ {
		index := current + step*direction
		if f.Wrap {
			index = (index%len(f.fields) + len(f.fields)) % len(f.fields)
		} else if index < 0 || index >= len(f.fields) {
			return -1
		}
		if f.fields[index].active() {
			return index
		}
	}
	return -1
}

func (f *Form) transition(nextID string) {
	if nextID == f.focusedID {
		return
	}
	previousID := f.focusedID
	if previous, ok := f.field(previousID); ok {
		if previous.internalOnBlur != nil {
			previous.internalOnBlur()
		}
		if previous.onBlur != nil {
			previous.onBlur()
		}
	}
	f.focusedID = nextID
	if next, ok := f.field(nextID); ok && next.onFocus != nil {
		next.onFocus()
	}
	f.change = &FocusChange{From: previousID, To: nextID}
}

func (f *Form) field(id string) (*field, bool) {
	for _, field := range f.fields {
		if field.id == id {
			return field, true
		}
	}
	return nil, false
}

func (f *Form) requireField(id string) (*field, error) {
	field, ok := f.field(id)
	if !ok {
		return nil, fmt.Errorf("unknown form control %q", id)
	}
	return field, nil
}
