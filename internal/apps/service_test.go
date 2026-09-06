package apps

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"sync"
	"testing"

	"github.com/google/uuid"

	"github.com/Orctatech-Engineering-Team/orcta-pay/internal/config"
)

type fakeStore struct {
	mu   sync.Mutex
	apps map[uuid.UUID]App
	hash map[string]uuid.UUID
	name map[string]uuid.UUID
}

func newFakeStore() *fakeStore {
	return &fakeStore{
		apps: make(map[uuid.UUID]App),
		hash: make(map[string]uuid.UUID),
		name: make(map[string]uuid.UUID),
	}
}

func (f *fakeStore) InsertApp(_ context.Context, app App, hash string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.name[app.Name]; ok {
		return ErrConflict
	}
	if _, ok := f.hash[hash]; ok {
		return ErrConflict
	}
	f.apps[app.ID] = app
	f.hash[hash] = app.ID
	f.name[app.Name] = app.ID
	return nil
}

func (f *fakeStore) GetApp(_ context.Context, id uuid.UUID) (App, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	a, ok := f.apps[id]
	if !ok {
		return App{}, ErrNotFound
	}
	return a, nil
}

func (f *fakeStore) ListApps(_ context.Context) ([]App, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]App, 0, len(f.apps))
	for _, a := range f.apps {
		out = append(out, a)
	}
	return out, nil
}

func (f *fakeStore) UpdateAppKeyHash(_ context.Context, id uuid.UUID, newHash, newPrefix string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	a, ok := f.apps[id]
	if !ok {
		return ErrNotFound
	}
	for h, oid := range f.hash {
		if oid == id {
			delete(f.hash, h)
			break
		}
	}
	if _, ok := f.hash[newHash]; ok {
		return ErrConflict
	}
	a.Prefix = newPrefix
	f.apps[id] = a
	f.hash[newHash] = id
	return nil
}

func (f *fakeStore) RevokeApp(_ context.Context, id uuid.UUID) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	a, ok := f.apps[id]
	if !ok {
		return ErrNotFound
	}
	if a.RevokedAt != nil {
		return nil
	}
	// Simulate revoked check via stored value; service will check status externally if needed
	// For test simplicity, just set local.
	now := a.CreatedAt
	a.RevokedAt = &now
	f.apps[id] = a
	return nil
}

func (f *fakeStore) GetAppByHash(_ context.Context, hash string) (App, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	id, ok := f.hash[hash]
	if !ok {
		return App{}, ErrNotFound
	}
	return f.apps[id], nil
}

func TestCreateApp(t *testing.T) {
	store := newFakeStore()
	svc := NewService(store, config.EnvDevelopment)
	res, err := svc.CreateApp(context.Background(), CreateAppRequest{Name: "myapp", Product: "orctago"})
	if err != nil {
		t.Fatalf("CreateApp: %v", err)
	}
	if res.APIKey == "" {
		t.Fatal("expected api_key")
	}
	if !strings.HasPrefix(res.APIKey, "pay_test_") {
		t.Fatalf("prefix = %q, want pay_test_", res.APIKey)
	}
	if res.Prefix != res.APIKey[:8] {
		t.Fatalf("prefix %q != key[:8] %q", res.Prefix, res.APIKey[:8])
	}
	// Hash is not plaintext: compute hash and ensure store has it.
	h := sha256.Sum256([]byte(res.APIKey))
	hash := hex.EncodeToString(h[:])
	store.mu.Lock()
	_, ok := store.hash[hash]
	store.mu.Unlock()
	if !ok {
		t.Fatal("hash not stored")
	}
	if hash == res.APIKey {
		t.Fatal("hash equals plaintext")
	}
	// Returned key once: ensure App without key does not expose it.
	list, err := svc.ListApps(context.Background())
	if err != nil {
		t.Fatalf("ListApps: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("list len = %d, want 1", len(list))
	}
	if strings.Contains(list[0].Name, res.APIKey) {
		t.Fatal("list exposed key")
	}
}

func TestCreateAppLivePrefix(t *testing.T) {
	store := newFakeStore()
	svc := NewService(store, config.EnvProduction)
	res, err := svc.CreateApp(context.Background(), CreateAppRequest{Name: "liveapp", Product: "pos"})
	if err != nil {
		t.Fatalf("CreateApp: %v", err)
	}
	if !strings.HasPrefix(res.APIKey, "pay_live_") {
		t.Fatalf("live prefix = %q, want pay_live_", res.APIKey)
	}
}

func TestRotateKey(t *testing.T) {
	store := newFakeStore()
	svc := NewService(store, config.EnvDevelopment)
	res, err := svc.CreateApp(context.Background(), CreateAppRequest{Name: "rot", Product: "orctago"})
	if err != nil {
		t.Fatalf("CreateApp: %v", err)
	}
	h1 := sha256.Sum256([]byte(res.APIKey))
	oldHash := hex.EncodeToString(h1[:])
	res2, err := svc.RotateKey(context.Background(), res.ID)
	if err != nil {
		t.Fatalf("RotateKey: %v", err)
	}
	if res2.APIKey == res.APIKey {
		t.Fatal("rotated key same as old")
	}
	h2 := sha256.Sum256([]byte(res2.APIKey))
	newHash := hex.EncodeToString(h2[:])
	if oldHash == newHash {
		t.Fatal("hash not changed")
	}
	store.mu.Lock()
	_, oldExists := store.hash[oldHash]
	_, newExists := store.hash[newHash]
	store.mu.Unlock()
	if oldExists {
		t.Fatal("old hash still present")
	}
	if !newExists {
		t.Fatal("new hash not stored")
	}
}

func TestRevoke(t *testing.T) {
	store := newFakeStore()
	svc := NewService(store, config.EnvDevelopment)
	res, err := svc.CreateApp(context.Background(), CreateAppRequest{Name: "torevoke", Product: "pos"})
	if err != nil {
		t.Fatalf("CreateApp: %v", err)
	}
	if err := svc.RevokeApp(context.Background(), res.ID); err != nil {
		t.Fatalf("RevokeApp: %v", err)
	}
	// Revoked app should still be fetchable but with RevokedAt set
	app, err := svc.GetApp(context.Background(), res.ID)
	if err != nil {
		t.Fatalf("GetApp: %v", err)
	}
	if app.RevokedAt == nil {
		t.Fatal("expected revoked_at")
	}
}

func TestCreateAppValidation(t *testing.T) {
	store := newFakeStore()
	svc := NewService(store, config.EnvDevelopment)
	_, err := svc.CreateApp(context.Background(), CreateAppRequest{Name: "", Product: "orctago"})
	if err == nil {
		t.Fatal("expected error for empty name")
	}
	_, err = svc.CreateApp(context.Background(), CreateAppRequest{Name: "n", Product: ""})
	if err == nil {
		t.Fatal("expected error for empty product")
	}
}
