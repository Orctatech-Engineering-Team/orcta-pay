package config

import (
	"strings"
	"testing"
	"time"
)

func validTestConfig() Config {
	return Config{
		Environment: EnvDevelopment,
		HTTP: HTTPConfig{
			Port: 8080,
		},
		Database: DatabaseConfig{URL: "postgres://localhost/test", MaxConns: 1},
		Valkey:   ValkeyConfig{Addr: "localhost:6379"},
		Observ: ObservabilityConfig{
			LogLevel:         "info",
			TraceSampleRatio: 1,
		},
		Payments: PaymentsConfig{Primary: "paystack", PendingTimeout: time.Minute},
	}
}

func TestValidateRequiresAPIKeyInProduction(t *testing.T) {
	cfg := validTestConfig()
	cfg.Environment = EnvProduction

	err := cfg.Validate()
	if err == nil || !strings.Contains(err.Error(), "ORCTA_PAY_API_KEY is required in production") {
		t.Fatalf("Validate() error = %v, want missing production API key", err)
	}

	cfg.Auth.APIKey = "service-secret"
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() with API key: %v", err)
	}
}

func TestValidateAllowsMissingAPIKeyInDevelopment(t *testing.T) {
	cfg := validTestConfig()
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate(): %v", err)
	}
}
