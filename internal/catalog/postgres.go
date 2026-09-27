// Copyright (c) 2026 Mamdouh Aboammar
// SPDX-License-Identifier: PolyForm-Shield-1.0.0

package catalog

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/imMamdouhaboammar/interactive-buying-journey/internal/ingest"
	"github.com/imMamdouhaboammar/interactive-buying-journey/internal/storage/postgres"
)

var (
	ErrNoActiveCatalogVersion = errors.New("no active catalog version for tenant")
	ErrVariantNotFound        = errors.New("variant not found")
)

const MaxSearchResults = 200

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
	var versionID string
	err := c.db.WithTenantQuery(ctx, tenantID, func(tx pgx.Tx) error {
		return tx.QueryRow(ctx, `
			SELECT version_id
			FROM catalog_versions
			WHERE tenant_id = $1 AND status = 'ACTIVE'
		`, tenantID).Scan(&versionID)
	})

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", ErrNoActiveCatalogVersion
		}
		return "", fmt.Errorf("query active catalog version: %w", err)
	}

	return versionID, nil
}

// GetVariant retrieves a single active eligible variant by ID.
func (c *PostgresCatalog) GetVariant(ctx context.Context, tenantID, variantID string) (*Variant, error) {
	var v Variant
	var brand *string
	var attrsRaw []byte

	err := c.db.WithTenantQuery(ctx, tenantID, func(tx pgx.Tx) error {
		return tx.QueryRow(ctx, `
			SELECT v.variant_id, v.product_id, v.sku, v.title, v.category, v.brand,
			       v.currency, v.price_minor, v.inventory_status, v.attributes
			FROM variants v
			WHERE v.tenant_id = $1 AND v.variant_id = $2 AND v.published = TRUE AND v.is_tombstoned = FALSE
		`, tenantID, variantID).Scan(
			&v.ID, &v.ProductID, &v.SKU, &v.Title, &v.Category, &brand,
			&v.Currency, &v.PriceMinor, &v.InventoryStatus, &attrsRaw,
		)
	})

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrVariantNotFound
		}
		return nil, fmt.Errorf("get variant: %w", err)
	}

	v.WeightG, v.BatteryWh, v.USBCPD = extractLaptopAttrs(attrsRaw)
	return &v, nil
}

// ListLaptops returns all active eligible laptops ordered by product_id, then variant_id.
func (c *PostgresCatalog) ListLaptops(ctx context.Context, tenantID string) ([]Variant, error) {
	res, err := c.Search(ctx, tenantID, SearchQuery{
		Category: "laptops",
		Limit:    MaxSearchResults,
	})
	if err != nil {
		return nil, err
	}
	return res.Variants, nil
}

// Search queries variants using full-text search, filters, and deterministic tie-breaking.
func (c *PostgresCatalog) Search(ctx context.Context, tenantID string, query SearchQuery) (*SearchResult, error) {
	limit := query.Limit
	if limit <= 0 || limit > MaxSearchResults {
		limit = MaxSearchResults
	}

	cleanQ := strings.TrimSpace(query.Query)
	if len(cleanQ) > 256 {
		cleanQ = cleanQ[:256]
	}

	var sb strings.Builder
	args := []any{tenantID}

	sb.WriteString(`
		SELECT v.variant_id, v.product_id, v.sku, v.title, v.category, v.brand,
		       v.currency, v.price_minor, v.inventory_status, v.attributes
		FROM variants v
		WHERE v.tenant_id = $1
		  AND v.published = TRUE
		  AND v.is_tombstoned = FALSE
		  AND v.inventory_status = 'in_stock'
		  AND v.last_verified_at >= NOW() - ((SELECT COALESCE(max_staleness_seconds, 86400) FROM tenants WHERE tenant_id = $1) * INTERVAL '1 second')
	`)

	if query.Category != "" {
		args = append(args, query.Category)
		sb.WriteString(fmt.Sprintf(" AND v.category = $%d", len(args)))
	}

	if query.Currency != "" {
		args = append(args, query.Currency)
		sb.WriteString(fmt.Sprintf(" AND v.currency = $%d", len(args)))
	}

	if query.MaxBudget != nil {
		args = append(args, *query.MaxBudget)
		sb.WriteString(fmt.Sprintf(" AND v.price_minor <= $%d", len(args)))
	}

	if len(query.BrandIDs) > 0 {
		args = append(args, query.BrandIDs)
		sb.WriteString(fmt.Sprintf(" AND v.brand = ANY($%d)", len(args)))
	}

	if cleanQ != "" {
		normQ := ingest.NormalizeArabicForSearch(cleanQ)
		args = append(args, cleanQ, normQ)
		qIdx := len(args) - 1
		normIdx := len(args)

		sb.WriteString(fmt.Sprintf(`
			AND (v.tsv_en @@ plainto_tsquery('english', $%d) OR v.tsv_ar @@ plainto_tsquery('arabic', $%d))
			ORDER BY GREATEST(
				ts_rank(v.tsv_en, plainto_tsquery('english', $%d)),
				ts_rank(v.tsv_ar, plainto_tsquery('arabic', $%d))
			) DESC, v.product_id ASC, v.variant_id ASC
		`, qIdx, normIdx, qIdx, normIdx))
	} else {
		sb.WriteString(" ORDER BY v.product_id ASC, v.variant_id ASC")
	}

	args = append(args, limit)
	sb.WriteString(fmt.Sprintf(" LIMIT $%d", len(args)))

	var variants []Variant
	err := c.db.WithTenantQuery(ctx, tenantID, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, sb.String(), args...)
		if err != nil {
			return err
		}
		defer rows.Close()

		for rows.Next() {
			var v Variant
			var brand *string
			var attrsRaw []byte

			if err := rows.Scan(
				&v.ID, &v.ProductID, &v.SKU, &v.Title, &v.Category, &brand,
				&v.Currency, &v.PriceMinor, &v.InventoryStatus, &attrsRaw,
			); err != nil {
				return err
			}

			v.WeightG, v.BatteryWh, v.USBCPD = extractLaptopAttrs(attrsRaw)
			variants = append(variants, v)
		}
		return rows.Err()
	})

	if err != nil {
		return nil, fmt.Errorf("search variants: %w", err)
	}

	if variants == nil {
		variants = []Variant{}
	}

	return &SearchResult{
		Variants:      variants,
		ReturnedCount: len(variants),
	}, nil
}

func extractLaptopAttrs(raw []byte) (*int, *float64, *bool) {
	if len(raw) == 0 {
		return nil, nil, nil
	}

	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, nil, nil
	}

	var weightG *int
	if w, ok := m["weight_g"]; ok {
		switch v := w.(type) {
		case float64:
			i := int(v)
			weightG = &i
		}
	}

	var batteryWh *float64
	if b, ok := m["battery_wh"]; ok {
		switch v := b.(type) {
		case float64:
			batteryWh = &v
		}
	}

	var usbCPD *bool
	if u, ok := m["usb_c_pd"]; ok {
		switch v := u.(type) {
		case bool:
			usbCPD = &v
		}
	}

	return weightG, batteryWh, usbCPD
}
