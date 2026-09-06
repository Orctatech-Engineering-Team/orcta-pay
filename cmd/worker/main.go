package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Orctatech-Engineering-Team/orcta-pay/internal/config"
	"github.com/Orctatech-Engineering-Team/orcta-pay/internal/platform"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	app, err := platform.Build(ctx, cfg)
	if err != nil {
		return err
	}
	defer app.Close()

	app.Observ.Logger.InfoContext(ctx, "worker started")
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			app.Observ.Logger.InfoContext(ctx, "worker shutting down")
			return nil
		case <-ticker.C:
			jobCtx, jobCancel := context.WithTimeout(ctx, cfg.Worker.JobTimeout)
			drainOutbox(jobCtx, app)
			reconcile(jobCtx, app)
			jobCancel()
		}
	}
}

func drainOutbox(ctx context.Context, app *platform.App) {
	// Real: SELECT FOR UPDATE SKIP LOCKED from gateway_events where status pending,
	// then Initiate/Verify with Valkey SETNX guard.
	app.Observ.Logger.InfoContext(ctx, "outbox drain tick")
}

func reconcile(ctx context.Context, app *platform.App) {
	// Real: daily diff aggregator statements vs ledger_entries; mismatches alert.
	app.Observ.Logger.InfoContext(ctx, "reconciliation tick")
}
