package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Orctatech-Engineering-Team/orcta-pay/internal/charges"
	"github.com/Orctatech-Engineering-Team/orcta-pay/internal/config"
	"github.com/Orctatech-Engineering-Team/orcta-pay/internal/gateway"
	"github.com/Orctatech-Engineering-Team/orcta-pay/internal/payouts"
	"github.com/Orctatech-Engineering-Team/orcta-pay/internal/platform"
	postgres "github.com/Orctatech-Engineering-Team/orcta-pay/internal/storage/postgres"
	valkeystore "github.com/Orctatech-Engineering-Team/orcta-pay/internal/storage/valkey"
)

func TestReadyzReportsUnavailableDependencies(t *testing.T) {
	router := NewRouter(&platform.App{})
	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	res := httptest.NewRecorder()

	router.ServeHTTP(res, req)

	if res.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusServiceUnavailable)
	}
	var body struct {
		Status string            `json:"status"`
		Checks map[string]string `json:"checks"`
	}
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Status != "not_ready" || body.Checks["postgres"] != "unavailable" || body.Checks["valkey"] != "unavailable" {
		t.Fatalf("unexpected readiness response: %+v", body)
	}
}

func TestV1RequiresConfiguredBearerToken(t *testing.T) {
	app := &platform.App{Config: config.Config{Auth: config.AuthConfig{APIKey: "service-secret"}}}
	router := NewRouter(app)

	for _, tc := range []struct {
		name   string
		header string
	}{
		{name: "missing"},
		{name: "wrong", header: "Bearer wrong-secret"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/v1/charges", nil)
			if tc.header != "" {
				req.Header.Set("Authorization", tc.header)
			}
			res := httptest.NewRecorder()

			router.ServeHTTP(res, req)

			if res.Code != http.StatusUnauthorized {
				t.Fatalf("status = %d, want %d", res.Code, http.StatusUnauthorized)
			}
		})
	}
}

func TestOperationalRoutesRemainPublic(t *testing.T) {
	router := NewRouter(&platform.App{Config: config.Config{Auth: config.AuthConfig{APIKey: "service-secret"}}})
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	res := httptest.NewRecorder()

	router.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusOK)
	}
}

func TestOperatorLoginSessionAndLogout(t *testing.T) {
	app := &platform.App{Config: config.Config{Auth: config.AuthConfig{APIKey: "operator-secret"}}}
	router := NewRouter(app)

	login := httptest.NewRequest(http.MethodPost, "/auth/login", bytes.NewBufferString(`{"api_key":"operator-secret"}`))
	login.Header.Set("Content-Type", "application/json")
	loginRes := httptest.NewRecorder()
	router.ServeHTTP(loginRes, login)
	if loginRes.Code != http.StatusOK {
		t.Fatalf("login status = %d, want %d", loginRes.Code, http.StatusOK)
	}
	cookies := loginRes.Result().Cookies()
	if len(cookies) != 1 || !cookies[0].HttpOnly {
		t.Fatalf("expected one HttpOnly session cookie, got %+v", cookies)
	}

	session := httptest.NewRequest(http.MethodGet, "/auth/session", nil)
	session.AddCookie(cookies[0])
	sessionRes := httptest.NewRecorder()
	router.ServeHTTP(sessionRes, session)
	if sessionRes.Code != http.StatusOK {
		t.Fatalf("session status = %d, want %d", sessionRes.Code, http.StatusOK)
	}

	protected := httptest.NewRequest(http.MethodGet, "/v1/charges", nil)
	protected.AddCookie(cookies[0])
	protectedRes := httptest.NewRecorder()
	router.ServeHTTP(protectedRes, protected)
	if protectedRes.Code == http.StatusUnauthorized {
		t.Fatal("valid operator session was rejected")
	}

	logout := httptest.NewRequest(http.MethodPost, "/auth/logout", nil)
	logoutRes := httptest.NewRecorder()
	router.ServeHTTP(logoutRes, logout)
	if logoutRes.Code != http.StatusNoContent || logoutRes.Result().Cookies()[0].MaxAge != -1 {
		t.Fatalf("logout did not clear session cookie")
	}
}

