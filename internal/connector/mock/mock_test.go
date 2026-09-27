package mock_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/imMamdouhaboammar/interactive-buying-journey/internal/connector/mock"
	"github.com/imMamdouhaboammar/interactive-buying-journey/internal/contracts"
	"github.com/imMamdouhaboammar/interactive-buying-journey/internal/ingest"
)

func TestGenerateBatch_ValidStructure(t *testing.T) {
	tenantID := "demo_store"
	count := 50
	batchID := "batch_gen_001"
	sourceVer := "v1.0"

	payload, err := mock.GenerateBatch(tenantID, count, batchID, sourceVer)
	if err != nil {
		t.Fatalf("unexpected error generating batch: %v", err)
	}

	var parsed ingest.CatalogBatch
	if err := json.Unmarshal(payload, &parsed); err != nil {
		t.Fatalf("failed to unmarshal generated batch: %v", err)
	}

	if parsed.BatchID != batchID {
		t.Errorf("expected batchID %q, got %q", batchID, parsed.BatchID)
	}
	if parsed.TenantID != tenantID {
		t.Errorf("expected tenantID %q, got %q", tenantID, parsed.TenantID)
	}
	if len(parsed.Upserts) != count {
		t.Fatalf("expected %d upserts, got %d", count, len(parsed.Upserts))
	}

	// Verify schema validation on the generated payload
	validator, err := contracts.NewValidator("../../../contracts/schemas")
	if err != nil {
		t.Fatalf("failed to create validator: %v", err)
	}
	if err := validator.Validate("catalog-batch.schema.json", payload); err != nil {
		t.Fatalf("generated payload failed schema validation: %v", err)
	}
}

func TestGenerateBatches_MultiBatch10k(t *testing.T) {
	tenantID := "demo_store"
	totalCount := 2500
	batchSize := 1000
	sourceVer := "v1.0"

	batches, err := mock.GenerateBatches(tenantID, totalCount, batchSize, sourceVer)
	if err != nil {
		t.Fatalf("unexpected error generating batches: %v", err)
	}

	if len(batches) != 3 {
		t.Fatalf("expected 3 batches for 2500 items, got %d", len(batches))
	}

	seenIDs := make(map[string]bool)
	for i, b := range batches {
		var parsed ingest.CatalogBatch
		if err := json.Unmarshal(b, &parsed); err != nil {
			t.Fatalf("batch %d failed to unmarshal: %v", i, err)
		}
		for _, u := range parsed.Upserts {
			if seenIDs[u.VariantID] {
				t.Fatalf("duplicate variant_id detected across batches: %s", u.VariantID)
			}
			seenIDs[u.VariantID] = true
		}
	}

	if len(seenIDs) != totalCount {
		t.Fatalf("expected %d unique variants, got %d", totalCount, len(seenIDs))
	}
}

func TestDispatcher_DispatchBatch(t *testing.T) {
	tenantID := "demo_store"
	secret := "test_secret_123"

	var receivedHeaders http.Header
	var receivedBody []byte

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedHeaders = r.Header.Clone()
		var err error
		receivedBody = make([]byte, r.ContentLength)
		_, err = r.Body.Read(receivedBody)
		if err != nil && err.Error() != "EOF" {
			t.Errorf("read body error: %v", err)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte(`{"batch_id":"b1","tenant_id":"demo_store","state":"received"}`))
	}))
	defer srv.Close()

	dispatcher := mock.NewDispatcher(srv.URL+"/catalog/batches", tenantID, secret, srv.Client())

	payload, err := mock.GenerateBatch(tenantID, 5, "batch_test", "v1.0")
	if err != nil {
		t.Fatalf("generate batch failed: %v", err)
	}

	receipt, err := dispatcher.DispatchBatch(context.Background(), payload)
	if err != nil {
		t.Fatalf("dispatch batch failed: %v", err)
	}

	if receipt.StatusCode != http.StatusAccepted {
		t.Errorf("expected 202 Accepted, got %d", receipt.StatusCode)
	}

	// Verify signature on received body
	sigHeader := receivedHeaders.Get("X-IBJ-Signature")
	tsHeader := receivedHeaders.Get("X-IBJ-Timestamp")
	tenantHeader := receivedHeaders.Get("X-IBJ-Tenant")

	if tenantHeader != tenantID {
		t.Errorf("expected tenant header %q, got %q", tenantID, tenantHeader)
	}

	err = ingest.VerifyHeaders(tenantHeader, tsHeader, sigHeader, tenantID, secret, "", time.Now(), 300*time.Second, receivedBody)
	if err != nil {
		t.Errorf("VerifyHeaders failed on dispatched request: %v", err)
	}
}
