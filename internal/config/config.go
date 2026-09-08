// Package config parses and validates process configuration from the environment once at startup.
package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	defaultHTTPPort        = 8080
	defaultReadTimeout     = 10 * time.Second
	defaultWriteTimeout    = 30 * time.Second
	defaultIdleTimeout     = 120 * time.Second
	defaultShutdownTimeout = 20 * time.Second
	defaultDBMaxConns      = 20
	defaultLogLevel        = "info"
	defaultServiceName     = "orcta-pay"
	defaultTraceSample     = 1.0
	defaultHandlerTimeout  = 15 * time.Second
	defaultJobTimeout      = 5 * time.Minute
	defaultPendingTimeout  = 5 * time.Minute
)

// Environment gates production behaviour.
type Environment string

const (
	EnvDevelopment Environment = "development"
	EnvStaging     Environment = "staging"
	EnvProduction  Environment = "production"
)

// Valid reports whether e is recognised.
func (e Environment) Valid() bool {
	switch e {
	case EnvDevelopment, EnvStaging, EnvProduction:
		return true
	default:
		return false
	}
}

// AuthConfig holds API key and Vault settings.
type AuthConfig struct {
	APIKey    string
	VaultAddr string
}

// RateLimitConfig configures request throttling.
type RateLimitConfig struct {
	Enabled           bool
	RPS               int // RATE_LIMIT_RPS alias for IP limit per minute
	IPPerMinute       int
	APIKeyPerMinute   int
	WebhookPerMinute  int
}

// Config is built once in main and passed down.
type Config struct {
	Environment Environment
	HTTP        HTTPConfig
	Database    DatabaseConfig
	Valkey      ValkeyConfig
	Observ      ObservabilityConfig
	Payments    PaymentsConfig
	Worker      WorkerConfig
	Auth        AuthConfig
	RateLimit   RateLimitConfig
}

// HTTPConfig configures the API server.
type HTTPConfig struct {
	Port            int
	ReadTimeout     time.Duration
	WriteTimeout    time.Duration
	IdleTimeout     time.Duration
	ShutdownTimeout time.Duration
	HandlerTimeout  time.Duration
}

// Addr returns the listen address.
func (c HTTPConfig) Addr() string { return ":" + strconv.Itoa(c.Port) }

// DatabaseConfig configures Postgres.
type DatabaseConfig struct {
	URL      string
	MaxConns int32
}

// ValkeyConfig configures Valkey.
type ValkeyConfig struct {
	Addr     string
	Password string
	DB       int
}

// ObservabilityConfig configures logging and tracing.
type ObservabilityConfig struct {
	ServiceName      string
	LogLevel         string
	OTLPEndpoint     string
	TraceSampleRatio float64
}

// TracingEnabled reports whether traces should be exported.
func (c ObservabilityConfig) TracingEnabled() bool { return c.OTLPEndpoint != "" }

// WorkerConfig configures background jobs.
type WorkerConfig struct {
	JobTimeout time.Duration
}

// PaymentsConfig configures gateways and hubtel/paystack.
type PaymentsConfig struct {
	Primary         string
	PendingTimeout  time.Duration
	Hubtel          HubtelConfig
	Paystack        PaystackConfig
	Moolre          MoolreConfig
	CallbackBaseURL string
	WebhookSecrets  WebhookSecrets
}

// WebhookSecrets holds per-gateway HMAC secrets.
type WebhookSecrets struct {
	Hubtel   string
	Paystack string
	Moolre   string
}

// HubtelConfig holds Hubtel credentials.
type HubtelConfig struct {
	ClientID        string
	ClientSecret    string
	MerchantAccount string
	BaseURL         string
}

// Disabled reports whether Hubtel is not configured.
func (c HubtelConfig) Disabled() bool { return c.ClientID == "" || c.ClientSecret == "" }

// PaystackConfig holds Paystack credentials.
type PaystackConfig struct {
	SecretKey string
	BaseURL   string
}

// Disabled reports whether Paystack is not configured.
func (c PaystackConfig) Disabled() bool { return c.SecretKey == "" }

