# T02 Design Specification: Catalog-to-Baseline Ingest Tracer

- **Feature:** Slice 2 = T02 "Catalog-to-baseline tracer"
- **Status:** APPROVED DESIGN (Pre-Implementation Gate)
- **Author:** Senior Backend Engineer (IBJ Engine)
- **Date:** 2026-09-27
- **Target Specification Commit:** `d772e05` (`interactive-buying-journey-spec`)
- **Vendored Contracts Commit:** `6d473fa` (byte-identical `contracts/` with `d772e05`)

---

## 1. Scope and Non-Goals

### 1.1 In-Scope (T02 Deliverables)
1. **Mock Feed Ingestion Endpoint (`POST /catalog/batches`):**
   - Implements the OpenAPI 3.1 specification for `/catalog/batches`.
   - HMAC-SHA256 signature verification over headers `X-IBJ-Tenant`, `X-IBJ-Timestamp`, and `X-IBJ-Signature: v1=<hex>` with a 300-second timestamp tolerance and support for dual active secrets (current and previous) for zero-downtime rotation.
   - Strict transport validation: 1MB payload ceiling (413), JSON content-type enforcement (415), schema validation against `catalog-batch.schema.json` Draft 2020-12, duplicate key rejection, and canonical attribute validation.
2. **Feed State Machine:**
   - Full progression: `RECEIVED -> AUTHENTICATED -> VALIDATED -> APPLIED -> INDEXED -> ACTIVE`.
   - Terminal failure state: `QUARANTINED` with item-level error reporting, preserving the prior active catalog version untouched.
   - Variant-level deletion state: `TOMBSTONED`.
3. **Storage & Multi-Tenant Isolation:**
   - PostgreSQL persistence utilizing composite `(tenant_id, id)` primary and foreign keys.
   - Row-Level Security (`FORCE ROW LEVEL SECURITY`) with an application role `ibj_app` lacking `BYPASSRLS`.
   - Per-transaction tenant context binding via `SET LOCAL app.current_tenant = $1` where a missing or unset context returns zero rows.
   - Per-tenant transaction serialization via advisory locks (`pg_advisory_xact_lock`) ensuring arrival-order independence.
4. **Catalog Projection & SearchPort:**
   - `CatalogPort` and `SearchPort` implemented on top of PostgreSQL.
   - Deterministic eligibility gating: tenant isolation, `published = true`, `inventory_status = 'in_stock'`, `is_tombstoned = false`, matching currency, and `last_verified_at` within configured staleness limits.
   - PostgreSQL Full-Text Search (FTS) with English stemming, Arabic normalization (tashkeel/tatweel stripping, alef unification, yaa/taa marbuta folding), deterministic tie-breaking (relevance, product ID, variant ID), and a strict candidate cap of 200.
5. **Feed Sender CLI (`cmd/ibj-feed`):**
   - Developer and E2E CLI tool to sign and dispatch versioned fixture batches.
6. **Journey Orchestrator (`compose` integration):**
   - Minimal update: `CatalogVersion` in `ExperiencePlan` tracks the real active projection version from `CatalogPort`, falling back cleanly to `"unavailable"` with `fallback_reason: "catalog_unavailable"` when the catalog store is unreachable.

### 1.2 Non-Goals (Explicitly Deferred)
- **T03 Scope:** Preference filtering, intent classification, dynamic ranking boosts, and adaptive UI slot rendering (`product-strip`).
- **T04 Scope:** Buyer consent capture, consent withdrawal cascades, session event persistence.
- **T05 Scope:** Third-party model providers (Laya, Kev, Jev), LLM sidecars, semantic vector inference.
- **T16 Scope:** Offline catalog enrichment, merchant review queues, proposal approval flows.
- **T17 Scope:** Adaptive User Interface (AUI) layout mutations, cognitive overload detection, client idle queue scheduling.
- **Platform Connectors:** Live Shopify or WooCommerce GraphQL/REST adapters, webhook listener daemons, platform cart mutations.
- **Distributed Infrastructure:** Redis caching, JetStream/Kafka queues, distributed workers.
- **Public API Expansions:** No new public endpoints beyond `POST /catalog/batches`.
- **Contracts:** Zero modifications to vendored schemas in `contracts/`.

