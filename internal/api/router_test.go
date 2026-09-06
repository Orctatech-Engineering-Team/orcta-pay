package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Orctatech-Engineering-Team/orcta-pay/internal/config"
	"github.com/Orctatech-Engineering-Team/orcta-pay/internal/platform"
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
