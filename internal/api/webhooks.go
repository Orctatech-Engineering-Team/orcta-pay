package api

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/Orctatech-Engineering-Team/orcta-pay/internal/gateway"
	"github.com/Orctatech-Engineering-Team/orcta-pay/internal/observability"
	"github.com/Orctatech-Engineering-Team/orcta-pay/internal/platform"
	"github.com/Orctatech-Engineering-Team/orcta-pay/internal/webhooks"
)

func handleWebhook(app *platform.App, gatewayName string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxBodyBytes))
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_request", "body too large")
			return
		}
		defer func() { _ = r.Body.Close() }()

		if !verifyWebhook(app, gatewayName, r.Header, body) {
			writeError(w, http.StatusUnauthorized, "unauthorized", "bad signature")
			return
		}

		err = app.Webhooks.Process(r.Context(), gateway.Gateway(gatewayName), body)
		switch {
		case err == nil:
			writeJSON(w, http.StatusOK, map[string]any{"status": "processed"})
		case errors.Is(err, webhooks.ErrDuplicate):
			// Already recorded: ack 200 so the gateway stops retrying.
			writeJSON(w, http.StatusOK, map[string]any{"status": "duplicate"})
		case errors.Is(err, webhooks.ErrUnknownRef):
			// No matching intent (e.g. event for another environment). Ack so
			// the gateway stops retrying; the payload is dropped deliberately.
			observability.LoggerFromContext(r.Context()).WarnContext(r.Context(),
				"webhook for unknown charge reference", "gateway", gatewayName, "error", err)
			writeJSON(w, http.StatusOK, map[string]any{"status": "ignored"})
		case errors.Is(err, webhooks.ErrBadPayload):
			writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
		default:
			// Transient failure (verify or persist): 5xx so the gateway retries.
			observability.LoggerFromContext(r.Context()).ErrorContext(r.Context(),
				"webhook processing failed", "gateway", gatewayName, "error", err)
			writeError(w, http.StatusInternalServerError, "internal_error", "processing failed")
		}
	}
}

// verifyWebhook checks the payload signature. Standard Webhooks headers
// (webhook-id/timestamp/signature, HMAC-SHA256 base64) take precedence per
// spec; legacy per-gateway HMAC headers (X-Hubtel-Signature, X-Paystack-
// Signature) are accepted for backward compatibility. Moolre publishes no
// HMAC — webhook_inbox dedup is the source of truth.
func verifyWebhook(app *platform.App, gatewayName string, h http.Header, body []byte) bool {
	secret := webhookSecret(app, gatewayName)
	if secret == "" {
		return gatewayName == "moolre" // unconfigured secret: only moolre proceeds unverified
	}
	if id := h.Get("Webhook-Id"); id != "" {
		return webhooks.VerifyStandardWebhook(
			secret,
			id,
			h.Get("Webhook-Timestamp"),
			h.Get("Webhook-Signature"),
			body,
			time.Now(),
		)
	}
	sig := h.Get("X-Hubtel-Signature")
	if gatewayName == "paystack" {
		sig = h.Get("X-Paystack-Signature")
	}
	if sig != "" {
		return verifyHMAC(secret, body, sig)
	}
	// Secret configured but no recognizable signature header: reject.
	return false
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
