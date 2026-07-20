package main

import (
	"fmt"
	"image"
	"image/color"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"gioui.org/app"
	"gioui.org/font"
	"gioui.org/io/pointer"
	"gioui.org/io/system"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"github.com/ncruces/zenity"
	"github.com/uija/eqdps/internal/skyquest"
	"github.com/uija/eqdps/internal/xp"
)

var palette = struct {
	window, chrome, rail, panel, panelAlt color.NRGBA
	line, text, muted, accent, success    color.NRGBA
}{
	window:   color.NRGBA{R: 18, G: 20, B: 22, A: 255},
	chrome:   color.NRGBA{R: 27, G: 30, B: 33, A: 255},
	rail:     color.NRGBA{R: 23, G: 26, B: 29, A: 255},
	panel:    color.NRGBA{R: 31, G: 34, B: 37, A: 255},
	panelAlt: color.NRGBA{R: 36, G: 39, B: 42, A: 255},
	line:     color.NRGBA{R: 57, G: 61, B: 65, A: 255},
	text:     color.NRGBA{R: 225, G: 226, B: 222, A: 255},
	muted:    color.NRGBA{R: 150, G: 154, B: 151, A: 255},
	accent:   color.NRGBA{R: 190, G: 155, B: 74, A: 255},
	success:  color.NRGBA{R: 109, G: 178, B: 124, A: 255},
}

type shell struct {
	theme           *material.Theme
	fightList       widget.List
	workspace       int
	activeMenu      int
	activeSub       int
	treeClicks      map[string]*widget.Clickable
	expanded        map[string]bool
	window          *app.Window
	settings        guiSettings
	currentLog      string
	statusText      string
	fileChosen      chan fileChoice
	combatUpdates   chan combatUpdate
	logCancel       chan struct{}
	loading         bool
	loadBytes       int64
	loadTotal       int64
	loadLines       int
	loadingTitle    string
	operationCancel widget.Clickable
	overlay         *combatOverlay
	overlayClosed   chan *combatOverlay
	waylandHelp     bool
	openAfterHelp   bool
	rememberHelp    bool
	helpClose       widget.Clickable
	aboutOpen       bool
	aboutClose      widget.Clickable
	mainScale       widget.Float
	dpsScale        widget.Float
	dpsOpacity      widget.Float
	prefsDirty      bool
	xpSnapshot      xp.Snapshot
	parserState     string
	allFights       []fakeFightSection
	fightFilter     string
	filterEditor    widget.Editor
	filterClear     widget.Clickable
	skyDatabase     skyquest.Database
	skyProgress     []skyquest.QuestProgress
	skyInventory    map[string]int
	skyRows         []skyRow
	skyIdentity     string
	skyMessage      string
	skyHideEmpty    bool
	skyHideClick    widget.Clickable
	skyStatusClick  widget.Clickable
	skyNoticeText   string
	skyNoticeUntil  time.Time
	skyList         widget.List
	skyTracker      *skyquest.PersistentTracker
	skyMu           sync.RWMutex
	skyUpdates      chan skyAsyncUpdate
	skyCancel       chan struct{}
	skySetupOpen    bool
	skyDenied       bool
	skyAllow        widget.Clickable
	skyDeny         widget.Clickable
	skyLoading      bool
	skyLoadBytes    int64
	skyLoadTotal    int64
	skyLoadLines    int
	skyLoadTitle    string
	fights          []fakeFightSection
	menus           []menu
	rail            []railItem
}

type menu struct {
	name  string
	click widget.Clickable
	items []menuItem
}

type menuItem struct {
	name    string
	detail  string
	click   widget.Clickable
	enabled bool
	action  string
	back    time.Duration
	path    string
	items   []menuItem
}

type fileChoice struct {
	path string
	back time.Duration
	err  error
}

type railItem struct {
	short string
	name  string
	click widget.Clickable
}

type fakeCombatant struct {
	name              string
	damage, dps, sdps int
	hits, crits       int
	active            string
	accent            bool
	details           []fakeBreakdown
}

type fakeBreakdown struct {
	name              string
	damage, dps, sdps int
	hits, crits       int
	active            string
	children          []fakeBreakdown
}

type fakeFightSection struct {
	name, status, duration string
	current                bool
	started                time.Time
	lastYouIntentional     time.Time
	combatants             []fakeCombatant
}

func main() {
	go func() {
		window := new(app.Window)
		window.Option(
			app.Title("eqdps"),
			app.Size(unit.Dp(1050), unit.Dp(700)),
			app.MinSize(unit.Dp(720), unit.Dp(460)),
		)
		if err := run(window); err != nil {
			log.Print(err)
		}
		os.Exit(0)
	}()
	app.Main()
}

func run(window *app.Window) error {
	ui := newShell(window)
	var ops op.Ops
	for {
		switch event := window.Event().(type) {
		case app.DestroyEvent:
			return event.Err
		case app.FrameEvent:
			gtx := app.NewContext(&ops, event)
			ui.layout(gtx)
			event.Frame(gtx.Ops)
		}
	}
}

