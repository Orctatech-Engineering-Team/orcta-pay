package orctapay

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestCreateChargeSuccess(t *testing.T) {
	var gotAuth string
	var gotBody map[string]any

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"ref":"optd-orctago-hubtel-01ARZ3NDEKTSV4RRFFQ69G5FAV","gateway":"hubtel","status":"pending","external_ref":"EXT123"}`))
	}))
	defer srv.Close()

	c := NewClient("", "test-key", WithBaseURL(srv.URL))

	res, err := c.CreateCharge(context.Background(), CreateChargeRequest{
		Product:       "orctago",
		AmountPesewas: 1800,
		Currency:      "GHS",
		Wallet:        "0241234567",
	})
	if err != nil {
		t.Fatalf("CreateCharge: %v", err)
	}
	if gotAuth != "Bearer test-key" {
		t.Fatalf("Authorization = %q, want %q", gotAuth, "Bearer test-key")
	}
	key, _ := gotBody["idempotency_key"].(string)
	if !strings.HasPrefix(key, "optd-") {
		t.Fatalf("idempotency_key = %q, want prefix optd-", key)
	}
	pending, ok := res.(ChargePending)
	if !ok {
		t.Fatalf("result type = %T, want ChargePending", res)
	}
	if pending.Ref == "" || pending.Gateway == "" {
		t.Fatalf("pending = %+v, want ref and gateway", pending)
	}
}

func TestCreateChargeWithExplicitIdempotencyKey(t *testing.T) {
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"ref":"optd-orctago-hubtel-01ARZ3NDEKTSV4RRFFQ69G5FAV","gateway":"hubtel","status":"pending"}`))
	}))
	defer srv.Close()

	c := NewClient("", "k", WithBaseURL(srv.URL))
	_, err := c.CreateCharge(context.Background(), CreateChargeRequest{
		Product:        "orctago",
		AmountPesewas:  100,
		Currency:       "GHS",
		Wallet:         "0241234567",
		IdempotencyKey: "optd-orctago-hubtel-01CUSTOMKEY12345678901234",
	})
	if err != nil {
		t.Fatalf("CreateCharge: %v", err)
	}
	if gotBody["idempotency_key"] != "optd-orctago-hubtel-01CUSTOMKEY12345678901234" {
		t.Fatalf("idempotency_key = %v", gotBody["idempotency_key"])
	}
}

func TestGetChargeStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/charges/optd-orctago-hubtel-01ARZ3NDEKTSV4RRFFQ69G5FAV/status" {
			t.Fatalf("path = %q", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer k" {
			t.Fatalf("auth = %q", r.Header.Get("Authorization"))
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ref":"optd-orctago-hubtel-01ARZ3NDEKTSV4RRFFQ69G5FAV","status":"succeeded","gateway":"hubtel","verified_at":"2026-08-31T00:00:00Z"}`))
	}))
	defer srv.Close()

	c := NewClient("", "k", WithBaseURL(srv.URL))
	st, err := c.GetChargeStatus(context.Background(), "optd-orctago-hubtel-01ARZ3NDEKTSV4RRFFQ69G5FAV")
	if err != nil {
		t.Fatalf("GetChargeStatus: %v", err)
	}
	if st.Status != "succeeded" {
		t.Fatalf("status = %q", st.Status)
	}
	if st.Ref == "" {
		t.Fatalf("ref empty")
	}
}