---

## 2. Feed State Machine

The feed lifecycle manages batch ingress, cryptographic authentication, schema validation, atomic transactional application, search indexing, and projection activation.

```
       +--------------------+
       |  HTTP POST Ingress |
       +---------+----------+
                 |
                 v
       +--------------------+
       |      RECEIVED      | (HTTP 202 Accepted returned)
       +---------+----------+
                 |
                 | [Trigger: Ingest Worker]
                 | [Guard: Verify HMAC & Timestamp skew <= 300s]
                 v
       +--------------------+          Guard Failed
       |   AUTHENTICATED    | -------------------------------+
       +---------+----------+                                |
                 |                                           |
                 | [Trigger: Validate Payload]               |
                 | [Guard: JSON Schema 2020-12,              |
                 |         attribute ranges, future skew]    |
                 v                                           v
       +--------------------+          Guard Failed    +-------------+
       |     VALIDATED      | -----------------------> | QUARANTINED |
       +---------+----------+                          +-------------+
                 |                                     (Prior ACTIVE
                 | [Trigger: Begin Tx + Advisory Lock]  version kept)
                 | [Guard: Evaluate Ingest Rules]
                 v
       +--------------------+
       |      APPLIED       | (Variants projected & tombstoned)
       +---------+----------+
                 |
                 | [Trigger: Update FTS TSVectors]
                 | [Guard: Text normalized en/ar]
                 v
       +--------------------+
       |      INDEXED       |
       +---------+----------+
                 |
                 | [Trigger: Commit Tx]
                 | [Guard: Catalog version updated]
                 v
       +--------------------+
       |       ACTIVE       |
       +--------------------+
```

### Transition Specifications

| State From | State To | Trigger | Guards & Invariants | Persisted State | API Response |
| :--- | :--- | :--- | :--- | :--- | :--- |
| `[INIT]` | `RECEIVED` | `POST /catalog/batches` | Body <= 1MB, valid JSON, non-empty, Content-Type `application/json`. Authenticated on transport before admission. | `feed_batches` record with `state = 'RECEIVED'`, `payload_hash = SHA256(body)`, raw JSON. | `202 Accepted` with `batch_id`, `state: "received"`. |
| `RECEIVED` | `AUTHENTICATED` | Worker pick-up | Headers `X-IBJ-Tenant`, `X-IBJ-Timestamp`, `X-IBJ-Signature` verified. Server clock skew <= 300s. Secret matches current or previous. Tenant matches body. | `feed_batches.state = 'AUTHENTICATED'`. | N/A (Async worker). |
| `AUTHENTICATED` | `VALIDATED` | Schema & semantics pass | Schema Draft 2020-12 valid, `maxItems <= 1000`, no duplicate variants in batch, no overlap between `upserts` and `deletes`, typed attributes within canonical bounds, `source_updated_at` future skew <= 300s. | `feed_batches.state = 'VALIDATED'`. | N/A (Async worker). |
| `AUTHENTICATED` / `VALIDATED` | `QUARANTINED` | Validation failure | Any invalid item, malformed attribute, duplicate variant ID within batch, future skew breach. Prior catalog projection remains completely untouched. | `feed_batches.state = 'QUARANTINED'`, `error_report = JSON({errors, item_index})`. | N/A (Async worker). |
| `VALIDATED` | `APPLIED` | Apply transaction | Tenant advisory lock `pg_advisory_xact_lock(hash(tenant_id))` acquired. Ingest rules evaluated per variant. Stale updates skipped, conflicts logged, tombstones written. | `variants` rows upserted or marked `is_tombstoned = true`. Counts recorded in `feed_batches`. | N/A (Async worker). |
| `APPLIED` | `INDEXED` | FTS generation | English stemming generated via `to_tsvector('english', ...)`; Arabic text normalized (tashkeel/tatweel stripped, alef unified) and indexed via `to_tsvector('arabic', ...)`. | `variants.tsv_en`, `variants.tsv_ar` columns updated. `feed_batches.state = 'INDEXED'`. | N/A (Async worker). |
| `INDEXED` | `ACTIVE` | Tx commit | Transaction commits cleanly. Previous active version marked `SUPERSEDED`. New active version registered. | `catalog_versions` row with `status = 'ACTIVE'`. `feed_batches.state = 'ACTIVE'`. | N/A (Async worker). |

