# Master Lessons Index

> **Directory:** `failure-lessons/`  
> **Coverage:** Slices 1 & 2 (Baseline & Catalog Ingest)

---

## 1. Slice 2 Lessons (Catalog Ingest & Storage)

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

## 2. Slice 1 Lessons (Baseline & Storefront SDK)

| Lesson | Failure Class | Prevention Rule | System | Status | Document |
| :--- | :--- | :--- | :---: | :--- | :--- |
| **FL-001** | Zero-Attribution Permissive Dependency Rejection | Permissive license allowlists must include zero-attribution public-domain variants (`MIT-0`, `0BSD`, `CC0-1.0`). | CI / License Audit | Resolved | [`dependency-licensing.md`](dependency-licensing.md) |
| **FL-002** | Root Source-Available Package Flagged by OSS Audit | Dependency license checkers must ignore the root repository module when using proprietary or source-available licenses. | CI / Go Toolchain | Resolved | [`dependency-licensing.md`](dependency-licensing.md) |
| **FL-003** | Secret Scanner False Positive on Contract Examples | Contract fixtures with synthetic auth/token fields must pair with scoped repo allowlists, never disabling scanners globally. | Security / Gitleaks | Resolved | [`secret-scanning-and-fixtures.md`](secret-scanning-and-fixtures.md) |
| **FL-004** | E2E Console Assertion Tripped by Native Transport Outage | Browser E2E tests for network outages must isolate browser transport logs (`net::ERR_*`) from application-level runtime errors. | E2E Testing / Playwright | Resolved | [`browser-e2e-and-network-failures.md`](browser-e2e-and-network-failures.md) |
| **FL-005** | Hybrid CJS/ESM Default Export Resolution Failure | TypeScript NodeNext builds targeting multiple runtimes must defensively unwrap default exports (`pkg.default || pkg`). | Storefront SDK / Toolchain | Resolved | [`typescript-runtime-interop.md`](typescript-runtime-interop.md) |
| **FL-006** | Workspace CLI Flag Ordering Incompatibility | Multi-package workspace scripts must use POSIX subshell execution (`cd <dir> && <cmd>`) rather than trailing directory flags. | Build / Workspace | Resolved | [`typescript-runtime-interop.md`](typescript-runtime-interop.md) |
| **FL-007** | Storefront Degradation Vulnerability Under Engine Failure | Client SDKs must never mutate, reflow, or blank merchant DOM when engine times out, fails with 5xx, or returns invalid schema. | Storefront SDK / Engine | Resolved | [`fail-open-resilience.md`](fail-open-resilience.md) |
