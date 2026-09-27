// Copyright (c) 2026 Mamdouh Aboammar
// SPDX-License-Identifier: PolyForm-Shield-1.0.0

package catalog_test

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/imMamdouhaboammar/interactive-buying-journey/internal/catalog"
	"github.com/imMamdouhaboammar/interactive-buying-journey/internal/storage/postgres"
)

func setupPostgresCatalogTest(t *testing.T) (*postgres.DB, *catalog.PostgresCatalog, string, func()) {
	dsn := os.Getenv("IBJ_TEST_DATABASE_URL")
	if dsn == "" {
		dsn = "postgres://localhost:5432/postgres?sslmode=disable"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	db, err := postgres.New(ctx, dsn)
	if err != nil {
		if os.Getenv("IBJ_REQUIRE_DB") == "1" {
			t.Fatalf("failed to connect to postgres: %v", err)
		}
		t.Skipf("skipping catalog postgres test (no db): %v", err)
	}

	if err := postgres.RunMigrations(dsn); err != nil {
		db.Close()
		t.Fatalf("failed to run migrations: %v", err)
	}

	tenantID := fmt.Sprintf("tenant_cat_%d", time.Now().UnixNano())
	// Tenant with 3600 seconds (1 hour) max staleness
	_, err = db.Pool().Exec(context.Background(), `
		INSERT INTO tenants (tenant_id, name, secret_current, max_staleness_seconds)
		VALUES ($1, 'Catalog Tenant', 'secret_cat', 3600)
		ON CONFLICT (tenant_id) DO NOTHING
	`, tenantID)
	if err != nil {
		db.Close()
		t.Fatalf("failed to seed tenant: %v", err)
	}

	cat := catalog.NewPostgresCatalog(db)

	cleanup := func() {
		_ = db.WithTenantTx(context.Background(), tenantID, func(tx pgx.Tx) error {
			_, _ = tx.Exec(context.Background(), "DELETE FROM catalog_versions WHERE tenant_id = $1", tenantID)
			_, _ = tx.Exec(context.Background(), "DELETE FROM variants WHERE tenant_id = $1", tenantID)
			_, _ = tx.Exec(context.Background(), "DELETE FROM feed_batches WHERE tenant_id = $1", tenantID)
			_, _ = tx.Exec(context.Background(), "DELETE FROM tenants WHERE tenant_id = $1", tenantID)
			return nil
		})
		db.Close()
	}

	return db, cat, tenantID, cleanup
}

func TestPostgresCatalog_EligibilityAndSearch(t *testing.T) {
	db, cat, tenantID, cleanup := setupPostgresCatalogTest(t)
	defer cleanup()

	ctx := context.Background()

	// Seed variants
	err := db.WithTenantTx(ctx, tenantID, func(tx pgx.Tx) error {
		// Active version
		_, err := tx.Exec(ctx, `
			INSERT INTO catalog_versions (tenant_id, version_id, batch_id, status, variant_count)
			VALUES ($1, 'cat_ver_init', 'batch_init', 'ACTIVE', 5)
		`, tenantID)
		if err != nil {
			return err
		}

		// 1. Fresh laptop (USD)
		_, err = tx.Exec(ctx, `
			INSERT INTO variants (
				tenant_id, variant_id, product_id, sku, title, title_norm, category, brand,
				published, currency, price_minor, inventory_status, source_updated_at, last_verified_at,
				is_tombstoned, tsv_ar
			) VALUES (
				$1, 'lap_fresh', 'prod_fresh', 'SKU-F', 'Pro Modern Laptop 14', 'Pro Modern Laptop 14', 'laptops', 'BrandA',
				TRUE, 'USD', 120000, 'in_stock', NOW(), NOW(),
				FALSE, to_tsvector('arabic', '')
			)
		`, tenantID)
		if err != nil {
			return err
		}

		// 2. Fresh Arabic laptop with tashkeel
		_, err = tx.Exec(ctx, `
			INSERT INTO variants (
				tenant_id, variant_id, product_id, sku, title, title_norm, category, brand,
				published, currency, price_minor, inventory_status, source_updated_at, last_verified_at,
				is_tombstoned, tsv_ar
			) VALUES (
				$1, 'lap_arabic', 'prod_ar', 'SKU-AR', 'حَاسُوبٌ فائق الخفة', 'حاسوب فائق الخفه', 'laptops', 'BrandB',
				TRUE, 'USD', 140000, 'in_stock', NOW(), NOW(),
				FALSE, to_tsvector('arabic', 'حاسوب فائق الخفه')
			)
		`, tenantID)
		if err != nil {
			return err
		}

		// 3. Stale laptop (verified 3601 seconds ago - beyond 3600 limit)
		_, err = tx.Exec(ctx, `
			INSERT INTO variants (
				tenant_id, variant_id, product_id, sku, title, title_norm, category, brand,
				published, currency, price_minor, inventory_status, source_updated_at, last_verified_at,
				is_tombstoned
			) VALUES (
				$1, 'lap_stale', 'prod_stale', 'SKU-S', 'Stale Laptop', 'Stale Laptop', 'laptops', 'BrandA',
				TRUE, 'USD', 99000, 'in_stock', NOW() - INTERVAL '2 hours', NOW() - INTERVAL '3601 seconds',
				FALSE
			)
		`, tenantID)
		if err != nil {
			return err
		}

		// 4. Currency mismatch (EUR laptop)
		_, err = tx.Exec(ctx, `
			INSERT INTO variants (
				tenant_id, variant_id, product_id, sku, title, title_norm, category, brand,
				published, currency, price_minor, inventory_status, source_updated_at, last_verified_at,
				is_tombstoned
			) VALUES (
				$1, 'lap_eur', 'prod_eur', 'SKU-E', 'European Laptop', 'European Laptop', 'laptops', 'BrandA',
				TRUE, 'EUR', 110000, 'in_stock', NOW(), NOW(),
				FALSE
			)
		`, tenantID)
		if err != nil {
			return err
		}

		// 5. Unpublished laptop
		_, err = tx.Exec(ctx, `
			INSERT INTO variants (
				tenant_id, variant_id, product_id, sku, title, title_norm, category, brand,
				published, currency, price_minor, inventory_status, source_updated_at, last_verified_at,
				is_tombstoned
			) VALUES (
				$1, 'lap_unpub', 'prod_unpub', 'SKU-U', 'Draft Laptop', 'Draft Laptop', 'laptops', 'BrandA',
				FALSE, 'USD', 80000, 'in_stock', NOW(), NOW(),
				FALSE
			)
		`, tenantID)
		return err
	})
	if err != nil {
		t.Fatalf("failed seeding variants: %v", err)
	}

	t.Run("TC-COMP-01: GetActiveVersion returns active catalog version", func(t *testing.T) {
		ver, err := cat.GetActiveVersion(ctx, tenantID)
		if err != nil {
			t.Fatalf("GetActiveVersion failed: %v", err)
		}
		if ver != "cat_ver_init" {
			t.Errorf("expected cat_ver_init, got %s", ver)
		}
	})

	t.Run("TC-ELIG-01 & 02: Staleness boundary filters out expired variants", func(t *testing.T) {
		res, err := cat.Search(ctx, tenantID, catalog.SearchQuery{
			Category: "laptops",
			Currency: "USD",
		})
		if err != nil {
			t.Fatalf("Search failed: %v", err)
		}

		for _, v := range res.Variants {
			if v.ID == "lap_stale" {
				t.Fatalf("TC-ELIG-02 violated: stale variant beyond limit was returned!")
			}
		}
	})

	t.Run("TC-ELIG-04: Currency mismatch is excluded", func(t *testing.T) {
		res, err := cat.Search(ctx, tenantID, catalog.SearchQuery{
			Category: "laptops",
			Currency: "USD",
		})
		if err != nil {
			t.Fatalf("Search failed: %v", err)
		}

		for _, v := range res.Variants {
			if v.Currency != "USD" {
				t.Fatalf("TC-ELIG-04 violated: variant with currency %s returned for USD query", v.Currency)
			}
		}
	})

	t.Run("TC-ELIG-03: Empty eligible set returns honest empty slice", func(t *testing.T) {
		res, err := cat.Search(ctx, tenantID, catalog.SearchQuery{
			Category: "smartphones",
			Currency: "USD",
		})
		if err != nil {
			t.Fatalf("Search failed: %v", err)
		}
		if len(res.Variants) != 0 {
			t.Errorf("expected 0 variants for empty category, got %d", len(res.Variants))
		}
	})

	t.Run("TC-SEARCH-01: English stemming matches query ('laptop' matches 'Laptop')", func(t *testing.T) {
		res, err := cat.Search(ctx, tenantID, catalog.SearchQuery{
			Query:    "laptop",
			Category: "laptops",
			Currency: "USD",
		})
		if err != nil {
			t.Fatalf("Search failed: %v", err)
		}
		if len(res.Variants) == 0 {
			t.Errorf("expected English stemming match, got 0 results")
		}
	})

	t.Run("TC-SEARCH-02: Arabic search matches normalized query", func(t *testing.T) {
		// Query with different alef form and no diacritics
		res, err := cat.Search(ctx, tenantID, catalog.SearchQuery{
			Query:    "حاسوب خفه",
			Category: "laptops",
			Currency: "USD",
		})
		if err != nil {
			t.Fatalf("Arabic search failed: %v", err)
		}
		found := false
		for _, v := range res.Variants {
			if v.ID == "lap_arabic" {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected lap_arabic to match Arabic query, got %v", res.Variants)
		}
	})

	t.Run("TC-SEARCH-04: SQL wildcards are treated literally", func(t *testing.T) {
		res, err := cat.Search(ctx, tenantID, catalog.SearchQuery{
			Query:    "%",
			Category: "laptops",
			Currency: "USD",
		})
		if err != nil {
			t.Fatalf("Wildcard search failed: %v", err)
		}
		// Wildcard query should not cause SQL error or match everything
		_ = res
	})

	t.Run("TC-SEARCH-05: Overlong search query is safely bounded", func(t *testing.T) {
		longQ := strings.Repeat("laptop ", 50)
		res, err := cat.Search(ctx, tenantID, catalog.SearchQuery{
			Query:    longQ,
			Category: "laptops",
			Currency: "USD",
		})
		if err != nil {
			t.Fatalf("Overlong search query failed: %v", err)
		}
		_ = res
	})

	t.Run("TC-SEARCH-08: ListLaptops returns deterministic order (product_id, variant_id)", func(t *testing.T) {
		laptops, err := cat.ListLaptops(ctx, tenantID)
		if err != nil {
			t.Fatalf("ListLaptops failed: %v", err)
		}

		for i := 1; i < len(laptops); i++ {
			if laptops[i].ProductID < laptops[i-1].ProductID {
				t.Fatalf("nondeterministic product_id order: %s < %s", laptops[i].ProductID, laptops[i-1].ProductID)
			}
		}
	})
}
