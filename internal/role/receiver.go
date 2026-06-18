package role

import (
	"context"
	"log"

	"github.com/eplata/audiosync/engine"
	"github.com/eplata/audiosync/internal/config"
)

// RunReceiver starts the receiver pipeline via the engine facade and blocks
// until ctx is cancelled. The desktop UI drives the same engine directly.
func RunReceiver(ctx context.Context, cfg config.Config) error {
	e := engine.New(cfg)
	if err := e.StartReceiver(); err != nil {
		return err
	}
	defer e.Stop()
	log.Printf("receiver: listening on %s, playing mixed output", e.Snapshot().ListenAddr)
	<-ctx.Done()
	return nil
}