---

## 3. Data Model and DDL (PostgreSQL)

All persistent tables adhere to composite keys `(tenant_id, id)` and strict Row-Level Security.

```sql
-- Schema migrations managed by goose
-- 00001_initial_catalog_schema.sql

-- 1. Tenants Table
CREATE TABLE IF NOT EXISTS tenants (
    tenant_id VARCHAR(64) PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    secret_current VARCHAR(255) NOT NULL,
    secret_previous VARCHAR(255),
    max_staleness_seconds INT NOT NULL DEFAULT 86400,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- 2. Feed Batches Table
CREATE TABLE IF NOT EXISTS feed_batches (
    tenant_id VARCHAR(64) NOT NULL REFERENCES tenants(tenant_id) ON DELETE CASCADE,
    batch_id VARCHAR(128) NOT NULL,
    source VARCHAR(128) NOT NULL,
    source_version VARCHAR(128) NOT NULL,
    payload_hash VARCHAR(64) NOT NULL,
    state VARCHAR(32) NOT NULL,
    raw_payload JSONB,
    stats_upserted INT NOT NULL DEFAULT 0,
    stats_stale INT NOT NULL DEFAULT 0,
    stats_conflicts INT NOT NULL DEFAULT 0,
    stats_tombstoned INT NOT NULL DEFAULT 0,
    error_report JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (tenant_id, batch_id)
);

CREATE INDEX IF NOT EXISTS idx_feed_batches_state ON feed_batches (tenant_id, state);

-- 3. Variants Projection Table
CREATE TABLE IF NOT EXISTS variants (
    tenant_id VARCHAR(64) NOT NULL REFERENCES tenants(tenant_id) ON DELETE CASCADE,
    variant_id VARCHAR(128) NOT NULL,
    product_id VARCHAR(128) NOT NULL,
    sku VARCHAR(128) NOT NULL,
    title TEXT NOT NULL,
    category VARCHAR(128) NOT NULL,
    brand VARCHAR(128),
    published BOOLEAN NOT NULL DEFAULT TRUE,
    currency VARCHAR(3) NOT NULL,
    price_minor BIGINT NOT NULL,
    inventory_status VARCHAR(32) NOT NULL,
    attributes JSONB NOT NULL DEFAULT '{}'::jsonb,
    source_updated_at TIMESTAMPTZ NOT NULL,
    last_verified_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    tombstoned_at TIMESTAMPTZ,
    is_tombstoned BOOLEAN NOT NULL DEFAULT FALSE,
    tsv_en TSVECTOR GENERATED ALWAYS AS (
        to_tsvector('english', coalesce(title, '') || ' ' || coalesce(brand, '') || ' ' || coalesce(category, ''))
    ) STORED,
    tsv_ar TSVECTOR,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (tenant_id, variant_id)
);

CREATE INDEX IF NOT EXISTS idx_variants_eligibility ON variants (
    tenant_id, category, published, inventory_status, is_tombstoned
);

CREATE INDEX IF NOT EXISTS idx_variants_tsv_en ON variants USING GIN (tsv_en);
CREATE INDEX IF NOT EXISTS idx_variants_tsv_ar ON variants USING GIN (tsv_ar);

-- 4. Catalog Versions Table
CREATE TABLE IF NOT EXISTS catalog_versions (
    tenant_id VARCHAR(64) NOT NULL REFERENCES tenants(tenant_id) ON DELETE CASCADE,
    version_id VARCHAR(128) NOT NULL,
    batch_id VARCHAR(128) NOT NULL,
    status VARCHAR(32) NOT NULL, -- 'ACTIVE', 'SUPERSEDED', 'FAILED'
    variant_count INT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (tenant_id, version_id)
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_catalog_versions_active ON catalog_versions (tenant_id) WHERE status = 'ACTIVE';
```

