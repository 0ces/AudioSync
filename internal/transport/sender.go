package transport

import (
	"fmt"
	"net"

	"github.com/eplata/audiosync/internal/audio"
)

// Sender transmits PCM frames as UDP datagrams to a single receiver.
// Not safe for concurrent use: drive it from one goroutine.
type Sender struct {
	conn     *net.UDPConn
	streamID uint32
	buf      []byte // reused header+payload scratch (zero-alloc steady state)
}

// NewSender dials peer (host:port) and prepares a send buffer large enough for
// HeaderSize + maxPayload bytes.
func NewSender(peer string, streamID uint32, maxPayload int) (*Sender, error) {
	addr, err := net.ResolveUDPAddr("udp", peer)
	if err != nil {
		return nil, fmt.Errorf("resolve peer %q: %w", peer, err)
	}
	conn, err := net.DialUDP("udp", nil, addr)
	if err != nil {
		return nil, fmt.Errorf("dial %q: %w", peer, err)
	}
	return &Sender{
		conn:     conn,
		streamID: streamID,
		buf:      make([]byte, HeaderSize+maxPayload),
	}, nil
}

// SendFrame marshals one PCM frame with its header and writes it as a datagram.
func (s *Sender) SendFrame(seq uint32, tsNS uint64, format audio.AudioFormat, pcm []byte) error {
	if len(pcm) > len(s.buf)-HeaderSize {
		return fmt.Errorf("transport: payload %d exceeds buffer", len(pcm))
	}
	h := Header{StreamID: s.streamID, Seq: seq, TimestampNS: tsNS, Format: format}
	n := h.Marshal(s.buf)
	n += copy(s.buf[n:], pcm)
	_, err := s.conn.Write(s.buf[:n])
	return err
}

// SendMeta transmits a sender-identity metadata datagram.
func (s *Sender) SendMeta(platform Platform, name string) error {
	m := Meta{StreamID: s.streamID, Platform: platform, Name: name}
	if m.MetaSize() > len(s.buf) {
		return fmt.Errorf("transport: meta too large")
	}
	n := m.MarshalMeta(s.buf)
	_, err := s.conn.Write(s.buf[:n])
	return err
}

// Close releases the socket.
func (s *Sender) Close() error { return s.conn.Close() }
