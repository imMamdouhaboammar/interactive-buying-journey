// Copyright (c) 2026 Mamdouh Aboammar
// SPDX-License-Identifier: PolyForm-Shield-1.0.0

package contracts

// ComposeRequest represents a client request to synthesize an adaptive journey plan.
type ComposeRequest struct {
	ContractVersion string             `json:"contract_version"`
	RequestID       string             `json:"request_id"`
	TenantID        string             `json:"tenant_id"`
	SessionToken    string             `json:"session_token"`
	Page            PageContext        `json:"page"`
	Locale          string             `json:"locale"`
	Currency        string             `json:"currency,omitempty"`
	Consent         ConsentContext     `json:"consent"`
	Preferences     PreferencesContext `json:"preferences"`
	AllowedSlots    []string           `json:"allowed_slots"`
	PageEvents      []string           `json:"page_events,omitempty"`
}

// PageContext defines the current storefront page boundary.
type PageContext struct {
	Kind       string  `json:"kind"`
	CategoryID *string `json:"category_id"`
	ProductID  *string `json:"product_id,omitempty"`
}

// ConsentContext defines lawful processing purposes granted for this session.
type ConsentContext struct {
	Version  string   `json:"version"`
	Purposes []string `json:"purposes"`
}

// PreferencesContext captures buyer-declared intent and constraints.
type PreferencesContext struct {
	Purpose         string   `json:"purpose,omitempty"`
	MaxBudgetMinor  *int64   `json:"max_budget_minor,omitempty"`
	MinBatteryHours *float64 `json:"min_battery_hours,omitempty"`
	MaxWeightGrams  *int     `json:"max_weight_grams,omitempty"`
	BrandIDs        []string `json:"brand_ids,omitempty"`
}

// ExperiencePlan defines the validated storefront adaptation plan.
type ExperiencePlan struct {
	ContractVersion string         `json:"contract_version"`
	RequestID       string         `json:"request_id"`
	PlanID          string         `json:"plan_id"`
	TenantID        string         `json:"tenant_id"`
	Status          string         `json:"status"` // "adapted", "baseline", "empty"
	Locale          string         `json:"locale"`
	CatalogVersion  string         `json:"catalog_version"`
	PolicyVersion   string         `json:"policy_version"`
	ExpiresAt       string         `json:"expires_at"`
	ExperimentArm   *string        `json:"experiment_arm,omitempty"`
	Sections        []PlanSection  `json:"sections"`
	Provenance      PlanProvenance `json:"provenance"`
}

// PlanSection defines one typed section mounted into a designated slot.
type PlanSection struct {
	SectionID   string         `json:"section_id"`
	Kind        string         `json:"kind"`
	SlotID      string         `json:"slot_id"`
	Priority    int            `json:"priority"`
	Items       []PlanItem     `json:"items"`
	ReasonCodes []string       `json:"reason_codes"`
	Config      *SectionConfig `json:"config,omitempty"`
}

// PlanItem references an eligible product variant.
type PlanItem struct {
	VariantID      string  `json:"variant_id"`
	OfferID        *string `json:"offer_id,omitempty"`
	CatalogVersion string  `json:"catalog_version,omitempty"`
}

// SectionConfig carries typed component parameters.
type SectionConfig struct {
	LabelKey              string         `json:"label_key,omitempty"`
	Options               []ConfigOption `json:"options,omitempty"`
	AttributeKeys         []string       `json:"attribute_keys,omitempty"`
	FacetKeys             []string       `json:"facet_keys,omitempty"`
	CollapsedFacetKeys    []string       `json:"collapsed_facet_keys,omitempty"`
	PreserveAppliedFacets *bool          `json:"preserve_applied_facets,omitempty"`
}

// ConfigOption defines one selectable choice in an intent picker.
type ConfigOption struct {
	ID       string `json:"id"`
	LabelKey string `json:"label_key"`
}

// PlanProvenance records how the experience plan was formed.
type PlanProvenance struct {
	Strategy       string   `json:"strategy"` // "deterministic", "deterministic_plus_semantic", "merchant_baseline"
	ModelUsed      *string  `json:"model_used"`
	Plugins        []string `json:"plugins"`
	FallbackReason *string  `json:"fallback_reason,omitempty"`
}
