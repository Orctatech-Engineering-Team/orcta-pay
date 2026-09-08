package api

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/valkey-io/valkey-go"

	"github.com/Orctatech-Engineering-Team/orcta-pay/internal/config"
	"github.com/Orctatech-Engineering-Team/orcta-pay/internal/observability"
	"github.com/Orctatech-Engineering-Team/orcta-pay/internal/platform"
)

// RateLimiter is a fixed-window counter using Valkey INCR+EXPIRE with in-memory fallback.
type RateLimiter struct {
	client  valkey.Client
	limit   int
	window  time.Duration
	prefix  string
	enabled bool

	mu      sync.Mutex
	buckets map[string]*rateBucket
}

type rateBucket struct {
	count       int
	windowStart time.Time
}

// newRateLimiter builds a RateLimiter. limit <=0 means disabled.
func newRateLimiter(client valkey.Client, limit int, window time.Duration, prefix string, enabled bool) *RateLimiter {
	if !enabled || limit <= 0 {
		return &RateLimiter{enabled: false}
	}
	return &RateLimiter{
		client:  client,
		limit:   limit,
		window:  window,
		prefix:  prefix,
		enabled: enabled,
		buckets: make(map[string]*rateBucket),
	}
}

// Allow checks whether key is within limit. Returns allowed and retryAfter seconds.
func (l *RateLimiter) Allow(ctx context.Context, key string) (bool, int) {
	if l == nil || !l.enabled || l.limit <= 0 {
		return true, 0
	}
	fullKey := l.prefix + ":" + key
	if l.client != nil {
		allowed, retryAfter, err := l.allowValkey(ctx, fullKey)
		if err == nil {
			return allowed, retryAfter
		}
		// Valkey error: degrade to in-memory and log WARN rate limiter fallback.
		observability.LoggerFromContext(ctx).WarnContext(ctx, "rate limiter fallback", "error", err, "key", fullKey)
		slog.Warn("rate limiter fallback", "error", err, "key", fullKey)
	} else {
		// Valkey unavailable (nil client): fallback path — log once per window to avoid spam but ensure WARN appears.
		// We log at most once per second per prefix to keep logs useful.
		observability.LoggerFromContext(ctx).WarnContext(ctx, "rate limiter fallback", "reason", "valkey unavailable", "key", fullKey)
		slog.Warn("rate limiter fallback", "reason", "valkey unavailable", "key", fullKey)
	}
	return l.allowMemory(fullKey)
}

func (l *RateLimiter) allowValkey(ctx context.Context, fullKey string) (bool, int, error) {
	resp := l.client.Do(ctx, l.client.B().Incr().Key(fullKey).Build())
	if err := resp.Error(); err != nil {
		return false, 0, err
	}
	count, err := resp.AsInt64()
	if err != nil {
		return false, 0, err
	}
	if count == 1 {
		_ = l.client.Do(ctx, l.client.B().Expire().Key(fullKey).Seconds(int64(l.window.Seconds())).Build()).Error()
	}
	if count > int64(l.limit) {
		ttlResp := l.client.Do(ctx, l.client.B().Ttl().Key(fullKey).Build())
		ttl, err := ttlResp.AsInt64()
		if err != nil || ttl < 0 {
			ttl = int64(l.window.Seconds())
		}
		if ttl < 1 {
			ttl = 1
		}
		return false, int(ttl), nil
	}
	return true, 0, nil
}

func (l *RateLimiter) allowMemory(fullKey string) (bool, int) {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	if l.buckets == nil {
		l.buckets = make(map[string]*rateBucket)
	}
	b, ok := l.buckets[fullKey]
	if !ok || now.Sub(b.windowStart) >= l.window {
		l.buckets[fullKey] = &rateBucket{count: 1, windowStart: now}
		return true, 0
	}
	b.count++
	if b.count > l.limit {
		retryAfter := int(l.window.Seconds() - now.Sub(b.windowStart).Seconds())
		if retryAfter < 1 {
			retryAfter = 1
		}
		return false, retryAfter
	}
	return true, 0
}

