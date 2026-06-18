# AudioSync — UI/UX Design Prompt

Paste the prompt below into Claude (claude.ai, an artifact session, or any design-capable
model). It is self-contained: it tells the model what the product is, who uses it, the exact
data the interface must surface, the screens and states, and the deliverables to produce. Edit
the **Visual direction** section if you want a different aesthetic.

---

## PROMPT

You are a senior product designer. Design the complete UI/UX for a cross-platform desktop app
called **AudioSync**. Produce both an interaction/spec document and high-fidelity visual mockups
(as React + Tailwind components or annotated SVG screens — your choice, but they must be concrete
and buildable, not vague descriptions).

### What the product does
AudioSync lets one person hear several computers through a single pair of headphones. Each
"sender" machine (Windows, macOS, or Linux) captures its own system audio output and streams it
over the local network to one "receiver" machine — the one with the headphones — which mixes all
the incoming streams together and plays the result. Think of it as a small live audio mixing
console for "all my other computers," running quietly in the background.

Typical user: a developer, streamer, trader, or sysadmin who runs 2–5 machines at once (a gaming
PC, a work laptop, a Linux server, a test Mac) and is tired of swapping headsets. They want every
machine audible at once, with independent volume per machine, and the whole thing to "just work"
and stay out of the way.

The app is built on a real-time audio engine, so it is latency- and health-sensitive: the UI must
make stream health legible at a glance (is a source live? clipping? dropping packets? drifting?).

### Form factor (required)
Two surfaces, sharing one visual language:

1. **Tray/menubar popover** — a compact panel that drops down from the menubar (macOS) or system
   tray (Windows/Linux) icon. This is the everyday surface: glance at status, nudge a volume,
   mute a machine, open the full window. Roughly 360px wide, height fits content. Frameless,
   feels native to the OS menubar.

2. **Full window** — the complete app: a proper mixer, device pickers, connection management,
   diagnostics, and settings. Resizable, ~900×640 default. This is where the user sets things up
   and digs into problems.

The tray icon itself must convey state (idle / active / problem) — design the icon states too.

### Roles and modes
A machine runs as **Receiver** (has the headphones, mixes), **Sender** (captures + streams its
audio out), or **Both**. The UI adapts to role:
- On a **Receiver**, the main surface is the **mixer** of incoming machines.
- On a **Sender**, the main surface shows **what it's capturing and where it's sending**, plus a
  local output-level meter.
- **Both** shows both, sender controls secondary.
Design the role switch and how each role's home view looks.

### The data the UI must surface (this is the real engine's model — design around it)

**Per source / stream (the channel strip), one per connected sender:**
- A label — defaults to the sender's hostname, user-renamable (e.g. "Gaming PC", "Work Mac").
- Platform indicator (Windows / macOS / Linux).
- A live **stereo level meter** (L/R peak, updates ~30×/sec) with a clipping indicator.
- **Volume** control, 0–150% (unity at 100%), with a numeric readout.
- **Mute** and **Solo**.
- **Buffer health** — one of: *Good*, *Filling* (latency creeping), *Starving* (about to drop),
  shown as a small, calm indicator (not alarming unless actually bad).
- **Packet loss %** and **estimated latency (ms)** — secondary, shown on hover or in diagnostics.
- **Source sample rate** (e.g. 48 kHz, 44.1 kHz) — informational.
- Connection state: *Connecting*, *Live*, *Stalled*, *Gone*.

