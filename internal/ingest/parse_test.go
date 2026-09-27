// Copyright (c) 2026 Mamdouh Aboammar
// SPDX-License-Identifier: PolyForm-Shield-1.0.0

package ingest_test

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/imMamdouhaboammar/interactive-buying-journey/internal/ingest"
)

const testSchemaDir = "../../contracts/schemas"

func validBatchJSON() string {
	return `{
		"batch_id": "batch_001",
		"tenant_id": "demo_store",
		"source": "manual",
		"source_version": "v1.0.0",
		"upserts": [
			{
				"product_id": "prod_1",
				"variant_id": "var_1",
				"sku": "SKU-001",
				"title": "Pro Laptop 14",
				"category": "laptops",
				"published": true,
				"currency": "USD",
				"price_minor": 129900,
				"inventory_status": "in_stock",
				"attributes": {
					"weight_g": 1350,
					"battery_wh": 65.5,
					"usb_c_pd": true
				},
				"source_updated_at": "2026-09-27T10:00:00Z"
			}
		],
		"deletes": []
	}`
}

func TestBatchParser_Transport(t *testing.T) {
	parser, err := ingest.NewBatchParser(testSchemaDir)
	if err != nil {
		t.Fatalf("failed to create parser: %v", err)
	}
	fixedNow := time.Date(2026, 9, 27, 10, 0, 0, 0, time.UTC)

	t.Run("TC-TRANS-01: body exceeds 1MB ceiling rejected", func(t *testing.T) {
		oversized := make([]byte, ingest.MaxPayloadBytes+1)
		err := ingest.ValidateTransport("application/json", oversized)
		if !errors.Is(err, ingest.ErrPayloadTooLarge) {
			t.Errorf("expected ErrPayloadTooLarge, got: %v", err)
		}
	})

	t.Run("TC-TRANS-02: empty body payload rejected", func(t *testing.T) {
		_, err := parser.ParseAndValidate("application/json", []byte(""), fixedNow)
		if !errors.Is(err, ingest.ErrMalformedJSON) {
			t.Errorf("expected ErrMalformedJSON, got: %v", err)
		}
	})

	t.Run("TC-TRANS-03: non-JSON text body rejected", func(t *testing.T) {
		_, err := parser.ParseAndValidate("application/json", []byte("not a json payload"), fixedNow)
		if !errors.Is(err, ingest.ErrMalformedJSON) {
			t.Errorf("expected ErrMalformedJSON, got: %v", err)
		}
	})

	t.Run("TC-TRANS-04: content-type text/plain rejected", func(t *testing.T) {
		err := ingest.ValidateTransport("text/plain", []byte(validBatchJSON()))
		if !errors.Is(err, ingest.ErrUnsupportedMediaType) {
			t.Errorf("expected ErrUnsupportedMediaType, got: %v", err)
		}
	})

	t.Run("content-type application/json with charset accepted", func(t *testing.T) {
		err := ingest.ValidateTransport("application/json; charset=utf-8", []byte(validBatchJSON()))
		if err != nil {
			t.Errorf("expected nil error, got: %v", err)
		}
	})

	t.Run("TC-TRANS-05: duplicate keys in JSON payload rejected", func(t *testing.T) {
		dupJSON := []byte(`{
			"batch_id": "batch_001",
			"tenant_id": "demo_store",
			"batch_id": "batch_002",
			"source": "manual",
			"source_version": "v1.0.0",
			"upserts": [],
			"deletes": []
		}`)
		err := ingest.CheckDuplicateJSONKeys(dupJSON)
		if !errors.Is(err, ingest.ErrDuplicateJSONKeys) {
			t.Errorf("expected ErrDuplicateJSONKeys, got: %v", err)
		}
	})

	t.Run("TC-TRANS-06: unknown fields rejected by additionalProperties: false", func(t *testing.T) {
		unknownFieldJSON := strings.Replace(validBatchJSON(), `"deletes": []`, `"deletes": [], "unexpected_field": 123`, 1)
		_, err := parser.ParseAndValidate("application/json", []byte(unknownFieldJSON), fixedNow)
		if !errors.Is(err, ingest.ErrSchemaValidation) {
			t.Errorf("expected ErrSchemaValidation, got: %v", err)
		}
	})
}

