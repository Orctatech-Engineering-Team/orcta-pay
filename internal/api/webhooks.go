package api

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"

	"github.com/orctatech/orcta-pay/internal/observability"
	"github.com/orctatech/orcta-pay/internal/platform"
)

func handleWebhook(app *platform.App, gateway string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxBodyBytes))
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_request", "body too large")
			return
		}
		defer func() { _ = r.Body.Close() }()

		secret := webhookSecret(app, gateway)
		if gateway != "moolre" && secret != "" {
			sig := r.Header.Get("X-Hubtel-Signature")
			if gateway == "paystack" {
				sig = r.Header.Get("X-Paystack-Signature")
			}
			if !verifyHMAC(secret, body, sig) {
				writeError(w, http.StatusUnauthorized, "unauthorized", "bad signature")
				return
			}
		}
		// Moolre has no published HMAC header — dedup via webhook_inbox is the source of truth.
		// Dedup via webhook_inbox unique on aggregator_event_id — stub always accepts.
		// Real path: insert, on conflict return 200, else call GetTransactionStatus and write ledger in same Tx.
		_ = body
		observability.LoggerFromContext(r.Context()).InfoContext(r.Context(), "webhook received", "gateway", gateway)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}
}

func webhookSecret(app *platform.App, gateway string) string {
	switch gateway {
	case "hubtel":
		return app.Config.Payments.WebhookSecrets.Hubtel
	case "paystack":
		return app.Config.Payments.WebhookSecrets.Paystack
	case "moolre":
		return app.Config.Payments.WebhookSecrets.Moolre
	default:
		return ""
	}
}

func verifyHMAC(secret string, body []byte, signature string) bool {
	if secret == "" || signature == "" {
		return false
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	expected := hex.EncodeToString(mac.Sum(nil))
	// Prefer hex; also accept raw hex comparison with constant-time.
	if len(signature) == len(expected) {
		return hmac.Equal([]byte(signature), []byte(expected))
	}
	// Some providers send "sha256=hex"
	const prefix = "sha256="
	if len(signature) > len(prefix) && signature[:len(prefix)] == prefix {
		return hmac.Equal([]byte(signature[len(prefix):]), []byte(expected))
	}
	return false
}
