package api

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/Orctatech-Engineering-Team/orcta-pay/internal/config"
	"github.com/Orctatech-Engineering-Team/orcta-pay/internal/gateway"
	"github.com/Orctatech-Engineering-Team/orcta-pay/internal/money"
	"github.com/Orctatech-Engineering-Team/orcta-pay/internal/platform"
	postgres "github.com/Orctatech-Engineering-Team/orcta-pay/internal/storage/postgres"
	"github.com/Orctatech-Engineering-Team/orcta-pay/internal/webhooks"
)

func TestVerifyPaystackHMAC(t *testing.T) {
	secret := "sk_test_example"
	body := []byte(`{"event":"charge.success"}`)
	mac := hmac.New(sha512.New, []byte(secret))
	_, _ = mac.Write(body)
	signature := hex.EncodeToString(mac.Sum(nil))

	if !verifyPaystackHMAC(secret, body, signature) {
		t.Fatal("valid Paystack signature was rejected")
	}
	if verifyPaystackHMAC(secret, body, "invalid") {
		t.Fatal("invalid Paystack signature was accepted")
	}
}

func TestVerifyMoolreRequiresStandardWebhookWhenSecretSet(t *testing.T) {
	secret := "whsec_test_moolre_secret_123456"
	body := []byte(`{"transactionid":"txn_123","externalref":"optd-01ARZ3NDEKTSV4RRFFQ69G5FAV"}`)

	app := &platform.App{Config: config.Config{
		Payments: config.PaymentsConfig{WebhookSecrets: config.WebhookSecrets{Moolre: secret}},
	}}

	// Valid Standard Webhooks signature must be accepted.
	nowTS := time.Now()
	id := "msg_now"
	sigNow := webhooks.SignStandardWebhook(secret, id, nowTS, body)
	hValid := http.Header{}
	hValid.Set("Webhook-Id", id)
	hValid.Set("Webhook-Timestamp", strconv.FormatInt(nowTS.Unix(), 10))
	hValid.Set("Webhook-Signature", sigNow)
	if !verifyWebhook(app, "moolre", hValid, body) {
		t.Fatal("valid Standard Webhooks signature for moolre was rejected")
	}

	// Missing Standard Webhooks headers must be rejected when secret is set.
	hMissing := http.Header{}
	if verifyWebhook(app, "moolre", hMissing, body) {
		t.Fatal("missing Standard Webhooks headers accepted for moolre with secret")
	}

	// Legacy HMAC header must NOT be accepted for moolre even if valid.
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	legacySig := hex.EncodeToString(mac.Sum(nil))
	hLegacy := http.Header{}
	hLegacy.Set("X-Hubtel-Signature", legacySig)
	if verifyWebhook(app, "moolre", hLegacy, body) {
		t.Fatal("legacy HMAC signature accepted for moolre; must require Standard Webhooks")
	}
	hLegacy2 := http.Header{}
	hLegacy2.Set("X-Hubtel-Signature", "sha256="+legacySig)
	if verifyWebhook(app, "moolre", hLegacy2, body) {
		t.Fatal("legacy sha256= HMAC accepted for moolre; must require Standard Webhooks")
	}

	// Tampered payload must be rejected.
	hTamper := http.Header{}
	hTamper.Set("Webhook-Id", id)
	hTamper.Set("Webhook-Timestamp", strconv.FormatInt(nowTS.Unix(), 10))
	hTamper.Set("Webhook-Signature", sigNow)
	if verifyWebhook(app, "moolre", hTamper, []byte(`{"transactionid":"txn_999","externalref":"optd-tampered"}`)) {
		t.Fatal("tampered payload accepted for moolre")
	}
}

