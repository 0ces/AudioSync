// Package audio defines the audio format model and the capture/playback
// backend abstractions used across AudioSync.
package audio

// SampleFormat identifies the binary layout of one PCM sample.
type SampleFormat uint8

const (
	// FormatS16LE is signed 16-bit little-endian PCM. Default wire/Phase-1 format.
	FormatS16LE SampleFormat = 0
	// FormatF32LE is 32-bit float little-endian PCM.
	FormatF32LE SampleFormat = 1
)

// BytesPerSample returns the size in bytes of a single sample in this format.
func (f SampleFormat) BytesPerSample() int {
	switch f {
	case FormatF32LE:
		return 4
	default:
		return 2
	}
}

// AudioFormat describes an interleaved PCM stream.
type AudioFormat struct {
	SampleRate uint32
	Channels   uint8
	Sample     SampleFormat
}

// DefaultFormat is the canonical Phase-1 format: 48kHz stereo S16LE.
var DefaultFormat = AudioFormat{SampleRate: 48000, Channels: 2, Sample: FormatS16LE}

// FrameBytes returns the byte size of `ms` milliseconds of audio in this format.
// One "frame" here means one full interleaved buffer of the given duration.
func (a AudioFormat) FrameBytes(ms int) int {
	samplesPerChan := int(a.SampleRate) * ms / 1000
	return samplesPerChan * int(a.Channels) * a.Sample.BytesPerSample()
}

// BytesPerFrame returns the byte size of one sample across all channels
// (i.e. one PCM frame in the audio-engineering sense).
func (a AudioFormat) BytesPerFrame() int {
	return int(a.Channels) * a.Sample.BytesPerSample()
}
