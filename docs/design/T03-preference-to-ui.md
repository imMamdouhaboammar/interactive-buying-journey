---
doc_id: IBJ-CODE-0031
title: "T03 Buyer Input-to-UI Tracer Design Specification"
lifecycle: transient
status: active
visibility: public
owner: Mamdouh Aboammar
last_reviewed: 2026-09-27
review_by: 2027-03-27
expires_when: "T03 PR merged"
superseded_by: null
archived_on: null
archive_reason: null
---
# T03: Buyer Input-to-UI Tracer Design Specification

> **Task ID:** T03 (Milestone M0 / P0)  
> **Preceding Task:** T02 Catalog Ingest & Baseline Tracer  
> **Downstream Blocked Tasks:** T04, T05, T06, T07, T08, T09, T17  
> **Architectural Decisions:** [ADR-0006](../decisions/0006-t03-contract-gaps.md)

---

## 1. Scope & Non-Goals

### In Scope
1. **Buyer Preference Input:** One-tap interactive chips on the collection page for purpose (`portable_work`, `performance`, `everyday_value`) and budget (`max_budget_minor`), plus a clear/reset control.
2. **Context Request Contract:** Client SDK sends structured preferences conforming to `compose-request.schema.json`.
3. **Exact Constraint Filtering:** Strict server-side filtering on:
   - Tenant isolation & allowed slots.
   - Catalog freshness (non-stale `last_verified_at` using injected Clock).
   - In-stock inventory status (`inventory_status = 'in_stock'`).
   - Hard upper budget limit (`price_amount_minor <= max_budget_minor`).
4. **Deterministic Ranking Engine (`rank_v1`):**
   - Untrained, verifiable heuristic combining lexical relevance, explicit attribute matches, merchant business priority, and data completeness.
   - Strict deterministic tie-breaking: `(score DESC, price_minor ASC, variant_id ASC)`.
5. **Experience Plan Synthesis:**
   - Status `adapted` with allowlisted `intent-picker` and `product-strip` sections when matches exist.
   - Status `empty` with `intent-picker` and `empty-state` section when no candidate matches budget/stock.
   - Status `baseline` when no preferences are set or when fail-open is triggered.
   - Reason codes: strictly compliant with schema enum (`matches_budget`, `matches_declared_portability`, `matches_declared_performance`, `data_available`).
6. **Storefront SDK & Presentation:**
   - CSP-safe DOM manipulation (no `innerHTML` on untrusted inputs, text setters only).
   - Focus retention across re-renderings.
   - Bi-directional parity: native RTL for Arabic (`ar`) and LTR for English (`en`).
   - Mobile (375px) and desktop (1440px) visual stability.
   - Non-consent path: executes cleanly using only necessary in-memory session preferences without persistent tracking (J-01, J-05).

### Non-Goals
1. Machine-learning models or LLM inference (Kev, Jev, or Laya are strictly deferred or optional downstream in T05).
2. Streaming or chunked HTTP responses (P0 uses single atomic HTTP POST/JSON).
3. Persistent profiling or cross-session tracking (reserved for consented returning buyers in P1).
4. Automatic add-to-cart or cart mutations (commerce authority remains with merchant).

---

## 2. Request and Data Flow

```
[Buyer Interaction]
       │
       ▼ (Clicks Chip: "Portable Work" / "$1,200")
[SDK State Machine] (ephemeral in-memory state)
       │
       ▼ (Abort previous inflight fetch; POST /v1/journeys/compose)
[HTTP API Router]
       │
       ▼ (Validate ComposeRequest against JSON Schema)
[Composer Engine]
       │
       ├─► [Policy Checker] (Tenant valid? Slots allowed? Kill switch off?)
       ├─► [Catalog Port] (Query candidate variants: tenant, category, freshness, stock, budget)
       ├─► [Intent Rules Port] (Fetch merchant intent rules for category and purpose)
       ├─► [Ranker rank_v1] (Score and rank candidates deterministically)
       └─► [Plan Builder] (Construct ExperiencePlan with sections and reason codes)
       │
       ▼ (Self-validate ExperiencePlan against JSON Schema)
[HTTP API Response] (200 OK with ExperiencePlan)
       │
       ▼
[SDK Client Validator] (Validate ExperiencePlan schema on client)
       │
       ▼
[SDK Renderer] (Focus-preserving DOM patch into approved slot)
       │
       ▼
[Storefront UI Updated] (Product strip adapts; baseline grid remains unchanged)
```