func TestOperatorLoginRejectsWrongKey(t *testing.T) {
	app := &platform.App{Config: config.Config{Auth: config.AuthConfig{APIKey: "operator-secret"}}}
	router := NewRouter(app)
	req := httptest.NewRequest(http.MethodPost, "/auth/login", bytes.NewBufferString(`{"api_key":"wrong"}`))
	req.Header.Set("Content-Type", "application/json")
	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)
	if res.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusUnauthorized)
	}
}

func newRateLimitedApp(ipLimit, keyLimit int, enabled bool) *platform.App {
	cfg := config.Config{
		Auth:      config.AuthConfig{APIKey: "test-secret"},
		RateLimit: config.RateLimitConfig{Enabled: enabled, IPPerMinute: ipLimit, APIKeyPerMinute: keyLimit, RPS: ipLimit, WebhookPerMinute: 600},
		Payments:  config.PaymentsConfig{Primary: "paystack"},
	}
	mem := postgres.NewMemoryStore()
	health := valkeystore.NewHealthStore(nil)
	locker := valkeystore.NewLocker(nil)
	router := gateway.NewChargerRouter(cfg.Payments, health, map[gateway.Gateway]gateway.AggregatorClient{})
	app := &platform.App{
		Config:   cfg,
		Charges:  charges.NewService(mem, router),
		Payouts:  payouts.NewService(mem, router, payouts.WithLocker(locker), payouts.WithLedger(mem)),
		Router:   router,
		Valkey:   nil,
		MemStore: mem,
	}
	return app
}

func TestRateLimitIPBurstReturns429(t *testing.T) {
	app := newRateLimitedApp(2, 100, true)
	router := NewRouter(app)
	body := `{"product":"test","amount_pesewas":1000,"currency":"GHS","wallet":"0550000000"}`
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodPost, "/v1/charges", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer test-secret")
		req.RemoteAddr = "10.0.0.1:1234"
		res := httptest.NewRecorder()
		router.ServeHTTP(res, req)
		if res.Code == http.StatusTooManyRequests {
			t.Fatalf("request %d should not be rate limited, got 429", i+1)
		}
	}
	req := httptest.NewRequest(http.MethodPost, "/v1/charges", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer test-secret")
	req.RemoteAddr = "10.0.0.1:1234"
	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)
	if res.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429 after burst, got %d", res.Code)
	}
	if res.Header().Get("Retry-After") == "" {
		t.Fatal("expected Retry-After header")
	}
	var env struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.NewDecoder(res.Body).Decode(&env); err != nil {
		t.Fatalf("decode error response: %v", err)
	}
	if env.Error.Code != "rate_limited" {
		t.Fatalf("expected code rate_limited, got %q", env.Error.Code)
	}
}

func TestRateLimitHealthProbesNotRateLimited(t *testing.T) {
	app := newRateLimitedApp(1, 1, true)
	router := NewRouter(app)
	for i := 0; i < 10; i++ {
		for _, path := range []string{"/healthz", "/readyz", "/metrics"} {
			req := httptest.NewRequest(http.MethodGet, path, nil)
			req.RemoteAddr = "10.0.0.2:1234"
			res := httptest.NewRecorder()
			router.ServeHTTP(res, req)
			if res.Code == http.StatusTooManyRequests {
				t.Fatalf("probe %s should not be rate limited on iteration %d", path, i)
			}
		}
	}
}

