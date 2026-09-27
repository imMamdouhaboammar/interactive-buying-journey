// Copyright (c) 2026 Mamdouh Aboammar
// SPDX-License-Identifier: PolyForm-Shield-1.0.0

package postgres

import (
	"context"
	"database/sql"
	"embed"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
)

//go:embed migrations/*.sql
var embedMigrations embed.FS

// DB wraps a PostgreSQL connection pool.
type DB struct {
	pool *pgxpool.Pool
}

// New connects to the PostgreSQL database using DSN.
func New(ctx context.Context, dsn string) (*DB, error) {
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("parse postgres dsn: %w", err)
	}

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("connect postgres pool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping postgres: %w", err)
	}

	return &DB{pool: pool}, nil
}

// Close closes the connection pool.
func (db *DB) Close() {
	if db.pool != nil {
		db.pool.Close()
	}
}

// Pool returns the underlying pgxpool.Pool.
func (db *DB) Pool() *pgxpool.Pool {
	return db.pool
}

// RunMigrations applies all pending migrations using Goose.
func RunMigrations(dsn string) error {
	goose.SetBaseFS(embedMigrations)
	if err := goose.SetDialect("postgres"); err != nil {
		return fmt.Errorf("set dialect: %w", err)
	}

	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return fmt.Errorf("open sql connection for migrations: %w", err)
	}
	defer func() { _ = db.Close() }()

	if err := goose.Up(db, "migrations"); err != nil {
		return fmt.Errorf("run goose up: %w", err)
	}

	return nil
}

// RollbackMigrations rolls back the most recent migration using Goose.
func RollbackMigrations(dsn string) error {
	goose.SetBaseFS(embedMigrations)
	if err := goose.SetDialect("postgres"); err != nil {
		return fmt.Errorf("set dialect: %w", err)
	}

	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return fmt.Errorf("open sql connection for rollback: %w", err)
	}
	defer func() { _ = db.Close() }()

	if err := goose.Down(db, "migrations"); err != nil {
		return fmt.Errorf("run goose down: %w", err)
	}

	return nil
}

// WithTenantTx executes fn inside a transaction serialized per tenant via advisory lock
// and isolated via app.current_tenant session variable.
func (db *DB) WithTenantTx(ctx context.Context, tenantID string, fn func(tx pgx.Tx) error) error {
	tx, err := db.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	// 1. Transaction advisory lock to serialize batches for this tenant
	if _, err := tx.Exec(ctx, "SELECT pg_advisory_xact_lock(hashtext($1))", tenantID); err != nil {
		return fmt.Errorf("acquire tenant advisory lock: %w", err)
	}

	// 2. Set tenant context for Row-Level Security
	if _, err := tx.Exec(ctx, "SELECT set_config('app.current_tenant', $1, true)", tenantID); err != nil {
		return fmt.Errorf("set tenant context: %w", err)
	}

	if err := fn(tx); err != nil {
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit tenant tx: %w", err)
	}

	return nil
}

// WithTenantQuery executes fn within a tenant-scoped read-only session context.
func (db *DB) WithTenantQuery(ctx context.Context, tenantID string, fn func(tx pgx.Tx) error) error {
	tx, err := db.pool.BeginTx(ctx, pgx.TxOptions{AccessMode: pgx.ReadOnly})
	if err != nil {
		return fmt.Errorf("begin read-only tx: %w", err)
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	if _, err := tx.Exec(ctx, "SELECT set_config('app.current_tenant', $1, true)", tenantID); err != nil {
		return fmt.Errorf("set tenant context: %w", err)
	}

	if err := fn(tx); err != nil {
		return err
	}

	return tx.Commit(ctx)
}
