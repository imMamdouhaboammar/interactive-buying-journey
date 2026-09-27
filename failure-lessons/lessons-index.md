# Master Lessons Index

> **Directory:** `failure-lessons/`  
> **Last Updated:** September 27, 2026 (Slice 2 = T02 Completion)

---

## 1. Master Index Table

| Lesson | Failure Class | Prevention Rule | System | Status | Document |
| :--- | :--- | :--- | :--- | :--- | :--- |
| **Superuser RLS Bypass & Missing Tenant SQL Qualifiers** | Multi-Tenant Data Leakage | All SQL queries must explicitly include `WHERE tenant_id = $1` as defense-in-depth, even when RLS is enabled. Non-superuser roles must be used for integration testing. | `internal/storage/postgres`, `internal/ingest` | Resolved | [`database-isolation-and-rls.md`](database-isolation-and-rls.md) |
| **Table Joins on Invariant Tenant Configurations** | Query Latency Degradation | Use scalar subquery InitPlans or application context parameters instead of row-by-row table joins for tenant-level invariants. | `internal/catalog` | Resolved | [`database-isolation-and-rls.md`](database-isolation-and-rls.md) |
| **Concurrent DDL & DML Deadlocks Across Test Packages** | Test Concurrency Collision | Enforce serial package execution (`-p 1`) when running integration tests against a shared test database; decouple DDL rollbacks from parallel test execution. | `internal/storage/postgres`, CI | Resolved | [`concurrent-ddl-and-test-isolation.md`](concurrent-ddl-and-test-isolation.md) |
| **Static Test Fixtures Colliding with Future Clock Skew** | Temporal Invariant Collision | Hardcoded test fixtures must anchor timestamps to fixed past epochs (e.g., > 1 day prior) rather than near-current times, or tests must inject synthetic Clocks. | `internal/ingest`, `testdata/feeds` | Resolved | [`temporal-invariants-and-clock-skew.md`](temporal-invariants-and-clock-skew.md) |
| **Heap Memory Metric Underflow During Benchmark Runs** | Runtime Accounting Failure | Always measure cumulative heap allocation using monotonic `TotalAlloc` delta; never subtract current `Alloc` across garbage collection boundaries. | `internal/catalog` | Resolved | [`runtime-metrics-and-memory-accounting.md`](runtime-metrics-and-memory-accounting.md) |
| **Nil Pointer Dereference on Quarantined Batch Error Return** | Nil Pointer Dereference | Functions returning `(T, error)` where `T` is a pointer must never be dereferenced when `error != nil`. Verify terminal states by querying durable storage directly. | `internal/ingest`, `test/e2e` | Resolved | [`error-handling-and-nil-dereferences.md`](error-handling-and-nil-dereferences.md) |
| **Unchecked Return Values on Deferred Resource Cleanup** | Linter/Static Analysis Failure | All deferred `Close()` calls on `io.Closer` resources must be explicitly handled or discarded with `defer func() { _ = r.Close() }()`. | `internal/httpapi`, `internal/connector/mock` | Resolved | [`testing-and-verification.md`](testing-and-verification.md) |

---

## 2. Rules We Now Enforce

These non-negotiable engineering invariants are enforced across the codebase and CI pipelines:

1. **Defense-in-Depth Tenant Isolation:**
   - PostgreSQL `FORCE ROW LEVEL SECURITY` is mandatory, but **never** relied upon as the sole barrier. Every application SQL query and test query must explicitly bind `WHERE tenant_id = $1`.
2. **Scalar InitPlans Over Configuration Joins:**
   - Never join static tenant configuration tables (`tenants`) in row-scanning search queries. Use scalar subqueries (`SELECT max_staleness_seconds FROM tenants WHERE tenant_id = $1`) so the database optimizer plans it once as an InitPlan.
3. **Database Test Seriality Under Shared Instances:**
   - Database integration suites sharing an active PostgreSQL instance must run with `go test -p 1` to prevent DDL migration rollback locks from deadlocking concurrent DML operations.
4. **Deterministic Past Epochs in Feed Fixtures:**
   - Test data fixtures must anchor `source_updated_at` timestamps strictly in the past (minimum 24 hours prior) to prevent false-positive future clock skew rejections across international timezones.
5. **Monotonic Allocation Delta Accounting:**
   - Benchmarks must measure allocation volume via `runtime.MemStats.TotalAlloc` to eliminate `uint64` underflow caused by runtime GC cycles.
6. **Zero Nil Dereferences on Domain Error Returns:**
   - Domain errors returned from transactional boundary services (such as `ErrBatchQuarantined`) guarantee that result pointers are `nil`. Callers must inspect errors first and verify side effects via durable database reads.
7. **Strict Errcheck Compliance on Defers:**
   - Every `defer resource.Close()` must explicitly handle or discard error returns using `defer func() { _ = resource.Close() }()`.