func TestRateLimitPerAPIKeyIndependent(t *testing.T) {
	body := `{"product":"test","amount_pesewas":1000,"currency":"GHS","wallet":"0550000000"}`
	// Create app with no auth so arbitrary Bearer keys pass and we can test per-key buckets.
	cfg := config.Config{
		Auth:      config.AuthConfig{APIKey: ""},
		RateLimit: config.RateLimitConfig{Enabled: true, IPPerMinute: 100, APIKeyPerMinute: 2, RPS: 100, WebhookPerMinute: 600},
		Payments:  config.PaymentsConfig{Primary: "paystack"},
	}
	mem := postgres.NewMemoryStore()
	health := valkeystore.NewHealthStore(nil)
	locker := valkeystore.NewLocker(nil)
	gwRouter := gateway.NewChargerRouter(cfg.Payments, health, map[gateway.Gateway]gateway.AggregatorClient{})
	app2 := &platform.App{
		Config:   cfg,
		Charges:  charges.NewService(mem, gwRouter),
		Payouts:  payouts.NewService(mem, gwRouter, payouts.WithLocker(locker), payouts.WithLedger(mem)),
		Router:   gwRouter,
		Valkey:   nil,
		MemStore: mem,
	}
	router2 := NewRouter(app2)
	// Burst key A to limit
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodPost, "/v1/charges", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer key-A")
		req.RemoteAddr = "10.0.0.99:1234"
		res := httptest.NewRecorder()
		router2.ServeHTTP(res, req)
		if res.Code == http.StatusTooManyRequests {
			t.Fatalf("key-A request %d should not be limited", i+1)
		}
	}
	// Third with same key A should be 429
	req := httptest.NewRequest(http.MethodPost, "/v1/charges", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer key-A")
	req.RemoteAddr = "10.0.0.99:1234"
	res := httptest.NewRecorder()
	router2.ServeHTTP(res, req)
	if res.Code != http.StatusTooManyRequests {
		t.Fatalf("expected key-A to be rate limited on 3rd request, got %d", res.Code)
	}
	// Same IP but different key B should succeed (independent bucket)
	reqB := httptest.NewRequest(http.MethodPost, "/v1/charges", strings.NewReader(body))
	reqB.Header.Set("Content-Type", "application/json")
	reqB.Header.Set("Authorization", "Bearer key-B")
	reqB.RemoteAddr = "10.0.0.99:1234"
	resB := httptest.NewRecorder()
	router2.ServeHTTP(resB, reqB)
	if resB.Code == http.StatusTooManyRequests {
		t.Fatalf("key-B should not share bucket with key-A, got 429")
	}
}

func TestRateLimitPayoutsIP(t *testing.T) {
	app := newRateLimitedApp(2, 100, true)
	router := NewRouter(app)
	body := `{"product":"test","entries":[{"recipient":"0550000000","amount_pesewas":1000,"currency":"GHS"}]}`
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodPost, "/v1/payouts", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer test-secret")
		req.RemoteAddr = "10.0.0.4:1234"
		res := httptest.NewRecorder()
		router.ServeHTTP(res, req)
		if res.Code == http.StatusTooManyRequests {
			t.Fatalf("payout request %d should not be limited", i+1)
		}
	}
	req := httptest.NewRequest(http.MethodPost, "/v1/payouts", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer test-secret")
	req.RemoteAddr = "10.0.0.4:1234"
	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)
	if res.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429 for payouts burst, got %d", res.Code)
	}
	if res.Header().Get("Retry-After") == "" {
		t.Fatal("expected Retry-After for payouts")
	}
}

func TestRateLimitDisabledAllowsBurst(t *testing.T) {
	app := newRateLimitedApp(1, 1, false)
	router := NewRouter(app)
	body := `{"product":"test","amount_pesewas":1000,"currency":"GHS","wallet":"0550000000"}`
	for i := 0; i < 10; i++ {
		req := httptest.NewRequest(http.MethodPost, "/v1/charges", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer test-secret")
		req.RemoteAddr = "10.0.0.5:1234"
		res := httptest.NewRecorder()
		router.ServeHTTP(res, req)
		if res.Code == http.StatusTooManyRequests {
			t.Fatalf("disabled limiter should not 429, got 429 on iteration %d", i)
		}
	}
}

func TestRateLimitInMemoryFallbackNoCrash(t *testing.T) {
	// Valkey nil already tests fallback; just ensure no panic and limiter still works
	app := newRateLimitedApp(2, 100, true)
	// Explicitly nil Valkey to force fallback
	app.Valkey = nil
	router := NewRouter(app)
	body := `{"product":"test","amount_pesewas":1000,"currency":"GHS","wallet":"0550000000"}`
	for i := 0; i < 3; i++ {
		req := httptest.NewRequest(http.MethodPost, "/v1/charges", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer test-secret")
		req.RemoteAddr = "10.0.0.6:1234"
		res := httptest.NewRecorder()
		// Should not panic even with Valkey down
		router.ServeHTTP(res, req)
		if i < 2 && res.Code == http.StatusTooManyRequests {
			t.Fatalf("fallback: request %d should not be limited", i+1)
		}
		if i == 2 && res.Code != http.StatusTooManyRequests {
			t.Fatalf("fallback: expected 429 on 3rd request, got %d", res.Code)
		}
	}
}