func newShell(window *app.Window) *shell {
	theme := material.NewTheme()
	theme.Palette.Fg = palette.text
	theme.Palette.Bg = palette.window
	settings, settingsErr := loadSettings()
	skyDatabase, skyDatabaseErr := skyquest.LoadDatabase()
	settings.normalize()
	theme.TextSize = unit.Sp(16 * settings.MainFontScale)
	statusText := "No logfile selected"
	currentLog := ""
	if settingsErr != nil {
		statusText = "Could not read saved GUI settings"
	} else if settings.LastLogfile != "" {
		if _, err := os.Stat(settings.LastLogfile); err == nil {
			currentLog = settings.LastLogfile
			statusText = "Reopened last logfile · live only"
		} else {
			statusText = "Last logfile is no longer available"
		}
	}
	ranges := historyRangeItems("open")
	recents := recentMenuItems(settings)
	result := &shell{
		theme:         theme,
		fightList:     widget.List{List: layout.List{Axis: layout.Vertical}},
		activeMenu:    -1,
		activeSub:     -1,
		window:        window,
		settings:      settings,
		currentLog:    currentLog,
		statusText:    statusText,
		fileChosen:    make(chan fileChoice, 1),
		combatUpdates: make(chan combatUpdate, 1),
		overlayClosed: make(chan *combatOverlay, 1),
		skyDatabase:   skyDatabase,
		skyInventory:  make(map[string]int),
		skyList:       widget.List{List: layout.List{Axis: layout.Vertical}},
		skyUpdates:    make(chan skyAsyncUpdate, 1),
		treeClicks:    make(map[string]*widget.Clickable),
		expanded:      make(map[string]bool),
		menus: []menu{
			{name: "File", items: []menuItem{{name: "Open logfile", detail: "Choose a file and initial history", enabled: true, items: ranges}, {name: "Recent logfiles", enabled: len(recents) > 0, items: recents}, {name: "Exit", enabled: true, action: "exit"}}},
			{name: "Combat", items: []menuItem{{name: "Current fight", enabled: true, action: "current"}, {name: "Load history", enabled: currentLog != "", items: historyRangeItems("reload")}, {name: "Filter…", enabled: true, action: "filter"}, {name: "Reset session", enabled: currentLog != "", action: "reset"}}},
			{name: "View", items: []menuItem{{name: "Damage meter", enabled: true, action: "damage"}, {name: "Plane of Sky", enabled: true, action: "sky"}, {name: "Show DPS overlay", detail: "Toggle compact current-fight window", enabled: true, action: "overlay"}}},
			{name: "Tools", items: []menuItem{{name: "Preferences…", enabled: true, action: "preferences"}}},
			{name: "Help", items: []menuItem{{name: "Wayland overlay setup…", enabled: true, action: "wayland-help"}, {name: "About eqdps", enabled: true, action: "about"}}},
		},
		rail: []railItem{{short: "DPS", name: "Combat Log"}, {short: "SKY", name: "Plane of Sky"}, {short: "SET", name: "Settings"}},
	}
	result.filterEditor.SingleLine = true
	result.mainScale.Value = settingToSlider(settings.MainFontScale, .75, 1.5)
	result.dpsScale.Value = settingToSlider(settings.DPSFontScale, .5, 1.5)
	result.dpsOpacity.Value = settingToSlider(settings.DPSOpacity, .35, 1)
	if skyDatabaseErr != nil {
		result.skyMessage = skyDatabaseErr.Error()
	} else {
		result.loadSkyState(currentLog)
	}
	if currentLog != "" {
		result.loadLog(currentLog, 0)
	}
	if settings.OverlayVisible {
		result.menus[2].items[2].name = "Hide DPS overlay"
		if result.showWaylandHelpOnce() {
			result.openAfterHelp = true
		} else {
			result.openOverlay()
		}
	}
	return result
}

func (s *shell) layout(gtx layout.Context) layout.Dimensions {
	paint.Fill(gtx.Ops, palette.window)
	s.update(gtx)
	return layout.Stack{}.Layout(gtx,
		layout.Expanded(func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(s.layoutMenuBar),
				layout.Flexed(1, s.layoutBody),
				layout.Rigid(s.layoutStatus),
			)
		}),
		layout.Stacked(s.layoutOpenMenu),
		layout.Stacked(s.layoutOpenSubmenu),
		layout.Expanded(s.layoutLoadingOverlay),
		layout.Expanded(s.layoutSkySetup),
		layout.Expanded(s.layoutWaylandHelp),
		layout.Expanded(s.layoutAbout),
	)
}

