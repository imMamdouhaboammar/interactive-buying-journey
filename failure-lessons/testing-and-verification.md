---
doc_id: IBJ-CODE-0027
title: Testing & Verification Patterns and Failure Mappings
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
# Testing & Verification Patterns and Failure Mappings

> **Scope:** Verification engineering across Slice 1 (Baseline) and Slice 2 (Catalog Ingest).

---

# Testing, Verification & Static Analysis Hygiene

---

## 1. Unchecked Return Values on Deferred Resource Cleanup (`errcheck`)

### Context
Static analysis with `golangci-lint` checking Go code quality across packages (`internal/storage/postgres`, `internal/connector/mock`, `test/e2e`).

### What happened
Running `make golangci-lint` produced 8 static analysis failures:
```text
internal/storage/postgres/db.go:69:16: Error return value of `db.Close` is not checked (errcheck)
	defer db.Close()
	              ^
internal/connector/mock/mock.go:177:23: Error return value of `resp.Body.Close` is not checked (errcheck)
	defer resp.Body.Close()
	                     ^
test/e2e/catalog_ingest_test.go:164:23: Error return value of `resp.Body.Close` is not checked (errcheck)
	defer resp.Body.Close()
	                     ^
```

### Observable symptom
`make golangci-lint` failed with exit code 2, halting the quality gate pipeline.

### Impact
Low-to-Medium. Failed CI quality gates; unchecked close operations can occasionally swallow flush/sync errors on network buffers and database connections.

### Incorrect assumption
The author assumed standard Go bare defers `defer resp.Body.Close()` were acceptable to the linter configuration.

### Root cause
**Confirmed**. The repository's `.golangci.yml` activates `errcheck`, which strictly demands that all function and method return values of type `error` be either assigned, verified, or explicitly ignored.

### Why the architecture allowed it
Bare `defer r.Close()` statements silently drop the returned error without an explicit discard token.

### Fix
Wrapped all deferred cleanup calls in anonymous closures with explicit discard assignment:
```go
defer func() { _ = resp.Body.Close() }()
defer func() { _ = db.Close() }()
```

### Verification
Ran `make golangci-lint`; exited with code 0 and zero lint warnings.

### Prevention rule
**Strict Errcheck Compliance on Defers:** Whenever deferring a call to an `io.Closer` or database connection, explicitly record or discard the error using `defer func() { _ = resource.Close() }()`.

### Reusable lesson
Strict linters distinguish intentional error dismissal (`_ = ...`) from accidental omission. Making the discard explicit documents intent and prevents automated gates from failing.

### Related code
- `.golangci.yml`
- `internal/storage/postgres/db.go`
- `internal/connector/mock/mock.go`
- `test/e2e/catalog_ingest_test.go`

### Related tests
- `make golangci-lint`

### Status
Resolved.

---

## 2. Successful Testing Patterns Proven in Slice 2

This session established and validated four testing methodologies that should be adopted across future vertical slices:

### Pattern A: Rapid Property-Based Invariant Testing
- **Location:** `internal/ingest/property_test.go:TestProperty_BatchPermutationConvergence`
- **Class of Bug Prevented:** Arrival-order dependent state corruption in asynchronous feed ingest.
- **Why It Worked:** Using `pgregory.net/rapid`, generated 100 permutations of concurrent conflicting batches across randomized tenant namespaces. Proved mathematically that applying batches in forward ($A \rightarrow B$) vs reverse ($B \rightarrow A$) arrival order converges to the exact same database projection state.
- **When to Use:** Any distributed, streaming, or queue-based projection system.

### Pattern B: Continuous Fuzzing on Untrusted Input Boundaries
- **Location:** `internal/ingest/auth_fuzz_test.go` (481k executions), `internal/ingest/parse_fuzz_test.go` (72k executions)
- **Class of Bug Prevented:** Panic-on-malformed-input, buffer overreads, and parser injection attacks.
- **Why It Worked:** Native Go fuzzing (`testing.F`) hammered `VerifyHeaders` and `ParseAndValidate` with random mutation streams, proving zero panic invariants on malformed unicode, truncated headers, and malformed JSON.
- **When to Use:** Every external HTTP endpoint or message ingestion adapter.

### Pattern C: Strict Zero-Sleep Test Concurrency
- **Invariant:** **Zero test sleeps permitted.**
- **Why It Worked:** In concurrent batch serialization tests (`TC-CONC-01`), workers synchronized via `sync.WaitGroup`, atomic counters, and PostgreSQL transaction locks rather than arbitrary `time.Sleep()`.
- **Outcome:** Eliminates flaky test failures on busy CI runners and keeps test execution times in milliseconds.

### Pattern D: Red-Green-Refactor with Separate Test Commits
- **Invariant:** Every test suite must be committed in a failing `test(...)` commit before the passing `feat(...)` commit is authored.
- **Why It Worked:** Confirmed causal failure for every single requirement (`TC-AUTH`, `TC-TRANS`, `TC-VAL`, `TC-ISOL`, `TC-ORD`, `TC-ELIG`, `TC-SEARCH`, `TC-COMP`). Prevents vacuous test passes where a test passes regardless of implementation.


---

