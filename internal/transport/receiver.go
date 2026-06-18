package transport

import (
	"context"
	"errors"
	"fmt"
	"net"
	"time"
)

// PacketHandler is invoked for each valid audio datagram. pcm aliases the
// receive buffer and is only valid for the duration of the call — copy it (e.g.
// into a jitter buffer) if it must outlive the handler. Runs on the receive
// goroutine, not the audio thread.
type PacketHandler func(h Header, pcm []byte)

// MetaHandler is invoked for each valid metadata datagram (sender identity).
type MetaHandler func(m Meta)

// Handlers bundles the per-kind datagram callbacks. Meta may be nil.
type Handlers struct {
	Audio PacketHandler
	Meta  MetaHandler
}

// Receiver reads AudioSync datagrams from a UDP socket.
type Receiver struct {
	conn *net.UDPConn
	buf  []byte
}

// NewReceiver binds a UDP socket on listenAddr (e.g. ":4010"). maxDatagram
// bounds the receive buffer.
func NewReceiver(listenAddr string, maxDatagram int) (*Receiver, error) {
	addr, err := net.ResolveUDPAddr("udp", listenAddr)
	if err != nil {
		return nil, fmt.Errorf("resolve listen %q: %w", listenAddr, err)
	}
	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		return nil, fmt.Errorf("listen %q: %w", listenAddr, err)
	}
	return &Receiver{conn: conn, buf: make([]byte, maxDatagram)}, nil
}

// LocalAddr reports the bound address.
func (r *Receiver) LocalAddr() net.Addr { return r.conn.LocalAddr() }

// Serve reads datagrams until ctx is cancelled, dispatching each to the matching
// handler by packet kind. Malformed datagrams are skipped.
func (r *Receiver) Serve(ctx context.Context, h Handlers) error {
	go func() {
		<-ctx.Done()
		_ = r.conn.Close()
	}()
	for {
		// Periodic deadline so ctx cancellation is observed even when idle.
		_ = r.conn.SetReadDeadline(time.Now().Add(time.Second))
		n, _, err := r.conn.ReadFromUDP(r.buf)
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			var ne net.Error
			if errors.As(err, &ne) && ne.Timeout() {
				continue
			}
			return err
		}
		switch Kind(r.buf[:n]) {
		case "audio":
			if hdr, pcm, perr := ParseHeader(r.buf[:n]); perr == nil && h.Audio != nil {
				h.Audio(hdr, pcm)
			}
		case "meta":
			if m, perr := ParseMeta(r.buf[:n]); perr == nil && h.Meta != nil {
				h.Meta(m)
			}
		}
	}
}

// Close releases the socket.
func (r *Receiver) Close() error { return r.conn.Close() }