func (s *shell) update(gtx layout.Context) {
	if s.operationCancel.Clicked(gtx) {
		s.cancelCurrentOperation()
	}
	if s.skyAllow.Clicked(gtx) {
		s.skySetupOpen = false
		s.startSkyInitialScan()
	}
	if s.skyDeny.Clicked(gtx) {
		s.skySetupOpen = false
		s.skyDenied = true
		s.skyMessage = "Plane of Sky tracking is disabled for this run. It will ask again next launch."
	}
	if s.helpClose.Clicked(gtx) {
		s.waylandHelp = false
		if s.rememberHelp {
			s.rememberHelp = false
			s.settings.WaylandNotice = true
			if err := saveSettings(s.settings); err != nil {
				s.statusText = "Wayland help preference could not be saved"
			}
		}
		if s.openAfterHelp {
			s.openAfterHelp = false
			s.openOverlay()
			s.setOverlayVisible(true)
		}
	}
	if s.aboutClose.Clicked(gtx) {
		s.aboutOpen = false
	}
	select {
	case closed := <-s.overlayClosed:
		if s.overlay == closed {
			s.overlay = nil
			s.setOverlayVisible(false)
		}
	default:
	}
	select {
	case choice := <-s.fileChosen:
		if choice.err == nil {
			s.rememberChosenFile(choice)
		} else if s.currentLog != "" {
			s.statusText = filepath.Base(s.currentLog) + " · live only"
		} else {
			s.statusText = "No logfile selected"
		}
	default:
	}
	select {
	case update := <-s.combatUpdates:
		if update.progress != nil {
			s.loading = true
			s.loadBytes = update.progress.Bytes
			s.loadTotal = update.progress.Total
			s.loadLines = update.progress.Lines
		}
		if update.loadDone {
			s.loading = false
		}
		if update.fights != nil {
			s.allFights = update.fights
			s.applyFightFilter()
			s.pushOverlay(update.fights)
		}
		if update.xp != nil {
			s.xpSnapshot = *update.xp
		}
		if update.state != "" {
			s.parserState = update.state
		}
		if update.status != "" {
			s.statusText = update.status
		}
	default:
	}
	select {
	case update := <-s.skyUpdates:
		s.applySkyAsyncUpdate(update)
	default:
	}
	if s.filterClear.Clicked(gtx) {
		s.fightFilter = ""
		s.filterEditor.SetText("")
		s.applyFightFilter()
		s.fightList.ScrollTo(0)
	}
	if s.skyHideClick.Clicked(gtx) {
		s.skyHideEmpty = !s.skyHideEmpty
		s.rebuildSkyRows()
		s.skyList.ScrollTo(0)
	}
	if s.skyStatusClick.Clicked(gtx) {
		s.workspace = 1
		s.activeMenu = -1
	}
	for index := range s.menus {
		if s.menus[index].click.Clicked(gtx) {
			if s.activeMenu == index {
				s.activeMenu = -1
			} else {
				s.activeMenu = index
				s.activeSub = -1
			}
		}
	}
	for index := range s.rail {
		if s.rail[index].click.Clicked(gtx) {
			s.workspace = index
			s.activeMenu = -1
		}
	}
	for key, click := range s.treeClicks {
		if click.Clicked(gtx) {
			s.expanded[key] = !s.expanded[key]
		}
	}
	if s.activeMenu >= 0 && s.activeMenu < len(s.menus) {
		for index := range s.menus[s.activeMenu].items {
			item := &s.menus[s.activeMenu].items[index]
			if item.enabled && item.click.Clicked(gtx) {
				if len(item.items) > 0 {
					s.activeSub = index
				} else {
					s.activateItem(*item)
					return
				}
			}
		}
		if s.activeSub >= 0 && s.activeSub < len(s.menus[s.activeMenu].items) {
			for index := range s.menus[s.activeMenu].items[s.activeSub].items {
				item := &s.menus[s.activeMenu].items[s.activeSub].items[index]
				if item.enabled && item.click.Clicked(gtx) {
					s.activateItem(*item)
					return
				}
			}
		}
	}
}

func (s *shell) layoutLoadingOverlay(gtx layout.Context) layout.Dimensions {
	if !s.loading && !s.skyLoading {
		return layout.Dimensions{}
	}
	loadBytes, loadTotal, loadLines, loadTitle := s.loadBytes, s.loadTotal, s.loadLines, s.loadingTitle
	if s.skyLoading {
		loadBytes, loadTotal, loadLines, loadTitle = s.skyLoadBytes, s.skyLoadTotal, s.skyLoadLines, s.skyLoadTitle
	}
	paint.Fill(gtx.Ops, color.NRGBA{A: 165})
	return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Min = image.Pt(gtx.Dp(unit.Dp(620)), gtx.Dp(unit.Dp(210)))
		gtx.Constraints.Max = gtx.Constraints.Min
		return outline(gtx, palette.line, func(gtx layout.Context) layout.Dimensions {
			fill(gtx, palette.panel)
			return layout.UniformInset(unit.Dp(22)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				progress := float32(0)
				if loadTotal > 0 {
					progress = float32(loadBytes) / float32(loadTotal)
				}
				percent := int(progress*100 + .5)
				detail := fmt.Sprintf("%d%% · %d lines processed", percent, loadLines)
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						title := loadTitle
						if title == "" {
							title = "Loading combat history…"
						}
						return labelWeight(gtx, s.theme, title, unit.Sp(20), palette.text, text.Start, font.SemiBold)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return inset(0, unit.Dp(16)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							bar := material.ProgressBar(s.theme, progress)
							bar.Color = palette.accent
							bar.TrackColor = palette.line
							bar.Height = unit.Dp(8)
							bar.Radius = unit.Dp(4)
							return bar.Layout(gtx)
						})
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return label(gtx, s.theme, detail, unit.Sp(15), palette.muted, text.Start)
					}),
					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
						return layout.SE.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							return s.operationCancel.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
								pointer.CursorPointer.Add(gtx.Ops)
								return layout.UniformInset(unit.Dp(8)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
									return labelWeight(gtx, s.theme, "Cancel", unit.Sp(15), palette.accent, text.Middle, font.SemiBold)
								})
							})
						})
					}),
				)
			})
		})
	})
}