func TestCreatePayout(t *testing.T) {
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		if r.Header.Get("Authorization") != "Bearer k" {
			t.Fatalf("auth = %q", r.Header.Get("Authorization"))
		}
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"batch_id":"550e8400-e29b-41d4-a716-446655440000","product":"orctago","status":"pending","total_pesewas":2000,"created_at":"2026-08-31T00:00:00Z"}`))
	}))
	defer srv.Close()

	c := NewClient("", "k", WithBaseURL(srv.URL))
	res, err := c.CreatePayout(context.Background(), CreatePayoutRequest{
		Product: "orctago",
		Entries: []PayoutEntry{
			{Recipient: "0241111111", AmountPesewas: 1000, Currency: "GHS"},
			{Recipient: "0242222222", AmountPesewas: 1000, Currency: "GHS"},
		},
	})
	if err != nil {
		t.Fatalf("CreatePayout: %v", err)
	}
	if res.BatchID == "" {
		t.Fatalf("batch_id empty")
	}
	if res.TotalPesewas != 2000 {
		t.Fatalf("total = %d", res.TotalPesewas)
	}
	if gotBody["product"] != "orctago" {
		t.Fatalf("product = %v", gotBody["product"])
	}
	entries, ok := gotBody["entries"].([]any)
	if !ok || len(entries) != 2 {
		t.Fatalf("entries = %v", gotBody["entries"])
	}
}

func TestUnauthorized(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":{"code":"unauthorized","message":"bad key"}}`))
	}))
	defer srv.Close()

	c := NewClient("", "bad", WithBaseURL(srv.URL))
	_, err := c.CreateCharge(context.Background(), CreateChargeRequest{
		Product:       "orctago",
		AmountPesewas: 100,
		Currency:      "GHS",
		Wallet:        "0241234567",
	})
	if err == nil {
		t.Fatal("want error")
	}
	if !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("err = %v, want ErrUnauthorized", err)
	}
}

func TestAppsEndpoints(t *testing.T) {
	var gotMethod, gotPath string
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod, gotPath = r.Method, r.URL.Path
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/v1/apps":
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"id":"11111111-1111-1111-1111-111111111111","name":"pos","product":"pos","api_key":"optdak_secret","api_key_prefix":"optdak_abc","created_at":"2026-08-31T00:00:00Z"}`))
		case r.Method == http.MethodGet && r.URL.Path == "/v1/apps":
			_, _ = w.Write([]byte(`[{"id":"11111111-1111-1111-1111-111111111111","name":"pos","product":"pos","api_key_prefix":"optdak_abc","created_at":"2026-08-31T00:00:00Z","created_by":"user1"}]`))
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/keys/rotate"):
			_, _ = w.Write([]byte(`{"api_key":"optdak_new","api_key_prefix":"optdak_def"}`))
		case r.Method == http.MethodDelete:
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
	}))
	defer srv.Close()

	c := NewClient("", "k", WithBaseURL(srv.URL))
	ctx := context.Background()

	created, err := c.CreateApp(ctx, CreateAppRequest{Name: "pos", Product: "pos"})
	if err != nil {
		t.Fatalf("CreateApp: %v", err)
	}
	if created.APIKey != "optdak_secret" || created.Prefix != "optdak_abc" {
		t.Fatalf("created = %+v", created)
	}

	apps, err := c.ListApps(ctx)
	if err != nil {
		t.Fatalf("ListApps: %v", err)
	}
	if len(apps) != 1 || apps[0].Prefix != "optdak_abc" || apps[0].CreatedBy != "user1" {
		t.Fatalf("apps = %+v", apps)
	}

	rot, err := c.RotateKey(ctx, "11111111-1111-1111-1111-111111111111")
	if err != nil {
		t.Fatalf("RotateKey: %v", err)
	}
	if rot.APIKey != "optdak_new" {
		t.Fatalf("rotated = %+v", rot)
	}
	if gotPath != "/v1/apps/11111111-1111-1111-1111-111111111111/keys/rotate" {
		t.Fatalf("rotate path = %q", gotPath)
	}

	if err := c.RevokeApp(ctx, "11111111-1111-1111-1111-111111111111"); err != nil {
		t.Fatalf("RevokeApp: %v", err)
	}
	if gotMethod != http.MethodDelete {
		t.Fatalf("revoke method = %q", gotMethod)
	}

	// Validation errors use sentinels and never hit the network.
	if _, err := c.CreateApp(ctx, CreateAppRequest{}); !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("CreateApp empty err = %v, want ErrInvalidRequest", err)
	}
	if err := c.RevokeApp(ctx, ""); !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("RevokeApp empty err = %v, want ErrInvalidRequest", err)
	}
}

func TestCreateChargeGatewayDeclineMapsToChargeFailed(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte(`{"error":{"code":"unavailable","message":"hubtel: insufficient funds"}}`))
	}))
	defer srv.Close()

	c := NewClient("", "k", WithBaseURL(srv.URL))
	res, err := c.CreateCharge(context.Background(), CreateChargeRequest{
		Product:       "orctago",
		AmountPesewas: 100,
		Currency:      "GHS",
		Wallet:        "0241234567",
	})
	if err != nil {
		t.Fatalf("CreateCharge: %v", err)
	}
	failed, ok := res.(ChargeFailed)
	if !ok {
		t.Fatalf("result type = %T, want ChargeFailed", res)
	}
	if failed.Reason != "hubtel: insufficient funds" {
		t.Fatalf("reason = %q, want gateway decline reason", failed.Reason)
	}
}

func TestServerErrorWrapsErrRequestFailed(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":{"code":"internal_error","message":"boom"}}`))
	}))
	defer srv.Close()

	c := NewClient("", "k", WithBaseURL(srv.URL))
	_, err := c.CreateCharge(context.Background(), CreateChargeRequest{
		Product:       "orctago",
		AmountPesewas: 100,
		Currency:      "GHS",
		Wallet:        "0241234567",
	})
	if err == nil {
		t.Fatal("want error")
	}
	if !errors.Is(err, ErrRequestFailed) {
		t.Fatalf("err = %v, want ErrRequestFailed", err)
	}

	// Also for GetChargeStatus.
	_, err = c.GetChargeStatus(context.Background(), "optd-orctago-hubtel-01ARZ3NDEKTSV4RRFFQ69G5FAV")
	if !errors.Is(err, ErrRequestFailed) {
		t.Fatalf("GetChargeStatus err = %v, want ErrRequestFailed", err)
	}

	// And CreatePayout.
	_, err = c.CreatePayout(context.Background(), CreatePayoutRequest{
		Product: "orctago",
		Entries: []PayoutEntry{{Recipient: "0241", AmountPesewas: 100, Currency: "GHS"}},
	})
	if !errors.Is(err, ErrRequestFailed) {
		t.Fatalf("CreatePayout err = %v, want ErrRequestFailed", err)
	}
}

func TestTimeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"ref":"optd-orctago-hubtel-01ARZ3NDEKTSV4RRFFQ69G5FAV","gateway":"hubtel","status":"pending"}`))
	}))
	defer srv.Close()

	c := NewClient("", "k", WithBaseURL(srv.URL))
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, err := c.CreateCharge(ctx, CreateChargeRequest{
		Product:       "orctago",
		AmountPesewas: 100,
		Currency:      "GHS",
		Wallet:        "0241234567",
	})
	if err == nil {
		t.Fatal("want timeout error")
	}
	if !errors.Is(err, context.DeadlineExceeded) && !strings.Contains(err.Error(), "context deadline") {
		// Wrapped via fmt %w from http.Client, check context cause.
		if !strings.Contains(strings.ToLower(err.Error()), "deadline") && !strings.Contains(strings.ToLower(err.Error()), "canceled") {
			t.Fatalf("err = %v, want deadline", err)
		}
	}
}

func TestNotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":{"code":"not_found","message":"no such charge"}}`))
	}))
	defer srv.Close()

	c := NewClient("", "k", WithBaseURL(srv.URL))
	_, err := c.GetChargeStatus(context.Background(), "optd-orctago-hubtel-01NOTFOUND0000000000000")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestCreateChargeUsesEnvFallback(t *testing.T) {
	// BaseURL empty uses ORCTA_PAY_URL env.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"ref":"optd-orctago-hubtel-01ARZ3NDEKTSV4RRFFQ69G5FAV","gateway":"hubtel","status":"pending"}`))
	}))
	defer srv.Close()

	t.Setenv("ORCTA_PAY_URL", srv.URL)
	c := NewClient("", "k")
	_, err := c.CreateCharge(context.Background(), CreateChargeRequest{
		Product:       "orctago",
		AmountPesewas: 100,
		Currency:      "GHS",
		Wallet:        "0241234567",
	})
	if err != nil {
		t.Fatalf("CreateCharge: %v", err)
	}
}
