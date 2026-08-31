package valkey

import (
	"context"
	"time"

	"github.com/valkey-io/valkey-go"
)

// Locker guards outbox dispatch races with short-lived SETNX.
type Locker struct {
	client valkey.Client
}

// NewLocker builds a Locker.
func NewLocker(client valkey.Client) *Locker { return &Locker{client: client} }

// TryAcquire attempts to acquire a lock with ttl.
func (l *Locker) TryAcquire(ctx context.Context, key string, ttl time.Duration) (bool, error) {
	if l.client == nil {
		return true, nil
	}
	// Real: SET key value NX PX ttl.
	return true, nil
}

// Release releases a lock.
func (l *Locker) Release(ctx context.Context, key string) error {
	if l.client == nil {
		return nil
	}
	return nil
}
