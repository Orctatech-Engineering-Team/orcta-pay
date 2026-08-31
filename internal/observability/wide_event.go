package observability

import (
	"context"
	"log/slog"
	"maps"
	"sync"
)

// WideEvent accumulates one structured event per unit of work.
type WideEvent struct {
	mu     sync.Mutex
	fields map[string]any
}

// NewWideEvent returns an empty event.
func NewWideEvent() *WideEvent {
	return &WideEvent{fields: make(map[string]any)}
}

// Set records a single field.
func (e *WideEvent) Set(key string, value any) {
	if e == nil {
		return
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	e.fields[key] = value
}

// SetAll records several fields at once.
func (e *WideEvent) SetAll(fields map[string]any) {
	if e == nil {
		return
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	maps.Copy(e.fields, fields)
}

func (e *WideEvent) attrs() []any {
	e.mu.Lock()
	defer e.mu.Unlock()
	attrs := make([]any, 0, len(e.fields))
	for k, v := range e.fields {
		attrs = append(attrs, slog.Any(k, v))
	}
	return attrs
}

// WithWideEvent returns a context carrying event.
func WithWideEvent(ctx context.Context, event *WideEvent) context.Context {
	return context.WithValue(ctx, wideEventKey, event)
}

// WideEventFromContext returns the event ctx carries.
func WideEventFromContext(ctx context.Context) *WideEvent {
	event, ok := ctx.Value(wideEventKey).(*WideEvent)
	if !ok {
		return nil
	}
	return event
}

// SetEventField contributes a field to the in-flight event.
func SetEventField(ctx context.Context, key string, value any) {
	WideEventFromContext(ctx).Set(key, value)
}

// FlushWideEvent emits the event as one INFO line.
func FlushWideEvent(ctx context.Context, name string) {
	event := WideEventFromContext(ctx)
	if event == nil {
		return
	}
	LoggerFromContext(ctx).InfoContext(ctx, name, event.attrs()...)
}
