// Package role wires the audio/transport/mixer components into the sender and
// receiver pipelines selected at runtime.
package role

import (
	"context"
	"fmt"
	"log"
	"os"
	"runtime"
	"time"

	"github.com/eplata/audiosync/internal/audio"
	"github.com/eplata/audiosync/internal/config"
	"github.com/eplata/audiosync/internal/discovery"
	"github.com/eplata/audiosync/internal/ring"
	"github.com/eplata/audiosync/internal/transport"
)

// RunSender captures audio and streams frames to the configured peer until ctx
// is cancelled. Phase 1 supports the "tone" source only.
func RunSender(ctx context.Context, cfg config.Config) error {
	format := cfg.Format()
	frameBytes := format.FrameBytes(cfg.FrameMs)

	backend, err := newCaptureBackend(cfg)
	if err != nil {
		return err
	}
	defer backend.Close()

	peer := cfg.Peer
	if cfg.Discover {
		log.Printf("sender: discovering receiver via mDNS...")
		p, derr := discovery.Discover(ctx, 10*time.Second)
		if derr != nil {
			return derr
		}
		log.Printf("sender: found receiver %q at %s", p.Name, p.Addr)
		peer = p.Addr
	}

	sender, err := transport.NewSender(peer, cfg.StreamID, frameBytes)
	if err != nil {
		return err
	}
	defer sender.Close()

	// Announce identity (hostname + platform) immediately and periodically so
	// the receiver can label this source.
	go announceIdentity(ctx, sender)

	// Decouple the capture thread from the network goroutine via a ring buffer.
	rb := ring.New(format.FrameBytes(cfg.BufferMs))
	if err := backend.Start(func(f audio.Frames) {
		rb.Write(f.Data)
	}); err != nil {
		return err
	}
	defer backend.Stop()

	log.Printf("sender: streaming id=%d source=%s -> %s (%d-byte frames)", cfg.StreamID, cfg.Source, peer, frameBytes)

	frame := make([]byte, frameBytes)
	var seq uint32
	for {
		if ctx.Err() != nil {
			return nil
		}
		if rb.Len() < frameBytes {
			// Wait for the capture thread to produce a full frame.
			select {
			case <-ctx.Done():
				return nil
			case <-time.After(200 * time.Microsecond):
			}
			continue
		}
		rb.Read(frame)
		if err := sender.SendFrame(seq, uint64(time.Now().UnixNano()), format, frame); err != nil {
			log.Printf("sender: send error: %v", err)
		}
		seq++
	}
}

// announceIdentity periodically sends this sender's hostname and platform so
// the receiver can show a real label instead of a generic "Machine N".
func announceIdentity(ctx context.Context, sender *transport.Sender) {
	name, _ := os.Hostname()
	if name == "" {
		name = "Unknown"
	}
	platform := currentPlatform()
	send := func() {
		if err := sender.SendMeta(platform, name); err != nil {
			log.Printf("sender: meta send error: %v", err)
		}
	}
	send()
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			send()
		}
	}
}

func currentPlatform() transport.Platform {
	switch runtime.GOOS {
	case "windows":
		return transport.PlatformWindows
	case "darwin":
		return transport.PlatformMacOS
	case "linux":
		return transport.PlatformLinux
	default:
		return transport.PlatformUnknown
	}
}

// newCaptureBackend selects the capture source. The OS loopback backends are
// added in later phases; for now only the cross-platform tone source exists.
func newCaptureBackend(cfg config.Config) (audio.CaptureBackend, error) {
	switch cfg.Source {
	case "tone":
		return audio.NewToneCapture(cfg.SampleRate, cfg.ToneHz, cfg.FrameMs), nil
	case "system":
		return audio.NewSystemCapture(cfg.SampleRate, cfg.FrameMs)
	default:
		return nil, fmt.Errorf("unknown source %q", cfg.Source)
	}
}
