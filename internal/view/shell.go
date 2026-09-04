package view

import (
	"log"
	"path/filepath"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"github.com/ncruces/zenity"
	"github.com/uija/eqdps/internal/module"
	"github.com/uija/eqdps/internal/style"
	"github.com/uija/eqdps/internal/ui"
	"github.com/uija/eqdps/internal/ui/menu"
)

type fileResult struct {
	Path  string
	Error error
}

type progress struct {
	title string
	value int64
	max   int64
}

// Shell is the root application view.
type Shell struct {
	Style         *ui.Style
	menuBar       *menu.Bar
	recentLogs    *menu.Menu
	status        string
	help          *helpView
	selectHistory *historySelection
	preferences   *Preferences
	context       *module.Context

	progress *progress

	invalidateFunc   func()
	fileSelectResult chan fileResult
	progressUpdate   chan progress

	settingsClick widget.Clickable
}

// NewShell constructs the root application view.
func NewShell(context *module.Context, closeWindow func(), invalidate func()) *Shell {
	ui.Init()

	style.Style.OriginalShaper = style.Style.Theme.Shaper
	style.Style.OriginalFace = style.Style.Theme.Face
	style.Style.DefaultPalette = style.Style.Palette
	if context.Config != nil && context.Config.UIConfig.Palette != nil {
		style.Style.Palette = *context.Config.UIConfig.Palette
	}

	style.Style.Theme.Palette.Bg = style.Style.Palette.Window
	style.Style.Theme.Palette.Fg = style.Style.Palette.Text

	if context.Config.UIConfig.FontPath != "" {
		if err := style.Style.LoadFont(context.Config.UIConfig.FontPath); err != nil {
			log.Printf("Unable to load userfont. %v", err)
		}
	}

	result := &Shell{
		Style:            &style.Style,
		status:           "Ready",
		help:             newHelpView(&style.Style, context),
		selectHistory:    NewhistorySelection(&style.Style, context.RequestReplay),
		preferences:      NewPreferences(context),
		context:          context,
		invalidateFunc:   invalidate,
		fileSelectResult: make(chan fileResult, 1),
		progressUpdate:   make(chan progress, 1),
	}
	context.RegisterProgressHandler(result.OnProgress)
	context.RegisterLogOpen(result.OnLogOpen)
	result.menuBar = menu.NewBar(&style.Style, "EVERQUEST LEGENDS")
	fileMenu := result.menuBar.AddMenu("File")
	fileMenu.AddItem("Open Log", result.OpenLogfile)
	result.recentLogs = fileMenu.AddMenu("Recent Logs")
	fileMenu.AddItem("Quit", closeWindow)
	viewMenu := result.menuBar.AddMenu("View")
	for _, entry := range context.ModuleNavigationItems {
		if entry.ViewLabel != "" {
			viewMenu.AddItem(entry.ViewLabel, func() {
				context.ActivateModule(entry.ID)
			})
		}
	}
	toolsMenu := result.menuBar.AddMenu("Tools")

	toolsMenu.AddItem("Combat History", func() {
		result.selectHistory.Show()
		result.invalidateFunc()
	})
	if len(context.ToolsMenuItems) > 0 {
		toolsMenu.AddSeparator()
		for _, entry := range context.ToolsMenuItems {
			toolsMenu.AddItem(entry.Name, entry.Action)
		}
	}
	toolsMenu.AddSeparator()
	toolsMenu.AddItem("Preferences", func() {
		result.context.SetMainView(result.preferences.Layout)
	})
	result.menuBar.AddAction("Help", result.help.Open)

	// check if there is a logfile to open
	if context.Config.LastLogfile != "" {
		result.fileSelectResult <- fileResult{Error: nil, Path: context.Config.LastLogfile}
	}

	return result
}

func (s *Shell) OnLogOpen(characterName string, serverName string, filesize int64, path string) bool {
	s.recentLogs.Clear()
	for _, path := range s.context.Config.RecentLogFiles {
		name := filepath.Base(path)
		s.recentLogs.AddItem(name, func() {
			s.fileSelectResult <- fileResult{Path: path, Error: nil}
			s.invalidateFunc()
		})
	}
	return true
}

func (s *Shell) OpenLogfile() {
	go func() {
		path, err := zenity.SelectFile(
			zenity.Title("Open EverQuest logfile"),
			zenity.FileFilters{
				{
					Name:     "Everquest logs",
					Patterns: []string{"eqlog_*_.txt", "*.txt"},
				},
			},
		)
		if err == nil {
			s.context.Config.LastLogfile = path
			if err := s.context.Config.Save(); err != nil {
				log.Printf("Unable to save config. %v", err)
			}
			s.fileSelectResult <- fileResult{Path: path, Error: err}
			s.invalidateFunc()
		}
	}()
}
func (s *Shell) OnProgress(title string, value int64, max int64) {
	update := progress{title: title, value: value, max: max}
	select {
	case s.progressUpdate <- update:
	default:
		select {
		case <-s.progressUpdate:
		default:
		}

		select {
		case s.progressUpdate <- update:
		default:
		}
	}
	s.invalidateFunc()
}

