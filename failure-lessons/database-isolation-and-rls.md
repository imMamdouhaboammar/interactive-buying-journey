---
doc_id: IBJ-CODE-0019
title: Database Isolation, Row-Level Security & Query Optimization
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
# Database Isolation, Row-Level Security & Query Optimization

---

## 1. Superuser RLS Bypass & Missing Tenant SQL Qualifiers

### Context
PostgreSQL multi-tenant schema with `FORCE ROW LEVEL SECURITY` tested via Go integration tests (`internal/ingest/service_test.go`, `internal/storage/postgres/db_test.go`).

### What happened
During concurrent test runs across multiple tenants (`tenant_comp_*` and `tenant_bench_10k`), an assertion in `service_test.go` checking whether an active catalog version remained unchanged after a quarantined batch failed with:
```text
service_test.go:207: active version changed unexpectedly: 
got cat_tenant_bench_10k_batch_10k_bench_0004, want cat_tenant_1790487354636970000_batch_valid_init
```

### Observable symptom
An integration test dedicated to `tenantA` retrieved active version metadata belonging to `tenantB` who was executing a benchmark concurrently in another package.

### Impact
High. If multi-tenant queries rely exclusively on session RLS context without explicit SQL predicates, running as a privileged database user (or in an improperly scoped connection pool) causes silent cross-tenant data contamination.

### Incorrect assumption
The test assumed that because `db.WithTenantTx(ctx, tenantID, ...)` set `SET LOCAL app.current_tenant = $1` and `catalog_versions` had `FORCE ROW LEVEL SECURITY`, `SELECT version_id FROM catalog_versions WHERE status = 'ACTIVE'` would only return the current tenant's active version.

### Root cause
**Confirmed**. In PostgreSQL, database superusers (`postgres`) bypass Row-Level Security by default regardless of `ENABLE ROW LEVEL SECURITY` or `FORCE ROW LEVEL SECURITY` unless connected as a dedicated non-superuser role (such as `SET ROLE ibj_test_app`). The query omitted `WHERE tenant_id = $1`, so the superuser connection returned rows across all tenants, picking up whatever tenant last updated an active version.

### Why the architecture allowed it
The test connection connected as default database superuser `postgres` without dropping privileges to a restricted role, and the test verification query relied on implicit RLS filtering rather than defense-in-depth SQL predicates.

### Fix
1. Explicitly qualified all queries with `WHERE tenant_id = $1`:
   ```go
   tx.QueryRow(ctx, "SELECT version_id FROM catalog_versions WHERE tenant_id = $1 AND status = 'ACTIVE'", tenantID).Scan(&currentActive)
   ```
2. Applied the same explicit `WHERE tenant_id = $1 AND variant_id = $2` filtering across all variant inspection queries in `service_test.go`.

### Verification
Ran integration tests concurrently across `internal/ingest` and `internal/catalog` with 10,000 variants actively being written; cross-tenant version collisions dropped to zero.

### Prevention rule
**Defense-in-Depth Tenant Binding:** Every SQL query executed by the application or test harness must include `WHERE tenant_id = $1` at the statement level. Never rely on connection-level RLS as the single point of security.

### Reusable lesson
PostgreSQL RLS is an outer safety net against developer error, not a license to write unqualified queries. If connection pooling, superuser maintenance, or migrations touch the table, missing tenant predicates immediately leak cross-tenant state.

### Related code
- `internal/storage/postgres/migrations/00001_initial_catalog_schema.sql`
- `internal/ingest/service_test.go`
- `internal/catalog/postgres.go`

### Related tests
- `internal/storage/postgres/db_test.go:TestPostgres_MigrationsAndRLS/TC-ISOL-01`
- `internal/ingest/service_test.go:TestService_IngestDecisionTable/TC-ORD-03`

### Status
Resolved.

---

## 2. Table Joins on Invariant Tenant Configurations in Search Queries

### Context
Full-text search and candidate eligibility evaluation in `internal/catalog/postgres.go`.

### What happened
During the 10,000 variants performance benchmark (`TestBenchmark_10kVariants`), the query latency P95 benchmark failed with:
```text
benchmark_test.go:187: P95 latency 50.118166ms exceeds target of 50ms
```

### Observable symptom
Search queries took just over 50ms at P95 under load across 100 queries.

### Impact
Violated the latency SLA target ($\le 50\text{ ms}$ at 10k variants).

### Incorrect assumption
The implementation assumed that joining `tenants t ON t.tenant_id = v.tenant_id` was negligible because `tenants` is a small table.

### Root cause
**Confirmed**. In `Search()`, the SQL query joined `variants v` with `tenants t` to retrieve `t.max_staleness_seconds` for the staleness cutoff calculation:
```sql
SELECT ... FROM variants v
JOIN tenants t ON t.tenant_id = v.tenant_id
WHERE v.tenant_id = $1 AND v.last_verified_at >= NOW() - (t.max_staleness_seconds * INTERVAL '1 second')
```
PostgreSQL planner executed a nested loop join across thousands of variant candidate rows, adding 10–12ms of CPU evaluation time per query.

### Why the architecture allowed it
The query mixed entity-level filtering (`v.last_verified_at`) with tenant configuration data (`t.max_staleness_seconds`) in a relational join rather than leveraging single-row scalar evaluation.

### Fix
Replaced the `JOIN` with a scalar subquery:
```sql
SELECT ... FROM variants v
WHERE v.tenant_id = $1
  AND v.last_verified_at >= NOW() - ((SELECT COALESCE(max_staleness_seconds, 86400) FROM tenants WHERE tenant_id = $1) * INTERVAL '1 second')
```
PostgreSQL's cost-based optimizer plans the scalar subquery as an **InitPlan**, executing it once before scanning `variants`.

### Verification
Reran `TestBenchmark_10kVariants`. P95 latency dropped from **50.12 ms** to **39.95 ms** (a 20.3% latency reduction), passing the 50ms SLA cleanly.

### Prevention rule
**Scalar InitPlans Over Configuration Joins:** In high-throughput entity search queries, never join tenant configuration tables across row streams. Use scalar subqueries or pass tenant configuration values as query parameters.

### Reusable lesson
When querying high-cardinality tables where a filter depends on a tenant-wide configuration value, relational joins force row-by-row correlation in query planners. InitPlans reduce complexity from $O(N)$ joins to $O(1)$ scalar evaluations.

### Related code
- `internal/catalog/postgres.go`

### Related tests
- `internal/catalog/benchmark_test.go:TestBenchmark_10kVariants`

### Status
Resolved.
