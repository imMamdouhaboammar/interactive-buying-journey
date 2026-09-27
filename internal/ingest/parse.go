// Copyright (c) 2026 Mamdouh Aboammar
// SPDX-License-Identifier: PolyForm-Shield-1.0.0

package ingest

import (
	"errors"
	"time"

	"github.com/imMamdouhaboammar/interactive-buying-journey/internal/contracts"
)

const MaxPayloadBytes = 1 << 20 // 1MB

var (
	ErrPayloadTooLarge       = errors.New("payload exceeds 1MB limit")
	ErrUnsupportedMediaType  = errors.New("content type must be application/json")
	ErrMalformedJSON         = errors.New("malformed json payload")
	ErrDuplicateJSONKeys     = errors.New("duplicate json keys detected")
	ErrSchemaValidation      = errors.New("schema validation failed")
	ErrInvalidAttributeValue = errors.New("invalid attribute value")
	ErrDuplicateVariant      = errors.New("duplicate variant in batch")
	ErrFutureTimestampSkew   = errors.New("source timestamp skewed into future")
)

// BatchUpsert represents an item to be upserted in the catalog.
type BatchUpsert struct {
	ProductID       string         `json:"product_id"`
	VariantID       string         `json:"variant_id"`
	SKU             string         `json:"sku"`
	Title           string         `json:"title"`
	Category        *string        `json:"category,omitempty"`
	Brand           *string        `json:"brand,omitempty"`
	Published       bool           `json:"published"`
	Currency        string         `json:"currency"`
	PriceMinor      int64          `json:"price_minor"`
	InventoryStatus string         `json:"inventory_status"`
	Attributes      map[string]any `json:"attributes,omitempty"`
	SourceUpdatedAt time.Time      `json:"source_updated_at"`
}

// CatalogBatch represents a validated batch payload.
type CatalogBatch struct {
	BatchID       string        `json:"batch_id"`
	TenantID      string        `json:"tenant_id"`
	Source        string        `json:"source"`
	SourceVersion string        `json:"source_version"`
	Upserts       []BatchUpsert `json:"upserts"`
	Deletes       []string      `json:"deletes"`
}

// BatchParser parses and validates incoming catalog batch payloads.
type BatchParser struct {
	validator *contracts.Validator
}

// NewBatchParser constructs a BatchParser with schema validator loaded from schemaDir.
func NewBatchParser(schemaDir string) (*BatchParser, error) {
	val, err := contracts.NewValidator(schemaDir)
	if err != nil {
		return nil, err
	}
	return &BatchParser{validator: val}, nil
}

// ValidateTransport checks media type and body size.
func ValidateTransport(contentType string, body []byte) error {
	return errors.New("not implemented")
}

// CheckDuplicateJSONKeys returns ErrDuplicateJSONKeys if body contains duplicate keys at any nesting level.
func CheckDuplicateJSONKeys(body []byte) error {
	return errors.New("not implemented")
}

// ParseAndValidate parses the JSON body, runs schema and semantic validations, and returns CatalogBatch.
func (p *BatchParser) ParseAndValidate(contentType string, body []byte, now time.Time) (*CatalogBatch, error) {
	return nil, errors.New("not implemented")
}
