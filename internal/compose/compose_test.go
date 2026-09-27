// Copyright (c) 2026 Mamdouh Aboammar
// SPDX-License-Identifier: PolyForm-Shield-1.0.0

package compose_test

import (
	"context"
	"errors"
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

	t.Run("catalog_version reflects active projection from catalog port", func(t *testing.T) {
		cat.SetVersion("demo_store", "cat_v2_updated")
		defer cat.SetVersion("demo_store", "cat_demo_v1")

		req := sampleRequest()
		plan, err := composer.ComposeJourney(ctx, req)
		if err != nil {
			t.Fatalf("unexpected compose error: %v", err)
		}
		if plan.CatalogVersion != "cat_v2_updated" {
			t.Errorf("expected catalog_version 'cat_v2_updated', got %q", plan.CatalogVersion)
		}
	})

	t.Run("catalog store unavailable falls back to schema-valid baseline with fallback reason", func(t *testing.T) {
		failCat := &failingCatalogPort{}
		compUnavailable := compose.NewComposer(failCat, pol, val)

		req := sampleRequest()
		plan, err := compUnavailable.ComposeJourney(ctx, req)
		if err != nil {
			t.Fatalf("unexpected compose error: %v", err)
		}
		if plan.Status != "baseline" {
			t.Errorf("expected status 'baseline', got %q", plan.Status)
		}
		if plan.CatalogVersion != "unavailable" {
			t.Errorf("expected catalog_version 'unavailable', got %q", plan.CatalogVersion)
		}
		if plan.Provenance.FallbackReason == nil || *plan.Provenance.FallbackReason != "catalog_unavailable" {
			t.Errorf("expected fallback_reason 'catalog_unavailable', got %v", plan.Provenance.FallbackReason)
		}
	})
}

type failingCatalogPort struct{}

func (f *failingCatalogPort) GetVariant(_ context.Context, _, _ string) (*catalog.Variant, error) {
	return nil, errors.New("db down")
}

func (f *failingCatalogPort) ListLaptops(_ context.Context, _ string) ([]catalog.Variant, error) {
	return nil, errors.New("db down")
}

func (f *failingCatalogPort) GetActiveVersion(_ context.Context, _ string) (string, error) {
	return "", errors.New("db down")
}

