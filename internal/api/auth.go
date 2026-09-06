package api

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Orctatech-Engineering-Team/orcta-pay/internal/config"
	"github.com/Orctatech-Engineering-Team/orcta-pay/internal/platform"
)

const (
	operatorCookie  = "orcta_pay_operator"
	sessionLifetime = 12 * time.Hour
)

func handleLogin(app *platform.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			APIKey string `json:"api_key"`
		}
		if err := decodeJSON(w, r, &body); err != nil {
			return
		}
		if !validSecret(body.APIKey, app.Config.Auth.APIKey) {
			writeError(w, http.StatusUnauthorized, "unauthorized", "invalid access key")
			return
		}
		expires := time.Now().Add(sessionLifetime)
		http.SetCookie(w, sessionCookie(app, signedSession(app.Config.Auth.APIKey, expires), expires))
		writeJSON(w, http.StatusOK, map[string]any{"authenticated": true, "expires_at": expires.UTC()})
	}
}

func handleSession(app *platform.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !hasValidSession(r, app.Config.Auth.APIKey) {
			writeError(w, http.StatusUnauthorized, "unauthorized", "authentication required")
			return
		}
		writeJSON(w, http.StatusOK, map[string]bool{"authenticated": true})
	}
}

func handleLogout(app *platform.App) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		expires := time.Unix(1, 0)
		cookie := sessionCookie(app, "", expires)
		cookie.MaxAge = -1
		http.SetCookie(w, cookie)
		w.WriteHeader(http.StatusNoContent)
	}
}

func bearerAuth(app *platform.App) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			expected := app.Config.Auth.APIKey
			if expected == "" || hasValidSession(r, expected) {
				next.ServeHTTP(w, r)
				return
			}
			token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
			if !validSecret(token, expected) {
				writeError(w, http.StatusUnauthorized, "unauthorized", "invalid or missing credentials")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func sessionCookie(app *platform.App, value string, expires time.Time) *http.Cookie {
	return &http.Cookie{
		Name: operatorCookie, Value: value, Path: "/", Expires: expires,
		MaxAge: int(sessionLifetime.Seconds()), HttpOnly: true, SameSite: http.SameSiteStrictMode,
		Secure: app.Config.Environment == config.EnvProduction,
	}
}

func signedSession(secret string, expires time.Time) string {
	payload := strconv.FormatInt(expires.Unix(), 10)
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(payload))
	return payload + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func hasValidSession(r *http.Request, secret string) bool {
	cookie, err := r.Cookie(operatorCookie)
	if err != nil || secret == "" {
		return false
	}
	parts := strings.Split(cookie.Value, ".")
	if len(parts) != 2 {
		return false
	}
	expires, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil || time.Now().Unix() >= expires {
		return false
	}
	return hmac.Equal([]byte(cookie.Value), []byte(signedSession(secret, time.Unix(expires, 0))))
}

func validSecret(got, expected string) bool {
	return got != "" && expected != "" && subtle.ConstantTimeCompare([]byte(got), []byte(expected)) == 1
}
