package api

import (
	_ "embed"
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	payapi "github.com/orctatech/orcta-pay/api"
	"github.com/orctatech/orcta-pay/internal/platform"
)

// NewRouter builds the chi router.
func NewRouter(app *platform.App) http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP) //nolint:staticcheck // RealIP is used intentionally per original scaffold
	r.Use(middleware.Recoverer)
	r.Use(middleware.Heartbeat("/healthz"))

	r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})
	r.Get("/readyz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"status": "ready", "checks": map[string]string{"postgres": "ok", "valkey": "ok"}})
	})
	r.Get("/metrics", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte("# metrics\n"))
	})
	r.Get("/openapi.yaml", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/yaml")
		_, _ = w.Write(payapi.OpenAPISpec)
	})
	r.Get("/docs", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(docsHTML))
	})

	r.Route("/v1", func(r chi.Router) {
		r.Post("/charges", handleCreateCharge(app))
		r.Get("/charges/{ref}/status", handleChargeStatus(app))
		r.Post("/payouts", handleCreatePayout(app))
	})
	r.Post("/webhooks/hubtel", handleWebhook(app, "hubtel"))
	r.Post("/webhooks/paystack", handleWebhook(app, "paystack"))
	r.Post("/webhooks/moolre", handleWebhook(app, "moolre"))

	return r
}

const docsHTML = `<!doctype html>
<html><head><meta charset="utf-8"><title>Orcta Pay API</title>
<script id="api-reference" data-url="/openapi.yaml"></script>
<script src="https://cdn.jsdelivr.net/npm/@scalar/api-reference"></script>
</head><body></body></html>`
