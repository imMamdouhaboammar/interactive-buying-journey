// Copyright (c) 2026 Mamdouh Aboammar
// SPDX-License-Identifier: PolyForm-Shield-1.0.0

package ingest_test

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/imMamdouhaboammar/interactive-buying-journey/internal/ingest"
	"github.com/imMamdouhaboammar/interactive-buying-journey/internal/storage/postgres"
	"pgregory.net/rapid"
)

func TestProperty_BatchPermutationConvergence(t *testing.T) {
	dsn := os.Getenv("IBJ_TEST_DATABASE_URL")
	if dsn == "" {
		dsn = "postgres://localhost:5432/postgres?sslmode=disable"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	db, err := postgres.New(ctx, dsn)
	if err != nil {
		if os.Getenv("IBJ_REQUIRE_DB") == "1" {
			t.Fatalf("failed to connect to db: %v", err)
		}
		t.Skipf("skipping property test (no db): %v", err)
	}
	defer db.Close()

	if err := postgres.RunMigrations(dsn); err != nil {
		t.Fatalf("failed migrations: %v", err)
	}

	parser, err := ingest.NewBatchParser(testSchemaDir)
	if err != nil {
		t.Fatalf("failed parser: %v", err)
	}

	clock := fixedClock{now: time.Date(2026, 9, 27, 10, 0, 0, 0, time.UTC)}
	svc := ingest.NewService(db, parser, clock)

	rapid.Check(t, func(rt *rapid.T) {
		tenantA := fmt.Sprintf("rapid_a_%d", rapid.Int64().Draw(rt, "id_a"))
		tenantB := fmt.Sprintf("rapid_b_%d", rapid.Int64().Draw(rt, "id_b"))

		// Seed tenants
		_, _ = db.Pool().Exec(context.Background(), `
			INSERT INTO tenants (tenant_id, name, secret_current)
			VALUES ($1, 'Tenant A', 'secret'), ($2, 'Tenant B', 'secret')
			ON CONFLICT (tenant_id) DO NOTHING
		`, tenantA, tenantB)

		defer func() {
			_, _ = db.Pool().Exec(context.Background(), "DELETE FROM variants WHERE tenant_id IN ($1, $2)", tenantA, tenantB)
			_, _ = db.Pool().Exec(context.Background(), "DELETE FROM feed_batches WHERE tenant_id IN ($1, $2)", tenantA, tenantB)
			_, _ = db.Pool().Exec(context.Background(), "DELETE FROM catalog_versions WHERE tenant_id IN ($1, $2)", tenantA, tenantB)
			_, _ = db.Pool().Exec(context.Background(), "DELETE FROM tenants WHERE tenant_id IN ($1, $2)", tenantA, tenantB)
		}()

		// Define two batches with ordered timestamps
		b1A := []byte(fmt.Sprintf(`{"batch_id":"b1","tenant_id":"%s","source":"m","source_version":"v1","upserts":[{"product_id":"p","variant_id":"v1","sku":"S1","title":"Ver 1","published":true,"currency":"USD","price_minor":100,"inventory_status":"in_stock","source_updated_at":"2026-09-27T08:00:00Z"}],"deletes":[]}`, tenantA))
		b2A := []byte(fmt.Sprintf(`{"batch_id":"b2","tenant_id":"%s","source":"m","source_version":"v1","upserts":[{"product_id":"p","variant_id":"v1","sku":"S1","title":"Ver 2","published":true,"currency":"USD","price_minor":200,"inventory_status":"in_stock","source_updated_at":"2026-09-27T09:00:00Z"}],"deletes":[]}`, tenantA))

		b1B := []byte(fmt.Sprintf(`{"batch_id":"b1","tenant_id":"%s","source":"m","source_version":"v1","upserts":[{"product_id":"p","variant_id":"v1","sku":"S1","title":"Ver 1","published":true,"currency":"USD","price_minor":100,"inventory_status":"in_stock","source_updated_at":"2026-09-27T08:00:00Z"}],"deletes":[]}`, tenantB))
		b2B := []byte(fmt.Sprintf(`{"batch_id":"b2","tenant_id":"%s","source":"m","source_version":"v1","upserts":[{"product_id":"p","variant_id":"v1","sku":"S1","title":"Ver 2","published":true,"currency":"USD","price_minor":200,"inventory_status":"in_stock","source_updated_at":"2026-09-27T09:00:00Z"}],"deletes":[]}`, tenantB))

		// Apply in order b1 then b2 for Tenant A
		_, _ = svc.ReceiveBatch(context.Background(), tenantA, "m", "v1", b1A)
		_, _ = svc.ProcessBatch(context.Background(), tenantA, "b1")
		_, _ = svc.ReceiveBatch(context.Background(), tenantA, "m", "v1", b2A)
		_, _ = svc.ProcessBatch(context.Background(), tenantA, "b2")

		// Apply in REVERSE order b2 then b1 for Tenant B
		_, _ = svc.ReceiveBatch(context.Background(), tenantB, "m", "v1", b2B)
		_, _ = svc.ProcessBatch(context.Background(), tenantB, "b2")
		_, _ = svc.ReceiveBatch(context.Background(), tenantB, "m", "v1", b1B)
		_, _ = svc.ProcessBatch(context.Background(), tenantB, "b1")

		// Query projection states
		var titleA, titleB string
		var priceA, priceB int64
		_ = db.WithTenantTx(context.Background(), tenantA, func(tx pgx.Tx) error {
			return tx.QueryRow(context.Background(), "SELECT title, price_minor FROM variants WHERE variant_id = 'v1'").Scan(&titleA, &priceA)
		})
		_ = db.WithTenantTx(context.Background(), tenantB, func(tx pgx.Tx) error {
			return tx.QueryRow(context.Background(), "SELECT title, price_minor FROM variants WHERE variant_id = 'v1'").Scan(&titleB, &priceB)
		})

		// Both must converge to the latest update (Ver 2, 200)
		if titleA != titleB || priceA != priceB {
			rt.Fatalf("permutation divergence: TenantA=(%s, %d) vs TenantB=(%s, %d)", titleA, priceA, titleB, priceB)
		}
		if titleA != "Ver 2" || priceA != 200 {
			rt.Fatalf("expected both to converge to Ver 2 / 200, got: %s / %d", titleA, priceA)
		}
	})
}