// MoolreConfig holds Moolre credentials per the Moolre 2.0 deep dive.
//
// Amounts are decimal GHS strings (e.g. "18.00") on the wire, derived from
// integer pesewas via fmt.Sprintf("%.2f", minor/100). Never send pesewas.
// Live BaseURL is https://api.moolre.com, sandbox is https://sandbox.moolre.com.
// Collections authenticate with X-API-USER + X-API-PUBKEY at POST /open/transact/payment.
// Transfers authenticate with X-API-USER + X-API-KEY at POST /open/transact/transfer
// and bulk is looped single transfers with distinct externalref per recipient.
// No webhook HMAC is published; webhook_inbox dedup is the source of truth.
type MoolreConfig struct {
	APIUser       string
	APIKey        string // private key for transfers / status
	APIPubKey     string // public key for collections
	AccountNumber string
	BaseURL       string
}

// Disabled reports whether Moolre is not configured.
// Log-and-noop when unconfigured so local dev needs no credentials.
func (c MoolreConfig) Disabled() bool { return c.APIKey == "" && c.APIPubKey == "" }

// ErrMissingRequired is returned when a required variable is unset.
var ErrMissingRequired = errors.New("config: required variable not set")

// Load reads, defaults, and validates configuration.
func Load() (Config, error) {
	var errs []error
	env := Environment(stringVar("ORCTA_ENV", string(EnvDevelopment)))
	port, err := intVar("HTTP_PORT", defaultHTTPPort)
	errs = append(errs, err)
	readTimeout, err := durationVar("HTTP_READ_TIMEOUT", defaultReadTimeout)
	errs = append(errs, err)
	writeTimeout, err := durationVar("HTTP_WRITE_TIMEOUT", defaultWriteTimeout)
	errs = append(errs, err)
	idleTimeout, err := durationVar("HTTP_IDLE_TIMEOUT", defaultIdleTimeout)
	errs = append(errs, err)
	shutdownTimeout, err := durationVar("HTTP_SHUTDOWN_TIMEOUT", defaultShutdownTimeout)
	errs = append(errs, err)
	handlerTimeout, err := durationVar("HTTP_HANDLER_TIMEOUT", defaultHandlerTimeout)
	errs = append(errs, err)
	jobTimeout, err := durationVar("WORKER_JOB_TIMEOUT", defaultJobTimeout)
	errs = append(errs, err)
	pendingTimeout, err := durationVar("PAYMENT_PENDING_TIMEOUT", defaultPendingTimeout)
	errs = append(errs, err)
	maxConns, err := int32Var("DATABASE_MAX_CONNS", defaultDBMaxConns)
	errs = append(errs, err)
	valkeyDB, err := intVar("VALKEY_DB", 0)
	errs = append(errs, err)
	sampleRatio, err := floatVar("OTEL_TRACE_SAMPLE_RATIO", defaultTraceSample)
	errs = append(errs, err)

	apiKey := os.Getenv("ORCTA_PAY_API_KEY")
	if apiKey == "" {
		apiKey = os.Getenv("AUTH_API_KEY")
	}
	rateLimit, err := rateLimitConfig()
	errs = append(errs, err)
	cfg := Config{
		RateLimit: rateLimit,
		Environment: env,
		HTTP: HTTPConfig{
			Port:            port,
			ReadTimeout:     readTimeout,
			WriteTimeout:    writeTimeout,
			IdleTimeout:     idleTimeout,
			ShutdownTimeout: shutdownTimeout,
			HandlerTimeout:  handlerTimeout,
		},
		Worker: WorkerConfig{JobTimeout: jobTimeout},
		Payments: PaymentsConfig{
			PendingTimeout:  pendingTimeout,
			Primary:         stringVar("PAYMENTS_PRIMARY", "paystack"),
			Hubtel:          hubtelConfig(),
			Paystack:        paystackConfig(),
			Moolre:          moolreConfig(),
			CallbackBaseURL: os.Getenv("PAYMENTS_CALLBACK_BASE_URL"),
			WebhookSecrets: WebhookSecrets{
				Hubtel:   os.Getenv("HUBTEL_WEBHOOK_SECRET"),
				Paystack: stringVar("PAYSTACK_WEBHOOK_SECRET", os.Getenv("PAYSTACK_SECRET_KEY")),
				Moolre:   os.Getenv("MOOLRE_WEBHOOK_SECRET"),
			},
		},
		Database: DatabaseConfig{URL: os.Getenv("DATABASE_URL"), MaxConns: maxConns},
		Valkey: ValkeyConfig{
			Addr:     stringVar("VALKEY_ADDR", "localhost:6379"),
			Password: os.Getenv("VALKEY_PASSWORD"),
			DB:       valkeyDB,
		},
		Observ: ObservabilityConfig{
			ServiceName:      stringVar("OTEL_SERVICE_NAME", defaultServiceName),
			LogLevel:         stringVar("LOG_LEVEL", defaultLogLevel),
			OTLPEndpoint:     os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT"),
			TraceSampleRatio: sampleRatio,
		},
		Auth: AuthConfig{
			APIKey:    apiKey,
			VaultAddr: os.Getenv("ORCTA_PAY_VAULT_ADDR"),
		},
	}
	errs = append(errs, cfg.Validate())
	if err := errors.Join(errs...); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

// Validate reports every semantic problem at once.
func (c Config) Validate() error {
	var errs []error
	if !c.Environment.Valid() {
		errs = append(errs, fmt.Errorf("config: ORCTA_ENV: unknown environment %q", c.Environment))
	}
	if c.Database.URL == "" {
		errs = append(errs, fmt.Errorf("%w: DATABASE_URL", ErrMissingRequired))
	}
	if c.HTTP.Port < 1 || c.HTTP.Port > 65535 {
		errs = append(errs, fmt.Errorf("config: HTTP_PORT: %d out of range", c.HTTP.Port))
	}
	if c.Database.MaxConns < 1 {
		errs = append(errs, fmt.Errorf("config: DATABASE_MAX_CONNS: %d must be at least 1", c.Database.MaxConns))
	}
	if c.Valkey.Addr == "" {
		errs = append(errs, fmt.Errorf("%w: VALKEY_ADDR", ErrMissingRequired))
	}
	if !validLogLevel(c.Observ.LogLevel) {
		errs = append(errs, fmt.Errorf("config: LOG_LEVEL: unknown level %q", c.Observ.LogLevel))
	}
	if c.Observ.TraceSampleRatio < 0 || c.Observ.TraceSampleRatio > 1 {
		errs = append(errs, fmt.Errorf("config: OTEL_TRACE_SAMPLE_RATIO: %v out of range 0-1", c.Observ.TraceSampleRatio))
	}
	if c.Environment == EnvProduction && strings.EqualFold(c.Observ.LogLevel, "debug") {
		errs = append(errs, errors.New("config: LOG_LEVEL: debug not permitted in production"))
	}
	if c.Payments.Primary != "" && c.Payments.Primary != "hubtel" && c.Payments.Primary != "paystack" && c.Payments.Primary != "moolre" {
		errs = append(errs, errors.New("config: PAYMENTS_PRIMARY must be hubtel, paystack, or moolre"))
	}
	if c.Payments.PendingTimeout <= 0 {
		errs = append(errs, errors.New("config: PAYMENT_PENDING_TIMEOUT must be > 0"))
	}
	// Auth: the v1 API is unauthenticated without an API key.
	// That is acceptable for local development only — refuse to boot in
	// production without one.
	if c.Environment == EnvProduction && c.Auth.APIKey == "" {
		errs = append(errs, errors.New("config: ORCTA_PAY_API_KEY is required in production"))
	}
	return errors.Join(errs...)
}

func validLogLevel(name string) bool {
	switch strings.ToLower(name) {
	case "debug", "info", "warn", "error":
		return true
	default:
		return false
	}
}

func stringVar(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func intVar(key string, fallback int) (int, error) {
	raw := os.Getenv(key)
	if raw == "" {
		return fallback, nil
	}
	v, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("config: %s: %w", key, err)
	}
	return v, nil
}

func int32Var(key string, fallback int32) (int32, error) {
	raw := os.Getenv(key)
	if raw == "" {
		return fallback, nil
	}
	v, err := strconv.ParseInt(raw, 10, 32)
	if err != nil {
		return 0, fmt.Errorf("config: %s: %w", key, err)
	}
	return int32(v), nil
}

func floatVar(key string, fallback float64) (float64, error) {
	raw := os.Getenv(key)
	if raw == "" {
		return fallback, nil
	}
	v, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return 0, fmt.Errorf("config: %s: %w", key, err)
	}
	return v, nil
}