func (s *shell) cancelCurrentOperation() {
	if s.skyLoading {
		if s.skyCancel != nil {
			close(s.skyCancel)
			s.skyCancel = nil
		}
		s.skyLoading = false
		s.skyMessage = "Plane of Sky scan cancelled."
		return
	}
	if s.loading {
		if s.logCancel != nil {
			close(s.logCancel)
			s.logCancel = nil
		}
		s.loading = false
		s.parserState = ""
		s.statusText = "Combat history loading cancelled"
	}
}

func historyRangeItems(action string) []menuItem {
	return []menuItem{
		{name: "Live only", enabled: true, action: action},
		{name: "Last 1 hour", enabled: true, action: action, back: time.Hour},
		{name: "Last 4 hours", enabled: true, action: action, back: 4 * time.Hour},
		{name: "Last 8 hours", enabled: true, action: action, back: 8 * time.Hour},
		{name: "Full history", enabled: true, action: action, back: -time.Nanosecond},
	}
}

func recentMenuItems(settings guiSettings) []menuItem {
	items := make([]menuItem, 0, len(settings.RecentLogfiles))
	for _, path := range settings.RecentLogfiles {
		if _, err := os.Stat(path); err == nil {
			items = append(items, menuItem{name: filepath.Base(path), detail: path, enabled: true, action: "recent", path: path})
		}
	}
	return items
}

func (s *shell) activateItem(item menuItem) {
	s.activeMenu, s.activeSub = -1, -1
	switch item.action {
	case "open":
		s.statusText = "Choosing logfile…"
		go func(back time.Duration) {
			path, err := zenity.SelectFile(zenity.Title("Open EverQuest logfile"), zenity.FileFilters{{Name: "EverQuest logs", Patterns: []string{"eqlog_*.txt", "*.txt"}}})
			s.fileChosen <- fileChoice{path: path, back: back, err: err}
			s.window.Invalidate()
		}(item.back)
	case "recent":
		s.rememberChosenFile(fileChoice{path: item.path})
	case "reload":
		s.loadLog(s.currentLog, item.back)
	case "overlay":
		s.toggleOverlay()
	case "wayland-help":
		s.showWaylandHelp()
	case "about":
		s.aboutOpen = true
	case "preferences":
		s.workspace = 2
	case "damage":
		s.workspace = 0
	case "sky":
		s.workspace = 1
	case "current":
		s.showCurrentFight()
	case "filter":
		s.workspace = 0
	case "reset":
		if s.currentLog != "" {
			s.fightFilter = ""
			s.filterEditor.SetText("")
			s.loadLog(s.currentLog, 0)
			s.statusText = filepath.Base(s.currentLog) + " · session reset"
		}
	case "exit":
		s.window.Perform(system.ActionClose)
	}
}

func (s *shell) layoutAbout(gtx layout.Context) layout.Dimensions {
	if !s.aboutOpen {
		return layout.Dimensions{}
	}
	paint.Fill(gtx.Ops, color.NRGBA{A: 175})
	return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Min = image.Pt(gtx.Dp(unit.Dp(520)), gtx.Dp(unit.Dp(300)))
		gtx.Constraints.Max = gtx.Constraints.Min
		return outline(gtx, palette.line, func(gtx layout.Context) layout.Dimensions {
			fill(gtx, palette.panel)
			return layout.UniformInset(unit.Dp(24)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				const description = "A live combat meter and Plane of Sky quest tracker built primarily for EverQuest Legends.\n\nThe graphical and terminal frontends share the same combat, experience, and quest parsers.\n\nLicensed under the MIT License."
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return labelWeight(gtx, s.theme, "eqdps", unit.Sp(24), palette.text, text.Start, font.SemiBold)
					}),
					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
						return inset(0, unit.Dp(18)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							return label(gtx, s.theme, description, unit.Sp(15), palette.text, text.Start)
						})
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return layout.E.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							return s.aboutClose.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
								pointer.CursorPointer.Add(gtx.Ops)
								fill(gtx, palette.panelAlt)
								return layout.UniformInset(unit.Dp(10)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
									return labelWeight(gtx, s.theme, "Close", unit.Sp(16), palette.accent, text.Middle, font.SemiBold)
								})
							})
						})
					}),
				)
			})
		})
	})
}

