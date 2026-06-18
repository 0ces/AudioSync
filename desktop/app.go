package main

import (
	"context"

	"github.com/eplata/audiosync/engine"
)

// App is the Wails-bound facade. It owns the audio engine and exposes its
// control surface (snapshot + commands) to the React frontend.
type App struct {
	ctx context.Context
	eng *engine.Engine
}

// NewApp creates the app shell.
func NewApp() *App { return &App{} }

// startup builds the engine and starts the receiver pipeline.
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	a.eng = engine.NewReceiver(":4010")
	_ = a.eng.StartReceiver() // status surfaced via GetSnapshot().Running
}

// shutdown tears the engine down cleanly.
func (a *App) shutdown(ctx context.Context) {
	if a.eng != nil {
		a.eng.Stop()
	}
}

// --- bound methods (callable from JS) ---

// GetSnapshot returns the current UI state. The frontend polls this ~30×/sec.
func (a *App) GetSnapshot() engine.Snapshot { return a.eng.Snapshot() }

// SetVolume sets a source's gain (1.0 == 100%).
func (a *App) SetVolume(id uint32, v float32) { a.eng.SetVolume(id, v) }

// SetMuted mutes/unmutes a source.
func (a *App) SetMuted(id uint32, muted bool) { a.eng.SetMuted(id, muted) }

// Rename sets a source's display label.
func (a *App) Rename(id uint32, name string) { a.eng.Rename(id, name) }

// SetSolo solos/unsolos a source.
func (a *App) SetSolo(id uint32, solo bool) { a.eng.SetSolo(id, solo) }

// SetMasterVolume sets the master output gain (1.0 == 100%).
func (a *App) SetMasterVolume(v float32) { a.eng.SetMasterVolume(v) }

// SetLatencyProfile adjusts jitter-buffer prefill/capacity (ms) and restarts.
func (a *App) SetLatencyProfile(prefillMs int, bufferMs int) string {
	if err := a.eng.SetLatencyProfile(prefillMs, bufferMs); err != nil {
		return err.Error()
	}
	return ""
}

// ListOutputDevices returns the system's playback devices.
func (a *App) ListOutputDevices() ([]engine.OutputDevice, error) { return a.eng.ListOutputDevices() }

// SetOutputDevice switches playback to the given device (hex id + display name).
func (a *App) SetOutputDevice(id string, name string) string {
	if err := a.eng.SetOutputDevice(id, name); err != nil {
		return err.Error()
	}
	return ""
}

// Start (re)starts the receiver pipeline; returns "" on success or an error string.
func (a *App) Start() string {
	if err := a.eng.StartReceiver(); err != nil {
		return err.Error()
	}
	return ""
}

// Stop halts the pipeline.
func (a *App) Stop() { a.eng.Stop() }

// Running reports whether the pipeline is active.
func (a *App) Running() bool { return a.eng.Running() }

// Version returns the app's build version (injected at link time).
func (a *App) Version() string { return version }
