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
	"github.com/Orctatech-Engineering-Team/orcta-pay/internal/worker"
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

func buildWorker(app *platform.App) *worker.Worker {
	var store worker.OutboxStore
	var ledger worker.LedgerReader
	if app.PgStore != nil {
		store = app.PgStore
		ledger = app.PgStore
	} else if app.MemStore != nil {
		store = app.MemStore
		ledger = app.MemStore
	} else {
		return nil
	}
	return worker.New(store, ledger, app.Router, app.Locker, app.Observ.Logger)
}

func drainOutbox(ctx context.Context, app *platform.App) {
	w := buildWorker(app)
	if w == nil {
		app.Observ.Logger.WarnContext(ctx, "worker outbox skipped: no store")
		return
	}
	if _, err := w.DrainOutbox(ctx); err != nil {
		app.Observ.Logger.ErrorContext(ctx, "outbox drain failed", "error", err)
	}
}

func reconcile(ctx context.Context, app *platform.App) {
	w := buildWorker(app)
	if w == nil {
		app.Observ.Logger.WarnContext(ctx, "reconciliation skipped: no store")
		return
	}
	if err := w.Reconcile(ctx); err != nil {
		app.Observ.Logger.ErrorContext(ctx, "reconciliation failed", "error", err)
	}
}
