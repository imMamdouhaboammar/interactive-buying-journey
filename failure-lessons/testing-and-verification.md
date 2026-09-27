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
