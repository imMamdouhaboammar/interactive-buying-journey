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
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/imMamdouhaboammar/interactive-buying-journey/internal/catalog"
	"github.com/imMamdouhaboammar/interactive-buying-journey/internal/contracts"
	"github.com/imMamdouhaboammar/interactive-buying-journey/internal/policy"
)

// Clock returns the current time.
type Clock func() time.Time

// IDGenerator generates unique IDs with the given prefix.
type IDGenerator func(prefix string) string

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
	catalog          catalog.CatalogPort
	policy           *policy.PolicyChecker
	val              *contracts.Validator
	clock            Clock
	idGen            IDGenerator
	versionMu        sync.RWMutex
	lastKnownVersion map[string]string
}

// NewComposer creates a new journey composer with default clock and ID generator.
func NewComposer(cat catalog.CatalogPort, pol *policy.PolicyChecker, val *contracts.Validator) *Composer {
	return NewComposerWithClockAndID(cat, pol, val, time.Now, defaultIDGenerator)
}

// NewComposerWithClockAndID creates a composer with injected clock and ID generator for deterministic testing.
func NewComposerWithClockAndID(cat catalog.CatalogPort, pol *policy.PolicyChecker, val *contracts.Validator, clock Clock, idGen IDGenerator) *Composer {
	if clock == nil {
		clock = time.Now
	}
	if idGen == nil {
		idGen = defaultIDGenerator
	}
	return &Composer{
		catalog:          cat,
		policy:           pol,
		val:              val,
		clock:            clock,
		idGen:            idGen,
		lastKnownVersion: make(map[string]string),
	}
}

func defaultIDGenerator(prefix string) string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return fmt.Sprintf("%s_%s", prefix, hex.EncodeToString(b))
}

// ComposeJourney synthesizes an experience plan according to the request lifecycle.
func (c *Composer) ComposeJourney(ctx context.Context, req *contracts.ComposeRequest) (*contracts.ExperiencePlan, error) {
	// Policy enforcement: Tenant validation
	if err := c.policy.ValidateTenant(ctx, req.TenantID); err != nil {
		return nil, fmt.Errorf("tenant policy validation failed: %w", err)
	}

	// Policy enforcement: Slot allowlist validation
	if err := c.policy.ValidateSlots(ctx, req.TenantID, req.AllowedSlots); err != nil {
		return nil, fmt.Errorf("slot policy validation failed: %w", err)
	}

	// Determine active catalog projection version
	activeVer, catErr := c.catalog.GetActiveVersion(ctx, req.TenantID)
	var fallbackReason *string
	catalogVersion := activeVer
	if catErr != nil {
		slog.Warn("catalog store unavailable; falling back to last known version",
			"tenant_id", req.TenantID,
			"error", catErr,
		)
		reason := "catalog_unavailable"
		fallbackReason = &reason

		c.versionMu.RLock()
		cached, ok := c.lastKnownVersion[req.TenantID]
		c.versionMu.RUnlock()

		if ok && cached != "" {
			catalogVersion = cached
		} else {
			catalogVersion = "unavailable"
		}
	} else {
		c.versionMu.Lock()
		c.lastKnownVersion[req.TenantID] = activeVer
		c.versionMu.Unlock()
	}

	// Kill Switch check
	if !c.policy.IsAdaptationEnabled() {
		reason := "adaptation_disabled_by_kill_switch"
		return c.buildBaselinePlan(req, catalogVersion, &reason), nil
	}

	// Build plan: if preferences declared and no fallback, build adapted plan; else baseline
	var plan *contracts.ExperiencePlan
	if hasPreferences(req.Preferences) && fallbackReason == nil {
		adapted, err := c.buildAdaptedPlan(ctx, req, catalogVersion)
		if err != nil {
			slog.Warn("failed to build adapted plan; falling back to baseline",
				"tenant_id", req.TenantID,
				"error", err,
			)
			reason := "adaptation_error"
			plan = c.buildBaselinePlan(req, catalogVersion, &reason)
		} else {
			plan = adapted
		}
	} else {
		plan = c.buildBaselinePlan(req, catalogVersion, fallbackReason)
	}

	// Runtime self-validation: ensure the plan is valid against experience-plan.schema.json
	data, err := json.Marshal(plan)
	if err != nil {
		slog.Error("failed to marshal experience plan for runtime self-validation",
			"request_id", req.RequestID,
			"tenant_id", req.TenantID,
			"error", err,
		)
		reason := "plan_marshal_error"
		return c.buildFallbackBaseline(req, catalogVersion, reason), nil
	}

	if err := c.val.Validate("experience-plan.schema.json", data); err != nil {
		slog.Error("runtime experience plan validation failed (server bug); falling back to safe baseline",
			"request_id", req.RequestID,
			"tenant_id", req.TenantID,
			"error", err,
		)
		reason := "schema_validation_server_bug"
		return c.buildFallbackBaseline(req, catalogVersion, reason), nil
	}

	return plan, nil
}

