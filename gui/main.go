package main

import (
	"log"
	"os"

	"gioui.org/app"
	"gioui.org/io/system"
	"gioui.org/op"
	"gioui.org/unit"
	"github.com/uija/eqdps/internal/module"
	"github.com/uija/eqdps/internal/module/dps"
	"github.com/uija/eqdps/internal/module/sky"
	"github.com/uija/eqdps/internal/module/xphour"
	"github.com/uija/eqdps/internal/view"
)

func main() {
	go func() {
		window := new(app.Window)
		context := module.NewContext(window.Invalidate)
		context.RegisterModule(dps.NewModule())
		context.RegisterModule(&sky.Module{})
		context.RegisterModule(&xphour.Module{})

		window.Option(
			app.Title("eqdps"),
			app.Size(unit.Dp(960), unit.Dp(640)),
			app.MinSize(unit.Dp(640), unit.Dp(400)),
		)
		if err := run(context, window); err != nil {
			log.Print(err)
		}
		os.Exit(0)
	}()
	app.Main()
}

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
			return event.Err
		case app.FrameEvent:
			gtx := app.NewContext(&ops, event)
			rootView.Layout(gtx)
			event.Frame(gtx.Ops)
		}
	}
}
