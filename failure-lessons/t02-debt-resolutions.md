---
doc_id: IBJ-CODE-0029
title: T02 Technical Debt Audit and Invariant Hardening
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
# T02 Technical Debt Audit and Invariant Hardening

This document captures lessons learned from auditing and hardening Slice 2 implementation debts prior to Slice 3 development.

---

## FL-008: Out-of-Stock Variants Surfacing in Catalog Search (FR-005 Violation)

### Context
Catalog search query in `internal/catalog/postgres.go` filtering candidate variants for recommendation strips and search responses.

### What happened
Catalog search queries returned variants regardless of inventory availability, surfacing variants marked `out_of_stock` or `backorder` to buyers.

### Observable symptom
Test `test: out-of-stock variants must not surface in catalog search` proved that `inventory_status != 'in_stock'` variants appeared in result sets.

### Impact
Violated core product invariant FR-005 ("Commerce Safety: Never recommend or display out-of-stock items as purchasable").

### Root cause
The SQL query in `SearchVariants` joined `variants v` and `products p` without filtering on `v.inventory_status = 'in_stock'`.

### Fix
Added `AND v.inventory_status = 'in_stock'` to the candidate filter in `internal/catalog/postgres.go`.

---

## FL-009: Misleading TotalCount Field Reporting Page Size Rather Than Population

### Context
`SearchResult` struct in `internal/catalog/catalog.go` returning metadata alongside items.

### What happened
The struct included a field named `TotalCount` which was populated with `len(variants)` (the number of items in the current page/slice), not the total number of matching items in the catalog.

### Observable symptom
Test `test: prove TotalCount field misleadingly reports returned page size` proved that pagination consumers could misinterpret `TotalCount` as the total catalog match count.

### Impact
Misleading pagination metadata leading to broken client pagination logic or inaccurate storefront counters.

### Root cause
Semantic confusion between slice length (`ReturnedCount`) and database total match count (`TotalMatchingCount`).

### Fix
Renamed `TotalCount` to `ReturnedCount` across `internal/catalog/catalog.go`, `internal/catalog/postgres.go`, and dependent tests.

---

## FL-010: Silent Defaulting of Tenant Max Staleness and Non-Injected Database Clock

### Context
Tenant freshness verification in `internal/catalog/postgres.go` against `tenants.max_staleness_seconds`.

### What happened
1. Migration `00001_initial_schema.sql` applied a default `max_staleness_seconds INT NOT NULL DEFAULT 86400`. If a tenant had unconfigured staleness, it silently defaulted to 24 hours.
2. The freshness check executed SQL `v.last_verified_at >= NOW() - ...`, making temporal logic untestable without altering host system time.

### Observable symptom
Tests `TC-DEBT-02` and `TC-DEBT-03` proved that unconfigured tenants silently passed with 86400s and that synthetic clocks could not control staleness evaluation.

### Impact
1. Silent default could serve stale catalog data for tenants expecting real-time synchronization.
2. Production clock skew or temporal test flakiness could not be deterministically evaluated.

### Root cause
Missing explicit tenant configuration enforcement and direct invocation of non-deterministic SQL `NOW()`.

### Fix
1. Migration `00002_require_tenant_staleness.sql` dropped the default and made column nullable.
2. If `max_staleness_seconds` is NULL, variants are marked ineligible and a warning log is emitted.
3. Injected `Clock` parameter `$2::timestamptz` into the query.

---

## FL-011: Plaintext HMAC Secrets Stored in Tenant Configuration Tables

### Context
Tenant authentication secrets for catalog ingest webhook verification.

### What happened
The `tenants` table contained columns `secret_current` and `secret_previous` storing plaintext HMAC secret strings directly in the database.

### Observable symptom
Test `TC-DEBT-04` proved that plaintext secrets were readable directly by any database query with access to the tenants table.

### Impact
Severe security risk: database leaks or SQL injection could compromise HMAC keys for all tenants.

### Root cause
Early prototype schema stored raw credentials in relational tables instead of key references delegating to a secrets manager.

### Fix
1. Migration `00003_drop_plaintext_secrets_store_key_reference.sql` dropped `secret_current` and `secret_previous` and added `secret_key_ref VARCHAR(128)`.
2. Introduced `internal/secret/secret.go` providing `SecretProvider` interface with `EnvSecretProvider` and `FileSecretProvider` implementations.
3. Updated `cmd/ibj-feed` to resolve secrets dynamically via references.

---

## FL-012: Toolchain Version and Unused Dependency Drift Across Documentation and CI

### Context
Repository configuration files: `.github/workflows/ci.yml`, `AGENTS.md`, and `docs/decisions/0005-t02-persistence-and-ingest.md`.

### What happened
1. CI ran `postgres:16-alpine` while ADR-0005 specified PostgreSQL 17 as standard.
2. `AGENTS.md` declared Redis as part of the target stack, though no Redis dependency or code existed.

### Observable symptom
Test `TC-DEBT-06` proved inconsistencies across CI container image, `AGENTS.md`, and ADR-0005.

### Impact
Confusion for developers and CI/CD divergence from architectural decisions.

### Root cause
Documentation and CI configuration were not updated when ADR-0005 formalized PostgreSQL 17 and rejected Redis.

### Fix
Aligned CI container to `postgres:17-alpine`, removed Redis from `AGENTS.md`, and pinned PostgreSQL 17.
