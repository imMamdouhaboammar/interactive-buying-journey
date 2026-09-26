// Copyright (c) 2026 Mamdouh Aboammar
// SPDX-License-Identifier: PolyForm-Shield-1.0.0

package policy_test

import (
	"context"
	"testing"

	"github.com/imMamdouhaboammar/interactive-buying-journey/internal/policy"
)

func TestPolicyChecker_ValidateTenant(t *testing.T) {
	ctx := context.Background()
	checker := policy.NewPolicyChecker([]string{"demo_store"}, []string{"collection_top", "collection_grid", "pdp_related"})

	t.Run("valid tenant passes", func(t *testing.T) {
		if err := checker.ValidateTenant(ctx, "demo_store"); err != nil {
			t.Errorf("expected nil error for demo_store, got %v", err)
		}
	})

	t.Run("unknown tenant fails", func(t *testing.T) {
		if err := checker.ValidateTenant(ctx, "malicious_store"); err == nil {
			t.Errorf("expected error for unknown tenant, got nil")
		}
	})
}

func TestPolicyChecker_ValidateSlots(t *testing.T) {
	ctx := context.Background()
	checker := policy.NewPolicyChecker([]string{"demo_store"}, []string{"collection_top", "collection_grid"})

	t.Run("allowed slots pass", func(t *testing.T) {
		err := checker.ValidateSlots(ctx, "demo_store", []string{"collection_top"})
		if err != nil {
			t.Errorf("expected allowed slots to pass, got %v", err)
		}
	})

	t.Run("disallowed slot fails", func(t *testing.T) {
		err := checker.ValidateSlots(ctx, "demo_store", []string{"unauthorized_slot"})
		if err == nil {
			t.Errorf("expected error for disallowed slot, got nil")
		}
	})

	t.Run("empty slots fails", func(t *testing.T) {
		err := checker.ValidateSlots(ctx, "demo_store", []string{})
		if err == nil {
			t.Errorf("expected error for empty slots, got nil")
		}
	})
}

func TestPolicyChecker_KillSwitch(t *testing.T) {
	checker := policy.NewPolicyChecker([]string{"demo_store"}, []string{"collection_top"})

	t.Run("enabled by default", func(t *testing.T) {
		if !checker.IsAdaptationEnabled() {
			t.Errorf("expected adaptation enabled by default")
		}
	})

	t.Run("toggled via setter", func(t *testing.T) {
		checker.SetAdaptationEnabled(false)
		if checker.IsAdaptationEnabled() {
			t.Errorf("expected adaptation disabled after toggle")
		}
		checker.SetAdaptationEnabled(true)
		if !checker.IsAdaptationEnabled() {
			t.Errorf("expected adaptation enabled after second toggle")
		}
	})
}