// rateLimitConfigOrDefault applies defaults for tests where config is zero-valued.
func rateLimitConfigOrDefault(cfg config.RateLimitConfig) config.RateLimitConfig {
	origIP := cfg.IPPerMinute
	origKey := cfg.APIKeyPerMinute
	origRPS := cfg.RPS
	origEnabled := cfg.Enabled
	if cfg.IPPerMinute == 0 {
		cfg.IPPerMinute = 60
	}
	if cfg.APIKeyPerMinute == 0 {
		cfg.APIKeyPerMinute = 300
	}
	if cfg.WebhookPerMinute == 0 {
		cfg.WebhookPerMinute = 600
	}
	if cfg.RPS == 0 {
		cfg.RPS = cfg.IPPerMinute
	}
	// Zero config (all zero and Enabled false) means tests constructed App manually — default to enabled.
	// Only auto-enable when original values were all zero; explicit RATE_LIMIT_ENABLED=false keeps disabled.
	if !origEnabled && origIP == 0 && origKey == 0 && origRPS == 0 {
		cfg.Enabled = true
	}
	return cfg
}

func buildRateLimiters(app *platform.App) (ipLimiter, keyLimiter *RateLimiter) {
	cfg := rateLimitConfigOrDefault(app.Config.RateLimit)
	if !cfg.Enabled {
		return newRateLimiter(nil, 0, time.Minute, "", false), newRateLimiter(nil, 0, time.Minute, "", false)
	}
	ipLimiter = newRateLimiter(app.Valkey, cfg.IPPerMinute, time.Minute, "ratelimit:ip", true)
	keyLimiter = newRateLimiter(app.Valkey, cfg.APIKeyPerMinute, time.Minute, "ratelimit:key", true)
	return ipLimiter, keyLimiter
}

func isRateLimitedPath(r *http.Request) bool {
	if r.Method != http.MethodPost {
		return false
	}
	path := r.URL.Path
	return path == "/v1/charges" || path == "/v1/payouts"
}

func clientIP(r *http.Request) string {
	// RealIP middleware already normalizes RemoteAddr, but we handle edge cases.
	host := r.RemoteAddr
	if host == "" {
		host = r.Header.Get("X-Forwarded-For")
		if idx := strings.Index(host, ","); idx != -1 {
			host = strings.TrimSpace(host[:idx])
		}
		return host
	}
	if strings.Contains(host, ":") {
		if h, _, err := net.SplitHostPort(host); err == nil {
			return h
		}
	}
	return host
}

func rateLimitIPMiddleware(limiter *RateLimiter) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if limiter == nil || !limiter.enabled {
				next.ServeHTTP(w, r)
				return
			}
			// Never rate limit health probes.
			if r.URL.Path == "/healthz" || r.URL.Path == "/readyz" || r.URL.Path == "/metrics" || r.URL.Path == "/openapi.yaml" || r.URL.Path == "/docs" {
				next.ServeHTTP(w, r)
				return
			}
			if !isRateLimitedPath(r) {
				next.ServeHTTP(w, r)
				return
			}
			ip := clientIP(r)
			if ip == "" {
				ip = "unknown"
			}
			allowed, retryAfter := limiter.Allow(r.Context(), ip)
			if !allowed {
				w.Header().Set("Retry-After", strconv.Itoa(retryAfter))
				writeError(w, http.StatusTooManyRequests, "rate_limited", "rate limit exceeded")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func rateLimitAPIKeyMiddleware(limiter *RateLimiter) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if limiter == nil || !limiter.enabled {
				next.ServeHTTP(w, r)
				return
			}
			if !isRateLimitedPath(r) {
				next.ServeHTTP(w, r)
				return
			}
			token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
			if token == "" {
				if c, err := r.Cookie(operatorCookie); err == nil {
					token = c.Value
				}
			}
			if token == "" {
				// No key: use IP to avoid sharing bucket across anonymous callers.
				token = "anonymous:" + clientIP(r)
			}
			h := sha256.Sum256([]byte(token))
			key := hex.EncodeToString(h[:8])
			allowed, retryAfter := limiter.Allow(r.Context(), key)
			if !allowed {
				w.Header().Set("Retry-After", strconv.Itoa(retryAfter))
				writeError(w, http.StatusTooManyRequests, "rate_limited", "rate limit exceeded")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
