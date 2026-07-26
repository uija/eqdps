package menu

import (
	"testing"

	"gioui.org/layout"
	"gioui.org/widget/material"
	"github.com/uija/eqdps/internal/ui"
)

func TestMenuItemInvokesActionAndClosesMenu(t *testing.T) {
	bar := NewBar(&ui.Style{Theme: material.NewTheme()}, "")
	menu := bar.AddMenu("File")
	called := false
	item := menu.AddItem("Action", func() {
		called = true
	})

	menu.click.Click()
	bar.Update(layout.Context{})
	if bar.active != menu {
		t.Fatal("menu did not open")
	}

	item.click.Click()
	bar.Update(layout.Context{})
	if !called {
		t.Fatal("menu item action was not called")
	}
	if bar.active != nil {
		t.Fatal("menu remained open after item activation")
	}
}

func TestMenuBarActionInvokesDirectCallback(t *testing.T) {
	bar := NewBar(&ui.Style{Theme: material.NewTheme()}, "")
	called := false
	bar.AddAction("Help", func() {
		called = true
	})

	bar.entries[0].click.Click()
	bar.Update(layout.Context{})
	if !called {
		t.Fatal("menu bar action was not called")
	}
}

func TestMenuBackdropClosesOpenMenu(t *testing.T) {
	bar := NewBar(&ui.Style{Theme: material.NewTheme()}, "")
	menu := bar.AddMenu("File")

	menu.click.Click()
	bar.Update(layout.Context{})
	if bar.active != menu {
		t.Fatal("menu did not open")
	}

	bar.backdrop.Click()
	bar.Update(layout.Context{})
	if bar.active != nil {
		t.Fatal("menu remained open after backdrop click")
	}
}
