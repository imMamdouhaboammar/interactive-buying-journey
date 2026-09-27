// Copyright (c) 2026 Mamdouh Aboammar
// SPDX-License-Identifier: PolyForm-Shield-1.0.0

package ingest_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/imMamdouhaboammar/interactive-buying-journey/internal/ingest"
	"github.com/imMamdouhaboammar/interactive-buying-journey/internal/storage/postgres"
)

type fixedClock struct {
	now time.Time
}

func (c fixedClock) Now() time.Time {
	return c.now
}

func setupServiceTest(t *testing.T) (*postgres.DB, *ingest.Service, string, func()) {
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
		t.Skipf("skipping ingest service test (no db): %v", err)
	}

	if err := postgres.RunMigrations(dsn); err != nil {
		db.Close()
		t.Fatalf("failed to run migrations: %v", err)
	}

	parser, err := ingest.NewBatchParser(testSchemaDir)
	if err != nil {
		db.Close()
		t.Fatalf("failed to create parser: %v", err)
	}

	tenantID := fmt.Sprintf("tenant_%d", time.Now().UnixNano())
	_, err = db.Pool().Exec(context.Background(), `
		INSERT INTO tenants (tenant_id, name, secret_current)
		VALUES ($1, 'Test Tenant', 'test_secret')
		ON CONFLICT (tenant_id) DO NOTHING
	`, tenantID)
	if err != nil {
		db.Close()
		t.Fatalf("failed to seed tenant: %v", err)
	}

	clock := fixedClock{now: time.Date(2026, 9, 27, 10, 0, 0, 0, time.UTC)}
	svc := ingest.NewService(db, parser, clock)

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

	return db, svc, tenantID, cleanup
}

func makeBatchPayload(tenantID, batchID string, upsertVariants []string, deletes []string) []byte {
	var upsertItems []string
	for _, v := range upsertVariants {
		upsertItems = append(upsertItems, fmt.Sprintf(`{
			"product_id": "p_%s",
			"variant_id": "%s",
			"sku": "SKU-%s",
			"title": "Title for %s",
			"category": "laptops",
			"published": true,
			"currency": "USD",
			"price_minor": 100000,
			"inventory_status": "in_stock",
			"source_updated_at": "2026-09-27T10:00:00Z"
		}`, v, v, v, v))
	}

	var delItems []string
	for _, d := range deletes {
		delItems = append(delItems, fmt.Sprintf(`"%s"`, d))
	}

	return []byte(fmt.Sprintf(`{
		"batch_id": "%s",
		"tenant_id": "%s",
		"source": "manual",
		"source_version": "v1.0.0",
		"upserts": [%s],
		"deletes": [%s]
	}`, batchID, tenantID, strings.Join(upsertItems, ","), strings.Join(delItems, ",")))
}

