package view

import (
	"image"
	"image/color"
	"log"

	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget/material"
	"github.com/ncruces/zenity"
	"github.com/uija/eqdps/internal/module"
	"github.com/uija/eqdps/internal/ui"
	"github.com/uija/eqdps/internal/ui/menu"
)

var style = ui.Style{
	Theme: material.NewTheme(),
	Palette: ui.Palette{
		Window: color.NRGBA{R: 18, G: 20, B: 22, A: 255},
		Chrome: color.NRGBA{R: 27, G: 30, B: 33, A: 255},
		Text:   color.NRGBA{R: 225, G: 226, B: 222, A: 255},
		Muted:  color.NRGBA{R: 150, G: 154, B: 151, A: 255},
		Hover:  color.NRGBA{R: 42, G: 46, B: 50, A: 255},
		Panel:  color.NRGBA{R: 31, G: 34, B: 37, A: 255},
		Shadow: color.NRGBA{A: 190},
	},
}

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
	style   *ui.Style
	menuBar *menu.Bar
	status  string
	help    *helpView
	context *module.Context

	progress *progress

	invalidateFunc   func()
	fileSelectResult chan fileResult
	progressUpdate   chan progress
}

// NewShell constructs the root application view.
func NewShell(context *module.Context, closeWindow func(), invalidate func()) *Shell {
	style.Theme.Palette.Bg = style.Palette.Window
	style.Theme.Palette.Fg = style.Palette.Text
	result := &Shell{
		style:            &style,
		status:           "Ready",
		help:             newHelpView(&style, context),
		context:          context,
		invalidateFunc:   invalidate,
		fileSelectResult: make(chan fileResult, 1),
		progressUpdate:   make(chan progress, 1),
	}
	context.RegisterProgressHandler(result.OnProgress)
	result.menuBar = menu.NewBar(&style, "EVERQUEST LEGENDS")
	fileMenu := result.menuBar.AddMenu("File")
	fileMenu.AddItem("Open Log", result.OpenLogfile)
	fileMenu.AddItem("Quit", closeWindow)
	viewMenu := result.menuBar.AddMenu("View")
	for _, entry := range context.ViewMenuItems {
		viewMenu.AddItem(entry.Name, entry.Action)
	}
	toolsMenu := result.menuBar.AddMenu("Tools")

	toolsMenu.AddSeparator()
	for _, entry := range context.ToolsMenuItems {
		toolsMenu.AddItem(entry.Name, entry.Action)
	}
	toolsMenu.AddItem("Preferences", func() {})
	result.menuBar.AddAction("Help", result.help.Open)

	return result
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
		s.fileSelectResult <- fileResult{Path: path, Error: err}
		s.invalidateFunc()
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
	paint.Fill(gtx.Ops, s.style.Palette.Window)
	s.update(gtx)
	return layout.Stack{}.Layout(gtx,
		layout.Expanded(func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min = gtx.Constraints.Max
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(s.menuBar.LayoutBar),
				layout.Flexed(1, s.layoutMain),
				layout.Rigid(s.layoutStatus),
			)
		}),
		layout.Stacked(s.menuBar.LayoutOverlay),
		layout.Expanded(s.help.Layout),
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
	s.menuBar.Update(gtx)
	s.help.Update(gtx)
}

func (s *Shell) layoutMain(gtx layout.Context) layout.Dimensions {
	return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return s.context.Layout(s.style, gtx)
	})
}

func (s *Shell) layoutStatus(gtx layout.Context) layout.Dimensions {
	gtx.Constraints.Min.Y = gtx.Dp(unit.Dp(30))
	gtx.Constraints.Max.Y = gtx.Constraints.Min.Y
	fill(gtx, s.style.Palette.Chrome)
	return layout.Inset{Left: unit.Dp(14), Right: unit.Dp(14), Top: unit.Dp(4)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		label := material.Label(s.style.Theme, unit.Sp(14), s.status)
		label.Color = s.style.Palette.Muted
		return layout.W.Layout(gtx, label.Layout)
	})
}

func fill(gtx layout.Context, background color.NRGBA) {
	defer clip.Rect(image.Rectangle{Max: gtx.Constraints.Min}).Push(gtx.Ops).Pop()
	paint.ColorOp{Color: background}.Add(gtx.Ops)
	paint.PaintOp{}.Add(gtx.Ops)
}
func (s *Shell) layoutProgressOverlay(gtx layout.Context) layout.Dimensions {
	if s.progress == nil {
		return layout.Dimensions{}
	}

	fill(gtx, s.style.Palette.Shadow)

	return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		width := min(gtx.Dp(unit.Dp(420)), gtx.Constraints.Max.X)
		height := min(gtx.Dp(unit.Dp(120)), gtx.Constraints.Max.Y)
		gtx.Constraints = layout.Exact(image.Pt(width, height))

		fill(gtx, s.style.Palette.Panel)

		return layout.Flex{
			Axis: layout.Vertical,
		}.Layout(gtx,
			layout.Rigid(ui.TitleBar(gtx, *s.style, s.progress.title)),
			layout.Flexed(1, s.layoutProgressContent),
		)
	})
}