---

## 4. Multi-Tenant Isolation and Row-Level Security

### 4.1 RLS Configuration
All tables enforce Row-Level Security:
```sql
ALTER TABLE tenants ENABLE ROW LEVEL SECURITY;
ALTER TABLE feed_batches ENABLE ROW LEVEL SECURITY;
ALTER TABLE variants ENABLE ROW LEVEL SECURITY;
ALTER TABLE catalog_versions ENABLE ROW LEVEL SECURITY;

ALTER TABLE tenants FORCE ROW LEVEL SECURITY;
ALTER TABLE feed_batches FORCE ROW LEVEL SECURITY;
ALTER TABLE variants FORCE ROW LEVEL SECURITY;
ALTER TABLE catalog_versions FORCE ROW LEVEL SECURITY;
```

### 4.2 Application Role and Policies
The application connects as a restricted non-superuser role `ibj_app` (or runtime user) that does not own tables and lacks `BYPASSRLS`.

```sql
CREATE POLICY tenant_isolation_feed_batches ON feed_batches
    FOR ALL
    USING (tenant_id = nullif(current_setting('app.current_tenant', true), ''))
    WITH CHECK (tenant_id = nullif(current_setting('app.current_tenant', true), ''));

CREATE POLICY tenant_isolation_variants ON variants
    FOR ALL
    USING (tenant_id = nullif(current_setting('app.current_tenant', true), ''))
    WITH CHECK (tenant_id = nullif(current_setting('app.current_tenant', true), ''));

CREATE POLICY tenant_isolation_catalog_versions ON catalog_versions
    FOR ALL
    USING (tenant_id = nullif(current_setting('app.current_tenant', true), ''))
    WITH CHECK (tenant_id = nullif(current_setting('app.current_tenant', true), ''));
```

### 4.3 Missing Tenant Context Invariant
`current_setting('app.current_tenant', true)` returns `NULL` when unset.
Because `nullif('', '')` returns `NULL`, any query without an explicitly set session context evaluates `tenant_id = NULL` -> `UNKNOWN` (falsy in SQL boolean logic).
**Guaranteed result:** A query executed without `SET LOCAL app.current_tenant` returns **zero rows**, never all rows.

---

## 5. Ingest and Ordering Decision Rules