func TestComposer_PreferenceToUI_Adaptation(t *testing.T) {
	ctx := context.Background()
	cat := catalog.NewInMemoryCatalog()
	pol := policy.NewPolicyChecker([]string{"demo_store"}, []string{"collection_top"})
	val, err := contracts.NewValidator("../../contracts/schemas")
	if err != nil {
		t.Fatalf("failed to create validator: %v", err)
	}

	composer := compose.NewComposer(cat, pol, val)

	t.Run("TC-RANK-01: budget filter excludes variants exceeding max_budget_minor", func(t *testing.T) {
		req := sampleRequest()
		budget := int64(110000) // $1,100
		req.Preferences = contracts.PreferencesContext{
			Purpose:        "everyday_value",
			MaxBudgetMinor: &budget,
		}

		plan, err := composer.ComposeJourney(ctx, req)
		if err != nil {
			t.Fatalf("unexpected compose error: %v", err)
		}

		if plan.Status != "adapted" {
			t.Fatalf("expected status 'adapted', got %q", plan.Status)
		}

		var strip *contracts.PlanSection
		for _, sec := range plan.Sections {
			if sec.Kind == "product-strip" {
				strip = &sec
				break
			}
		}
		if strip == nil {
			t.Fatal("expected product-strip section in adapted plan")
		}

		for _, item := range strip.Items {
			variant, err := cat.GetVariant(ctx, req.TenantID, item.VariantID)
			if err != nil {
				t.Fatalf("failed retrieving variant %s: %v", item.VariantID, err)
			}
			if variant.PriceMinor > budget {
				t.Errorf("variant %s price %d exceeds max budget %d", variant.ID, variant.PriceMinor, budget)
			}
		}
	})

	t.Run("TC-RANK-02: out-of-stock variants excluded even if within budget", func(t *testing.T) {
		req := sampleRequest()
		budget := int64(150000)
		req.Preferences = contracts.PreferencesContext{
			Purpose:        "portable_work",
			MaxBudgetMinor: &budget,
		}

		plan, err := composer.ComposeJourney(ctx, req)
		if err != nil {
			t.Fatalf("unexpected compose error: %v", err)
		}

		var strip *contracts.PlanSection
		for _, sec := range plan.Sections {
			if sec.Kind == "product-strip" {
				strip = &sec
				break
			}
		}
		if strip == nil {
			t.Fatal("expected product-strip section in adapted plan")
		}

		for _, item := range strip.Items {
			if item.VariantID == "lap_005" {
				t.Errorf("out-of-stock variant lap_005 surfaced in adapted strip")
			}
		}
	})

	t.Run("TC-RANK-03: rank_v1 deterministic scoring and tie-breaking for portable_work", func(t *testing.T) {
		req := sampleRequest()
		req.Preferences = contracts.PreferencesContext{
			Purpose: "portable_work",
		}

		plan, err := composer.ComposeJourney(ctx, req)
		if err != nil {
			t.Fatalf("unexpected compose error: %v", err)
		}

		var strip *contracts.PlanSection
		for _, sec := range plan.Sections {
			if sec.Kind == "product-strip" {
				strip = &sec
				break
			}
		}
		if strip == nil {
			t.Fatal("expected product-strip section in adapted plan")
		}

		if len(strip.Items) == 0 {
			t.Fatal("expected items in portable_work strip")
		}

		topItem := strip.Items[0].VariantID
		if topItem != "lap_007" && topItem != "lap_001" {
			t.Errorf("expected top item for portable_work to be lightweight laptop, got %s", topItem)
		}
	})

	t.Run("TC-RANK-04: zero matches emits empty-state section and status empty", func(t *testing.T) {
		req := sampleRequest()
		budget := int64(50000) // $500 (below any in-stock laptop)
		req.Preferences = contracts.PreferencesContext{
			MaxBudgetMinor: &budget,
		}

		plan, err := composer.ComposeJourney(ctx, req)
		if err != nil {
			t.Fatalf("unexpected compose error: %v", err)
		}

		if plan.Status != "empty" {
			t.Fatalf("expected status 'empty', got %q", plan.Status)
		}

		var emptySec *contracts.PlanSection
		for _, sec := range plan.Sections {
			if sec.Kind == "empty-state" {
				emptySec = &sec
				break
			}
		}
		if emptySec == nil {
			t.Fatal("expected empty-state section in empty plan")
		}
		foundBudgetReason := false
		for _, r := range emptySec.ReasonCodes {
			if r == "matches_budget" || r == "insufficient_evidence" {
				foundBudgetReason = true
				break
			}
		}
		if !foundBudgetReason {
			t.Errorf("expected reason code matches_budget or insufficient_evidence in empty-state, got %v", emptySec.ReasonCodes)
		}
	})

	t.Run("TC-RANK-05: valid matches emit product-strip with accurate reason codes", func(t *testing.T) {
		req := sampleRequest()
		budget := int64(120000)
		req.Preferences = contracts.PreferencesContext{
			Purpose:        "portable_work",
			MaxBudgetMinor: &budget,
		}

		plan, err := composer.ComposeJourney(ctx, req)
		if err != nil {
			t.Fatalf("unexpected compose error: %v", err)
		}

		if plan.Status != "adapted" {
			t.Fatalf("expected status 'adapted', got %q", plan.Status)
		}

		var strip *contracts.PlanSection
		for _, sec := range plan.Sections {
			if sec.Kind == "product-strip" {
				strip = &sec
				break
			}
		}
		if strip == nil {
			t.Fatal("expected product-strip section")
		}

		hasBudget := false
		hasPortability := false
		for _, r := range strip.ReasonCodes {
			if r == "matches_budget" {
				hasBudget = true
			}
			if r == "matches_declared_portability" {
				hasPortability = true
			}
		}
		if !hasBudget || !hasPortability {
			t.Errorf("expected reason codes to include matches_budget and matches_declared_portability, got %v", strip.ReasonCodes)
		}
	})
}
