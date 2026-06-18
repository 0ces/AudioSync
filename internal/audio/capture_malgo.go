//go:build windows || linux

package audio

import (
	"fmt"
	"runtime"
	"strings"
	"unsafe"

	"github.com/gen2brain/malgo"
)

// systemCapture captures system audio output via miniaudio (malgo):
//   - Windows: WASAPI loopback of the default output device.
//   - Linux:   the default sink's PulseAudio/PipeWire ".monitor" source.
//
// The malgo Data callback runs on miniaudio's capture thread and forwards the
// input buffer straight to the sink, which copies it into a ring buffer.
type systemCapture struct {
	ctx        *malgo.AllocatedContext
	device     *malgo.Device
	format     AudioFormat
	pendingCfg malgo.DeviceConfig
	monitorID  malgo.DeviceID // kept alive so pendingCfg.Capture.DeviceID stays valid
	sink       FrameSink
}

// NewSystemCapture creates the OS loopback capture backend for the current
// platform, requesting S16LE stereo at sampleRate.
func NewSystemCapture(sampleRate uint32, frameMs int) (CaptureBackend, error) {
	ctx, err := malgo.InitContext(nil, malgo.ContextConfig{}, nil)
	if err != nil {
		return nil, fmt.Errorf("capture: init context: %w", err)
	}
	c := &systemCapture{
		ctx:    ctx,
		format: AudioFormat{SampleRate: sampleRate, Channels: 2, Sample: FormatS16LE},
	}

	switch runtime.GOOS {
	case "windows":
		c.pendingCfg = malgo.DefaultDeviceConfig(malgo.Loopback)
	case "linux":
		c.pendingCfg = malgo.DefaultDeviceConfig(malgo.Capture)
		id, err := findMonitorDevice(ctx)
		if err != nil {
			_ = ctx.Uninit()
			ctx.Free()
			return nil, err
		}
		c.monitorID = id
		c.pendingCfg.Capture.DeviceID = unsafe.Pointer(&c.monitorID)
	default:
		_ = ctx.Uninit()
		ctx.Free()
		return nil, fmt.Errorf("capture: unsupported GOOS %q", runtime.GOOS)
	}
	c.pendingCfg.Capture.Format = malgo.FormatS16
	c.pendingCfg.Capture.Channels = uint32(c.format.Channels)
	c.pendingCfg.SampleRate = sampleRate
	c.pendingCfg.PeriodSizeInMilliseconds = uint32(frameMs)
	c.pendingCfg.PerformanceProfile = malgo.LowLatency
	return c, nil
}

func (c *systemCapture) Format() AudioFormat { return c.format }

func (c *systemCapture) Start(sink FrameSink) error {
	c.sink = sink
	device, err := malgo.InitDevice(c.ctx.Context, c.pendingCfg, malgo.DeviceCallbacks{
		Data: func(_, in []byte, _ uint32) {
			if len(in) > 0 && c.sink != nil {
				c.sink(Frames{Data: in, Format: c.format})
			}
		},
	})
	if err != nil {
		return fmt.Errorf("capture: init device: %w", err)
	}
	c.device = device
	if err := device.Start(); err != nil {
		return fmt.Errorf("capture: start: %w", err)
	}
	return nil
}

func (c *systemCapture) Stop() error {
	if c.device != nil {
		c.device.Uninit()
		c.device = nil
	}
	return nil
}

func (c *systemCapture) Close() error {
	if c.ctx != nil {
		_ = c.ctx.Uninit()
		c.ctx.Free()
		c.ctx = nil
	}
	return nil
}

// findMonitorDevice locates a Linux capture device that is a sink monitor
// (PulseAudio/PipeWire ".monitor"), preferring one tied to the default sink.
func findMonitorDevice(ctx *malgo.AllocatedContext) (malgo.DeviceID, error) {
	devices, err := ctx.Devices(malgo.Capture)
	if err != nil {
		return malgo.DeviceID{}, fmt.Errorf("capture: enumerate devices: %w", err)
	}
	var (
		fallback   malgo.DeviceID
		haveFallbk bool
	)
	for i := range devices {
		if !strings.Contains(strings.ToLower(devices[i].Name()), "monitor") {
			continue
		}
		if devices[i].IsDefault != 0 {
			return devices[i].ID, nil
		}
		if !haveFallbk {
			fallback, haveFallbk = devices[i].ID, true
		}
	}
	if haveFallbk {
		return fallback, nil
	}
	return malgo.DeviceID{}, fmt.Errorf("capture: no PulseAudio/PipeWire monitor source found; ensure pulseaudio or pipewire-pulse is running")
}