| Situation | Action / Result | Batch State | State Changes |
| :--- | :--- | :--- | :--- |
| **Same `batch_id`, identical payload hash** | Idempotent `202 Accepted`; no new version; no state change | Stays existing state | None |
| **Same `batch_id`, different payload** | Rejection with HTTP `409 Conflict`; no records written | None (rejected at gateway) | None |
| **Any invalid item in a batch** | Whole batch rejected; error report persisted; prior `ACTIVE` version stays active | `QUARANTINED` | `feed_batches` updated with error report; projection untouched |
| **Variant repeated inside one batch, or present in both `upserts` and `deletes`** | Batch quarantined with validation error | `QUARANTINED` | None to variants |
| **Upsert older than stored `source_updated_at`** | Ignored for that variant; counted as stale in batch report | Batch proceeds to `ACTIVE` | Variant unmodified; `stats_stale += 1` |
| **Upsert with equal timestamp and identical content** | No-op; treated as identical update | Batch proceeds to `ACTIVE` | `updated_at` refreshed |
| **Upsert with equal timestamp and different content** | Existing record kept; conflict recorded in batch report | Batch proceeds to `ACTIVE` | Variant unmodified; `stats_conflicts += 1` |
| **Delete** | Tombstone with `tombstoned_at` equal to latest `source_updated_at` in batch (or receive time if batch has no upserts) | Batch proceeds to `ACTIVE` | `is_tombstoned = true`, `tombstoned_at` set |
| **Upsert older than tombstone** | Stays deleted; counted as stale | Batch proceeds to `ACTIVE` | Variant remains tombstoned; `stats_stale += 1` |
| **Upsert newer than tombstone** | Variant is restored (`is_tombstoned = false`, `tombstoned_at = NULL`) | Batch proceeds to `ACTIVE` | Variant restored to active projection |
| **Delete of unknown variant** | Tombstone recorded in projection so late older upserts cannot recreate it | Batch proceeds to `ACTIVE` | Stub tombstone created; `stats_tombstoned += 1` |
| **`source_updated_at` skewed > 300s in future** | Item invalid; whole batch quarantined | `QUARANTINED` | Batch quarantined; projection untouched |
| **Crash or context cancel mid-apply** | Transaction rolled back; batch stays in pre-apply state for retry | Stays `RECEIVED` / `VALIDATED` | Clean transaction rollback; zero partial projections |
| **Two batches for same tenant concurrently** | Serialized per tenant using transaction advisory lock (`pg_advisory_xact_lock`) | Both complete in lock order | Final state is independent of arrival order |

---

## 6. Error Taxonomy

| Failure Mode | HTTP Status | Code | Batch State | Persisted Artifact |
| :--- | :--- | :--- | :--- | :--- |
| Missing auth header (`X-IBJ-Tenant`, etc.) | `401 Unauthorized` | `MISSING_AUTH_HEADERS` | None | None |
| Timestamp skewed > 300s (past or future) | `401 Unauthorized` | `TIMESTAMP_OUT_OF_WINDOW` | None | None |
| Invalid HMAC signature | `401 Unauthorized` | `INVALID_SIGNATURE` | None | None |
| Header tenant != body `tenant_id` | `403 Forbidden` | `TENANT_MISMATCH` | None | None |
| Unknown / inactive tenant | `403 Forbidden` | `UNKNOWN_TENANT` | None | None |
| Body size > 1MB | `413 Payload Too Large`| `PAYLOAD_TOO_LARGE` | None | None |
| Non-JSON content type | `415 Unsupported Media`| `UNSUPPORTED_MEDIA_TYPE` | None | None |
| Malformed / unparseable JSON | `400 Bad Request` | `MALFORMED_JSON` | None | None |
| Duplicate JSON keys | `400 Bad Request` | `DUPLICATE_JSON_KEYS` | None | None |
| Schema violation (`catalog-batch.schema.json`) | `400 Bad Request` | `SCHEMA_VALIDATION_ERROR` | `QUARANTINED` (if queued) | Error report in `feed_batches` |
| Canonical attribute range breach | `400 Bad Request` | `INVALID_ATTRIBUTE_VALUE` | `QUARANTINED` (if queued) | Item error report |
| Same `batch_id` with conflicting payload | `409 Conflict` | `BATCH_HASH_MISMATCH` | None | None |
| Database connection unavailable | `503 Service Unavailable`| `DATABASE_UNAVAILABLE` | None | Logged trace (no PII) |

---

## 7. Test Plan Matrix

