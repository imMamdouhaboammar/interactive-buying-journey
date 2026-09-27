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
