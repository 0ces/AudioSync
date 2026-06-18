// Package config parses AudioSync's command-line configuration.
package config

import (
	"flag"
	"fmt"

	"github.com/eplata/audiosync/internal/audio"
)

// Config is the resolved runtime configuration.
type Config struct {
	Role     string // "sender", "receiver", or "both"
	Peer     string // receiver address for a sender (host:port)
	Listen   string // bind address for a receiver (host:port)
	StreamID uint32 // identifies this sender's stream at the mixer
	Source   string // "tone" (Phase 1) or "system" (OS loopback, later)
	Discover bool   // sender: auto-find the receiver via mDNS (ignores -peer)

	SampleRate uint32
	FrameMs    int // packet/period size
	PrefillMs  int // jitter-buffer prefill before playout begins
	BufferMs   int // jitter-buffer capacity
	ToneHz     float64
}

// Format derives the canonical AudioFormat from the configured sample rate
// (stereo S16LE in Phase 1).
func (c Config) Format() audio.AudioFormat {
	return audio.AudioFormat{SampleRate: c.SampleRate, Channels: 2, Sample: audio.FormatS16LE}
}

// Parse reads flags and validates them.
func Parse(args []string) (Config, error) {
	fs := flag.NewFlagSet("audiosync", flag.ContinueOnError)
	var c Config
	fs.StringVar(&c.Role, "role", "both", "sender | receiver | both")
	fs.StringVar(&c.Peer, "peer", "127.0.0.1:4010", "receiver address (sender mode)")
	fs.StringVar(&c.Listen, "listen", ":4010", "bind address (receiver mode)")
	id := fs.Uint("id", 1, "stream id for this sender")
	fs.StringVar(&c.Source, "source", "tone", "audio source: tone | system")
	fs.BoolVar(&c.Discover, "discover", false, "sender: auto-find receiver via mDNS (ignores -peer)")
	rate := fs.Uint("rate", 48000, "sample rate (Hz)")
	fs.IntVar(&c.FrameMs, "frame-ms", 5, "frame/period size in ms")
	fs.IntVar(&c.PrefillMs, "prefill-ms", 20, "jitter buffer prefill in ms")
	fs.IntVar(&c.BufferMs, "buffer-ms", 200, "jitter buffer capacity in ms")
	fs.Float64Var(&c.ToneHz, "tone-hz", 440, "tone frequency for the tone source")
	if err := fs.Parse(args); err != nil {
		return Config{}, err
	}
	c.StreamID = uint32(*id)
	c.SampleRate = uint32(*rate)

	switch c.Role {
	case "sender", "receiver", "both":
	default:
		return Config{}, fmt.Errorf("invalid role %q", c.Role)
	}
	if c.FrameMs <= 0 || c.PrefillMs < 0 || c.BufferMs <= 0 {
		return Config{}, fmt.Errorf("invalid timing: frame=%d prefill=%d buffer=%d", c.FrameMs, c.PrefillMs, c.BufferMs)
	}
	return c, nil
}
