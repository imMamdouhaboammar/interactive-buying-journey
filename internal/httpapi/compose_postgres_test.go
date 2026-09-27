// Copyright (c) 2026 Mamdouh Aboammar
// SPDX-License-Identifier: PolyForm-Shield-1.0.0

package httpapi_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/imMamdouhaboammar/interactive-buying-journey/internal/catalog"
	"github.com/imMamdouhaboammar/interactive-buying-journey/internal/compose"
	"github.com/imMamdouhaboammar/interactive-buying-journey/internal/contracts"
	"github.com/imMamdouhaboammar/interactive-buying-journey/internal/httpapi"
	"github.com/imMamdouhaboammar/interactive-buying-journey/internal/ingest"
	"github.com/imMamdouhaboammar/interactive-buying-journey/internal/policy"
	"github.com/imMamdouhaboammar/interactive-buying-journey/internal/storage/postgres"
)

func TestCompose_WithPostgresCatalog(t *testing.T) {
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
		t.Skipf("skipping compose postgres test (no db): %v", err)
	}
	defer db.Close()

	if err := postgres.RunMigrations(dsn); err != nil {
		t.Fatalf("failed to run migrations: %v", err)
	}

	tenantID := fmt.Sprintf("tenant_comp_%d", time.Now().UnixNano())
	keyRef := "ref_comp_123"
	secret := "test_secret_123"

	// Seed tenant
	_, err = db.Pool().Exec(context.Background(), `
		INSERT INTO tenants (tenant_id, name, secret_key_ref, max_staleness_seconds)
		VALUES ($1, 'Compose Tenant', $2, 86400)
		ON CONFLICT (tenant_id) DO NOTHING
	`, tenantID, keyRef)
	if err != nil {
		t.Fatalf("failed to seed tenant: %v", err)
	}

	defer func() {
		_ = db.WithTenantTx(context.Background(), tenantID, func(tx pgx.Tx) error {
			_, _ = tx.Exec(context.Background(), "DELETE FROM catalog_versions WHERE tenant_id = $1", tenantID)
			_, _ = tx.Exec(context.Background(), "DELETE FROM variants WHERE tenant_id = $1", tenantID)
			_, _ = tx.Exec(context.Background(), "DELETE FROM feed_batches WHERE tenant_id = $1", tenantID)
			_, _ = tx.Exec(context.Background(), "DELETE FROM tenants WHERE tenant_id = $1", tenantID)
			return nil
		})
	}()

	cat := catalog.NewPostgresCatalog(db)
	pol := policy.NewPolicyChecker([]string{tenantID}, []string{"collection_top", "collection_grid", "collection_comparison", "collection_filters", "pdp_related", "cart_accessory"})
	val, err := contracts.NewValidator("../../contracts/schemas")
	if err != nil {
		t.Fatalf("failed to create validator: %v", err)
	}

	parser, err := ingest.NewBatchParser("../../contracts/schemas")
	if err != nil {
		t.Fatalf("failed to create parser: %v", err)
	}

	svc := ingest.NewService(db, parser, nil)
	composer := compose.NewComposer(cat, pol, val)
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))

	secretLookup := func(ctx context.Context, tID string) (string, string, error) {
		if tID == tenantID {
			return secret, "", nil
		}
		return "", "", errors.New("unknown tenant")
	}

	handler := httpapi.NewHandler(composer, val, logger, httpapi.WithIngest(svc, secretLookup, nil))

	validExample, err := os.ReadFile("../../contracts/examples/compose-request.json")
	if err != nil {
		t.Fatalf("failed reading compose-request.json: %v", err)
	}

	var composeMap map[string]any
	if err := json.Unmarshal(validExample, &composeMap); err != nil {
		t.Fatalf("failed unmarshaling composeMap: %v", err)
	}
	composeMap["tenant_id"] = tenantID
	composePayload, _ := json.Marshal(composeMap)

	t.Run("TC-COMP-03: compose before active catalog version falls back gracefully", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/v1/journeys/compose", bytes.NewReader(composePayload))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 OK fallback, got %d: %s", rec.Code, rec.Body.String())
		}

		var plan contracts.ExperiencePlan
		if err := json.Unmarshal(rec.Body.Bytes(), &plan); err != nil {
			t.Fatalf("failed to decode plan: %v", err)
		}
		if plan.Status != "baseline" {
			t.Errorf("expected baseline status, got %s", plan.Status)
		}
		if plan.CatalogVersion != "unavailable" {
			t.Errorf("expected catalog_version 'unavailable', got %s", plan.CatalogVersion)
		}
		if plan.Provenance.FallbackReason == nil || *plan.Provenance.FallbackReason != "catalog_unavailable" {
			t.Errorf("expected fallback_reason 'catalog_unavailable', got %v", plan.Provenance.FallbackReason)
		}
	})

	t.Run("TC-COMP-01: after batch ingest, compose reflects new active version", func(t *testing.T) {
		now := time.Now()
		batchID := "batch_comp_v1"
		batchBody := []byte(fmt.Sprintf(`{
			"batch_id": "%s",
			"tenant_id": "%s",
			"source": "manual",
			"source_version": "v1.0.0",
			"upserts": [
				{
					"product_id": "p_comp_1", "variant_id": "v_comp_1", "sku": "SKU-C1", "title": "Compose Laptop 14",
					"published": true, "currency": "USD", "price_minor": 99900, "inventory_status": "in_stock",
					"source_updated_at": %q
				}
			],
			"deletes": []
		}`, batchID, tenantID, now.Format(time.RFC3339)))
		sig := ingest.SignPayload(secret, now.Unix(), batchBody)

		// 1. Submit batch via POST /catalog/batches
		req := httptest.NewRequest(http.MethodPost, "/catalog/batches", bytes.NewReader(batchBody))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-IBJ-Tenant", tenantID)
		req.Header.Set("X-IBJ-Timestamp", fmt.Sprintf("%d", now.Unix()))
		req.Header.Set("X-IBJ-Signature", sig)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusAccepted {
			t.Fatalf("batch submit failed: %d: %s", rec.Code, rec.Body.String())
		}

		// 2. Synchronize batch application
		summary, err := svc.ProcessBatch(context.Background(), tenantID, batchID)
		if err != nil {
			t.Fatalf("process batch failed: %v", err)
		}
		expectedVer := summary.ActiveVersionID

		// 3. Compose again
		req = httptest.NewRequest(http.MethodPost, "/v1/journeys/compose", bytes.NewReader(composePayload))
		req.Header.Set("Content-Type", "application/json")
		rec = httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("compose failed: %d: %s", rec.Code, rec.Body.String())
		}

		var plan contracts.ExperiencePlan
		if err := json.Unmarshal(rec.Body.Bytes(), &plan); err != nil {
			t.Fatalf("failed to decode plan: %v", err)
		}
		if plan.CatalogVersion != expectedVer {
			t.Errorf("expected catalog_version %q, got %q", expectedVer, plan.CatalogVersion)
		}
	})

	t.Run("TC-COMP-02: quarantined batch preserves prior active catalog version", func(t *testing.T) {
		// Active version before invalid batch
		priorVer, err := cat.GetActiveVersion(context.Background(), tenantID)
		if err != nil {
			t.Fatalf("get prior version failed: %v", err)
		}

		// Submit invalid batch with negative price
		badBatchID := "batch_comp_bad"
		now := time.Now()
		badBody := []byte(fmt.Sprintf(`{
			"batch_id": "%s",
			"tenant_id": "%s",
			"source": "manual",
			"source_version": "v1.0.0",
			"upserts": [
				{
					"product_id": "p_b", "variant_id": "v_b", "sku": "SKU-B", "title": "Bad Laptop",
					"published": true, "currency": "USD", "price_minor": -100, "inventory_status": "in_stock",
					"source_updated_at": %q
				}
			],
			"deletes": []
		}`, badBatchID, tenantID, now.Format(time.RFC3339)))
		sig := ingest.SignPayload(secret, now.Unix(), badBody)

		req := httptest.NewRequest(http.MethodPost, "/catalog/batches", bytes.NewReader(badBody))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-IBJ-Tenant", tenantID)
		req.Header.Set("X-IBJ-Timestamp", fmt.Sprintf("%d", now.Unix()))
		req.Header.Set("X-IBJ-Signature", sig)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusAccepted {
			t.Fatalf("bad batch submit failed: %d: %s", rec.Code, rec.Body.String())
		}

		// Process batch -> quarantined
		_, err = svc.ProcessBatch(context.Background(), tenantID, badBatchID)
		if !errors.Is(err, ingest.ErrBatchQuarantined) {
			t.Fatalf("expected ErrBatchQuarantined, got %v", err)
		}

		// Compose should still reflect priorVer!
		req = httptest.NewRequest(http.MethodPost, "/v1/journeys/compose", bytes.NewReader(composePayload))
		req.Header.Set("Content-Type", "application/json")
		rec = httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("compose failed: %d: %s", rec.Code, rec.Body.String())
		}

		var plan contracts.ExperiencePlan
		if err := json.Unmarshal(rec.Body.Bytes(), &plan); err != nil {
			t.Fatalf("failed to decode plan: %v", err)
		}
		if plan.CatalogVersion != priorVer {
			t.Errorf("TC-COMP-02 violated: catalog_version changed after quarantine! got %q, want %q", plan.CatalogVersion, priorVer)
		}
	})
}
