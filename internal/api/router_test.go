package api

import (
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