---

## 3. Merchant Intent Rules Table

To support configurable merchant intent mapping without modifying Go source code, rules are stored in PostgreSQL:

```sql
CREATE TABLE IF NOT EXISTS merchant_intent_rules (
    rule_id VARCHAR(64) PRIMARY KEY,
    tenant_id VARCHAR(128) NOT NULL REFERENCES tenants(tenant_id) ON DELETE CASCADE,
    category_id VARCHAR(64) NOT NULL,
    intent_key VARCHAR(64) NOT NULL,
    label_en VARCHAR(128) NOT NULL,
    label_ar VARCHAR(128) NOT NULL,
    max_weight_grams INT,
    min_battery_hours NUMERIC(4, 1),
    min_ram_gb INT,
    weight_explicit NUMERIC(3, 2) NOT NULL DEFAULT 1.0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_tenant_category_intent UNIQUE (tenant_id, category_id, intent_key)
);
```

### Laptop Category Intent Definitions (`demo_store`):
| Intent Key | Label (EN) | Label (AR) | Constraints / Preference Targets | Reason Code |
| :--- | :--- | :--- | :--- | :--- |
| `portable_work` | Portable Work | عمل متنقل | `max_weight_grams <= 1500`, `min_battery_hours >= 8.0` | `matches_declared_portability` |
| `performance` | Performance | أداء عالي | `min_ram_gb >= 16`, high processor spec | `matches_declared_performance` |
| `everyday_value` | Everyday Value | استخدام يومي اقتصادي | Balanced specs, competitive pricing | `matches_budget` |

---

## 4. Deterministic Ranking Math (`rank_v1`)

The ranking algorithm operates strictly on candidates that have passed all hard gates (tenant match, freshness <= staleness threshold, inventory in_stock, price <= max_budget_minor).

### Formula:
$$\text{Score}(v) = w_{\text{lexical}} \cdot S_{\text{lex}} + w_{\text{explicit}} \cdot S_{\text{exp}} + w_{\text{business}} \cdot S_{\text{bus}} + w_{\text{quality}} \cdot S_{\text{qual}}$$

Where:
- $w_{\text{lexical}} = 0.20$
- $w_{\text{explicit}} = 0.50$ (intent match component)
- $w_{\text{business}} = 0.15$ (merchant organic boost)
- $w_{\text{quality}} = 0.15$ (data completeness ratio: known non-null attributes / total attributes)

### Deterministic Tie-Breaking Order:
1. `Score` descending (highest score first)
2. `PriceMinor` ascending (lower price first)
3. `VariantID` lexicographically ascending (ensures strict permutation invariance)

---

## 5. Section & Reason Code Rules

1. **`intent-picker` Section:**
   - Slot: `collection_top` (or first approved slot).
   - Kind: `intent-picker`.
   - Items: `[]` (empty array).
   - Reason Codes: `["data_available"]`.
   - Config: Includes options for `portable_work`, `performance`, `everyday_value`, `not_sure`.
2. **`product-strip` Section (when candidates exist):**
   - Slot: `collection_top`.
   - Kind: `product-strip`.
   - Items: Up to 4 top-ranked variants with `variant_id` and `catalog_version`.
   - Reason Codes: Computed from preference matches:
     - `matches_budget` (if buyer set budget and variant satisfies it).
     - `matches_declared_portability` (if `portable_work` was selected and variant meets weight/battery criteria).
     - `matches_declared_performance` (if `performance` was selected).
