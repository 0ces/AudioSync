package transport

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/eplata/audiosync/internal/audio"
)

// TestSenderReceiverLoopback streams frames through real UDP sockets on
// localhost and verifies the receiver observes them in order with intact PCM.
func TestSenderReceiverLoopback(t *testing.T) {
	recv, err := NewReceiver("127.0.0.1:0", 2048)
	if err != nil {
		t.Fatal(err)
	}
	defer recv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	const frames = 50
	var (
		mu       sync.Mutex
		gotSeq   []uint32
		gotFirst []byte
	)
	done := make(chan struct{})
	go func() {
		_ = recv.Serve(ctx, Handlers{Audio: func(h Header, pcm []byte) {
			mu.Lock()
			gotSeq = append(gotSeq, h.Seq)
			if len(gotSeq) == 1 {
				gotFirst = append([]byte(nil), pcm...)
			}
			n := len(gotSeq)
			mu.Unlock()
			if n == frames {
				close(done)
			}
		}})
	}()

	sender, err := NewSender(recv.LocalAddr().String(), 7, 1024)
	if err != nil {
		t.Fatal(err)
	}
	defer sender.Close()

	format := audio.AudioFormat{SampleRate: 48000, Channels: 2, Sample: audio.FormatS16LE}
	payload := []byte{0xDE, 0xAD, 0xBE, 0xEF}
	for i := range frames {
		if err := sender.SendFrame(uint32(i), uint64(i)*1000, format, payload); err != nil {
			t.Fatal(err)
		}
		time.Sleep(time.Millisecond) // pace so UDP buffers don't drop on localhost
	}

	select {
	case <-done:
	case <-time.After(3 * time.Second):
		mu.Lock()
		t.Fatalf("timeout: received %d/%d frames", len(gotSeq), frames)
	}

	mu.Lock()
	defer mu.Unlock()
	for i, seq := range gotSeq {
		if seq != uint32(i) {
			t.Fatalf("seq[%d]=%d, out of order", i, seq)
		}
	}
	if string(gotFirst) != string(payload) {
		t.Fatalf("payload=%v want %v", gotFirst, payload)
	}
}
