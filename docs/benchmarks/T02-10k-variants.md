---
doc_id: IBJ-CODE-0009
title: T02 10,000 Variants Ingestion and Query Benchmark Report
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
# T02 10,000 Variants Ingestion and Query Benchmark Report

> **Date:** September 27, 2026  
> **Status:** PASSED (All Targets Met)  
> **Target System:** PostgreSQL 18.4 + IBJ Catalog Engine (`internal/catalog`, `internal/ingest`)

---

## 1. Hardware & Environment Specifications

| Component | Specification |
| :--- | :--- |
| **Processor** | Apple M4 (ARM64) |
| **Memory** | 16 GB Unified Memory (17,179,869,184 bytes) |
| **Operating System** | macOS 26.6.2 (Darwin 25.6.2) |
| **Database** | PostgreSQL 18.4 (Homebrew) |
| **Go Runtime** | Go 1.26.5 darwin/arm64 |
| **Multi-Tenancy** | PostgreSQL Row-Level Security (`FORCE ROW LEVEL SECURITY`) with `SET LOCAL app.current_tenant` |

---

## 2. Test Parameters & Methodology

- **Synthetic Generator:** `internal/connector/mock.GenerateBatches`
- **Total Variants Ingested:** 10,000 unique SKUs
- **Batch Size:** 1,000 variants per batch (10 batches total, conforming to Draft 2020-12 `maxItems: 1000` schema constraint)
- **Attribute Profile:**
  - Product Category: `laptops`
  - Typed Attributes: `weight_g` (1100–1700g), `battery_wh` (50–100Wh), `usb_c_pd: true`
  - Dual Bilingual Titles: English & Normalized Arabic (`Apex Pro Model X / حاسوب Apex برو X`)
  - Inventory: `in_stock`
  - Currencies: `USD`
- **Ingestion Pipeline:**
  1. JSON streaming token validation & duplicate key detection
  2. Draft 2020-12 Schema validation
  3. Unicode NFC normalization & Arabic diacritic/tashkeel stripping
  4. Tenant advisory lock acquisition (`hashtext(tenant_id)`)
  5. Transactional batch insert (`feed_batches`)
  6. Variant upsert with conflict resolution & bilingual TSVECTOR generation (`tsv_en`, `tsv_ar`)
  7. Catalog version creation and promotion to `ACTIVE`
- **Query Distribution (100 Queries across 10,000 variants):**
  - English stemmed queries (`Apex`, `Nova`, `Titan`, `Vision`, `Pro`, `Model`, `laptop`)
  - Arabic normalized queries (`حاسوب`, `برو`, `موديل`)
  - Randomized budget bounds (`max_budget_minor` between $1,000.00 and $3,000.00)
  - Limit: 20 variants per search page

---

## 3. Benchmark Results

### 3.1 Ingestion Throughput

| Metric | Measured Value | SLA / Target | Status |
| :--- | :--- | :--- | :--- |
| **Total Ingest Duration** | **8.27s** | — | — |
| **Ingestion Throughput** | **1,209.29 variants/sec** | **>= 1,000 variants/sec** | **PASS** |
| **Cumulative Heap Allocation** | **156.32 MB** | — | — |
| **Active Heap In-Use** | **6.11 MB** | < 100 MB | **PASS** |
| **Quarantine & Poison Defense** | Verified (0 rows leaked) | Zero data corruption | **PASS** |

### 3.2 Query Latency (100 Search Queries @ 10,000 Variants)

| Percentile | Measured Latency | SLA / Target | Status |
| :--- | :--- | :--- | :--- |
| **Min Latency** | **2.63 ms** | — | — |
| **P50 Latency (Median)** | **22.95 ms** | <= 30 ms | **PASS** |
| **P95 Latency** | **39.95 ms** | **<= 50 ms** | **PASS** |
| **P99 Latency** | **82.88 ms** | < 120 ms | **PASS** |
| **Max Latency** | **82.88 ms** | — | — |

---

## 4. Analysis & Bottleneck Identification

1. **Ingest Phase Analysis:**
   - **Throughput:** Ingestion exceeded the target throughput at **1,209 variants/sec** (exceeding the >= 1,000/sec target by 20.9%).
   - **Advisory Locks & RLS:** Tenant advisory locks (`pg_advisory_xact_lock`) serialized batch execution per tenant without deadlocks or contention.
   - **GIN Indexing:** PostgreSQL GIN indexes on `tsv_en` and `tsv_ar` updated synchronously during batch upsert. Using stored generated columns for `tsv_en` minimized Go-side expression evaluation.

2. **Query Latency Analysis:**
   - **Subquery InitPlan Optimization:** Eliminating the `JOIN tenants t ON t.tenant_id = v.tenant_id` in favor of an optimized subquery `(SELECT COALESCE(max_staleness_seconds, 86400) FROM tenants WHERE tenant_id = $1)` reduced query planning and execution overhead, dropping P95 latency from 50.1ms to 39.95ms.
   - **Deterministic Tie-Breaking:** `ts_rank DESC, product_id ASC, variant_id ASC` maintained millisecond predictability even when filtering 10k rows.

3. **Future Scaling Optimization Opportunities (for Slice 10 / T10):**
   - **Prepared Statements / Statement Caching:** Query parsing and planning overhead can be eliminated by enabling pgx prepared statements for `Search` queries.
   - **Partial / Covering Indexes:** Adding `INCLUDE (title, price_minor, currency, attributes)` on `idx_variants_eligibility` will allow index-only scans for catalog listing queries.
   - **Asynchronous Ingest Workers:** For 100k+ variants, partitioning batches into async worker pools with buffered copy (`pgx.CopyFrom`) will further elevate ingest throughput to > 5,000 variants/sec.
