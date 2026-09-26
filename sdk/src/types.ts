// Copyright (c) 2026 Mamdouh Aboammar
// SPDX-License-Identifier: PolyForm-Shield-1.0.0

export type Locale = "en" | "ar";
export type PageKind = "home" | "collection" | "pdp" | "cart";
export type PlanStatus = "adapted" | "baseline" | "empty";
export type ProvenanceStrategy = "deterministic" | "deterministic_plus_semantic" | "merchant_baseline";

export interface IBJClientOptions {
  tenantKey: string;
  endpoint: string;
  timeoutMs?: number;
}

export interface ComposeParams {
  requestId: string;
  sessionToken: string;
  locale: Locale;
  currency?: string;
  page: {
    kind: PageKind;
    categoryId?: string | null;
    productId?: string | null;
  };
  allowedSlots: string[];
  consent?: {
    version: string;
    purposes: ("necessary" | "analytics" | "personalization" | "external_ai")[];
  };
  preferences?: {
    purpose?: string;
    maxBudgetMinor?: number;
    minBatteryHours?: number;
    maxWeightGrams?: number;
    brandIds?: string[];
  };
}

export interface PlanItem {
  variant_id: string;
  offer_id?: string | null;
  catalog_version?: string;
}

export interface PlanSection {
  section_id: string;
  kind:
    | "intent-picker"
    | "product-strip"
    | "comparison-table"
    | "accessory-strip"
    | "explanation"
    | "empty-state"
    | "facet-panel";
  slot_id: string;
  priority: number;
  items: PlanItem[];
  reason_codes: string[];
  config?: Record<string, unknown>;
}

export interface ExperiencePlan {
  contract_version: "1.0";
  request_id: string;
  plan_id: string;
  tenant_id: string;
  status: PlanStatus;
  locale: Locale;
  catalog_version: string;
  policy_version: string;
  expires_at: string;
  experiment_arm?: string | null;
  sections: PlanSection[];
  provenance: {
    strategy: ProvenanceStrategy;
    model_used: string | null;
    plugins: string[];
    fallback_reason?: string | null;
  };
}