| Test ID | Requirement / AC | Layer | Description / Edge Case | Expected Result |
| :--- | :--- | :--- | :--- | :--- |
| `TC-AUTH-01` | FR-018, AC-16 | Unit | Missing `X-IBJ-Tenant` header | 401 `MISSING_AUTH_HEADERS` |
| `TC-AUTH-02` | FR-018, AC-16 | Unit | Missing `X-IBJ-Timestamp` header | 401 `MISSING_AUTH_HEADERS` |
| `TC-AUTH-03` | FR-018, AC-16 | Unit | Missing `X-IBJ-Signature` header | 401 `MISSING_AUTH_HEADERS` |
| `TC-AUTH-04` | FR-018, AC-16 | Unit | Malformed hex in signature (`v1=not_hex!`) | 401 `INVALID_SIGNATURE` |
| `TC-AUTH-05` | FR-018, AC-16 | Unit | Valid signature with wrong secret | 401 `INVALID_SIGNATURE` |
| `TC-AUTH-06` | FR-018, AC-16 | Unit | Payload altered by 1 byte (whitespace or char) | 401 `INVALID_SIGNATURE` |
| `TC-AUTH-07` | FR-018, AC-16 | Unit | Timestamp 301 seconds in the past | 401 `TIMESTAMP_OUT_OF_WINDOW` |
| `TC-AUTH-08` | FR-018, AC-16 | Unit | Timestamp 301 seconds in the future | 401 `TIMESTAMP_OUT_OF_WINDOW` |
| `TC-AUTH-09` | FR-018, AC-16 | Unit | Non-numeric timestamp header | 401 `TIMESTAMP_OUT_OF_WINDOW` |
| `TC-AUTH-10` | FR-018, AC-16 | Unit | Rotation: Previous secret accepted | 202 Accepted |
| `TC-AUTH-11` | FR-018, AC-16 | Unit | Revocation: Removed/expired secret rejected | 401 `INVALID_SIGNATURE` |
| `TC-AUTH-12` | FR-018, AC-16 | Unit | Header tenant != body `tenant_id` | 403 `TENANT_MISMATCH` |
| `TC-TRANS-01` | FR-004 | Unit | Body exceeds 1MB ceiling | 413 `PAYLOAD_TOO_LARGE` |
| `TC-TRANS-02` | FR-004 | Unit | Empty body payload | 400 `MALFORMED_JSON` |
| `TC-TRANS-03` | FR-004 | Unit | Non-JSON text body | 400 `MALFORMED_JSON` |
| `TC-TRANS-04` | FR-004 | Unit | Content-Type `text/plain` | 415 `UNSUPPORTED_MEDIA_TYPE` |
| `TC-TRANS-05` | FR-004 | Unit | Duplicate keys in JSON payload | 400 `DUPLICATE_JSON_KEYS` |
| `TC-TRANS-06` | FR-004 | Unit | Unknown fields with `additionalProperties: false` | 400 `SCHEMA_VALIDATION_ERROR` |
| `TC-VAL-01` | FR-004 | Unit | Lowercase currency (`"usd"`) | 400 / Quarantined (`pattern: ^[A-Z]{3}$`) |
| `TC-VAL-02` | FR-004 | Unit | Negative price (`price_minor = -1`) | 400 / Quarantined (`minimum: 0`) |
| `TC-VAL-03` | FR-004 | Unit | Fractional price (`price_minor = 109.99`) | 400 / Quarantined (`type: integer`) |
| `TC-VAL-04` | FR-004 | Unit | Title > 250 characters | 400 / Quarantined (`maxLength: 250`) |
| `TC-VAL-05` | FR-004 | Unit | Empty string IDs (`variant_id = ""`) | 400 / Quarantined |
| `TC-VAL-06` | FR-004 | Unit | 1001 upserts in single batch | 400 / Quarantined (`maxItems: 1000`) |
| `TC-VAL-07` | FR-004 | Unit | 1001 deletes in single batch | 400 / Quarantined (`maxItems: 1000`) |
| `TC-VAL-08` | FR-011 | Unit | Wrong attribute type (`weight_g = "1200g"`) | 400 / Quarantined |
| `TC-VAL-09` | FR-011 | Unit | Zero weight (`weight_g = 0`) | 400 / Quarantined (must be > 0) |
| `TC-VAL-10` | FR-011 | Unit | String boolean (`usb_c_pd = "true"`) | 400 / Quarantined (must be bool) |
| `TC-VAL-11` | FR-005 | Unit | Inventory status `"preorder"` | Ineligible for baseline shortlist |
| `TC-VAL-12` | FR-005 | Unit | Inventory status `"unknown"` | Ineligible for baseline shortlist |
| `TC-VAL-13` | FR-005 | Unit | Published flag `false` | Ineligible for baseline shortlist |
| `TC-VAL-14` | FR-004 | Unit | Duplicate variant inside single batch | 400 / Quarantined |
| `TC-UNICODE-01`| FR-017 | Unit | Arabic title with diacritics (tashkeel/tatweel)| Normalized & stripped in FTS |
| `TC-UNICODE-02`| FR-017 | Unit | Arabic title with alef variants (أ إ آ -> ا) | Unified and searchable in FTS |
| `TC-UNICODE-03`| FR-017 | Unit | Title in NFD form | Normalized to NFC on ingestion |
| `TC-UNICODE-04`| FR-017 | Unit | Title containing emoji and RTL marks | Safely stored without corruption |
| `TC-UNICODE-05`| FR-018 | Unit | SQL metacharacters in title (`'; DROP TABLE;--`) | Stored literally via parameterized SQL |
| `TC-UNICODE-06`| FR-018, AC-07 | E2E | Title with `<script>alert(1)</script>` | Escaped on storefront, no execution |
| `TC-ORD-01` | FR-004, AC-04 | Integration | Same `batch_id`, identical payload hash | 202 Idempotent, no projection churn |
| `TC-ORD-02` | FR-004, AC-04 | Integration | Same `batch_id`, different payload hash | 409 Conflict |
| `TC-ORD-03` | FR-004, AC-04 | Integration | Single invalid item in batch of 50 | Whole batch quarantined, prior ACTIVE intact |
| `TC-ORD-04` | FR-004, AC-04 | Integration | Upsert older than stored `source_updated_at` | Stale update ignored, stats recorded |
| `TC-ORD-05` | FR-004, AC-04 | Integration | Upsert with equal timestamp, same content | No-op |
| `TC-ORD-06` | FR-004, AC-04 | Integration | Upsert with equal timestamp, different content| Existing record retained, conflict recorded |
| `TC-ORD-07` | FR-004, AC-04 | Integration | Delete sets tombstone with latest batch time | `is_tombstoned = true`, excluded from query |
| `TC-ORD-08` | FR-004, AC-04 | Integration | Upsert older than tombstone | Stays deleted |
| `TC-ORD-09` | FR-004, AC-04 | Integration | Upsert newer than tombstone | Variant restored to active projection |
| `TC-ORD-10` | FR-004, AC-04 | Integration | Delete of unknown variant | Tombstone stub created |
| `TC-ORD-11` | FR-004, AC-04 | Integration | Future timestamp skew > 300s | Whole batch quarantined |
| `TC-ORD-12` | FR-004, AC-04 | Integration | Crash / context cancellation mid-apply | Tx rolls back, zero partial projections |
| `TC-ORD-13` | FR-004, AC-04 | Property | Permutations of valid batches yield same state| Property test via `pgregory.net/rapid` |
| `TC-ORD-14` | FR-004, AC-04 | Property | Applying any batch twice equals applying once | Idempotency property test |
| `TC-ISOL-01` | FR-018, AC-16 | Integration | Same `variant_id` across tenants | Completely independent data |
| `TC-ISOL-02` | FR-018, AC-16 | Integration | Tenant B batch cannot mutate Tenant A variants| Strict isolation verified |
| `TC-ISOL-03` | FR-018, AC-16 | Integration | Application role query with crafted SQL | RLS restricts access to `current_tenant` |
| `TC-ISOL-04` | FR-018, AC-16 | Integration | Missing tenant context (`SET LOCAL` omitted) | Query returns 0 rows |
| `TC-ISOL-05` | FR-018, AC-16 | Integration | Table owner subject to `FORCE RLS` | RLS active even on table owner |
| `TC-CONC-01` | FR-004 | Integration | Concurrent batches for same tenant | Serialized cleanly by advisory lock |
| `TC-CONC-02` | FR-004 | Integration | Parallel batches for different tenants | Execute in parallel without contention |
| `TC-CONC-03` | FR-001 | Integration | Database down mid-request | 503 on ingest, baseline compose fallback |
| `TC-CONC-04` | FR-004 | Integration | Worker restart with `RECEIVED` batches | Processes pending batches cleanly |
| `TC-ELIG-01` | FR-005 | Unit | Variant exactly at staleness limit | Eligible |
| `TC-ELIG-02` | FR-005 | Unit | Variant 1 second past staleness limit | Ineligible (suppressed) |
| `TC-ELIG-03` | FR-005 | Unit | Empty eligible set | Honest empty results, no invented SKUs |
| `TC-ELIG-04` | FR-005 | Unit | Currency mismatch (req USD, variant EUR) | Ineligible |
| `TC-SEARCH-01`| FR-006 | Integration | English stemming search ("laptop" -> "laptops")| Matches correctly |
| `TC-SEARCH-02`| FR-006, FR-017| Integration | Arabic search across diacritics & alef forms | Matches correctly |
| `TC-SEARCH-03`| FR-006 | Integration | Empty query returns full category | Returns category laptops |
| `TC-SEARCH-04`| FR-006 | Integration | SQL wildcards (`%`, `_`) in search query | Safely escaped, literal search |
| `TC-SEARCH-05`| FR-006 | Integration | Overlong search query (> 256 chars) | Bounded and sanitized |
| `TC-SEARCH-06`| FR-006 | Integration | Cursor pagination stability | Identical candidate slices across pages |
| `TC-SEARCH-07`| FR-006 | Integration | Result candidate cap | Exactly <= 200 candidates |
| `TC-SEARCH-08`| FR-006 | Integration | Deterministic tie-breaking | Sorted: relevance DESC, prod ID, var ID |
| `TC-COMP-01` | FR-006, AC-04 | Integration | Compose `catalog_version` updates after apply | Reflects new active version |
| `TC-COMP-02` | FR-006, AC-04 | Integration | Compose `catalog_version` unchanged on quarantine| Prior active version preserved |
| `TC-COMP-03` | FR-001, AC-06 | Unit/Integ | Store unavailable compose fallback | Schema-valid baseline, `catalog_unavailable` |

