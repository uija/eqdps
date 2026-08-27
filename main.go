package main

import (
	"log"
	"os"
	"path/filepath"

	"gioui.org/app"
	"gioui.org/io/system"
	"gioui.org/op"
	"gioui.org/unit"
	"github.com/joho/godotenv"
	"github.com/uija/eqdps/internal/module"
	"github.com/uija/eqdps/internal/module/dps"
	"github.com/uija/eqdps/internal/module/eqldb"
	"github.com/uija/eqdps/internal/module/events"
	"github.com/uija/eqdps/internal/module/sky"
	"github.com/uija/eqdps/internal/module/statistics"
	"github.com/uija/eqdps/internal/module/xphour"
	"github.com/uija/eqdps/internal/view"
)

func main() {

	godotenv.Load() // ignore error

	var logfile *os.File

	mode := os.Getenv("EQDPS_MODE")
	if mode != "development" {
		baseDir, err := os.UserConfigDir()
		if err == nil {
			configDir := filepath.Join(baseDir, "eqdps")
			if err := os.MkdirAll(configDir, 0o700); err != nil {
				log.Printf("Unable to create config dir. %v", err)
			} else {
				logpath := filepath.Join(configDir, "eqdps.log")
				logfile, err = os.OpenFile(logpath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
				if err != nil {
					log.Printf("Unable to create logfile. %v", err)
				} else {
					log.SetOutput(logfile)
				}
			}
		}
	}

	go func() {
		window := new(app.Window)
		context := module.NewContext(window.Invalidate)
		context.RegisterModule(dps.NewModule())
		context.RegisterModule(xphour.NewModule())
		context.RegisterModule(sky.NewModule())
		context.RegisterModule(events.NewModule())
		context.RegisterModule(eqldb.NewModule())
		context.RegisterModule(statistics.NewModule())
		width := max(1100, context.Config.UIConfig.MainWindowWidth)
		height := max(640, context.Config.UIConfig.MainWindowHeight)
		window.Option(
			app.Title("eqdps"),
			app.Size(unit.Dp(width), unit.Dp(height)),
			app.MinSize(unit.Dp(1100), unit.Dp(400)),
		)
		if err := run(context, window); err != nil {
			log.Print(err)
		}
		context.Shutdown()
		if logfile != nil {
			logfile.Close()
		}
		os.Exit(0)
	}()
	app.Main()
}

var lastMainWidth int
var lastMainHeight int

func run(context *module.Context, window *app.Window) error {
	rootView := view.NewShell(context, func() {
		window.Perform(system.ActionClose)
	}, func() {
		window.Invalidate()
	})

	var ops op.Ops
	for {
		switch event := window.Event().(type) {
		case app.DestroyEvent:
			context.Config.UIConfig.MainWindowWidth = lastMainWidth
			context.Config.UIConfig.MainWindowHeight = lastMainHeight
			if context.Overlay != nil {
				context.Overlay.Close()
			}
			context.Config.Save()
			return event.Err
		case app.FrameEvent:
			gtx := app.NewContext(&ops, event)
			lastMainWidth = int(gtx.Metric.PxToDp(gtx.Constraints.Max.X))
			lastMainHeight = int(gtx.Metric.PxToDp(gtx.Constraints.Max.Y))
			rootView.Layout(gtx)
			event.Frame(gtx.Ops)
		}
	}
}