func (s *shell) layoutWaylandHelp(gtx layout.Context) layout.Dimensions {
	if !s.waylandHelp {
		return layout.Dimensions{}
	}
	paint.Fill(gtx.Ops, color.NRGBA{A: 175})
	return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Min = image.Pt(gtx.Dp(unit.Dp(640)), gtx.Dp(unit.Dp(390)))
		gtx.Constraints.Max = gtx.Constraints.Min
		return outline(gtx, palette.line, func(gtx layout.Context) layout.Dimensions {
			fill(gtx, palette.panel)
			return layout.UniformInset(unit.Dp(24)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				const guidance = "Wayland compositors decide whether windows float, stay above other windows, and use opacity. eqdps cannot set these properties portably.\n\nHyprland 0.55+: add the title-based rule from the README to hyprland.lua. It can float, pin, position, resize, and apply opacity to ‘eqdps — Current Fight’.\n\nKDE Plasma: create a Window Rule matching that title. Sway: use a for_window rule matching the title. GNOME may require an extension for persistent always-on-top behavior."
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return labelWeight(gtx, s.theme, "Configure the DPS overlay on Wayland", unit.Sp(21), palette.text, text.Start, font.SemiBold)
					}),
					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
						return inset(0, unit.Dp(18)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							return label(gtx, s.theme, guidance, unit.Sp(15), palette.text, text.Start)
						})
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return layout.E.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							return s.helpClose.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
								pointer.CursorPointer.Add(gtx.Ops)
								fill(gtx, palette.panelAlt)
								return layout.UniformInset(unit.Dp(10)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
									return labelWeight(gtx, s.theme, "Got it", unit.Sp(16), palette.accent, text.Middle, font.SemiBold)
								})
							})
						})
					}),
				)
			})
		})
	})
}

func (s *shell) rememberChosenFile(choice fileChoice) {
	s.currentLog = choice.path
	s.settings.rememberLog(choice.path)
	recents := recentMenuItems(s.settings)
	s.menus[0].items[1].items = recents
	s.menus[0].items[1].enabled = len(recents) > 0
	if err := saveSettings(s.settings); err != nil {
		s.statusText = "Log opened; settings could not be saved"
	} else {
		s.statusText = filepath.Base(choice.path) + " · " + historyStatus(choice.back)
	}
	// Enable history loading now that a current logfile exists.
	s.menus[1].items[1].enabled = true
	s.menus[1].items[3].enabled = true
	s.loadSkyState(choice.path)
	s.loadLog(choice.path, choice.back)
}

func historyStatus(back time.Duration) string {
	switch back {
	case 0:
		return "live only"
	case time.Hour:
		return "last 1 hour"
	case 4 * time.Hour:
		return "last 4 hours"
	case 8 * time.Hour:
		return "last 8 hours"
	default:
		return "full history"
	}
}

func (s *shell) layoutMenuBar(gtx layout.Context) layout.Dimensions {
	gtx.Constraints.Min.Y = gtx.Dp(unit.Dp(38))
	gtx.Constraints.Max.Y = gtx.Constraints.Min.Y
	fill(gtx, palette.chrome)
	return inset(unit.Dp(14), 0).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		children := make([]layout.FlexChild, 0, len(s.menus)+1)
		for index := range s.menus {
			index := index
			children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return flatTextControl(gtx, s.theme, &s.menus[index].click, s.menus[index].name, s.activeMenu == index)
			}))
		}
		children = append(children, layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return layout.E.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return label(gtx, s.theme, "EVERQUEST LEGENDS", unit.Sp(15), palette.muted, text.End)
			})
		}))
		return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx, children...)
	})
}

func (s *shell) layoutBody(gtx layout.Context) layout.Dimensions {
	return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
		layout.Rigid(s.layoutRail),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return inset(unit.Dp(22), unit.Dp(20)).Layout(gtx, s.layoutWorkspace)
		}),
	)
}

func (s *shell) layoutRail(gtx layout.Context) layout.Dimensions {
	gtx.Constraints.Min.X = gtx.Dp(unit.Dp(68))
	gtx.Constraints.Max.X = gtx.Constraints.Min.X
	fill(gtx, palette.rail)
	items := make([]layout.FlexChild, 0, len(s.rail)+1)
	for index := range s.rail {
		index := index
		if index == len(s.rail)-1 {
			items = append(items, layout.Flexed(1, func(gtx layout.Context) layout.Dimensions { return layout.Dimensions{Size: gtx.Constraints.Min} }))
		}
		items = append(items, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return railControl(gtx, s.theme, &s.rail[index].click, s.rail[index], s.workspace == index)
		}))
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, items...)
}

func (s *shell) layoutWorkspace(gtx layout.Context) layout.Dimensions {
	switch s.workspace {
	case 1:
		return s.layoutSkyWorkspace(gtx)
	case 2:
		return s.layoutPreferences(gtx)
	default:
		return s.layoutDamageMeter(gtx)
	}
}

