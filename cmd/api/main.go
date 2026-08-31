package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/orctatech/orcta-pay/internal/api"
	"github.com/orctatech/orcta-pay/internal/config"
	"github.com/orctatech/orcta-pay/internal/platform"
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
	ctx := context.Background()
	app, err := platform.Build(ctx, cfg)
	if err != nil {
		return err
	}
	defer app.Close()

	router := api.NewRouter(app)
	srv := &http.Server{
		Addr:         cfg.HTTP.Addr(),
		Handler:      router,
		ReadTimeout:  cfg.HTTP.ReadTimeout,
		WriteTimeout: cfg.HTTP.WriteTimeout,
		IdleTimeout:  cfg.HTTP.IdleTimeout,
	}
	errCh := make(chan error, 1)
	go func() {
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	select {
	case sig := <-quit:
		app.Observ.Logger.Info("shutdown signal", "signal", sig.String())
	case err := <-errCh:
		return err
	}
	ctxShutdown, cancel := context.WithTimeout(context.Background(), cfg.HTTP.ShutdownTimeout)
	defer cancel()
	// Give in-flight requests time to finish; prefer handler timeout over hard cut.
	time.Sleep(100 * time.Millisecond)
	if err := srv.Shutdown(ctxShutdown); err != nil {
		return err
	}
	return nil
}
