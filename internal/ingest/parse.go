// Copyright (c) 2026 Mamdouh Aboammar
// SPDX-License-Identifier: PolyForm-Shield-1.0.0

package ingest

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
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
	if len(body) > MaxPayloadBytes {
		return ErrPayloadTooLarge
	}

	mediaType := strings.ToLower(strings.TrimSpace(strings.Split(contentType, ";")[0]))
	if mediaType != "application/json" {
		return ErrUnsupportedMediaType
	}

	return nil
}

// CheckDuplicateJSONKeys returns ErrDuplicateJSONKeys if body contains duplicate keys at any nesting level.
func CheckDuplicateJSONKeys(body []byte) error {
	dec := json.NewDecoder(bytes.NewReader(body))
	tok, err := dec.Token()
	if err != nil {
		if errors.Is(err, io.EOF) {
			return ErrMalformedJSON
		}
		return fmt.Errorf("%w: %v", ErrMalformedJSON, err)
	}

	if delim, ok := tok.(json.Delim); ok {
		if err := checkTokenValue(delim, dec); err != nil {
			return err
		}
	}

	// Ensure there are no unexpected trailing tokens
	if dec.More() {
		return ErrMalformedJSON
	}

	return nil
}

func checkTokenValue(tok any, dec *json.Decoder) error {
	delim, ok := tok.(json.Delim)
	if !ok {
		return nil
	}

	switch delim {
	case '{':
		keys := make(map[string]struct{})
		for dec.More() {
			kTok, err := dec.Token()
			if err != nil {
				return fmt.Errorf("%w: %v", ErrMalformedJSON, err)
			}
			key, ok := kTok.(string)
			if !ok {
				return ErrMalformedJSON
			}
			if _, exists := keys[key]; exists {
				return fmt.Errorf("%w: duplicate key %q", ErrDuplicateJSONKeys, key)
			}
			keys[key] = struct{}{}

			vTok, err := dec.Token()
			if err != nil {
				return fmt.Errorf("%w: %v", ErrMalformedJSON, err)
			}
			if err := checkTokenValue(vTok, dec); err != nil {
				return err
			}
		}
		// Consume closing '}'
		endTok, err := dec.Token()
		if err != nil || endTok != json.Delim('}') {
			return ErrMalformedJSON
		}

	case '[':
		for dec.More() {
			itemTok, err := dec.Token()
			if err != nil {
				return fmt.Errorf("%w: %v", ErrMalformedJSON, err)
			}
			if err := checkTokenValue(itemTok, dec); err != nil {
				return err
			}
		}
		// Consume closing ']'
		endTok, err := dec.Token()
		if err != nil || endTok != json.Delim(']') {
			return ErrMalformedJSON
		}
	}

	return nil
}

type rawUpsertItem struct {
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
	SourceUpdatedAt string         `json:"source_updated_at"`
}

type rawBatchPayload struct {
	BatchID       string          `json:"batch_id"`
	TenantID      string          `json:"tenant_id"`
	Source        string          `json:"source"`
	SourceVersion string          `json:"source_version"`
	Upserts       []rawUpsertItem `json:"upserts"`
	Deletes       []string        `json:"deletes"`
}

