package form

import (
	"reflect"
	"strings"
	"testing"
)

func TestRegistrationValidatesIdentity(t *testing.T) {
	form := New()
	if err := form.addField(&field{id: "one", tag: new(int)}); err != nil {
		t.Fatal(err)
	}
	if err := form.addField(&field{id: "one", tag: new(int)}); err == nil || !strings.Contains(err.Error(), "duplicate") {
		t.Fatalf("duplicate ID error = %v", err)
	}
	tag := new(int)
	if err := form.addField(&field{id: "two", tag: tag}); err != nil {
		t.Fatal(err)
	}
	if err := form.addField(&field{id: "three", tag: tag}); err == nil || !strings.Contains(err.Error(), "share focus tag") {
		t.Fatalf("duplicate tag error = %v", err)
	}
}

func TestRuntimeStateCanBeReplaced(t *testing.T) {
	form := New()
	if err := form.addField(&field{id: "one", tag: new(int)}); err != nil {
		t.Fatal(err)
	}

	if !form.IsVisible("one") || !form.IsEnabled("one") {
		t.Fatal("new control is not active")
	}
	if err := form.SetVisible("one", false); err != nil {
		t.Fatal(err)
	}
	if form.IsVisible("one") {
		t.Fatal("fixed visibility was not applied")
	}
	visible := true
	if err := form.SetVisibleFunc("one", func() bool { return visible }); err != nil {
		t.Fatal(err)
	}
	if !form.IsVisible("one") {
		t.Fatal("visibility function was not applied")
	}
	visible = false
	if form.IsVisible("one") {
		t.Fatal("visibility function was not evaluated again")
	}
	if err := form.SetVisible("one", true); err != nil {
		t.Fatal(err)
	}
	if !form.IsVisible("one") {
		t.Fatal("fixed visibility did not replace the function")
	}
}

func TestFocusCallbacksCanBeReplaced(t *testing.T) {
	form := New()
	var calls []string
	for _, id := range []string{"one", "two"} {
		if err := form.addField(&field{id: id, tag: new(int)}); err != nil {
			t.Fatal(err)
		}
	}
	if err := form.SetOnFocus("one", func() { calls = append(calls, "focus one") }); err != nil {
		t.Fatal(err)
	}
	if err := form.SetOnBlur("one", func() { calls = append(calls, "old blur") }); err != nil {
		t.Fatal(err)
	}
	form.transition("one")
	if err := form.SetOnBlur("one", func() { calls = append(calls, "new blur") }); err != nil {
		t.Fatal(err)
	}
	form.transition("two")

	want := []string{"focus one", "new blur"}
	if !reflect.DeepEqual(calls, want) {
		t.Fatalf("callbacks = %v, want %v", calls, want)
	}
}

func TestTabOrderSkipsInactiveControls(t *testing.T) {
	form := New()
	for _, id := range []string{"one", "hidden", "disabled", "four"} {
		if err := form.addField(&field{id: id, tag: new(int)}); err != nil {
			t.Fatal(err)
		}
	}
	if err := form.SetVisible("hidden", false); err != nil {
		t.Fatal(err)
	}
	if err := form.SetEnabled("disabled", false); err != nil {
		t.Fatal(err)
	}

	if got := form.nextActive(0, 1); got != 3 {
		t.Fatalf("forward index = %d, want 3", got)
	}
	if got := form.nextActive(0, -1); got != 3 {
		t.Fatalf("wrapped reverse index = %d, want 3", got)
	}
	form.Wrap = false
	if got := form.nextActive(0, -1); got != -1 {
		t.Fatalf("non-wrapped reverse index = %d, want -1", got)
	}
}

func TestSelectBoxClosesOnBlur(t *testing.T) {
	selector := NewSelectBox([]string{"one"}, 0)
	selector.open = true
	form := New()
	if err := form.AddSelectBox("selector", selector); err != nil {
		t.Fatal(err)
	}
	form.transition("selector")
	form.transition("")
	if selector.Open() {
		t.Fatal("selector stayed open after losing form focus")
	}
}
