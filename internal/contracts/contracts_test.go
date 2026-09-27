// Copyright (c) 2026 Mamdouh Aboammar
// SPDX-License-Identifier: PolyForm-Shield-1.0.0

package contracts_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/imMamdouhaboammar/interactive-buying-journey/internal/contracts"
)

func TestCanonicalExamplesValidateAgainstSchemas(t *testing.T) {
	validator, err := contracts.NewValidator("../../contracts/schemas")
	if err != nil {
		t.Fatalf("failed to initialize schema validator: %v", err)
	}

	examples := map[string]string{
		"catalog-batch.json":           "catalog-batch.schema.json",
		"compose-request.json":         "compose-request.schema.json",
		"decision-envelope.json":       "decision-envelope.schema.json",
		"enrichment-compatibility.json": "enrichment-record.schema.json",
		"enrichment-record.json":       "enrichment-record.schema.json",
		"enrichment-review.json":       "enrichment-review.schema.json",
		"event-envelope.json":          "event-envelope.schema.json",
		"experience-plan.json":         "experience-plan.schema.json",
	}

	for exampleFile, schemaFile := range examples {
		t.Run(exampleFile, func(t *testing.T) {
			path := filepath.Join("../../contracts/examples", exampleFile)
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("failed to read example file %s: %v", path, err)
			}

			if err := validator.Validate(schemaFile, data); err != nil {
				t.Errorf("example %s failed validation against %s: %v", exampleFile, schemaFile, err)
			}
		})
	}
}

func TestNegativeFixturesFailValidation(t *testing.T) {
	validator, err := contracts.NewValidator("../../contracts/schemas")
	if err != nil {
		t.Fatalf("failed to initialize schema validator: %v", err)
	}

	rawPlan, err := os.ReadFile("../../contracts/examples/experience-plan.json")
	if err != nil {
		t.Fatalf("failed to read experience-plan.json: %v", err)
	}

	t.Run("empty reason_codes rejected", func(t *testing.T) {
		var plan map[string]any
		if err := json.Unmarshal(rawPlan, &plan); err != nil {
			t.Fatalf("unmarshal error: %v", err)
		}
		sections := plan["sections"].([]any)
		sec0 := sections[0].(map[string]any)
		sec0["reason_codes"] = []any{}

		mutated, _ := json.Marshal(plan)
		if err := validator.Validate("experience-plan.schema.json", mutated); err == nil {
			t.Error("expected error for empty reason_codes, got nil")
		}
	})

	t.Run("unknown section kind rejected", func(t *testing.T) {
		var plan map[string]any
		if err := json.Unmarshal(rawPlan, &plan); err != nil {
			t.Fatalf("unmarshal error: %v", err)
		}
		sections := plan["sections"].([]any)
		sec0 := sections[0].(map[string]any)
		sec0["kind"] = "arbitrary-banner"

		mutated, _ := json.Marshal(plan)
		if err := validator.Validate("experience-plan.schema.json", mutated); err == nil {
			t.Error("expected error for unknown section kind, got nil")
		}
	})

	t.Run("facet-panel carrying items rejected", func(t *testing.T) {
		var plan map[string]any
		if err := json.Unmarshal(rawPlan, &plan); err != nil {
			t.Fatalf("unmarshal error: %v", err)
		}
		facetSec := map[string]any{
			"section_id":   "sec_facets_01",
			"kind":         "facet-panel",
			"slot_id":      "collection_facets",
			"priority":     10,
			"items":        []any{map[string]any{"variant_id": "lap_001"}},
			"reason_codes": []any{"facet_variance"},
			"config": map[string]any{
				"facet_keys":             []any{"brand", "weight_g"},
				"preserve_applied_facets": true,
			},
		}
		plan["sections"] = []any{facetSec}

		mutated, _ := json.Marshal(plan)
		if err := validator.Validate("experience-plan.schema.json", mutated); err == nil {
			t.Error("expected error for facet-panel carrying items, got nil")
		}
	})

	t.Run("enrichment status approved with pending review rejected", func(t *testing.T) {
		rawEnrichment, err := os.ReadFile("../../contracts/examples/enrichment-record.json")
		if err != nil {
			t.Fatalf("failed to read enrichment-record.json: %v", err)
		}
		var record map[string]any
		if err := json.Unmarshal(rawEnrichment, &record); err != nil {
			t.Fatalf("unmarshal error: %v", err)
		}
		record["status"] = "approved"
		record["review"] = map[string]any{
			"decision":    "pending",
			"reviewer_id": nil,
			"reviewed_at": nil,
			"note":        "Still awaiting manual review",
		}

		mutated, _ := json.Marshal(record)
		if err := validator.Validate("enrichment-record.schema.json", mutated); err == nil {
			t.Error("expected error for approved enrichment with pending review, got nil")
		}
	})
}