---

## 8. Open Questions & Conservative Architectural Decisions

### Open Question 1: Single `title` Field in `catalog-batch.schema.json`
- **Fact:** `contracts/schemas/catalog-batch.schema.json` defines a single string `title` without locale tags, whereas the internal data model and storefront expect localized titles (`title`, `title_ar`).
- **Conservative Decision:** Ingest treats `title` as the merchant's canonical source-locale title. We do not synthesize or machine-translate titles. In Arabic responses, if `title_ar` is absent, the source `title` is preserved and rendered with appropriate `dir` and `lang` markup.
- **Contract Recommendation:** Propose an additive schema update to the spec repo for future review: an optional `localized_titles` map (`Record<string, string>`).

### Open Question 2: Per-Tenant Maximum Staleness Default
- **Fact:** The spec requires `last_verified_at` to be validated against the tenant's configured maximum staleness without an invented default.
- **Conservative Decision:** All tenant records in the database must explicitly declare `max_staleness_seconds`. For the demo store (`demo_store`), we explicitly configure `max_staleness_seconds = 86400` (24 hours) as an explicit demo configuration.

### Open Question 3: PostgreSQL Arabic Text Search Configuration
- **Fact:** PostgreSQL 18 provides a built-in `arabic` text search configuration.
- **Conservative Decision:** We verify `arabic` is present in `pg_ts_config`. We use `arabic` for Arabic tsvector generation while also applying application-level Unicode NFC normalization, tatweel/tashkeel stripping, and alef folding at both index time and query time.