3. **`empty-state` Section (when zero candidates match):**
   - Slot: `collection_top`.
   - Kind: `empty-state`.
   - Items: `[]`.
   - Reason Codes: `["matches_budget"]` (or `["insufficient_evidence"]`).
   - Config: Includes reset action label and explanatory message.

---

## 6. SDK State Machine & Focus Guard

```
               ┌───────────────┐
               │    INITIAL    │
               └───────┬───────┘
                       │ mount()
                       ▼
               ┌───────────────┐
       ┌──────►│  INTERACTIVE  │◄──────┐
       │       └───────┬───────┘       │
       │               │ chip click    │
       │               ▼               │
       │       ┌───────────────┐       │
       │       │ CAPTURE FOCUS │       │
       │       └───────┬───────┘       │
       │               │ fetch()       │
       │               ▼               │
       │       ┌───────────────┐       │
       │       │ AWAIT / ABORT │       │
       │       └───────┬───────┘       │
       │               │ plan ready    │
       │               ▼               │
       │       ┌───────────────┐       │
       │       │  RENDER DOM   │       │
       │       └───────┬───────┘       │
       │               │               │
       │               ▼               │
       │       ┌───────────────┐       │
       └───────┤ RESTORE FOCUS ├───────┘
               └───────────────┘
```

1. **Focus Capture:** Active element selector/ID and scroll position are recorded before DOM updates.
2. **Focus Restoration:** Focus is restored to the clicked chip or next logical sibling so screen reader and keyboard users do not lose context.
3. **Aria Announcements:** `aria-live="polite"` container announces result count changes (e.g. "Showing 3 recommendations for Portable Work").

---

## 7. Merchant Truth Resolution

Per FR-005 and ADR-0003, the engine provides variant IDs and reason codes, but the merchant storefront holds authoritative presentation truth:
- If a recommended variant in `ExperiencePlan.items` is not present in the merchant's live client catalog (e.g. removed or unavailable), the client renderer omits that specific card.
- If all recommended cards are missing or out of stock, the SDK falls back open and hides the strip without disturbing the merchant collection grid.

---

## 8. Test Plan Matrix

| Test ID | Area | Scenario | Expected Behavior |
| :--- | :--- | :--- | :--- |
| `TC-PREF-01` | Backend | Parse and validate compose request with valid preferences | Valid request parses successfully into domain preferences struct |
| `TC-PREF-02` | Backend | Reject negative budget or invalid purpose length | Schema validation fails with 400 Bad Request |
| `TC-RANK-01` | Backend | Budget filter excludes variants exceeding `max_budget_minor` | Zero variants above budget in candidate list |
| `TC-RANK-02` | Backend | Out of stock variants excluded even if within budget | Zero variants with `inventory_status != 'in_stock'` |
| `TC-RANK-03` | Backend | `rank_v1` orders by score, price, and variant_id tie-breaker | Deterministic ordering verified across permutations |
| `TC-RANK-04` | Backend | Zero matches emits `empty-state` section and status `empty` | Plan contains `empty-state` section with reason code `matches_budget` |
| `TC-RANK-05` | Backend | Valid matches emit `product-strip` with accurate reason codes | Plan status `adapted`, includes `matches_declared_portability` |
| `TC-SDK-01` | SDK | Render intent chips above collection | Chips mount cleanly in `collection_top` slot |
| `TC-SDK-02` | SDK | Clicking chip updates plan without full page reload | Fetch dispatched with updated preference payload |
| `TC-SDK-03` | SDK | Focus retained on active chip after render | `document.activeElement` matches clicked chip |
| `TC-SDK-04` | SDK | Bi-directional parity: English LTR and Arabic RTL | Verified layout direction, Arabic labels, numerals preserved |
| `TC-SDK-05` | SDK | Rapid clicks abort previous request | Inflight request aborted via `AbortController` |
| `TC-SDK-06` | SDK | Fail-open on 500 error or network outage | Merchant baseline grid intact, no error dialogs |
| `TC-SDK-07` | SDK | Axe accessibility audit passes with 0 violations | WCAG 2.1 AA compliant across both locales |
