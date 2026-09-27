// Copyright (c) 2026 Mamdouh Aboammar
// SPDX-License-Identifier: PolyForm-Shield-1.0.0

package httpapi_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/imMamdouhaboammar/interactive-buying-journey/internal/catalog"
	"github.com/imMamdouhaboammar/interactive-buying-journey/internal/compose"
	"github.com/imMamdouhaboammar/interactive-buying-journey/internal/contracts"
	"github.com/imMamdouhaboammar/interactive-buying-journey/internal/httpapi"
	"github.com/imMamdouhaboammar/interactive-buying-journey/internal/ingest"
	"github.com/imMamdouhaboammar/interactive-buying-journey/internal/policy"
)

func setupTestServer(t *testing.T) (http.Handler, *policy.PolicyChecker) {
	cat := catalog.NewInMemoryCatalog()
	pol := policy.NewPolicyChecker([]string{"demo_store"}, []string{"collection_top", "collection_grid", "collection_comparison", "collection_filters", "pdp_related", "cart_accessory"})
	val, err := contracts.NewValidator("../../contracts/schemas")
	if err != nil {
		t.Fatalf("failed to create validator: %v", err)
	}
	composer := compose.NewComposer(cat, pol, val)
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	handler := httpapi.NewHandler(composer, val, logger)
	return handler, pol
}

func TestHealthzAndReadyz(t *testing.T) {
	handler, _ := setupTestServer(t)

	t.Run("GET /healthz returns 200 ok", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", rec.Code)
		}
		var res map[string]string
		if err := json.Unmarshal(rec.Body.Bytes(), &res); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if res["status"] != "ok" {
			t.Errorf("expected status 'ok', got %q", res["status"])
		}
	})

	t.Run("GET /readyz returns 200 ready", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", rec.Code)
		}
		var res map[string]string
		if err := json.Unmarshal(rec.Body.Bytes(), &res); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if res["status"] != "ready" {
			t.Errorf("expected status 'ready', got %q", res["status"])
		}
	})
}

func TestComposeEndpoint(t *testing.T) {
	handler, pol := setupTestServer(t)

	validPayload, err := os.ReadFile("../../contracts/examples/compose-request.json")
	if err != nil {
		t.Fatalf("failed to read compose-request.json: %v", err)
	}

	t.Run("valid compose request returns 200 with baseline plan", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/v1/journeys/compose", bytes.NewReader(validPayload))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
		}

		var plan contracts.ExperiencePlan
		if err := json.Unmarshal(rec.Body.Bytes(), &plan); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if plan.Status != "baseline" {
			t.Errorf("expected status 'baseline', got %q", plan.Status)
		}
		if plan.Provenance.Strategy != "merchant_baseline" {
			t.Errorf("expected strategy 'merchant_baseline', got %q", plan.Provenance.Strategy)
		}
		if len(plan.Sections) != 0 {
			t.Errorf("expected 0 sections, got %d", len(plan.Sections))
		}
	})

	t.Run("malformed json returns 400 bad request", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/v1/journeys/compose", bytes.NewReader([]byte("{malformed")))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("expected 400 for malformed json, got %d", rec.Code)
		}
	})

	t.Run("schema-invalid request returns 400", func(t *testing.T) {
		// Missing required tenant_id
		invalidReq := map[string]any{
			"contract_version": "1.0",
			"request_id":       "req_test_invalid",
		}
		data, _ := json.Marshal(invalidReq)
		req := httptest.NewRequest(http.MethodPost, "/v1/journeys/compose", bytes.NewReader(data))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("expected 400 for schema invalid request, got %d", rec.Code)
		}
	})

	t.Run("unknown tenant returns 403 forbidden", func(t *testing.T) {
		var reqMap map[string]any
		if err := json.Unmarshal(validPayload, &reqMap); err != nil {
			t.Fatalf("failed to decode validPayload: %v", err)
		}
		reqMap["tenant_id"] = "unknown_tenant_xyz"
		data, _ := json.Marshal(reqMap)

		req := httptest.NewRequest(http.MethodPost, "/v1/journeys/compose", bytes.NewReader(data))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusForbidden {
			t.Errorf("expected 403 for unknown tenant, got %d: %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("disallowed slot returns 400 bad request", func(t *testing.T) {
		var reqMap map[string]any
		if err := json.Unmarshal(validPayload, &reqMap); err != nil {
			t.Fatalf("failed to decode validPayload: %v", err)
		}
		reqMap["allowed_slots"] = []any{"disallowed_slot"}
		data, _ := json.Marshal(reqMap)

		req := httptest.NewRequest(http.MethodPost, "/v1/journeys/compose", bytes.NewReader(data))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("expected 400 for disallowed slot, got %d: %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("kill switch returns baseline immediately", func(t *testing.T) {
		pol.SetAdaptationEnabled(false)
		defer pol.SetAdaptationEnabled(true)

		req := httptest.NewRequest(http.MethodPost, "/v1/journeys/compose", bytes.NewReader(validPayload))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
		}
		var plan contracts.ExperiencePlan
		if err := json.Unmarshal(rec.Body.Bytes(), &plan); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if plan.Status != "baseline" {
			t.Errorf("expected status 'baseline', got %q", plan.Status)
		}
	})
}

func TestCatalogBatchesEndpoint(t *testing.T) {
	handler, _ := setupTestServer(t)

	tenantID := "demo_store"
	secret := "test_secret_123"
	fixedNow := time.Date(2026, 9, 27, 10, 0, 0, 0, time.UTC)
	tsStr := fmt.Sprintf("%d", fixedNow.Unix())
	body := []byte(fmt.Sprintf(`{"batch_id":"b1","tenant_id":"%s","source":"manual","source_version":"v1","upserts":[],"deletes":[]}`, tenantID))
	sig := ingest.SignPayload(secret, fixedNow.Unix(), body)

	t.Run("TC-AUTH-01: missing X-IBJ-Tenant header returns 401", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/catalog/batches", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-IBJ-Timestamp", tsStr)
		req.Header.Set("X-IBJ-Signature", sig)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Errorf("expected 401 for missing tenant header, got %d: %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("TC-AUTH-02: missing X-IBJ-Timestamp header returns 401", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/catalog/batches", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-IBJ-Tenant", tenantID)
		req.Header.Set("X-IBJ-Signature", sig)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Errorf("expected 401 for missing timestamp header, got %d: %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("TC-AUTH-03: missing X-IBJ-Signature header returns 401", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/catalog/batches", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-IBJ-Tenant", tenantID)
		req.Header.Set("X-IBJ-Timestamp", tsStr)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Errorf("expected 401 for missing signature header, got %d: %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("TC-TRANS-04: non-json content-type returns 415", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/catalog/batches", bytes.NewReader(body))
		req.Header.Set("Content-Type", "text/plain")
		req.Header.Set("X-IBJ-Tenant", tenantID)
		req.Header.Set("X-IBJ-Timestamp", tsStr)
		req.Header.Set("X-IBJ-Signature", sig)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnsupportedMediaType {
			t.Errorf("expected 415 for text/plain, got %d: %s", rec.Code, rec.Body.String())
		}
	})
}
