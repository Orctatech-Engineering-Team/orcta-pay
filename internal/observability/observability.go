// Package observability provides slog + OTel behind vendor-agnostic accessors.
package observability

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"

	"github.com/Orctatech-Engineering-Team/orcta-pay/internal/config"
)

// Provider holds the telemetry stack and owns shutdown.
type Provider struct {
	Logger   *slog.Logger
	tracer   trace.Tracer
	shutdown []func(context.Context) error
}

// Setup initialises logging and, with an OTLP endpoint, trace export.
func Setup(ctx context.Context, cfg config.ObservabilityConfig) (*Provider, error) {
	logger := NewLogger(cfg, os.Stdout)
	slog.SetDefault(logger)
	p := &Provider{Logger: logger}
	if !cfg.TracingEnabled() {
		p.tracer = noop.NewTracerProvider().Tracer(cfg.ServiceName)
		return p, nil
	}
	exporter, err := otlptracehttp.New(ctx, otlptracehttp.WithEndpointURL(cfg.OTLPEndpoint))
	if err != nil {
		return nil, fmt.Errorf("observability: otlp exporter: %w", err)
	}
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithSampler(sdktrace.ParentBased(sdktrace.TraceIDRatioBased(cfg.TraceSampleRatio))),
		sdktrace.WithResource(newResource(cfg.ServiceName)),
	)
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}))
	p.tracer = tp.Tracer(cfg.ServiceName)
	p.shutdown = append(p.shutdown, tp.Shutdown)
	return p, nil
}

// Tracer returns the configured tracer.
func (p *Provider) Tracer() trace.Tracer { return p.tracer }

// Shutdown flushes exporters.
func (p *Provider) Shutdown(ctx context.Context) error {
	var errs []error
	for _, fn := range p.shutdown {
		errs = append(errs, fn(ctx))
	}
	if err := errors.Join(errs...); err != nil {
		return fmt.Errorf("observability: shutdown: %w", err)
	}
	return nil
}

// NewLogger builds the JSON slog logger.
func NewLogger(cfg config.ObservabilityConfig, w io.Writer) *slog.Logger {
	handler := slog.NewJSONHandler(w, &slog.HandlerOptions{Level: parseLevel(cfg.LogLevel)})
	return slog.New(handler).With(slog.String("service", cfg.ServiceName))
}

func parseLevel(name string) slog.Level {
	switch strings.ToLower(name) {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

func newResource(serviceName string) *resource.Resource {
	return resource.NewSchemaless(attribute.String("service.name", serviceName))
}
