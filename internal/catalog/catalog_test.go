// Copyright (c) 2026 Mamdouh Aboammar
// SPDX-License-Identifier: PolyForm-Shield-1.0.0

package catalog_test

import (
	"context"
	"testing"

	"github.com/imMamdouhaboammar/interactive-buying-journey/internal/catalog"
)

func TestInMemoryCatalog_ListLaptops(t *testing.T) {
	ctx := context.Background()
	repo := catalog.NewInMemoryCatalog()

	t.Run("returns synthetic laptops for valid tenant", func(t *testing.T) {
		laptops, err := repo.ListLaptops(ctx, "demo_store")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(laptops) == 0 {
			t.Fatalf("expected at least one laptop, got 0")
		}

		// Verify edge case 1: missing attribute on lap_006
		var lap006 *catalog.Variant
		for i := range laptops {
			if laptops[i].ID == "lap_006" {
				lap006 = &laptops[i]
				break
			}
		}
		if lap006 == nil {
			t.Fatalf("lap_006 not found in catalog")
		}
		if lap006.WeightG != nil {
			t.Errorf("lap_006 weight must be nil (unknown), got %v", *lap006.WeightG)
		}
		if lap006.BatteryWh != nil {
			t.Errorf("lap_006 battery must be nil (unknown), got %v", *lap006.BatteryWh)
		}
		if lap006.USBCPD != nil {
			t.Errorf("lap_006 usb_c_pd must be nil (unknown), got %v", *lap006.USBCPD)
		}

		// Verify edge case 2: Arabic product title present
		var foundArabic bool
		for _, lap := range laptops {
			if lap.TitleAR != "" {
				foundArabic = true
				break
			}
		}
		if !foundArabic {
			t.Errorf("expected at least one laptop with Arabic title")
		}
	})

	t.Run("unknown tenant returns error", func(t *testing.T) {
		_, err := repo.ListLaptops(ctx, "non_existent_tenant")
		if err == nil {
			t.Errorf("expected error for unknown tenant, got nil")
		}
	})
}

func TestInMemoryCatalog_GetVariant(t *testing.T) {
	ctx := context.Background()
	repo := catalog.NewInMemoryCatalog()

	t.Run("returns existing variant", func(t *testing.T) {
		v, err := repo.GetVariant(ctx, "demo_store", "lap_001")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if v.ID != "lap_001" {
			t.Errorf("expected id lap_001, got %s", v.ID)
		}
		if v.PriceMinor != 109900 {
			t.Errorf("expected price 109900, got %d", v.PriceMinor)
		}
		if v.InventoryStatus != catalog.InventoryInStock {
			t.Errorf("expected in_stock, got %s", v.InventoryStatus)
		}
	})

	t.Run("returns error for unknown variant", func(t *testing.T) {
		_, err := repo.GetVariant(ctx, "demo_store", "lap_999")
		if err == nil {
			t.Errorf("expected error for unknown variant, got nil")
		}
	})
}
