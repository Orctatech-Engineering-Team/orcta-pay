package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// DB holds the pool.
type DB struct {
	pool *pgxpool.Pool
}

// NewDB builds a DB.
func NewDB(pool *pgxpool.Pool) *DB { return &DB{pool: pool} }

// WithTx runs fn inside a transaction. fn receives a Tx-bound queries set.
func (db *DB) WithTx(ctx context.Context, fn func(ctx context.Context, tx pgx.Tx) error) error {
	tx, err := db.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("postgres: begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := fn(ctx, tx); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("postgres: commit: %w", err)
	}
	return nil
}

// Pool returns the underlying pool.
func (db *DB) Pool() *pgxpool.Pool { return db.pool }