// ParseAndValidate parses the JSON body, runs schema and semantic validations, and returns CatalogBatch.
func (p *BatchParser) ParseAndValidate(contentType string, body []byte, now time.Time) (*CatalogBatch, error) {
	if err := ValidateTransport(contentType, body); err != nil {
		return nil, err
	}

	if len(bytes.TrimSpace(body)) == 0 {
		return nil, ErrMalformedJSON
	}

	if err := CheckDuplicateJSONKeys(body); err != nil {
		return nil, err
	}

	if err := p.validator.Validate("catalog-batch.schema.json", body); err != nil {
		var syntaxErr *json.SyntaxError
		if errors.As(err, &syntaxErr) || strings.Contains(err.Error(), "invalid json payload") {
			return nil, fmt.Errorf("%w: %v", ErrMalformedJSON, err)
		}
		return nil, fmt.Errorf("%w: %v", ErrSchemaValidation, err)
	}

	var raw rawBatchPayload
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.UseNumber()
	if err := dec.Decode(&raw); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrMalformedJSON, err)
	}

	seenUpserts := make(map[string]struct{}, len(raw.Upserts))
	upserts := make([]BatchUpsert, 0, len(raw.Upserts))

	for _, item := range raw.Upserts {
		if strings.TrimSpace(item.VariantID) == "" ||
			strings.TrimSpace(item.ProductID) == "" ||
			strings.TrimSpace(item.SKU) == "" ||
			strings.TrimSpace(item.Title) == "" {
			return nil, fmt.Errorf("%w: required string fields must not be blank", ErrInvalidAttributeValue)
		}

		if _, exists := seenUpserts[item.VariantID]; exists {
			return nil, fmt.Errorf("%w: variant_id %q duplicated in upserts", ErrDuplicateVariant, item.VariantID)
		}
		seenUpserts[item.VariantID] = struct{}{}

		tUpdated, err := time.Parse(time.RFC3339, item.SourceUpdatedAt)
		if err != nil {
			return nil, fmt.Errorf("%w: invalid source_updated_at RFC3339: %v", ErrInvalidAttributeValue, err)
		}

		if tUpdated.After(now.Add(300 * time.Second)) {
			return nil, fmt.Errorf("%w: source_updated_at %v is skewed > 300s in future", ErrFutureTimestampSkew, tUpdated)
		}

		if err := validateLaptopAttributes(item.Attributes); err != nil {
			return nil, err
		}

		// Unicode NFC normalization
		normalizedTitle := NormalizeNFC(item.Title)
		var catPtr, brandPtr *string
		if item.Category != nil {
			c := NormalizeNFC(*item.Category)
			catPtr = &c
		}
		if item.Brand != nil {
			b := NormalizeNFC(*item.Brand)
			brandPtr = &b
		}

		upserts = append(upserts, BatchUpsert{
			ProductID:       NormalizeNFC(item.ProductID),
			VariantID:       NormalizeNFC(item.VariantID),
			SKU:             NormalizeNFC(item.SKU),
			Title:           normalizedTitle,
			Category:        catPtr,
			Brand:           brandPtr,
			Published:       item.Published,
			Currency:        item.Currency,
			PriceMinor:      item.PriceMinor,
			InventoryStatus: item.InventoryStatus,
			Attributes:      item.Attributes,
			SourceUpdatedAt: tUpdated,
		})
	}

	seenDeletes := make(map[string]struct{}, len(raw.Deletes))
	deletes := make([]string, 0, len(raw.Deletes))
	for _, del := range raw.Deletes {
		normDel := NormalizeNFC(strings.TrimSpace(del))
		if normDel == "" {
			return nil, fmt.Errorf("%w: empty delete variant_id", ErrInvalidAttributeValue)
		}
		if _, exists := seenDeletes[normDel]; exists {
			return nil, fmt.Errorf("%w: variant_id %q duplicated in deletes", ErrDuplicateVariant, normDel)
		}
		if _, exists := seenUpserts[normDel]; exists {
			return nil, fmt.Errorf("%w: variant_id %q appears in both upserts and deletes", ErrDuplicateVariant, normDel)
		}
		seenDeletes[normDel] = struct{}{}
		deletes = append(deletes, normDel)
	}

	return &CatalogBatch{
		BatchID:       NormalizeNFC(raw.BatchID),
		TenantID:      NormalizeNFC(raw.TenantID),
		Source:        NormalizeNFC(raw.Source),
		SourceVersion: NormalizeNFC(raw.SourceVersion),
		Upserts:       upserts,
		Deletes:       deletes,
	}, nil
}

func validateLaptopAttributes(attrs map[string]any) error {
	if attrs == nil {
		return nil
	}

	if w, ok := attrs["weight_g"]; ok {
		switch v := w.(type) {
		case json.Number:
			intVal, err := v.Int64()
			if err != nil || intVal <= 0 {
				return fmt.Errorf("%w: weight_g must be positive integer", ErrInvalidAttributeValue)
			}
		case int:
			if v <= 0 {
				return fmt.Errorf("%w: weight_g must be positive integer", ErrInvalidAttributeValue)
			}
		case int64:
			if v <= 0 {
				return fmt.Errorf("%w: weight_g must be positive integer", ErrInvalidAttributeValue)
			}
		case float64:
			if v <= 0 || v != float64(int64(v)) {
				return fmt.Errorf("%w: weight_g must be positive integer", ErrInvalidAttributeValue)
			}
		default:
			return fmt.Errorf("%w: weight_g must be integer, got %T", ErrInvalidAttributeValue, w)
		}
	}

	if b, ok := attrs["battery_wh"]; ok {
		switch v := b.(type) {
		case json.Number:
			fltVal, err := v.Float64()
			if err != nil || fltVal <= 0 {
				return fmt.Errorf("%w: battery_wh must be positive number", ErrInvalidAttributeValue)
			}
		case float64:
			if v <= 0 {
				return fmt.Errorf("%w: battery_wh must be positive number", ErrInvalidAttributeValue)
			}
		case int:
			if v <= 0 {
				return fmt.Errorf("%w: battery_wh must be positive number", ErrInvalidAttributeValue)
			}
		case int64:
			if v <= 0 {
				return fmt.Errorf("%w: battery_wh must be positive number", ErrInvalidAttributeValue)
			}
		default:
			return fmt.Errorf("%w: battery_wh must be numeric, got %T", ErrInvalidAttributeValue, b)
		}
	}

	if u, ok := attrs["usb_c_pd"]; ok {
		if _, ok := u.(bool); !ok {
			return fmt.Errorf("%w: usb_c_pd must be boolean, got %T", ErrInvalidAttributeValue, u)
		}
	}

	return nil
}
