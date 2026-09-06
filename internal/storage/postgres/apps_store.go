package postgres

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/orctatech/orcta-pay/internal/apps"
)

// PostgresAppsStore is the sqlc-backed apps store.
type PostgresAppsStore struct {
	pool *pgxpool.Pool
}

// NewPostgresAppsStore builds a store.
func NewPostgresAppsStore(pool *pgxpool.Pool) *PostgresAppsStore {
	return &PostgresAppsStore{pool: pool}
}

var _ apps.Store = (*PostgresAppsStore)(nil)

// InsertApp inserts an app.
func (s *PostgresAppsStore) InsertApp(ctx context.Context, app apps.App, hash string) error {
	_, err := s.pool.Exec(ctx,
		`INSERT INTO apps (id, name, product, api_key_hash, api_key_prefix, created_at, created_by)
		 VALUES ($1,$2,$3,$4,$5,$6,$7)`,
		app.ID, app.Name, app.Product, hash, app.Prefix, app.CreatedAt, app.CreatedBy,
	)
	if err != nil {
		return fmt.Errorf("postgres: insert app: %w", err)
	}
	return nil
}

// GetApp fetches an app.
func (s *PostgresAppsStore) GetApp(ctx context.Context, id uuid.UUID) (apps.App, error) {
	var a apps.App
	err := s.pool.QueryRow(ctx,
		`SELECT id, name, product, api_key_prefix, created_at, last_used_at, revoked_at, created_by FROM apps WHERE id=$1`, id,
	).Scan(&a.ID, &a.Name, &a.Product, &a.Prefix, &a.CreatedAt, &a.LastUsedAt, &a.RevokedAt, &a.CreatedBy)
	if err != nil {
		return apps.App{}, fmt.Errorf("%w: %w", apps.ErrNotFound, err)
	}
	return a, nil
}

// GetAppByHash fetches by hash.
func (s *PostgresAppsStore) GetAppByHash(ctx context.Context, hash string) (apps.App, error) {
	var a apps.App
	err := s.pool.QueryRow(ctx,
		`SELECT id, name, product, api_key_prefix, created_at, last_used_at, revoked_at, created_by FROM apps WHERE api_key_hash=$1`, hash,
	).Scan(&a.ID, &a.Name, &a.Product, &a.Prefix, &a.CreatedAt, &a.LastUsedAt, &a.RevokedAt, &a.CreatedBy)
	if err != nil {
		return apps.App{}, fmt.Errorf("%w: %w", apps.ErrNotFound, err)
	}
	return a, nil
}

// ListApps lists all apps.
func (s *PostgresAppsStore) ListApps(ctx context.Context) ([]apps.App, error) {
	rows, err := s.pool.Query(ctx, `SELECT id, name, product, api_key_prefix, created_at, last_used_at, revoked_at, created_by FROM apps ORDER BY created_at DESC`)
	if err != nil {
		return nil, fmt.Errorf("postgres: list apps: %w", err)
	}
	defer rows.Close()
	var out []apps.App
	for rows.Next() {
		var a apps.App
		if err := rows.Scan(&a.ID, &a.Name, &a.Product, &a.Prefix, &a.CreatedAt, &a.LastUsedAt, &a.RevokedAt, &a.CreatedBy); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// UpdateAppKeyHash rotates the key.
func (s *PostgresAppsStore) UpdateAppKeyHash(ctx context.Context, id uuid.UUID, newHash, newPrefix string) error {
	_, err := s.pool.Exec(ctx, `UPDATE apps SET api_key_hash=$2, api_key_prefix=$3 WHERE id=$1`, id, newHash, newPrefix)
	if err != nil {
		return fmt.Errorf("postgres: update app key: %w", err)
	}
	return nil
}

// RevokeApp marks revoked.
func (s *PostgresAppsStore) RevokeApp(ctx context.Context, id uuid.UUID) error {
	ct, err := s.pool.Exec(ctx, `UPDATE apps SET revoked_at=now() WHERE id=$1 AND revoked_at IS NULL`, id)
	if err != nil {
		return fmt.Errorf("postgres: revoke app: %w", err)
	}
	if ct.RowsAffected() == 0 {
		// Check existence.
		var exists bool
		_ = s.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM apps WHERE id=$1)`, id).Scan(&exists)
		if !exists {
			return fmt.Errorf("%w", apps.ErrNotFound)
		}
	}
	return nil
}
