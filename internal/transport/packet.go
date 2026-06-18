// Package transport defines the UDP wire format and the send/receive paths
// that carry PCM audio frames from senders to the receiver.
package transport

import (
	"encoding/binary"
	"errors"

	"github.com/eplata/audiosync/internal/audio"
)

// HeaderSize is the fixed byte size of a packet header preceding the PCM payload.
const HeaderSize = 24

// magic identifies an AudioSync audio datagram (version 1).
const magic uint32 = 0x41534E31 // "ASN1"

// metaMagic identifies a metadata datagram carrying sender identity.
const metaMagic uint32 = 0x41534D31 // "ASM1"

// Platform identifies a sender's operating system.
type Platform uint8

const (
	PlatformUnknown Platform = 0
	PlatformWindows Platform = 1
	PlatformMacOS   Platform = 2
	PlatformLinux   Platform = 3
)

// Meta is sender-identity metadata, sent periodically alongside the audio
// stream so the receiver can label each source with a real name and platform.
type Meta struct {
	StreamID uint32
	Platform Platform
	Name     string // hostname or user label, <=255 bytes UTF-8
}

// Kind reports whether buf is an audio packet, a meta packet, or unknown.
func Kind(buf []byte) string {
	if len(buf) < 4 {
		return "unknown"
	}
	switch binary.LittleEndian.Uint32(buf[0:4]) {
	case magic:
		return "audio"
	case metaMagic:
		return "meta"
	default:
		return "unknown"
	}
}

// MarshalMeta encodes the metadata into dst and returns the byte length used.
// Layout: magic(4) streamID(4) platform(1) nameLen(1) name(nameLen).
func (m Meta) MarshalMeta(dst []byte) int {
	name := m.Name
	if len(name) > 255 {
		name = name[:255]
	}
	binary.LittleEndian.PutUint32(dst[0:4], metaMagic)
	binary.LittleEndian.PutUint32(dst[4:8], m.StreamID)
	dst[8] = byte(m.Platform)
	dst[9] = byte(len(name))
	n := copy(dst[10:], name)
	return 10 + n
}

// MetaSize returns the marshalled size of this Meta.
func (m Meta) MetaSize() int { return 10 + min(len(m.Name), 255) }

// ParseMeta decodes a metadata datagram.
func ParseMeta(buf []byte) (Meta, error) {
	if len(buf) < 10 {
		return Meta{}, ErrShortPacket
	}
	if binary.LittleEndian.Uint32(buf[0:4]) != metaMagic {
		return Meta{}, ErrBadMagic
	}
	nameLen := int(buf[9])
	if len(buf) < 10+nameLen {
		return Meta{}, ErrShortPacket
	}
	return Meta{
		StreamID: binary.LittleEndian.Uint32(buf[4:8]),
		Platform: Platform(buf[8]),
		Name:     string(buf[10 : 10+nameLen]),
	}, nil
}

// ErrShortPacket is returned when a datagram is too small to contain a header.
var ErrShortPacket = errors.New("transport: short packet")

// ErrBadMagic is returned when the datagram is not an AudioSync v1 packet.
var ErrBadMagic = errors.New("transport: bad magic")

// Header is the per-datagram metadata. Layout (little-endian), 24 bytes:
//
//	[0:4]   magic
//	[4:8]   streamID
//	[8:12]  seq
//	[12:20] timestampNS
//	[20:22] sampleRate/100 (uint16, so 48000 -> 480)
//	[22]    channels
//	[23]    sampleFormat
type Header struct {
	StreamID    uint32
	Seq         uint32
	TimestampNS uint64
	Format      audio.AudioFormat
}

// Marshal writes the header into dst[:HeaderSize]. dst must be >= HeaderSize.
// Returns the number of bytes written. Zero-allocation.
func (h Header) Marshal(dst []byte) int {
	_ = dst[HeaderSize-1] // bounds-check hint
	binary.LittleEndian.PutUint32(dst[0:4], magic)
	binary.LittleEndian.PutUint32(dst[4:8], h.StreamID)
	binary.LittleEndian.PutUint32(dst[8:12], h.Seq)
	binary.LittleEndian.PutUint64(dst[12:20], h.TimestampNS)
	binary.LittleEndian.PutUint16(dst[20:22], uint16(h.Format.SampleRate/100))
	dst[22] = h.Format.Channels
	dst[23] = byte(h.Format.Sample)
	return HeaderSize
}

// ParseHeader decodes a header from the front of buf and returns the header
// plus the PCM payload as a subslice of buf (no copy).
func ParseHeader(buf []byte) (Header, []byte, error) {
	if len(buf) < HeaderSize {
		return Header{}, nil, ErrShortPacket
	}
	if binary.LittleEndian.Uint32(buf[0:4]) != magic {
		return Header{}, nil, ErrBadMagic
	}
	h := Header{
		StreamID:    binary.LittleEndian.Uint32(buf[4:8]),
		Seq:         binary.LittleEndian.Uint32(buf[8:12]),
		TimestampNS: binary.LittleEndian.Uint64(buf[12:20]),
		Format: audio.AudioFormat{
			SampleRate: uint32(binary.LittleEndian.Uint16(buf[20:22])) * 100,
			Channels:   buf[22],
			Sample:     audio.SampleFormat(buf[23]),
		},
	}
	return h, buf[HeaderSize:], nil
}
