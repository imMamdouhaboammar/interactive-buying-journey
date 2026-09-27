// Copyright (c) 2026 Mamdouh Aboammar
// SPDX-License-Identifier: PolyForm-Shield-1.0.0

package postgres_test

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/imMamdouhaboammar/interactive-buying-journey/internal/storage/postgres"
	"github.com/jackc/pgx/v5"
)

func getTestDSN(t *testing.T) string {
	dsn := os.Getenv("IBJ_TEST_DATABASE_URL")
	if dsn == "" {
		dsn = "postgres://localhost:5432/postgres?sslmode=disable"
	}
	return dsn
}

func setupTestDB(t *testing.T) (*postgres.DB, func()) {
	dsn := getTestDSN(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	db, err := postgres.New(ctx, dsn)
	if err != nil {
		if os.Getenv("IBJ_REQUIRE_DB") == "1" {
			t.Fatalf("failed to connect to postgres: %v", err)
		}
		t.Skipf("skipping postgres integration test (no database connection): %v", err)
	}

	// Run migrations
	if err := postgres.RunMigrations(dsn); err != nil {
		db.Close()
		t.Fatalf("failed to run migrations: %v", err)
	}

	// Ensure ibj_test_app role exists for RLS testing
	_, _ = db.Pool().Exec(context.Background(), `
		DO $$
		BEGIN
			IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = 'ibj_test_app') THEN
				CREATE ROLE ibj_test_app NOSUPERUSER NOBYPASSRLS;
			END IF;
		END $$;
		GRANT ALL PRIVILEGES ON ALL TABLES IN SCHEMA public TO ibj_test_app;
	`)

	cleanup := func() {
		db.Close()
	}

	return db, cleanup
}

func TestPostgres_MigrationsAndRLS(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()
	tenantA := "tenant_isol_a"
	tenantB := "tenant_isol_b"

	// Seed tenant records
	_, err := db.Pool().Exec(ctx, `
		INSERT INTO tenants (tenant_id, name, secret_current)
		VALUES ($1, 'Tenant A', 'secret_a'), ($2, 'Tenant B', 'secret_b')
		ON CONFLICT (tenant_id) DO NOTHING
	`, tenantA, tenantB)
	if err != nil {
		t.Fatalf("failed to seed tenants: %v", err)
	}

	t.Run("TC-ISOL-01: queries return only the authenticated tenant rows", func(t *testing.T) {
		// Clean up previous runs
		_ = db.WithTenantTx(ctx, tenantA, func(tx pgx.Tx) error {
			_, err := tx.Exec(ctx, "DELETE FROM variants WHERE tenant_id = $1", tenantA)
			return err
		})
		_ = db.WithTenantTx(ctx, tenantB, func(tx pgx.Tx) error {
			_, err := tx.Exec(ctx, "DELETE FROM variants WHERE tenant_id = $1", tenantB)
			return err
		})

		// Insert variant under tenantA
		err := db.WithTenantTx(ctx, tenantA, func(tx pgx.Tx) error {
			_, err := tx.Exec(ctx, `
				INSERT INTO variants (tenant_id, variant_id, product_id, sku, title, category, currency, price_minor, inventory_status, source_updated_at)
				VALUES ($1, 'var_a_1', 'prod_a', 'SKU-A', 'Laptop A', 'laptops', 'USD', 1000, 'in_stock', NOW())
			`, tenantA)
			return err
		})
		if err != nil {
			t.Fatalf("failed to insert tenantA variant: %v", err)
		}

		// Insert variant under tenantB
		err = db.WithTenantTx(ctx, tenantB, func(tx pgx.Tx) error {
			_, err := tx.Exec(ctx, `
				INSERT INTO variants (tenant_id, variant_id, product_id, sku, title, category, currency, price_minor, inventory_status, source_updated_at)
				VALUES ($1, 'var_b_1', 'prod_b', 'SKU-B', 'Laptop B', 'laptops', 'USD', 2000, 'in_stock', NOW())
			`, tenantB)
			return err
		})
		if err != nil {
			t.Fatalf("failed to insert tenantB variant: %v", err)
		}

		// Query as tenantA under non-superuser role
		err = db.WithTenantTx(ctx, tenantA, func(tx pgx.Tx) error {
			_, _ = tx.Exec(ctx, "SET ROLE ibj_test_app")
			rows, err := tx.Query(ctx, "SELECT variant_id FROM variants")
			if err != nil {
				return err
			}
			defer rows.Close()

			var ids []string
			for rows.Next() {
				var id string
				if err := rows.Scan(&id); err != nil {
					return err
				}
				ids = append(ids, id)
			}

			if len(ids) != 1 || ids[0] != "var_a_1" {
				t.Fatalf("expected only tenantA variants ['var_a_1'], got %v", ids)
			}
			return nil
		})
		if err != nil {
			t.Fatalf("failed querying tenantA: %v", err)
		}
	})

	t.Run("TC-ISOL-02: query without tenant context returns zero rows", func(t *testing.T) {
		tx, err := db.Pool().Begin(ctx)
		if err != nil {
			t.Fatalf("failed to begin tx: %v", err)
		}
		defer func() { _ = tx.Rollback(ctx) }()

		// Switch to non-superuser role and explicitly reset session tenant
		_, _ = tx.Exec(ctx, "SET ROLE ibj_test_app")
		_, _ = tx.Exec(ctx, "RESET app.current_tenant")

		rows, err := tx.Query(ctx, "SELECT variant_id FROM variants")
		if err != nil {
			t.Fatalf("query failed: %v", err)
		}
		defer rows.Close()

		count := 0
		for rows.Next() {
			count++
		}

		if count != 0 {
			t.Fatalf("TC-ISOL-02 invariant violated: query without tenant context returned %d rows; must return 0", count)
		}
	})

	t.Run("TC-ISOL-03: feed_batches inserted by tenantA cannot be seen by tenantB", func(t *testing.T) {
		batchID := "batch_isol_test_1"
		err := db.WithTenantTx(ctx, tenantA, func(tx pgx.Tx) error {
			_, err := tx.Exec(ctx, `
				INSERT INTO feed_batches (tenant_id, batch_id, source, source_version, payload_hash, state)
				VALUES ($1, $2, 'test', 'v1', 'hash_123', 'RECEIVED')
				ON CONFLICT (tenant_id, batch_id) DO NOTHING
			`, tenantA, batchID)
			return err
		})
		if err != nil {
			t.Fatalf("failed to insert batch: %v", err)
		}

		// Query as tenantB
		err = db.WithTenantTx(ctx, tenantB, func(tx pgx.Tx) error {
			_, _ = tx.Exec(ctx, "SET ROLE ibj_test_app")
			var count int
			err := tx.QueryRow(ctx, "SELECT count(*) FROM feed_batches WHERE batch_id = $1", batchID).Scan(&count)
			if err != nil {
				return err
			}
			if count != 0 {
				t.Fatalf("tenantB can see tenantA batch! count = %d", count)
			}
			return nil
		})
		if err != nil {
			t.Fatalf("failed querying tenantB: %v", err)
		}
	})

	t.Run("TC-ISOL-04: cross-tenant insert/update blocked by RLS WITH CHECK", func(t *testing.T) {
		err := db.WithTenantTx(ctx, tenantA, func(tx pgx.Tx) error {
			_, _ = tx.Exec(ctx, "SET ROLE ibj_test_app")
			// Attempt to insert a row with tenant_id = tenantB while session is tenantA
			_, err := tx.Exec(ctx, `
				INSERT INTO variants (tenant_id, variant_id, product_id, sku, title, category, currency, price_minor, inventory_status, source_updated_at)
				VALUES ($1, 'cross_var', 'prod_c', 'SKU-C', 'Cross', 'laptops', 'USD', 1000, 'in_stock', NOW())
			`, tenantB)
			if err == nil {
				return errors.New("expected RLS check violation for cross-tenant insert, got nil")
			}
			return err
		})
		if err == nil || !strings.Contains(err.Error(), "violates row-level security policy") {
			t.Fatalf("expected error mentioning row-level security policy violation, got: %v", err)
		}
	})

	t.Run("migration rollback and re-apply works cleanly", func(t *testing.T) {
		dsn := getTestDSN(t)
		if err := postgres.RollbackMigrations(dsn); err != nil {
			t.Fatalf("failed rollback: %v", err)
		}
		if err := postgres.RunMigrations(dsn); err != nil {
			t.Fatalf("failed re-apply migration: %v", err)
		}
	})
}
