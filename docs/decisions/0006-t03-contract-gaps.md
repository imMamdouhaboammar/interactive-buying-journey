---
doc_id: IBJ-CODE-0030
title: "ADR-0006: T03 Contract Schema Observations, Intent Rules, and Client State"
lifecycle: durable
status: active
visibility: public
owner: Mamdouh Aboammar
last_reviewed: 2026-09-27
review_by: 2027-03-27
expires_when: null
superseded_by: null
archived_on: null
archive_reason: null
---
# ADR-0006: T03 Contract Schema Observations, Intent Rules, and Client State

- **Status:** APPROVED (T03 Implementation Gate)
- **Date:** 2026-09-27
- **Context:** Slice 3 (T03 "Buyer input-to-UI tracer") connects buyer preference input (purpose, budget) to an adapted storefront experience (`product-strip` or `empty-state`) while preserving fail-open baseline resilience and bi-directional layout parity.

---

## 1. Vendored Contract Boundaries and Observations

Per repository constitution, `contracts/schemas/` are read-only vendored artifacts synchronized exclusively from `spec/` via `tools/sync_contracts.py`. Implementation code must conform strictly to these schemas without local modifications:

1. **Section Requirements Across Diverse Kinds:**
   - In `experience-plan.schema.json`, all sections require the fields `["section_id", "kind", "slot_id", "priority", "items", "reason_codes"]`.
   - For non-product sections (`intent-picker`, `empty-state`), `items` must be populated as an empty array `[]` (`minItems` is not set; `maxItems: 12`).
   - `reason_codes` requires `minItems: 1`. Sections of kind `intent-picker` must include an approved reason code such as `data_available` or `merchant_editorial`.
2. **Reason Code Vocabulary Conformance:**
   - The contract schema strictly enumerates: `matches_budget`, `matches_declared_portability`, `matches_declared_performance`, `compatible_accessory`, `data_available`, `merchant_editorial`, `insufficient_evidence`, `facet_variance`.
   - Domain logic must emit only members of this enum. For purpose-based ranking:
     - `portable_work` maps to `matches_declared_portability`.
     - `performance` maps to `matches_declared_performance`.
     - Budget matching maps to `matches_budget`.
3. **Plan Status Lifecycle:**
   - When eligible candidate variants match the buyer's preferences, status is `"adapted"`.
   - When no variants satisfy hard constraints (budget, stock, freshness), status is `"empty"`, emitting an `empty-state` section.
   - When no preferences are supplied, adaptation is disabled by kill switch, or an engine error occurs, status is `"baseline"`.

---

## 2. Intent Rules Architecture & Storage

To prevent hardcoding merchant domain logic in application binaries:
- Tenant intent rules are persisted in PostgreSQL in the `merchant_intent_rules` table.
- Each rule maps `(tenant_id, category_id, intent_key)` to attribute weights, upper thresholds (e.g. max weight in grams), lower thresholds (e.g. min battery watt-hours), and localized option labels (`en`, `ar`).
- RLS is strictly enforced: `merchant_intent_rules` requires `FORCE ROW LEVEL SECURITY` with `app.current_tenant` scoping.

---

## 3. Client State Machine and Focus Preservation

1. **State Isolation:**
   - Client preferences are stored in ephemeral client memory within the page session. No persistent cookies or local storage are created without explicit consent (J-01, J-05).
2. **Focus Retention:**
   - When a buyer clicks a preference chip or adjusts budget, the active element's identity and focus state are captured before DOM patch application and restored immediately after rendering.
3. **Sequence Abort Controller:**
   - Rapid sequential chip selections abort preceding inflight fetch requests via `AbortController` to prevent stale responses from overwriting newer user selections.