**Master / output (the receiver):**
- Output **device picker** (list of the machine's output devices) + the current device.
- **Master volume** + master level meter.
- Overall status line: "3 machines · 1.2 ms jitter · wired".

**App-level:**
- Role switch (Receiver / Sender / Both).
- Engine on/off (big, obvious start/stop).
- Network: listen port (receiver), peer address (sender) — note a future version auto-discovers
  peers via mDNS, so design for both "manually add a machine by IP" now and "machines appear
  automatically" later.
- Settings: latency profile (a simple **Latency ↔ Stability** slider that maps to buffer size —
  hide the raw milliseconds behind "Advanced"), frame size, port, launch-at-login, theme.
- Diagnostics view: per-stream latency, buffer-fill over time (small sparkline), drift correction,
  loss — for when something sounds wrong.

### Key screens to deliver
1. **Tray popover — Receiver, active**: master at top, a vertical list of compact channel rows
   (label, platform dot, mini meter, volume slider, mute), status footer, "Open AudioSync" +
   power button. This is the hero screen — make it excellent.
2. **Tray popover — empty / first run**: no machines yet. Guide the user: "Run AudioSync on
   another computer and point it here" with this machine's address shown and copyable.
3. **Window — Mixer**: full channel strips side by side (console layout), master strip on the
   right, output device picker, transport/engine controls. The desktop counterpart to the popover.
4. **Window — Connections**: machines list with add-by-IP, rename, per-machine status, remove;
   plus this machine's own identity (name, address, role).
5. **Window — Diagnostics**: health detail per stream (meters, buffer sparkline, loss, latency,
   drift), good for screenshots when reporting a problem.
6. **Window — Settings**: role, latency/stability slider, device, network, launch-at-login, theme.
7. **Sender home** (popover + window): "Capturing system audio → sending to <receiver>", local
   meter, a pause/mute-send control.
8. **Tray icon states**: idle, active, warning/problem.

### States to design for every surface
Empty (no sources), connecting, healthy/active, degraded (loss/drift/starving — show it calmly
and actionably), error (engine failed to start, device unavailable, permission denied — on macOS
the audio-capture permission may be denied; design that prompt/explainer), and muted/paused.

### Interactions & behaviors
- Volume: drag slider, scroll over it, double-click to reset to 100%, type a number.
- Mute toggles instantly with clear visual feedback; Solo dims the others.
- Meters are smooth (peak + short decay), with a clip hold.
- Adding a machine: paste/enter an IP, or (future) pick from an auto-discovered list — design the
  affordance so the manual path now and the auto path later feel like the same control.
- Keep destructive/confirm actions (remove machine, change output device mid-stream) safe and
  obvious.
- The whole thing should feel quiet and dependable — this runs all day in the background. No
  gratuitous animation; motion only to communicate state (meters, a source going live/away).

### Visual direction
Modern, calm, "pro audio utility" — closer to a refined developer/menubar tool than a flashy
consumer app. Dark theme as default with a light theme. Tight, legible typography; a restrained
palette with one accent color used for "live/active" and meaningful color only for state (green =
healthy, amber = filling/degraded, red = clip/starving/error). Channel meters and sliders are the
visual centerpiece — make them feel precise and tactile. Respect native menubar conventions for
the popover. Accessible: meet WCAG AA contrast, full keyboard navigation, don't rely on color
alone for state (pair every color with an icon/label).

### Deliverables
1. A short **IA / interaction spec**: navigation model, the component inventory (channel strip,
   meter, volume slider, status pill, device picker, machine card, tray icon), and the state
   matrix (component × state).
2. **High-fidelity mockups** of the 8 key screens above, in a buildable form (React+Tailwind
   components preferred, or detailed SVG), including the tray icon state set.
3. A **design-token sheet**: color palette (with the state colors), typography scale, spacing,
   radii, and the meter/slider styling, so it can be implemented consistently in React.
4. Notes on **responsive/resize** behavior of the window and the fixed-width popover.

Optimize for: legibility of stream health at a glance, fast everyday volume/mute control from the
tray, and a calm, trustworthy feel. Start with the tray-popover Receiver-active screen — it's the
one the user sees most.

---

## How this maps to the engine (for whoever implements the mockups)

The React UI is pure presentation over the Go engine's telemetry. Real fields available today or
trivially exposable:

| UI element | Engine source |
|---|---|
| Per-stream level meter | peak/RMS computed in `mixer.PullMix` (to be added), per `mixer.Stream` |
| Volume slider | `mixer.Stream.SetGain(float32)` |
| Mute | `mixer.Stream.SetMuted(bool)` |
| Buffer health | `mixer.Stream.rb.Len()` vs prefill target |
| Drift correction | `drift.Controller` output per stream |
| Source sample rate | `transport.Header.Format.SampleRate` (carried per packet) |
| Output device picker | malgo `ctx.Devices(malgo.Playback)` |
| Role / port / peer | `config.Config` |
| Engine on/off | start/stop the `role` pipeline goroutine |

Telemetry (meters, buffer fill) is pushed to React via Wails events at ~30 Hz; commands (gain,
mute, device, role, start/stop) are Go methods bound to JS. No data lives in the frontend that the
engine doesn't own.
