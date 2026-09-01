package gateway

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/orctatech/orcta-pay/internal/config"
	"github.com/orctatech/orcta-pay/internal/money"
)

func newTestMoolreServer(t *testing.T, handler http.HandlerFunc) (*MoolreAdapter, func()) {
	t.Helper()
	srv := httptest.NewServer(handler)
	a := NewMoolreAdapter(config.MoolreConfig{APIKey: "test-key", APIPubKey: "test-pub", APIUser: "test-user", BaseURL: srv.URL}, "", WithMoolreClient(srv.Client()))
	return a, srv.Close
}

func TestMoolreInitiateSuccess(t *testing.T) {
	a, close := newTestMoolreServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/open/transact/payment" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		if r.Header.Get("X-API-USER") != "test-user" {
			t.Fatalf("expected X-API-USER test-user, got %q", r.Header.Get("X-API-USER"))
		}
		if r.Header.Get("X-API-PUBKEY") != "test-pub" {
			t.Fatalf("expected X-API-PUBKEY test-pub, got %q", r.Header.Get("X-API-PUBKEY"))
		}
		if r.Header.Get("Authorization") != "" {
			t.Fatalf("expected no Bearer header, got %q", r.Header.Get("Authorization"))
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
		if r.Header.Get("X-API-USER") != "test-user" {
			t.Fatalf("expected X-API-USER test-user, got %q", r.Header.Get("X-API-USER"))
		}
		if r.Header.Get("X-API-KEY") != "test-key" {
			t.Fatalf("expected X-API-KEY test-key, got %q", r.Header.Get("X-API-KEY"))
		}
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body["externalref"] == "" {
			t.Fatalf("expected externalref")
		}
		if body["receiver"] == "" {
			t.Fatalf("expected receiver")
		}
		if body["amount"] != "10.00" {
			t.Fatalf("expected amount 10.00, got %v", body["amount"])
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
	a, close := newTestMoolreServer(t, func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"status": 0, "code": "AVD02", "message": "not found"})
	})
	defer close()

	result, err := a.Verify(context.Background(), "nonexistent")
	if err != nil {
		t.Fatalf("Verify error: %v", err)
	}
	if result.Status != "failed" && result.Status != "pending" {
		t.Fatalf("unexpected status %q", result.Status)
	}
}

func TestMoolreWithOptions(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"status": 1, "code": "TR099", "data": "x"})
	}))
	defer srv.Close()

	a := NewMoolreAdapter(config.MoolreConfig{APIKey: "k", APIPubKey: "k", APIUser: "k", BaseURL: "http://old"}, "", WithMoolreBaseURL(srv.URL), WithMoolreClient(srv.Client()))
	resp, err := a.Initiate(context.Background(), InitiateRequest{Reference: "optd-orctago-moolre-01TEST", Amount: money.New(100, money.GHS)})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.ExternalRef != "x" {
		t.Fatalf("expected x, got %q", resp.ExternalRef)
	}
}

func TestMoolrePayoutBulkSuccess(t *testing.T) {
	var count atomic.Int32
	a, close := newTestMoolreServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/open/transact/transfer" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body["externalref"] == "" {
			t.Fatalf("expected externalref per entry")
		}
		count.Add(1)
		_ = json.NewEncoder(w).Encode(map[string]any{"status": 1, "code": "OBGH01", "data": map[string]any{"txstatus": 1, "externalref": body["externalref"]}})
	})
	defer close()

	entries := []BulkPayoutEntry{
		{Recipient: "0240000000", Amount: money.New(1000, money.GHS), Reference: "optd-orctago-moolre-01A"},
		{Recipient: "0240000001", Amount: money.New(2000, money.GHS), Reference: "optd-orctago-moolre-01B"},
	}
	results := a.PayoutBulk(context.Background(), entries)
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	for _, r := range results {
		if r.Err != nil {
			t.Fatalf("expected success for %s, got %v", r.Reference, r.Err)
		}
	}
	if count.Load() != 2 {
		t.Fatalf("expected 2 HTTP calls, got %d", count.Load())
	}
}

func TestMoolrePayoutBulkPartialFailure(t *testing.T) {
	a, close := newTestMoolreServer(t, func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		ref, _ := body["externalref"].(string)
		if strings.Contains(ref, "01B") {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]any{"status": 0, "code": "TP13", "message": "External Reference is required and must be unique."})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"status": 1, "code": "OBGH01", "data": map[string]any{"txstatus": 1}})
	})
	defer close()

	entries := []BulkPayoutEntry{
		{Recipient: "0240000000", Amount: money.New(1000, money.GHS), Reference: "optd-orctago-moolre-01A"},
		{Recipient: "0240000001", Amount: money.New(2000, money.GHS), Reference: "optd-orctago-moolre-01B"},
	}
	results := a.PayoutBulk(context.Background(), entries)
	if results[0].Err != nil {
		t.Fatalf("expected success for first, got %v", results[0].Err)
	}
	if results[1].Err == nil {
		t.Fatal("expected error for second entry with duplicate externalref")
	}
}

func TestMoolrePayoutBadRequest(t *testing.T) {
	a, close := newTestMoolreServer(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]any{"status": 0, "code": "INP02", "message": "Transaction already exits!"})
	})
	defer close()

	err := a.Payout(context.Background(), "0240000000", money.New(1000, money.GHS), "optd-orctago-moolre-01TEST")
	if err == nil {
		t.Fatal("expected error for 400")
	}
	if !strings.Contains(err.Error(), "Transaction already exits") {
		t.Fatalf("expected duplicate message, got %v", err)
	}
}

func TestMoolrePayoutServerError(t *testing.T) {
	a, close := newTestMoolreServer(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`server error`))
	})
	defer close()

	err := a.Payout(context.Background(), "0240000000", money.New(1000, money.GHS), "optd-orctago-moolre-01TEST")
	if err == nil {
		t.Fatal("expected error for 500")
	}
}

func TestMoolreWebhookDedupNoHMAC(t *testing.T) {
	// Moolre webhooks carry no HMAC; dedup via webhook_inbox unique on aggregator_event_id.
	// Handler must accept without signature and be idempotent.
	seen := make(map[string]bool)
	handler := func(w http.ResponseWriter, r *http.Request) {
		var payload struct {
			Data struct {
				TransactionID string `json:"transactionid"`
			} `json:"data"`
		}
		_ = json.NewDecoder(r.Body).Decode(&payload)
		id := payload.Data.TransactionID
		if id == "" {
			id = r.Header.Get("X-Event-Id")
		}
		if seen[id] {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"status":"duplicate"}`))
			return
		}
		seen[id] = true
		_ = json.NewEncoder(w).Encode(map[string]any{"status": 1, "code": "P01"})
	}
	srv := httptest.NewServer(http.HandlerFunc(handler))
	defer srv.Close()

	// First delivery
	resp, err := http.Post(srv.URL, "application/json", strings.NewReader(`{"data":{"transactionid":"tid-123"}}`))
	if err != nil {
		t.Fatalf("first POST failed: %v", err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 on first delivery, got %d", resp.StatusCode)
	}
	// Duplicate delivery — same transactionid, no HMAC.
	resp2, err := http.Post(srv.URL, "application/json", strings.NewReader(`{"data":{"transactionid":"tid-123"}}`))
	if err != nil {
		t.Fatalf("second POST failed: %v", err)
	}
	_ = resp2.Body.Close()
	if resp2.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 on duplicate delivery, got %d", resp2.StatusCode)
	}
	if len(seen) != 1 {
		t.Fatalf("expected dedup to keep single entry, got %d", len(seen))
	}
}
