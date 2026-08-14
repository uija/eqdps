package form

import "testing"

func TestSelectBoxSelectionAndOptions(t *testing.T) {
	selector := NewSelectBox([]string{"One", "Two", "Three"}, 1)
	if selector.Selected() != 1 || selector.Value() != "Two" {
		t.Fatalf("initial selection = %d %q", selector.Selected(), selector.Value())
	}

	selector.SetSelected(99)
	if selector.Selected() != 2 || selector.Value() != "Three" {
		t.Fatalf("clamped selection = %d %q", selector.Selected(), selector.Value())
	}

	selector.SetOptions([]string{"Only"})
	if selector.Selected() != 0 || selector.Value() != "Only" {
		t.Fatalf("selection after options change = %d %q", selector.Selected(), selector.Value())
	}
	if selector.Changed() {
		t.Fatal("programmatic selection reported a user change")
	}
}

func TestSelectBoxEmptyOptions(t *testing.T) {
	selector := NewSelectBox(nil, 0)
	if selector.Selected() != -1 || selector.Value() != "" {
		t.Fatalf("empty selection = %d %q", selector.Selected(), selector.Value())
	}
}

func TestSelectBoxSelectsByValue(t *testing.T) {
	selector := NewSelectBox([]string{"hasso", "humpa", "dengel"}, 0)

	if !selector.Select("humpa") {
		t.Fatal("existing value was not found")
	}
	if selector.Selected() != 1 || selector.Value() != "humpa" {
		t.Fatalf("selection = %d %q", selector.Selected(), selector.Value())
	}
	if selector.Select("missing") {
		t.Fatal("missing value was reported as found")
	}
	if selector.Selected() != 1 {
		t.Fatalf("missing value changed selection to %d", selector.Selected())
	}
	if selector.Changed() {
		t.Fatal("programmatic value selection reported a user change")
	}
}

func TestSelectBoxKeyboardHighlightAndCommit(t *testing.T) {
	selector := NewSelectBox([]string{"one", "two", "three"}, 1)
	selector.openPopup()
	if !selector.Open() || selector.highlighted != 1 {
		t.Fatalf("opened state = %t, highlighted = %d", selector.Open(), selector.highlighted)
	}

	selector.moveHighlight(1)
	if selector.highlighted != 2 || selector.Selected() != 1 {
		t.Fatalf("navigation changed highlight=%d selection=%d", selector.highlighted, selector.Selected())
	}
	selector.commitHighlight()
	if selector.Open() || selector.Selected() != 2 || !selector.Changed() {
		t.Fatalf("commit open=%t selection=%d", selector.Open(), selector.Selected())
	}
}

func TestSelectBoxCloseKeepsSelection(t *testing.T) {
	selector := NewSelectBox([]string{"one", "two", "three"}, 1)
	selector.openPopup()
	selector.moveHighlight(-1)
	selector.Close()

	if selector.Open() || selector.Selected() != 1 || selector.highlighted != 1 {
		t.Fatalf("close open=%t selection=%d highlight=%d", selector.Open(), selector.Selected(), selector.highlighted)
	}
	if selector.Changed() {
		t.Fatal("closing an uncommitted highlight reported a selection change")
	}
}