func TestMoolreWebhookHandlerAuth(t *testing.T) {
	secret := "whsec_test_handler_secret_123"
	body := []byte(`{"transactionid":"txn_abc","externalref":"optd-01ARZ3NDEKTSV4RRFFQ69G5FAV"}`)

	newApp := func(env config.Environment, moolreSecret string) *platform.App {
		store := postgres.NewMemoryStore()
		adapter := &moolreFakeAdapter{}
		router := gateway.NewChargerRouter(
			config.PaymentsConfig{Primary: "moolre"},
			nil,
			map[gateway.Gateway]gateway.AggregatorClient{gateway.GatewayMoolre: adapter},
		)
		return &platform.App{
			Config: config.Config{
				Environment: env,
				Payments:    config.PaymentsConfig{WebhookSecrets: config.WebhookSecrets{Moolre: moolreSecret}},
			},
			Webhooks: webhooks.NewService(store, router),
		}
	}

	t.Run("production empty secret rejects with 401", func(t *testing.T) {
		app := newApp(config.EnvProduction, "")
		handler := handleWebhook(app, "moolre")
		req := httptest.NewRequest(http.MethodPost, "/webhooks/moolre", bytes.NewReader(body))
		res := httptest.NewRecorder()
		handler.ServeHTTP(res, req)
		if res.Code != http.StatusUnauthorized {
			t.Fatalf("status = %d, want %d", res.Code, http.StatusUnauthorized)
		}
	})

	t.Run("development empty secret allows without signature", func(t *testing.T) {
		app := newApp(config.EnvDevelopment, "")
		handler := handleWebhook(app, "moolre")
		req := httptest.NewRequest(http.MethodPost, "/webhooks/moolre", bytes.NewReader(body))
		res := httptest.NewRecorder()
		handler.ServeHTTP(res, req)
		if res.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d (dev should allow without secret)", res.Code, http.StatusOK)
		}
	})

	t.Run("staging empty secret allows without signature", func(t *testing.T) {
		app := newApp(config.EnvStaging, "")
		handler := handleWebhook(app, "moolre")
		req := httptest.NewRequest(http.MethodPost, "/webhooks/moolre", bytes.NewReader(body))
		res := httptest.NewRecorder()
		handler.ServeHTTP(res, req)
		if res.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d (staging should allow without secret)", res.Code, http.StatusOK)
		}
	})

	t.Run("secret set missing signature rejects with 401", func(t *testing.T) {
		app := newApp(config.EnvDevelopment, secret)
		handler := handleWebhook(app, "moolre")
		req := httptest.NewRequest(http.MethodPost, "/webhooks/moolre", bytes.NewReader(body))
		res := httptest.NewRecorder()
		handler.ServeHTTP(res, req)
		if res.Code != http.StatusUnauthorized {
			t.Fatalf("status = %d, want %d", res.Code, http.StatusUnauthorized)
		}
	})

	t.Run("secret set valid Standard Webhooks signature allows", func(t *testing.T) {
		app := newApp(config.EnvDevelopment, secret)
		handler := handleWebhook(app, "moolre")
		ts := time.Now()
		id := "msg_handler_1"
		sig := webhooks.SignStandardWebhook(secret, id, ts, body)
		req := httptest.NewRequest(http.MethodPost, "/webhooks/moolre", bytes.NewReader(body))
		req.Header.Set("Webhook-Id", id)
		req.Header.Set("Webhook-Timestamp", strconv.FormatInt(ts.Unix(), 10))
		req.Header.Set("Webhook-Signature", sig)
		res := httptest.NewRecorder()
		handler.ServeHTTP(res, req)
		if res.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d with valid signature", res.Code, http.StatusOK)
		}
	})

	t.Run("secret set invalid signature rejects with 401", func(t *testing.T) {
		app := newApp(config.EnvDevelopment, secret)
		handler := handleWebhook(app, "moolre")
		ts := time.Now()
		id := "msg_handler_2"
		req := httptest.NewRequest(http.MethodPost, "/webhooks/moolre", bytes.NewReader(body))
		req.Header.Set("Webhook-Id", id)
		req.Header.Set("Webhook-Timestamp", strconv.FormatInt(ts.Unix(), 10))
		req.Header.Set("Webhook-Signature", "v1,invalidsignaturebase64==")
		res := httptest.NewRecorder()
		handler.ServeHTTP(res, req)
		if res.Code != http.StatusUnauthorized {
			t.Fatalf("status = %d, want %d for invalid signature", res.Code, http.StatusUnauthorized)
		}
	})

	t.Run("secret set production still requires valid signature", func(t *testing.T) {
		app := newApp(config.EnvProduction, secret)
		handler := handleWebhook(app, "moolre")
		req := httptest.NewRequest(http.MethodPost, "/webhooks/moolre", bytes.NewReader(body))
		res := httptest.NewRecorder()
		handler.ServeHTTP(res, req)
		if res.Code != http.StatusUnauthorized {
			t.Fatalf("status = %d, want 401 when secret set but no signature in prod", res.Code)
		}
	})
}

// moolreFakeAdapter is a minimal gateway.AggregatorClient for handler tests.

type moolreFakeAdapter struct{}

func (a *moolreFakeAdapter) Initiate(_ context.Context, _ gateway.InitiateRequest) (gateway.InitiateResponse, error) {
	return gateway.InitiateResponse{}, nil
}

func (a *moolreFakeAdapter) Verify(_ context.Context, ref string) (gateway.VerifyResult, error) {
	return gateway.VerifyResult{Reference: ref, Status: "succeeded", Amount: money.New(100, money.GHS), VerifiedAt: time.Now().UTC()}, nil
}

func (a *moolreFakeAdapter) Refund(_ context.Context, _ string, _ money.Money) error { return nil }

func (a *moolreFakeAdapter) Payout(_ context.Context, _ string, _ money.Money, _ string) error {
	return nil
}
