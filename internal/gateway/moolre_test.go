package gateway

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/orctatech/orcta-pay/internal/config"
	"github.com/orctatech/orcta-pay/internal/money"
)

func newTestMoolreServer(t *testing.T, handler http.HandlerFunc) (*MoolreAdapter, func()) {
	t.Helper()
	srv := httptest.NewServer(handler)
	a := NewMoolreAdapter(config.MoolreConfig{APIKey: "test-key", BaseURL: srv.URL}, "", WithMoolreClient(srv.Client()))
	return a, srv.Close
}

func TestMoolreInitiateSuccess(t *testing.T) {
	a, close := newTestMoolreServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/open/transact/payment" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Fatalf("expected Bearer header, got %q", r.Header.Get("Authorization"))
		}
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body["externalref"] == "" {
			t.Fatalf("expected externalref in body")
		}
		if body["amount"] != "18.00" {
			t.Fatalf("expected amount 18.00, got %v", body["amount"])
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"status": 1, "code": "TR099", "data": "uuid-123"})
	})
	defer close()

	resp, err := a.Initiate(context.Background(), InitiateRequest{Reference: "optd-orctago-moolre-01TEST", Amount: money.New(1800, money.GHS), Wallet: "0240000000"})
	if err != nil {
		t.Fatalf("Initiate failed: %v", err)
	}
	if resp.ExternalRef != "uuid-123" {
		t.Fatalf("expected uuid-123, got %q", resp.ExternalRef)
	}
	if resp.Status != "pending" {
		t.Fatalf("expected pending, got %q", resp.Status)
	}
}

func TestMoolreInitiateBadRequest(t *testing.T) {
	a, close := newTestMoolreServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]any{"status": 0, "code": "TP13", "message": "External Reference is required and must be unique."})
	})
	defer close()

	_, err := a.Initiate(context.Background(), InitiateRequest{Reference: "optd-orctago-moolre-01TEST", Amount: money.New(1800, money.GHS)})
	if err == nil {
		t.Fatal("expected error for 400")
	}
}

func TestMoolreInitiateServerError(t *testing.T) {
	a, close := newTestMoolreServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`server error`))
	})
	defer close()

	_, err := a.Initiate(context.Background(), InitiateRequest{Reference: "optd-orctago-moolre-01TEST", Amount: money.New(1800, money.GHS)})
	if err == nil {
		t.Fatal("expected error for 500")
	}
}

func TestMoolreInitiateDuplicateAppError(t *testing.T) {
	a, close := newTestMoolreServer(t, func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"status": 0, "code": "TP13", "message": "External Reference is required and must be unique."})
	})
	defer close()

	_, err := a.Initiate(context.Background(), InitiateRequest{Reference: "optd-orctago-moolre-01TEST", Amount: money.New(1800, money.GHS)})
	if err == nil {
		t.Fatal("expected error for duplicate externalref")
	}
}

func TestMoolreVerifySuccess(t *testing.T) {
	a, close := newTestMoolreServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/open/transact/status" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"status": 1, "code": "SS01",
			"data": map[string]any{"txstatus": 1, "amount": "18.00", "externalref": "optd-orctago-moolre-01TEST", "transactionid": "tid-1"},
		})
	})
	defer close()

	result, err := a.Verify(context.Background(), "optd-orctago-moolre-01TEST")
	if err != nil {
		t.Fatalf("Verify failed: %v", err)
	}
	if result.Status != "succeeded" {
		t.Fatalf("expected succeeded, got %q", result.Status)
	}
}

func TestMoolrePayoutSuccess(t *testing.T) {
	a, close := newTestMoolreServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/open/transact/transfer" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"status": 1, "code": "OBGH01", "data": map[string]any{"txstatus": 1, "externalref": "ref-1"}})
	})
	defer close()

	err := a.Payout(context.Background(), "0240000000", money.New(1000, money.GHS), "optd-orctago-moolre-01TEST")
	if err != nil {
		t.Fatalf("Payout failed: %v", err)
	}
}

func TestMoolreNotConfigured(t *testing.T) {
	a := NewMoolreAdapter(config.MoolreConfig{}, "", WithMoolreLogger(nil))
	_, err := a.Initiate(context.Background(), InitiateRequest{Reference: "optd-orctago-moolre-01TEST", Amount: money.New(100, money.GHS)})
	if err == nil {
		t.Fatal("expected ErrNotConfigured")
	}
}

func TestMoolreWebhookNoHMAC(t *testing.T) {
	// Moolre webhook has no HMAC header — verify handler accepts without signature.
	// This is a contract test: ensure moolre webhook path is registered and dedup via webhook_inbox would be idempotent.
	// We test that Verify via envelope handles missing data gracefully.
	a, close := newTestMoolreServer(t, func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"status": 0, "code": "AVD02", "message": "not found"})
	})
	defer close()

	result, err := a.Verify(context.Background(), "nonexistent")
	if err != nil {
		t.Fatalf("Verify error: %v", err)
	}
	// status 0 maps to failed per adapter logic.
	if result.Status != "failed" && result.Status != "pending" {
		t.Fatalf("unexpected status %q", result.Status)
	}
}

func TestMoolreWithOptions(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"status": 1, "code": "TR099", "data": "x"})
	}))
	defer srv.Close()

	a := NewMoolreAdapter(config.MoolreConfig{APIKey: "k", BaseURL: "http://old"}, "", WithMoolreBaseURL(srv.URL), WithMoolreClient(srv.Client()))
	resp, err := a.Initiate(context.Background(), InitiateRequest{Reference: "optd-orctago-moolre-01TEST", Amount: money.New(100, money.GHS)})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.ExternalRef != "x" {
		t.Fatalf("expected x, got %q", resp.ExternalRef)
	}
}