func TestPreferencesValidation(t *testing.T) {
	validator, err := contracts.NewValidator("../../contracts/schemas")
	if err != nil {
		t.Fatalf("failed to initialize schema validator: %v", err)
	}

	baseRequestRaw, err := os.ReadFile("../../contracts/examples/compose-request.json")
	if err != nil {
		t.Fatalf("failed to read compose-request.json: %v", err)
	}

	t.Run("TC-PREF-01: valid preferences pass schema validation and deserialize", func(t *testing.T) {
		var reqMap map[string]any
		if err := json.Unmarshal(baseRequestRaw, &reqMap); err != nil {
			t.Fatalf("unmarshal error: %v", err)
		}
		reqMap["preferences"] = map[string]any{
			"purpose":           "portable_work",
			"max_budget_minor":  120000,
			"min_battery_hours": 8.5,
			"max_weight_grams":  1500,
			"brand_ids":         []string{"brand_demo"},
		}

		payload, err := json.Marshal(reqMap)
		if err != nil {
			t.Fatalf("marshal error: %v", err)
		}

		if err := validator.Validate("compose-request.schema.json", payload); err != nil {
			t.Fatalf("valid preferences failed schema validation: %v", err)
		}

		var req contracts.ComposeRequest
		if err := json.Unmarshal(payload, &req); err != nil {
			t.Fatalf("failed deserializing into ComposeRequest: %v", err)
		}
		if req.Preferences.Purpose != "portable_work" {
			t.Errorf("expected purpose 'portable_work', got %q", req.Preferences.Purpose)
		}
		if req.Preferences.MaxBudgetMinor == nil || *req.Preferences.MaxBudgetMinor != 120000 {
			t.Errorf("expected max_budget_minor 120000, got %v", req.Preferences.MaxBudgetMinor)
		}
	})

	t.Run("TC-PREF-02: invalid preferences rejected by schema", func(t *testing.T) {
		cases := []struct {
			name   string
			mutate func(p map[string]any)
		}{
			{
				name: "negative budget rejected",
				mutate: func(p map[string]any) {
					p["max_budget_minor"] = -500
				},
			},
			{
				name: "negative battery rejected",
				mutate: func(p map[string]any) {
					p["min_battery_hours"] = -2.0
				},
			},
			{
				name: "zero weight rejected",
				mutate: func(p map[string]any) {
					p["max_weight_grams"] = 0
				},
			},
			{
				name: "too many brand IDs rejected",
				mutate: func(p map[string]any) {
					p["brand_ids"] = []string{"b1", "b2", "b3", "b4", "b5", "b6", "b7", "b8", "b9", "b10", "b11"}
				},
			},
		}

		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				var reqMap map[string]any
				if err := json.Unmarshal(baseRequestRaw, &reqMap); err != nil {
					t.Fatalf("unmarshal error: %v", err)
				}
				prefs := map[string]any{
					"purpose": "portable_work",
				}
				tc.mutate(prefs)
				reqMap["preferences"] = prefs

				payload, _ := json.Marshal(reqMap)
				if err := validator.Validate("compose-request.schema.json", payload); err == nil {
					t.Errorf("expected validation failure for %s, but passed", tc.name)
				}
			})
		}
	})
}