// Layout renders the complete application view.
func (s *Shell) Layout(gtx layout.Context) layout.Dimensions {
	paint.Fill(gtx.Ops, s.Style.Palette.Window)
	s.update(gtx)
	return layout.Stack{}.Layout(gtx,
		layout.Expanded(func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min = gtx.Constraints.Max
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(s.menuBar.LayoutBar),
				layout.Flexed(1,
					func(gtx layout.Context) layout.Dimensions {
						children := make([]layout.FlexChild, 0)
						children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return layout.UniformInset(unit.Dp(8)).Layout(gtx, material.Body1(style.Style.Theme, "").Layout)
						}))
						for idx, i := range s.context.ModuleNavigationItems {
							if i.SidebarLabel != "" {
								children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									link := ui.Link(s.Style, &s.context.ModuleNavigationItems[idx].Click, i.SidebarLabel)
									link.Size = 16
									link.FontWeight = font.SemiBold
									if i.Active {
										link.TextColor = style.Style.Palette.Accent
									} else {
										link.TextColor = style.Style.Palette.Muted
									}
									link.Padding[ui.PADDING_BOTTOM] = 8
									link.Padding[ui.PADDING_TOP] = 16
									link.Padding[ui.PADDING_LEFT] = 16
									link.Padding[ui.PADDING_RIGHT] = 16
									return layout.Inset{Bottom: unit.Dp(4)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
										return link.Layout(gtx)
									})
								}))
							}
						}
						children = append(children, layout.Flexed(1, material.Body1(style.Style.Theme, " ").Layout))
						children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							link := ui.Link(s.Style, &s.settingsClick, "PREF")
							link.Size = 16
							link.FontWeight = font.SemiBold
							link.TextColor = style.Style.Palette.Muted
							link.Padding[ui.PADDING_BOTTOM] = 8
							link.Padding[ui.PADDING_TOP] = 16
							link.Padding[ui.PADDING_LEFT] = 16
							link.Padding[ui.PADDING_RIGHT] = 16
							return layout.Inset{Bottom: unit.Dp(4)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
								return ui.ColoredAccentedRow(gtx, style.Style.Palette.Panel, style.Style.Palette.Accent, false, link.Layout)
							})
						}))

						return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								return ui.ColoredRow(gtx, style.Style.Palette.Panel, func(gtx layout.Context) layout.Dimensions {
									return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
										children...,
									)
								})
							}),
							layout.Flexed(1, s.layoutMain),
						)
					},
				),
				layout.Rigid(s.layoutStatus),
			)
		}),
		layout.Stacked(s.menuBar.LayoutOverlay),
		layout.Expanded(s.help.Layout),
		layout.Expanded(s.selectHistory.Layout),
		layout.Expanded(s.layoutProgressOverlay),
	)
}

func (s *Shell) update(gtx layout.Context) {
	select {
	case result := <-s.fileSelectResult:
		if result.Error != nil {
			log.Printf("Unable to open file %v", result.Error)
			break
		}
		s.context.ParserLogFileOpened(result.Path)
	case p := <-s.progressUpdate:
		if p.value >= p.max {
			s.progress = nil
		} else {
			s.progress = &p
		}
	default:
	}
	for idx := range s.context.ModuleNavigationItems {
		if s.context.ModuleNavigationItems[idx].Click.Clicked(gtx) {
			s.context.ActivateModule(s.context.ModuleNavigationItems[idx].ID)
		}
	}
	if s.settingsClick.Clicked(gtx) {
		s.context.SetMainView(s.preferences.Layout)
	}
	s.context.Update(gtx)
	// check menu items
	s.menuBar.Update(gtx)
	s.selectHistory.Update(gtx)
	s.help.Update(gtx)
	s.preferences.Update(gtx)
}

func (s *Shell) layoutMain(gtx layout.Context) layout.Dimensions {
	return layout.Inset{Left: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return s.context.Layout(s.Style, gtx)
	})
}

func (s *Shell) layoutStatus(gtx layout.Context) layout.Dimensions {
	gtx.Constraints.Min.Y = gtx.Dp(unit.Dp(30))
	gtx.Constraints.Max.Y = gtx.Constraints.Min.Y
	ui.Fill(gtx, s.Style.Palette.Chrome)

	items := s.context.CompactStatusElements(s.Style, gtx)
	if len(items) == 0 {
		items = append(items, layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{}.Layout(gtx, material.Body1(s.Style.Theme, s.status).Layout)
		}))
	}

	return layout.Inset{Left: unit.Dp(14), Right: unit.Dp(14), Top: unit.Dp(4)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{}.Layout(gtx, items...)
	})
}
