// Copyright (c) 2026 Mamdouh Aboammar
// SPDX-License-Identifier: PolyForm-Shield-1.0.0

package catalog

import (
	"context"
	"fmt"
	"sync"
)

// InventoryStatus represents stock availability states.
type InventoryStatus string

const (
	InventoryInStock    InventoryStatus = "in_stock"
	InventoryOutOfStock InventoryStatus = "out_of_stock"
)

// Variant represents a purchasable SKU with physical and functional attributes.
type Variant struct {
	ID              string          `json:"id"`
	Title           string          `json:"title"`
	TitleAR         string          `json:"title_ar,omitempty"`
	Category        string          `json:"category"`
	PriceMinor      int64           `json:"price_minor"`
	Currency        string          `json:"currency"`
	InventoryStatus InventoryStatus `json:"inventory_status"`
	WeightG         *int            `json:"weight_g"`
	BatteryWh       *float64        `json:"battery_wh"`
	USBCPD          *bool           `json:"usb_c_pd"`
}

// CatalogPort defines the contract for catalog access.
type CatalogPort interface {
	GetVariant(ctx context.Context, tenantID, variantID string) (*Variant, error)
	ListLaptops(ctx context.Context, tenantID string) ([]Variant, error)
}

// InMemoryCatalog implements CatalogPort using pre-loaded synthetic fixtures.
type InMemoryCatalog struct {
	mu       sync.RWMutex
	catalogs map[string]map[string]Variant
}

func intPtr(v int) *int { return &v }
func floatPtr(v float64) *float64 { return &v }
func boolPtr(v bool) *bool { return &v }

// NewInMemoryCatalog builds an in-memory catalog populated with synthetic fixtures.
func NewInMemoryCatalog() *InMemoryCatalog {
	demoVariants := map[string]Variant{
		"lap_001": {
			ID:              "lap_001",
			Title:           "Light 13 Demo",
			TitleAR:         "حاسوب محمول خفيف 13 تجريبي",
			Category:        "laptops",
			PriceMinor:      109900,
			Currency:        "USD",
			InventoryStatus: InventoryInStock,
			WeightG:         intPtr(1270),
			BatteryWh:       floatPtr(58),
			USBCPD:          boolPtr(true),
		},
		"lap_002": {
			ID:              "lap_002",
			Title:           "Travel 14 Demo",
			TitleAR:         "حاسوب محمول للسفر 14 تجريبي",
			Category:        "laptops",
			PriceMinor:      124900,
			Currency:        "USD",
			InventoryStatus: InventoryInStock,
			WeightG:         intPtr(1450),
			BatteryWh:       floatPtr(65),
			USBCPD:          boolPtr(true),
		},
		"lap_003": {
			ID:              "lap_003",
			Title:           "Heavy 17 Demo",
			TitleAR:         "حاسوب محمول مكتبي 17 تجريبي",
			Category:        "laptops",
			PriceMinor:      169900,
			Currency:        "USD",
			InventoryStatus: InventoryInStock,
			WeightG:         intPtr(2650),
			BatteryWh:       floatPtr(83),
			USBCPD:          boolPtr(false),
		},
		"lap_004": {
			ID:              "lap_004",
			Title:           "Everyday 15 Demo",
			TitleAR:         "حاسوب محمول للاستخدام اليومي 15 تجريبي",
			Category:        "laptops",
			PriceMinor:      79900,
			Currency:        "USD",
			InventoryStatus: InventoryInStock,
			WeightG:         intPtr(1840),
			BatteryWh:       floatPtr(45),
			USBCPD:          boolPtr(true),
		},
		"lap_005": {
			ID:              "lap_005",
			Title:           "Out of stock Light",
			TitleAR:         "حاسوب خفيف (نفد من المخزون)",
			Category:        "laptops",
			PriceMinor:      89900,
			Currency:        "USD",
			InventoryStatus: InventoryOutOfStock,
			WeightG:         intPtr(1200),
			BatteryWh:       floatPtr(55),
			USBCPD:          boolPtr(true),
		},
		"lap_006": {
			ID:              "lap_006",
			Title:           "Unknown Weight Demo",
			TitleAR:         "حاسوب بمواصفات غير محددة تجريبي",
			Category:        "laptops",
			PriceMinor:      99900,
			Currency:        "USD",
			InventoryStatus: InventoryInStock,
			WeightG:         nil, // Missing attribute edge case
			BatteryWh:       nil,
			USBCPD:          nil,
		},
		"lap_007": {
			ID:              "lap_007",
			Title:           "Ultra Light Arabic 14 Demo",
			TitleAR:         "حاسوب محمول فائق الخفة 14",
			Category:        "laptops",
			PriceMinor:      115000,
			Currency:        "USD",
			InventoryStatus: InventoryInStock,
			WeightG:         intPtr(1190),
			BatteryWh:       floatPtr(60),
			USBCPD:          boolPtr(true),
		},
	}

	return &InMemoryCatalog{
		catalogs: map[string]map[string]Variant{
			"demo_store": demoVariants,
		},
	}
}

// GetVariant looks up a specific variant within a tenant catalog.
func (c *InMemoryCatalog) GetVariant(_ context.Context, tenantID, variantID string) (*Variant, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	tenantCatalog, ok := c.catalogs[tenantID]
	if !ok {
		return nil, fmt.Errorf("tenant %q not found", tenantID)
	}

	v, ok := tenantCatalog[variantID]
	if !ok {
		return nil, fmt.Errorf("variant %q not found in tenant %q", variantID, tenantID)
	}

	res := v
	return &res, nil
}

// ListLaptops returns all laptop variants for a given tenant.
func (c *InMemoryCatalog) ListLaptops(_ context.Context, tenantID string) ([]Variant, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	tenantCatalog, ok := c.catalogs[tenantID]
	if !ok {
		return nil, fmt.Errorf("tenant %q not found", tenantID)
	}

	var laptops []Variant
	for _, v := range tenantCatalog {
		if v.Category == "laptops" {
			laptops = append(laptops, v)
		}
	}
	return laptops, nil
}
