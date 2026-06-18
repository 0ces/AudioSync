# Changelog

All notable changes to AudioSync are documented here. The format follows
[Keep a Changelog](https://keepachangelog.com/), and the project aims to follow
[Semantic Versioning](https://semver.org/).

## [Unreleased]

## [0.1.1] - 2026-06-18

### Fixed
- Corrected the macOS first-run instructions: the right-click → Open Gatekeeper
  bypass was removed in macOS 15, so docs/release notes now use quarantine removal
  (`xattr -dr com.apple.quarantine`) or System Settings → Open Anyway.

### Added
- `scripts/macos-first-run.sh` to de-quarantine and open a downloaded build.
- Dormant Developer ID signing + notarization path in the release workflow,
  activated by adding the documented macOS signing secrets.

## [0.1.0] - 2026-06-18

First tagged release. Stream system audio from N computers to one set of
headphones, over the LAN.

### Engine (Go)
- Realtime pipeline: lock-free SPSC ring buffers isolate the audio threads from
  Go's GC; raw PCM over UDP; per-stream jitter buffers with loss concealment.
- OS loopback capture: WASAPI (Windows), PulseAudio/PipeWire monitor (Linux),
  CoreAudio process tap (macOS 14.2+, via a cgo bridge).
- Per-stream linear resampler + PI drift controller — handles mismatched sample
  rates and clock drift between machines.
- N-stream mixer with per-source gain / mute / solo and master gain.
- Sender identity (hostname + platform) via a metadata packet.
- mDNS discovery: receiver advertises, sender auto-finds it (`-discover`).

### Desktop app (Wails v2 + React/TS)
- Live mixer (metering, faders, mute/solo), Connections, Diagnostics, Settings
  (latency profile, dark/light theme), output-device picker, Add-machine helper.
- Menu-bar tray; closing the window hides to tray.

### Known limitations
- macOS verified end-to-end; Windows/Linux capture is compile-correct but
  untested on their own hardware.
- macOS release artifacts are unsigned; on macOS 15+ open them via
  `xattr -dr com.apple.quarantine` or System Settings → Open Anyway.
- Tray opens the full window rather than a compact native popover panel.

[Unreleased]: https://github.com/0ces/AudioSync/compare/v0.1.1...HEAD
[0.1.1]: https://github.com/0ces/AudioSync/compare/v0.1.0...v0.1.1
[0.1.0]: https://github.com/0ces/AudioSync/releases/tag/v0.1.0
