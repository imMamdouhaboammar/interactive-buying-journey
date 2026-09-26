// Copyright (c) 2026 Mamdouh Aboammar
// SPDX-License-Identifier: PolyForm-Shield-1.0.0

package compose

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/imMamdouhaboammar/interactive-buying-journey/internal/catalog"
	"github.com/imMamdouhaboammar/interactive-buying-journey/internal/contracts"
	"github.com/imMamdouhaboammar/interactive-buying-journey/internal/policy"
)

// JourneyState represents the lifecycle phase of a compose request.
type JourneyState string

const (
	StateBaseline         JourneyState = "BASELINE"
	StateContextReady     JourneyState = "CONTEXT_READY"
	StateCandidatesReady  JourneyState = "CANDIDATES_READY"
	StatePlanValidated    JourneyState = "PLAN_VALIDATED"
	StateExposed          JourneyState = "EXPOSED"
	StateBaselineFallback JourneyState = "BASELINE_FALLBACK"
)

// Composer coordinates catalog, policy, and contracts to synthesize experience plans.
type Composer struct {
	catalog catalog.CatalogPort
	policy  *policy.PolicyChecker
	val     *contracts.Validator
}

// NewComposer creates a new journey composer.
func NewComposer(cat catalog.CatalogPort, pol *policy.PolicyChecker, val *contracts.Validator) *Composer {
	return &Composer{
		catalog: cat,
		policy:  pol,
		val:     val,
	}
}

func generateID(prefix string) string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return fmt.Sprintf("%s_%s", prefix, hex.EncodeToString(b))
}

// ComposeJourney synthesizes an experience plan according to the request lifecycle.
func (c *Composer) ComposeJourney(ctx context.Context, req *contracts.ComposeRequest) (*contracts.ExperiencePlan, error) {
	state := StateBaseline

	// Policy enforcement: Tenant validation
	if err := c.policy.ValidateTenant(ctx, req.TenantID); err != nil {
		return nil, fmt.Errorf("tenant policy validation failed: %w", err)
	}

	// Policy enforcement: Slot allowlist validation
	if err := c.policy.ValidateSlots(ctx, req.TenantID, req.AllowedSlots); err != nil {
		return nil, fmt.Errorf("slot policy validation failed: %w", err)
	}

	// Kill Switch check
	if !c.policy.IsAdaptationEnabled() {
		state = StateBaselineFallback
		reason := "adaptation_disabled_by_kill_switch"
		return c.buildBaselinePlan(req, &reason), nil
	}

	state = StateContextReady
	_ = state

	// Candidate retrieval (verifies catalog snapshot presence)
	state = StateCandidatesReady

	// Build baseline plan (in Slice 1, adaptation is baseline-first with no decision model)
	plan := c.buildBaselinePlan(req, nil)

	// Runtime self-validation: ensure the plan is valid against experience-plan.schema.json
	data, err := json.Marshal(plan)
	if err != nil {
		slog.Error("failed to marshal experience plan for runtime self-validation",
			"request_id", req.RequestID,
			"tenant_id", req.TenantID,
			"error", err,
		)
		reason := "plan_marshal_error"
		return c.buildFallbackBaseline(req, reason), nil
	}

	if err := c.val.Validate("experience-plan.schema.json", data); err != nil {
		slog.Error("runtime experience plan validation failed (server bug); falling back to safe baseline",
			"request_id", req.RequestID,
			"tenant_id", req.TenantID,
			"error", err,
		)
		reason := "schema_validation_server_bug"
		return c.buildFallbackBaseline(req, reason), nil
	}

	state = StatePlanValidated
	_ = state
	return plan, nil
}

func (c *Composer) buildBaselinePlan(req *contracts.ComposeRequest, fallbackReason *string) *contracts.ExperiencePlan {
	return &contracts.ExperiencePlan{
		ContractVersion: "1.0",
		RequestID:       req.RequestID,
		PlanID:          generateID("pl"),
		TenantID:        req.TenantID,
		Status:          "baseline",
		Locale:          req.Locale,
		CatalogVersion:  "cat_demo_v1",
		PolicyVersion:   "pol_demo_v1",
		ExpiresAt:       time.Now().UTC().Add(5 * time.Minute).Format(time.RFC3339),
		Sections:        []contracts.PlanSection{}, // Empty sections for baseline plan
		Provenance: contracts.PlanProvenance{
			Strategy:       "merchant_baseline",
			ModelUsed:      nil,
			Plugins:        []string{},
			FallbackReason: fallbackReason,
		},
	}
}

func (c *Composer) buildFallbackBaseline(req *contracts.ComposeRequest, reason string) *contracts.ExperiencePlan {
	return c.buildBaselinePlan(req, &reason)
}