func durationVar(key string, fallback time.Duration) (time.Duration, error) {
	raw := os.Getenv(key)
	if raw == "" {
		return fallback, nil
	}
	v, err := time.ParseDuration(raw)
	if err != nil {
		return 0, fmt.Errorf("config: %s: %w", key, err)
	}
	return v, nil
}

func hubtelConfig() HubtelConfig {
	return HubtelConfig{
		ClientID:        os.Getenv("HUBTEL_CLIENT_ID"),
		ClientSecret:    os.Getenv("HUBTEL_CLIENT_SECRET"),
		MerchantAccount: os.Getenv("HUBTEL_MERCHANT_ACCOUNT"),
		BaseURL:         stringVar("HUBTEL_BASE_URL", "https://payproxyapi.hubtel.com"),
	}
}

func paystackConfig() PaystackConfig {
	return PaystackConfig{
		SecretKey: os.Getenv("PAYSTACK_SECRET_KEY"),
		BaseURL:   stringVar("PAYSTACK_BASE_URL", "https://api.paystack.co"),
	}
}

func moolreConfig() MoolreConfig {
	user := os.Getenv("MOOLRE_API_USER")
	key := os.Getenv("MOOLRE_API_KEY")
	pub := os.Getenv("MOOLRE_API_PUBKEY")
	acct := os.Getenv("MOOLRE_ACCOUNT_NUMBER")
	// Back-compat: single MOOLRE_API_KEY sets all three if dedicated vars are empty.
	if user == "" && key != "" {
		user = key
	}
	if pub == "" && key != "" {
		pub = key
	}
	return MoolreConfig{
		APIUser:       user,
		APIKey:        key,
		APIPubKey:     pub,
		AccountNumber: acct,
		BaseURL:       stringVar("MOOLRE_BASE_URL", "https://api.moolre.com"),
	}
}

