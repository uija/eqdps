package module

import (
	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget/material"
	"github.com/uija/eqdps/internal/data"
	"github.com/uija/eqdps/internal/ui"
)

type MenuItem struct {
	Name   string
	Action UIActionFunc
}

type UIActionFunc func()
type OnLogOpenFunc func(characterName string, serverName string, filesize int64)
type OnLogRowFunc func(event *data.LogEvent)

type Context struct {
	currentMainView ui.Widget
	ViewMenuItems   []MenuItem
	onLogOpenFuncs  []OnLogOpenFunc
	onLogRowFuncs   []OnLogRowFunc
	onStatus        []ui.Widget
}

func NewContext() *Context {
	return &Context{
		ViewMenuItems:  make([]MenuItem, 0),
		onLogOpenFuncs: make([]OnLogOpenFunc, 0),
		onLogRowFuncs:  make([]OnLogRowFunc, 0),
		onStatus:       make([]ui.Widget, 0),
	}
}

func (c *Context) AddViewMenuItem(name string, action UIActionFunc) {
	c.ViewMenuItems = append(c.ViewMenuItems, MenuItem{Name: name, Action: action})
}
func (c *Context) OnLogOpen(f OnLogOpenFunc) {
	c.onLogOpenFuncs = append(c.onLogOpenFuncs, f)
}
func (c *Context) OnLogRow(f OnLogRowFunc) {
	c.onLogRowFuncs = append(c.onLogRowFuncs, f)
}
func (c *Context) OnStatus(f ui.Widget) {
	c.onStatus = append(c.onStatus, f)
}
func (c *Context) SetMainView(f ui.Widget) {
	c.currentMainView = f
}

func (c *Context) RegisterModule(m Module) error {
	return m.Init(c)
}

func (c *Context) Layout(style *ui.Style, gtx layout.Context) layout.Dimensions {
	if c.currentMainView != nil {
		return c.currentMainView(style, gtx)
	}
	return layout.Inset{Top: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		label := material.Label(style.Theme, unit.Sp(15), "No modules registered")
		//label.Color = palette.muted
		return label.Layout(gtx)
	})
}
