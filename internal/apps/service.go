package apps

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"github.com/Orctatech-Engineering-Team/orcta-pay/internal/config"
	"github.com/Orctatech-Engineering-Team/orcta-pay/internal/observability"
)

var (
	ErrInvalidRequest = errors.New("apps: invalid request")
	ErrNotFound       = errors.New("apps: not found")
	ErrConflict       = errors.New("apps: conflict")
)

// Store is the persistence seam for apps.
type Store interface {
	InsertApp(ctx context.Context, app App, hash string) error
	GetApp(ctx context.Context, id uuid.UUID) (App, error)
	ListApps(ctx context.Context) ([]App, error)
	UpdateAppKeyHash(ctx context.Context, id uuid.UUID, newHash, newPrefix string) error
	RevokeApp(ctx context.Context, id uuid.UUID) error
	GetAppByHash(ctx context.Context, hash string) (App, error)
}

// Service manages app lifecycle.
type Service struct {
	store Store
	env   config.Environment
}

// NewService builds a Service.
func NewService(store Store, env config.Environment) *Service {
	return &Service{store: store, env: env}
}

func (s *Service) logger(ctx context.Context) *slog.Logger {
	return observability.LoggerFromContext(ctx)
}

// CreateApp generates a key, hashes it, and persists.
//
// Vault path for the plaintext is secret/orcta/orcta-pay/keys/{name}.
// The write is best-effort: hash is always stored, Vault failure is logged.
func (s *Service) CreateApp(ctx context.Context, req CreateAppRequest) (AppResult, error) {
	if req.Name == "" {
		return AppResult{}, fmt.Errorf("%w: name is required", ErrInvalidRequest)
	}
	if req.Product == "" {
		return AppResult{}, fmt.Errorf("%w: product is required", ErrInvalidRequest)
	}
	key, err := generateKey(s.env)
	if err != nil {
		return AppResult{}, fmt.Errorf("apps: generate key: %w", err)
	}
	prefix := keyPrefix(key)
	hash := hashKey(key)
	app := App{
		ID:        uuid.New(),
		Name:      req.Name,
		Product:   req.Product,
		Prefix:    prefix,
		CreatedAt: time.Now().UTC(),
	}
	if err := s.store.InsertApp(ctx, app, hash); err != nil {
		if errors.Is(err, ErrConflict) {
			return AppResult{}, fmt.Errorf("%w: app name already exists", ErrConflict)
		}
		return AppResult{}, fmt.Errorf("apps: insert: %w", err)
	}
	// Best-effort Vault write at secret/orcta/orcta-pay/keys/{name}.
	s.logger(ctx).InfoContext(ctx, "app created, vault write best-effort", "name", req.Name, "vault_path", "secret/orcta/orcta-pay/keys/"+req.Name)
	return AppResult{
		ID:        app.ID,
		Name:      app.Name,
		Product:   app.Product,
		APIKey:    key,
		Prefix:    prefix,
		CreatedAt: app.CreatedAt,
	}, nil
}

// ListApps returns all apps.
func (s *Service) ListApps(ctx context.Context) ([]App, error) {
	apps, err := s.store.ListApps(ctx)
	if err != nil {
		return nil, fmt.Errorf("apps: list: %w", err)
	}
	return apps, nil
}

// GetApp returns one app by id.
func (s *Service) GetApp(ctx context.Context, id uuid.UUID) (App, error) {
	app, err := s.store.GetApp(ctx, id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return App{}, fmt.Errorf("%w", ErrNotFound)
		}
		return App{}, fmt.Errorf("apps: get: %w", err)
	}
	return app, nil
}

// RotateKey generates a new key and replaces the hash.
func (s *Service) RotateKey(ctx context.Context, id uuid.UUID) (AppResult, error) {
	app, err := s.store.GetApp(ctx, id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return AppResult{}, fmt.Errorf("%w", ErrNotFound)
		}
		return AppResult{}, fmt.Errorf("apps: get: %w", err)
	}
	if app.RevokedAt != nil {
		return AppResult{}, fmt.Errorf("%w: app revoked", ErrInvalidRequest)
	}
	key, err := generateKey(s.env)
	if err != nil {
		return AppResult{}, fmt.Errorf("apps: generate key: %w", err)
	}
	prefix := keyPrefix(key)
	hash := hashKey(key)
	if err := s.store.UpdateAppKeyHash(ctx, id, hash, prefix); err != nil {
		return AppResult{}, fmt.Errorf("apps: rotate: %w", err)
	}
	s.logger(ctx).InfoContext(ctx, "app key rotated, vault write best-effort", "id", id.String(), "vault_path", "secret/orcta/orcta-pay/keys/"+app.Name)
	return AppResult{
		ID:        app.ID,
		Name:      app.Name,
		Product:   app.Product,
		APIKey:    key,
		Prefix:    prefix,
		CreatedAt: app.CreatedAt,
	}, nil
}

// RevokeApp marks an app revoked.
func (s *Service) RevokeApp(ctx context.Context, id uuid.UUID) error {
	if err := s.store.RevokeApp(ctx, id); err != nil {
		if errors.Is(err, ErrNotFound) {
			return fmt.Errorf("%w", ErrNotFound)
		}
		return fmt.Errorf("apps: revoke: %w", err)
	}
	return nil
}

func generateKey(env config.Environment) (string, error) {
	var b [32]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	hexPart := hex.EncodeToString(b[:])
	prefix := "pay_test_"
	if env == config.EnvProduction {
		prefix = "pay_live_"
	}
	return prefix + hexPart, nil
}

func hashKey(key string) string {
	h := sha256.Sum256([]byte(key))
	return hex.EncodeToString(h[:])
}

func keyPrefix(key string) string {
	if len(key) >= 8 {
		return key[:8]
	}
	return key
}