func TestService_IngestDecisionTable(t *testing.T) {
	db, svc, tenantID, cleanup := setupServiceTest(t)
	defer cleanup()

	ctx := context.Background()

	t.Run("TC-ORD-01: same batch_id, identical payload hash is idempotent 202", func(t *testing.T) {
		body := makeBatchPayload(tenantID, "batch_idemp_1", []string{"lap_1"}, nil)
		rec1, err := svc.ReceiveBatch(ctx, tenantID, "manual", "v1.0.0", body)
		if err != nil {
			t.Fatalf("first receive failed: %v", err)
		}
		if rec1.IsDuplicate {
			t.Errorf("expected first receive not to be duplicate")
		}

		rec2, err := svc.ReceiveBatch(ctx, tenantID, "manual", "v1.0.0", body)
		if err != nil {
			t.Fatalf("second receive failed: %v", err)
		}
		if !rec2.IsDuplicate {
			t.Errorf("expected second receive to be marked duplicate")
		}
	})

	t.Run("TC-ORD-02: same batch_id, different payload hash returns 409 conflict", func(t *testing.T) {
		body1 := makeBatchPayload(tenantID, "batch_conflict_1", []string{"lap_1"}, nil)
		body2 := makeBatchPayload(tenantID, "batch_conflict_1", []string{"lap_2"}, nil)

		_, err := svc.ReceiveBatch(ctx, tenantID, "manual", "v1.0.0", body1)
		if err != nil {
			t.Fatalf("first receive failed: %v", err)
		}

		_, err = svc.ReceiveBatch(ctx, tenantID, "manual", "v1.0.0", body2)
		if !errors.Is(err, ingest.ErrBatchHashMismatch) {
			t.Errorf("expected ErrBatchHashMismatch, got: %v", err)
		}
	})

	t.Run("TC-ORD-03: single invalid item causes whole batch quarantine, projection intact", func(t *testing.T) {
		// Valid initial batch to set projection
		initBody := makeBatchPayload(tenantID, "batch_valid_init", []string{"lap_init"}, nil)
		_, err := svc.ReceiveBatch(ctx, tenantID, "manual", "v1.0.0", initBody)
		if err != nil {
			t.Fatalf("receive init failed: %v", err)
		}
		sumInit, err := svc.ProcessBatch(ctx, tenantID, "batch_valid_init")
		if err != nil {
			t.Fatalf("process init failed: %v", err)
		}
		activeVer := sumInit.ActiveVersionID

		// Corrupted batch: one item has negative price
		badBody := []byte(fmt.Sprintf(`{
			"batch_id": "batch_bad_item",
			"tenant_id": "%s",
			"source": "manual",
			"source_version": "v1.0.0",
			"upserts": [
				{
					"product_id": "p_ok", "variant_id": "v_ok", "sku": "S-OK", "title": "Good",
					"published": true, "currency": "USD", "price_minor": 100, "inventory_status": "in_stock",
					"source_updated_at": "2026-09-27T10:00:00Z"
				},
				{
					"product_id": "p_bad", "variant_id": "v_bad", "sku": "S-BAD", "title": "Bad",
					"published": true, "currency": "USD", "price_minor": -50, "inventory_status": "in_stock",
					"source_updated_at": "2026-09-27T10:00:00Z"
				}
			],
			"deletes": []
		}`, tenantID))

		_, err = svc.ReceiveBatch(ctx, tenantID, "manual", "v1.0.0", badBody)
		if err != nil {
			t.Fatalf("receive bad batch failed: %v", err)
		}

		_, err = svc.ProcessBatch(ctx, tenantID, "batch_bad_item")
		if !errors.Is(err, ingest.ErrBatchQuarantined) {
			t.Errorf("expected ErrBatchQuarantined, got: %v", err)
		}

		// Verify prior active version is intact
		var currentActive string
		_ = db.WithTenantTx(ctx, tenantID, func(tx pgx.Tx) error {
			return tx.QueryRow(ctx, "SELECT version_id FROM catalog_versions WHERE status = 'ACTIVE'").Scan(&currentActive)
		})
		if currentActive != activeVer {
			t.Errorf("active version changed unexpectedly: got %s, want %s", currentActive, activeVer)
		}
	})

	t.Run("TC-ORD-04: upsert older than stored source_updated_at is skipped as stale", func(t *testing.T) {
		vID := "var_stale_test"
		// 1. Ingest newer version
		newerBody := []byte(fmt.Sprintf(`{
			"batch_id": "batch_newer",
			"tenant_id": "%s",
			"source": "manual",
			"source_version": "v1",
			"upserts": [{
				"product_id": "p1", "variant_id": "%s", "sku": "S1", "title": "Newer Title",
				"published": true, "currency": "USD", "price_minor": 100, "inventory_status": "in_stock",
				"source_updated_at": "2026-09-27T10:00:00Z"
			}],
			"deletes": []
		}`, tenantID, vID))
		_, _ = svc.ReceiveBatch(ctx, tenantID, "manual", "v1", newerBody)
		_, err := svc.ProcessBatch(ctx, tenantID, "batch_newer")
		if err != nil {
			t.Fatalf("newer batch process failed: %v", err)
		}

		// 2. Ingest older version (source_updated_at is 1 hour earlier)
		olderBody := []byte(fmt.Sprintf(`{
			"batch_id": "batch_older",
			"tenant_id": "%s",
			"source": "manual",
			"source_version": "v1",
			"upserts": [{
				"product_id": "p1", "variant_id": "%s", "sku": "S1", "title": "Stale Older Title",
				"published": true, "currency": "USD", "price_minor": 100, "inventory_status": "in_stock",
				"source_updated_at": "2026-09-27T09:00:00Z"
			}],
			"deletes": []
		}`, tenantID, vID))
		_, _ = svc.ReceiveBatch(ctx, tenantID, "manual", "v1", olderBody)
		summary, err := svc.ProcessBatch(ctx, tenantID, "batch_older")
		if err != nil {
			t.Fatalf("older batch process failed: %v", err)
		}

		if summary.StatsStale != 1 {
			t.Errorf("expected StatsStale = 1, got %d", summary.StatsStale)
		}

		// Verify existing title was preserved
		var title string
		_ = db.WithTenantTx(ctx, tenantID, func(tx pgx.Tx) error {
			return tx.QueryRow(ctx, "SELECT title FROM variants WHERE variant_id = $1", vID).Scan(&title)
		})
		if title != "Newer Title" {
			t.Errorf("expected title 'Newer Title', got %q", title)
		}
	})

	t.Run("TC-ORD-05 & 06: equal timestamp conflict vs identical update", func(t *testing.T) {
		vID := "var_eq_ts_test"
		ts := "2026-09-27T10:00:00Z"

		// 1. Initial
		b1 := []byte(fmt.Sprintf(`{"batch_id":"b_eq_1","tenant_id":"%s","source":"m","source_version":"v1","upserts":[{"product_id":"p","variant_id":"%s","sku":"S","title":"Original","published":true,"currency":"USD","price_minor":100,"inventory_status":"in_stock","source_updated_at":"%s"}],"deletes":[]}`, tenantID, vID, ts))
		_, _ = svc.ReceiveBatch(ctx, tenantID, "m", "v1", b1)
		_, _ = svc.ProcessBatch(ctx, tenantID, "b_eq_1")

		// 2. Identical update (TC-ORD-05)
		b2 := []byte(fmt.Sprintf(`{"batch_id":"b_eq_2","tenant_id":"%s","source":"m","source_version":"v1","upserts":[{"product_id":"p","variant_id":"%s","sku":"S","title":"Original","published":true,"currency":"USD","price_minor":100,"inventory_status":"in_stock","source_updated_at":"%s"}],"deletes":[]}`, tenantID, vID, ts))
		_, _ = svc.ReceiveBatch(ctx, tenantID, "m", "v1", b2)
		s2, err := svc.ProcessBatch(ctx, tenantID, "b_eq_2")
		if err != nil {
			t.Fatalf("identical batch failed: %v", err)
		}
		if s2.StatsConflicts != 0 {
			t.Errorf("expected 0 conflicts for identical payload, got %d", s2.StatsConflicts)
		}

		// 3. Different content with same timestamp (TC-ORD-06)
		b3 := []byte(fmt.Sprintf(`{"batch_id":"b_eq_3","tenant_id":"%s","source":"m","source_version":"v1","upserts":[{"product_id":"p","variant_id":"%s","sku":"S","title":"Conflicting Different Title","published":true,"currency":"USD","price_minor":200,"inventory_status":"in_stock","source_updated_at":"%s"}],"deletes":[]}`, tenantID, vID, ts))
		_, _ = svc.ReceiveBatch(ctx, tenantID, "m", "v1", b3)
		s3, err := svc.ProcessBatch(ctx, tenantID, "b_eq_3")
		if err != nil {
			t.Fatalf("conflicting batch failed: %v", err)
		}
		if s3.StatsConflicts != 1 {
			t.Errorf("expected 1 conflict, got %d", s3.StatsConflicts)
		}

		// Original record retained
		var title string
		_ = db.WithTenantTx(ctx, tenantID, func(tx pgx.Tx) error {
			return tx.QueryRow(ctx, "SELECT title FROM variants WHERE variant_id = $1", vID).Scan(&title)
		})
		if title != "Original" {
			t.Errorf("expected original record kept, got %q", title)
		}
	})

	t.Run("TC-ORD-07 to 10: Delete, tombstone, stale older upsert, and resurrection", func(t *testing.T) {
		vID := "var_tombstone_cycle"
		// 1. Create variant at 10:00
		b1 := []byte(fmt.Sprintf(`{"batch_id":"b_tomb_1","tenant_id":"%s","source":"m","source_version":"v1","upserts":[{"product_id":"p","variant_id":"%s","sku":"S","title":"Live Variant","published":true,"currency":"USD","price_minor":100,"inventory_status":"in_stock","source_updated_at":"2026-09-27T10:00:00Z"}],"deletes":[]}`, tenantID, vID))
		_, _ = svc.ReceiveBatch(ctx, tenantID, "m", "v1", b1)
		_, _ = svc.ProcessBatch(ctx, tenantID, "b_tomb_1")

		// 2. Delete at 11:00 (TC-ORD-07)
		b2 := []byte(fmt.Sprintf(`{"batch_id":"b_tomb_2","tenant_id":"%s","source":"m","source_version":"v1","upserts":[{"product_id":"p2","variant_id":"v2","sku":"S2","title":"Other","published":true,"currency":"USD","price_minor":100,"inventory_status":"in_stock","source_updated_at":"2026-09-27T11:00:00Z"}],"deletes":["%s"]}`, tenantID, vID))
		_, _ = svc.ReceiveBatch(ctx, tenantID, "m", "v1", b2)
		s2, err := svc.ProcessBatch(ctx, tenantID, "b_tomb_2")
		if err != nil {
			t.Fatalf("delete batch failed: %v", err)
		}
		if s2.StatsTombstoned != 1 {
			t.Errorf("expected StatsTombstoned = 1, got %d", s2.StatsTombstoned)
		}

		var isTomb bool
		_ = db.WithTenantTx(ctx, tenantID, func(tx pgx.Tx) error {
			return tx.QueryRow(ctx, "SELECT is_tombstoned FROM variants WHERE variant_id = $1", vID).Scan(&isTomb)
		})
		if !isTomb {
			t.Errorf("expected variant to be tombstoned")
		}

		// 3. Late older upsert at 10:30 (TC-ORD-08) -> stays deleted
		b3 := []byte(fmt.Sprintf(`{"batch_id":"b_tomb_3","tenant_id":"%s","source":"m","source_version":"v1","upserts":[{"product_id":"p","variant_id":"%s","sku":"S","title":"Late Older Live","published":true,"currency":"USD","price_minor":100,"inventory_status":"in_stock","source_updated_at":"2026-09-27T10:30:00Z"}],"deletes":[]}`, tenantID, vID))
		_, _ = svc.ReceiveBatch(ctx, tenantID, "m", "v1", b3)
		s3, _ := svc.ProcessBatch(ctx, tenantID, "b_tomb_3")
		if s3.StatsStale != 1 {
			t.Errorf("expected StatsStale = 1 for upsert older than tombstone, got %d", s3.StatsStale)
		}

		_ = db.WithTenantTx(ctx, tenantID, func(tx pgx.Tx) error {
			return tx.QueryRow(ctx, "SELECT is_tombstoned FROM variants WHERE variant_id = $1", vID).Scan(&isTomb)
		})
		if !isTomb {
			t.Errorf("expected variant to remain tombstoned")
		}

		// 4. Newer upsert at 12:00 (TC-ORD-09) -> resurrected
		b4 := []byte(fmt.Sprintf(`{"batch_id":"b_tomb_4","tenant_id":"%s","source":"m","source_version":"v1","upserts":[{"product_id":"p","variant_id":"%s","sku":"S","title":"Resurrected Variant","published":true,"currency":"USD","price_minor":100,"inventory_status":"in_stock","source_updated_at":"2026-09-27T12:00:00Z"}],"deletes":[]}`, tenantID, vID))
		_, _ = svc.ReceiveBatch(ctx, tenantID, "m", "v1", b4)
		s4, _ := svc.ProcessBatch(ctx, tenantID, "b_tomb_4")
		if s4.StatsUpserted != 1 {
			t.Errorf("expected StatsUpserted = 1 for resurrected variant, got %d", s4.StatsUpserted)
		}

		_ = db.WithTenantTx(ctx, tenantID, func(tx pgx.Tx) error {
			return tx.QueryRow(ctx, "SELECT is_tombstoned FROM variants WHERE variant_id = $1", vID).Scan(&isTomb)
		})
		if isTomb {
			t.Errorf("expected variant to be active (not tombstoned)")
		}
	})

	t.Run("TC-ORD-11: Delete of unknown variant records stub tombstone", func(t *testing.T) {
		unknownID := "unknown_var_stub"
		b := []byte(fmt.Sprintf(`{"batch_id":"b_stub_1","tenant_id":"%s","source":"m","source_version":"v1","upserts":[],"deletes":["%s"]}`, tenantID, unknownID))
		_, _ = svc.ReceiveBatch(ctx, tenantID, "m", "v1", b)
		s, err := svc.ProcessBatch(ctx, tenantID, "b_stub_1")
		if err != nil {
			t.Fatalf("delete batch failed: %v", err)
		}
		if s.StatsTombstoned != 1 {
			t.Errorf("expected StatsTombstoned = 1, got %d", s.StatsTombstoned)
		}

		var isTomb bool
		_ = db.WithTenantTx(ctx, tenantID, func(tx pgx.Tx) error {
			return tx.QueryRow(ctx, "SELECT is_tombstoned FROM variants WHERE variant_id = $1", unknownID).Scan(&isTomb)
		})
		if !isTomb {
			t.Errorf("expected stub tombstone in projection")
		}
	})

	t.Run("TC-CONC-01: Concurrent batches for same tenant are serialized cleanly", func(t *testing.T) {
		var wg sync.WaitGroup
		errs := make(chan error, 2)

		for i := 1; i <= 2; i++ {
			bID := fmt.Sprintf("batch_conc_%d", i)
			vID := fmt.Sprintf("var_conc_%d", i)
			payload := makeBatchPayload(tenantID, bID, []string{vID}, nil)
			_, err := svc.ReceiveBatch(ctx, tenantID, "manual", "v1", payload)
			if err != nil {
				t.Fatalf("receive %s failed: %v", bID, err)
			}

			wg.Add(1)
			go func(batchID string) {
				defer wg.Done()
				_, pErr := svc.ProcessBatch(ctx, tenantID, batchID)
				if pErr != nil {
					errs <- pErr
				}
			}(bID)
		}

		wg.Wait()
		close(errs)

		for e := range errs {
			t.Errorf("concurrent batch failed: %v", e)
		}
	})
}