func (s *shell) layoutDamageMeter(gtx layout.Context) layout.Dimensions {
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(s.layoutFightFilterBar),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return s.layoutCombatRow(gtx, fakeCombatant{}, true, false)
		}),
		layout.Rigid(separator),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			if len(s.fights) == 0 {
				return s.layoutPlaceholder(gtx, "No fights yet", "Following the logfile for new combat.")
			}
			list := material.List(s.theme, &s.fightList)
			list.AnchorStrategy = material.Occupy
			list.Indicator.Color = palette.muted
			list.Indicator.HoverColor = palette.text
			list.Indicator.MinorWidth = unit.Dp(7)
			list.Indicator.CornerRadius = unit.Dp(3.5)
			return list.Layout(gtx, len(s.fights), func(gtx layout.Context, index int) layout.Dimensions {
				fight := s.fights[index]
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions { return s.layoutFightHeader(gtx, fight) }),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions { return s.layoutCombatRows(gtx, index, fight.combatants) }),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return layout.Dimensions{Size: image.Pt(gtx.Constraints.Max.X, gtx.Dp(unit.Dp(14)))}
					}),
				)
			})
		}),
	)
}

func (s *shell) layoutFightHeader(gtx layout.Context, fight fakeFightSection) layout.Dimensions {
	gtx.Constraints.Min.Y = gtx.Dp(unit.Dp(42))
	gtx.Constraints.Max.Y = gtx.Constraints.Min.Y
	fill(gtx, palette.panelAlt)
	return centerContent(gtx, func(gtx layout.Context) layout.Dimensions {
		return inset(unit.Dp(14), 0).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return labelWeight(gtx, s.theme, fight.name, unit.Sp(18), palette.text, text.Start, font.SemiBold)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return inset(unit.Dp(14), 0).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						statusColor := palette.muted
						if fight.current {
							statusColor = palette.success
						}
						return label(gtx, s.theme, fight.status, unit.Sp(15), statusColor, text.Start)
					})
				}),
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					return layout.E.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return label(gtx, s.theme, fight.duration, unit.Sp(16), palette.accent, text.End)
					})
				}),
			)
		})
	})
}

func (s *shell) layoutCombatRows(gtx layout.Context, fightIndex int, combatants []fakeCombatant) layout.Dimensions {
	children := make([]layout.FlexChild, 0, len(combatants)*2)
	for index, combatant := range combatants {
		index, combatant := index, combatant
		key := fmt.Sprintf("fight:%d:%s", fightIndex, combatant.name)
		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			if len(combatant.details) == 0 {
				return s.layoutCombatRow(gtx, combatant, false, index%2 == 1)
			}
			return s.layoutToggleRow(gtx, key, combatant, index%2 == 1)
		}))
		if len(combatant.details) > 0 && s.expanded[key] {
			children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return s.layoutBreakdowns(gtx, key, combatant.details, 1)
			}))
		}
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
}

func (s *shell) layoutBreakdowns(gtx layout.Context, parent string, details []fakeBreakdown, level int) layout.Dimensions {
	children := make([]layout.FlexChild, 0, len(details)*2)
	for _, detail := range details {
		detail := detail
		key := parent + ":detail:" + detail.name
		row := fakeCombatant{name: detail.name, damage: detail.damage, dps: detail.dps, sdps: detail.sdps, hits: detail.hits, crits: detail.crits, active: detail.active}
		row.name = strings.Repeat("    ", level) + row.name
		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			if len(detail.children) == 0 {
				return s.layoutCombatRow(gtx, row, false, false)
			}
			return s.layoutToggleRow(gtx, key, row, true)
		}))
		if len(detail.children) > 0 && s.expanded[key] {
			children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return s.layoutBreakdowns(gtx, key, detail.children, level+1)
			}))
		}
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
}

func (s *shell) layoutToggleRow(gtx layout.Context, key string, row fakeCombatant, alternate bool) layout.Dimensions {
	click := s.treeClicks[key]
	if click == nil {
		click = new(widget.Clickable)
		s.treeClicks[key] = click
	}
	marker := "+ "
	if s.expanded[key] {
		marker = "- "
	}
	indent := len(row.name) - len(strings.TrimLeft(row.name, " "))
	row.name = row.name[:indent] + marker + row.name[indent:]
	return click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		pointer.CursorPointer.Add(gtx.Ops)
		return s.layoutCombatRow(gtx, row, false, alternate)
	})
}

func (s *shell) layoutCombatRow(gtx layout.Context, row fakeCombatant, header, alternate bool) layout.Dimensions {
	gtx.Constraints.Min.Y = gtx.Dp(unit.Dp(34))
	gtx.Constraints.Max.Y = gtx.Constraints.Min.Y
	if alternate {
		fill(gtx, palette.panel)
	}
	if header {
		fill(gtx, palette.chrome)
	}
	if row.accent {
		paint.FillShape(gtx.Ops, palette.accent, clip.Rect{Max: image.Pt(gtx.Dp(unit.Dp(3)), gtx.Constraints.Max.Y)}.Op())
	}
	nameColor := palette.text
	fontWeight := font.Normal
	if header {
		nameColor = palette.muted
		fontWeight = font.SemiBold
	}
	cell := func(value string, weight float32, align text.Alignment) layout.FlexChild {
		return layout.Flexed(weight, func(gtx layout.Context) layout.Dimensions {
			return labelWeight(gtx, s.theme, value, unit.Sp(17), nameColor, align, fontWeight)
		})
	}
	values := []string{row.name, formatCell(row.damage, "DAMAGE"), formatCell(row.dps, "DPS"), formatCell(row.sdps, "SDPS"), formatCell(row.hits, "HITS"), formatCell(row.crits, "CRITS"), row.active}
	if header {
		values = []string{"COMBATANT", "DAMAGE", "DPS", "SDPS", "HITS", "CRITS", "ACTIVE"}
	}
	return centerContent(gtx, func(gtx layout.Context) layout.Dimensions {
		return inset(unit.Dp(14), 0).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
				cell(values[0], 3.8, text.Start), cell(values[1], 1.4, text.End), cell(values[2], 1, text.End), cell(values[3], 1, text.End), cell(values[4], 1, text.End), cell(values[5], 1, text.End), cell(values[6], 1.2, text.End),
			)
		})
	})
}