# Slice 1 Testing Verification Baseline

# Testing and Verification Strategy

This document records the testing patterns, falsification techniques, and failure-to-test mappings proven effective during the implementation of the Interactive Buying Journey (IBJ) engine.

---

## 1. Core Testing Patterns

### A. Contract Falsification (Red-Green Schema Validation)
- **Problem**: Validating only happy-path examples leads to false confidence. Schemas that inadvertently omit `required` fields or use permissive types will still pass positive tests.
- **Pattern**: Every contract schema must be tested against both positive canonical examples and explicit negative fixtures.
- **Implementation in IBJ**:
  - Python: `tools/validate_contracts.py` validates 8 positive examples and asserts failure on 4 negative fixtures:
    1. Empty `reason_codes` in `experience-plan` (rejected: `minItems: 1`).
    2. Unknown `section kind` (e.g. `arbitrary-banner`) in `experience-plan` (rejected: enum restriction).
    3. `facet-panel` section improperly carrying `items` (rejected: items disallowed on facet panels).
    4. Enrichment record with `status: "approved"` lacking approved review details (rejected: conditional schema requirement).
  - Go: `internal/contracts/contracts_test.go` executes `TestNegativeFixturesFailValidation` using `github.com/santhosh-tekuri/jsonschema/v6`.

### B. Fail-Open DOM Isolation Testing
- **Problem**: Asserting that an error is thrown does not prove that a storefront remained visually intact.
- **Pattern**: Test that upon failure, the merchant's DOM is byte-for-byte identical before and after the SDK invocation.
- **Implementation in IBJ**:
  - `sdk/tests/client.test.ts` uses `jsdom` to take snapshot representations of `document.body.innerHTML`.
  - In 5 simulated failure modes (timeout, 500 error, malformed JSON, schema violation, unknown section kind), the test asserts:
    ```typescript
    expect(document.body.innerHTML).toBe(initialHtml);
    ```

### C. Live Dual-Locale E2E with Accessibility Gates
- **Problem**: Mocking browser behavior hides layout direction bugs (RTL), missing attribute rendering, and WCAG accessibility regressions.
- **Pattern**: Run headless Chromium against the real SSR server across all locales (`en` and `ar`), asserting both visual integrity and zero accessibility violations.
- **Implementation in IBJ**:
  - `demo-storefront/tests/e2e.spec.ts` executes Playwright tests for:
    - English LTR (`dir="ltr"`) with engine running.
    - Arabic RTL (`dir="rtl"`) with engine running.
    - English and Arabic during complete engine outage.
    - English and Arabic when engine latency exceeds client deadline (>350ms).
  - Integrates `@axe-core/playwright` (`AxeBuilder.analyze()`) requiring 0 critical, serious, moderate, or minor WCAG 2.2 AA violations.

---

## 2. Failure-to-Test Coverage Mappings

| Failure Class | Root Cause | Regression Test | Invariant Protected |
| :--- | :--- | :--- | :--- |
| **FL-001** (License rejection) | Omission of `MIT-0`/`CC0-1.0` in allowlist | `tools/check_licenses.py` (checked in CI) | Only permitted licenses in dependency tree |
| **FL-002** (Root package audit) | `go-licenses` scanning root proprietary module | `check_go_licenses()` in `tools/check_licenses.py` | CI passes with PolyForm Shield 1.0.0 root module |
| **FL-003** (Secret false positive) | Generic API key regex matching example token | Gitleaks scan in CI & `make check` | High-entropy real secrets caught without false alarms on fixtures |
| **FL-004** (E2E transport log) | Browser network errors polluting console assertion | `page.on('console')` filter in `e2e.spec.ts` | SDK emits 0 application errors during backend outages |
| **FL-005** (ESM/CJS default export) | NodeNext runtime differences on hybrid packages | `sdk/tests/client.test.ts`, `demo-storefront/tests/e2e.spec.ts` | Runtime compatibility across Bun, Node, and browser bundlers |
| **FL-006** (CLI flag ordering) | Bun CLI parsing of trailing `--cwd` | `Makefile` (`cd <dir> && bun run ...`) | Deterministic build scripts across all environments |
| **FL-007** (Storefront blanking) | SDK modifying DOM on unvalidated payloads | `sdk/tests/client.test.ts` (6 cases) | Normal merchant browsing never disrupted by engine failures |

---

## 3. When Tests Give False Confidence

1. **In-Memory Catalog vs Persistent Storage**:
   - `InMemoryCatalog` in Slice 1 does not test SQL connection pooling, transactions, migration rollbacks, or lock contention.
   - *Mitigation*: Task T02 must introduce real PostgreSQL container integration tests.
2. **Synchronous JSON Schema Compilation**:
   - Compiling schemas per-request causes latency spikes under high load.
   - *Mitigation*: `internal/contracts/validator.go` pre-compiles and caches schema instances at startup.
3. **Local Chrome vs Containerized Headless Chromium**:
   - Playwright on macOS may use installed Google Chrome with different font rendering than Linux CI runners.
   - *Mitigation*: CI pins headless Chromium via `playwright install --with-deps chromium` and validates structural DOM and accessibility rather than raw pixel diffs.