func TestBatchParser_Validation(t *testing.T) {
	parser, err := ingest.NewBatchParser(testSchemaDir)
	if err != nil {
		t.Fatalf("failed to create parser: %v", err)
	}
	fixedNow := time.Date(2026, 9, 27, 10, 0, 0, 0, time.UTC)

	t.Run("valid batch passes parsing", func(t *testing.T) {
		batch, err := parser.ParseAndValidate("application/json", []byte(validBatchJSON()), fixedNow)
		if err != nil {
			t.Fatalf("expected valid batch to pass, got: %v", err)
		}
		if batch.BatchID != "batch_001" || len(batch.Upserts) != 1 {
			t.Errorf("unexpected batch content: %+v", batch)
		}
	})

	t.Run("TC-VAL-01: lowercase currency rejected", func(t *testing.T) {
		badJSON := strings.Replace(validBatchJSON(), `"currency": "USD"`, `"currency": "usd"`, 1)
		_, err := parser.ParseAndValidate("application/json", []byte(badJSON), fixedNow)
		if !errors.Is(err, ingest.ErrSchemaValidation) {
			t.Errorf("expected ErrSchemaValidation for lowercase currency, got: %v", err)
		}
	})

	t.Run("TC-VAL-02: negative price rejected", func(t *testing.T) {
		badJSON := strings.Replace(validBatchJSON(), `"price_minor": 129900`, `"price_minor": -1`, 1)
		_, err := parser.ParseAndValidate("application/json", []byte(badJSON), fixedNow)
		if !errors.Is(err, ingest.ErrSchemaValidation) {
			t.Errorf("expected ErrSchemaValidation for negative price, got: %v", err)
		}
	})

	t.Run("TC-VAL-03: fractional price rejected", func(t *testing.T) {
		badJSON := strings.Replace(validBatchJSON(), `"price_minor": 129900`, `"price_minor": 109.99`, 1)
		_, err := parser.ParseAndValidate("application/json", []byte(badJSON), fixedNow)
		if !errors.Is(err, ingest.ErrSchemaValidation) {
			t.Errorf("expected ErrSchemaValidation for fractional price, got: %v", err)
		}
	})

	t.Run("TC-VAL-04: title > 250 chars rejected", func(t *testing.T) {
		longTitle := strings.Repeat("A", 251)
		badJSON := strings.Replace(validBatchJSON(), `"title": "Pro Laptop 14"`, fmt.Sprintf(`"title": %q`, longTitle), 1)
		_, err := parser.ParseAndValidate("application/json", []byte(badJSON), fixedNow)
		if !errors.Is(err, ingest.ErrSchemaValidation) {
			t.Errorf("expected ErrSchemaValidation for title > 250 chars, got: %v", err)
		}
	})

	t.Run("TC-VAL-05: empty string variant_id rejected", func(t *testing.T) {
		badJSON := strings.Replace(validBatchJSON(), `"variant_id": "var_1"`, `"variant_id": "   "`, 1)
		_, err := parser.ParseAndValidate("application/json", []byte(badJSON), fixedNow)
		if !errors.Is(err, ingest.ErrInvalidAttributeValue) {
			t.Errorf("expected ErrInvalidAttributeValue for blank variant_id, got: %v", err)
		}
	})

	t.Run("TC-VAL-06: 1001 upserts rejected", func(t *testing.T) {
		var upserts []string
		for i := 0; i < 1001; i++ {
			upserts = append(upserts, fmt.Sprintf(`{
				"product_id": "p_%d", "variant_id": "v_%d", "sku": "S-%d", "title": "Lap",
				"published": true, "currency": "USD", "price_minor": 100, "inventory_status": "in_stock",
				"source_updated_at": "2026-09-27T10:00:00Z"
			}`, i, i, i))
		}
		raw := fmt.Sprintf(`{"batch_id":"b1","tenant_id":"t1","source":"m","source_version":"v1","upserts":[%s],"deletes":[]}`, strings.Join(upserts, ","))
		_, err := parser.ParseAndValidate("application/json", []byte(raw), fixedNow)
		if !errors.Is(err, ingest.ErrSchemaValidation) {
			t.Errorf("expected ErrSchemaValidation for 1001 upserts, got: %v", err)
		}
	})

	t.Run("TC-VAL-07: 1001 deletes rejected", func(t *testing.T) {
		var dels []string
		for i := 0; i < 1001; i++ {
			dels = append(dels, fmt.Sprintf(`"v_%d"`, i))
		}
		raw := fmt.Sprintf(`{"batch_id":"b1","tenant_id":"t1","source":"m","source_version":"v1","upserts":[],"deletes":[%s]}`, strings.Join(dels, ","))
		_, err := parser.ParseAndValidate("application/json", []byte(raw), fixedNow)
		if !errors.Is(err, ingest.ErrSchemaValidation) {
			t.Errorf("expected ErrSchemaValidation for 1001 deletes, got: %v", err)
		}
	})

	t.Run("TC-VAL-08: wrong attribute type (weight_g string) rejected", func(t *testing.T) {
		badJSON := strings.Replace(validBatchJSON(), `"weight_g": 1350`, `"weight_g": "1200g"`, 1)
		_, err := parser.ParseAndValidate("application/json", []byte(badJSON), fixedNow)
		if !errors.Is(err, ingest.ErrInvalidAttributeValue) {
			t.Errorf("expected ErrInvalidAttributeValue for string weight_g, got: %v", err)
		}
	})

	t.Run("TC-VAL-09: zero weight rejected", func(t *testing.T) {
		badJSON := strings.Replace(validBatchJSON(), `"weight_g": 1350`, `"weight_g": 0`, 1)
		_, err := parser.ParseAndValidate("application/json", []byte(badJSON), fixedNow)
		if !errors.Is(err, ingest.ErrInvalidAttributeValue) {
			t.Errorf("expected ErrInvalidAttributeValue for zero weight, got: %v", err)
		}
	})

	t.Run("TC-VAL-10: string boolean (usb_c_pd = 'true') rejected", func(t *testing.T) {
		badJSON := strings.Replace(validBatchJSON(), `"usb_c_pd": true`, `"usb_c_pd": "true"`, 1)
		_, err := parser.ParseAndValidate("application/json", []byte(badJSON), fixedNow)
		if !errors.Is(err, ingest.ErrInvalidAttributeValue) {
			t.Errorf("expected ErrInvalidAttributeValue for string bool, got: %v", err)
		}
	})

	t.Run("TC-VAL-14: duplicate variant in upserts rejected", func(t *testing.T) {
		dupVariantJSON := `{
			"batch_id": "batch_001",
			"tenant_id": "demo_store",
			"source": "manual",
			"source_version": "v1.0.0",
			"upserts": [
				{
					"product_id": "prod_1", "variant_id": "var_1", "sku": "SKU-001", "title": "A",
					"published": true, "currency": "USD", "price_minor": 100, "inventory_status": "in_stock",
					"source_updated_at": "2026-09-27T10:00:00Z"
				},
				{
					"product_id": "prod_2", "variant_id": "var_1", "sku": "SKU-002", "title": "B",
					"published": true, "currency": "USD", "price_minor": 200, "inventory_status": "in_stock",
					"source_updated_at": "2026-09-27T10:00:00Z"
				}
			],
			"deletes": []
		}`
		_, err := parser.ParseAndValidate("application/json", []byte(dupVariantJSON), fixedNow)
		if !errors.Is(err, ingest.ErrDuplicateVariant) {
			t.Errorf("expected ErrDuplicateVariant, got: %v", err)
		}
	})

	t.Run("TC-VAL-14b: variant in both upserts and deletes rejected", func(t *testing.T) {
		conflictJSON := strings.Replace(validBatchJSON(), `"deletes": []`, `"deletes": ["var_1"]`, 1)
		_, err := parser.ParseAndValidate("application/json", []byte(conflictJSON), fixedNow)
		if !errors.Is(err, ingest.ErrDuplicateVariant) {
			t.Errorf("expected ErrDuplicateVariant for item in upserts and deletes, got: %v", err)
		}
	})

	t.Run("future timestamp skew > 300s rejected", func(t *testing.T) {
		futureTime := fixedNow.Add(301 * time.Second).Format(time.RFC3339)
		badJSON := strings.Replace(validBatchJSON(), `"source_updated_at": "2026-09-27T10:00:00Z"`, fmt.Sprintf(`"source_updated_at": %q`, futureTime), 1)
		_, err := parser.ParseAndValidate("application/json", []byte(badJSON), fixedNow)
		if !errors.Is(err, ingest.ErrFutureTimestampSkew) {
			t.Errorf("expected ErrFutureTimestampSkew for future timestamp, got: %v", err)
		}
	})
}
