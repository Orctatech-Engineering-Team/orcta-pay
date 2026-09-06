package api

import (
	"context"
	_ "embed"
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"

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
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-Request-Id"},
		ExposedHeaders:   []string{"X-Request-Id"},
		AllowCredentials: false,
		MaxAge:           300,
	}))

	r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})
	r.Get("/readyz", readinessHandler(app))
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
		r.Use(bearerAuth(app))
		r.Post("/charges", handleCreateCharge(app))
		r.Get("/charges", handleListCharges(app))
		r.Get("/charges/{ref}/status", handleChargeStatus(app))
		r.Post("/payouts", handleCreatePayout(app))
		r.Get("/payouts", handleListPayouts(app))
		r.Get("/ledger", handleListLedger(app))
		r.Get("/webhooks", handleListWebhooks(app))
		r.Get("/gateways/health", handleGatewayHealth(app))

		r.Post("/apps", handleCreateApp(app))
		r.Get("/apps", handleListApps(app))
		r.Get("/apps/{id}", handleGetApp(app))
		r.Post("/apps/{id}/keys/rotate", handleRotateAppKey(app))
		r.Delete("/apps/{id}", handleRevokeApp(app))
		// OpenAPI uses {appID}; support both forms.
		r.Get("/apps/{appID}", handleGetApp(app))
		r.Post("/apps/{appID}/keys/rotate", handleRotateAppKey(app))
		r.Delete("/apps/{appID}", handleRevokeApp(app))
	})
	r.Post("/webhooks/hubtel", handleWebhook(app, "hubtel"))
	r.Post("/webhooks/paystack", handleWebhook(app, "paystack"))
	r.Post("/webhooks/moolre", handleWebhook(app, "moolre"))

	return r
}

func readinessHandler(app *platform.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()

		checks := map[string]string{"postgres": "ok", "valkey": "ok"}
		if app.Pool == nil || app.Pool.Ping(ctx) != nil {
			checks["postgres"] = "unavailable"
		}
		if app.Valkey == nil || app.Valkey.Do(ctx, app.Valkey.B().Ping().Build()).Error() != nil {
			checks["valkey"] = "unavailable"
		}

		status := "ready"
		statusCode := http.StatusOK
		if checks["postgres"] != "ok" || checks["valkey"] != "ok" {
			status = "not_ready"
			statusCode = http.StatusServiceUnavailable
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(statusCode)
		_ = json.NewEncoder(w).Encode(map[string]any{"status": status, "checks": checks})
	}
}

const docsHTML = `<!doctype html>
<html><head><meta charset="utf-8"><title>Orcta Pay API</title>
<script id="api-reference" data-url="/openapi.yaml"></script>
<script src="https://cdn.jsdelivr.net/npm/@scalar/api-reference"></script>
</head><body></body></html>`
