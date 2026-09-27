// Copyright (c) 2026 Mamdouh Aboammar
// SPDX-License-Identifier: PolyForm-Shield-1.0.0

package catalog

import (
	"context"
	"errors"

	"github.com/imMamdouhaboammar/interactive-buying-journey/internal/storage/postgres"
)

var (
	ErrNoActiveCatalogVersion = errors.New("no active catalog version for tenant")
	ErrVariantNotFound        = errors.New("variant not found")
)

// PostgresCatalog implements CatalogPort and SearchPort backed by PostgreSQL with RLS.
type PostgresCatalog struct {
	db *postgres.DB
}

// NewPostgresCatalog builds a new PostgresCatalog.
func NewPostgresCatalog(db *postgres.DB) *PostgresCatalog {
	return &PostgresCatalog{db: db}
}

// GetActiveVersion retrieves the current ACTIVE catalog version ID.
func (c *PostgresCatalog) GetActiveVersion(ctx context.Context, tenantID string) (string, error) {
	return "", errors.New("not implemented")
}

// GetVariant retrieves a single active eligible variant by ID.
func (c *PostgresCatalog) GetVariant(ctx context.Context, tenantID, variantID string) (*Variant, error) {
	return nil, errors.New("not implemented")
}

// ListLaptops returns all active eligible laptops ordered by product_id, then variant_id.
func (c *PostgresCatalog) ListLaptops(ctx context.Context, tenantID string) ([]Variant, error) {
	return nil, errors.New("not implemented")
}

// Search queries variants using full-text search, filters, and deterministic tie-breaking.
func (c *PostgresCatalog) Search(ctx context.Context, tenantID string, query SearchQuery) (*SearchResult, error) {
	return nil, errors.New("not implemented")
}
