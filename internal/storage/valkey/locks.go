package valkey

import (
	"context"
	"fmt"
	"time"

	"github.com/valkey-io/valkey-go"
)

// Locker guards outbox dispatch races with short-lived SETNX.
type Locker struct {
	client valkey.Client
}

// NewLocker builds a Locker.
func NewLocker(client valkey.Client) *Locker { return &Locker{client: client} }

// TryAcquire attempts to acquire a lock with ttl (SET key NX PX ttl).
// Returns true when the lock was acquired; false when held elsewhere.
func (l *Locker) TryAcquire(ctx context.Context, key string, ttl time.Duration) (bool, error) {
	if l.client == nil {
		return true, nil
	}
	resp := l.client.Do(ctx, l.client.B().Set().Key(key).Value("1").Nx().Px(ttl).Build())
	if err := resp.Error(); err != nil {
		if valkey.IsValkeyNil(err) {
			return false, nil
		}
		return false, fmt.Errorf("valkey: set nx: %w", err)
	}
	ok, err := resp.AsBool()
	if err != nil {
		if valkey.IsValkeyNil(err) {
			return false, nil
		}
		return false, fmt.Errorf("valkey: set nx: %w", err)
	}
	return ok, nil
}

// Release releases a lock.
func (l *Locker) Release(ctx context.Context, key string) error {
	if l.client == nil {
		return nil
	}
	if err := l.client.Do(ctx, l.client.B().Del().Key(key).Build()).Error(); err != nil {
		return fmt.Errorf("valkey: del lock: %w", err)
	}
	return nil
}
