// Copyright (c) 2026 Mamdouh Aboammar
// SPDX-License-Identifier: PolyForm-Shield-1.0.0

package compose_test

import (
	"context"
	"testing"

	"github.com/imMamdouhaboammar/interactive-buying-journey/internal/catalog"
	"github.com/imMamdouhaboammar/interactive-buying-journey/internal/compose"
	"github.com/imMamdouhaboammar/interactive-buying-journey/internal/contracts"
	"github.com/imMamdouhaboammar/interactive-buying-journey/internal/policy"
)

func sampleRequest() *contracts.ComposeRequest {
	catID := "laptops"
	return &contracts.ComposeRequest{
		ContractVersion: "1.0",
		RequestID:       "req_compose_test_001",
		TenantID:        "demo_store",
		SessionToken:    "sess_test_token_12345",
		Page: contracts.PageContext{
			Kind:       "collection",
			CategoryID: &catID,
		},
		Locale: "en",
		Consent: contracts.ConsentContext{
			Version:  "1.0",
			Purposes: []string{"necessary"},
		},
		Preferences:  contracts.PreferencesContext{},
		AllowedSlots: []string{"collection_top"},
	}
}

func TestComposer_ComposeJourney_BaselinePlan(t *testing.T) {
	ctx := context.Background()
	cat := catalog.NewInMemoryCatalog()
	pol := policy.NewPolicyChecker([]string{"demo_store"}, []string{"collection_top"})
	val, err := contracts.NewValidator("../../contracts/schemas")
	if err != nil {
		t.Fatalf("failed to create validator: %v", err)
	}

	composer := compose.NewComposer(cat, pol, val)

	t.Run("returns valid baseline plan with empty sections", func(t *testing.T) {
		req := sampleRequest()
		plan, err := composer.ComposeJourney(ctx, req)
		if err != nil {
			t.Fatalf("unexpected compose error: %v", err)
		}

		if plan.Status != "baseline" {
			t.Errorf("expected status 'baseline', got %q", plan.Status)
		}
		if plan.Provenance.Strategy != "merchant_baseline" {
			t.Errorf("expected strategy 'merchant_baseline', got %q", plan.Provenance.Strategy)
		}
		if len(plan.Sections) != 0 {
			t.Errorf("expected empty sections array for baseline plan, got %d sections", len(plan.Sections))
		}
		if plan.TenantID != "demo_store" {
			t.Errorf("expected tenant_id 'demo_store', got %q", plan.TenantID)
		}
		if plan.RequestID != req.RequestID {
			t.Errorf("expected request_id %q, got %q", req.RequestID, plan.RequestID)
		}
	})

	t.Run("kill switch returns baseline immediately", func(t *testing.T) {
		pol.SetAdaptationEnabled(false)
		defer pol.SetAdaptationEnabled(true)

		req := sampleRequest()
		plan, err := composer.ComposeJourney(ctx, req)
		if err != nil {
			t.Fatalf("unexpected compose error: %v", err)
		}

		if plan.Status != "baseline" {
			t.Errorf("expected status 'baseline' under kill switch, got %q", plan.Status)
		}
		if plan.Provenance.Strategy != "merchant_baseline" {
			t.Errorf("expected strategy 'merchant_baseline', got %q", plan.Provenance.Strategy)
		}
	})

	t.Run("unauthorized tenant is rejected", func(t *testing.T) {
		req := sampleRequest()
		req.TenantID = "unauthorized_tenant"
		_, err := composer.ComposeJourney(ctx, req)
		if err == nil {
			t.Errorf("expected error for unauthorized tenant, got nil")
		}
	})

	t.Run("disallowed slot is rejected", func(t *testing.T) {
		req := sampleRequest()
		req.AllowedSlots = []string{"disallowed_slot"}
		_, err := composer.ComposeJourney(ctx, req)
		if err == nil {
			t.Errorf("expected error for disallowed slot, got nil")
		}
	})
}
