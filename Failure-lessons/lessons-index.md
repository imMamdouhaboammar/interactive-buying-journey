# Lessons Index & Enforced Rules

This index summarizes all codified failure classes across the Interactive Buying Journey (IBJ) engine and toolchain.

---

## Lessons Index

| Lesson | Failure Class | Prevention Rule | System | Status | Document |
| :--- | :--- | :--- | :--- | :---: | :--- |
| **FL-001** | Zero-Attribution Permissive Dependency Rejection | Permissive license allowlists must include zero-attribution public-domain variants (`MIT-0`, `0BSD`, `CC0-1.0`). | CI / License Audit | Resolved | [`dependency-licensing.md`](file:///Users/mamdouhaboammar/Documents/interactive-buying-journey/Failure-lessons/dependency-licensing.md) |
| **FL-002** | Root Source-Available Package Flagged by OSS Audit | Dependency license checkers must ignore the root repository module when using proprietary or source-available licenses. | CI / Go Toolchain | Resolved | [`dependency-licensing.md`](file:///Users/mamdouhaboammar/Documents/interactive-buying-journey/Failure-lessons/dependency-licensing.md) |
| **FL-003** | Secret Scanner False Positive on Contract Examples | Contract fixtures with synthetic auth/token fields must pair with scoped repo allowlists, never disabling scanners globally. | Security / Gitleaks | Resolved | [`secret-scanning-and-fixtures.md`](file:///Users/mamdouhaboammar/Documents/interactive-buying-journey/Failure-lessons/secret-scanning-and-fixtures.md) |
| **FL-004** | E2E Console Assertion Tripped by Native Transport Outage | Browser E2E tests for network outages must isolate browser transport logs (`net::ERR_*`) from application-level runtime errors. | E2E Testing / Playwright | Resolved | [`browser-e2e-and-network-failures.md`](file:///Users/mamdouhaboammar/Documents/interactive-buying-journey/Failure-lessons/browser-e2e-and-network-failures.md) |
| **FL-005** | Hybrid CJS/ESM Default Export Resolution Failure | TypeScript NodeNext builds targeting multiple runtimes must defensively unwrap default exports (`pkg.default || pkg`). | Storefront SDK / Toolchain | Resolved | [`typescript-runtime-interop.md`](file:///Users/mamdouhaboammar/Documents/interactive-buying-journey/Failure-lessons/typescript-runtime-interop.md) |
| **FL-006** | Workspace CLI Flag Ordering Incompatibility | Multi-package workspace scripts must use POSIX subshell execution (`cd <dir> && <cmd>`) rather than trailing directory flags. | Build / Workspace | Resolved | [`typescript-runtime-interop.md`](file:///Users/mamdouhaboammar/Documents/interactive-buying-journey/Failure-lessons/typescript-runtime-interop.md) |
| **FL-007** | Storefront Degradation Vulnerability Under Engine Failure | Client SDKs must never mutate, reflow, or blank merchant DOM when engine times out, fails with 5xx, or returns invalid schema. | Storefront SDK / Engine | Resolved | [`fail-open-resilience.md`](file:///Users/mamdouhaboammar/Documents/interactive-buying-journey/Failure-lessons/fail-open-resilience.md) |

---

## Rules We Now Enforce

1. **Fail-Open Presentation, Fail-Closed Commerce:**
   If any API, model, or ranking heuristic fails, times out, or returns an unvalidated payload, the client SDK must leave the merchant baseline storefront 100% untouched. For pricing, discounts, and inventory, execution must fail-closed.
2. **Contract Falsification Over Positive Confirmation:**
   A schema or API contract is not verified until intentionally malformed negative fixtures are proven to be rejected by the validation pipeline.
3. **Scoped Security Allowlists Over Global Disabling:**
   Never disable secret detection or license compliance globally to silence false positives; target exact relative file paths and specific synthetic token strings.
4. **Zero Attributed PII in Ingest & Logs:**
   Session logs must never record buyer IDs, raw IP addresses, or un-sanitized user prompts. Only tenant and request correlation IDs are permitted.
5. **Cross-Platform Transitive Dependency Parity:**
   Never assume Linux CI dependency trees match local macOS developer workstations. Validate licenses and builds against clean containerized/virtualized environments.
6. **Defensive ESM/CJS Runtime Interop:**
   Always use defensive unwrapping (`(pkg as any).default || pkg`) when importing CommonJS dependencies into TypeScript projects configured with native ESM (`NodeNext`).
7. **POSIX Subshell Directory Discipline:**
   Use `cd <dir> && bun run <script>` in Makefiles and CI pipelines to guarantee deterministic execution across package manager version variations.
