package transport

import (
	"bytes"
	"testing"

	"github.com/eplata/audiosync/internal/audio"
)

func TestHeaderRoundtrip(t *testing.T) {
	h := Header{
		StreamID:    42,
		Seq:         12345,
		TimestampNS: 9_876_543_210,
		Format:      audio.AudioFormat{SampleRate: 48000, Channels: 2, Sample: audio.FormatS16LE},
	}
	pcm := []byte("PCMPAYLOAD")
	buf := make([]byte, HeaderSize+len(pcm))
	n := h.Marshal(buf)
	if n != HeaderSize {
		t.Fatalf("Marshal=%d want %d", n, HeaderSize)
	}
	copy(buf[n:], pcm)

	got, payload, err := ParseHeader(buf)
	if err != nil {
		t.Fatalf("ParseHeader: %v", err)
	}
	if got != h {
		t.Fatalf("header roundtrip\n got=%+v\nwant=%+v", got, h)
	}
	if !bytes.Equal(payload, pcm) {
		t.Fatalf("payload=%q want %q", payload, pcm)
	}
}

func TestParseShortPacket(t *testing.T) {
	if _, _, err := ParseHeader(make([]byte, HeaderSize-1)); err != ErrShortPacket {
		t.Fatalf("err=%v want ErrShortPacket", err)
	}
}

func TestParseBadMagic(t *testing.T) {
	buf := make([]byte, HeaderSize)
	for i := range buf {
		buf[i] = 0xFF
	}
	if _, _, err := ParseHeader(buf); err != ErrBadMagic {
		t.Fatalf("err=%v want ErrBadMagic", err)
	}
}

func TestMetaRoundtrip(t *testing.T) {
	m := Meta{StreamID: 7, Platform: PlatformLinux, Name: "Gaming-PC"}
	buf := make([]byte, 1500)
	n := m.MarshalMeta(buf)
	if n != m.MetaSize() {
		t.Fatalf("MarshalMeta=%d MetaSize=%d", n, m.MetaSize())
	}
	if Kind(buf[:n]) != "meta" {
		t.Fatalf("Kind=%q want meta", Kind(buf[:n]))
	}
	got, err := ParseMeta(buf[:n])
	if err != nil {
		t.Fatal(err)
	}
	if got != m {
		t.Fatalf("meta roundtrip got %+v want %+v", got, m)
	}
}

func TestKindDistinguishesAudioAndMeta(t *testing.T) {
	ab := make([]byte, HeaderSize)
	Header{Format: audio.AudioFormat{SampleRate: 48000, Channels: 2}}.Marshal(ab)
	if Kind(ab) != "audio" {
		t.Fatalf("audio Kind=%q", Kind(ab))
	}
	mb := make([]byte, 64)
	n := Meta{StreamID: 1, Name: "x"}.MarshalMeta(mb)
	if Kind(mb[:n]) != "meta" {
		t.Fatalf("meta Kind=%q", Kind(mb[:n]))
	}
}

func TestSampleRateEncoding(t *testing.T) {
	// SampleRate is carried /100 in a uint16; verify common rates survive.
	for _, rate := range []uint32{44100, 48000, 96000} {
		h := Header{Format: audio.AudioFormat{SampleRate: rate, Channels: 2}}
		buf := make([]byte, HeaderSize)
		h.Marshal(buf)
		got, _, err := ParseHeader(buf)
		if err != nil {
			t.Fatal(err)
		}
		if got.Format.SampleRate != rate {
			t.Errorf("rate roundtrip=%d want %d", got.Format.SampleRate, rate)
		}
	}
}
