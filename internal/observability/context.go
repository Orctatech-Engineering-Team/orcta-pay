package observability

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log/slog"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

type contextKey int

const (
	loggerKey contextKey = iota
	wideEventKey
)

// WithLogger returns a context carrying logger.
func WithLogger(ctx context.Context, logger *slog.Logger) context.Context {
	return context.WithValue(ctx, loggerKey, logger)
}

// LoggerFromContext returns the request-scoped logger.
func LoggerFromContext(ctx context.Context) *slog.Logger {
	if logger, ok := ctx.Value(loggerKey).(*slog.Logger); ok {
		return logger
	}
	return slog.Default()
}

// TraceIDFromContext returns the current trace ID.
func TraceIDFromContext(ctx context.Context) string {
	sc := trace.SpanContextFromContext(ctx)
	if !sc.HasTraceID() {
		return ""
	}
	return sc.TraceID().String()
}

// NewTraceID generates a random trace ID for local runs.
func NewTraceID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}

// Span is a vendor-agnostic handle.
type Span struct{ span trace.Span }

// End closes the span.
func (s Span) End() { s.span.End() }

// RecordError marks the span failed.
func (s Span) RecordError(err error) {
	if err == nil {
		return
	}
	s.span.RecordError(err)
	s.span.SetStatus(codes.Error, err.Error())
}

// SetString attaches a string attribute.
func (s Span) SetString(key, value string) {
	s.span.SetAttributes(attribute.String(key, value))
}

// SetInt attaches an integer attribute.
func (s Span) SetInt(key string, value int64) {
	s.span.SetAttributes(attribute.Int64(key, value))
}

// StartSpan begins a child span.
func StartSpan(ctx context.Context, name string) (context.Context, Span) {
	ctx, span := otel.Tracer(tracerName).Start(ctx, name)
	return ctx, Span{span: span}
}

const tracerName = "github.com/Orctatech-Engineering-Team/orcta-pay"
