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

func TestSubmenuItemInvokesActionAndClosesMenu(t *testing.T) {
	bar := NewBar(&ui.Style{Theme: material.NewTheme()}, "")
	fileMenu := bar.AddMenu("File")
	recentMenu := fileMenu.AddMenu("Recent")
	called := false
	item := recentMenu.AddItem("example.txt", func() {
		called = true
	})

	fileMenu.click.Click()
	bar.Update(layout.Context{})
	fileMenu.items[0].click.Click()
	bar.Update(layout.Context{})
	if fileMenu.openItem != fileMenu.items[0] {
		t.Fatal("submenu did not open")
	}

	item.click.Click()
	bar.Update(layout.Context{})
	if !called {
		t.Fatal("submenu action was not called")
	}
	if bar.active != nil {
		t.Fatal("menu remained open after submenu action")
	}
}

func TestMenuClearRemovesItemsAndOpenSubmenu(t *testing.T) {
	bar := NewBar(&ui.Style{Theme: material.NewTheme()}, "")
	fileMenu := bar.AddMenu("File")
	fileMenu.AddItem("Open", func() {})
	recentMenu := fileMenu.AddMenu("Recent")
	recentMenu.AddItem("example.txt", func() {})
	fileMenu.openItem = fileMenu.items[1]

	fileMenu.Clear()

	if len(fileMenu.items) != 0 {
		t.Fatal("menu items were not removed")
	}
	if fileMenu.openItem != nil {
		t.Fatal("open submenu was not cleared")
	}
}
