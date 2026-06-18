// Command audiosync streams system audio from N sender machines to one
// receiver so a single set of headphones hears them all mixed together.
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/eplata/audiosync/internal/config"
	"github.com/eplata/audiosync/internal/role"
)

func main() {
	cfg, err := config.Parse(os.Args[1:])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := run(ctx, cfg); err != nil {
		log.Fatalf("audiosync: %v", err)
	}
}

func run(ctx context.Context, cfg config.Config) error {
	switch cfg.Role {
	case "receiver":
		return role.RunReceiver(ctx, cfg)
	case "sender":
		return role.RunSender(ctx, cfg)
	case "both":
		// Single-process loopback test: receiver + sender targeting it. If
		// either side returns, cancel the other so the process exits cleanly.
		ctx, cancel := context.WithCancel(ctx)
		defer cancel()
		errc := make(chan error, 2)
		go func() { errc <- role.RunReceiver(ctx, cfg) }()
		go func() { errc <- role.RunSender(ctx, cfg) }()
		err := <-errc
		cancel()
		<-errc
		return err
	default:
		return fmt.Errorf("invalid role %q", cfg.Role)
	}
}