func hasPreferences(prefs contracts.PreferencesContext) bool {
	return prefs.Purpose != "" || prefs.MaxBudgetMinor != nil || prefs.MinBatteryHours != nil || prefs.MaxWeightGrams != nil || len(prefs.BrandIDs) > 0
}

func (c *Composer) buildAdaptedPlan(ctx context.Context, req *contracts.ComposeRequest, catalogVersion string) (*contracts.ExperiencePlan, error) {
	slotID := "collection_top"
	if len(req.AllowedSlots) > 0 {
		slotID = req.AllowedSlots[0]
		for _, s := range req.AllowedSlots {
			if s == "collection_top" {
				slotID = "collection_top"
				break
			}
		}
	}

	laptops, err := c.catalog.ListLaptops(ctx, req.TenantID)
	if err != nil {
		return nil, fmt.Errorf("list laptops: %w", err)
	}

	var eligible []catalog.Variant
	for _, v := range laptops {
		if v.InventoryStatus != catalog.InventoryInStock {
			continue
		}
		if req.Preferences.MaxBudgetMinor != nil && v.PriceMinor > *req.Preferences.MaxBudgetMinor {
			continue
		}
		if req.Preferences.MaxWeightGrams != nil && v.WeightG != nil && *v.WeightG > *req.Preferences.MaxWeightGrams {
			continue
		}
		eligible = append(eligible, v)
	}

	type scoredCandidate struct {
		variant catalog.Variant
		score   float64
	}

	scored := make([]scoredCandidate, len(eligible))
	for i, v := range eligible {
		sExp := 0.5
		switch req.Preferences.Purpose {
		case "portable_work":
			if v.WeightG != nil {
				if *v.WeightG <= 1200 {
					sExp = 1.0
				} else if *v.WeightG <= 1500 {
					sExp = 0.85
				} else {
					sExp = 0.3
				}
			}
			if v.BatteryWh != nil && *v.BatteryWh >= 55 {
				sExp += 0.15
			}
		case "performance":
			if v.BatteryWh != nil && *v.BatteryWh >= 70 {
				sExp = 0.9
			} else {
				sExp = 0.5
			}
		case "everyday_value":
			if v.PriceMinor <= 90000 {
				sExp = 0.95
			} else {
				sExp = 0.5
			}
		}
		if sExp > 1.0 {
			sExp = 1.0
		}

		sLex := 0.5
		cleanTitle := strings.ToLower(v.Title)
		cleanTitleAR := strings.ToLower(v.TitleAR)
		if req.Preferences.Purpose == "portable_work" && (strings.Contains(cleanTitle, "light") || strings.Contains(cleanTitle, "travel") || strings.Contains(cleanTitleAR, "خفيف") || strings.Contains(cleanTitleAR, "سفر")) {
			sLex = 1.0
		}

		sBus := 0.5
		qualCount := 0
		if v.WeightG != nil {
			qualCount++
		}
		if v.BatteryWh != nil {
			qualCount++
		}
		if v.USBCPD != nil {
			qualCount++
		}
		sQual := float64(qualCount) / 3.0

		totalScore := 0.20*sLex + 0.50*sExp + 0.15*sBus + 0.15*sQual
		scored[i] = scoredCandidate{variant: v, score: totalScore}
	}

	sort.Slice(scored, func(i, j int) bool {
		diff := scored[i].score - scored[j].score
		if diff > 1e-6 {
			return true
		} else if diff < -1e-6 {
			return false
		}
		if scored[i].variant.PriceMinor != scored[j].variant.PriceMinor {
			return scored[i].variant.PriceMinor < scored[j].variant.PriceMinor
		}
		return scored[i].variant.ID < scored[j].variant.ID
	})

	intentPicker := contracts.PlanSection{
		SectionID:   "intent_picker",
		Kind:        "intent-picker",
		SlotID:      slotID,
		Priority:    10,
		Items:       []contracts.PlanItem{},
		ReasonCodes: []string{"data_available"},
		Config: &contracts.SectionConfig{
			LabelKey: "what_matters_most",
			Options: []contracts.ConfigOption{
				{ID: "portable_work", LabelKey: "intent_portable_work"},
				{ID: "performance", LabelKey: "intent_performance"},
				{ID: "everyday_value", LabelKey: "intent_everyday_value"},
				{ID: "not_sure", LabelKey: "intent_not_sure"},
			},
		},
	}

	if len(scored) == 0 {
		reasonCodes := []string{"matches_budget"}
		if req.Preferences.MaxBudgetMinor == nil {
			reasonCodes = []string{"insufficient_evidence"}
		}

		emptyState := contracts.PlanSection{
			SectionID:   "empty_shortlist",
			Kind:        "empty-state",
			SlotID:      slotID,
			Priority:    20,
			Items:       []contracts.PlanItem{},
			ReasonCodes: reasonCodes,
			Config: &contracts.SectionConfig{
				LabelKey: "no_matching_laptops",
			},
		}

		return &contracts.ExperiencePlan{
			ContractVersion: "1.0",
			RequestID:       req.RequestID,
			PlanID:          c.idGen("pl"),
			TenantID:        req.TenantID,
			Status:          "empty",
			Locale:          req.Locale,
			CatalogVersion:  catalogVersion,
			PolicyVersion:   "pol_demo_v1",
			ExpiresAt:       c.clock().UTC().Add(5 * time.Minute).Format(time.RFC3339),
			Sections:        []contracts.PlanSection{intentPicker, emptyState},
			Provenance: contracts.PlanProvenance{
				Strategy:       "deterministic",
				ModelUsed:      nil,
				Plugins:        []string{"ibj.preference-ranker"},
				FallbackReason: nil,
			},
		}, nil
	}

	limit := 4
	if len(scored) < limit {
		limit = len(scored)
	}
	items := make([]contracts.PlanItem, limit)
	for i := 0; i < limit; i++ {
		items[i] = contracts.PlanItem{
			VariantID:      scored[i].variant.ID,
			CatalogVersion: catalogVersion,
		}
	}

	var reasonCodes []string
	if req.Preferences.MaxBudgetMinor != nil {
		reasonCodes = append(reasonCodes, "matches_budget")
	}
	if req.Preferences.Purpose == "portable_work" {
		reasonCodes = append(reasonCodes, "matches_declared_portability")
	} else if req.Preferences.Purpose == "performance" {
		reasonCodes = append(reasonCodes, "matches_declared_performance")
	}
	if len(reasonCodes) == 0 {
		reasonCodes = append(reasonCodes, "data_available")
	}

	productStrip := contracts.PlanSection{
		SectionID:   "adapted_shortlist",
		Kind:        "product-strip",
		SlotID:      slotID,
		Priority:    20,
		Items:       items,
		ReasonCodes: reasonCodes,
		Config: &contracts.SectionConfig{
			LabelKey: "recommended_laptops",
		},
	}

	return &contracts.ExperiencePlan{
		ContractVersion: "1.0",
		RequestID:       req.RequestID,
		PlanID:          c.idGen("pl"),
		TenantID:        req.TenantID,
		Status:          "adapted",
		Locale:          req.Locale,
		CatalogVersion:  catalogVersion,
		PolicyVersion:   "pol_demo_v1",
		ExpiresAt:       c.clock().UTC().Add(5 * time.Minute).Format(time.RFC3339),
		Sections:        []contracts.PlanSection{intentPicker, productStrip},
		Provenance: contracts.PlanProvenance{
			Strategy:       "deterministic",
			ModelUsed:      nil,
			Plugins:        []string{"ibj.preference-ranker"},
			FallbackReason: nil,
		},
	}, nil
}

func (c *Composer) buildBaselinePlan(req *contracts.ComposeRequest, catalogVersion string, fallbackReason *string) *contracts.ExperiencePlan {
	return &contracts.ExperiencePlan{
		ContractVersion: "1.0",
		RequestID:       req.RequestID,
		PlanID:          c.idGen("pl"),
		TenantID:        req.TenantID,
		Status:          "baseline",
		Locale:          req.Locale,
		CatalogVersion:  catalogVersion,
		PolicyVersion:   "pol_demo_v1",
		ExpiresAt:       c.clock().UTC().Add(5 * time.Minute).Format(time.RFC3339),
		Sections:        []contracts.PlanSection{}, // Empty sections for baseline plan
		Provenance: contracts.PlanProvenance{
			Strategy:       "merchant_baseline",
			ModelUsed:      nil,
			Plugins:        []string{},
			FallbackReason: fallbackReason,
		},
	}
}

func (c *Composer) buildFallbackBaseline(req *contracts.ComposeRequest, catalogVersion string, reason string) *contracts.ExperiencePlan {
	return c.buildBaselinePlan(req, catalogVersion, &reason)
}
