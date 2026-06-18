# AudioSync

Stream system audio from N computers to one, so a single set of headphones hears
them all mixed together. Cross-platform (Windows, macOS, Linux), Go.

Each **sender** captures its machine's audio output and streams it over the LAN as
raw PCM/UDP. One **receiver** mixes every incoming stream and plays the result to
its output device. Audio is one-directional; the receiver's own local audio still
reaches the headphones normally and the OS mixes everything in hardware.

> **Status: Phase 3.** Full pipeline with **real system-audio capture**
> (`-source=system`): WASAPI loopback (Windows), PulseAudio/PipeWire monitor
> (Linux), CoreAudio process-tap (macOS). Each stream is **resampled to the
> output rate with per-stream clock-drift compensation**, so mismatched sample
> rates (44.1k vs 48k) and audio-clock drift between machines no longer cause
> dropouts or latency creep. A tone generator (`-source=tone`) remains for
> testing. Still to come: mDNS discovery (Phase 5), tray UI (Phase 6).

## Architecture

```
SENDER: capture (C audio thread) → SPSC ring → UDP packet{id,seq,ts,fmt,PCM}
                                                      │
══════════════════════════ LAN (UDP unicast) ════════╪═══════════════════════
                                                      ▼
RECEIVER: demux by streamID → per-stream jitter buffer → mix+clip → output device
```

Two realtime audio threads (capture, playback) are isolated from Go's GC/scheduler
by lock-free SPSC ring buffers; everything between them is plain Go. This is what
keeps the latency budget intact (target ~<50ms on **wired** LAN; WiFi is best-effort
and higher).

| Package | Purpose |
|---|---|
| `internal/audio` | format model, capture/playback backends (malgo), tone source |
| `internal/ring` | lock-free SPSC byte ring — the spine |
| `internal/transport` | UDP wire format + send/receive |
| `internal/mixer` | per-stream jitter buffers, sum/clip, per-source gain/mute |
| `internal/resample` | per-stream linear resampler (rate conversion + drift trim) |
| `internal/drift` | PI controller on buffer fill → resample-ratio correction |
| `internal/role` | sender & receiver pipeline wiring |
| `internal/config` | CLI flags |

## Build

Requires Go 1.24+ and a C compiler (CGO is mandatory — malgo wraps miniaudio).

```sh
go build -o audiosync ./cmd/audiosync
go test ./...
```

## Run

### Real use — capture other machines' audio

On the machine with the headphones (the **receiver**):

```sh
./audiosync -role=receiver -listen=:4010
```

On every **other** machine, capture its system output and stream it (use the
receiver's LAN IP; give each sender a unique `-id`):

```sh
./audiosync -role=sender -source=system -peer=192.168.1.50:4010 -id=2
```

Now the receiver's headphones play every sender mixed together.

- **macOS sender** needs macOS 14.4+ and audio-recording permission (first run may
  prompt; grant it in System Settings → Privacy & Security).
- **Linux sender** needs PulseAudio or pipewire-pulse running (it captures the
  default sink's `.monitor` source).
- **Windows sender** captures the default output via WASAPI loopback — no setup.
- Do **not** run `-role=both -source=system` on one machine: the tap captures the
  app's own playback and feeds back. `both` is for the tone source only.

### Tone smoke test (makes sound!)

One machine, full sender→UDP→mix→playback path with a 440Hz tone:

```sh
./audiosync -role=both
```

### Key flags

| Flag | Default | Meaning |
|---|---|---|
| `-role` | `both` | `sender` \| `receiver` \| `both` |
| `-peer` | `127.0.0.1:4010` | receiver address (sender mode) |
| `-listen` | `:4010` | bind address (receiver mode) |
| `-id` | `1` | stream id (unique per sender) |
| `-source` | `tone` | `tone` (test) \| `system` (real OS loopback capture) |
| `-discover` | `false` | sender: auto-find the receiver via mDNS (ignores `-peer`) |
| `-frame-ms` | `5` | packet/period size |
| `-prefill-ms` | `20` | jitter-buffer prefill before playout |
| `-buffer-ms` | `200` | jitter-buffer capacity |
| `-tone-hz` | `440` | tone frequency (tone source) |

## Desktop app (Wails + React)

A desktop UI lives in `desktop/` — a [Wails](https://wails.io) app (Go shell + React/TS
frontend) that drives the same audio engine through the `engine` package. The engine is
the public API; the UI is pure presentation over `engine.Snapshot()` (polled ~30 Hz) and
its command methods (volume, mute, rename, start/stop).

```
desktop/
  app.go            # Wails bindings -> engine facade
  main.go           # window config
  frontend/src/     # React: Sidebar, ChannelStrip, Meter, App (mixer view)
```

Run / build (requires the Wails CLI and **Node 18+** — note the repo's default `node` on
this machine is an old nvm shim, so select a modern one first):

```sh
# one-time: go install github.com/wailsapp/wails/v2/cmd/wails@latest
export PATH="$HOME/.nvm/versions/node/v22.15.1/bin:$(go env GOPATH)/bin:$PATH"
cd desktop
wails dev      # hot-reload dev window
wails build    # -> desktop/build/bin/audiosync-desktop.app
```

The window opens on the receiver and shows the live mixer: one channel strip per sender
(meter, volume fader, mute, health, sample rate) plus a master/output strip. With no
senders it shows the first-run state with this machine's address to point senders at.

> Status: feature-complete window app + menu-bar tray. Mixer (live metering, per-source +
> master gain, mute, solo), Connections (rename), Diagnostics (peak/drift/health), Settings
> (latency profile, dark/light theme), output-device picker, and an "Add machine" helper —
> all wired to the real engine. Senders announce hostname + platform (metadata packet) →
> real names/OS badges. Receivers advertise over **mDNS** so senders auto-discover them
> (`-discover`). A menu-bar tray icon keeps the app resident (close hides to tray; Open/Quit
> from the menu). Remaining polish: a compact native menu-bar *popover* panel (today the tray
> menu opens the full window) and launch-at-login.

## Roadmap

1. ✅ **Pipeline** — source → UDP → mix → playback, manual IP.
2. ✅ **OS capture** — WASAPI loopback (Win), Pulse monitor (Linux), CoreAudio tap (mac).
3. ✅ **Drift compensation** — per-stream resampler + PI controller on buffer fill.
4. **Multi-sender mixing** — structurally supported; harden + real multi-box soak.
5. ✅ **Discovery** — receiver advertises over mDNS; sender `-discover` auto-finds it.
6. ✅ **UX** — Wails+React desktop app (mixer, devices, per-source volume/mute/solo, master
   gain, diagnostics, theme) + menu-bar tray.
