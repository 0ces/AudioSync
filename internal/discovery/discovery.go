// Package discovery advertises and finds AudioSync receivers on the LAN via
// mDNS (zeroconf), so senders can auto-connect without a manually configured
// peer address.
package discovery

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/grandcat/zeroconf"
)

const (
	service = "_audiosync._udp"
	domain  = "local."
)

// ErrNotFound is returned when no receiver is discovered before the timeout.
var ErrNotFound = errors.New("discovery: no AudioSync receiver found")

// Advertiser is a running mDNS advertisement; call Close to withdraw it.
type Advertiser struct{ server *zeroconf.Server }

// Advertise announces a receiver named instance listening on port. The returned
// Advertiser must be Closed when the receiver stops.
func Advertise(instance string, port int) (*Advertiser, error) {
	if instance == "" {
		instance = "AudioSync"
	}
	server, err := zeroconf.Register(instance, service, domain, port, []string{"audiosync=1"}, nil)
	if err != nil {
		return nil, fmt.Errorf("discovery: advertise: %w", err)
	}
	return &Advertiser{server: server}, nil
}

// Close withdraws the advertisement.
func (a *Advertiser) Close() {
	if a != nil && a.server != nil {
		a.server.Shutdown()
	}
}

// Peer is a discovered receiver.
type Peer struct {
	Name string
	Addr string // host:port
}

// Discover browses for a receiver, returning the first one found or ErrNotFound
// after timeout.
func Discover(ctx context.Context, timeout time.Duration) (Peer, error) {
	resolver, err := zeroconf.NewResolver(nil)
	if err != nil {
		return Peer{}, fmt.Errorf("discovery: resolver: %w", err)
	}
	cctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	entries := make(chan *zeroconf.ServiceEntry)
	if err := resolver.Browse(cctx, service, domain, entries); err != nil {
		return Peer{}, fmt.Errorf("discovery: browse: %w", err)
	}
	for {
		select {
		case e, ok := <-entries:
			if !ok {
				return Peer{}, ErrNotFound
			}
			if len(e.AddrIPv4) > 0 {
				return Peer{Name: e.Instance, Addr: fmt.Sprintf("%s:%d", e.AddrIPv4[0].String(), e.Port)}, nil
			}
		case <-cctx.Done():
			return Peer{}, ErrNotFound
		}
	}
}
