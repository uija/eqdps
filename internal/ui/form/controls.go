package form

import (
	"fmt"

	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/widget"
)

type editorEventHandler struct {
	fn func(widget.EditorEvent)
}

type checkboxChangeHandler struct {
	fn func(bool)
}

type buttonClickHandler struct {
	fn func()
}

// AddEditor registers an existing Gio editor at the next position in the
// form's Tab order. onEvent receives every editor event and may be nil.
func (f *Form) AddEditor(id string, editor *widget.Editor, onEvent func(widget.EditorEvent)) error {
	if editor == nil {
		return fmt.Errorf("form editor %q is nil", id)
	}
	field := &field{
		id:            id,
		tag:           editor,
		editorHandler: &editorEventHandler{fn: onEvent},
	}
	field.update = func(gtx layout.Context) {
		for {
			event, ok := editor.Update(gtx)
			if !ok {
				return
			}
			if onEvent := field.editorHandler.fn; onEvent != nil {
				onEvent(event)
			}
		}
	}
	return f.addField(field)
}

// AddCheckbox registers an existing Gio Bool at the next position in the
// form's Tab order. onChange receives the new value and may be nil.
func (f *Form) AddCheckbox(id string, checkbox *widget.Bool, onChange func(bool)) error {
	if checkbox == nil {
		return fmt.Errorf("form checkbox %q is nil", id)
	}
	field := &field{
		id:              id,
		tag:             checkbox,
		checkboxHandler: &checkboxChangeHandler{fn: onChange},
	}
	field.update = func(gtx layout.Context) {
		if !checkbox.Update(gtx) {
			return
		}

		// widget.Bool handles Return and Space while focused, but unlike an
		// editor it does not request focus when clicked.
		gtx.Execute(key.FocusCmd{Tag: checkbox})
		if onChange := field.checkboxHandler.fn; onChange != nil {
			onChange(checkbox.Value)
		}
	}
	return f.addField(field)
}

// AddButton registers an existing Gio Clickable at the next position in the
// form's Tab order. onClick is called whenever the button is activated and may
// be nil. Gio's Clickable supplies Return and Space keyboard activation.
func (f *Form) AddButton(id string, button *widget.Clickable, onClick func()) error {
	if button == nil {
		return fmt.Errorf("form button %q is nil", id)
	}
	field := &field{
		id:            id,
		tag:           button,
		buttonHandler: &buttonClickHandler{fn: onClick},
	}
	field.update = func(gtx layout.Context) {
		for button.Clicked(gtx) {
			// Clickable supports keyboard focus but does not request it after a
			// pointer click by itself.
			gtx.Execute(key.FocusCmd{Tag: button})
			if onClick := field.buttonHandler.fn; onClick != nil {
				onClick()
			}
		}
	}
	return f.addField(field)
}

// AddSelectBox registers a SelectBox at the next position in the form's Tab
// order. The form closes its popup whenever it loses form focus.
func (f *Form) AddSelectBox(id string, selector *SelectBox) error {
	if selector == nil {
		return fmt.Errorf("form select box %q is nil", id)
	}
	return f.addField(&field{
		id:             id,
		tag:            selector.FocusTag(),
		update:         selector.Update,
		internalOnBlur: selector.Close,
	})
}

// SetEditorEventHandler replaces the event handler for a registered editor.
func (f *Form) SetEditorEventHandler(id string, onEvent func(widget.EditorEvent)) error {
	field, err := f.requireField(id)
	if err != nil {
		return err
	}
	if field.editorHandler == nil {
		return fmt.Errorf("form control %q is not an editor", id)
	}
	field.editorHandler.fn = onEvent
	return nil
}

// SetCheckboxChangeHandler replaces the change handler for a registered
// checkbox.
func (f *Form) SetCheckboxChangeHandler(id string, onChange func(bool)) error {
	field, err := f.requireField(id)
	if err != nil {
		return err
	}
	if field.checkboxHandler == nil {
		return fmt.Errorf("form control %q is not a checkbox", id)
	}
	field.checkboxHandler.fn = onChange
	return nil
}

// SetButtonClickHandler replaces the click handler for a registered button.
func (f *Form) SetButtonClickHandler(id string, onClick func()) error {
	field, err := f.requireField(id)
	if err != nil {
		return err
	}
	if field.buttonHandler == nil {
		return fmt.Errorf("form control %q is not a button", id)
	}
	field.buttonHandler.fn = onClick
	return nil
}
