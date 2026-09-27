// Copyright (c) 2026 Mamdouh Aboammar
// SPDX-License-Identifier: PolyForm-Shield-1.0.0

package e2e_test

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
	"github.com/imMamdouhaboammar/interactive-buying-journey/internal/connector/mock"
	"github.com/imMamdouhaboammar/interactive-buying-journey/internal/contracts"
	"github.com/imMamdouhaboammar/interactive-buying-journey/internal/httpapi"
	"github.com/imMamdouhaboammar/interactive-buying-journey/internal/ingest"
	"github.com/imMamdouhaboammar/interactive-buying-journey/internal/policy"
	"github.com/imMamdouhaboammar/interactive-buying-journey/internal/secret"
	"github.com/imMamdouhaboammar/interactive-buying-journey/internal/storage/postgres"
)

func TestE2E_CatalogIngest_Lifecycle(t *testing.T) {
	dsn := os.Getenv("IBJ_TEST_DATABASE_URL")
	if dsn == "" {
		dsn = "postgres://localhost:5432/postgres?sslmode=disable"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	db, err := postgres.New(ctx, dsn)
	if err != nil {
		if os.Getenv("IBJ_REQUIRE_DB") == "1" {
			t.Fatalf("failed to connect to postgres: %v", err)
		}
		t.Skipf("skipping e2e test (no db): %v", err)
	}
	defer db.Close()

	if err := postgres.RunMigrations(dsn); err != nil {
		t.Fatalf("failed to run migrations: %v", err)
	}

	tenantA := "demo_store"
	secretA := "test_secret_123"

	tenantB := "demo_store_b"
	secretB := "test_secret_b_456"

	keyRefA := "REF_DEMO_STORE_A"
	keyRefB := "REF_DEMO_STORE_B"
	t.Setenv(keyRefA, secretA)
	t.Setenv(keyRefB, secretB)

	// Seed tenants
	_, err = db.Pool().Exec(ctx, `
		INSERT INTO tenants (tenant_id, name, secret_key_ref, max_staleness_seconds)
		VALUES 
			($1, 'Demo Store A', $2, 86400),
			($3, 'Demo Store B', $4, 86400)
		ON CONFLICT (tenant_id) DO UPDATE SET 
			secret_key_ref = EXCLUDED.secret_key_ref,
			max_staleness_seconds = EXCLUDED.max_staleness_seconds
	`, tenantA, keyRefA, tenantB, keyRefB)
	if err != nil {
		t.Fatalf("failed to seed tenants: %v", err)
	}

	defer func() {
		for _, tid := range []string{tenantA, tenantB} {
			_ = db.WithTenantTx(context.Background(), tid, func(tx pgx.Tx) error {
				_, _ = tx.Exec(context.Background(), "DELETE FROM catalog_versions WHERE tenant_id = $1", tid)
				_, _ = tx.Exec(context.Background(), "DELETE FROM variants WHERE tenant_id = $1", tid)
				_, _ = tx.Exec(context.Background(), "DELETE FROM feed_batches WHERE tenant_id = $1", tid)
				return nil
			})
		}
	}()

	cat := catalog.NewPostgresCatalog(db)
	pol := policy.NewPolicyChecker([]string{tenantA, tenantB}, []string{
		"collection_top", "collection_comparison", "collection_filters", "collection_grid", "pdp_related", "cart_accessory",
	})
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

	secProvider := secret.NewEnvSecretProvider("")
	secretLookup := func(ctx context.Context, tID string) (string, string, error) {
		var keyRef string
		err := db.Pool().QueryRow(ctx, "SELECT secret_key_ref FROM tenants WHERE tenant_id = $1", tID).Scan(&keyRef)
		if err != nil {
			return "", "", fmt.Errorf("unknown tenant: %s", tID)
		}
		sec, err := secProvider.GetSecret(ctx, keyRef)
		if err != nil {
			return "", "", err
		}
		return sec, "", nil
	}

	handler := httpapi.NewHandler(composer, val, logger, httpapi.WithIngest(svc, secretLookup, nil))
	server := httptest.NewServer(handler)
	defer server.Close()

	// -------------------------------------------------------------
	// Step 1: Ingest demo_store_batch_v1.json (12 variants)
	// -------------------------------------------------------------
	v1Bytes, err := os.ReadFile("../../testdata/feeds/demo_store_batch_v1.json")
	if err != nil {
		t.Fatalf("failed to read v1 fixture: %v", err)
	}

	dispatcherA := mock.NewDispatcher(server.URL+"/catalog/batches", tenantA, secretA, server.Client())
	receipt1, err := dispatcherA.DispatchBatch(context.Background(), v1Bytes)
	if err != nil {
		t.Fatalf("failed to dispatch v1: %v", err)
	}
	if receipt1.StatusCode != http.StatusAccepted {
		t.Fatalf("expected 202 Accepted, got %d: %s", receipt1.StatusCode, string(receipt1.RawBody))
	}

	// Synchronously process batch to ensure indexing is complete
	procBatch1, err := svc.ProcessBatch(context.Background(), tenantA, "batch_demo_001")
	if err != nil {
		t.Fatalf("failed to process v1 batch: %v", err)
	}
	if procBatch1.State != "ACTIVE" {
		t.Fatalf("expected v1 state ACTIVE, got %s", procBatch1.State)
	}

	// Verify Compose endpoint reflects newly ingested catalog version dynamically
	composeReqA := []byte(fmt.Sprintf(`{
		"contract_version": "1.0",
		"request_id": "req_e2e_001",
		"tenant_id": "%s",
		"session_token": "sess_001",
		"page": { "kind": "collection", "category_id": "laptops" },
		"locale": "en",
		"currency": "USD",
		"consent": { "version": "v1", "purposes": ["necessary"] },
		"preferences": { "purpose": "portable work", "max_budget_minor": 250000, "max_weight_grams": 2500 },
		"allowed_slots": ["collection_top", "collection_comparison", "collection_filters"],
		"page_events": []
	}`, tenantA))

	req, _ := http.NewRequest(http.MethodPost, server.URL+"/v1/journeys/compose", bytes.NewReader(composeReqA))
	req.Header.Set("Content-Type", "application/json")
	resp, err := server.Client().Do(req)
	if err != nil {
		t.Fatalf("failed to call compose: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK from compose, got %d", resp.StatusCode)
	}

	var plan1 contracts.ExperiencePlan
	if err := json.NewDecoder(resp.Body).Decode(&plan1); err != nil {
		t.Fatalf("failed to decode compose response: %v", err)
	}

	if plan1.CatalogVersion != procBatch1.ActiveVersionID {
		t.Errorf("expected dynamic catalog version %q, got %q", procBatch1.ActiveVersionID, plan1.CatalogVersion)
	}

	// Verify Candidate Retrieval via CatalogPort & SearchPort
	laptops1, err := cat.ListLaptops(context.Background(), tenantA)
	if err != nil {
		t.Fatalf("failed to list laptops: %v", err)
	}

	foundV1002 := false
	for _, l := range laptops1 {
		if l.ID == "v1-002" {
			foundV1002 = true
			if l.PriceMinor != 99900 {
				t.Errorf("expected price 99900 for v1-002, got %d", l.PriceMinor)
			}
		}
	}
	if !foundV1002 {
		t.Errorf("expected to find variant v1-002 in laptops list, got %v", laptops1)
	}

	v1002, err := cat.GetVariant(context.Background(), tenantA, "v1-002")
	if err != nil {
		t.Fatalf("failed to get variant v1-002: %v", err)
	}
	if v1002.Title != "Apex Slim 13 / أيبكس سليم ١٣" {
		t.Errorf("unexpected title for v1-002: %s", v1002.Title)
	}

	// -------------------------------------------------------------
	// Step 2: Ingest demo_store_batch_v2.json (Updates, new variants, delete v1-002)
	// -------------------------------------------------------------
	v2Bytes, err := os.ReadFile("../../testdata/feeds/demo_store_batch_v2.json")
	if err != nil {
		t.Fatalf("failed to read v2 fixture: %v", err)
	}

	receipt2, err := dispatcherA.DispatchBatch(context.Background(), v2Bytes)
	if err != nil {
		t.Fatalf("failed to dispatch v2: %v", err)
	}
	if receipt2.StatusCode != http.StatusAccepted {
		t.Fatalf("expected 202 Accepted for v2, got %d: %s", receipt2.StatusCode, string(receipt2.RawBody))
	}

	procBatch2, err := svc.ProcessBatch(context.Background(), tenantA, "batch_demo_002")
	if err != nil {
		t.Fatalf("failed to process v2 batch: %v", err)
	}
	if procBatch2.State != "ACTIVE" {
		t.Fatalf("expected v2 state ACTIVE, got %s", procBatch2.State)
	}

	// Verify Compose endpoint reflects new active version
	reqV2, _ := http.NewRequest(http.MethodPost, server.URL+"/v1/journeys/compose", bytes.NewReader(composeReqA))
	reqV2.Header.Set("Content-Type", "application/json")
	respV2, err := server.Client().Do(reqV2)
	if err != nil {
		t.Fatalf("failed to call compose after v2: %v", err)
	}
	defer func() { _ = respV2.Body.Close() }()

	var plan2 contracts.ExperiencePlan
	if err := json.NewDecoder(respV2.Body).Decode(&plan2); err != nil {
		t.Fatalf("failed to decode compose response v2: %v", err)
	}

	if plan2.CatalogVersion != procBatch2.ActiveVersionID {
		t.Errorf("expected plan2 catalog version %q, got %q", procBatch2.ActiveVersionID, plan2.CatalogVersion)
	}

	// Verify updated variant price
	v1001, err := cat.GetVariant(context.Background(), tenantA, "v1-001")
	if err != nil {
		t.Fatalf("failed to get variant v1-001: %v", err)
	}
	if v1001.PriceMinor != 119900 {
		t.Errorf("expected updated price 119900 for v1-001, got %d", v1001.PriceMinor)
	}

	// Verify deleted variant v1-002 no longer retrievable
	_, err = cat.GetVariant(context.Background(), tenantA, "v1-002")
	if !errors.Is(err, catalog.ErrVariantNotFound) {
		t.Errorf("expected ErrVariantNotFound for tombstoned variant v1-002, got %v", err)
	}

	// Verify newly added variant v2-001
	v2001, err := cat.GetVariant(context.Background(), tenantA, "v2-001")
	if err != nil {
		t.Fatalf("failed to get newly added variant v2-001: %v", err)
	}
	if v2001.PriceMinor != 139900 {
		t.Errorf("expected price 139900 for v2-001, got %d", v2001.PriceMinor)
	}

	// Verify restocked variant v1-003 is now in ListLaptops
	laptops2, err := cat.ListLaptops(context.Background(), tenantA)
	if err != nil {
		t.Fatalf("failed to list laptops after v2: %v", err)
	}

	foundV1003Restocked := false
	for _, l := range laptops2 {
		if l.ID == "v1-002" {
			t.Fatalf("tombstoned variant v1-002 still returned in ListLaptops")
		}
		if l.ID == "v1-003" {
			foundV1003Restocked = true
		}
	}
	if !foundV1003Restocked {
		t.Errorf("expected restocked variant v1-003 in ListLaptops")
	}

	// -------------------------------------------------------------
	// Step 3: Ingest demo_store_b_batch_v1.json (Tenant B isolation)
	// -------------------------------------------------------------
	vBBytes, err := os.ReadFile("../../testdata/feeds/demo_store_b_batch_v1.json")
	if err != nil {
		t.Fatalf("failed to read tenant B fixture: %v", err)
	}

	dispatcherB := mock.NewDispatcher(server.URL+"/catalog/batches", tenantB, secretB, server.Client())
	receiptB, err := dispatcherB.DispatchBatch(context.Background(), vBBytes)
	if err != nil {
		t.Fatalf("failed to dispatch tenant B batch: %v", err)
	}
	if receiptB.StatusCode != http.StatusAccepted {
		t.Fatalf("expected 202 Accepted for tenant B, got %d", receiptB.StatusCode)
	}

	procBatchB, err := svc.ProcessBatch(context.Background(), tenantB, "batch_b_001")
	if err != nil {
		t.Fatalf("failed to process tenant B batch: %v", err)
	}
	if procBatchB.State != "ACTIVE" {
		t.Fatalf("expected tenant B state ACTIVE, got %s", procBatchB.State)
	}

	// Verify Tenant A compose does NOT see Tenant B catalog version
	reqAAfterB, _ := http.NewRequest(http.MethodPost, server.URL+"/v1/journeys/compose", bytes.NewReader(composeReqA))
	reqAAfterB.Header.Set("Content-Type", "application/json")
	respAAfterB, err := server.Client().Do(reqAAfterB)
	if err != nil {
		t.Fatalf("failed to call compose tenant A: %v", err)
	}
	defer func() { _ = respAAfterB.Body.Close() }()

	var planAAfterB contracts.ExperiencePlan
	_ = json.NewDecoder(respAAfterB.Body).Decode(&planAAfterB)
	if planAAfterB.CatalogVersion != procBatch2.ActiveVersionID {
		t.Errorf("expected tenant A to remain on version %s, got %s", procBatch2.ActiveVersionID, planAAfterB.CatalogVersion)
	}

	// Verify Tenant A CatalogPort NEVER returns Tenant B items
	laptopsA, err := cat.ListLaptops(context.Background(), tenantA)
	if err != nil {
		t.Fatalf("failed to list tenant A laptops: %v", err)
	}
	for _, l := range laptopsA {
		if l.ID == "vb-001" || l.ID == "vb-002" {
			t.Fatalf("CROSS-TENANT LEAK: tenant A saw tenant B variant: %v", l.ID)
		}
	}

	// Verify Tenant B compose reflects tenant B active version
	composeReqB := []byte(fmt.Sprintf(`{
		"contract_version": "1.0",
		"request_id": "req_e2e_002",
		"tenant_id": "%s",
		"session_token": "sess_002",
		"page": { "kind": "collection", "category_id": "laptops" },
		"locale": "ar",
		"currency": "SAR",
		"consent": { "version": "v1", "purposes": ["necessary"] },
		"preferences": { "purpose": "portable work", "max_budget_minor": 800000, "max_weight_grams": 2500 },
		"allowed_slots": ["collection_top", "collection_comparison", "collection_filters"],
		"page_events": []
	}`, tenantB))

	reqB, _ := http.NewRequest(http.MethodPost, server.URL+"/v1/journeys/compose", bytes.NewReader(composeReqB))
	reqB.Header.Set("Content-Type", "application/json")
	respB, err := server.Client().Do(reqB)
	if err != nil {
		t.Fatalf("failed to call compose tenant B: %v", err)
	}
	defer func() { _ = respB.Body.Close() }()

	if respB.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK from tenant B compose, got %d", respB.StatusCode)
	}

	var planB contracts.ExperiencePlan
	_ = json.NewDecoder(respB.Body).Decode(&planB)
	if planB.CatalogVersion != procBatchB.ActiveVersionID {
		t.Errorf("expected tenant B plan catalog version %q, got %q", procBatchB.ActiveVersionID, planB.CatalogVersion)
	}

	// Verify Tenant B CatalogPort sees its own products in SAR
	laptopsB, err := cat.ListLaptops(context.Background(), tenantB)
	if err != nil {
		t.Fatalf("failed to list tenant B laptops: %v", err)
	}

	foundVB001 := false
	for _, l := range laptopsB {
		if l.ID == "vb-001" {
			foundVB001 = true
			if l.PriceMinor != 450000 || l.Currency != "SAR" {
				t.Errorf("expected 450000 SAR for vb-001, got %d %s", l.PriceMinor, l.Currency)
			}
		}
		if l.ID == "v1-001" {
			t.Fatalf("CROSS-TENANT LEAK: tenant B saw tenant A variant: %v", l.ID)
		}
	}
	if !foundVB001 {
		t.Errorf("expected tenant B to see vb-001 in ListLaptops")
	}

	// -------------------------------------------------------------
	// Step 4: Ingest demo_store_invalid_batch.json (Quarantined)
	// -------------------------------------------------------------
	vInvBytes, err := os.ReadFile("../../testdata/feeds/demo_store_invalid_batch.json")
	if err != nil {
		t.Fatalf("failed to read invalid fixture: %v", err)
	}

	receiptInv, err := dispatcherA.DispatchBatch(context.Background(), vInvBytes)
	if err != nil {
		t.Fatalf("failed to dispatch invalid batch: %v", err)
	}
	if receiptInv.StatusCode != http.StatusAccepted {
		t.Fatalf("expected 202 Accepted for invalid batch receipt, got %d", receiptInv.StatusCode)
	}

	_, err = svc.ProcessBatch(context.Background(), tenantA, "batch_invalid_001")
	if !errors.Is(err, ingest.ErrBatchQuarantined) {
		t.Fatalf("expected ErrBatchQuarantined, got %v", err)
	}

	var storedState string
	err = db.Pool().QueryRow(context.Background(), `
		SELECT state FROM feed_batches WHERE tenant_id = $1 AND batch_id = $2
	`, tenantA, "batch_invalid_001").Scan(&storedState)
	if err != nil {
		t.Fatalf("failed to query stored batch state: %v", err)
	}
	if storedState != "QUARANTINED" {
		t.Fatalf("expected stored batch state QUARANTINED, got %s", storedState)
	}

	// Verify Tenant A compose still serves the active catalog from v2 with 0 disruption
	reqAfterInv, _ := http.NewRequest(http.MethodPost, server.URL+"/v1/journeys/compose", bytes.NewReader(composeReqA))
	reqAfterInv.Header.Set("Content-Type", "application/json")
	respAfterInv, err := server.Client().Do(reqAfterInv)
	if err != nil {
		t.Fatalf("failed to call compose after quarantine: %v", err)
	}
	defer func() { _ = respAfterInv.Body.Close() }()

	if respAfterInv.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK after quarantine, got %d", respAfterInv.StatusCode)
	}

	var planAfterInv contracts.ExperiencePlan
	_ = json.NewDecoder(respAfterInv.Body).Decode(&planAfterInv)
	if planAfterInv.CatalogVersion != procBatch2.ActiveVersionID {
		t.Errorf("expected active version %s to remain unchanged after quarantine, got %s",
			procBatch2.ActiveVersionID, planAfterInv.CatalogVersion)
	}
}