func (s *shell) layoutPlaceholder(gtx layout.Context, title, description string) layout.Dimensions {
	return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical, Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return label(gtx, s.theme, title, unit.Sp(27), palette.text, text.Middle)
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return inset(0, unit.Dp(10)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return label(gtx, s.theme, description, unit.Sp(16), palette.muted, text.Middle)
				})
			}),
		)
	})
}

func (s *shell) layoutStatus(gtx layout.Context) layout.Dimensions {
	gtx.Constraints.Min.Y = gtx.Dp(unit.Dp(34))
	gtx.Constraints.Max.Y = gtx.Constraints.Min.Y
	fill(gtx, palette.chrome)
	return centerContent(gtx, func(gtx layout.Context) layout.Dimensions {
		return inset(unit.Dp(14), 0).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
				layout.Flexed(3, func(gtx layout.Context) layout.Dimensions {
					return label(gtx, s.theme, s.statusText, unit.Sp(15), palette.text, text.Start)
				}),
				layout.Flexed(2, func(gtx layout.Context) layout.Dimensions {
					return inset(unit.Dp(28), 0).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return label(gtx, s.theme, xpStatusText(s.xpSnapshot, s.fightFilter), unit.Sp(15), palette.muted, text.Start)
					})
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					ready := s.skyReadyCount()
					foreground := palette.muted
					status := fmt.Sprintf("PoS: %d ready", ready)
					if ready > 0 {
						foreground = skyReadyColor
					}
					notice := !s.skyNoticeUntil.IsZero() && time.Now().Before(s.skyNoticeUntil)
					if notice {
						status = s.skyNoticeText
						gtx.Execute(op.InvalidateCmd{At: s.skyNoticeUntil})
					}
					return s.skyStatusClick.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						pointer.CursorPointer.Add(gtx.Ops)
						if notice {
							fill(gtx, color.NRGBA{R: 31, G: 65, B: 39, A: 255})
						}
						return inset(unit.Dp(18), 0).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							return label(gtx, s.theme, status, unit.Sp(15), foreground, text.Start)
						})
					})
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.E.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						stateText, stateColor := parserStatus(s.parserState, s.currentLog != "")
						return label(gtx, s.theme, stateText, unit.Sp(15), stateColor, text.End)
					})
				}),
			)
		})
	})
}

func (s *shell) layoutOpenMenu(gtx layout.Context) layout.Dimensions {
	if s.activeMenu < 0 {
		return layout.Dimensions{}
	}
	xOffsets := []int{12, 68, 151, 215, 278}
	offset := op.Offset(image.Pt(gtx.Dp(unit.Dp(xOffsets[s.activeMenu])), gtx.Dp(unit.Dp(38)))).Push(gtx.Ops)
	defer offset.Pop()
	gtx.Constraints.Min = image.Pt(gtx.Dp(unit.Dp(230)), 0)
	gtx.Constraints.Max.X = gtx.Constraints.Min.X
	menu := &s.menus[s.activeMenu]
	menuHeight := gtx.Dp(unit.Dp(42*len(menu.items) + 2))
	gtx.Constraints.Min.Y = menuHeight
	gtx.Constraints.Max.Y = menuHeight
	children := make([]layout.FlexChild, 0, len(menu.items))
	for index := range menu.items {
		index := index
		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return menuControl(gtx, s.theme, &menu.items[index])
		}))
	}
	return outline(gtx, palette.line, func(gtx layout.Context) layout.Dimensions {
		fill(gtx, palette.panel)
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
	})
}

func (s *shell) layoutOpenSubmenu(gtx layout.Context) layout.Dimensions {
	if s.activeMenu < 0 || s.activeSub < 0 {
		return layout.Dimensions{}
	}
	xOffsets := []int{12, 68, 151, 215, 278}
	x := xOffsets[s.activeMenu] + 230
	y := 38 + s.activeSub*42
	offset := op.Offset(image.Pt(gtx.Dp(unit.Dp(x)), gtx.Dp(unit.Dp(y)))).Push(gtx.Ops)
	defer offset.Pop()
	items := s.menus[s.activeMenu].items[s.activeSub].items
	gtx.Constraints.Min = image.Pt(gtx.Dp(unit.Dp(250)), gtx.Dp(unit.Dp(42*len(items)+2)))
	gtx.Constraints.Max = gtx.Constraints.Min
	children := make([]layout.FlexChild, 0, len(items))
	for index := range items {
		index := index
		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return menuControl(gtx, s.theme, &items[index])
		}))
	}
	return outline(gtx, palette.line, func(gtx layout.Context) layout.Dimensions {
		fill(gtx, palette.panel)
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
	})
}

