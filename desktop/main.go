package main

import (
	"context"
	"embed"

	"github.com/energye/systray"
	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/mac"
	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

//go:embed all:frontend/dist
var assets embed.FS

//go:embed build/trayicon.png
var trayIcon []byte

// version is injected at link time via -ldflags "-X main.version=...".
var version = "dev"

func main() {
	app := NewApp()

	err := wails.Run(&options.App{
		Title:     "AudioSync",
		Width:     960,
		Height:    640,
		MinWidth:  720,
		MinHeight: 480,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 10, G: 11, B: 13, A: 1},
		// Closing the window hides it; the app keeps running in the menu bar.
		HideWindowOnClose: true,
		OnStartup: func(ctx context.Context) {
			app.startup(ctx)
			// energye/systray.Register is non-blocking and cooperates with the
			// Wails (Cocoa) run loop, so the tray and window coexist.
			systray.Register(func() { setupTray(app) }, nil)
		},
		OnShutdown: app.shutdown,
		Mac:        &mac.Options{},
		Bind:       []any{app},
	})
	if err != nil {
		println("Error:", err.Error())
	}
}

// setupTray builds the menu-bar icon and menu. Items drive the Wails window via
// the runtime using the app's startup context.
func setupTray(app *App) {
	// Template icon: macOS tints it to match the menu bar (light/dark).
	systray.SetTemplateIcon(trayIcon, trayIcon)
	systray.SetTooltip("AudioSync")

	show := systray.AddMenuItem("Open AudioSync", "Show the AudioSync window")
	show.Click(func() {
		wruntime.WindowShow(app.ctx)
		wruntime.WindowUnminimise(app.ctx)
	})

	systray.AddSeparator()
	quit := systray.AddMenuItem("Quit AudioSync", "Stop and quit")
	quit.Click(func() { wruntime.Quit(app.ctx) })
}
