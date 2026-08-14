package form

import (
	"testing"

	"gioui.org/widget"
)

func TestControlsRegisterInOrder(t *testing.T) {
	form := New()
	editor := new(widget.Editor)
	checkbox := new(widget.Bool)
	button := new(widget.Clickable)
	selector := NewSelectBox([]string{"one"}, 0)

	if err := form.AddEditor("title", editor, nil); err != nil {
		t.Fatal(err)
	}
	if err := form.AddCheckbox("active", checkbox, nil); err != nil {
		t.Fatal(err)
	}
	if err := form.AddButton("save", button, nil); err != nil {
		t.Fatal(err)
	}
	if err := form.AddSelectBox("choice", selector); err != nil {
		t.Fatal(err)
	}
	want := []string{"title", "active", "save", "choice"}
	for index, id := range want {
		if form.fields[index].id != id {
			t.Fatalf("control %d = %q, want %q", index, form.fields[index].id, id)
		}
	}
}

func TestControlRegistrationRejectsNilWidgets(t *testing.T) {
	form := New()
	if err := form.AddEditor("title", nil, nil); err == nil {
		t.Fatal("nil editor was accepted")
	}
	if err := form.AddCheckbox("active", nil, nil); err == nil {
		t.Fatal("nil checkbox was accepted")
	}
	if err := form.AddButton("save", nil, nil); err == nil {
		t.Fatal("nil button was accepted")
	}
	if err := form.AddSelectBox("choice", nil); err == nil {
		t.Fatal("nil select box was accepted")
	}
}

func TestControlHandlersCanBeReplaced(t *testing.T) {
	form := New()
	if err := form.AddEditor("title", new(widget.Editor), nil); err != nil {
		t.Fatal(err)
	}
	if err := form.AddCheckbox("active", new(widget.Bool), nil); err != nil {
		t.Fatal(err)
	}
	if err := form.AddButton("save", new(widget.Clickable), nil); err != nil {
		t.Fatal(err)
	}

	editorCalled := false
	if err := form.SetEditorEventHandler("title", func(widget.EditorEvent) { editorCalled = true }); err != nil {
		t.Fatal(err)
	}
	form.fields[0].editorHandler.fn(widget.ChangeEvent{})
	if !editorCalled {
		t.Fatal("replacement editor handler was not stored")
	}

	checkboxValue := false
	if err := form.SetCheckboxChangeHandler("active", func(value bool) { checkboxValue = value }); err != nil {
		t.Fatal(err)
	}
	form.fields[1].checkboxHandler.fn(true)
	if !checkboxValue {
		t.Fatal("replacement checkbox handler was not stored")
	}

	buttonCalled := false
	if err := form.SetButtonClickHandler("save", func() { buttonCalled = true }); err != nil {
		t.Fatal(err)
	}
	form.fields[2].buttonHandler.fn()
	if !buttonCalled {
		t.Fatal("replacement button handler was not stored")
	}
}