func rateLimitConfig() (RateLimitConfig, error) {
	enabled, err := boolVar("RATE_LIMIT_ENABLED", true)
	if err != nil {
		return RateLimitConfig{}, fmt.Errorf("config: RATE_LIMIT_ENABLED: %w", err)
	}
	rps, err := intVar("RATE_LIMIT_RPS", 60)
	if err != nil {
		return RateLimitConfig{}, fmt.Errorf("config: RATE_LIMIT_RPS: %w", err)
	}
	ipPerMin, err := intVar("RATE_LIMIT_IP_PER_MINUTE", rps)
	if err != nil {
		return RateLimitConfig{}, fmt.Errorf("config: RATE_LIMIT_IP_PER_MINUTE: %w", err)
	}
	// Explicit per-IP override if RATE_LIMIT_RPS was customized.
	if v := os.Getenv("RATE_LIMIT_RPS"); v != "" {
		ipPerMin = rps
	}
	apiKeyPerMin, err := intVar("RATE_LIMIT_API_KEY_PER_MINUTE", 300)
	if err != nil {
		return RateLimitConfig{}, fmt.Errorf("config: RATE_LIMIT_API_KEY_PER_MINUTE: %w", err)
	}
	webhookPerMin, err := intVar("RATE_LIMIT_WEBHOOK_PER_MINUTE", 600)
	if err != nil {
		return RateLimitConfig{}, fmt.Errorf("config: RATE_LIMIT_WEBHOOK_PER_MINUTE: %w", err)
	}
	if ipPerMin < 1 {
		ipPerMin = 60
	}
	if apiKeyPerMin < 1 {
		apiKeyPerMin = 300
	}
	if webhookPerMin < 1 {
		webhookPerMin = 600
	}
	return RateLimitConfig{
		Enabled:          enabled,
		RPS:              rps,
		IPPerMinute:      ipPerMin,
		APIKeyPerMinute:  apiKeyPerMin,
		WebhookPerMinute: webhookPerMin,
	}, nil
}

func boolVar(key string, fallback bool) (bool, error) {
	raw := os.Getenv(key)
	if raw == "" {
		return fallback, nil
	}
	v, err := strconv.ParseBool(raw)
	if err != nil {
		return false, err
	}
	return v, nil
}