func flatTextControl(gtx layout.Context, theme *material.Theme, click *widget.Clickable, value string, active bool) layout.Dimensions {
	return click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		pointer.CursorPointer.Add(gtx.Ops)
		if active || click.Hovered() {
			fill(gtx, palette.panelAlt)
		}
		return inset(unit.Dp(12), 0).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return label(gtx, theme, value, unit.Sp(17), palette.text, text.Start)
		})
	})
}

func railControl(gtx layout.Context, theme *material.Theme, click *widget.Clickable, item railItem, selected bool) layout.Dimensions {
	gtx.Constraints.Min.Y = gtx.Dp(unit.Dp(58))
	gtx.Constraints.Max.Y = gtx.Constraints.Min.Y
	return click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		pointer.CursorPointer.Add(gtx.Ops)
		if selected || click.Hovered() {
			fill(gtx, palette.panel)
		}
		if selected {
			paint.FillShape(gtx.Ops, palette.accent, clip.Rect{Max: image.Pt(gtx.Dp(unit.Dp(3)), gtx.Constraints.Max.Y)}.Op())
		}
		return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			valueColor := palette.muted
			if selected {
				valueColor = palette.accent
			}
			return labelWeight(gtx, theme, item.short, unit.Sp(15), valueColor, text.Middle, font.SemiBold)
		})
	})
}

func menuControl(gtx layout.Context, theme *material.Theme, item *menuItem) layout.Dimensions {
	gtx.Constraints.Min.Y = gtx.Dp(unit.Dp(42))
	gtx.Constraints.Max.Y = gtx.Constraints.Min.Y
	return item.click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		if item.enabled {
			pointer.CursorPointer.Add(gtx.Ops)
			if item.click.Hovered() {
				fill(gtx, palette.panelAlt)
			}
		}
		foreground := palette.text
		if !item.enabled {
			foreground = palette.muted
		}
		return inset(unit.Dp(12), 0).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			name := item.name
			if len(item.items) > 0 {
				name += "  ›"
			}
			return layout.Flex{Axis: layout.Vertical, Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return label(gtx, theme, name, unit.Sp(16), foreground, text.Start)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					if item.detail == "" {
						return layout.Dimensions{}
					}
					return label(gtx, theme, item.detail, unit.Sp(13), palette.muted, text.Start)
				}),
			)
		})
	})
}

func label(gtx layout.Context, theme *material.Theme, value string, size unit.Sp, foreground color.NRGBA, align text.Alignment) layout.Dimensions {
	return labelWeight(gtx, theme, value, size, foreground, align, font.Normal)
}

func labelWeight(gtx layout.Context, theme *material.Theme, value string, size unit.Sp, foreground color.NRGBA, align text.Alignment, weight font.Weight) layout.Dimensions {
	style := material.Label(theme, size*theme.TextSize/16, value)
	style.Color = foreground
	style.Alignment = align
	style.Font.Weight = weight
	return style.Layout(gtx)
}

func formatCell(value int, header string) string {
	if value < 0 {
		return header
	}
	if value == 0 {
		return "—"
	}
	return fmt.Sprintf("%d", value)
}

func inset(horizontal, vertical unit.Dp) layout.Inset {
	return layout.Inset{Left: horizontal, Right: horizontal, Top: vertical, Bottom: vertical}
}

func fill(gtx layout.Context, fill color.NRGBA) {
	paint.FillShape(gtx.Ops, fill, clip.Rect{Max: gtx.Constraints.Max}.Op())
}

func separator(gtx layout.Context) layout.Dimensions {
	height := gtx.Dp(unit.Dp(1))
	paint.FillShape(gtx.Ops, palette.line, clip.Rect{Max: image.Pt(gtx.Constraints.Max.X, height)}.Op())
	return layout.Dimensions{Size: image.Pt(gtx.Constraints.Max.X, height)}
}

func outline(gtx layout.Context, border color.NRGBA, content layout.Widget) layout.Dimensions {
	paint.FillShape(gtx.Ops, border, clip.Rect{Max: gtx.Constraints.Max}.Op())
	return layout.UniformInset(unit.Dp(1)).Layout(gtx, content)
}

// centerContent lays out a naturally sized child inside an exact-height area.
// Passing the area's minimum height directly to a label makes its glyphs hug
// the top of the row instead of centering the label itself.
func centerContent(gtx layout.Context, content layout.Widget) layout.Dimensions {
	size := image.Pt(gtx.Constraints.Max.X, gtx.Constraints.Min.Y)
	macro := op.Record(gtx.Ops)
	child := gtx
	child.Constraints.Min = image.Point{}
	child.Constraints.Max = size
	dimensions := content(child)
	call := macro.Stop()
	offset := op.Offset(image.Pt(0, max(0, (size.Y-dimensions.Size.Y)/2))).Push(gtx.Ops)
	call.Add(gtx.Ops)
	offset.Pop()
	return layout.Dimensions{Size: size}
}
